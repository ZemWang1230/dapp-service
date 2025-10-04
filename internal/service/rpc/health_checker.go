package rpc

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/redis"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/ethclient"
)

// HealthChecker RPC健康和能力统一检查器
type HealthChecker struct {
	config   *config.RPCPoolConfig
	rpcCache *redis.RPCCache
}

// NewHealthChecker 创建统一检查器
func NewHealthChecker(config *config.RPCPoolConfig, rpcCache *redis.RPCCache) *HealthChecker {
	return &HealthChecker{
		config:   config,
		rpcCache: rpcCache,
	}
}

// CheckHealth 检查单个RPC的健康状态和能力（统一检查）
func (hc *HealthChecker) CheckHealth(ctx context.Context, rpcURL string, chainID int) (*types.HealthCheckResult, error) {
	startTime := time.Now()

	result := &types.HealthCheckResult{
		URL:       rpcURL,
		ChainID:   chainID,
		CheckedAt: startTime,
	}

	// 创建带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second) // 增加超时时间以进行能力测试
	defer cancel()

	// 尝试连接RPC
	client, err := ethclient.DialContext(checkCtx, rpcURL)
	if err != nil {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("failed to connect: %v", err)
		logger.Debug("RPC check failed - connection", "url", rpcURL, "chain_id", chainID, "error", err)
		return result, nil
	}
	defer client.Close()

	// 测试基本RPC调用 - 获取最新区块号
	_, err = client.BlockNumber(checkCtx)
	if err != nil {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("failed to get block number: %v", err)
		logger.Debug("RPC check failed - block number", "url", rpcURL, "chain_id", chainID, "error", err)
		return result, nil
	}

	// 验证链ID
	actualChainID, err := client.ChainID(checkCtx)
	if err != nil {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("failed to get chain ID: %v", err)
		logger.Debug("RPC check failed - chain ID", "url", rpcURL, "chain_id", chainID, "error", err)
		return result, nil
	}

	if actualChainID.Int64() != int64(chainID) {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("chain ID mismatch: expected %d, got %d", chainID, actualChainID.Int64())
		logger.Debug("RPC check failed - chain ID mismatch", "url", rpcURL, "expected", chainID, "actual", actualChainID.Int64())
		return result, nil
	}

	// 测试FilterLogs能力，确定MaxSafeRange
	maxSafeRange := hc.testFilterLogsCapability(checkCtx, client)
	result.MaxSafeRange = maxSafeRange

	// 健康检查通过
	result.IsHealthy = true
	result.ResponseTime = time.Since(startTime)

	logger.Debug("RPC check passed", "url", rpcURL, "chain_id", chainID, "max_safe_range", maxSafeRange, "response_time", result.ResponseTime)
	return result, nil
}

// testFilterLogsCapability 测试RPC的FilterLogs能力，返回MaxSafeRange
func (hc *HealthChecker) testFilterLogsCapability(ctx context.Context, client *ethclient.Client) int {
	// 测试范围序列：从大到小
	testRanges := []int{50000, 2000, 500, 100}

	for _, rangeSize := range testRanges {
		// 为每个测试创建独立的超时上下文
		testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		success := hc.testRangeSize(testCtx, client, rangeSize)
		cancel()

		// 如果成功，这就是最大安全范围
		if success {
			return rangeSize
		}

		// 测试间隔，避免过于频繁
		time.Sleep(2 * time.Second)

		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return 100 // 如果被取消，返回最小值
		default:
		}
	}

	// 所有测试都失败，返回最小值
	return 100
}

// testRangeSize 测试指定范围大小的FilterLogs调用
func (hc *HealthChecker) testRangeSize(ctx context.Context, client *ethclient.Client, rangeSize int) bool {
	// 使用安全的起始区块，避免区块1可能的问题
	fromBlock := int64(10000)
	toBlock := fromBlock + int64(rangeSize) - 1

	// 创建通用的FilterQuery
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(fromBlock),
		ToBlock:   big.NewInt(toBlock),
	}

	// 执行FilterLogs调用
	_, err := client.FilterLogs(ctx, query)
	return err == nil
}

// UpdateMetadataFromHealthCheck 根据统一检查结果更新RPC元数据
func (hc *HealthChecker) UpdateMetadataFromHealthCheck(ctx context.Context, result *types.HealthCheckResult) error {
	// 获取现有元数据
	metadata, err := hc.rpcCache.GetRPCMetadata(ctx, result.ChainID, result.URL)
	if err != nil {
		return fmt.Errorf("failed to get existing metadata: %w", err)
	}

	// 如果不存在元数据，创建新的
	if metadata == nil {
		metadata = &redis.RPCMetadata{
			URL:           result.URL,
			ChainID:       result.ChainID,
			MaxSafeRange:  result.MaxSafeRange,
			LastCheckedAt: result.CheckedAt,
		}
	}

	// 更新健康状态和能力信息
	metadata.IsHealthy = result.IsHealthy
	metadata.LastCheckedAt = result.CheckedAt
	metadata.ResponseTimeMs = result.ResponseTime.Milliseconds()

	// 更新MaxSafeRange（统一检查时已测试）
	if result.IsHealthy && result.MaxSafeRange > 0 {
		metadata.MaxSafeRange = result.MaxSafeRange
	}

	if result.IsHealthy {
		// 健康状态良好，重置错误计数
		metadata.ErrorCount = 0
		metadata.LastError = nil
	} else {
		// 健康状态不好，增加错误计数
		metadata.ErrorCount++
		metadata.LastError = &result.Error
	}

	// 保存更新的元数据
	if err := hc.rpcCache.SetRPCMetadata(ctx, metadata); err != nil {
		return fmt.Errorf("failed to save updated metadata: %w", err)
	}

	return nil
}

// CheckAllRPCs 检查指定链的所有RPC健康状态
func (hc *HealthChecker) CheckAllRPCs(ctx context.Context, chainID int, rpcURLs []string) ([]*types.HealthCheckResult, error) {
	if len(rpcURLs) == 0 {
		return nil, fmt.Errorf("no RPC URLs provided for chain %d", chainID)
	}

	results := make([]*types.HealthCheckResult, len(rpcURLs))

	// 并发检查所有RPC
	resultChan := make(chan struct {
		index  int
		result *types.HealthCheckResult
		error  error
	}, len(rpcURLs))

	for i, rpcURL := range rpcURLs {
		go func(index int, url string) {
			result, err := hc.CheckHealth(ctx, url, chainID)
			resultChan <- struct {
				index  int
				result *types.HealthCheckResult
				error  error
			}{index, result, err}
		}(i, rpcURL)
	}

	// 收集结果
	healthyCount := 0
	for i := 0; i < len(rpcURLs); i++ {
		select {
		case res := <-resultChan:
			if res.error != nil {
				logger.Error("Health check failed", res.error, "url", rpcURLs[res.index], "chain_id", chainID)
				// 创建一个失败的结果
				results[res.index] = &types.HealthCheckResult{
					URL:       rpcURLs[res.index],
					ChainID:   chainID,
					IsHealthy: false,
					Error:     res.error.Error(),
					CheckedAt: time.Now(),
				}
			} else {
				results[res.index] = res.result
				if res.result.IsHealthy {
					healthyCount++
				}
			}

			// 更新元数据
			if err := hc.UpdateMetadataFromHealthCheck(ctx, results[res.index]); err != nil {
				logger.Error("Failed to update metadata from health check", err, "url", results[res.index].URL)
			}

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return results, nil
}
