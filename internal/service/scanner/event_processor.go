package scanner

import (
	"context"
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
	// 完善流程
	return &types.CompoundTimelockTransaction{
		TxHash:          event.TxHash,
		BlockNumber:     int64(event.BlockNumber),
		BlockTimestamp:  time.Unix(int64(event.BlockTimestamp), 0),
		ChainID:         event.ChainID,
		ChainName:       event.ChainName,
		ContractAddress: event.ContractAddress,
	}
}

// convertOpenZeppelinEvent 转换OpenZeppelin事件为数据库记录
func (ep *EventProcessor) convertOpenZeppelinEvent(event *types.OpenZeppelinTimelockEvent) *types.OpenZeppelinTimelockTransaction {
	// 完善流程
	return &types.OpenZeppelinTimelockTransaction{
		TxHash:          event.TxHash,
		BlockNumber:     int64(event.BlockNumber),
		BlockTimestamp:  time.Unix(int64(event.BlockTimestamp), 0),
		ChainID:         event.ChainID,
		ChainName:       event.ChainName,
		ContractAddress: event.ContractAddress,
	}
}

// processCompoundFlow 处理Compound流程关联
func (ep *EventProcessor) processCompoundFlow(ctx context.Context, event *types.CompoundTimelockEvent) error {
	// 完善流程
	return nil
}

// processOpenZeppelinFlow 处理OpenZeppelin流程关联
func (ep *EventProcessor) processOpenZeppelinFlow(ctx context.Context, event *types.OpenZeppelinTimelockEvent) error {
	// 完善流程
	return nil
}

// processUserRelations 处理用户关联关系
func (ep *EventProcessor) processUserRelations(ctx context.Context, chainID int, contractAddress, timelockStandard, userAddress, eventType string) error {
	// 完善用户关联
	return nil
}
