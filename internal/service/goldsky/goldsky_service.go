package goldsky

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"timelocker-backend/internal/config"
	chainRepo "timelocker-backend/internal/repository/chain"
	goldskyRepo "timelocker-backend/internal/repository/goldsky"
	publicRepo "timelocker-backend/internal/repository/public"
	timelockRepo "timelocker-backend/internal/repository/timelock"
	"timelocker-backend/internal/service/email"
	"timelocker-backend/internal/service/notification"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"
)

// GoldskyService Goldsky 订阅服务
type GoldskyService struct {
	chainRepo           chainRepo.Repository
	timelockRepo        timelockRepo.Repository
	flowRepo            goldskyRepo.FlowRepository
	publicRepo          publicRepo.Repository
	emailSvc            email.EmailService
	notificationSvc     notification.NotificationService
	dispatcher          *NotificationDispatcher
	clients             map[int]*GoldskyClient // chainID -> client
	mu                  sync.RWMutex
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	syncInterval        time.Duration
	statusCheckInterval time.Duration
	syncPageSize        int
}

// NewGoldskyService 创建新的 Goldsky 服务
func NewGoldskyService(
	chainRepo chainRepo.Repository,
	timelockRepo timelockRepo.Repository,
	flowRepo goldskyRepo.FlowRepository,
	publicRepo publicRepo.Repository,
	emailSvc email.EmailService,
	notificationSvc notification.NotificationService,
	cfg *config.Config,
) *GoldskyService {
	ctx, cancel := context.WithCancel(context.Background())

	syncInterval := 10 * time.Minute
	statusCheckInterval := 30 * time.Second
	syncPageSize := 500
	var workers, buffer int
	if cfg != nil {
		if cfg.Goldsky.SyncInterval > 0 {
			syncInterval = cfg.Goldsky.SyncInterval
		}
		if cfg.Goldsky.StatusCheckInterval > 0 {
			statusCheckInterval = cfg.Goldsky.StatusCheckInterval
		}
		if cfg.Goldsky.SyncPageSize > 0 {
			syncPageSize = cfg.Goldsky.SyncPageSize
		}
		workers = cfg.Notification.WorkerCount
		buffer = cfg.Notification.QueueBuffer
	}

	dispatcher := NewNotificationDispatcher(emailSvc, notificationSvc, workers, buffer)

	return &GoldskyService{
		chainRepo:           chainRepo,
		timelockRepo:        timelockRepo,
		flowRepo:            flowRepo,
		publicRepo:          publicRepo,
		emailSvc:            emailSvc,
		notificationSvc:     notificationSvc,
		dispatcher:          dispatcher,
		clients:             make(map[int]*GoldskyClient),
		ctx:                 ctx,
		cancel:              cancel,
		syncInterval:        syncInterval,
		statusCheckInterval: statusCheckInterval,
		syncPageSize:        syncPageSize,
	}
}

// Dispatcher 对外暴露 dispatcher，方便 webhook_processor 复用 worker 池
func (s *GoldskyService) Dispatcher() *NotificationDispatcher {
	return s.dispatcher
}

// Start 启动 Goldsky 服务
func (s *GoldskyService) Start() error {
	logger.Info("Starting Goldsky service...",
		"sync_interval", s.syncInterval.String(),
		"status_check_interval", s.statusCheckInterval.String(),
		"sync_page_size", s.syncPageSize,
	)

	// 初始化所有链的客户端
	if err := s.initializeClients(); err != nil {
		return fmt.Errorf("failed to initialize Goldsky clients: %w", err)
	}

	// 启动通知分发器 worker 池
	if s.dispatcher != nil {
		s.dispatcher.Start(s.ctx)
	}

	// 启动同步任务
	s.wg.Add(1)
	go s.syncFlowsLoop()

	// 启动状态检查任务
	s.wg.Add(1)
	go s.checkFlowStatusLoop()

	logger.Info("Goldsky service started successfully")
	return nil
}

// Stop 停止 Goldsky 服务
func (s *GoldskyService) Stop() {
	logger.Info("Stopping Goldsky service...")
	s.cancel()
	s.wg.Wait()
	if s.dispatcher != nil {
		s.dispatcher.Stop()
	}
	logger.Info("Goldsky service stopped")
}

