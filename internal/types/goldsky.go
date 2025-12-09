package types

import "time"

// CompoundTimelockFlowDB Compound Timelock Flow 数据库模型
type CompoundTimelockFlowDB struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	FlowID            string     `gorm:"size:128;not null;index"`
	TimelockStandard  string     `gorm:"size:20;not null;default:'compound'"`
	ChainID           int        `gorm:"not null;index"`
	ContractAddress   string     `gorm:"size:42;not null;index"`
	Status            string     `gorm:"size:20;not null;default:'waiting';index"`
	QueueTxHash       *string    `gorm:"size:66"`
	ExecuteTxHash     *string    `gorm:"size:66"`
	CancelTxHash      *string    `gorm:"size:66"`
	InitiatorAddress  *string    `gorm:"size:42"`
	TargetAddress     *string    `gorm:"size:42"`
	Value             string     `gorm:"type:decimal(78,0);not null;default:0"`
	CallData          []byte     `gorm:"type:bytea"`
	FunctionSignature *string    `gorm:"type:text"`
	QueuedAt          *time.Time `gorm:"type:timestamptz"`
	Eta               *time.Time `gorm:"type:timestamptz"`
	GracePeriod       *int64
	ExpiredAt         *time.Time `gorm:"type:timestamptz"`
	ExecutedAt        *time.Time `gorm:"type:timestamptz"`
	CancelledAt       *time.Time `gorm:"type:timestamptz"`
	CreatedAt         time.Time  `gorm:"not null;default:now()"`
	UpdatedAt         time.Time  `gorm:"not null;default:now()"`
}

// TableName 设置表名
func (CompoundTimelockFlowDB) TableName() string {
	return "compound_timelock_flows"
}

// OpenzeppelinTimelockFlowDB OpenZeppelin Timelock Flow 数据库模型
type OpenzeppelinTimelockFlowDB struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	FlowID           string     `gorm:"size:128;not null;index"`
	TimelockStandard string     `gorm:"size:20;not null;default:'openzeppelin'"`
	ChainID          int        `gorm:"not null;index"`
	ContractAddress  string     `gorm:"size:42;not null;index"`
	Status           string     `gorm:"size:20;not null;default:'waiting';index"`
	ScheduleTxHash   *string    `gorm:"size:66"`
	ExecuteTxHash    *string    `gorm:"size:66"`
	CancelTxHash     *string    `gorm:"size:66"`
	InitiatorAddress *string    `gorm:"size:42"`
	TargetAddress    *string    `gorm:"size:42"`
	Value            string     `gorm:"type:decimal(78,0);not null;default:0"`
	CallData         []byte     `gorm:"type:bytea"`
	QueuedAt         *time.Time `gorm:"type:timestamptz"`
	Delay            *int64
	Eta              *time.Time `gorm:"type:timestamptz"`
	ExecutedAt       *time.Time `gorm:"type:timestamptz"`
	CancelledAt      *time.Time `gorm:"type:timestamptz"`
	CreatedAt        time.Time  `gorm:"not null;default:now()"`
	UpdatedAt        time.Time  `gorm:"not null;default:now()"`
}

// TableName 设置表名
func (OpenzeppelinTimelockFlowDB) TableName() string {
	return "openzeppelin_timelock_flows"
}

// GraphQL 返回的数据结构（从 Goldsky 获取）

// GoldskyCompoundFlow Goldsky 返回的 Compound Flow 数据
type GoldskyCompoundFlow struct {
	ID                 string                      `json:"id"`
	FlowID             string                      `json:"flowId"`
	TimelockStandard   string                      `json:"timelockStandard"`
	ContractAddress    string                      `json:"contractAddress"`
	Status             string                      `json:"status"`
	QueueTransaction   *GoldskyCompoundTransaction `json:"queueTransaction"`
	ExecuteTransaction *GoldskyCompoundTransaction `json:"executeTransaction"`
	CancelTransaction  *GoldskyCompoundTransaction `json:"cancelTransaction"`
	InitiatorAddress   *string                     `json:"initiatorAddress"`
	TargetAddress      *string                     `json:"targetAddress"`
	Value              string                      `json:"value"`
	CallData           *string                     `json:"callData"`
	FunctionSignature  *string                     `json:"functionSignature"`
	QueuedAt           *string                     `json:"queuedAt"`
	Eta                *string                     `json:"eta"`
	GracePeriod        *string                     `json:"gracePeriod"`
	ExpiredAt          *string                     `json:"expiredAt"`
	ExecutedAt         *string                     `json:"executedAt"`
	CancelledAt        *string                     `json:"cancelledAt"`
	CreatedAt          string                      `json:"createdAt"`
	UpdatedAt          string                      `json:"updatedAt"`
}

