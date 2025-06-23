package asset

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"
	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/asset"
	"timelocker-backend/internal/repository/chain"
	"timelocker-backend/internal/repository/chaintoken"
	"timelocker-backend/internal/repository/user"
	priceService "timelocker-backend/internal/service/price"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/blockchain"
	"timelocker-backend/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// Service 资产服务接口
type Service interface {
	Start(ctx context.Context) error
	Stop() error
	GetUserAssets(walletAddress string, chainID int64, forceRefresh bool) (*types.UserAssetResponse, error)
	RefreshUserAssets(walletAddress string, chainID int64) error
	RefreshAllUserAssets() error
}

// service 资产服务实现
type service struct {
	config         *config.AssetConfig
	rpcConfig      *config.RPCConfig
	userRepo       user.Repository
	chainRepo      chain.Repository
	chainTokenRepo chaintoken.Repository
	assetRepo      asset.Repository
	priceService   priceService.Service
	rpcClient      blockchain.RPCClient
	redisClient    *redis.Client
	ticker         *time.Ticker
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// NewService 创建新的资产服务
func NewService(
	cfg *config.AssetConfig,
	rpcCfg *config.RPCConfig,
	userRepo user.Repository,
	chainRepo chain.Repository,
	chainTokenRepo chaintoken.Repository,
	assetRepo asset.Repository,
	priceService priceService.Service,
	redisClient *redis.Client,
) (Service, error) {

	// 创建RPC客户端
	rpcClient, err := blockchain.NewRPCClient(rpcCfg)
	if err != nil {
		logger.Error("Failed to create RPC client", err)
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	return &service{
		config:         cfg,
		rpcConfig:      rpcCfg,
		userRepo:       userRepo,
		chainRepo:      chainRepo,
		chainTokenRepo: chainTokenRepo,
		assetRepo:      assetRepo,
		priceService:   priceService,
		rpcClient:      rpcClient,
		redisClient:    redisClient,
		stopCh:         make(chan struct{}),
	}, nil
}

// Start 启动资产服务
func (s *service) Start(ctx context.Context) error {
	logger.Info("Asset service starting", "update_interval", s.config.UpdateInterval)

	// 启动定时更新
	s.ticker = time.NewTicker(s.config.UpdateInterval)
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.ticker.C:
				if err := s.RefreshAllUserAssets(); err != nil {
					logger.Error("Failed to refresh all user assets", err)
				}
			case <-s.stopCh:
				logger.Info("Asset service stopping")
				return
			}
		}
	}()

	logger.Info("Asset service started successfully")
	return nil
}

// Stop 停止资产服务
func (s *service) Stop() error {
	if s.ticker != nil {
		s.ticker.Stop()
	}

	close(s.stopCh)
	s.wg.Wait()

	if s.rpcClient != nil {
		s.rpcClient.Close()
	}

	logger.Info("Asset service stopped")
	return nil
}

