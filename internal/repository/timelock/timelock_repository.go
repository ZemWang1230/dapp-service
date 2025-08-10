package timelock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository timelock仓库接口
type Repository interface {
	// Compound Timelock操作
	CreateCompoundTimeLock(ctx context.Context, timeLock *types.CompoundTimeLock) error
	GetCompoundTimeLockByChainAndAddress(ctx context.Context, chainID int, contractAddress string) (*types.CompoundTimeLock, error)
	UpdateCompoundTimeLock(ctx context.Context, timeLock *types.CompoundTimeLock) error
	DeleteCompoundTimeLock(ctx context.Context, chainID int, contractAddress string) error
	UpdateCompoundTimeLockRemark(ctx context.Context, chainID int, contractAddress string, remark string) error

	// OpenZeppelin Timelock操作
	CreateOpenzeppelinTimeLock(ctx context.Context, timeLock *types.OpenzeppelinTimeLock) error
	GetOpenzeppelinTimeLockByChainAndAddress(ctx context.Context, chainID int, contractAddress string) (*types.OpenzeppelinTimeLock, error)
	UpdateOpenzeppelinTimeLock(ctx context.Context, timeLock *types.OpenzeppelinTimeLock) error
	DeleteOpenzeppelinTimeLock(ctx context.Context, chainID int, contractAddress string) error
	UpdateOpenzeppelinTimeLockRemark(ctx context.Context, chainID int, contractAddress string, remark string) error

	// 查询操作
	CheckCompoundTimeLockExists(ctx context.Context, chainID int, contractAddress string) (bool, error)
	CheckOpenzeppelinTimeLockExists(ctx context.Context, chainID int, contractAddress string) (bool, error)

	// 权限相关查询
	GetTimeLocksByUserPermissions(ctx context.Context, userAddress string, req *types.GetTimeLockListRequest) ([]types.CompoundTimeLockWithPermission, []types.OpenzeppelinTimeLockWithPermission, int64, error)

	// 验证操作
	ValidateCompoundOwnership(ctx context.Context, chainID int, contractAddress string, userAddress string) (bool, error)
	ValidateOpenzeppelinOwnership(ctx context.Context, chainID int, contractAddress string, userAddress string) (bool, error)

	// 获取用户相关的timelock合约（用于权限刷新）
	GetAllCompoundTimeLocksByUser(ctx context.Context, userAddress string) ([]types.CompoundTimeLock, error)
	GetAllOpenzeppelinTimeLocksByUser(ctx context.Context, userAddress string) ([]types.OpenzeppelinTimeLock, error)

	// 获取所有活跃timelock合约（用于定时刷新）
	GetAllActiveCompoundTimeLocks(ctx context.Context) ([]types.CompoundTimeLock, error)
	GetAllActiveOpenzeppelinTimeLocks(ctx context.Context) ([]types.OpenzeppelinTimeLock, error)
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

// CreateCompoundTimeLock 创建compound timelock合约记录
func (r *repository) CreateCompoundTimeLock(ctx context.Context, timeLock *types.CompoundTimeLock) error {
	if err := r.db.WithContext(ctx).Create(timeLock).Error; err != nil {
		logger.Error("CreateCompoundTimeLock error", err, "creator_address", timeLock.CreatorAddress, "contract_address", timeLock.ContractAddress)
		return err
	}

	logger.Info("CreateCompoundTimeLock success", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress, "contract_address", timeLock.ContractAddress)
	return nil
}

// GetCompoundTimeLockByChainAndAddress 根据链ID和合约地址获取compound timelock合约
func (r *repository) GetCompoundTimeLockByChainAndAddress(ctx context.Context, chainID int, contractAddress string) (*types.CompoundTimeLock, error) {
	var timeLock types.CompoundTimeLock
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ? AND status != ?", chainID, contractAddress, "deleted").
		First(&timeLock).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("timelock not found")
		}
		logger.Error("GetCompoundTimeLockByChainAndAddress error", err, "chain_id", chainID, "contract_address", contractAddress)
		return nil, err
	}

	logger.Info("GetCompoundTimeLockByChainAndAddress success", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return &timeLock, nil
}

