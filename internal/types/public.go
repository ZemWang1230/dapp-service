package types

// GetStatsRequest 获取统计数据请求
type GetStatsRequest struct{}

// GetStatsResponse 获取统计数据响应
type GetStatsResponse struct {
	ChainCount       int64 `json:"chain_count"`       // 支持的链数量
	ContractCount    int64 `json:"contract_count"`    // timelock合约数量
	TransactionCount int64 `json:"transaction_count"` // 交易数量
}
