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
	GetUserByID(ctx context.Context, id int64) (*types.User, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdateChainID(ctx context.Context, id int64, chainID int) error
	UpdateUser(ctx context.Context, user *types.User) error
	DeleteUser(ctx context.Context, id int64) error
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

// UpdateChainID 更新用户链ID
func (r *repository) UpdateChainID(ctx context.Context, id int64, chainID int) error {
	logger.Info("UpdateChainID: ", "user_id: ", id, "chain_id: ", chainID)
	return r.db.WithContext(ctx).
		Model(&types.User{}).
		Where("id = ?", id).
		Update("chain_id", chainID).Error
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
