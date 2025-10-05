package rpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/redis"

	"github.com/ethereum/go-ethereum/ethclient"
)

// Pool RPC连接池
type Pool struct {
	chainID       int
	rpcURLs       []string
	clients       map[string]*ethclient.Client
	config        *config.RPCPoolConfig
	rpcCache      *redis.RPCCache
	healthChecker *HealthChecker // 统一的健康和能力检查器

	// 控制
	mutex     sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
	isRunning bool

	// 维护任务
	healthCheckTicker *time.Ticker
}

// NewPool 创建RPC连接池
func NewPool(chainID int, rpcURLs []string, config *config.RPCPoolConfig, rpcCache *redis.RPCCache) *Pool {
	healthChecker := NewHealthChecker(config, rpcCache)

	return &Pool{
		chainID:       chainID,
		rpcURLs:       rpcURLs,
		clients:       make(map[string]*ethclient.Client),
		config:        config,
		rpcCache:      rpcCache,
		healthChecker: healthChecker,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动RPC连接池
func (p *Pool) Start(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isRunning {
		return fmt.Errorf("RPC pool for chain %d is already running", p.chainID)
	}

	logger.Info("Starting RPC pool", "chain_id", p.chainID, "rpc_count", len(p.rpcURLs))

	// 初始化所有RPC连接
	if err := p.initializeRPCs(ctx); err != nil {
		return fmt.Errorf("failed to initialize RPCs: %w", err)
	}

	// 执行初始统一检查（健康+能力）
	if err := p.performInitialCheck(ctx); err != nil {
		logger.Error("Initial check failed", err, "chain_id", p.chainID)
	}

	// 初始化FIFO队列：将所有RPC按顺序放入队列（不管健康与否）
	// 通过容错机制（3次RPC切换）来保证系统稳定性
	for url := range p.clients {
		if err := p.rpcCache.PushRPCToFIFOQueue(ctx, p.chainID, url); err != nil {
			logger.Error("Failed to push RPC to FIFO queue", err, "url", url, "chain_id", p.chainID)
		} else {
			logger.Debug("Pushed RPC to FIFO queue", "url", url, "chain_id", p.chainID)
		}
	}

	// 启动维护任务（只需要一个检查任务）
	p.startMaintenanceTasks()

	p.isRunning = true
	logger.Info("RPC pool started successfully", "chain_id", p.chainID)
	return nil
}

// Stop 停止RPC连接池
func (p *Pool) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.isRunning {
		return
	}

	logger.Info("Stopping RPC pool", "chain_id", p.chainID)

	// 停止维护任务
	p.stopMaintenanceTasks()

	// 关闭所有RPC连接
	for url, client := range p.clients {
		if client != nil {
			client.Close()
			logger.Debug("Closed RPC client", "chain_id", p.chainID, "url", url)
		}
	}
	p.clients = make(map[string]*ethclient.Client)

	p.isRunning = false
	logger.Info("RPC pool stopped", "chain_id", p.chainID)
}

// GetHealthyClient 获取一个健康的RPC客户端
func (p *Pool) GetHealthyClient(ctx context.Context) (*ethclient.Client, string, error) {
	p.mutex.RLock()
	maxAttempts := len(p.rpcURLs) // 最多尝试队列中所有的 RPC
	p.mutex.RUnlock()

	if maxAttempts == 0 {
		return nil, "", fmt.Errorf("no RPC URLs available for chain %d", p.chainID)
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 从FIFO队列取出一个RPC
		rpcURL, err := p.rpcCache.PopRPCFromFIFOQueue(ctx, p.chainID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get RPC from fifo queue: %w", err)
		}

		// 检查 errorCount
		metadata, err := p.rpcCache.GetRPCMetadata(ctx, p.chainID, rpcURL)
		if err != nil {
			logger.Warn("Failed to get RPC metadata, will try to use it anyway", "url", rpcURL, "chain_id", p.chainID, "error", err)
			// 如果获取元数据失败，仍然尝试使用该 RPC
		} else if metadata != nil && metadata.ErrorCount > 3 {
			// 如果 errorCount > 3，放回队列尾部，尝试下一个
			if pushErr := p.rpcCache.PushRPCToFIFOQueue(ctx, p.chainID, rpcURL); pushErr != nil {
				logger.Error("Failed to push RPC back to queue", pushErr, "url", rpcURL, "chain_id", p.chainID)
			}
			continue
		}

		// 获取或创建客户端
		client, err := p.getOrCreateClient(rpcURL)
		if err != nil {
			// 如果获取客户端失败，放回队列并尝试下一个
			if pushErr := p.rpcCache.PushRPCToFIFOQueue(ctx, p.chainID, rpcURL); pushErr != nil {
				logger.Error("Failed to push RPC back to queue", pushErr, "url", rpcURL, "chain_id", p.chainID)
			}
			continue
		}

		return client, rpcURL, nil
	}

	return nil, "", fmt.Errorf("no available RPC after checking all %d RPCs in queue for chain %d", maxAttempts, p.chainID)
}

// ExecuteWithRetry 带重试机制执行RPC调用
func (p *Pool) ExecuteWithRetry(ctx context.Context, fn func(*ethclient.Client) error) error {
	var lastErr error
	rpcSwitchCount := 0

	for rpcSwitchCount < p.config.MaxRPCSwitchCount {
		// 获取健康的RPC客户端
		client, rpcURL, err := p.GetHealthyClient(ctx)
		if err != nil {
			lastErr = err
			rpcSwitchCount++
			logger.Warn("Failed to get healthy RPC client", "chain_id", p.chainID, "attempt", rpcSwitchCount, "error", err)
			continue
		}

		// 对单个RPC进行重试
		retryErr := p.executeWithSingleRPCRetry(ctx, client, rpcURL, fn)
		if retryErr == nil {
			// 执行成功
			return nil
		}

		lastErr = retryErr
		rpcSwitchCount++
		logger.Warn("RPC execution failed, switching to next RPC", "chain_id", p.chainID, "url", rpcURL, "switch_count", rpcSwitchCount, "error", retryErr)
	}

	return fmt.Errorf("RPC execution failed after %d RPC switches: %w", rpcSwitchCount, lastErr)
}

// ExecuteWithRPCInfoDo 使用带RPC信息的执行（单轮重试，返回所用RPC与其maxSafeRange，并将maxSafeRange传入回调）
func (p *Pool) ExecuteWithRPCInfoDo(ctx context.Context, fn func(*ethclient.Client, int) error) (string, int, error) {
	var lastErr error
	var usedRPCURL string
	var maxSafeRange int
	rpcSwitchCount := 0

	for rpcSwitchCount < p.config.MaxRPCSwitchCount {
		client, rpcURL, err := p.GetHealthyClient(ctx)
		if err != nil {
			lastErr = err
			rpcSwitchCount++
			logger.Warn("Failed to get healthy RPC client", "chain_id", p.chainID, "attempt", rpcSwitchCount, "error", err)
			continue
		}

		usedRPCURL = rpcURL

		// 获取该RPC的安全范围
		metadata, err := p.rpcCache.GetRPCMetadata(ctx, p.chainID, rpcURL)
		if err != nil {
			logger.Warn("Failed to get RPC metadata", "chain_id", p.chainID, "url", rpcURL, "error", err)
			maxSafeRange = 0
		} else if metadata != nil {
			maxSafeRange = metadata.MaxSafeRange
		} else {
			maxSafeRange = 0
		}

		// 在该RPC上按单RPC重试策略执行（把maxSafeRange传入）
		retryErr := p.executeWithSingleRPCRetry(ctx, client, rpcURL, func(c *ethclient.Client) error {
			return fn(c, maxSafeRange)
		})
		if retryErr == nil {
			return usedRPCURL, maxSafeRange, nil
		}

		lastErr = retryErr
		rpcSwitchCount++
		logger.Warn("RPC execution failed, switching to next RPC", "chain_id", p.chainID, "url", rpcURL, "switch_count", rpcSwitchCount, "error", retryErr)
	}

	return usedRPCURL, maxSafeRange, fmt.Errorf("RPC execution failed after %d RPC switches: %w", rpcSwitchCount, lastErr)
}

// ExecuteWithRPCInfoDoInfiniteRetry 使用带RPC信息的执行（无限轮重试），并将maxSafeRange传入回调
func (p *Pool) ExecuteWithRPCInfoDoInfiniteRetry(ctx context.Context, fn func(*ethclient.Client, int) error) (string, int, error) {
	var lastErr error
	var usedRPCURL string
	var maxSafeRange int
	attemptCount := 0

	for {
		select {
		case <-ctx.Done():
			return usedRPCURL, maxSafeRange, ctx.Err()
		default:
		}

		attemptCount++
		rpcSwitchCount := 0

		for rpcSwitchCount < p.config.MaxRPCSwitchCount {
			client, rpcURL, err := p.GetHealthyClient(ctx)
			if err != nil {
				lastErr = err
				rpcSwitchCount++
				logger.Warn("Failed to get healthy RPC client", "chain_id", p.chainID, "attempt", rpcSwitchCount, "error", err)
				if rpcSwitchCount >= p.config.MaxRPCSwitchCount {
					break
				}
				continue
			}

			usedRPCURL = rpcURL

			metadata, err := p.rpcCache.GetRPCMetadata(ctx, p.chainID, rpcURL)
			if err != nil {
				logger.Warn("Failed to get RPC metadata", "chain_id", p.chainID, "url", rpcURL, "error", err)
				maxSafeRange = 0
			} else if metadata != nil {
				maxSafeRange = metadata.MaxSafeRange
			} else {
				maxSafeRange = 0
			}

			retryErr := p.executeWithSingleRPCRetry(ctx, client, rpcURL, func(c *ethclient.Client) error {
				return fn(c, maxSafeRange)
			})
			if retryErr == nil {
				return usedRPCURL, maxSafeRange, nil
			}

			lastErr = retryErr
			rpcSwitchCount++
			logger.Warn("RPC execution failed, switching to next RPC", "chain_id", p.chainID, "url", rpcURL, "switch_count", rpcSwitchCount, "error", retryErr)
		}

		backoffDuration := p.calculateBackoffDuration(attemptCount)
		logger.Info("All RPCs failed in this round, waiting before retry",
			"chain_id", p.chainID,
			"attempt_count", attemptCount,
			"backoff_duration", backoffDuration,
			"last_error", lastErr)

		select {
		case <-time.After(backoffDuration):
		case <-ctx.Done():
			return usedRPCURL, maxSafeRange, ctx.Err()
		}
	}
}

// calculateBackoffDuration 计算退避时间
func (p *Pool) calculateBackoffDuration(attemptCount int) time.Duration {
	// 指数退避，但有最大限制
	baseDuration := time.Second * 30
	maxDuration := time.Minute * 5

	backoff := time.Duration(attemptCount) * baseDuration
	if backoff > maxDuration {
		backoff = maxDuration
	}

	return backoff
}

// executeWithSingleRPCRetry 对单个RPC执行重试
func (p *Pool) executeWithSingleRPCRetry(ctx context.Context, client *ethclient.Client, rpcURL string, fn func(*ethclient.Client) error) error {
	var lastErr error

	// 使用 defer 确保无论成功或失败，RPC都会被放回队列
	defer func() {
		// 无论成功失败，都将RPC放回队列尾部
		if err := p.rpcCache.PushRPCToFIFOQueue(ctx, p.chainID, rpcURL); err != nil {
			logger.Error("Failed to push RPC back to FIFO queue", err, "url", rpcURL, "chain_id", p.chainID)
		}
	}()

	for attempt := 1; attempt <= p.config.MaxRetryCount; attempt++ {
		err := fn(client)
		if err == nil {
			// 执行成功，更新元数据
			p.updateRPCMetadataOnSuccess(ctx, rpcURL)
			return nil
		}

		lastErr = err

		// 如果不是最后一次重试，等待一段时间
		if attempt < p.config.MaxRetryCount {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second): // 线性退避
			}
		}
	}

	// 执行失败，更新元数据
	p.updateRPCMetadataOnFailure(ctx, rpcURL, lastErr.Error())
	return fmt.Errorf("RPC call failed after %d retries: %w", p.config.MaxRetryCount, lastErr)
}

