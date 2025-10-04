package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthereumProcessor 以太坊通用处理器（适用于大部分EVM兼容链）
type EthereumProcessor struct {
	*BaseProcessor

	// Compound Timelock 事件签名和ABI
	compoundEventSignatures map[string]common.Hash
	compoundABI             abi.ABI

	// OpenZeppelin Timelock 事件签名和ABI
	ozEventSignatures map[string]common.Hash
	ozABI             abi.ABI
}

// NewEthereumProcessor 创建以太坊处理器
func NewEthereumProcessor() *EthereumProcessor {
	ep := &EthereumProcessor{
		BaseProcessor:           NewBaseProcessor(0, "unknown"), // 默认值，将在使用时设置
		compoundEventSignatures: make(map[string]common.Hash),
		ozEventSignatures:       make(map[string]common.Hash),
	}

	// 初始化事件签名和ABI
	if err := ep.initEventSignaturesAndABI(); err != nil {
		logger.Error("Failed to initialize Ethereum processor", err)
	}

	return ep
}

// NewEthereumProcessorWithRPCManager 创建带RPC管理器的以太坊处理器
func NewEthereumProcessorWithRPCManager(rpcManager RPCManager) *EthereumProcessor {
	ep := NewEthereumProcessor()
	ep.SetRPCManager(rpcManager)
	return ep
}

// ProcessLog 处理日志事件
func (ep *EthereumProcessor) ProcessLog(ctx context.Context, client *ethclient.Client, log *ethtypes.Log) (types.TimelockEvent, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}

	eventSignature := log.Topics[0]

	// 先尝试解析事件（仅使用log数据）
	var event types.TimelockEvent

	// 检查是否是Compound事件
	if compoundEvent := ep.parseCompoundEvent(log, eventSignature); compoundEvent != nil {
		event = compoundEvent
	} else if ozEvent := ep.parseOpenZeppelinEvent(log, eventSignature); ozEvent != nil {
		// 检查是否是OpenZeppelin事件
		event = ozEvent
	} else {
		return nil, fmt.Errorf("unknown event signature: %s", eventSignature.Hex())
	}

	// 尝试丰富事件数据（可选，失败不影响主流程）
	ep.tryEnrichEventData(ctx, client, log, event)

	return event, nil
}

// tryEnrichEventData 使用正确的chainID尝试丰富事件数据（时间戳必须获取成功）
func (ep *EthereumProcessor) tryEnrichEventData(ctx context.Context, client *ethclient.Client, log *ethtypes.Log, event types.TimelockEvent) {
	if ep.rpcManager != nil {
		// 分两步执行：
		// 1. 时间戳获取使用无限重试（必须成功）
		// 2. 其他信息使用普通重试（可选）

		// 第一步：获取时间戳（使用无限重试，因为这是必须的）
		_, _, err := ep.rpcManager.ExecuteWithRPCInfoDoInfiniteRetry(ctx, ep.chainID, func(c *ethclient.Client, _ int) error {
			return ep.getBlockTimestamp(ctx, c, log, event)
		})
		if err != nil {
			logger.Error("Failed to get block timestamp (should not happen with infinite retry)", err,
				"chain_id", ep.chainID, "tx_hash", log.TxHash.Hex(), "block_number", log.BlockNumber)
			// 即使无限重试失败（例如context取消），也记录错误
			return
		}

		// 第二步：获取其他信息（使用普通重试，失败不影响主流程）
		_, _, err = ep.rpcManager.ExecuteWithRPCInfoDo(ctx, ep.chainID, func(c *ethclient.Client, _ int) error {
			return ep.getTransactionInfo(ctx, c, log, event, ep.chainID)
		})
		if err != nil {
			logger.Warn("Failed to enrich transaction info (non-critical)",
				"chain_id", ep.chainID, "tx_hash", log.TxHash.Hex(), "error", err)
		}
	}
}

