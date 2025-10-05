package processor

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// L2Processor L2通用处理器（支持Arbitrum、Optimism、Base等L2链）
type L2Processor struct {
	*EthereumProcessor
	chainConfigs map[int]*L2ChainConfig
}

// L2ChainConfig L2链配置
type L2ChainConfig struct {
	ChainID          int
	ChainName        string
	BlockTime        time.Duration // 平均出块时间
	MaxRetries       int           // 最大重试次数
	TimeoutPerCall   time.Duration // 单次调用超时
	SupportedSigners []*big.Int    // 支持的签名器链ID
}

// NewL2Processor 创建L2处理器
func NewL2Processor() *L2Processor {
	l2p := &L2Processor{
		EthereumProcessor: NewEthereumProcessor(),
		chainConfigs:      make(map[int]*L2ChainConfig),
	}

	// 初始化L2链配置
	l2p.initL2ChainConfigs()

	return l2p
}

// NewL2ProcessorWithRPCManager 创建带RPC管理器的L2处理器
func NewL2ProcessorWithRPCManager(rpcManager RPCManager) *L2Processor {
	l2p := &L2Processor{
		EthereumProcessor: NewEthereumProcessorWithRPCManager(rpcManager),
		chainConfigs:      make(map[int]*L2ChainConfig),
	}

	// 初始化L2链配置
	l2p.initL2ChainConfigs()

	return l2p
}

// initL2ChainConfigs 初始化L2链配置
func (l2p *L2Processor) initL2ChainConfigs() {
	// Arbitrum 配置
	l2p.chainConfigs[42161] = &L2ChainConfig{
		ChainID:        42161,
		ChainName:      "arbitrum-mainnet",
		BlockTime:      250 * time.Millisecond, // ~0.25秒
		MaxRetries:     5,
		TimeoutPerCall: 10 * time.Second,
		SupportedSigners: []*big.Int{
			big.NewInt(42161), // arbitrum-mainnet
		},
	}

	// Optimism 配置
	l2p.chainConfigs[10] = &L2ChainConfig{
		ChainID:        10,
		ChainName:      "optimism-mainnet",
		BlockTime:      2 * time.Second, // ~2秒
		MaxRetries:     5,
		TimeoutPerCall: 10 * time.Second,
		SupportedSigners: []*big.Int{
			big.NewInt(10), // optimism-mainnet
		},
	}

	// Base 配置
	l2p.chainConfigs[8453] = &L2ChainConfig{
		ChainID:        8453,
		ChainName:      "base-mainnet",
		BlockTime:      2 * time.Second, // ~2秒
		MaxRetries:     5,
		TimeoutPerCall: 10 * time.Second,
		SupportedSigners: []*big.Int{
			big.NewInt(8453), // base-mainnet
		},
	}

	// mode,optimism派生链
	l2p.chainConfigs[34443] = &L2ChainConfig{
		ChainID:        34443,
		ChainName:      "mode-mainnet",
		BlockTime:      2 * time.Second, // ~2秒
		MaxRetries:     5,
		TimeoutPerCall: 10 * time.Second,
	}

	// zklink
	l2p.chainConfigs[810180] = &L2ChainConfig{
		ChainID:        810180,
		ChainName:      "zklink-mainnet",
		BlockTime:      2 * time.Second, // ~2秒
		MaxRetries:     5,
		TimeoutPerCall: 10 * time.Second,
	}

	// add more L2 chains here
}

// ProcessLog 处理日志事件（L2特殊处理）
func (l2p *L2Processor) ProcessLog(ctx context.Context, client *ethclient.Client, log *ethtypes.Log) (types.TimelockEvent, error) {
	if len(log.Topics) == 0 {
		return nil, fmt.Errorf("log has no topics")
	}

	eventSignature := log.Topics[0]

	// 先尝试解析事件（仅使用log数据）
	var event types.TimelockEvent

	// 检查是否是Compound事件
	if compoundEvent := l2p.parseCompoundEvent(log, eventSignature); compoundEvent != nil {
		event = compoundEvent
	} else if ozEvent := l2p.parseOpenZeppelinEvent(log, eventSignature); ozEvent != nil {
		// 检查是否是OpenZeppelin事件
		event = ozEvent
	} else {
		return nil, fmt.Errorf("unknown event signature: %s", eventSignature.Hex())
	}

	// L2特殊处理：时间戳必须成功，其他信息尽力获取
	l2p.tryEnrichEventDataWithL2Strategy(ctx, client, log, event)

	return event, nil
}

// tryEnrichEventDataWithL2Strategy L2数据丰富策略（简化重试逻辑）
func (l2p *L2Processor) tryEnrichEventDataWithL2Strategy(ctx context.Context, client *ethclient.Client, log *ethtypes.Log, event types.TimelockEvent) {
	// 获取链配置
	chainConfig := l2p.getChainConfigByChainID(l2p.chainID)

	// 设置链信息
	l2p.setChainInfo(event, chainConfig)

	// 获取区块时间戳（带重试）
	if err := l2p.getBlockTimestampWithRetry(ctx, client, log, event, chainConfig); err != nil {
		logger.Error("Failed to get block timestamp", err,
			"chain_id", l2p.chainID, "chain_name", chainConfig.ChainName,
			"tx_hash", log.TxHash.Hex(), "block_number", log.BlockNumber)
		return
	}

	// 获取交易信息（带重试，失败不影响主流程）
	if err := l2p.getTransactionInfoWithRetry(ctx, client, log, event, chainConfig); err != nil {
		logger.Warn("Failed to get transaction info (non-critical)",
			"chain_id", l2p.chainID, "chain_name", chainConfig.ChainName,
			"tx_hash", log.TxHash.Hex(), "error", err)
	}
}