// UpdateCompoundTimeLock 更新compound timelock合约信息
func (r *repository) UpdateCompoundTimeLock(ctx context.Context, timeLock *types.CompoundTimeLock) error {
	if err := r.db.WithContext(ctx).Save(timeLock).Error; err != nil {
		logger.Error("UpdateCompoundTimeLock error", err, "timelock_id", timeLock.ID)
		return err
	}

	logger.Info("UpdateCompoundTimeLock success", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return nil
}

// DeleteCompoundTimeLock 硬删除 compound timelock 合约（仅删除合约行，不清理其他表）
func (r *repository) DeleteCompoundTimeLock(ctx context.Context, chainID int, contractAddress string) error {
	if err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ?", chainID, contractAddress).
		Delete(&types.CompoundTimeLock{}).Error; err != nil {
		logger.Error("DeleteCompoundTimeLock error", err, "chain_id", chainID, "contract_address", contractAddress)
		return err
	}
	logger.Info("DeleteCompoundTimeLock success", "chain_id", chainID, "contract_address", contractAddress)
	return nil
}

// UpdateCompoundTimeLockRemark 更新compound timelock备注
func (r *repository) UpdateCompoundTimeLockRemark(ctx context.Context, chainID int, contractAddress string, remark string) error {
	if err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where("chain_id = ? AND contract_address = ?", chainID, contractAddress).
		Update("remark", remark).Error; err != nil {
		logger.Error("UpdateCompoundTimeLockRemark error", err, "chain_id", chainID, "contract_address", contractAddress)
		return err
	}

	logger.Info("UpdateCompoundTimeLockRemark success", "chain_id", chainID, "contract_address", contractAddress, "remark_length", len(remark))
	return nil
}

// CreateOpenzeppelinTimeLock 创建openzeppelin timelock合约记录
func (r *repository) CreateOpenzeppelinTimeLock(ctx context.Context, timeLock *types.OpenzeppelinTimeLock) error {
	if err := r.db.WithContext(ctx).Create(timeLock).Error; err != nil {
		logger.Error("CreateOpenzeppelinTimeLock error", err, "creator_address", timeLock.CreatorAddress, "contract_address", timeLock.ContractAddress)
		return err
	}

	logger.Info("CreateOpenzeppelinTimeLock success", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress, "contract_address", timeLock.ContractAddress)
	return nil
}

// GetOpenzeppelinTimeLockByChainAndAddress 根据链ID和合约地址获取openzeppelin timelock合约
func (r *repository) GetOpenzeppelinTimeLockByChainAndAddress(ctx context.Context, chainID int, contractAddress string) (*types.OpenzeppelinTimeLock, error) {
	var timeLock types.OpenzeppelinTimeLock
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ? AND status != ?", chainID, contractAddress, "deleted").
		First(&timeLock).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("timelock not found")
		}
		logger.Error("GetOpenzeppelinTimeLockByChainAndAddress error", err, "chain_id", chainID, "contract_address", contractAddress)
		return nil, err
	}

	logger.Info("GetOpenzeppelinTimeLockByChainAndAddress success", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return &timeLock, nil
}

