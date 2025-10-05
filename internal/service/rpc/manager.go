package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/chain"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/redis"

	"github.com/ethereum/go-ethereum/ethclient"
)

// Manager RPC管理器
type Manager struct {
	config           *config.Config
	chainRepo        chain.Repository
	redisClient      *redis.Client
	rpcCache         *redis.RPCCache
	chainlistFetcher *ChainlistFetcher
	pools            map[int]*Pool
	mutex            sync.RWMutex
}

// NewManager 创建RPC管理器
func NewManager(cfg *config.Config, chainRepo chain.Repository, redisClient *redis.Client) *Manager {
	rpcCache := redis.NewRPCCache(redisClient)
	chainlistFetcher := NewChainlistFetcher()

	return &Manager{
		config:           cfg,
		chainRepo:        chainRepo,
		redisClient:      redisClient,
		rpcCache:         rpcCache,
		chainlistFetcher: chainlistFetcher,
		pools:            make(map[int]*Pool),
	}
}

// Start 启动RPC管理器
func (m *Manager) Start(ctx context.Context) error {
	logger.Info("Starting RPC Manager")

	// 获取启用RPC的链
	chains, err := m.chainRepo.GetRPCEnabledChains(ctx, m.config.RPC.IncludeTestnets)
	if err != nil {
		return fmt.Errorf("failed to get RPC enabled chains: %w", err)
	}

	if len(chains) == 0 {
		logger.Warn("No RPC enabled chains found")
		return nil
	}

	// ********先不从chainlist获取数据********
	// // 检查并获取链数据
	// if err := m.ensureChainData(ctx, chains); err != nil {
	// 	return fmt.Errorf("failed to ensure chain data: %w", err)
	// }

	// // 重新获取更新后的链数据
	// chains, err = m.chainRepo.GetRPCEnabledChains(ctx, m.config.RPC.IncludeTestnets)
	// if err != nil {
	// 	return fmt.Errorf("failed to get updated RPC enabled chains: %w", err)
	// }

	// 并行为每条链创建和启动RPC池
	var wg sync.WaitGroup
	var successCount int32
	var mu sync.Mutex

	// 控制并发数，避免过多并发连接
	maxConcurrent := 5
	if len(chains) < maxConcurrent {
		maxConcurrent = len(chains)
	}
	semaphore := make(chan struct{}, maxConcurrent)

	for _, chainInfo := range chains {
		wg.Add(1)
		go func(chain types.ChainRPCInfo) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := m.initChainRPCPool(ctx, &chain); err != nil {
				logger.Error("Failed to init RPC pool for chain", err, "chain_id", chain.ChainID, "chain_name", chain.ChainName)
				return
			}

			// 原子递增成功计数
			mu.Lock()
			successCount++
			mu.Unlock()
		}(chainInfo)
	}

	// 等待所有初始化完成
	wg.Wait()

	successCountInt := int(successCount)

	if successCountInt == 0 {
		return fmt.Errorf("failed to initialize any RPC pools")
	}

	logger.Info("RPC Manager started successfully", "total_chains", len(chains), "success_chains", successCountInt)
	return nil
}

// Stop 停止RPC管理器
func (m *Manager) Stop() {
	logger.Info("Stopping RPC Manager")

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 停止所有RPC池
	for chainID, pool := range m.pools {
		if pool != nil {
			pool.Stop()
			logger.Debug("Stopped RPC pool", "chain_id", chainID)
		}
	}

	// 清理池映射
	m.pools = make(map[int]*Pool)

	logger.Info("RPC Manager stopped successfully")
}

// GetClient 获取指定链的RPC客户端
func (m *Manager) GetClient(ctx context.Context, chainID int) (*ethclient.Client, error) {
	m.mutex.RLock()
	pool, exists := m.pools[chainID]
	m.mutex.RUnlock()

	if !exists || pool == nil {
		return nil, fmt.Errorf("no RPC pool available for chain %d", chainID)
	}

	client, _, err := pool.GetHealthyClient(ctx)
	return client, err
}

