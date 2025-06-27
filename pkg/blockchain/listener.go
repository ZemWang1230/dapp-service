package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"timelocker-backend/internal/config"
	chainRepository "timelocker-backend/internal/repository/chain"
	timelockRepository "timelocker-backend/internal/repository/timelock"
	"timelocker-backend/internal/service/transaction"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

// EventListener 区块链事件监听器
type EventListener struct {
	config         *config.Config
	transactionSvc transaction.Service
	chainRepo      chainRepository.Repository
	timelockRepo   timelockRepository.Repository
	db             *gorm.DB

	// 核心状态
	clients      map[int]*ethclient.Client
	blockNumbers map[int]*big.Int // 每个链的最新处理区块号
	quit         chan struct{}

	// 并发控制
	mu sync.RWMutex // 统一的读写锁

	// 简化缓存 - 只保留最必要的
	txCache sync.Map // 交易哈希缓存 map[string]*types.Transaction
}

// TimelockEvent 时间锁事件接口
type TimelockEvent interface {
	GetTxHash() string
	GetStatus() types.TransactionStatus
	GetBlockNumber() *big.Int
	GetContractAddress() string
}

// CompoundQueuedEvent Compound排队事件
type CompoundQueuedEvent struct {
	TxHash          string
	ContractAddress string
	Target          common.Address
	Value           *big.Int
	Signature       string
	Data            []byte
	ETA             *big.Int
	BlockNumber     *big.Int
}

func (e *CompoundQueuedEvent) GetTxHash() string                  { return e.TxHash }
func (e *CompoundQueuedEvent) GetStatus() types.TransactionStatus { return types.TransactionQueued }
func (e *CompoundQueuedEvent) GetBlockNumber() *big.Int           { return e.BlockNumber }
func (e *CompoundQueuedEvent) GetContractAddress() string         { return e.ContractAddress }

// CompoundExecutedEvent Compound执行事件
type CompoundExecutedEvent struct {
	TxHash          string
	ContractAddress string
	Target          common.Address
	Value           *big.Int
	Signature       string
	Data            []byte
	ETA             *big.Int
	BlockNumber     *big.Int
}

func (e *CompoundExecutedEvent) GetTxHash() string                  { return e.TxHash }
func (e *CompoundExecutedEvent) GetStatus() types.TransactionStatus { return types.TransactionExecuted }
func (e *CompoundExecutedEvent) GetBlockNumber() *big.Int           { return e.BlockNumber }
func (e *CompoundExecutedEvent) GetContractAddress() string         { return e.ContractAddress }

// CompoundCancelledEvent Compound取消事件
type CompoundCancelledEvent struct {
	TxHash          string
	ContractAddress string
	Target          common.Address
	Value           *big.Int
	Signature       string
	Data            []byte
	ETA             *big.Int
	BlockNumber     *big.Int
}

func (e *CompoundCancelledEvent) GetTxHash() string { return e.TxHash }
func (e *CompoundCancelledEvent) GetStatus() types.TransactionStatus {
	return types.TransactionCanceled
}
func (e *CompoundCancelledEvent) GetBlockNumber() *big.Int   { return e.BlockNumber }
func (e *CompoundCancelledEvent) GetContractAddress() string { return e.ContractAddress }

// OpenZeppelinCallScheduledEvent OpenZeppelin调度事件
type OpenZeppelinCallScheduledEvent struct {
	TxHash          string
	ContractAddress string
	ID              [32]byte
	Index           *big.Int
	Target          common.Address
	Value           *big.Int
	Data            []byte
	Predecessor     [32]byte
	Delay           *big.Int
	BlockNumber     *big.Int
}

func (e *OpenZeppelinCallScheduledEvent) GetTxHash() string { return e.TxHash }
func (e *OpenZeppelinCallScheduledEvent) GetStatus() types.TransactionStatus {
	return types.TransactionQueued
}
func (e *OpenZeppelinCallScheduledEvent) GetBlockNumber() *big.Int   { return e.BlockNumber }
func (e *OpenZeppelinCallScheduledEvent) GetContractAddress() string { return e.ContractAddress }

// OpenZeppelinCallExecutedEvent OpenZeppelin执行事件
type OpenZeppelinCallExecutedEvent struct {
	TxHash          string
	ContractAddress string
	ID              [32]byte
	Index           *big.Int
	Target          common.Address
	Value           *big.Int
	Data            []byte
	BlockNumber     *big.Int
}

func (e *OpenZeppelinCallExecutedEvent) GetTxHash() string { return e.TxHash }
func (e *OpenZeppelinCallExecutedEvent) GetStatus() types.TransactionStatus {
	return types.TransactionExecuted
}
func (e *OpenZeppelinCallExecutedEvent) GetBlockNumber() *big.Int   { return e.BlockNumber }
func (e *OpenZeppelinCallExecutedEvent) GetContractAddress() string { return e.ContractAddress }

// OpenZeppelinCallCancelledEvent OpenZeppelin取消事件
type OpenZeppelinCallCancelledEvent struct {
	TxHash          string
	ContractAddress string
	ID              [32]byte
	BlockNumber     *big.Int
}

