package scanner

import (
	"context"
	"fmt"
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

	// 状态管理相关方法
	GetWaitingFlowsDue(ctx context.Context, now time.Time, limit int) ([]types.TimelockTransactionFlow, error)
	GetCompoundFlowsExpired(ctx context.Context, now time.Time, limit int) ([]types.TimelockTransactionFlow, error)
	UpdateFlowStatus(ctx context.Context, flowID, timelockStandard string, chainID int, contractAddress string, fromStatus, toStatus string) error
	BatchUpdateFlowStatus(ctx context.Context, flows []types.TimelockTransactionFlow, toStatus string) error

	// 新API查询方法
	GetUserRelatedFlows(ctx context.Context, userAddress string, status *string, standard *string) ([]types.TimelockTransactionFlow, int64, error)
	GetTransactionDetail(ctx context.Context, standard string, txHash string) (*types.TimelockTransactionDetail, error)
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

// GetUserRelatedFlows 获取与用户相关的流程列表
func (r *flowRepository) GetUserRelatedFlows(ctx context.Context, userAddress string, status *string, standard *string) ([]types.TimelockTransactionFlow, int64, error) {
	userAddress = strings.ToLower(userAddress)

	// 构建查询条件
	query := r.db.WithContext(ctx).Model(&types.TimelockTransactionFlow{})

	// 构建WHERE条件，包含两种情况：
	// 1. initiator_address是该地址
	// 2. 该flow的合约中，该地址是管理员或有权限的用户
	whereConditions := []string{}
	args := []interface{}{}

	// 第一种情况：initiator_address是该地址
	whereConditions = append(whereConditions, "initiator_address = ?")
	args = append(args, userAddress)

	// 第二种情况：根据合约权限查询
	// 需要联表查询compound_timelocks和openzeppelin_timelocks

	// Compound权限查询：admin或pending_admin
	compoundCondition := `(
		timelock_standard = 'compound' AND 
		(chain_id, contract_address) IN (
			SELECT chain_id, contract_address FROM compound_timelocks 
			WHERE admin = ? OR pending_admin = ?
		)
	)`
	whereConditions = append(whereConditions, compoundCondition)
	args = append(args, userAddress, userAddress)

	// OpenZeppelin权限查询：proposers或executors中的一个
	ozCondition := `(
		timelock_standard = 'openzeppelin' AND 
		(chain_id, contract_address) IN (
			SELECT chain_id, contract_address FROM openzeppelin_timelocks 
			WHERE proposers LIKE '%' || ? || '%' OR
				executors LIKE '%' || ? || '%'
		)
	)`
	whereConditions = append(whereConditions, ozCondition)
	args = append(args, userAddress, userAddress)

	// 组合所有条件
	finalWhere := "(" + strings.Join(whereConditions, " OR ") + ")"

	// 添加状态过滤
	if status != nil && *status != "all" {
		finalWhere += " AND status = ?"
		args = append(args, *status)
	}

	// 添加标准过滤
	if standard != nil {
		finalWhere += " AND timelock_standard = ?"
		args = append(args, *standard)
	}

	// 计算总数
	var total int64
	if err := query.Where(finalWhere, args...).Count(&total).Error; err != nil {
		logger.Error("GetUserRelatedFlows Count Error", err, "user", userAddress)
		return nil, 0, err
	}

	// 获取数据
	var flows []types.TimelockTransactionFlow
	err := r.db.WithContext(ctx).
		Where(finalWhere, args...).
		Order("created_at DESC").
		Find(&flows).Error

	if err != nil {
		logger.Error("GetUserRelatedFlows Error", err, "user", userAddress)
		return nil, 0, err
	}

	return flows, total, nil
}

// GetTransactionDetail 获取交易详情
func (r *flowRepository) GetTransactionDetail(ctx context.Context, standard string, txHash string) (*types.TimelockTransactionDetail, error) {
	var detail types.TimelockTransactionDetail

	if standard == "compound" {
		// 查询compound交易表
		var tx types.CompoundTimelockTransaction
		err := r.db.WithContext(ctx).
			Where("tx_hash = ?", txHash).
			First(&tx).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil
			}
			logger.Error("GetTransactionDetail Compound Error", err, "tx_hash", txHash)
			return nil, err
		}

		// 转换为统一格式
		detail = types.TimelockTransactionDetail{
			TxHash:          tx.TxHash,
			BlockNumber:     tx.BlockNumber,
			BlockTimestamp:  tx.BlockTimestamp,
			ChainID:         tx.ChainID,
			ChainName:       tx.ChainName,
			ContractAddress: tx.ContractAddress,
			FromAddress:     tx.FromAddress,
			ToAddress:       tx.ToAddress,
			TxStatus:        tx.TxStatus,
		}

	} else if standard == "openzeppelin" {
		// 查询openzeppelin交易表
		var tx types.OpenZeppelinTimelockTransaction
		err := r.db.WithContext(ctx).
			Where("tx_hash = ?", txHash).
			First(&tx).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, nil
			}
			logger.Error("GetTransactionDetail OpenZeppelin Error", err, "tx_hash", txHash)
			return nil, err
		}

		// 转换为统一格式
		detail = types.TimelockTransactionDetail{
			TxHash:          tx.TxHash,
			BlockNumber:     tx.BlockNumber,
			BlockTimestamp:  tx.BlockTimestamp,
			ChainID:         tx.ChainID,
			ChainName:       tx.ChainName,
			ContractAddress: tx.ContractAddress,
			FromAddress:     tx.FromAddress,
			ToAddress:       tx.ToAddress,
			TxStatus:        tx.TxStatus,
		}
	} else {
		return nil, fmt.Errorf("unsupported standard: %s", standard)
	}

	return &detail, nil
}
