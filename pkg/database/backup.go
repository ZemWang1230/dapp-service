package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// BackupManager 备份管理器
type BackupManager struct {
	db *gorm.DB
}

// NewBackupManager 创建备份管理器
func NewBackupManager(db *gorm.DB) *BackupManager {
	return &BackupManager{db: db}
}

// BackupData 备份数据结构
type BackupData struct {
	Version                string                   `json:"version"`
	Timestamp              time.Time                `json:"timestamp"`
	Users                  []map[string]interface{} `json:"users"`
	UserAssets             []map[string]interface{} `json:"user_assets"`
	ABIs                   []map[string]interface{} `json:"abis"`
	CompoundTimelocks      []map[string]interface{} `json:"compound_timelocks"`
	OpenzeppelinTimelocks  []map[string]interface{} `json:"openzeppelin_timelocks"`
	Transactions           []map[string]interface{} `json:"transactions"`
	EmailNotifications     []map[string]interface{} `json:"email_notifications"`
	EmailSendLogs          []map[string]interface{} `json:"email_send_logs"`
	EmergencyNotifications []map[string]interface{} `json:"emergency_notifications"`
	Sponsors               []map[string]interface{} `json:"sponsors"`
}

// CreateBackup 创建完整数据备份
func (bm *BackupManager) CreateBackup(backupPath string) error {
	ctx := context.Background()
	logger.Info("Starting database backup creation", "path", backupPath)

	// 确保备份目录存在
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		logger.Error("Failed to create backup directory", err)
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	backup := BackupData{
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}

	// 备份用户数据
	if err := bm.backupTable(ctx, "users", &backup.Users); err != nil {
		return fmt.Errorf("failed to backup users: %w", err)
	}

	// 备份用户资产数据
	if err := bm.backupTable(ctx, "user_assets", &backup.UserAssets); err != nil {
		return fmt.Errorf("failed to backup user_assets: %w", err)
	}

	// 备份用户自定义ABI（不包含系统共享ABI）
	if err := bm.backupTableWithCondition(ctx, "abis", "is_shared = ?", false, &backup.ABIs); err != nil {
		return fmt.Errorf("failed to backup abis: %w", err)
	}

	// 备份Compound timelock合约
	if err := bm.backupTable(ctx, "compound_timelocks", &backup.CompoundTimelocks); err != nil {
		return fmt.Errorf("failed to backup compound_timelocks: %w", err)
	}

	// 备份OpenZeppelin timelock合约
	if err := bm.backupTable(ctx, "openzeppelin_timelocks", &backup.OpenzeppelinTimelocks); err != nil {
		return fmt.Errorf("failed to backup openzeppelin_timelocks: %w", err)
	}

	// 备份交易记录
	if err := bm.backupTable(ctx, "transactions", &backup.Transactions); err != nil {
		return fmt.Errorf("failed to backup transactions: %w", err)
	}

	// 备份邮件通知配置
	if err := bm.backupTable(ctx, "email_notifications", &backup.EmailNotifications); err != nil {
		return fmt.Errorf("failed to backup email_notifications: %w", err)
	}

	// 备份邮件发送记录
	if err := bm.backupTable(ctx, "email_send_logs", &backup.EmailSendLogs); err != nil {
		return fmt.Errorf("failed to backup email_send_logs: %w", err)
	}

	// 备份应急通知记录
	if err := bm.backupTable(ctx, "emergency_notifications", &backup.EmergencyNotifications); err != nil {
		return fmt.Errorf("failed to backup emergency_notifications: %w", err)
	}

	// 备份赞助方数据
	if err := bm.backupTable(ctx, "sponsors", &backup.Sponsors); err != nil {
		return fmt.Errorf("failed to backup sponsors: %w", err)
	}

	// 写入备份文件
	file, err := os.Create(backupPath)
	if err != nil {
		logger.Error("Failed to create backup file", err, "path", backupPath)
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(backup); err != nil {
		logger.Error("Failed to encode backup data", err)
		return fmt.Errorf("failed to encode backup data: %w", err)
	}

	logger.Info("Database backup created successfully",
		"path", backupPath,
		"users", len(backup.Users),
		"assets", len(backup.UserAssets),
		"abis", len(backup.ABIs),
		"compound_timelocks", len(backup.CompoundTimelocks),
		"openzeppelin_timelocks", len(backup.OpenzeppelinTimelocks),
		"transactions", len(backup.Transactions),
		"email_notifications", len(backup.EmailNotifications),
		"sponsors", len(backup.Sponsors),
	)

	return nil
}

// RestoreBackup 从备份文件恢复数据
func (bm *BackupManager) RestoreBackup(backupPath string, options RestoreOptions) error {
	ctx := context.Background()
	logger.Info("Starting database restore from backup", "path", backupPath)

	// 读取备份文件
	file, err := os.Open(backupPath)
	if err != nil {
		logger.Error("Failed to open backup file", err, "path", backupPath)
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	var backup BackupData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&backup); err != nil {
		logger.Error("Failed to decode backup data", err)
		return fmt.Errorf("failed to decode backup data: %w", err)
	}

	logger.Info("Backup file loaded",
		"version", backup.Version,
		"timestamp", backup.Timestamp,
		"users", len(backup.Users),
	)

	// 在事务中执行恢复操作
	return bm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 如果需要清空现有数据
		if options.ClearExisting {
			if err := bm.clearUserData(ctx, tx); err != nil {
				return fmt.Errorf("failed to clear existing data: %w", err)
			}
		}

		// 恢复用户数据
		if err := bm.restoreTable(ctx, tx, "users", backup.Users, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore users: %w", err)
		}

		// 恢复用户资产数据
		if err := bm.restoreTable(ctx, tx, "user_assets", backup.UserAssets, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore user_assets: %w", err)
		}

		// 恢复用户自定义ABI
		if err := bm.restoreTable(ctx, tx, "abis", backup.ABIs, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore abis: %w", err)
		}

		// 恢复Compound timelock合约
		if err := bm.restoreTable(ctx, tx, "compound_timelocks", backup.CompoundTimelocks, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore compound_timelocks: %w", err)
		}

		// 恢复OpenZeppelin timelock合约
		if err := bm.restoreTable(ctx, tx, "openzeppelin_timelocks", backup.OpenzeppelinTimelocks, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore openzeppelin_timelocks: %w", err)
		}

		// 恢复交易记录
		if err := bm.restoreTable(ctx, tx, "transactions", backup.Transactions, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore transactions: %w", err)
		}

		// 恢复邮件通知配置
		if err := bm.restoreTable(ctx, tx, "email_notifications", backup.EmailNotifications, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore email_notifications: %w", err)
		}

		// 恢复邮件发送记录
		if err := bm.restoreTable(ctx, tx, "email_send_logs", backup.EmailSendLogs, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore email_send_logs: %w", err)
		}

		// 恢复应急通知记录
		if err := bm.restoreTable(ctx, tx, "emergency_notifications", backup.EmergencyNotifications, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore emergency_notifications: %w", err)
		}

		// 恢复赞助方数据
		if err := bm.restoreTable(ctx, tx, "sponsors", backup.Sponsors, options.OnConflict); err != nil {
			return fmt.Errorf("failed to restore sponsors: %w", err)
		}

		return nil
	})
}

