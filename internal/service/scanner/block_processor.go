package scanner

import (
	"context"
	"fmt"
	"math/big"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// BlockProcessor 区块处理器
type BlockProcessor struct {
	config    *config.Config
	chainInfo *types.ChainRPCInfo

	// Compound Timelock 事件签名
	compoundEventSignatures map[string]common.Hash

	// OpenZeppelin Timelock 事件签名
	ozEventSignatures map[string]common.Hash
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

	// 初始化事件签名
	bp.initEventSignatures()

	return bp
}

// initEventSignatures 初始化事件签名
func (bp *BlockProcessor) initEventSignatures() {
	// Compound Timelock 事件签名
	bp.compoundEventSignatures["QueueTransaction"] = crypto.Keccak256Hash([]byte("QueueTransaction(bytes32,address,uint256,string,bytes,uint256)"))
	bp.compoundEventSignatures["ExecuteTransaction"] = crypto.Keccak256Hash([]byte("ExecuteTransaction(bytes32,address,uint256,string,bytes,uint256)"))
	bp.compoundEventSignatures["CancelTransaction"] = crypto.Keccak256Hash([]byte("CancelTransaction(bytes32,address,uint256,string,bytes,uint256)"))
	bp.compoundEventSignatures["NewDelay"] = crypto.Keccak256Hash([]byte("NewDelay(uint256)"))
	bp.compoundEventSignatures["NewAdmin"] = crypto.Keccak256Hash([]byte("NewAdmin(address)"))
	bp.compoundEventSignatures["NewPendingAdmin"] = crypto.Keccak256Hash([]byte("NewPendingAdmin(address)"))

	// OpenZeppelin Timelock 事件签名
	bp.ozEventSignatures["CallScheduled"] = crypto.Keccak256Hash([]byte("CallScheduled(bytes32,uint256,address,uint256,bytes,bytes32,uint256)"))
	bp.ozEventSignatures["CallExecuted"] = crypto.Keccak256Hash([]byte("CallExecuted(bytes32,uint256,address,uint256,bytes)"))
	bp.ozEventSignatures["Cancelled"] = crypto.Keccak256Hash([]byte("Cancelled(bytes32)"))
	bp.ozEventSignatures["MinDelayChange"] = crypto.Keccak256Hash([]byte("MinDelayChange(uint256,uint256)"))
}

// GetBlockData 获取区块数据
func (bp *BlockProcessor) GetBlockData(ctx context.Context, client *ethclient.Client, blockNumber *big.Int) (*ethtypes.Block, error) {
	block, err := client.BlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %s: %w", blockNumber.String(), err)
	}

	return block, nil
}

// ProcessBlock 处理区块
func (bp *BlockProcessor) ProcessBlock(ctx context.Context, client *ethclient.Client, block *ethtypes.Block) ([]TimelockEvent, error) {
	var events []TimelockEvent

	// 获取区块的交易回执
	receipts, err := bp.getBlockReceipts(ctx, client, block)
	if err != nil {
		return nil, fmt.Errorf("failed to get block receipts: %w", err)
	}

	// 处理每个交易回执
	for _, receipt := range receipts {
		txEvents := bp.processTransactionReceipt(receipt, block)
		events = append(events, txEvents...)
	}

	return events, nil
}

// getBlockReceipts 获取区块的所有交易回执
func (bp *BlockProcessor) getBlockReceipts(ctx context.Context, client *ethclient.Client, block *ethtypes.Block) ([]*ethtypes.Receipt, error) {
	var receipts []*ethtypes.Receipt

	for _, tx := range block.Transactions() {
		receipt, err := client.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			logger.Error("Failed to get transaction receipt", err, "tx_hash", tx.Hash().Hex())
			continue // 跳过失败的交易，继续处理其他交易
		}
		receipts = append(receipts, receipt)
	}

	return receipts, nil
}

