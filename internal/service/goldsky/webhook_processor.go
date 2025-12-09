package goldsky

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	goldskyRepo "timelocker-backend/internal/repository/goldsky"
	timelockRepo "timelocker-backend/internal/repository/timelock"
	"timelocker-backend/internal/service/email"
	"timelocker-backend/internal/service/notification"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// WebhookProcessor Webhook 事件处理器
type WebhookProcessor struct {
	timelockRepo    timelockRepo.Repository
	flowRepo        goldskyRepo.FlowRepository
	emailSvc        email.EmailService
	notificationSvc notification.NotificationService
}

// NewWebhookProcessor 创建 Webhook 处理器
func NewWebhookProcessor(
	timelockRepo timelockRepo.Repository,
	flowRepo goldskyRepo.FlowRepository,
	emailSvc email.EmailService,
	notificationSvc notification.NotificationService,
) *WebhookProcessor {
	return &WebhookProcessor{
		timelockRepo:    timelockRepo,
		flowRepo:        flowRepo,
		emailSvc:        emailSvc,
		notificationSvc: notificationSvc,
	}
}

// ProcessCompoundTransaction 处理 Compound Transaction Webhook
func (p *WebhookProcessor) ProcessCompoundTransaction(ctx context.Context, tx types.GoldskyCompoundTransactionWebhook, chainID int) error {
	logger.Info("Processing Compound transaction webhook",
		"chain_id", chainID,
		"tx_hash", tx.TxHash,
		"event_type", tx.EventType,
		"contract", tx.ContractAddress)

	// 1. 检查是否为平台合约
	isPlatformContract, err := p.isPlatformContract(ctx, "compound", chainID, tx.ContractAddress)
	if err != nil {
		return fmt.Errorf("failed to check if contract is in platform: %w", err)
	}

	if !isPlatformContract {
		logger.Info("Contract not in platform, skipping",
			"contract", tx.ContractAddress,
			"chain_id", chainID)
		return nil
	}

	// 2. 根据事件类型处理
	switch tx.EventType {
	case "QueueTransaction":
		return p.handleCompoundQueue(ctx, tx, chainID)
	case "ExecuteTransaction":
		return p.handleCompoundExecute(ctx, tx, chainID)
	case "CancelTransaction":
		return p.handleCompoundCancel(ctx, tx, chainID)
	default:
		logger.Warn("Unknown Compound event type", "event_type", tx.EventType)
		return nil
	}
}

// ProcessOpenzeppelinTransaction 处理 OpenZeppelin Transaction Webhook
func (p *WebhookProcessor) ProcessOpenzeppelinTransaction(ctx context.Context, tx types.GoldskyOpenzeppelinTransactionWebhook, chainID int) error {
	logger.Info("Processing OpenZeppelin transaction webhook",
		"chain_id", chainID,
		"tx_hash", tx.TxHash,
		"event_type", tx.EventType,
		"contract", tx.ContractAddress)

	isPlatformContract, err := p.isPlatformContract(ctx, "openzeppelin", chainID, tx.ContractAddress)
	if err != nil {
		return fmt.Errorf("failed to check if contract is in platform: %w", err)
	}

	if !isPlatformContract {
		logger.Info("Contract not in platform, skipping",
			"contract", tx.ContractAddress,
			"chain_id", chainID)
		return nil
	}

	switch tx.EventType {
	case "CallScheduled":
		return p.handleOpenzeppelinSchedule(ctx, tx, chainID)
	case "CallExecuted":
		return p.handleOpenzeppelinExecute(ctx, tx, chainID)
	case "Cancelled":
		return p.handleOpenzeppelinCancel(ctx, tx, chainID)
	default:
		logger.Warn("Unknown OpenZeppelin event type", "event_type", tx.EventType)
		return nil
	}
}

