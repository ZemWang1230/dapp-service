package timelock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	"timelocker-backend/internal/repository/timelock"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/crypto"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

var (
	ErrTimeLockNotFound      = errors.New("timelock not found")
	ErrTimeLockExists        = errors.New("timelock already exists")
	ErrInvalidContract       = errors.New("invalid timelock contract")
	ErrInvalidStandard       = errors.New("invalid contract standard")
	ErrUnauthorized          = errors.New("unauthorized access")
	ErrInvalidRemark         = errors.New("invalid remark content")
	ErrInvalidContractParams = errors.New("invalid contract parameters")
	ErrInvalidPermissions    = errors.New("insufficient permissions")
)

// Service timelock服务接口
type Service interface {
	// 创建timelock合约
	CreateTimeLock(ctx context.Context, userAddress string, req *types.CreateTimeLockRequest) (interface{}, error)

	// 导入timelock合约
	ImportTimeLock(ctx context.Context, userAddress string, req *types.ImportTimeLockRequest) (interface{}, error)

	// 获取timelock列表（按权限筛选）
	GetTimeLockList(ctx context.Context, userAddress string, req *types.GetTimeLockListRequest) (*types.GetTimeLockListResponse, error)

	// 获取timelock详情
	GetTimeLockDetail(ctx context.Context, userAddress string, standard types.TimeLockStandard, id int64) (*types.TimeLockDetailResponse, error)

	// 更新timelock备注
	UpdateTimeLock(ctx context.Context, userAddress string, req *types.UpdateTimeLockRequest) error

	// 删除timelock
	DeleteTimeLock(ctx context.Context, userAddress string, req *types.DeleteTimeLockRequest) error

	// Compound特有功能 - 设置pending admin
	SetPendingAdmin(ctx context.Context, userAddress string, req *types.SetPendingAdminRequest) error

	// Compound特有功能 - 接受admin权限
	AcceptAdmin(ctx context.Context, userAddress string, req *types.AcceptAdminRequest) error

	// 检查用户对compound timelock的admin权限
	CheckAdminPermissions(ctx context.Context, userAddress string, req *types.CheckAdminPermissionRequest) (*types.CheckAdminPermissionResponse, error)
}

type service struct {
	timeLockRepo timelock.Repository
}

// NewService 创建timelock服务实例
func NewService(timeLockRepo timelock.Repository) Service {
	return &service{
		timeLockRepo: timeLockRepo,
	}
}

// CreateTimeLock 创建timelock合约记录
func (s *service) CreateTimeLock(ctx context.Context, userAddress string, req *types.CreateTimeLockRequest) (interface{}, error) {
	logger.Info("CreateTimeLock: ", "user_address", userAddress, "contract_address", req.ContractAddress, "standard", req.Standard)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)
	normalizedContract := crypto.NormalizeAddress(req.ContractAddress)

	// 验证请求参数
	if err := s.validateCreateRequest(req); err != nil {
		logger.Error("CreateTimeLock Validation Error: ", err, "user_address", normalizedUser)
		return nil, err
	}

	// 检查合约是否已存在
	if err := s.checkContractExists(ctx, req.Standard, req.ChainID, normalizedContract); err != nil {
		logger.Error("CreateTimeLock Check Exists Error: ", err, "user_address", normalizedUser)
		return nil, err
	}

	switch req.Standard {
	case types.CompoundStandard:
		logger.Info("CreateTimeLock Compound Standard: ", "user_address", normalizedUser, "contract_address", normalizedContract)
		return s.createCompoundTimeLock(ctx, normalizedUser, normalizedContract, req)
	case types.OpenzeppelinStandard:
		logger.Info("CreateTimeLock Openzeppelin Standard: ", "user_address", normalizedUser, "contract_address", normalizedContract)
		return s.createOpenzeppelinTimeLock(ctx, normalizedUser, normalizedContract, req)
	default:
		logger.Error("CreateTimeLock Error: ", fmt.Errorf("invalid standard: %s", req.Standard))
		return nil, ErrInvalidStandard
	}
}

// ImportTimeLock 导入timelock合约
func (s *service) ImportTimeLock(ctx context.Context, userAddress string, req *types.ImportTimeLockRequest) (interface{}, error) {
	logger.Info("ImportTimeLock: ", "user_address", userAddress, "contract_address", req.ContractAddress, "standard", req.Standard)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)
	normalizedContract := crypto.NormalizeAddress(req.ContractAddress)

	// 验证请求参数
	if err := s.validateImportRequest(req); err != nil {
		logger.Error("ImportTimeLock Validation Error: ", err, "user_address", normalizedUser)
		return nil, err
	}

	// 检查合约是否已存在
	if err := s.checkContractExists(ctx, req.Standard, req.ChainID, normalizedContract); err != nil {
		logger.Error("ImportTimeLock Check Exists Error: ", err, "user_address", normalizedUser)
		return nil, err
	}

	switch req.Standard {
	case types.CompoundStandard:
		logger.Info("ImportTimeLock Compound Standard: ", "user_address", normalizedUser, "contract_address", normalizedContract)
		return s.importCompoundTimeLock(ctx, normalizedUser, normalizedContract, req)
	case types.OpenzeppelinStandard:
		logger.Info("ImportTimeLock Openzeppelin Standard: ", "user_address", normalizedUser, "contract_address", normalizedContract)
		return s.importOpenzeppelinTimeLock(ctx, normalizedUser, normalizedContract, req)
	default:
		logger.Error("ImportTimeLock Error: ", fmt.Errorf("invalid standard: %s", req.Standard))
		return nil, ErrInvalidStandard
	}
}