// processTransactionReceipt 处理交易回执
func (bp *BlockProcessor) processTransactionReceipt(receipt *ethtypes.Receipt, block *ethtypes.Block) []TimelockEvent {
	var events []TimelockEvent

	// 只处理成功的交易
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return events
	}

	// 处理交易日志
	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 {
			continue
		}

		eventSignature := log.Topics[0]

		// 检查是否是Compound Timelock事件
		if compoundEvent := bp.parseCompoundEvent(log, receipt, block, eventSignature); compoundEvent != nil {
			events = append(events, compoundEvent)
			continue
		}

		// 检查是否是OpenZeppelin Timelock事件
		if ozEvent := bp.parseOpenZeppelinEvent(log, receipt, block, eventSignature); ozEvent != nil {
			events = append(events, ozEvent)
		}
	}

	return events
}

// parseCompoundEvent 解析Compound Timelock事件
func (bp *BlockProcessor) parseCompoundEvent(log *ethtypes.Log, receipt *ethtypes.Receipt, block *ethtypes.Block, eventSignature common.Hash) TimelockEvent {
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

	logger.Debug("Found Compound timelock event", "type", eventType, "contract", log.Address.Hex(), "tx_hash", receipt.TxHash.Hex())

	// 获取交易发送者和接收者
	tx := bp.getTransactionFromBlock(block, receipt.TxHash)
	fromAddress := ""
	toAddress := ""
	if tx != nil {
		if tx.To() != nil {
			toAddress = tx.To().Hex()
		}
		// 通过sender获取from地址 (需要解析签名，这里简化处理)
		fromAddress = log.Address.Hex() // 简化：使用合约地址
	}

	// 创建Compound事件
	event := &types.CompoundTimelockEvent{
		EventType:       eventType,
		TxHash:          receipt.TxHash.Hex(),
		BlockNumber:     block.Number().Uint64(),
		BlockTimestamp:  block.Time(),
		ChainID:         bp.chainInfo.ChainID,
		ChainName:       bp.chainInfo.ChainName,
		ContractAddress: log.Address.Hex(),
		FromAddress:     fromAddress,
		ToAddress:       toAddress,

		// 解析事件数据
		EventData: bp.parseCompoundEventData(eventType, log),
	}

	// 根据事件类型解析特定字段
	bp.extractCompoundEventFields(event, log)

	return event
}

// parseOpenZeppelinEvent 解析OpenZeppelin Timelock事件
func (bp *BlockProcessor) parseOpenZeppelinEvent(log *ethtypes.Log, receipt *ethtypes.Receipt, block *ethtypes.Block, eventSignature common.Hash) TimelockEvent {
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

	logger.Debug("Found OpenZeppelin timelock event", "type", eventType, "contract", log.Address.Hex(), "tx_hash", receipt.TxHash.Hex())

	// 获取交易发送者和接收者
	tx := bp.getTransactionFromBlock(block, receipt.TxHash)
	fromAddress := ""
	toAddress := ""
	if tx != nil {
		if tx.To() != nil {
			toAddress = tx.To().Hex()
		}
		// 通过sender获取from地址 (需要解析签名，这里简化处理)
		fromAddress = log.Address.Hex() // 简化：使用合约地址
	}

	// 创建OpenZeppelin事件
	event := &types.OpenZeppelinTimelockEvent{
		EventType:       eventType,
		TxHash:          receipt.TxHash.Hex(),
		BlockNumber:     block.Number().Uint64(),
		BlockTimestamp:  block.Time(),
		ChainID:         bp.chainInfo.ChainID,
		ChainName:       bp.chainInfo.ChainName,
		ContractAddress: log.Address.Hex(),
		FromAddress:     fromAddress,
		ToAddress:       toAddress,

		// 解析事件数据
		EventData: bp.parseOpenZeppelinEventData(eventType, log),
	}

	// 根据事件类型解析特定字段
	bp.extractOpenZeppelinEventFields(event, log)

	return event
}

// parseCompoundEventData 解析Compound事件数据
func (bp *BlockProcessor) parseCompoundEventData(eventType string, log *ethtypes.Log) map[string]interface{} {
	data := make(map[string]interface{})

	// 基本信息
	data["event_type"] = eventType
	data["contract_address"] = log.Address.Hex()
	data["topics"] = make([]string, len(log.Topics))
	for i, topic := range log.Topics {
		data["topics"].([]string)[i] = topic.Hex()
	}
	data["data"] = common.Bytes2Hex(log.Data)

	return data
}

