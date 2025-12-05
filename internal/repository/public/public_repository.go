package public

import (
	"context"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 公共数据仓库接口
type Repository interface {
	GetChainCount(ctx context.Context) (int64, error)
	GetContractCount(ctx context.Context) (int64, error)
	GetTransactionCount(ctx context.Context) (int64, error)
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

// GetChainCount 获取支持的链数量
func (r *repository) GetChainCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&struct{ ID int64 }{}).Table("support_chains").Count(&count).Error
	if err != nil {
		logger.Error("GetChainCount Error: ", err)
		return 0, err
	}

	logger.Info("GetChainCount: ", "count", count)
	return count, nil
}

// GetContractCount 获取timelock合约数量（去重）
func (r *repository) GetContractCount(ctx context.Context) (int64, error) {
	var count int64
	// 从timelock_transaction_flows表中统计去重后的合约数量（根据chain_id和contract_address）
	err := r.db.WithContext(ctx).Table("timelock_transaction_flows").
		Select("COUNT(DISTINCT CONCAT(chain_id, '-', contract_address)) as count").
		Scan(&count).Error
	if err != nil {
		logger.Error("GetContractCount Error: ", err)
		return 0, err
	}

	logger.Info("GetContractCount: ", "count", count)
	return count, nil
}

// GetTransactionCount 获取交易数量
func (r *repository) GetTransactionCount(ctx context.Context) (int64, error) {
	var compoundCount int64
	var openzeppelinCount int64

	// 统计compound_timelock_transactions表的记录数
	err := r.db.WithContext(ctx).Model(&struct{ ID int64 }{}).Table("compound_timelock_transactions").Count(&compoundCount).Error
	if err != nil {
		logger.Error("GetTransactionCount: failed to count compound transactions", err)
		return 0, err
	}

	// 统计openzeppelin_timelock_transactions表的记录数
	err = r.db.WithContext(ctx).Model(&struct{ ID int64 }{}).Table("openzeppelin_timelock_transactions").Count(&openzeppelinCount).Error
	if err != nil {
		logger.Error("GetTransactionCount: failed to count openzeppelin transactions", err)
		return 0, err
	}

	totalCount := compoundCount + openzeppelinCount
	logger.Info("GetTransactionCount: ", "compound", compoundCount, "openzeppelin", openzeppelinCount, "total", totalCount)
	return totalCount, nil
}
