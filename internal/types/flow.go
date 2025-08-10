package types

import "time"

// GetFlowListRequest 获取流程列表请求
type GetFlowListRequest struct {
	Status   *string `form:"status"`    // 状态all, waiting, ready, executed, cancelled, expired
	Standard *string `form:"standard"`  // 标准compound, openzeppelin
	Page     int     `form:"page"`      // 页码，默认为1
	PageSize int     `form:"page_size"` // 每页大小，默认为10，最大100
}

// GetFlowListResponse 获取流程列表响应
type GetFlowListResponse struct {
	Flows []FlowResponse `json:"flows"` // 流程列表
	Total int64          `json:"total"` // 总数
}

// FlowResponse 流程响应结构
type FlowResponse struct {
	ID               int64      `json:"id"`                          // ID
	FlowID           string     `json:"flow_id"`                     // 流程ID
	TimelockStandard string     `json:"timelock_standard"`           // Timelock标准
	ChainID          int        `json:"chain_id"`                    // 链ID
	ContractAddress  string     `json:"contract_address"`            // 合约地址
	Status           string     `json:"status"`                      // 状态
	QueueTxHash      *string    `json:"queue_tx_hash,omitempty"`     // 排队交易哈希
	ExecuteTxHash    *string    `json:"execute_tx_hash,omitempty"`   // 执行交易哈希
	CancelTxHash     *string    `json:"cancel_tx_hash,omitempty"`    // 取消交易哈希
	InitiatorAddress *string    `json:"initiator_address,omitempty"` // 发起者地址(FromAddress)
	TargetAddress    *string    `json:"target_address,omitempty"`    // 目标地址
	CallDataHex      *string    `json:"call_data_hex,omitempty"`     // 调用数据
	Value            string     `json:"value"`                       // 价值
	Eta              *time.Time `json:"eta,omitempty"`               // 执行时间
	ExpiredAt        *time.Time `json:"expired_at,omitempty"`        // 过期时间
	ExecutedAt       *time.Time `json:"executed_at,omitempty"`       // 执行时间
	CancelledAt      *time.Time `json:"cancelled_at,omitempty"`      // 取消时间
	CreatedAt        time.Time  `json:"created_at"`                  // 创建时间
	UpdatedAt        time.Time  `json:"updated_at"`                  // 更新时间
}
