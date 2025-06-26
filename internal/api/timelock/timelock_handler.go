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
		// 创建timelock合约
		// POST /api/v1/timelock/create
		// http://localhost:8080/api/v1/timelock/create
		timeLockGroup.POST("/create", h.CreateTimeLock)

		// 导入timelock合约
		// POST /api/v1/timelock/import
		// http://localhost:8080/api/v1/timelock/import
		timeLockGroup.POST("/import", h.ImportTimeLock)

		// 获取timelock列表（根据用户权限筛选）
		// GET /api/v1/timelock/list
		// http://localhost:8080/api/v1/timelock/list
		timeLockGroup.GET("/list", h.GetTimeLockList)

		// Compound特有功能 - 设置pending admin
		// POST /api/v1/timelock/compound/:id/set-pending-admin
		// http://localhost:8080/api/v1/timelock/compound/1/set-pending-admin
		timeLockGroup.POST("/compound/:id/set-pending-admin", h.SetPendingAdmin)

		// Compound特有功能 - 接受admin权限
		// POST /api/v1/timelock/compound/:id/accept-admin
		// http://localhost:8080/api/v1/timelock/compound/1/accept-admin
		timeLockGroup.POST("/compound/:id/accept-admin", h.AcceptAdmin)

		// 检查用户对compound timelock的admin权限
		// GET /api/v1/timelock/compound/:id/admin-permissions
		// http://localhost:8080/api/v1/timelock/compound/1/admin-permissions
		timeLockGroup.GET("/compound/:id/admin-permissions", h.CheckAdminPermissions)

		// 获取timelock详情
		// GET /api/v1/timelock/detail/:standard/:id
		// http://localhost:8080/api/v1/timelock/detail/compound/1
		timeLockGroup.GET("/detail/:standard/:id", h.GetTimeLockDetail)

		// 更新timelock备注
		// PUT /api/v1/timelock/:standard/:id
		// http://localhost:8080/api/v1/timelock/compound/1
		timeLockGroup.PUT("/:standard/:id", h.UpdateTimeLock)

		// 删除timelock
		// DELETE /api/v1/timelock/:standard/:id
		// http://localhost:8080/api/v1/timelock/compound/1
		timeLockGroup.DELETE("/:standard/:id", h.DeleteTimeLock)
	}
}

// CreateTimeLock 创建timelock合约
// @Summary 创建timelock合约记录
// @Description 创建新的timelock合约记录。支持Compound和OpenZeppelin两种标准。前端需要提供合约的详细信息，包括链ID、合约地址、标准类型、创建交易哈希以及相关的治理参数。(Compound标准需要提供admin, pendingAdmin需要为空; OpenZeppelin标准需要提供proposers, executors, cancellers, admin需要为全0地址, proposers就是cancellers)
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.CreateTimeLockRequest true "创建timelock合约的请求体"
// @Success 200 {object} types.APIResponse{data=object} "成功创建timelock合约记录"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "timelock合约已存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/timelock/create [post]
func (h *Handler) CreateTimeLock(c *gin.Context) {
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
		logger.Error("CreateTimeLock Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 调用service层
	result, err := h.timeLockService.CreateTimeLock(c.Request.Context(), userAddress, &req)
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
		logger.Error("CreateTimeLock Error: ", err, "user_address", userAddress, "error_code", errorCode)
		return
	}

	logger.Info("CreateTimeLock Success: ", "user_address", userAddress, "standard", req.Standard, "contract_address", req.ContractAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    result,
	})
}

// ImportTimeLock 导入timelock合约
// @Summary 导入已存在的timelock合约
// @Description 导入已在区块链上部署的timelock合约。前端需要从区块链读取合约的创建参数(Compound标准需要提供admin, pendingAdmin有则传入, 没有则传空; OpenZeppelin标准需要提供proposers, executors, cancellers, admin需要为全0地址, proposers就是cancellers)并提供给后端，或者用户自己提供，用于精细化权限管理。
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.ImportTimeLockRequest true "导入timelock合约的请求体"
// @Success 200 {object} types.APIResponse{data=object} "成功导入timelock合约"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "timelock合约已存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/timelock/import [post]
func (h *Handler) ImportTimeLock(c *gin.Context) {
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
		logger.Error("ImportTimeLock Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 调用service层
	result, err := h.timeLockService.ImportTimeLock(c.Request.Context(), userAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrTimeLockExists:
			statusCode = http.StatusConflict
			errorCode = "TIMELOCK_EXISTS"
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
		logger.Error("ImportTimeLock Error: ", err, "user_address", userAddress, "error_code", errorCode)
		return
	}

	logger.Info("ImportTimeLock Success: ", "user_address", userAddress, "standard", req.Standard, "contract_address", req.ContractAddress)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    result,
	})
}

