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
	GetChainByChainID(chainID int64) (*types.SupportChain, error)
	CreateChain(chain *types.SupportChain) error
	UpdateChain(chain *types.SupportChain) error
	EnableChain(id int64) error
	DisableChain(id int64) error
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

// GetChainByChainID 根据链ID获取链信息
func (r *repository) GetChainByChainID(chainID int64) (*types.SupportChain, error) {
	var chain types.SupportChain

	err := r.db.Where("chain_id = ?", chainID).First(&chain).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("GetChainByChainID: chain not found", "chain_id", chainID)
			return nil, nil
		}
		logger.Error("GetChainByChainID Error: ", err, "chain_id", chainID)
		return nil, err
	}

	logger.Info("GetChainByChainID: ", "chain_id", chainID, "found", chain.ID)
	return &chain, nil
}

// CreateChain 创建链
func (r *repository) CreateChain(chain *types.SupportChain) error {
	err := r.db.Create(chain).Error
	if err != nil {
		logger.Error("CreateChain Error: ", err, "chain_id", chain.ChainID, "name", chain.Name)
		return err
	}

	logger.Info("CreateChain: ", "chain_id", chain.ChainID, "name", chain.Name, "id", chain.ID)
	return nil
}

// UpdateChain 更新链
func (r *repository) UpdateChain(chain *types.SupportChain) error {
	err := r.db.Save(chain).Error
	if err != nil {
		logger.Error("UpdateChain Error: ", err, "id", chain.ID)
		return err
	}

	logger.Info("UpdateChain: ", "id", chain.ID, "chain_id", chain.ChainID, "name", chain.Name)
	return nil
}

// EnableChain 启用链
func (r *repository) EnableChain(id int64) error {
	err := r.db.Model(&types.SupportChain{}).Where("id = ?", id).Update("is_active", true).Error
	if err != nil {
		logger.Error("EnableChain Error: ", err, "id", id)
		return err
	}

	logger.Info("EnableChain: ", "id", id)
	return nil
}

// DisableChain 禁用链
func (r *repository) DisableChain(id int64) error {
	err := r.db.Model(&types.SupportChain{}).Where("id = ?", id).Update("is_active", false).Error
	if err != nil {
		logger.Error("DisableChain Error: ", err, "id", id)
		return err
	}

	logger.Info("DisableChain: ", "id", id)
	return nil
}
