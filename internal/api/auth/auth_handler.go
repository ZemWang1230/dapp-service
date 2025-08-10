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

// RegisterRoutes 注册认证路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// 认证API组
	authGroup := router.Group("/auth")
	{
		// 钱包连接
		// POST /api/v1/auth/wallet-connect
		// http://localhost:8080/api/v1/auth/wallet-connect
		authGroup.POST("/wallet-connect", h.WalletConnect)

		// 刷新令牌
		// POST /api/v1/auth/refresh-token
		// http://localhost:8080/api/v1/auth/refresh-token
		authGroup.POST("/refresh-token", h.RefreshToken)

		// 获取用户资料
		// POST /api/v1/auth/profile
		// http://localhost:8080/api/v1/auth/profile
		authGroup.POST("/profile", middleware.AuthMiddleware(h.authService), h.GetProfile)
	}
}

// WalletConnect 钱包连接认证
// @Summary 钱包连接认证
// @Description 通过钱包签名进行用户认证。前端需要先让用户用钱包对特定消息进行签名，然后将钱包地址、签名和消息发送到此接口进行验证。验证成功后返回JWT访问令牌和刷新令牌。钱包地址必须是有效以太坊地址（长度42，0x前缀）。
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body types.WalletConnectRequest true "钱包连接认证请求体"
// @Success 200 {object} types.APIResponse{data=types.WalletConnectResponse} "认证成功，返回访问令牌和用户信息"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误（INVALID_WALLET_ADDRESS等）"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "认证失败，可能是签名无效或地址恢复失败"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
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
	logger.Info("WalletConnect :", "User: ", response.User.WalletAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// RefreshToken 刷新访问令牌
// @Summary 刷新访问令牌
// @Description 使用刷新令牌获取新的访问令牌。当访问令牌过期时，前端可以使用此接口通过刷新令牌重新获取新的访问令牌和刷新令牌，无需重新进行钱包签名认证。
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body types.RefreshTokenRequest true "刷新令牌请求体"
// @Success 200 {object} types.APIResponse{data=types.WalletConnectResponse} "刷新成功，返回新的访问令牌和刷新令牌"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "刷新令牌无效或已过期"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "用户不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/auth/refresh-token [post]
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

	logger.Info("RefreshToken :", "User: ", response.User.WalletAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetProfile 获取用户资料
// @Summary 获取用户资料
// @Description 获取当前认证用户的详细资料信息，包括钱包地址、创建时间等。需要有效的JWT令牌。
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} types.APIResponse{data=types.UserProfile} "成功获取用户资料"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "用户不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/auth/profile [post]
func (h *Handler) GetProfile(c *gin.Context) {
	// 从上下文获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
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

	// 调用认证服务
	profile, err := h.authService.GetProfile(c.Request.Context(), walletAddress)
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

	logger.Info("GetProfile: ", "User: ", profile.WalletAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    profile,
	})
}
