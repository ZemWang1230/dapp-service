package sponsor

import (
	"net/http"
	"strconv"

	"timelocker-backend/internal/middleware"
	authService "timelocker-backend/internal/service/auth"
	sponsorService "timelocker-backend/internal/service/sponsor"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Handler 赞助方处理器
type Handler struct {
	sponsorService sponsorService.Service
	authService    authService.Service
}

// NewHandler 创建新的赞助方处理器
func NewHandler(sponsorService sponsorService.Service, authService authService.Service) *Handler {
	return &Handler{
		sponsorService: sponsorService,
		authService:    authService,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// 公开接口（无需认证）
	publicGroup := router.Group("/sponsors")
	{
		// 获取公开的赞助方和生态伙伴列表
		// http://localhost:8080/api/v1/sponsors/public
		publicGroup.GET("/public", h.GetPublicSponsors)
	}

	// 管理接口（需要认证）
	adminGroup := router.Group("/admin/sponsors")
	adminGroup.Use(middleware.AuthMiddleware(h.authService)) // 使用JWT认证中间件
	{
		// 创建赞助方
		// http://localhost:8080/api/v1/admin/sponsors
		adminGroup.POST("/", h.CreateSponsor)

		// 获取赞助方列表（管理用）
		// http://localhost:8080/api/v1/admin/sponsors?type=sponsor&page=1&page_size=20
		adminGroup.GET("/", h.GetSponsorsList)

		// 根据ID获取赞助方
		// http://localhost:8080/api/v1/admin/sponsors/:id
		adminGroup.GET("/:id", h.GetSponsorByID)

		// 更新赞助方
		// http://localhost:8080/api/v1/admin/sponsors/:id
		adminGroup.PUT("/:id", h.UpdateSponsor)

		// 删除赞助方
		// http://localhost:8080/api/v1/admin/sponsors/:id
		adminGroup.DELETE("/:id", h.DeleteSponsor)
	}
}

// GetPublicSponsors 获取公开的赞助方和生态伙伴列表
// @Summary 获取公开的赞助方和生态伙伴列表
// @Description 获取所有激活的赞助方和生态伙伴信息，用于在前端展示。返回的数据按照排序权重和创建时间排序。
// @Tags Sponsors
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=types.GetPublicSponsorsResponse} "成功获取赞助方和生态伙伴列表"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "获取赞助方列表失败"
// @Router /api/v1/sponsors/public [get]
func (h *Handler) GetPublicSponsors(c *gin.Context) {
	logger.Info("GetPublicSponsors: getting public sponsors")

	sponsors, err := h.sponsorService.GetPublicSponsors()
	if err != nil {
		logger.Error("GetPublicSponsors: failed to get public sponsors", err)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "QUERY_FAILED",
				Message: "Failed to get sponsors",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    sponsors,
	})
}

// CreateSponsor 创建赞助方
// @Summary 创建赞助方
// @Description 创建新的赞助方或生态伙伴。需要管理员权限。
// @Tags Sponsors
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body types.CreateSponsorRequest true "创建赞助方请求"
// @Success 201 {object} types.APIResponse{data=types.Sponsor} "成功创建赞助方"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "创建赞助方失败"
// @Router /api/v1/admin/sponsors [post]
func (h *Handler) CreateSponsor(c *gin.Context) {
	// 从JWT中获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		logger.Error("CreateSponsor: failed to get user from context", nil)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		return
	}

	var req types.CreateSponsorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("CreateSponsor: invalid request", err, "wallet_address", walletAddress)
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		return
	}

	sponsor, err := h.sponsorService.CreateSponsor(&req)
	if err != nil {
		logger.Error("CreateSponsor: failed to create sponsor", err, "wallet_address", walletAddress, "request", req)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "CREATE_FAILED",
				Message: "Failed to create sponsor",
				Details: err.Error(),
			},
		})
		return
	}

	logger.Info("CreateSponsor: created sponsor successfully", "sponsor_id", sponsor.ID, "wallet_address", walletAddress)
	c.JSON(http.StatusCreated, types.APIResponse{
		Success: true,
		Data:    sponsor,
	})
}

// GetSponsorsList 获取赞助方列表（管理用）
// @Summary 获取赞助方列表（管理用）
// @Description 获取赞助方列表，支持分页和过滤。需要管理员权限。
// @Tags Sponsors
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param type query string false "过滤类型" Enums(sponsor, partner)
// @Param is_active query boolean false "过滤激活状态"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(20)
// @Success 200 {object} types.APIResponse{data=types.GetSponsorsResponse} "成功获取赞助方列表"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "获取赞助方列表失败"
// @Router /api/v1/admin/sponsors [get]
func (h *Handler) GetSponsorsList(c *gin.Context) {
	// 从JWT中获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		logger.Error("GetSponsorsList: failed to get user from context", nil)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		return
	}

	var req types.GetSponsorsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		logger.Error("GetSponsorsList: invalid request", err, "wallet_address", walletAddress)
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		return
	}

	sponsors, err := h.sponsorService.GetSponsorsList(&req)
	if err != nil {
		logger.Error("GetSponsorsList: failed to get sponsors list", err, "wallet_address", walletAddress, "request", req)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "QUERY_FAILED",
				Message: "Failed to get sponsors list",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    sponsors,
	})
}

