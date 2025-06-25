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
)

// Service timelock服务接口
type Service interface {
	// 检查timelock状态
	CheckTimeLockStatus(ctx context.Context, walletAddress string) (*types.CheckTimeLockStatusResponse, error)

	// 创建timelock合约
	CreateTimeLock(ctx context.Context, walletAddress string, req *types.CreateTimeLockRequest) (*types.TimeLock, error)

	// 导入timelock合约
	ImportTimeLock(ctx context.Context, walletAddress string, req *types.ImportTimeLockRequest) (*types.TimeLock, error)

	// 获取timelock列表
	GetTimeLockList(ctx context.Context, walletAddress string, req *types.GetTimeLockListRequest) (*types.GetTimeLockListResponse, error)

	// 获取timelock详情
	GetTimeLockDetail(ctx context.Context, walletAddress string, id int64) (*types.TimeLockDetailResponse, error)

	// 更新timelock
	UpdateTimeLock(ctx context.Context, walletAddress string, req *types.UpdateTimeLockRequest) error

	// 删除timelock
	DeleteTimeLock(ctx context.Context, walletAddress string, req *types.DeleteTimeLockRequest) error
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

// CheckTimeLockStatus 检查用户的timelock状态
func (s *service) CheckTimeLockStatus(ctx context.Context, walletAddress string) (*types.CheckTimeLockStatusResponse, error) {
	logger.Info("CheckTimeLockStatus: ", "wallet_address", walletAddress)

	// 标准化钱包地址
	normalizedAddress := crypto.NormalizeAddress(walletAddress)

	// 获取用户的所有timelock合约
	timeLocks, err := s.timeLockRepo.GetTimeLocksByWallet(ctx, normalizedAddress)
	if err != nil {
		logger.Error("CheckTimeLockStatus Error: ", err, "wallet_address", normalizedAddress)
		return nil, fmt.Errorf("failed to get timelocks: %w", err)
	}

	response := &types.CheckTimeLockStatusResponse{
		HasTimeLocks: len(timeLocks) > 0,
		TimeLocks:    timeLocks,
	}

	logger.Info("CheckTimeLockStatus: ", "wallet_address", normalizedAddress, "has_timelocks", response.HasTimeLocks, "count", len(timeLocks))
	return response, nil
}

// CreateTimeLock 创建timelock合约记录
func (s *service) CreateTimeLock(ctx context.Context, walletAddress string, req *types.CreateTimeLockRequest) (*types.TimeLock, error) {
	logger.Info("CreateTimeLock: ", "wallet_address", walletAddress, "contract_address", req.ContractAddress, "standard", req.Standard)

	// 1. 标准化钱包地址和合约地址
	normalizedAddress := crypto.NormalizeAddress(walletAddress)
	normalizedContract := crypto.NormalizeAddress(req.ContractAddress)
	normalizedCreator := crypto.NormalizeAddress(req.CreatorAddress)

	// 2. 验证输入参数
	if err := s.validateCreateRequest(req); err != nil {
		logger.Error("CreateTimeLock Validation Error: ", err, "wallet_address", normalizedAddress)
		return nil, err
	}

	// 3. 检查合约是否已存在
	exists, err := s.timeLockRepo.CheckTimeLockExists(ctx, normalizedAddress, req.ChainID, normalizedContract)
	if err != nil {
		logger.Error("CreateTimeLock Check Exists Error: ", err, "wallet_address", normalizedAddress)
		return nil, fmt.Errorf("failed to check timelock existence: %w", err)
	}
	if exists {
		logger.Error("CreateTimeLock Error: ", ErrTimeLockExists, "wallet_address", normalizedAddress, "contract_address", normalizedContract)
		return nil, ErrTimeLockExists
	}

	// 4. 构建timelock实体
	timeLock := &types.TimeLock{
		WalletAddress:   normalizedAddress,
		ChainID:         req.ChainID,
		ChainName:       req.ChainName,
		ContractAddress: normalizedContract,
		Standard:        req.Standard,
		CreatorAddress:  &normalizedCreator,
		TxHash:          &req.TxHash,
		MinDelay:        req.MinDelay,
		Remark:          html.EscapeString(strings.TrimSpace(req.Remark)),
		Status:          types.TimeLockActive,
	}

	// 5. 处理不同标准的特定参数
	if err := s.processStandardSpecificParams(timeLock, req); err != nil {
		logger.Error("CreateTimeLock Process Params Error: ", err, "wallet_address", normalizedAddress)
		return nil, err
	}

	// 6. 创建记录
	if err := s.timeLockRepo.CreateTimeLock(ctx, timeLock); err != nil {
		logger.Error("CreateTimeLock Repository Error: ", err, "wallet_address", normalizedAddress)
		return nil, fmt.Errorf("failed to create timelock: %w", err)
	}

	logger.Info("CreateTimeLock Success: ", "timelock_id", timeLock.ID, "wallet_address", normalizedAddress, "contract_address", normalizedContract)
	return timeLock, nil
}

// ImportTimeLock 导入timelock合约
func (s *service) ImportTimeLock(ctx context.Context, walletAddress string, req *types.ImportTimeLockRequest) (*types.TimeLock, error) {
	logger.Info("ImportTimeLock: ", "wallet_address", walletAddress, "contract_address", req.ContractAddress, "standard", req.Standard)

	// 1. 标准化地址
	normalizedAddress := crypto.NormalizeAddress(walletAddress)
	normalizedContract := crypto.NormalizeAddress(req.ContractAddress)

	// 2. 验证输入参数
	if err := s.validateImportRequest(req); err != nil {
		logger.Error("ImportTimeLock Validation Error: ", err, "wallet_address", normalizedAddress)
		return nil, err
	}

	// 3. 检查合约是否已存在
	exists, err := s.timeLockRepo.CheckTimeLockExists(ctx, normalizedAddress, req.ChainID, normalizedContract)
	if err != nil {
		logger.Error("ImportTimeLock Check Exists Error: ", err, "wallet_address", normalizedAddress)
		return nil, fmt.Errorf("failed to check timelock existence: %w", err)
	}
	if exists {
		logger.Error("ImportTimeLock Error: ", ErrTimeLockExists, "wallet_address", normalizedAddress, "contract_address", normalizedContract)
		return nil, ErrTimeLockExists
	}

	// 4. 验证合约是否为有效的timelock合约
	if err := s.validateTimeLockContract(req.ContractAddress, req.Standard, req.ABI); err != nil {
		logger.Error("ImportTimeLock Contract Validation Error: ", err, "wallet_address", normalizedAddress)
		return nil, err
	}

	// 5. 构建timelock实体
	timeLock := &types.TimeLock{
		WalletAddress:   normalizedAddress,
		ChainID:         req.ChainID,
		ChainName:       req.ChainName,
		ContractAddress: normalizedContract,
		Standard:        req.Standard,
		Remark:          html.EscapeString(strings.TrimSpace(req.Remark)),
		Status:          types.TimeLockActive,
	}

	// 6. 创建记录
	if err := s.timeLockRepo.CreateTimeLock(ctx, timeLock); err != nil {
		logger.Error("ImportTimeLock Repository Error: ", err, "wallet_address", normalizedAddress)
		return nil, fmt.Errorf("failed to import timelock: %w", err)
	}

	logger.Info("ImportTimeLock Success: ", "timelock_id", timeLock.ID, "wallet_address", normalizedAddress, "contract_address", normalizedContract)
	return timeLock, nil
}

// GetTimeLockList 获取timelock列表
func (s *service) GetTimeLockList(ctx context.Context, walletAddress string, req *types.GetTimeLockListRequest) (*types.GetTimeLockListResponse, error) {
	logger.Info("GetTimeLockList: ", "wallet_address", walletAddress, "page", req.Page, "page_size", req.PageSize)

	// 标准化钱包地址
	normalizedAddress := crypto.NormalizeAddress(walletAddress)

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 获取列表
	timeLocks, total, err := s.timeLockRepo.GetTimeLockList(ctx, normalizedAddress, req)
	if err != nil {
		logger.Error("GetTimeLockList Error: ", err, "wallet_address", normalizedAddress)
		return nil, fmt.Errorf("failed to get timelock list: %w", err)
	}

	response := &types.GetTimeLockListResponse{
		List:     timeLocks,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	logger.Info("GetTimeLockList Success: ", "wallet_address", normalizedAddress, "total", total, "current_page_count", len(timeLocks))
	return response, nil
}

// GetTimeLockDetail 获取timelock详情
func (s *service) GetTimeLockDetail(ctx context.Context, walletAddress string, id int64) (*types.TimeLockDetailResponse, error) {
	logger.Info("GetTimeLockDetail: ", "wallet_address", walletAddress, "timelock_id", id)

	// 标准化钱包地址
	normalizedAddress := crypto.NormalizeAddress(walletAddress)

	// 验证所有权
	isOwner, err := s.timeLockRepo.ValidateOwnership(ctx, id, normalizedAddress)
	if err != nil {
		logger.Error("GetTimeLockDetail Validate Ownership Error: ", err, "timelock_id", id, "wallet_address", normalizedAddress)
		return nil, fmt.Errorf("failed to validate ownership: %w", err)
	}
	if !isOwner {
		logger.Error("GetTimeLockDetail Error: ", ErrUnauthorized, "timelock_id", id, "wallet_address", normalizedAddress)
		return nil, ErrUnauthorized
	}

	// 获取timelock
	timeLock, err := s.timeLockRepo.GetTimeLockByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("GetTimeLockDetail Error: ", ErrTimeLockNotFound, "timelock_id", id)
			return nil, ErrTimeLockNotFound
		}
		logger.Error("GetTimeLockDetail Error: ", err, "timelock_id", id)
		return nil, fmt.Errorf("failed to get timelock: %w", err)
	}

	// 构建详情响应
	response := &types.TimeLockDetailResponse{
		TimeLock: *timeLock,
	}

	// 解析JSON字段
	if timeLock.Proposers != nil && *timeLock.Proposers != "" {
		var proposers []string
		if err := json.Unmarshal([]byte(*timeLock.Proposers), &proposers); err == nil {
			response.ProposersList = proposers
		}
	}

	if timeLock.Executors != nil && *timeLock.Executors != "" {
		var executors []string
		if err := json.Unmarshal([]byte(*timeLock.Executors), &executors); err == nil {
			response.ExecutorsList = executors
		}
	}

	logger.Info("GetTimeLockDetail Success: ", "timelock_id", id, "wallet_address", normalizedAddress)
	return response, nil
}