func (e *OpenZeppelinCallCancelledEvent) GetTxHash() string { return e.TxHash }
func (e *OpenZeppelinCallCancelledEvent) GetStatus() types.TransactionStatus {
	return types.TransactionCanceled
}
func (e *OpenZeppelinCallCancelledEvent) GetBlockNumber() *big.Int   { return e.BlockNumber }
func (e *OpenZeppelinCallCancelledEvent) GetContractAddress() string { return e.ContractAddress }

// TimelockContract timelock合约信息
type TimelockContract struct {
	ContractAddress string
	Standard        string
}

// 事件签名哈希常量
var (
	// Compound Timelock事件
	compoundQueuedEventSig    = common.HexToHash("0x76e2796dc3a81d57b0e8504b647febcbeeb5f4af818e164f11eef8131a6a763f")
	compoundExecutedEventSig  = common.HexToHash("0xa560e3198060a2f10670c1ec5b403077ea6ae93ca8de1c32b451dc1a943cd6e7")
	compoundCancelledEventSig = common.HexToHash("0x2fffc091a501fd91bfbff27141450d3acb40fb8e6d8382b243ec7a812a8b8b23")

	// OpenZeppelin TimelockController事件
	ozCallScheduledEventSig = common.HexToHash("0x4cf4410cc57040e44862ef2f45bedf5f3d098289d8e43ece8d66c6a616b7e31e")
	ozCallExecutedEventSig  = common.HexToHash("0xc2617efa69bab66782fa219543714338489c4e9e178271560a91b82c3f612b58")
	ozCallCancelledEventSig = common.HexToHash("0xbaa1eb22f2a492ba1a5fea61b8df4d27c6c8b5f3971e63bb58fa14ff72eedb70")
)

// ABI定义常量
const (
	// Compound Timelock ABI - 用于解析事件参数
	compoundQueuedABI = `[{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "txHash", "type": "bytes32"},
			{"indexed": true, "name": "target", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"},
			{"indexed": false, "name": "signature", "type": "string"},
			{"indexed": false, "name": "data", "type": "bytes"},
			{"indexed": false, "name": "eta", "type": "uint256"}
		],
		"name": "QueueTransaction",
		"type": "event"
	}]`

	compoundExecutedABI = `[{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "txHash", "type": "bytes32"},
			{"indexed": true, "name": "target", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"},
			{"indexed": false, "name": "signature", "type": "string"},
			{"indexed": false, "name": "data", "type": "bytes"},
			{"indexed": false, "name": "eta", "type": "uint256"}
		],
		"name": "ExecuteTransaction",
		"type": "event"
	}]`

	compoundCancelledABI = `[{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "txHash", "type": "bytes32"},
			{"indexed": true, "name": "target", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"},
			{"indexed": false, "name": "signature", "type": "string"},
			{"indexed": false, "name": "data", "type": "bytes"},
			{"indexed": false, "name": "eta", "type": "uint256"}
		],
		"name": "CancelTransaction",
		"type": "event"
	}]`

	// OpenZeppelin TimelockController ABI
	ozCallScheduledABI = `[{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "id", "type": "bytes32"},
			{"indexed": true, "name": "index", "type": "uint256"},
			{"indexed": false, "name": "target", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"},
			{"indexed": false, "name": "data", "type": "bytes"},
			{"indexed": false, "name": "predecessor", "type": "bytes32"},
			{"indexed": false, "name": "delay", "type": "uint256"}
		],
		"name": "CallScheduled",
		"type": "event"
	}]`

	ozCallExecutedABI = `[{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "id", "type": "bytes32"},
			{"indexed": true, "name": "index", "type": "uint256"},
			{"indexed": false, "name": "target", "type": "address"},
			{"indexed": false, "name": "value", "type": "uint256"},
			{"indexed": false, "name": "data", "type": "bytes"}
		],
		"name": "CallExecuted",
		"type": "event"
	}]`

	ozCallCancelledABI = `[{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "id", "type": "bytes32"}
		],
		"name": "Cancelled",
		"type": "event"
	}]`
)

// NewEventListener 创建新的事件监听器
func NewEventListener(config *config.Config, transactionSvc transaction.Service, chainRepo chainRepository.Repository, timelockRepo timelockRepository.Repository, db *gorm.DB) *EventListener {
	return &EventListener{
		config:         config,
		transactionSvc: transactionSvc,
		chainRepo:      chainRepo,
		timelockRepo:   timelockRepo,
		db:             db,
		clients:        make(map[int]*ethclient.Client),
		quit:           make(chan struct{}),
		blockNumbers:   make(map[int]*big.Int),
	}
}

// Start 启动事件监听器
func (el *EventListener) Start(ctx context.Context) error {
	logger.Info("EventListener Starting...")

	// 初始化RPC客户端
	if err := el.initializeClients(); err != nil {
		logger.Error("EventListener: failed to initialize clients", err)
		return fmt.Errorf("failed to initialize clients: %w", err)
	}

	// 启动协程管理器
	go el.runEventManager(ctx)

	logger.Info("EventListener Started successfully")
	return nil
}

