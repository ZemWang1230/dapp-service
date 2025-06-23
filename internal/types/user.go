package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
	"timelocker-backend/pkg/logger"
)

// JSONB 自定义类型用于处理PostgreSQL的JSONB字段
type JSONB map[string]interface{}

// Value 实现 driver.Valuer 接口，用于将Go值转换为数据库值
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口，用于将数据库值转换为Go值
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONB)
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		logger.Error("Scan JSONB Error: ", errors.New("cannot scan non-string/[]byte value into JSONB"))
		return errors.New("cannot scan non-string/[]byte value into JSONB")
	}

	if len(bytes) == 0 {
		*j = make(JSONB)
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// User 用户模型
type User struct {
	ID            int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	WalletAddress string     `json:"wallet_address" gorm:"uniqueIndex:idx_users_wallet_address;size:42;not null"`
	ChainID       int        `json:"chain_id" gorm:"not null;default:1"`
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	LastLogin     *time.Time `json:"last_login"`
	Preferences   JSONB      `json:"preferences" gorm:"type:jsonb;default:'{}'"`
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
	ChainId       int    `json:"chain_id" binding:"required"`
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

// SwitchChainRequest 切换链请求
type SwitchChainRequest struct {
	ChainID int `json:"chain_id" binding:"required"`
}

// SwitchChainResponse 切换链响应
type SwitchChainResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         User      `json:"user"`
	Message      string    `json:"message"`
}

// UserProfile 用户资料
type UserProfile struct {
	WalletAddress string     `json:"wallet_address"`
	ChainID       int        `json:"chain_id"`
	CreatedAt     time.Time  `json:"created_at"`
	LastLogin     *time.Time `json:"last_login"`
	Preferences   JSONB      `json:"preferences"`
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
