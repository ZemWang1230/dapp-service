package timelock

import (
	"errors"
	"net/http"
	"strconv"

	"timelocker-backend/internal/middleware"
	"timelocker-backend/internal/service/auth"
	"timelocker-backend/internal/service/timelock"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Handler timelock处理器
type Handler struct {
	timeLockService timelock.Service
	authService     auth.Service
}

// NewHandler 创建timelock处理器
func NewHandler(timeLockService timelock.Service, authService auth.Service) *Handler {
	return &Handler{
		timeLockService: timeLockService,
		authService:     authService,
	}
}

// RegisterRoutes 注册timelock相关路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// 创建timelock路由组
	timeLockGroup := router.Group("/timelock")
	timeLockGroup.Use(middleware.AuthMiddleware(h.authService))
	{
		// 检查timelock状态
		// GET /api/v1/timelock/status
		// http://localhost:8080/api/v1/timelock/status
		timeLockGroup.GET("/status", h.CheckTimeLockStatus)

		// 创建timelock合约
		// POST /api/v1/timelock/create
		// http://localhost:8080/api/v1/timelock/create
		timeLockGroup.POST("/create", h.CreateTimeLock)

		// 导入timelock合约
		// POST /api/v1/timelock/import
		// http://localhost:8080/api/v1/timelock/import
		timeLockGroup.POST("/import", h.ImportTimeLock)

		// 获取timelock列表
		// GET /api/v1/timelock/list
		// http://localhost:8080/api/v1/timelock/list
		timeLockGroup.GET("/list", h.GetTimeLockList)

		// 获取timelock详情
		// GET /api/v1/timelock/:id
		// http://localhost:8080/api/v1/timelock/1
		timeLockGroup.GET("/:id", h.GetTimeLockDetail)

		// 更新timelock备注
		// PUT /api/v1/timelock/:id
		// http://localhost:8080/api/v1/timelock/1
		timeLockGroup.PUT("/:id", h.UpdateTimeLock)

		// 删除timelock
		// DELETE /api/v1/timelock/:id
		// http://localhost:8080/api/v1/timelock/1
		timeLockGroup.DELETE("/:id", h.DeleteTimeLock)
	}
}

// CheckTimeLockStatus 检查timelock状态
// @Summary 检查timelock状态
// @Description 检查用户是否有timelock合约，返回timelock状态
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} types.APIResponse{data=types.CheckTimeLockStatusResponse}
// @Failure 401 {object} types.APIResponse
// @Failure 500 {object} types.APIResponse
// @Router /api/v1/timelock/status [get]
func (h *Handler) CheckTimeLockStatus(c *gin.Context) {
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
		logger.Error("CheckTimeLockStatus Error: ", errors.New("user not authenticated"))
		return
	}

	// 调用service层
	response, err := h.timeLockService.CheckTimeLockStatus(c.Request.Context(), walletAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("CheckTimeLockStatus Error: ", err, "wallet_address", walletAddress)
		return
	}

	logger.Info("CheckTimeLockStatus Success: ", "wallet_address", walletAddress, "has_timelocks", response.HasTimeLocks)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// CreateTimeLock 创建timelock合约
// @Summary 创建timelock合约
// @Description 创建新的timelock合约记录
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.CreateTimeLockRequest true "创建timelock请求"
// @Success 200 {object} types.APIResponse{data=types.TimeLock}
// @Failure 400 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse
// @Failure 409 {object} types.APIResponse
// @Failure 500 {object} types.APIResponse
// @Router /api/v1/timelock/create [post]
func (h *Handler) CreateTimeLock(c *gin.Context) {
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
		logger.Error("CreateTimeLock Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.CreateTimeLockRequest
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
		logger.Error("CreateTimeLock Error: ", errors.New("invalid request parameters"), "error", err, "wallet_address", walletAddress)
		return
	}

	// 调用service层
	timeLock, err := h.timeLockService.CreateTimeLock(c.Request.Context(), walletAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrTimeLockExists:
			statusCode = http.StatusConflict
			errorCode = "TIMELOCK_EXISTS"
		case timelock.ErrInvalidContractParams:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_PARAMETERS"
		case timelock.ErrInvalidStandard:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_STANDARD"
		case timelock.ErrInvalidRemark:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_REMARK"
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
		logger.Error("CreateTimeLock Error: ", err, "wallet_address", walletAddress, "error_code", errorCode)
		return
	}

	logger.Info("CreateTimeLock Success: ", "wallet_address", walletAddress, "timelock_id", timeLock.ID, "contract_address", timeLock.ContractAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    timeLock,
	})
}

