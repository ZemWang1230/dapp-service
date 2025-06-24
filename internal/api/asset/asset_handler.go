package asset

import (
	"net/http"
	"strconv"

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
		// 请求体：
		// {
		// 	"wallet_address": "0x1234567890123456789012345678901234567890",
		// 	"force_refresh": true
		// }
		assetGroup.POST("/refresh", h.RefreshUserAssets)
	}
}

// GetUserAssets 获取用户资产
// @Summary 获取用户资产
// @Description 获取用户在所有链上的资产信息
// @Tags 资产
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param force_refresh query bool false "是否强制刷新"
// @Success 200 {object} types.APIResponse{data=types.UserAssetResponse}
// @Failure 400 {object} types.APIResponse{error=types.APIError}
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

	// 获取查询参数
	forceRefreshStr := c.DefaultQuery("force_refresh", "false")
	forceRefresh, _ := strconv.ParseBool(forceRefreshStr)

	// 调用服务获取用户资产
	assets, err := h.assetService.GetUserAssets(walletAddress, forceRefresh)
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
// @Param request body types.AssetQueryRequest true "刷新请求"
// @Success 200 {object} types.APIResponse{data=string}
// @Failure 400 {object} types.APIResponse{error=types.APIError}
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

	// 解析请求参数
	var request types.AssetQueryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error("RefreshUserAssets: invalid request", err)
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

	// 验证钱包地址是否匹配
	if request.WalletAddress != walletAddress {
		logger.Error("RefreshUserAssets: wallet address mismatch", nil, "request_wallet", request.WalletAddress, "user_wallet", walletAddress)
		c.JSON(http.StatusForbidden, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "WALLET_MISMATCH",
				Message: "Wallet address mismatch",
			},
		})
		return
	}

	// 刷新用户资产
	if err := h.assetService.RefreshUserAssets(request.WalletAddress); err != nil {
		logger.Error("RefreshUserAssets: failed to refresh assets", err, "wallet_address", request.WalletAddress)
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
