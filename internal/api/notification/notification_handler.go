package notification

import (
	"net/http"
	"strings"
	"timelocker-backend/internal/middleware"
	"timelocker-backend/internal/service/auth"
	"timelocker-backend/internal/service/notification"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// NotificationHandler 通知API处理器
type NotificationHandler struct {
	notificationService notification.NotificationService
	authService         auth.Service
}

// NewNotificationHandler 创建通知处理器实例
func NewNotificationHandler(notificationService notification.NotificationService, authService auth.Service) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
		authService:         authService,
	}
}

// RegisterRoutes 注册通知相关路由
func (h *NotificationHandler) RegisterRoutes(router *gin.RouterGroup) {
	// 通知API组 - 需要认证
	notificationGroup := router.Group("/notifications", middleware.AuthMiddleware(h.authService))
	{
		// 获取所有通知配置
		// POST /api/v1/notifications/configs
		// http://localhost:8080/api/v1/notifications/configs
		notificationGroup.POST("/configs", h.GetAllNotificationConfigs)

		// Telegram配置管理
		telegramGroup := notificationGroup.Group("/telegram")
		{
			// 创建Telegram配置
			// POST /api/v1/notifications/telegram/create
			// http://localhost:8080/api/v1/notifications/telegram/create
			telegramGroup.POST("/create", h.CreateTelegramConfig)
			// 获取Telegram配置列表
			// POST /api/v1/notifications/telegram/list
			// http://localhost:8080/api/v1/notifications/telegram/list
			telegramGroup.POST("/list", h.GetTelegramConfigs)
			// 更新Telegram配置
			// POST /api/v1/notifications/telegram/update
			// http://localhost:8080/api/v1/notifications/telegram/update
			telegramGroup.POST("/update", h.UpdateTelegramConfig)
			// 删除Telegram配置
			// POST /api/v1/notifications/telegram/delete
			// http://localhost:8080/api/v1/notifications/telegram/delete
			telegramGroup.POST("/delete", h.DeleteTelegramConfig)
		}

		// Lark配置管理
		larkGroup := notificationGroup.Group("/lark")
		{
			// 创建Lark配置
			// POST /api/v1/notifications/lark/create
			// http://localhost:8080/api/v1/notifications/lark/create
			larkGroup.POST("/create", h.CreateLarkConfig)
			// 获取Lark配置列表
			// POST /api/v1/notifications/lark/list
			// http://localhost:8080/api/v1/notifications/lark/list
			larkGroup.POST("/list", h.GetLarkConfigs)
			// 更新Lark配置
			// POST /api/v1/notifications/lark/update
			// http://localhost:8080/api/v1/notifications/lark/update
			larkGroup.POST("/update", h.UpdateLarkConfig)
			// 删除Lark配置
			// POST /api/v1/notifications/lark/delete
			// http://localhost:8080/api/v1/notifications/lark/delete
			larkGroup.POST("/delete", h.DeleteLarkConfig)
		}

		// Feishu配置管理
		feishuGroup := notificationGroup.Group("/feishu")
		{
			// 创建Feishu配置
			// POST /api/v1/notifications/feishu/create
			// http://localhost:8080/api/v1/notifications/feishu/create
			feishuGroup.POST("/create", h.CreateFeishuConfig)
			// 获取Feishu配置列表
			// POST /api/v1/notifications/feishu/list
			// http://localhost:8080/api/v1/notifications/feishu/list
			feishuGroup.POST("/list", h.GetFeishuConfigs)
			// 更新Feishu配置
			// POST /api/v1/notifications/feishu/update
			// http://localhost:8080/api/v1/notifications/feishu/update
			feishuGroup.POST("/update", h.UpdateFeishuConfig)
			// 删除Feishu配置
			// POST /api/v1/notifications/feishu/delete
			// http://localhost:8080/api/v1/notifications/feishu/delete
			feishuGroup.POST("/delete", h.DeleteFeishuConfig)
		}
	}
}

// ===== 通用API =====

