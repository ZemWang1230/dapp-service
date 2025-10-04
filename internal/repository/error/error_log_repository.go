package error

import (
	"context"
	"fmt"
	"timelocker-backend/internal/types"

	"gorm.io/gorm"
)

// ErrorLogRepository 错误日志仓储接口
type ErrorLogRepository interface {
	// CreateErrorLog 创建错误日志
	CreateErrorLog(ctx context.Context, errorLog *types.ErrorLog) error

	// BatchCreateErrorLogs 批量创建错误日志
	BatchCreateErrorLogs(ctx context.Context, errorLogs []types.ErrorLog) error

	// GetErrorLogsByChainID 根据链ID获取错误日志
	GetErrorLogsByChainID(ctx context.Context, chainID int, limit, offset int) ([]types.ErrorLog, int64, error)

	// GetErrorLogsByType 根据错误类型获取错误日志
	GetErrorLogsByType(ctx context.Context, errorType string, limit, offset int) ([]types.ErrorLog, int64, error)

	// GetRecentErrorLogs 获取最近的错误日志
	GetRecentErrorLogs(ctx context.Context, limit int) ([]types.ErrorLog, error)

	// DeleteOldErrorLogs 删除旧的错误日志（清理任务）
	DeleteOldErrorLogs(ctx context.Context, daysOld int) error
}

// errorLogRepository 错误日志仓储实现
type errorLogRepository struct {
	db *gorm.DB
}

// NewErrorLogRepository 创建错误日志仓储
func NewErrorLogRepository(db *gorm.DB) ErrorLogRepository {
	return &errorLogRepository{
		db: db,
	}
}

// CreateErrorLog 创建错误日志
func (r *errorLogRepository) CreateErrorLog(ctx context.Context, errorLog *types.ErrorLog) error {
	if err := r.db.WithContext(ctx).Create(errorLog).Error; err != nil {
		return fmt.Errorf("failed to create error log: %w", err)
	}
	return nil
}

// BatchCreateErrorLogs 批量创建错误日志
func (r *errorLogRepository) BatchCreateErrorLogs(ctx context.Context, errorLogs []types.ErrorLog) error {
	if len(errorLogs) == 0 {
		return nil
	}

	if err := r.db.WithContext(ctx).CreateInBatches(errorLogs, 100).Error; err != nil {
		return fmt.Errorf("failed to batch create error logs: %w", err)
	}
	return nil
}

// GetErrorLogsByChainID 根据链ID获取错误日志
func (r *errorLogRepository) GetErrorLogsByChainID(ctx context.Context, chainID int, limit, offset int) ([]types.ErrorLog, int64, error) {
	var errorLogs []types.ErrorLog
	var total int64

	// 计算总数
	if err := r.db.WithContext(ctx).Model(&types.ErrorLog{}).
		Where("chain_id = ?", chainID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count error logs by chain_id: %w", err)
	}

	// 获取数据
	if err := r.db.WithContext(ctx).
		Where("chain_id = ?", chainID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&errorLogs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get error logs by chain_id: %w", err)
	}

	return errorLogs, total, nil
}

// GetErrorLogsByType 根据错误类型获取错误日志
func (r *errorLogRepository) GetErrorLogsByType(ctx context.Context, errorType string, limit, offset int) ([]types.ErrorLog, int64, error) {
	var errorLogs []types.ErrorLog
	var total int64

	// 计算总数
	if err := r.db.WithContext(ctx).Model(&types.ErrorLog{}).
		Where("error_type = ?", errorType).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count error logs by type: %w", err)
	}

	// 获取数据
	if err := r.db.WithContext(ctx).
		Where("error_type = ?", errorType).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&errorLogs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get error logs by type: %w", err)
	}

	return errorLogs, total, nil
}

// GetRecentErrorLogs 获取最近的错误日志
func (r *errorLogRepository) GetRecentErrorLogs(ctx context.Context, limit int) ([]types.ErrorLog, error) {
	var errorLogs []types.ErrorLog

	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&errorLogs).Error; err != nil {
		return nil, fmt.Errorf("failed to get recent error logs: %w", err)
	}

	return errorLogs, nil
}

// DeleteOldErrorLogs 删除旧的错误日志（清理任务）
func (r *errorLogRepository) DeleteOldErrorLogs(ctx context.Context, daysOld int) error {
	result := r.db.WithContext(ctx).
		Where("created_at < NOW() - INTERVAL ? DAY", daysOld).
		Delete(&types.ErrorLog{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete old error logs: %w", result.Error)
	}

	return nil
}
