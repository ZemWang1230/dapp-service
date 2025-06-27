package transaction

import (
	"context"
	"fmt"
	"strings"
	"time"

	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// FindTransactionParams 查找交易的参数结构
type FindTransactionParams struct {
	ChainID          *int                    `json:"chain_id"`
	TimelockAddress  *string                 `json:"timelock_address"`
	TimelockStandard *types.TimeLockStandard `json:"timelock_standard"`
	Target           *string                 `json:"target"`
	Value            *string                 `json:"value"`
	FunctionSig      *string                 `json:"function_sig"`
	ETA              *int64                  `json:"eta"`
}

// Repository 交易仓库接口
type Repository interface {
	// 基础CRUD操作
	CreateTransaction(ctx context.Context, tx *types.Transaction) error
	GetTransactionByID(ctx context.Context, id int64) (*types.Transaction, error)
	GetTransactionByTxHash(ctx context.Context, txHash string) (*types.Transaction, error)
	UpdateTransactionStatus(ctx context.Context, id int64, status types.TransactionStatus, updatedAt *time.Time, txHash *string) error
	DeleteTransaction(ctx context.Context, id int64) error

	// 查询操作
	GetTransactionsByUserPermissions(ctx context.Context, userAddress string, req *types.GetTransactionListRequest) ([]types.Transaction, int64, error)
	GetPendingTransactions(ctx context.Context, userAddress string, req *types.GetPendingTransactionsRequest) ([]types.Transaction, int64, error)
	GetTransactionsByTimelock(ctx context.Context, timelockAddress string, chainID int) ([]types.Transaction, error)
	GetExpiredTransactionsNotUpdated(ctx context.Context) ([]types.Transaction, error)
	GetReadyTransactionsNotUpdated(ctx context.Context) ([]types.Transaction, error)

	// 统计操作
	GetTransactionStats(ctx context.Context, userAddress string) (*types.TransactionStatsResponse, error)
	GetTransactionStatsByTimelock(ctx context.Context, timelockAddress string, chainID int) (*types.TransactionStatsResponse, error)

	// 状态检查
	CheckTransactionExists(ctx context.Context, txHash string) (bool, error)

	// 批量更新
	UpdateTransactionFields(ctx context.Context, id int64, updates map[string]interface{}) error

	// 事件监听器专用查询方法
	FindTransactionsByParams(ctx context.Context, params *FindTransactionParams) ([]types.Transaction, error)
	GetTransactionByOperationID(ctx context.Context, operationID string) (*types.Transaction, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository 创建交易仓库实例
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// CreateTransaction 创建交易记录
func (r *repository) CreateTransaction(ctx context.Context, tx *types.Transaction) error {
	if err := r.db.WithContext(ctx).Create(tx).Error; err != nil {
		logger.Error("CreateTransaction Error: ", err, "creator_address", tx.CreatorAddress, "tx_hash", tx.TxHash)
		return err
	}

	logger.Info("CreateTransaction: ", "transaction_id", tx.ID, "creator_address", tx.CreatorAddress, "tx_hash", tx.TxHash)
	return nil
}

// GetTransactionByID 根据ID获取交易
func (r *repository) GetTransactionByID(ctx context.Context, id int64) (*types.Transaction, error) {
	var tx types.Transaction
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&tx).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("transaction not found")
		}
		logger.Error("GetTransactionByID Error: ", err, "transaction_id", id)
		return nil, err
	}

	logger.Info("GetTransactionByID: ", "transaction_id", tx.ID, "creator_address", tx.CreatorAddress)
	return &tx, nil
}

// GetTransactionByTxHash 根据交易哈希获取交易
func (r *repository) GetTransactionByTxHash(ctx context.Context, txHash string) (*types.Transaction, error) {
	var tx types.Transaction
	err := r.db.WithContext(ctx).
		Where("tx_hash = ?", txHash).
		First(&tx).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("transaction not found")
		}
		logger.Error("GetTransactionByTxHash Error: ", err, "tx_hash", txHash)
		return nil, err
	}

	logger.Info("GetTransactionByTxHash: ", "transaction_id", tx.ID, "tx_hash", txHash)
	return &tx, nil
}