// Stop 停止事件监听器
func (el *EventListener) Stop() {
	logger.Info("EventListener Stopping...")
	close(el.quit)

	el.mu.Lock()
	defer el.mu.Unlock()

	// 关闭所有客户端
	for chainID, client := range el.clients {
		if client != nil {
			client.Close()
			logger.Info("EventListener: closed client", "chain_id", chainID)
		}
	}

	logger.Info("EventListener Stopped")
}

// initializeClients 初始化RPC客户端
func (el *EventListener) initializeClients() error {
	// 从数据库获取启用RPC的链配置
	ctx := context.Background()
	chains, err := el.chainRepo.GetRPCEnabledChains(ctx, el.config.RPC.IncludeTestnets)
	if err != nil {
		logger.Error("EventListener Error: ", fmt.Errorf("failed to get RPC enabled chains: %w", err))
		return fmt.Errorf("failed to get RPC enabled chains: %w", err)
	}

	if len(chains) == 0 {
		logger.Error("EventListener Error: ", fmt.Errorf("no RPC enabled chains found in database"))
		return fmt.Errorf("no RPC enabled chains found in database")
	}

	// 遍历启用的链配置
	for _, chainInfo := range chains {
		// 获取RPC URL
		rpcURL, err := el.config.GetRPCURL(&chainInfo)
		if err != nil {
			logger.Warn("EventListener: failed to get RPC URL", "chain", chainInfo.ChainName, "error", err)
			continue
		}

		// 检查API KEY占位符
		if strings.Contains(rpcURL, "YOUR_API_KEY") || strings.Contains(rpcURL, "YOUR_ALCHEMY_API_KEY") || strings.Contains(rpcURL, "YOUR_INFURA_API_KEY") {
			logger.Warn("EventListener: RPC URL contains placeholder", "chain", chainInfo.ChainName, "url", rpcURL)
			continue
		}

		client, err := ethclient.Dial(rpcURL)
		if err != nil {
			logger.Error("EventListener: failed to connect to RPC", err, "chain", chainInfo.ChainName, "url", rpcURL)
			continue
		}

		// 测试连接
		testCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = client.ChainID(testCtx)
		cancel()

		if err != nil {
			logger.Error("EventListener: failed to get chain ID", err, "chain", chainInfo.ChainName)
			client.Close()
			continue
		}

		el.clients[chainInfo.ChainID] = client
		logger.Info("EventListener: connected to RPC", "chain", chainInfo.ChainName, "chain_id", chainInfo.ChainID, "url", rpcURL)
	}

	if len(el.clients) == 0 {
		return fmt.Errorf("no RPC clients initialized")
	}

	logger.Info("EventListener: initialized clients", "count", len(el.clients))
	return nil
}

// runEventManager 运行事件管理器
func (el *EventListener) runEventManager(ctx context.Context) {
	eventTicker := time.NewTicker(30 * time.Second)
	statusTicker := time.NewTicker(2 * time.Minute)
	cacheTicker := time.NewTicker(10 * time.Minute)

	defer eventTicker.Stop()
	defer statusTicker.Stop()
	defer cacheTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-el.quit:
			return
		case <-eventTicker.C:
			el.processAllChains(ctx)
		case <-statusTicker.C:
			el.processStatusMaintenance(ctx)
		case <-cacheTicker.C:
			el.clearCache()
		}
	}
}

// processAllChains 处理所有链的事件
func (el *EventListener) processAllChains(ctx context.Context) {
	el.mu.RLock()
	clients := make(map[int]*ethclient.Client)
	for chainID, client := range el.clients {
		clients[chainID] = client
	}
	el.mu.RUnlock()

	for chainID, client := range clients {
		if client != nil {
			if err := el.processChainEvents(ctx, chainID, client); err != nil {
				logger.Error("EventListener: failed to process chain", err, "chain_id", chainID)
			}
		}
	}
}

// processStatusMaintenance 处理状态维护
func (el *EventListener) processStatusMaintenance(ctx context.Context) {
	if err := el.transactionSvc.ProcessExpiredTransactions(ctx); err != nil {
		logger.Error("EventListener: failed to process expired transactions", err)
	}
	if err := el.transactionSvc.ProcessReadyTransactions(ctx); err != nil {
		logger.Error("EventListener: failed to process ready transactions", err)
	}
}

// clearCache 清理缓存
func (el *EventListener) clearCache() {
	// 简单的缓存清理：计数并定期清空
	count := 0
	el.txCache.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	// 如果缓存过大，清空缓存
	if count > 1000 {
		el.txCache = sync.Map{}
		logger.Info("EventListener: cleared cache", "old_count", count)
	}
}