// UpdateTimeLock 更新timelock
func (s *service) UpdateTimeLock(ctx context.Context, walletAddress string, req *types.UpdateTimeLockRequest) error {
	logger.Info("UpdateTimeLock: ", "wallet_address", walletAddress, "timelock_id", req.ID)

	// 标准化钱包地址
	normalizedAddress := crypto.NormalizeAddress(walletAddress)

	// 验证所有权
	isOwner, err := s.timeLockRepo.ValidateOwnership(ctx, req.ID, normalizedAddress)
	if err != nil {
		logger.Error("UpdateTimeLock Validate Ownership Error: ", err, "timelock_id", req.ID, "wallet_address", normalizedAddress)
		return fmt.Errorf("failed to validate ownership: %w", err)
	}
	if !isOwner {
		logger.Error("UpdateTimeLock Error: ", ErrUnauthorized, "timelock_id", req.ID, "wallet_address", normalizedAddress)
		return ErrUnauthorized
	}

	// 验证备注安全性
	if err := s.validateRemark(req.Remark); err != nil {
		logger.Error("UpdateTimeLock Remark Validation Error: ", err, "timelock_id", req.ID, "wallet_address", normalizedAddress)
		return err
	}

	// 更新备注
	sanitizedRemark := html.EscapeString(strings.TrimSpace(req.Remark))
	if err := s.timeLockRepo.UpdateTimeLockRemark(ctx, req.ID, sanitizedRemark); err != nil {
		logger.Error("UpdateTimeLock Repository Error: ", err, "timelock_id", req.ID, "wallet_address", normalizedAddress)
		return fmt.Errorf("failed to update timelock: %w", err)
	}

	logger.Info("UpdateTimeLock Success: ", "timelock_id", req.ID, "wallet_address", normalizedAddress)
	return nil
}