// ImportTimeLock 导入timelock合约
// @Summary 导入timelock合约
// @Description 导入已存在的timelock合约
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.ImportTimeLockRequest true "导入timelock请求"
// @Success 200 {object} types.APIResponse{data=types.TimeLock}
// @Failure 400 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse
// @Failure 409 {object} types.APIResponse
// @Failure 500 {object} types.APIResponse
// @Router /api/v1/timelock/import [post]
func (h *Handler) ImportTimeLock(c *gin.Context) {
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
		logger.Error("ImportTimeLock Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.ImportTimeLockRequest
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
		logger.Error("ImportTimeLock Error: ", errors.New("invalid request parameters"), "error", err, "wallet_address", walletAddress)
		return
	}

	// 调用service层
	timeLock, err := h.timeLockService.ImportTimeLock(c.Request.Context(), walletAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrTimeLockExists:
			statusCode = http.StatusConflict
			errorCode = "TIMELOCK_EXISTS"
		case timelock.ErrInvalidContract:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_CONTRACT"
		case timelock.ErrInvalidStandard:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_STANDARD"
		case timelock.ErrInvalidContractParams:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_PARAMETERS"
		case timelock.ErrInvalidRemark:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_REMARK"
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
		logger.Error("ImportTimeLock Error: ", err, "wallet_address", walletAddress, "error_code", errorCode)
		return
	}

	logger.Info("ImportTimeLock Success: ", "wallet_address", walletAddress, "timelock_id", timeLock.ID, "contract_address", timeLock.ContractAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    timeLock,
	})
}

// GetTimeLockList 获取timelock列表
// @Summary 获取timelock列表
// @Description 获取用户的timelock合约列表（分页）
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param chain_id query int false "链ID筛选"
// @Param standard query string false "合约标准筛选" Enums(compound,openzeppelin)
// @Param status query string false "状态筛选" Enums(active,inactive)
// @Success 200 {object} types.APIResponse{data=types.GetTimeLockListResponse}
// @Failure 400 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse
// @Failure 500 {object} types.APIResponse
// @Router /api/v1/timelock/list [get]
func (h *Handler) GetTimeLockList(c *gin.Context) {
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
		logger.Error("GetTimeLockList Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.GetTimeLockListRequest
	// 绑定查询参数
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid query parameters",
				Details: err.Error(),
			},
		})
		logger.Error("GetTimeLockList Error: ", errors.New("invalid query parameters"), "error", err, "wallet_address", walletAddress)
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	// 调用service层
	response, err := h.timeLockService.GetTimeLockList(c.Request.Context(), walletAddress, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetTimeLockList Error: ", err, "wallet_address", walletAddress)
		return
	}

	logger.Info("GetTimeLockList Success: ", "wallet_address", walletAddress, "total", response.Total, "page", response.Page)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetTimeLockDetail 获取timelock详情
// @Summary 获取timelock详情
// @Description 获取指定timelock合约的详细信息
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Timelock ID"
// @Success 200 {object} types.APIResponse{data=types.TimeLockDetailResponse}
// @Failure 400 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse
// @Failure 403 {object} types.APIResponse
// @Failure 404 {object} types.APIResponse
// @Failure 500 {object} types.APIResponse
// @Router /api/v1/timelock/{id} [get]
func (h *Handler) GetTimeLockDetail(c *gin.Context) {
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
		logger.Error("GetTimeLockDetail Error: ", errors.New("user not authenticated"))
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
				Message: "Invalid timelock ID",
				Details: err.Error(),
			},
		})
		logger.Error("GetTimeLockDetail Error: ", errors.New("invalid timelock ID"), "id", idStr, "wallet_address", walletAddress)
		return
	}

	// 调用service层
	response, err := h.timeLockService.GetTimeLockDetail(c.Request.Context(), walletAddress, id)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrTimeLockNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TIMELOCK_NOT_FOUND"
		case timelock.ErrUnauthorized:
			statusCode = http.StatusForbidden
			errorCode = "UNAUTHORIZED_ACCESS"
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
		logger.Error("GetTimeLockDetail Error: ", err, "wallet_address", walletAddress, "timelock_id", id, "error_code", errorCode)
		return
	}

	logger.Info("GetTimeLockDetail Success: ", "wallet_address", walletAddress, "timelock_id", id)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// UpdateTimeLock 更新timelock