// processChainEvents 处理链事件 - 优化版本
func (el *EventListener) processChainEvents(ctx context.Context, chainID int, client *ethclient.Client) error {
	// 获取最新区块号
	latestBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest block number: %w", err)
	}

	// 安全访问和更新区块号
	el.mu.Lock()
	fromBlock := el.blockNumbers[chainID]
	if fromBlock == nil {
		// 初始化：从最近100个区块开始
		startBlock := int64(latestBlock) - 100
		if startBlock < 0 {
			startBlock = 0
		}
		fromBlock = big.NewInt(startBlock)
		el.blockNumbers[chainID] = fromBlock
		logger.Info("EventListener: initializing from block", "chain_id", chainID, "from_block", fromBlock)
	}
	el.mu.Unlock()

	// 计算处理区块范围
	toBlock := big.NewInt(int64(latestBlock))
	if toBlock.Cmp(fromBlock) <= 0 {
		return nil // 没有新区块
	}

	// 限制单次处理区块数量，避免RPC超时
	maxBlocks := big.NewInt(500) // 减少处理量，提高稳定性
	if new(big.Int).Sub(toBlock, fromBlock).Cmp(maxBlocks) > 0 {
		toBlock = new(big.Int).Add(fromBlock, maxBlocks)
	}

	logger.Info("EventListener: processing blocks", "chain_id", chainID, "from_block", fromBlock, "to_block", toBlock)

	// 批量查询timelock事件
	allEvents, err := el.queryAllTimelockEvents(ctx, client, fromBlock, toBlock)
	if err != nil {
		logger.Error("EventListener: failed to query timelock events", err, "chain_id", chainID)
		return fmt.Errorf("failed to query timelock events: %w", err)
	}

	// 获取活跃的timelock合约
	timelocks, err := el.getActiveTimelocks(ctx, chainID)
	if err != nil {
		logger.Error("EventListener: failed to get active timelocks", err, "chain_id", chainID)
		return fmt.Errorf("failed to get active timelocks: %w", err)
	}

	if len(timelocks) == 0 {
		logger.Info("EventListener: no active timelocks configured", "chain_id", chainID)
		// 更新区块号并返回
		el.mu.Lock()
		el.blockNumbers[chainID] = new(big.Int).Add(toBlock, big.NewInt(1))
		el.mu.Unlock()
		return nil
	}

	// 筛选事件
	filteredEvents := el.filterEventsByOurContracts(allEvents, timelocks, chainID)

	// 处理事件
	if len(filteredEvents) > 0 {
		logger.Info("EventListener: processing filtered events",
			"chain_id", chainID,
			"total_events", len(allEvents),
			"filtered_events", len(filteredEvents))
		el.processEvents(ctx, filteredEvents, chainID)
	}

	// 安全更新区块号
	el.mu.Lock()
	el.blockNumbers[chainID] = new(big.Int).Add(toBlock, big.NewInt(1))
	el.mu.Unlock()

	return nil
}

// getActiveTimelocks 获取指定链上活跃的timelock合约
func (el *EventListener) getActiveTimelocks(ctx context.Context, chainID int) ([]TimelockContract, error) {
	var timelocks []TimelockContract

	// 获取Compound timelocks
	compoundTimelocks, err := el.timelockRepo.GetActiveCompoundTimelocksByChain(ctx, chainID)
	if err != nil {
		logger.Error("EventListener: failed to get compound timelocks", err, "chain_id", chainID)
		return nil, fmt.Errorf("failed to get compound timelocks: %w", err)
	}

	for _, tl := range compoundTimelocks {
		timelocks = append(timelocks, TimelockContract{
			ContractAddress: tl.ContractAddress,
			Standard:        "compound",
		})
	}

	// 获取OpenZeppelin timelocks
	ozTimelocks, err := el.timelockRepo.GetActiveOpenZeppelinTimelocksByChain(ctx, chainID)
	if err != nil {
		logger.Error("EventListener: failed to get openzeppelin timelocks", err, "chain_id", chainID)
		return nil, fmt.Errorf("failed to get openzeppelin timelocks: %w", err)
	}

	for _, tl := range ozTimelocks {
		timelocks = append(timelocks, TimelockContract{
			ContractAddress: tl.ContractAddress,
			Standard:        "openzeppelin",
		})
	}

	logger.Info("EventListener: found active timelocks", "chain_id", chainID, "count", len(timelocks))
	return timelocks, nil
}

// queryAllTimelockEvents 批量查询所有timelock事件（不限制合约地址）
func (el *EventListener) queryAllTimelockEvents(ctx context.Context, client *ethclient.Client, fromBlock, toBlock *big.Int) ([]TimelockEvent, error) {
	var events []TimelockEvent

	// 批量查询所有timelock事件签名，不限制合约地址
	allTimelockTopics := []common.Hash{
		// Compound events
		compoundQueuedEventSig,
		compoundExecutedEventSig,
		compoundCancelledEventSig,
		// OpenZeppelin events
		ozCallScheduledEventSig,
		ozCallExecutedEventSig,
		ozCallCancelledEventSig,
	}

	// 构建批量查询（不指定合约地址，查询所有合约的timelock事件）
	query := ethereum.FilterQuery{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		// Addresses: 不设置，表示查询所有合约
		Topics: [][]common.Hash{allTimelockTopics},
	}

	logger.Info("EventListener: batch querying all timelock events",
		"from_block", fromBlock,
		"to_block", toBlock,
		"event_signatures", len(allTimelockTopics))

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to batch filter timelock logs: %w", err)
	}

	logger.Info("EventListener: batch query found raw events",
		"raw_event_count", len(logs),
		"from_block", fromBlock,
		"to_block", toBlock)

	// 解析所有事件日志
	for _, log := range logs {
		event, err := el.parseEvent(log)
		if err != nil {
			logger.Error("EventListener: failed to parse event in batch query", err,
				"tx_hash", log.TxHash.Hex(),
				"contract_address", log.Address.Hex())
			continue
		}
		if event != nil {
			events = append(events, event)
		}
	}

	logger.Info("EventListener: batch parsing completed",
		"parsed_events", len(events),
		"raw_events", len(logs))

	return events, nil
}

