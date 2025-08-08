package types

import "time"

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

// FlowListResponse 流程列表响应
type FlowListResponse struct {
	Flows []FlowResponse `json:"flows"` // 流程列表
	Total int64          `json:"total"` // 总数
}

// FlowDetailResponse 流程详细信息响应
type FlowDetailResponse struct {
	Flow             FlowResponse `json:"flow"`                         // 流程
	TimeToExecution  *int64       `json:"time_to_execution,omitempty"`  // 距离可执行时间的秒数
	TimeToExpiration *int64       `json:"time_to_expiration,omitempty"` // 距离过期时间的秒数（仅Compound）
}

// FlowStatsResponse 流程统计响应
type FlowStatsResponse struct {
	TotalCount     int64 `json:"total_count"`     // 总数
	WaitingCount   int64 `json:"waiting_count"`   // 等待中
	ReadyCount     int64 `json:"ready_count"`     // 准备中
	ExecutedCount  int64 `json:"executed_count"`  // 执行中
	CancelledCount int64 `json:"cancelled_count"` // 取消中
	ExpiredCount   int64 `json:"expired_count"`   // 过期中
}

// FlowQueryRequest 流程查询请求
type FlowQueryRequest struct {
	Status           *string `form:"status"`                            // 状态
	InitiatorAddress *string `form:"initiator_address"`                 // 发起者地址
	TimelockStandard *string `form:"timelock_standard"`                 // Timelock标准
	ChainID          *int    `form:"chain_id"`                          // 链ID
	Page             int     `form:"page" binding:"min=1"`              // 页码
	PageSize         int     `form:"page_size" binding:"min=1,max=100"` // 每页数量
}

// FlowDetailRequest 流程详细信息请求
type FlowDetailRequest struct {
	FlowID           string `uri:"flow_id" binding:"required"`                                        // 流程ID
	TimelockStandard string `form:"timelock_standard" binding:"required,oneof=compound openzeppelin"` // Timelock标准
	ChainID          int    `form:"chain_id" binding:"required"`                                      // 链ID
	ContractAddress  string `form:"contract_address" binding:"required,len=42"`                       // 合约地址
}

// FlowStatsRequest 流程统计请求
type FlowStatsRequest struct {
	InitiatorAddress *string `form:"initiator_address"` // 发起者地址
}
