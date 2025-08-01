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

		// 获取钱包插件添加链的配置数据
		// GET /api/v1/chain/wallet-config/:chain_id
		// http://localhost:8080/api/v1/chain/wallet-config/1
		chainGroup.GET("/wallet-config/:chain_id", h.GetWalletChainConfig)
	}
}

// GetSupportChains 获取支持链列表
// @Summary 获取支持的区块链列表
// @Description 获取所有支持的区块链列表，可根据是否测试网和是否激活状态进行筛选。返回链的详细信息包括名称、链ID、原生代币、Logo等信息。
// @Tags Chain
// @Accept json
// @Produce json
// @Param is_testnet query bool false "是否筛选测试网" example(false)
// @Param is_active query bool false "是否筛选激活状态" example(true)
// @Success 200 {object} types.APIResponse{data=types.GetSupportChainsResponse} "成功获取支持链列表"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/chain/list [get]
func (h *Handler) GetSupportChains(c *gin.Context) {
	var req types.GetSupportChainsRequest

	// 绑定查询参数
	if err := c.ShouldBindQuery(&req); err != nil {
		logger.Error("GetSupportChains BindQuery Error: ", err)
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
	response, err := h.chainService.GetSupportChains(c.Request.Context(), &req)
	if err != nil {
		logger.Error("GetSupportChains Service Error: ", err)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to get support chains",
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

// GetChainByID 根据ID获取链信息
// @Summary 根据ID获取链信息
// @Description 根据指定的ID获取单个支持链的详细信息，包括链名称、链ID、原生代币、Logo等基本信息。
// @Tags Chain
// @Accept json
// @Produce json
// @Param id path int true "链的数据库ID" example(1)
// @Success 200 {object} types.APIResponse{data=types.SupportChain} "成功获取链信息"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "链不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/chain/{id} [get]
func (h *Handler) GetChainByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_ID",
				Message: "Invalid ID format",
				Details: err.Error(),
			},
		})
		logger.Error("GetChainByID Error: ", err, "id", idStr)
		return
	}

	// 调用服务层
	chain, err := h.chainService.GetChainByID(c.Request.Context(), id)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err.Error() {
		case "chain not found":
			statusCode = http.StatusNotFound
			errorCode = "CHAIN_NOT_FOUND"
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
		logger.Error("GetChainByID Error: ", err, "id", id)
		return
	}

	logger.Info("GetChainByID: ", "id", id, "chain_name", chain.ChainName)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    chain,
	})
}

// GetChainByChainID 根据ChainID获取链信息
// @Summary 根据ChainID获取链信息
// @Description 根据指定的链ID（如1代表以太坊主网）获取单个支持链的详细信息，包括链名称、原生代币、Logo等基本信息。
// @Tags Chain
// @Accept json
// @Produce json
// @Param chain_id path int true "链ID（区块链网络ID）" example(1)
// @Success 200 {object} types.APIResponse{data=types.SupportChain} "成功获取链信息"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "链不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/chain/chainid/{chain_id} [get]
func (h *Handler) GetChainByChainID(c *gin.Context) {
	chainIDStr := c.Param("chain_id")
	chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_CHAIN_ID",
				Message: "Invalid chain ID format",
				Details: err.Error(),
			},
		})
		logger.Error("GetChainByChainID Error: ", err, "chain_id", chainIDStr)
		return
	}

	// 调用服务层
	chain, err := h.chainService.GetChainByChainID(c.Request.Context(), chainID)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err.Error() {
		case "chain not found":
			statusCode = http.StatusNotFound
			errorCode = "CHAIN_NOT_FOUND"
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
		logger.Error("GetChainByChainID Error: ", err, "chain_id", chainID)
		return
	}

	logger.Info("GetChainByChainID: ", "chain_id", chainID, "chain_name", chain.ChainName)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    chain,
	})
}

// GetWalletChainConfig 获取钱包插件添加链的配置数据
// @Summary 获取钱包插件添加链的配置数据
// @Description 获取指定链ID的钱包插件配置数据，包括chainId、chainName、nativeCurrency、rpcUrls、blockExplorerUrls等信息，用于帮助用户在钱包插件中添加该链。
// @Tags Chain
// @Accept json
// @Produce json
// @Param chain_id path int true "链ID" example(137)
// @Success 200 {object} types.APIResponse{data=types.WalletChainConfig} "成功获取钱包配置数据"
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "链不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/chain/wallet-config/{chain_id} [get]
func (h *Handler) GetWalletChainConfig(c *gin.Context) {
	chainIDStr := c.Param("chain_id")
	chainID := int64(0)

	// 转换链ID
	if parsed, err := strconv.ParseInt(chainIDStr, 10, 64); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_CHAIN_ID",
				Message: "Invalid chain ID format",
				Details: err.Error(),
			},
		})
		logger.Error("GetWalletChainConfig Error: ", err, "chain_id", chainIDStr)
		return
	} else {
		chainID = parsed
	}

	// 调用服务层
	config, err := h.chainService.GetWalletChainConfig(c.Request.Context(), chainID)
	if err != nil {
		var statusCode int
		var errorCode string

		switch err.Error() {
		case "chain not found":
			statusCode = http.StatusNotFound
			errorCode = "CHAIN_NOT_FOUND"
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
		logger.Error("GetWalletChainConfig Error: ", err, "chain_id", chainID)
		return
	}

	logger.Info("GetWalletChainConfig: ", "chain_id", chainID, "chain_name", config.ChainName)
	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    config,
	})
}
