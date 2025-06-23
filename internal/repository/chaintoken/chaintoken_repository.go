package chaintoken

import (
	"errors"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 链代币关联仓库接口
type Repository interface {
	GetTokensByChainID(chainID int64) ([]*types.ChainToken, error)
	GetChainTokenByChainAndToken(chainID, tokenID int64) (*types.ChainToken, error)
	GetChainTokenByContractAddress(chainID int64, contractAddress string) (*types.ChainToken, error)
	CreateChainToken(chainToken *types.ChainToken) error
	UpdateChainToken(chainToken *types.ChainToken) error
	EnableChainToken(id int64) error
	DisableChainToken(id int64) error
	GetAllActiveChainTokens() ([]*types.ChainToken, error)
}

// repository 链代币关联仓库实现
type repository struct {
	db *gorm.DB
}

// NewRepository 创建新的链代币关联仓库
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// GetTokensByChainID 根据链ID获取该链上的所有代币
func (r *repository) GetTokensByChainID(chainID int64) ([]*types.ChainToken, error) {
	var chainTokens []*types.ChainToken

	err := r.db.Preload("Token").Preload("Chain").
		Where("chain_id = ? AND is_active = ?", chainID, true).
		Find(&chainTokens).Error
	if err != nil {
		logger.Error("GetTokensByChainID Error: ", err, "chain_id", chainID)
		return nil, err
	}

	logger.Info("GetTokensByChainID: ", "chain_id", chainID, "count", len(chainTokens))
	return chainTokens, nil
}

// GetChainTokenByChainAndToken 根据链ID和代币ID获取链代币关联
func (r *repository) GetChainTokenByChainAndToken(chainID, tokenID int64) (*types.ChainToken, error) {
	var chainToken types.ChainToken

	err := r.db.Preload("Token").Preload("Chain").
		Where("chain_id = ? AND token_id = ?", chainID, tokenID).
		First(&chainToken).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("GetChainTokenByChainAndToken: not found", "chain_id", chainID, "token_id", tokenID)
			return nil, nil
		}
		logger.Error("GetChainTokenByChainAndToken Error: ", err, "chain_id", chainID, "token_id", tokenID)
		return nil, err
	}

	logger.Info("GetChainTokenByChainAndToken: ", "chain_id", chainID, "token_id", tokenID, "found", chainToken.ID)
	return &chainToken, nil
}

// GetChainTokenByContractAddress 根据链ID和合约地址获取链代币关联
func (r *repository) GetChainTokenByContractAddress(chainID int64, contractAddress string) (*types.ChainToken, error) {
	var chainToken types.ChainToken

	err := r.db.Preload("Token").Preload("Chain").
		Where("chain_id = ? AND contract_address = ?", chainID, contractAddress).
		First(&chainToken).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("GetChainTokenByContractAddress: not found", "chain_id", chainID, "contract_address", contractAddress)
			return nil, nil
		}
		logger.Error("GetChainTokenByContractAddress Error: ", err, "chain_id", chainID, "contract_address", contractAddress)
		return nil, err
	}

	logger.Info("GetChainTokenByContractAddress: ", "chain_id", chainID, "contract_address", contractAddress, "found", chainToken.ID)
	return &chainToken, nil
}

// CreateChainToken 创建链代币关联
func (r *repository) CreateChainToken(chainToken *types.ChainToken) error {
	err := r.db.Create(chainToken).Error
	if err != nil {
		logger.Error("CreateChainToken Error: ", err, "chain_id", chainToken.ChainID, "token_id", chainToken.TokenID)
		return err
	}

	logger.Info("CreateChainToken: ", "chain_id", chainToken.ChainID, "token_id", chainToken.TokenID, "id", chainToken.ID)
	return nil
}

// UpdateChainToken 更新链代币关联
func (r *repository) UpdateChainToken(chainToken *types.ChainToken) error {
	err := r.db.Save(chainToken).Error
	if err != nil {
		logger.Error("UpdateChainToken Error: ", err, "id", chainToken.ID)
		return err
	}

	logger.Info("UpdateChainToken: ", "id", chainToken.ID, "chain_id", chainToken.ChainID, "token_id", chainToken.TokenID)
	return nil
}

// EnableChainToken 启用链代币关联
func (r *repository) EnableChainToken(id int64) error {
	err := r.db.Model(&types.ChainToken{}).Where("id = ?", id).Update("is_active", true).Error
	if err != nil {
		logger.Error("EnableChainToken Error: ", err, "id", id)
		return err
	}

	logger.Info("EnableChainToken: ", "id", id)
	return nil
}

// DisableChainToken 禁用链代币关联
func (r *repository) DisableChainToken(id int64) error {
	err := r.db.Model(&types.ChainToken{}).Where("id = ?", id).Update("is_active", false).Error
	if err != nil {
		logger.Error("DisableChainToken Error: ", err, "id", id)
		return err
	}

	logger.Info("DisableChainToken: ", "id", id)
	return nil
}

// GetAllActiveChainTokens 获取所有激活的链代币关联
func (r *repository) GetAllActiveChainTokens() ([]*types.ChainToken, error) {
	var chainTokens []*types.ChainToken

	err := r.db.Preload("Token").Preload("Chain").
		Where("is_active = ?", true).
		Find(&chainTokens).Error
	if err != nil {
		logger.Error("GetAllActiveChainTokens Error: ", err)
		return nil, err
	}

	logger.Info("GetAllActiveChainTokens: ", "count", len(chainTokens))
	return chainTokens, nil
}