// DeleteTimeLock 删除timelock
func (s *service) DeleteTimeLock(ctx context.Context, walletAddress string, req *types.DeleteTimeLockRequest) error {
	logger.Info("DeleteTimeLock: ", "wallet_address", walletAddress, "timelock_id", req.ID)

	// 标准化钱包地址
	normalizedAddress := crypto.NormalizeAddress(walletAddress)

	// 验证所有权
	isOwner, err := s.timeLockRepo.ValidateOwnership(ctx, req.ID, normalizedAddress)
	if err != nil {
		logger.Error("DeleteTimeLock Validate Ownership Error: ", err, "timelock_id", req.ID, "wallet_address", normalizedAddress)
		return fmt.Errorf("failed to validate ownership: %w", err)
	}
	if !isOwner {
		logger.Error("DeleteTimeLock Error: ", ErrUnauthorized, "timelock_id", req.ID, "wallet_address", normalizedAddress)
		return ErrUnauthorized
	}

	// 删除timelock（软删除）
	if err := s.timeLockRepo.DeleteTimeLock(ctx, req.ID); err != nil {
		logger.Error("DeleteTimeLock Repository Error: ", err, "timelock_id", req.ID, "wallet_address", normalizedAddress)
		return fmt.Errorf("failed to delete timelock: %w", err)
	}

	logger.Info("DeleteTimeLock Success: ", "timelock_id", req.ID, "wallet_address", normalizedAddress)
	return nil
}