// GetAllNotificationConfigs 获取所有通知配置
// @Summary 获取所有通知配置
// @Description 获取当前用户的所有通知渠道配置
// @Tags Notification
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=types.NotificationConfigListResponse} "获取成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/configs [post]
func (h *NotificationHandler) GetAllNotificationConfigs(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("GetAllNotificationConfigs error", nil, "message", "user not authenticated")
		return
	}

	// 调用service层
	response, err := h.notificationService.GetAllNotificationConfigs(c.Request.Context(), userAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetAllNotificationConfigs error", err, "user_address", userAddress)
		return
	}

	logger.Info("GetAllNotificationConfigs success", "user_address", userAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// ===== Telegram配置管理 =====

// CreateTelegramConfig 创建Telegram配置
// @Summary 创建Telegram配置
// @Description 为当前用户创建新的Telegram通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Param request body types.CreateTelegramConfigRequest true "创建请求"
// @Success 200 {object} types.APIResponse{data=types.TelegramConfig} "创建成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "配置名称已存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/telegram/create [post]
func (h *NotificationHandler) CreateTelegramConfig(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("CreateTelegramConfig error", nil, "message", "user not authenticated")
		return
	}

	var req types.CreateTelegramConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("CreateTelegramConfig error", err, "message", "invalid request parameters", "user_address", userAddress)
		return
	}

	// 标准化名称
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_NAME",
				Message: "Name cannot be empty",
			},
		})
		return
	}

	// 调用service层
	result, err := h.notificationService.CreateTelegramConfig(c.Request.Context(), userAddress, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorCode = "CONFIG_EXISTS"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("CreateTelegramConfig error", err, "user_address", userAddress, "name", req.Name)
		return
	}

	logger.Info("CreateTelegramConfig success", "user_address", userAddress, "name", req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    result,
	})
}

// GetTelegramConfigs 获取Telegram配置列表
// @Summary 获取Telegram配置列表
// @Description 获取当前用户的所有Telegram通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=[]types.TelegramConfig} "获取成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/telegram/list [post]
func (h *NotificationHandler) GetTelegramConfigs(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("GetTelegramConfigs error", nil, "message", "user not authenticated")
		return
	}

	// 调用service层
	configs, err := h.notificationService.GetTelegramConfigs(c.Request.Context(), userAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetTelegramConfigs error", err, "user_address", userAddress)
		return
	}

	logger.Info("GetTelegramConfigs success", "user_address", userAddress, "count", len(configs))
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    configs,
	})
}

// UpdateTelegramConfig 更新Telegram配置
// @Summary 更新Telegram配置
// @Description 更新当前用户的Telegram通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Param request body types.UpdateTelegramConfigRequest true "更新请求"
// @Success 200 {object} types.APIResponse "更新成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/telegram/update [post]
func (h *NotificationHandler) UpdateTelegramConfig(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("UpdateTelegramConfig error", nil, "message", "user not authenticated")
		return
	}

	var req types.UpdateTelegramConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("UpdateTelegramConfig error", err, "message", "invalid request parameters", "user_address", userAddress)
		return
	}

	// 验证名称
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_NAME",
				Message: "Name is required",
			},
		})
		return
	}

	// 调用service层
	err := h.notificationService.UpdateTelegramConfig(c.Request.Context(), userAddress, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorCode = "CONFIG_NOT_FOUND"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("UpdateTelegramConfig error", err, "user_address", userAddress, "name", *req.Name)
		return
	}

	logger.Info("UpdateTelegramConfig success", "user_address", userAddress, "name", *req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Telegram config updated successfully"},
	})
}

// DeleteTelegramConfig 删除Telegram配置
// @Summary 删除Telegram配置
// @Description 删除当前用户的Telegram通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Param request body types.DeleteTelegramConfigRequest true "删除请求"
// @Success 200 {object} types.APIResponse "删除成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/telegram/delete [post]
func (h *NotificationHandler) DeleteTelegramConfig(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("DeleteTelegramConfig error", nil, "message", "user not authenticated")
		return
	}

	var req types.DeleteTelegramConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("DeleteTelegramConfig error", err, "message", "invalid request parameters", "user_address", userAddress)
		return
	}

	// 标准化名称
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_NAME",
				Message: "Name cannot be empty",
			},
		})
		return
	}

	// 调用service层
	err := h.notificationService.DeleteTelegramConfig(c.Request.Context(), userAddress, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorCode = "CONFIG_NOT_FOUND"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("DeleteTelegramConfig error", err, "user_address", userAddress, "name", req.Name)
		return
	}

	logger.Info("DeleteTelegramConfig success", "user_address", userAddress, "name", req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Telegram config deleted successfully"},
	})
}

