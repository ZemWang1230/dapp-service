package goldsky

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// FlowRepository Goldsky Flow 数据库操作接口
type FlowRepository interface {
	// Compound Flow 操作
	CreateOrUpdateCompoundFlow(ctx context.Context, flow *types.CompoundTimelockFlowDB) error
	GetCompoundFlowByID(ctx context.Context, flowID string, chainID int, contractAddress string) (*types.CompoundTimelockFlowDB, error)
	UpdateCompoundFlowStatus(ctx context.Context, flowID string, chainID int, contractAddress string, status string) error
	GetCompoundFlowsNeedStatusUpdate(ctx context.Context, now time.Time, limit int) ([]types.CompoundTimelockFlowDB, error)
	GetCompoundFlowsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.CompoundTimelockFlowDB, error)

	// OpenZeppelin Flow 操作
	CreateOrUpdateOpenzeppelinFlow(ctx context.Context, flow *types.OpenzeppelinTimelockFlowDB) error
	GetOpenzeppelinFlowByID(ctx context.Context, flowID string, chainID int, contractAddress string) (*types.OpenzeppelinTimelockFlowDB, error)
	UpdateOpenzeppelinFlowStatus(ctx context.Context, flowID string, chainID int, contractAddress string, status string) error
	GetOpenzeppelinFlowsNeedStatusUpdate(ctx context.Context, now time.Time, limit int) ([]types.OpenzeppelinTimelockFlowDB, error)
	GetOpenzeppelinFlowsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.OpenzeppelinTimelockFlowDB, error)

	// 用户相关查询（用于 API）
	GetUserRelatedFlows(ctx context.Context, userAddress string, status *string, standard *string, offset int, limit int) ([]types.CompoundFlowResponse, int64, error)
	GetUserRelatedFlowsCount(ctx context.Context, userAddress string, standard *string) (*types.FlowStatusCount, error)
}

type flowRepository struct {
	db *gorm.DB
}

// NewFlowRepository 创建新的 Flow Repository
func NewFlowRepository(db *gorm.DB) FlowRepository {
	return &flowRepository{db: db}
}

// CreateOrUpdateCompoundFlow 创建或更新 Compound Flow
func (r *flowRepository) CreateOrUpdateCompoundFlow(ctx context.Context, flow *types.CompoundTimelockFlowDB) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing types.CompoundTimelockFlowDB
		err := tx.Where("flow_id = ? AND chain_id = ? AND LOWER(contract_address) = LOWER(?)",
			flow.FlowID, flow.ChainID, flow.ContractAddress).
			First(&existing).Error

		if err == gorm.ErrRecordNotFound {
			// 创建新记录
			return tx.Create(flow).Error
		} else if err != nil {
			return err
		}

		// 更新现有记录
		flow.ID = existing.ID
		flow.CreatedAt = existing.CreatedAt
		return tx.Save(flow).Error
	})
}

// GetCompoundFlowByID 根据 Flow ID 获取 Compound Flow
func (r *flowRepository) GetCompoundFlowByID(ctx context.Context, flowID string, chainID int, contractAddress string) (*types.CompoundTimelockFlowDB, error) {
	var flow types.CompoundTimelockFlowDB
	err := r.db.WithContext(ctx).
		Where("flow_id = ? AND chain_id = ? AND LOWER(contract_address) = LOWER(?)",
			flowID, chainID, contractAddress).
		First(&flow).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		logger.Error("Failed to get compound flow by ID", err, "flow_id", flowID, "chain_id", chainID)
		return nil, err
	}

	return &flow, nil
}

// UpdateCompoundFlowStatus 更新 Compound Flow 状态
func (r *flowRepository) UpdateCompoundFlowStatus(ctx context.Context, flowID string, chainID int, contractAddress string, status string) error {
	result := r.db.WithContext(ctx).
		Model(&types.CompoundTimelockFlowDB{}).
		Where("flow_id = ? AND chain_id = ? AND LOWER(contract_address) = LOWER(?)",
			flowID, chainID, contractAddress).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		logger.Error("Failed to update compound flow status", result.Error, "flow_id", flowID, "status", status)
		return result.Error
	}

	return nil
}

