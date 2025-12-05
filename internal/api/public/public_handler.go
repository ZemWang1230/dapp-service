package public

import (
	"net/http"
	"timelocker-backend/internal/service/public"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Handler 公共数据处理器
type Handler struct {
	publicService public.Service
}

// NewHandler 创建新的公共数据处理器
func NewHandler(publicService public.Service) *Handler {
	return &Handler{
		publicService: publicService,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// 公共API组（不需要认证）
	publicGroup := router.Group("/public")
	{
		// 获取统计数据
		// POST /api/v1/public/stats
		// http://localhost:8080/api/v1/public/stats
		publicGroup.POST("/stats", h.GetStats)
	}
}

// GetStats 获取统计数据
// @Summary 获取首页统计数据
// @Description 获取支持的链数量、timelock合约数量和交易数量等统计信息，无需认证
// @Tags Public
// @Accept json
// @Produce json
// @Param request body types.GetStatsRequest false "统计数据请求参数"
// @Success 200 {object} types.APIResponse{data=types.GetStatsResponse} "成功获取统计数据"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/public/stats [post]
func (h *Handler) GetStats(c *gin.Context) {
	var req types.GetStatsRequest
	// 绑定参数（支持 body 优先，兼容 query）
	if err := c.ShouldBindQuery(&req); err != nil {
		// ignore, will try JSON
	}
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		logger.Error("GetStats BindQuery Error: ", err)
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_REQUEST",
				Message: "Invalid query parameters",
				Details: err.Error(),
			},
		})
		return
	}

	// 调用服务
	response, err := h.publicService.GetStats(c.Request.Context(), &req)
	if err != nil {
		logger.Error("GetStats Service Error: ", err)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to get stats",
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
