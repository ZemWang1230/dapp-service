package transaction

import (
	"errors"
	"net/http"
	"strconv"

	"timelocker-backend/internal/middleware"
	"timelocker-backend/internal/service/auth"
	"timelocker-backend/internal/service/transaction"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Handler 交易处理器
type Handler struct {
	transactionService transaction.Service
	authService        auth.Service
}

// NewHandler 创建交易处理器
func NewHandler(transactionService transaction.Service, authService auth.Service) *Handler {
	return &Handler{
		transactionService: transactionService,
		authService:        authService,
	}
}

// RegisterRoutes 注册交易相关路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// 创建交易路由组
	transactionGroup := router.Group("/transaction")
	transactionGroup.Use(middleware.AuthMiddleware(h.authService))
	{
		// 创建交易记录
		// POST /api/v1/transaction/create
		transactionGroup.POST("/create", h.CreateTransaction)

		// 获取交易列表
		// GET /api/v1/transaction/list
		transactionGroup.GET("/list", h.GetTransactionList)

		// 获取交易详情
		// GET /api/v1/transaction/:id
		transactionGroup.GET("/:id", h.GetTransactionDetail)

		// 获取待处理交易
		// GET /api/v1/transaction/pending
		transactionGroup.GET("/pending", h.GetPendingTransactions)

		// 执行交易
		// POST /api/v1/transaction/:id/execute
		transactionGroup.POST("/:id/execute", h.ExecuteTransaction)

		// 取消交易
		// POST /api/v1/transaction/:id/cancel
		transactionGroup.POST("/:id/cancel", h.CancelTransaction)

		// 标记交易失败
		// POST /api/v1/transaction/:id/mark-failed
		transactionGroup.POST("/:id/mark-failed", h.MarkTransactionFailed)

		// 标记交易提交失败
		// POST /api/v1/transaction/:id/mark-submit-failed
		transactionGroup.POST("/:id/mark-submit-failed", h.MarkTransactionSubmitFailed)

		// 重试提交交易
		// POST /api/v1/transaction/:id/retry-submit
		transactionGroup.POST("/:id/retry-submit", h.RetrySubmitTransaction)

		// 获取交易统计
		// GET /api/v1/transaction/stats
		transactionGroup.GET("/stats", h.GetTransactionStats)
	}
}

// CreateTransaction 创建交易记录
// @Summary 创建交易记录
// @Description 创建新的timelock交易记录。前端发起交易后，需要提供chainid、timelock合约地址、timelock标准、txhash、txData、eta等信息。系统会验证用户权限（compound的admin，openzeppelin的proposer）后创建记录。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.CreateTransactionRequest true "创建交易的请求体"
// @Success 200 {object} types.APIResponse{data=types.TransactionWithPermission} "成功创建交易记录"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "权限不足"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "交易已存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/create [post]
func (h *Handler) CreateTransaction(c *gin.Context) {
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
		logger.Error("CreateTransaction Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.CreateTransactionRequest
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
		logger.Error("CreateTransaction Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 调用service层
	result, err := h.transactionService.CreateTransaction(c.Request.Context(), userAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case transaction.ErrTransactionExists:
			statusCode = http.StatusConflict
			errorCode = "TRANSACTION_EXISTS"
		case transaction.ErrInvalidTransactionData:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_TRANSACTION_DATA"
		case transaction.ErrInvalidETA:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_ETA"
		case transaction.ErrInsufficientPermissions:
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
		logger.Error("CreateTransaction Error: ", err, "user_address", userAddress, "error_code", errorCode)
		return
	}

	logger.Info("CreateTransaction Success: ", "user_address", userAddress, "tx_hash", req.TxHash)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    result,
	})
}