// GetTimeLockList 获取timelock列表（根据用户权限筛选，所有链）
func (s *service) GetTimeLockList(ctx context.Context, userAddress string, req *types.GetTimeLockListRequest) (*types.GetTimeLockListResponse, error) {
	logger.Info("GetTimeLockList: ", "user_address", userAddress, "standard", req.Standard, "status", req.Status)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 如果指定了standard，只查询对应类型
	if req.Standard != nil {
		switch *req.Standard {
		case types.CompoundStandard:
			return s.getCompoundTimeLockList(ctx, normalizedUser, req)
		case types.OpenzeppelinStandard:
			return s.getOpenzeppelinTimeLockList(ctx, normalizedUser, req)
		default:
			logger.Error("GetTimeLockList Error: ", fmt.Errorf("invalid standard: %s", *req.Standard))
			return nil, ErrInvalidStandard
		}
	}

	// 查询所有有权限的timelock（所有链）
	compoundList, openzeppelinList, total, err := s.timeLockRepo.GetTimeLocksByUserPermissions(ctx, normalizedUser, req)
	if err != nil {
		logger.Error("GetTimeLockList Error: ", err, "user_address", normalizedUser)
		return nil, fmt.Errorf("failed to get timelock list: %w", err)
	}

	response := &types.GetTimeLockListResponse{
		CompoundTimeLocks:     compoundList,
		OpenzeppelinTimeLocks: openzeppelinList,
		Total:                 total,
	}

	logger.Info("GetTimeLockList Success: ", "user_address", normalizedUser, "total", total, "compound_count", len(compoundList), "openzeppelin_count", len(openzeppelinList))
	return response, nil
}

// GetTimeLockDetail 获取timelock详情
func (s *service) GetTimeLockDetail(ctx context.Context, userAddress string, standard types.TimeLockStandard, id int64) (*types.TimeLockDetailResponse, error) {
	logger.Info("GetTimeLockDetail: ", "user_address", userAddress, "standard", standard, "timelock_id", id)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)

	switch standard {
	case types.CompoundStandard:
		return s.getCompoundTimeLockDetail(ctx, normalizedUser, id)
	case types.OpenzeppelinStandard:
		return s.getOpenzeppelinTimeLockDetail(ctx, normalizedUser, id)
	default:
		logger.Error("GetTimeLockDetail Error: ", fmt.Errorf("invalid standard: %s", standard))
		return nil, ErrInvalidStandard
	}
}

