package token

import (
	"errors"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 支持代币仓库接口
type Repository interface {
	GetAllActiveTokens() ([]*types.SupportToken, error)
	GetTokenBySymbol(symbol string) (*types.SupportToken, error)
	GetTokenByCoingeckoID(coingeckoID string) (*types.SupportToken, error)
	CreateToken(token *types.SupportToken) error
	UpdateToken(token *types.SupportToken) error
	EnableToken(id int64) error
	DisableToken(id int64) error
}

// repository 支持代币仓库实现
type repository struct {
	db *gorm.DB
}

// NewRepository 创建新的支持代币仓库
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// GetAllActiveTokens 获取所有激活的代币
func (r *repository) GetAllActiveTokens() ([]*types.SupportToken, error) {
	var tokens []*types.SupportToken

	err := r.db.Where("is_active = ?", true).Find(&tokens).Error
	if err != nil {
		logger.Error("GetAllActiveTokens Error: ", err)
		return nil, err
	}

	logger.Info("GetAllActiveTokens: ", "count", len(tokens))
	return tokens, nil
}

// GetTokenBySymbol 根据符号获取代币
func (r *repository) GetTokenBySymbol(symbol string) (*types.SupportToken, error) {
	var token types.SupportToken

	err := r.db.Where("symbol = ?", symbol).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("GetTokenBySymbol: ", err, "symbol", symbol)
			return nil, nil
		}
		logger.Error("GetTokenBySymbol Error: ", err, "symbol", symbol)
		return nil, err
	}

	logger.Info("GetTokenBySymbol: ", "symbol", symbol, "found", token.ID)
	return &token, nil
}

// GetTokenByCoingeckoID 根据CoinGecko ID获取代币
func (r *repository) GetTokenByCoingeckoID(coingeckoID string) (*types.SupportToken, error) {
	var token types.SupportToken

	err := r.db.Where("coingecko_id = ?", coingeckoID).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("GetTokenByCoingeckoID: ", err, "coingecko_id", coingeckoID)
			return nil, nil
		}
		logger.Error("GetTokenByCoingeckoID Error: ", err, "coingecko_id", coingeckoID)
		return nil, err
	}

	logger.Info("GetTokenByCoingeckoID: ", "coingecko_id", coingeckoID, "found", token.ID)
	return &token, nil
}

// CreateToken 创建代币
func (r *repository) CreateToken(token *types.SupportToken) error {
	err := r.db.Create(token).Error
	if err != nil {
		logger.Error("CreateToken Error: ", err, "symbol", token.Symbol)
		return err
	}

	logger.Info("CreateToken: ", "symbol", token.Symbol, "id", token.ID)
	return nil
}

// UpdateToken 更新代币
func (r *repository) UpdateToken(token *types.SupportToken) error {
	err := r.db.Save(token).Error
	if err != nil {
		logger.Error("UpdateToken Error: ", err, "id", token.ID)
		return err
	}

	logger.Info("UpdateToken: ", "id", token.ID, "symbol", token.Symbol)
	return nil
}

// EnableToken 启用代币
func (r *repository) EnableToken(id int64) error {
	err := r.db.Model(&types.SupportToken{}).Where("id = ?", id).Update("is_active", true).Error
	if err != nil {
		logger.Error("EnableToken Error: ", err, "id", id)
		return err
	}

	logger.Info("EnableToken: ", "id", id)
	return nil
}

// DisableToken 禁用代币
func (r *repository) DisableToken(id int64) error {
	err := r.db.Model(&types.SupportToken{}).Where("id = ?", id).Update("is_active", false).Error
	if err != nil {
		logger.Error("DisableToken Error: ", err, "id", id)
		return err
	}

	logger.Info("DisableToken: ", "id", id)
	return nil
}