// GetTransactionList 获取交易列表
// @Summary 获取用户交易列表
// @Description 获取当前用户的交易列表，支持按链ID、timelock地址、标准和状态进行筛选。返回带权限信息的交易列表，包括用户可执行和可取消的操作。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chain_id query int false "按链ID筛选" example(1)
// @Param timelock_address query string false "按timelock地址筛选" example(0x1234567890123456789012345678901234567890)
// @Param timelock_standard query string false "按timelock标准筛选" Enums(compound,openzeppelin) example(compound)
// @Param status query string false "按状态筛选" Enums(queued,ready,executed,expired,canceled) example(queued)
// @Param page query int true "页码" minimum(1) example(1)
// @Param page_size query int true "每页大小" minimum(1) maximum(100) example(20)
// @Success 200 {object} types.APIResponse{data=types.GetTransactionListResponse} "成功获取交易列表"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/list [get]
func (h *Handler) GetTransactionList(c *gin.Context) {
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
		logger.Error("GetTransactionList Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.GetTransactionListRequest
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
		logger.Error("GetTransactionList Error: ", errors.New("invalid query parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 设置默认分页参数
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	// 调用service层
	response, err := h.transactionService.GetTransactionList(c.Request.Context(), userAddress, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetTransactionList Error: ", err, "user_address", userAddress)
		return
	}

	logger.Info("GetTransactionList Success: ", "user_address", userAddress, "total", response.Total, "count", len(response.Transactions))
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetTransactionDetail 获取交易详情
// @Summary 获取交易详细信息
// @Description 获取指定交易的完整详细信息，包括交易数据、状态、权限信息以及关联的timelock合约信息。只有有权限的用户才能查看详细信息。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "交易ID" example(1)
// @Success 200 {object} types.APIResponse{data=types.TransactionDetailResponse} "成功获取交易详情"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "无权访问此交易"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "交易不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/{id} [get]
func (h *Handler) GetTransactionDetail(c *gin.Context) {
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
		logger.Error("GetTransactionDetail Error: ", errors.New("user not authenticated"))
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
				Message: "Invalid transaction ID",
				Details: err.Error(),
			},
		})
		logger.Error("GetTransactionDetail Error: ", errors.New("invalid transaction ID"), "id", idStr, "user_address", userAddress)
		return
	}

	// 调用service层
	response, err := h.transactionService.GetTransactionDetail(c.Request.Context(), userAddress, id)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case transaction.ErrTransactionNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TRANSACTION_NOT_FOUND"
		case transaction.ErrUnauthorized:
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
		logger.Error("GetTransactionDetail Error: ", err, "user_address", userAddress, "transaction_id", id, "error_code", errorCode)
		return
	}

	logger.Info("GetTransactionDetail Success: ", "user_address", userAddress, "transaction_id", id)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetPendingTransactions 获取待处理交易
