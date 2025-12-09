package public

import (
	"context"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 公共数据仓库接口
type Repository interface {
	GetChainCount(ctx context.Context) (int64, error)
}

// repository 公共数据仓库实现
type repository struct {
	db *gorm.DB
}

// NewRepository 创建新的公共数据仓库
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// GetChainCount 获取支持的链数量（只统计active的）
func (r *repository) GetChainCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&struct{ ID int64 }{}).Table("support_chains").Where("is_active = ?", true).Count(&count).Error
	if err != nil {
		logger.Error("GetChainCount Error: ", err)
		return 0, err
	}

	logger.Info("GetChainCount: ", "count", count)
	return count, nil
}