// UpdateTimeLock 更新timelock备注
func (s *service) UpdateTimeLock(ctx context.Context, userAddress string, req *types.UpdateTimeLockRequest) error {
	logger.Info("UpdateTimeLock: ", "user_address", userAddress, "standard", req.Standard, "timelock_id", req.ID)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 验证备注
	if err := s.validateRemark(req.Remark); err != nil {
		logger.Error("UpdateTimeLock Remark Validation Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
		return err
	}

	sanitizedRemark := html.EscapeString(strings.TrimSpace(req.Remark))

	switch req.Standard {
	case types.CompoundStandard:
		// 验证所有权
		isOwner, err := s.timeLockRepo.ValidateCompoundOwnership(ctx, req.ID, normalizedUser)
		if err != nil {
			logger.Error("UpdateTimeLock Validate Ownership Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
			return fmt.Errorf("failed to validate ownership: %w", err)
		}
		if !isOwner {
			logger.Error("UpdateTimeLock Error: ", ErrUnauthorized, "timelock_id", req.ID, "user_address", normalizedUser)
			return ErrUnauthorized
		}

		if err := s.timeLockRepo.UpdateCompoundTimeLockRemark(ctx, req.ID, sanitizedRemark); err != nil {
			logger.Error("UpdateTimeLock Repository Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
			return fmt.Errorf("failed to update timelock: %w", err)
		}

	case types.OpenzeppelinStandard:
		// 验证所有权
		isOwner, err := s.timeLockRepo.ValidateOpenzeppelinOwnership(ctx, req.ID, normalizedUser)
		if err != nil {
			logger.Error("UpdateTimeLock Validate Ownership Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
			return fmt.Errorf("failed to validate ownership: %w", err)
		}
		if !isOwner {
			logger.Error("UpdateTimeLock Error: ", ErrUnauthorized, "timelock_id", req.ID, "user_address", normalizedUser)
			return ErrUnauthorized
		}

		if err := s.timeLockRepo.UpdateOpenzeppelinTimeLockRemark(ctx, req.ID, sanitizedRemark); err != nil {
			logger.Error("UpdateTimeLock Repository Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
			return fmt.Errorf("failed to update timelock: %w", err)
		}

	default:
		return ErrInvalidStandard
	}

	logger.Info("UpdateTimeLock Success: ", "timelock_id", req.ID, "user_address", normalizedUser)
	return nil
}

// DeleteTimeLock 删除timelock
func (s *service) DeleteTimeLock(ctx context.Context, userAddress string, req *types.DeleteTimeLockRequest) error {
	logger.Info("DeleteTimeLock: ", "user_address", userAddress, "standard", req.Standard, "timelock_id", req.ID)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)

	switch req.Standard {
	case types.CompoundStandard:
		// 验证所有权
		isOwner, err := s.timeLockRepo.ValidateCompoundOwnership(ctx, req.ID, normalizedUser)
		if err != nil {
			logger.Error("DeleteTimeLock Validate Ownership Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
			return fmt.Errorf("failed to validate ownership: %w", err)
		}
		if !isOwner {
			logger.Error("DeleteTimeLock Error: ", ErrUnauthorized, "timelock_id", req.ID, "user_address", normalizedUser)
			return ErrUnauthorized
		}

		if err := s.timeLockRepo.DeleteCompoundTimeLock(ctx, req.ID); err != nil {
			logger.Error("DeleteTimeLock Repository Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
			return fmt.Errorf("failed to delete timelock: %w", err)
		}

	case types.OpenzeppelinStandard:
		// 验证所有权
		isOwner, err := s.timeLockRepo.ValidateOpenzeppelinOwnership(ctx, req.ID, normalizedUser)
		if err != nil {
			logger.Error("DeleteTimeLock Validate Ownership Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
			return fmt.Errorf("failed to validate ownership: %w", err)
		}
		if !isOwner {
			logger.Error("DeleteTimeLock Error: ", ErrUnauthorized, "timelock_id", req.ID, "user_address", normalizedUser)
			return ErrUnauthorized
		}

		if err := s.timeLockRepo.DeleteOpenzeppelinTimeLock(ctx, req.ID); err != nil {
			logger.Error("DeleteTimeLock Repository Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
			return fmt.Errorf("failed to delete timelock: %w", err)
		}

	default:
		return ErrInvalidStandard
	}

	logger.Info("DeleteTimeLock Success: ", "timelock_id", req.ID, "user_address", normalizedUser)
	return nil
}

// SetPendingAdmin 设置pending admin（仅限compound）
func (s *service) SetPendingAdmin(ctx context.Context, userAddress string, req *types.SetPendingAdminRequest) error {
	logger.Info("SetPendingAdmin: ", "user_address", userAddress, "timelock_id", req.ID, "new_pending_admin", req.NewPendingAdmin)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)
	normalizedPendingAdmin := crypto.NormalizeAddress(req.NewPendingAdmin)

	// 验证地址格式
	if !crypto.ValidateEthereumAddress(req.NewPendingAdmin) {
		logger.Error("SetPendingAdmin Error: ", fmt.Errorf("%w: invalid pending admin address", ErrInvalidContractParams))
		return fmt.Errorf("%w: invalid pending admin address", ErrInvalidContractParams)
	}

	// 检查权限
	canSetPendingAdmin, _, err := s.timeLockRepo.CheckCompoundAdminPermissions(ctx, req.ID, normalizedUser)
	if err != nil {
		logger.Error("SetPendingAdmin Check Permissions Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
		return fmt.Errorf("failed to check permissions: %w", err)
	}
	if !canSetPendingAdmin {
		logger.Error("SetPendingAdmin Error: ", ErrInvalidPermissions, "timelock_id", req.ID, "user_address", normalizedUser)
		return ErrInvalidPermissions
	}

	// 设置pending admin
	if err := s.timeLockRepo.SetPendingAdmin(ctx, req.ID, normalizedPendingAdmin); err != nil {
		logger.Error("SetPendingAdmin Repository Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
		return fmt.Errorf("failed to set pending admin: %w", err)
	}

	logger.Info("SetPendingAdmin Success: ", "timelock_id", req.ID, "user_address", normalizedUser, "new_pending_admin", normalizedPendingAdmin)
	return nil
}

// AcceptAdmin 接受admin权限（仅限compound）
func (s *service) AcceptAdmin(ctx context.Context, userAddress string, req *types.AcceptAdminRequest) error {
	logger.Info("AcceptAdmin: ", "user_address", userAddress, "timelock_id", req.ID)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 检查权限
	_, canAcceptAdmin, err := s.timeLockRepo.CheckCompoundAdminPermissions(ctx, req.ID, normalizedUser)
	if err != nil {
		logger.Error("AcceptAdmin Check Permissions Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
		return fmt.Errorf("failed to check permissions: %w", err)
	}
	if !canAcceptAdmin {
		logger.Error("AcceptAdmin Error: ", ErrInvalidPermissions, "timelock_id", req.ID, "user_address", normalizedUser)
		return ErrInvalidPermissions
	}

	// 接受admin权限
	if err := s.timeLockRepo.AcceptAdmin(ctx, req.ID, normalizedUser); err != nil {
		logger.Error("AcceptAdmin Repository Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
		return fmt.Errorf("failed to accept admin: %w", err)
	}

	logger.Info("AcceptAdmin Success: ", "timelock_id", req.ID, "user_address", normalizedUser)
	return nil
}

// CheckAdminPermissions 检查admin权限（仅限compound）
func (s *service) CheckAdminPermissions(ctx context.Context, userAddress string, req *types.CheckAdminPermissionRequest) (*types.CheckAdminPermissionResponse, error) {
	logger.Info("CheckAdminPermissions: ", "user_address", userAddress, "timelock_id", req.ID)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 检查权限
	canSetPendingAdmin, canAcceptAdmin, err := s.timeLockRepo.CheckCompoundAdminPermissions(ctx, req.ID, normalizedUser)
	if err != nil {
		logger.Error("CheckAdminPermissions Error: ", err, "timelock_id", req.ID, "user_address", normalizedUser)
		return nil, fmt.Errorf("failed to check permissions: %w", err)
	}

	response := &types.CheckAdminPermissionResponse{
		CanSetPendingAdmin: canSetPendingAdmin,
		CanAcceptAdmin:     canAcceptAdmin,
	}

	logger.Info("CheckAdminPermissions Success: ", "timelock_id", req.ID, "user_address", normalizedUser, "can_set_pending_admin", canSetPendingAdmin, "can_accept_admin", canAcceptAdmin)
	return response, nil
}

// 私有方法 - 创建Compound timelock
func (s *service) createCompoundTimeLock(ctx context.Context, userAddress, contractAddress string, req *types.CreateTimeLockRequest) (*types.CompoundTimeLock, error) {
	if req.Admin == nil {
		logger.Error("createCompoundTimeLock Error: ", fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams))
		return nil, fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams)
	}

	normalizedAdmin := crypto.NormalizeAddress(*req.Admin)

	timeLock := &types.CompoundTimeLock{
		CreatorAddress:  userAddress,
		ChainID:         req.ChainID,
		ChainName:       req.ChainName,
		ContractAddress: contractAddress,
		TxHash:          &req.TxHash,
		MinDelay:        req.MinDelay,
		Admin:           normalizedAdmin,
		Remark:          html.EscapeString(strings.TrimSpace(req.Remark)),
		Status:          types.TimeLockActive,
		IsImported:      false,
	}

	if err := s.timeLockRepo.CreateCompoundTimeLock(ctx, timeLock); err != nil {
		logger.Error("CreateCompoundTimeLock Error: ", fmt.Errorf("failed to create compound timelock: %w", err))
		return nil, fmt.Errorf("failed to create compound timelock: %w", err)
	}

	logger.Info("CreateCompoundTimeLock Success: ", "timelock_id", timeLock.ID, "user_address", userAddress, "contract_address", contractAddress)
	return timeLock, nil
}

// 私有方法 - 创建OpenZeppelin timelock
func (s *service) createOpenzeppelinTimeLock(ctx context.Context, userAddress, contractAddress string, req *types.CreateTimeLockRequest) (*types.OpenzeppelinTimeLock, error) {
	if len(req.Proposers) == 0 || len(req.Executors) == 0 || len(req.Cancellers) == 0 {
		logger.Error("createOpenzeppelinTimeLock Error: ", fmt.Errorf("%w: proposers, executors and cancellers are required for openzeppelin standard", ErrInvalidContractParams))
		return nil, fmt.Errorf("%w: proposers, executors and cancellers are required for openzeppelin standard", ErrInvalidContractParams)
	}

	// 标准化地址列表
	normalizedProposers := make([]string, len(req.Proposers))
	for i, addr := range req.Proposers {
		normalizedProposers[i] = crypto.NormalizeAddress(addr)
	}

	normalizedExecutors := make([]string, len(req.Executors))
	for i, addr := range req.Executors {
		normalizedExecutors[i] = crypto.NormalizeAddress(addr)
	}

	normalizedCancellers := make([]string, len(req.Cancellers))
	for i, addr := range req.Cancellers {
		normalizedCancellers[i] = crypto.NormalizeAddress(addr)
	}

	// JSON序列化
	proposersJSON, _ := json.Marshal(normalizedProposers)
	executorsJSON, _ := json.Marshal(normalizedExecutors)
	cancellersJSON, _ := json.Marshal(normalizedCancellers)

	timeLock := &types.OpenzeppelinTimeLock{
		CreatorAddress:  userAddress,
		ChainID:         req.ChainID,
		ChainName:       req.ChainName,
		ContractAddress: contractAddress,
		TxHash:          &req.TxHash,
		MinDelay:        req.MinDelay,
		Proposers:       string(proposersJSON),
		Executors:       string(executorsJSON),
		Cancellers:      string(cancellersJSON),
		Remark:          html.EscapeString(strings.TrimSpace(req.Remark)),
		Status:          types.TimeLockActive,
		IsImported:      false,
	}

	if err := s.timeLockRepo.CreateOpenzeppelinTimeLock(ctx, timeLock); err != nil {
		logger.Error("CreateOpenzeppelinTimeLock Error: ", fmt.Errorf("failed to create openzeppelin timelock: %w", err))
		return nil, fmt.Errorf("failed to create openzeppelin timelock: %w", err)
	}

	logger.Info("CreateOpenzeppelinTimeLock Success: ", "timelock_id", timeLock.ID, "user_address", userAddress, "contract_address", contractAddress)
	return timeLock, nil
}

// 私有方法 - 导入Compound timelock
func (s *service) importCompoundTimeLock(ctx context.Context, userAddress, contractAddress string, req *types.ImportTimeLockRequest) (*types.CompoundTimeLock, error) {
	if req.Admin == nil {
		logger.Error("importCompoundTimeLock Error: ", fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams))
		return nil, fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams)
	}

	normalizedAdmin := crypto.NormalizeAddress(*req.Admin)

	timeLock := &types.CompoundTimeLock{
		CreatorAddress:  userAddress,
		ChainID:         req.ChainID,
		ChainName:       req.ChainName,
		ContractAddress: contractAddress,
		MinDelay:        req.MinDelay,
		Admin:           normalizedAdmin,
		Remark:          html.EscapeString(strings.TrimSpace(req.Remark)),
		Status:          types.TimeLockActive,
		IsImported:      true,
	}

	if req.PendingAdmin != nil {
		normalizedPendingAdmin := crypto.NormalizeAddress(*req.PendingAdmin)
		timeLock.PendingAdmin = &normalizedPendingAdmin
	}

	if err := s.timeLockRepo.CreateCompoundTimeLock(ctx, timeLock); err != nil {
		logger.Error("ImportCompoundTimeLock Error: ", fmt.Errorf("failed to import compound timelock: %w", err))
		return nil, fmt.Errorf("failed to import compound timelock: %w", err)
	}

	logger.Info("ImportCompoundTimeLock Success: ", "timelock_id", timeLock.ID, "user_address", userAddress, "contract_address", contractAddress)
	return timeLock, nil
}

// 私有方法 - 导入OpenZeppelin timelock
func (s *service) importOpenzeppelinTimeLock(ctx context.Context, userAddress, contractAddress string, req *types.ImportTimeLockRequest) (*types.OpenzeppelinTimeLock, error) {
	if len(req.Proposers) == 0 || len(req.Executors) == 0 || len(req.Cancellers) == 0 {
		logger.Error("importOpenzeppelinTimeLock Error: ", fmt.Errorf("%w: proposers, executors and cancellers are required for openzeppelin standard", ErrInvalidContractParams))
		return nil, fmt.Errorf("%w: proposers, executors and cancellers are required for openzeppelin standard", ErrInvalidContractParams)
	}

	// 标准化地址列表
	normalizedProposers := make([]string, len(req.Proposers))
	for i, addr := range req.Proposers {
		normalizedProposers[i] = crypto.NormalizeAddress(addr)
	}

	normalizedExecutors := make([]string, len(req.Executors))
	for i, addr := range req.Executors {
		normalizedExecutors[i] = crypto.NormalizeAddress(addr)
	}

	normalizedCancellers := make([]string, len(req.Cancellers))
	for i, addr := range req.Cancellers {
		normalizedCancellers[i] = crypto.NormalizeAddress(addr)
	}

	// JSON序列化
	proposersJSON, _ := json.Marshal(normalizedProposers)
	executorsJSON, _ := json.Marshal(normalizedExecutors)
	cancellersJSON, _ := json.Marshal(normalizedCancellers)

	timeLock := &types.OpenzeppelinTimeLock{
		CreatorAddress:  userAddress,
		ChainID:         req.ChainID,
		ChainName:       req.ChainName,
		ContractAddress: contractAddress,
		MinDelay:        req.MinDelay,
		Proposers:       string(proposersJSON),
		Executors:       string(executorsJSON),
		Cancellers:      string(cancellersJSON),
		Remark:          html.EscapeString(strings.TrimSpace(req.Remark)),
		Status:          types.TimeLockActive,
		IsImported:      true,
	}

	if err := s.timeLockRepo.CreateOpenzeppelinTimeLock(ctx, timeLock); err != nil {
		logger.Error("ImportOpenzeppelinTimeLock Error: ", fmt.Errorf("failed to import openzeppelin timelock: %w", err))
		return nil, fmt.Errorf("failed to import openzeppelin timelock: %w", err)
	}

	logger.Info("ImportOpenzeppelinTimeLock Success: ", "timelock_id", timeLock.ID, "user_address", userAddress, "contract_address", contractAddress)
	return timeLock, nil
}

// 私有方法 - 获取Compound timelock列表
func (s *service) getCompoundTimeLockList(ctx context.Context, userAddress string, req *types.GetTimeLockListRequest) (*types.GetTimeLockListResponse, error) {
	compoundList, _, _, err := s.timeLockRepo.GetTimeLocksByUserPermissions(ctx, userAddress, req)
	if err != nil {
		logger.Error("getCompoundTimeLockList Error: ", err)
		return nil, err
	}

	// 只返回compound类型
	return &types.GetTimeLockListResponse{
		CompoundTimeLocks:     compoundList,
		OpenzeppelinTimeLocks: []types.OpenzeppelinTimeLockWithPermission{}, // 空数组
		Total:                 int64(len(compoundList)),
	}, nil
}

// 私有方法 - 获取OpenZeppelin timelock列表
func (s *service) getOpenzeppelinTimeLockList(ctx context.Context, userAddress string, req *types.GetTimeLockListRequest) (*types.GetTimeLockListResponse, error) {
	_, openzeppelinList, _, err := s.timeLockRepo.GetTimeLocksByUserPermissions(ctx, userAddress, req)
	if err != nil {
		logger.Error("getOpenzeppelinTimeLockList Error: ", err)
		return nil, err
	}

	// 只返回openzeppelin类型
	return &types.GetTimeLockListResponse{
		CompoundTimeLocks:     []types.CompoundTimeLockWithPermission{}, // 空数组
		OpenzeppelinTimeLocks: openzeppelinList,
		Total:                 int64(len(openzeppelinList)),
	}, nil
}

// 私有方法 - 获取Compound timelock详情
func (s *service) getCompoundTimeLockDetail(ctx context.Context, userAddress string, id int64) (*types.TimeLockDetailResponse, error) {
	timeLock, err := s.timeLockRepo.GetCompoundTimeLockByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTimeLockNotFound
		}
		logger.Error("getCompoundTimeLockDetail Error: ", err)
		return nil, fmt.Errorf("failed to get timelock: %w", err)
	}

	// 检查用户是否有权限查看（创建者、管理员或待定管理员）
	hasPermission := timeLock.CreatorAddress == userAddress ||
		timeLock.Admin == userAddress ||
		(timeLock.PendingAdmin != nil && *timeLock.PendingAdmin == userAddress)

	if !hasPermission {
		logger.Error("getCompoundTimeLockDetail Error: ", ErrUnauthorized)
		return nil, ErrUnauthorized
	}

	// 构建权限信息
	var permissions []string
	if timeLock.CreatorAddress == userAddress {
		permissions = append(permissions, "creator")
	}
	if timeLock.Admin == userAddress {
		permissions = append(permissions, "admin")
	}
	if timeLock.PendingAdmin != nil && *timeLock.PendingAdmin == userAddress {
		permissions = append(permissions, "pending_admin")
	}

	compoundData := &types.CompoundTimeLockWithPermission{
		CompoundTimeLock:   *timeLock,
		UserPermissions:    permissions,
		CanSetPendingAdmin: timeLock.Admin == userAddress,
		CanAcceptAdmin:     timeLock.PendingAdmin != nil && *timeLock.PendingAdmin == userAddress,
	}

	return &types.TimeLockDetailResponse{
		Standard:     types.CompoundStandard,
		CompoundData: compoundData,
	}, nil
}

// 私有方法 - 获取OpenZeppelin timelock详情
func (s *service) getOpenzeppelinTimeLockDetail(ctx context.Context, userAddress string, id int64) (*types.TimeLockDetailResponse, error) {
	timeLock, err := s.timeLockRepo.GetOpenzeppelinTimeLockByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTimeLockNotFound
		}
		logger.Error("getOpenzeppelinTimeLockDetail Error: ", err)
		return nil, fmt.Errorf("failed to get timelock: %w", err)
	}

	// 检查用户是否有权限查看
	hasPermission := timeLock.CreatorAddress == userAddress ||
		s.containsAddress(timeLock.Proposers, userAddress) ||
		s.containsAddress(timeLock.Executors, userAddress) ||
		s.containsAddress(timeLock.Cancellers, userAddress)

	if !hasPermission {
		logger.Error("getOpenzeppelinTimeLockDetail Error: ", ErrUnauthorized)
		return nil, ErrUnauthorized
	}

	// 构建权限信息
	var permissions []string
	if timeLock.CreatorAddress == userAddress {
		permissions = append(permissions, "creator")
	}
	if s.containsAddress(timeLock.Proposers, userAddress) {
		permissions = append(permissions, "proposer")
	}
	if s.containsAddress(timeLock.Executors, userAddress) {
		permissions = append(permissions, "executor")
	}
	if s.containsAddress(timeLock.Cancellers, userAddress) {
		permissions = append(permissions, "canceller")
	}

	// 解析地址列表
	proposersList, _ := s.parseAddressList(timeLock.Proposers)
	executorsList, _ := s.parseAddressList(timeLock.Executors)
	cancellersList, _ := s.parseAddressList(timeLock.Cancellers)

	openzeppelinData := &types.OpenzeppelinTimeLockWithPermission{
		OpenzeppelinTimeLock: *timeLock,
		UserPermissions:      permissions,
		ProposersList:        proposersList,
		ExecutorsList:        executorsList,
		CancellersList:       cancellersList,
	}

	return &types.TimeLockDetailResponse{
		Standard:         types.OpenzeppelinStandard,
		OpenzeppelinData: openzeppelinData,
	}, nil
}

// validateCreateRequest 验证创建timelock合约的请求
func (s *service) validateCreateRequest(req *types.CreateTimeLockRequest) error {
	// 验证合约地址格式
	if !crypto.ValidateEthereumAddress(req.ContractAddress) {
		logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: invalid contract address", ErrInvalidContractParams))
		return fmt.Errorf("%w: invalid contract address", ErrInvalidContractParams)
	}

	// 验证交易hash格式
	if len(req.TxHash) != 66 || !strings.HasPrefix(req.TxHash, "0x") {
		logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: invalid transaction hash", ErrInvalidContractParams))
		return fmt.Errorf("%w: invalid transaction hash", ErrInvalidContractParams)
	}

	// 验证备注
	if err := s.validateRemark(req.Remark); err != nil {
		logger.Error("validateCreateRequest Error: ", err)
		return err
	}

	// 根据标准验证特定参数
	switch req.Standard {
	case types.CompoundStandard:
		if req.Admin == nil {
			logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams)
		}
		if !crypto.ValidateEthereumAddress(*req.Admin) {
			logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: invalid admin address", ErrInvalidContractParams))
			return fmt.Errorf("%w: invalid admin address", ErrInvalidContractParams)
		}
	case types.OpenzeppelinStandard:
		if len(req.Proposers) == 0 {
			logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: proposers are required for openzeppelin standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: proposers are required for openzeppelin standard", ErrInvalidContractParams)
		}
		if len(req.Executors) == 0 {
			logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: executors are required for openzeppelin standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: executors are required for openzeppelin standard", ErrInvalidContractParams)
		}
		if len(req.Cancellers) == 0 {
			logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: cancellers are required for openzeppelin standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: cancellers are required for openzeppelin standard", ErrInvalidContractParams)
		}
		// 验证地址格式
		for _, addr := range req.Proposers {
			if !crypto.ValidateEthereumAddress(addr) {
				logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: invalid proposer address: %s", ErrInvalidContractParams, addr))
				return fmt.Errorf("%w: invalid proposer address: %s", ErrInvalidContractParams, addr)
			}
		}
		for _, addr := range req.Executors {
			if !crypto.ValidateEthereumAddress(addr) {
				logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: invalid executor address: %s", ErrInvalidContractParams, addr))
				return fmt.Errorf("%w: invalid executor address: %s", ErrInvalidContractParams, addr)
			}
		}
		for _, addr := range req.Cancellers {
			if !crypto.ValidateEthereumAddress(addr) {
				logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: invalid canceller address: %s", ErrInvalidContractParams, addr))
				return fmt.Errorf("%w: invalid canceller address: %s", ErrInvalidContractParams, addr)
			}
		}
	default:
		logger.Error("validateCreateRequest Error: ", fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard))
		return fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard)
	}

	return nil
}

