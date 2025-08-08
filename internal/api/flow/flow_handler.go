package flow

import (
	"net/http"
	"strconv"

	"timelocker-backend/internal/service/flow"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// FlowHandler 流程处理器
type FlowHandler struct {
	flowService flow.FlowService
}

// NewFlowHandler 创建新的流程处理器
func NewFlowHandler(flowService flow.FlowService) *FlowHandler {
	return &FlowHandler{
		flowService: flowService,
	}
}

// RegisterRoutes 注册路由
func (h *FlowHandler) RegisterRoutes(router *gin.RouterGroup) {
	flows := router.Group("/flows")
	{
		// 获取等待中的流程
		// http://localhost:8080/api/v1/flows/waiting?page=1&page_size=20
		flows.GET("/waiting", h.GetWaitingFlows)
		// 获取准备执行的流程
		// http://localhost:8080/api/v1/flows/ready?page=1&page_size=20
		flows.GET("/ready", h.GetReadyFlows)
		// 获取已执行的流程
		// http://localhost:8080/api/v1/flows/executed?page=1&page_size=20
		flows.GET("/executed", h.GetExecutedFlows)
		// 获取已取消的流程
		// http://localhost:8080/api/v1/flows/cancelled?page=1&page_size=20
		flows.GET("/cancelled", h.GetCancelledFlows)
		// 获取已过期的流程
		// http://localhost:8080/api/v1/flows/expired?page=1&page_size=20
		flows.GET("/expired", h.GetExpiredFlows)
		// 获取用户的流程列表
		// http://localhost:8080/api/v1/flows/user?initiator_address=0x7148C25A8C78b841f771b2b2eeaD6A6220718390&status=waiting&page=1&page_size=20
		flows.GET("/user", h.GetUserFlows)
		// 获取流程详细信息
		// http://localhost:8080/api/v1/flows/1/detail?timelock_standard=compound&chain_id=1&contract_address=0x7148C25A8C78b841f771b2b2eeaD6A6220718390
		flows.GET("/:flow_id/detail", h.GetFlowDetail)
		// 获取流程统计信息
		// http://localhost:8080/api/v1/flows/stats?initiator_address=0x7148C25A8C78b841f771b2b2eeaD6A6220718390
		flows.GET("/stats", h.GetFlowStats)
	}
}

// GetWaitingFlows 获取等待中的流程
// @Summary 获取等待中的流程
// @Description 获取等待中的timelock流程列表
// @Tags Flow
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} types.FlowListResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/flows/waiting [get]
func (h *FlowHandler) GetWaitingFlows(c *gin.Context) {
	page, pageSize := h.getPageParams(c)

	flows, err := h.flowService.GetWaitingFlows(c.Request.Context(), page, pageSize)
	if err != nil {
		logger.Error("Failed to get waiting flows", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "Failed to get waiting flows",
		})
		return
	}

	c.JSON(http.StatusOK, flows)
}

// GetReadyFlows 获取准备执行的流程
// @Summary 获取准备执行的流程
// @Description 获取准备执行的timelock流程列表
// @Tags Flow
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} types.FlowListResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/flows/ready [get]
func (h *FlowHandler) GetReadyFlows(c *gin.Context) {
	page, pageSize := h.getPageParams(c)

	flows, err := h.flowService.GetReadyFlows(c.Request.Context(), page, pageSize)
	if err != nil {
		logger.Error("Failed to get ready flows", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "Failed to get ready flows",
		})
		return
	}

	c.JSON(http.StatusOK, flows)
}

// GetExecutedFlows 获取已执行的流程
// @Summary 获取已执行的流程
// @Description 获取已执行的timelock流程列表
// @Tags Flow
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} types.FlowListResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/flows/executed [get]
func (h *FlowHandler) GetExecutedFlows(c *gin.Context) {
	page, pageSize := h.getPageParams(c)

	flows, err := h.flowService.GetExecutedFlows(c.Request.Context(), page, pageSize)
	if err != nil {
		logger.Error("Failed to get executed flows", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "Failed to get executed flows",
		})
		return
	}

	c.JSON(http.StatusOK, flows)
}

// GetCancelledFlows 获取已取消的流程
// @Summary 获取已取消的流程
// @Description 获取已取消的timelock流程列表
// @Tags Flow
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} types.FlowListResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/flows/cancelled [get]
func (h *FlowHandler) GetCancelledFlows(c *gin.Context) {
	page, pageSize := h.getPageParams(c)

	flows, err := h.flowService.GetCancelledFlows(c.Request.Context(), page, pageSize)
	if err != nil {
		logger.Error("Failed to get cancelled flows", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "Failed to get cancelled flows",
		})
		return
	}

	c.JSON(http.StatusOK, flows)
}