// initializeClients 初始化所有链的 Goldsky 客户端
func (s *GoldskyService) initializeClients() error {
	chains, err := s.chainRepo.GetAllActiveChains()
	if err != nil {
		return fmt.Errorf("failed to get active chains: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, chain := range chains {
		if chain.SubgraphURL != "" {
			client := NewGoldskyClient(chain.SubgraphURL, int(chain.ChainID))
			s.clients[int(chain.ChainID)] = client
			logger.Info("Initialized Goldsky client", "chain_id", chain.ChainID, "chain_name", chain.ChainName)
		}
	}

	logger.Info("Initialized Goldsky clients", "count", len(s.clients))
	return nil
}

// syncFlowsLoop 同步 Flows 的循环任务
func (s *GoldskyService) syncFlowsLoop() {
	defer s.wg.Done()
	defer logger.Info("Goldsky sync flows loop stopped")

	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	// 启动时立即执行一次
	s.syncAllFlows()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.syncAllFlows()
		}
	}
}

// syncAllFlows 同步所有链的 Flows
func (s *GoldskyService) syncAllFlows() {
	logger.Info("Starting to sync flows from Goldsky...")

	s.mu.RLock()
	clients := make(map[int]*GoldskyClient)
	for chainID, client := range s.clients {
		clients[chainID] = client
	}
	s.mu.RUnlock()

	var wg sync.WaitGroup
	for chainID, client := range clients {
		wg.Add(1)
		go func(cid int, c *GoldskyClient) {
			defer wg.Done()
			if err := s.syncFlowsForChain(cid, c); err != nil {
				logger.Error("Failed to sync flows for chain", err, "chain_id", cid)
			}
		}(chainID, client)
	}

	wg.Wait()
	logger.Info("Finished syncing flows from Goldsky")
}

// syncFlowsForChain 同步指定链的 Flows 和统计数据
func (s *GoldskyService) syncFlowsForChain(chainID int, client *GoldskyClient) error {
	// 获取该链上所有激活的合约地址
	compoundContracts, err := s.timelockRepo.GetAllActiveCompoundTimelocks(s.ctx, chainID)
	if err != nil {
		return fmt.Errorf("failed to get compound contracts: %w", err)
	}

	// 同步 Compound Flows
	if len(compoundContracts) > 0 {
		compoundAddresses := make([]string, len(compoundContracts))
		for i, contract := range compoundContracts {
			compoundAddresses[i] = contract.ContractAddress
		}

		if err := s.syncCompoundFlows(chainID, client, compoundAddresses); err != nil {
			logger.Error("Failed to sync compound flows", err, "chain_id", chainID)
		}
	}

	// 同步该链的统计数据
	if err := s.syncChainStatistics(chainID, client); err != nil {
		logger.Error("Failed to sync chain statistics", err, "chain_id", chainID)
	}

	return nil
}