// ===== Lark配置管理 =====

// CreateLarkConfig 创建Lark配置
// @Summary 创建Lark配置
// @Description 为当前用户创建新的Lark通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Param request body types.CreateLarkConfigRequest true "创建请求"
// @Success 200 {object} types.APIResponse{data=types.LarkConfig} "创建成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "配置名称已存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/lark/create [post]
func (h *NotificationHandler) CreateLarkConfig(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("CreateLarkConfig error", nil, "message", "user not authenticated")
		return
	}

	var req types.CreateLarkConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("CreateLarkConfig error", err, "message", "invalid request parameters", "user_address", userAddress)
		return
	}

	// 标准化名称
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_NAME",
				Message: "Name cannot be empty",
			},
		})
		return
	}

	// 调用service层
	result, err := h.notificationService.CreateLarkConfig(c.Request.Context(), userAddress, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorCode = "CONFIG_EXISTS"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("CreateLarkConfig error", err, "user_address", userAddress, "name", req.Name)
		return
	}

	logger.Info("CreateLarkConfig success", "user_address", userAddress, "name", req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    result,
	})
}

// GetLarkConfigs 获取Lark配置列表
// @Summary 获取Lark配置列表
// @Description 获取当前用户的所有Lark通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=[]types.LarkConfig} "获取成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/lark/list [post]
func (h *NotificationHandler) GetLarkConfigs(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("GetLarkConfigs error", nil, "message", "user not authenticated")
		return
	}

	// 调用service层
	configs, err := h.notificationService.GetLarkConfigs(c.Request.Context(), userAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetLarkConfigs error", err, "user_address", userAddress)
		return
	}

	logger.Info("GetLarkConfigs success", "user_address", userAddress, "count", len(configs))
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    configs,
	})
}

// UpdateLarkConfig 更新Lark配置
// @Summary 更新Lark配置
// @Description 更新当前用户的Lark通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Param request body types.UpdateLarkConfigRequest true "更新请求"
// @Success 200 {object} types.APIResponse "更新成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/lark/update [post]
func (h *NotificationHandler) UpdateLarkConfig(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("UpdateLarkConfig error", nil, "message", "user not authenticated")
		return
	}

	var req types.UpdateLarkConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("UpdateLarkConfig error", err, "message", "invalid request parameters", "user_address", userAddress)
		return
	}

	// 验证名称
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_NAME",
				Message: "Name is required",
			},
		})
		return
	}

	// 调用service层
	err := h.notificationService.UpdateLarkConfig(c.Request.Context(), userAddress, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorCode = "CONFIG_NOT_FOUND"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("UpdateLarkConfig error", err, "user_address", userAddress, "name", *req.Name)
		return
	}

	logger.Info("UpdateLarkConfig success", "user_address", userAddress, "name", *req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Lark config updated successfully"},
	})
}

// DeleteLarkConfig 删除Lark配置
// @Summary 删除Lark配置
// @Description 删除当前用户的Lark通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Param request body types.DeleteLarkConfigRequest true "删除请求"
// @Success 200 {object} types.APIResponse "删除成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/lark/delete [post]
func (h *NotificationHandler) DeleteLarkConfig(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("DeleteLarkConfig error", nil, "message", "user not authenticated")
		return
	}

	var req types.DeleteLarkConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("DeleteLarkConfig error", err, "message", "invalid request parameters", "user_address", userAddress)
		return
	}

	// 标准化名称
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_NAME",
				Message: "Name cannot be empty",
			},
		})
		return
	}

	// 调用service层
	err := h.notificationService.DeleteLarkConfig(c.Request.Context(), userAddress, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorCode = "CONFIG_NOT_FOUND"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("DeleteLarkConfig error", err, "user_address", userAddress, "name", req.Name)
		return
	}

	logger.Info("DeleteLarkConfig success", "user_address", userAddress, "name", req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Lark config deleted successfully"},
	})
}

// ===== Feishu配置管理 =====