// GetTimeLockList 获取timelock列表
// @Summary 获取用户timelock合约列表（按权限筛选，所有链）
// @Description 获取当前用户在所有链上有权限访问的timelock合约列表。支持按合约标准和状态进行筛选。返回的列表根据用户权限进行精细控制，只显示用户作为创建者、管理员、提议者、执行者或取消者的合约。前端自行实现分页功能。
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param standard query string false "按合约标准筛选" Enums(compound,openzeppelin) example(openzeppelin)
// @Param status query string false "按状态筛选" Enums(active,inactive) example(active)
// @Success 200 {object} types.APIResponse{data=types.GetTimeLockListResponse} "成功获取timelock合约列表"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/timelock/list [get]
func (h *Handler) GetTimeLockList(c *gin.Context) {
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
		logger.Error("GetTimeLockList Error: ", errors.New("invalid query parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 调用service层
	response, err := h.timeLockService.GetTimeLockList(c.Request.Context(), userAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrInvalidStandard:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_STANDARD"
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
		logger.Error("GetTimeLockList Error: ", err, "user_address", userAddress)
		return
	}

	logger.Info("GetTimeLockList Success: ", "user_address", userAddress, "total", response.Total, "compound_count", len(response.CompoundTimeLocks), "openzeppelin_count", len(response.OpenzeppelinTimeLocks))
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetTimeLockDetail 获取timelock详情
// @Summary 获取timelock合约详细信息
// @Description 获取指定timelock合约的完整详细信息，包括合约的基本信息、治理参数以及用户权限信息。只有具有相应权限的用户才能查看详细信息。
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param standard path string true "合约标准" Enums(compound,openzeppelin) example(compound)
// @Param id path int true "Timelock合约的数据库ID" example(1)
// @Success 200 {object} types.APIResponse{data=types.TimeLockDetailResponse} "成功获取timelock合约详情"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "无权访问此timelock合约"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "timelock合约不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/timelock/detail/{standard}/{id} [get]
func (h *Handler) GetTimeLockDetail(c *gin.Context) {
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
		logger.Error("GetTimeLockDetail Error: ", errors.New("user not authenticated"))
		return
	}

	// 获取路径参数
	standardStr := c.Param("standard")
	standard := types.TimeLockStandard(standardStr)

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
		logger.Error("GetTimeLockDetail Error: ", errors.New("invalid timelock ID"), "id", idStr, "user_address", userAddress)
		return
	}

	// 验证标准
	if standard != types.CompoundStandard && standard != types.OpenzeppelinStandard {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_STANDARD",
				Message: "Invalid timelock standard",
			},
		})
		logger.Error("GetTimeLockDetail Error: ", errors.New("invalid timelock standard"), "standard", standardStr, "user_address", userAddress)
		return
	}

	// 调用service层
	response, err := h.timeLockService.GetTimeLockDetail(c.Request.Context(), userAddress, standard, id)
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
		case timelock.ErrInvalidStandard:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_STANDARD"
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
		logger.Error("GetTimeLockDetail Error: ", err, "user_address", userAddress, "timelock_id", id, "standard", standard, "error_code", errorCode)
		return
	}

	logger.Info("GetTimeLockDetail Success: ", "user_address", userAddress, "timelock_id", id, "standard", standard)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// UpdateTimeLock 更新timelock备注
