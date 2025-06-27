package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"strings"
	"time"

	"timelocker-backend/internal/repository/timelock"
	"timelocker-backend/internal/repository/transaction"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/crypto"
	"timelocker-backend/pkg/logger"
)

// 常量定义
const (
	// CompoundGracePeriodDays Compound timelock的grace period（天数）
	CompoundGracePeriodDays = 14
	// CompoundGracePeriodSeconds Compound timelock的grace period（秒数）
	CompoundGracePeriodSeconds = CompoundGracePeriodDays * 24 * 60 * 60
)

var (
	ErrTransactionNotFound      = errors.New("transaction not found")
	ErrTransactionExists        = errors.New("transaction already exists")
	ErrUnauthorized             = errors.New("unauthorized access")
	ErrInvalidTransactionData   = errors.New("invalid transaction data")
	ErrInvalidETA               = errors.New("invalid ETA")
	ErrTransactionNotReady      = errors.New("transaction not ready for execution")
	ErrTransactionNotCancelable = errors.New("transaction not cancelable")
	ErrTimelockNotFound         = errors.New("timelock not found")
	ErrInsufficientPermissions  = errors.New("insufficient permissions")
)

// Service 交易服务接口
type Service interface {
	// 基础操作
	CreateTransaction(ctx context.Context, userAddress string, req *types.CreateTransactionRequest) (*types.TransactionWithPermission, error)
	GetTransactionList(ctx context.Context, userAddress string, req *types.GetTransactionListRequest) (*types.GetTransactionListResponse, error)
	GetTransactionDetail(ctx context.Context, userAddress string, id int64) (*types.TransactionDetailResponse, error)
	GetPendingTransactions(ctx context.Context, userAddress string, req *types.GetPendingTransactionsRequest) (*types.GetTransactionListResponse, error)

	// 执行和取消操作
	ExecuteTransaction(ctx context.Context, userAddress string, req *types.ExecuteTransactionRequest) error
	CancelTransaction(ctx context.Context, userAddress string, req *types.CancelTransactionRequest) error

	// 统计操作
	GetTransactionStats(ctx context.Context, userAddress string) (*types.TransactionStatsResponse, error)

	// 状态更新（用于区块链监听器）
	UpdateTransactionStatusByTxHash(ctx context.Context, txHash string, status types.TransactionStatus, blockNumber *int64) error
	MarkTransactionFailed(ctx context.Context, transactionID int64, reason string) error
	MarkTransactionSubmitFailed(ctx context.Context, transactionID int64, reason string) error
	RetrySubmitTransaction(ctx context.Context, userAddress string, transactionID int64, newTxHash string) error
	ProcessExpiredTransactions(ctx context.Context) error
	ProcessReadyTransactions(ctx context.Context) error

	// 事件监听器专用方法
	FindTransactionByCompoundParams(ctx context.Context, chainID int, timelockAddress, target, value, functionSig string, eta int64) (*types.Transaction, error)
	FindTransactionByOperationID(ctx context.Context, operationID string) (*types.Transaction, error)
}

type service struct {
	transactionRepo transaction.Repository
	timelockRepo    timelock.Repository
}

// NewService 创建交易服务实例
func NewService(transactionRepo transaction.Repository, timelockRepo timelock.Repository) Service {
	return &service{
		transactionRepo: transactionRepo,
		timelockRepo:    timelockRepo,
	}
}

