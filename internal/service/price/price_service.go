package price

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/token"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// Service 价格服务接口
type Service interface {
	Start(ctx context.Context) error
	Stop() error
	GetPrice(symbol string) (*types.TokenPrice, error)
	GetAllPrices() (map[string]*types.TokenPrice, error)
}

// service 价格服务实现
type service struct {
	config      *config.PriceConfig
	tokenRepo   token.Repository
	redisClient *redis.Client
	httpClient  *http.Client
	ticker      *time.Ticker
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewService 创建新的价格服务
func NewService(cfg *config.PriceConfig, tokenRepo token.Repository, redisClient *redis.Client) Service {
	return &service{
		config:      cfg,
		tokenRepo:   tokenRepo,
		redisClient: redisClient,
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
			},
		},
		stopCh: make(chan struct{}),
	}
}

// Start 启动价格服务
func (s *service) Start(ctx context.Context) error {
	logger.Info("Price service starting", "provider", s.config.Provider, "interval", s.config.UpdateInterval)

	// 立即执行一次价格更新
	if err := s.updatePrices(ctx); err != nil {
		logger.Error("Initial price update failed", err)
	}

	// 启动定时更新
	s.ticker = time.NewTicker(s.config.UpdateInterval)
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.ticker.C:
				if err := s.updatePrices(ctx); err != nil {
					logger.Error("Price update failed", err)
				}
			case <-s.stopCh:
				logger.Info("Price service stopping")
				return
			}
		}
	}()

	logger.Info("Price service started successfully")
	return nil
}

// Stop 停止价格服务
func (s *service) Stop() error {
	if s.ticker != nil {
		s.ticker.Stop()
	}

	close(s.stopCh)
	s.wg.Wait()

	logger.Info("Price service stopped")
	return nil
}

// GetPrice 获取指定代币的价格
func (s *service) GetPrice(symbol string) (*types.TokenPrice, error) {
	ctx := context.Background()
	key := s.config.CachePrefix + strings.ToUpper(symbol)

	result, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			logger.Info("Price not found in cache", "symbol", symbol)
			return nil, nil
		}
		logger.Error("Failed to get price from cache", err, "symbol", symbol)
		return nil, err
	}

	var price types.TokenPrice
	if err := json.Unmarshal([]byte(result), &price); err != nil {
		logger.Error("Failed to unmarshal price data", err, "symbol", symbol)
		return nil, err
	}

	return &price, nil
}

// GetAllPrices 获取所有代币价格
func (s *service) GetAllPrices() (map[string]*types.TokenPrice, error) {
	ctx := context.Background()
	pattern := s.config.CachePrefix + "*"

	keys, err := s.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		logger.Error("Failed to get price keys from cache", err)
		return nil, err
	}

	if len(keys) == 0 {
		return make(map[string]*types.TokenPrice), nil
	}

	results, err := s.redisClient.MGet(ctx, keys...).Result()
	if err != nil {
		logger.Error("Failed to get prices from cache", err)
		return nil, err
	}

	prices := make(map[string]*types.TokenPrice)
	for i, result := range results {
		if result == nil {
			continue
		}

		var price types.TokenPrice
		if err := json.Unmarshal([]byte(result.(string)), &price); err != nil {
			logger.Error("Failed to unmarshal price data", err, "key", keys[i])
			continue
		}

		// 从key中提取symbol（去掉前缀）
		symbol := strings.TrimPrefix(keys[i], s.config.CachePrefix)
		prices[symbol] = &price
	}

	return prices, nil
}

// updatePrices 更新所有代币价格
func (s *service) updatePrices(ctx context.Context) error {
	logger.Info("Starting price update")

	// 获取所有激活的代币
	tokens, err := s.tokenRepo.GetAllActiveTokens()
	if err != nil {
		return fmt.Errorf("failed to get active tokens: %w", err)
	}

	if len(tokens) == 0 {
		logger.Info("No active tokens found")
		return nil
	}

	// 根据价格源更新价格
	switch strings.ToLower(s.config.Provider) {
	case "coingecko":
		return s.updatePricesFromCoinGecko(ctx, tokens)
	default:
		return fmt.Errorf("unsupported price provider: %s", s.config.Provider)
	}
}