// getBlockTimestamp 获取区块时间戳（独立方法，用于无限重试）
func (ep *EthereumProcessor) getBlockTimestamp(ctx context.Context, client *ethclient.Client, log *ethtypes.Log, event types.TimelockEvent) error {
	header, err := client.HeaderByNumber(ctx, big.NewInt(int64(log.BlockNumber)))
	if err != nil {
		return fmt.Errorf("failed to get block header: %w", err)
	}

	// 设置区块时间戳
	switch e := event.(type) {
	case *types.CompoundTimelockEvent:
		e.BlockTimestamp = header.Time
	case *types.OpenZeppelinTimelockEvent:
		e.BlockTimestamp = header.Time
	}

	return nil
}

// getTransactionInfo 获取交易信息（独立方法，失败不影响主流程）
func (ep *EthereumProcessor) getTransactionInfo(ctx context.Context, client *ethclient.Client, log *ethtypes.Log, event types.TimelockEvent, chainID int) error {
	// 获取交易信息
	var tx *ethtypes.Transaction
	var err error

	tx, _, err = client.TransactionByHash(ctx, log.TxHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// 获取交易回执
	receipt, err := client.TransactionReceipt(ctx, log.TxHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	// 获取发送者地址
	sender, err := ep.getTransactionSender(tx, chainID)
	if err != nil {
		return fmt.Errorf("failed to get transaction sender: %w", err)
	}

	// 确定交易状态
	txStatus := "failed"
	if receipt.Status == ethtypes.ReceiptStatusSuccessful {
		txStatus = "success"
	}

	// 设置事件数据
	switch e := event.(type) {
	case *types.CompoundTimelockEvent:
		e.FromAddress = sender
		e.TxStatus = txStatus
	case *types.OpenZeppelinTimelockEvent:
		e.FromAddress = sender
		e.TxStatus = txStatus
	}

	return nil
}

// getTransactionSender 获取交易发送者地址
func (ep *EthereumProcessor) getTransactionSender(tx *ethtypes.Transaction, chainID int) (string, error) {
	signer := ethtypes.LatestSignerForChainID(big.NewInt(int64(chainID)))
	sender, err := ethtypes.Sender(signer, tx)
	if err != nil {
		return "", fmt.Errorf("failed to get transaction sender: %w", err)
	}
	return sender.Hex(), nil
}

// initEventSignaturesAndABI 初始化事件签名和ABI
func (ep *EthereumProcessor) initEventSignaturesAndABI() error {
	// Compound Timelock ABI定义
	compoundABIJSON := `[
		{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"txHash","type":"bytes32"},{"indexed":true,"internalType":"address","name":"target","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"},{"indexed":false,"internalType":"string","name":"signature","type":"string"},{"indexed":false,"internalType":"bytes","name":"data","type":"bytes"},{"indexed":false,"internalType":"uint256","name":"eta","type":"uint256"}],"name":"QueueTransaction","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"txHash","type":"bytes32"},{"indexed":true,"internalType":"address","name":"target","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"},{"indexed":false,"internalType":"string","name":"signature","type":"string"},{"indexed":false,"internalType":"bytes","name":"data","type":"bytes"},{"indexed":false,"internalType":"uint256","name":"eta","type":"uint256"}],"name":"ExecuteTransaction","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"txHash","type":"bytes32"},{"indexed":true,"internalType":"address","name":"target","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"},{"indexed":false,"internalType":"string","name":"signature","type":"string"},{"indexed":false,"internalType":"bytes","name":"data","type":"bytes"},{"indexed":false,"internalType":"uint256","name":"eta","type":"uint256"}],"name":"CancelTransaction","type":"event"}
	]`

	// OpenZeppelin Timelock ABI定义
	ozABIJSON := `[
		{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"id","type":"bytes32"},{"indexed":true,"internalType":"uint256","name":"index","type":"uint256"},{"indexed":false,"internalType":"address","name":"target","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"},{"indexed":false,"internalType":"bytes","name":"data","type":"bytes"},{"indexed":false,"internalType":"bytes32","name":"predecessor","type":"bytes32"},{"indexed":false,"internalType":"uint256","name":"delay","type":"uint256"}],"name":"CallScheduled","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"id","type":"bytes32"},{"indexed":true,"internalType":"uint256","name":"index","type":"uint256"},{"indexed":false,"internalType":"address","name":"target","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"},{"indexed":false,"internalType":"bytes","name":"data","type":"bytes"}],"name":"CallExecuted","type":"event"},
		{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"Cancelled","type":"event"}
	]`

	// 解析Compound ABI
	compoundABI, err := abi.JSON(strings.NewReader(compoundABIJSON))
	if err != nil {
		return fmt.Errorf("failed to parse Compound ABI: %w", err)
	}
	ep.compoundABI = compoundABI

	// 解析OpenZeppelin ABI
	ozABI, err := abi.JSON(strings.NewReader(ozABIJSON))
	if err != nil {
		return fmt.Errorf("failed to parse OpenZeppelin ABI: %w", err)
	}
	ep.ozABI = ozABI

	// 初始化Compound事件签名
	ep.compoundEventSignatures["QueueTransaction"] = compoundABI.Events["QueueTransaction"].ID
	ep.compoundEventSignatures["ExecuteTransaction"] = compoundABI.Events["ExecuteTransaction"].ID
	ep.compoundEventSignatures["CancelTransaction"] = compoundABI.Events["CancelTransaction"].ID

	// 初始化OpenZeppelin事件签名
	ep.ozEventSignatures["CallScheduled"] = ozABI.Events["CallScheduled"].ID
	ep.ozEventSignatures["CallExecuted"] = ozABI.Events["CallExecuted"].ID
	ep.ozEventSignatures["Cancelled"] = ozABI.Events["Cancelled"].ID

	return nil
}

// parseCompoundEvent 解析Compound Timelock事件
func (ep *EthereumProcessor) parseCompoundEvent(log *ethtypes.Log, eventSignature common.Hash) *types.CompoundTimelockEvent {
	// 查找匹配的事件类型
	var eventType string
	for name, signature := range ep.compoundEventSignatures {
		if signature == eventSignature {
			eventType = name
			break
		}
	}

	if eventType == "" {
		return nil
	}

	// 创建基础事件
	event := &types.CompoundTimelockEvent{
		EventType:       eventType,
		TxHash:          log.TxHash.Hex(),
		BlockNumber:     log.BlockNumber,
		ContractAddress: log.Address.Hex(),
		ToAddress:       log.Address.Hex(),

		// 默认值，后续可以通过RPC丰富
		BlockTimestamp: 0,
		FromAddress:    "0x0000000000000000000000000000000000000000",
		TxStatus:       "success",
		ChainID:        ep.chainID,   // 使用processor的chainID
		ChainName:      ep.chainName, // 使用processor的chainName
	}

	// 解析事件数据
	if eventData, err := ep.parseCompoundEventData(eventType, log); err == nil {
		event.EventData = eventData
	}

	// 解析特定字段
	ep.extractCompoundEventFields(event, log)

	return event
}

// parseOpenZeppelinEvent 解析OpenZeppelin Timelock事件
func (ep *EthereumProcessor) parseOpenZeppelinEvent(log *ethtypes.Log, eventSignature common.Hash) *types.OpenZeppelinTimelockEvent {
	// 查找匹配的事件类型
	var eventType string
	for name, signature := range ep.ozEventSignatures {
		if signature == eventSignature {
			eventType = name
			break
		}
	}

	if eventType == "" {
		return nil
	}

	// 创建基础事件
	event := &types.OpenZeppelinTimelockEvent{
		EventType:       eventType,
		TxHash:          log.TxHash.Hex(),
		BlockNumber:     log.BlockNumber,
		ContractAddress: log.Address.Hex(),
		ToAddress:       log.Address.Hex(),

		// 默认值
		BlockTimestamp: 0,
		FromAddress:    "0x0000000000000000000000000000000000000000",
		TxStatus:       "success",
		ChainID:        ep.chainID,   // 使用processor的chainID
		ChainName:      ep.chainName, // 使用processor的chainName
	}

	// 解析事件数据
	if eventData, err := ep.parseOpenZeppelinEventData(eventType, log); err == nil {
		event.EventData = eventData
	}

	// 解析特定字段
	ep.extractOpenZeppelinEventFields(event, log)

	return event
}

// parseCompoundEventData 解析Compound事件数据
func (ep *EthereumProcessor) parseCompoundEventData(eventType string, log *ethtypes.Log) (string, error) {
	event, exists := ep.compoundABI.Events[eventType]
	if !exists {
		return "", fmt.Errorf("event %s not found in Compound ABI", eventType)
	}

	// 解析事件数据
	eventData := make(map[string]interface{})
	if err := event.Inputs.UnpackIntoMap(eventData, log.Data); err != nil {
		return "", fmt.Errorf("failed to unpack event data: %w", err)
	}

	// 解析indexed参数（topics）
	indexedData := make(map[string]interface{})
	topicIndex := 1 // 第0个topic是事件签名
	for _, input := range event.Inputs {
		if input.Indexed && topicIndex < len(log.Topics) {
			switch input.Type.String() {
			case "bytes32":
				indexedData[input.Name] = log.Topics[topicIndex].Hex()
			case "address":
				indexedData[input.Name] = common.HexToAddress(log.Topics[topicIndex].Hex()).Hex()
			default:
				indexedData[input.Name] = log.Topics[topicIndex].Hex()
			}
			topicIndex++
		}
	}

	// 合并数据
	allData := make(map[string]interface{})
	allData["indexed"] = indexedData
	allData["non_indexed"] = eventData
	allData["event_type"] = eventType
	allData["contract_address"] = log.Address.Hex()

	// 转换为JSON字符串
	jsonData, err := json.Marshal(allData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event data: %w", err)
	}

	return string(jsonData), nil
}

// parseOpenZeppelinEventData 解析OpenZeppelin事件数据
func (ep *EthereumProcessor) parseOpenZeppelinEventData(eventType string, log *ethtypes.Log) (map[string]interface{}, error) {
	event, exists := ep.ozABI.Events[eventType]
	if !exists {
		return nil, fmt.Errorf("event %s not found in OpenZeppelin ABI", eventType)
	}

	// 解析事件数据
	eventData := make(map[string]interface{})
	if err := event.Inputs.UnpackIntoMap(eventData, log.Data); err != nil {
		return nil, fmt.Errorf("failed to unpack event data: %w", err)
	}

	// 解析indexed参数（topics）
	indexedData := make(map[string]interface{})
	topicIndex := 1 // 第0个topic是事件签名
	for _, input := range event.Inputs {
		if input.Indexed && topicIndex < len(log.Topics) {
			switch input.Type.String() {
			case "bytes32":
				indexedData[input.Name] = log.Topics[topicIndex].Hex()
			case "address":
				indexedData[input.Name] = common.HexToAddress(log.Topics[topicIndex].Hex()).Hex()
			case "uint256":
				indexedData[input.Name] = log.Topics[topicIndex].Big().String()
			default:
				indexedData[input.Name] = log.Topics[topicIndex].Hex()
			}
			topicIndex++
		}
	}

	// 合并数据
	allData := make(map[string]interface{})
	allData["indexed"] = indexedData
	allData["non_indexed"] = eventData
	allData["event_type"] = eventType
	allData["contract_address"] = log.Address.Hex()

	return allData, nil
}

// extractCompoundEventFields 提取Compound事件特定字段
func (ep *EthereumProcessor) extractCompoundEventFields(event *types.CompoundTimelockEvent, log *ethtypes.Log) {
	abiEvent, exists := ep.compoundABI.Events[event.EventType]
	if !exists {
		logger.Error("Event not found in ABI", fmt.Errorf("event %s not found", event.EventType), "event_type", event.EventType)
		return
	}

	// 解析非索引数据
	eventData := make(map[string]interface{})
	if err := abiEvent.Inputs.UnpackIntoMap(eventData, log.Data); err != nil {
		logger.Error("Failed to unpack event data", err)
		return
	}

	// 解析索引数据（topics）
	topicIndex := 1
	for _, input := range abiEvent.Inputs {
		if input.Indexed && topicIndex < len(log.Topics) {
			switch input.Name {
			case "txHash":
				txHashHex := log.Topics[topicIndex].Hex()
				event.EventTxHash = &txHashHex
			case "target":
				targetAddr := common.HexToAddress(log.Topics[topicIndex].Hex()).Hex()
				event.EventTarget = &targetAddr
			}
			topicIndex++
		}
	}

	// 解析非索引数据中的字段
	if value, ok := eventData["value"]; ok {
		if bigIntValue, ok := value.(*big.Int); ok {
			event.EventValue = bigIntValue.String()
		}
	}

	if signature, ok := eventData["signature"]; ok {
		if sigStr, ok := signature.(string); ok {
			event.EventFunctionSignature = &sigStr
		}
	}

	if data, ok := eventData["data"]; ok {
		if dataBytes, ok := data.([]byte); ok {
			event.EventCallData = dataBytes
		}
	}

	if eta, ok := eventData["eta"]; ok {
		if bigIntEta, ok := eta.(*big.Int); ok {
			etaInt64 := bigIntEta.Int64()
			event.EventEta = &etaInt64
		}
	}
}

// extractOpenZeppelinEventFields 提取OpenZeppelin事件特定字段
func (ep *EthereumProcessor) extractOpenZeppelinEventFields(event *types.OpenZeppelinTimelockEvent, log *ethtypes.Log) {
	abiEvent, exists := ep.ozABI.Events[event.EventType]
	if !exists {
		logger.Error("Event not found in ABI", fmt.Errorf("event %s not found", event.EventType), "event_type", event.EventType)
		return
	}

	// 解析非索引数据
	eventData := make(map[string]interface{})
	if err := abiEvent.Inputs.UnpackIntoMap(eventData, log.Data); err != nil {
		logger.Error("Failed to unpack event data", err)
		return
	}

	// 解析索引数据（topics）
	topicIndex := 1
	for _, input := range abiEvent.Inputs {
		if input.Indexed && topicIndex < len(log.Topics) {
			switch input.Name {
			case "id":
				idHex := log.Topics[topicIndex].Hex()
				event.EventID = &idHex
			case "index":
				if event.EventType == "CallScheduled" || event.EventType == "CallExecuted" {
					event.EventIndex = int(log.Topics[topicIndex].Big().Int64())
				}
			}
			topicIndex++
		}
	}

	// 解析非索引数据中的字段
	if target, ok := eventData["target"]; ok {
		if targetAddr, ok := target.(common.Address); ok {
			targetStr := targetAddr.Hex()
			event.EventTarget = &targetStr
		}
	}

	if value, ok := eventData["value"]; ok {
		if bigIntValue, ok := value.(*big.Int); ok {
			event.EventValue = bigIntValue.String()
		}
	}

	if data, ok := eventData["data"]; ok {
		if dataBytes, ok := data.([]byte); ok {
			event.EventCallData = dataBytes
		}
	}

	if predecessor, ok := eventData["predecessor"]; ok {
		if predBytes, ok := predecessor.([32]byte); ok {
			predHex := common.BytesToHash(predBytes[:]).Hex()
			event.EventPredecessor = &predHex
		}
	}

	if delay, ok := eventData["delay"]; ok {
		if bigIntDelay, ok := delay.(*big.Int); ok {
			delayInt64 := bigIntDelay.Int64()
			event.EventDelay = &delayInt64
		}
	}
}