// getChainConfigByChainID 根据chainID获取链配置
func (l2p *L2Processor) getChainConfigByChainID(chainID int) *L2ChainConfig {
	if config, exists := l2p.chainConfigs[chainID]; exists {
		return config
	}

	// 返回默认配置
	return &L2ChainConfig{
		ChainID:          chainID,
		ChainName:        fmt.Sprintf("Chain-%d", chainID),
		BlockTime:        2 * time.Second,
		MaxRetries:       2,
		TimeoutPerCall:   5 * time.Second,
		SupportedSigners: []*big.Int{big.NewInt(int64(chainID))},
	}
}

// setChainInfo 设置默认事件值
func (l2p *L2Processor) setChainInfo(event types.TimelockEvent, config *L2ChainConfig) {

	switch e := event.(type) {
	case *types.CompoundTimelockEvent:
		// 设置链信息
		e.ChainID = config.ChainID
		e.ChainName = config.ChainName
	case *types.OpenZeppelinTimelockEvent:
		// 设置链信息
		e.ChainID = config.ChainID
		e.ChainName = config.ChainName
	}
}

// getBlockTimestampWithRetry 获取区块时间戳（带简单重试）
func (l2p *L2Processor) getBlockTimestampWithRetry(ctx context.Context, client *ethclient.Client, log *ethtypes.Log, event types.TimelockEvent, config *L2ChainConfig) error {
	var header *ethtypes.Header
	var lastErr error

	// 简单重试5次
	for attempt := 0; attempt < 5; attempt++ {
		// 创建带超时的上下文
		timeoutCtx, cancel := context.WithTimeout(ctx, config.TimeoutPerCall)
		var err error
		header, err = client.HeaderByNumber(timeoutCtx, big.NewInt(int64(log.BlockNumber)))
		cancel()

		if err == nil {
			break
		}
		lastErr = err
		if attempt < 4 {
			logger.Debug("Retrying HeaderByNumber (L2)", "attempt", attempt+1, "error", err)
		}
	}

	if lastErr != nil {
		return fmt.Errorf("failed to get block header after retries: %w", lastErr)
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

// getTransactionInfoWithRetry 获取交易信息（带简单重试，失败不影响主流程）
func (l2p *L2Processor) getTransactionInfoWithRetry(ctx context.Context, client *ethclient.Client, log *ethtypes.Log, event types.TimelockEvent, config *L2ChainConfig) error {
	// 获取交易信息（带重试）
	var tx *ethtypes.Transaction
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, config.TimeoutPerCall)
		var err error
		tx, _, err = client.TransactionByHash(timeoutCtx, log.TxHash)
		cancel()

		if err == nil {
			break
		}
		lastErr = err
		if attempt < 2 {
			logger.Debug("Retrying TransactionByHash (L2)", "attempt", attempt+1, "error", err)
		}
	}
	if lastErr != nil {
		return fmt.Errorf("failed to get transaction after retries: %w", lastErr)
	}

	// 获取交易回执（带重试）
	var receipt *ethtypes.Receipt
	for attempt := 0; attempt < 3; attempt++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, config.TimeoutPerCall)
		var err error
		receipt, err = client.TransactionReceipt(timeoutCtx, log.TxHash)
		cancel()

		if err == nil {
			break
		}
		lastErr = err
		if attempt < 2 {
			logger.Debug("Retrying TransactionReceipt (L2)", "attempt", attempt+1, "error", err)
		}
	}
	if lastErr != nil {
		return fmt.Errorf("failed to get transaction receipt after retries: %w", lastErr)
	}

	// 获取发送者地址（L2特殊处理）
	sender, err := l2p.getTransactionSenderL2(tx, config)
	if err != nil {
		return fmt.Errorf("failed to get transaction sender: %w", err)
	}

	// 设置交易状态
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

// getTransactionSenderL2 获取交易发送者地址（L2特殊处理）
func (l2p *L2Processor) getTransactionSenderL2(tx *ethtypes.Transaction, config *L2ChainConfig) (string, error) {
	// L2可能有特殊的交易类型，需要特殊处理
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Recovered from panic in getTransactionSenderL2", errors.New(r.(string)))
		}
	}()

	// 尝试使用配置中的签名器
	for _, chainID := range config.SupportedSigners {
		signer := ethtypes.LatestSignerForChainID(chainID)
		if sender, err := ethtypes.Sender(signer, tx); err == nil {
			return sender.Hex(), nil
		}
	}

	// 如果所有方法都失败，尝试使用London signer
	if sender, err := ethtypes.Sender(ethtypes.NewLondonSigner(big.NewInt(int64(config.ChainID))), tx); err == nil {
		return sender.Hex(), nil
	}

	return "", fmt.Errorf("failed to recover sender for %s transaction", config.ChainName)
}
