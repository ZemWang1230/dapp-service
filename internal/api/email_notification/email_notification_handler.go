package email_notification

import (
	"errors"
	"net/http"
	"strconv"

	"timelocker-backend/internal/middleware"
	"timelocker-backend/internal/service/auth"
	"timelocker-backend/internal/service/email_notification"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Handler 邮件通知处理器
type Handler struct {
	emailNotificationService email_notification.Service
	authService              auth.Service
}

// NewHandler 创建邮件通知处理器
func NewHandler(emailNotificationService email_notification.Service, authService auth.Service) *Handler {
	return &Handler{
		emailNotificationService: emailNotificationService,
		authService:              authService,
	}
}

// RegisterRoutes 注册邮件通知相关路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// 创建邮件通知路由组
	emailGroup := router.Group("/email-notifications")
	{
		// 公开端点 - 应急邮件回复（不需要认证）
		emailGroup.GET("/emergency-reply", h.EmergencyReply)

		// 需要认证的端点
		authEmailGroup := emailGroup.Use(middleware.AuthMiddleware(h.authService))
		{
			// 邮件通知配置管理
			authEmailGroup.POST("", h.AddEmailNotification)               // 添加邮件通知
			authEmailGroup.POST("/verify", h.VerifyEmail)                 // 验证邮箱
			authEmailGroup.POST("/resend-code", h.ResendVerificationCode) // 重发验证码
			authEmailGroup.GET("", h.GetEmailNotifications)               // 获取邮件通知列表
			authEmailGroup.GET("/:email", h.GetEmailNotification)         // 获取单个邮件通知
			authEmailGroup.PUT("/:email", h.UpdateEmailNotification)      // 更新邮件通知
			authEmailGroup.DELETE("/:email", h.DeleteEmailNotification)   // 删除邮件通知

			// 邮件发送记录
			authEmailGroup.GET("/logs", h.GetEmailSendLogs) // 获取邮件发送记录

			// 内部API - 用于timelock合约管理
			authEmailGroup.GET("/verified-emails", h.GetVerifiedEmails) // 获取已验证邮箱列表
		}
	}
}

// AddEmailNotification 添加邮件通知配置
// @Summary 添加邮件通知配置
// @Description 为用户添加邮件通知配置，支持设置邮箱地址、备注和监听的timelock合约。添加后会发送验证码到邮箱进行验证。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.AddEmailNotificationRequest true "添加邮件通知请求体"
// @Success 200 {object} types.APIResponse{data=types.EmailNotificationResponse} "添加成功，返回邮件通知配置信息"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "邮箱已存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications [post]
func (h *Handler) AddEmailNotification(c *gin.Context) {
	// 获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("AddEmailNotification Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.AddEmailNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("AddEmailNotification Error: ", errors.New("invalid request parameters"), "error", err)
		return
	}

	// 调用服务
	response, err := h.emailNotificationService.AddEmailNotification(c.Request.Context(), walletAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case email_notification.ErrEmailAlreadyExists:
			statusCode = http.StatusConflict
			errorCode = "EMAIL_ALREADY_EXISTS"
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
		logger.Error("AddEmailNotification Error: ", err, "wallet_address", walletAddress)
		return
	}

	logger.Info("AddEmailNotification: ", "wallet_address", walletAddress, "email", req.Email)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// VerifyEmail 验证邮箱
// @Summary 验证邮箱
// @Description 使用验证码验证邮箱地址。验证成功后，该邮箱可以接收timelock合约的通知邮件。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.VerifyEmailRequest true "验证邮箱请求体"
// @Success 200 {object} types.APIResponse "验证成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误或验证码无效"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "邮件配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications/verify [post]
func (h *Handler) VerifyEmail(c *gin.Context) {
	// 获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("VerifyEmail Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("VerifyEmail Error: ", errors.New("invalid request parameters"), "error", err)
		return
	}

	// 调用服务
	err := h.emailNotificationService.VerifyEmail(c.Request.Context(), walletAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case email_notification.ErrInvalidVerificationCode:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_VERIFICATION_CODE"
		case email_notification.ErrEmailNotFound:
			statusCode = http.StatusNotFound
			errorCode = "EMAIL_NOT_FOUND"
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
		logger.Error("VerifyEmail Error: ", err, "wallet_address", walletAddress)
		return
	}

	logger.Info("VerifyEmail: ", "wallet_address", walletAddress, "email", req.Email)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Email verified successfully"},
	})
}

