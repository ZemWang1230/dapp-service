package types

import (
	"time"
)

// Email 邮箱主表模型
type Email struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Email         string    `json:"email" gorm:"unique;size:200;not null"`
	IsDeliverable bool      `json:"is_deliverable" gorm:"not null;default:true"`
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (Email) TableName() string {
	return "emails"
}

// UserEmail 用户与邮箱的关系表模型
type UserEmail struct {
	ID             int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int64      `json:"user_id" gorm:"not null"`
	EmailID        int64      `json:"email_id" gorm:"not null"`
	Remark         *string    `json:"remark" gorm:"size:200"`
	IsVerified     bool       `json:"is_verified" gorm:"not null;default:false"`
	LastVerifiedAt *time.Time `json:"last_verified_at"`
	CreatedAt      time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	// 关联
	Email *Email `json:"email,omitempty" gorm:"foreignKey:EmailID"`
	User  *User  `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName 设置表名
func (UserEmail) TableName() string {
	return "user_emails"
}

// EmailVerificationCode 邮箱验证码模型
type EmailVerificationCode struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserEmailID  int64     `json:"user_email_id" gorm:"not null"`
	Code         string    `json:"code" gorm:"size:16;not null"`
	ExpiresAt    time.Time `json:"expires_at" gorm:"not null"`
	SentAt       time.Time `json:"sent_at" gorm:"not null;autoCreateTime"`
	AttemptCount int       `json:"attempt_count" gorm:"not null;default:0"`
	IsUsed       bool      `json:"is_used" gorm:"not null;default:false"`

	// 关联
	UserEmail *UserEmail `json:"user_email,omitempty" gorm:"foreignKey:UserEmailID"`
}

// TableName 设置表名
func (EmailVerificationCode) TableName() string {
	return "email_verification_codes"
}

// UserEmailSubscription 用户邮箱订阅模型
type UserEmailSubscription struct {
	ID               int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserEmailID      int64     `json:"user_email_id" gorm:"not null"`
	TimelockStandard string    `json:"timelock_standard" gorm:"size:20;not null;check:timelock_standard IN ('compound','openzeppelin')"`
	ChainID          int       `json:"chain_id" gorm:"not null"`
	ContractAddress  string    `json:"contract_address" gorm:"size:42;not null"`
	NotifyOn         []string  `json:"notify_on" gorm:"type:jsonb;not null;default:'[]';serializer:json"`
	IsActive         bool      `json:"is_active" gorm:"not null;default:true"`
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// 关联
	UserEmail *UserEmail `json:"user_email,omitempty" gorm:"foreignKey:UserEmailID"`
}

// TableName 设置表名
func (UserEmailSubscription) TableName() string {
	return "user_email_subscriptions"
}

// EmailSendLog 邮件发送日志模型
type EmailSendLog struct {
	ID               int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	EmailID          int64     `json:"email_id" gorm:"not null"`
	FlowID           string    `json:"flow_id" gorm:"size:128;not null"`
	TimelockStandard string    `json:"timelock_standard" gorm:"size:20;not null"`
	ChainID          int       `json:"chain_id" gorm:"not null"`
	ContractAddress  string    `json:"contract_address" gorm:"size:42;not null"`
	StatusFrom       *string   `json:"status_from" gorm:"size:20"`
	StatusTo         string    `json:"status_to" gorm:"size:20;not null"`
	TxHash           *string   `json:"tx_hash" gorm:"size:66"`
	SendStatus       string    `json:"send_status" gorm:"size:20;not null;check:send_status IN ('success','failed')"`
	ErrorMessage     *string   `json:"error_message" gorm:"type:text"`
	RetryCount       int       `json:"retry_count" gorm:"not null;default:0"`
	SentAt           time.Time `json:"sent_at" gorm:"not null;autoCreateTime"`

	// 关联
	Email *Email `json:"email,omitempty" gorm:"foreignKey:EmailID"`
}

// TableName 设置表名
func (EmailSendLog) TableName() string {
	return "email_send_logs"
}

// ===== 请求响应结构体 =====

// AddEmailRequest 添加邮箱请求
type AddEmailRequest struct {
	Email  string  `json:"email" binding:"required,email"`
	Remark *string `json:"remark"`
}

// AddEmailResponse 添加邮箱响应
type AddEmailResponse struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

// UpdateEmailRemarkRequest 更新邮箱备注请求
type UpdateEmailRemarkRequest struct {
	Remark *string `json:"remark"`
}

// SendVerificationCodeRequest 发送验证码请求
type SendVerificationCodeRequest struct {
	UserEmailID int64 `json:"user_email_id" binding:"required"`
}

// VerifyEmailRequest 验证邮箱请求
type VerifyEmailRequest struct {
	UserEmailID int64  `json:"user_email_id" binding:"required"`
	Code        string `json:"code" binding:"required"`
}

// UserEmailResponse 用户邮箱响应
type UserEmailResponse struct {
	ID             int64      `json:"id"`
	Email          string     `json:"email"`
	Remark         *string    `json:"remark"`
	IsVerified     bool       `json:"is_verified"`
	LastVerifiedAt *time.Time `json:"last_verified_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CreateSubscriptionRequest 创建订阅请求
type CreateSubscriptionRequest struct {
	UserEmailID      int64    `json:"user_email_id" binding:"required"`
	TimelockStandard string   `json:"timelock_standard" binding:"required,oneof=compound openzeppelin"`
	ChainID          int      `json:"chain_id" binding:"required"`
	ContractAddress  string   `json:"contract_address" binding:"required,len=42"`
	NotifyOn         []string `json:"notify_on" binding:"required"`
}

// UpdateSubscriptionRequest 更新订阅请求
type UpdateSubscriptionRequest struct {
	NotifyOn []string `json:"notify_on" binding:"required"`
	IsActive *bool    `json:"is_active"`
}

// SubscriptionResponse 订阅响应
type SubscriptionResponse struct {
	ID               int64     `json:"id"`
	UserEmailID      int64     `json:"user_email_id"`
	Email            string    `json:"email"`
	TimelockStandard string    `json:"timelock_standard"`
	ChainID          int       `json:"chain_id"`
	ChainName        string    `json:"chain_name,omitempty"`
	ContractAddress  string    `json:"contract_address"`
	NotifyOn         []string  `json:"notify_on"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// EmailListResponse 邮箱列表响应
type EmailListResponse struct {
	Emails []UserEmailResponse `json:"emails"`
	Total  int64               `json:"total"`
}

// SubscriptionListResponse 订阅列表响应
type SubscriptionListResponse struct {
	Subscriptions []SubscriptionResponse `json:"subscriptions"`
	Total         int64                  `json:"total"`
}

// NotificationStatus 通知状态枚举
var NotificationStatus = struct {
	Waiting   string
	Ready     string
	Executed  string
	Cancelled string
	Expired   string
}{
	Waiting:   "waiting",
	Ready:     "ready",
	Executed:  "executed",
	Cancelled: "cancelled",
	Expired:   "expired",
}

// ValidNotificationStatuses 有效的通知状态列表
var ValidNotificationStatuses = []string{
	NotificationStatus.Waiting,
	NotificationStatus.Ready,
	NotificationStatus.Executed,
	NotificationStatus.Cancelled,
	NotificationStatus.Expired,
}

// TimelockStandard Timelock标准枚举
var TimelockStandard = struct {
	Compound     string
	OpenZeppelin string
}{
	Compound:     "compound",
	OpenZeppelin: "openzeppelin",
}

// ValidTimelockStandards 有效的Timelock标准列表
var ValidTimelockStandards = []string{
	TimelockStandard.Compound,
	TimelockStandard.OpenZeppelin,
}
