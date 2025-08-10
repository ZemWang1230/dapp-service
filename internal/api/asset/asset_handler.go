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
		assetGroup.POST("/", h.GetUserAssets)
		// 刷新用户资产
		// http://localhost:8080/api/v1/assets/refresh
		assetGroup.POST("/refresh", h.RefreshUserAssets)
	}
}

// GetUserAssets 获取用户资产
// @Summary 获取用户资产列表
// @Description 获取当前认证用户在所有支持的区块链上的资产信息，包括代币余额、USD价值、价格变化等。如果数据库中没有缓存数据，系统会自动从区块链上刷新最新的资产信息。资产按USD价值从高到低排序。
// @Tags Assets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=types.UserAssetResponse} "成功获取用户资产信息"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "获取资产信息失败或区块链查询错误"
// @Router /api/v1/assets [post]
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
// @Summary 强制刷新用户资产信息
// @Description 强制从区块链上重新获取用户在所有支持链上的最新资产信息，更新数据库缓存。此操作可能需要较长时间，因为需要查询多个区块链网络的数据。
// @Tags Assets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=string} "资产刷新成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "刷新资产失败或区块链查询错误"
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
