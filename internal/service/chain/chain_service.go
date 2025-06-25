package chain

import (
	"context"
	"fmt"
	"timelocker-backend/internal/repository/chain"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// Service 支持链服务接口
type Service interface {
	GetSupportChains(ctx context.Context, req *types.GetSupportChainsRequest) (*types.GetSupportChainsResponse, error)
	GetChainByID(ctx context.Context, req *types.GetChainByIDRequest) (*types.SupportChain, error)
	GetChainByChainID(ctx context.Context, req *types.GetChainByChainIDRequest) (*types.SupportChain, error)
}

// service 支持链服务实现
type service struct {
	chainRepo chain.Repository
}

// NewService 创建新的支持链服务
func NewService(chainRepo chain.Repository) Service {
	return &service{
		chainRepo: chainRepo,
	}
}

// GetSupportChains 获取支持链列表
func (s *service) GetSupportChains(ctx context.Context, req *types.GetSupportChainsRequest) (*types.GetSupportChainsResponse, error) {
	logger.Info("GetSupportChains: ", "is_testnet", req.IsTestnet, "is_active", req.IsActive)

	// 调用repository获取数据
	chains, total, err := s.chainRepo.GetSupportChains(ctx, req)
	if err != nil {
		logger.Error("GetSupportChains Error: ", err)
		return nil, fmt.Errorf("failed to get support chains: %w", err)
	}

	response := &types.GetSupportChainsResponse{
		Chains: chains,
		Total:  total,
	}

	logger.Info("GetSupportChains: ", "total", total, "count", len(chains))
	return response, nil
}

// GetChainByID 根据ID获取链信息
func (s *service) GetChainByID(ctx context.Context, req *types.GetChainByIDRequest) (*types.SupportChain, error) {
	logger.Info("GetChainByID: ", "id", req.ID)

	chain, err := s.chainRepo.GetChainByID(ctx, req.ID)
	if err != nil {
		logger.Error("GetChainByID Error: ", err, "id", req.ID)
		return nil, fmt.Errorf("failed to get chain by id: %w", err)
	}

	if chain == nil {
		return nil, nil
	}

	logger.Info("GetChainByID: ", "id", req.ID, "chain_name", chain.ChainName)
	return chain, nil
}

// GetChainByChainID 根据ChainID获取链信息
func (s *service) GetChainByChainID(ctx context.Context, req *types.GetChainByChainIDRequest) (*types.SupportChain, error) {
	logger.Info("GetChainByChainID: ", "chain_id", req.ChainID)

	chain, err := s.chainRepo.GetChainByChainID(ctx, req.ChainID)
	if err != nil {
		logger.Error("GetChainByChainID Error: ", err, "chain_id", req.ChainID)
		return nil, fmt.Errorf("failed to get chain by chain id: %w", err)
	}

	if chain == nil {
		return nil, nil
	}

	logger.Info("GetChainByChainID: ", "chain_id", req.ChainID, "chain_name", chain.ChainName)
	return chain, nil
}