// initializeRPCs 初始化所有RPC连接
func (p *Pool) initializeRPCs(ctx context.Context) error {

	for _, rpcURL := range p.rpcURLs {
		client, err := p.createClient(rpcURL)
		if err != nil {
			logger.Warn("Failed to create RPC client", "chain_id", p.chainID, "url", rpcURL, "error", err)
			continue
		}

		p.clients[rpcURL] = client
	}

	if len(p.clients) == 0 {
		return fmt.Errorf("no RPC clients could be created for chain %d", p.chainID)
	}

	return nil
}

// createClient 创建RPC客户端
func (p *Pool) createClient(rpcURL string) (*ethclient.Client, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to dial RPC %s: %w", rpcURL, err)
	}

	return client, nil
}

// getOrCreateClient 获取或创建RPC客户端
func (p *Pool) getOrCreateClient(rpcURL string) (*ethclient.Client, error) {
	p.mutex.RLock()
	client, exists := p.clients[rpcURL]
	p.mutex.RUnlock()

	if exists && client != nil {
		return client, nil
	}

	// 需要创建新客户端
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 双重检查
	if client, exists := p.clients[rpcURL]; exists && client != nil {
		return client, nil
	}

	// 创建新客户端
	newClient, err := p.createClient(rpcURL)
	if err != nil {
		return nil, err
	}

	p.clients[rpcURL] = newClient
	return newClient, nil
}