// RestoreOptions 恢复选项
type RestoreOptions struct {
	ClearExisting bool           // 是否清空现有用户数据
	OnConflict    ConflictAction // 冲突处理策略
}

// ConflictAction 冲突处理策略
type ConflictAction string

const (
	ConflictSkip    ConflictAction = "skip"    // 跳过冲突记录
	ConflictReplace ConflictAction = "replace" // 替换冲突记录
	ConflictError   ConflictAction = "error"   // 遇到冲突报错
)

// backupTable 备份指定表的所有数据
func (bm *BackupManager) backupTable(ctx context.Context, tableName string, result interface{}) error {
	logger.Info("Backing up table", "table", tableName)

	rows, err := bm.db.WithContext(ctx).Table(tableName).Rows()
	if err != nil {
		return fmt.Errorf("failed to query table %s: %w", tableName, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
	}

	var records []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row from table %s: %w", tableName, err)
		}

		record := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				record[col] = string(b)
			} else {
				record[col] = val
			}
		}
		records = append(records, record)
	}

	// 使用反射设置结果
	switch v := result.(type) {
	case *[]map[string]interface{}:
		*v = records
	default:
		return fmt.Errorf("unsupported result type for table %s", tableName)
	}

	logger.Info("Table backup completed", "table", tableName, "records", len(records))
	return nil
}

// backupTableWithCondition 带条件备份指定表的数据
func (bm *BackupManager) backupTableWithCondition(ctx context.Context, tableName, condition string, args interface{}, result interface{}) error {
	logger.Info("Backing up table with condition", "table", tableName, "condition", condition)

	rows, err := bm.db.WithContext(ctx).Table(tableName).Where(condition, args).Rows()
	if err != nil {
		return fmt.Errorf("failed to query table %s with condition: %w", tableName, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
	}

	var records []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row from table %s: %w", tableName, err)
		}

		record := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				record[col] = string(b)
			} else {
				record[col] = val
			}
		}
		records = append(records, record)
	}

	// 使用反射设置结果
	switch v := result.(type) {
	case *[]map[string]interface{}:
		*v = records
	default:
		return fmt.Errorf("unsupported result type for table %s", tableName)
	}

	logger.Info("Conditional table backup completed", "table", tableName, "records", len(records))
	return nil
}

// restoreTable 恢复表数据
func (bm *BackupManager) restoreTable(ctx context.Context, tx *gorm.DB, tableName string, records []map[string]interface{}, onConflict ConflictAction) error {
	if len(records) == 0 {
		logger.Info("No records to restore for table", "table", tableName)
		return nil
	}

	logger.Info("Restoring table", "table", tableName, "records", len(records))

	for _, record := range records {
		if err := bm.insertRecord(ctx, tx, tableName, record, onConflict); err != nil {
			if onConflict == ConflictError {
				return fmt.Errorf("failed to insert record into %s: %w", tableName, err)
			}
			logger.Warn("Skipped record due to conflict", "table", tableName, "error", err)
		}
	}

	logger.Info("Table restore completed", "table", tableName)
	return nil
}