// handleCompoundQueue 处理 Compound Queue 事件 - 创建 Flow
func (p *WebhookProcessor) handleCompoundQueue(ctx context.Context, tx types.GoldskyCompoundTransactionWebhook, chainID int) error {
	if tx.EventTxHash == nil {
		return fmt.Errorf("missing eventTxHash for Queue transaction")
	}

	flowID := *tx.EventTxHash // Compound 使用 eventTxHash 作为 flowId

	// 检查 Flow 是否已存在
	existingFlow, err := p.flowRepo.GetCompoundFlowByID(ctx, flowID, chainID, tx.ContractAddress)
	if err != nil {
		return fmt.Errorf("failed to check existing flow: %w", err)
	}

	if existingFlow != nil {
		logger.Info("Flow already exists, skipping creation", "flow_id", flowID)
		return nil // 已存在，跳过（幂等性）
	}

	// 创建新的 Flow
	flow := &types.CompoundTimelockFlowDB{
		FlowID:           flowID,
		TimelockStandard: "compound",
		ChainID:          chainID,
		ContractAddress:  tx.ContractAddress,
		Status:           "waiting", // 默认 waiting 状态
		QueueTxHash:      &tx.TxHash,
		InitiatorAddress: &tx.FromAddress,
		Value:            tx.EventValue,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// 填充详细信息
	if tx.EventTarget != nil {
		flow.TargetAddress = tx.EventTarget
	}
	if tx.EventSignature != nil {
		flow.FunctionSignature = tx.EventSignature
	}
	if tx.EventData != nil && *tx.EventData != "" {
		callDataStr := strings.TrimPrefix(*tx.EventData, "0x")
		callDataBytes, err := hex.DecodeString(callDataStr)
		if err == nil {
			flow.CallData = callDataBytes
		}
	}
	if tx.EventEta != nil {
		if eta, err := strconv.ParseInt(*tx.EventEta, 10, 64); err == nil {
			etaTime := time.Unix(eta, 0)
			flow.Eta = &etaTime
		}
	}

	// 解析 queuedAt
	if blockTs, err := strconv.ParseInt(tx.BlockTimestamp, 10, 64); err == nil {
		queuedAt := time.Unix(blockTs, 0)
		flow.QueuedAt = &queuedAt
	}

	// 创建 Flow
	if err := p.flowRepo.CreateOrUpdateCompoundFlow(ctx, flow); err != nil {
		return fmt.Errorf("failed to create flow: %w", err)
	}

	logger.Info("Created new Compound flow", "flow_id", flowID, "status", "waiting")

	// 异步发送通知
	go p.sendFlowNotification(chainID, tx.ContractAddress, flowID, "compound", "", "waiting", &tx.TxHash, tx.FromAddress)

	return nil
}

// handleCompoundExecute 处理 Compound Execute 事件 - 更新 Flow 状态
func (p *WebhookProcessor) handleCompoundExecute(ctx context.Context, tx types.GoldskyCompoundTransactionWebhook, chainID int) error {
	if tx.EventTxHash == nil {
		return fmt.Errorf("missing eventTxHash for Execute transaction")
	}

	flowID := *tx.EventTxHash

	// 获取现有 Flow
	existingFlow, err := p.flowRepo.GetCompoundFlowByID(ctx, flowID, chainID, tx.ContractAddress)
	if err != nil || existingFlow == nil {
		logger.Warn("Flow not found for Execute transaction", "flow_id", flowID)
		return nil // Flow 不存在，可能是其他链或非平台合约
	}

	oldStatus := existingFlow.Status

	// 更新 Flow 状态
	existingFlow.Status = "executed"
	existingFlow.ExecuteTxHash = &tx.TxHash

	if blockTs, err := strconv.ParseInt(tx.BlockTimestamp, 10, 64); err == nil {
		executedAt := time.Unix(blockTs, 0)
		existingFlow.ExecutedAt = &executedAt
	}
	existingFlow.UpdatedAt = time.Now()

	if err := p.flowRepo.CreateOrUpdateCompoundFlow(ctx, existingFlow); err != nil {
		return fmt.Errorf("failed to update flow: %w", err)
	}

	logger.Info("Updated Compound flow to executed", "flow_id", flowID, "old_status", oldStatus)

	// 异步发送通知
	go p.sendFlowNotification(chainID, tx.ContractAddress, flowID, "compound", oldStatus, "executed", &tx.TxHash, tx.FromAddress)

	return nil
}

// handleCompoundCancel 处理 Compound Cancel 事件 - 更新 Flow 状态
func (p *WebhookProcessor) handleCompoundCancel(ctx context.Context, tx types.GoldskyCompoundTransactionWebhook, chainID int) error {
	if tx.EventTxHash == nil {
		return fmt.Errorf("missing eventTxHash for Cancel transaction")
	}

	flowID := *tx.EventTxHash

	existingFlow, err := p.flowRepo.GetCompoundFlowByID(ctx, flowID, chainID, tx.ContractAddress)
	if err != nil || existingFlow == nil {
		logger.Warn("Flow not found for Cancel transaction", "flow_id", flowID)
		return nil
	}

	oldStatus := existingFlow.Status

	existingFlow.Status = "cancelled"
	existingFlow.CancelTxHash = &tx.TxHash

	if blockTs, err := strconv.ParseInt(tx.BlockTimestamp, 10, 64); err == nil {
		cancelledAt := time.Unix(blockTs, 0)
		existingFlow.CancelledAt = &cancelledAt
	}
	existingFlow.UpdatedAt = time.Now()

	if err := p.flowRepo.CreateOrUpdateCompoundFlow(ctx, existingFlow); err != nil {
		return fmt.Errorf("failed to update flow: %w", err)
	}

	logger.Info("Updated Compound flow to cancelled", "flow_id", flowID, "old_status", oldStatus)

	// 异步发送通知
	go p.sendFlowNotification(chainID, tx.ContractAddress, flowID, "compound", oldStatus, "cancelled", &tx.TxHash, tx.FromAddress)

	return nil
}

// handleOpenzeppelinSchedule 处理 OpenZeppelin Schedule 事件 - 创建 Flow
func (p *WebhookProcessor) handleOpenzeppelinSchedule(ctx context.Context, tx types.GoldskyOpenzeppelinTransactionWebhook, chainID int) error {
	if tx.EventId == nil {
		return fmt.Errorf("missing eventId for Schedule transaction")
	}

	flowID := *tx.EventId

	existingFlow, err := p.flowRepo.GetOpenzeppelinFlowByID(ctx, flowID, chainID, tx.ContractAddress)
	if err != nil {
		return fmt.Errorf("failed to check existing flow: %w", err)
	}

	if existingFlow != nil {
		logger.Info("Flow already exists, skipping creation", "flow_id", flowID)
		return nil
	}

	flow := &types.OpenzeppelinTimelockFlowDB{
		FlowID:           flowID,
		TimelockStandard: "openzeppelin",
		ChainID:          chainID,
		ContractAddress:  tx.ContractAddress,
		Status:           "waiting",
		ScheduleTxHash:   &tx.TxHash,
		InitiatorAddress: &tx.FromAddress,
		Value:            tx.EventValue,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if tx.EventTarget != nil {
		flow.TargetAddress = tx.EventTarget
	}
	if tx.EventData != nil && *tx.EventData != "" {
		callDataStr := strings.TrimPrefix(*tx.EventData, "0x")
		callDataBytes, err := hex.DecodeString(callDataStr)
		if err == nil {
			flow.CallData = callDataBytes
		}
	}
	if tx.EventDelay != nil {
		if delay, err := strconv.ParseInt(*tx.EventDelay, 10, 64); err == nil {
			flow.Delay = &delay
		}
	}

	// 计算 ETA = blockTimestamp + delay
	if blockTs, err := strconv.ParseInt(tx.BlockTimestamp, 10, 64); err == nil {
		queuedAt := time.Unix(blockTs, 0)
		flow.QueuedAt = &queuedAt

		if flow.Delay != nil {
			eta := queuedAt.Add(time.Duration(*flow.Delay) * time.Second)
			flow.Eta = &eta
		}
	}

	if err := p.flowRepo.CreateOrUpdateOpenzeppelinFlow(ctx, flow); err != nil {
		return fmt.Errorf("failed to create flow: %w", err)
	}

	logger.Info("Created new OpenZeppelin flow", "flow_id", flowID, "status", "waiting")

	// 异步发送通知
	go p.sendFlowNotification(chainID, tx.ContractAddress, flowID, "openzeppelin", "", "waiting", &tx.TxHash, tx.FromAddress)

	return nil
}

// handleOpenzeppelinExecute 处理 OpenZeppelin Execute 事件
func (p *WebhookProcessor) handleOpenzeppelinExecute(ctx context.Context, tx types.GoldskyOpenzeppelinTransactionWebhook, chainID int) error {
	if tx.EventId == nil {
		return fmt.Errorf("missing eventId for Execute transaction")
	}

	flowID := *tx.EventId

	existingFlow, err := p.flowRepo.GetOpenzeppelinFlowByID(ctx, flowID, chainID, tx.ContractAddress)
	if err != nil || existingFlow == nil {
		logger.Warn("Flow not found for Execute transaction", "flow_id", flowID)
		return nil
	}

	oldStatus := existingFlow.Status

	existingFlow.Status = "executed"
	existingFlow.ExecuteTxHash = &tx.TxHash

	if blockTs, err := strconv.ParseInt(tx.BlockTimestamp, 10, 64); err == nil {
		executedAt := time.Unix(blockTs, 0)
		existingFlow.ExecutedAt = &executedAt
	}
	existingFlow.UpdatedAt = time.Now()

	if err := p.flowRepo.CreateOrUpdateOpenzeppelinFlow(ctx, existingFlow); err != nil {
		return fmt.Errorf("failed to update flow: %w", err)
	}

	logger.Info("Updated OpenZeppelin flow to executed", "flow_id", flowID, "old_status", oldStatus)

	// 异步发送通知
	go p.sendFlowNotification(chainID, tx.ContractAddress, flowID, "openzeppelin", oldStatus, "executed", &tx.TxHash, tx.FromAddress)

	return nil
}

// handleOpenzeppelinCancel 处理 OpenZeppelin Cancel 事件
func (p *WebhookProcessor) handleOpenzeppelinCancel(ctx context.Context, tx types.GoldskyOpenzeppelinTransactionWebhook, chainID int) error {
	if tx.EventId == nil {
		return fmt.Errorf("missing eventId for Cancel transaction")
	}

	flowID := *tx.EventId

	existingFlow, err := p.flowRepo.GetOpenzeppelinFlowByID(ctx, flowID, chainID, tx.ContractAddress)
	if err != nil || existingFlow == nil {
		logger.Warn("Flow not found for Cancel transaction", "flow_id", flowID)
		return nil
	}

	oldStatus := existingFlow.Status

	existingFlow.Status = "cancelled"
	existingFlow.CancelTxHash = &tx.TxHash

	if blockTs, err := strconv.ParseInt(tx.BlockTimestamp, 10, 64); err == nil {
		cancelledAt := time.Unix(blockTs, 0)
		existingFlow.CancelledAt = &cancelledAt
	}
	existingFlow.UpdatedAt = time.Now()

	if err := p.flowRepo.CreateOrUpdateOpenzeppelinFlow(ctx, existingFlow); err != nil {
		return fmt.Errorf("failed to update flow: %w", err)
	}

	logger.Info("Updated OpenZeppelin flow to cancelled", "flow_id", flowID, "old_status", oldStatus)

	// 异步发送通知
	go p.sendFlowNotification(chainID, tx.ContractAddress, flowID, "openzeppelin", oldStatus, "cancelled", &tx.TxHash, tx.FromAddress)

	return nil
}

// isPlatformContract 检查合约是否在平台中
func (p *WebhookProcessor) isPlatformContract(ctx context.Context, standard string, chainID int, contractAddress string) (bool, error) {
	if standard == "compound" {
		contract, err := p.timelockRepo.GetCompoundTimeLockByChainAndAddress(ctx, chainID, contractAddress)
		if err != nil {
			return false, nil
		}
		return contract != nil && contract.Status == "active", nil
	} else if standard == "openzeppelin" {
		contract, err := p.timelockRepo.GetOpenzeppelinTimeLockByChainAndAddress(ctx, chainID, contractAddress)
		if err != nil {
			return false, nil
		}
		return contract != nil && contract.Status == "active", nil
	}
	return false, fmt.Errorf("unsupported standard: %s", standard)
}

// sendFlowNotification 异步发送 Flow 通知（邮件 + 渠道）
func (p *WebhookProcessor) sendFlowNotification(chainID int, contractAddress, flowID, standard, statusFrom, statusTo string, txHash *string, initiatorAddress string) {
	ctx := context.Background()

	logger.Info("Sending flow notification (webhook)",
		"chain_id", chainID,
		"contract", contractAddress,
		"flow_id", flowID,
		"standard", standard,
		"from", statusFrom,
		"to", statusTo)

	// 1. 发送邮件通知
	if err := p.emailSvc.SendFlowNotification(ctx, standard, chainID, contractAddress, flowID, statusFrom, statusTo, txHash, initiatorAddress); err != nil {
		logger.Error("Failed to send email notification", err,
			"chain_id", chainID,
			"flow_id", flowID,
			"status_to", statusTo)
	} else {
		logger.Info("Email notification sent successfully", "flow_id", flowID, "status", statusTo)
	}

	// 2. 发送渠道通知（Telegram/Lark/Feishu 等）
	if err := p.notificationSvc.SendFlowNotification(ctx, standard, chainID, contractAddress, flowID, statusFrom, statusTo, txHash, initiatorAddress); err != nil {
		logger.Error("Failed to send channel notification", err,
			"chain_id", chainID,
			"flow_id", flowID,
			"status_to", statusTo)
	} else {
		logger.Info("Channel notification sent successfully", "flow_id", flowID, "status", statusTo)
	}
}
