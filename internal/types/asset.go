package types

import (
	"time"
)

// SupportChain 支持的区块链模型（简化版）
type SupportChain struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ChainName   string    `json:"chain_name" gorm:"size:50;not null;unique"` // Covalent API的chainName
	DisplayName string    `json:"display_name" gorm:"size:100;not null"`     // 显示名称
	ChainID     int64     `json:"chain_id" gorm:"not null"`                  // 链ID
	NativeToken string    `json:"native_token" gorm:"size:10;not null"`      // 原生代币符号
	LogoURL     string    `json:"logo_url" gorm:"type:text"`                 // 链Logo URL
	IsTestnet   bool      `json:"is_testnet" gorm:"not null;default:false"`  // 是否是测试网
	IsActive    bool      `json:"is_active" gorm:"not null;default:true"`    // 是否激活
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (SupportChain) TableName() string {
	return "support_chains"
}

// UserAsset 用户资产模型 - 优化版
type UserAsset struct {
	ID              int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	WalletAddress   string    `json:"wallet_address" gorm:"size:42;not null;index:idx_user_assets_wallet_address;uniqueIndex:idx_user_assets_unique,priority:1"` // 钱包地址
	ChainName       string    `json:"chain_name" gorm:"size:50;not null;index:idx_user_assets_chain_name;uniqueIndex:idx_user_assets_unique,priority:2"`         // Covalent chainName
	ContractAddress string    `json:"contract_address" gorm:"size:42;uniqueIndex:idx_user_assets_unique,priority:3"`                                             // 合约地址
	TokenSymbol     string    `json:"token_symbol" gorm:"size:20;not null"`                                                                                      // 代币符号
	TokenName       string    `json:"token_name" gorm:"size:100;not null"`                                                                                       // 代币名称
	TokenDecimals   int       `json:"token_decimals" gorm:"not null;default:18"`                                                                                 // 代币精度
	Balance         string    `json:"balance" gorm:"type:varchar(100);not null;default:'0'"`                                                                     // 格式化余额
	BalanceWei      string    `json:"balance_wei" gorm:"type:varchar(100);not null;default:'0'"`                                                                 // Wei单位余额
	USDValue        float64   `json:"usd_value" gorm:"type:decimal(20,8);default:0"`                                                                             // USD价值
	TokenPrice      float64   `json:"token_price" gorm:"type:decimal(20,8);default:0"`                                                                           // 代币价格
	PriceChange24h  float64   `json:"price_change_24h" gorm:"type:decimal(10,4);default:0"`                                                                      // 24小时价格涨跌幅（%）
	IsNative        bool      `json:"is_native" gorm:"not null;default:false"`                                                                                   // 是否为原生代币
	TokenLogoURL    string    `json:"token_logo_url" gorm:"type:text"`                                                                                           // 代币Logo URL
	ChainLogoURL    string    `json:"chain_logo_url" gorm:"type:text"`                                                                                           // 链Logo URL
	LastUpdated     time.Time `json:"last_updated" gorm:"autoUpdateTime"`                                                                                        // 最后更新时间
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (UserAsset) TableName() string {
	return "user_assets"
}

// AssetInfo 资产信息响应结构 - 优化版
type AssetInfo struct {
	ChainName        string    `json:"chain_name"`
	ChainDisplayName string    `json:"chain_display_name"`
	ChainID          int64     `json:"chain_id"`
	ContractAddress  string    `json:"contract_address,omitempty"`
	TokenSymbol      string    `json:"token_symbol"`
	TokenName        string    `json:"token_name"`
	TokenDecimals    int       `json:"token_decimals"`
	Balance          string    `json:"balance"`
	BalanceWei       string    `json:"balance_wei"`
	USDValue         float64   `json:"usd_value"`
	TokenPrice       float64   `json:"token_price"`
	PriceChange24h   float64   `json:"price_change_24h"` // 24小时价格涨跌幅（%）
	IsNative         bool      `json:"is_native"`
	IsTestnet        bool      `json:"is_testnet"`
	TokenLogoURL     string    `json:"token_logo_url,omitempty"`
	ChainLogoURL     string    `json:"chain_logo_url,omitempty"`
	LastUpdated      time.Time `json:"last_updated"`
}

// UserAssetResponse 用户资产查询响应
type UserAssetResponse struct {
	WalletAddress string      `json:"wallet_address"`
	Assets        []AssetInfo `json:"assets"` // 所有支持链上的资产，按价值从高到低排序
	TotalUSDValue float64     `json:"total_usd_value"`
	LastUpdated   time.Time   `json:"last_updated"`
}

// CovalentAssetResponse Covalent API响应结构
type CovalentAssetResponse struct {
	Data  CovalentAssetData `json:"data"`
	Error bool              `json:"error"`
}

type CovalentAssetData struct {
	Address       string              `json:"address"`
	UpdatedAt     time.Time           `json:"updated_at"`
	NextUpdateAt  time.Time           `json:"next_update_at"`
	QuoteCurrency string              `json:"quote_currency"`
	ChainID       int64               `json:"chain_id"`
	ChainName     string              `json:"chain_name"`
	Items         []CovalentAssetItem `json:"items"`
	Links         CovalentLinks       `json:"links"`
}

type CovalentAssetItem struct {
	ContractDecimals     int      `json:"contract_decimals"`
	ContractName         string   `json:"contract_name"`
	ContractTickerSymbol string   `json:"contract_ticker_symbol"`
	ContractAddress      string   `json:"contract_address"`
	LogoURL              string   `json:"logo_url"`
	LogoURLs             LogoURLs `json:"logo_urls"`
	NativeToken          bool     `json:"native_token"`
	Type                 string   `json:"type"`
	Balance              string   `json:"balance"`
	Balance24h           string   `json:"balance_24h"`
	QuoteRate            float64  `json:"quote_rate"`
	QuoteRate24h         float64  `json:"quote_rate_24h"`
	Quote                float64  `json:"quote"`
	Quote24h             float64  `json:"quote_24h"`
}

type LogoURLs struct {
	TokenLogoURL string `json:"token_logo_url"`
	ChainLogoURL string `json:"chain_logo_url"`
}

type CovalentLinks struct {
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
}

// GetSupportChainsRequest 获取支持链列表请求
type GetSupportChainsRequest struct {
	IsTestnet *bool `json:"is_testnet" form:"is_testnet"` // 筛选测试网/主网
	IsActive  *bool `json:"is_active" form:"is_active"`   // 筛选激活状态
}

// GetSupportChainsResponse 获取支持链列表响应
type GetSupportChainsResponse struct {
	Chains []SupportChain `json:"chains"`
	Total  int64          `json:"total"`
}

// GetChainByIDRequest 根据ID获取链信息请求
type GetChainByIDRequest struct {
	ID int64 `json:"id" form:"id" binding:"required"`
}

// GetChainByChainIDRequest 根据ChainID获取链信息请求
type GetChainByChainIDRequest struct {
	ChainID int64 `json:"chain_id" form:"chain_id" binding:"required"`
}
