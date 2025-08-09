package types

import "time"

// GetTransactionDetailRequest 获取交易详情请求
type GetTransactionDetailRequest struct {
	Standard string `form:"standard" binding:"required,oneof=compound openzeppelin"` // 标准
	TxHash   string `form:"tx_hash" binding:"required"`                              // 交易哈希
}

// GetTransactionDetailResponse 获取交易详情响应
type GetTransactionDetailResponse struct {
	Detail TimelockTransactionDetail `json:"detail"` // 交易详情
}

// TimelockTransactionDetail 交易详情
type TimelockTransactionDetail struct {
	TxHash          string    `json:"tx_hash"`          // 交易哈希
	BlockNumber     int64     `json:"block_number"`     // 区块高度
	BlockTimestamp  time.Time `json:"block_timestamp"`  // 区块时间
	ChainID         int       `json:"chain_id"`         // 链ID
	ChainName       string    `json:"chain_name"`       // 链名称
	ContractAddress string    `json:"contract_address"` // 合约地址
	FromAddress     string    `json:"from_address"`     // 发起地址
	ToAddress       string    `json:"to_address"`       // 接收地址
	TxStatus        string    `json:"tx_status"`        // 交易状态（success, failed）
}
