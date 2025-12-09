package flow

import (
	"context"
	"fmt"
	"strings"

	chainRepo "timelocker-backend/internal/repository/chain"
	goldskyRepo "timelocker-backend/internal/repository/goldsky"
	"timelocker-backend/internal/service/goldsky"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/utils"
)

// FlowService 流程服务接口
type FlowService interface {
	// 获取与用户相关的流程列表
	GetCompoundFlowList(ctx context.Context, userAddress string, req *types.GetCompoundFlowListRequest) (*types.GetCompoundFlowListResponse, error)

	// 获取与用户相关的流程数量统计
	GetCompoundFlowListCount(ctx context.Context, userAddress string, req *types.GetCompoundFlowListCountRequest) (*types.GetCompoundFlowListCountResponse, error)

	// 获取交易详情
	GetCompoundTransactionDetail(ctx context.Context, req *types.GetTransactionDetailRequest) (*types.GetTransactionDetailResponse, error)
}

// flowService 流程服务实现
type flowService struct {
	flowRepo   goldskyRepo.FlowRepository
	chainRepo  chainRepo.Repository
	goldskySvc *goldsky.GoldskyService
}

// NewFlowService 创建流程服务实例
func NewFlowService(flowRepo goldskyRepo.FlowRepository, chainRepo chainRepo.Repository, goldskySvc *goldsky.GoldskyService) FlowService {
	return &flowService{
		flowRepo:   flowRepo,
		chainRepo:  chainRepo,
		goldskySvc: goldskySvc,
	}
}

// GetFlowList 获取与用户相关的流程列表
func (s *flowService) GetCompoundFlowList(ctx context.Context, userAddress string, req *types.GetCompoundFlowListRequest) (*types.GetCompoundFlowListResponse, error) {
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

	flows, total, err := s.flowRepo.GetUserRelatedCompoundFlows(ctx, userAddress, req.Status, req.Standard, offset, pageSize)
	if err != nil {
		logger.Error("Failed to get user related compound flows", err, "user", userAddress)
		return nil, fmt.Errorf("failed to get user related compound flows: %w", err)
	}

	return &types.GetCompoundFlowListResponse{
		Flows: flows,
		Total: total,
	}, nil
}

// GetCompoundFlowListCount 获取与用户相关的流程数量统计
func (s *flowService) GetCompoundFlowListCount(ctx context.Context, userAddress string, req *types.GetCompoundFlowListCountRequest) (*types.GetCompoundFlowListCountResponse, error) {

	// 调用repository层获取数量统计
	flowCount, err := s.flowRepo.GetUserRelatedCompoundFlowsCount(ctx, userAddress, req.Standard)
	if err != nil {
		logger.Error("Failed to get user related compound flows count", err, "user", userAddress)
		return nil, fmt.Errorf("failed to get user related compound flows count: %w", err)
	}

	return &types.GetCompoundFlowListCountResponse{
		FlowCount: *flowCount,
	}, nil
}

// GetTransactionDetail 获取交易详情
func (s *flowService) GetCompoundTransactionDetail(ctx context.Context, req *types.GetTransactionDetailRequest) (*types.GetTransactionDetailResponse, error) {
	// 标准化
	req.Standard = strings.ToLower(strings.TrimSpace(req.Standard))
	req.TxHash = strings.TrimSpace(req.TxHash)
	if req.Standard != "compound" && req.Standard != "openzeppelin" {
		return nil, fmt.Errorf("invalid standard: %s", req.Standard)
	}
	if !utils.IsValidTxHash(req.TxHash) {
		return nil, fmt.Errorf("invalid tx hash")
	}

	// 需要 chainID 从 request 中获取
	if req.ChainID == 0 {
		return nil, fmt.Errorf("chain_id is required")
	}

	detail, err := s.goldskySvc.GetTransactionDetail(ctx, req.ChainID, req.Standard, req.TxHash)
	if err != nil {
		logger.Error("Failed to get transaction detail", err, "standard", req.Standard, "tx_hash", req.TxHash, "chain_id", req.ChainID)
		return nil, fmt.Errorf("failed to get transaction detail: %w", err)
	}

	if detail == nil {
		return nil, fmt.Errorf("transaction not found")
	}

	return &types.GetTransactionDetailResponse{
		Detail: *detail,
	}, nil
}
