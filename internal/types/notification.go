package types

import (
	"time"
)

// NotificationChannel 通知通道类型
type NotificationChannel string

const (
	ChannelTelegram NotificationChannel = "telegram"
	ChannelLark     NotificationChannel = "lark"
	ChannelFeishu   NotificationChannel = "feishu"
)

// TelegramConfig Telegram通知配置
type TelegramConfig struct {
	ID          uint      `json:"id" gorm:"primaryKey"`                       // ID
	UserAddress string    `json:"user_address" gorm:"not null;index;size:42"` // 用户地址
	Name        string    `json:"name" gorm:"size:100"`                       // 名称
	BotToken    string    `json:"bot_token" gorm:"not null;size:500"`         // 机器人token
	ChatID      string    `json:"chat_id" gorm:"not null;size:100"`           // 聊天ID
	IsActive    bool      `json:"is_active" gorm:"default:true"`              // 是否激活
	CreatedAt   time.Time `json:"created_at"`                                 // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`                                 // 更新时间
}

func (TelegramConfig) TableName() string {
	return "telegram_configs"
}

// LarkConfig Lark通知配置
type LarkConfig struct {
	ID          uint      `json:"id" gorm:"primaryKey"`                       // ID
	UserAddress string    `json:"user_address" gorm:"not null;index;size:42"` // 用户地址
	Name        string    `json:"name" gorm:"size:100"`                       // 名称
	WebhookURL  string    `json:"webhook_url" gorm:"not null;size:1000"`      // 网络钩子URL
	Secret      string    `json:"secret" gorm:"size:500"`                     // 签名验证时的密钥
	IsActive    bool      `json:"is_active" gorm:"default:true"`              // 是否激活
	CreatedAt   time.Time `json:"created_at"`                                 // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`                                 // 更新时间
}

func (LarkConfig) TableName() string {
	return "lark_configs"
}

// FeishuConfig Feishu通知配置
type FeishuConfig struct {
	ID          uint      `json:"id" gorm:"primaryKey"`                       // ID
	UserAddress string    `json:"user_address" gorm:"not null;index;size:42"` // 用户地址
	Name        string    `json:"name" gorm:"size:100"`                       // 名称
	WebhookURL  string    `json:"webhook_url" gorm:"not null;size:1000"`      // 网络钩子URL
	Secret      string    `json:"secret" gorm:"size:500"`                     // 签名验证时的密钥
	IsActive    bool      `json:"is_active" gorm:"default:true"`              // 是否激活
	CreatedAt   time.Time `json:"created_at"`                                 // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`                                 // 更新时间
}

func (FeishuConfig) TableName() string {
	return "feishu_configs"
}

// NotificationLog 通知发送日志
type NotificationLog struct {
	ID               uint                `json:"id" gorm:"primaryKey"`
	UserAddress      string              `json:"user_address" gorm:"not null;index;size:42"` // 用户地址
	Channel          NotificationChannel `json:"channel" gorm:"not null;size:20"`            // 通道
	ConfigID         uint                `json:"config_id" gorm:"not null"`                  // 配置ID
	FlowID           string              `json:"flow_id" gorm:"not null;index;size:128"`     // 流程ID
	TimelockStandard string              `json:"timelock_standard" gorm:"not null;size:20"`  // 时间锁标准
	ChainID          int                 `json:"chain_id" gorm:"not null"`                   // 链ID
	ContractAddress  string              `json:"contract_address" gorm:"not null;size:42"`   // 合约地址
	StatusFrom       string              `json:"status_from" gorm:"size:20"`                 // 状态从
	StatusTo         string              `json:"status_to" gorm:"not null;size:20"`          // 状态到
	TxHash           string              `json:"tx_hash" gorm:"size:66"`                     // 交易哈希
	SendStatus       string              `json:"send_status" gorm:"not null;size:20"`        // 发送状态
	ErrorMessage     string              `json:"error_message" gorm:"type:text"`             // 错误消息
	SentAt           time.Time           `json:"sent_at"`                                    // 发送时间
}

func (NotificationLog) TableName() string {
	return "notification_logs"
}

// CreateTelegramConfigRequest Telegram配置创建请求
type CreateTelegramConfigRequest struct {
	Name     string `json:"name" binding:"required"`      // 名称
	BotToken string `json:"bot_token" binding:"required"` // 机器人token
	ChatID   string `json:"chat_id" binding:"required"`   // 聊天ID
}

// UpdateTelegramConfigRequest Telegram配置更新请求
type UpdateTelegramConfigRequest struct {
	Name     *string `json:"name"`      // 名称
	BotToken *string `json:"bot_token"` // 机器人token
	ChatID   *string `json:"chat_id"`   // 聊天ID
	IsActive *bool   `json:"is_active"` // 是否激活
}

// DeleteTelegramConfigRequest Telegram配置删除请求
type DeleteTelegramConfigRequest struct {
	Name string `json:"name" binding:"required"` // 名称
}

// CreateLarkConfigRequest Lark配置创建请求
type CreateLarkConfigRequest struct {
	Name       string `json:"name" binding:"required"`        // 名称
	WebhookURL string `json:"webhook_url" binding:"required"` // 网络钩子URL
	Secret     string `json:"secret"`                         // 签名验证时的密钥
}

// UpdateLarkConfigRequest Lark配置更新请求
type UpdateLarkConfigRequest struct {
	Name       *string `json:"name"`        // 名称
	WebhookURL *string `json:"webhook_url"` // 网络钩子URL
	Secret     *string `json:"secret"`      // 签名验证时的密钥
	IsActive   *bool   `json:"is_active"`   // 是否激活
}

// DeleteLarkConfigRequest Lark配置删除请求
type DeleteLarkConfigRequest struct {
	Name string `json:"name" binding:"required"` // 名称
}

// CreateFeishuConfigRequest Feishu配置创建请求
type CreateFeishuConfigRequest struct {
	Name       string `json:"name" binding:"required"`        // 名称
	WebhookURL string `json:"webhook_url" binding:"required"` // 网络钩子URL
	Secret     string `json:"secret"`                         // 签名验证时的密钥
}

// UpdateFeishuConfigRequest Feishu配置更新请求
type UpdateFeishuConfigRequest struct {
	Name       *string `json:"name"`        // 名称
	WebhookURL *string `json:"webhook_url"` // 网络钩子URL
	Secret     *string `json:"secret"`      // 签名验证时的密钥
	IsActive   *bool   `json:"is_active"`   // 是否激活
}

// DeleteFeishuConfigRequest Feishu配置删除请求
type DeleteFeishuConfigRequest struct {
	Name string `json:"name" binding:"required"` // 名称
}

// UserNotificationConfigs 用户通知配置集合
type UserNotificationConfigs struct {
	TelegramConfigs []*TelegramConfig `json:"telegram_configs"`
	LarkConfigs     []*LarkConfig     `json:"lark_configs"`
	FeishuConfigs   []*FeishuConfig   `json:"feishu_configs"`
}

// NotificationConfigListResponse 通知配置列表响应
type NotificationConfigListResponse struct {
	TelegramConfigs []*TelegramConfig `json:"telegram_configs"`
	LarkConfigs     []*LarkConfig     `json:"lark_configs"`
	FeishuConfigs   []*FeishuConfig   `json:"feishu_configs"`
}