// performInitialCheck 执行初始统一检查（健康+能力）
func (p *Pool) performInitialCheck(ctx context.Context) error {
	_, err := p.healthChecker.CheckAllRPCs(ctx, p.chainID, p.rpcURLs)
	if err != nil {
		return fmt.Errorf("initial check failed: %w", err)
	}

	return nil
}

// updateRPCMetadataOnSuccess 更新RPC元数据（成功时）
func (p *Pool) updateRPCMetadataOnSuccess(ctx context.Context, rpcURL string) {
	metadata, err := p.rpcCache.GetRPCMetadata(ctx, p.chainID, rpcURL)
	if err != nil {
		logger.Error("Failed to get RPC metadata", err, "url", rpcURL)
		return
	}

	if metadata != nil {
		wasUnhealthy := !metadata.IsHealthy

		metadata.IsHealthy = true
		metadata.ErrorCount = 0
		metadata.LastError = nil
		metadata.LastCheckedAt = time.Now()

		// 保存元数据
		p.rpcCache.SetRPCMetadata(ctx, metadata)

		if wasUnhealthy {
			logger.Info("RPC recovered", "url", rpcURL, "chain_id", p.chainID)
		}
	}
}

// updateRPCMetadataOnFailure 更新RPC元数据（失败时）
func (p *Pool) updateRPCMetadataOnFailure(ctx context.Context, rpcURL string, errorMsg string) {
	metadata, err := p.rpcCache.GetRPCMetadata(ctx, p.chainID, rpcURL)
	if err != nil {
		logger.Error("Failed to get RPC metadata", err, "url", rpcURL)
		return
	}

	if metadata != nil {
		metadata.IsHealthy = false
		metadata.ErrorCount++
		metadata.LastError = &errorMsg
		metadata.LastCheckedAt = time.Now()

		// 保存元数据
		p.rpcCache.SetRPCMetadata(ctx, metadata)
	}
}

