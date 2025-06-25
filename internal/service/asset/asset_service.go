package asset

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"
	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/asset"
	"timelocker-backend/internal/repository/chain"
	"timelocker-backend/internal/repository/user"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// Service 资产服务接口 - 简化版
type Service interface {
	GetUserAssets(walletAddress string) (*types.UserAssetResponse, error)
	RefreshUserAssets(walletAddress string) error
}

// service 资产服务实现
type service struct {
	config      *config.CovalentConfig
	userRepo    user.Repository
	chainRepo   chain.Repository
	assetRepo   asset.Repository
	redisClient *redis.Client
	httpClient  *http.Client
}

// NewService 创建新的资产服务
func NewService(
	cfg *config.CovalentConfig,
	userRepo user.Repository,
	chainRepo chain.Repository,
	assetRepo asset.Repository,
	redisClient *redis.Client,
) Service {
	return &service{
		config:      cfg,
		userRepo:    userRepo,
		chainRepo:   chainRepo,
		assetRepo:   assetRepo,
		redisClient: redisClient,
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
	}
}

// GetUserAssets 获取用户资产（优先从数据库获取，如果为空则自动刷新）
func (s *service) GetUserAssets(walletAddress string) (*types.UserAssetResponse, error) {
	logger.Info("Getting user assets", "wallet_address", walletAddress)

	// 从数据库获取用户资产
	assets, err := s.assetRepo.GetUserAssets(walletAddress)
	if err != nil {
		logger.Error("Failed to get user assets from database", err, "wallet_address", walletAddress)
		return nil, fmt.Errorf("failed to get user assets: %w", err)
	}

	// 如果没有数据，则自动刷新
	if len(assets) == 0 {
		logger.Info("No assets found, refreshing automatically", "wallet_address", walletAddress)
		if err := s.RefreshUserAssets(walletAddress); err != nil {
			logger.Error("Failed to refresh user assets", err, "wallet_address", walletAddress)
		} else {
			// 重新获取数据
			assets, err = s.assetRepo.GetUserAssets(walletAddress)
			if err != nil {
				logger.Error("Failed to get user assets after refresh", err, "wallet_address", walletAddress)
				return nil, fmt.Errorf("failed to get user assets after refresh: %w", err)
			}
		}
	}

	// 获取链信息映射
	chains, err := s.chainRepo.GetAllActiveChains()
	if err != nil {
		logger.Error("Failed to get chains", err)
		return nil, fmt.Errorf("failed to get chains: %w", err)
	}

	chainMap := make(map[string]*types.SupportChain)
	for _, chain := range chains {
		chainMap[chain.ChainName] = chain
	}

	// 构建响应
	response := &types.UserAssetResponse{
		WalletAddress: walletAddress,
		Assets:        make([]types.AssetInfo, 0),
		LastUpdated:   time.Now(),
	}

	totalUSDValue := 0.0
	for _, asset := range assets {
		chain := chainMap[asset.ChainName]
		if chain == nil {
			continue
		}

		assetInfo := types.AssetInfo{
			ChainName:        asset.ChainName,
			ChainDisplayName: chain.DisplayName,
			ChainID:          chain.ChainID,
			ContractAddress:  asset.ContractAddress,
			TokenSymbol:      asset.TokenSymbol,
			TokenName:        asset.TokenName,
			TokenDecimals:    asset.TokenDecimals,
			Balance:          asset.Balance,
			BalanceWei:       asset.BalanceWei,
			USDValue:         asset.USDValue,
			TokenPrice:       asset.TokenPrice,
			PriceChange24h:   asset.PriceChange24h,
			IsNative:         asset.IsNative,
			IsTestnet:        chain.IsTestnet,
			TokenLogoURL:     asset.TokenLogoURL,
			ChainLogoURL:     asset.ChainLogoURL,
			LastUpdated:      asset.LastUpdated,
		}

		response.Assets = append(response.Assets, assetInfo)

		// 测试网不计入总价值
		if !chain.IsTestnet {
			totalUSDValue += asset.USDValue
		}
	}

	// 按优先级排序：
	// 1. 主网资产按USD价值从高到低
	// 2. 测试网资产按余额从高到低（USD价值都是0）
	sort.Slice(response.Assets, func(i, j int) bool {
		assetI := response.Assets[i]
		assetJ := response.Assets[j]

		// 主网资产优先于测试网资产
		if !assetI.IsTestnet && assetJ.IsTestnet {
			return true
		}
		if assetI.IsTestnet && !assetJ.IsTestnet {
			return false
		}

		// 如果都是主网或都是测试网，按USD价值排序
		if assetI.USDValue != assetJ.USDValue {
			return assetI.USDValue > assetJ.USDValue
		}

		// 如果USD价值相同（测试网都是0），按代币符号排序
		return assetI.TokenSymbol < assetJ.TokenSymbol
	})

	response.TotalUSDValue = totalUSDValue

	logger.Info("Got user assets", "wallet_address", walletAddress, "total_usd_value", totalUSDValue, "assets_count", len(response.Assets))
	return response, nil
}

