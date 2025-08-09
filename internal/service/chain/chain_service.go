package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"timelocker-backend/internal/repository/chain"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// Service 支持链服务接口
type Service interface {
	GetSupportChains(ctx context.Context, req *types.GetSupportChainsRequest) (*types.GetSupportChainsResponse, error)
	GetChainByChainID(ctx context.Context, chainID int64) (*types.SupportChain, error)
	GetWalletChainConfig(ctx context.Context, chainID int64) (*types.WalletChainConfig, error)
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

// GetChainByChainID 根据ChainID获取链信息
func (s *service) GetChainByChainID(ctx context.Context, chainID int64) (*types.SupportChain, error) {
	logger.Info("GetChainByChainID start: ", "chain_id", chainID)

	chain, err := s.chainRepo.GetChainByChainID(ctx, chainID)
	if err != nil {
		logger.Error("GetChainByChainID Error: ", err, "chain_id", chainID)
		return nil, err
	}

	logger.Info("GetChainByChainID success: ", "chain_id", chainID, "chain_name", chain.ChainName)
	return chain, nil
}

// GetWalletChainConfig 获取钱包插件添加链的配置数据
func (s *service) GetWalletChainConfig(ctx context.Context, chainID int64) (*types.WalletChainConfig, error) {
	logger.Info("GetWalletChainConfig start: ", "chain_id", chainID)

	// 获取链信息
	chain, err := s.chainRepo.GetChainByChainID(ctx, chainID)
	if err != nil {
		logger.Error("GetWalletChainConfig Error: ", err, "chain_id", chainID)
		return nil, err
	}

	// 解析官方RPC URLs
	var officialRPCs []string
	if err := json.Unmarshal([]byte(chain.OfficialRPCUrls), &officialRPCs); err != nil {
		logger.Error("GetWalletChainConfig: failed to parse official RPC URLs", err, "chain_id", chainID)
		// 使用默认值
		officialRPCs = []string{}
	}

	// 解析区块浏览器URLs
	var blockExplorers []string
	if err := json.Unmarshal([]byte(chain.BlockExplorerUrls), &blockExplorers); err != nil {
		logger.Error("GetWalletChainConfig: failed to parse block explorer URLs", err, "chain_id", chainID)
		// 使用默认值
		blockExplorers = []string{}
	}

	// 构建钱包配置
	config := &types.WalletChainConfig{
		ChainID:   fmt.Sprintf("0x%X", chain.ChainID), // 转换为十六进制格式
		ChainName: chain.DisplayName,
		NativeCurrency: types.NativeCurrencyConfig{
			Name:     chain.NativeCurrencyName,
			Symbol:   chain.NativeCurrencySymbol,
			Decimals: chain.NativeCurrencyDecimals,
		},
		RPCUrls:           officialRPCs,
		BlockExplorerUrls: blockExplorers,
	}

	logger.Info("GetWalletChainConfig success: ", "chain_id", chainID, "chain_name", config.ChainName)
	return config, nil
}