// validateCreateRequest 验证创建请求
func (s *service) validateCreateRequest(req *types.CreateTimeLockRequest) error {
	// 验证合约地址格式
	if !crypto.ValidateEthereumAddress(req.ContractAddress) {
		return fmt.Errorf("%w: invalid contract address", ErrInvalidContractParams)
	}

	// 验证创建者地址格式
	if !crypto.ValidateEthereumAddress(req.CreatorAddress) {
		return fmt.Errorf("%w: invalid creator address", ErrInvalidContractParams)
	}

	// 验证交易hash格式
	if len(req.TxHash) != 66 || !strings.HasPrefix(req.TxHash, "0x") {
		return fmt.Errorf("%w: invalid transaction hash", ErrInvalidContractParams)
	}

	// 验证备注
	if err := s.validateRemark(req.Remark); err != nil {
		return err
	}

	// 根据标准验证特定参数
	switch req.Standard {
	case types.CompoundStandard:
		if req.MinDelay == nil {
			logger.Error("CreateTimeLock Error: ", fmt.Errorf("%w: min_delay is required for compound standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: min_delay is required for compound standard", ErrInvalidContractParams)
		}
		if req.Admin == nil {
			logger.Error("CreateTimeLock Error: ", fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: admin is required for compound standard", ErrInvalidContractParams)
		}
		if !crypto.ValidateEthereumAddress(*req.Admin) {
			logger.Error("CreateTimeLock Error: ", fmt.Errorf("%w: invalid admin address", ErrInvalidContractParams))
			return fmt.Errorf("%w: invalid admin address", ErrInvalidContractParams)
		}
	case types.OpenzeppelinStandard:
		if req.MinDelay == nil {
			logger.Error("CreateTimeLock Error: ", fmt.Errorf("%w: min_delay is required for openzeppelin standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: min_delay is required for openzeppelin standard", ErrInvalidContractParams)
		}
		if len(req.Proposers) == 0 {
			logger.Error("CreateTimeLock Error: ", fmt.Errorf("%w: proposers are required for openzeppelin standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: proposers are required for openzeppelin standard", ErrInvalidContractParams)
		}
		if len(req.Executors) == 0 {
			logger.Error("CreateTimeLock Error: ", fmt.Errorf("%w: executors are required for openzeppelin standard", ErrInvalidContractParams))
			return fmt.Errorf("%w: executors are required for openzeppelin standard", ErrInvalidContractParams)
		}
		// 验证地址格式
		for _, addr := range req.Proposers {
			if !crypto.ValidateEthereumAddress(addr) {
				logger.Error("CreateTimeLock Error: ", fmt.Errorf("%w: invalid proposer address: %s", ErrInvalidContractParams, addr))
				return fmt.Errorf("%w: invalid proposer address: %s", ErrInvalidContractParams, addr)
			}
		}
		for _, addr := range req.Executors {
			if !crypto.ValidateEthereumAddress(addr) {
				logger.Error("CreateTimeLock Error: ", fmt.Errorf("%w: invalid executor address: %s", ErrInvalidContractParams, addr))
				return fmt.Errorf("%w: invalid executor address: %s", ErrInvalidContractParams, addr)
			}
		}
	default:
		logger.Error("CreateTimeLock Error: ", fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard))
		return fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard)
	}

	return nil
}

// validateImportRequest 验证导入请求
func (s *service) validateImportRequest(req *types.ImportTimeLockRequest) error {
	// 验证合约地址格式
	if !crypto.ValidateEthereumAddress(req.ContractAddress) {
		logger.Error("ImportTimeLock Error: ", fmt.Errorf("%w: invalid contract address", ErrInvalidContractParams))
		return fmt.Errorf("%w: invalid contract address", ErrInvalidContractParams)
	}

	// 验证标准
	if req.Standard != types.CompoundStandard && req.Standard != types.OpenzeppelinStandard {
		logger.Error("ImportTimeLock Error: ", fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard))
		return fmt.Errorf("%w: %s", ErrInvalidStandard, req.Standard)
	}

	// 验证ABI
	if strings.TrimSpace(req.ABI) == "" {
		logger.Error("ImportTimeLock Error: ", fmt.Errorf("%w: ABI is required", ErrInvalidContractParams))
		return fmt.Errorf("%w: ABI is required", ErrInvalidContractParams)
	}

	// 验证备注
	if err := s.validateRemark(req.Remark); err != nil {
		logger.Error("ImportTimeLock Error: ", err)
		return err
	}

	return nil
}

// validateTimeLockContract 验证timelock合约
func (s *service) validateTimeLockContract(contractAddress string, standard types.TimeLockStandard, abi string) error {
	// 这里应该调用区块链节点验证合约
	// 由于示例目的，这里简化为基本验证
	logger.Info("ValidateTimeLockContract: ", "contract_address", contractAddress, "standard", standard)

	// 验证ABI是否包含timelock相关的函数
	var requiredFunctions []string
	switch standard {
	case types.CompoundStandard:
		requiredFunctions = []string{"delay", "admin", "queueTransaction", "executeTransaction", "cancelTransaction"}
	case types.OpenzeppelinStandard:
		requiredFunctions = []string{"getMinDelay", "schedule", "execute", "cancel"}
	}

	// 检查ABI中是否包含必要的函数
	abiLower := strings.ToLower(abi)
	for _, fn := range requiredFunctions {
		if !strings.Contains(abiLower, strings.ToLower(fn)) {
			logger.Error("ValidateTimeLockContract Error: missing function", errors.New("missing required function"), "function", fn, "standard", standard)
			return fmt.Errorf("%w: missing required function %s for %s standard", ErrInvalidContract, fn, standard)
		}
	}

	logger.Info("ValidateTimeLockContract Success: ", "contract_address", contractAddress, "standard", standard)
	return nil
}

// processStandardSpecificParams 处理不同标准的特定参数
func (s *service) processStandardSpecificParams(timeLock *types.TimeLock, req *types.CreateTimeLockRequest) error {
	switch req.Standard {
	case types.CompoundStandard:
		if req.Admin != nil {
			normalizedAdmin := crypto.NormalizeAddress(*req.Admin)
			timeLock.Admin = &normalizedAdmin
		}
	case types.OpenzeppelinStandard:
		// 序列化proposers
		if len(req.Proposers) > 0 {
			normalizedProposers := make([]string, len(req.Proposers))
			for i, addr := range req.Proposers {
				normalizedProposers[i] = crypto.NormalizeAddress(addr)
			}
			proposersJSON, err := json.Marshal(normalizedProposers)
			if err != nil {
				logger.Error("ProcessStandardSpecificParams Error: ", err)
				return fmt.Errorf("failed to marshal proposers: %w", err)
			}
			proposersStr := string(proposersJSON)
			timeLock.Proposers = &proposersStr
		}

		// 序列化executors
		if len(req.Executors) > 0 {
			normalizedExecutors := make([]string, len(req.Executors))
			for i, addr := range req.Executors {
				normalizedExecutors[i] = crypto.NormalizeAddress(addr)
			}
			executorsJSON, err := json.Marshal(normalizedExecutors)
			if err != nil {
				logger.Error("ProcessStandardSpecificParams Error: ", err)
				return fmt.Errorf("failed to marshal executors: %w", err)
			}
			executorsStr := string(executorsJSON)
			timeLock.Executors = &executorsStr
		}
	}

	return nil
}

// validateRemark 验证备注安全性
func (s *service) validateRemark(remark string) error {
	// 检查长度
	if len(remark) > 500 {
		logger.Error("ValidateRemark Error: remark too long (max 500 characters)", errors.New("remark too long (max 500 characters)"))
		return fmt.Errorf("%w: remark too long (max 500 characters)", ErrInvalidRemark)
	}

	// 检查是否包含恶意内容
	suspicious := []string{"<script", "</script", "javascript:", "data:", "vbscript:", "onload=", "onerror="}
	remarkLower := strings.ToLower(remark)
	for _, pattern := range suspicious {
		if strings.Contains(remarkLower, pattern) {
			logger.Error("ValidateRemark Error: contains suspicious content", errors.New("contains suspicious content"), "pattern", pattern)
			return fmt.Errorf("%w: contains suspicious content", ErrInvalidRemark)
		}
	}

	// 使用正则表达式检查SQL注入模式
	sqlPatterns := []string{
		`(?i)(union\s+select)`,
		`(?i)(drop\s+table)`,
		`(?i)(delete\s+from)`,
		`(?i)(insert\s+into)`,
		`(?i)(update\s+.*set)`,
		`(?i)(exec\s*\()`,
		`(?i)(execute\s*\()`,
	}

	for _, pattern := range sqlPatterns {
		matched, err := regexp.MatchString(pattern, remark)
		if err != nil {
			logger.Error("ValidateRemark Regex Error: ", err, "pattern", pattern)
			continue
		}
		if matched {
			logger.Error("ValidateRemark Error: contains SQL injection pattern", errors.New("contains SQL injection pattern"), "pattern", pattern)
			return fmt.Errorf("%w: contains SQL injection pattern", ErrInvalidRemark)
		}
	}

	return nil
}
