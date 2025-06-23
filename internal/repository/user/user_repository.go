package user

import (
	"context"
	"time"

	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

type Repository interface {
	CreateUser(ctx context.Context, user *types.User) error
	GetUserByWallet(ctx context.Context, walletAddress string) (*types.User, error)
	GetUserByWalletAndChain(ctx context.Context, walletAddress string, chainID int) (*types.User, error)
	GetUserByID(ctx context.Context, id int64) (*types.User, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdateUser(ctx context.Context, user *types.User) error
	DeleteUser(ctx context.Context, id int64) error
	GetByWalletAddress(walletAddress string) (*types.User, error)
	GetByWalletAndChain(walletAddress string, chainID int) (*types.User, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// CreateUser 创建新用户
func (r *repository) CreateUser(ctx context.Context, user *types.User) error {
	logger.Info("CreateUser: ", "user_id: ", user.ID, "wallet_address: ", user.WalletAddress)
	return r.db.WithContext(ctx).Create(user).Error
}

// GetUserByWallet 根据钱包地址获取用户
func (r *repository) GetUserByWallet(ctx context.Context, walletAddress string) (*types.User, error) {
	var user types.User
	err := r.db.WithContext(ctx).
		Where("wallet_address = ?", walletAddress).
		First(&user).Error

	if err != nil {
		logger.Error("GetUserByWallet Error: ", err)
		return nil, err
	}
	logger.Info("GetUserByWallet: ", "user_id: ", user.ID, "wallet_address: ", user.WalletAddress)
	return &user, nil
}

// GetUserByID 根据ID获取用户
func (r *repository) GetUserByID(ctx context.Context, id int64) (*types.User, error) {
	var user types.User
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&user).Error

	if err != nil {
		logger.Error("GetUserByID Error: ", err)
		return nil, err
	}
	logger.Info("GetUserByID: ", "user_id: ", user.ID, "wallet_address: ", user.WalletAddress)
	return &user, nil
}

// UpdateLastLogin 更新用户最后登录时间
func (r *repository) UpdateLastLogin(ctx context.Context, id int64) error {
	now := time.Now()
	logger.Info("UpdateLastLogin: ", "user_id: ", id, "last_login: ", now)
	return r.db.WithContext(ctx).
		Model(&types.User{}).
		Where("id = ?", id).
		Update("last_login", &now).Error
}

// UpdateUser 更新用户信息
func (r *repository) UpdateUser(ctx context.Context, user *types.User) error {
	logger.Info("UpdateUser: ", "user_id: ", user.ID, "wallet_address: ", user.WalletAddress)
	return r.db.WithContext(ctx).
		Model(user).
		Where("id = ?", user.ID).
		Updates(user).Error
}

// DeleteUser 删除用户（软删除）
func (r *repository) DeleteUser(ctx context.Context, id int64) error {
	logger.Info("DeleteUser: ", "user_id: ", id)
	return r.db.WithContext(ctx).
		Model(&types.User{}).
		Where("id = ?", id).
		Update("status", 0).Error
}

// GetByWalletAddress 根据钱包地址获取用户（简化版本，不需要context）
func (r *repository) GetByWalletAddress(walletAddress string) (*types.User, error) {
	var user types.User
	err := r.db.Where("wallet_address = ?", walletAddress).
		First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Info("GetByWalletAddress: user not found", "wallet_address", walletAddress)
			return nil, nil
		}
		logger.Error("GetByWalletAddress Error: ", err, "wallet_address", walletAddress)
		return nil, err
	}

	logger.Info("GetByWalletAddress: ", "user_id", user.ID, "wallet_address", user.WalletAddress)
	return &user, nil
}

// GetUserByWalletAndChain 根据钱包地址和链ID获取用户
func (r *repository) GetUserByWalletAndChain(ctx context.Context, walletAddress string, chainID int) (*types.User, error) {
	var user types.User
	err := r.db.WithContext(ctx).
		Where("wallet_address = ? AND chain_id = ?", walletAddress, chainID).
		First(&user).Error

	if err != nil {
		logger.Error("GetUserByWalletAndChain Error: ", err)
		return nil, err
	}
	logger.Info("GetUserByWalletAndChain: ", "user_id: ", user.ID, "wallet_address: ", user.WalletAddress, "chain_id: ", user.ChainID)
	return &user, nil
}

// GetByWalletAndChain 根据钱包地址和链ID获取用户（简化版本，不需要context）
func (r *repository) GetByWalletAndChain(walletAddress string, chainID int) (*types.User, error) {
	var user types.User
	err := r.db.Where("wallet_address = ? AND chain_id = ?", walletAddress, chainID).
		First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Info("GetByWalletAndChain: user not found", "wallet_address", walletAddress, "chain_id", chainID)
			return nil, nil
		}
		logger.Error("GetByWalletAndChain Error: ", err, "wallet_address", walletAddress, "chain_id", chainID)
		return nil, err
	}

	logger.Info("GetByWalletAndChain: ", "user_id", user.ID, "wallet_address", user.WalletAddress, "chain_id", user.ChainID)
	return &user, nil
}
