package scanner

import (
	"context"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TransactionRepository 交易记录仓库接口
type TransactionRepository interface {
	// Compound Timelock 交易
	CreateCompoundTransaction(ctx context.Context, tx *types.CompoundTimelockTransaction) error
	GetCompoundTransactionsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.CompoundTimelockTransaction, error)
	GetCompoundTransactionByHash(ctx context.Context, txHash, contractAddress, eventType string) (*types.CompoundTimelockTransaction, error)

	// OpenZeppelin Timelock 交易
	CreateOpenZeppelinTransaction(ctx context.Context, tx *types.OpenZeppelinTimelockTransaction) error
	GetOpenZeppelinTransactionsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.OpenZeppelinTimelockTransaction, error)
	GetOpenZeppelinTransactionByHash(ctx context.Context, txHash, contractAddress, eventType string) (*types.OpenZeppelinTimelockTransaction, error)

	// 批量操作
	BatchCreateCompoundTransactions(ctx context.Context, txs []types.CompoundTimelockTransaction) error
	BatchCreateOpenZeppelinTransactions(ctx context.Context, txs []types.OpenZeppelinTimelockTransaction) error
}

type transactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository 创建新的交易记录仓库
func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{
		db: db,
	}
}

// CreateCompoundTransaction 创建Compound交易记录
func (r *transactionRepository) CreateCompoundTransaction(ctx context.Context, tx *types.CompoundTimelockTransaction) error {
	if err := r.db.WithContext(ctx).Clauses(
		clause.OnConflict{DoNothing: true},
	).Create(tx).Error; err != nil {
		logger.Error("CreateCompoundTransaction Error", err, "tx_hash", tx.TxHash, "event_type", tx.EventType)
		return err
	}
	return nil
}

// GetCompoundTransactionsByContract 获取合约的Compound交易记录
func (r *transactionRepository) GetCompoundTransactionsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.CompoundTimelockTransaction, error) {
	var transactions []types.CompoundTimelockTransaction
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ?", chainID, contractAddress).
		Order("block_number DESC, created_at DESC").
		Find(&transactions).Error

	if err != nil {
		logger.Error("GetCompoundTransactionsByContract Error", err, "chain_id", chainID, "contract", contractAddress)
		return nil, err
	}

	return transactions, nil
}

// GetCompoundTransactionByHash 根据交易哈希获取Compound交易记录
func (r *transactionRepository) GetCompoundTransactionByHash(ctx context.Context, txHash, contractAddress, eventType string) (*types.CompoundTimelockTransaction, error) {
	var transaction types.CompoundTimelockTransaction
	err := r.db.WithContext(ctx).
		Where("tx_hash = ? AND contract_address = ? AND event_type = ?", txHash, contractAddress, eventType).
		First(&transaction).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Error("GetCompoundTransactionByHash Error", err, "tx_hash", txHash)
		return nil, err
	}

	return &transaction, nil
}

// CreateOpenZeppelinTransaction 创建OpenZeppelin交易记录
func (r *transactionRepository) CreateOpenZeppelinTransaction(ctx context.Context, tx *types.OpenZeppelinTimelockTransaction) error {
	if err := r.db.WithContext(ctx).Clauses(
		clause.OnConflict{DoNothing: true},
	).Create(tx).Error; err != nil {
		logger.Error("CreateOpenZeppelinTransaction Error", err, "tx_hash", tx.TxHash, "event_type", tx.EventType)
		return err
	}

	return nil
}

// GetOpenZeppelinTransactionsByContract 获取合约的OpenZeppelin交易记录
func (r *transactionRepository) GetOpenZeppelinTransactionsByContract(ctx context.Context, chainID int, contractAddress string) ([]types.OpenZeppelinTimelockTransaction, error) {
	var transactions []types.OpenZeppelinTimelockTransaction
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ?", chainID, contractAddress).
		Order("block_number DESC, created_at DESC").
		Find(&transactions).Error

	if err != nil {
		logger.Error("GetOpenZeppelinTransactionsByContract Error", err, "chain_id", chainID, "contract", contractAddress)
		return nil, err
	}

	return transactions, nil
}

// GetOpenZeppelinTransactionByHash 根据交易哈希获取OpenZeppelin交易记录
func (r *transactionRepository) GetOpenZeppelinTransactionByHash(ctx context.Context, txHash, contractAddress, eventType string) (*types.OpenZeppelinTimelockTransaction, error) {
	var transaction types.OpenZeppelinTimelockTransaction
	err := r.db.WithContext(ctx).
		Where("tx_hash = ? AND contract_address = ? AND event_type = ?", txHash, contractAddress, eventType).
		First(&transaction).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Error("GetOpenZeppelinTransactionByHash Error", err, "tx_hash", txHash)
		return nil, err
	}

	return &transaction, nil
}

// BatchCreateCompoundTransactions 批量创建Compound交易记录
func (r *transactionRepository) BatchCreateCompoundTransactions(ctx context.Context, txs []types.CompoundTimelockTransaction) error {
	if len(txs) == 0 {
		return nil
	}

	// 使用 ON CONFLICT DO NOTHING 来避免重复插入错误
	if err := r.db.WithContext(ctx).Clauses(
		clause.OnConflict{DoNothing: true},
	).CreateInBatches(&txs, 100).Error; err != nil {
		logger.Error("BatchCreateCompoundTransactions Error", err, "count", len(txs))
		return err
	}

	return nil
}

// BatchCreateOpenZeppelinTransactions 批量创建OpenZeppelin交易记录
func (r *transactionRepository) BatchCreateOpenZeppelinTransactions(ctx context.Context, txs []types.OpenZeppelinTimelockTransaction) error {
	if len(txs) == 0 {
		return nil
	}

	// 使用 ON CONFLICT DO NOTHING 来避免重复插入错误
	if err := r.db.WithContext(ctx).Clauses(
		clause.OnConflict{DoNothing: true},
	).CreateInBatches(&txs, 100).Error; err != nil {
		logger.Error("BatchCreateOpenZeppelinTransactions Error", err, "count", len(txs))
		return err
	}

	return nil
}