// ExecuteWithRetry 带重试机制执行RPC调用
func (m *Manager) ExecuteWithRetry(ctx context.Context, chainID int, fn func(*ethclient.Client) error) error {
	m.mutex.RLock()
	pool, exists := m.pools[chainID]
	m.mutex.RUnlock()

	if !exists || pool == nil {
		return fmt.Errorf("no RPC pool available for chain %d", chainID)
	}

	return pool.ExecuteWithRetry(ctx, fn)
}

// ExecuteWithRPCInfoDo 单轮重试，返回RPC与maxSafeRange，并把maxSafeRange传入回调
func (m *Manager) ExecuteWithRPCInfoDo(ctx context.Context, chainID int, fn func(*ethclient.Client, int) error) (string, int, error) {
	m.mutex.RLock()
	pool, exists := m.pools[chainID]
	m.mutex.RUnlock()

	if !exists || pool == nil {
		return "", 0, fmt.Errorf("no RPC pool available for chain %d", chainID)
	}

	return pool.ExecuteWithRPCInfoDo(ctx, fn)
}

// ExecuteWithRPCInfoDoInfiniteRetry 无限轮重试，返回RPC与maxSafeRange，并把maxSafeRange传入回调
func (m *Manager) ExecuteWithRPCInfoDoInfiniteRetry(ctx context.Context, chainID int, fn func(*ethclient.Client, int) error) (string, int, error) {
	m.mutex.RLock()
	pool, exists := m.pools[chainID]
	m.mutex.RUnlock()

	if !exists || pool == nil {
		return "", 0, fmt.Errorf("no RPC pool available for chain %d", chainID)
	}

	return pool.ExecuteWithRPCInfoDoInfiniteRetry(ctx, fn)
}

// ensureChainData 确保链数据完整性
func (m *Manager) ensureChainData(ctx context.Context, chains []types.ChainRPCInfo) error {
	// 全部重新获取
	chainsNeedingData := []int{}
	for _, chain := range chains {
		chainsNeedingData = append(chainsNeedingData, chain.ChainID)
	}

	// 从chainlist.org获取数据
	chainDataMap, err := m.chainlistFetcher.FetchMultipleChainData(ctx, chainsNeedingData)
	if err != nil {
		return fmt.Errorf("failed to fetch chain data: %w", err)
	}

	// 更新数据库
	for chainID, chainData := range chainDataMap {
		if err := m.updateChainRPCData(ctx, chainID, chainData); err != nil {
			logger.Error("Failed to update chain RPC data", err, "chain_id", chainID)
			continue
		}
	}

	return nil
}

// updateChainRPCData 更新链的RPC数据
func (m *Manager) updateChainRPCData(ctx context.Context, chainID int, chainData *types.ChainInfo) error {
	// 验证数据完整性
	if err := m.chainlistFetcher.ValidateChainData(chainData); err != nil {
		return fmt.Errorf("invalid chain data: %w", err)
	}

	// 过滤HTTPS RPC URLs
	httpsRPCs := m.chainlistFetcher.FilterHTTPSRPCs(chainData.RPC)
	if len(httpsRPCs) == 0 {
		return fmt.Errorf("no valid HTTPS RPC URLs found for chain %d", chainID)
	}

	// 序列化RPC URLs为JSON
	rpcURLsJSON, err := json.Marshal(httpsRPCs)
	if err != nil {
		return fmt.Errorf("failed to marshal RPC URLs: %w", err)
	}

	// 获取第一个区块浏览器URL
	explorerURL := m.chainlistFetcher.GetFirstExplorerURL(chainData.Explorers)
	if explorerURL == "" {
		logger.Warn("No explorer URL found for chain", "chain_id", chainID)
		explorerURL = "https://etherscan.io" // 默认值
	}

	// 序列化区块浏览器URL为JSON数组
	explorerURLsJSON, err := json.Marshal([]string{explorerURL})
	if err != nil {
		return fmt.Errorf("failed to marshal explorer URLs: %w", err)
	}

	// 更新数据库
	updateData := map[string]interface{}{
		"native_currency_name":     chainData.NativeCurrency.Name,
		"native_currency_symbol":   chainData.NativeCurrency.Symbol,
		"native_currency_decimals": chainData.NativeCurrency.Decimals,
		"official_rpc_urls":        string(rpcURLsJSON),
		"block_explorer_urls":      string(explorerURLsJSON),
	}

	return m.chainRepo.UpdateChainRPCData(ctx, chainID, updateData)
}