// GetUserAssets 获取用户资产
func (s *service) GetUserAssets(walletAddress string, chainID int64, forceRefresh bool) (*types.UserAssetResponse, error) {
	logger.Info("Getting user assets", "wallet_address", walletAddress, "chain_id", chainID, "force_refresh", forceRefresh)

	// 如果强制刷新，先更新数据
	if forceRefresh {
		if err := s.RefreshUserAssets(walletAddress, chainID); err != nil {
			logger.Error("Failed to refresh user assets", err, "wallet_address", walletAddress, "chain_id", chainID)
		}
	}

	// 从数据库获取用户资产
	assets, err := s.assetRepo.GetUserAssets(walletAddress)
	if err != nil {
		logger.Error("Failed to get user assets from database", err, "wallet_address", walletAddress)
		return nil, fmt.Errorf("failed to get user assets: %w", err)
	}

	// 获取所有链信息
	chains, err := s.chainRepo.GetAllActiveChains()
	if err != nil {
		logger.Error("Failed to get active chains", err)
		return nil, fmt.Errorf("failed to get active chains: %w", err)
	}

	// 组织响应数据
	response := &types.UserAssetResponse{
		WalletAddress:  walletAddress,
		PrimaryChainID: chainID,
		OtherChains:    make([]types.ChainAssetInfo, 0),
		LastUpdated:    time.Now(),
	}

	// 按链分组资产
	chainAssets := make(map[int64][]types.AssetInfo)
	totalUSDValue := 0.0

	for _, asset := range assets {
		if asset.Token == nil {
			continue
		}

		// 获取代币价格
		price, _ := s.priceService.GetPrice(asset.Token.Symbol)
		tokenPrice := 0.0
		change24h := 0.0
		if price != nil {
			tokenPrice = price.Price
			change24h = price.Change24h
		}

		// 计算USD价值
		balance, _ := strconv.ParseFloat(asset.Balance, 64)
		usdValue := balance * tokenPrice

		assetInfo := types.AssetInfo{
			TokenSymbol: asset.Token.Symbol,
			TokenName:   asset.Token.Name,
			Balance:     asset.Balance,
			BalanceWei:  asset.BalanceWei,
			USDValue:    usdValue,
			TokenPrice:  tokenPrice,
			Change24h:   change24h,
			LastUpdated: asset.LastUpdated,
		}

		chainAssets[asset.ChainID] = append(chainAssets[asset.ChainID], assetInfo)
		totalUSDValue += usdValue
	}

	// 构建链资产信息
	for _, chainInfo := range chains {
		assets, exists := chainAssets[chainInfo.ChainID]
		if !exists {
			assets = []types.AssetInfo{}
		}

		// 计算该链的总价值
		chainTotalValue := 0.0
		for _, asset := range assets {
			chainTotalValue += asset.USDValue
		}

		chainAssetInfo := types.ChainAssetInfo{
			ChainID:       chainInfo.ChainID,
			ChainName:     chainInfo.Name,
			ChainSymbol:   chainInfo.Symbol,
			Assets:        assets,
			TotalUSDValue: chainTotalValue,
			LastUpdated:   time.Now(),
		}

		if chainInfo.ChainID == chainID {
			response.PrimaryChain = chainAssetInfo
		} else {
			response.OtherChains = append(response.OtherChains, chainAssetInfo)
		}
	}

	response.TotalUSDValue = totalUSDValue

	logger.Info("Got user assets", "wallet_address", walletAddress, "total_usd_value", totalUSDValue, "chains_count", len(response.OtherChains)+1)
	return response, nil
}

