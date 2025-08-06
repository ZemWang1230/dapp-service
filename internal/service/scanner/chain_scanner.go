package scanner

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/scanner"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/ethereum/go-ethereum/ethclient"
)

// ChainScanner 单链扫描器
type ChainScanner struct {
	config       *config.Config
	chainInfo    *types.ChainRPCInfo
	progress     *types.BlockScanProgress
	rpcManager   *RPCManager
	progressRepo scanner.ProgressRepository
	txRepo       scanner.TransactionRepository
	flowRepo     scanner.FlowRepository
	relationRepo scanner.RelationRepository

	blockProcessor *BlockProcessor
	eventProcessor *EventProcessor

	mutex      sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
	isRunning  bool
	lastUpdate time.Time
}

// ChainScannerStatus 链扫描器状态
type ChainScannerStatus struct {
	ChainID            int       `json:"chain_id"`
	ChainName          string    `json:"chain_name"`
	ScanStatus         string    `json:"scan_status"`
	LastScannedBlock   int64     `json:"last_scanned_block"`
	LatestNetworkBlock int64     `json:"latest_network_block"`
	BlocksLag          int64     `json:"blocks_lag"`
	ScanSpeed          string    `json:"scan_speed"`
	LastUpdate         time.Time `json:"last_update"`
	ErrorMessage       *string   `json:"error_message,omitempty"`
}

// NewChainScanner 创建新的链扫描器
func NewChainScanner(
	cfg *config.Config,
	chainInfo *types.ChainRPCInfo,
	progress *types.BlockScanProgress,
	rpcManager *RPCManager,
	progressRepo scanner.ProgressRepository,
	txRepo scanner.TransactionRepository,
	flowRepo scanner.FlowRepository,
	relationRepo scanner.RelationRepository,
) *ChainScanner {
	cs := &ChainScanner{
		config:       cfg,
		chainInfo:    chainInfo,
		progress:     progress,
		rpcManager:   rpcManager,
		progressRepo: progressRepo,
		txRepo:       txRepo,
		flowRepo:     flowRepo,
		relationRepo: relationRepo,
		stopCh:       make(chan struct{}),
		lastUpdate:   time.Now(),
	}

	// 创建处理器
	cs.blockProcessor = NewBlockProcessor(cfg, cs.chainInfo)
	cs.eventProcessor = NewEventProcessor(cfg, txRepo, flowRepo, relationRepo)

	return cs
}

// Start 启动链扫描器
func (cs *ChainScanner) Start(ctx context.Context) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if cs.isRunning {
		return fmt.Errorf("chain scanner for chain %d is already running", cs.chainInfo.ChainID)
	}

	// 启动扫描协程
	cs.wg.Add(1)
	go cs.scanLoop(ctx)

	cs.isRunning = true
	return nil
}

// Stop 停止链扫描器
func (cs *ChainScanner) Stop() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if !cs.isRunning {
		return
	}

	// 发送停止信号
	close(cs.stopCh)

	// 等待协程结束
	cs.wg.Wait()

	cs.isRunning = false
	logger.Info("Chain scanner stopped", "chain_id", cs.chainInfo.ChainID)
}