// @Summary 更新timelock合约备注
// @Description 更新指定timelock合约的备注信息。只有合约的创建者/导入者才能更新备注。备注信息用于帮助用户管理和识别不同的timelock合约。
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param standard path string true "合约标准" Enums(compound,openzeppelin) example(compound)
// @Param id path int true "Timelock合约的数据库ID" example(1)
// @Param request body types.UpdateTimeLockRequest true "更新请求体"
// @Success 200 {object} types.APIResponse{data=object} "成功更新timelock合约备注"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "无权访问此timelock合约"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "timelock合约不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/timelock/{standard}/{id} [put]
func (h *Handler) UpdateTimeLock(c *gin.Context) {
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
		logger.Error("UpdateTimeLock Error: ", errors.New("user not authenticated"))
		return
	}

	// 获取路径参数
	standardStr := c.Param("standard")
	standard := types.TimeLockStandard(standardStr)

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
		logger.Error("UpdateTimeLock Error: ", errors.New("invalid timelock ID"), "id", idStr, "user_address", userAddress)
		return
	}

	// 验证标准
	if standard != types.CompoundStandard && standard != types.OpenzeppelinStandard {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_STANDARD",
				Message: "Invalid timelock standard",
			},
		})
		logger.Error("UpdateTimeLock Error: ", errors.New("invalid timelock standard"), "standard", standardStr, "user_address", userAddress)
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
		logger.Error("UpdateTimeLock Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 设置从路径获取的参数
	req.ID = id
	req.Standard = standard

	// 调用service层
	err = h.timeLockService.UpdateTimeLock(c.Request.Context(), userAddress, &req)
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
		logger.Error("UpdateTimeLock Error: ", err, "user_address", userAddress, "timelock_id", id, "standard", standard, "error_code", errorCode)
		return
	}

	logger.Info("UpdateTimeLock Success: ", "user_address", userAddress, "timelock_id", id, "standard", standard)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Timelock updated successfully"},
	})
}

// DeleteTimeLock 删除timelock
// @Summary 删除timelock合约记录
// @Description 软删除指定的timelock合约记录。只有合约的创建者/导入者才能删除合约记录。删除操作是软删除，数据仍保留在数据库中但标记为已删除状态。
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param standard path string true "合约标准" Enums(compound,openzeppelin) example(compound)
// @Param id path int true "Timelock合约的数据库ID" example(1)
// @Success 200 {object} types.APIResponse{data=object} "成功删除timelock合约记录"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "无权访问此timelock合约"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "timelock合约不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/timelock/{standard}/{id} [delete]
func (h *Handler) DeleteTimeLock(c *gin.Context) {
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
		logger.Error("DeleteTimeLock Error: ", errors.New("user not authenticated"))
		return
	}

	// 获取路径参数
	standardStr := c.Param("standard")
	standard := types.TimeLockStandard(standardStr)

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
		logger.Error("DeleteTimeLock Error: ", errors.New("invalid timelock ID"), "id", idStr, "user_address", userAddress)
		return
	}

	// 验证标准
	if standard != types.CompoundStandard && standard != types.OpenzeppelinStandard {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_STANDARD",
				Message: "Invalid timelock standard",
			},
		})
		logger.Error("DeleteTimeLock Error: ", errors.New("invalid timelock standard"), "standard", standardStr, "user_address", userAddress)
		return
	}

	// 构建请求
	req := &types.DeleteTimeLockRequest{
		ID:       id,
		Standard: standard,
	}

	// 调用service层
	err = h.timeLockService.DeleteTimeLock(c.Request.Context(), userAddress, req)
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
		case timelock.ErrInvalidStandard:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_STANDARD"
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
		logger.Error("DeleteTimeLock Error: ", err, "user_address", userAddress, "timelock_id", id, "standard", standard, "error_code", errorCode)
		return
	}

	logger.Info("DeleteTimeLock Success: ", "user_address", userAddress, "timelock_id", id, "standard", standard)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Timelock deleted successfully"},
	})
}