// syncCompoundFlows 同步 Compound Flows（游标分页 + 批量读本地 DB 避免 N+1）
func (s *GoldskyService) syncCompoundFlows(chainID int, client *GoldskyClient, contractAddresses []string) error {
	start := time.Now()
	pageSize := s.syncPageSize
	if pageSize <= 0 {
		pageSize = 500
	}

	// 提前把这批合约现有的 flow 拉成 map 一次，避免每条 flow 一次 SELECT
	localMap, err := s.flowRepo.GetCompoundFlowsMapByContracts(s.ctx, chainID, contractAddresses)
	if err != nil {
		logger.Error("Failed to batch load local compound flows", err, "chain_id", chainID)
		// 失败也不阻塞，让后面每条自己走 GetCompoundFlowByID 兜底
		localMap = map[string]*types.CompoundTimelockFlowDB{}
	}

	var totalFetched int
	var totalUpserted int
	skip := 0
	for {
		flows, err := client.QueryCompoundFlowsPage(s.ctx, contractAddresses, pageSize, skip)
		if err != nil {
			return fmt.Errorf("failed to query compound flows (skip=%d): %w", skip, err)
		}
		if len(flows) == 0 {
			break
		}
		totalFetched += len(flows)

		for _, goldskyFlow := range flows {
			dbFlow, err := ConvertGoldskyCompoundFlowToDB(goldskyFlow, chainID)
			if err != nil {
				logger.Error("Failed to convert compound flow", err, "flow_id", goldskyFlow.FlowID)
				continue
			}

			key := goldskyRepo.CompoundFlowKey(dbFlow.FlowID, dbFlow.ContractAddress)
			if oldFlow, ok := localMap[key]; ok && oldFlow != nil {
				// 保护本地状态：ready/expired 不被 goldsky 的 waiting 覆盖
				if (oldFlow.Status == "ready" || oldFlow.Status == "expired") && dbFlow.Status == "waiting" {
					dbFlow.Status = oldFlow.Status
				}
			}

			if err := s.flowRepo.CreateOrUpdateCompoundFlow(s.ctx, dbFlow); err != nil {
				logger.Error("Failed to create or update compound flow", err, "flow_id", dbFlow.FlowID)
				continue
			}
			totalUpserted++
		}

		// 不足一页意味着已经拉完
		if len(flows) < pageSize {
			break
		}
		skip += len(flows)

		// subgraph skip 上限通常是 5000，超过后要改用 createdAt 游标；
		// 这里安全起见做个硬阈值，避免死循环
		if skip >= 20000 {
			logger.Warn("syncCompoundFlows reached skip hard limit", "chain_id", chainID, "skip", skip)
			break
		}
	}

	logger.Info("Synced compound flows from Goldsky",
		"chain_id", chainID,
		"contracts", len(contractAddresses),
		"fetched", totalFetched,
		"upserted", totalUpserted,
		"elapsed_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// checkFlowStatusLoop 检查 Flow 状态的循环任务（每30秒）
func (s *GoldskyService) checkFlowStatusLoop() {
	defer s.wg.Done()
	defer logger.Info("Goldsky check flow status loop stopped")

	ticker := time.NewTicker(s.statusCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAndUpdateFlowStatus()
		}
	}
}

// checkAndUpdateFlowStatus 检查并更新 Flow 状态
func (s *GoldskyService) checkAndUpdateFlowStatus() {
	now := time.Now()

	// 检查 Compound Flows
	compoundFlows, err := s.flowRepo.GetCompoundFlowsNeedStatusUpdate(s.ctx, now, 100)
	if err != nil {
		logger.Error("Failed to get compound flows need status update", err)
	} else {
		for _, flow := range compoundFlows {
			newStatus := s.calculateNewStatus(flow.Status, flow.Eta, flow.ExpiredAt, now)
			if newStatus != flow.Status {
				if err := s.flowRepo.UpdateCompoundFlowStatus(s.ctx, flow.FlowID, flow.ChainID, flow.ContractAddress, newStatus); err != nil {
					logger.Error("Failed to update compound flow status", err, "flow_id", flow.FlowID, "new_status", newStatus)
					continue
				}

				logger.Info("Updated compound flow status", "flow_id", flow.FlowID, "old_status", flow.Status, "new_status", newStatus)
				s.enqueueFlowNotification(flow.ChainID, flow.ContractAddress, flow.FlowID, "compound", flow.Status, newStatus, nil, "", "status_check")
			}
		}
	}
}

// calculateNewStatus 计算新的状态
func (s *GoldskyService) calculateNewStatus(currentStatus string, eta *time.Time, expiredAt *time.Time, now time.Time) string {
	if currentStatus == "waiting" && eta != nil && eta.Before(now) {
		return "ready"
	}

	if currentStatus == "ready" && expiredAt != nil && expiredAt.Before(now) {
		return "expired"
	}

	return currentStatus
}

// enqueueFlowNotification 把状态变化通知任务投递到 worker 池（dispatcher 为空时降级为同步执行）
func (s *GoldskyService) enqueueFlowNotification(chainID int, contractAddress, flowID, standard, oldStatus, newStatus string, txHash *string, initiator string, source string) {
	job := flowNotificationJob{
		Standard:         standard,
		ChainID:          chainID,
		ContractAddress:  contractAddress,
		FlowID:           flowID,
		StatusFrom:       oldStatus,
		StatusTo:         newStatus,
		TxHash:           txHash,
		InitiatorAddress: initiator,
		Source:           source,
	}
	if s.dispatcher != nil {
		s.dispatcher.Enqueue(job)
		return
	}

	// 未启动 dispatcher 时退回到原先的 `go ...` 行为
	go func() {
		ctx := context.Background()
		if err := s.emailSvc.SendFlowNotification(ctx, standard, chainID, contractAddress, flowID, oldStatus, newStatus, txHash, initiator); err != nil {
			logger.Error("Failed to send email notification", err, "chain_id", chainID, "flow_id", flowID, "status_to", newStatus)
		}
		if err := s.notificationSvc.SendFlowNotification(ctx, standard, chainID, contractAddress, flowID, oldStatus, newStatus, txHash, initiator); err != nil {
			logger.Error("Failed to send channel notification", err, "chain_id", chainID, "flow_id", flowID, "status_to", newStatus)
		}
	}()
}

// SyncFlowsForContract 同步特定合约的flows
func (s *GoldskyService) SyncFlowsForContract(ctx context.Context, chainID int, standard, contractAddress string) error {
	client, exists := s.clients[chainID]
	if !exists {
		return fmt.Errorf("no goldsky client for chain %d", chainID)
	}

	switch standard {
	case "compound":
		return s.syncCompoundFlowsForContract(ctx, chainID, client, contractAddress)
	default:
		return fmt.Errorf("unsupported timelock standard: %s", standard)
	}
}

// syncCompoundFlowsForContract 同步特定Compound合约的flows（游标分页 + 批量读本地 DB）
func (s *GoldskyService) syncCompoundFlowsForContract(ctx context.Context, chainID int, client *GoldskyClient, contractAddress string) error {
	start := time.Now()
	pageSize := s.syncPageSize
	if pageSize <= 0 {
		pageSize = 500
	}

	localMap, err := s.flowRepo.GetCompoundFlowsMapByContracts(ctx, chainID, []string{contractAddress})
	if err != nil {
		logger.Error("Failed to batch load local compound flows", err, "chain_id", chainID, "contract_address", contractAddress)
		localMap = map[string]*types.CompoundTimelockFlowDB{}
	}

	var totalFetched, totalUpserted int
	skip := 0
	for {
		flows, err := client.QueryCompoundFlowsPage(ctx, []string{contractAddress}, pageSize, skip)
		if err != nil {
			return fmt.Errorf("failed to query compound flows for contract %s (skip=%d): %w", contractAddress, skip, err)
		}
		if len(flows) == 0 {
			break
		}
		totalFetched += len(flows)

		for _, goldskyFlow := range flows {
			dbFlow, err := ConvertGoldskyCompoundFlowToDB(goldskyFlow, chainID)
			if err != nil {
				logger.Error("Failed to convert compound flow", err, "flow_id", goldskyFlow.FlowID, "contract_address", contractAddress)
				continue
			}

			key := goldskyRepo.CompoundFlowKey(dbFlow.FlowID, dbFlow.ContractAddress)
			if oldFlow, ok := localMap[key]; ok && oldFlow != nil {
				if (oldFlow.Status == "ready" || oldFlow.Status == "expired") && dbFlow.Status == "waiting" {
					dbFlow.Status = oldFlow.Status
				}
			}

			if err := s.flowRepo.CreateOrUpdateCompoundFlow(ctx, dbFlow); err != nil {
				logger.Error("Failed to create or update compound flow", err, "flow_id", dbFlow.FlowID, "contract_address", contractAddress)
				continue
			}
			totalUpserted++
		}

		if len(flows) < pageSize {
			break
		}
		skip += len(flows)
		if skip >= 20000 {
			logger.Warn("syncCompoundFlowsForContract reached skip hard limit", "chain_id", chainID, "skip", skip)
			break
		}
	}

	logger.Info("Synced compound flows from Goldsky for contract",
		"chain_id", chainID,
		"contract_address", contractAddress,
		"fetched", totalFetched,
		"upserted", totalUpserted,
		"elapsed_ms", time.Since(start).Milliseconds(),
	)

	// 同步完成后立即检查并更新状态，确保处理已过期或已就绪的 flows
	s.checkAndUpdateFlowStatusForContract(ctx, chainID, "compound", contractAddress)

	return nil
}

// checkAndUpdateFlowStatusForContract 检查并更新特定合约的flow状态
func (s *GoldskyService) checkAndUpdateFlowStatusForContract(ctx context.Context, chainID int, standard, contractAddress string) {
	now := time.Now()

	switch standard {
	case "compound":
		// 获取该合约的所有flows
		flows, err := s.flowRepo.GetCompoundFlowsByContract(ctx, chainID, contractAddress)
		if err != nil {
			logger.Error("Failed to get compound flows for status check", err, "contract_address", contractAddress, "chain_id", chainID)
			return
		}

		for _, flow := range flows {
			var newStatus string
			if now.After(*flow.ExpiredAt) && flow.Status == "waiting" {
				newStatus = "expired"
			} else {
				newStatus = s.calculateNewStatus(flow.Status, flow.Eta, flow.ExpiredAt, now)
			}
			if newStatus != flow.Status {
				if err := s.flowRepo.UpdateCompoundFlowStatus(ctx, flow.FlowID, flow.ChainID, flow.ContractAddress, newStatus); err != nil {
					logger.Error("Failed to update compound flow status", err, "flow_id", flow.FlowID, "new_status", newStatus)
					continue
				}
				logger.Info("Updated compound flow status during sync", "flow_id", flow.FlowID, "old_status", flow.Status, "new_status", newStatus, "contract_address", contractAddress)
			}
		}
	}
}

// GetCompoundFlowDetail 获取 Compound Flow 详情（用于 Webhook 优化）
func (s *GoldskyService) GetCompoundFlowDetail(ctx context.Context, chainID int, flowID string) (*types.GoldskyCompoundFlow, error) {
	s.mu.RLock()
	client, exists := s.clients[chainID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no Goldsky client for chain %d", chainID)
	}

	flow, err := client.QueryCompoundFlowByFlowID(ctx, flowID)
	if err != nil {
		return nil, fmt.Errorf("failed to query compound flow: %w", err)
	}

	return flow, nil
}

// GetTransactionDetail 获取交易详情（用于 API）
func (s *GoldskyService) GetTransactionDetail(ctx context.Context, chainID int, standard, txHash string) (*types.CompoundTimelockTransactionDetail, error) {
	s.mu.RLock()
	client, exists := s.clients[chainID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no Goldsky client for chain %d", chainID)
	}

	if standard == "compound" {
		tx, err := client.QueryCompoundTransactionByTxHash(ctx, txHash)
		if err != nil {
			return nil, fmt.Errorf("failed to query compound transaction: %w", err)
		}
		if tx == nil {
			return nil, fmt.Errorf("transaction not found")
		}

		// 转换为响应格式
		return s.convertCompoundTransactionToDetail(tx, chainID)
	}

	return nil, fmt.Errorf("invalid standard: %s", standard)
}

// convertCompoundTransactionToDetail 转换 Compound Transaction 为详情格式
func (s *GoldskyService) convertCompoundTransactionToDetail(tx *types.GoldskyCompoundTransaction, chainID int) (*types.CompoundTimelockTransactionDetail, error) {
	blockNumber, err := parseTimestamp(tx.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block number: %w", err)
	}

	blockTimestamp, err := parseTimestamp(tx.BlockTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block timestamp: %w", err)
	}

	detail := &types.CompoundTimelockTransactionDetail{
		TxHash:          tx.TxHash,
		BlockNumber:     blockNumber.Unix(),
		BlockTimestamp:  blockTimestamp,
		ChainID:         chainID,
		ContractAddress: tx.ContractAddress,
		FromAddress:     tx.FromAddress,
		ToAddress:       tx.ContractAddress,
		TxStatus:        "success",
		EventValue:      tx.EventValue,
	}

	if tx.EventTxHash != nil {
		detail.EventTxHash = tx.EventTxHash
	}
	if tx.EventTarget != nil {
		detail.EventTarget = tx.EventTarget
	}
	if tx.EventSignature != nil {
		detail.EventFunctionSignature = tx.EventSignature
	}
	if tx.EventEta != nil {
		if eta, err := strconv.ParseInt(*tx.EventEta, 10, 64); err == nil {
			detail.EventEta = &eta
		}
	}

	// 解析 EventData (hex string -> bytes)
	if tx.EventData != nil && *tx.EventData != "" {
		callDataStr := strings.TrimPrefix(*tx.EventData, "0x")
		callDataBytes, err := hex.DecodeString(callDataStr)
		if err == nil {
			detail.EventCallData = callDataBytes
		}
	}

	// 从 chain repository 获取 chain_name
	chain, err := s.chainRepo.GetChainByChainID(s.ctx, int64(chainID))
	if err != nil {
		logger.Warn("Failed to get chain info", "chain_id", chainID, "error", err)
		// 设置默认值或者留空
		detail.ChainName = ""
	} else {
		detail.ChainName = chain.ChainName
	}

	return detail, nil
}

// GetGlobalContractCount 获取全局合约数量（从Goldsky GlobalStatistics获取）
func (s *GoldskyService) GetGlobalContractCount(ctx context.Context) (int64, error) {
	totalContracts := int64(0)
	chains, err := s.chainRepo.GetAllActiveChains()
	if err != nil {
		return 0, fmt.Errorf("failed to get active chains: %w", err)
	}

	// 尝试从每个链的subgraph查询GlobalStatistics，取第一个成功的
	for _, chain := range chains {
		if chain.SubgraphURL == "" {
			continue
		}

		client := NewGoldskyClient(chain.SubgraphURL, int(chain.ChainID))
		stats, err := client.QueryGlobalStatistics(ctx)
		if err != nil {
			logger.Warn("Failed to query global statistics for chain", "chain_id", chain.ChainID, "chain_name", chain.ChainName, "error", err)
			continue
		}

		// 解析总数
		contracts, err := strconv.ParseInt(stats.TotalContracts, 10, 64)
		if err != nil {
			logger.Warn("Failed to parse total contracts", "value", stats.TotalContracts, "error", err)
			continue
		}

		logger.Info("Got global contract count from Goldsky", "chain_id", chain.ChainID, "chain_name", chain.ChainName, "total_contracts", totalContracts)
		totalContracts += contracts
	}

	return totalContracts, nil
}

// GetGlobalTransactionCount 获取全局交易数量（从Goldsky GlobalStatistics获取）
func (s *GoldskyService) GetGlobalTransactionCount(ctx context.Context) (int64, error) {
	totalTransactions := int64(0)
	chains, err := s.chainRepo.GetAllActiveChains()
	if err != nil {
		return 0, fmt.Errorf("failed to get active chains: %w", err)
	}

	// 尝试从每个链的subgraph查询GlobalStatistics，取第一个成功的
	for _, chain := range chains {
		if chain.SubgraphURL == "" {
			continue
		}

		client := NewGoldskyClient(chain.SubgraphURL, int(chain.ChainID))
		stats, err := client.QueryGlobalStatistics(ctx)
		if err != nil {
			logger.Warn("Failed to query global statistics for chain", "chain_id", chain.ChainID, "chain_name", chain.ChainName, "error", err)
			continue
		}

		// 使用TotalTransactions作为交易数量
		transactions, err := strconv.ParseInt(stats.TotalTransactions, 10, 64)
		if err != nil {
			logger.Warn("Failed to parse total transactions", "value", stats.TotalTransactions, "error", err)
			continue
		}

		logger.Info("Got global transaction count from Goldsky", "chain_id", chain.ChainID, "chain_name", chain.ChainName, "total_transactions", totalTransactions)
		totalTransactions += transactions
	}

	return totalTransactions, nil
}

// syncChainStatistics 同步指定链的统计数据
func (s *GoldskyService) syncChainStatistics(chainID int, client *GoldskyClient) error {
	// 获取该链的信息
	chain, err := s.chainRepo.GetChainByChainID(s.ctx, int64(chainID))
	if err != nil {
		return fmt.Errorf("failed to get chain info: %w", err)
	}

	// 从 Goldsky 获取统计数据
	stats, err := client.QueryGlobalStatistics(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to query global statistics for chain %d: %w", chainID, err)
	}

	// 解析统计数据
	contractCount, err := strconv.ParseInt(stats.TotalContracts, 10, 64)
	if err != nil {
		logger.Warn("Failed to parse total contracts", "chain_id", chainID, "value", stats.TotalContracts, "error", err)
		contractCount = 0
	}

	transactionCount, err := strconv.ParseInt(stats.TotalTransactions, 10, 64)
	if err != nil {
		logger.Warn("Failed to parse total transactions", "chain_id", chainID, "value", stats.TotalTransactions, "error", err)
		transactionCount = 0
	}

	// 更新数据库中的统计数据
	if s.publicRepo != nil {
		err = s.publicRepo.UpdateChainStatistics(s.ctx, chainID, chain.ChainName, contractCount, transactionCount)
		if err != nil {
			return fmt.Errorf("failed to update chain statistics: %w", err)
		}
	}

	logger.Info("Synced chain statistics", "chain_id", chainID, "chain_name", chain.ChainName, "contracts", contractCount, "transactions", transactionCount)
	return nil
}
