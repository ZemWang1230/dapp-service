package types

import (
	"time"
)

// EmailNotification 邮件通知配置模型
type EmailNotification struct {
	ID                    int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	WalletAddress         string     `json:"wallet_address" gorm:"size:42;not null"`
	Email                 string     `json:"email" gorm:"size:255;not null"`
	EmailRemark           string     `json:"email_remark" gorm:"size:200"`
	TimelockContracts     string     `json:"timelock_contracts" gorm:"type:text;not null;default:'[]'"` // JSON格式的地址列表
	IsVerified            bool       `json:"is_verified" gorm:"not null;default:false"`
	VerificationCode      *string    `json:"verification_code,omitempty" gorm:"size:6"`
	VerificationExpiresAt *time.Time `json:"verification_expires_at,omitempty"`
	IsActive              bool       `json:"is_active" gorm:"not null;default:true"`
	CreatedAt             time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (EmailNotification) TableName() string {
	return "email_notifications"
}

// EmailSendLog 邮件发送记录模型
type EmailSendLog struct {
	ID                  int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	EmailNotificationID int64      `json:"email_notification_id" gorm:"not null"`
	Email               string     `json:"email" gorm:"size:255;not null"`
	TimelockAddress     string     `json:"timelock_address" gorm:"size:42;not null"`
	TransactionHash     *string    `json:"transaction_hash" gorm:"size:66"`
	EventType           string     `json:"event_type" gorm:"size:50;not null"`
	Subject             string     `json:"subject" gorm:"size:500;not null"`
	Content             string     `json:"content" gorm:"type:text;not null"`
	IsEmergency         bool       `json:"is_emergency" gorm:"not null;default:false"`
	EmergencyReplyToken *string    `json:"emergency_reply_token,omitempty" gorm:"size:64"`
	IsReplied           bool       `json:"is_replied" gorm:"not null;default:false"`
	RepliedAt           *time.Time `json:"replied_at"`
	SendStatus          string     `json:"send_status" gorm:"size:20;not null;default:'pending'"`
	SendAttempts        int        `json:"send_attempts" gorm:"not null;default:0"`
	ErrorMessage        *string    `json:"error_message" gorm:"type:text"`
	SentAt              *time.Time `json:"sent_at"`
	CreatedAt           time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (EmailSendLog) TableName() string {
	return "email_send_logs"
}

// EmergencyNotification 应急通知追踪模型 - 简化版本
type EmergencyNotification struct {
	ID              int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	TimelockAddress string     `json:"timelock_address" gorm:"size:42;not null"`
	TransactionHash string     `json:"transaction_hash" gorm:"size:66;not null"`
	EventType       string     `json:"event_type" gorm:"size:50;not null"`         // 事件类型：proposal_created等
	RepliedEmails   int        `json:"replied_emails" gorm:"not null;default:0"`   // 已回复邮箱数量
	IsCompleted     bool       `json:"is_completed" gorm:"not null;default:false"` // 是否完成（至少一个邮箱回复）
	NextSendAt      *time.Time `json:"next_send_at"`                               // 下次发送时间
	SendCount       int        `json:"send_count" gorm:"not null;default:1"`       // 发送次数
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 设置表名
func (EmergencyNotification) TableName() string {
	return "emergency_notifications"
}

// 邮件通知API请求响应结构体

// AddEmailNotificationRequest 添加邮件通知请求
type AddEmailNotificationRequest struct {
	Email             string   `json:"email" binding:"required,email"`
	EmailRemark       string   `json:"email_remark"`
	TimelockContracts []string `json:"timelock_contracts"` // 可选，可以为空数组
}

// VerifyEmailRequest 验证邮箱请求
type VerifyEmailRequest struct {
	Email            string `json:"email" binding:"required,email"`
	VerificationCode string `json:"verification_code" binding:"required,len=6"`
}

// UpdateEmailNotificationRequest 更新邮件通知请求
type UpdateEmailNotificationRequest struct {
	EmailRemark       string   `json:"email_remark"`
	TimelockContracts []string `json:"timelock_contracts"`
}

// ResendVerificationRequest 重发验证码请求
type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// EmailNotificationResponse 邮件通知响应
type EmailNotificationResponse struct {
	ID                int64     `json:"id"`
	Email             string    `json:"email"`
	EmailRemark       string    `json:"email_remark"`
	TimelockContracts []string  `json:"timelock_contracts"`
	IsVerified        bool      `json:"is_verified"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// EmailNotificationListResponse 邮件通知列表响应
type EmailNotificationListResponse struct {
	Items      []EmailNotificationResponse `json:"items"`
	Total      int                         `json:"total"`
	Page       int                         `json:"page"`
	PageSize   int                         `json:"page_size"`
	TotalPages int                         `json:"total_pages"`
}

// EmailSendLogResponse 邮件发送记录响应
type EmailSendLogResponse struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	TimelockAddress string     `json:"timelock_address"`
	TransactionHash *string    `json:"transaction_hash"`
	EventType       string     `json:"event_type"`
	Subject         string     `json:"subject"`
	IsEmergency     bool       `json:"is_emergency"`
	IsReplied       bool       `json:"is_replied"`
	RepliedAt       *time.Time `json:"replied_at"`
	SendStatus      string     `json:"send_status"`
	SentAt          *time.Time `json:"sent_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

// EventType 事件类型常量
const (
	EventTypeProposalCreated  = "proposal_created"
	EventTypeProposalCanceled = "proposal_canceled"
	EventTypeReadyToExecute   = "ready_to_execute"
	EventTypeExecuted         = "executed"
	EventTypeExpired          = "expired"
)

// SendStatus 发送状态常量
const (
	SendStatusPending = "pending"
	SendStatusSent    = "sent"
	SendStatusFailed  = "failed"
)

// EmergencyReplyRequest 应急邮件回复请求
type EmergencyReplyRequest struct {
	Token string `json:"token" binding:"required"`
}

// EmergencyReplyResponse 应急邮件回复响应
type EmergencyReplyResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	RepliedAt time.Time `json:"replied_at"`
}
