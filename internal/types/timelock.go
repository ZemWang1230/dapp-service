package types

import (
	"time"
)

// TimeLockStandard timelock合约标准类型
type TimeLockStandard string

const (
	CompoundStandard     TimeLockStandard = "compound"
	OpenzeppelinStandard TimeLockStandard = "openzeppelin"
)

// TimeLockStatus timelock合约状态
type TimeLockStatus string

const (
	TimeLockActive   TimeLockStatus = "active"   // 激活状态
	TimeLockInactive TimeLockStatus = "inactive" // 非激活状态
	TimeLockDeleted  TimeLockStatus = "deleted"  // 已删除
)

// CompoundTimeLock Compound标准timelock合约模型
type CompoundTimeLock struct {
	ID              int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatorAddress  string         `json:"creator_address" gorm:"size:42;not null;index"`         // 创建者/导入者地址
	ChainID         int            `json:"chain_id" gorm:"not null;index"`                        // 所在链ID
	ChainName       string         `json:"chain_name" gorm:"size:50;not null;index"`              // 链名称
	ContractAddress string         `json:"contract_address" gorm:"size:42;not null;index"`        // 合约地址
	TxHash          *string        `json:"tx_hash" gorm:"size:66"`                                // 创建交易hash（创建时）
	MinDelay        int64          `json:"min_delay" gorm:"not null"`                             // 最小延迟时间（秒）
	Admin           string         `json:"admin" gorm:"size:42;not null;index"`                   // 管理员地址
	PendingAdmin    *string        `json:"pending_admin" gorm:"size:42;index"`                    // 待定管理员地址
	Remark          string         `json:"remark" gorm:"size:500"`                                // 备注
	Status          TimeLockStatus `json:"status" gorm:"size:20;not null;default:'active';index"` // 状态
	IsImported      bool           `json:"is_imported" gorm:"not null;default:false"`             // 是否导入的合约
	CreatedAt       time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (CompoundTimeLock) TableName() string {
	return "compound_timelocks"
}

// OpenzeppelinTimeLock OpenZeppelin标准timelock合约模型
type OpenzeppelinTimeLock struct {
	ID              int64          `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatorAddress  string         `json:"creator_address" gorm:"size:42;not null;index"`         // 创建者/导入者地址
	ChainID         int            `json:"chain_id" gorm:"not null;index"`                        // 所在链ID
	ChainName       string         `json:"chain_name" gorm:"size:50;not null;index"`              // 链名称
	ContractAddress string         `json:"contract_address" gorm:"size:42;not null;index"`        // 合约地址
	TxHash          *string        `json:"tx_hash" gorm:"size:66"`                                // 创建交易hash（创建时）
	MinDelay        int64          `json:"min_delay" gorm:"not null"`                             // 最小延迟时间（秒）
	Proposers       string         `json:"proposers" gorm:"type:text;not null"`                   // 提议者地址列表（JSON）
	Executors       string         `json:"executors" gorm:"type:text;not null"`                   // 执行者地址列表（JSON）
	Cancellers      string         `json:"cancellers" gorm:"type:text;not null"`                  // 取消者地址列表（JSON）
	Remark          string         `json:"remark" gorm:"size:500"`                                // 备注
	Status          TimeLockStatus `json:"status" gorm:"size:20;not null;default:'active';index"` // 状态
	IsImported      bool           `json:"is_imported" gorm:"not null;default:false"`             // 是否导入的合约
	CreatedAt       time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (OpenzeppelinTimeLock) TableName() string {
	return "openzeppelin_timelocks"
}

// TimeLockPermission 用户在timelock合约中的权限
type TimeLockPermission struct {
	Standard   TimeLockStandard `json:"standard"`
	Permission string           `json:"permission"` // creator, admin, pending_admin, proposer, executor, canceller
}

// CreateTimeLockRequest 创建timelock合约请求
type CreateTimeLockRequest struct {
	ChainID         int              `json:"chain_id" binding:"required"`
	ChainName       string           `json:"chain_name" binding:"required"`
	ContractAddress string           `json:"contract_address" binding:"required,len=42"`
	Standard        TimeLockStandard `json:"standard" binding:"required,oneof=compound openzeppelin"`
	TxHash          string           `json:"tx_hash" binding:"required"`
	MinDelay        int64            `json:"min_delay" binding:"required,min=0"`

	// Compound标准参数
	Admin *string `json:"admin"` // Compound标准必需

	// OpenZeppelin标准参数
	Proposers  []string `json:"proposers"`  // OpenZeppelin标准必需
	Executors  []string `json:"executors"`  // OpenZeppelin标准必需
	Cancellers []string `json:"cancellers"` // OpenZeppelin标准必需

	Remark string `json:"remark" binding:"max=500"`
}

// ImportTimeLockRequest 导入timelock合约请求
type ImportTimeLockRequest struct {
	ChainID         int              `json:"chain_id" binding:"required"`
	ChainName       string           `json:"chain_name" binding:"required"`
	ContractAddress string           `json:"contract_address" binding:"required,len=42"`
	Standard        TimeLockStandard `json:"standard" binding:"required,oneof=compound openzeppelin"`

	// 合约当时创建的参数（从区块链读取）
	MinDelay int64 `json:"min_delay" binding:"required,min=0"`

	// Compound标准参数
	Admin        *string `json:"admin"`
	PendingAdmin *string `json:"pending_admin"`

	// OpenZeppelin标准参数
	Proposers  []string `json:"proposers"`
	Executors  []string `json:"executors"`
	Cancellers []string `json:"cancellers"`

	Remark string `json:"remark" binding:"max=500"`
}

// UpdateTimeLockRequest 更新timelock合约请求
type UpdateTimeLockRequest struct {
	ID       int64            `json:"id" binding:"required"`
	Standard TimeLockStandard `json:"standard" binding:"required,oneof=compound openzeppelin"`
	Remark   string           `json:"remark" binding:"max=500"`
}

// DeleteTimeLockRequest 删除timelock合约请求
type DeleteTimeLockRequest struct {
	ID       int64            `json:"id" binding:"required"`
	Standard TimeLockStandard `json:"standard" binding:"required,oneof=compound openzeppelin"`
}

// GetTimeLockListRequest 获取timelock列表请求
type GetTimeLockListRequest struct {
	Standard *TimeLockStandard `json:"standard" form:"standard"`
	Status   *TimeLockStatus   `json:"status" form:"status"`
}

// CompoundTimeLockWithPermission Compound timelock with permission info
type CompoundTimeLockWithPermission struct {
	CompoundTimeLock
	UserPermissions    []string `json:"user_permissions"` // creator, admin, pending_admin
	CanSetPendingAdmin bool     `json:"can_set_pending_admin"`
	CanAcceptAdmin     bool     `json:"can_accept_admin"`
}

// OpenzeppelinTimeLockWithPermission OpenZeppelin timelock with permission info
type OpenzeppelinTimeLockWithPermission struct {
	OpenzeppelinTimeLock
	UserPermissions []string `json:"user_permissions"` // creator, proposer, executor, canceller
	ProposersList   []string `json:"proposers_list"`
	ExecutorsList   []string `json:"executors_list"`
	CancellersList  []string `json:"cancellers_list"`
}

// GetTimeLockListResponse 获取timelock列表响应
type GetTimeLockListResponse struct {
	CompoundTimeLocks     []CompoundTimeLockWithPermission     `json:"compound_timelocks"`
	OpenzeppelinTimeLocks []OpenzeppelinTimeLockWithPermission `json:"openzeppelin_timelocks"`
	Total                 int64                                `json:"total"`
}

// TimeLockDetailResponse timelock详情响应
type TimeLockDetailResponse struct {
	Standard         TimeLockStandard                    `json:"standard"`
	CompoundData     *CompoundTimeLockWithPermission     `json:"compound_data,omitempty"`
	OpenzeppelinData *OpenzeppelinTimeLockWithPermission `json:"openzeppelin_data,omitempty"`
}

// SetPendingAdminRequest 设置pending admin请求
type SetPendingAdminRequest struct {
	ID              int64  `json:"id" binding:"required"`
	NewPendingAdmin string `json:"new_pending_admin" binding:"required,len=42"`
}

// AcceptAdminRequest 接受admin请求
type AcceptAdminRequest struct {
	ID int64 `json:"id" binding:"required"`
}

// CheckAdminPermissionRequest 检查admin权限请求
type CheckAdminPermissionRequest struct {
	ID int64 `json:"id" binding:"required"`
}

// CheckAdminPermissionResponse 检查admin权限响应
type CheckAdminPermissionResponse struct {
	CanSetPendingAdmin bool `json:"can_set_pending_admin"`
	CanAcceptAdmin     bool `json:"can_accept_admin"`
}
