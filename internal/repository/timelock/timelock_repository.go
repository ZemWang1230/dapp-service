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
	GetCompoundTimeLockByID(ctx context.Context, id int64) (*types.CompoundTimeLock, error)
	GetCompoundTimeLockByAddress(ctx context.Context, chainID int, contractAddress string, timeLock *types.CompoundTimeLock) error
	UpdateCompoundTimeLock(ctx context.Context, timeLock *types.CompoundTimeLock) error
	DeleteCompoundTimeLock(ctx context.Context, id int64) error
	UpdateCompoundTimeLockRemark(ctx context.Context, id int64, remark string) error
	SetPendingAdmin(ctx context.Context, id int64, pendingAdmin string) error
	AcceptAdmin(ctx context.Context, id int64, newAdmin string) error

	// OpenZeppelin Timelock操作
	CreateOpenzeppelinTimeLock(ctx context.Context, timeLock *types.OpenzeppelinTimeLock) error
	GetOpenzeppelinTimeLockByID(ctx context.Context, id int64) (*types.OpenzeppelinTimeLock, error)
	GetOpenzeppelinTimeLockByAddress(ctx context.Context, chainID int, contractAddress string, timeLock *types.OpenzeppelinTimeLock) error
	UpdateOpenzeppelinTimeLock(ctx context.Context, timeLock *types.OpenzeppelinTimeLock) error
	DeleteOpenzeppelinTimeLock(ctx context.Context, id int64) error
	UpdateOpenzeppelinTimeLockRemark(ctx context.Context, id int64, remark string) error

	// 查询操作
	CheckCompoundTimeLockExists(ctx context.Context, chainID int, contractAddress string) (bool, error)
	CheckOpenzeppelinTimeLockExists(ctx context.Context, chainID int, contractAddress string) (bool, error)

	// 权限相关查询
	GetTimeLocksByUserPermissions(ctx context.Context, userAddress string, req *types.GetTimeLockListRequest) ([]types.CompoundTimeLockWithPermission, []types.OpenzeppelinTimeLockWithPermission, int64, error)

	// 验证操作
	ValidateCompoundOwnership(ctx context.Context, id int64, userAddress string) (bool, error)
	ValidateOpenzeppelinOwnership(ctx context.Context, id int64, userAddress string) (bool, error)
	CheckCompoundAdminPermissions(ctx context.Context, id int64, userAddress string) (bool, bool, error) // canSetPendingAdmin, canAcceptAdmin

	// 获取活跃timelock合约（用于事件监听）
	GetActiveCompoundTimelocksByChain(ctx context.Context, chainID int) ([]types.CompoundTimeLock, error)
	GetActiveOpenZeppelinTimelocksByChain(ctx context.Context, chainID int) ([]types.OpenzeppelinTimeLock, error)
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
		logger.Error("CreateCompoundTimeLock Error: ", err, "creator_address", timeLock.CreatorAddress, "contract_address", timeLock.ContractAddress)
		return err
	}

	logger.Info("CreateCompoundTimeLock: ", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress, "contract_address", timeLock.ContractAddress)
	return nil
}

// GetCompoundTimeLockByID 根据ID获取compound timelock合约
func (r *repository) GetCompoundTimeLockByID(ctx context.Context, id int64) (*types.CompoundTimeLock, error) {
	var timeLock types.CompoundTimeLock
	err := r.db.WithContext(ctx).
		Where("id = ? AND status != ?", id, types.TimeLockDeleted).
		First(&timeLock).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("timelock not found")
		}
		logger.Error("GetCompoundTimeLockByID Error: ", err, "timelock_id", id)
		return nil, err
	}

	logger.Info("GetCompoundTimeLockByID: ", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return &timeLock, nil
}

// GetCompoundTimeLockByAddress 根据链ID和合约地址获取compound timelock合约
func (r *repository) GetCompoundTimeLockByAddress(ctx context.Context, chainID int, contractAddress string, timeLock *types.CompoundTimeLock) error {
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ? AND status != ?", chainID, contractAddress, types.TimeLockDeleted).
		First(timeLock).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("timelock not found")
		}
		logger.Error("GetCompoundTimeLockByAddress Error: ", err, "chain_id", chainID, "contract_address", contractAddress)
		return err
	}

	logger.Info("GetCompoundTimeLockByAddress: ", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return nil
}

