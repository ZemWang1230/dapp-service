package scanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/scanner"
	"timelocker-backend/internal/repository/timelock"
	"timelocker-backend/internal/service/rpc"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/redis"

	"github.com/ethereum/go-ethereum/ethclient"
)

// EmailService 邮件服务接口（避免循环依赖）
type EmailService interface {
	SendFlowNotification(ctx context.Context, standard string, chainID int, contractAddress string, flowID string, statusFrom, statusTo string, txHash *string, initiatorAddress string) error
}

// NotificationService 通知服务接口（避免循环依赖）
type NotificationService interface {
	SendFlowNotification(ctx context.Context, standard string, chainID int, contractAddress string, flowID string, statusFrom, statusTo string, txHash *string, initiatorAddress string) error
}

// ChainScanner 单链扫描器
type ChainScanner struct {
	config       *config.Config
	chainInfo    *types.ChainRPCInfo
	progress     *types.BlockScanProgress
	rpcManager   *rpc.Manager
	queueManager *redis.QueueManager
	progressRepo scanner.ProgressRepository
	txRepo       scanner.TransactionRepository
	flowRepo     scanner.FlowRepository
	timelockRepo timelock.Repository

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
	rpcManager *rpc.Manager,
	queueManager *redis.QueueManager,
	progressRepo scanner.ProgressRepository,
	txRepo scanner.TransactionRepository,
	flowRepo scanner.FlowRepository,
	emailService EmailService,
	notificationService NotificationService,
	timelockRepo timelock.Repository,
) *ChainScanner {
	cs := &ChainScanner{
		config:       cfg,
		chainInfo:    chainInfo,
		progress:     progress,
		rpcManager:   rpcManager,
		queueManager: queueManager,
		progressRepo: progressRepo,
		txRepo:       txRepo,
		flowRepo:     flowRepo,
		timelockRepo: timelockRepo,
		stopCh:       make(chan struct{}),
		lastUpdate:   time.Now(),
	}

	// 创建处理器
	cs.blockProcessor = NewBlockProcessorWithRPCManager(cfg, cs.chainInfo, rpcManager)
	cs.eventProcessor = NewEventProcessor(cfg, txRepo, flowRepo, emailService, notificationService, timelockRepo)

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

	// 启动事件处理协程
	cs.wg.Add(1)
	go cs.eventProcessLoop(ctx)

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

	logger.Info("Stopping chain scanner", "chain_id", cs.chainInfo.ChainID)

	// 先设置为停止状态，避免新的异步数据库更新
	cs.isRunning = false

	// 安全地关闭channel
	select {
	case <-cs.stopCh:
	// channel已经关闭
	default:
		close(cs.stopCh)
	}

	// 等待协程结束
	cs.wg.Wait()

	// 更新本地状态为 paused (等扫链器完全停止后再更新)
	cs.progress.ScanStatus = "paused"
	// cs.progress.ErrorMessage = nil
	cs.progress.LastUpdateTime = time.Now()

	// 同步更新数据库状态
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := cs.progressRepo.UpdateProgress(ctx, cs.progress); err != nil {
		logger.Error("Failed to update progress status during stop", err, "chain_id", cs.chainInfo.ChainID)
	}

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
			// 若状态为 paused（例如上次优雅退出后残留），在真正开始扫描前恢复为 running
			if cs.progress.ScanStatus == "paused" {
				cs.updateProgressStatus("running", "")
			}

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
				// 扫描成功，确保状态为 running（处理从错误状态恢复的情况）
				if cs.progress.ScanStatus == "error" || cs.progress.ErrorMessage != nil {
					cs.updateProgressStatus("running", "")
				}

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
	// 在同一RPC上：获取最新区块、计算范围并扫描
	_, _, err := cs.rpcManager.ExecuteWithRPCInfoDoInfiniteRetry(ctx, cs.chainInfo.ChainID, func(client *ethclient.Client, maxSafeRange int) error {
		// 1) 最新区块
		latestBlock, err := client.BlockNumber(ctx)
		if err != nil {
			return err
		}

		// 2) 更新最新网络区块号
		cs.progress.LatestNetworkBlock = int64(latestBlock)

		// 3) 计算区间（使用该RPC的maxSafeRange）
		fromBlock := cs.progress.LastScannedBlock + 1
		if fromBlock > int64(latestBlock) {
			logger.Debug("No new blocks to scan", "chain_id", cs.chainInfo.ChainID, "latest", latestBlock)
			return nil
		}
		toBlock := cs.calculateToBlockWithSafeRange(fromBlock, int64(latestBlock), maxSafeRange)

		// 4) 扫描并入队
		if fromBlock <= toBlock {
			logs, err := cs.blockProcessor.ScanBlockRangeRaw(ctx, client, fromBlock, toBlock)
			if err != nil {
				return err
			}

			if len(logs) > 0 {
				rawLogs := make([]interface{}, len(logs))
				for i, lg := range logs {
					rawLogs[i] = lg
				}
				if err := cs.queueManager.PushLogs(ctx, cs.chainInfo.ChainID, rawLogs); err != nil {
					return fmt.Errorf("failed to push logs to queue: %w", err)
				}
			}

			cs.progress.LastScannedBlock = toBlock
			cs.progress.LastUpdateTime = time.Now()
			cs.lastUpdate = time.Now()
			if err := cs.progressRepo.UpdateProgressBlock(ctx, cs.chainInfo.ChainID, toBlock, cs.progress.LatestNetworkBlock); err != nil {
				logger.Error("Failed to update progress", err, "chain_id", cs.chainInfo.ChainID, "block", toBlock)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan blocks (context cancelled): %w", err)
	}
	return nil
}

// calculateToBlockWithSafeRange 根据RPC安全范围计算要扫描到的区块号
func (cs *ChainScanner) calculateToBlockWithSafeRange(fromBlock, latestBlock int64, maxSafeRange int) int64 {
	// 根据RPC的安全范围确定批次大小
	var batchSize int64

	batchSize = int64(maxSafeRange)

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

	if lag > int64(cs.config.Scanner.NearLatestThreshold) {
		// 落后较多，使用较短间隔
		return time.Second * 15
	} else {
		// 接近最新区块，使用正常扫描间隔
		return cs.config.Scanner.ScanIntervalSlow
	}
}

// eventProcessLoop 事件处理循环
func (cs *ChainScanner) eventProcessLoop(ctx context.Context) {
	defer cs.wg.Done()

	logger.Info("Starting event process loop", "chain_id", cs.chainInfo.ChainID)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Event process loop stopped by context", "chain_id", cs.chainInfo.ChainID)
			return
		case <-cs.stopCh:
			logger.Info("Event process loop stopped by stop channel", "chain_id", cs.chainInfo.ChainID)
			return
		default:
			// 从Redis队列中获取日志进行处理
			if err := cs.processQueuedLogs(ctx); err != nil {
				// 出错时短暂休息
				select {
				case <-time.After(time.Second * 5):
				case <-cs.stopCh:
					return
				case <-ctx.Done():
					return
				}
			} else {
				// 没有数据时短暂休息
				select {
				case <-time.After(time.Second * 1):
				case <-cs.stopCh:
					return
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// processQueuedLogs 处理队列中的日志
func (cs *ChainScanner) processQueuedLogs(ctx context.Context) error {
	// 批量获取日志
	rawLogs, err := cs.queueManager.PopLogs(ctx, cs.chainInfo.ChainID, int64(cs.config.Scanner.LogQueueBatchSize))
	if err != nil {
		return fmt.Errorf("failed to pop logs from queue: %w", err)
	}

	if len(rawLogs) == 0 {
		return nil // 没有数据，正常返回
	}

	// 处理每个日志
	for _, rawLog := range rawLogs {
		if err := cs.processSingleQueuedLog(ctx, rawLog); err != nil {
			logger.Error("Failed to process single log", err, "chain_id", cs.chainInfo.ChainID, "raw_log", rawLog)
			// 单个日志处理失败不影响其他日志的处理
		}
	}

	return nil
}

// processSingleQueuedLog 处理单个队列中的日志（简化重试逻辑）
func (cs *ChainScanner) processSingleQueuedLog(ctx context.Context, rawLogData string) error {
	// 获取RPC客户端（由RPC manager内部处理健康检查和切换）
	client, err := cs.rpcManager.GetClient(ctx, cs.chainInfo.ChainID)
	if err != nil {
		logger.Error("Failed to get RPC client", err, "chain_id", cs.chainInfo.ChainID)
		return fmt.Errorf("failed to get RPC client: %w", err)
	}

	// 处理日志（processor内部会对必要的RPC调用进行重试）
	event, err := cs.blockProcessor.ProcessLogFromRawData(ctx, client, rawLogData)
	if err != nil {
		logger.Error("Failed to process log", err, "chain_id", cs.chainInfo.ChainID)
		return fmt.Errorf("failed to process log: %w", err)
	}

	if event != nil {
		// 处理单个事件
		events := []TimelockEvent{event}
		if err := cs.eventProcessor.ProcessEvents(ctx, cs.chainInfo.ChainID, cs.chainInfo.ChainName, events); err != nil {
			logger.Error("Failed to process event", err, "chain_id", cs.chainInfo.ChainID, "events", events)
			return fmt.Errorf("failed to process event: %w", err)
		}
	}

	return nil
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

	// 异步更新数据库，但要管理goroutine
	// 检查是否正在停止中，如果是则不启动新的goroutine
	if !cs.isRunning {
		// 扫描器已经停止或正在停止，同步更新
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		if err := cs.progressRepo.UpdateProgress(ctx, cs.progress); err != nil {
			logger.Error("Failed to update progress status during stop", err, "chain_id", cs.chainInfo.ChainID)
		}
		return
	}

	cs.wg.Add(1)
	go func() {
		defer cs.wg.Done()
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
