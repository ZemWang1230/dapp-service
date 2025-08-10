package flow

import (
	"net/http"
	"strings"

	"timelocker-backend/internal/middleware"
	"timelocker-backend/internal/service/auth"
	"timelocker-backend/internal/service/flow"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// FlowHandler 流程处理器
type FlowHandler struct {
	flowService flow.FlowService
	authService auth.Service
}

// NewFlowHandler 创建新的流程处理器
func NewFlowHandler(flowService flow.FlowService, authService auth.Service) *FlowHandler {
	return &FlowHandler{
		flowService: flowService,
		authService: authService,
	}
}

// RegisterRoutes 注册路由
func (h *FlowHandler) RegisterRoutes(router *gin.RouterGroup) {
	flows := router.Group("/flows")
	{
		// 获取与用户相关的流程列表（需要鉴权）
		// GET /api/v1/flows/list
		// http://localhost:8080/api/v1/flows/list?status=all&standard=openzeppelin
		flows.GET("/list", middleware.AuthMiddleware(h.authService), h.GetFlowList)
		// 获取交易详情
		// GET /api/v1/flows/transaction/detail
		// http://localhost:8080/api/v1/flows/transaction/detail?standard=openzeppelin&tx_hash=0x...
		flows.GET("/transaction/detail", h.GetTransactionDetail)
	}
}

// GetFlowList 获取与用户相关的流程列表
// @Summary 获取与用户相关的流程列表
// @Description 获取与用户相关的timelock流程列表，包括发起的和有权限管理的
// @Tags Flow
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param status query string false "流程状态" Enums(all,waiting,ready,executed,cancelled,expired)
// @Param standard query string false "Timelock标准" Enums(compound,openzeppelin)
// @Param page query int false "页码，默认1" default(1)
// @Param page_size query int false "每页大小，默认10，最大100" default(10)
// @Success 200 {object} types.APIResponse{data=types.GetFlowListResponse}
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未认证或令牌无效"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/flows/list [get]
func (h *FlowHandler) GetFlowList(c *gin.Context) {
	// 从鉴权中间件获取用户地址
	_, userAddressStr, ok := middleware.GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "UNAUTHORIZED",
				Message: "User address not found in token",
			},
		})
		return
	}

	// 解析请求参数
	var req types.GetFlowListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_PARAMS",
				Message: "Invalid query parameters",
				Details: err.Error(),
			},
		})
		return
	}

	// 调用服务层
	response, err := h.flowService.GetFlowList(c.Request.Context(), userAddressStr, &req)
	if err != nil {
		logger.Error("Failed to get flow list", err, "user", userAddressStr)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to get flow list",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetTransactionDetail 获取交易详情
// @Summary 获取交易详情
// @Description 根据交易哈希和标准获取交易详情。standard 仅支持 compound/openzeppelin；tx_hash 必须为 0x 开头的64位十六进制。
// @Tags Flow
// @Accept json
// @Produce json
// @Param standard query string true "Timelock标准" Enums(compound,openzeppelin)
// @Param tx_hash query string true "交易哈希 (0x + 64位十六进制)"
// @Success 200 {object} types.APIResponse{data=types.GetTransactionDetailResponse}
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误（INVALID_STANDARD / INVALID_TX_HASH）"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "交易不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/flows/transaction/detail [get]
func (h *FlowHandler) GetTransactionDetail(c *gin.Context) {
	// 解析请求参数
	var req types.GetTransactionDetailRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_PARAMS",
				Message: "Invalid query parameters",
				Details: err.Error(),
			},
		})
		return
	}

	// 标准化
	req.Standard = strings.ToLower(strings.TrimSpace(req.Standard))
	req.TxHash = strings.TrimSpace(req.TxHash)
	// 校验标准
	if req.Standard != "compound" && req.Standard != "openzeppelin" {
		c.JSON(http.StatusBadRequest, types.APIResponse{Success: false, Error: &types.APIError{Code: "INVALID_STANDARD", Message: "Invalid timelock standard"}})
		return
	}
	// 校验交易哈希格式
	if !utils.IsValidTxHash(req.TxHash) {
		c.JSON(http.StatusBadRequest, types.APIResponse{Success: false, Error: &types.APIError{Code: "INVALID_TX_HASH", Message: "Invalid tx hash format"}})
		return
	}

	// 调用服务层
	response, err := h.flowService.GetTransactionDetail(c.Request.Context(), &req)
	if err != nil {
		if err.Error() == "transaction not found" {
			c.JSON(http.StatusNotFound, types.APIResponse{
				Success: false,
				Error: &types.APIError{
					Code:    "TRANSACTION_NOT_FOUND",
					Message: "Transaction not found",
				},
			})
			return
		}

		logger.Error("Failed to get transaction detail", err, "standard", req.Standard, "tx_hash", req.TxHash)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to get transaction detail",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    response,
	})
}
