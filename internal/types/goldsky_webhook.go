package types

// GoldskyWebhookPayload Goldsky Webhook 请求体
type GoldskyWebhookPayload struct {
	Event struct {
		Data struct {
			// Compound Transaction 事件
			CompoundTimelockTransactions []GoldskyCompoundTransactionWebhook `json:"compoundTimelockTransactions,omitempty"`

			// OpenZeppelin Transaction 事件
			OpenzeppelinTimelockTransactions []GoldskyOpenzeppelinTransactionWebhook `json:"openzeppelinTimelockTransactions,omitempty"`
		} `json:"data"`
	} `json:"event"`
}

// GoldskyCompoundTransactionWebhook Webhook 推送的 Compound Transaction
type GoldskyCompoundTransactionWebhook struct {
	ID              string `json:"id"`
	TxHash          string `json:"txHash"`
	LogIndex        string `json:"logIndex"`
	BlockNumber     string `json:"blockNumber"`
	BlockTimestamp  string `json:"blockTimestamp"`
	ContractAddress string `json:"contractAddress"`
	FromAddress     string `json:"fromAddress"`
	EventType       string `json:"eventType"` // QueueTransaction, ExecuteTransaction, CancelTransaction

	// Event 数据
	EventTxHash    *string `json:"eventTxHash"`    // Flow ID (Compound 使用 txHash 作为 flowId)
	EventTarget    *string `json:"eventTarget"`    // 目标地址
	EventValue     string  `json:"eventValue"`     // 金额
	EventSignature *string `json:"eventSignature"` // 函数签名
	EventData      *string `json:"eventData"`      // 调用数据
	EventEta       *string `json:"eventEta"`       // ETA 时间戳
}

// GoldskyOpenzeppelinTransactionWebhook Webhook 推送的 OpenZeppelin Transaction
type GoldskyOpenzeppelinTransactionWebhook struct {
	ID              string `json:"id"`
	TxHash          string `json:"txHash"`
	LogIndex        string `json:"logIndex"`
	BlockNumber     string `json:"blockNumber"`
	BlockTimestamp  string `json:"blockTimestamp"`
	ContractAddress string `json:"contractAddress"`
	FromAddress     string `json:"fromAddress"`
	EventType       string `json:"eventType"` // CallScheduled, CallExecuted, Cancelled

	// Event 数据
	EventId          *string `json:"eventId"`          // Flow ID (OpenZeppelin 使用 id 作为 flowId)
	EventIndex       *string `json:"eventIndex"`       // 索引
	EventTarget      *string `json:"eventTarget"`      // 目标地址
	EventValue       string  `json:"eventValue"`       // 金额
	EventData        *string `json:"eventData"`        // 调用数据
	EventPredecessor *string `json:"eventPredecessor"` // 前驱
	EventDelay       *string `json:"eventDelay"`       // 延迟时间
}

// WebhookEventType Webhook 事件类型
type WebhookEventType string

const (
	// Compound 事件类型
	WebhookEventCompoundQueue   WebhookEventType = "QueueTransaction"
	WebhookEventCompoundExecute WebhookEventType = "ExecuteTransaction"
	WebhookEventCompoundCancel  WebhookEventType = "CancelTransaction"

	// OpenZeppelin 事件类型
	WebhookEventOZSchedule WebhookEventType = "CallScheduled"
	WebhookEventOZExecute  WebhookEventType = "CallExecuted"
	WebhookEventOZCancel   WebhookEventType = "Cancelled"
)

// ParsedWebhookTransaction 解析后的 Webhook 交易
type ParsedWebhookTransaction struct {
	EventType       WebhookEventType
	Standard        string // compound or openzeppelin
	ChainID         int
	ContractAddress string
	FlowID          string // eventTxHash for Compound, eventId for OpenZeppelin
	TxHash          string
	FromAddress     string
	BlockTimestamp  string

	// 交易详细信息
	TargetAddress     *string
	Value             string
	CallData          *string
	FunctionSignature *string
	Eta               *string
	Delay             *string
}
