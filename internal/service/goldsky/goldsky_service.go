package goldsky

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	chainRepo "timelocker-backend/internal/repository/chain"
	goldskyRepo "timelocker-backend/internal/repository/goldsky"
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
	emailSvc            email.EmailService
	notificationSvc     notification.NotificationService
	clients             map[int]*GoldskyClient // chainID -> client
	mu                  sync.RWMutex
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	syncInterval        time.Duration
	statusCheckInterval time.Duration
}

// NewGoldskyService 创建新的 Goldsky 服务
func NewGoldskyService(
	chainRepo chainRepo.Repository,
	timelockRepo timelockRepo.Repository,
	flowRepo goldskyRepo.FlowRepository,
	emailSvc email.EmailService,
	notificationSvc notification.NotificationService,
) *GoldskyService {
	ctx, cancel := context.WithCancel(context.Background())
	return &GoldskyService{
		chainRepo:           chainRepo,
		timelockRepo:        timelockRepo,
		flowRepo:            flowRepo,
		emailSvc:            emailSvc,
		notificationSvc:     notificationSvc,
		clients:             make(map[int]*GoldskyClient),
		ctx:                 ctx,
		cancel:              cancel,
		syncInterval:        10 * time.Minute, // 每10分钟同步一次
		statusCheckInterval: 1 * time.Minute,  // 每1分钟检查一次状态
	}
}

