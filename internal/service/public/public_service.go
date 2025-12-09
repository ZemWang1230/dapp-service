package public

import (
	"context"
	"fmt"
	goldskyRepo "timelocker-backend/internal/repository/goldsky"
	"timelocker-backend/internal/repository/public"
	goldskySvc "timelocker-backend/internal/service/goldsky"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// Service 公共数据服务接口
type Service interface {
	GetStats(ctx context.Context, req *types.GetStatsRequest) (*types.GetStatsResponse, error)
}

// service 公共数据服务实现
type service struct {
	publicRepo  public.Repository
	goldskyRepo goldskyRepo.FlowRepository
	goldskySvc  *goldskySvc.GoldskyService
}

// NewService 创建新的公共数据服务
func NewService(publicRepo public.Repository, goldskyRepo goldskyRepo.FlowRepository, goldskySvc *goldskySvc.GoldskyService) Service {
	return &service{
		publicRepo:  publicRepo,
		goldskyRepo: goldskyRepo,
		goldskySvc:  goldskySvc,
	}
}

// GetStats 获取统计数据
func (s *service) GetStats(ctx context.Context, req *types.GetStatsRequest) (*types.GetStatsResponse, error) {
	logger.Info("GetStats start")

	// 获取链数量（active的）
	chainCount, err := s.publicRepo.GetChainCount(ctx)
	if err != nil {
		logger.Error("GetStats: failed to get chain count", err)
		return nil, fmt.Errorf("failed to get chain count: %w", err)
	}

	// 获取合约数量（Compound+OZ，每个链上的，从goldsky中获取）
	contractCount, err := s.goldskySvc.GetGlobalContractCount(ctx)
	if err != nil {
		logger.Error("GetStats: failed to get contract count", err)
		return nil, fmt.Errorf("failed to get contract count: %w", err)
	}

	// 获取交易数量（compound+OZ的，每个链上的，从goldsky中获取）
	transactionCount, err := s.goldskySvc.GetGlobalTransactionCount(ctx)
	if err != nil {
		logger.Error("GetStats: failed to get transaction count", err)
		return nil, fmt.Errorf("failed to get transaction count: %w", err)
	}

	response := &types.GetStatsResponse{
		ChainCount:       chainCount,
		ContractCount:    contractCount,
		TransactionCount: transactionCount,
	}

	logger.Info("GetStats: ", "chains", chainCount, "contracts", contractCount, "transactions", transactionCount)
	return response, nil
}