// @Summary 获取待处理交易列表
// @Description 获取状态为queued、ready、executing或failed的交易列表，可选择只显示当前用户可执行的交易。包含失败后可重试的交易。这是交易执行界面使用的主要接口，按ETA排序显示最紧急的交易。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chain_id query int false "按链ID筛选" example(1)
// @Param only_can_exec query bool false "只显示当前用户可执行的交易" example(false)
// @Param page query int true "页码" minimum(1) example(1)
// @Param page_size query int true "每页大小" minimum(1) maximum(100) example(20)
// @Success 200 {object} types.APIResponse{data=types.GetTransactionListResponse} "成功获取待处理交易列表"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/pending [get]
func (h *Handler) GetPendingTransactions(c *gin.Context) {
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
		logger.Error("GetPendingTransactions Error: ", errors.New("user not authenticated"))
		return
	}

	var req types.GetPendingTransactionsRequest
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
		logger.Error("GetPendingTransactions Error: ", errors.New("invalid query parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 设置默认分页参数
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}

	// 调用service层
	response, err := h.transactionService.GetPendingTransactions(c.Request.Context(), userAddress, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetPendingTransactions Error: ", err, "user_address", userAddress)
		return
	}

	logger.Info("GetPendingTransactions Success: ", "user_address", userAddress, "total", response.Total, "count", len(response.Transactions))
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// ExecuteTransaction 执行交易
// @Summary 执行已就绪的timelock交易
// @Description 执行已就绪的timelock交易或重试失败的交易。支持ready状态（ETA已到达）和failed状态（执行失败后重试）的交易。需要验证用户权限（compound的admin，openzeppelin的executor）。执行成功后更新交易状态。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "交易ID" example(1)
// @Param request body types.ExecuteTransactionRequest true "执行交易的请求体"
// @Success 200 {object} types.APIResponse{data=object} "成功执行交易"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "权限不足"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "交易不存在"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "交易状态不允许执行"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/{id}/execute [post]
func (h *Handler) ExecuteTransaction(c *gin.Context) {
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
		logger.Error("ExecuteTransaction Error: ", errors.New("user not authenticated"))
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
				Message: "Invalid transaction ID",
				Details: err.Error(),
			},
		})
		logger.Error("ExecuteTransaction Error: ", errors.New("invalid transaction ID"), "id", idStr, "user_address", userAddress)
		return
	}

	var req types.ExecuteTransactionRequest
	req.ID = id

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
		logger.Error("ExecuteTransaction Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 调用service层
	err = h.transactionService.ExecuteTransaction(c.Request.Context(), userAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case transaction.ErrTransactionNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TRANSACTION_NOT_FOUND"
		case transaction.ErrTransactionNotReady:
			statusCode = http.StatusConflict
			errorCode = "TRANSACTION_NOT_READY"
		case transaction.ErrInsufficientPermissions:
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
		logger.Error("ExecuteTransaction Error: ", err, "user_address", userAddress, "transaction_id", id, "error_code", errorCode)
		return
	}

	logger.Info("ExecuteTransaction Success: ", "user_address", userAddress, "transaction_id", id, "execute_tx_hash", req.ExecuteTxHash)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data: gin.H{
			"message":         "Transaction executed successfully",
			"transaction_id":  id,
			"execute_tx_hash": req.ExecuteTxHash,
		},
	})
}

// CancelTransaction 取消交易
// @Summary 取消timelock交易
// @Description 取消处于queued或ready状态的timelock交易。需要验证用户权限（compound的admin，openzeppelin的canceller或创建者）。取消成功后更新交易状态。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "交易ID" example(1)
// @Success 200 {object} types.APIResponse{data=object} "成功取消交易"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "权限不足"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "交易不存在"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "交易状态不允许取消"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/{id}/cancel [post]
func (h *Handler) CancelTransaction(c *gin.Context) {
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
		logger.Error("CancelTransaction Error: ", errors.New("user not authenticated"))
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
				Message: "Invalid transaction ID",
				Details: err.Error(),
			},
		})
		logger.Error("CancelTransaction Error: ", errors.New("invalid transaction ID"), "id", idStr, "user_address", userAddress)
		return
	}

	var req types.CancelTransactionRequest
	req.ID = id

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
		logger.Error("CancelTransaction Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 调用service层
	err = h.transactionService.CancelTransaction(c.Request.Context(), userAddress, &req)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case transaction.ErrTransactionNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TRANSACTION_NOT_FOUND"
		case transaction.ErrTransactionNotCancelable:
			statusCode = http.StatusConflict
			errorCode = "TRANSACTION_NOT_CANCELABLE"
		case transaction.ErrInsufficientPermissions:
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
		logger.Error("CancelTransaction Error: ", err, "user_address", userAddress, "transaction_id", id, "error_code", errorCode)
		return
	}

	logger.Info("CancelTransaction Success: ", "user_address", userAddress, "transaction_id", id, "cancel_tx_hash", req.CancelTxHash)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data: gin.H{
			"message":        "Transaction canceled successfully",
			"transaction_id": id,
			"cancel_tx_hash": req.CancelTxHash,
		},
	})
}