// scanLoop 扫描循环
func (cs *ChainScanner) scanLoop(ctx context.Context) {
	defer cs.wg.Done()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Scan loop stopped by context", "chain_id", cs.chainInfo.ChainID)
			return
		case <-cs.stopCh:
			logger.Info("Scan loop stopped by stop channel", "chain_id", cs.chainInfo.ChainID)
			return
		default:
			if err := cs.scanBlocks(ctx); err != nil {
				logger.Error("Scan blocks failed", err, "chain_id", cs.chainInfo.ChainID)
				cs.updateProgressStatus("error", err.Error())

				// 发生错误时等待一段时间再重试
				select {
				case <-time.After(time.Second * 30):
				case <-cs.stopCh:
					return
				case <-ctx.Done():
					return
				}
			} else {
				// 根据是否跟上最新区块调整扫描间隔
				interval := cs.getScanInterval()
				select {
				case <-time.After(interval):
				case <-cs.stopCh:
					return
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// scanBlocks 扫描区块
func (cs *ChainScanner) scanBlocks(ctx context.Context) error {
	// 获取RPC客户端
	client, err := cs.rpcManager.GetOrCreateClient(ctx, cs.chainInfo.ChainID)
	if err != nil {
		return fmt.Errorf("failed to get RPC client: %w", err)
	}

	// 获取最新网络区块号
	latestBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest block number: %w", err)
	}

	// 更新最新网络区块号
	cs.progress.LatestNetworkBlock = int64(latestBlock)

	// 计算需要扫描的区块范围
	fromBlock := cs.progress.LastScannedBlock + 1
	toBlock := cs.calculateToBlock(fromBlock, int64(latestBlock))

	if fromBlock > int64(latestBlock) {
		logger.Debug("No new blocks to scan", "chain_id", cs.chainInfo.ChainID, "latest", latestBlock)
		return nil
	}

	// 批量扫描区块
	for currentBlock := fromBlock; currentBlock <= toBlock; currentBlock++ {
		select {
		case <-cs.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := cs.scanSingleBlock(ctx, client, currentBlock); err != nil {
				return fmt.Errorf("failed to scan block %d: %w", currentBlock, err)
			}

			// 更新进度（但不要太频繁）
			if currentBlock%10 == 0 || currentBlock == toBlock {
				cs.progress.LastScannedBlock = currentBlock
				cs.progress.LastUpdateTime = time.Now()
				cs.lastUpdate = time.Now()

				if err := cs.progressRepo.UpdateProgressBlock(ctx, cs.chainInfo.ChainID, currentBlock, int64(latestBlock)); err != nil {
					logger.Error("Failed to update progress", err, "chain_id", cs.chainInfo.ChainID, "block", currentBlock)
				}
			}
		}
	}

	return nil
}

// scanSingleBlock 扫描单个区块
func (cs *ChainScanner) scanSingleBlock(ctx context.Context, client *ethclient.Client, blockNumber int64) error {
	// 获取区块数据
	block, err := cs.blockProcessor.GetBlockData(ctx, client, big.NewInt(blockNumber))
	if err != nil {
		return fmt.Errorf("failed to get block data: %w", err)
	}

	if block == nil {
		logger.Debug("Block not found", "chain_id", cs.chainInfo.ChainID, "block", blockNumber)
		return nil
	}

	// 处理区块中的交易
	events, err := cs.blockProcessor.ProcessBlock(ctx, client, block)
	if err != nil {
		return fmt.Errorf("failed to process block: %w", err)
	}

	if len(events) > 0 {
		logger.Info("Found timelock events", "chain_id", cs.chainInfo.ChainID, "block", blockNumber, "events_count", len(events))

		// 处理事件
		if err := cs.eventProcessor.ProcessEvents(ctx, cs.chainInfo.ChainID, cs.chainInfo.ChainName, events); err != nil {
			return fmt.Errorf("failed to process events: %w", err)
		}
	}

	return nil
}

// calculateToBlock 计算要扫描到的区块号
func (cs *ChainScanner) calculateToBlock(fromBlock, latestBlock int64) int64 {
	batchSize := int64(cs.config.Scanner.ScanBatchSize)

	// 计算批次结束区块
	toBlock := fromBlock + batchSize - 1

	// 不超过最新区块
	if toBlock > latestBlock {
		toBlock = latestBlock
	}

	// 不超过确认区块数
	confirmations := int64(cs.config.Scanner.ScanConfirmations)
	if toBlock > latestBlock-confirmations {
		toBlock = latestBlock - confirmations
		if toBlock < fromBlock {
			toBlock = fromBlock
		}
	}

	return toBlock
}

// getScanInterval 获取扫描间隔
func (cs *ChainScanner) getScanInterval() time.Duration {
	// 计算落后的区块数
	lag := cs.progress.LatestNetworkBlock - cs.progress.LastScannedBlock

	if lag > 100 {
		// 落后超过100个区块，使用快速扫描
		return cs.config.Scanner.ScanInterval
	} else {
		// 接近最新区块，使用慢速扫描
		return cs.config.Scanner.ScanIntervalSlow
	}
}

// updateProgressStatus 更新进度状态
func (cs *ChainScanner) updateProgressStatus(status string, errorMsg string) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.progress.ScanStatus = status
	if errorMsg != "" {
		cs.progress.ErrorMessage = &errorMsg
	} else {
		cs.progress.ErrorMessage = nil
	}
	cs.progress.LastUpdateTime = time.Now()

	// 异步更新数据库
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		if err := cs.progressRepo.UpdateProgress(ctx, cs.progress); err != nil {
			logger.Error("Failed to update progress status", err, "chain_id", cs.chainInfo.ChainID)
		}
	}()
}

// GetStatus 获取扫描器状态
func (cs *ChainScanner) GetStatus() ChainScannerStatus {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	lag := cs.progress.LatestNetworkBlock - cs.progress.LastScannedBlock
	scanSpeed := "slow"
	if lag > 100 {
		scanSpeed = "fast"
	}

	status := ChainScannerStatus{
		ChainID:            cs.chainInfo.ChainID,
		ChainName:          cs.chainInfo.ChainName,
		ScanStatus:         cs.progress.ScanStatus,
		LastScannedBlock:   cs.progress.LastScannedBlock,
		LatestNetworkBlock: cs.progress.LatestNetworkBlock,
		BlocksLag:          lag,
		ScanSpeed:          scanSpeed,
		LastUpdate:         cs.lastUpdate,
	}

	if cs.progress.ErrorMessage != nil {
		status.ErrorMessage = cs.progress.ErrorMessage
	}

	return status
}
