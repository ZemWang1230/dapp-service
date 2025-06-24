package asset

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
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
	GetUserAssets(walletAddress string, forceRefresh bool) (*types.UserAssetResponse, error)
	RefreshUserAssets(walletAddress string) error
	RefreshUserAssetsOnChainConnect(walletAddress string) error
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
	}, nil
}

// GetUserAssets 获取用户资产（所有支持链上的资产，按价值排序）
func (s *service) GetUserAssets(walletAddress string, forceRefresh bool) (*types.UserAssetResponse, error) {
	logger.Info("Getting user assets", "wallet_address", walletAddress, "force_refresh", forceRefresh)

	// 如果强制刷新，先更新数据
	if forceRefresh {
		if err := s.RefreshUserAssets(walletAddress); err != nil {
			logger.Error("Failed to refresh user assets", err, "wallet_address", walletAddress)
		}
	}

	// 从数据库获取用户所有资产
	assets, err := s.assetRepo.GetUserAssets(walletAddress)
	if err != nil {
		logger.Error("Failed to get user assets from database", err, "wallet_address", walletAddress)
		return nil, fmt.Errorf("failed to get user assets: %w", err)
	}

	// 组织响应数据
	response := &types.UserAssetResponse{
		WalletAddress: walletAddress,
		Assets:        make([]types.AssetInfo, 0),
		LastUpdated:   time.Now(),
	}

	// 构建资产信息（已按价值排序）
	totalUSDValue := 0.0

	for _, asset := range assets {
		if asset.Token == nil || asset.Chain == nil {
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

		// 获取链代币信息以确定合约地址和是否为原生代币
		chainToken, err := s.chainTokenRepo.GetChainTokenByChainAndToken(asset.ChainID, asset.TokenID)
		var contractAddr string
		var isNative bool
		if err == nil && chainToken != nil {
			contractAddr = chainToken.ContractAddress
			isNative = chainToken.IsNative
		}

		assetInfo := types.AssetInfo{
			ChainID:      asset.ChainID,
			ChainName:    asset.Chain.Name,
			ChainSymbol:  asset.Chain.Symbol,
			TokenSymbol:  asset.Token.Symbol,
			TokenName:    asset.Token.Name,
			ContractAddr: contractAddr,
			Balance:      asset.Balance,
			BalanceWei:   asset.BalanceWei,
			USDValue:     usdValue,
			TokenPrice:   tokenPrice,
			Change24h:    change24h,
			IsNative:     isNative,
			LastUpdated:  asset.LastUpdated,
		}

		response.Assets = append(response.Assets, assetInfo)
		totalUSDValue += usdValue
	}

	response.TotalUSDValue = totalUSDValue

	logger.Info("Got user assets", "wallet_address", walletAddress, "total_usd_value", totalUSDValue, "assets_count", len(response.Assets))
	return response, nil
}

// RefreshUserAssets 刷新用户资产（手动刷新所有支持链）
func (s *service) RefreshUserAssets(walletAddress string) error {
	logger.Info("Refreshing user assets", "wallet_address", walletAddress)
	return s.refreshUserAssetsInternal(walletAddress)
}

// RefreshUserAssetsOnChainConnect 用户连接钱包时刷新所有支持链上的资产
func (s *service) RefreshUserAssetsOnChainConnect(walletAddress string) error {
	logger.Info("Refreshing user assets on chain connect", "wallet_address", walletAddress)
	return s.refreshUserAssetsInternal(walletAddress)
}

// refreshUserAssetsInternal 内部资产刷新逻辑（刷新所有支持链上的资产）
func (s *service) refreshUserAssetsInternal(walletAddress string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 5分钟超时，因为要查询多条链
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

	// 获取所有支持的链
	chains, err := s.chainRepo.GetAllActiveChains()
	if err != nil {
		logger.Error("Failed to get active chains", err)
		return fmt.Errorf("failed to get active chains: %w", err)
	}

	var allAssets []*types.UserAsset

	// 遍历每条链
	for _, chain := range chains {
		chainID := chain.ChainID
		logger.Info("Refreshing assets for chain", "chain_id", chainID, "wallet_address", walletAddress)

		// 获取该链的代币配置
		chainTokens, err := s.chainTokenRepo.GetTokensByChainID(chainID)
		if err != nil {
			logger.Error("Failed to get chain tokens", err, "chain_id", chainID)
			continue // 继续查询其他链
		}

		if len(chainTokens) == 0 {
			logger.Info("No tokens configured for chain", "chain_id", chainID)
			continue
		}

		// 查询各代币余额
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
				continue // 继续查询其他代币
			}

			// 过滤余额为0的资产，不保存到数据库
			if balance.Cmp(big.NewInt(0)) == 0 {
				logger.Info("Skipping zero balance token", "chain_id", chainID, "token", chainToken.Token.Symbol, "wallet_address", walletAddress)
				continue
			}

			// 创建或更新用户资产记录
			asset := &types.UserAsset{
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

			allAssets = append(allAssets, asset)
		}
	}

	// 使用 UPSERT 逻辑批量保存到数据库（只保存余额>0的记录）
	if len(allAssets) > 0 {
		if err := s.assetRepo.BatchUpsertUserAssets(allAssets); err != nil {
			logger.Error("Failed to upsert user assets", err, "wallet_address", walletAddress)
			return fmt.Errorf("failed to upsert user assets: %w", err)
		}
	}

	// 保存到缓存
	if err := s.cacheUserAssets(walletAddress, allAssets); err != nil {
		logger.Error("Failed to cache user assets", err, "wallet_address", walletAddress)
		// 不返回错误，缓存失败不影响主要功能
	}

	logger.Info("Refreshed user assets", "wallet_address", walletAddress, "total_assets_count", len(allAssets))
	return nil
}

// cacheUserAssets 缓存用户资产
func (s *service) cacheUserAssets(walletAddress string, assets []*types.UserAsset) error {
	key := fmt.Sprintf("%s%s", s.config.CachePrefix, walletAddress)

	data, err := json.Marshal(assets)
	if err != nil {
		return fmt.Errorf("failed to marshal assets: %w", err)
	}

	ctx := context.Background()
	expiration := s.config.UpdateInterval * 2 // 缓存时间为更新间隔的2倍

	return s.redisClient.Set(ctx, key, data, expiration).Err()
}