// RefreshUserAssets 刷新用户资产（从Covalent API获取）
func (s *service) RefreshUserAssets(walletAddress string) error {
	logger.Info("Refreshing user assets", "wallet_address", walletAddress)

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

	// 遍历每条链，调用Covalent API
	for _, chain := range chains {
		logger.Info("Fetching assets for chain", "chain_name", chain.ChainName, "wallet_address", walletAddress)

		assets, err := s.fetchAssetsFromCovalent(walletAddress, chain.ChainName)
		if err != nil {
			logger.Error("Failed to fetch assets from Covalent", err, "chain_name", chain.ChainName, "wallet_address", walletAddress)
			continue // 继续处理其他链
		}

		// 处理获取到的资产
		for _, covalentAsset := range assets {
			// 测试网：只显示原生代币
			if chain.IsTestnet && !covalentAsset.NativeToken {
				continue
			}

			// 测试网余额为0也不显示
			if chain.IsTestnet && covalentAsset.Balance == "0" {
				continue
			}

			// 计算24h价格涨幅（百分比）
			priceChange24h := 0.0
			if covalentAsset.QuoteRate24h > 0 && covalentAsset.QuoteRate > 0 {
				priceChange24h = ((covalentAsset.QuoteRate - covalentAsset.QuoteRate24h) / covalentAsset.QuoteRate24h) * 100
			}

			// 转换为我们的数据结构
			asset := &types.UserAsset{
				WalletAddress:   walletAddress,
				ChainName:       chain.ChainName,
				ContractAddress: covalentAsset.ContractAddress,
				TokenSymbol:     covalentAsset.ContractTickerSymbol,
				TokenName:       covalentAsset.ContractName,
				TokenDecimals:   covalentAsset.ContractDecimals,
				BalanceWei:      covalentAsset.Balance,
				TokenLogoURL:    covalentAsset.LogoURL,
				ChainLogoURL:    chain.LogoURL,
				IsNative:        covalentAsset.NativeToken,
				PriceChange24h:  priceChange24h,
				LastUpdated:     time.Now(),
			}

			// 测试网：只显示原生代币，金额为0
			if chain.IsTestnet {
				asset.USDValue = 0
				asset.TokenPrice = 0
				// 计算格式化的余额，但不计算价值
				asset.Balance = s.formatBalance(covalentAsset.Balance, covalentAsset.ContractDecimals)
			} else {
				// 主网：只显示有余额的资产
				if covalentAsset.Balance == "0" || covalentAsset.Quote <= 0 {
					continue
				}
				asset.USDValue = covalentAsset.Quote
				asset.TokenPrice = covalentAsset.QuoteRate
				// 计算格式化的余额
				asset.Balance = s.formatBalance(covalentAsset.Balance, covalentAsset.ContractDecimals)
			}

			// 原生代币的合约地址设为空字符串
			if covalentAsset.NativeToken {
				asset.ContractAddress = ""
			}

			// 尝试从LogoURLs获取更准确的Logo
			if covalentAsset.LogoURLs.TokenLogoURL != "" {
				asset.TokenLogoURL = covalentAsset.LogoURLs.TokenLogoURL
			}
			if covalentAsset.LogoURLs.ChainLogoURL != "" {
				asset.ChainLogoURL = covalentAsset.LogoURLs.ChainLogoURL
			}

			allAssets = append(allAssets, asset)
		}
	}

	// 构建链映射表以便排序时使用
	chainSortMap := make(map[string]bool) // chainName -> isTestnet
	for _, chain := range chains {
		chainSortMap[chain.ChainName] = chain.IsTestnet
	}

	// 按优先级排序：
	// 1. 主网资产按USD价值从高到低
	// 2. 测试网资产按余额从高到低（USD价值都是0）
	sort.Slice(allAssets, func(i, j int) bool {
		assetI := allAssets[i]
		assetJ := allAssets[j]

		// 获取链信息以判断是否为测试网
		isTestnetI := chainSortMap[assetI.ChainName]
		isTestnetJ := chainSortMap[assetJ.ChainName]

		// 主网资产优先于测试网资产
		if !isTestnetI && isTestnetJ {
			return true
		}
		if isTestnetI && !isTestnetJ {
			return false
		}

		// 如果都是主网或都是测试网，按USD价值排序
		if assetI.USDValue != assetJ.USDValue {
			return assetI.USDValue > assetJ.USDValue
		}

		// 如果USD价值相同（测试网都是0），按代币符号排序
		return assetI.TokenSymbol < assetJ.TokenSymbol
	})

	// 批量保存到数据库（先清空再插入）
	if err := s.assetRepo.ClearUserAssets(walletAddress); err != nil {
		logger.Error("Failed to clear user assets", err, "wallet_address", walletAddress)
		return fmt.Errorf("failed to clear user assets: %w", err)
	}

	if len(allAssets) > 0 {
		if err := s.assetRepo.BatchUpsertUserAssets(allAssets); err != nil {
			logger.Error("Failed to upsert user assets", err, "wallet_address", walletAddress)
			return fmt.Errorf("failed to upsert user assets: %w", err)
		}
	}

	// 缓存到Redis
	if err := s.cacheUserAssets(walletAddress, allAssets); err != nil {
		logger.Error("Failed to cache user assets", err, "wallet_address", walletAddress)
		// 不影响主要流程
	}

	logger.Info("Refreshed user assets", "wallet_address", walletAddress, "total_assets_count", len(allAssets))
	return nil
}

