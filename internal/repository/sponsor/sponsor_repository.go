package sponsor

import (
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Repository 赞助方存储库接口
type Repository interface {
	// 创建赞助方
	Create(sponsor *types.Sponsor) error
	// 根据ID获取赞助方
	GetByID(id int64) (*types.Sponsor, error)
	// 更新赞助方
	Update(id int64, updates map[string]interface{}) error
	// 删除赞助方（软删除）
	Delete(id int64) error
	// 获取赞助方列表（分页、筛选）
	GetList(req *types.GetSponsorsRequest) ([]*types.Sponsor, int64, error)
	// 获取所有激活的赞助方（公开接口使用）
	GetAllActive() ([]*types.Sponsor, error)
	// 根据类型获取激活的赞助方
	GetActiveByType(sponsorType types.SponsorType) ([]*types.Sponsor, error)
}

// repository 赞助方存储库实现
type repository struct {
	db *gorm.DB
}

// NewRepository 创建新的赞助方存储库
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// Create 创建赞助方
func (r *repository) Create(sponsor *types.Sponsor) error {
	err := r.db.Create(sponsor).Error
	if err != nil {
		logger.Error("Create sponsor error", err, "sponsor", sponsor)
		return err
	}

	logger.Info("Created sponsor", "id", sponsor.ID, "name", sponsor.Name, "type", sponsor.Type)
	return nil
}

// GetByID 根据ID获取赞助方
func (r *repository) GetByID(id int64) (*types.Sponsor, error) {
	var sponsor types.Sponsor
	err := r.db.Where("id = ?", id).First(&sponsor).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Info("Sponsor not found", "id", id)
			return nil, nil
		}
		logger.Error("GetByID sponsor error", err, "id", id)
		return nil, err
	}

	logger.Info("Got sponsor by ID", "id", id, "name", sponsor.Name)
	return &sponsor, nil
}

// Update 更新赞助方
func (r *repository) Update(id int64, updates map[string]interface{}) error {
	result := r.db.Model(&types.Sponsor{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		logger.Error("Update sponsor error", result.Error, "id", id, "updates", updates)
		return result.Error
	}

	if result.RowsAffected == 0 {
		logger.Warn("No sponsor updated", "id", id)
		return gorm.ErrRecordNotFound
	}

	logger.Info("Updated sponsor", "id", id, "affected_rows", result.RowsAffected)
	return nil
}

// Delete 删除赞助方（软删除 - 设置为不激活）
func (r *repository) Delete(id int64) error {
	result := r.db.Model(&types.Sponsor{}).Where("id = ?", id).Update("is_active", false)
	if result.Error != nil {
		logger.Error("Delete sponsor error", result.Error, "id", id)
		return result.Error
	}

	if result.RowsAffected == 0 {
		logger.Warn("No sponsor deleted", "id", id)
		return gorm.ErrRecordNotFound
	}

	logger.Info("Deleted sponsor", "id", id)
	return nil
}

// GetList 获取赞助方列表（分页、筛选）
func (r *repository) GetList(req *types.GetSponsorsRequest) ([]*types.Sponsor, int64, error) {
	var sponsors []*types.Sponsor
	var total int64

	// 构建查询
	query := r.db.Model(&types.Sponsor{})

	// 过滤条件
	if req.Type != nil {
		query = query.Where("type = ?", *req.Type)
	}
	if req.IsActive != nil {
		query = query.Where("is_active = ?", *req.IsActive)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		logger.Error("GetList count error", err, "request", req)
		return nil, 0, err
	}

	// 设置默认分页参数
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Order("sort_order DESC, created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&sponsors).Error

	if err != nil {
		logger.Error("GetList sponsors error", err, "request", req)
		return nil, 0, err
	}

	logger.Info("Got sponsors list", "total", total, "page", page, "page_size", pageSize, "count", len(sponsors))
	return sponsors, total, nil
}

// GetAllActive 获取所有激活的赞助方（公开接口使用）
func (r *repository) GetAllActive() ([]*types.Sponsor, error) {
	var sponsors []*types.Sponsor
	err := r.db.Where("is_active = ?", true).
		Order("sort_order DESC, created_at DESC").
		Find(&sponsors).Error

	if err != nil {
		logger.Error("GetAllActive sponsors error", err)
		return nil, err
	}

	logger.Info("Got all active sponsors", "count", len(sponsors))
	return sponsors, nil
}

// GetActiveByType 根据类型获取激活的赞助方
func (r *repository) GetActiveByType(sponsorType types.SponsorType) ([]*types.Sponsor, error) {
	var sponsors []*types.Sponsor
	err := r.db.Where("is_active = ? AND type = ?", true, sponsorType).
		Order("sort_order DESC, created_at DESC").
		Find(&sponsors).Error

	if err != nil {
		logger.Error("GetActiveByType sponsors error", err, "type", sponsorType)
		return nil, err
	}

	logger.Info("Got active sponsors by type", "type", sponsorType, "count", len(sponsors))
	return sponsors, nil
}