// UpdateCompoundTimeLock 更新compound timelock合约信息
func (r *repository) UpdateCompoundTimeLock(ctx context.Context, timeLock *types.CompoundTimeLock) error {
	if err := r.db.WithContext(ctx).
		Model(timeLock).
		Where("id = ?", timeLock.ID).
		Updates(timeLock).Error; err != nil {
		logger.Error("UpdateCompoundTimeLock Error: ", err, "timelock_id", timeLock.ID)
		return err
	}

	logger.Info("UpdateCompoundTimeLock: ", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return nil
}

// DeleteCompoundTimeLock 删除compound timelock合约（软删除）
func (r *repository) DeleteCompoundTimeLock(ctx context.Context, id int64) error {
	if err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where("id = ?", id).
		Update("status", types.TimeLockDeleted).Error; err != nil {
		logger.Error("DeleteCompoundTimeLock Error: ", err, "timelock_id", id)
		return err
	}

	logger.Info("DeleteCompoundTimeLock: ", "timelock_id", id)
	return nil
}

// UpdateCompoundTimeLockRemark 更新compound timelock备注
func (r *repository) UpdateCompoundTimeLockRemark(ctx context.Context, id int64, remark string) error {
	if err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where("id = ?", id).
		Update("remark", remark).Error; err != nil {
		logger.Error("UpdateCompoundTimeLockRemark Error: ", err, "timelock_id", id)
		return err
	}

	logger.Info("UpdateCompoundTimeLockRemark: ", "timelock_id", id, "remark_length", len(remark))
	return nil
}

// SetPendingAdmin 设置待定管理员
func (r *repository) SetPendingAdmin(ctx context.Context, id int64, pendingAdmin string) error {
	if err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where("id = ?", id).
		Update("pending_admin", pendingAdmin).Error; err != nil {
		logger.Error("SetPendingAdmin Error: ", err, "timelock_id", id)
		return err
	}

	logger.Info("SetPendingAdmin: ", "timelock_id", id, "pending_admin", pendingAdmin)
	return nil
}

// AcceptAdmin 接受管理员权限
func (r *repository) AcceptAdmin(ctx context.Context, id int64, newAdmin string) error {
	if err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"admin":         newAdmin,
			"pending_admin": nil,
		}).Error; err != nil {
		logger.Error("AcceptAdmin Error: ", err, "timelock_id", id)
		return err
	}

	logger.Info("AcceptAdmin: ", "timelock_id", id, "new_admin", newAdmin)
	return nil
}

// CreateOpenzeppelinTimeLock 创建openzeppelin timelock合约记录
func (r *repository) CreateOpenzeppelinTimeLock(ctx context.Context, timeLock *types.OpenzeppelinTimeLock) error {
	if err := r.db.WithContext(ctx).Create(timeLock).Error; err != nil {
		logger.Error("CreateOpenzeppelinTimeLock Error: ", err, "creator_address", timeLock.CreatorAddress, "contract_address", timeLock.ContractAddress)
		return err
	}

	logger.Info("CreateOpenzeppelinTimeLock: ", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress, "contract_address", timeLock.ContractAddress)
	return nil
}

// GetOpenzeppelinTimeLockByID 根据ID获取openzeppelin timelock合约
func (r *repository) GetOpenzeppelinTimeLockByID(ctx context.Context, id int64) (*types.OpenzeppelinTimeLock, error) {
	var timeLock types.OpenzeppelinTimeLock
	err := r.db.WithContext(ctx).
		Where("id = ? AND status != ?", id, types.TimeLockDeleted).
		First(&timeLock).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("timelock not found")
		}
		logger.Error("GetOpenzeppelinTimeLockByID Error: ", err, "timelock_id", id)
		return nil, err
	}

	logger.Info("GetOpenzeppelinTimeLockByID: ", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return &timeLock, nil
}

// GetOpenzeppelinTimeLockByAddress 根据链ID和合约地址获取openzeppelin timelock合约
func (r *repository) GetOpenzeppelinTimeLockByAddress(ctx context.Context, chainID int, contractAddress string, timeLock *types.OpenzeppelinTimeLock) error {
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ? AND status != ?", chainID, contractAddress, types.TimeLockDeleted).
		First(timeLock).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("timelock not found")
		}
		logger.Error("GetOpenzeppelinTimeLockByAddress Error: ", err, "chain_id", chainID, "contract_address", contractAddress)
		return err
	}

	logger.Info("GetOpenzeppelinTimeLockByAddress: ", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return nil
}

