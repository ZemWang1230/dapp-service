package types

import "time"

// GetStatsRequest 获取统计数据请求
type GetStatsRequest struct{}

// GetStatsResponse 获取统计数据响应
type GetStatsResponse struct {
	ChainCount       int64 `json:"chain_count"`       // 支持的链数量
	ContractCount    int64 `json:"contract_count"`    // timelock合约数量
	TransactionCount int64 `json:"transaction_count"` // 交易数量
}

// ChainStatistics 链统计数据
type ChainStatistics struct {
	ID               int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ChainID          int       `json:"chain_id" gorm:"not null;unique"`
	ChainName        string    `json:"chain_name" gorm:"size:100;not null"`
	ContractCount    int64     `json:"contract_count" gorm:"not null;default:0"`
	TransactionCount int64     `json:"transaction_count" gorm:"not null;default:0"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (ChainStatistics) TableName() string {
	return "chain_statistics"
}