// CreateTransaction 创建交易记录
func (s *service) CreateTransaction(ctx context.Context, userAddress string, req *types.CreateTransactionRequest) (*types.TransactionWithPermission, error) {
	logger.Info("CreateTransaction: ", "user_address", userAddress, "tx_hash", req.TxHash, "timelock_address", req.TimelockAddress)

	// 标准化地址
	normalizedUser := crypto.NormalizeAddress(userAddress)
	normalizedTimelock := crypto.NormalizeAddress(req.TimelockAddress)
	normalizedTarget := crypto.NormalizeAddress(req.Target)

	// 验证请求参数
	if err := s.validateCreateRequest(req); err != nil {
		logger.Error("CreateTransaction Validation Error: ", err, "user_address", normalizedUser)
		return nil, err
	}

	// 检查交易是否已存在
	exists, err := s.transactionRepo.CheckTransactionExists(ctx, req.TxHash)
	if err != nil {
		return nil, fmt.Errorf("failed to check transaction existence: %w", err)
	}
	if exists {
		return nil, ErrTransactionExists
	}

	// 验证用户对timelock的权限
	canPropose, err := s.checkProposePermission(ctx, normalizedUser, normalizedTimelock, req.TimelockStandard, req.ChainID)
	if err != nil {
		logger.Error("CreateTransaction: failed to check permissions: ", err, "user_address", normalizedUser, "timelock_address", normalizedTimelock)
		return nil, fmt.Errorf("failed to check permissions: %w", err)
	}
	if !canPropose {
		logger.Error("CreateTransaction: insufficient permissions: ", ErrInsufficientPermissions, "user_address", normalizedUser, "timelock_address", normalizedTimelock)
		return nil, ErrInsufficientPermissions
	}

	// 创建交易记录
	tx := &types.Transaction{
		CreatorAddress:   normalizedUser,
		ChainID:          req.ChainID,
		ChainName:        req.ChainName,
		TimelockAddress:  normalizedTimelock,
		TimelockStandard: req.TimelockStandard,
		TxHash:           req.TxHash,
		TxData:           req.TxData,
		Target:           normalizedTarget,
		Value:            s.normalizeValue(req.Value),
		FunctionSig:      req.FunctionSig,
		ETA:              req.ETA,
		Status:           types.TransactionSubmitting, // 初始状态为正在提交
		Description:      html.EscapeString(strings.TrimSpace(req.Description)),
	}

	// 不设置QueuedAt，因为还没有成功提交到timelock合约

	if err := s.transactionRepo.CreateTransaction(ctx, tx); err != nil {
		logger.Error("CreateTransaction: failed to create transaction: ", err, "user_address", normalizedUser, "tx_hash", req.TxHash)
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// 构建带权限信息的响应
	txWithPermission := s.buildTransactionWithPermission(ctx, tx, normalizedUser)

	logger.Info("CreateTransaction Success: ", "transaction_id", tx.ID, "user_address", normalizedUser, "tx_hash", req.TxHash)
	return &txWithPermission, nil
}

// GetTransactionList 获取交易列表
func (s *service) GetTransactionList(ctx context.Context, userAddress string, req *types.GetTransactionListRequest) (*types.GetTransactionListResponse, error) {
	logger.Info("GetTransactionList: ", "user_address", userAddress, "page", req.Page, "page_size", req.PageSize)

	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 从repository获取交易列表
	transactions, total, err := s.transactionRepo.GetTransactionsByUserPermissions(ctx, normalizedUser, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	// 构建带权限信息的交易列表
	txWithPermissions := make([]types.TransactionWithPermission, len(transactions))
	for i, tx := range transactions {
		txWithPermissions[i] = s.buildTransactionWithPermission(ctx, &tx, normalizedUser)
	}

	// 计算总页数
	totalPages := int(math.Ceil(float64(total) / float64(req.PageSize)))

	response := &types.GetTransactionListResponse{
		Transactions: txWithPermissions,
		Total:        total,
		Page:         req.Page,
		PageSize:     req.PageSize,
		TotalPages:   totalPages,
	}

	logger.Info("GetTransactionList Success: ", "user_address", normalizedUser, "total", total, "count", len(txWithPermissions))
	return response, nil
}

// GetTransactionDetail 获取交易详情
func (s *service) GetTransactionDetail(ctx context.Context, userAddress string, id int64) (*types.TransactionDetailResponse, error) {
	logger.Info("GetTransactionDetail: ", "user_address", userAddress, "transaction_id", id)

	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 获取交易
	tx, err := s.transactionRepo.GetTransactionByID(ctx, id)
	if err != nil {
		if err.Error() == "transaction not found" {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// 验证访问权限
	if !s.canAccessTransaction(ctx, normalizedUser, tx) {
		return nil, ErrUnauthorized
	}

	// 构建带权限信息的交易
	txWithPermission := s.buildTransactionWithPermission(ctx, tx, normalizedUser)

	// 获取timelock信息
	timelockInfo, err := s.getTimelockInfo(ctx, tx.TimelockAddress, tx.TimelockStandard, tx.ChainID)
	if err != nil {
		logger.Warn("GetTransactionDetail: failed to get timelock info", "error", err)
		timelockInfo = nil
	}

	response := &types.TransactionDetailResponse{
		TransactionWithPermission: txWithPermission,
		TimelockInfo:              timelockInfo,
	}

	logger.Info("GetTransactionDetail Success: ", "user_address", normalizedUser, "transaction_id", id)
	return response, nil
}

// GetPendingTransactions 获取待处理交易
func (s *service) GetPendingTransactions(ctx context.Context, userAddress string, req *types.GetPendingTransactionsRequest) (*types.GetTransactionListResponse, error) {
	logger.Info("GetPendingTransactions: ", "user_address", userAddress, "only_can_exec", req.OnlyCanExec)

	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 从repository获取待处理交易（已包含权限筛选）
	transactions, total, err := s.transactionRepo.GetPendingTransactions(ctx, normalizedUser, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending transactions: %w", err)
	}

	// 构建带权限信息的交易列表，并根据权限过滤
	var txWithPermissions []types.TransactionWithPermission
	for _, tx := range transactions {
		txWithPermission := s.buildTransactionWithPermission(ctx, &tx, normalizedUser)

		// 如果只显示可执行的交易，检查执行权限
		if req.OnlyCanExec && !txWithPermission.CanExecute {
			continue
		}

		txWithPermissions = append(txWithPermissions, txWithPermission)
	}

	// 如果需要过滤，重新计算总数和分页信息
	var filteredTotal int64
	if req.OnlyCanExec {
		filteredTotal = int64(len(txWithPermissions))
	} else {
		filteredTotal = total
	}
	totalPages := int(math.Ceil(float64(filteredTotal) / float64(req.PageSize)))

	response := &types.GetTransactionListResponse{
		Transactions: txWithPermissions,
		Total:        filteredTotal,
		Page:         req.Page,
		PageSize:     req.PageSize,
		TotalPages:   totalPages,
	}

	logger.Info("GetPendingTransactions Success: ", "user_address", normalizedUser, "total", filteredTotal, "count", len(txWithPermissions))
	return response, nil
}

// ExecuteTransaction 执行交易
func (s *service) ExecuteTransaction(ctx context.Context, userAddress string, req *types.ExecuteTransactionRequest) error {
	logger.Info("ExecuteTransaction: ", "user_address", userAddress, "transaction_id", req.ID, "execute_tx_hash", req.ExecuteTxHash)

	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 获取交易
	tx, err := s.transactionRepo.GetTransactionByID(ctx, req.ID)
	if err != nil {
		if err.Error() == "transaction not found" {
			logger.Error("ExecuteTransaction error: ", ErrTransactionNotFound, "transaction_id", req.ID)
			return ErrTransactionNotFound
		}
		logger.Error("ExecuteTransaction error: ", err, "transaction_id", req.ID)
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// 检查交易状态 - 允许Ready和Failed状态执行
	if tx.Status != types.TransactionReady && tx.Status != types.TransactionFailed {
		logger.Error("ExecuteTransaction: transaction not in executable state", errors.New("transaction not in executable state"), "transaction_id", req.ID, "current_status", tx.Status)
		return fmt.Errorf("transaction is not in executable state (current: %s, expected: ready or failed)", tx.Status)
	}

	// 对于Ready状态，检查ETA是否已到达
	if tx.Status == types.TransactionReady {
		now := time.Now().Unix()
		if tx.ETA > now {
			logger.Error("ExecuteTransaction: ETA not reached", errors.New("ETA not reached"), "transaction_id", req.ID, "eta", tx.ETA, "current_time", now)
			return fmt.Errorf("transaction ETA not reached yet")
		}
	}

	// 检查执行权限
	canExecute, err := s.checkExecutePermission(ctx, normalizedUser, tx.TimelockAddress, tx.TimelockStandard, tx.ChainID)
	if err != nil {
		logger.Error("ExecuteTransaction error: ", err, "transaction_id", req.ID)
		return fmt.Errorf("failed to check execute permission: %w", err)
	}
	if !canExecute {
		logger.Error("ExecuteTransaction error: ", ErrInsufficientPermissions, "transaction_id", req.ID)
		return ErrInsufficientPermissions
	}

	// 更新交易状态为执行中（等待区块链确认）
	now := time.Now()
	if err := s.transactionRepo.UpdateTransactionStatus(ctx, req.ID, types.TransactionExecuting, &now, &req.ExecuteTxHash); err != nil {
		logger.Error("ExecuteTransaction error: ", err, "transaction_id", req.ID)
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	logger.Info("ExecuteTransaction Success: status set to executing, waiting for blockchain confirmation", "transaction_id", req.ID, "user_address", normalizedUser, "execute_tx_hash", req.ExecuteTxHash)
	return nil
}

// CancelTransaction 取消交易
func (s *service) CancelTransaction(ctx context.Context, userAddress string, req *types.CancelTransactionRequest) error {
	logger.Info("CancelTransaction: ", "user_address", userAddress, "transaction_id", req.ID, "cancel_tx_hash", req.CancelTxHash)

	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 获取交易
	tx, err := s.transactionRepo.GetTransactionByID(ctx, req.ID)
	if err != nil {
		if err.Error() == "transaction not found" {
			return ErrTransactionNotFound
		}
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// 检查交易状态 (executing, executed, expired状态不能取消)
	if tx.Status != types.TransactionSubmitting && tx.Status != types.TransactionQueued &&
		tx.Status != types.TransactionReady && tx.Status != types.TransactionFailed &&
		tx.Status != types.TransactionSubmitFailed {
		logger.Error("CancelTransaction: transaction not cancelable", ErrTransactionNotCancelable, "transaction_id", req.ID, "current_status", tx.Status)
		return ErrTransactionNotCancelable
	}

	// 检查取消权限
	canCancel, err := s.checkCancelPermission(ctx, normalizedUser, tx)
	if err != nil {
		return fmt.Errorf("failed to check cancel permission: %w", err)
	}
	if !canCancel {
		return ErrInsufficientPermissions
	}

	// 更新交易状态为已取消
	now := time.Now()
	if err := s.transactionRepo.UpdateTransactionStatus(ctx, req.ID, types.TransactionCanceled, &now, &req.CancelTxHash); err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	logger.Info("CancelTransaction Success: ", "transaction_id", req.ID, "user_address", normalizedUser, "cancel_tx_hash", req.CancelTxHash)
	return nil
}

// GetTransactionStats 获取交易统计
func (s *service) GetTransactionStats(ctx context.Context, userAddress string) (*types.TransactionStatsResponse, error) {
	logger.Info("GetTransactionStats: ", "user_address", userAddress)

	normalizedUser := crypto.NormalizeAddress(userAddress)

	stats, err := s.transactionRepo.GetTransactionStats(ctx, normalizedUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction stats: %w", err)
	}

	logger.Info("GetTransactionStats Success: ", "user_address", normalizedUser, "total", stats.TotalTransactions)
	return stats, nil
}

// UpdateTransactionStatusByTxHash 根据交易哈希更新状态（用于区块链监听器）
func (s *service) UpdateTransactionStatusByTxHash(ctx context.Context, txHash string, status types.TransactionStatus, blockNumber *int64) error {
	logger.Info("UpdateTransactionStatusByTxHash: ", "tx_hash", txHash, "status", status, "block_number", blockNumber)

	// 根据交易哈希查找交易
	tx, err := s.transactionRepo.GetTransactionByTxHash(ctx, txHash)
	if err != nil {
		if err.Error() == "transaction not found" {
			// 交易不存在，可能还未被记录到数据库，忽略
			logger.Warn("UpdateTransactionStatusByTxHash: transaction not found", "tx_hash", txHash)
			return nil
		}
		return fmt.Errorf("failed to get transaction by hash: %w", err)
	}

	// 更新状态
	now := time.Now()
	if err := s.transactionRepo.UpdateTransactionStatus(ctx, tx.ID, status, &now, nil); err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	logger.Info("UpdateTransactionStatusByTxHash Success: ", "transaction_id", tx.ID, "tx_hash", txHash, "status", status)
	return nil
}

// MarkTransactionFailed 标记交易执行失败
func (s *service) MarkTransactionFailed(ctx context.Context, transactionID int64, reason string) error {
	logger.Info("MarkTransactionFailed: ", "transaction_id", transactionID, "reason", reason)

	// 获取交易
	tx, err := s.transactionRepo.GetTransactionByID(ctx, transactionID)
	if err != nil {
		if err.Error() == "transaction not found" {
			return ErrTransactionNotFound
		}
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// 检查当前状态是否为executing
	if tx.Status != types.TransactionExecuting {
		logger.Warn("MarkTransactionFailed: transaction not in executing state", "transaction_id", transactionID, "current_status", tx.Status)
		return fmt.Errorf("transaction is not in executing state")
	}

	// 更新状态为失败
	now := time.Now()
	if err := s.transactionRepo.UpdateTransactionStatus(ctx, transactionID, types.TransactionFailed, &now, nil); err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	logger.Info("MarkTransactionFailed Success: ", "transaction_id", transactionID, "reason", reason)
	return nil
}

// MarkTransactionSubmitFailed 标记交易提交失败
func (s *service) MarkTransactionSubmitFailed(ctx context.Context, transactionID int64, reason string) error {
	logger.Info("MarkTransactionSubmitFailed: ", "transaction_id", transactionID, "reason", reason)

	// 获取交易
	tx, err := s.transactionRepo.GetTransactionByID(ctx, transactionID)
	if err != nil {
		if err.Error() == "transaction not found" {
			return ErrTransactionNotFound
		}
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// 检查当前状态是否为submitting
	if tx.Status != types.TransactionSubmitting {
		logger.Warn("MarkTransactionSubmitFailed: transaction not in submitting state", "transaction_id", transactionID, "current_status", tx.Status)
		return fmt.Errorf("transaction is not in submitting state")
	}

	// 更新状态为提交失败
	now := time.Now()
	if err := s.transactionRepo.UpdateTransactionStatus(ctx, transactionID, types.TransactionSubmitFailed, &now, nil); err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	logger.Info("MarkTransactionSubmitFailed Success: ", "transaction_id", transactionID, "reason", reason)
	return nil
}

// RetrySubmitTransaction 重试提交交易
func (s *service) RetrySubmitTransaction(ctx context.Context, userAddress string, transactionID int64, newTxHash string) error {
	logger.Info("RetrySubmitTransaction: ", "user_address", userAddress, "transaction_id", transactionID, "new_tx_hash", newTxHash)

	normalizedUser := crypto.NormalizeAddress(userAddress)

	// 获取交易
	tx, err := s.transactionRepo.GetTransactionByID(ctx, transactionID)
	if err != nil {
		if err.Error() == "transaction not found" {
			return ErrTransactionNotFound
		}
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// 检查当前状态是否为submit_failed
	if tx.Status != types.TransactionSubmitFailed {
		logger.Warn("RetrySubmitTransaction: transaction not in submit_failed state", "transaction_id", transactionID, "current_status", tx.Status)
		return fmt.Errorf("transaction is not in submit_failed state")
	}

	// 检查权限（只有创建者或有提议权限的用户可以重试）
	if tx.CreatorAddress != normalizedUser {
		canPropose, err := s.checkProposePermission(ctx, normalizedUser, tx.TimelockAddress, tx.TimelockStandard, tx.ChainID)
		if err != nil {
			return fmt.Errorf("failed to check propose permission: %w", err)
		}
		if !canPropose {
			return ErrInsufficientPermissions
		}
	}

	// 验证新交易哈希格式
	if len(newTxHash) != 66 || !strings.HasPrefix(newTxHash, "0x") {
		return fmt.Errorf("%w: invalid transaction hash", ErrInvalidTransactionData)
	}

	// 检查新交易哈希是否已存在
	exists, err := s.transactionRepo.CheckTransactionExists(ctx, newTxHash)
	if err != nil {
		return fmt.Errorf("failed to check transaction existence: %w", err)
	}
	if exists {
		return ErrTransactionExists
	}

	// 更新交易哈希并将状态重置为submitting
	updates := map[string]interface{}{
		"tx_hash": newTxHash,
		"status":  types.TransactionSubmitting,
	}

	if err := s.transactionRepo.UpdateTransactionFields(ctx, transactionID, updates); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	logger.Info("RetrySubmitTransaction Success: ", "transaction_id", transactionID, "user_address", normalizedUser, "new_tx_hash", newTxHash)
	return nil
}

// ProcessExpiredTransactions 处理过期交易
// 注意：只处理Compound标准的交易过期，OpenZeppelin标准没有过期概念
// Compound交易过期规则：当前时间 > ETA + grace period（14天）
func (s *service) ProcessExpiredTransactions(ctx context.Context) error {
	logger.Info("ProcessExpiredTransactions: processing expired transactions (Compound standard only)")

	// 获取过期交易（只包含Compound标准的交易）
	expiredTxs, err := s.transactionRepo.GetExpiredTransactionsNotUpdated(ctx)
	if err != nil {
		logger.Error("ProcessExpiredTransactions: failed to get expired transactions", err)
		return fmt.Errorf("failed to get expired transactions: %w", err)
	}

	// 更新过期交易状态
	for _, tx := range expiredTxs {
		// 验证确实是Compound标准（双重检查）
		if tx.TimelockStandard != types.CompoundStandard {
			logger.Warn("ProcessExpiredTransactions: skipping non-compound transaction", "transaction_id", tx.ID, "standard", tx.TimelockStandard)
			continue
		}

		// 再次检查是否真的过期（防止数据不一致）
		if !s.isCompoundTransactionExpired(&tx) {
			logger.Warn("ProcessExpiredTransactions: transaction not actually expired", "transaction_id", tx.ID, "eta", tx.ETA, "current_time", time.Now().Unix())
			continue
		}

		now := time.Now()
		if err := s.transactionRepo.UpdateTransactionStatus(ctx, tx.ID, types.TransactionExpired, &now, nil); err != nil {
			logger.Error("ProcessExpiredTransactions: failed to update transaction", err, "transaction_id", tx.ID)
			continue
		}
		logger.Info("ProcessExpiredTransactions: marked compound transaction as expired", "transaction_id", tx.ID, "tx_hash", tx.TxHash, "eta", tx.ETA)
	}

	logger.Info("ProcessExpiredTransactions Success: ", "processed_count", len(expiredTxs), "standard", "compound")
	return nil
}

// ProcessReadyTransactions 处理就绪交易
func (s *service) ProcessReadyTransactions(ctx context.Context) error {
	logger.Info("ProcessReadyTransactions: processing ready transactions")

	// 获取就绪交易
	readyTxs, err := s.transactionRepo.GetReadyTransactionsNotUpdated(ctx)
	if err != nil {
		return fmt.Errorf("failed to get ready transactions: %w", err)
	}

	// 更新就绪交易状态
	for _, tx := range readyTxs {
		now := time.Now()
		if err := s.transactionRepo.UpdateTransactionStatus(ctx, tx.ID, types.TransactionReady, &now, nil); err != nil {
			logger.Error("ProcessReadyTransactions: failed to update transaction", err, "transaction_id", tx.ID)
			continue
		}
		logger.Info("ProcessReadyTransactions: marked transaction as ready", "transaction_id", tx.ID, "tx_hash", tx.TxHash)
	}

	logger.Info("ProcessReadyTransactions Success: ", "processed_count", len(readyTxs))
	return nil
}

// 辅助方法

// validateCreateRequest 验证创建交易请求
func (s *service) validateCreateRequest(req *types.CreateTransactionRequest) error {
	// 验证地址格式
	if !crypto.ValidateEthereumAddress(req.TimelockAddress) {
		return fmt.Errorf("%w: invalid timelock address", ErrInvalidTransactionData)
	}
	if !crypto.ValidateEthereumAddress(req.Target) {
		return fmt.Errorf("%w: invalid target address", ErrInvalidTransactionData)
	}

	// 验证交易哈希格式
	if len(req.TxHash) != 66 || !strings.HasPrefix(req.TxHash, "0x") {
		return fmt.Errorf("%w: invalid transaction hash", ErrInvalidTransactionData)
	}

	// 验证ETA
	if req.ETA <= time.Now().Unix() {
		return fmt.Errorf("%w: ETA must be in the future", ErrInvalidETA)
	}

	// 验证交易数据
	if len(req.TxData) == 0 {
		return fmt.Errorf("%w: transaction data cannot be empty", ErrInvalidTransactionData)
	}

	return nil
}

// normalizeValue 标准化value值
func (s *service) normalizeValue(value string) string {
	if value == "" {
		return "0"
	}
	return value
}

// buildTransactionWithPermission 构建带权限信息的交易
func (s *service) buildTransactionWithPermission(ctx context.Context, tx *types.Transaction, userAddress string) types.TransactionWithPermission {
	now := time.Now().Unix()

	// 计算剩余时间
	timeRemaining := tx.ETA - now

	// 生成状态消息
	var statusMessage string
	switch tx.Status {
	case types.TransactionSubmitting:
		statusMessage = "Submitting to timelock contract"
	case types.TransactionQueued:
		if timeRemaining > 0 {
			hours := timeRemaining / 3600
			if hours > 24 {
				days := hours / 24
				statusMessage = fmt.Sprintf("Waiting for %d days to execute", days)
			} else if hours > 0 {
				statusMessage = fmt.Sprintf("Waiting for %d hours to execute", hours)
			} else {
				minutes := timeRemaining / 60
				statusMessage = fmt.Sprintf("Waiting for %d minutes to execute", minutes)
			}
		} else {
			statusMessage = "Ready to execute"
		}
	case types.TransactionReady:
		if timeRemaining <= 0 {
			statusMessage = "Ready to execute"
		} else {
			statusMessage = fmt.Sprintf("Ready to execute in %d seconds", timeRemaining)
		}
	case types.TransactionExecuting:
		statusMessage = "Executing - waiting for blockchain confirmation"
	case types.TransactionExecuted:
		if tx.ExecutedAt != nil {
			statusMessage = fmt.Sprintf("Executed at %s", tx.ExecutedAt.Format("2006-01-02 15:04:05"))
		} else {
			statusMessage = "Executed"
		}
	case types.TransactionFailed:
		statusMessage = "Execution failed - can retry"
	case types.TransactionSubmitFailed:
		statusMessage = "Submit failed - can retry"
	case types.TransactionExpired:
		// 只有Compound标准才会有过期状态
		if tx.TimelockStandard == types.CompoundStandard {
			expiredTime := s.getCompoundTransactionExpiryTime(tx)
			if expiredTime < now {
				expiredDays := (now - expiredTime) / (24 * 60 * 60)
				statusMessage = fmt.Sprintf("Expired %d days (Grace Period: %d days)", expiredDays, CompoundGracePeriodDays)
			} else {
				statusMessage = "Expired"
			}
		} else {
			// OpenZeppelin标准不应该有过期状态，这里是异常情况
			statusMessage = "Status error"
		}
	case types.TransactionCanceled:
		if tx.CanceledAt != nil {
			statusMessage = fmt.Sprintf("Canceled at %s", tx.CanceledAt.Format("2006-01-02 15:04:05"))
		} else {
			statusMessage = "Canceled"
		}
	}

	// 确定用户权限
	var userPermissions []string
	if tx.CreatorAddress == userAddress {
		userPermissions = append(userPermissions, "creator")
	}

	// 检查用户在该timelock中的权限
	timelockPermissions := s.getUserTimelockPermissions(ctx, userAddress, tx.TimelockAddress, tx.TimelockStandard, tx.ChainID)
	userPermissions = append(userPermissions, timelockPermissions...)

	// 检查执行权限
	canExecute := false
	// Ready状态：ETA已到达可以执行
	// Failed状态：执行失败后可以重新尝试执行
	if (tx.Status == types.TransactionReady && timeRemaining <= 0) || tx.Status == types.TransactionFailed {
		canExecute, _ = s.checkExecutePermission(ctx, userAddress, tx.TimelockAddress, tx.TimelockStandard, tx.ChainID)
	}

	// 检查重试提交权限（用于submit_failed状态）
	canRetrySubmit := false
	if tx.Status == types.TransactionSubmitFailed {
		// 创建者总是可以重试提交
		if tx.CreatorAddress == userAddress {
			canRetrySubmit = true
		} else {
			// 有提议权限的用户也可以重试提交
			canRetrySubmit, _ = s.checkProposePermission(ctx, userAddress, tx.TimelockAddress, tx.TimelockStandard, tx.ChainID)
		}
	}

	// 检查取消权限
	canCancel := false
	// Submitting, Queued, Ready, Failed, SubmitFailed状态都可以取消，但executing, executed, expired不能取消
	if tx.Status == types.TransactionSubmitting || tx.Status == types.TransactionQueued ||
		tx.Status == types.TransactionReady || tx.Status == types.TransactionFailed ||
		tx.Status == types.TransactionSubmitFailed {
		canCancel, _ = s.checkCancelPermission(ctx, userAddress, tx)
	}

	return types.TransactionWithPermission{
		Transaction:     *tx,
		UserPermissions: userPermissions,
		CanExecute:      canExecute,
		CanCancel:       canCancel,
		CanRetrySubmit:  canRetrySubmit,
		TimeRemaining:   timeRemaining,
		StatusMessage:   statusMessage,
	}
}

// getUserTimelockPermissions 获取用户在timelock中的权限
func (s *service) getUserTimelockPermissions(ctx context.Context, userAddress, timelockAddress string, standard types.TimeLockStandard, chainID int) []string {
	var permissions []string

	switch standard {
	case types.CompoundStandard:
		var timelock types.CompoundTimeLock
		err := s.timelockRepo.GetCompoundTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
		if err != nil {
			return permissions
		}

		if timelock.CreatorAddress == userAddress {
			permissions = append(permissions, "timelock_creator")
		}
		if timelock.Admin == userAddress {
			permissions = append(permissions, "admin")
		}
		if timelock.PendingAdmin != nil && *timelock.PendingAdmin == userAddress {
			permissions = append(permissions, "pending_admin")
		}

	case types.OpenzeppelinStandard:
		var timelock types.OpenzeppelinTimeLock
		err := s.timelockRepo.GetOpenzeppelinTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
		if err != nil {
			return permissions
		}

		if timelock.CreatorAddress == userAddress {
			permissions = append(permissions, "timelock_creator")
		}
		if s.containsAddress(timelock.Proposers, userAddress) {
			permissions = append(permissions, "proposer")
		}
		if s.containsAddress(timelock.Executors, userAddress) {
			permissions = append(permissions, "executor")
		}
		if s.containsAddress(timelock.Cancellers, userAddress) {
			permissions = append(permissions, "canceller")
		}
	}

	return permissions
}

// isCompoundTransactionExpired 检查Compound交易是否过期
func (s *service) isCompoundTransactionExpired(tx *types.Transaction) bool {
	if tx.TimelockStandard != types.CompoundStandard {
		return false
	}

	now := time.Now().Unix()
	expiryTime := tx.ETA + CompoundGracePeriodSeconds
	return now > expiryTime
}

// getCompoundTransactionExpiryTime 获取Compound交易的过期时间
func (s *service) getCompoundTransactionExpiryTime(tx *types.Transaction) int64 {
	if tx.TimelockStandard != types.CompoundStandard {
		return 0
	}
	return tx.ETA + CompoundGracePeriodSeconds
}

// canAccessTransaction 检查用户是否可以访问交易
func (s *service) canAccessTransaction(ctx context.Context, userAddress string, tx *types.Transaction) bool {
	// 创建者总是可以访问
	if tx.CreatorAddress == userAddress {
		return true
	}

	// 检查用户是否对相关timelock有权限
	switch tx.TimelockStandard {
	case types.CompoundStandard:
		// Compound: 检查是否为admin或pending admin
		hasPermission, _ := s.checkCompoundTimelockAccess(ctx, userAddress, tx.TimelockAddress, tx.ChainID)
		return hasPermission
	case types.OpenzeppelinStandard:
		// OpenZeppelin: 检查是否为proposer、executor或canceller
		hasPermission, _ := s.checkOpenZeppelinTimelockAccess(ctx, userAddress, tx.TimelockAddress, tx.ChainID)
		return hasPermission
	default:
		return false
	}
}

// checkProposePermission 检查提议权限
func (s *service) checkProposePermission(ctx context.Context, userAddress, timelockAddress string, standard types.TimeLockStandard, chainID int) (bool, error) {
	switch standard {
	case types.CompoundStandard:
		// Compound: 检查是否为admin
		return s.checkCompoundAdminPermission(ctx, userAddress, timelockAddress, chainID)
	case types.OpenzeppelinStandard:
		// OpenZeppelin: 检查是否为proposer
		return s.checkOpenZeppelinProposerPermission(ctx, userAddress, timelockAddress, chainID)
	default:
		logger.Error("checkProposePermission error: ", fmt.Errorf("unsupported timelock standard: %s", standard))
		return false, fmt.Errorf("unsupported timelock standard: %s", standard)
	}
}

// checkExecutePermission 检查执行权限
func (s *service) checkExecutePermission(ctx context.Context, userAddress, timelockAddress string, standard types.TimeLockStandard, chainID int) (bool, error) {
	switch standard {
	case types.CompoundStandard:
		// Compound: 检查是否为admin
		return s.checkCompoundAdminPermission(ctx, userAddress, timelockAddress, chainID)
	case types.OpenzeppelinStandard:
		// OpenZeppelin: 检查是否为executor
		return s.checkOpenZeppelinExecutorPermission(ctx, userAddress, timelockAddress, chainID)
	default:
		logger.Error("checkExecutePermission error: ", fmt.Errorf("unsupported timelock standard: %s", standard))
		return false, fmt.Errorf("unsupported timelock standard: %s", standard)
	}
}

// checkCancelPermission 检查取消权限
func (s *service) checkCancelPermission(ctx context.Context, userAddress string, tx *types.Transaction) (bool, error) {
	// 创建者总是可以取消
	if tx.CreatorAddress == userAddress {
		return true, nil
	}

	// 根据timelock标准检查具体权限
	switch tx.TimelockStandard {
	case types.CompoundStandard:
		// Compound: 检查是否为admin
		return s.checkCompoundAdminPermission(ctx, userAddress, tx.TimelockAddress, tx.ChainID)
	case types.OpenzeppelinStandard:
		// OpenZeppelin: 检查是否为canceller
		return s.checkOpenZeppelinCancellerPermission(ctx, userAddress, tx.TimelockAddress, tx.ChainID)
	default:
		logger.Error("checkCancelPermission error: ", fmt.Errorf("unsupported timelock standard: %s", tx.TimelockStandard))
		return false, fmt.Errorf("unsupported timelock standard: %s", tx.TimelockStandard)
	}
}

// getTimelockInfo 获取timelock信息
func (s *service) getTimelockInfo(ctx context.Context, timelockAddress string, standard types.TimeLockStandard, chainID int) (interface{}, error) {
	switch standard {
	case types.CompoundStandard:
		// 查询Compound timelock信息
		var timelock types.CompoundTimeLock
		err := s.timelockRepo.GetCompoundTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
		if err != nil {
			logger.Error("getTimelockInfo error: ", err, "timelock_address", timelockAddress)
			return nil, err
		}
		return timelock, nil
	case types.OpenzeppelinStandard:
		// 查询OpenZeppelin timelock信息
		var timelock types.OpenzeppelinTimeLock
		err := s.timelockRepo.GetOpenzeppelinTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
		if err != nil {
			logger.Error("getTimelockInfo error: ", err, "timelock_address", timelockAddress)
			return nil, err
		}
		return timelock, nil
	default:
		logger.Error("getTimelockInfo error: ", fmt.Errorf("unsupported timelock standard: %s", standard))
		return nil, fmt.Errorf("unsupported timelock standard: %s", standard)
	}
}

// checkCompoundAdminPermission 检查Compound admin权限
func (s *service) checkCompoundAdminPermission(ctx context.Context, userAddress, timelockAddress string, chainID int) (bool, error) {
	var timelock types.CompoundTimeLock
	err := s.timelockRepo.GetCompoundTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
	if err != nil {
		logger.Error("checkCompoundAdminPermission error: ", err, "user_address", userAddress, "timelock_address", timelockAddress)
		return false, err
	}

	// 检查是否为admin
	return timelock.Admin == userAddress, nil
}

// checkCompoundTimelockAccess 检查Compound timelock访问权限
func (s *service) checkCompoundTimelockAccess(ctx context.Context, userAddress, timelockAddress string, chainID int) (bool, error) {
	var timelock types.CompoundTimeLock
	err := s.timelockRepo.GetCompoundTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
	if err != nil {
		logger.Error("checkCompoundTimelockAccess error: ", err, "user_address", userAddress, "timelock_address", timelockAddress)
		return false, err
	}

	// 检查是否为创建者、admin或pending admin
	if timelock.CreatorAddress == userAddress ||
		timelock.Admin == userAddress ||
		(timelock.PendingAdmin != nil && *timelock.PendingAdmin == userAddress) {
		return true, nil
	}

	return false, nil
}

// checkOpenZeppelinProposerPermission 检查OpenZeppelin proposer权限
func (s *service) checkOpenZeppelinProposerPermission(ctx context.Context, userAddress, timelockAddress string, chainID int) (bool, error) {
	var timelock types.OpenzeppelinTimeLock
	err := s.timelockRepo.GetOpenzeppelinTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
	if err != nil {
		logger.Error("checkOpenZeppelinProposerPermission error: ", err, "user_address", userAddress, "timelock_address", timelockAddress)
		return false, err
	}

	// 检查是否在proposers列表中
	return s.containsAddress(timelock.Proposers, userAddress), nil
}

// checkOpenZeppelinExecutorPermission 检查OpenZeppelin executor权限
func (s *service) checkOpenZeppelinExecutorPermission(ctx context.Context, userAddress, timelockAddress string, chainID int) (bool, error) {
	var timelock types.OpenzeppelinTimeLock
	err := s.timelockRepo.GetOpenzeppelinTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
	if err != nil {
		logger.Error("checkOpenZeppelinExecutorPermission error: ", err, "user_address", userAddress, "timelock_address", timelockAddress)
		return false, err
	}

	// 检查是否在executors列表中
	return s.containsAddress(timelock.Executors, userAddress), nil
}

// checkOpenZeppelinCancellerPermission 检查OpenZeppelin canceller权限
func (s *service) checkOpenZeppelinCancellerPermission(ctx context.Context, userAddress, timelockAddress string, chainID int) (bool, error) {
	var timelock types.OpenzeppelinTimeLock
	err := s.timelockRepo.GetOpenzeppelinTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
	if err != nil {
		logger.Error("checkOpenZeppelinCancellerPermission error: ", err, "user_address", userAddress, "timelock_address", timelockAddress)
		return false, err
	}

	// 检查是否在cancellers列表中
	return s.containsAddress(timelock.Cancellers, userAddress), nil
}

// checkOpenZeppelinTimelockAccess 检查OpenZeppelin timelock访问权限
func (s *service) checkOpenZeppelinTimelockAccess(ctx context.Context, userAddress, timelockAddress string, chainID int) (bool, error) {
	var timelock types.OpenzeppelinTimeLock
	err := s.timelockRepo.GetOpenzeppelinTimeLockByAddress(ctx, chainID, timelockAddress, &timelock)
	if err != nil {
		logger.Error("checkOpenZeppelinTimelockAccess error: ", err, "user_address", userAddress, "timelock_address", timelockAddress)
		return false, err
	}

	// 检查是否为创建者或在任何权限列表中
	if timelock.CreatorAddress == userAddress ||
		s.containsAddress(timelock.Proposers, userAddress) ||
		s.containsAddress(timelock.Executors, userAddress) ||
		s.containsAddress(timelock.Cancellers, userAddress) {
		return true, nil
	}

	return false, nil
}

// containsAddress 检查地址是否在JSON地址列表中
func (s *service) containsAddress(jsonAddresses string, address string) bool {
	var addresses []string
	if err := json.Unmarshal([]byte(jsonAddresses), &addresses); err != nil {
		return false
	}

	for _, addr := range addresses {
		if strings.EqualFold(addr, address) {
			return true
		}
	}
	return false
}

// FindTransactionByCompoundParams 根据Compound参数查找交易（用于事件监听器）
func (s *service) FindTransactionByCompoundParams(ctx context.Context, chainID int, timelockAddress, target, value, functionSig string, eta int64) (*types.Transaction, error) {
	logger.Info("FindTransactionByCompoundParams: ",
		"chain_id", chainID,
		"timelock_address", timelockAddress,
		"target", target,
		"value", value,
		"function_sig", functionSig,
		"eta", eta)

	// 标准化地址
	normalizedTimelock := crypto.NormalizeAddress(timelockAddress)
	normalizedTarget := crypto.NormalizeAddress(target)
	normalizedValue := s.normalizeValue(value)

	// 在数据库中查找匹配的交易
	transactions, err := s.transactionRepo.FindTransactionsByParams(ctx, &transaction.FindTransactionParams{
		ChainID:          &chainID,
		TimelockAddress:  &normalizedTimelock,
		TimelockStandard: (*types.TimeLockStandard)(stringPtr("compound")),
		Target:           &normalizedTarget,
		Value:            &normalizedValue,
		FunctionSig:      &functionSig,
		ETA:              &eta,
	})

	if err != nil {
		logger.Error("FindTransactionByCompoundParams error: ", err)
		return nil, fmt.Errorf("failed to find transaction by compound params: %w", err)
	}

	if len(transactions) == 0 {
		logger.Error("FindTransactionByCompoundParams Error: ", fmt.Errorf("no matching transaction found"))
		return nil, nil
	}

	if len(transactions) > 1 {
		logger.Warn("FindTransactionByCompoundParams: multiple transactions found", "count", len(transactions))
	}

	// 返回第一个匹配的交易
	tx := &transactions[0]
	logger.Info("FindTransactionByCompoundParams: found transaction", "transaction_id", tx.ID, "tx_hash", tx.TxHash)
	return tx, nil
}

// FindTransactionByOperationID 根据操作ID查找交易（用于事件监听器）
func (s *service) FindTransactionByOperationID(ctx context.Context, operationID string) (*types.Transaction, error) {
	logger.Info("FindTransactionByOperationID: ", "operation_id", operationID)

	// 调用repository方法查找交易
	tx, err := s.transactionRepo.GetTransactionByOperationID(ctx, operationID)
	if err != nil {
		if err.Error() == "transaction not found" {
			logger.Error("FindTransactionByOperationID Error: ", fmt.Errorf("transaction not found"), "operation_id", operationID)
			return nil, nil
		}
		logger.Error("FindTransactionByOperationID error: ", err)
		return nil, fmt.Errorf("failed to find transaction by operation id: %w", err)
	}

	logger.Info("FindTransactionByOperationID: found transaction", "transaction_id", tx.ID, "tx_hash", tx.TxHash)
	return tx, nil
}

// stringPtr 返回字符串指针的辅助函数
func stringPtr(s string) *string {
	return &s
}