// UpdateOpenzeppelinTimeLock 更新openzeppelin timelock合约信息
func (r *repository) UpdateOpenzeppelinTimeLock(ctx context.Context, timeLock *types.OpenzeppelinTimeLock) error {
	if err := r.db.WithContext(ctx).
		Model(timeLock).
		Where("id = ?", timeLock.ID).
		Updates(timeLock).Error; err != nil {
		logger.Error("UpdateOpenzeppelinTimeLock Error: ", err, "timelock_id", timeLock.ID)
		return err
	}

	logger.Info("UpdateOpenzeppelinTimeLock: ", "timelock_id", timeLock.ID, "creator_address", timeLock.CreatorAddress)
	return nil
}

// DeleteOpenzeppelinTimeLock 删除openzeppelin timelock合约（软删除）
func (r *repository) DeleteOpenzeppelinTimeLock(ctx context.Context, id int64) error {
	if err := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Where("id = ?", id).
		Update("status", types.TimeLockDeleted).Error; err != nil {
		logger.Error("DeleteOpenzeppelinTimeLock Error: ", err, "timelock_id", id)
		return err
	}

	logger.Info("DeleteOpenzeppelinTimeLock: ", "timelock_id", id)
	return nil
}

// UpdateOpenzeppelinTimeLockRemark 更新openzeppelin timelock备注
func (r *repository) UpdateOpenzeppelinTimeLockRemark(ctx context.Context, id int64, remark string) error {
	if err := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Where("id = ?", id).
		Update("remark", remark).Error; err != nil {
		logger.Error("UpdateOpenzeppelinTimeLockRemark Error: ", err, "timelock_id", id)
		return err
	}

	logger.Info("UpdateOpenzeppelinTimeLockRemark: ", "timelock_id", id, "remark_length", len(remark))
	return nil
}

// CheckCompoundTimeLockExists 检查compound timelock合约是否已存在
func (r *repository) CheckCompoundTimeLockExists(ctx context.Context, chainID int, contractAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where("chain_id = ? AND contract_address = ? AND status != ?",
			chainID, contractAddress, types.TimeLockDeleted).
		Count(&count).Error

	if err != nil {
		logger.Error("CheckCompoundTimeLockExists Error: ", err, "chain_id", chainID, "contract_address", contractAddress)
		return false, err
	}

	exists := count > 0
	logger.Info("CheckCompoundTimeLockExists: ", "chain_id", chainID, "contract_address", contractAddress, "exists", exists)
	return exists, nil
}

// CheckOpenzeppelinTimeLockExists 检查openzeppelin timelock合约是否已存在
func (r *repository) CheckOpenzeppelinTimeLockExists(ctx context.Context, chainID int, contractAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Where("chain_id = ? AND contract_address = ? AND status != ?",
			chainID, contractAddress, types.TimeLockDeleted).
		Count(&count).Error

	if err != nil {
		logger.Error("CheckOpenzeppelinTimeLockExists Error: ", err, "chain_id", chainID, "contract_address", contractAddress)
		return false, err
	}

	exists := count > 0
	logger.Info("CheckOpenzeppelinTimeLockExists: ", "chain_id", chainID, "contract_address", contractAddress, "exists", exists)
	return exists, nil
}

