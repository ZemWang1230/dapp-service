package sponsor

import (
	"fmt"
	"timelocker-backend/internal/repository/sponsor"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// Service 赞助方服务接口
type Service interface {
	// 创建赞助方
	CreateSponsor(req *types.CreateSponsorRequest) (*types.Sponsor, error)
	// 根据ID获取赞助方
	GetSponsorByID(id int64) (*types.Sponsor, error)
	// 更新赞助方
	UpdateSponsor(id int64, req *types.UpdateSponsorRequest) (*types.Sponsor, error)
	// 删除赞助方
	DeleteSponsor(id int64) error
	// 获取赞助方列表（管理接口）
	GetSponsorsList(req *types.GetSponsorsRequest) (*types.GetSponsorsResponse, error)
	// 获取公开的赞助方列表（公开接口）
	GetPublicSponsors() (*types.GetPublicSponsorsResponse, error)
}

// service 赞助方服务实现
type service struct {
	sponsorRepo sponsor.Repository
}

// NewService 创建新的赞助方服务
func NewService(sponsorRepo sponsor.Repository) Service {
	return &service{
		sponsorRepo: sponsorRepo,
	}
}

// CreateSponsor 创建赞助方
func (s *service) CreateSponsor(req *types.CreateSponsorRequest) (*types.Sponsor, error) {
	logger.Info("Creating sponsor", "name", req.Name, "type", req.Type)

	// 验证请求参数
	if err := s.validateCreateRequest(req); err != nil {
		logger.Error("Invalid create sponsor request", err, "request", req)
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 创建赞助方对象
	sponsor := &types.Sponsor{
		Name:        req.Name,
		LogoURL:     req.LogoURL,
		Link:        req.Link,
		Description: req.Description,
		Type:        req.Type,
		SortOrder:   req.SortOrder,
		IsActive:    true,
	}

	// 保存到数据库
	if err := s.sponsorRepo.Create(sponsor); err != nil {
		logger.Error("Failed to create sponsor", err, "sponsor", sponsor)
		return nil, fmt.Errorf("failed to create sponsor: %w", err)
	}

	logger.Info("Created sponsor successfully", "id", sponsor.ID, "name", sponsor.Name)
	return sponsor, nil
}

// GetSponsorByID 根据ID获取赞助方
func (s *service) GetSponsorByID(id int64) (*types.Sponsor, error) {
	logger.Info("Getting sponsor by ID", "id", id)

	sponsor, err := s.sponsorRepo.GetByID(id)
	if err != nil {
		logger.Error("Failed to get sponsor by ID", err, "id", id)
		return nil, fmt.Errorf("failed to get sponsor: %w", err)
	}

	if sponsor == nil {
		logger.Info("Sponsor not found", "id", id)
		return nil, fmt.Errorf("sponsor not found")
	}

	return sponsor, nil
}

// UpdateSponsor 更新赞助方
func (s *service) UpdateSponsor(id int64, req *types.UpdateSponsorRequest) (*types.Sponsor, error) {
	logger.Info("Updating sponsor", "id", id)

	// 验证赞助方是否存在
	existingSponsor, err := s.sponsorRepo.GetByID(id)
	if err != nil {
		logger.Error("Failed to get sponsor for update", err, "id", id)
		return nil, fmt.Errorf("failed to get sponsor: %w", err)
	}
	if existingSponsor == nil {
		return nil, fmt.Errorf("sponsor not found")
	}

	// 验证请求参数
	if err := s.validateUpdateRequest(req); err != nil {
		logger.Error("Invalid update sponsor request", err, "request", req)
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 构建更新数据
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.LogoURL != nil {
		updates["logo_url"] = *req.LogoURL
	}
	if req.Link != nil {
		updates["link"] = *req.Link
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		logger.Info("No updates provided", "id", id)
		return existingSponsor, nil
	}

	// 执行更新
	if err := s.sponsorRepo.Update(id, updates); err != nil {
		logger.Error("Failed to update sponsor", err, "id", id, "updates", updates)
		return nil, fmt.Errorf("failed to update sponsor: %w", err)
	}

	// 获取更新后的数据
	updatedSponsor, err := s.sponsorRepo.GetByID(id)
	if err != nil {
		logger.Error("Failed to get updated sponsor", err, "id", id)
		return nil, fmt.Errorf("failed to get updated sponsor: %w", err)
	}

	logger.Info("Updated sponsor successfully", "id", id)
	return updatedSponsor, nil
}

// DeleteSponsor 删除赞助方
func (s *service) DeleteSponsor(id int64) error {
	logger.Info("Deleting sponsor", "id", id)

	// 验证赞助方是否存在
	existingSponsor, err := s.sponsorRepo.GetByID(id)
	if err != nil {
		logger.Error("Failed to get sponsor for delete", err, "id", id)
		return fmt.Errorf("failed to get sponsor: %w", err)
	}
	if existingSponsor == nil {
		return fmt.Errorf("sponsor not found")
	}

	// 执行删除（软删除）
	if err := s.sponsorRepo.Delete(id); err != nil {
		logger.Error("Failed to delete sponsor", err, "id", id)
		return fmt.Errorf("failed to delete sponsor: %w", err)
	}

	logger.Info("Deleted sponsor successfully", "id", id, "name", existingSponsor.Name)
	return nil
}

// GetSponsorsList 获取赞助方列表（管理接口）
func (s *service) GetSponsorsList(req *types.GetSponsorsRequest) (*types.GetSponsorsResponse, error) {
	logger.Info("Getting sponsors list", "request", req)

	sponsors, total, err := s.sponsorRepo.GetList(req)
	if err != nil {
		logger.Error("Failed to get sponsors list", err, "request", req)
		return nil, fmt.Errorf("failed to get sponsors list: %w", err)
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

	// 转换为响应格式
	sponsorsList := make([]types.Sponsor, len(sponsors))
	for i, sponsor := range sponsors {
		sponsorsList[i] = *sponsor
	}

	response := &types.GetSponsorsResponse{
		Sponsors: sponsorsList,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}

	logger.Info("Got sponsors list", "total", total, "page", page, "count", len(sponsorsList))
	return response, nil
}

// GetPublicSponsors 获取公开的赞助方列表（公开接口）
func (s *service) GetPublicSponsors() (*types.GetPublicSponsorsResponse, error) {
	logger.Info("Getting public sponsors")

	// 获取所有激活的赞助方
	allSponsors, err := s.sponsorRepo.GetAllActive()
	if err != nil {
		logger.Error("Failed to get active sponsors", err)
		return nil, fmt.Errorf("failed to get active sponsors: %w", err)
	}

	// 分离赞助方和生态伙伴
	var sponsors []types.SponsorInfo
	var partners []types.SponsorInfo

	for _, sponsor := range allSponsors {
		sponsorInfo := types.SponsorInfo{
			ID:          sponsor.ID,
			Name:        sponsor.Name,
			LogoURL:     sponsor.LogoURL,
			Link:        sponsor.Link,
			Description: sponsor.Description,
			Type:        sponsor.Type,
			SortOrder:   sponsor.SortOrder,
		}

		if sponsor.Type == types.SponsorTypeSponsor {
			sponsors = append(sponsors, sponsorInfo)
		} else if sponsor.Type == types.SponsorTypePartner {
			partners = append(partners, sponsorInfo)
		}
	}

	response := &types.GetPublicSponsorsResponse{
		Sponsors: sponsors,
		Partners: partners,
	}

	logger.Info("Got public sponsors", "sponsors_count", len(sponsors), "partners_count", len(partners))
	return response, nil
}

// validateCreateRequest 验证创建请求
func (s *service) validateCreateRequest(req *types.CreateSponsorRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if req.LogoURL == "" {
		return fmt.Errorf("logo_url is required")
	}
	if req.Link == "" {
		return fmt.Errorf("link is required")
	}
	if req.Description == "" {
		return fmt.Errorf("description is required")
	}
	if req.Type != types.SponsorTypeSponsor && req.Type != types.SponsorTypePartner {
		return fmt.Errorf("invalid type: %s", req.Type)
	}
	return nil
}

// validateUpdateRequest 验证更新请求
func (s *service) validateUpdateRequest(req *types.UpdateSponsorRequest) error {
	if req.Name != nil && *req.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if req.LogoURL != nil && *req.LogoURL == "" {
		return fmt.Errorf("logo_url cannot be empty")
	}
	if req.Link != nil && *req.Link == "" {
		return fmt.Errorf("link cannot be empty")
	}
	if req.Description != nil && *req.Description == "" {
		return fmt.Errorf("description cannot be empty")
	}
	if req.Type != nil && *req.Type != types.SponsorTypeSponsor && *req.Type != types.SponsorTypePartner {
		return fmt.Errorf("invalid type: %s", *req.Type)
	}
	return nil
}