// CreateFeishuConfig 创建Feishu配置
// @Summary 创建Feishu配置
// @Description 为当前用户创建新的Feishu通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Param request body types.CreateFeishuConfigRequest true "创建请求"
// @Success 200 {object} types.APIResponse{data=types.FeishuConfig} "创建成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "配置名称已存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/feishu/create [post]
func (h *NotificationHandler) CreateFeishuConfig(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("CreateFeishuConfig error", nil, "message", "user not authenticated")
		return
	}

	var req types.CreateFeishuConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("CreateFeishuConfig error", err, "message", "invalid request parameters", "user_address", userAddress)
		return
	}

	// 标准化名称
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_NAME",
				Message: "Name cannot be empty",
			},
		})
		return
	}

	// 调用service层
	result, err := h.notificationService.CreateFeishuConfig(c.Request.Context(), userAddress, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorCode = "CONFIG_EXISTS"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("CreateFeishuConfig error", err, "user_address", userAddress, "name", req.Name)
		return
	}

	logger.Info("CreateFeishuConfig success", "user_address", userAddress, "name", req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    result,
	})
}

// GetFeishuConfigs 获取Feishu配置列表
// @Summary 获取Feishu配置列表
// @Description 获取当前用户的所有Feishu通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=[]types.FeishuConfig} "获取成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/feishu/list [post]
func (h *NotificationHandler) GetFeishuConfigs(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("GetFeishuConfigs error", nil, "message", "user not authenticated")
		return
	}

	// 调用service层
	configs, err := h.notificationService.GetFeishuConfigs(c.Request.Context(), userAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetFeishuConfigs error", err, "user_address", userAddress)
		return
	}

	logger.Info("GetFeishuConfigs success", "user_address", userAddress, "count", len(configs))
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    configs,
	})
}

// UpdateFeishuConfig 更新Feishu配置
// @Summary 更新Feishu配置
// @Description 更新当前用户的Feishu通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Param request body types.UpdateFeishuConfigRequest true "更新请求"
// @Success 200 {object} types.APIResponse "更新成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/feishu/update [post]
func (h *NotificationHandler) UpdateFeishuConfig(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("UpdateFeishuConfig error", nil, "message", "user not authenticated")
		return
	}

	var req types.UpdateFeishuConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("UpdateFeishuConfig error", err, "message", "invalid request parameters", "user_address", userAddress)
		return
	}

	// 验证名称
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_NAME",
				Message: "Name is required",
			},
		})
		return
	}

	// 调用service层
	err := h.notificationService.UpdateFeishuConfig(c.Request.Context(), userAddress, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorCode = "CONFIG_NOT_FOUND"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("UpdateFeishuConfig error", err, "user_address", userAddress, "name", *req.Name)
		return
	}

	logger.Info("UpdateFeishuConfig success", "user_address", userAddress, "name", *req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Feishu config updated successfully"},
	})
}

// DeleteFeishuConfig 删除Feishu配置
// @Summary 删除Feishu配置
// @Description 删除当前用户的Feishu通知配置
// @Tags Notification
// @Accept json
// @Produce json
// @Param request body types.DeleteFeishuConfigRequest true "删除请求"
// @Success 200 {object} types.APIResponse "删除成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "配置不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/notifications/feishu/delete [post]
func (h *NotificationHandler) DeleteFeishuConfig(c *gin.Context) {
	// 从上下文获取用户信息
	_, userAddress, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("DeleteFeishuConfig error", nil, "message", "user not authenticated")
		return
	}

	var req types.DeleteFeishuConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		logger.Error("DeleteFeishuConfig error", err, "message", "invalid request parameters", "user_address", userAddress)
		return
	}

	// 标准化名称
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_NAME",
				Message: "Name cannot be empty",
			},
		})
		return
	}

	// 调用service层
	err := h.notificationService.DeleteFeishuConfig(c.Request.Context(), userAddress, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorCode := "INTERNAL_ERROR"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorCode = "CONFIG_NOT_FOUND"
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("DeleteFeishuConfig error", err, "user_address", userAddress, "name", req.Name)
		return
	}

	logger.Info("DeleteFeishuConfig success", "user_address", userAddress, "name", req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Feishu config deleted successfully"},
	})
}