// GetSponsorByID 根据ID获取赞助方
// @Summary 根据ID获取赞助方
// @Description 获取指定ID的赞助方详细信息。需要管理员权限。
// @Tags Sponsors
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "赞助方ID"
// @Success 200 {object} types.APIResponse{data=types.Sponsor} "成功获取赞助方信息"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "赞助方不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "获取赞助方失败"
// @Router /api/v1/admin/sponsors/{id} [get]
func (h *Handler) GetSponsorByID(c *gin.Context) {
	// 从JWT中获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		logger.Error("GetSponsorByID: failed to get user from context", nil)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.Error("GetSponsorByID: invalid ID", err, "id_str", idStr, "wallet_address", walletAddress)
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_ID",
				Message: "Invalid sponsor ID",
				Details: err.Error(),
			},
		})
		return
	}

	sponsor, err := h.sponsorService.GetSponsorByID(id)
	if err != nil {
		logger.Error("GetSponsorByID: failed to get sponsor", err, "id", id, "wallet_address", walletAddress)

		// 判断是否为不存在的错误
		if err.Error() == "sponsor not found" {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error: &types.APIError{
					Code:    "NOT_FOUND",
					Message: "Sponsor not found",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "QUERY_FAILED",
				Message: "Failed to get sponsor",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    sponsor,
	})
}

// UpdateSponsor 更新赞助方
// @Summary 更新赞助方
// @Description 更新指定ID的赞助方信息。需要管理员权限。
// @Tags Sponsors
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "赞助方ID"
// @Param request body types.UpdateSponsorRequest true "更新赞助方请求"
// @Success 200 {object} types.APIResponse{data=types.Sponsor} "成功更新赞助方"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "赞助方不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "更新赞助方失败"
// @Router /api/v1/admin/sponsors/{id} [put]
func (h *Handler) UpdateSponsor(c *gin.Context) {
	// 从JWT中获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		logger.Error("UpdateSponsor: failed to get user from context", nil)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.Error("UpdateSponsor: invalid ID", err, "id_str", idStr, "wallet_address", walletAddress)
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_ID",
				Message: "Invalid sponsor ID",
				Details: err.Error(),
			},
		})
		return
	}

	var req types.UpdateSponsorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("UpdateSponsor: invalid request", err, "id", id, "wallet_address", walletAddress)
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		return
	}

	sponsor, err := h.sponsorService.UpdateSponsor(id, &req)
	if err != nil {
		logger.Error("UpdateSponsor: failed to update sponsor", err, "id", id, "wallet_address", walletAddress, "request", req)

		// 判断是否为不存在的错误
		if err.Error() == "sponsor not found" {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error: &types.APIError{
					Code:    "NOT_FOUND",
					Message: "Sponsor not found",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update sponsor",
				Details: err.Error(),
			},
		})
		return
	}

	logger.Info("UpdateSponsor: updated sponsor successfully", "id", id, "wallet_address", walletAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    sponsor,
	})
}

// DeleteSponsor 删除赞助方
// @Summary 删除赞助方
// @Description 删除指定ID的赞助方（软删除，设置为不激活）。需要管理员权限。
// @Tags Sponsors
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "赞助方ID"
// @Success 200 {object} types.APIResponse{data=string} "成功删除赞助方"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "赞助方不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "删除赞助方失败"
// @Router /api/v1/admin/sponsors/{id} [delete]
func (h *Handler) DeleteSponsor(c *gin.Context) {
	// 从JWT中获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		logger.Error("DeleteSponsor: failed to get user from context", nil)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.Error("DeleteSponsor: invalid ID", err, "id_str", idStr, "wallet_address", walletAddress)
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_ID",
				Message: "Invalid sponsor ID",
				Details: err.Error(),
			},
		})
		return
	}

	err = h.sponsorService.DeleteSponsor(id)
	if err != nil {
		logger.Error("DeleteSponsor: failed to delete sponsor", err, "id", id, "wallet_address", walletAddress)

		// 判断是否为不存在的错误
		if err.Error() == "sponsor not found" {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error: &types.APIError{
					Code:    "NOT_FOUND",
					Message: "Sponsor not found",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete sponsor",
				Details: err.Error(),
			},
		})
		return
	}

	logger.Info("DeleteSponsor: deleted sponsor successfully", "id", id, "wallet_address", walletAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    "Sponsor deleted successfully",
	})
}
