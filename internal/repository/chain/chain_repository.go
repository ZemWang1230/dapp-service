package chain

import (
	"errors"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 支持链仓库接口
type Repository interface {
	GetAllActiveChains() ([]*types.SupportChain, error)
	GetChainByChainName(chainName string) (*types.SupportChain, error)
	GetActiveMainnetChains() ([]*types.SupportChain, error)
	GetActiveTestnetChains() ([]*types.SupportChain, error)
}

// repository 支持链仓库实现
type repository struct {
	db *gorm.DB
}

// NewRepository 创建新的支持链仓库
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// GetAllActiveChains 获取所有激活的链
func (r *repository) GetAllActiveChains() ([]*types.SupportChain, error) {
	var chains []*types.SupportChain

	err := r.db.Where("is_active = ?", true).Find(&chains).Error
	if err != nil {
		logger.Error("GetAllActiveChains Error: ", err)
		return nil, err
	}

	logger.Info("GetAllActiveChains: ", "count", len(chains))
	return chains, nil
}

// GetChainByChainName 根据链名称获取链信息
func (r *repository) GetChainByChainName(chainName string) (*types.SupportChain, error) {
	var chain types.SupportChain

	err := r.db.Where("chain_name = ?", chainName).First(&chain).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("GetChainByChainName: chain not found", "chain_name", chainName)
			return nil, nil
		}
		logger.Error("GetChainByChainName Error: ", err, "chain_name", chainName)
		return nil, err
	}

	logger.Info("GetChainByChainName: ", "chain_name", chainName, "found", chain.ID)
	return &chain, nil
}

// GetActiveMainnetChains 获取所有激活的主网链
func (r *repository) GetActiveMainnetChains() ([]*types.SupportChain, error) {
	var chains []*types.SupportChain

	err := r.db.Where("is_active = ? AND is_testnet = ?", true, false).Find(&chains).Error
	if err != nil {
		logger.Error("GetActiveMainnetChains Error: ", err)
		return nil, err
	}

	logger.Info("GetActiveMainnetChains: ", "count", len(chains))
	return chains, nil
}

// GetActiveTestnetChains 获取所有激活的测试网链
func (r *repository) GetActiveTestnetChains() ([]*types.SupportChain, error) {
	var chains []*types.SupportChain

	err := r.db.Where("is_active = ? AND is_testnet = ?", true, true).Find(&chains).Error
	if err != nil {
		logger.Error("GetActiveTestnetChains Error: ", err)
		return nil, err
	}

	logger.Info("GetActiveTestnetChains: ", "count", len(chains))
	return chains, nil
}