// GetCompoundFlowsNeedStatusUpdate 获取需要更新状态的 Compound Flows
// 返回：waiting -> ready (eta <= now)，ready -> expired (expired_at <= now)
func (r *flowRepository) GetCompoundFlowsNeedStatusUpdate(ctx context.Context, now time.Time, limit int) ([]types.CompoundTimelockFlowDB, error) {
	var flows []types.CompoundTimelockFlowDB

	query := r.db.WithContext(ctx).Where(
		"(status = ? AND eta IS NOT NULL AND eta <= ?) OR (status = ? AND expired_at IS NOT NULL AND expired_at <= ?)",
		"waiting", now, "ready", now,
	).Order("eta ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&flows).Error
	if err != nil {
		logger.Error("Failed to get compound flows need status update", err)
		return nil, err
	}

	return flows, nil
}

// CreateOrUpdateOpenzeppelinFlow 创建或更新 OpenZeppelin Flow
func (r *flowRepository) CreateOrUpdateOpenzeppelinFlow(ctx context.Context, flow *types.OpenzeppelinTimelockFlowDB) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing types.OpenzeppelinTimelockFlowDB
		err := tx.Where("flow_id = ? AND chain_id = ? AND LOWER(contract_address) = LOWER(?)",
			flow.FlowID, flow.ChainID, flow.ContractAddress).
			First(&existing).Error

		if err == gorm.ErrRecordNotFound {
			// 创建新记录
			return tx.Create(flow).Error
		} else if err != nil {
			return err
		}

		// 更新现有记录
		flow.ID = existing.ID
		flow.CreatedAt = existing.CreatedAt
		return tx.Save(flow).Error
	})
}

// GetOpenzeppelinFlowByID 根据 Flow ID 获取 OpenZeppelin Flow
func (r *flowRepository) GetOpenzeppelinFlowByID(ctx context.Context, flowID string, chainID int, contractAddress string) (*types.OpenzeppelinTimelockFlowDB, error) {
	var flow types.OpenzeppelinTimelockFlowDB
	err := r.db.WithContext(ctx).
		Where("flow_id = ? AND chain_id = ? AND LOWER(contract_address) = LOWER(?)",
			flowID, chainID, contractAddress).
		First(&flow).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		logger.Error("Failed to get openzeppelin flow by ID", err, "flow_id", flowID, "chain_id", chainID)
		return nil, err
	}

	return &flow, nil
}

// UpdateOpenzeppelinFlowStatus 更新 OpenZeppelin Flow 状态
func (r *flowRepository) UpdateOpenzeppelinFlowStatus(ctx context.Context, flowID string, chainID int, contractAddress string, status string) error {
	result := r.db.WithContext(ctx).
		Model(&types.OpenzeppelinTimelockFlowDB{}).
		Where("flow_id = ? AND chain_id = ? AND LOWER(contract_address) = LOWER(?)",
			flowID, chainID, contractAddress).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		logger.Error("Failed to update openzeppelin flow status", result.Error, "flow_id", flowID, "status", status)
		return result.Error
	}

	return nil
}

