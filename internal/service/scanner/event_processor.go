package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/scanner"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// EventProcessor 事件处理器
type EventProcessor struct {
	config       *config.Config
	txRepo       scanner.TransactionRepository
	flowRepo     scanner.FlowRepository
	relationRepo scanner.RelationRepository
}

// NewEventProcessor 创建新的事件处理器
func NewEventProcessor(
	cfg *config.Config,
	txRepo scanner.TransactionRepository,
	flowRepo scanner.FlowRepository,
	relationRepo scanner.RelationRepository,
) *EventProcessor {
	return &EventProcessor{
		config:       cfg,
		txRepo:       txRepo,
		flowRepo:     flowRepo,
		relationRepo: relationRepo,
	}
}

// ProcessEvents 处理事件列表
func (ep *EventProcessor) ProcessEvents(ctx context.Context, chainID int, chainName string, events []TimelockEvent) error {
	if len(events) == 0 {
		return nil
	}

	var compoundEvents []types.CompoundTimelockTransaction
	var ozEvents []types.OpenZeppelinTimelockTransaction

	// 分类处理事件
	for _, event := range events {
		switch e := event.(type) {
		case *types.CompoundTimelockEvent:
			tx := ep.convertCompoundEvent(e)
			compoundEvents = append(compoundEvents, *tx)

			// 处理流程关联
			if err := ep.processCompoundFlow(ctx, e); err != nil {
				logger.Error("Failed to process Compound flow", err, "tx_hash", e.TxHash)
			}

			// 处理用户关联
			if err := ep.processUserRelations(ctx, e.ChainID, e.ContractAddress, "compound", e.FromAddress, e.EventType); err != nil {
				logger.Error("Failed to process user relations", err, "tx_hash", e.TxHash)
			}

		case *types.OpenZeppelinTimelockEvent:
			tx := ep.convertOpenZeppelinEvent(e)
			ozEvents = append(ozEvents, *tx)

			// 处理流程关联
			if err := ep.processOpenZeppelinFlow(ctx, e); err != nil {
				logger.Error("Failed to process OpenZeppelin flow", err, "tx_hash", e.TxHash)
			}

			// 处理用户关联
			if err := ep.processUserRelations(ctx, e.ChainID, e.ContractAddress, "openzeppelin", e.FromAddress, e.EventType); err != nil {
				logger.Error("Failed to process user relations", err, "tx_hash", e.TxHash)
			}
		default:
			logger.Warn("Unknown event type", "event", event)
		}
	}

	// 批量存储事件
	if len(compoundEvents) > 0 {
		if err := ep.txRepo.BatchCreateCompoundTransactions(ctx, compoundEvents); err != nil {
			logger.Error("Failed to batch create Compound transactions", err)
			return fmt.Errorf("failed to create Compound transactions: %w", err)
		}
	}

	if len(ozEvents) > 0 {
		if err := ep.txRepo.BatchCreateOpenZeppelinTransactions(ctx, ozEvents); err != nil {
			logger.Error("Failed to batch create OpenZeppelin transactions", err)
			return fmt.Errorf("failed to create OpenZeppelin transactions: %w", err)
		}
	}

	return nil
}

// convertCompoundEvent 转换Compound事件为数据库记录
func (ep *EventProcessor) convertCompoundEvent(event *types.CompoundTimelockEvent) *types.CompoundTimelockTransaction {
	// 序列化EventData
	eventDataJSON := ""
	if event.EventData != nil {
		if jsonData, err := json.Marshal(event.EventData); err == nil {
			eventDataJSON = string(jsonData)
		}
	}

	tx := &types.CompoundTimelockTransaction{
		TxHash:          event.TxHash,
		BlockNumber:     int64(event.BlockNumber),
		BlockTimestamp:  time.Unix(int64(event.BlockTimestamp), 0),
		ChainID:         event.ChainID,
		ChainName:       event.ChainName,
		ContractAddress: event.ContractAddress,
		FromAddress:     event.FromAddress,
		ToAddress:       event.ToAddress,
		EventType:       event.EventType,
		EventData:       eventDataJSON,
	}

	// 设置特定字段
	if event.ProposalID != nil {
		tx.ProposalID = event.ProposalID
	}
	if event.TargetAddress != nil {
		tx.TargetAddress = event.TargetAddress
	}
	if event.FunctionSignature != nil {
		tx.FunctionSignature = event.FunctionSignature
	}
	if event.Eta != nil {
		eta := int64(*event.Eta)
		tx.Eta = &eta
	}
	if event.Value != nil {
		tx.Value = *event.Value
	}

	return tx
}