// Start 启动 Goldsky 服务
func (s *GoldskyService) Start() error {
	logger.Info("Starting Goldsky service...")

	// 初始化所有链的客户端
	if err := s.initializeClients(); err != nil {
		return fmt.Errorf("failed to initialize Goldsky clients: %w", err)
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

// syncFlowsForChain 同步指定链的 Flows
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

	return nil
}

// syncCompoundFlows 同步 Compound Flows
func (s *GoldskyService) syncCompoundFlows(chainID int, client *GoldskyClient, contractAddresses []string) error {
	flows, err := client.QueryCompoundFlows(s.ctx, contractAddresses, 1000)
	if err != nil {
		return fmt.Errorf("failed to query compound flows: %w", err)
	}

	logger.Info("Queried compound flows from Goldsky", "chain_id", chainID, "count", len(flows))

	for _, goldskyFlow := range flows {
		// 转换为数据库模型
		dbFlow, err := ConvertGoldskyCompoundFlowToDB(goldskyFlow, chainID)
		if err != nil {
			logger.Error("Failed to convert compound flow", err, "flow_id", goldskyFlow.FlowID)
			continue
		}

		// 获取旧的 Flow 状态（用于检测状态变化）
		oldFlow, err := s.flowRepo.GetCompoundFlowByID(s.ctx, dbFlow.FlowID, chainID, dbFlow.ContractAddress)
		if err != nil {
			logger.Error("Failed to get old compound flow", err, "flow_id", dbFlow.FlowID)
		}

		// 【重要】保护本地状态：ready 和 expired 状态不能被 Goldsky 的 waiting 覆盖
		// 因为 ready 和 expired 是后端定时任务设置的，Goldsky 中没有这些状态
		if oldFlow != nil {
			// 如果本地是 ready 或 expired，而 Goldsky 是 waiting，保持本地状态
			if (oldFlow.Status == "ready" || oldFlow.Status == "expired") && dbFlow.Status == "waiting" {
				logger.Debug("Preserving local status",
					"flow_id", dbFlow.FlowID,
					"local_status", oldFlow.Status,
					"goldsky_status", dbFlow.Status)
				dbFlow.Status = oldFlow.Status // 保持本地状态
			}
		}

		// 创建或更新
		if err := s.flowRepo.CreateOrUpdateCompoundFlow(s.ctx, dbFlow); err != nil {
			logger.Error("Failed to create or update compound flow", err, "flow_id", dbFlow.FlowID)
			continue
		}
	}

	return nil
}

// syncOpenzeppelinFlows 同步 OpenZeppelin Flows
// func (s *GoldskyService) syncOpenzeppelinFlows(chainID int, client *GoldskyClient, contractAddresses []string) error {
// 	flows, err := client.QueryOpenzeppelinFlows(s.ctx, contractAddresses, 1000)
// 	if err != nil {
// 		return fmt.Errorf("failed to query openzeppelin flows: %w", err)
// 	}

// 	logger.Info("Queried openzeppelin flows from Goldsky", "chain_id", chainID, "count", len(flows))

// 	for _, goldskyFlow := range flows {
// 		// 转换为数据库模型
// 		dbFlow, err := ConvertGoldskyOpenzeppelinFlowToDB(goldskyFlow, chainID)
// 		if err != nil {
// 			logger.Error("Failed to convert openzeppelin flow", err, "flow_id", goldskyFlow.FlowID)
// 			continue
// 		}

// 		// 获取旧的 Flow 状态（用于检测状态变化）
// 		oldFlow, err := s.flowRepo.GetOpenzeppelinFlowByID(s.ctx, dbFlow.FlowID, chainID, dbFlow.ContractAddress)
// 		if err != nil {
// 			logger.Error("Failed to get old openzeppelin flow", err, "flow_id", dbFlow.FlowID)
// 		}

// 		// 【重要】保护本地状态：ready 状态不能被 Goldsky 的 waiting 覆盖
// 		// OpenZeppelin 没有 expired 状态
// 		if oldFlow != nil {
// 			if oldFlow.Status == "ready" && dbFlow.Status == "waiting" {
// 				logger.Debug("Preserving local status",
// 					"flow_id", dbFlow.FlowID,
// 					"local_status", oldFlow.Status,
// 					"goldsky_status", dbFlow.Status)
// 				dbFlow.Status = oldFlow.Status
// 			}
// 		}

// 		// 创建或更新
// 		if err := s.flowRepo.CreateOrUpdateOpenzeppelinFlow(s.ctx, dbFlow); err != nil {
// 			logger.Error("Failed to create or update openzeppelin flow", err, "flow_id", dbFlow.FlowID)
// 			continue
// 		}
// 	}

// 	return nil
// }

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
				go s.sendFlowStatusChangeNotification(flow.ChainID, flow.ContractAddress, flow.FlowID, "compound", flow.Status, newStatus)
			}
		}
	}

	// 检查 OpenZeppelin Flows
	ozFlows, err := s.flowRepo.GetOpenzeppelinFlowsNeedStatusUpdate(s.ctx, now, 100)
	if err != nil {
		logger.Error("Failed to get openzeppelin flows need status update", err)
	} else {
		for _, flow := range ozFlows {
			newStatus := s.calculateNewStatus(flow.Status, flow.Eta, nil, now)
			if newStatus != flow.Status {
				if err := s.flowRepo.UpdateOpenzeppelinFlowStatus(s.ctx, flow.FlowID, flow.ChainID, flow.ContractAddress, newStatus); err != nil {
					logger.Error("Failed to update openzeppelin flow status", err, "flow_id", flow.FlowID, "new_status", newStatus)
					continue
				}

				logger.Info("Updated openzeppelin flow status", "flow_id", flow.FlowID, "old_status", flow.Status, "new_status", newStatus)
				s.sendFlowStatusChangeNotification(flow.ChainID, flow.ContractAddress, flow.FlowID, "openzeppelin", flow.Status, newStatus)
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

// sendFlowStatusChangeNotification 发送 Flow 状态变化通知
func (s *GoldskyService) sendFlowStatusChangeNotification(chainID int, contractAddress, flowID, standard, oldStatus, newStatus string) {
	ctx := context.Background()

	logger.Info("Sending flow notification (status check)",
		"chain_id", chainID,
		"contract", contractAddress,
		"flow_id", flowID,
		"standard", standard,
		"from", oldStatus,
		"to", newStatus)

	// 1. 发送邮件通知
	if err := s.emailSvc.SendFlowNotification(ctx, standard, chainID, contractAddress, flowID, oldStatus, newStatus, nil, ""); err != nil {
		logger.Error("Failed to send email notification", err,
			"chain_id", chainID,
			"flow_id", flowID,
			"status_to", newStatus)
	} else {
		logger.Info("Email notification sent successfully", "flow_id", flowID, "status", newStatus)
	}

	// 2. 发送渠道通知（Telegram/Lark/Feishu 等）
	if err := s.notificationSvc.SendFlowNotification(ctx, standard, chainID, contractAddress, flowID, oldStatus, newStatus, nil, ""); err != nil {
		logger.Error("Failed to send channel notification", err,
			"chain_id", chainID,
			"flow_id", flowID,
			"status_to", newStatus)
	} else {
		logger.Info("Channel notification sent successfully", "flow_id", flowID, "status", newStatus)
	}
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
	case "openzeppelin":
		return s.syncOpenzeppelinFlowsForContract(ctx, chainID, client, contractAddress)
	default:
		return fmt.Errorf("unsupported timelock standard: %s", standard)
	}
}

// syncCompoundFlowsForContract 同步特定Compound合约的flows
func (s *GoldskyService) syncCompoundFlowsForContract(ctx context.Context, chainID int, client *GoldskyClient, contractAddress string) error {
	flows, err := client.QueryCompoundFlows(ctx, []string{contractAddress}, 1000)
	if err != nil {
		return fmt.Errorf("failed to query compound flows for contract %s: %w", contractAddress, err)
	}

	logger.Info("Queried compound flows from Goldsky for contract", "chain_id", chainID, "contract_address", contractAddress, "count", len(flows))

	for _, goldskyFlow := range flows {
		// 转换为数据库模型
		dbFlow, err := ConvertGoldskyCompoundFlowToDB(goldskyFlow, chainID)
		if err != nil {
			logger.Error("Failed to convert compound flow", err, "flow_id", goldskyFlow.FlowID, "contract_address", contractAddress)
			continue
		}

		// 获取旧的 Flow 状态（用于检测状态变化）
		oldFlow, err := s.flowRepo.GetCompoundFlowByID(ctx, dbFlow.FlowID, chainID, contractAddress)
		if err != nil && err.Error() != "record not found" {
			logger.Error("Failed to get old compound flow", err, "flow_id", dbFlow.FlowID, "contract_address", contractAddress)
		}

		// 【重要】保护本地状态：ready 和 expired 状态不能被 Goldsky 的 waiting 覆盖
		if oldFlow != nil {
			// 如果本地是 ready 或 expired，而 Goldsky 是 waiting，保持本地状态
			if (oldFlow.Status == "ready" || oldFlow.Status == "expired") && dbFlow.Status == "waiting" {
				logger.Debug("Preserving local status",
					"flow_id", dbFlow.FlowID,
					"local_status", oldFlow.Status,
					"goldsky_status", dbFlow.Status)
				dbFlow.Status = oldFlow.Status // 保持本地状态
			}
		}

		// 创建或更新
		if err := s.flowRepo.CreateOrUpdateCompoundFlow(ctx, dbFlow); err != nil {
			logger.Error("Failed to create or update compound flow", err, "flow_id", dbFlow.FlowID, "contract_address", contractAddress)
			continue
		}

		logger.Debug("Synced compound flow", "flow_id", dbFlow.FlowID, "status", dbFlow.Status, "contract_address", contractAddress)
	}

	// 【优化】同步完成后立即检查并更新状态，确保处理已过期或已就绪的flows
	logger.Info("Running immediate status check for compound contract", "contract_address", contractAddress, "chain_id", chainID)
	s.checkAndUpdateFlowStatusForContract(ctx, chainID, "compound", contractAddress)

	return nil
}

// syncOpenzeppelinFlowsForContract 同步特定OpenZeppelin合约的flows
func (s *GoldskyService) syncOpenzeppelinFlowsForContract(ctx context.Context, chainID int, client *GoldskyClient, contractAddress string) error {
	flows, err := client.QueryOpenzeppelinFlows(ctx, []string{contractAddress}, 1000)
	if err != nil {
		return fmt.Errorf("failed to query openzeppelin flows for contract %s: %w", contractAddress, err)
	}

	logger.Info("Queried openzeppelin flows from Goldsky for contract", "chain_id", chainID, "contract_address", contractAddress, "count", len(flows))

	for _, goldskyFlow := range flows {
		// 转换为数据库模型
		dbFlow, err := ConvertGoldskyOpenzeppelinFlowToDB(goldskyFlow, chainID)
		if err != nil {
			logger.Error("Failed to convert openzeppelin flow", err, "flow_id", goldskyFlow.FlowID, "contract_address", contractAddress)
			continue
		}

		// 获取旧的 Flow 状态（用于检测状态变化）
		oldFlow, err := s.flowRepo.GetOpenzeppelinFlowByID(ctx, dbFlow.FlowID, chainID, contractAddress)
		if err != nil && err.Error() != "record not found" {
			logger.Error("Failed to get old openzeppelin flow", err, "flow_id", dbFlow.FlowID, "contract_address", contractAddress)
		}

		// 【重要】保护本地状态：ready 和 expired 状态不能被 Goldsky 的 waiting 覆盖
		if oldFlow != nil {
			// 如果本地是 ready 或 expired，而 Goldsky 是 waiting，保持本地状态
			if (oldFlow.Status == "ready" || oldFlow.Status == "expired") && dbFlow.Status == "waiting" {
				logger.Debug("Preserving local status",
					"flow_id", dbFlow.FlowID,
					"local_status", oldFlow.Status,
					"goldsky_status", dbFlow.Status)
				dbFlow.Status = oldFlow.Status // 保持本地状态
			}
		}

		// 创建或更新
		if err := s.flowRepo.CreateOrUpdateOpenzeppelinFlow(ctx, dbFlow); err != nil {
			logger.Error("Failed to create or update openzeppelin flow", err, "flow_id", dbFlow.FlowID, "contract_address", contractAddress)
			continue
		}

		logger.Debug("Synced openzeppelin flow", "flow_id", dbFlow.FlowID, "status", dbFlow.Status, "contract_address", contractAddress)
	}

	// 【优化】同步完成后立即检查并更新状态，确保处理已过期或已就绪的flows
	logger.Info("Running immediate status check for openzeppelin contract", "contract_address", contractAddress, "chain_id", chainID)
	s.checkAndUpdateFlowStatusForContract(ctx, chainID, "openzeppelin", contractAddress)

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

	case "openzeppelin":
		// 获取该合约的所有flows
		flows, err := s.flowRepo.GetOpenzeppelinFlowsByContract(ctx, chainID, contractAddress)
		if err != nil {
			logger.Error("Failed to get openzeppelin flows for status check", err, "contract_address", contractAddress, "chain_id", chainID)
			return
		}

		for _, flow := range flows {
			newStatus := s.calculateNewStatus(flow.Status, flow.Eta, nil, now)
			if newStatus != flow.Status {
				if err := s.flowRepo.UpdateOpenzeppelinFlowStatus(ctx, flow.FlowID, flow.ChainID, flow.ContractAddress, newStatus); err != nil {
					logger.Error("Failed to update openzeppelin flow status", err, "flow_id", flow.FlowID, "new_status", newStatus)
					continue
				}
				logger.Info("Updated openzeppelin flow status during sync", "flow_id", flow.FlowID, "old_status", flow.Status, "new_status", newStatus, "contract_address", contractAddress)
			}
		}
	}
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
	} else if standard == "openzeppelin" {
		// TODO: 实现 OpenZeppelin 交易详情转换
		return nil, fmt.Errorf("openzeppelin transaction detail not implemented yet")
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
		totalContracts, err := strconv.ParseInt(stats.TotalContracts, 10, 64)
		if err != nil {
			logger.Warn("Failed to parse total contracts", "value", stats.TotalContracts, "error", err)
			continue
		}

		logger.Info("Got global contract count from Goldsky", "chain_id", chain.ChainID, "chain_name", chain.ChainName, "total_contracts", totalContracts)
		return totalContracts, nil
	}

	return 0, fmt.Errorf("no chains returned valid global statistics")
}