// GetTransactionStats 获取交易统计
// @Summary 获取交易统计信息
// @Description 获取当前用户的交易统计信息，包括总交易数和各状态的交易数量。用于仪表板显示。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} types.APIResponse{data=types.TransactionStatsResponse} "成功获取交易统计"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/stats [get]
func (h *Handler) GetTransactionStats(c *gin.Context) {
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
		logger.Error("GetTransactionStats Error: ", errors.New("user not authenticated"))
		return
	}

	// 调用service层
	stats, err := h.transactionService.GetTransactionStats(c.Request.Context(), userAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		logger.Error("GetTransactionStats Error: ", err, "user_address", userAddress)
		return
	}

	logger.Info("GetTransactionStats Success: ", "user_address", userAddress, "total", stats.TotalTransactions)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    stats,
	})
}

// MarkTransactionFailed 标记交易失败
// @Summary 标记交易执行失败
// @Description 将执行中的交易标记为失败状态。这个接口通常由前端在区块链执行失败时调用，或者由后端监控程序调用。只能标记状态为executing的交易。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "交易ID" example(1)
// @Param request body types.MarkTransactionFailedRequest true "标记失败的请求体"
// @Success 200 {object} types.APIResponse{data=object} "成功标记交易失败"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "交易不存在"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "交易状态不允许标记失败"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/{id}/mark-failed [post]
func (h *Handler) MarkTransactionFailed(c *gin.Context) {
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
		logger.Error("MarkTransactionFailed Error: ", errors.New("user not authenticated"))
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
				Message: "Invalid transaction ID",
				Details: err.Error(),
			},
		})
		logger.Error("MarkTransactionFailed Error: ", errors.New("invalid transaction ID"), "id", idStr, "user_address", userAddress)
		return
	}

	var req types.MarkTransactionFailedRequest
	req.ID = id

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
		logger.Error("MarkTransactionFailed Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 调用service层
	err = h.transactionService.MarkTransactionFailed(c.Request.Context(), id, req.Reason)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case transaction.ErrTransactionNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TRANSACTION_NOT_FOUND"
		default:
			if err.Error() == "transaction is not in executing state" {
				statusCode = http.StatusConflict
				errorCode = "TRANSACTION_NOT_EXECUTING"
			} else {
				statusCode = http.StatusInternalServerError
				errorCode = "INTERNAL_ERROR"
			}
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("MarkTransactionFailed Error: ", err, "user_address", userAddress, "transaction_id", id, "error_code", errorCode)
		return
	}

	logger.Info("MarkTransactionFailed Success: ", "user_address", userAddress, "transaction_id", id, "reason", req.Reason)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data: gin.H{
			"message":        "Transaction marked as failed successfully",
			"transaction_id": id,
			"reason":         req.Reason,
		},
	})
}

// MarkTransactionSubmitFailed 标记交易提交失败
// @Summary 标记交易提交失败
// @Description 将正在提交的交易标记为提交失败状态。这个接口通常由前端在提交到timelock合约失败时调用。只能标记状态为submitting的交易。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "交易ID" example(1)
// @Param request body types.MarkTransactionSubmitFailedRequest true "标记提交失败的请求体"
// @Success 200 {object} types.APIResponse{data=object} "成功标记交易提交失败"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "交易不存在"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "交易状态不允许标记提交失败"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/{id}/mark-submit-failed [post]
func (h *Handler) MarkTransactionSubmitFailed(c *gin.Context) {
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
		logger.Error("MarkTransactionSubmitFailed Error: ", errors.New("user not authenticated"))
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
				Message: "Invalid transaction ID",
				Details: err.Error(),
			},
		})
		logger.Error("MarkTransactionSubmitFailed Error: ", errors.New("invalid transaction ID"), "id", idStr, "user_address", userAddress)
		return
	}

	var req types.MarkTransactionSubmitFailedRequest
	req.ID = id

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
		logger.Error("MarkTransactionSubmitFailed Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 调用service层
	err = h.transactionService.MarkTransactionSubmitFailed(c.Request.Context(), id, req.Reason)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case transaction.ErrTransactionNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TRANSACTION_NOT_FOUND"
		default:
			if err.Error() == "transaction is not in submitting state" {
				statusCode = http.StatusConflict
				errorCode = "TRANSACTION_NOT_SUBMITTING"
			} else {
				statusCode = http.StatusInternalServerError
				errorCode = "INTERNAL_ERROR"
			}
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("MarkTransactionSubmitFailed Error: ", err, "user_address", userAddress, "transaction_id", id, "error_code", errorCode)
		return
	}

	logger.Info("MarkTransactionSubmitFailed Success: ", "user_address", userAddress, "transaction_id", id, "reason", req.Reason)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data: gin.H{
			"message":        "Transaction marked as submit failed successfully",
			"transaction_id": id,
			"reason":         req.Reason,
		},
	})
}

