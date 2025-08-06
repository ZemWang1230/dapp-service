package types

import (
	"math/big"
	"time"
)

// BlockScanProgress 区块扫描进度模型
type BlockScanProgress struct {
	ID                 int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ChainID            int       `json:"chain_id" gorm:"not null;unique;index"`
	ChainName          string    `json:"chain_name" gorm:"size:50;not null"`
	LastScannedBlock   int64     `json:"last_scanned_block" gorm:"not null;default:0"`
	LatestNetworkBlock int64     `json:"latest_network_block" gorm:"default:0"`
	ScanStatus         string    `json:"scan_status" gorm:"size:20;not null;default:'running';index"`
	ErrorMessage       *string   `json:"error_message" gorm:"type:text"`
	LastUpdateTime     time.Time `json:"last_update_time" gorm:"default:CURRENT_TIMESTAMP"`
	CreatedAt          time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (BlockScanProgress) TableName() string {
	return "block_scan_progress"
}

// CompoundTimelockTransaction Compound Timelock 交易记录模型
type CompoundTimelockTransaction struct {
	ID                int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TxHash            string    `json:"tx_hash" gorm:"size:66;not null;index"`
	BlockNumber       int64     `json:"block_number" gorm:"not null;index"`
	BlockTimestamp    time.Time `json:"block_timestamp" gorm:"not null"`
	ChainID           int       `json:"chain_id" gorm:"not null;index"`
	ChainName         string    `json:"chain_name" gorm:"size:50;not null"`
	ContractAddress   string    `json:"contract_address" gorm:"size:42;not null;index"`
	FromAddress       string    `json:"from_address" gorm:"size:42;not null;index"`
	ToAddress         string    `json:"to_address" gorm:"size:42;not null"`
	EventType         string    `json:"event_type" gorm:"size:50;not null;index"`
	EventData         string    `json:"event_data" gorm:"type:jsonb;not null"`
	ProposalID        *string   `json:"proposal_id" gorm:"size:128;index"`
	TargetAddress     *string   `json:"target_address" gorm:"size:42"`
	FunctionSignature *string   `json:"function_signature" gorm:"size:200"`
	CallData          []byte    `json:"call_data" gorm:"type:bytea"`
	Eta               *int64    `json:"eta"`
	Value             string    `json:"value" gorm:"type:decimal(36,0);default:0"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (CompoundTimelockTransaction) TableName() string {
	return "compound_timelock_transactions"
}

// OpenZeppelinTimelockTransaction OpenZeppelin Timelock 交易记录模型
type OpenZeppelinTimelockTransaction struct {
	ID                int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TxHash            string    `json:"tx_hash" gorm:"size:66;not null;index"`
	BlockNumber       int64     `json:"block_number" gorm:"not null;index"`
	BlockTimestamp    time.Time `json:"block_timestamp" gorm:"not null"`
	ChainID           int       `json:"chain_id" gorm:"not null;index"`
	ChainName         string    `json:"chain_name" gorm:"size:50;not null"`
	ContractAddress   string    `json:"contract_address" gorm:"size:42;not null;index"`
	FromAddress       string    `json:"from_address" gorm:"size:42;not null;index"`
	ToAddress         string    `json:"to_address" gorm:"size:42;not null"`
	EventType         string    `json:"event_type" gorm:"size:50;not null;index"`
	EventData         string    `json:"event_data" gorm:"type:jsonb;not null"`
	OperationID       *string   `json:"operation_id" gorm:"size:66;index"`
	TargetAddress     *string   `json:"target_address" gorm:"size:42"`
	FunctionSignature *string   `json:"function_signature" gorm:"size:200"`
	CallData          []byte    `json:"call_data" gorm:"type:bytea"`
	Delay             *int64    `json:"delay"`
	Value             string    `json:"value" gorm:"type:decimal(36,0);default:0"`
	CreatedAt         time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (OpenZeppelinTimelockTransaction) TableName() string {
	return "openzeppelin_timelock_transactions"
}

// TimelockTransactionFlow Timelock 交易流程关联模型
type TimelockTransactionFlow struct {
	ID                int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	FlowID            string     `json:"flow_id" gorm:"size:128;not null;index"`
	TimelockStandard  string     `json:"timelock_standard" gorm:"size:20;not null"`
	ChainID           int        `json:"chain_id" gorm:"not null;index"`
	ContractAddress   string     `json:"contract_address" gorm:"size:42;not null;index"`
	Status            string     `json:"status" gorm:"size:20;not null;default:'proposed';index"`
	ProposeTxID       *int64     `json:"propose_tx_id"`
	QueueTxID         *int64     `json:"queue_tx_id"`
	ExecuteTxID       *int64     `json:"execute_tx_id"`
	CancelTxID        *int64     `json:"cancel_tx_id"`
	ProposedAt        *time.Time `json:"proposed_at"`
	QueuedAt          *time.Time `json:"queued_at"`
	ExecutedAt        *time.Time `json:"executed_at"`
	CancelledAt       *time.Time `json:"cancelled_at"`
	Eta               *time.Time `json:"eta"`
	TargetAddress     *string    `json:"target_address" gorm:"size:42"`
	FunctionSignature *string    `json:"function_signature" gorm:"size:200"`
	CallData          []byte     `json:"call_data" gorm:"type:bytea"`
	Value             string     `json:"value" gorm:"type:decimal(36,0);default:0"`
	Description       *string    `json:"description" gorm:"type:text"`
	CreatedAt         time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (TimelockTransactionFlow) TableName() string {
	return "timelock_transaction_flows"
}

// UserTimelockRelation 用户-合约关联模型
type UserTimelockRelation struct {
	ID               int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserAddress      string    `json:"user_address" gorm:"size:42;not null;index"`
	ChainID          int       `json:"chain_id" gorm:"not null;index"`
	ContractAddress  string    `json:"contract_address" gorm:"size:42;not null;index"`
	TimelockStandard string    `json:"timelock_standard" gorm:"size:20;not null"`
	RelationType     string    `json:"relation_type" gorm:"size:20;not null;index"`
	RelatedAt        time.Time `json:"related_at" gorm:"not null"`
	IsActive         bool      `json:"is_active" gorm:"not null;default:true;index"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (UserTimelockRelation) TableName() string {
	return "user_timelock_relations"
}

// CompoundTimelockEvent Compound Timelock 事件结构
type CompoundTimelockEvent struct {
	EventType       string                 `json:"event_type"`
	TxHash          string                 `json:"tx_hash"`
	BlockNumber     uint64                 `json:"block_number"`
	BlockTimestamp  uint64                 `json:"block_timestamp"`
	ContractAddress string                 `json:"contract_address"`
	ChainID         int                    `json:"chain_id"`
	ChainName       string                 `json:"chain_name"`
	FromAddress     string                 `json:"from_address"`
	ToAddress       string                 `json:"to_address"`
	EventData       map[string]interface{} `json:"event_data"`

	// QueueTransaction / ExecuteTransaction / CancelTransaction
	ProposalID        *string `json:"proposal_id,omitempty"`
	TargetAddress     *string `json:"target_address,omitempty"`
	Value             *string `json:"value,omitempty"`
	FunctionSignature *string `json:"function_signature,omitempty"`
	CallData          *string `json:"call_data,omitempty"`
	Eta               *uint64 `json:"eta,omitempty"`

	// NewDelay
	NewDelay *uint64 `json:"new_delay,omitempty"`

	// NewAdmin / NewPendingAdmin
	NewAdmin        *string `json:"new_admin,omitempty"`
	NewPendingAdmin *string `json:"new_pending_admin,omitempty"`
}

// 实现TimelockEvent接口
func (e *CompoundTimelockEvent) GetEventType() string {
	return e.EventType
}

func (e *CompoundTimelockEvent) GetContractAddress() string {
	return e.ContractAddress
}

func (e *CompoundTimelockEvent) GetTxHash() string {
	return e.TxHash
}

func (e *CompoundTimelockEvent) GetBlockNumber() uint64 {
	return e.BlockNumber
}

// OpenZeppelinTimelockEvent OpenZeppelin Timelock 事件结构
type OpenZeppelinTimelockEvent struct {
	EventType       string                 `json:"event_type"`
	TxHash          string                 `json:"tx_hash"`
	BlockNumber     uint64                 `json:"block_number"`
	BlockTimestamp  uint64                 `json:"block_timestamp"`
	ContractAddress string                 `json:"contract_address"`
	ChainID         int                    `json:"chain_id"`
	ChainName       string                 `json:"chain_name"`
	FromAddress     string                 `json:"from_address"`
	ToAddress       string                 `json:"to_address"`
	EventData       map[string]interface{} `json:"event_data"`

	// CallScheduled / CallExecuted / Cancelled
	OperationID       *string `json:"operation_id,omitempty"`
	Index             *uint64 `json:"index,omitempty"`
	TargetAddress     *string `json:"target_address,omitempty"`
	Value             *string `json:"value,omitempty"`
	CallData          *string `json:"call_data,omitempty"`
	FunctionSignature *string `json:"function_signature,omitempty"`
	Predecessor       *string `json:"predecessor,omitempty"`
	Delay             *uint64 `json:"delay,omitempty"`

	// MinDelayChange
	OldDuration *uint64 `json:"old_duration,omitempty"`
	NewDuration *uint64 `json:"new_duration,omitempty"`

	// RoleGranted / RoleRevoked
	Role    *string `json:"role,omitempty"`
	Account *string `json:"account,omitempty"`
	Sender  *string `json:"sender,omitempty"`
}

// 实现TimelockEvent接口
func (e *OpenZeppelinTimelockEvent) GetEventType() string {
	return e.EventType
}

func (e *OpenZeppelinTimelockEvent) GetContractAddress() string {
	return e.ContractAddress
}

func (e *OpenZeppelinTimelockEvent) GetTxHash() string {
	return e.TxHash
}

func (e *OpenZeppelinTimelockEvent) GetBlockNumber() uint64 {
	return e.BlockNumber
}

// CompoundTimelockInfo Compound Timelock 合约信息
type CompoundTimelockInfo struct {
	GRACE_PERIOD  *big.Int `json:"grace_period"`  // 宽限期
	MINIMUM_DELAY *big.Int `json:"minimum_delay"` // 最小延迟
	MAXIMUM_DELAY *big.Int `json:"maximum_delay"` // 最大延迟
	Admin         string   `json:"admin"`         // 当前管理员
	PendingAdmin  *string  `json:"pending_admin"` // 待定管理员
	Delay         *big.Int `json:"delay"`         // 当前延迟
}

// OpenZeppelinTimelockInfo OpenZeppelin Timelock 合约信息
type OpenZeppelinTimelockInfo struct {
	MinDelay  *big.Int `json:"min_delay"` // 最小延迟
	Proposers []string `json:"proposers"` // 提议者列表
	Executors []string `json:"executors"` // 执行者列表
	Admin     *string  `json:"admin"`     // 管理员 (可能为空)
}

// ImportTimelockContractRequest 导入合约请求
type ImportTimelockContractRequest struct {
	UserAddress     string `json:"user_address" binding:"required"`
	Standard        string `json:"standard" binding:"required,oneof=compound openzeppelin"`
	ContractAddress string `json:"contract_address" binding:"required"`
	ChainID         int    `json:"chain_id" binding:"required"`
	Remark          string `json:"remark" binding:"max=500"`
}

// TimelockContractInfo 合约信息详情
type TimelockContractInfo struct {
	// 基本信息
	ContractAddress string `json:"contract_address"`
	Standard        string `json:"standard"`
	ChainID         int    `json:"chain_id"`
	ChainName       string `json:"chain_name"`

	// 链上获取的信息
	MinDelay      *big.Int `json:"min_delay"`
	CreationBlock *uint64  `json:"creation_block,omitempty"`
	CreationTx    *string  `json:"creation_tx,omitempty"`

	// Compound 特有
	Admin        *string  `json:"admin,omitempty"`
	PendingAdmin *string  `json:"pending_admin,omitempty"`
	GracePeriod  *big.Int `json:"grace_period,omitempty"`
	MaxDelay     *big.Int `json:"max_delay,omitempty"`

	// OpenZeppelin 特有
	Proposers  []string `json:"proposers,omitempty"`
	Executors  []string `json:"executors,omitempty"`
	Cancellers []string `json:"cancellers,omitempty"`

	// 验证状态
	IsValid         bool    `json:"is_valid"`
	ValidationError *string `json:"validation_error,omitempty"`
}

// GetUserTimelockTransactionsRequest 获取用户相关的timelock交易请求
type GetUserTimelockTransactionsRequest struct {
	UserAddress string  `json:"user_address" binding:"required"`
	ChainID     *int    `json:"chain_id,omitempty"`
	Standard    *string `json:"standard,omitempty"`
	Status      *string `json:"status,omitempty"`
	Page        int     `json:"page" binding:"min=1"`
	PageSize    int     `json:"page_size" binding:"min=1,max=100"`
}

// GetUserTimelockTransactionsResponse 获取用户相关的timelock交易响应
type GetUserTimelockTransactionsResponse struct {
	Transactions []UserTimelockTransaction `json:"transactions"`
	Total        int64                     `json:"total"`
	Page         int                       `json:"page"`
	PageSize     int                       `json:"page_size"`
}

// UserTimelockTransaction 用户相关的timelock交易详情
type UserTimelockTransaction struct {
	// 基本交易信息
	TxHash         string    `json:"tx_hash"`
	BlockNumber    uint64    `json:"block_number"`
	BlockTimestamp time.Time `json:"block_timestamp"`
	ChainID        int       `json:"chain_id"`
	ChainName      string    `json:"chain_name"`

	// 合约信息
	ContractAddress string `json:"contract_address"`
	Standard        string `json:"standard"`

	// 用户角色和关系
	UserRole     string `json:"user_role"`     // creator, admin, proposer, executor, etc.
	UserRelation string `json:"user_relation"` // 用户与该交易的关系

	// 交易详情
	EventType     string  `json:"event_type"`
	FlowID        *string `json:"flow_id,omitempty"`
	FlowStatus    *string `json:"flow_status,omitempty"`
	TargetAddress *string `json:"target_address,omitempty"`
	FunctionSig   *string `json:"function_signature,omitempty"`
	Value         *string `json:"value,omitempty"`
	Description   *string `json:"description,omitempty"`

	// 时间信息
	ProposedAt   *time.Time `json:"proposed_at,omitempty"`
	ExecutableAt *time.Time `json:"executable_at,omitempty"`
	ExecutedAt   *time.Time `json:"executed_at,omitempty"`
	CancelledAt  *time.Time `json:"cancelled_at,omitempty"`
}

// RescanRequest 重扫请求
type RescanRequest struct {
	ChainID         int     `json:"chain_id" binding:"required"`
	FromBlock       uint64  `json:"from_block" binding:"required"`
	ToBlock         *uint64 `json:"to_block,omitempty"` // 空表示扫到最新
	ForceRescan     bool    `json:"force_rescan"`       // 是否强制重扫已扫描的区块
	CleanupExisting bool    `json:"cleanup_existing"`   // 是否清理现有数据
}

// RescanResponse 重扫响应
type RescanResponse struct {
	TaskID    string          `json:"task_id"`
	Status    string          `json:"status"`
	StartTime time.Time       `json:"start_time"`
	Progress  *RescanProgress `json:"progress,omitempty"`
}

// RescanProgress 重扫进度
type RescanProgress struct {
	CurrentBlock    uint64  `json:"current_block"`
	TargetBlock     uint64  `json:"target_block"`
	ProcessedBlocks uint64  `json:"processed_blocks"`
	FoundEvents     uint64  `json:"found_events"`
	ProcessedEvents uint64  `json:"processed_events"`
	ErrorCount      uint64  `json:"error_count"`
	ProgressPercent float64 `json:"progress_percent"`
}

// NotificationEvent 通知事件 (预留)
type NotificationEvent struct {
	EventType       string `json:"event_type"`
	UserAddress     string `json:"user_address"`
	ChainID         int    `json:"chain_id"`
	ContractAddress string `json:"contract_address"`

	// 事件详情
	FlowID      *string `json:"flow_id,omitempty"`
	TxHash      string  `json:"tx_hash"`
	BlockNumber uint64  `json:"block_number"`

	// 通知内容
	Title    string `json:"title"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // info, warning, critical

	// 时间信息
	OccurredAt     time.Time `json:"occurred_at"`
	NotificationAt time.Time `json:"notification_at"`
}

// ScannerStatus 扫链状态枚举
const (
	ScanStatusRunning = "running"
	ScanStatusPaused  = "paused"
	ScanStatusError   = "error"
)

// Event Type 事件类型枚举
const (
	// Compound Timelock Events
	EventQueueTransaction   = "QueueTransaction"
	EventExecuteTransaction = "ExecuteTransaction"
	EventCancelTransaction  = "CancelTransaction"
	EventNewDelay           = "NewDelay"
	EventNewAdmin           = "NewAdmin"
	EventNewPendingAdmin    = "NewPendingAdmin"

	// OpenZeppelin Timelock Events
	EventCallScheduled  = "CallScheduled"
	EventCallExecuted   = "CallExecuted"
	EventCancelled      = "Cancelled"
	EventMinDelayChange = "MinDelayChange"
	EventRoleGranted    = "RoleGranted"
	EventRoleRevoked    = "RoleRevoked"
)

// Flow Status 流程状态枚举
const (
	FlowStatusProposed  = "proposed"
	FlowStatusQueued    = "queued"
	FlowStatusExecuted  = "executed"
	FlowStatusCancelled = "cancelled"
	FlowStatusExpired   = "expired"
)

// Relation Type 关联类型枚举
const (
	RelationCreator      = "creator"
	RelationAdmin        = "admin"
	RelationPendingAdmin = "pending_admin"
	RelationProposer     = "proposer"
	RelationExecutor     = "executor"
	RelationCanceller    = "canceller"
)

// RPCProvider RPC提供商类型
type RPCProvider string

const (
	ProviderAlchemy  RPCProvider = "alchemy"
	ProviderInfura   RPCProvider = "infura"
	ProviderOfficial RPCProvider = "official"
)

// RPCHealth RPC健康状态
type RPCHealth struct {
	Provider     RPCProvider   `json:"provider"`
	URL          string        `json:"url"`
	IsHealthy    bool          `json:"is_healthy"`
	LastCheck    time.Time     `json:"last_check"`
	ResponseTime time.Duration `json:"response_time"`
	ErrorCount   int           `json:"error_count"`
	LastError    string        `json:"last_error,omitempty"`
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	ChainID      int           `json:"chain_id"`
	ChainName    string        `json:"chain_name"`
	Provider     RPCProvider   `json:"provider"`
	URL          string        `json:"url"`
	IsHealthy    bool          `json:"is_healthy"`
	ResponseTime time.Duration `json:"response_time"`
	BlockNumber  uint64        `json:"block_number,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
}