// GetGlobalTransactionCount 获取全局交易数量（从Goldsky GlobalStatistics获取）
func (s *GoldskyService) GetGlobalTransactionCount(ctx context.Context) (int64, error) {
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
		totalTransactions, err := strconv.ParseInt(stats.TotalTransactions, 10, 64)
		if err != nil {
			logger.Warn("Failed to parse total transactions", "value", stats.TotalTransactions, "error", err)
			continue
		}

		logger.Info("Got global transaction count from Goldsky", "chain_id", chain.ChainID, "chain_name", chain.ChainName, "total_transactions", totalTransactions)
		return totalTransactions, nil
	}

	return 0, fmt.Errorf("no chains returned valid global statistics")
}

// SyncAllFlowsNow 手动同步所有链的Flows（用于API调用）
func (s *GoldskyService) SyncAllFlowsNow(ctx context.Context) error {
	logger.Info("Manually syncing all flows from Goldsky...")

	s.mu.RLock()
	clients := make(map[int]*GoldskyClient)
	for chainID, client := range s.clients {
		clients[chainID] = client
	}
	s.mu.RUnlock()

	var wg sync.WaitGroup
	var firstError error
	var mu sync.Mutex

	for chainID, client := range clients {
		wg.Add(1)
		go func(cid int, c *GoldskyClient) {
			defer wg.Done()
			if err := s.syncFlowsForChain(cid, c); err != nil {
				mu.Lock()
				if firstError == nil {
					firstError = fmt.Errorf("failed to sync flows for chain %d: %w", cid, err)
				}
				mu.Unlock()
				logger.Error("Failed to sync flows for chain", err, "chain_id", cid)
			}
		}(chainID, client)
	}

	wg.Wait()

	if firstError != nil {
		return firstError
	}

	logger.Info("Manually synced all flows from Goldsky successfully")
	return nil
}
