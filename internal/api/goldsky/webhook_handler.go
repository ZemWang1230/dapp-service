package goldsky

import (
	"net/http"

	chainRepo "timelocker-backend/internal/repository/chain"
	"timelocker-backend/internal/service/goldsky"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// WebhookHandler Goldsky Webhook HTTP 处理器
type WebhookHandler struct {
	processor *goldsky.WebhookProcessor
	chainRepo chainRepo.Repository
}

// NewWebhookHandler 创建 Webhook Handler
func NewWebhookHandler(processor *goldsky.WebhookProcessor, chainRepo chainRepo.Repository) *WebhookHandler {
	return &WebhookHandler{
		processor: processor,
		chainRepo: chainRepo,
	}
}

// RegisterRoutes 注册路由
func (h *WebhookHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/goldsky/webhook", h.HandleWebhook)
}

// HandleWebhook 处理 Goldsky Webhook 请求
// @Summary Goldsky Webhook 接收端点
// @Description 接收 Goldsky 推送的 Timelock 交易事件
// @Tags Goldsky
// @Accept json
// @Produce json
// @Param goldsky-webhook-secret header string true "Webhook Secret"
// @Param payload body types.GoldskyWebhookPayload true "Webhook Payload"
// @Success 200 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse "Invalid webhook secret"
// @Failure 400 {object} types.APIResponse "Invalid payload"
// @Failure 500 {object} types.APIResponse "Internal error"
// @Router /api/v1/goldsky/webhook [post]
func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	// 1. 获取 Webhook Secret
	secret := c.GetHeader("goldsky-webhook-secret")
	if secret == "" {
		logger.Warn("Webhook request missing secret header")
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "MISSING_SECRET",
				Message: "Missing goldsky-webhook-secret header",
			},
		})
		return
	}

	// 2. 通过 secret 查找对应的链和类型（compound/openzeppelin）
	chain, standard, err := h.chainRepo.GetChainByWebhookSecret(c.Request.Context(), secret)
	if err != nil {
		logger.Warn("Invalid webhook secret", "secret", secret, "error", err)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_SECRET",
				Message: "Invalid webhook secret",
			},
		})
		return
	}

	// 3. 解析 Payload
	var payload types.GoldskyWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Error("Failed to parse webhook payload", err)
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_PAYLOAD",
				Message: "Invalid webhook payload",
				Details: err.Error(),
			},
		})
		return
	}

	chainID := int(chain.ChainID)

	// 4. 根据标准类型处理对应的交易
	if standard == "compound" {
		// 处理 Compound Transactions
		txCount := len(payload.Event.Data.CompoundTimelockTransactions)
		logger.Info("Processing Compound webhook", "chain_id", chainID, "tx_count", txCount)

		for _, tx := range payload.Event.Data.CompoundTimelockTransactions {
			// 异步处理，不阻塞 Webhook 响应
			go func(transaction types.GoldskyCompoundTransactionWebhook, cid int) {
				ctx := c.Request.Context()
				if err := h.processor.ProcessCompoundTransaction(ctx, transaction, cid); err != nil {
					logger.Error("Failed to process Compound transaction",
						err,
						"chain_id", cid,
						"tx_hash", transaction.TxHash,
						"event_type", transaction.EventType)
				}
			}(tx, chainID)
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data: gin.H{
				"message":  "Webhook received and processing",
				"chain_id": chainID,
				"standard": "compound",
				"tx_count": txCount,
			},
		})
	} else if standard == "openzeppelin" {
		// 处理 OpenZeppelin Transactions
		txCount := len(payload.Event.Data.OpenzeppelinTimelockTransactions)
		logger.Info("Processing OpenZeppelin webhook", "chain_id", chainID, "tx_count", txCount)

		for _, tx := range payload.Event.Data.OpenzeppelinTimelockTransactions {
			go func(transaction types.GoldskyOpenzeppelinTransactionWebhook, cid int) {
				ctx := c.Request.Context()
				if err := h.processor.ProcessOpenzeppelinTransaction(ctx, transaction, cid); err != nil {
					logger.Error("Failed to process OpenZeppelin transaction",
						err,
						"chain_id", cid,
						"tx_hash", transaction.TxHash,
						"event_type", transaction.EventType)
				}
			}(tx, chainID)
		}

		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data: gin.H{
				"message":  "Webhook received and processing",
				"chain_id": chainID,
				"standard": "openzeppelin",
				"tx_count": txCount,
			},
		})
	} else {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_STANDARD",
				Message: "Invalid timelock standard",
			},
		})
	}
}