// GetTimeLocksByUserPermissions 根据用户权限获取timelock列表（所有链）
func (r *repository) GetTimeLocksByUserPermissions(ctx context.Context, userAddress string, req *types.GetTimeLockListRequest) ([]types.CompoundTimeLockWithPermission, []types.OpenzeppelinTimeLockWithPermission, int64, error) {
	var compoundTimeLocks []types.CompoundTimeLock
	var openzeppelinTimeLocks []types.OpenzeppelinTimeLock
	var totalCount int64

	// 构建查询基础条件
	baseQuery := "status != ?"
	baseArgs := []interface{}{types.TimeLockDeleted}

	// 添加状态筛选条件
	if req.Status != nil {
		baseQuery += " AND status = ?"
		baseArgs = append(baseArgs, *req.Status)
	}

	// 查询Compound timelocks - 用户是创建者、管理员或待定管理员
	compoundQuery := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where(baseQuery+" AND (creator_address = ? OR admin = ? OR pending_admin = ?)",
			append(baseArgs, userAddress, userAddress, userAddress)...)

	var compoundCount int64
	if err := compoundQuery.Count(&compoundCount).Error; err != nil {
		logger.Error("GetTimeLocksByUserPermissions Compound Count Error: ", err, "user_address", userAddress)
		return nil, nil, 0, err
	}

	// 查询所有Compound timelocks（无分页）
	if err := compoundQuery.
		Order("created_at DESC").
		Find(&compoundTimeLocks).Error; err != nil {
		logger.Error("GetTimeLocksByUserPermissions Compound Query Error: ", err, "user_address", userAddress)
		return nil, nil, 0, err
	}

	// 查询OpenZeppelin timelocks - 用户是创建者、提议者、执行者或取消者
	openzeppelinQuery := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Where(baseQuery+" AND (creator_address = ? OR proposers LIKE ? OR executors LIKE ? OR cancellers LIKE ?)",
			append(baseArgs, userAddress, "%"+userAddress+"%", "%"+userAddress+"%", "%"+userAddress+"%")...)

	var openzeppelinCount int64
	if err := openzeppelinQuery.Count(&openzeppelinCount).Error; err != nil {
		logger.Error("GetTimeLocksByUserPermissions OpenZeppelin Count Error: ", err, "user_address", userAddress)
		return nil, nil, 0, err
	}

	// 查询所有OpenZeppelin timelocks（无分页）
	if err := openzeppelinQuery.
		Order("created_at DESC").
		Find(&openzeppelinTimeLocks).Error; err != nil {
		logger.Error("GetTimeLocksByUserPermissions OpenZeppelin Query Error: ", err, "user_address", userAddress)
		return nil, nil, 0, err
	}

	totalCount = compoundCount + openzeppelinCount

	// 构建带权限信息的响应
	compoundWithPermissions := make([]types.CompoundTimeLockWithPermission, len(compoundTimeLocks))
	for i, tl := range compoundTimeLocks {
		permissions := r.getCompoundUserPermissions(tl, userAddress)
		compoundWithPermissions[i] = types.CompoundTimeLockWithPermission{
			CompoundTimeLock:   tl,
			UserPermissions:    permissions,
			CanSetPendingAdmin: r.canSetPendingAdmin(tl, userAddress),
			CanAcceptAdmin:     r.canAcceptAdmin(tl, userAddress),
		}
	}

	openzeppelinWithPermissions := make([]types.OpenzeppelinTimeLockWithPermission, len(openzeppelinTimeLocks))
	for i, tl := range openzeppelinTimeLocks {
		permissions := r.getOpenzeppelinUserPermissions(tl, userAddress)
		proposersList, _ := r.parseAddressList(tl.Proposers)
		executorsList, _ := r.parseAddressList(tl.Executors)
		cancellersList, _ := r.parseAddressList(tl.Cancellers)

		openzeppelinWithPermissions[i] = types.OpenzeppelinTimeLockWithPermission{
			OpenzeppelinTimeLock: tl,
			UserPermissions:      permissions,
			ProposersList:        proposersList,
			ExecutorsList:        executorsList,
			CancellersList:       cancellersList,
		}
	}

	logger.Info("GetTimeLocksByUserPermissions: ", "user_address", userAddress, "compound_count", len(compoundWithPermissions), "openzeppelin_count", len(openzeppelinWithPermissions), "total", totalCount)
	return compoundWithPermissions, openzeppelinWithPermissions, totalCount, nil
}

// ValidateCompoundOwnership 验证compound timelock合约的所有权
func (r *repository) ValidateCompoundOwnership(ctx context.Context, id int64, userAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.CompoundTimeLock{}).
		Where("id = ? AND creator_address = ? AND status != ?", id, userAddress, types.TimeLockDeleted).
		Count(&count).Error

	if err != nil {
		logger.Error("ValidateCompoundOwnership Error: ", err, "timelock_id", id, "user_address", userAddress)
		return false, err
	}

	isOwner := count > 0
	logger.Info("ValidateCompoundOwnership: ", "timelock_id", id, "user_address", userAddress, "is_owner", isOwner)
	return isOwner, nil
}

