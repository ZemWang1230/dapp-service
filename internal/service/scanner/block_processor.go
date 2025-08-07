package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// BlockProcessor 区块处理器
type BlockProcessor struct {
	config    *config.Config
	chainInfo *types.ChainRPCInfo

	// Compound Timelock 事件签名和ABI
	compoundEventSignatures map[string]common.Hash
	compoundABI             abi.ABI

	// OpenZeppelin Timelock 事件签名和ABI
	ozEventSignatures map[string]common.Hash
	ozABI             abi.ABI
}

// TimelockEvent Timelock事件接口
type TimelockEvent interface {
	GetEventType() string
	GetContractAddress() string
	GetTxHash() string
	GetBlockNumber() uint64
}

// NewBlockProcessor 创建新的区块处理器
func NewBlockProcessor(cfg *config.Config, chainInfo *types.ChainRPCInfo) *BlockProcessor {
	bp := &BlockProcessor{
		config:                  cfg,
		chainInfo:               chainInfo,
		compoundEventSignatures: make(map[string]common.Hash),
		ozEventSignatures:       make(map[string]common.Hash),
	}

	// 初始化事件签名和ABI
	if err := bp.initEventSignaturesAndABI(); err != nil {
		logger.Error("Failed to initialize event signatures and ABI", err)
	}

	return bp
}

// initEventSignaturesAndABI 初始化事件签名和ABI
func (bp *BlockProcessor) initEventSignaturesAndABI() error {
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
	bp.compoundABI = compoundABI

	// 解析OpenZeppelin ABI
	ozABI, err := abi.JSON(strings.NewReader(ozABIJSON))
	if err != nil {
		return fmt.Errorf("failed to parse OpenZeppelin ABI: %w", err)
	}
	bp.ozABI = ozABI

	// 初始化Compound事件签名
	bp.compoundEventSignatures["QueueTransaction"] = compoundABI.Events["QueueTransaction"].ID
	bp.compoundEventSignatures["ExecuteTransaction"] = compoundABI.Events["ExecuteTransaction"].ID
	bp.compoundEventSignatures["CancelTransaction"] = compoundABI.Events["CancelTransaction"].ID

	// 初始化OpenZeppelin事件签名
	bp.ozEventSignatures["CallScheduled"] = ozABI.Events["CallScheduled"].ID
	bp.ozEventSignatures["CallExecuted"] = ozABI.Events["CallExecuted"].ID
	bp.ozEventSignatures["Cancelled"] = ozABI.Events["Cancelled"].ID

	return nil
}

// ScanBlockRange 扫描区块范围获取timelock事件
func (bp *BlockProcessor) ScanBlockRange(ctx context.Context, client *ethclient.Client, fromBlock, toBlock int64) ([]TimelockEvent, error) {
	var allEvents []TimelockEvent

	// 获取所有相关事件的topics
	topics := bp.getAllEventTopics()

	// 使用FilterLogs获取事件
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(fromBlock),
		ToBlock:   big.NewInt(toBlock),
		Topics:    [][]common.Hash{topics}, // 第一个topic是事件签名
	}

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to filter logs from block %d to %d: %w", fromBlock, toBlock, err)
	}

	// 处理每个日志
	for _, log := range logs {
		event, err := bp.processLog(ctx, client, &log)
		if err != nil {
			logger.Error("Failed to process log", err, "tx_hash", log.TxHash.Hex(), "block", log.BlockNumber)
			continue
		}
		if event != nil {
			allEvents = append(allEvents, event)
		}
	}

	return allEvents, nil
}

// getAllEventTopics 获取所有事件的topic
func (bp *BlockProcessor) getAllEventTopics() []common.Hash {
	var topics []common.Hash

	// 添加Compound事件签名
	for _, hash := range bp.compoundEventSignatures {
		topics = append(topics, hash)
	}

	// 添加OpenZeppelin事件签名
	for _, hash := range bp.ozEventSignatures {
		topics = append(topics, hash)
	}

	return topics
}

// processLog 处理单个日志事件
func (bp *BlockProcessor) processLog(ctx context.Context, client *ethclient.Client, log *ethtypes.Log) (TimelockEvent, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}

	eventSignature := log.Topics[0]

	// 1. eth_getTransactionByHash 获取交易信息
	tx, _, err := client.TransactionByHash(ctx, log.TxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction by hash %s: %w", log.TxHash.Hex(), err)
	}

	// 2. eth_getTransactionReceipt 获取交易回执
	receipt, err := client.TransactionReceipt(ctx, log.TxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt %s: %w", log.TxHash.Hex(), err)
	}

	// 3. eth_getBlockByNumber 获取区块信息（用于时间戳）
	blockTimestamp, err := bp.getBlockTimestamp(ctx, client, log.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %d: %w", log.BlockNumber, err)
	}

	// 4. 解析from地址
	fromAddress, err := bp.getTransactionSender(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction sender: %w", err)
	}

	// 5. 确定交易状态
	txStatus := "failed"
	if receipt.Status == ethtypes.ReceiptStatusSuccessful {
		txStatus = "success"
	}

	// 6. 检查是否是Compound事件
	if event := bp.parseCompoundEvent(log, eventSignature, fromAddress, txStatus, blockTimestamp); event != nil {
		return event, nil
	}

	// 7. 检查是否是OpenZeppelin事件
	if event := bp.parseOpenZeppelinEvent(log, eventSignature, fromAddress, txStatus, blockTimestamp); event != nil {
		return event, nil
	}

	return nil, fmt.Errorf("unknown event signature: %s", eventSignature.Hex())
}

