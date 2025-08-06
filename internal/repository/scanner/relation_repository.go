package scanner

import (
	"context"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// RelationRepository 用户关联仓库接口
type RelationRepository interface {
	CreateRelation(ctx context.Context, relation *types.UserTimelockRelation) error
	GetRelationsByUser(ctx context.Context, userAddress string, chainID *int) ([]types.UserTimelockRelation, error)
	GetRelationsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.UserTimelockRelation, error)
	UpdateRelationStatus(ctx context.Context, userAddress string, chainID int, contractAddress, relationType string, isActive bool) error
	BatchCreateRelations(ctx context.Context, relations []types.UserTimelockRelation) error
	GetActiveRelationsByUser(ctx context.Context, userAddress string, chainID *int) ([]types.UserTimelockRelation, error)
}

type relationRepository struct {
	db *gorm.DB
}

// NewRelationRepository 创建新的用户关联仓库
func NewRelationRepository(db *gorm.DB) RelationRepository {
	return &relationRepository{
		db: db,
	}
}

// CreateRelation 创建用户关联关系
func (r *relationRepository) CreateRelation(ctx context.Context, relation *types.UserTimelockRelation) error {
	// 使用 UPSERT 避免重复插入
	err := r.db.WithContext(ctx).
		Where("user_address = ? AND chain_id = ? AND contract_address = ? AND relation_type = ?",
			relation.UserAddress, relation.ChainID, relation.ContractAddress, relation.RelationType).
		Assign(map[string]interface{}{
			"timelock_standard": relation.TimelockStandard,
			"related_at":        relation.RelatedAt,
			"is_active":         relation.IsActive,
		}).
		FirstOrCreate(relation).Error

	if err != nil {
		logger.Error("CreateRelation Error", err, "user", relation.UserAddress, "type", relation.RelationType)
		return err
	}

	return nil
}

// GetRelationsByUser 获取用户的所有关联关系
func (r *relationRepository) GetRelationsByUser(ctx context.Context, userAddress string, chainID *int) ([]types.UserTimelockRelation, error) {
	var relations []types.UserTimelockRelation
	query := r.db.WithContext(ctx).Where("user_address = ?", userAddress)

	if chainID != nil {
		query = query.Where("chain_id = ?", *chainID)
	}

	err := query.Order("created_at DESC").Find(&relations).Error
	if err != nil {
		logger.Error("GetRelationsByUser Error", err, "user", userAddress)
		return nil, err
	}

	return relations, nil
}

// GetActiveRelationsByUser 获取用户的活跃关联关系
func (r *relationRepository) GetActiveRelationsByUser(ctx context.Context, userAddress string, chainID *int) ([]types.UserTimelockRelation, error) {
	var relations []types.UserTimelockRelation
	query := r.db.WithContext(ctx).
		Where("user_address = ? AND is_active = ?", userAddress, true)

	if chainID != nil {
		query = query.Where("chain_id = ?", *chainID)
	}

	err := query.Order("created_at DESC").Find(&relations).Error
	if err != nil {
		logger.Error("GetActiveRelationsByUser Error", err, "user", userAddress)
		return nil, err
	}

	return relations, nil
}

// GetRelationsByContract 获取合约的所有关联关系
func (r *relationRepository) GetRelationsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.UserTimelockRelation, error) {
	var relations []types.UserTimelockRelation
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ?", chainID, contractAddress).
		Order("created_at DESC").
		Find(&relations).Error

	if err != nil {
		logger.Error("GetRelationsByContract Error", err, "chain_id", chainID, "contract", contractAddress)
		return nil, err
	}

	return relations, nil
}

// UpdateRelationStatus 更新关联关系状态
func (r *relationRepository) UpdateRelationStatus(ctx context.Context, userAddress string, chainID int, contractAddress, relationType string, isActive bool) error {
	err := r.db.WithContext(ctx).
		Model(&types.UserTimelockRelation{}).
		Where("user_address = ? AND chain_id = ? AND contract_address = ? AND relation_type = ?",
			userAddress, chainID, contractAddress, relationType).
		Update("is_active", isActive).Error

	if err != nil {
		logger.Error("UpdateRelationStatus Error", err, "user", userAddress, "type", relationType)
		return err
	}

	return nil
}

// BatchCreateRelations 批量创建用户关联关系
func (r *relationRepository) BatchCreateRelations(ctx context.Context, relations []types.UserTimelockRelation) error {
	if len(relations) == 0 {
		return nil
	}

	// 使用批量插入，忽略重复
	err := r.db.WithContext(ctx).
		CreateInBatches(&relations, 100).Error

	if err != nil {
		logger.Error("BatchCreateRelations Error", err, "count", len(relations))
		return err
	}

	return nil
}