// RetrySubmitTransaction 重试提交交易
// @Summary 重试提交交易
// @Description 重试提交失败的交易。用于submit_failed状态的交易，允许用户使用新的交易哈希重新提交。需要验证用户权限（创建者或有提议权限的用户）。
// @Tags Transaction
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "交易ID" example(1)
// @Param request body types.RetrySubmitTransactionRequest true "重试提交的请求体"
// @Success 200 {object} types.APIResponse{data=object} "成功重试提交交易"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "权限不足"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "交易不存在"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "交易状态不允许重试或交易哈希已存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/transaction/{id}/retry-submit [post]
func (h *Handler) RetrySubmitTransaction(c *gin.Context) {
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
		logger.Error("RetrySubmitTransaction Error: ", errors.New("user not authenticated"))
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
				Message: "Invalid transaction ID",
				Details: err.Error(),
			},
		})
		logger.Error("RetrySubmitTransaction Error: ", errors.New("invalid transaction ID"), "id", idStr, "user_address", userAddress)
		return
	}

	var req types.RetrySubmitTransactionRequest
	req.ID = id

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
		logger.Error("RetrySubmitTransaction Error: ", errors.New("invalid request parameters"), "error", err, "user_address", userAddress)
		return
	}

	// 调用service层
	err = h.transactionService.RetrySubmitTransaction(c.Request.Context(), userAddress, id, req.TxHash)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err {
		case transaction.ErrTransactionNotFound:
			statusCode = http.StatusNotFound
			errorCode = "TRANSACTION_NOT_FOUND"
		case transaction.ErrTransactionExists:
			statusCode = http.StatusConflict
			errorCode = "TRANSACTION_EXISTS"
		case transaction.ErrInvalidTransactionData:
			statusCode = http.StatusBadRequest
			errorCode = "INVALID_TRANSACTION_DATA"
		case transaction.ErrInsufficientPermissions:
			statusCode = http.StatusForbidden
			errorCode = "INSUFFICIENT_PERMISSIONS"
		default:
			if err.Error() == "transaction is not in submit_failed state" {
				statusCode = http.StatusConflict
				errorCode = "TRANSACTION_NOT_SUBMIT_FAILED"
			} else {
				statusCode = http.StatusInternalServerError
				errorCode = "INTERNAL_ERROR"
			}
		}

		c.JSON(statusCode, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    errorCode,
				Message: err.Error(),
			},
		})
		logger.Error("RetrySubmitTransaction Error: ", err, "user_address", userAddress, "transaction_id", id, "error_code", errorCode)
		return
	}

	logger.Info("RetrySubmitTransaction Success: ", "user_address", userAddress, "transaction_id", id, "new_tx_hash", req.TxHash)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data: gin.H{
			"message":        "Transaction retry submitted successfully",
			"transaction_id": id,
			"new_tx_hash":    req.TxHash,
		},
	})
}
