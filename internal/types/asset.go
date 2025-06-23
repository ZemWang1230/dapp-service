package types

import (
	"math/big"
	"time"
)

// SupportChain 支持的区块链模型
type SupportChain struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ChainID     int64     `json:"chain_id" gorm:"not null;unique"`                        // 区块链ID，如 1(以太坊), 56(BSC)
	Name        string    `json:"name" gorm:"size:50;not null"`                           // 链名称，如 Ethereum, BSC
	Symbol      string    `json:"symbol" gorm:"size:10;not null"`                         // 原生代币符号，如 ETH, BNB
	RpcProvider string    `json:"rpc_provider" gorm:"size:20;not null;default:'alchemy'"` // RPC提供商，如 alchemy, infura
	IsActive    bool      `json:"is_active" gorm:"not null;default:true"`                 // 是否激活
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (SupportChain) TableName() string {
	return "support_chains"
}

// ChainToken 链上代币配置模型（关联链和代币，包含合约地址）
type ChainToken struct {
	ID              int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ChainID         int64     `json:"chain_id" gorm:"not null;index:idx_chain_tokens_chain_id;uniqueIndex:idx_chain_tokens_unique"` // 关联support_chains表
	TokenID         int64     `json:"token_id" gorm:"not null;index:idx_chain_tokens_token_id;uniqueIndex:idx_chain_tokens_unique"` // 关联support_tokens表
	ContractAddress string    `json:"contract_address" gorm:"size:42"`                                                              // ERC-20合约地址，原生代币为空
	IsNative        bool      `json:"is_native" gorm:"not null;default:false"`                                                      // 是否为原生代币
	IsActive        bool      `json:"is_active" gorm:"not null;default:true"`                                                       // 是否在该链上激活
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// 关联查询
	Chain *SupportChain `json:"chain,omitempty" gorm:"foreignKey:ChainID;references:ID"`
	Token *SupportToken `json:"token,omitempty" gorm:"foreignKey:TokenID;references:ID"`
}

// TableName 设置表名
func (ChainToken) TableName() string {
	return "chain_tokens"
}

// UserAsset 用户资产模型
type UserAsset struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID        int64     `json:"user_id" gorm:"not null;index:idx_user_assets_user_id"`                       // 关联users表
	WalletAddress string    `json:"wallet_address" gorm:"size:42;not null;index:idx_user_assets_wallet_address"` // 钱包地址
	ChainID       int64     `json:"chain_id" gorm:"not null;index:idx_user_assets_chain_id"`                     // 区块链ID
	TokenID       int64     `json:"token_id" gorm:"not null;index:idx_user_assets_token_id"`                     // 代币ID
	Balance       string    `json:"balance" gorm:"type:varchar(100);not null;default:'0'"`                       // 余额，使用字符串存储避免精度问题
	BalanceWei    string    `json:"balance_wei" gorm:"type:varchar(100);not null;default:'0'"`                   // Wei单位余额
	USDValue      float64   `json:"usd_value" gorm:"type:decimal(20,8);default:0"`                               // USD价值
	LastUpdated   time.Time `json:"last_updated" gorm:"autoUpdateTime"`                                          // 最后更新时间
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// 关联查询
	User  *User         `json:"user,omitempty" gorm:"foreignKey:UserID;references:ID"`
	Token *SupportToken `json:"token,omitempty" gorm:"foreignKey:TokenID;references:ID"`
}

// TableName 设置表名
func (UserAsset) TableName() string {
	return "user_assets"
}

// GetBalanceBigInt 获取大整数格式的余额
func (ua *UserAsset) GetBalanceBigInt() *big.Int {
	balance := new(big.Int)
	balance.SetString(ua.BalanceWei, 10)
	return balance
}

// SetBalanceFromBigInt 从大整数设置余额
func (ua *UserAsset) SetBalanceFromBigInt(balance *big.Int, decimals int) {
	ua.BalanceWei = balance.String()

	// 计算可读格式的余额
	divisor := new(big.Int)
	divisor.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	quotient := new(big.Int)
	quotient.Div(balance, divisor)

	remainder := new(big.Int)
	remainder.Mod(balance, divisor)

	// 简单的格式化，避免浮点数精度问题
	if remainder.Cmp(big.NewInt(0)) == 0 {
		ua.Balance = quotient.String()
	} else {
		// 这里可以根据需要实现更精确的小数处理
		ua.Balance = quotient.String() + "." + remainder.String()
	}
}

// AssetQueryRequest 资产查询请求
type AssetQueryRequest struct {
	WalletAddress string `json:"wallet_address" binding:"required,len=42"`
	ChainID       int64  `json:"chain_id" binding:"required"`
	ForceRefresh  bool   `json:"force_refresh"`
}

// AssetInfo 资产信息
type AssetInfo struct {
	TokenSymbol  string    `json:"token_symbol"`
	TokenName    string    `json:"token_name"`
	ContractAddr string    `json:"contract_address,omitempty"`
	Balance      string    `json:"balance"`
	BalanceWei   string    `json:"balance_wei"`
	USDValue     float64   `json:"usd_value"`
	TokenPrice   float64   `json:"token_price"`
	Change24h    float64   `json:"change_24h"`
	IsNative     bool      `json:"is_native"`
	LastUpdated  time.Time `json:"last_updated"`
}

// ChainAssetInfo 链资产信息
type ChainAssetInfo struct {
	ChainID       int64       `json:"chain_id"`
	ChainName     string      `json:"chain_name"`
	ChainSymbol   string      `json:"chain_symbol"`
	Assets        []AssetInfo `json:"assets"`
	TotalUSDValue float64     `json:"total_usd_value"`
	LastUpdated   time.Time   `json:"last_updated"`
}

// UserAssetResponse 用户资产查询响应
type UserAssetResponse struct {
	WalletAddress  string           `json:"wallet_address"`
	PrimaryChainID int64            `json:"primary_chain_id"`
	PrimaryChain   ChainAssetInfo   `json:"primary_chain"`
	OtherChains    []ChainAssetInfo `json:"other_chains"`
	TotalUSDValue  float64          `json:"total_usd_value"`
	LastUpdated    time.Time        `json:"last_updated"`
}
