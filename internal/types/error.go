package types

import "time"

// ErrorLog 错误日志模型
type ErrorLog struct {
	ID           int64                  `json:"id" gorm:"primaryKey;autoIncrement"`
	ErrorType    string                 `json:"error_type" gorm:"size:50;not null"`      // 错误类型
	ErrorMessage string                 `json:"error_message" gorm:"type:text;not null"` // 错误消息
	Context      map[string]interface{} `json:"context" gorm:"type:jsonb"`               // 上下文信息
	CreatedAt    time.Time              `json:"created_at" gorm:"autoCreateTime"`        // 创建时间
}

// TableName 设置表名
func (ErrorLog) TableName() string {
	return "error_logs"
}
