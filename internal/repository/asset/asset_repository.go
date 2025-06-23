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
	BatchUpsertUserAssets(assets []*types.UserAsset) error
	DeleteUserAsset(id int64) error
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

// BatchUpsertUserAssets 批量UPSERT用户资产（支持PostgreSQL ON CONFLICT语法）
func (r *repository) BatchUpsertUserAssets(assets []*types.UserAsset) error {
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
		// 使用 PostgreSQL 的 ON CONFLICT 语法实现 UPSERT
		sql := `
			INSERT INTO user_assets (user_id, wallet_address, chain_id, token_id, balance, balance_wei, usd_value, last_updated, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NOW())
			ON CONFLICT (user_id, chain_id, token_id)
			DO UPDATE SET
				balance = EXCLUDED.balance,
				balance_wei = EXCLUDED.balance_wei,
				usd_value = EXCLUDED.usd_value,
				last_updated = NOW(),
				updated_at = NOW()
		`

		if err := tx.Exec(sql,
			asset.UserID,
			asset.WalletAddress,
			asset.ChainID,
			asset.TokenID,
			asset.Balance,
			asset.BalanceWei,
			asset.USDValue,
		).Error; err != nil {
			tx.Rollback()
			logger.Error("BatchUpsertUserAssets Error: ", err, "wallet_address", asset.WalletAddress, "chain_id", asset.ChainID, "token_id", asset.TokenID)
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		logger.Error("BatchUpsertUserAssets Commit Error: ", err)
		return err
	}

	logger.Info("BatchUpsertUserAssets: ", "assets_count", len(assets))
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
