package types

import (
	"time"
)

// TransactionStatus 交易状态
type TransactionStatus string

const (
	TransactionSubmitting   TransactionStatus = "submitting"    // 正在提交到timelock合约
	TransactionQueued       TransactionStatus = "queued"        // 已成功提交到timelock，等待ETA
	TransactionReady        TransactionStatus = "ready"         // 就绪（可以执行）
	TransactionExecuting    TransactionStatus = "executing"     // 执行中（等待区块链确认）
	TransactionExecuted     TransactionStatus = "executed"      // 已执行并确认
	TransactionFailed       TransactionStatus = "failed"        // 执行失败
	TransactionSubmitFailed TransactionStatus = "submit_failed" // 提交失败
	TransactionExpired      TransactionStatus = "expired"       // 已过期
	TransactionCanceled     TransactionStatus = "canceled"      // 已取消
)

// Transaction 交易记录模型
type Transaction struct {
	ID               int64             `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatorAddress   string            `json:"creator_address" gorm:"size:42;not null;index"`         // 交易创建者地址
	ChainID          int               `json:"chain_id" gorm:"not null;index"`                        // 链ID
	ChainName        string            `json:"chain_name" gorm:"size:50;not null"`                    // 链名称
	TimelockAddress  string            `json:"timelock_address" gorm:"size:42;not null;index"`        // Timelock合约地址
	TimelockStandard TimeLockStandard  `json:"timelock_standard" gorm:"size:20;not null"`             // Timelock标准
	TxHash           string            `json:"tx_hash" gorm:"size:66;not null;unique;index"`          // 交易哈希
	TxData           string            `json:"tx_data" gorm:"type:text;not null"`                     // 交易数据
	Target           string            `json:"target" gorm:"size:42;not null"`                        // 目标合约地址
	Value            string            `json:"value" gorm:"size:100;not null;default:'0'"`            // 转账金额(wei)
	FunctionSig      string            `json:"function_sig" gorm:"size:200"`                          // 函数签名
	OperationID      string            `json:"operation_id" gorm:"size:66;index"`                     // OpenZeppelin操作ID (32字节哈希)
	ETA              int64             `json:"eta" gorm:"not null;index"`                             // 预计执行时间(Unix时间戳)
	QueuedAt         *time.Time        `json:"queued_at"`                                             // 入队时间
	ExecutedAt       *time.Time        `json:"executed_at"`                                           // 执行时间
	CanceledAt       *time.Time        `json:"canceled_at"`                                           // 取消时间
	Status           TransactionStatus `json:"status" gorm:"size:20;not null;default:'queued';index"` // 状态
	Description      string            `json:"description" gorm:"size:500"`                           // 交易描述
	CreatedAt        time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (Transaction) TableName() string {
	return "transactions"
}

// CreateTransactionRequest 创建交易请求
type CreateTransactionRequest struct {
	ChainID          int              `json:"chain_id" binding:"required"`
	ChainName        string           `json:"chain_name" binding:"required"`
	TimelockAddress  string           `json:"timelock_address" binding:"required,len=42"`
	TimelockStandard TimeLockStandard `json:"timelock_standard" binding:"required,oneof=compound openzeppelin"`
	TxHash           string           `json:"tx_hash" binding:"required,len=66"`
	TxData           string           `json:"tx_data" binding:"required"`
	Target           string           `json:"target" binding:"required,len=42"`
	Value            string           `json:"value"`
	FunctionSig      string           `json:"function_sig"`
	OperationID      string           `json:"operation_id"` // OpenZeppelin操作ID
	ETA              int64            `json:"eta" binding:"required"`
	Description      string           `json:"description" binding:"max=500"`
}

// GetTransactionListRequest 获取交易列表请求
type GetTransactionListRequest struct {
	ChainID          *int               `json:"chain_id" form:"chain_id"`
	TimelockAddress  *string            `json:"timelock_address" form:"timelock_address"`
	TimelockStandard *TimeLockStandard  `json:"timelock_standard" form:"timelock_standard"`
	Status           *TransactionStatus `json:"status" form:"status"`
	Page             int                `json:"page" form:"page" binding:"min=1"`
	PageSize         int                `json:"page_size" form:"page_size" binding:"min=1,max=100"`
}

// GetTransactionListResponse 获取交易列表响应
type GetTransactionListResponse struct {
	Transactions []TransactionWithPermission `json:"transactions"`
	Total        int64                       `json:"total"`
	Page         int                         `json:"page"`
	PageSize     int                         `json:"page_size"`
	TotalPages   int                         `json:"total_pages"`
}

// TransactionWithPermission 带权限信息的交易
type TransactionWithPermission struct {
	Transaction
	UserPermissions []string `json:"user_permissions"` // creator, executor, canceller
	CanExecute      bool     `json:"can_execute"`      // 是否可以执行
	CanCancel       bool     `json:"can_cancel"`       // 是否可以取消
	CanRetrySubmit  bool     `json:"can_retry_submit"` // 是否可以重试提交
	TimeRemaining   int64    `json:"time_remaining"`   // 剩余时间(秒)，负数表示已过期
	StatusMessage   string   `json:"status_message"`   // 状态消息
}

// ExecuteTransactionRequest 执行交易请求
type ExecuteTransactionRequest struct {
	ID            int64  `json:"id" binding:"required"`
	ExecuteTxHash string `json:"execute_tx_hash" binding:"required,len=66"` // 执行交易的哈希
}

// CancelTransactionRequest 取消交易请求
type CancelTransactionRequest struct {
	ID           int64  `json:"id" binding:"required"`
	CancelTxHash string `json:"cancel_tx_hash" binding:"required,len=66"` // 取消交易的哈希
}

// MarkTransactionFailedRequest 标记交易失败请求
type MarkTransactionFailedRequest struct {
	ID     int64  `json:"id" binding:"required"`
	Reason string `json:"reason" binding:"required,max=200"` // 失败原因
}

// MarkTransactionSubmitFailedRequest 标记交易提交失败请求
type MarkTransactionSubmitFailedRequest struct {
	ID     int64  `json:"id" binding:"required"`
	Reason string `json:"reason" binding:"required,max=200"` // 提交失败原因
}

// RetrySubmitTransactionRequest 重试提交交易请求
type RetrySubmitTransactionRequest struct {
	ID     int64  `json:"id" binding:"required"`
	TxHash string `json:"tx_hash" binding:"required,len=66"` // 新的交易哈希
}

// TransactionDetailResponse 交易详情响应
type TransactionDetailResponse struct {
	TransactionWithPermission
	TimelockInfo interface{} `json:"timelock_info"` // Timelock合约信息
}

// GetPendingTransactionsRequest 获取待处理交易请求
type GetPendingTransactionsRequest struct {
	ChainID     *int `json:"chain_id" form:"chain_id"`
	Page        int  `json:"page" form:"page" binding:"min=1"`
	PageSize    int  `json:"page_size" form:"page_size" binding:"min=1,max=100"`
	OnlyCanExec bool `json:"only_can_exec" form:"only_can_exec"` // 只显示当前用户可执行的
}

// TransactionStatsResponse 交易统计响应
type TransactionStatsResponse struct {
	TotalTransactions int64 `json:"total_transactions"`
	SubmittingCount   int64 `json:"submitting_count"`
	QueuedCount       int64 `json:"queued_count"`
	ReadyCount        int64 `json:"ready_count"`
	ExecutingCount    int64 `json:"executing_count"`
	ExecutedCount     int64 `json:"executed_count"`
	FailedCount       int64 `json:"failed_count"`
	SubmitFailedCount int64 `json:"submit_failed_count"`
	ExpiredCount      int64 `json:"expired_count"`
	CanceledCount     int64 `json:"canceled_count"`
}