// ValidateOpenzeppelinOwnership 验证openzeppelin timelock合约的所有权
func (r *repository) ValidateOpenzeppelinOwnership(ctx context.Context, id int64, userAddress string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimeLock{}).
		Where("id = ? AND creator_address = ? AND status != ?", id, userAddress, types.TimeLockDeleted).
		Count(&count).Error

	if err != nil {
		logger.Error("ValidateOpenzeppelinOwnership Error: ", err, "timelock_id", id, "user_address", userAddress)
		return false, err
	}

	isOwner := count > 0
	logger.Info("ValidateOpenzeppelinOwnership: ", "timelock_id", id, "user_address", userAddress, "is_owner", isOwner)
	return isOwner, nil
}

// CheckCompoundAdminPermissions 检查compound timelock的管理员权限
func (r *repository) CheckCompoundAdminPermissions(ctx context.Context, id int64, userAddress string) (bool, bool, error) {
	var timeLock types.CompoundTimeLock
	err := r.db.WithContext(ctx).
		Where("id = ? AND status != ?", id, types.TimeLockDeleted).
		First(&timeLock).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, false, fmt.Errorf("timelock not found")
		}
		logger.Error("CheckCompoundAdminPermissions Error: ", err, "timelock_id", id)
		return false, false, err
	}

	canSetPendingAdmin := r.canSetPendingAdmin(timeLock, userAddress)
	canAcceptAdmin := r.canAcceptAdmin(timeLock, userAddress)

	logger.Info("CheckCompoundAdminPermissions: ", "timelock_id", id, "user_address", userAddress,
		"can_set_pending_admin", canSetPendingAdmin, "can_accept_admin", canAcceptAdmin)
	return canSetPendingAdmin, canAcceptAdmin, nil
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
	if r.containsAddress(tl.Cancellers, userAddress) {
		permissions = append(permissions, "canceller")
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

// parseAddressList 解析JSON地址列表
func (r *repository) parseAddressList(jsonAddresses string) ([]string, error) {
	var addresses []string
	if err := json.Unmarshal([]byte(jsonAddresses), &addresses); err != nil {
		return nil, err
	}
	return addresses, nil
}

// canSetPendingAdmin 检查用户是否可以设置待定管理员
func (r *repository) canSetPendingAdmin(tl types.CompoundTimeLock, userAddress string) bool {
	return tl.Admin == userAddress
}

// canAcceptAdmin 检查用户是否可以接受管理员权限
func (r *repository) canAcceptAdmin(tl types.CompoundTimeLock, userAddress string) bool {
	return tl.PendingAdmin != nil && *tl.PendingAdmin == userAddress
}

// GetActiveCompoundTimelocksByChain 获取指定链上活跃的compound timelock合约
func (r *repository) GetActiveCompoundTimelocksByChain(ctx context.Context, chainID int) ([]types.CompoundTimeLock, error) {
	var timelocks []types.CompoundTimeLock

	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND status = ?", chainID, types.TimeLockActive).
		Find(&timelocks).Error

	if err != nil {
		logger.Error("GetActiveCompoundTimelocksByChain Error: ", err, "chain_id", chainID)
		return nil, err
	}

	logger.Info("GetActiveCompoundTimelocksByChain: ", "chain_id", chainID, "count", len(timelocks))
	return timelocks, nil
}

// GetActiveOpenZeppelinTimelocksByChain 获取指定链上活跃的openzeppelin timelock合约
func (r *repository) GetActiveOpenZeppelinTimelocksByChain(ctx context.Context, chainID int) ([]types.OpenzeppelinTimeLock, error) {
	var timelocks []types.OpenzeppelinTimeLock

	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND status = ?", chainID, types.TimeLockActive).
		Find(&timelocks).Error

	if err != nil {
		logger.Error("GetActiveOpenZeppelinTimelocksByChain Error: ", err, "chain_id", chainID)
		return nil, err
	}

	logger.Info("GetActiveOpenZeppelinTimelocksByChain: ", "chain_id", chainID, "count", len(timelocks))
	return timelocks, nil
}