// UpdateOpenzeppelinTimeLock 更新openzeppelin timelock合约信息
func (r *repository) UpdateOpenzeppelinTimeLock(ctx context.Context, timeLock *types.OpenzeppelinTimeLock) error {
	if err := r.db.WithContext(ctx).Save(timeLock).Error; err != nil {
		logger.Error("UpdateOpenzeppelinTimeLock error", err, "timelock_id", timeLock.ID)
		return err
	}

	logger.Info("UpdateOpenzeppelinTimeLock success", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return nil
}

// DeleteOpenzeppelinTimeLock 硬删除 openzeppelin timelock 合约（仅删除合约行，不清理其他表）
func (r *repository) DeleteOpenzeppelinTimeLock(ctx context.Context, chainID int, contractAddress string) error {
	if err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ?", chainID, contractAddress).
		Delete(&types.OpenzeppelinTimeLock{}).Error; err != nil {
		logger.Error("DeleteOpenzeppelinTimeLock error", err, "chain_id", chainID, "contract_address", contractAddress)
		return err
	}
	logger.Info("DeleteOpenzeppelinTimeLock success", "chain_id", chainID, "contract_address", contractAddress)
	return nil
}

// UpdateOpenzeppelinTimeLockRemark 更新openzeppelin timelock备注
func (r *repository) UpdateOpenzeppelinTimeLockRemark(ctx context.Context, chainID int, contractAddress string, remark string) error {
	if err := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Where("chain_id = ? AND contract_address = ?", chainID, contractAddress).
		Update("remark", remark).Error; err != nil {
		logger.Error("UpdateOpenzeppelinTimeLockRemark error", err, "chain_id", chainID, "contract_address", contractAddress)
		return err
	}

	logger.Info("UpdateOpenzeppelinTimeLockRemark success", "chain_id", chainID, "contract_address", contractAddress, "remark_length", len(remark))
	return nil
}

// CheckCompoundTimeLockExists 检查compound timelock合约是否已存在
func (r *repository) CheckCompoundTimeLockExists(ctx context.Context, chainID int, contractAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where("chain_id = ? AND contract_address = ? AND status != ?", chainID, contractAddress, "deleted").
		Count(&count).Error

	if err != nil {
		logger.Error("CheckCompoundTimeLockExists error", err, "chain_id", chainID, "contract_address", contractAddress)
		return false, err
	}

	exists := count > 0
	logger.Info("CheckCompoundTimeLockExists", "chain_id", chainID, "contract_address", contractAddress, "exists", exists)
	return exists, nil
}

// CheckOpenzeppelinTimeLockExists 检查openzeppelin timelock合约是否已存在
func (r *repository) CheckOpenzeppelinTimeLockExists(ctx context.Context, chainID int, contractAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Where("chain_id = ? AND contract_address = ? AND status != ?", chainID, contractAddress, "deleted").
		Count(&count).Error

	if err != nil {
		logger.Error("CheckOpenzeppelinTimeLockExists error", err, "chain_id", chainID, "contract_address", contractAddress)
		return false, err
	}

	exists := count > 0
	logger.Info("CheckOpenzeppelinTimeLockExists", "chain_id", chainID, "contract_address", contractAddress, "exists", exists)
	return exists, nil
}

// GetTimeLocksByUserPermissions 根据用户权限获取timelock列表
func (r *repository) GetTimeLocksByUserPermissions(ctx context.Context, userAddress string, req *types.GetTimeLockListRequest) ([]types.CompoundTimeLockWithPermission, []types.OpenzeppelinTimeLockWithPermission, int64, error) {
	var compoundTimeLocks []types.CompoundTimeLock
	var openzeppelinTimeLocks []types.OpenzeppelinTimeLock
	var totalCount int64

	// 构建查询基础条件
	baseQuery := "status != ?"
	baseArgs := []interface{}{"deleted"}

	// 添加状态筛选条件
	if req.Status != "" {
		baseQuery += " AND status = ?"
		baseArgs = append(baseArgs, req.Status)
	}

	// 查询Compound timelocks - 用户是创建者、管理员或待定管理员
	compoundQuery := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where(baseQuery+" AND (creator_address = ? OR admin = ? OR pending_admin = ?)",
			append(baseArgs, userAddress, userAddress, userAddress)...)

	var compoundCount int64
	if err := compoundQuery.Count(&compoundCount).Error; err != nil {
		logger.Error("GetTimeLocksByUserPermissions compound count error", err, "user_address", userAddress)
		return nil, nil, 0, err
	}

	// 查询所有Compound timelocks（无分页）
	if err := compoundQuery.Order("created_at DESC").Find(&compoundTimeLocks).Error; err != nil {
		logger.Error("GetTimeLocksByUserPermissions compound query error", err, "user_address", userAddress)
		return nil, nil, 0, err
	}

	// 查询OpenZeppelin timelocks - 用户是创建者、提议者或执行者
	openzeppelinQuery := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Where(baseQuery+" AND (creator_address = ? OR proposers LIKE ? OR executors LIKE ?)",
			append(baseArgs, userAddress, "%"+userAddress+"%", "%"+userAddress+"%")...)

	var openzeppelinCount int64
	if err := openzeppelinQuery.Count(&openzeppelinCount).Error; err != nil {
		logger.Error("GetTimeLocksByUserPermissions openzeppelin count error", err, "user_address", userAddress)
		return nil, nil, 0, err
	}

	// 查询所有OpenZeppelin timelocks（无分页）
	if err := openzeppelinQuery.Order("created_at DESC").Find(&openzeppelinTimeLocks).Error; err != nil {
		logger.Error("GetTimeLocksByUserPermissions openzeppelin query error", err, "user_address", userAddress)
		return nil, nil, 0, err
	}

	totalCount = compoundCount + openzeppelinCount

	// 构建带权限信息的响应
	compoundWithPermissions := make([]types.CompoundTimeLockWithPermission, len(compoundTimeLocks))
	for i, tl := range compoundTimeLocks {
		permissions := r.getCompoundUserPermissions(tl, userAddress)
		compoundWithPermissions[i] = types.CompoundTimeLockWithPermission{
			CompoundTimeLock: tl,
			UserPermissions:  permissions,
		}
	}

	openzeppelinWithPermissions := make([]types.OpenzeppelinTimeLockWithPermission, len(openzeppelinTimeLocks))
	for i, tl := range openzeppelinTimeLocks {
		permissions := r.getOpenzeppelinUserPermissions(tl, userAddress)
		openzeppelinWithPermissions[i] = types.OpenzeppelinTimeLockWithPermission{
			OpenzeppelinTimeLock: tl,
			UserPermissions:      permissions,
		}
	}

	logger.Info("GetTimeLocksByUserPermissions success", "user_address", userAddress, "compound_count", len(compoundWithPermissions), "openzeppelin_count", len(openzeppelinWithPermissions), "total", totalCount)
	return compoundWithPermissions, openzeppelinWithPermissions, totalCount, nil
}

// ValidateCompoundOwnership 验证compound timelock合约的所有权
func (r *repository) ValidateCompoundOwnership(ctx context.Context, chainID int, contractAddress string, userAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where("chain_id = ? AND contract_address = ? AND creator_address = ? AND status != ?", chainID, contractAddress, userAddress, "deleted").
		Count(&count).Error

	if err != nil {
		logger.Error("ValidateCompoundOwnership error", err, "chain_id", chainID, "contract_address", contractAddress, "user_address", userAddress)
		return false, err
	}

	isOwner := count > 0
	logger.Info("ValidateCompoundOwnership", "chain_id", chainID, "contract_address", contractAddress, "user_address", userAddress, "is_owner", isOwner)
	return isOwner, nil
}

// ValidateOpenzeppelinOwnership 验证openzeppelin timelock合约的所有权
func (r *repository) ValidateOpenzeppelinOwnership(ctx context.Context, chainID int, contractAddress string, userAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Where("chain_id = ? AND contract_address = ? AND creator_address = ? AND status != ?", chainID, contractAddress, userAddress, "deleted").
		Count(&count).Error

	if err != nil {
		logger.Error("ValidateOpenzeppelinOwnership error", err, "chain_id", chainID, "contract_address", contractAddress, "user_address", userAddress)
		return false, err
	}

	isOwner := count > 0
	logger.Info("ValidateOpenzeppelinOwnership", "chain_id", chainID, "contract_address", contractAddress, "user_address", userAddress, "is_owner", isOwner)
	return isOwner, nil
}

// GetAllCompoundTimeLocksByUser 获取用户相关的所有compound timelock合约
func (r *repository) GetAllCompoundTimeLocksByUser(ctx context.Context, userAddress string) ([]types.CompoundTimeLock, error) {
	var timelocks []types.CompoundTimeLock

	err := r.db.WithContext(ctx).
		Where("(creator_address = ? OR admin = ? OR pending_admin = ?) AND status != ?", userAddress, userAddress, userAddress, "deleted").
		Find(&timelocks).Error

	if err != nil {
		logger.Error("GetAllCompoundTimeLocksByUser error", err, "user_address", userAddress)
		return nil, err
	}

	logger.Info("GetAllCompoundTimeLocksByUser success", "user_address", userAddress, "count", len(timelocks))
	return timelocks, nil
}

// GetAllOpenzeppelinTimeLocksByUser 获取用户相关的所有openzeppelin timelock合约
func (r *repository) GetAllOpenzeppelinTimeLocksByUser(ctx context.Context, userAddress string) ([]types.OpenzeppelinTimeLock, error) {
	var timelocks []types.OpenzeppelinTimeLock

	err := r.db.WithContext(ctx).
		Where("(creator_address = ? OR proposers LIKE ? OR executors LIKE ?) AND status != ?", userAddress, "%"+userAddress+"%", "%"+userAddress+"%", "deleted").
		Find(&timelocks).Error

	if err != nil {
		logger.Error("GetAllOpenzeppelinTimeLocksByUser error", err, "user_address", userAddress)
		return nil, err
	}

	logger.Info("GetAllOpenzeppelinTimeLocksByUser success", "user_address", userAddress, "count", len(timelocks))
	return timelocks, nil
}

// GetAllActiveCompoundTimeLocks 获取所有活跃的compound timelock合约
func (r *repository) GetAllActiveCompoundTimeLocks(ctx context.Context) ([]types.CompoundTimeLock, error) {
	var timelocks []types.CompoundTimeLock

	err := r.db.WithContext(ctx).
		Where("status = ?", "active").
		Find(&timelocks).Error

	if err != nil {
		logger.Error("GetAllActiveCompoundTimeLocks error", err)
		return nil, err
	}

	logger.Info("GetAllActiveCompoundTimeLocks success", "count", len(timelocks))
	return timelocks, nil
}

// GetAllActiveOpenzeppelinTimeLocks 获取所有活跃的openzeppelin timelock合约
func (r *repository) GetAllActiveOpenzeppelinTimeLocks(ctx context.Context) ([]types.OpenzeppelinTimeLock, error) {
	var timelocks []types.OpenzeppelinTimeLock

	err := r.db.WithContext(ctx).
		Where("status = ?", "active").
		Find(&timelocks).Error

	if err != nil {
		logger.Error("GetAllActiveOpenzeppelinTimeLocks error", err)
		return nil, err
	}

	logger.Info("GetAllActiveOpenzeppelinTimeLocks success", "count", len(timelocks))
	return timelocks, nil
}

// getCompoundUserPermissions 获取compound timelock合约的用户权限
func (r *repository) getCompoundUserPermissions(tl types.CompoundTimeLock, userAddress string) []string {
	var permissions []string

	if tl.CreatorAddress == userAddress {
		permissions = append(permissions, "creator")
	}
	if tl.Admin == userAddress {
		permissions = append(permissions, "admin")
	}
	if tl.PendingAdmin != nil && *tl.PendingAdmin == userAddress {
		permissions = append(permissions, "pending_admin")
	}

	return permissions
}

// getOpenzeppelinUserPermissions 获取openzeppelin timelock合约的用户权限
func (r *repository) getOpenzeppelinUserPermissions(tl types.OpenzeppelinTimeLock, userAddress string) []string {
	var permissions []string

	if tl.CreatorAddress == userAddress {
		permissions = append(permissions, "creator")
	}
	if r.containsAddress(tl.Proposers, userAddress) {
		permissions = append(permissions, "proposer")
	}
	if r.containsAddress(tl.Executors, userAddress) {
		permissions = append(permissions, "executor")
	}

	return permissions
}

// containsAddress 检查地址是否在列表中
func (r *repository) containsAddress(jsonAddresses, address string) bool {
	var addresses []string
	if err := json.Unmarshal([]byte(jsonAddresses), &addresses); err != nil {
		return false
	}

	for _, addr := range addresses {
		if strings.EqualFold(addr, address) {
			return true
		}
	}
	return false
}