// fetchAssetsFromCovalent 从Covalent API获取资产数据 - 使用新的URL格式
func (s *service) fetchAssetsFromCovalent(walletAddress string, chainName string) ([]types.CovalentAssetItem, error) {
	url := fmt.Sprintf("%s/%s/address/%s/balances_v2/?key=%s", s.config.BaseURL, chainName, walletAddress, s.config.APIKey)

	logger.Info("Making Covalent API request", "url", strings.Replace(url, s.config.APIKey, "***", 1), "chain_name", chainName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var response types.CovalentAssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error {
		return nil, fmt.Errorf("API returned error response")
	}

	logger.Info("Covalent API response", "chain_name", chainName, "items_count", len(response.Data.Items))
	return response.Data.Items, nil
}

// formatBalance 格式化余额（从wei转换为可读格式）
func (s *service) formatBalance(balanceWei string, decimals int) string {
	balance := new(big.Int)
	balance.SetString(balanceWei, 10)

	if decimals == 0 {
		return balance.String()
	}

	// 创建除数 (10^decimals)
	divisor := new(big.Int)
	divisor.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	// 整数部分
	quotient := new(big.Int)
	quotient.Div(balance, divisor)

	// 小数部分
	remainder := new(big.Int)
	remainder.Mod(balance, divisor)

	if remainder.Cmp(big.NewInt(0)) == 0 {
		return quotient.String()
	}

	// 格式化小数部分，去掉尾随零
	decimalStr := remainder.String()
	decimalStr = strings.TrimRight(fmt.Sprintf("%0*s", decimals, decimalStr), "0")

	if decimalStr == "" {
		return quotient.String()
	}

	return quotient.String() + "." + decimalStr
}

// cacheUserAssets 缓存用户资产到Redis
func (s *service) cacheUserAssets(walletAddress string, assets []*types.UserAsset) error {
	key := s.config.CachePrefix + walletAddress

	data, err := json.Marshal(assets)
	if err != nil {
		return fmt.Errorf("failed to marshal assets: %w", err)
	}

	ctx := context.Background()
	expiration := time.Duration(s.config.CacheExpiry) * time.Second

	return s.redisClient.Set(ctx, key, data, expiration).Err()
}