// validateImportRequest 验证导入timelock合约的请求
func (s *service) validateImportRequest(req *types.ImportTimeLockRequest) error {
	// 验证合约地址格式
	if !crypto.ValidateEthereumAddress(req.ContractAddress) {
		logger.Error("validateImportRequest Error: ", fmt.Errorf("%w: invalid contract address", ErrInvalidContractParams))
		return fmt.Errorf("%w: invalid contract address", ErrInvalidContractParams)
	}

	// 验证标准
	if req.Standard != types.CompoundStandard && req.Standard != types.OpenzeppelinStandard {
		logger.Error("validateImportRequest Error: ", fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard))
		return fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard)
	}

	// 验证备注
	if err := s.validateRemark(req.Remark); err != nil {
		logger.Error("validateImportRequest Error: ", err)
		return err
	}

	// 根据标准验证特定参数
	switch req.Standard {
	case types.CompoundStandard:
		if req.Admin == nil {
			logger.Error("validateImportRequest Error: ", fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams)
		}
		if !crypto.ValidateEthereumAddress(*req.Admin) {
			logger.Error("validateImportRequest Error: ", fmt.Errorf("%w: invalid admin address", ErrInvalidContractParams))
			return fmt.Errorf("%w: invalid admin address", ErrInvalidContractParams)
		}
		if req.PendingAdmin != nil && !crypto.ValidateEthereumAddress(*req.PendingAdmin) {
			logger.Error("validateImportRequest Error: ", fmt.Errorf("%w: invalid pending admin address", ErrInvalidContractParams))
			return fmt.Errorf("%w: invalid pending admin address", ErrInvalidContractParams)
		}
	case types.OpenzeppelinStandard:
		if len(req.Proposers) == 0 || len(req.Executors) == 0 || len(req.Cancellers) == 0 {
			logger.Error("validateImportRequest Error: ", fmt.Errorf("%w: proposers, executors and cancellers are required for openzeppelin standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: proposers, executors and cancellers are required for openzeppelin standard", ErrInvalidContractParams)
		}
		// 验证地址格式
		for _, addr := range req.Proposers {
			if !crypto.ValidateEthereumAddress(addr) {
				logger.Error("validateImportRequest Error: ", fmt.Errorf("%w: invalid proposer address: %s", ErrInvalidContractParams, addr))
				return fmt.Errorf("%w: invalid proposer address: %s", ErrInvalidContractParams, addr)
			}
		}
		for _, addr := range req.Executors {
			if !crypto.ValidateEthereumAddress(addr) {
				logger.Error("validateImportRequest Error: ", fmt.Errorf("%w: invalid executor address: %s", ErrInvalidContractParams, addr))
				return fmt.Errorf("%w: invalid executor address: %s", ErrInvalidContractParams, addr)
			}
		}
		for _, addr := range req.Cancellers {
			if !crypto.ValidateEthereumAddress(addr) {
				logger.Error("validateImportRequest Error: ", fmt.Errorf("%w: invalid canceller address: %s", ErrInvalidContractParams, addr))
				return fmt.Errorf("%w: invalid canceller address: %s", ErrInvalidContractParams, addr)
			}
		}
	}

	return nil
}

