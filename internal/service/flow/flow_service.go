package flow

import (
	"context"
	"fmt"
	"time"

	"timelocker-backend/internal/repository/scanner"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// FlowService 流程服务接口
type FlowService interface {
	// 获取不同状态的交易列表
	GetWaitingFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error)
	GetReadyFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error)
	GetExecutedFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error)
	GetCancelledFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error)
	GetExpiredFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error)

	// 根据发起人获取交易列表
	GetUserFlows(ctx context.Context, initiatorAddress string, status string, page, pageSize int) (*types.FlowListResponse, error)

	// 获取特定流程详细信息
	GetFlowDetail(ctx context.Context, flowID, timelockStandard string, chainID int, contractAddress string) (*types.FlowDetailResponse, error)

	// 获取流程统计信息
	GetFlowStats(ctx context.Context, initiatorAddress *string) (*types.FlowStatsResponse, error)
}

// flowService 流程服务实现
type flowService struct {
	flowRepo scanner.FlowRepository
}

// NewFlowService 创建流程服务实例
func NewFlowService(flowRepo scanner.FlowRepository) FlowService {
	return &flowService{
		flowRepo: flowRepo,
	}
}

// GetWaitingFlows 获取等待中的流程
func (s *flowService) GetWaitingFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error) {
	return s.getFlowsByStatus(ctx, "waiting", page, pageSize)
}

// GetReadyFlows 获取准备执行的流程
func (s *flowService) GetReadyFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error) {
	return s.getFlowsByStatus(ctx, "ready", page, pageSize)
}

// GetExecutedFlows 获取已执行的流程
func (s *flowService) GetExecutedFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error) {
	return s.getFlowsByStatus(ctx, "executed", page, pageSize)
}

// GetCancelledFlows 获取已取消的流程
func (s *flowService) GetCancelledFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error) {
	return s.getFlowsByStatus(ctx, "cancelled", page, pageSize)
}

// GetExpiredFlows 获取已过期的流程
func (s *flowService) GetExpiredFlows(ctx context.Context, page, pageSize int) (*types.FlowListResponse, error) {
	return s.getFlowsByStatus(ctx, "expired", page, pageSize)
}

