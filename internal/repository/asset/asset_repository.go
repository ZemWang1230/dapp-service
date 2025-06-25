package asset

import (
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 资产存储库接口
type Repository interface {
	GetUserAssets(walletAddress string) ([]*types.UserAsset, error)
	BatchUpsertUserAssets(assets []*types.UserAsset) error
	ClearUserAssets(walletAddress string) error
	GetUserAssetsByChain(walletAddress string, chainName string) ([]*types.UserAsset, error)
}

// repository 资产存储库实现
type repository struct {
	db *gorm.DB
}

// NewRepository 创建新的资产存储库
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// GetUserAssets 获取用户所有资产，按USD价值降序排列
func (r *repository) GetUserAssets(walletAddress string) ([]*types.UserAsset, error) {
	var assets []*types.UserAsset

	err := r.db.Where("wallet_address = ?", walletAddress).
		Order("usd_value DESC").
		Find(&assets).Error

	if err != nil {
		logger.Error("GetUserAssets Error: ", err, "wallet_address", walletAddress)
		return nil, err
	}

	logger.Info("GetUserAssets: ", "wallet_address", walletAddress, "count", len(assets))
	return assets, nil
}

// GetUserAssetsByChain 获取用户特定链上的资产
func (r *repository) GetUserAssetsByChain(walletAddress string, chainName string) ([]*types.UserAsset, error) {
	var assets []*types.UserAsset

	err := r.db.Where("wallet_address = ? AND chain_name = ?", walletAddress, chainName).
		Order("usd_value DESC").
		Find(&assets).Error

	if err != nil {
		logger.Error("GetUserAssetsByChain Error: ", err, "wallet_address", walletAddress, "chain_name", chainName)
		return nil, err
	}

	logger.Info("GetUserAssetsByChain: ", "wallet_address", walletAddress, "chain_name", chainName, "count", len(assets))
	return assets, nil
}

// BatchUpsertUserAssets 批量插入或更新用户资产
func (r *repository) BatchUpsertUserAssets(assets []*types.UserAsset) error {
	if len(assets) == 0 {
		return nil
	}

	// 使用ON CONFLICT进行UPSERT操作
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, asset := range assets {
			err := tx.Where("wallet_address = ? AND chain_name = ? AND contract_address = ?",
				asset.WalletAddress, asset.ChainName, asset.ContractAddress).
				Assign(asset).
				FirstOrCreate(asset).Error
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		logger.Error("BatchUpsertUserAssets Error: ", err, "count", len(assets))
		return err
	}

	logger.Info("BatchUpsertUserAssets: ", "count", len(assets))
	return nil
}

// ClearUserAssets 清空用户资产（用于完全刷新）
func (r *repository) ClearUserAssets(walletAddress string) error {
	err := r.db.Where("wallet_address = ?", walletAddress).Delete(&types.UserAsset{}).Error
	if err != nil {
		logger.Error("ClearUserAssets Error: ", err, "wallet_address", walletAddress)
		return err
	}

	logger.Info("ClearUserAssets: ", "wallet_address", walletAddress)
	return nil
}
