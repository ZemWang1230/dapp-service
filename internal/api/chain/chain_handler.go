package chain

import (
	"net/http"
	"strconv"

	"timelocker-backend/internal/service/chain"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Handler 支持链处理器
type Handler struct {
	chainService chain.Service
}

// NewHandler 创建新的支持链处理器
func NewHandler(chainService chain.Service) *Handler {
	return &Handler{
		chainService: chainService,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	// 支持链API组
	chainGroup := router.Group("/chain")
	{
		// 获取支持链列表
		// GET /api/v1/chain/list
		// http://localhost:8080/api/v1/chain/list?is_testnet=false&is_active=true
		chainGroup.GET("/list", h.GetSupportChains)

		// 根据ID获取链信息
		// GET /api/v1/chain/:id
		// http://localhost:8080/api/v1/chain/1
		chainGroup.GET("/:id", h.GetChainByID)

		// 根据ChainID获取链信息
		// GET /api/v1/chain/chainid/:chain_id
		// http://localhost:8080/api/v1/chain/chainid/1
		chainGroup.GET("/chainid/:chain_id", h.GetChainByChainID)
	}
}

// GetSupportChains 获取支持链列表
// @Summary 获取支持链列表
// @Description 获取所有支持的区块链列表，可根据是否测试网和是否激活进行筛选
// @Tags chain
// @Accept json
// @Produce json
// @Param is_testnet query bool false "是否测试网"
// @Param is_active query bool false "是否激活"
// @Success 200 {object} map[string]interface{} "{"code":200,"message":"success","data":{"chains":[...],"total":10}}"
// @Failure 400 {object} map[string]interface{} "{"code":400,"message":"参数错误","data":null}"
// @Failure 500 {object} map[string]interface{} "{"code":500,"message":"服务器内部错误","data":null}"
// @Router /api/v1/chain/list [get]
func (h *Handler) GetSupportChains(c *gin.Context) {
	var req types.GetSupportChainsRequest

	// 绑定查询参数
	if err := c.ShouldBindQuery(&req); err != nil {
		logger.Error("GetSupportChains BindQuery Error: ", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	// 调用服务
	response, err := h.chainService.GetSupportChains(c.Request.Context(), &req)
	if err != nil {
		logger.Error("GetSupportChains Service Error: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "服务器内部错误",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    response,
	})
}

// GetChainByID 根据ID获取链信息
// @Summary 根据ID获取链信息
// @Description 根据链的ID获取具体的链信息
// @Tags chain
// @Accept json
// @Produce json
// @Param id path int true "链ID"
// @Success 200 {object} map[string]interface{} "{"code":200,"message":"success","data":{...}}"
// @Failure 400 {object} map[string]interface{} "{"code":400,"message":"参数错误","data":null}"
// @Failure 404 {object} map[string]interface{} "{"code":404,"message":"链信息不存在","data":null}"
// @Failure 500 {object} map[string]interface{} "{"code":500,"message":"服务器内部错误","data":null}"
// @Router /api/v1/chain/{id} [get]
func (h *Handler) GetChainByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.Error("GetChainByID ParseInt Error: ", err, "id", idStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	req := &types.GetChainByIDRequest{
		ID: id,
	}

	// 调用服务
	chain, err := h.chainService.GetChainByID(c.Request.Context(), req)
	if err != nil {
		logger.Error("GetChainByID Service Error: ", err, "id", id)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "服务器内部错误",
			"data":    nil,
		})
		return
	}

	if chain == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "链信息不存在",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    chain,
	})
}

// GetChainByChainID 根据ChainID获取链信息
// @Summary 根据ChainID获取链信息
// @Description 根据区块链的ChainID获取具体的链信息
// @Tags chain
// @Accept json
// @Produce json
// @Param chain_id path int true "区块链ID"
// @Success 200 {object} map[string]interface{} "{"code":200,"message":"success","data":{...}}"
// @Failure 400 {object} map[string]interface{} "{"code":400,"message":"参数错误","data":null}"
// @Failure 404 {object} map[string]interface{} "{"code":404,"message":"链信息不存在","data":null}"
// @Failure 500 {object} map[string]interface{} "{"code":500,"message":"服务器内部错误","data":null}"
// @Router /api/v1/chain/chainid/{chain_id} [get]
func (h *Handler) GetChainByChainID(c *gin.Context) {
	chainIDStr := c.Param("chain_id")
	chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
	if err != nil {
		logger.Error("GetChainByChainID ParseInt Error: ", err, "chain_id", chainIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	req := &types.GetChainByChainIDRequest{
		ChainID: chainID,
	}

	// 调用服务
	chain, err := h.chainService.GetChainByChainID(c.Request.Context(), req)
	if err != nil {
		logger.Error("GetChainByChainID Service Error: ", err, "chain_id", chainID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "服务器内部错误",
			"data":    nil,
		})
		return
	}

	if chain == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "链信息不存在",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    chain,
	})
}