// convertOpenZeppelinEvent 转换OpenZeppelin事件为数据库记录
func (ep *EventProcessor) convertOpenZeppelinEvent(event *types.OpenZeppelinTimelockEvent) *types.OpenZeppelinTimelockTransaction {
	// 序列化EventData
	eventDataJSON := ""
	if event.EventData != nil {
		if jsonData, err := json.Marshal(event.EventData); err == nil {
			eventDataJSON = string(jsonData)
		}
	}

	tx := &types.OpenZeppelinTimelockTransaction{
		TxHash:          event.TxHash,
		BlockNumber:     int64(event.BlockNumber),
		BlockTimestamp:  time.Unix(int64(event.BlockTimestamp), 0),
		ChainID:         event.ChainID,
		ChainName:       event.ChainName,
		ContractAddress: event.ContractAddress,
		FromAddress:     event.FromAddress,
		ToAddress:       event.ToAddress,
		EventType:       event.EventType,
		EventData:       eventDataJSON,
	}

	// 设置特定字段
	if event.OperationID != nil {
		tx.OperationID = event.OperationID
	}
	if event.TargetAddress != nil {
		tx.TargetAddress = event.TargetAddress
	}
	if event.FunctionSignature != nil {
		tx.FunctionSignature = event.FunctionSignature
	}
	if event.Delay != nil {
		delay := int64(*event.Delay)
		tx.Delay = &delay
	}
	if event.Value != nil {
		tx.Value = *event.Value
	}

	return tx
}

// processCompoundFlow 处理Compound流程关联
func (ep *EventProcessor) processCompoundFlow(ctx context.Context, event *types.CompoundTimelockEvent) error {
	if event.ProposalID == nil {
		return nil // 没有提案ID，无法关联流程
	}

	flowID := *event.ProposalID

	// 查找现有流程
	flow, err := ep.flowRepo.GetFlowByID(ctx, flowID, "compound", event.ChainID, event.ContractAddress)
	if err != nil {
		return fmt.Errorf("failed to get flow: %w", err)
	}

	if flow == nil {
		// 创建新流程
		flow = &types.TimelockTransactionFlow{
			FlowID:           flowID,
			TimelockStandard: "compound",
			ChainID:          event.ChainID,
			ContractAddress:  event.ContractAddress,
			Status:           "proposed",
		}

		// 设置提案信息
		if event.TargetAddress != nil {
			flow.TargetAddress = event.TargetAddress
		}
		if event.FunctionSignature != nil {
			flow.FunctionSignature = event.FunctionSignature
		}
		if event.Value != nil {
			flow.Value = *event.Value
		}
	}

	// 根据事件类型更新流程状态
	switch event.EventType {
	case "QueueTransaction":
		flow.Status = "queued"
		flow.QueuedAt = &time.Time{}
		*flow.QueuedAt = time.Unix(int64(event.BlockTimestamp), 0)
		if event.Eta != nil {
			etaTime := time.Unix(int64(*event.Eta), 0)
			flow.Eta = &etaTime
		}

	case "ExecuteTransaction":
		flow.Status = "executed"
		flow.ExecutedAt = &time.Time{}
		*flow.ExecutedAt = time.Unix(int64(event.BlockTimestamp), 0)

	case "CancelTransaction":
		flow.Status = "cancelled"
		flow.CancelledAt = &time.Time{}
		*flow.CancelledAt = time.Unix(int64(event.BlockTimestamp), 0)
	}

	// 保存或更新流程
	if flow.ID == 0 {
		flow.ProposedAt = &time.Time{}
		*flow.ProposedAt = time.Unix(int64(event.BlockTimestamp), 0)
		return ep.flowRepo.CreateFlow(ctx, flow)
	} else {
		return ep.flowRepo.UpdateFlow(ctx, flow)
	}
}