// filterEventsByOurContracts 筛选出属于我们数据库中timelock合约的事件
func (el *EventListener) filterEventsByOurContracts(allEvents []TimelockEvent, ourContracts []TimelockContract, chainID int) []TimelockEvent {
	if len(allEvents) == 0 || len(ourContracts) == 0 {
		return []TimelockEvent{}
	}

	// 构建我们的合约地址映射表（用于快速查找）
	contractMap := make(map[string]TimelockContract)
	for _, contract := range ourContracts {
		// 标准化地址（转为小写）
		normalizedAddr := strings.ToLower(contract.ContractAddress)
		contractMap[normalizedAddr] = contract
	}

	var filteredEvents []TimelockEvent

	for _, event := range allEvents {
		// 从事件中提取合约地址
		contractAddr := el.extractContractAddressFromEvent(event)
		if contractAddr == "" {
			logger.Warn("EventListener: could not extract contract address from event",
				"event_type", fmt.Sprintf("%T", event),
				"tx_hash", event.GetTxHash())
			continue
		}

		// 标准化地址进行比较
		normalizedAddr := strings.ToLower(contractAddr)

		// 检查是否是我们关心的合约
		if contract, exists := contractMap[normalizedAddr]; exists {
			logger.Info("EventListener: event matches our timelock contract",
				"contract_address", contractAddr,
				"standard", contract.Standard,
				"tx_hash", event.GetTxHash(),
				"event_type", fmt.Sprintf("%T", event))
			filteredEvents = append(filteredEvents, event)
		} else {
			logger.Info("EventListener: event from unknown contract, skipping",
				"contract_address", contractAddr,
				"tx_hash", event.GetTxHash())
		}
	}

	logger.Info("EventListener: contract filtering completed",
		"chain_id", chainID,
		"total_events", len(allEvents),
		"our_contracts", len(ourContracts),
		"filtered_events", len(filteredEvents))

	return filteredEvents
}

// extractContractAddressFromEvent 从事件中提取合约地址
func (el *EventListener) extractContractAddressFromEvent(event TimelockEvent) string {
	// 现在事件结构中直接包含了合约地址，可以直接提取
	switch e := event.(type) {
	case *CompoundQueuedEvent:
		return e.ContractAddress
	case *CompoundExecutedEvent:
		return e.ContractAddress
	case *CompoundCancelledEvent:
		return e.ContractAddress
	case *OpenZeppelinCallScheduledEvent:
		return e.ContractAddress
	case *OpenZeppelinCallExecutedEvent:
		return e.ContractAddress
	case *OpenZeppelinCallCancelledEvent:
		return e.ContractAddress
	default:
		logger.Warn("EventListener: unknown event type for address extraction",
			"event_type", fmt.Sprintf("%T", e))
		return ""
	}
}