// initChainRPCPool 初始化单链的RPC连接池
func (m *Manager) initChainRPCPool(ctx context.Context, chainInfo *types.ChainRPCInfo) error {
	// 解析RPC URLs
	var rpcURLs []string
	if err := json.Unmarshal([]byte(chainInfo.OfficialRPCUrls), &rpcURLs); err != nil {
		return fmt.Errorf("failed to parse RPC URLs for chain %s: %w", chainInfo.ChainName, err)
	}

	if len(rpcURLs) == 0 {
		return fmt.Errorf("no RPC URLs available for chain %s", chainInfo.ChainName)
	}

	// 创建RPC池
	pool := NewPool(chainInfo.ChainID, rpcURLs, &m.config.RPCPool, m.rpcCache)

	// 启动RPC池
	if err := pool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start RPC pool for chain %s: %w", chainInfo.ChainName, err)
	}

	// 保存RPC池
	m.mutex.Lock()
	m.pools[chainInfo.ChainID] = pool
	m.mutex.Unlock()

	return nil
}

// GetStatus 获取RPC管理器状态
func (m *Manager) GetStatus(ctx context.Context) (map[string]interface{}, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	poolStatuses := make(map[string]interface{})
	chains := make([]int, 0, len(m.pools))
	totalRPCs := 0
	totalHealthyRPCs := 0

	for chainID, pool := range m.pools {
		chains = append(chains, chainID)
		if pool != nil {
			status, err := pool.GetStatus(ctx)
			if err != nil {
				logger.Error("Failed to get pool status", err, "chain_id", chainID)
				continue
			}

			poolStatuses[fmt.Sprintf("chain_%d", chainID)] = status
			totalRPCs += status.TotalRPCs
			totalHealthyRPCs += status.HealthyRPCs
		}
	}

	return map[string]interface{}{
		"total_pools":        len(m.pools),
		"total_rpcs":         totalRPCs,
		"total_healthy_rpcs": totalHealthyRPCs,
		"chains":             chains,
		"pool_status":        poolStatuses,
	}, nil
}

// GetChainRPCStatus 获取指定链的RPC状态
func (m *Manager) GetChainRPCStatus(ctx context.Context, chainID int) (*types.RPCPoolStatus, error) {
	m.mutex.RLock()
	pool, exists := m.pools[chainID]
	m.mutex.RUnlock()

	if !exists || pool == nil {
		return nil, fmt.Errorf("no RPC pool found for chain %d", chainID)
	}

	return pool.GetStatus(ctx)
}

// RefreshChainRPCData 刷新指定链的RPC数据
func (m *Manager) RefreshChainRPCData(ctx context.Context, chainID int) error {
	// 从chainlist.org获取最新数据
	chainData, err := m.chainlistFetcher.FetchChainData(ctx, chainID)
	if err != nil {
		return fmt.Errorf("failed to fetch chain data: %w", err)
	}

	// 更新数据库
	if err := m.updateChainRPCData(ctx, chainID, chainData); err != nil {
		return fmt.Errorf("failed to update chain data: %w", err)
	}

	// 重启该链的RPC池
	m.mutex.Lock()
	if pool, exists := m.pools[chainID]; exists && pool != nil {
		pool.Stop()
		delete(m.pools, chainID)
	}
	m.mutex.Unlock()

	// 获取更新后的链信息
	chains, err := m.chainRepo.GetRPCEnabledChains(ctx, m.config.RPC.IncludeTestnets)
	if err != nil {
		return fmt.Errorf("failed to get updated chain info: %w", err)
	}

	// 找到对应的链并重新初始化
	for _, chain := range chains {
		if chain.ChainID == chainID {
			if err := m.initChainRPCPool(ctx, &chain); err != nil {
				return fmt.Errorf("failed to reinitialize RPC pool: %w", err)
			}
			break
		}
	}

	return nil
}
