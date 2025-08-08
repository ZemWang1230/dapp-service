package scanner

import (
	"context"
	"strings"
	"time"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// FlowRepository 流程管理仓库接口
type FlowRepository interface {
	CreateFlow(ctx context.Context, flow *types.TimelockTransactionFlow) error
	GetFlowByID(ctx context.Context, flowID, timelockStandard string, chainID int, contractAddress string) (*types.TimelockTransactionFlow, error)
	UpdateFlow(ctx context.Context, flow *types.TimelockTransactionFlow) error
	GetFlowsByContract(ctx context.Context, chainID int, contractAddress string, timelockStandard string) ([]types.TimelockTransactionFlow, error)
	GetActiveFlows(ctx context.Context, chainID int, contractAddress string) ([]types.TimelockTransactionFlow, error)

	// 状态管理相关方法
	GetWaitingFlowsDue(ctx context.Context, now time.Time, limit int) ([]types.TimelockTransactionFlow, error)
	GetCompoundFlowsExpired(ctx context.Context, now time.Time, limit int) ([]types.TimelockTransactionFlow, error)
	UpdateFlowStatus(ctx context.Context, flowID, timelockStandard string, chainID int, contractAddress string, fromStatus, toStatus string) error
	BatchUpdateFlowStatus(ctx context.Context, flows []types.TimelockTransactionFlow, toStatus string) error

	// API查询方法
	GetFlowsByStatus(ctx context.Context, status string, page, pageSize int) ([]types.TimelockTransactionFlow, int64, error)
	GetFlowsByStatusAndInitiator(ctx context.Context, status string, initiatorAddress string, page, pageSize int) ([]types.TimelockTransactionFlow, int64, error)
	GetFlowDetail(ctx context.Context, flowID, timelockStandard string, chainID int, contractAddress string) (*types.TimelockTransactionFlow, error)
}

type flowRepository struct {
	db *gorm.DB
}

// NewFlowRepository 创建新的流程管理仓库
func NewFlowRepository(db *gorm.DB) FlowRepository {
	return &flowRepository{
		db: db,
	}
}

// CreateFlow 创建交易流程记录
func (r *flowRepository) CreateFlow(ctx context.Context, flow *types.TimelockTransactionFlow) error {
	if err := r.db.WithContext(ctx).Create(flow).Error; err != nil {
		logger.Error("CreateFlow Error", err, "flow_id", flow.FlowID, "standard", flow.TimelockStandard)
		return err
	}

	return nil
}

// GetFlowByID 根据流程ID获取交易流程
func (r *flowRepository) GetFlowByID(ctx context.Context, flowID, timelockStandard string, chainID int, contractAddress string) (*types.TimelockTransactionFlow, error) {
	var flow types.TimelockTransactionFlow
	err := r.db.WithContext(ctx).
		Where("flow_id = ? AND timelock_standard = ? AND chain_id = ? AND contract_address = ?",
			flowID, timelockStandard, chainID, contractAddress).
		First(&flow).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Error("GetFlowByID Error", err, "flow_id", flowID)
		return nil, err
	}

	return &flow, nil
}

// UpdateFlow 更新交易流程
func (r *flowRepository) UpdateFlow(ctx context.Context, flow *types.TimelockTransactionFlow) error {
	if err := r.db.WithContext(ctx).Save(flow).Error; err != nil {
		logger.Error("UpdateFlow Error", err, "flow_id", flow.FlowID)
		return err
	}

	return nil
}

// GetFlowsByContract 获取合约的所有交易流程
func (r *flowRepository) GetFlowsByContract(ctx context.Context, chainID int, contractAddress string, timelockStandard string) ([]types.TimelockTransactionFlow, error) {
	var flows []types.TimelockTransactionFlow
	query := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ?", chainID, contractAddress)

	if timelockStandard != "" {
		query = query.Where("timelock_standard = ?", timelockStandard)
	}

	err := query.Order("created_at DESC").Find(&flows).Error
	if err != nil {
		logger.Error("GetFlowsByContract Error", err, "chain_id", chainID, "contract", contractAddress)
		return nil, err
	}

	return flows, nil
}

// GetActiveFlows 获取活跃的交易流程（未完成的）
func (r *flowRepository) GetActiveFlows(ctx context.Context, chainID int, contractAddress string) ([]types.TimelockTransactionFlow, error) {
	var flows []types.TimelockTransactionFlow
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ? AND status IN (?)",
			chainID, contractAddress, []string{"waiting", "ready"}).
		Order("created_at DESC").
		Find(&flows).Error

	if err != nil {
		logger.Error("GetActiveFlows Error", err, "chain_id", chainID, "contract", contractAddress)
		return nil, err
	}

	return flows, nil
}

