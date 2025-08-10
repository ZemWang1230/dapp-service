package flow

import (
	"context"
	"fmt"

	"timelocker-backend/internal/repository/scanner"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// FlowService 流程服务接口
type FlowService interface {
	// 获取与用户相关的流程列表
	GetFlowList(ctx context.Context, userAddress string, req *types.GetFlowListRequest) (*types.GetFlowListResponse, error)

	// 获取交易详情
	GetTransactionDetail(ctx context.Context, req *types.GetTransactionDetailRequest) (*types.GetTransactionDetailResponse, error)
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

// GetFlowList 获取与用户相关的流程列表
func (s *flowService) GetFlowList(ctx context.Context, userAddress string, req *types.GetFlowListRequest) (*types.GetFlowListResponse, error) {
	// 验证状态参数
	if req.Status != nil {
		validStatuses := []string{"all", "waiting", "ready", "executed", "cancelled", "expired"}
		isValidStatus := false
		for _, validStatus := range validStatuses {
			if *req.Status == validStatus {
				isValidStatus = true
				break
			}
		}
		if !isValidStatus {
			return nil, fmt.Errorf("invalid status: %s", *req.Status)
		}
	}

	// 验证标准参数
	if req.Standard != nil {
		validStandards := []string{"compound", "openzeppelin"}
		isValidStandard := false
		for _, validStandard := range validStandards {
			if *req.Standard == validStandard {
				isValidStandard = true
				break
			}
		}
		if !isValidStandard {
			return nil, fmt.Errorf("invalid standard: %s", *req.Standard)
		}
	}

	// 计算分页
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	flows, total, err := s.flowRepo.GetUserRelatedFlows(ctx, userAddress, req.Status, req.Standard, offset, pageSize)
	if err != nil {
		logger.Error("Failed to get user related flows", err, "user", userAddress)
		return nil, fmt.Errorf("failed to get user related flows: %w", err)
	}

	// 转换为响应格式
	flowResponses := make([]types.FlowResponse, len(flows))
	for i, flow := range flows {
		flowResponses[i] = s.convertToFlowResponse(flow)
	}

	return &types.GetFlowListResponse{
		Flows: flowResponses,
		Total: total,
	}, nil
}

// GetTransactionDetail 获取交易详情
func (s *flowService) GetTransactionDetail(ctx context.Context, req *types.GetTransactionDetailRequest) (*types.GetTransactionDetailResponse, error) {
	detail, err := s.flowRepo.GetTransactionDetail(ctx, req.Standard, req.TxHash)
	if err != nil {
		logger.Error("Failed to get transaction detail", err, "standard", req.Standard, "tx_hash", req.TxHash)
		return nil, fmt.Errorf("failed to get transaction detail: %w", err)
	}

	if detail == nil {
		return nil, fmt.Errorf("transaction not found")
	}

	return &types.GetTransactionDetailResponse{
		Detail: *detail,
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