// GetOpenzeppelinFlowsNeedStatusUpdate 获取需要更新状态的 OpenZeppelin Flows
// 返回：waiting -> ready (eta <= now)
func (r *flowRepository) GetOpenzeppelinFlowsNeedStatusUpdate(ctx context.Context, now time.Time, limit int) ([]types.OpenzeppelinTimelockFlowDB, error) {
	var flows []types.OpenzeppelinTimelockFlowDB

	query := r.db.WithContext(ctx).Where(
		"status = ? AND eta IS NOT NULL AND eta <= ?",
		"waiting", now,
	).Order("eta ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&flows).Error
	if err != nil {
		logger.Error("Failed to get openzeppelin flows need status update", err)
		return nil, err
	}

	return flows, nil
}

// GetUserRelatedFlows 获取用户相关的 Flows（用于 API）
func (r *flowRepository) GetUserRelatedFlows(ctx context.Context, userAddress string, status *string, standard *string, offset int, limit int) ([]types.CompoundFlowResponse, int64, error) {
	// 1. 获取用户相关的合约地址列表
	var contractAddresses []string
	if standard == nil || *standard == "" || *standard == "compound" {
		contractAddresses, err := r.getUserRelatedCompoundContracts(ctx, userAddress)
		if err != nil {
			return nil, 0, err
		}

		if len(contractAddresses) == 0 {
			return []types.CompoundFlowResponse{}, 0, nil
		}
	}

	if standard == nil || *standard == "" || *standard == "openzeppelin" {
		contractAddresses, err := r.getUserRelatedOpenzeppelinContracts(ctx, userAddress)
		if err != nil {
			return nil, 0, err
		}
		if len(contractAddresses) == 0 {
			return []types.CompoundFlowResponse{}, 0, nil
		}
	}

	// 2. 构建查询
	var responses []types.CompoundFlowResponse
	var total int64

	// 根据 standard 决定查询哪个表
	if standard == nil || *standard == "" || *standard == "compound" {
		// 查询 Compound Flows
		compoundFlows, compoundTotal, err := r.queryCompoundFlows(ctx, contractAddresses, status, offset, limit)
		if err != nil {
			return nil, 0, err
		}
		responses = append(responses, compoundFlows...)
		total += compoundTotal
	}

	if standard == nil || *standard == "" || *standard == "openzeppelin" {
		// 查询 OpenZeppelin Flows
		ozFlows, ozTotal, err := r.queryOpenzeppelinFlows(ctx, contractAddresses, status, offset, limit)
		if err != nil {
			return nil, 0, err
		}
		responses = append(responses, ozFlows...)
		total += ozTotal
	}

	return responses, total, nil
}

// getUserRelatedContracts 获取用户相关的合约地址
func (r *flowRepository) getUserRelatedCompoundContracts(ctx context.Context, userAddress string) ([]string, error) {
	var contracts []string
	userAddressLower := strings.ToLower(userAddress)
	err := r.db.WithContext(ctx).
		Model(&struct {
			ChainID         int
			ContractAddress string
		}{}).
		Table("compound_timelocks").
		Where("LOWER(creator_address) = ? OR LOWER(admin) = ? OR LOWER(pending_admin) = ?",
			userAddressLower, userAddressLower, userAddressLower).
		Where("status = ?", "active").
		Pluck("CONCAT(chain_id, ':', contract_address)", &contracts).Error
	if err != nil {
		logger.Error("Failed to get user related compound contracts", err, "user", userAddress)
		return nil, err
	}
	return contracts, nil
}

func (r *flowRepository) getUserRelatedOpenzeppelinContracts(ctx context.Context, userAddress string) ([]string, error) {
	var contracts []string
	userAddressLower := strings.ToLower(userAddress)
	err := r.db.WithContext(ctx).
		Model(&struct {
			ChainID         int
			ContractAddress string
			Proposers       string
			Executors       string
		}{}).
		Table("openzeppelin_timelocks").
		Where("LOWER(creator_address) = ? OR LOWER(admin) = ? OR LOWER(proposers) LIKE ? OR LOWER(executors) LIKE ?",
			userAddressLower, userAddressLower, "%"+userAddressLower+"%", "%"+userAddressLower+"%").
		Where("status = ?", "active").
		Pluck("CONCAT(chain_id, ':', contract_address)", &contracts).Error

	if err != nil {
		logger.Error("Failed to get user related openzeppelin contracts", err, "user", userAddress)
		return nil, err
	}
	return contracts, nil
}

// queryCompoundFlows 查询 Compound Flows
func (r *flowRepository) queryCompoundFlows(ctx context.Context, contractAddresses []string, status *string, offset int, limit int) ([]types.CompoundFlowResponse, int64, error) {
	var flows []types.CompoundTimelockFlowDB
	var total int64

	query := r.db.WithContext(ctx).Model(&types.CompoundTimelockFlowDB{})

	// 构建合约地址过滤条件
	if len(contractAddresses) > 0 {
		var conditions []string
		for _, addr := range contractAddresses {
			conditions = append(conditions, fmt.Sprintf("CONCAT(chain_id, ':', LOWER(contract_address)) = '%s'", strings.ToLower(addr)))
		}
		query = query.Where(strings.Join(conditions, " OR "))
	}

	// 状态过滤
	if status != nil && *status != "" && *status != "all" {
		query = query.Where("status = ?", *status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count compound flows", err)
		return nil, 0, err
	}

	// 分页查询
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&flows).Error; err != nil {
		logger.Error("Failed to query compound flows", err)
		return nil, 0, err
	}

	// 转换为响应格式
	responses := make([]types.CompoundFlowResponse, len(flows))
	for i, flow := range flows {
		responses[i] = r.convertCompoundFlowToResponse(ctx, flow)
	}

	return responses, total, nil
}

// queryOpenzeppelinFlows 查询 OpenZeppelin Flows
func (r *flowRepository) queryOpenzeppelinFlows(ctx context.Context, contractAddresses []string, status *string, offset int, limit int) ([]types.CompoundFlowResponse, int64, error) {
	var flows []types.OpenzeppelinTimelockFlowDB
	var total int64

	query := r.db.WithContext(ctx).Model(&types.OpenzeppelinTimelockFlowDB{})

	// 构建合约地址过滤条件
	if len(contractAddresses) > 0 {
		var conditions []string
		for _, addr := range contractAddresses {
			conditions = append(conditions, fmt.Sprintf("CONCAT(chain_id, ':', LOWER(contract_address)) = '%s'", strings.ToLower(addr)))
		}
		query = query.Where(strings.Join(conditions, " OR "))
	}

	// 状态过滤
	if status != nil && *status != "" && *status != "all" {
		query = query.Where("status = ?", *status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count openzeppelin flows", err)
		return nil, 0, err
	}

	// 分页查询
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&flows).Error; err != nil {
		logger.Error("Failed to query openzeppelin flows", err)
		return nil, 0, err
	}

	// 转换为响应格式
	responses := make([]types.CompoundFlowResponse, len(flows))
	for i, flow := range flows {
		responses[i] = r.convertOpenzeppelinFlowToResponse(ctx, flow)
	}

	return responses, total, nil
}

// convertCompoundFlowToResponse 转换 Compound Flow 为响应格式
func (r *flowRepository) convertCompoundFlowToResponse(ctx context.Context, flow types.CompoundTimelockFlowDB) types.CompoundFlowResponse {
	// 获取合约备注
	var remark string
	r.db.WithContext(ctx).
		Model(&struct{ Remark string }{}).
		Table("compound_timelocks").
		Where("chain_id = ? AND LOWER(contract_address) = LOWER(?)", flow.ChainID, flow.ContractAddress).
		Pluck("remark", &remark)

	callDataHex := hex.EncodeToString(flow.CallData)

	return types.CompoundFlowResponse{
		ID:                flow.ID,
		FlowID:            flow.FlowID,
		TimelockStandard:  flow.TimelockStandard,
		ChainID:           flow.ChainID,
		ContractAddress:   flow.ContractAddress,
		ContractRemark:    remark,
		Status:            flow.Status,
		QueueTxHash:       flow.QueueTxHash,
		ExecuteTxHash:     flow.ExecuteTxHash,
		CancelTxHash:      flow.CancelTxHash,
		InitiatorAddress:  flow.InitiatorAddress,
		TargetAddress:     flow.TargetAddress,
		FunctionSignature: flow.FunctionSignature,
		CallDataHex:       &callDataHex,
		Value:             flow.Value,
		Eta:               flow.Eta,
		ExpiredAt:         flow.ExpiredAt,
		ExecutedAt:        flow.ExecutedAt,
		CancelledAt:       flow.CancelledAt,
		CreatedAt:         flow.CreatedAt,
		UpdatedAt:         flow.UpdatedAt,
	}
}

// convertOpenzeppelinFlowToResponse 转换 OpenZeppelin Flow 为响应格式
func (r *flowRepository) convertOpenzeppelinFlowToResponse(ctx context.Context, flow types.OpenzeppelinTimelockFlowDB) types.CompoundFlowResponse {
	// 获取合约备注
	var remark string
	r.db.WithContext(ctx).
		Model(&struct{ Remark string }{}).
		Table("openzeppelin_timelocks").
		Where("chain_id = ? AND LOWER(contract_address) = LOWER(?)", flow.ChainID, flow.ContractAddress).
		Pluck("remark", &remark)

	callDataHex := hex.EncodeToString(flow.CallData)

	return types.CompoundFlowResponse{
		ID:               flow.ID,
		FlowID:           flow.FlowID,
		TimelockStandard: flow.TimelockStandard,
		ChainID:          flow.ChainID,
		ContractAddress:  flow.ContractAddress,
		ContractRemark:   remark,
		Status:           flow.Status,
		QueueTxHash:      flow.ScheduleTxHash,
		ExecuteTxHash:    flow.ExecuteTxHash,
		CancelTxHash:     flow.CancelTxHash,
		InitiatorAddress: flow.InitiatorAddress,
		TargetAddress:    flow.TargetAddress,
		CallDataHex:      &callDataHex,
		Value:            flow.Value,
		Eta:              flow.Eta,
		ExpiredAt:        nil, // OpenZeppelin 没有 expired
		ExecutedAt:       flow.ExecutedAt,
		CancelledAt:      flow.CancelledAt,
		CreatedAt:        flow.CreatedAt,
		UpdatedAt:        flow.UpdatedAt,
	}
}

// GetUserRelatedFlowsCount 获取用户相关的 Flows 数量统计
func (r *flowRepository) GetUserRelatedFlowsCount(ctx context.Context, userAddress string, standard *string) (*types.FlowStatusCount, error) {
	// 1. 获取用户相关的合约地址列表
	var contractAddresses []string

	if standard == nil || *standard == "" || *standard == "compound" {
		compoundAddresses, err := r.getUserRelatedCompoundContracts(ctx, userAddress)
		if err != nil {
			return nil, err
		}
		contractAddresses = append(contractAddresses, compoundAddresses...)
	}

	if standard == nil || *standard == "" || *standard == "openzeppelin" {
		ozAddresses, err := r.getUserRelatedOpenzeppelinContracts(ctx, userAddress)
		if err != nil {
			return nil, err
		}
		contractAddresses = append(contractAddresses, ozAddresses...)
	}

	if len(contractAddresses) == 0 {
		return &types.FlowStatusCount{}, nil
	}

	count := &types.FlowStatusCount{}

	// 2. 统计 Compound Flows
	if standard == nil || *standard == "" || *standard == "compound" {
		compoundCount, err := r.countCompoundFlows(ctx, contractAddresses)
		if err != nil {
			return nil, err
		}
		count.Count += compoundCount.Count
		count.Waiting += compoundCount.Waiting
		count.Ready += compoundCount.Ready
		count.Executed += compoundCount.Executed
		count.Cancelled += compoundCount.Cancelled
		count.Expired += compoundCount.Expired
	}

	// 3. 统计 OpenZeppelin Flows
	if standard == nil || *standard == "" || *standard == "openzeppelin" {
		ozCount, err := r.countOpenzeppelinFlows(ctx, contractAddresses)
		if err != nil {
			return nil, err
		}
		count.Count += ozCount.Count
		count.Waiting += ozCount.Waiting
		count.Ready += ozCount.Ready
		count.Executed += ozCount.Executed
		count.Cancelled += ozCount.Cancelled
		// OpenZeppelin 没有 expired 状态
	}

	return count, nil
}

// GetCompoundFlowsByContract 获取特定合约的所有Compound Flows
func (r *flowRepository) GetCompoundFlowsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.CompoundTimelockFlowDB, error) {
	var flows []types.CompoundTimelockFlowDB
	err := r.db.WithContext(ctx).Where("chain_id = ? AND LOWER(contract_address) = LOWER(?)", chainID, contractAddress).
		Order("eta ASC"). // 按执行时间排序
		Find(&flows).Error
	return flows, err
}