// GetWaitingFlowsDue 获取等待中但ETA已到的流程
func (r *flowRepository) GetWaitingFlowsDue(ctx context.Context, now time.Time, limit int) ([]types.TimelockTransactionFlow, error) {
	var flows []types.TimelockTransactionFlow
	query := r.db.WithContext(ctx).
		Where("status = ? AND eta IS NOT NULL AND eta <= ?", "waiting", now).
		Order("eta ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&flows).Error
	if err != nil {
		logger.Error("GetWaitingFlowsDue Error", err, "now", now, "limit", limit)
		return nil, err
	}

	return flows, nil
}

// GetCompoundFlowsExpired 获取Compound中已过期的流程
func (r *flowRepository) GetCompoundFlowsExpired(ctx context.Context, now time.Time, limit int) ([]types.TimelockTransactionFlow, error) {
	var flows []types.TimelockTransactionFlow
	query := r.db.WithContext(ctx).
		Where("timelock_standard = ? AND status IN (?) AND expired_at IS NOT NULL AND expired_at <= ?",
			"compound", []string{"waiting", "ready"}, now).
		Order("expired_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&flows).Error
	if err != nil {
		logger.Error("GetCompoundFlowsExpired Error", err, "now", now, "limit", limit)
		return nil, err
	}

	return flows, nil
}

// UpdateFlowStatus 更新单个流程状态
func (r *flowRepository) UpdateFlowStatus(ctx context.Context, flowID, timelockStandard string, chainID int, contractAddress string, fromStatus, toStatus string) error {
	result := r.db.WithContext(ctx).
		Model(&types.TimelockTransactionFlow{}).
		Where("flow_id = ? AND timelock_standard = ? AND chain_id = ? AND contract_address = ? AND status = ?",
			flowID, timelockStandard, chainID, contractAddress, fromStatus).
		Update("status", toStatus)

	if result.Error != nil {
		logger.Error("UpdateFlowStatus Error", result.Error, "flow_id", flowID, "from", fromStatus, "to", toStatus)
		return result.Error
	}

	if result.RowsAffected == 0 {
		logger.Warn("UpdateFlowStatus: No rows affected", "flow_id", flowID, "from", fromStatus, "to", toStatus)
	}

	return nil
}

// BatchUpdateFlowStatus 批量更新流程状态
func (r *flowRepository) BatchUpdateFlowStatus(ctx context.Context, flows []types.TimelockTransactionFlow, toStatus string) error {
	if len(flows) == 0 {
		return nil
	}

	// 构建WHERE条件：(flow_id = ? AND timelock_standard = ? AND chain_id = ? AND contract_address = ?) OR ...
	var conditions []string
	var args []interface{}

	for _, flow := range flows {
		conditions = append(conditions, "(flow_id = ? AND timelock_standard = ? AND chain_id = ? AND contract_address = ?)")
		args = append(args, flow.FlowID, flow.TimelockStandard, flow.ChainID, flow.ContractAddress)
	}

	whereClause := "(" + strings.Join(conditions, " OR ") + ")"

	result := r.db.WithContext(ctx).
		Model(&types.TimelockTransactionFlow{}).
		Where(whereClause, args...).
		Update("status", toStatus)

	if result.Error != nil {
		logger.Error("BatchUpdateFlowStatus Error", result.Error, "count", len(flows), "to_status", toStatus)
		return result.Error
	}

	logger.Info("BatchUpdateFlowStatus completed", "updated", result.RowsAffected, "to_status", toStatus)
	return nil
}

// GetFlowsByStatus 根据状态获取流程列表（分页）
func (r *flowRepository) GetFlowsByStatus(ctx context.Context, status string, page, pageSize int) ([]types.TimelockTransactionFlow, int64, error) {
	var flows []types.TimelockTransactionFlow
	var total int64

	// 计算总数
	if err := r.db.WithContext(ctx).
		Model(&types.TimelockTransactionFlow{}).
		Where("status = ?", status).
		Count(&total).Error; err != nil {
		logger.Error("GetFlowsByStatus Count Error", err, "status", status)
		return nil, 0, err
	}

	// 获取分页数据
	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&flows).Error

	if err != nil {
		logger.Error("GetFlowsByStatus Error", err, "status", status, "page", page)
		return nil, 0, err
	}

	return flows, total, nil
}

// GetFlowsByStatusAndInitiator 根据状态和发起人获取流程列表（分页）
func (r *flowRepository) GetFlowsByStatusAndInitiator(ctx context.Context, status string, initiatorAddress string, page, pageSize int) ([]types.TimelockTransactionFlow, int64, error) {
	var flows []types.TimelockTransactionFlow
	var total int64

	// 计算总数
	if err := r.db.WithContext(ctx).
		Model(&types.TimelockTransactionFlow{}).
		Where("status = ? AND initiator_address = ?", status, strings.ToLower(initiatorAddress)).
		Count(&total).Error; err != nil {
		logger.Error("GetFlowsByStatusAndInitiator Count Error", err, "status", status, "initiator", initiatorAddress)
		return nil, 0, err
	}

	// 获取分页数据
	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).
		Where("status = ? AND initiator_address = ?", status, strings.ToLower(initiatorAddress)).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&flows).Error

	if err != nil {
		logger.Error("GetFlowsByStatusAndInitiator Error", err, "status", status, "initiator", initiatorAddress, "page", page)
		return nil, 0, err
	}

	return flows, total, nil
}

// GetFlowDetail 获取流程详细信息
func (r *flowRepository) GetFlowDetail(ctx context.Context, flowID, timelockStandard string, chainID int, contractAddress string) (*types.TimelockTransactionFlow, error) {
	var flow types.TimelockTransactionFlow
	err := r.db.WithContext(ctx).
		Where("flow_id = ? AND timelock_standard = ? AND chain_id = ? AND contract_address = ?",
			flowID, timelockStandard, chainID, contractAddress).
		First(&flow).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Error("GetFlowDetail Error", err, "flow_id", flowID)
		return nil, err
	}

	return &flow, nil
}