// RefreshUserAssets 刷新用户资产
func (s *service) RefreshUserAssets(walletAddress string, chainID int64) error {
	logger.Info("Refreshing user assets", "wallet_address", walletAddress, "chain_id", chainID)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 获取用户信息
	user, err := s.userRepo.GetByWalletAddress(walletAddress)
	if err != nil {
		logger.Error("Failed to get user", err, "wallet_address", walletAddress)
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s", walletAddress)
	}

	// 获取该链的代币配置
	chainTokens, err := s.chainTokenRepo.GetTokensByChainID(chainID)
	if err != nil {
		logger.Error("Failed to get chain tokens", err, "chain_id", chainID)
		return fmt.Errorf("failed to get chain tokens: %w", err)
	}

	if len(chainTokens) == 0 {
		logger.Info("No tokens configured for chain", "chain_id", chainID)
		return nil
	}

	// 查询各代币余额
	var assets []*types.UserAsset
	for _, chainToken := range chainTokens {
		if chainToken.Token == nil {
			continue
		}

		var balance *big.Int
		var err error

		if chainToken.IsNative {
			// 查询原生代币余额
			balance, err = s.rpcClient.GetNativeBalance(ctx, chainID, walletAddress)
		} else {
			// 查询ERC-20代币余额
			balance, err = s.rpcClient.GetTokenBalance(ctx, chainID, walletAddress, chainToken.ContractAddress)
		}

		if err != nil {
			logger.Error("Failed to get token balance", err, "chain_id", chainID, "token", chainToken.Token.Symbol, "is_native", chainToken.IsNative)
			// 不返回错误，继续查询其他代币
			continue
		}

		// 创建或更新用户资产记录
		asset := &types.UserAsset{
			UserID:        user.ID,
			WalletAddress: walletAddress,
			ChainID:       chainID,
			TokenID:       chainToken.TokenID,
		}

		// 设置余额
		asset.SetBalanceFromBigInt(balance, chainToken.Token.Decimals)

		// 计算USD价值
		price, _ := s.priceService.GetPrice(chainToken.Token.Symbol)
		if price != nil {
			balanceFloat, _ := strconv.ParseFloat(asset.Balance, 64)
			asset.USDValue = balanceFloat * price.Price
		}

		assets = append(assets, asset)
	}

	// 批量保存到数据库
	if len(assets) > 0 {
		if err := s.assetRepo.BatchCreateOrUpdateUserAssets(assets); err != nil {
			logger.Error("Failed to save user assets", err, "wallet_address", walletAddress, "chain_id", chainID)
			return fmt.Errorf("failed to save user assets: %w", err)
		}
	}

	// 保存到缓存
	if err := s.cacheUserAssets(walletAddress, chainID, assets); err != nil {
		logger.Error("Failed to cache user assets", err, "wallet_address", walletAddress, "chain_id", chainID)
		// 不返回错误，缓存失败不影响主要功能
	}

	logger.Info("Refreshed user assets", "wallet_address", walletAddress, "chain_id", chainID, "assets_count", len(assets))
	return nil
}

// RefreshAllUserAssets 刷新所有用户资产
func (s *service) RefreshAllUserAssets() error {
	logger.Info("Starting to refresh all user assets")

	// 获取所有用户
	users, err := s.userRepo.GetAllActiveUsers()
	if err != nil {
		logger.Error("Failed to get all active users", err)
		return fmt.Errorf("failed to get all active users: %w", err)
	}

	// 获取所有活跃链
	chains, err := s.chainRepo.GetAllActiveChains()
	if err != nil {
		logger.Error("Failed to get all active chains", err)
		return fmt.Errorf("failed to get all active chains: %w", err)
	}

	logger.Info("Refreshing assets", "users_count", len(users), "chains_count", len(chains))

	// 并发刷新用户资产，但限制并发数
	semaphore := make(chan struct{}, 5) // 限制并发数为5
	var wg sync.WaitGroup

	for _, user := range users {
		for _, chain := range chains {
			wg.Add(1)
			go func(walletAddress string, chainID int64) {
				defer wg.Done()
				semaphore <- struct{}{}        // 获取信号
				defer func() { <-semaphore }() // 释放信号

				if err := s.RefreshUserAssets(walletAddress, chainID); err != nil {
					logger.Error("Failed to refresh user assets", err, "wallet_address", walletAddress, "chain_id", chainID)
				}
			}(user.WalletAddress, chain.ChainID)
		}
	}

	wg.Wait()
	logger.Info("Finished refreshing all user assets")
	return nil
}

// cacheUserAssets 缓存用户资产
func (s *service) cacheUserAssets(walletAddress string, chainID int64, assets []*types.UserAsset) error {
	key := fmt.Sprintf("%s%s:%d", s.config.CachePrefix, walletAddress, chainID)

	data, err := json.Marshal(assets)
	if err != nil {
		return fmt.Errorf("failed to marshal assets: %w", err)
	}

	ctx := context.Background()
	expiration := s.config.UpdateInterval * 2 // 缓存时间为更新间隔的2倍

	return s.redisClient.Set(ctx, key, data, expiration).Err()
}
