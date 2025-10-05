package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/service/processor"
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
	config           *config.Config
	chainInfo        *types.ChainRPCInfo
	processorManager *processor.ProcessorManager
	rpcManager       RPCManager

	// 保留原有的ABI用于扫描（获取topics）
	compoundEventSignatures map[string]common.Hash
	compoundABI             abi.ABI
	ozEventSignatures       map[string]common.Hash
	ozABI                   abi.ABI
}

// RPCManager 接口定义（与processor包中的接口保持一致）
type RPCManager interface {
	ExecuteWithRPCInfoDo(ctx context.Context, chainID int, fn func(*ethclient.Client, int) error) (string, int, error)
	ExecuteWithRPCInfoDoInfiniteRetry(ctx context.Context, chainID int, fn func(*ethclient.Client, int) error) (string, int, error)
}

// TimelockEvent Timelock事件接口
type TimelockEvent interface {
	GetEventType() string
	GetContractAddress() string
	GetTxHash() string
	GetBlockNumber() uint64
}

// NewBlockProcessorWithRPCManager 创建带RPC管理器的区块处理器
func NewBlockProcessorWithRPCManager(cfg *config.Config, chainInfo *types.ChainRPCInfo, rpcManager RPCManager) *BlockProcessor {
	bp := &BlockProcessor{
		config:                  cfg,
		chainInfo:               chainInfo,
		processorManager:        processor.NewProcessorManagerWithRPCManager(rpcManager),
		rpcManager:              rpcManager,
		compoundEventSignatures: make(map[string]common.Hash),
		ozEventSignatures:       make(map[string]common.Hash),
	}

	// 初始化事件签名和ABI（用于扫描）
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

// ScanBlockRangeRaw 扫描区块范围获取原始日志数据
func (bp *BlockProcessor) ScanBlockRangeRaw(ctx context.Context, client *ethclient.Client, fromBlock, toBlock int64) ([]ethtypes.Log, error) {
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
		logger.Error("Failed to filter logs", err, "chain_id", bp.chainInfo.ChainID, "from_block", fromBlock, "to_block", toBlock)
		return nil, fmt.Errorf("failed to filter logs from block %d to %d: %w", fromBlock, toBlock, err)
	}

	return logs, nil
}

// ProcessLog 处理日志事件（公开方法）
func (bp *BlockProcessor) ProcessLog(ctx context.Context, client *ethclient.Client, log ethtypes.Log) (TimelockEvent, error) {
	return bp.processLog(ctx, client, &log)
}

// ProcessLogFromRawData 从原始日志数据处理事件
func (bp *BlockProcessor) ProcessLogFromRawData(ctx context.Context, client *ethclient.Client, rawLogData string) (types.TimelockEvent, error) {
	// 解析原始日志数据
	var log ethtypes.Log
	if err := json.Unmarshal([]byte(rawLogData), &log); err != nil {
		logger.Error("Failed to unmarshal log data", err, "chain_id", bp.chainInfo.ChainID, "raw_log_data", rawLogData)
		return nil, fmt.Errorf("failed to unmarshal log data: %w", err)
	}

	// 使用现有的processLog方法处理
	return bp.processLog(ctx, client, &log)
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

	// 使用新的处理器架构
	event, err := bp.processorManager.ProcessLog(ctx, client, log, bp.chainInfo.ChainID)
	if err != nil {
		logger.Error("Failed to process log with chain processor", err,
			"chain_id", bp.chainInfo.ChainID,
			"tx_hash", log.TxHash.Hex(),
			"block_number", log.BlockNumber)
		return nil, err
	}

	return event, nil
}
