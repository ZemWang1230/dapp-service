package timelock

import (
	"context"
	"errors"

	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository timelock仓库接口
type Repository interface {
	// 基础CRUD操作
	CreateTimeLock(ctx context.Context, timeLock *types.TimeLock) error
	GetTimeLockByID(ctx context.Context, id int64) (*types.TimeLock, error)
	UpdateTimeLock(ctx context.Context, timeLock *types.TimeLock) error
	DeleteTimeLock(ctx context.Context, id int64) error

	// 查询操作
	GetTimeLocksByWallet(ctx context.Context, walletAddress string) ([]types.TimeLock, error)
	CheckTimeLockExists(ctx context.Context, walletAddress string, chainID int, contractAddress string) (bool, error)
	GetTimeLockList(ctx context.Context, walletAddress string, req *types.GetTimeLockListRequest) ([]types.TimeLock, int64, error)

	// 状态管理
	UpdateTimeLockStatus(ctx context.Context, id int64, status types.TimeLockStatus) error
	UpdateTimeLockRemark(ctx context.Context, id int64, remark string) error

	// 验证操作
	ValidateOwnership(ctx context.Context, id int64, walletAddress string) (bool, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository 创建timelock仓库实例
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// CreateTimeLock 创建timelock合约记录
func (r *repository) CreateTimeLock(ctx context.Context, timeLock *types.TimeLock) error {
	if err := r.db.WithContext(ctx).Create(timeLock).Error; err != nil {
		logger.Error("CreateTimeLock Error: ", err, "wallet_address", timeLock.WalletAddress, "contract_address", timeLock.ContractAddress)
		return err
	}

	logger.Info("CreateTimeLock: ", "timelock_id", timeLock.ID, "wallet_address", timeLock.WalletAddress, "contract_address", timeLock.ContractAddress, "standard", timeLock.Standard)
	return nil
}

// GetTimeLockByID 根据ID获取timelock合约
func (r *repository) GetTimeLockByID(ctx context.Context, id int64) (*types.TimeLock, error) {
	var timeLock types.TimeLock
	err := r.db.WithContext(ctx).
		Where("id = ? AND status != ?", id, types.TimeLockDeleted).
		First(&timeLock).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("GetTimeLockByID: timelock not found", "timelock_id", id)
			return nil, err
		}
		logger.Error("GetTimeLockByID Error: ", err, "timelock_id", id)
		return nil, err
	}

	logger.Info("GetTimeLockByID: ", "timelock_id", timeLock.ID, "wallet_address", timeLock.WalletAddress)
	return &timeLock, nil
}

// UpdateTimeLock 更新timelock合约信息
func (r *repository) UpdateTimeLock(ctx context.Context, timeLock *types.TimeLock) error {
	if err := r.db.WithContext(ctx).
		Model(timeLock).
		Where("id = ?", timeLock.ID).
		Updates(timeLock).Error; err != nil {
		logger.Error("UpdateTimeLock Error: ", err, "timelock_id", timeLock.ID)
		return err
	}

	logger.Info("UpdateTimeLock: ", "timelock_id", timeLock.ID, "wallet_address", timeLock.WalletAddress)
	return nil
}

// DeleteTimeLock 删除timelock合约（软删除）
func (r *repository) DeleteTimeLock(ctx context.Context, id int64) error {
	if err := r.db.WithContext(ctx).
		Model(&types.TimeLock{}).
		Where("id = ?", id).
		Update("status", types.TimeLockDeleted).Error; err != nil {
		logger.Error("DeleteTimeLock Error: ", err, "timelock_id", id)
		return err
	}

	logger.Info("DeleteTimeLock: ", "timelock_id", id)
	return nil
}

// GetTimeLocksByWallet 获取钱包地址下的所有timelock合约
func (r *repository) GetTimeLocksByWallet(ctx context.Context, walletAddress string) ([]types.TimeLock, error) {
	var timeLocks []types.TimeLock
	err := r.db.WithContext(ctx).
		Where("wallet_address = ? AND status != ?", walletAddress, types.TimeLockDeleted).
		Order("created_at DESC").
		Find(&timeLocks).Error

	if err != nil {
		logger.Error("GetTimeLocksByWallet Error: ", err, "wallet_address", walletAddress)
		return nil, err
	}

	logger.Info("GetTimeLocksByWallet: ", "wallet_address", walletAddress, "count", len(timeLocks))
	return timeLocks, nil
}

// CheckTimeLockExists 检查timelock合约是否已存在
func (r *repository) CheckTimeLockExists(ctx context.Context, walletAddress string, chainID int, contractAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.TimeLock{}).
		Where("wallet_address = ? AND chain_id = ? AND contract_address = ? AND status != ?",
			walletAddress, chainID, contractAddress, types.TimeLockDeleted).
		Count(&count).Error

	if err != nil {
		logger.Error("CheckTimeLockExists Error: ", err, "wallet_address", walletAddress, "chain_id", chainID, "contract_address", contractAddress)
		return false, err
	}

	exists := count > 0
	logger.Info("CheckTimeLockExists: ", "wallet_address", walletAddress, "chain_id", chainID, "contract_address", contractAddress, "exists", exists)
	return exists, nil
}