// ResendVerificationCode 重发验证码
// @Summary 重发验证码
// @Description 重新发送邮箱验证码。如果之前的验证码过期或丢失，可以使用此接口重新发送。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.ResendVerificationRequest true "重发验证码请求体"
// @Success 200 {object} types.APIResponse "验证码发送成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误或邮箱已验证"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "邮件配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications/resend-code [post]
func (h *Handler) ResendVerificationCode(c *gin.Context) {
	// 获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("ResendVerificationCode Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("ResendVerificationCode Error: ", errors.New("invalid request parameters"), "error", err)
		return
	}

	// 调用服务
	err := h.emailNotificationService.ResendVerificationCode(c.Request.Context(), walletAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case email_notification.ErrEmailNotFound:
			statusCode = http.StatusNotFound
			errorCode = "EMAIL_NOT_FOUND"
		default:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_REQUEST"
			if err.Error() == "email already verified" {
				errorCode = "EMAIL_ALREADY_VERIFIED"
			}
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("ResendVerificationCode Error: ", err, "wallet_address", walletAddress)
		return
	}

	logger.Info("ResendVerificationCode: ", "wallet_address", walletAddress, "email", req.Email)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Verification code sent successfully"},
	})
}

// GetEmailNotifications 获取邮件通知配置列表
// @Summary 获取邮件通知配置列表
// @Description 获取用户的邮件通知配置列表，支持分页查询。返回邮箱地址、验证状态、监听的合约等信息。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} types.APIResponse{data=types.EmailNotificationListResponse} "获取成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications [get]
func (h *Handler) GetEmailNotifications(c *gin.Context) {
	// 获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("GetEmailNotifications Error: ", errors.New("user not authenticated"))
		return
	}

	// 获取分页参数
	page := 1
	pageSize := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// 调用服务
	response, err := h.emailNotificationService.GetEmailNotifications(c.Request.Context(), walletAddress, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetEmailNotifications Error: ", err, "wallet_address", walletAddress)
		return
	}

	logger.Info("GetEmailNotifications: ", "wallet_address", walletAddress, "total", response.Total)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetEmailNotification 获取单个邮件通知配置
// @Summary 获取单个邮件通知配置
// @Description 根据邮箱地址获取特定的邮件通知配置详情，包括验证状态和监听的合约列表。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param email path string true "邮箱地址"
// @Success 200 {object} types.APIResponse{data=types.EmailNotificationResponse} "获取成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "邮件配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications/{email} [get]
func (h *Handler) GetEmailNotification(c *gin.Context) {
	// 获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("GetEmailNotification Error: ", errors.New("user not authenticated"))
		return
	}

	email := c.Param("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Email parameter is required",
			},
		})
		logger.Error("GetEmailNotification Error: ", errors.New("email parameter is required"))
		return
	}

	// 调用服务
	response, err := h.emailNotificationService.GetEmailNotification(c.Request.Context(), walletAddress, email)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case email_notification.ErrEmailNotFound:
			statusCode = http.StatusNotFound
			errorCode = "EMAIL_NOT_FOUND"
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
		logger.Error("GetEmailNotification Error: ", err, "wallet_address", walletAddress, "email", email)
		return
	}

	logger.Info("GetEmailNotification: ", "wallet_address", walletAddress, "email", email)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// UpdateEmailNotification 更新邮件通知配置
// @Summary 更新邮件通知配置
// @Description 更新邮件通知配置的备注和监听的timelock合约列表。注意：无法更改邮箱地址，只能更新备注和合约列表。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param email path string true "邮箱地址"
// @Param request body types.UpdateEmailNotificationRequest true "更新邮件通知请求体"
// @Success 200 {object} types.APIResponse{data=types.EmailNotificationResponse} "更新成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误或邮箱未验证"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "邮件配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications/{email} [put]
func (h *Handler) UpdateEmailNotification(c *gin.Context) {
	// 获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("UpdateEmailNotification Error: ", errors.New("user not authenticated"))
		return
	}

	email := c.Param("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Email parameter is required",
			},
		})
		logger.Error("UpdateEmailNotification Error: ", errors.New("email parameter is required"))
		return
	}

	var req types.UpdateEmailNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("UpdateEmailNotification Error: ", errors.New("invalid request parameters"), "error", err)
		return
	}

	// 调用服务
	response, err := h.emailNotificationService.UpdateEmailNotification(c.Request.Context(), walletAddress, email, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case email_notification.ErrEmailNotFound:
			statusCode = http.StatusNotFound
			errorCode = "EMAIL_NOT_FOUND"
		case email_notification.ErrEmailNotVerified:
			statusCode = http.StatusBadRequest
			errorCode = "EMAIL_NOT_VERIFIED"
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
		logger.Error("UpdateEmailNotification Error: ", err, "wallet_address", walletAddress, "email", email)
		return
	}

	logger.Info("UpdateEmailNotification: ", "wallet_address", walletAddress, "email", email)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// DeleteEmailNotification 删除邮件通知配置