// parseOpenZeppelinEventData 解析OpenZeppelin事件数据
func (bp *BlockProcessor) parseOpenZeppelinEventData(eventType string, log *ethtypes.Log) map[string]interface{} {
	data := make(map[string]interface{})

	// 基本信息
	data["event_type"] = eventType
	data["contract_address"] = log.Address.Hex()
	data["topics"] = make([]string, len(log.Topics))
	for i, topic := range log.Topics {
		data["topics"].([]string)[i] = topic.Hex()
	}
	data["data"] = common.Bytes2Hex(log.Data)

	return data
}

// extractCompoundEventFields 提取Compound事件特定字段
func (bp *BlockProcessor) extractCompoundEventFields(event *types.CompoundTimelockEvent, log *ethtypes.Log) {
	switch event.EventType {
	case "QueueTransaction", "ExecuteTransaction", "CancelTransaction":
		if len(log.Topics) >= 2 {
			// txHash (bytes32)
			proposalID := log.Topics[1].Hex()
			event.ProposalID = &proposalID
		}

		// 解析data中的其他字段 (target, value, signature, data, eta)
		// 这里简化处理，实际应该使用ABI解码
		if len(log.Data) > 0 {
			// 简化版本，实际需要完整的ABI解析
			event.TargetAddress = extractAddressFromData(log.Data, 0)
			event.FunctionSignature = extractStringFromData(log.Data, 96) // 简化
		}

	case "NewDelay":
		if len(log.Data) >= 32 {
			delay := new(big.Int).SetBytes(log.Data[:32])
			newDelay := delay.Uint64()
			event.NewDelay = &newDelay
		}

	case "NewAdmin":
		if len(log.Topics) >= 2 {
			newAdmin := log.Topics[1].Hex()
			event.NewAdmin = &newAdmin
		}

	case "NewPendingAdmin":
		if len(log.Topics) >= 2 {
			newPendingAdmin := log.Topics[1].Hex()
			event.NewPendingAdmin = &newPendingAdmin
		}
	}
}

// extractOpenZeppelinEventFields 提取OpenZeppelin事件特定字段
func (bp *BlockProcessor) extractOpenZeppelinEventFields(event *types.OpenZeppelinTimelockEvent, log *ethtypes.Log) {
	switch event.EventType {
	case "CallScheduled", "CallExecuted":
		if len(log.Topics) >= 2 {
			// operationId (bytes32)
			operationID := log.Topics[1].Hex()
			event.OperationID = &operationID
		}

		// 解析其他字段
		if len(log.Data) > 0 {
			// 简化版本，实际需要完整的ABI解析
			event.TargetAddress = extractAddressFromData(log.Data, 32) // index之后
		}

	case "Cancelled":
		if len(log.Topics) >= 2 {
			operationID := log.Topics[1].Hex()
			event.OperationID = &operationID
		}

	case "MinDelayChange":
		if len(log.Data) >= 64 {
			oldDuration := new(big.Int).SetBytes(log.Data[:32])
			newDuration := new(big.Int).SetBytes(log.Data[32:64])
			oldDur := oldDuration.Uint64()
			newDur := newDuration.Uint64()
			event.OldDuration = &oldDur
			event.NewDuration = &newDur
		}
	}
}

// extractAddressFromData 从data中提取地址 (简化版本)
func extractAddressFromData(data []byte, offset int) *string {
	if len(data) < offset+32 {
		return nil
	}

	addressBytes := data[offset+12 : offset+32] // 地址占20字节，在32字节的后20字节
	address := common.BytesToAddress(addressBytes).Hex()
	return &address
}

// extractStringFromData 从data中提取字符串 (简化版本)
func extractStringFromData(data []byte, offset int) *string {
	if len(data) < offset+32 {
		return nil
	}

	// 这是一个非常简化的版本，实际应该使用完整的ABI解析
	// 这里只是为了演示，返回一个占位符
	placeholder := "function_signature_placeholder"
	return &placeholder
}

// getTransactionFromBlock 从区块中获取指定交易
func (bp *BlockProcessor) getTransactionFromBlock(block *ethtypes.Block, txHash common.Hash) *ethtypes.Transaction {
	for _, tx := range block.Transactions() {
		if tx.Hash() == txHash {
			return tx
		}
	}
	return nil
}
