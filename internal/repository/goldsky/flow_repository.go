package goldsky

import (
	"context"
	"encoding/hex"
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
	GetUserRelatedCompoundFlows(ctx context.Context, userAddress string, status *string, standard *string, offset int, limit int) ([]types.CompoundFlowResponse, int64, error)
	GetUserRelatedCompoundFlowsCount(ctx context.Context, userAddress string, standard *string) (*types.FlowStatusCount, error)
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

// GetUserRelatedCompoundFlows 获取用户相关的 Compound Flows（用于 API）
func (r *flowRepository) GetUserRelatedCompoundFlows(ctx context.Context, userAddress string, status *string, standard *string, offset int, limit int) ([]types.CompoundFlowResponse, int64, error) {
	normalizedUserAddress := strings.ToLower(userAddress)

	var responses []types.CompoundFlowResponse
	var total int64

	// 只处理 compound 类型的请求
	if standard != nil && *standard == "compound" {
		// 查询 Compound Flows
		compoundFlows, compoundTotal, err := r.queryCompoundFlowsWithPermission(ctx, normalizedUserAddress, status, offset, limit)
		if err != nil {
			return nil, 0, err
		}
		responses = append(responses, compoundFlows...)
		total += compoundTotal
	}

	return responses, total, nil
}

// queryCompoundFlowsWithPermission 使用子查询方式查询用户有权限的 Compound Flows
func (r *flowRepository) queryCompoundFlowsWithPermission(ctx context.Context, normalizedUserAddress string, status *string, offset int, limit int) ([]types.CompoundFlowResponse, int64, error) {
	var flows []types.CompoundTimelockFlowDB
	var total int64

	// 构建WHERE条件，包含两种情况：
	// 1. initiator_address是该地址
	// 2. 该flow的合约中，该地址是管理员（admin、pending_admin或creator）
	whereConditions := []string{}
	args := []interface{}{}

	// 第一种情况：initiator_address是该地址
	whereConditions = append(whereConditions, "LOWER(initiator_address) = ?")
	args = append(args, normalizedUserAddress)

	// 第二种情况：根据合约权限查询（admin、pending_admin或creator）
	compoundCondition := `(chain_id, contract_address) IN (
		SELECT chain_id, contract_address FROM compound_timelocks 
		WHERE (LOWER(admin) = ? OR LOWER(pending_admin) = ? OR LOWER(creator_address) = ?)
		AND status = ?
	)`
	whereConditions = append(whereConditions, compoundCondition)
	args = append(args, normalizedUserAddress, normalizedUserAddress, normalizedUserAddress, "active")

	// 组合所有条件
	finalWhere := "(" + strings.Join(whereConditions, " OR ") + ")"

	// 添加状态过滤
	if status != nil && *status != "" && *status != "all" {
		finalWhere += " AND status = ?"
		args = append(args, *status)
	}

	// 计算总数
	if err := r.db.WithContext(ctx).Model(&types.CompoundTimelockFlowDB{}).
		Where(finalWhere, args...).
		Count(&total).Error; err != nil {
		logger.Error("Failed to count compound flows with permission", err, "user", normalizedUserAddress)
		return nil, 0, err
	}

	// 分页查询
	if err := r.db.WithContext(ctx).
		Where(finalWhere, args...).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&flows).Error; err != nil {
		logger.Error("Failed to query compound flows with permission", err, "user", normalizedUserAddress)
		return nil, 0, err
	}

	// 转换为响应格式
	responses := make([]types.CompoundFlowResponse, len(flows))
	for i, flow := range flows {
		responses[i] = r.convertCompoundFlowToResponse(ctx, flow)
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

// GetUserRelatedCompoundFlowsCount 获取用户相关的 Compound Flows 数量统计
func (r *flowRepository) GetUserRelatedCompoundFlowsCount(ctx context.Context, userAddress string, standard *string) (*types.FlowStatusCount, error) {
	normalizedUserAddress := strings.ToLower(userAddress)
	count := &types.FlowStatusCount{}

	// 统计 Compound Flows
	if standard != nil && *standard == "compound" {
		compoundCount, err := r.countCompoundFlowsWithPermission(ctx, normalizedUserAddress)
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

// countCompoundFlowsWithPermission 统计用户有权限的 Compound Flows
func (r *flowRepository) countCompoundFlowsWithPermission(ctx context.Context, normalizedUserAddress string) (*types.FlowStatusCount, error) {
	count := &types.FlowStatusCount{}

	// 构建WHERE条件（与查询逻辑一致）
	whereConditions := []string{}
	args := []interface{}{}

	// 第一种情况：initiator_address是该地址
	whereConditions = append(whereConditions, "LOWER(initiator_address) = ?")
	args = append(args, normalizedUserAddress)

	// 第二种情况：根据合约权限查询
	compoundCondition := `(chain_id, contract_address) IN (
		SELECT chain_id, contract_address FROM compound_timelocks 
		WHERE (LOWER(admin) = ? OR LOWER(pending_admin) = ? OR LOWER(creator_address) = ?)
		AND status = ?
	)`
	whereConditions = append(whereConditions, compoundCondition)
	args = append(args, normalizedUserAddress, normalizedUserAddress, normalizedUserAddress, "active")

	finalWhere := "(" + strings.Join(whereConditions, " OR ") + ")"

	// 总数
	if err := r.db.WithContext(ctx).Model(&types.CompoundTimelockFlowDB{}).
		Where(finalWhere, args...).
		Count(&count.Count).Error; err != nil {
		return nil, err
	}

	// 按状态统计
	rows, err := r.db.WithContext(ctx).Model(&types.CompoundTimelockFlowDB{}).
		Select("status, COUNT(*) as count").
		Where(finalWhere, args...).
		Group("status").
		Rows()
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