// @Summary 删除邮件通知配置
// @Description 删除指定的邮件通知配置。删除后该邮箱将不再接收timelock合约的通知邮件。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param email path string true "邮箱地址"
// @Success 200 {object} types.APIResponse "删除成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "邮件配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications/{email} [delete]
func (h *Handler) DeleteEmailNotification(c *gin.Context) {
	// 获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("DeleteEmailNotification Error: ", errors.New("user not authenticated"))
		return
	}

	email := c.Param("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Email parameter is required",
			},
		})
		logger.Error("DeleteEmailNotification Error: ", errors.New("email parameter is required"))
		return
	}

	// 调用服务
	err := h.emailNotificationService.DeleteEmailNotification(c.Request.Context(), walletAddress, email)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case email_notification.ErrEmailNotFound:
			statusCode = http.StatusNotFound
			errorCode = "EMAIL_NOT_FOUND"
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
		logger.Error("DeleteEmailNotification Error: ", err, "wallet_address", walletAddress, "email", email)
		return
	}

	logger.Info("DeleteEmailNotification: ", "wallet_address", walletAddress, "email", email)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Email notification deleted successfully"},
	})
}

// GetEmailSendLogs 获取邮件发送记录
// @Summary 获取邮件发送记录
// @Description 获取用户相关的邮件发送记录，包括发送状态、时间和应急邮件回复状态等信息。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} types.APIResponse{data=[]types.EmailSendLogResponse} "获取成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications/logs [get]
func (h *Handler) GetEmailSendLogs(c *gin.Context) {
	// 获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("GetEmailSendLogs Error: ", errors.New("user not authenticated"))
		return
	}

	// 获取分页参数
	page := 1
	pageSize := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// 调用服务
	logs, total, err := h.emailNotificationService.GetEmailSendLogs(c.Request.Context(), walletAddress, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetEmailSendLogs Error: ", err, "wallet_address", walletAddress)
		return
	}

	logger.Info("GetEmailSendLogs: ", "wallet_address", walletAddress, "total", total)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data: gin.H{
			"items":       logs,
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"total_pages": (total + pageSize - 1) / pageSize,
		},
	})
}

// GetVerifiedEmails 获取已验证邮箱列表（用于timelock合约选择）
// @Summary 获取已验证邮箱列表
// @Description 获取用户的已验证邮箱列表，用于在timelock合约管理中选择监听邮箱。只返回已验证且激活的邮箱。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} types.APIResponse{data=[]types.EmailNotificationResponse} "获取成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications/verified-emails [get]
func (h *Handler) GetVerifiedEmails(c *gin.Context) {
	// 获取用户信息
	_, walletAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("GetVerifiedEmails Error: ", errors.New("user not authenticated"))
		return
	}

	// 调用服务
	emails, err := h.emailNotificationService.GetVerifiedEmailsByWallet(c.Request.Context(), walletAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetVerifiedEmails Error: ", err, "wallet_address", walletAddress)
		return
	}

	logger.Info("GetVerifiedEmails: ", "wallet_address", walletAddress, "count", len(emails))
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    emails,
	})
}

// EmergencyReply 应急邮件回复（公开端点）
// @Summary 应急邮件回复
// @Description 处理应急邮件的回复确认。用户点击应急邮件中的确认按钮时调用此接口。不需要认证，通过token验证身份。
// @Tags Email Notifications
// @Accept json
// @Produce json
// @Param token query string true "应急邮件回复token"
// @Success 200 {object} types.APIResponse{data=types.EmergencyReplyResponse} "回复成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "回复token不存在或已过期"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/email-notifications/emergency-reply [get]
func (h *Handler) EmergencyReply(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Token parameter is required",
			},
		})
		logger.Error("EmergencyReply Error: ", errors.New("token parameter is required"))
		return
	}

	// 调用服务
	response, err := h.emailNotificationService.ReplyEmergencyEmail(c.Request.Context(), token)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case email_notification.ErrEmergencyReplyNotFound:
			statusCode = http.StatusNotFound
			errorCode = "EMERGENCY_REPLY_NOT_FOUND"
		case email_notification.ErrEmergencyAlreadyReplied:
			statusCode = http.StatusBadRequest
			errorCode = "EMERGENCY_ALREADY_REPLIED"
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
		logger.Error("EmergencyReply Error: ", err, "token", token)
		return
	}

	logger.Info("EmergencyReply: ", "token", token, "replied_at", response.RepliedAt)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}
