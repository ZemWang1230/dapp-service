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

// TimeLock timelock合约模型
type TimeLock struct {
	ID              int64            `json:"id" gorm:"primaryKey;autoIncrement"`
	WalletAddress   string           `json:"wallet_address" gorm:"size:42;not null;index"`          // 钱包地址
	ChainID         int              `json:"chain_id" gorm:"not null;index"`                        // 所在链ID
	ChainName       string           `json:"chain_name" gorm:"size:50;not null;index"`              // 链名称（如eth-mainnet）
	ContractAddress string           `json:"contract_address" gorm:"size:42;not null;index"`        // 合约地址
	Standard        TimeLockStandard `json:"standard" gorm:"size:20;not null;index"`                // 合约标准
	CreatorAddress  *string          `json:"creator_address" gorm:"size:42"`                        // 创建者地址（创建时使用）
	TxHash          *string          `json:"tx_hash" gorm:"size:66"`                                // 创建交易hash（创建时使用）
	MinDelay        *int64           `json:"min_delay"`                                             // 最小延迟时间（秒）
	Proposers       *string          `json:"proposers" gorm:"type:text"`                            // 提议者地址列表（JSON格式）
	Executors       *string          `json:"executors" gorm:"type:text"`                            // 执行者地址列表（JSON格式）
	Admin           *string          `json:"admin" gorm:"size:42"`                                  // 管理员地址
	Remark          string           `json:"remark" gorm:"size:500"`                                // 备注
	Status          TimeLockStatus   `json:"status" gorm:"size:20;not null;default:'active';index"` // 状态
	CreatedAt       time.Time        `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time        `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (TimeLock) TableName() string {
	return "timelocks"
}

// CheckTimeLockStatusRequest 检查timelock状态请求
type CheckTimeLockStatusRequest struct {
	WalletAddress string `json:"wallet_address" binding:"required,len=42"`
}

// CheckTimeLockStatusResponse 检查timelock状态响应
type CheckTimeLockStatusResponse struct {
	HasTimeLocks bool       `json:"has_timelocks"`
	TimeLocks    []TimeLock `json:"timelocks,omitempty"`
}

// CreateTimeLockRequest 创建timelock合约请求
type CreateTimeLockRequest struct {
	ChainID         int              `json:"chain_id" binding:"required"`
	ChainName       string           `json:"chain_name" binding:"required"`
	ContractAddress string           `json:"contract_address" binding:"required,len=42"`
	Standard        TimeLockStandard `json:"standard" binding:"required,oneof=compound openzeppelin"`
	CreatorAddress  string           `json:"creator_address" binding:"required,len=42"`
	TxHash          string           `json:"tx_hash" binding:"required"`
	MinDelay        *int64           `json:"min_delay"` // Compound和Openzeppelin都有的参数
	Proposers       []string         `json:"proposers"` // Openzeppelin标准的参数
	Executors       []string         `json:"executors"` // Openzeppelin标准的参数
	Admin           *string          `json:"admin"`     // Compound标准的参数
	Remark          string           `json:"remark" binding:"max=500"`
}

// ImportTimeLockRequest 导入timelock合约请求
type ImportTimeLockRequest struct {
	ChainID         int              `json:"chain_id" binding:"required"`
	ChainName       string           `json:"chain_name" binding:"required"`
	ContractAddress string           `json:"contract_address" binding:"required,len=42"`
	Standard        TimeLockStandard `json:"standard" binding:"required,oneof=compound openzeppelin"`
	ABI             string           `json:"abi" binding:"required"` // 用于验证合约
	Remark          string           `json:"remark" binding:"max=500"`
}

// UpdateTimeLockRequest 更新timelock合约请求
type UpdateTimeLockRequest struct {
	ID     int64  `json:"id" binding:"required"`
	Remark string `json:"remark" binding:"max=500"`
}

// DeleteTimeLockRequest 删除timelock合约请求
type DeleteTimeLockRequest struct {
	ID int64 `json:"id" binding:"required"`
}

// GetTimeLockListRequest 获取timelock列表请求
type GetTimeLockListRequest struct {
	Page     int               `json:"page" form:"page" binding:"min=1"`
	PageSize int               `json:"page_size" form:"page_size" binding:"min=1,max=100"`
	ChainID  *int              `json:"chain_id" form:"chain_id"`
	Standard *TimeLockStandard `json:"standard" form:"standard"`
	Status   *TimeLockStatus   `json:"status" form:"status"`
}

// GetTimeLockListResponse 获取timelock列表响应
type GetTimeLockListResponse struct {
	List     []TimeLock `json:"list"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// TimeLockDetailResponse timelock详情响应
type TimeLockDetailResponse struct {
	TimeLock
	ProposersList []string `json:"proposers_list,omitempty"` // 解析后的提议者列表
	ExecutorsList []string `json:"executors_list,omitempty"` // 解析后的执行者列表
}