// parseEvent 解析事件日志
func (el *EventListener) parseEvent(log ethTypes.Log) (TimelockEvent, error) {
	if len(log.Topics) == 0 {
		logger.Error("EventListener Error: ", fmt.Errorf("no topics in log"))
		return nil, fmt.Errorf("no topics in log")
	}

	eventSig := log.Topics[0]
	blockNumber := big.NewInt(int64(log.BlockNumber))
	txHash := log.TxHash.Hex()
	contractAddress := log.Address.Hex() // 从日志中获取合约地址

	switch eventSig {
	case compoundQueuedEventSig:
		// 解析Compound QueueTransaction事件
		// QueueTransaction(bytes32 indexed txHash, address indexed target, uint value, string signature, bytes data, uint eta)
		if len(log.Topics) < 3 {
			logger.Error("EventListener Error: ", fmt.Errorf("insufficient topics for compound queued event"))
			return nil, fmt.Errorf("insufficient topics for compound queued event")
		}

		// 使用ABI解析Data字段中的非indexed参数
		value, signature, data, eta, err := el.parseCompoundEventData(log, compoundQueuedABI)
		if err != nil {
			logger.Error("EventListener: failed to parse compound queued event data", err, "tx_hash", txHash)
			// 使用默认值继续处理，确保系统稳定性
			value = big.NewInt(0)
			signature = ""
			data = []byte{}
			eta = big.NewInt(0)
		}

		return &CompoundQueuedEvent{
			TxHash:          txHash,
			ContractAddress: contractAddress,
			Target:          common.HexToAddress(log.Topics[2].Hex()),
			Value:           value,
			Signature:       signature,
			Data:            data,
			ETA:             eta,
			BlockNumber:     blockNumber,
		}, nil

	case compoundExecutedEventSig:
		// 解析Compound ExecuteTransaction事件
		// ExecuteTransaction(bytes32 indexed txHash, address indexed target, uint value, string signature, bytes data, uint eta)
		if len(log.Topics) < 3 {
			logger.Error("EventListener Error: ", fmt.Errorf("insufficient topics for compound executed event"))
			return nil, fmt.Errorf("insufficient topics for compound executed event")
		}

		// 使用ABI解析Data字段中的非indexed参数
		value, signature, data, eta, err := el.parseCompoundEventData(log, compoundExecutedABI)
		if err != nil {
			logger.Error("EventListener: failed to parse compound executed event data", err, "tx_hash", txHash)
			// 使用默认值继续处理
			value = big.NewInt(0)
			signature = ""
			data = []byte{}
			eta = big.NewInt(0)
		}

		return &CompoundExecutedEvent{
			TxHash:          txHash,
			ContractAddress: contractAddress,
			Target:          common.HexToAddress(log.Topics[2].Hex()),
			Value:           value,
			Signature:       signature,
			Data:            data,
			ETA:             eta,
			BlockNumber:     blockNumber,
		}, nil

	case compoundCancelledEventSig:
		// 解析Compound CancelTransaction事件
		// CancelTransaction(bytes32 indexed txHash, address indexed target, uint value, string signature, bytes data, uint eta)
		if len(log.Topics) < 3 {
			logger.Error("EventListener Error: ", fmt.Errorf("insufficient topics for compound cancelled event"))
			return nil, fmt.Errorf("insufficient topics for compound cancelled event")
		}

		// 使用ABI解析Data字段中的非indexed参数
		value, signature, data, eta, err := el.parseCompoundEventData(log, compoundCancelledABI)
		if err != nil {
			logger.Error("EventListener: failed to parse compound cancelled event data", err, "tx_hash", txHash)
			// 使用默认值继续处理
			value = big.NewInt(0)
			signature = ""
			data = []byte{}
			eta = big.NewInt(0)
		}

		return &CompoundCancelledEvent{
			TxHash:          txHash,
			ContractAddress: contractAddress,
			Target:          common.HexToAddress(log.Topics[2].Hex()),
			Value:           value,
			Signature:       signature,
			Data:            data,
			ETA:             eta,
			BlockNumber:     blockNumber,
		}, nil

	case ozCallScheduledEventSig:
		// 解析OpenZeppelin CallScheduled事件
		// CallScheduled(bytes32 indexed id, uint256 indexed index, address target, uint256 value, bytes data, bytes32 predecessor, uint256 delay)
		if len(log.Topics) < 3 {
			logger.Error("EventListener Error: ", fmt.Errorf("insufficient topics for openzeppelin scheduled event"))
			return nil, fmt.Errorf("insufficient topics for openzeppelin scheduled event")
		}

		// 使用ABI解析Data字段中的非indexed参数
		target, value, data, predecessor, delay, err := el.parseOpenZeppelinEventData(log, ozCallScheduledABI)
		if err != nil {
			logger.Error("EventListener: failed to parse openzeppelin scheduled event data", err, "tx_hash", txHash)
			// 使用默认值继续处理
			target = common.Address{}
			value = big.NewInt(0)
			data = []byte{}
			predecessor = [32]byte{}
			delay = big.NewInt(0)
		}

		return &OpenZeppelinCallScheduledEvent{
			TxHash:          txHash,
			ContractAddress: contractAddress,
			ID:              log.Topics[1],                           // indexed id
			Index:           new(big.Int).SetBytes(log.Topics[2][:]), // indexed index
			Target:          target,
			Value:           value,
			Data:            data,
			Predecessor:     predecessor,
			Delay:           delay,
			BlockNumber:     blockNumber,
		}, nil

	case ozCallExecutedEventSig:
		// 解析OpenZeppelin CallExecuted事件
		// CallExecuted(bytes32 indexed id, uint256 indexed index, address target, uint256 value, bytes data)
		if len(log.Topics) < 3 {
			logger.Error("EventListener Error: ", fmt.Errorf("insufficient topics for openzeppelin executed event"))
			return nil, fmt.Errorf("insufficient topics for openzeppelin executed event")
		}

		// 使用ABI解析Data字段中的非indexed参数
		target, value, data, _, _, err := el.parseOpenZeppelinEventData(log, ozCallExecutedABI)
		if err != nil {
			logger.Error("EventListener: failed to parse openzeppelin executed event data", err, "tx_hash", txHash)
			// 使用默认值继续处理
			target = common.Address{}
			value = big.NewInt(0)
			data = []byte{}
		}

		return &OpenZeppelinCallExecutedEvent{
			TxHash:          txHash,
			ContractAddress: contractAddress,
			ID:              log.Topics[1],                           // indexed id
			Index:           new(big.Int).SetBytes(log.Topics[2][:]), // indexed index
			Target:          target,
			Value:           value,
			Data:            data,
			BlockNumber:     blockNumber,
		}, nil

	case ozCallCancelledEventSig:
		// 解析OpenZeppelin Cancelled事件
		// Cancelled(bytes32 indexed id)
		if len(log.Topics) < 2 {
			logger.Error("EventListener Error: ", fmt.Errorf("insufficient topics for openzeppelin cancelled event"))
			return nil, fmt.Errorf("insufficient topics for openzeppelin cancelled event")
		}

		// OpenZeppelin Cancelled事件只有indexed参数，没有data字段需要解析
		return &OpenZeppelinCallCancelledEvent{
			TxHash:          txHash,
			ContractAddress: contractAddress,
			ID:              log.Topics[1], // indexed id
			BlockNumber:     blockNumber,
		}, nil

	default:
		logger.Error("EventListener Error: ", fmt.Errorf("unknown event signature: %s", eventSig.Hex()))
		return nil, fmt.Errorf("unknown event signature: %s", eventSig.Hex())
	}
}