// startMaintenanceTasks 启动维护任务
func (p *Pool) startMaintenanceTasks() {
	// 启动统一检查任务（健康+能力）
	p.healthCheckTicker = time.NewTicker(p.config.HealthCheckInterval)
	p.wg.Add(1)
	go p.checkTask()

	logger.Info("Maintenance tasks started", "chain_id", p.chainID)
}

// stopMaintenanceTasks 停止维护任务
func (p *Pool) stopMaintenanceTasks() {
	// 停止定时器
	if p.healthCheckTicker != nil {
		p.healthCheckTicker.Stop()
	}

	// 发送停止信号
	close(p.stopCh)

	// 等待所有任务结束
	p.wg.Wait()

	logger.Info("Maintenance tasks stopped", "chain_id", p.chainID)
}

// checkTask 定期执行统一检查（健康+能力）
func (p *Pool) checkTask() {
	defer p.wg.Done()

	for {
		select {
		case <-p.healthCheckTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute) // 增加超时以适应能力测试
			if err := p.performCheck(ctx); err != nil {
				logger.Error("Periodic check failed", err, "chain_id", p.chainID)
			}
			cancel()

		case <-p.stopCh:
			return
		}
	}
}

// performCheck 执行统一检查（健康+能力）
func (p *Pool) performCheck(ctx context.Context) error {
	logger.Debug("Performing periodic RPC check (health + capability)", "chain_id", p.chainID)
	_, err := p.healthChecker.CheckAllRPCs(ctx, p.chainID, p.rpcURLs)
	return err
}

// GetStatus 获取连接池状态
func (p *Pool) GetStatus(ctx context.Context) (*types.RPCPoolStatus, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	status := &types.RPCPoolStatus{
		ChainID:     p.chainID,
		TotalRPCs:   len(p.rpcURLs),
		RPCs:        make([]types.RPCNodeStatus, 0, len(p.rpcURLs)),
		LastUpdated: time.Now(),
	}

	healthyCount := 0

	for _, rpcURL := range p.rpcURLs {
		metadata, err := p.rpcCache.GetRPCMetadata(ctx, p.chainID, rpcURL)
		if err != nil {
			continue
		}

		if metadata == nil {
			continue
		}

		if metadata.IsHealthy {
			healthyCount++
		}

		nodeStatus := types.RPCNodeStatus{
			URL:            metadata.URL,
			IsHealthy:      metadata.IsHealthy,
			MaxSafeRange:   metadata.MaxSafeRange,
			ResponseTimeMs: metadata.ResponseTimeMs,
			ErrorCount:     metadata.ErrorCount,
			LastError:      metadata.LastError,
			LastCheckedAt:  metadata.LastCheckedAt,
		}

		status.RPCs = append(status.RPCs, nodeStatus)
	}

	status.HealthyRPCs = healthyCount
	return status, nil
}