// updatePricesFromCoinGecko 从CoinGecko更新价格（目前使用随机数据替代）
func (s *service) updatePricesFromCoinGecko(ctx context.Context, tokens []*types.SupportToken) error {
	// 构建token映射
	tokenMap := make(map[string]*types.SupportToken)
	for _, token := range tokens {
		if token.CoingeckoID != "" {
			tokenMap[token.CoingeckoID] = token
		}
	}

	if len(tokenMap) == 0 {
		logger.Info("No tokens with CoinGecko ID found")
		return nil
	}

	// 调用CoinGecko APIAdd commentMore actions
	// url := fmt.Sprintf("%s/simple/price?ids=%s&vs_currencies=usd&include_24hr_change=true",
	// 	s.config.BaseURL,
	// 	strings.Join(ids, ","))

	// logger.Info("Making price request", "url", url, "token_count", len(ids))

	// req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	// if err != nil {
	// 	logger.Error("Failed to create request", err)
	// 	return fmt.Errorf("failed to create request: %w", err)
	// }

	// req.Header.Set("Accept", "application/json")

	// // 添加API Key
	// if s.config.APIKey != "" {
	// 	req.Header.Set("X-CG-Demo-API-Key", s.config.APIKey)
	// }

	// resp, err := s.httpClient.Do(req)
	// if err != nil {
	// 	logger.Error("Failed to make request", err)
	// 	return fmt.Errorf("failed to make request: %w", err)
	// }
	// defer resp.Body.Close()

	// if resp.StatusCode != http.StatusOK {
	// 	logger.Error("API request failed with status", fmt.Errorf("status: %d", resp.StatusCode))
	// 	return fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	// }

	// var priceData types.CoinGeckoPriceResponse
	// if err := json.NewDecoder(resp.Body).Decode(&priceData); err != nil {
	// 	logger.Error("Failed to decode response", err)
	// 	return fmt.Errorf("failed to decode response: %w", err)
	// }

	// // 更新Redis缓存
	// now := time.Now()
	// for coingeckoID, priceInfo := range priceData {
	// 	token, exists := tokenMap[coingeckoID]
	// 	if !exists {
	// 		continue
	// 	}

	// 	tokenPrice := &types.TokenPrice{
	// 		Symbol:      strings.ToUpper(token.Symbol),
	// 		Name:        token.Name,
	// 		Price:       priceInfo.USD,
	// 		Change24h:   priceInfo.USD24hChange,
	// 		LastUpdated: now,
	// 	}

	// 	// 保存到Redis
	// 	if err := s.savePriceToCache(ctx, tokenPrice); err != nil {
	// 		logger.Error("Failed to save price to cache", err, "symbol", tokenPrice.Symbol)
	// 	} else {
	// 		logger.Info("Price updated", "symbol", tokenPrice.Symbol, "price", tokenPrice.Price)
	// 	}
	// }

	// logger.Info("Price update completed", "tokens_updated", len(priceData))

	// 网络原因，此处利用随机数据替代，并更新redis缓存
	now := time.Now()
	for _, token := range tokens {
		tokenPrice := &types.TokenPrice{
			Symbol:      strings.ToUpper(token.Symbol),
			Name:        token.Name,
			Price:       rand.Float64() * 1000,
			Change24h:   rand.Float64() * 100,
			LastUpdated: now,
		}

		if err := s.savePriceToCache(ctx, tokenPrice); err != nil {
			logger.Error("Failed to save price to cache", err, "symbol", tokenPrice.Symbol)
		} else {
			logger.Info("Price updated", "symbol", tokenPrice.Symbol, "price", tokenPrice.Price)
		}
	}

	logger.Info("Random price update completed", "tokens_updated", len(tokens))
	return nil
}

// savePriceToCache 保存价格到缓存
func (s *service) savePriceToCache(ctx context.Context, price *types.TokenPrice) error {
	key := s.config.CachePrefix + price.Symbol

	data, err := json.Marshal(price)
	if err != nil {
		logger.Error("Failed to marshal price data", err)
		return fmt.Errorf("failed to marshal price data: %w", err)
	}

	// 设置过期时间为更新间隔的2倍，确保数据不会过期
	expiration := s.config.UpdateInterval * 2

	return s.redisClient.Set(ctx, key, data, expiration).Err()
}