// GetUserFlows 获取用户的流程列表
func (s *flowService) GetUserFlows(ctx context.Context, initiatorAddress string, status string, page, pageSize int) (*types.FlowListResponse, error) {
	// 验证状态参数
	validStatuses := []string{"waiting", "ready", "executed", "cancelled", "expired"}
	isValidStatus := false
	for _, validStatus := range validStatuses {
		if status == validStatus {
			isValidStatus = true
			break
		}
	}
	if !isValidStatus {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	flows, total, err := s.flowRepo.GetFlowsByStatusAndInitiator(ctx, status, initiatorAddress, page, pageSize)
	if err != nil {
		logger.Error("Failed to get user flows", err, "initiator", initiatorAddress, "status", status)
		return nil, fmt.Errorf("failed to get user flows: %w", err)
	}

	// 转换为响应格式
	flowResponses := make([]types.FlowResponse, len(flows))
	for i, flow := range flows {
		flowResponses[i] = s.convertToFlowResponse(flow)
	}

	return &types.FlowListResponse{
		Flows: flowResponses,
		Total: total,
	}, nil
}

// GetFlowDetail 获取流程详细信息
func (s *flowService) GetFlowDetail(ctx context.Context, flowID, timelockStandard string, chainID int, contractAddress string) (*types.FlowDetailResponse, error) {
	flow, err := s.flowRepo.GetFlowDetail(ctx, flowID, timelockStandard, chainID, contractAddress)
	if err != nil {
		logger.Error("Failed to get flow detail", err, "flow_id", flowID)
		return nil, fmt.Errorf("failed to get flow detail: %w", err)
	}

	if flow == nil {
		return nil, fmt.Errorf("flow not found")
	}

	// 计算时间相关信息
	var timeToExecution *int64
	var timeToExpiration *int64
	now := time.Now()

	if flow.Eta != nil {
		if flow.Status == "waiting" && flow.Eta.After(now) {
			duration := flow.Eta.Sub(now)
			seconds := int64(duration.Seconds())
			timeToExecution = &seconds
		}
	}

	if flow.ExpiredAt != nil && flow.TimelockStandard == "compound" {
		if (flow.Status == "waiting" || flow.Status == "ready") && flow.ExpiredAt.After(now) {
			duration := flow.ExpiredAt.Sub(now)
			seconds := int64(duration.Seconds())
			timeToExpiration = &seconds
		}
	}

	return &types.FlowDetailResponse{
		Flow:             s.convertToFlowResponse(*flow),
		TimeToExecution:  timeToExecution,
		TimeToExpiration: timeToExpiration,
	}, nil
}

// GetFlowStats 获取流程统计信息
func (s *flowService) GetFlowStats(ctx context.Context, initiatorAddress *string) (*types.FlowStatsResponse, error) {
	stats := &types.FlowStatsResponse{}

	statuses := []string{"waiting", "ready", "executed", "cancelled", "expired"}
	for _, status := range statuses {
		var count int64
		var err error

		if initiatorAddress != nil {
			_, count, err = s.flowRepo.GetFlowsByStatusAndInitiator(ctx, status, *initiatorAddress, 1, 1)
		} else {
			_, count, err = s.flowRepo.GetFlowsByStatus(ctx, status, 1, 1)
		}

		if err != nil {
			logger.Error("Failed to get flow stats", err, "status", status)
			return nil, fmt.Errorf("failed to get flow stats: %w", err)
		}

		switch status {
		case "waiting":
			stats.WaitingCount = count
		case "ready":
			stats.ReadyCount = count
		case "executed":
			stats.ExecutedCount = count
		case "cancelled":
			stats.CancelledCount = count
		case "expired":
			stats.ExpiredCount = count
		}
	}

	stats.TotalCount = stats.WaitingCount + stats.ReadyCount + stats.ExecutedCount + stats.CancelledCount + stats.ExpiredCount

	return stats, nil
}

// getFlowsByStatus 根据状态获取流程列表（内部方法）
func (s *flowService) getFlowsByStatus(ctx context.Context, status string, page, pageSize int) (*types.FlowListResponse, error) {
	flows, total, err := s.flowRepo.GetFlowsByStatus(ctx, status, page, pageSize)
	if err != nil {
		logger.Error("Failed to get flows by status", err, "status", status)
		return nil, fmt.Errorf("failed to get flows by status: %w", err)
	}

	// 转换为响应格式
	flowResponses := make([]types.FlowResponse, len(flows))
	for i, flow := range flows {
		flowResponses[i] = s.convertToFlowResponse(flow)
	}

	return &types.FlowListResponse{
		Flows: flowResponses,
		Total: total,
	}, nil
}

// convertToFlowResponse 转换为流程响应格式
func (s *flowService) convertToFlowResponse(flow types.TimelockTransactionFlow) types.FlowResponse {
	response := types.FlowResponse{
		ID:               flow.ID,
		FlowID:           flow.FlowID,
		TimelockStandard: flow.TimelockStandard,
		ChainID:          flow.ChainID,
		ContractAddress:  flow.ContractAddress,
		Status:           flow.Status,
		InitiatorAddress: flow.InitiatorAddress,
		TargetAddress:    flow.TargetAddress,
		Value:            flow.Value,
		Eta:              flow.Eta,
		ExpiredAt:        flow.ExpiredAt,
		CreatedAt:        flow.CreatedAt,
		UpdatedAt:        flow.UpdatedAt,
	}

	// 设置交易哈希
	if flow.QueueTxHash != "" {
		response.QueueTxHash = &flow.QueueTxHash
	}
	if flow.ExecuteTxHash != "" {
		response.ExecuteTxHash = &flow.ExecuteTxHash
	}
	if flow.CancelTxHash != "" {
		response.CancelTxHash = &flow.CancelTxHash
	}

	// 设置时间戳
	response.ExecutedAt = flow.ExecutedAt
	response.CancelledAt = flow.CancelledAt

	// 设置调用数据（转换为十六进制字符串）
	if len(flow.CallData) > 0 {
		callDataHex := fmt.Sprintf("0x%x", flow.CallData)
		response.CallDataHex = &callDataHex
	}

	return response
}