// GetOpenzeppelinFlowsByContract 获取特定合约的所有OpenZeppelin Flows
func (r *flowRepository) GetOpenzeppelinFlowsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.OpenzeppelinTimelockFlowDB, error) {
	var flows []types.OpenzeppelinTimelockFlowDB
	err := r.db.WithContext(ctx).Where("chain_id = ? AND LOWER(contract_address) = LOWER(?)", chainID, contractAddress).
		Order("eta ASC"). // 按执行时间排序
		Find(&flows).Error
	return flows, err
}

// countCompoundFlows 统计 Compound Flows
func (r *flowRepository) countCompoundFlows(ctx context.Context, contractAddresses []string) (*types.FlowStatusCount, error) {
	count := &types.FlowStatusCount{}

	query := r.db.WithContext(ctx).Model(&types.CompoundTimelockFlowDB{})

	// 构建合约地址过滤条件
	if len(contractAddresses) > 0 {
		var conditions []string
		for _, addr := range contractAddresses {
			conditions = append(conditions, fmt.Sprintf("CONCAT(chain_id, ':', LOWER(contract_address)) = '%s'", strings.ToLower(addr)))
		}
		query = query.Where(strings.Join(conditions, " OR "))
	}

	// 总数
	if err := query.Count(&count.Count).Error; err != nil {
		return nil, err
	}

	// 按状态统计
	rows, err := query.Select("status, COUNT(*) as count").Group("status").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var cnt int64
		if err := rows.Scan(&status, &cnt); err != nil {
			return nil, err
		}

		switch status {
		case "waiting":
			count.Waiting = cnt
		case "ready":
			count.Ready = cnt
		case "executed":
			count.Executed = cnt
		case "cancelled":
			count.Cancelled = cnt
		case "expired":
			count.Expired = cnt
		}
	}

	return count, nil
}