// GoldskyCompoundTransaction Goldsky 返回的 Compound Transaction 数据
type GoldskyCompoundTransaction struct {
	ID              string  `json:"id"`
	TxHash          string  `json:"txHash"`
	LogIndex        string  `json:"logIndex"`
	BlockNumber     string  `json:"blockNumber"`
	BlockTimestamp  string  `json:"blockTimestamp"`
	ContractAddress string  `json:"contractAddress"`
	FromAddress     string  `json:"fromAddress"`
	EventType       string  `json:"eventType"`
	EventTxHash     *string `json:"eventTxHash"`
	EventTarget     *string `json:"eventTarget"`
	EventValue      string  `json:"eventValue"`
	EventSignature  *string `json:"eventSignature"`
	EventData       *string `json:"eventData"`
	EventEta        *string `json:"eventEta"`
}

// GoldskyOpenzeppelinFlow Goldsky 返回的 OpenZeppelin Flow 数据
type GoldskyOpenzeppelinFlow struct {
	ID                  string                          `json:"id"`
	FlowID              string                          `json:"flowId"`
	TimelockStandard    string                          `json:"timelockStandard"`
	ContractAddress     string                          `json:"contractAddress"`
	Status              string                          `json:"status"`
	ScheduleTransaction *GoldskyOpenzeppelinTransaction `json:"scheduleTransaction"`
	ExecuteTransaction  *GoldskyOpenzeppelinTransaction `json:"executeTransaction"`
	CancelTransaction   *GoldskyOpenzeppelinTransaction `json:"cancelTransaction"`
	InitiatorAddress    *string                         `json:"initiatorAddress"`
	TargetAddress       *string                         `json:"targetAddress"`
	Value               string                          `json:"value"`
	CallData            *string                         `json:"callData"`
	QueuedAt            *string                         `json:"queuedAt"`
	Delay               *string                         `json:"delay"`
	Eta                 *string                         `json:"eta"`
	ExecutedAt          *string                         `json:"executedAt"`
	CancelledAt         *string                         `json:"cancelledAt"`
	CreatedAt           string                          `json:"createdAt"`
	UpdatedAt           string                          `json:"updatedAt"`
}

// GoldskyOpenzeppelinTransaction Goldsky 返回的 OpenZeppelin Transaction 数据
type GoldskyOpenzeppelinTransaction struct {
	ID               string  `json:"id"`
	TxHash           string  `json:"txHash"`
	LogIndex         string  `json:"logIndex"`
	BlockNumber      string  `json:"blockNumber"`
	BlockTimestamp   string  `json:"blockTimestamp"`
	ContractAddress  string  `json:"contractAddress"`
	FromAddress      string  `json:"fromAddress"`
	EventType        string  `json:"eventType"`
	EventId          *string `json:"eventId"`
	EventIndex       *string `json:"eventIndex"`
	EventTarget      *string `json:"eventTarget"`
	EventValue       string  `json:"eventValue"`
	EventData        *string `json:"eventData"`
	EventPredecessor *string `json:"eventPredecessor"`
	EventDelay       *string `json:"eventDelay"`
}

// GraphQL Query 响应结构

// GoldskyCompoundFlowsResponse Compound Flows 查询响应
type GoldskyCompoundFlowsResponse struct {
	Data struct {
		CompoundTimelockFlows []GoldskyCompoundFlow `json:"compoundTimelockFlows"`
	} `json:"data"`
}

// GoldskyOpenzeppelinFlowsResponse OpenZeppelin Flows 查询响应
type GoldskyOpenzeppelinFlowsResponse struct {
	Data struct {
		OpenzeppelinTimelockFlows []GoldskyOpenzeppelinFlow `json:"openzeppelinTimelockFlows"`
	} `json:"data"`
}

// GoldskyCompoundTransactionResponse Compound Transaction 查询响应
type GoldskyCompoundTransactionResponse struct {
	Data struct {
		CompoundTimelockTransactions []GoldskyCompoundTransaction `json:"compoundTimelockTransactions"`
	} `json:"data"`
}

// GoldskyOpenzeppelinTransactionResponse OpenZeppelin Transaction 查询响应
type GoldskyOpenzeppelinTransactionResponse struct {
	Data struct {
		OpenzeppelinTimelockTransactions []GoldskyOpenzeppelinTransaction `json:"openzeppelinTimelockTransactions"`
	} `json:"data"`
}

// SyncFlowsResponse 同步flows响应
type SyncFlowsResponse struct {
	Message  string `json:"message"`
	Duration string `json:"duration"`
}