// @Summary 更新timelock备注
// @Description 更新timelock合约的备注信息
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Timelock ID"
// @Param request body types.UpdateTimeLockRequest true "更新timelock请求"
// @Success 200 {object} types.APIResponse
// @Failure 400 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse
// @Failure 403 {object} types.APIResponse
// @Failure 404 {object} types.APIResponse
// @Failure 500 {object} types.APIResponse
// @Router /api/v1/timelock/{id} [put]
func (h *Handler) UpdateTimeLock(c *gin.Context) {
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
		logger.Error("UpdateTimeLock Error: ", errors.New("user not authenticated"))
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
				Message: "Invalid timelock ID",
				Details: err.Error(),
			},
		})
		logger.Error("UpdateTimeLock Error: ", errors.New("invalid timelock ID"), "id", idStr, "wallet_address", walletAddress)
		return
	}

	var req types.UpdateTimeLockRequest
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
		logger.Error("UpdateTimeLock Error: ", errors.New("invalid request parameters"), "error", err, "wallet_address", walletAddress)
		return
	}

	// 设置ID
	req.ID = id

	// 调用service层
	err = h.timeLockService.UpdateTimeLock(c.Request.Context(), walletAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrTimeLockNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TIMELOCK_NOT_FOUND"
		case timelock.ErrUnauthorized:
			statusCode = http.StatusForbidden
			errorCode = "UNAUTHORIZED_ACCESS"
		case timelock.ErrInvalidRemark:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_REMARK"
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
		logger.Error("UpdateTimeLock Error: ", err, "wallet_address", walletAddress, "timelock_id", id, "error_code", errorCode)
		return
	}

	logger.Info("UpdateTimeLock Success: ", "wallet_address", walletAddress, "timelock_id", id)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Timelock updated successfully"},
	})
}

// DeleteTimeLock 删除timelock
// @Summary 删除timelock
// @Description 删除指定的timelock合约（软删除）
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Timelock ID"
// @Success 200 {object} types.APIResponse
// @Failure 400 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse
// @Failure 403 {object} types.APIResponse
// @Failure 404 {object} types.APIResponse
// @Failure 500 {object} types.APIResponse
// @Router /api/v1/timelock/{id} [delete]
func (h *Handler) DeleteTimeLock(c *gin.Context) {
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
		logger.Error("DeleteTimeLock Error: ", errors.New("user not authenticated"))
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
				Message: "Invalid timelock ID",
				Details: err.Error(),
			},
		})
		logger.Error("DeleteTimeLock Error: ", errors.New("invalid timelock ID"), "id", idStr, "wallet_address", walletAddress)
		return
	}

	// 构建请求
	req := &types.DeleteTimeLockRequest{
		ID: id,
	}

	// 调用service层
	err = h.timeLockService.DeleteTimeLock(c.Request.Context(), walletAddress, req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrTimeLockNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TIMELOCK_NOT_FOUND"
		case timelock.ErrUnauthorized:
			statusCode = http.StatusForbidden
			errorCode = "UNAUTHORIZED_ACCESS"
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
		logger.Error("DeleteTimeLock Error: ", err, "wallet_address", walletAddress, "timelock_id", id, "error_code", errorCode)
		return
	}

	logger.Info("DeleteTimeLock Success: ", "wallet_address", walletAddress, "timelock_id", id)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Timelock deleted successfully"},
	})
}
