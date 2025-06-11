package auth

import (
	"errors"
	"net/http"

	"timelocker-backend/internal/middleware"
	"timelocker-backend/internal/service/auth"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Handler 认证处理器
type Handler struct {
	authService auth.Service
}

// NewHandler 创建认证处理器
func NewHandler(authService auth.Service) *Handler {
	return &Handler{
		authService: authService,
	}
}

// RegisterRoutes 注册认证相关路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// 创建认证路由组
	authGroup := router.Group("/auth")
	{
		// 公开端点
		// 钱包连接认证
		// http://localhost:8080/api/v1/auth/wallet-connect
		authGroup.POST("/wallet-connect", h.WalletConnect)
		// 刷新访问令牌
		// http://localhost:8080/api/v1/auth/refresh
		authGroup.POST("/refresh", h.RefreshToken)

		// 需要认证的端点
		// 获取用户资料
		// http://localhost:8080/api/v1/auth/profile
		authGroup.GET("/profile", middleware.AuthMiddleware(h.authService), h.GetProfile)
	}
}

// WalletConnect 钱包连接认证
// @Summary 钱包连接认证
// @Description 通过钱包签名进行用户认证
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body types.WalletConnectRequest true "钱包连接请求"
// @Success 200 {object} types.APIResponse{data=types.WalletConnectResponse}
// @Failure 400 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse
// @Router /api/v1/auth/wallet-connect [post]
func (h *Handler) WalletConnect(c *gin.Context) {
	var req types.WalletConnectRequest
	// 绑定请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("WalletConnect Error: ", errors.New("invalid request parameters"), "error: ", err)
		return
	}

	// 调用认证服务，返回响应数据
	response, err := h.authService.WalletConnect(c.Request.Context(), &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case auth.ErrInvalidAddress:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_WALLET_ADDRESS"
		case auth.ErrInvalidSignature:
			statusCode = http.StatusUnauthorized
			errorCode = "INVALID_SIGNATURE"
		case auth.ErrSignatureRecovery:
			statusCode = http.StatusUnauthorized
			errorCode = "SIGNATURE_RECOVERY_FAILED"
		default:
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("WalletConnect Error: ", err, "errorCode: ", errorCode)
		return
	}
	logger.Info("WalletConnect :", "User: ", response.User.WalletAddress, "ChainID: ", response.User.ChainID)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// RefreshToken 刷新访问令牌
// @Summary 刷新访问令牌
// @Description 使用刷新令牌获取新的访问令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body types.RefreshTokenRequest true "刷新令牌请求"
// @Success 200 {object} types.APIResponse{data=types.WalletConnectResponse}
// @Failure 400 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse
// @Router /api/v1/auth/refresh [post]
func (h *Handler) RefreshToken(c *gin.Context) {
	var req types.RefreshTokenRequest
	// 绑定请求参数
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("RefreshToken Error: ", errors.New("invalid request parameters"), "error: ", err)
		return
	}

	// 调用认证服务
	response, err := h.authService.RefreshToken(c.Request.Context(), &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case auth.ErrInvalidToken:
			statusCode = http.StatusUnauthorized
			errorCode = "INVALID_REFRESH_TOKEN"
		case auth.ErrUserNotFound:
			statusCode = http.StatusNotFound
			errorCode = "USER_NOT_FOUND"
		default:
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}
		logger.Error("RefreshToken Error: ", err, "errorCode: ", errorCode)
		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		return
	}

	logger.Info("RefreshToken :", "User: ", response.User.WalletAddress, "ChainID: ", response.User.ChainID)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetProfile 获取用户资料
// @Summary 获取用户资料
// @Description 获取当前认证用户的资料信息
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} types.APIResponse{data=types.UserProfile}
// @Failure 401 {object} types.APIResponse
// @Failure 404 {object} types.APIResponse
// @Router /api/v1/auth/profile [get]
func (h *Handler) GetProfile(c *gin.Context) {
	// 从上下文获取用户ID
	userID, _, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("GetProfile Error: ", errors.New("user not authenticated"))
		return
	}

	// 获取用户资料
	profile, err := h.authService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case auth.ErrUserNotFound:
			statusCode = http.StatusNotFound
			errorCode = "USER_NOT_FOUND"
		default:
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}
		logger.Error("GetProfile Error: ", err, "errorCode: ", errorCode)
		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		return
	}

	logger.Info("GetProfile :", "User: ", profile.WalletAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    profile,
	})
}