// countOpenzeppelinFlows 统计 OpenZeppelin Flows
func (r *flowRepository) countOpenzeppelinFlows(ctx context.Context, contractAddresses []string) (*types.FlowStatusCount, error) {
	count := &types.FlowStatusCount{}

	query := r.db.WithContext(ctx).Model(&types.OpenzeppelinTimelockFlowDB{})

	// 构建合约地址过滤条件
	if len(contractAddresses) > 0 {
		var conditions []string
		for _, addr := range contractAddresses {
			conditions = append(conditions, fmt.Sprintf("CONCAT(chain_id, ':', LOWER(contract_address)) = '%s'", strings.ToLower(addr)))
		}
		query = query.Where(strings.Join(conditions, " OR "))
	}

	// 总数
	if err := query.Count(&count.Count).Error; err != nil {
		return nil, err
	}

	// 按状态统计
	rows, err := query.Select("status, COUNT(*) as count").Group("status").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var cnt int64
		if err := rows.Scan(&status, &cnt); err != nil {
			return nil, err
		}

		switch status {
		case "waiting":
			count.Waiting = cnt
		case "ready":
			count.Ready = cnt
		case "executed":
			count.Executed = cnt
		case "cancelled":
			count.Cancelled = cnt
		}
	}

	return count, nil
}