// processEvents 处理事件列表 - 简化版本
func (el *EventListener) processEvents(ctx context.Context, events []TimelockEvent, chainID int) {
	for _, event := range events {
		blockNumber := event.GetBlockNumber().Int64()
		eventTxHash := event.GetTxHash()
		newStatus := event.GetStatus()
		contractAddr := event.GetContractAddress()

		logger.Info("EventListener: processing event",
			"event_tx_hash", eventTxHash,
			"new_status", newStatus,
			"block_number", blockNumber,
			"contract_addr", contractAddr)

		// 简化的交易匹配逻辑
		var dbTxHash string

		switch event.(type) {
		case *CompoundQueuedEvent, *CompoundExecutedEvent, *CompoundCancelledEvent:
			// Compound事件：通过合约地址和参数匹配
			dbTxHash = el.findCompoundTransaction(ctx, event, chainID)
		case *OpenZeppelinCallScheduledEvent:
			// OpenZeppelin事件：直接使用事件哈希
			dbTxHash = eventTxHash
		case *OpenZeppelinCallExecutedEvent, *OpenZeppelinCallCancelledEvent:
			// OpenZeppelin事件：通过operation_id匹配
			dbTxHash = el.findOpenZeppelinTransaction(ctx, event)
		default:
			logger.Error("EventListener: unknown event type", nil, "event_tx_hash", eventTxHash)
			continue
		}

		if dbTxHash == "" {
			logger.Warn("EventListener: no matching transaction found", "event_tx_hash", eventTxHash, "new_status", newStatus)
			continue
		}

		// 更新交易状态
		err := el.transactionSvc.UpdateTransactionStatusByTxHash(ctx, dbTxHash, newStatus, &blockNumber)
		if err != nil {
			logger.Error("EventListener: failed to update transaction status", err,
				"db_tx_hash", dbTxHash,
				"event_tx_hash", eventTxHash,
				"status", newStatus)
		} else {
			logger.Info("EventListener: updated transaction status",
				"db_tx_hash", dbTxHash,
				"event_tx_hash", eventTxHash,
				"status", newStatus,
				"block_number", blockNumber)
		}
	}
}

// findCompoundTransaction 查找Compound交易 - 简化版本
func (el *EventListener) findCompoundTransaction(ctx context.Context, event TimelockEvent, chainID int) string {
	eventTxHash := event.GetTxHash()
	contractAddr := event.GetContractAddress()

	// 优先从缓存查找
	if cachedTx := el.getCachedTransaction(eventTxHash); cachedTx != nil {
		return cachedTx.TxHash
	}

	// 直接数据库查询
	if dbTx := el.findTransactionByDirectHash(ctx, eventTxHash); dbTx != nil {
		el.cacheTransaction(eventTxHash, dbTx)
		return dbTx.TxHash
	}

	// 通过参数匹配
	var target, value, signature string
	var eta int64

	switch e := event.(type) {
	case *CompoundQueuedEvent:
		target = e.Target.Hex()
		value = e.Value.String()
		signature = e.Signature
		eta = e.ETA.Int64()
	case *CompoundExecutedEvent:
		target = e.Target.Hex()
		value = e.Value.String()
		signature = e.Signature
		eta = e.ETA.Int64()
	case *CompoundCancelledEvent:
		target = e.Target.Hex()
		value = e.Value.String()
		signature = e.Signature
		eta = e.ETA.Int64()
	}

	// 通过service查找
	tx, err := el.transactionSvc.FindTransactionByCompoundParams(ctx, chainID, contractAddr, target, value, signature, eta)
	if err == nil && tx != nil {
		el.cacheTransaction(eventTxHash, tx)
		return tx.TxHash
	}

	// 降级方案：返回事件哈希
	logger.Info("EventListener: using event hash as fallback", "event_tx_hash", eventTxHash)
	return eventTxHash
}

// findOpenZeppelinTransaction 查找OpenZeppelin交易 - 简化版本
func (el *EventListener) findOpenZeppelinTransaction(ctx context.Context, event TimelockEvent) string {
	eventTxHash := event.GetTxHash()

	// 提取operation_id
	var operationID string
	switch e := event.(type) {
	case *OpenZeppelinCallExecutedEvent:
		operationID = fmt.Sprintf("0x%x", e.ID)
	case *OpenZeppelinCallCancelledEvent:
		operationID = fmt.Sprintf("0x%x", e.ID)
	default:
		return eventTxHash
	}

	// 通过operation_id查找
	tx, err := el.transactionSvc.FindTransactionByOperationID(ctx, operationID)
	if err == nil && tx != nil {
		return tx.TxHash
	}

	// 降级方案：返回事件哈希
	logger.Info("EventListener: using event hash as fallback for OpenZeppelin", "event_tx_hash", eventTxHash, "operation_id", operationID)
	return eventTxHash
}

