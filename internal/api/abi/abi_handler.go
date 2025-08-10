package abi

import (
	"errors"
	"net/http"
	"strconv"

	"timelocker-backend/internal/middleware"
	abiService "timelocker-backend/internal/service/abi"
	authService "timelocker-backend/internal/service/auth"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Handler ABI处理器
type Handler struct {
	abiService  abiService.Service
	authService authService.Service
}

// NewHandler 创建ABI处理器
func NewHandler(abiService abiService.Service, authService authService.Service) *Handler {
	return &Handler{
		abiService:  abiService,
		authService: authService,
	}
}

// RegisterRoutes 注册ABI相关路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// 创建ABI路由组
	abiGroup := router.Group("/abi")
	{
		// 需要认证的端点
		abiGroup.Use(middleware.AuthMiddleware(h.authService))

		// 获取ABI列表（用户的+共享的）
		// GET /api/v1/abi/list
		abiGroup.GET("/list", h.GetABIList)

		// 创建新的ABI
		// POST /api/v1/abi
		abiGroup.POST("", h.CreateABI)

		// 验证ABI格式
		// POST /api/v1/abi/validate
		abiGroup.POST("/validate", h.ValidateABI)

		// 获取ABI详情
		// GET /api/v1/abi/:id
		abiGroup.GET("/:id", h.GetABIByID)

		// 更新ABI
		// PUT /api/v1/abi/:id
		abiGroup.PUT("/:id", h.UpdateABI)

		// 删除ABI
		// DELETE /api/v1/abi/:id
		abiGroup.DELETE("/:id", h.DeleteABI)
	}
}