// insertRecord 插入单条记录，处理冲突
func (bm *BackupManager) insertRecord(ctx context.Context, tx *gorm.DB, tableName string, record map[string]interface{}, onConflict ConflictAction) error {
	// 构建列名和占位符
	var columns []string
	var values []interface{}
	var placeholders []string

	for col, val := range record {
		columns = append(columns, col)
		values = append(values, val)
		placeholders = append(placeholders, "?")
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	// 根据冲突策略调整SQL
	switch onConflict {
	case ConflictSkip:
		sql += " ON CONFLICT DO NOTHING"
	case ConflictReplace:
		sql += fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET ", bm.getPrimaryKeyColumn(tableName))
		var updates []string
		for _, col := range columns {
			if col != "id" { // 通常不更新主键
				updates = append(updates, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
			}
		}
		if len(updates) > 0 {
			sql += strings.Join(updates, ", ")
		}
	case ConflictError:
		// 默认行为，遇到冲突会报错
	}

	return tx.WithContext(ctx).Exec(sql, values...).Error
}

// getPrimaryKeyColumn 获取表的主键列名（简化实现）
func (bm *BackupManager) getPrimaryKeyColumn(tableName string) string {
	// 大部分表都使用id作为主键
	// 在实际实现中，可以通过数据库元数据查询获取真实的主键
	switch tableName {
	case "user_assets":
		return "wallet_address, chain_name, contract_address" // 复合主键
	case "email_notifications":
		return "wallet_address, email" // 复合主键
	case "support_chains":
		return "chain_name" // 唯一键
	case "abis":
		return "name, owner" // 复合唯一键
	default:
		return "id" // 默认主键
	}
}

// clearUserData 清空用户相关数据（保留系统数据）
func (bm *BackupManager) clearUserData(ctx context.Context, tx *gorm.DB) error {
	logger.Warn("Clearing existing user data")

	// 按依赖关系顺序删除
	tables := []string{
		"sponsors",
		"emergency_notifications",
		"email_send_logs",
		"email_notifications",
		"transactions",
		"compound_timelocks",
		"openzeppelin_timelocks",
		"user_assets",
		"users",
	}

	for _, table := range tables {
		if err := tx.WithContext(ctx).Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
			logger.Error("Failed to clear table", err, "table", table)
			return fmt.Errorf("failed to clear table %s: %w", table, err)
		}
		logger.Info("Cleared table", "table", table)
	}

	// 清空用户自定义ABI（保留共享ABI）
	if err := tx.WithContext(ctx).Exec("DELETE FROM abis WHERE is_shared = ?", false).Error; err != nil {
		logger.Error("Failed to clear user ABIs", err)
		return fmt.Errorf("failed to clear user ABIs: %w", err)
	}

	logger.Info("User data cleared successfully")
	return nil
}

// ValidateBackup 验证备份文件的完整性
func (bm *BackupManager) ValidateBackup(backupPath string) error {
	logger.Info("Validating backup file", "path", backupPath)

	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	var backup BackupData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&backup); err != nil {
		return fmt.Errorf("failed to decode backup data: %w", err)
	}

	// 基本验证
	if backup.Version == "" {
		return errors.New("backup version is missing")
	}

	if backup.Timestamp.IsZero() {
		return errors.New("backup timestamp is missing")
	}

	// 数据完整性检查
	userAddresses := make(map[string]bool)
	for _, user := range backup.Users {
		if addr, ok := user["wallet_address"].(string); ok {
			userAddresses[addr] = true
		}
	}

	// 检查资产数据是否有对应的用户
	for _, asset := range backup.UserAssets {
		if addr, ok := asset["wallet_address"].(string); ok {
			if !userAddresses[addr] {
				return fmt.Errorf("user asset references non-existent user: %s", addr)
			}
		}
	}

	logger.Info("Backup validation completed successfully",
		"version", backup.Version,
		"users", len(backup.Users),
		"timestamp", backup.Timestamp,
	)

	return nil
}

// GetBackupInfo 获取备份文件信息
func (bm *BackupManager) GetBackupInfo(backupPath string) (*BackupData, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	var backup BackupData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&backup); err != nil {
		return nil, fmt.Errorf("failed to decode backup data: %w", err)
	}

	// 清空实际数据，只返回元信息
	backup.Users = nil
	backup.UserAssets = nil
	backup.ABIs = nil
	backup.CompoundTimelocks = nil
	backup.OpenzeppelinTimelocks = nil
	backup.Transactions = nil
	backup.EmailNotifications = nil
	backup.EmailSendLogs = nil
	backup.EmergencyNotifications = nil
	backup.Sponsors = nil

	return &backup, nil
}
