package scanner

import (
	"context"
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
			chainID, contractAddress, []string{"proposed", "queued"}).
		Order("created_at DESC").
		Find(&flows).Error

	if err != nil {
		logger.Error("GetActiveFlows Error", err, "chain_id", chainID, "contract", contractAddress)
		return nil, err
	}

	return flows, nil
}
