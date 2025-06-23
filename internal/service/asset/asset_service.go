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
	GetUserAssets(walletAddress string, chainID int64, forceRefresh bool) (*types.UserAssetResponse, error)
	RefreshUserAssets(walletAddress string, chainID int64) error
	RefreshUserAssetsOnChainConnect(walletAddress string, chainID int64) error
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

// GetUserAssets 获取用户资产（只返回指定链上的资产）
func (s *service) GetUserAssets(walletAddress string, chainID int64, forceRefresh bool) (*types.UserAssetResponse, error) {
	logger.Info("Getting user assets", "wallet_address", walletAddress, "chain_id", chainID, "force_refresh", forceRefresh)

	// 如果强制刷新，先更新数据
	if forceRefresh {
		if err := s.RefreshUserAssets(walletAddress, chainID); err != nil {
			logger.Error("Failed to refresh user assets", err, "wallet_address", walletAddress, "chain_id", chainID)
		}
	}

	// 从数据库获取用户在指定链上的资产
	assets, err := s.assetRepo.GetUserAssetsByChain(walletAddress, chainID)
	if err != nil {
		logger.Error("Failed to get user assets from database", err, "wallet_address", walletAddress, "chain_id", chainID)
		return nil, fmt.Errorf("failed to get user assets: %w", err)
	}

	// 获取链信息
	chainInfo, err := s.chainRepo.GetChainByChainID(chainID)
	if err != nil {
		logger.Error("Failed to get chain info", err, "chain_id", chainID)
		return nil, fmt.Errorf("failed to get chain info: %w", err)
	}
	if chainInfo == nil {
		logger.Error("Chain not found", nil, "chain_id", chainID)
		return nil, fmt.Errorf("chain not found: %d", chainID)
	}

	// 组织响应数据
	response := &types.UserAssetResponse{
		WalletAddress:  walletAddress,
		PrimaryChainID: chainID,
		OtherChains:    make([]types.ChainAssetInfo, 0), // 不返回其他链的数据
		LastUpdated:    time.Now(),
	}

	// 构建当前链的资产信息
	var assetInfos []types.AssetInfo
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

		assetInfos = append(assetInfos, assetInfo)
		totalUSDValue += usdValue
	}

	// 构建主链资产信息
	response.PrimaryChain = types.ChainAssetInfo{
		ChainID:       chainInfo.ChainID,
		ChainName:     chainInfo.Name,
		ChainSymbol:   chainInfo.Symbol,
		Assets:        assetInfos,
		TotalUSDValue: totalUSDValue,
		LastUpdated:   time.Now(),
	}

	response.TotalUSDValue = totalUSDValue

	logger.Info("Got user assets", "wallet_address", walletAddress, "chain_id", chainID, "total_usd_value", totalUSDValue, "assets_count", len(assetInfos))
	return response, nil
}

// RefreshUserAssets 刷新用户资产（手动刷新）
func (s *service) RefreshUserAssets(walletAddress string, chainID int64) error {
	logger.Info("Refreshing user assets", "wallet_address", walletAddress, "chain_id", chainID)
	return s.refreshUserAssetsInternal(walletAddress, chainID)
}

// RefreshUserAssetsOnChainConnect 用户连接钱包时刷新该链上的资产
func (s *service) RefreshUserAssetsOnChainConnect(walletAddress string, chainID int64) error {
	logger.Info("Refreshing user assets on chain connect", "wallet_address", walletAddress, "chain_id", chainID)
	return s.refreshUserAssetsInternal(walletAddress, chainID)
}

// refreshUserAssetsInternal 内部资产刷新逻辑
func (s *service) refreshUserAssetsInternal(walletAddress string, chainID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 按钱包地址+链ID获取用户信息
	user, err := s.userRepo.GetByWalletAndChain(walletAddress, int(chainID))
	if err != nil {
		logger.Error("Failed to get user", err, "wallet_address", walletAddress, "chain_id", chainID)
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found: %s on chain %d", walletAddress, chainID)
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

		// 过滤余额为0的资产，不保存到数据库
		if balance.Cmp(big.NewInt(0)) == 0 {
			logger.Info("Skipping zero balance token", "chain_id", chainID, "token", chainToken.Token.Symbol, "wallet_address", walletAddress)
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

	// 使用 UPSERT 逻辑批量保存到数据库（只保存余额>0的记录）
	if len(assets) > 0 {
		if err := s.assetRepo.BatchUpsertUserAssets(assets); err != nil {
			logger.Error("Failed to upsert user assets", err, "wallet_address", walletAddress, "chain_id", chainID)
			return fmt.Errorf("failed to upsert user assets: %w", err)
		}
	}

	// 保存到缓存
	if err := s.cacheUserAssets(walletAddress, chainID, assets); err != nil {
		logger.Error("Failed to cache user assets", err, "wallet_address", walletAddress, "chain_id", chainID)
		// 不返回错误，缓存失败不影响主要功能
	}

	logger.Info("Refreshed user assets", "wallet_address", walletAddress, "chain_id", chainID, "assets_count", len(assets), "user_id", user.ID)
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