// UpdateTransactionStatus 更新交易状态
func (r *repository) UpdateTransactionStatus(ctx context.Context, id int64, status types.TransactionStatus, updatedAt *time.Time, txHash *string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// 根据状态设置对应的时间字段
	switch status {
	case types.TransactionQueued:
		if updatedAt != nil {
			updates["queued_at"] = *updatedAt
		}
	case types.TransactionExecuted:
		if updatedAt != nil {
			updates["executed_at"] = *updatedAt
		}
	case types.TransactionCanceled:
		if updatedAt != nil {
			updates["canceled_at"] = *updatedAt
		}
	}

	if err := r.db.WithContext(ctx).
		Model(&types.Transaction{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		logger.Error("UpdateTransactionStatus Error: ", err, "transaction_id", id, "status", status)
		return err
	}

	logger.Info("UpdateTransactionStatus: ", "transaction_id", id, "status", status)
	return nil
}

// DeleteTransaction 删除交易记录
func (r *repository) DeleteTransaction(ctx context.Context, id int64) error {
	if err := r.db.WithContext(ctx).
		Delete(&types.Transaction{}, id).Error; err != nil {
		logger.Error("DeleteTransaction Error: ", err, "transaction_id", id)
		return err
	}

	logger.Info("DeleteTransaction: ", "transaction_id", id)
	return nil
}

// GetTransactionsByUserPermissions 根据用户权限获取交易列表
func (r *repository) GetTransactionsByUserPermissions(ctx context.Context, userAddress string, req *types.GetTransactionListRequest) ([]types.Transaction, int64, error) {
	// 先获取用户有权限的timelock地址列表
	authorizedTimelocks, err := r.getAuthorizedTimelocks(ctx, userAddress)
	if err != nil {
		logger.Error("GetTransactionsByUserPermissions GetAuthorizedTimelocks Error: ", err, "user_address", userAddress)
		return nil, 0, err
	}

	query := r.db.WithContext(ctx).Model(&types.Transaction{})

	// 构建权限查询条件：
	// 1. 用户是交易创建者
	// 2. 交易属于用户有权限的timelock
	if len(authorizedTimelocks) > 0 {
		// 构建timelock条件
		var timelockConditions []string
		var args []interface{}

		for _, tl := range authorizedTimelocks {
			timelockConditions = append(timelockConditions, "(timelock_address = ? AND chain_id = ? AND timelock_standard = ?)")
			args = append(args, tl.Address, tl.ChainID, tl.Standard)
		}

		timelockCondition := "(" + strings.Join(timelockConditions, " OR ") + ")"
		query = query.Where("creator_address = ? OR "+timelockCondition, append([]interface{}{userAddress}, args...)...)
	}

	// 添加筛选条件
	if req.ChainID != nil {
		query = query.Where("chain_id = ?", *req.ChainID)
	}
	if req.TimelockAddress != nil {
		query = query.Where("timelock_address = ?", *req.TimelockAddress)
	}
	if req.TimelockStandard != nil {
		query = query.Where("timelock_standard = ?", *req.TimelockStandard)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logger.Error("GetTransactionsByUserPermissions Count Error: ", err, "user_address", userAddress)
		return nil, 0, err
	}

	// 分页
	offset := (req.Page - 1) * req.PageSize
	var transactions []types.Transaction
	if err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(req.PageSize).
		Find(&transactions).Error; err != nil {
		logger.Error("GetTransactionsByUserPermissions Query Error: ", err, "user_address", userAddress)
		return nil, 0, err
	}

	logger.Info("GetTransactionsByUserPermissions: ", "user_address", userAddress, "total", total, "count", len(transactions))
	return transactions, total, nil
}

// TimelockInfo 存储timelock基本信息
type TimelockInfo struct {
	Address  string
	ChainID  int
	Standard string
}

// getAuthorizedTimelocks 获取用户有权限的timelock列表
func (r *repository) getAuthorizedTimelocks(ctx context.Context, userAddress string) ([]TimelockInfo, error) {
	var result []TimelockInfo

	// 查询Compound timelocks
	var compoundTimelocks []struct {
		ContractAddress string `gorm:"column:contract_address"`
		ChainID         int    `gorm:"column:chain_id"`
	}

	if err := r.db.WithContext(ctx).
		Table("compound_timelocks").
		Select("contract_address, chain_id").
		Where("status != 'deleted' AND (creator_address = ? OR admin = ? OR pending_admin = ?)",
			userAddress, userAddress, userAddress).
		Find(&compoundTimelocks).Error; err != nil {
		logger.Error("getAuthorizedTimelocks Compound Error: ", err, "user_address", userAddress)
		return nil, err
	}

	// 添加Compound timelocks到结果
	for _, tl := range compoundTimelocks {
		result = append(result, TimelockInfo{
			Address:  tl.ContractAddress,
			ChainID:  tl.ChainID,
			Standard: "compound",
		})
	}

	// 查询OpenZeppelin timelocks
	var openzeppelinTimelocks []struct {
		ContractAddress string `gorm:"column:contract_address"`
		ChainID         int    `gorm:"column:chain_id"`
	}

	if err := r.db.WithContext(ctx).
		Table("openzeppelin_timelocks").
		Select("contract_address, chain_id").
		Where("status != 'deleted' AND (creator_address = ? OR proposers LIKE ? OR executors LIKE ? OR cancellers LIKE ?)",
			userAddress, "%"+userAddress+"%", "%"+userAddress+"%", "%"+userAddress+"%").
		Find(&openzeppelinTimelocks).Error; err != nil {
		logger.Error("getAuthorizedTimelocks OpenZeppelin Error: ", err, "user_address", userAddress)
		return nil, err
	}

	// 添加OpenZeppelin timelocks到结果
	for _, tl := range openzeppelinTimelocks {
		result = append(result, TimelockInfo{
			Address:  tl.ContractAddress,
			ChainID:  tl.ChainID,
			Standard: "openzeppelin",
		})
	}

	logger.Info("getAuthorizedTimelocks: ", "user_address", userAddress, "count", len(result))
	return result, nil
}

// GetPendingTransactions 获取待处理交易
// 查询条件：status IN (submitting, queued, ready, executing, failed, submit_failed)
func (r *repository) GetPendingTransactions(ctx context.Context, userAddress string, req *types.GetPendingTransactionsRequest) ([]types.Transaction, int64, error) {
	// 先获取用户有权限的timelock地址列表
	authorizedTimelocks, err := r.getAuthorizedTimelocks(ctx, userAddress)
	if err != nil {
		logger.Error("GetPendingTransactions GetAuthorizedTimelocks Error: ", err, "user_address", userAddress)
		return nil, 0, err
	}

	query := r.db.WithContext(ctx).Model(&types.Transaction{})

	// 只查询待处理状态的交易（submitting, queued, ready, executing, failed, submit_failed）
	query = query.Where("status IN (?)", []types.TransactionStatus{
		types.TransactionSubmitting,
		types.TransactionQueued,
		types.TransactionReady,
		types.TransactionExecuting,
		types.TransactionFailed,
		types.TransactionSubmitFailed,
	})

	// 权限过滤：用户必须是交易创建者或对相关timelock有权限
	if len(authorizedTimelocks) > 0 {
		// 构建timelock条件
		var timelockConditions []string
		var args []interface{}

		for _, tl := range authorizedTimelocks {
			timelockConditions = append(timelockConditions, "(timelock_address = ? AND chain_id = ? AND timelock_standard = ?)")
			args = append(args, tl.Address, tl.ChainID, tl.Standard)
		}

		timelockCondition := "(" + strings.Join(timelockConditions, " OR ") + ")"
		query = query.Where("creator_address = ? OR "+timelockCondition, append([]interface{}{userAddress}, args...)...)
	}

	// 添加筛选条件
	if req.ChainID != nil {
		query = query.Where("chain_id = ?", *req.ChainID)
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logger.Error("GetPendingTransactions Count Error: ", err, "user_address", userAddress)
		return nil, 0, err
	}

	// 分页
	offset := (req.Page - 1) * req.PageSize
	var transactions []types.Transaction
	if err := query.
		Order("eta ASC"). // 按ETA排序，最快到期的排在前面
		Offset(offset).
		Limit(req.PageSize).
		Find(&transactions).Error; err != nil {
		logger.Error("GetPendingTransactions Query Error: ", err, "user_address", userAddress)
		return nil, 0, err
	}

	logger.Info("GetPendingTransactions: ", "user_address", userAddress, "total", total, "count", len(transactions))
	return transactions, total, nil
}

// GetTransactionsByTimelock 获取指定timelock的所有交易
func (r *repository) GetTransactionsByTimelock(ctx context.Context, timelockAddress string, chainID int) ([]types.Transaction, error) {
	var transactions []types.Transaction
	err := r.db.WithContext(ctx).
		Where("timelock_address = ? AND chain_id = ?", timelockAddress, chainID).
		Order("created_at DESC").
		Find(&transactions).Error

	if err != nil {
		logger.Error("GetTransactionsByTimelock Error: ", err, "timelock_address", timelockAddress, "chain_id", chainID)
		return nil, err
	}

	logger.Info("GetTransactionsByTimelock: ", "timelock_address", timelockAddress, "chain_id", chainID, "count", len(transactions))
	return transactions, nil
}

// GetExpiredTransactionsNotUpdated 获取已过期但状态仍为queued或ready的交易
// 注意：只有Compound标准的交易会过期，OpenZeppelin标准没有过期概念
// Compound过期规则：当前时间 > ETA + grace period（14天）
func (r *repository) GetExpiredTransactionsNotUpdated(ctx context.Context) ([]types.Transaction, error) {
	currentTime := time.Now().Unix()
	gracePeriod := int64(14 * 24 * 60 * 60) // 14天的秒数，与service层保持一致
	// 查询条件：ETA + grace period < 当前时间，即 ETA < 当前时间 - grace period
	expiryThreshold := currentTime - gracePeriod

	var transactions []types.Transaction

	// 只查询Compound标准的交易，且ETA + grace period < 当前时间
	// 注意：executing状态的交易不应该被标记为过期，应该等待区块链确认结果
	err := r.db.WithContext(ctx).
		Where("timelock_standard = ? AND eta < ? AND status IN (?)",
			"compound", expiryThreshold, []types.TransactionStatus{
				types.TransactionQueued,
				types.TransactionReady,
			}).
		Find(&transactions).Error

	if err != nil {
		logger.Error("GetExpiredTransactions Error: ", err)
		return nil, err
	}

	logger.Info("GetExpiredTransactions: ", "compound_standard_only", true, "grace_period_days", 14, "count", len(transactions))
	return transactions, nil
}

// GetReadyTransactionsNotUpdated 获取就绪的交易（ETA已到但未执行）
// 只查询queued状态的交易，executing状态的交易已经在处理中，不需要重复处理
func (r *repository) GetReadyTransactionsNotUpdated(ctx context.Context) ([]types.Transaction, error) {
	currentTime := time.Now().Unix()
	var transactions []types.Transaction

	err := r.db.WithContext(ctx).
		Where("eta <= ? AND status = ?", currentTime, types.TransactionQueued).
		Find(&transactions).Error

	if err != nil {
		logger.Error("GetReadyTransactions Error: ", err)
		return nil, err
	}

	logger.Info("GetReadyTransactions: ", "count", len(transactions))
	return transactions, nil
}

// GetTransactionStats 获取用户交易统计
func (r *repository) GetTransactionStats(ctx context.Context, userAddress string) (*types.TransactionStatsResponse, error) {
	// 先获取用户有权限的timelock地址列表
	authorizedTimelocks, err := r.getAuthorizedTimelocks(ctx, userAddress)
	if err != nil {
		logger.Error("GetTransactionStats GetAuthorizedTimelocks Error: ", err, "user_address", userAddress)
		return nil, err
	}

	var stats types.TransactionStatsResponse

	// 构建权限查询条件
	query := r.db.WithContext(ctx).Model(&types.Transaction{})

	if len(authorizedTimelocks) > 0 {
		// 构建timelock条件
		var timelockConditions []string
		var args []interface{}

		for _, tl := range authorizedTimelocks {
			timelockConditions = append(timelockConditions, "(timelock_address = ? AND chain_id = ? AND timelock_standard = ?)")
			args = append(args, tl.Address, tl.ChainID, tl.Standard)
		}

		timelockCondition := "(" + strings.Join(timelockConditions, " OR ") + ")"
		query = query.Where("creator_address = ? OR "+timelockCondition, append([]interface{}{userAddress}, args...)...)
	}

	// 总交易数
	if err := query.Count(&stats.TotalTransactions).Error; err != nil {
		logger.Error("GetTransactionStats Total Error: ", err, "user_address", userAddress)
		return nil, err
	}

	// 各状态统计
	var statusCounts []struct {
		Status types.TransactionStatus
		Count  int64
	}

	// 重新构建查询（因为Count会修改query）
	if len(authorizedTimelocks) > 0 {
		var timelockConditions []string
		var args []interface{}

		for _, tl := range authorizedTimelocks {
			timelockConditions = append(timelockConditions, "(timelock_address = ? AND chain_id = ? AND timelock_standard = ?)")
			args = append(args, tl.Address, tl.ChainID, tl.Standard)
		}

		timelockCondition := "(" + strings.Join(timelockConditions, " OR ") + ")"
		query = r.db.WithContext(ctx).Model(&types.Transaction{}).
			Where("creator_address = ? OR "+timelockCondition, append([]interface{}{userAddress}, args...)...)
	} else {
		query = r.db.WithContext(ctx).Model(&types.Transaction{}).
			Where("creator_address = ?", userAddress)
	}

	if err := query.
		Select("status, count(*) as count").
		Group("status").
		Find(&statusCounts).Error; err != nil {
		logger.Error("GetTransactionStats Status Error: ", err, "user_address", userAddress)
		return nil, err
	}

	// 分配到对应字段
	for _, sc := range statusCounts {
		switch sc.Status {
		case types.TransactionSubmitting:
			stats.SubmittingCount = sc.Count
		case types.TransactionQueued:
			stats.QueuedCount = sc.Count
		case types.TransactionReady:
			stats.ReadyCount = sc.Count
		case types.TransactionExecuting:
			stats.ExecutingCount = sc.Count
		case types.TransactionExecuted:
			stats.ExecutedCount = sc.Count
		case types.TransactionFailed:
			stats.FailedCount = sc.Count
		case types.TransactionSubmitFailed:
			stats.SubmitFailedCount = sc.Count
		case types.TransactionExpired:
			stats.ExpiredCount = sc.Count
		case types.TransactionCanceled:
			stats.CanceledCount = sc.Count
		}
	}

	logger.Info("GetTransactionStats: ", "user_address", userAddress, "total", stats.TotalTransactions)
	return &stats, nil
}

// GetTransactionStatsByTimelock 获取指定timelock的交易统计
func (r *repository) GetTransactionStatsByTimelock(ctx context.Context, timelockAddress string, chainID int) (*types.TransactionStatsResponse, error) {
	var stats types.TransactionStatsResponse

	// 总交易数
	if err := r.db.WithContext(ctx).
		Model(&types.Transaction{}).
		Where("timelock_address = ? AND chain_id = ?", timelockAddress, chainID).
		Count(&stats.TotalTransactions).Error; err != nil {
		logger.Error("GetTransactionStatsByTimelock Total Error: ", err, "timelock_address", timelockAddress)
		return nil, err
	}

	// 各状态统计
	var statusCounts []struct {
		Status types.TransactionStatus
		Count  int64
	}

	if err := r.db.WithContext(ctx).
		Model(&types.Transaction{}).
		Select("status, count(*) as count").
		Where("timelock_address = ? AND chain_id = ?", timelockAddress, chainID).
		Group("status").
		Find(&statusCounts).Error; err != nil {
		logger.Error("GetTransactionStatsByTimelock Status Error: ", err, "timelock_address", timelockAddress)
		return nil, err
	}

	// 分配到对应字段
	for _, sc := range statusCounts {
		switch sc.Status {
		case types.TransactionSubmitting:
			stats.SubmittingCount = sc.Count
		case types.TransactionQueued:
			stats.QueuedCount = sc.Count
		case types.TransactionReady:
			stats.ReadyCount = sc.Count
		case types.TransactionExecuting:
			stats.ExecutingCount = sc.Count
		case types.TransactionExecuted:
			stats.ExecutedCount = sc.Count
		case types.TransactionFailed:
			stats.FailedCount = sc.Count
		case types.TransactionSubmitFailed:
			stats.SubmitFailedCount = sc.Count
		case types.TransactionExpired:
			stats.ExpiredCount = sc.Count
		case types.TransactionCanceled:
			stats.CanceledCount = sc.Count
		}
	}

	logger.Info("GetTransactionStatsByTimelock: ", "timelock_address", timelockAddress, "chain_id", chainID, "total", stats.TotalTransactions)
	return &stats, nil
}

// CheckTransactionExists 检查交易是否已存在
func (r *repository) CheckTransactionExists(ctx context.Context, txHash string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.Transaction{}).
		Where("tx_hash = ?", txHash).
		Count(&count).Error

	if err != nil {
		logger.Error("CheckTransactionExists Error: ", err, "tx_hash", txHash)
		return false, err
	}

	exists := count > 0
	logger.Info("CheckTransactionExists: ", "tx_hash", txHash, "exists", exists)
	return exists, nil
}