// GetTimeLockList 获取timelock列表（支持分页和筛选）
func (r *repository) GetTimeLockList(ctx context.Context, walletAddress string, req *types.GetTimeLockListRequest) ([]types.TimeLock, int64, error) {
	query := r.db.WithContext(ctx).
		Where("wallet_address = ? AND status != ?", walletAddress, types.TimeLockDeleted)

	// 添加筛选条件
	if req.ChainID != nil {
		query = query.Where("chain_id = ?", *req.ChainID)
	}
	if req.Standard != nil {
		query = query.Where("standard = ?", *req.Standard)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}

	// 获取总数
	var total int64
	if err := query.Model(&types.TimeLock{}).Count(&total).Error; err != nil {
		logger.Error("GetTimeLockList Count Error: ", err, "wallet_address", walletAddress)
		return nil, 0, err
	}

	// 分页查询
	var timeLocks []types.TimeLock
	offset := (req.Page - 1) * req.PageSize
	if err := query.
		Order("created_at DESC").
		Limit(req.PageSize).
		Offset(offset).
		Find(&timeLocks).Error; err != nil {
		logger.Error("GetTimeLockList Error: ", err, "wallet_address", walletAddress)
		return nil, 0, err
	}

	logger.Info("GetTimeLockList: ", "wallet_address", walletAddress, "total", total, "page", req.Page, "page_size", req.PageSize)
	return timeLocks, total, nil
}

// UpdateTimeLockStatus 更新timelock状态
func (r *repository) UpdateTimeLockStatus(ctx context.Context, id int64, status types.TimeLockStatus) error {
	if err := r.db.WithContext(ctx).
		Model(&types.TimeLock{}).
		Where("id = ?", id).
		Update("status", status).Error; err != nil {
		logger.Error("UpdateTimeLockStatus Error: ", err, "timelock_id", id, "status", status)
		return err
	}

	logger.Info("UpdateTimeLockStatus: ", "timelock_id", id, "status", status)
	return nil
}

// UpdateTimeLockRemark 更新timelock备注
func (r *repository) UpdateTimeLockRemark(ctx context.Context, id int64, remark string) error {
	if err := r.db.WithContext(ctx).
		Model(&types.TimeLock{}).
		Where("id = ?", id).
		Update("remark", remark).Error; err != nil {
		logger.Error("UpdateTimeLockRemark Error: ", err, "timelock_id", id)
		return err
	}

	logger.Info("UpdateTimeLockRemark: ", "timelock_id", id, "remark_length", len(remark))
	return nil
}

// ValidateOwnership 验证timelock合约的所有权
func (r *repository) ValidateOwnership(ctx context.Context, id int64, walletAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.TimeLock{}).
		Where("id = ? AND wallet_address = ? AND status != ?", id, walletAddress, types.TimeLockDeleted).
		Count(&count).Error

	if err != nil {
		logger.Error("ValidateOwnership Error: ", err, "timelock_id", id, "wallet_address", walletAddress)
		return false, err
	}

	isOwner := count > 0
	logger.Info("ValidateOwnership: ", "timelock_id", id, "wallet_address", walletAddress, "is_owner", isOwner)
	return isOwner, nil
}