// findTransactionByDirectHash 直接通过交易哈希查找
func (el *EventListener) findTransactionByDirectHash(ctx context.Context, txHash string) *types.Transaction {
	var transaction types.Transaction
	err := el.db.WithContext(ctx).Where("tx_hash = ?", txHash).First(&transaction).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logger.Error("EventListener: database error while finding transaction by hash", err, "tx_hash", txHash)
		}
		return nil
	}
	return &transaction
}

// getCachedTransaction 从缓存获取交易
func (el *EventListener) getCachedTransaction(txHash string) *types.Transaction {
	if cached, ok := el.txCache.Load(txHash); ok {
		if tx, ok := cached.(*types.Transaction); ok {
			return tx
		}
	}
	return nil
}

// cacheTransaction 缓存交易
func (el *EventListener) cacheTransaction(txHash string, tx *types.Transaction) {
	el.txCache.Store(txHash, tx)
}

// parseCompoundEventData 解析Compound事件的Data字段
func (el *EventListener) parseCompoundEventData(log ethTypes.Log, abiString string) (*big.Int, string, []byte, *big.Int, error) {
	// 解析ABI
	contractABI, err := abi.JSON(strings.NewReader(abiString))
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// 找到对应的事件
	var eventABI abi.Event
	for _, event := range contractABI.Events {
		if event.ID == log.Topics[0] {
			eventABI = event
			break
		}
	}

	if eventABI.Name == "" {
		return nil, "", nil, nil, fmt.Errorf("event not found in ABI")
	}

	// 解析Data字段中的非indexed参数
	dataMap := make(map[string]interface{})
	err = contractABI.UnpackIntoMap(dataMap, eventABI.Name, log.Data)
	if err != nil {
		// 如果解析失败，返回默认值而不是错误，确保系统继续运行
		logger.Warn("EventListener: failed to unpack event data, using defaults", "error", err)
		return big.NewInt(0), "", []byte{}, big.NewInt(0), nil
	}

	// 提取各个参数
	var value *big.Int
	var signature string
	var data []byte
	var eta *big.Int

	if v, ok := dataMap["value"]; ok {
		if bigInt, ok := v.(*big.Int); ok {
			value = bigInt
		}
	}
	if value == nil {
		value = big.NewInt(0)
	}

	if s, ok := dataMap["signature"]; ok {
		if str, ok := s.(string); ok {
			signature = str
		}
	}

	if d, ok := dataMap["data"]; ok {
		if bytes, ok := d.([]byte); ok {
			data = bytes
		}
	}
	if data == nil {
		data = []byte{}
	}

	if e, ok := dataMap["eta"]; ok {
		if bigInt, ok := e.(*big.Int); ok {
			eta = bigInt
		}
	}
	if eta == nil {
		eta = big.NewInt(0)
	}

	return value, signature, data, eta, nil
}

// parseOpenZeppelinEventData 解析OpenZeppelin事件的Data字段
func (el *EventListener) parseOpenZeppelinEventData(log ethTypes.Log, abiString string) (common.Address, *big.Int, []byte, [32]byte, *big.Int, error) {
	// 解析ABI
	contractABI, err := abi.JSON(strings.NewReader(abiString))
	if err != nil {
		return common.Address{}, nil, nil, [32]byte{}, nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// 找到对应的事件
	var eventABI abi.Event
	for _, event := range contractABI.Events {
		if event.ID == log.Topics[0] {
			eventABI = event
			break
		}
	}

	if eventABI.Name == "" {
		return common.Address{}, nil, nil, [32]byte{}, nil, fmt.Errorf("event not found in ABI")
	}

	// 解析Data字段中的非indexed参数
	dataMap := make(map[string]interface{})
	err = contractABI.UnpackIntoMap(dataMap, eventABI.Name, log.Data)
	if err != nil {
		// 如果解析失败，返回默认值
		logger.Warn("EventListener: failed to unpack OpenZeppelin event data, using defaults", "error", err)
		return common.Address{}, big.NewInt(0), []byte{}, [32]byte{}, big.NewInt(0), nil
	}

	// 提取各个参数
	var target common.Address
	var value *big.Int
	var data []byte
	var predecessor [32]byte
	var delay *big.Int

	if t, ok := dataMap["target"]; ok {
		if addr, ok := t.(common.Address); ok {
			target = addr
		}
	}

	if v, ok := dataMap["value"]; ok {
		if bigInt, ok := v.(*big.Int); ok {
			value = bigInt
		}
	}
	if value == nil {
		value = big.NewInt(0)
	}

	if d, ok := dataMap["data"]; ok {
		if bytes, ok := d.([]byte); ok {
			data = bytes
		}
	}
	if data == nil {
		data = []byte{}
	}

	if p, ok := dataMap["predecessor"]; ok {
		if bytes32, ok := p.([32]byte); ok {
			predecessor = bytes32
		}
	}

	if del, ok := dataMap["delay"]; ok {
		if bigInt, ok := del.(*big.Int); ok {
			delay = bigInt
		}
	}
	if delay == nil {
		delay = big.NewInt(0)
	}

	return target, value, data, predecessor, delay, nil
}
