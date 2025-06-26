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
		// 切换链
		// http://localhost:8080/api/v1/auth/switch-chain
		authGroup.POST("/switch-chain", middleware.AuthMiddleware(h.authService), h.SwitchChain)
	}
}

// WalletConnect 钱包连接认证
// @Summary 钱包连接认证
// @Description 通过钱包签名进行用户认证。前端需要先让用户用钱包对特定消息进行签名，然后将钱包地址、签名和消息发送到此接口进行验证。验证成功后返回JWT访问令牌和刷新令牌。
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body types.WalletConnectRequest true "钱包连接认证请求体"
// @Success 200 {object} types.APIResponse{data=types.WalletConnectResponse} "认证成功，返回访问令牌和用户信息"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误，可能是钱包地址格式不正确"
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
	logger.Info("WalletConnect :", "User: ", response.User.WalletAddress, "ChainID: ", response.User.ChainID)
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
// @Description 获取当前认证用户的资料信息，包括钱包地址、当前选择的链ID、注册时间和最后登录时间等。此接口需要在请求头中携带有效的JWT访问令牌。
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} types.APIResponse{data=types.UserProfile} "成功获取用户资料"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "用户不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/auth/profile [get]
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

	// 获取用户资料
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

	logger.Info("GetProfile :", "User: ", profile.WalletAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    profile,
	})
}

// SwitchChain 切换链
// @Summary 切换区块链网络
// @Description 用户切换到新的区块链网络。由于不同链的安全性考虑，切换链需要重新进行钱包签名验证。成功后会更新用户的当前链ID，并返回新的访问令牌。
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.SwitchChainRequest true "切换链请求体，包含新的链ID和签名信息"
// @Success 200 {object} types.APIResponse{data=types.SwitchChainResponse} "切换成功，返回新的访问令牌和用户信息"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "用户不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/auth/switch-chain [post]
func (h *Handler) SwitchChain(c *gin.Context) {
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
		logger.Error("SwitchChain Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.SwitchChainRequest
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
		logger.Error("SwitchChain Error: ", errors.New("invalid request parameters"), "error: ", err)
		return
	}

	// 调用认证服务
	response, err := h.authService.SwitchChain(c.Request.Context(), walletAddress, &req)
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
		logger.Error("SwitchChain Error: ", err, "errorCode: ", errorCode)
		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		return
	}

	logger.Info("SwitchChain :", "User: ", response.User.WalletAddress, "ChainID: ", response.User.ChainID)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}