// UpdateTransactionFields 批量更新交易字段
func (r *repository) UpdateTransactionFields(ctx context.Context, id int64, updates map[string]interface{}) error {
	if err := r.db.WithContext(ctx).
		Model(&types.Transaction{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		logger.Error("UpdateTransactionFields Error: ", err, "transaction_id", id)
		return err
	}

	logger.Info("UpdateTransactionFields: ", "transaction_id", id, "fields", updates)
	return nil
}

// FindTransactionsByParams 根据参数查找交易（用于事件监听器）
func (r *repository) FindTransactionsByParams(ctx context.Context, params *FindTransactionParams) ([]types.Transaction, error) {
	logger.Info("FindTransactionsByParams: ", "params", params)

	var transactions []types.Transaction
	query := r.db.WithContext(ctx).Model(&types.Transaction{})

	// 添加查询条件
	if params.ChainID != nil {
		query = query.Where("chain_id = ?", *params.ChainID)
	}
	if params.TimelockAddress != nil {
		query = query.Where("timelock_address = ?", *params.TimelockAddress)
	}
	if params.TimelockStandard != nil {
		query = query.Where("timelock_standard = ?", *params.TimelockStandard)
	}
	if params.Target != nil {
		query = query.Where("target = ?", *params.Target)
	}
	if params.Value != nil {
		query = query.Where("value = ?", *params.Value)
	}
	if params.FunctionSig != nil {
		query = query.Where("function_sig = ?", *params.FunctionSig)
	}
	if params.ETA != nil {
		query = query.Where("eta = ?", *params.ETA)
	}

	// 执行查询
	if err := query.Find(&transactions).Error; err != nil {
		logger.Error("FindTransactionsByParams Error: ", err)
		return nil, err
	}

	logger.Info("FindTransactionsByParams: ", "count", len(transactions))
	return transactions, nil
}

// GetTransactionByOperationID 根据操作ID获取交易（用于事件监听器）
func (r *repository) GetTransactionByOperationID(ctx context.Context, operationID string) (*types.Transaction, error) {
	logger.Info("GetTransactionByOperationID: ", "operation_id", operationID)

	var tx types.Transaction
	err := r.db.WithContext(ctx).
		Where("operation_id = ?", operationID).
		First(&tx).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("transaction not found")
		}
		logger.Error("GetTransactionByOperationID Error: ", err, "operation_id", operationID)
		return nil, err
	}

	logger.Info("GetTransactionByOperationID: ", "transaction_id", tx.ID, "operation_id", operationID)
	return &tx, nil
}