// GetExpiredFlows 获取已过期的流程
// @Summary 获取已过期的流程
// @Description 获取已过期的timelock流程列表（仅Compound）
// @Tags Flow
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} types.FlowListResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/flows/expired [get]
func (h *FlowHandler) GetExpiredFlows(c *gin.Context) {
	page, pageSize := h.getPageParams(c)

	flows, err := h.flowService.GetExpiredFlows(c.Request.Context(), page, pageSize)
	if err != nil {
		logger.Error("Failed to get expired flows", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "Failed to get expired flows",
		})
		return
	}

	c.JSON(http.StatusOK, flows)
}

// GetUserFlows 获取用户的流程列表
// @Summary 获取用户的流程列表
// @Description 根据发起人地址和状态获取流程列表
// @Tags Flow
// @Accept json
// @Produce json
// @Param initiator_address query string true "发起人地址"
// @Param status query string true "流程状态" Enums(waiting,ready,executed,cancelled,expired)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} types.FlowListResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/flows/user [get]
func (h *FlowHandler) GetUserFlows(c *gin.Context) {
	initiatorAddress := c.Query("initiator_address")
	status := c.Query("status")

	if initiatorAddress == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Code:    "INVALID_PARAMS",
			Message: "initiator_address is required",
		})
		return
	}

	if status == "" {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Code:    "INVALID_PARAMS",
			Message: "status is required",
		})
		return
	}

	page, pageSize := h.getPageParams(c)

	flows, err := h.flowService.GetUserFlows(c.Request.Context(), initiatorAddress, status, page, pageSize)
	if err != nil {
		logger.Error("Failed to get user flows", err, "initiator", initiatorAddress, "status", status)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "Failed to get user flows",
		})
		return
	}

	c.JSON(http.StatusOK, flows)
}

// GetFlowDetail 获取流程详细信息
// @Summary 获取流程详细信息
// @Description 获取特定流程的详细信息，包括时间计算
// @Tags Flow
// @Accept json
// @Produce json
// @Param flow_id path string true "流程ID"
// @Param timelock_standard query string true "Timelock标准" Enums(compound,openzeppelin)
// @Param chain_id query int true "链ID"
// @Param contract_address query string true "合约地址"
// @Success 200 {object} types.FlowDetailResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 404 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/flows/{flow_id}/detail [get]
func (h *FlowHandler) GetFlowDetail(c *gin.Context) {
	var req types.FlowDetailRequest
	if err := c.ShouldBindUri(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Code:    "INVALID_PARAMS",
			Message: "Invalid flow_id",
		})
		return
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.ErrorResponse{
			Code:    "INVALID_PARAMS",
			Message: "Invalid query parameters",
		})
		return
	}

	detail, err := h.flowService.GetFlowDetail(c.Request.Context(), req.FlowID, req.TimelockStandard, req.ChainID, req.ContractAddress)
	if err != nil {
		if err.Error() == "flow not found" {
			c.JSON(http.StatusNotFound, types.ErrorResponse{
				Code:    "FLOW_NOT_FOUND",
				Message: "Flow not found",
			})
			return
		}

		logger.Error("Failed to get flow detail", err, "flow_id", req.FlowID)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "Failed to get flow detail",
		})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// GetFlowStats 获取流程统计信息
// @Summary 获取流程统计信息
// @Description 获取流程的统计信息，可按发起人过滤
// @Tags Flow
// @Accept json
// @Produce json
// @Param initiator_address query string false "发起人地址（可选）"
// @Success 200 {object} types.FlowStatsResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/flows/stats [get]
func (h *FlowHandler) GetFlowStats(c *gin.Context) {
	initiatorAddress := c.Query("initiator_address")
	var initiatorPtr *string
	if initiatorAddress != "" {
		initiatorPtr = &initiatorAddress
	}

	stats, err := h.flowService.GetFlowStats(c.Request.Context(), initiatorPtr)
	if err != nil {
		logger.Error("Failed to get flow stats", err)
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "Failed to get flow stats",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// getPageParams 获取分页参数
func (h *FlowHandler) getPageParams(c *gin.Context) (int, int) {
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

	return page, pageSize
}
