package types

import (
	"time"
)

// User 用户模型 - 支持链切换功能
type User struct {
	ID            int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	WalletAddress string     `json:"wallet_address" gorm:"unique;size:42;not null"` // 钱包地址作为唯一标识
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	LastLogin     *time.Time `json:"last_login"`
	Status        int        `json:"status" gorm:"default:1"`
}

// TableName 设置表名
func (User) TableName() string {
	return "users"
}

// WalletConnectRequest 钱包连接请求
type WalletConnectRequest struct {
	WalletAddress string `json:"wallet_address" binding:"required,len=42"`
	Signature     string `json:"signature" binding:"required"`
	Message       string `json:"message" binding:"required"`
}

// WalletConnectResponse 钱包连接响应
type WalletConnectResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         User      `json:"user"`
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UserProfile 用户资料
type UserProfile struct {
	WalletAddress string     `json:"wallet_address"`
	CreatedAt     time.Time  `json:"created_at"`
	LastLogin     *time.Time `json:"last_login"`
}

// JWTClaims JWT声明
type JWTClaims struct {
	UserID        int64  `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
	Type          string `json:"type"` // access or refresh
}

// APIResponse 统一API响应格式
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError API错误格式
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ErrorResponse 简单错误响应格式
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
