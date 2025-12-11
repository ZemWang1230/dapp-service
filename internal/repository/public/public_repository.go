package public

import (
	"context"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 公共数据仓库接口
type Repository interface {
	GetChainCount(ctx context.Context) (int64, error)
	GetTotalContractCount(ctx context.Context) (int64, error)
	GetTotalTransactionCount(ctx context.Context) (int64, error)
	UpdateChainStatistics(ctx context.Context, chainID int, chainName string, contractCount, transactionCount int64) error
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

// GetTotalContractCount 获取所有链的总合同数量
func (r *repository) GetTotalContractCount(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&struct{ ContractCount int64 }{}).Table("chain_statistics").Select("COALESCE(SUM(contract_count), 0)").Scan(&total).Error
	if err != nil {
		logger.Error("GetTotalContractCount Error: ", err)
		return 0, err
	}

	logger.Info("GetTotalContractCount: ", "total", total)
	return total, nil
}

// GetTotalTransactionCount 获取所有链的总交易数量
func (r *repository) GetTotalTransactionCount(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&struct{ TransactionCount int64 }{}).Table("chain_statistics").Select("COALESCE(SUM(transaction_count), 0)").Scan(&total).Error
	if err != nil {
		logger.Error("GetTotalTransactionCount Error: ", err)
		return 0, err
	}

	logger.Info("GetTotalTransactionCount: ", "total", total)
	return total, nil
}

// UpdateChainStatistics 更新指定链的统计数据
func (r *repository) UpdateChainStatistics(ctx context.Context, chainID int, chainName string, contractCount, transactionCount int64) error {
	sql := `
		INSERT INTO chain_statistics (chain_id, chain_name, contract_count, transaction_count, updated_at)
		VALUES (?, ?, ?, ?, NOW())
		ON CONFLICT (chain_id)
		DO UPDATE SET
			chain_name = EXCLUDED.chain_name,
			contract_count = EXCLUDED.contract_count,
			transaction_count = EXCLUDED.transaction_count,
			updated_at = NOW()
	`

	err := r.db.WithContext(ctx).Exec(sql, chainID, chainName, contractCount, transactionCount).Error
	if err != nil {
		logger.Error("UpdateChainStatistics Error: ", err, "chain_id", chainID)
		return err
	}

	logger.Info("UpdateChainStatistics: ", "chain_id", chainID, "chain_name", chainName, "contracts", contractCount, "transactions", transactionCount)
	return nil
}