// processOpenZeppelinFlow 处理OpenZeppelin流程关联
func (ep *EventProcessor) processOpenZeppelinFlow(ctx context.Context, event *types.OpenZeppelinTimelockEvent) error {
	if event.OperationID == nil {
		return nil // 没有操作ID，无法关联流程
	}

	flowID := *event.OperationID

	// 查找现有流程
	flow, err := ep.flowRepo.GetFlowByID(ctx, flowID, "openzeppelin", event.ChainID, event.ContractAddress)
	if err != nil {
		return fmt.Errorf("failed to get flow: %w", err)
	}

	if flow == nil {
		// 创建新流程
		flow = &types.TimelockTransactionFlow{
			FlowID:           flowID,
			TimelockStandard: "openzeppelin",
			ChainID:          event.ChainID,
			ContractAddress:  event.ContractAddress,
			Status:           "proposed",
		}

		// 设置提案信息
		if event.TargetAddress != nil {
			flow.TargetAddress = event.TargetAddress
		}
		if event.FunctionSignature != nil {
			flow.FunctionSignature = event.FunctionSignature
		}
		if event.Value != nil {
			flow.Value = *event.Value
		}
	}

	// 根据事件类型更新流程状态
	switch event.EventType {
	case "CallScheduled":
		flow.Status = "proposed" // OpenZeppelin没有queue状态，直接是proposed
		flow.ProposedAt = &time.Time{}
		*flow.ProposedAt = time.Unix(int64(event.BlockTimestamp), 0)

		// 计算ETA (当前时间 + delay)
		if event.Delay != nil {
			etaTime := time.Unix(int64(event.BlockTimestamp)+int64(*event.Delay), 0)
			flow.Eta = &etaTime
		}

	case "CallExecuted":
		flow.Status = "executed"
		flow.ExecutedAt = &time.Time{}
		*flow.ExecutedAt = time.Unix(int64(event.BlockTimestamp), 0)

	case "Cancelled":
		flow.Status = "cancelled"
		flow.CancelledAt = &time.Time{}
		*flow.CancelledAt = time.Unix(int64(event.BlockTimestamp), 0)
	}

	// 保存或更新流程
	if flow.ID == 0 {
		return ep.flowRepo.CreateFlow(ctx, flow)
	} else {
		return ep.flowRepo.UpdateFlow(ctx, flow)
	}
}

// processUserRelations 处理用户关联关系
func (ep *EventProcessor) processUserRelations(ctx context.Context, chainID int, contractAddress, timelockStandard, userAddress, eventType string) error {
	// 确定关联类型
	var relationType string
	switch eventType {
	case "QueueTransaction", "CallScheduled":
		relationType = types.RelationProposer
	case "ExecuteTransaction", "CallExecuted":
		relationType = types.RelationExecutor
	case "CancelTransaction", "Cancelled":
		relationType = types.RelationCanceller
	case "NewAdmin", "RoleGranted":
		// 需要进一步分析是否是管理员角色
		if eventType == "RoleGranted" {
			// 这里需要检查角色类型，简化处理
			relationType = types.RelationAdmin
		} else {
			relationType = types.RelationAdmin
		}
	case "NewPendingAdmin":
		relationType = types.RelationPendingAdmin
	default:
		// 对于其他事件类型，暂不创建关联关系
		return nil
	}

	// 创建用户关联关系
	relation := &types.UserTimelockRelation{
		UserAddress:      userAddress,
		ChainID:          chainID,
		ContractAddress:  contractAddress,
		TimelockStandard: timelockStandard,
		RelationType:     relationType,
		RelatedAt:        time.Now(),
		IsActive:         true,
	}

	// 创建关联关系（如果已存在则忽略）
	if err := ep.relationRepo.CreateRelation(ctx, relation); err != nil {
		logger.Debug("Failed to create relation (may already exist)", "user", userAddress, "contract", contractAddress, "type", relationType)
		// 不返回错误，因为关联关系可能已经存在
	}

	return nil
}