// CreateABI 创建新的ABI
// @Summary 创建ABI
// @Description 用户创建新的智能合约ABI。系统会验证ABI格式的正确性。每个用户在同一名称下只能创建一个ABI。
// @Tags ABI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.CreateABIRequest true "创建ABI请求体"
// @Success 201 {object} types.APIResponse{data=types.ABIResponse} "ABI创建成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误或ABI格式无效"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "ABI名称已存在"
// @Failure 422 {object} types.APIResponse{error=types.APIError} "参数校验失败"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/abi [post]
func (h *Handler) CreateABI(c *gin.Context) {
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
		logger.Error("CreateABI Error:", errors.New("user not authenticated"))
		return
	}

	var req types.CreateABIRequest
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
		logger.Error("CreateABI Error:", errors.New("invalid request parameters"), "error", err)
		return
	}

	// 调用服务层
	response, err := h.abiService.CreateABI(c.Request.Context(), walletAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch {
		case errors.Is(err, abiService.ErrInvalidABI):
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_ABI"
		case errors.Is(err, abiService.ErrABINameExists):
			statusCode = http.StatusConflict
			errorCode = "ABI_NAME_EXISTS"
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
		logger.Error("CreateABI Error:", err, "wallet_address", walletAddress, "name", req.Name)
		return
	}

	logger.Info("CreateABI Success:", "wallet_address", walletAddress, "name", req.Name, "id", response.ID)
	c.JSON(http.StatusCreated, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetABIList 获取ABI列表
// @Summary 获取ABI列表
// @Description 获取用户可访问的ABI列表，包括用户自己创建的ABI和平台共享的ABI。返回结果分为两个部分：用户ABI和共享ABI。
// @Tags ABI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} types.APIResponse{data=types.ABIListResponse} "获取ABI列表成功"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/abi/list [get]
func (h *Handler) GetABIList(c *gin.Context) {
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
		logger.Error("GetABIList Error:", errors.New("user not authenticated"))
		return
	}

	// 调用服务层
	response, err := h.abiService.GetABIList(c.Request.Context(), walletAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetABIList Error:", err, "wallet_address", walletAddress)
		return
	}

	logger.Info("GetABIList Success:", "wallet_address", walletAddress, "user_count", len(response.UserABIs), "shared_count", len(response.SharedABIs))
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetABIByID 根据ID获取ABI详情
// @Summary 获取ABI详情
// @Description 根据ABI ID获取详细信息。用户只能访问自己创建的ABI或平台共享的ABI。
// @Tags ABI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ABI ID"
// @Success 200 {object} types.APIResponse{data=types.ABIResponse} "获取ABI详情成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "无效的ABI ID"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "无权访问该ABI"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "ABI不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/abi/{id} [get]
func (h *Handler) GetABIByID(c *gin.Context) {
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
		logger.Error("GetABIByID Error:", errors.New("user not authenticated"))
		return
	}

	// 获取路径参数
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_ID",
				Message: "Invalid ABI ID",
				Details: err.Error(),
			},
		})
		logger.Error("GetABIByID Error:", errors.New("invalid ABI ID"), "id", idStr)
		return
	}

	// 调用服务层
	response, err := h.abiService.GetABIByID(c.Request.Context(), id, walletAddress)
	if err != nil {
		var statusCode int
		var errorCode string

		switch {
		case errors.Is(err, abiService.ErrABINotFound):
			statusCode = http.StatusNotFound
			errorCode = "ABI_NOT_FOUND"
		case errors.Is(err, abiService.ErrAccessDenied):
			statusCode = http.StatusForbidden
			errorCode = "ACCESS_DENIED"
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
		logger.Error("GetABIByID Error:", err, "id", id, "wallet_address", walletAddress)
		return
	}

	logger.Info("GetABIByID Success:", "id", id, "wallet_address", walletAddress, "name", response.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// UpdateABI 更新ABI
// @Summary 更新ABI
// @Description 更新用户创建的ABI。系统会重新验证ABI格式。用户只能更新自己创建的ABI。
// @Tags ABI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ABI ID"
// @Param request body types.UpdateABIRequest true "更新ABI请求体"
// @Success 200 {object} types.APIResponse{data=types.ABIResponse} "ABI更新成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误或ABI格式无效"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "无权更新该ABI"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "ABI不存在"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "ABI名称已存在"
// @Failure 422 {object} types.APIResponse{error=types.APIError} "参数校验失败"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/abi/{id} [put]
func (h *Handler) UpdateABI(c *gin.Context) {
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
		logger.Error("UpdateABI Error:", errors.New("user not authenticated"))
		return
	}

	// 获取路径参数
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_ID",
				Message: "Invalid ABI ID",
				Details: err.Error(),
			},
		})
		logger.Error("UpdateABI Error:", errors.New("invalid ABI ID"), "id", idStr)
		return
	}

	var req types.UpdateABIRequest
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
		logger.Error("UpdateABI Error:", errors.New("invalid request parameters"), "error", err)
		return
	}

	// 调用服务层
	response, err := h.abiService.UpdateABI(c.Request.Context(), id, walletAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch {
		case errors.Is(err, abiService.ErrABINotFound):
			statusCode = http.StatusNotFound
			errorCode = "ABI_NOT_FOUND"
		case errors.Is(err, abiService.ErrAccessDenied):
			statusCode = http.StatusForbidden
			errorCode = "ACCESS_DENIED"
		case errors.Is(err, abiService.ErrInvalidABI):
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_ABI"
		case errors.Is(err, abiService.ErrABINameExists):
			statusCode = http.StatusConflict
			errorCode = "ABI_NAME_EXISTS"
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
		logger.Error("UpdateABI Error:", err, "id", id, "wallet_address", walletAddress)
		return
	}

	logger.Info("UpdateABI Success:", "id", id, "wallet_address", walletAddress, "name", req.Name)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// DeleteABI 删除ABI
// @Summary 删除ABI
// @Description 删除用户创建的ABI。用户只能删除自己创建的ABI，不能删除平台共享的ABI。删除操作是不可逆的。
// @Tags ABI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ABI ID"
// @Success 200 {object} types.APIResponse "ABI删除成功"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "无效的ABI ID"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "无权删除该ABI或尝试删除共享ABI"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "ABI不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/abi/{id} [delete]
func (h *Handler) DeleteABI(c *gin.Context) {
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
		logger.Error("DeleteABI Error:", errors.New("user not authenticated"))
		return
	}

	// 获取路径参数
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_ID",
				Message: "Invalid ABI ID",
				Details: err.Error(),
			},
		})
		logger.Error("DeleteABI Error:", errors.New("invalid ABI ID"), "id", idStr)
		return
	}

	// 调用服务层
	err = h.abiService.DeleteABI(c.Request.Context(), id, walletAddress)
	if err != nil {
		var statusCode int
		var errorCode string

		switch {
		case errors.Is(err, abiService.ErrABINotFound):
			statusCode = http.StatusNotFound
			errorCode = "ABI_NOT_FOUND"
		case errors.Is(err, abiService.ErrAccessDenied):
			statusCode = http.StatusForbidden
			errorCode = "ACCESS_DENIED"
		case errors.Is(err, abiService.ErrCannotDeleteShared):
			statusCode = http.StatusForbidden
			errorCode = "CANNOT_DELETE_SHARED_ABI"
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
		logger.Error("DeleteABI Error:", err, "id", id, "wallet_address", walletAddress)
		return
	}

	logger.Info("DeleteABI Success:", "id", id, "wallet_address", walletAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "ABI deleted successfully"},
	})
}

// ValidateABI 验证ABI格式
// @Summary 验证ABI格式
// @Description 验证智能合约ABI的格式正确性，返回详细的验证结果。此接口可用于在创建或更新ABI前进行预验证。
// @Tags ABI
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body object{abi_content=string} true "验证ABI请求体"
// @Success 200 {object} types.APIResponse{data=types.ABIValidationResult} "ABI验证完成"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 422 {object} types.APIResponse{error=types.APIError} "参数校验失败"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/abi/validate [post]
func (h *Handler) ValidateABI(c *gin.Context) {
	// 从上下文获取用户信息（确保已认证）
	_, _, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User not authenticated",
			},
		})
		logger.Error("ValidateABI Error:", errors.New("user not authenticated"))
		return
	}

	var req struct {
		ABIContent string `json:"abi_content" binding:"required"`
	}

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
		logger.Error("ValidateABI Error:", errors.New("invalid request parameters"), "error", err)
		return
	}

	// 调用服务层
	result, err := h.abiService.ValidateABI(c.Request.Context(), req.ABIContent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("ValidateABI Error:", err)
		return
	}

	logger.Info("ValidateABI Success:", "is_valid", result.IsValid, "function_count", result.FunctionCount, "event_count", result.EventCount)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    result,
	})
}