// SetPendingAdmin 设置pending admin
// @Summary 设置Compound timelock的pending admin
// @Description 为Compound标准的timelock合约设置pending admin，需要前端调用钱包实现，通过该timelock合约执行setPendingAdmin函数。只有当前admin才能执行此操作。这是Compound timelock权限转移流程的第一步。
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Compound timelock合约的数据库ID" example(1)
// @Param request body types.SetPendingAdminRequest true "设置pending admin的请求体"
// @Success 200 {object} types.APIResponse{data=object} "成功设置pending admin"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "权限不足"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "timelock合约不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/timelock/compound/{id}/set-pending-admin [post]
func (h *Handler) SetPendingAdmin(c *gin.Context) {
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
		logger.Error("SetPendingAdmin Error: ", errors.New("user not authenticated"))
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
		logger.Error("SetPendingAdmin Error: ", errors.New("invalid timelock ID"), "id", idStr, "user_address", userAddress)
		return
	}

	var req types.SetPendingAdminRequest
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
		logger.Error("SetPendingAdmin Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 设置ID
	req.ID = id

	// 调用service层
	err = h.timeLockService.SetPendingAdmin(c.Request.Context(), userAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrTimeLockNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TIMELOCK_NOT_FOUND"
		case timelock.ErrInvalidPermissions:
			statusCode = http.StatusForbidden
			errorCode = "INSUFFICIENT_PERMISSIONS"
		case timelock.ErrInvalidContractParams:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_PARAMETERS"
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
		logger.Error("SetPendingAdmin Error: ", err, "user_address", userAddress, "timelock_id", id, "error_code", errorCode)
		return
	}

	logger.Info("SetPendingAdmin Success: ", "user_address", userAddress, "timelock_id", id, "new_pending_admin", req.NewPendingAdmin)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Pending admin set successfully"},
	})
}

// AcceptAdmin 接受admin权限
// @Summary 接受Compound timelock的admin权限
// @Description 接受成为Compound标准timelock合约的admin，需要前端调用钱包实现，通过该timelock合约执行acceptAdmin函数。只有当前的pending admin才能执行此操作。这是Compound timelock权限转移流程的第二步。
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Compound timelock合约的数据库ID" example(1)
// @Success 200 {object} types.APIResponse{data=object} "成功接受admin权限"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "权限不足"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "timelock合约不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/timelock/compound/{id}/accept-admin [post]
func (h *Handler) AcceptAdmin(c *gin.Context) {
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
		logger.Error("AcceptAdmin Error: ", errors.New("user not authenticated"))
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
		logger.Error("AcceptAdmin Error: ", errors.New("invalid timelock ID"), "id", idStr, "user_address", userAddress)
		return
	}

	// 构建请求
	req := &types.AcceptAdminRequest{
		ID: id,
	}

	// 调用service层
	err = h.timeLockService.AcceptAdmin(c.Request.Context(), userAddress, req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrTimeLockNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TIMELOCK_NOT_FOUND"
		case timelock.ErrInvalidPermissions:
			statusCode = http.StatusForbidden
			errorCode = "INSUFFICIENT_PERMISSIONS"
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
		logger.Error("AcceptAdmin Error: ", err, "user_address", userAddress, "timelock_id", id, "error_code", errorCode)
		return
	}

	logger.Info("AcceptAdmin Success: ", "user_address", userAddress, "timelock_id", id)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Admin role accepted successfully"},
	})
}

// CheckAdminPermissions 检查admin权限
// @Summary 检查用户对Compound timelock的admin权限
// @Description 检查当前用户对指定Compound timelock合约的管理权限，返回用户是否可以设置pending admin和是否可以接受admin权限。
// @Tags Timelock
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Compound timelock合约的数据库ID" example(1)
// @Success 200 {object} types.APIResponse{data=types.CheckAdminPermissionResponse} "成功获取权限信息"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "timelock合约不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/timelock/compound/{id}/admin-permissions [get]
func (h *Handler) CheckAdminPermissions(c *gin.Context) {
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
		logger.Error("CheckAdminPermissions Error: ", errors.New("user not authenticated"))
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
		logger.Error("CheckAdminPermissions Error: ", errors.New("invalid timelock ID"), "id", idStr, "user_address", userAddress)
		return
	}

	// 构建请求
	req := &types.CheckAdminPermissionRequest{
		ID: id,
	}

	// 调用service层
	response, err := h.timeLockService.CheckAdminPermissions(c.Request.Context(), userAddress, req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case timelock.ErrTimeLockNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TIMELOCK_NOT_FOUND"
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
		logger.Error("CheckAdminPermissions Error: ", err, "user_address", userAddress, "timelock_id", id, "error_code", errorCode)
		return
	}

	logger.Info("CheckAdminPermissions Success: ", "user_address", userAddress, "timelock_id", id, "can_set_pending_admin", response.CanSetPendingAdmin, "can_accept_admin", response.CanAcceptAdmin)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}
