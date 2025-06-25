package asset

import (
	"net/http"

	"timelocker-backend/internal/middleware"
	assetService "timelocker-backend/internal/service/asset"
	authService "timelocker-backend/internal/service/auth"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Handler 资产处理器
type Handler struct {
	assetService assetService.Service
	authService  authService.Service
}

// NewHandler 创建新的资产处理器
func NewHandler(assetService assetService.Service, authService authService.Service) *Handler {
	return &Handler{
		assetService: assetService,
		authService:  authService,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	assetGroup := router.Group("/assets")
	assetGroup.Use(middleware.AuthMiddleware(h.authService)) // 使用JWT认证中间件
	{
		// 获取用户资产
		// http://localhost:8080/api/v1/assets
		assetGroup.GET("/", h.GetUserAssets)
		// 刷新用户资产
		// http://localhost:8080/api/v1/assets/refresh
		assetGroup.POST("/refresh", h.RefreshUserAssets)
	}
}

// GetUserAssets 获取用户资产
// @Summary 获取用户资产
// @Description 获取用户在所有支持链上的资产信息，如果数据库中没有数据会自动刷新
// @Tags 资产
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=types.UserAssetResponse}
// @Failure 401 {object} types.APIResponse{error=types.APIError}
// @Failure 500 {object} types.APIResponse{error=types.APIError}
// @Router /api/v1/assets [get]
func (h *Handler) GetUserAssets(c *gin.Context) {
	// 从JWT中获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		logger.Error("GetUserAssets: failed to get user from context", nil)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		return
	}

	// 调用服务获取用户资产
	assets, err := h.assetService.GetUserAssets(walletAddress)
	if err != nil {
		logger.Error("GetUserAssets: failed to get user assets", err, "wallet_address", walletAddress)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "QUERY_FAILED",
				Message: "Failed to query user assets",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    assets,
	})
}

// RefreshUserAssets 刷新用户资产
// @Summary 刷新用户资产
// @Description 强制刷新用户在所有支持链上的资产信息
// @Tags 资产
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=string}
// @Failure 401 {object} types.APIResponse{error=types.APIError}
// @Failure 500 {object} types.APIResponse{error=types.APIError}
// @Router /api/v1/assets/refresh [post]
func (h *Handler) RefreshUserAssets(c *gin.Context) {
	// 从JWT中获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		logger.Error("RefreshUserAssets: failed to get user from context", nil)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		return
	}

	// 刷新用户资产
	if err := h.assetService.RefreshUserAssets(walletAddress); err != nil {
		logger.Error("RefreshUserAssets: failed to refresh assets", err, "wallet_address", walletAddress)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "REFRESH_FAILED",
				Message: "Failed to refresh assets",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    "Assets refreshed successfully",
	})
}
