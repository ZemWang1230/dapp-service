package types

import "time"

// SupportToken 支持的代币模型
type SupportToken struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Symbol      string    `json:"symbol" gorm:"uniqueIndex:idx_support_tokens_symbol;size:10;not null"`    // 代币符号，如 BTC, ETH, USDC
	Name        string    `json:"name" gorm:"size:100;not null"`                                           // 代币名称，如 Bitcoin, Ethereum, USD Coin
	CoingeckoID string    `json:"coingecko_id" gorm:"uniqueIndex:idx_support_tokens_coingecko_id;size:50"` // CoinGecko ID，如 bitcoin, ethereum, usd-coin
	Decimals    int       `json:"decimals" gorm:"not null;default:18"`                                     // 代币精度
	IsActive    bool      `json:"is_active" gorm:"not null;default:true"`                                  // 是否激活价格查询
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (SupportToken) TableName() string {
	return "support_tokens"
}

// TokenPrice 代币价格缓存结构（用于Redis）
type TokenPrice struct {
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	Price       float64   `json:"price"` // USD价格
	LastUpdated time.Time `json:"last_updated"`
	Change24h   float64   `json:"change_24h"` // 24小时变化百分比
}

// CoinGeckoResponse CoinGecko API响应结构
type CoinGeckoResponse struct {
	ID             string  `json:"id"`
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name"`
	CurrentPrice   float64 `json:"current_price"`
	PriceChange24h float64 `json:"price_change_percentage_24h"`
	LastUpdated    string  `json:"last_updated"`
}

// CoinGeckoPriceResponse CoinGecko批量价格查询响应
type CoinGeckoPriceResponse map[string]struct {
	USD          float64 `json:"usd"`
	USD24hChange float64 `json:"usd_24h_change"`
}
