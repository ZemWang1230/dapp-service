package goldsky

import (
	"context"
	"fmt"
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
// @Param payload body types.GraphQLWebhookPayload true "Webhook Payload"
// @Success 200 {object} types.APIResponse
// @Failure 401 {object} types.APIResponse "Invalid webhook"
// @Failure 400 {object} types.APIResponse "Invalid payload"
// @Failure 500 {object} types.APIResponse "Internal error"
// @Router /api/v1/goldsky/webhook [post]
func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	// 1. 解析 Payload
	var payload types.GraphQLWebhookPayload
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

	// 2. 通过 webhook_id 查找对应的链和类型
	chain, standard, err := h.chainRepo.GetChainByWebhookSecret(c.Request.Context(), payload.WebhookID)
	if err != nil {
		logger.Warn("Invalid webhook_id", "webhook_id", payload.WebhookID, "error", err)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_WEBHOOK",
				Message: "Invalid webhook_id",
			},
		})
		return
	}

	// 3. 检查是否有新数据
	if payload.Data.New == nil {
		logger.Info("No new data in webhook payload", "webhook_id", payload.WebhookID)
		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data: gin.H{
				"message":    "No new data to process",
				"webhook_id": payload.WebhookID,
			},
		})
		return
	}

	chainID := int(chain.ChainID)
	txData := payload.Data.New

	// 4. 创建独立的context用于异步处理
	// 不能使用 c.Request.Context()，因为HTTP响应后该context会被取消
	bgCtx := context.Background()

	// 5. 根据事件类型处理交易
	var processErr error
	switch txData.EventType {
	case "QueueTransaction":
		processErr = h.processQueueTransaction(bgCtx, txData, chainID, standard)
	case "ExecuteTransaction":
		processErr = h.processExecuteTransaction(bgCtx, txData, chainID, standard)
	case "CancelTransaction":
		processErr = h.processCancelTransaction(bgCtx, txData, chainID, standard)
	default:
		logger.Warn("Unknown event type", "event_type", txData.EventType)
		processErr = fmt.Errorf("unknown event type: %s", txData.EventType)
	}

	if processErr != nil {
		logger.Error("Failed to process transaction", processErr,
			"chain_id", chainID,
			"event_type", txData.EventType,
			"tx_hash", txData.TxHash)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "PROCESS_ERROR",
				Message: "Failed to process transaction",
				Details: processErr.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data: gin.H{
			"message":    "Webhook received and processed",
			"chain_id":   chainID,
			"standard":   standard,
			"webhook_id": payload.WebhookID,
			"event_type": txData.EventType,
			"tx_hash":    txData.TxHash,
		},
	})
}

// processQueueTransaction 处理排队交易
func (h *WebhookHandler) processQueueTransaction(ctx context.Context, txData *types.GraphQLTransactionData, chainID int, standard string) error {
	// 转换为对应的webhook交易格式并处理
	if standard == "compound" {
		compoundTx := h.convertToCompoundTransaction(txData)
		return h.processor.ProcessCompoundTransaction(ctx, compoundTx, chainID)
	} else if standard == "openzeppelin" {
		ozTx := h.convertToOpenzeppelinTransaction(txData)
		return h.processor.ProcessOpenzeppelinTransaction(ctx, ozTx, chainID)
	}

	return fmt.Errorf("unsupported standard: %s", standard)
}

// processExecuteTransaction 处理执行交易
func (h *WebhookHandler) processExecuteTransaction(ctx context.Context, txData *types.GraphQLTransactionData, chainID int, standard string) error {
	// 转换为对应的webhook交易格式并处理
	if standard == "compound" {
		compoundTx := h.convertToCompoundTransaction(txData)
		return h.processor.ProcessCompoundTransaction(ctx, compoundTx, chainID)
	} else if standard == "openzeppelin" {
		ozTx := h.convertToOpenzeppelinTransaction(txData)
		return h.processor.ProcessOpenzeppelinTransaction(ctx, ozTx, chainID)
	}

	return fmt.Errorf("unsupported standard: %s", standard)
}

// processCancelTransaction 处理取消交易
func (h *WebhookHandler) processCancelTransaction(ctx context.Context, txData *types.GraphQLTransactionData, chainID int, standard string) error {
	// 转换为对应的webhook交易格式并处理
	if standard == "compound" {
		compoundTx := h.convertToCompoundTransaction(txData)
		return h.processor.ProcessCompoundTransaction(ctx, compoundTx, chainID)
	} else if standard == "openzeppelin" {
		ozTx := h.convertToOpenzeppelinTransaction(txData)
		return h.processor.ProcessOpenzeppelinTransaction(ctx, ozTx, chainID)
	}

	return fmt.Errorf("unsupported standard: %s", standard)
}

// convertToCompoundTransaction 将GraphQL数据转换为Compound格式
func (h *WebhookHandler) convertToCompoundTransaction(txData *types.GraphQLTransactionData) types.GoldskyCompoundTransactionWebhook {
	return types.GoldskyCompoundTransactionWebhook{
		ID:              txData.ID,
		TxHash:          txData.TxHash,
		LogIndex:        txData.LogIndex,
		BlockNumber:     txData.BlockNumber,
		BlockTimestamp:  txData.BlockTimestamp,
		ContractAddress: txData.ContractAddress,
		FromAddress:     txData.FromAddress,
		EventType:       txData.EventType,
		EventTxHash:     &txData.Flow, // Compound使用flow作为eventTxHash
		EventTarget:     &txData.EventTarget,
		EventValue:      txData.EventValue,
		EventSignature:  &txData.EventSignature,
		EventData:       &txData.EventData,
		EventEta:        &txData.EventEta,
	}
}

// convertToOpenzeppelinTransaction 将GraphQL数据转换为OpenZeppelin格式
func (h *WebhookHandler) convertToOpenzeppelinTransaction(txData *types.GraphQLTransactionData) types.GoldskyOpenzeppelinTransactionWebhook {
	return types.GoldskyOpenzeppelinTransactionWebhook{
		ID:              txData.ID,
		TxHash:          txData.TxHash,
		LogIndex:        txData.LogIndex,
		BlockNumber:     txData.BlockNumber,
		BlockTimestamp:  txData.BlockTimestamp,
		ContractAddress: txData.ContractAddress,
		FromAddress:     txData.FromAddress,
		EventType:       txData.EventType,
		EventId:         &txData.Flow, // OpenZeppelin使用flow作为eventId
		EventTarget:     &txData.EventTarget,
		EventValue:      txData.EventValue,
		EventData:       &txData.EventData,
		// EventDelay需要从其他地方获取，这里暂时为空
	}
}