// checkContractExists 检查合约是否存在
func (s *service) checkContractExists(ctx context.Context, standard types.TimeLockStandard, chainID int, contractAddress string) error {
	switch standard {
	case types.CompoundStandard:
		exists, err := s.timeLockRepo.CheckCompoundTimeLockExists(ctx, chainID, contractAddress)
		if err != nil {
			logger.Error("checkContractExists Error: ", err)
			return fmt.Errorf("failed to check compound timelock existence: %w", err)
		}
		if exists {
			return ErrTimeLockExists
		}
	case types.OpenzeppelinStandard:
		exists, err := s.timeLockRepo.CheckOpenzeppelinTimeLockExists(ctx, chainID, contractAddress)
		if err != nil {
			logger.Error("checkContractExists Error: ", err)
			return fmt.Errorf("failed to check openzeppelin timelock existence: %w", err)
		}
		if exists {
			return ErrTimeLockExists
		}
	}
	return nil
}

// validateRemark 验证备注,长度不超过500字符,不允许包含特殊字符
func (s *service) validateRemark(remark string) error {
	// 长度验证
	if len(remark) > 500 {
		return fmt.Errorf("%w: remark too long (max 500 characters)", ErrInvalidRemark)
	}

	// 基本内容安全验证
	if strings.ContainsAny(remark, "<>\"'&") {
		// 这些字符会被HTML转义，所以这里只是警告
		logger.Warn("Remark contains HTML-sensitive characters, will be escaped", "remark", remark)
	}

	// 正则验证：不允许一些特殊字符组合
	maliciousPatterns := []string{
		`(?i)javascript:`,
		`(?i)data:`,
		`(?i)vbscript:`,
		`(?i)<script`,
		`(?i)on\w+\s*=`,
	}

	for _, pattern := range maliciousPatterns {
		if matched, _ := regexp.MatchString(pattern, remark); matched {
			logger.Error("validateRemark Error: ", fmt.Errorf("%w: remark contains potentially malicious content", ErrInvalidRemark))
			return fmt.Errorf("%w: remark contains potentially malicious content", ErrInvalidRemark)
		}
	}

	return nil
}

// containsAddress 检查地址是否在列表中
func (s *service) containsAddress(jsonAddresses, address string) bool {
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
func (s *service) parseAddressList(jsonAddresses string) ([]string, error) {
	var addresses []string
	if err := json.Unmarshal([]byte(jsonAddresses), &addresses); err != nil {
		return nil, err
	}
	return addresses, nil
}
