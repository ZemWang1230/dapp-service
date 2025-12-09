package goldsky

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
	logger.Info("Received Goldsky webhook request", "method", c.Request.Method, "path", c.Request.URL.Path, "remote_addr", c.ClientIP(), "content_type", c.GetHeader("Content-Type"))

	// 1. 获取 Webhook Secret (优先从header获取)
	secret := c.GetHeader("goldsky-webhook-secret")
	logger.Info("Webhook headers", "goldsky-webhook-secret", secret)

	// 2. 解析 Payload
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

	logger.Info("Parsed webhook payload", "webhook_id", payload.WebhookID, "entity", payload.Entity)

	// 3. 如果header中没有secret，尝试使用payload中的webhook_id作为fallback
	if secret == "" {
		secret = payload.WebhookID
		logger.Info("No secret in header, using webhook_id from payload", "webhook_id", secret)
	}

	if secret == "" {
		logger.Warn("Missing webhook secret in both header and payload")
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "MISSING_SECRET",
				Message: "Missing webhook secret",
			},
		})
		return
	}

	// 4. 通过 secret 查找对应的链和类型
	chain, standard, err := h.chainRepo.GetChainByWebhookSecret(c.Request.Context(), secret)
	if err != nil {
		logger.Warn("Invalid webhook secret", "secret", secret, "error", err)
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error: &types.APIError{
				Code:    "INVALID_WEBHOOK",
				Message: "Invalid webhook secret",
			},
		})
		return
	}

	// 5. 检查是否有新数据
	if payload.Data.New == nil {
		logger.Info("No new data in webhook payload", "webhook_id", payload.WebhookID, "chain_id", chain.ChainID)
		c.JSON(http.StatusOK, types.APIResponse{
			Success: true,
			Data: gin.H{
				"message":    "No new data to process",
				"webhook_id": payload.WebhookID,
				"chain_id":   chain.ChainID,
			},
		})
		return
	}

	chainID := int(chain.ChainID)
	txData := payload.Data.New

	logger.Info("Processing webhook transaction",
		"chain_id", chainID,
		"standard", standard,
		"event_type", txData.EventType,
		"tx_hash", txData.TxHash,
		"flow", txData.Flow)

	// 6. 创建独立的context用于异步处理
	// 不能使用 c.Request.Context()，因为HTTP响应后该context会被取消
	bgCtx := context.Background()

	// 7. 根据事件类型处理交易
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
	// 转换地址格式：\\x -> 0x
	txHash := normalizeAddress(txData.TxHash)
	contractAddress := normalizeAddress(txData.ContractAddress)
	fromAddress := normalizeAddress(txData.FromAddress)
	eventTxHash := normalizeAddress(txData.EventTxHash)
	eventTarget := normalizeAddress(txData.EventTarget)
	eventData := normalizeAddress(txData.EventData)

	return types.GoldskyCompoundTransactionWebhook{
		ID:              txData.ID,
		TxHash:          txHash,
		LogIndex:        txData.LogIndex,
		BlockNumber:     txData.BlockNumber,
		BlockTimestamp:  txData.BlockTimestamp,
		ContractAddress: contractAddress,
		FromAddress:     fromAddress,
		EventType:       txData.EventType,
		EventTxHash:     &eventTxHash, // 使用event_tx_hash字段
		EventTarget:     &eventTarget,
		EventValue:      txData.EventValue,
		EventSignature:  &txData.EventSignature,
		EventData:       &eventData,
		EventEta:        &txData.EventEta,
	}
}

// convertToOpenzeppelinTransaction 将GraphQL数据转换为OpenZeppelin格式
func (h *WebhookHandler) convertToOpenzeppelinTransaction(txData *types.GraphQLTransactionData) types.GoldskyOpenzeppelinTransactionWebhook {
	// 转换地址格式：\\x -> 0x
	txHash := normalizeAddress(txData.TxHash)
	contractAddress := normalizeAddress(txData.ContractAddress)
	fromAddress := normalizeAddress(txData.FromAddress)
	flow := normalizeAddress(txData.Flow)
	eventTarget := normalizeAddress(txData.EventTarget)
	eventData := normalizeAddress(txData.EventData)

	return types.GoldskyOpenzeppelinTransactionWebhook{
		ID:              txData.ID,
		TxHash:          txHash,
		LogIndex:        txData.LogIndex,
		BlockNumber:     txData.BlockNumber,
		BlockTimestamp:  txData.BlockTimestamp,
		ContractAddress: contractAddress,
		FromAddress:     fromAddress,
		EventType:       txData.EventType,
		EventId:         &flow, // OpenZeppelin使用flow作为eventId
		EventTarget:     &eventTarget,
		EventValue:      txData.EventValue,
		EventData:       &eventData,
		// EventDelay需要从其他地方获取，这里暂时为空
	}
}

// normalizeAddress 将PostgreSQL字节数组格式(\\x)转换为以太坊地址格式(0x)
func normalizeAddress(addr string) string {
	// 将 \\x 替换为 0x
	if strings.HasPrefix(addr, "\\x") {
		return "0x" + strings.TrimPrefix(addr, "\\x")
	}
	// 如果已经是0x开头，直接返回
	if strings.HasPrefix(addr, "0x") {
		return addr
	}
	// 如果既不是\\x也不是0x，添加0x前缀
	return "0x" + addr
}