// getBlockTimestamp 获取区块时间戳
func (bp *BlockProcessor) getBlockTimestamp(ctx context.Context, client *ethclient.Client, blockNumber uint64) (uint64, error) {
	header, err := client.HeaderByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		logger.Warn("Failed to get block header by number", "block", blockNumber, "chain", bp.chainInfo.ChainName, "error", err)
		return 0, err
	}
	return header.Time, nil
}

// getTransactionSender 获取交易发送者地址
func (bp *BlockProcessor) getTransactionSender(tx *ethtypes.Transaction) (string, error) {
	// 使用Sender方法获取发送者地址
	signer := ethtypes.LatestSignerForChainID(big.NewInt(int64(bp.chainInfo.ChainID)))
	sender, err := ethtypes.Sender(signer, tx)
	if err != nil {
		return "", fmt.Errorf("failed to get transaction sender: %w", err)
	}
	return sender.Hex(), nil
}

// parseCompoundEvent 解析Compound Timelock事件
func (bp *BlockProcessor) parseCompoundEvent(log *ethtypes.Log, eventSignature common.Hash, fromAddress, txStatus string, blockTimestamp uint64) TimelockEvent {
	// 查找匹配的事件类型
	var eventType string
	for name, signature := range bp.compoundEventSignatures {
		if signature == eventSignature {
			eventType = name
			break
		}
	}

	if eventType == "" {
		return nil
	}

	// 解析事件数据
	eventData, err := bp.parseCompoundEventData(eventType, log)
	if err != nil {
		logger.Error("Failed to parse Compound event data", err, "event_type", eventType, "tx_hash", log.TxHash.Hex())
		return nil
	}

	// 创建Compound事件
	event := &types.CompoundTimelockEvent{
		EventType:       eventType,
		TxHash:          log.TxHash.Hex(),
		BlockNumber:     log.BlockNumber,
		BlockTimestamp:  blockTimestamp,
		ChainID:         bp.chainInfo.ChainID,
		ChainName:       bp.chainInfo.ChainName,
		ContractAddress: log.Address.Hex(),
		FromAddress:     fromAddress,
		ToAddress:       log.Address.Hex(), // 对于事件，to地址就是合约地址
		TxStatus:        txStatus,
		EventData:       eventData,
	}

	// 解析特定字段
	bp.extractCompoundEventFields(event, log)

	return event
}

// parseOpenZeppelinEvent 解析OpenZeppelin Timelock事件
func (bp *BlockProcessor) parseOpenZeppelinEvent(log *ethtypes.Log, eventSignature common.Hash, fromAddress, txStatus string, blockTimestamp uint64) TimelockEvent {
	// 查找匹配的事件类型
	var eventType string
	for name, signature := range bp.ozEventSignatures {
		if signature == eventSignature {
			eventType = name
			break
		}
	}

	if eventType == "" {
		return nil
	}

	// 解析事件数据
	eventData, err := bp.parseOpenZeppelinEventData(eventType, log)
	if err != nil {
		logger.Error("Failed to parse OpenZeppelin event data", err, "event_type", eventType, "tx_hash", log.TxHash.Hex())
		return nil
	}

	// 创建OpenZeppelin事件
	event := &types.OpenZeppelinTimelockEvent{
		EventType:       eventType,
		TxHash:          log.TxHash.Hex(),
		BlockNumber:     log.BlockNumber,
		BlockTimestamp:  blockTimestamp,
		ChainID:         bp.chainInfo.ChainID,
		ChainName:       bp.chainInfo.ChainName,
		ContractAddress: log.Address.Hex(),
		FromAddress:     fromAddress,
		ToAddress:       log.Address.Hex(), // 对于事件，to地址就是合约地址
		TxStatus:        txStatus,
		EventData:       eventData,
	}

	// 解析特定字段
	bp.extractOpenZeppelinEventFields(event, log)

	return event
}

// parseCompoundEventData 解析Compound事件数据
func (bp *BlockProcessor) parseCompoundEventData(eventType string, log *ethtypes.Log) (string, error) {
	event, exists := bp.compoundABI.Events[eventType]
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
func (bp *BlockProcessor) parseOpenZeppelinEventData(eventType string, log *ethtypes.Log) (map[string]interface{}, error) {
	event, exists := bp.ozABI.Events[eventType]
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
func (bp *BlockProcessor) extractCompoundEventFields(event *types.CompoundTimelockEvent, log *ethtypes.Log) {
	abiEvent, exists := bp.compoundABI.Events[event.EventType]
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
func (bp *BlockProcessor) extractOpenZeppelinEventFields(event *types.OpenZeppelinTimelockEvent, log *ethtypes.Log) {
	abiEvent, exists := bp.ozABI.Events[event.EventType]
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
