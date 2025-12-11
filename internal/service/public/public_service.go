package public

import (
	"context"
	"fmt"
	"timelocker-backend/internal/repository/public"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// Service 公共数据服务接口
type Service interface {
	GetStats(ctx context.Context, req *types.GetStatsRequest) (*types.GetStatsResponse, error)
}

// service 公共数据服务实现
type service struct {
	publicRepo public.Repository
}

// NewService 创建新的公共数据服务
func NewService(publicRepo public.Repository) Service {
	return &service{
		publicRepo: publicRepo,
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

	// 从数据库获取总合约数量和总交易数量
	contractCount, err := s.publicRepo.GetTotalContractCount(ctx)
	if err != nil {
		logger.Error("GetStats: failed to get total contract count", err)
		return nil, fmt.Errorf("failed to get total contract count: %w", err)
	}

	transactionCount, err := s.publicRepo.GetTotalTransactionCount(ctx)
	if err != nil {
		logger.Error("GetStats: failed to get total transaction count", err)
		return nil, fmt.Errorf("failed to get total transaction count: %w", err)
	}

	response := &types.GetStatsResponse{
		ChainCount:       chainCount,
		ContractCount:    contractCount,
		TransactionCount: transactionCount,
	}

	logger.Info("GetStats: ", "chains", chainCount, "contracts", contractCount, "transactions", transactionCount)
	return response, nil
}
