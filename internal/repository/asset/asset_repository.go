package asset

import (
	"errors"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 用户资产仓库接口
type Repository interface {
	GetUserAssets(walletAddress string) ([]*types.UserAsset, error)
	GetUserAssetsByChain(walletAddress string, chainID int64) ([]*types.UserAsset, error)
	GetUserAsset(walletAddress string, chainID, tokenID int64) (*types.UserAsset, error)
	CreateOrUpdateUserAsset(asset *types.UserAsset) error
	BatchCreateOrUpdateUserAssets(assets []*types.UserAsset) error
	DeleteUserAsset(id int64) error
	GetUserAssetsForUpdate(walletAddress string, chainID int64) ([]*types.UserAsset, error)
}

// repository 用户资产仓库实现
type repository struct {
	db *gorm.DB
}

// NewRepository 创建新的用户资产仓库
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// GetUserAssets 获取用户所有资产
func (r *repository) GetUserAssets(walletAddress string) ([]*types.UserAsset, error) {
	var assets []*types.UserAsset

	err := r.db.Preload("Token").
		Where("wallet_address = ?", walletAddress).
		Order("chain_id, token_id").
		Find(&assets).Error
	if err != nil {
		logger.Error("GetUserAssets Error: ", err, "wallet_address", walletAddress)
		return nil, err
	}

	logger.Info("GetUserAssets: ", "wallet_address", walletAddress, "count", len(assets))
	return assets, nil
}

// GetUserAssetsByChain 获取用户在指定链上的资产
func (r *repository) GetUserAssetsByChain(walletAddress string, chainID int64) ([]*types.UserAsset, error) {
	var assets []*types.UserAsset

	err := r.db.Preload("Token").
		Where("wallet_address = ? AND chain_id = ?", walletAddress, chainID).
		Order("token_id").
		Find(&assets).Error
	if err != nil {
		logger.Error("GetUserAssetsByChain Error: ", err, "wallet_address", walletAddress, "chain_id", chainID)
		return nil, err
	}

	logger.Info("GetUserAssetsByChain: ", "wallet_address", walletAddress, "chain_id", chainID, "count", len(assets))
	return assets, nil
}

// GetUserAsset 获取用户指定资产
func (r *repository) GetUserAsset(walletAddress string, chainID, tokenID int64) (*types.UserAsset, error) {
	var asset types.UserAsset

	err := r.db.Preload("Token").
		Where("wallet_address = ? AND chain_id = ? AND token_id = ?", walletAddress, chainID, tokenID).
		First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("GetUserAsset: not found", "wallet_address", walletAddress, "chain_id", chainID, "token_id", tokenID)
			return nil, nil
		}
		logger.Error("GetUserAsset Error: ", err, "wallet_address", walletAddress, "chain_id", chainID, "token_id", tokenID)
		return nil, err
	}

	logger.Info("GetUserAsset: ", "wallet_address", walletAddress, "chain_id", chainID, "token_id", tokenID, "found", asset.ID)
	return &asset, nil
}

// CreateOrUpdateUserAsset 创建或更新用户资产
func (r *repository) CreateOrUpdateUserAsset(asset *types.UserAsset) error {
	// 使用 UPSERT 操作，基于复合唯一键更新
	err := r.db.Save(asset).Error
	if err != nil {
		logger.Error("CreateOrUpdateUserAsset Error: ", err, "wallet_address", asset.WalletAddress, "chain_id", asset.ChainID, "token_id", asset.TokenID)
		return err
	}

	logger.Info("CreateOrUpdateUserAsset: ", "wallet_address", asset.WalletAddress, "chain_id", asset.ChainID, "token_id", asset.TokenID, "balance", asset.Balance)
	return nil
}

// BatchCreateOrUpdateUserAssets 批量创建或更新用户资产
func (r *repository) BatchCreateOrUpdateUserAssets(assets []*types.UserAsset) error {
	if len(assets) == 0 {
		return nil
	}

	// 开启事务
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, asset := range assets {
		if err := tx.Save(asset).Error; err != nil {
			tx.Rollback()
			logger.Error("BatchCreateOrUpdateUserAssets Error: ", err, "wallet_address", asset.WalletAddress, "chain_id", asset.ChainID, "token_id", asset.TokenID)
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		logger.Error("BatchCreateOrUpdateUserAssets Commit Error: ", err)
		return err
	}

	logger.Info("BatchCreateOrUpdateUserAssets: ", "assets_count", len(assets))
	return nil
}

// DeleteUserAsset 删除用户资产
func (r *repository) DeleteUserAsset(id int64) error {
	err := r.db.Delete(&types.UserAsset{}, id).Error
	if err != nil {
		logger.Error("DeleteUserAsset Error: ", err, "id", id)
		return err
	}

	logger.Info("DeleteUserAsset: ", "id", id)
	return nil
}

// GetUserAssetsForUpdate 获取用户资产并加锁（用于更新）
func (r *repository) GetUserAssetsForUpdate(walletAddress string, chainID int64) ([]*types.UserAsset, error) {
	var assets []*types.UserAsset

	err := r.db.Set("gorm:query_option", "FOR UPDATE").
		Preload("Token").
		Where("wallet_address = ? AND chain_id = ?", walletAddress, chainID).
		Order("token_id").
		Find(&assets).Error
	if err != nil {
		logger.Error("GetUserAssetsForUpdate Error: ", err, "wallet_address", walletAddress, "chain_id", chainID)
		return nil, err
	}

	logger.Info("GetUserAssetsForUpdate: ", "wallet_address", walletAddress, "chain_id", chainID, "count", len(assets))
	return assets, nil
}
