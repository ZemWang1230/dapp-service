package migrations

import (
	"context"
	"fmt"
	"time"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// Migration 表示一个数据库迁移版本
type Migration struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Version     string `gorm:"unique;size:50;not null"`
	Description string `gorm:"size:200;not null"`
	Applied     bool   `gorm:"not null;default:false"`
	AppliedAt   *time.Time
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

// TableName 设置表名
func (Migration) TableName() string {
	return "schema_migrations"
}

// MigrationHandler 迁移处理器
type MigrationHandler struct {
	db *gorm.DB
}

// NewMigrationHandler 创建迁移处理器
func NewMigrationHandler(db *gorm.DB) *MigrationHandler {
	return &MigrationHandler{db: db}
}

// InitTables 安全的数据库初始化 - 不会删除现有数据
func InitTables(db *gorm.DB) error {
	ctx := context.Background()
	logger.Info("Starting safe database initialization...")

	handler := NewMigrationHandler(db)

	// 1. 创建迁移记录表
	if err := handler.ensureMigrationTable(ctx); err != nil {
		logger.Error("Failed to create migration table", err)
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	// 2. 执行所有待执行的迁移
	if err := handler.runMigrations(ctx); err != nil {
		logger.Error("Failed to run migrations", err)
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("Database initialization completed successfully")
	return nil
}

// ensureMigrationTable 确保迁移记录表存在
func (h *MigrationHandler) ensureMigrationTable(ctx context.Context) error {
	if h.db.Migrator().HasTable(&Migration{}) {
		logger.Info("Migration table already exists")
		return nil
	}

	logger.Info("Creating migration table...")
	if err := h.db.WithContext(ctx).AutoMigrate(&Migration{}); err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	logger.Info("Migration table created successfully")
	return nil
}

// runMigrations 执行所有待执行的迁移
func (h *MigrationHandler) runMigrations(ctx context.Context) error {
	migrations := []migrationFunc{
		{"v1.0.0", "Create initial tables", h.createInitialTables},
		{"v1.0.1", "Create indexes", h.createIndexes},
		{"v1.0.2", "Insert default chains data", h.insertSupportedChains},
		{"v1.0.3", "Insert shared ABIs data", h.insertSharedABIs},
		{"v1.0.4", "Insert default sponsors data", h.insertDefaultSponsors},
	}

	for _, migration := range migrations {
		if err := h.runSingleMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", migration.version, err)
		}
	}

	return nil
}

// migrationFunc 迁移函数类型
type migrationFunc struct {
	version     string
	description string
	fn          func(context.Context) error
}

// runSingleMigration 执行单个迁移
func (h *MigrationHandler) runSingleMigration(ctx context.Context, migration migrationFunc) error {
	// 检查迁移是否已执行
	var existingMigration Migration
	result := h.db.WithContext(ctx).Where("version = ?", migration.version).First(&existingMigration)

	if result.Error == nil && existingMigration.Applied {
		logger.Info("Migration already applied", "version", migration.version)
		return nil
	}

	logger.Info("Running migration", "version", migration.version, "description", migration.description)

	// 开始事务执行迁移
	err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 执行迁移逻辑
		if err := migration.fn(ctx); err != nil {
			return err
		}

		// 记录迁移执行情况
		now := time.Now()
		migrationRecord := Migration{
			Version:     migration.version,
			Description: migration.description,
			Applied:     true,
			AppliedAt:   &now,
		}

		if result.Error != nil {
			// 创建新记录
			return tx.Create(&migrationRecord).Error
		} else {
			// 更新现有记录
			return tx.Model(&existingMigration).Updates(map[string]interface{}{
				"applied":    true,
				"applied_at": &now,
			}).Error
		}
	})

	if err != nil {
		logger.Error("Migration failed", err, "version", migration.version)
		return err
	}

	logger.Info("Migration completed successfully", "version", migration.version)
	return nil
}

// createInitialTables 创建初始表结构（v1.0.0）
func (h *MigrationHandler) createInitialTables(ctx context.Context) error {
	logger.Info("Creating initial database tables...")

	// 1. 用户表
	if !h.db.Migrator().HasTable("users") {
		createUsersTable := `
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			wallet_address VARCHAR(42) NOT NULL UNIQUE,
			chain_id INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			last_login TIMESTAMP WITH TIME ZONE,
			status INTEGER DEFAULT 1
		)`

		if err := h.db.WithContext(ctx).Exec(createUsersTable).Error; err != nil {
			return fmt.Errorf("failed to create users table: %w", err)
		}
		logger.Info("Created table: users")
	}

	// 2. 支持的区块链表
	if !h.db.Migrator().HasTable("support_chains") {
		createSupportChainsTable := `
		CREATE TABLE support_chains (
			id BIGSERIAL PRIMARY KEY,
			chain_name VARCHAR(50) NOT NULL UNIQUE,
			display_name VARCHAR(100) NOT NULL,
			chain_id BIGINT NOT NULL,
			native_currency_name VARCHAR(50) NOT NULL,
			native_currency_symbol VARCHAR(10) NOT NULL,
			native_currency_decimals INTEGER NOT NULL DEFAULT 18,
			logo_url TEXT,
			is_testnet BOOLEAN NOT NULL DEFAULT false,
			is_active BOOLEAN NOT NULL DEFAULT true,
			alchemy_rpc_template TEXT,
			infura_rpc_template TEXT,
			official_rpc_urls TEXT NOT NULL,
			block_explorer_urls TEXT NOT NULL,
			rpc_enabled BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`

		if err := h.db.WithContext(ctx).Exec(createSupportChainsTable).Error; err != nil {
			return fmt.Errorf("failed to create support_chains table: %w", err)
		}
		logger.Info("Created table: support_chains")
	}

	// 3. 用户资产表
	if !h.db.Migrator().HasTable("user_assets") {
		createUserAssetsTable := `
		CREATE TABLE user_assets (
			id BIGSERIAL PRIMARY KEY,
			wallet_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE,
			chain_name VARCHAR(50) NOT NULL,
			contract_address VARCHAR(42) NOT NULL DEFAULT '',
			token_symbol VARCHAR(20) NOT NULL,
			token_name VARCHAR(100) NOT NULL,
			token_decimals INTEGER NOT NULL DEFAULT 18,
			balance VARCHAR(100) NOT NULL DEFAULT '0',
			balance_wei VARCHAR(100) NOT NULL DEFAULT '0',
			usd_value DECIMAL(20,8) DEFAULT 0,
			token_price DECIMAL(20,8) DEFAULT 0,
			price_change24h DECIMAL(10,4) DEFAULT 0,
			is_native BOOLEAN NOT NULL DEFAULT false,
			token_logo_url TEXT,
			chain_logo_url TEXT,
			last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(wallet_address, chain_name, contract_address)
		)`

		if err := h.db.WithContext(ctx).Exec(createUserAssetsTable).Error; err != nil {
			return fmt.Errorf("failed to create user_assets table: %w", err)
		}
		logger.Info("Created table: user_assets")
	}

	// 4. ABI库表
	if !h.db.Migrator().HasTable("abis") {
		createABIsTable := `
		CREATE TABLE abis (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(200) NOT NULL,
			abi_content TEXT NOT NULL,
			owner VARCHAR(42) NOT NULL,
			description VARCHAR(500) DEFAULT '',
			is_shared BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(name, owner)
		)`

		if err := h.db.WithContext(ctx).Exec(createABIsTable).Error; err != nil {
			return fmt.Errorf("failed to create abis table: %w", err)
		}
		logger.Info("Created table: abis")
	}

	// 5. Compound标准Timelock合约表
	if !h.db.Migrator().HasTable("compound_timelocks") {
		createCompoundTimelocksTable := `
		CREATE TABLE compound_timelocks (
			id BIGSERIAL PRIMARY KEY,
			creator_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE,
			chain_id INTEGER NOT NULL,
			chain_name VARCHAR(50) NOT NULL,
			contract_address VARCHAR(42) NOT NULL,
			tx_hash VARCHAR(66),
			min_delay BIGINT NOT NULL,
			admin VARCHAR(42) NOT NULL,
			pending_admin VARCHAR(42),
			remark VARCHAR(500) DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'deleted')),
			is_imported BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(chain_id, contract_address)
		)`

		if err := h.db.WithContext(ctx).Exec(createCompoundTimelocksTable).Error; err != nil {
			return fmt.Errorf("failed to create compound_timelocks table: %w", err)
		}
		logger.Info("Created table: compound_timelocks")
	}

	// 6. OpenZeppelin标准Timelock合约表
	if !h.db.Migrator().HasTable("openzeppelin_timelocks") {
		createOpenzeppelinTimelocksTable := `
		CREATE TABLE openzeppelin_timelocks (
			id BIGSERIAL PRIMARY KEY,
			creator_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE,
			chain_id INTEGER NOT NULL,
			chain_name VARCHAR(50) NOT NULL,
			contract_address VARCHAR(42) NOT NULL,
			tx_hash VARCHAR(66),
			min_delay BIGINT NOT NULL,
			proposers TEXT NOT NULL,
			executors TEXT NOT NULL,
			cancellers TEXT NOT NULL,
			remark VARCHAR(500) DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'deleted')),
			is_imported BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(chain_id, contract_address)
		)`

		if err := h.db.WithContext(ctx).Exec(createOpenzeppelinTimelocksTable).Error; err != nil {
			return fmt.Errorf("failed to create openzeppelin_timelocks table: %w", err)
		}
		logger.Info("Created table: openzeppelin_timelocks")
	}

	// 7. 赞助方和生态伙伴表
	if !h.db.Migrator().HasTable("sponsors") {
		createSponsorsTable := `
		CREATE TABLE sponsors (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(200) NOT NULL,
			logo_url TEXT NOT NULL,
			link TEXT NOT NULL,
			description TEXT NOT NULL,
			type VARCHAR(20) NOT NULL CHECK (type IN ('sponsor', 'partner')),
			sort_order INTEGER NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`

		if err := h.db.WithContext(ctx).Exec(createSponsorsTable).Error; err != nil {
			return fmt.Errorf("failed to create sponsors table: %w", err)
		}
		logger.Info("Created table: sponsors")
	}

	// 8. 区块扫描进度表
	if !h.db.Migrator().HasTable("block_scan_progress") {
		createBlockScanProgressTable := `
		CREATE TABLE block_scan_progress (
			id BIGSERIAL PRIMARY KEY,
			chain_id INTEGER NOT NULL UNIQUE,
			chain_name VARCHAR(50) NOT NULL,
			last_scanned_block BIGINT NOT NULL DEFAULT 0,
			latest_network_block BIGINT DEFAULT 0,
			scan_status VARCHAR(20) NOT NULL DEFAULT 'running' CHECK (scan_status IN ('running', 'paused', 'error')),
			error_message TEXT,
			last_update_time TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`

		if err := h.db.WithContext(ctx).Exec(createBlockScanProgressTable).Error; err != nil {
			return fmt.Errorf("failed to create block_scan_progress table: %w", err)
		}
		logger.Info("Created table: block_scan_progress")
	}

	// 9. Compound Timelock 交易记录表
	if !h.db.Migrator().HasTable("compound_timelock_transactions") {
		createCompoundTimelockTransactionsTable := `
		CREATE TABLE compound_timelock_transactions (
			id BIGSERIAL PRIMARY KEY,
			tx_hash VARCHAR(66) NOT NULL,
			block_number BIGINT NOT NULL,
			block_timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
			chain_id INTEGER NOT NULL,
			chain_name VARCHAR(50) NOT NULL,
			contract_address VARCHAR(42) NOT NULL,
			from_address VARCHAR(42) NOT NULL,
			to_address VARCHAR(42) NOT NULL,
			tx_status VARCHAR(20) NOT NULL DEFAULT 'failed' CHECK (tx_status IN ('success', 'failed')),
			event_type VARCHAR(50) NOT NULL CHECK (event_type IN ('QueueTransaction', 'ExecuteTransaction', 'CancelTransaction')),
			event_data JSONB NOT NULL,
			event_tx_hash VARCHAR(128),
			event_target VARCHAR(42),
			event_value DECIMAL(36,0) DEFAULT 0,
			event_function_signature VARCHAR(200),
			event_call_data BYTEA,
			event_eta BIGINT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(tx_hash, contract_address, event_type)
		)`

		if err := h.db.WithContext(ctx).Exec(createCompoundTimelockTransactionsTable).Error; err != nil {
			return fmt.Errorf("failed to create compound_timelock_transactions table: %w", err)
		}
		logger.Info("Created table: compound_timelock_transactions")
	}

	// 10. OpenZeppelin Timelock 交易记录表
	if !h.db.Migrator().HasTable("openzeppelin_timelock_transactions") {
		createOpenzeppelinTimelockTransactionsTable := `
		CREATE TABLE openzeppelin_timelock_transactions (
			id BIGSERIAL PRIMARY KEY,
			tx_hash VARCHAR(66) NOT NULL,
			block_number BIGINT NOT NULL,
			block_timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
			chain_id INTEGER NOT NULL,
			chain_name VARCHAR(50) NOT NULL,
			contract_address VARCHAR(42) NOT NULL,
			from_address VARCHAR(42) NOT NULL,
			to_address VARCHAR(42) NOT NULL,
			tx_status VARCHAR(20) NOT NULL DEFAULT 'failed' CHECK (tx_status IN ('success', 'failed')),
			event_type VARCHAR(50) NOT NULL CHECK (event_type IN ('CallScheduled', 'CallExecuted', 'Cancelled')),
			event_data JSONB NOT NULL,
			event_id VARCHAR(66),
			event_index INTEGER,
			event_target VARCHAR(42),
			event_value DECIMAL(36,0) DEFAULT 0,
			event_call_data BYTEA,
			event_predecessor VARCHAR(66),
			event_delay BIGINT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(tx_hash, contract_address, event_type)
		)`

		if err := h.db.WithContext(ctx).Exec(createOpenzeppelinTimelockTransactionsTable).Error; err != nil {
			return fmt.Errorf("failed to create openzeppelin_timelock_transactions table: %w", err)
		}
		logger.Info("Created table: openzeppelin_timelock_transactions")
	}

	// 11. Timelock 交易流程关联表
	if !h.db.Migrator().HasTable("timelock_transaction_flows") {
		createTimelockTransactionFlowsTable := `
		CREATE TABLE timelock_transaction_flows (
			id BIGSERIAL PRIMARY KEY,
			flow_id VARCHAR(128) NOT NULL,
			timelock_standard VARCHAR(20) NOT NULL CHECK (timelock_standard IN ('compound', 'openzeppelin')),
			chain_id INTEGER NOT NULL,
			contract_address VARCHAR(42) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'proposed' CHECK (status IN ('proposed', 'queued', 'executed', 'cancelled', 'expired')),
			propose_tx_hash VARCHAR(66),
			queue_tx_hash VARCHAR(66),
			execute_tx_hash VARCHAR(66),
			cancel_tx_hash VARCHAR(66),
			proposed_at TIMESTAMP WITH TIME ZONE,
			queued_at TIMESTAMP WITH TIME ZONE,
			executed_at TIMESTAMP WITH TIME ZONE,
			cancelled_at TIMESTAMP WITH TIME ZONE,
			eta TIMESTAMP WITH TIME ZONE,
			target_address VARCHAR(42),
			call_data BYTEA,
			value DECIMAL(36,0) DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(flow_id, timelock_standard, chain_id, contract_address)
		)`

		if err := h.db.WithContext(ctx).Exec(createTimelockTransactionFlowsTable).Error; err != nil {
			return fmt.Errorf("failed to create timelock_transaction_flows table: %w", err)
		}
		logger.Info("Created table: timelock_transaction_flows")
	}

	// 12. 用户-合约关联表
	if !h.db.Migrator().HasTable("user_timelock_relations") {
		createUserTimelockRelationsTable := `
		CREATE TABLE user_timelock_relations (
			id BIGSERIAL PRIMARY KEY,
			user_address VARCHAR(42) NOT NULL,
			chain_id INTEGER NOT NULL,
			contract_address VARCHAR(42) NOT NULL,
			timelock_standard VARCHAR(20) NOT NULL CHECK (timelock_standard IN ('compound', 'openzeppelin')),
			relation_type VARCHAR(20) NOT NULL CHECK (relation_type IN ('creator', 'admin', 'pending_admin', 'proposer', 'executor', 'canceller')),
			related_at TIMESTAMP WITH TIME ZONE NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(user_address, chain_id, contract_address, relation_type)
		)`

		if err := h.db.WithContext(ctx).Exec(createUserTimelockRelationsTable).Error; err != nil {
			return fmt.Errorf("failed to create user_timelock_relations table: %w", err)
		}
		logger.Info("Created table: user_timelock_relations")
	}

	logger.Info("All tables created successfully")
	return nil
}

// createIndexes 创建索引（v1.0.1）
func (h *MigrationHandler) createIndexes(ctx context.Context) error {
	logger.Info("Creating database indexes...")

	indexes := []string{
		// Users 表索引
		`CREATE INDEX IF NOT EXISTS idx_users_wallet_address ON users(wallet_address)`,
		`CREATE INDEX IF NOT EXISTS idx_users_chain_id ON users(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_status ON users(status)`,
		`CREATE INDEX IF NOT EXISTS idx_users_last_login ON users(last_login)`,

		// Support Chains 表索引
		`CREATE INDEX IF NOT EXISTS idx_support_chains_chain_name ON support_chains(chain_name)`,
		`CREATE INDEX IF NOT EXISTS idx_support_chains_chain_id ON support_chains(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_support_chains_is_testnet ON support_chains(is_testnet)`,
		`CREATE INDEX IF NOT EXISTS idx_support_chains_is_active ON support_chains(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_support_chains_rpc_enabled ON support_chains(rpc_enabled)`,

		// User Assets 表索引
		`CREATE INDEX IF NOT EXISTS idx_user_assets_wallet_address ON user_assets(wallet_address)`,
		`CREATE INDEX IF NOT EXISTS idx_user_assets_chain_name ON user_assets(chain_name)`,
		`CREATE INDEX IF NOT EXISTS idx_user_assets_contract_address ON user_assets(contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_user_assets_token_symbol ON user_assets(token_symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_user_assets_is_native ON user_assets(is_native)`,
		`CREATE INDEX IF NOT EXISTS idx_user_assets_usd_value ON user_assets(usd_value DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_user_assets_last_updated ON user_assets(last_updated)`,
		`CREATE INDEX IF NOT EXISTS idx_user_assets_wallet_chain ON user_assets(wallet_address, chain_name)`,
		`CREATE INDEX IF NOT EXISTS idx_user_assets_wallet_value ON user_assets(wallet_address, usd_value DESC)`,

		// ABIs 表索引
		`CREATE INDEX IF NOT EXISTS idx_abis_name ON abis(name)`,
		`CREATE INDEX IF NOT EXISTS idx_abis_owner ON abis(owner)`,
		`CREATE INDEX IF NOT EXISTS idx_abis_is_shared ON abis(is_shared)`,
		`CREATE INDEX IF NOT EXISTS idx_abis_owner_shared ON abis(owner, is_shared)`,

		// Compound Timelocks 表索引
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_creator ON compound_timelocks(creator_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_chain_id ON compound_timelocks(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_chain_name ON compound_timelocks(chain_name)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_contract ON compound_timelocks(contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_admin ON compound_timelocks(admin)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_pending_admin ON compound_timelocks(pending_admin)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_status ON compound_timelocks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_is_imported ON compound_timelocks(is_imported)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_creator_chain ON compound_timelocks(creator_address, chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_chain_contract ON compound_timelocks(chain_id, contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_created_at ON compound_timelocks(created_at DESC)`,

		// OpenZeppelin Timelocks 表索引
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_timelocks_creator ON openzeppelin_timelocks(creator_address)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_timelocks_chain_id ON openzeppelin_timelocks(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_timelocks_chain_name ON openzeppelin_timelocks(chain_name)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_timelocks_contract ON openzeppelin_timelocks(contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_timelocks_status ON openzeppelin_timelocks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_timelocks_is_imported ON openzeppelin_timelocks(is_imported)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_timelocks_creator_chain ON openzeppelin_timelocks(creator_address, chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_timelocks_chain_contract ON openzeppelin_timelocks(chain_id, contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_timelocks_created_at ON openzeppelin_timelocks(created_at DESC)`,

		// Sponsors 表索引
		`CREATE INDEX IF NOT EXISTS idx_sponsors_type ON sponsors(type)`,
		`CREATE INDEX IF NOT EXISTS idx_sponsors_is_active ON sponsors(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_sponsors_sort_order ON sponsors(sort_order DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_sponsors_type_active ON sponsors(type, is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_sponsors_type_sort ON sponsors(type, sort_order DESC)`,

		// Block Scan Progress 表索引
		`CREATE INDEX IF NOT EXISTS idx_block_scan_progress_chain_id ON block_scan_progress(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_block_scan_progress_chain_name ON block_scan_progress(chain_name)`,
		`CREATE INDEX IF NOT EXISTS idx_block_scan_progress_status ON block_scan_progress(scan_status)`,
		`CREATE INDEX IF NOT EXISTS idx_block_scan_progress_last_update ON block_scan_progress(last_update_time DESC)`,

		// Compound Timelock Transactions 表索引
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_hash ON compound_timelock_transactions(tx_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_block_number ON compound_timelock_transactions(block_number DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_block_timestamp ON compound_timelock_transactions(block_timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_chain_id ON compound_timelock_transactions(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_chain_name ON compound_timelock_transactions(chain_name)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_contract ON compound_timelock_transactions(contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_from_address ON compound_timelock_transactions(from_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_to_address ON compound_timelock_transactions(to_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_status ON compound_timelock_transactions(tx_status)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_event_type ON compound_timelock_transactions(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_event_tx_hash ON compound_timelock_transactions(event_tx_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_event_target ON compound_timelock_transactions(event_target)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_event_eta ON compound_timelock_transactions(event_eta)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_contract_type ON compound_timelock_transactions(contract_address, event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_contract_timestamp ON compound_timelock_transactions(contract_address, block_timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_chain_block ON compound_timelock_transactions(chain_id, block_number DESC)`,

		// OpenZeppelin Timelock Transactions 表索引
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_hash ON openzeppelin_timelock_transactions(tx_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_block_number ON openzeppelin_timelock_transactions(block_number DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_block_timestamp ON openzeppelin_timelock_transactions(block_timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_chain_id ON openzeppelin_timelock_transactions(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_chain_name ON openzeppelin_timelock_transactions(chain_name)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_contract ON openzeppelin_timelock_transactions(contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_from_address ON openzeppelin_timelock_transactions(from_address)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_to_address ON openzeppelin_timelock_transactions(to_address)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_status ON openzeppelin_timelock_transactions(tx_status)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_event_type ON openzeppelin_timelock_transactions(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_event_id ON openzeppelin_timelock_transactions(event_id)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_event_target ON openzeppelin_timelock_transactions(event_target)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_event_predecessor ON openzeppelin_timelock_transactions(event_predecessor)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_contract_type ON openzeppelin_timelock_transactions(contract_address, event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_contract_timestamp ON openzeppelin_timelock_transactions(contract_address, block_timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_chain_block ON openzeppelin_timelock_transactions(chain_id, block_number DESC)`,

		// Timelock Transaction Flows 表索引
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_flow_id ON timelock_transaction_flows(flow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_standard ON timelock_transaction_flows(timelock_standard)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_chain_id ON timelock_transaction_flows(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_contract ON timelock_transaction_flows(contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_status ON timelock_transaction_flows(status)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_propose_tx ON timelock_transaction_flows(propose_tx_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_queue_tx ON timelock_transaction_flows(queue_tx_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_execute_tx ON timelock_transaction_flows(execute_tx_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_cancel_tx ON timelock_transaction_flows(cancel_tx_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_eta ON timelock_transaction_flows(eta)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_target ON timelock_transaction_flows(target_address)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_proposed_at ON timelock_transaction_flows(proposed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_executed_at ON timelock_transaction_flows(executed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_contract_status ON timelock_transaction_flows(contract_address, status)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_chain_contract ON timelock_transaction_flows(chain_id, contract_address)`,

		// User Timelock Relations 表索引
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_user ON user_timelock_relations(user_address)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_chain_id ON user_timelock_relations(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_contract ON user_timelock_relations(contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_standard ON user_timelock_relations(timelock_standard)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_type ON user_timelock_relations(relation_type)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_is_active ON user_timelock_relations(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_related_at ON user_timelock_relations(related_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_user_chain ON user_timelock_relations(user_address, chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_user_contract ON user_timelock_relations(user_address, contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_user_type ON user_timelock_relations(user_address, relation_type)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_contract_type ON user_timelock_relations(contract_address, relation_type)`,
		`CREATE INDEX IF NOT EXISTS idx_user_timelock_relations_user_active ON user_timelock_relations(user_address, is_active)`,

		// 复合索引，用于优化常见查询
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_chain_contract_type ON compound_timelock_transactions(chain_id, contract_address, event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_chain_contract_type ON openzeppelin_timelock_transactions(chain_id, contract_address, event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_user_assets_wallet_native_value ON user_assets(wallet_address, is_native, usd_value DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_timelock_flows_contract_status_eta ON timelock_transaction_flows(contract_address, status, eta)`,

		// JSONB 索引，用于事件数据查询
		`CREATE INDEX IF NOT EXISTS idx_compound_tx_event_data_gin ON compound_timelock_transactions USING gin(event_data)`,
		`CREATE INDEX IF NOT EXISTS idx_openzeppelin_tx_event_data_gin ON openzeppelin_timelock_transactions USING gin(event_data)`,
	}

	for _, indexSQL := range indexes {
		if err := h.db.WithContext(ctx).Exec(indexSQL).Error; err != nil {
			logger.Error("Failed to create index", err, "sql", indexSQL)
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	logger.Info("Created all indexes successfully")
	return nil
}

// insertSupportedChains 插入支持的链数据（v1.0.2）
func (h *MigrationHandler) insertSupportedChains(ctx context.Context) error {
	logger.Info("Inserting supported chains data...")

	// 检查是否已有数据
	var count int64
	h.db.WithContext(ctx).Table("support_chains").Count(&count)
	if count > 0 {
		logger.Info("Supported chains data already exists, skipping insertion")
		return nil
	}

	// 主网数据
	mainnets := []map[string]interface{}{
		{
			"chain_name":               "eth-mainnet",
			"display_name":             "Ethereum Mainnet",
			"chain_id":                 1,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/eth-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://eth-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://ethereum.publicnode.com","https://rpc.ankr.com/eth"]`,
			"block_explorer_urls":      `["https://etherscan.io"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "bsc-mainnet",
			"display_name":             "BNB Smart Chain",
			"chain_id":                 56,
			"native_currency_name":     "BNB",
			"native_currency_symbol":   "BNB",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/bsc-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://bnb-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://bsc-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://bsc.drpc.org", "https://bsc.blockrazor.xyz"]`,
			"block_explorer_urls":      `["https://bscscan.com"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "matic-mainnet",
			"display_name":             "Polygon Mainnet",
			"chain_id":                 137,
			"native_currency_name":     "MATIC",
			"native_currency_symbol":   "MATIC",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/matic-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://polygon-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://polygon-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://polygon-rpc.com","https://polygon.drpc.org"]`,
			"block_explorer_urls":      `["https://polygonscan.com"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "arbitrum-mainnet",
			"display_name":             "Arbitrum One",
			"chain_id":                 42161,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/arbitrum-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://arb-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://arbitrum-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://arbitrum.drpc.org", "https://arb-pokt.nodies.app"]`,
			"block_explorer_urls":      `["https://arbiscan.io"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "optimism-mainnet",
			"display_name":             "Optimism",
			"chain_id":                 10,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/optimism-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://opt-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://optimism-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://mainnet.optimism.io","https://optimism.drpc.org"]`,
			"block_explorer_urls":      `["https://optimistic.etherscan.io"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "base-mainnet",
			"display_name":             "Base",
			"chain_id":                 8453,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/base-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://base-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://base-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://mainnet.base.org","https://base.llamarpc.com"]`,
			"block_explorer_urls":      `["https://basescan.org"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "avalanche-mainnet",
			"display_name":             "Avalanche C-Chain",
			"chain_id":                 43114,
			"native_currency_name":     "Avalanche",
			"native_currency_symbol":   "AVAX",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/avalanche-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://avax-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://avalanche-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://avalanche.drpc.org","https://avalanche-c-chain-rpc.publicnode.com"]`,
			"block_explorer_urls":      `["https://subnets.avax.network/c-chain"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "world-mainnet",
			"display_name":             "World Chain",
			"chain_id":                 480,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/world-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://worldchain-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://worldchain-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://worldchain.drpc.org"]`,
			"block_explorer_urls":      `["https://worldscan.org"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "zksync-mainnet",
			"display_name":             "ZKSync Era",
			"chain_id":                 324,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/zksync-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://zksync-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://zksync-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://mainnet.era.zksync.io","https://rpc.ankr.com/zksync_era"]`,
			"block_explorer_urls":      `["https://explorer.zksync.io"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "berachain-mainnet",
			"display_name":             "Berachain",
			"chain_id":                 80094,
			"native_currency_name":     "BERA",
			"native_currency_symbol":   "BERA",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/berachain-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://berachain-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://berachain-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://berachain.drpc.org"]`,
			"block_explorer_urls":      `["https://berascan.com"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "mantle-mainnet",
			"display_name":             "Mantle",
			"chain_id":                 5000,
			"native_currency_name":     "MNT",
			"native_currency_symbol":   "MNT",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/mantle-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://mantle-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://mantle-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://rpc.mantle.xyz"]`,
			"block_explorer_urls":      `["https://explorer.mantle.xyz"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "linea-mainnet",
			"display_name":             "Linea",
			"chain_id":                 59144,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/linea-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://linea-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://linea-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://rpc.linea.build", "https://linea.drpc.org"]`,
			"block_explorer_urls":      `["https://lineascan.build"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "gnosis-mainnet",
			"display_name":             "Gnosis",
			"chain_id":                 100,
			"native_currency_name":     "xDai",
			"native_currency_symbol":   "xDai",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/gnosis-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://gnosis-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://gnosis-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://rpc.gnosis.gateway.fm", "https://gnosis.drpc.org"]`,
			"block_explorer_urls":      `["https://gnosisscan.io"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "ink-mainnet",
			"display_name":             "Ink",
			"chain_id":                 57073,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/ink-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://ink-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://ink-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://ink.drpc.org"]`,
			"block_explorer_urls":      `["https://explorer.inkonchain.com"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "scroll-mainnet",
			"display_name":             "Scroll",
			"chain_id":                 534352,
			"native_currency_name":     "SCROLL",
			"native_currency_symbol":   "SCROLL",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/scroll-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://scroll-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://scroll-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://rpc.scroll.io"]`,
			"block_explorer_urls":      `["https://scrollscan.com"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "celo-mainnet",
			"display_name":             "Celo",
			"chain_id":                 42220,
			"native_currency_name":     "CELO",
			"native_currency_symbol":   "CELO",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/celo-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://celo-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://celo-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://celo.drpc.org"]`,
			"block_explorer_urls":      `["https://celoscan.io"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "unichain-mainnet",
			"display_name":             "Unichain",
			"chain_id":                 130,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/unichain-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://unichain-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://unichain-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://unichain.drpc.org"]`,
			"block_explorer_urls":      `["https://unichain.blockscout.com"]`,
			"rpc_enabled":              true,
		},
		{
			// ronin-mainnet
			"chain_name":               "axie-mainnet",
			"display_name":             "Ronin",
			"chain_id":                 2020,
			"native_currency_name":     "RON",
			"native_currency_symbol":   "RON",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/ronin-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://ronin-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://ronin-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://ronin.drpc.org"]`,
			"block_explorer_urls":      `["https://explorer.roninchain.com"]`,
			"rpc_enabled":              true,
		},
	}

	// 测试网数据
	testnets := []map[string]interface{}{
		{
			"chain_name":               "eth-sepolia",
			"display_name":             "Ethereum Sepolia",
			"chain_id":                 11155111,
			"native_currency_name":     "Sepolia Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/eth-sepolia.png",
			"is_testnet":               true,
			"is_active":                true,
			"alchemy_rpc_template":     "https://eth-sepolia.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://sepolia.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://ethereum-sepolia-rpc.publicnode.com","https://1rpc.io/sepolia"]`,
			"block_explorer_urls":      `["https://sepolia.etherscan.io"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "monad-testnet",
			"display_name":             "Monad Testnet",
			"chain_id":                 10143,
			"native_currency_name":     "MON",
			"native_currency_symbol":   "MON",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/monad-testnet.png",
			"is_testnet":               true,
			"is_active":                true,
			"alchemy_rpc_template":     "https://monad-testnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://monad-testnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://monad-testnet.drpc.org", "https://testnet-rpc.monad.xyz"]`,
			"block_explorer_urls":      `["https://testnet.monadexplorer.com"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "bsc-testnet",
			"display_name":             "BNB Smart Chain Testnet",
			"chain_id":                 97,
			"native_currency_name":     "Test BNB",
			"native_currency_symbol":   "BNB",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/bsc-testnet.png",
			"is_testnet":               true,
			"is_active":                true,
			"alchemy_rpc_template":     "https://bnb-testnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://bsc-testnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://bsc-testnet-rpc.publicnode.com","https://bsc-testnet.drpc.org"]`,
			"block_explorer_urls":      `["https://testnet.bscscan.com"]`,
			"rpc_enabled":              true,
		},
	}

	// 插入主网数据
	for _, chain := range mainnets {
		if err := h.db.WithContext(ctx).Table("support_chains").Create(chain).Error; err != nil {
			logger.Error("Failed to insert mainnet chain", err, "chain_name", chain["chain_name"])
			return fmt.Errorf("failed to insert mainnet chain %s: %w", chain["chain_name"], err)
		}
	}

	// 插入测试网数据
	for _, chain := range testnets {
		if err := h.db.WithContext(ctx).Table("support_chains").Create(chain).Error; err != nil {
			logger.Error("Failed to insert testnet chain", err, "chain_name", chain["chain_name"])
			return fmt.Errorf("failed to insert testnet chain %s: %w", chain["chain_name"], err)
		}
	}

	logger.Info("Inserted all supported chains successfully")
	return nil
}

// insertSharedABIs 插入共享ABI数据（v1.0.3）
func (h *MigrationHandler) insertSharedABIs(ctx context.Context) error {
	logger.Info("Inserting shared ABIs data...")

	// 检查是否已有数据
	var count int64
	h.db.WithContext(ctx).Table("abis").Where("is_shared = ?", true).Count(&count)
	if count > 0 {
		logger.Info("Shared ABIs data already exists, skipping insertion")
		return nil
	}

	sharedABIs := []map[string]interface{}{
		{
			"name":        "ERC20 Token",
			"abi_content": `[{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transferFrom","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"spender","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]`,
			"owner":       "0x0000000000000000000000000000000000000000",
			"description": "Standard ERC-20 Token interface with basic functions for transferring tokens and checking balances.",
			"is_shared":   true,
		},
		{
			"name":        "ERC721 NFT",
			"abi_content": `[{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"approve","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"getApproved","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"operator","type":"address"}],"name":"isApprovedForAll","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"ownerOf","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"safeTransferFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"bytes","name":"data","type":"bytes"}],"name":"safeTransferFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"operator","type":"address"},{"internalType":"bool","name":"approved","type":"bool"}],"name":"setApprovalForAll","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes4","name":"interfaceId","type":"bytes4"}],"name":"supportsInterface","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"tokenURI","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"transferFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"approved","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"operator","type":"address"},{"indexed":false,"internalType":"bool","name":"approved","type":"bool"}],"name":"ApprovalForAll","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"Transfer","type":"event"}]`,
			"owner":       "0x0000000000000000000000000000000000000000",
			"description": "Standard ERC-721 Non-Fungible Token interface with functions for managing unique tokens.",
			"is_shared":   true,
		},
		{
			"name":        "Uniswap V2 Pair",
			"abi_content": `[{"inputs":[],"name":"DOMAIN_SEPARATOR","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"MINIMUM_LIQUIDITY","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"PERMIT_TYPEHASH","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"}],"name":"burn","outputs":[{"internalType":"uint256","name":"amount0","type":"uint256"},{"internalType":"uint256","name":"amount1","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"factory","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint112","name":"_reserve0","type":"uint112"},{"internalType":"uint112","name":"_reserve1","type":"uint112"},{"internalType":"uint32","name":"_blockTimestampLast","type":"uint32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"_token0","type":"address"},{"internalType":"address","name":"_token1","type":"address"}],"name":"initialize","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"kLast","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"}],"name":"mint","outputs":[{"internalType":"uint256","name":"liquidity","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"nonces","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"price0CumulativeLast","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"price1CumulativeLast","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"}],"name":"skim","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"amount0Out","type":"uint256"},{"internalType":"uint256","name":"amount1Out","type":"uint256"},{"internalType":"address","name":"to","type":"address"},{"internalType":"bytes","name":"data","type":"bytes"}],"name":"swap","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"sync","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"token0","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"token1","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`,
			"owner":       "0x0000000000000000000000000000000000000000",
			"description": "Uniswap V2 trading pair contract interface for decentralized token swapping.",
			"is_shared":   true,
		},
		{
			"name":        "OpenZeppelin TimelockController",
			"abi_content": `[{"inputs":[{"internalType":"uint256","name":"minDelay","type":"uint256"},{"internalType":"address[]","name":"proposers","type":"address[]"},{"internalType":"address[]","name":"executors","type":"address[]"},{"internalType":"address","name":"admin","type":"address"}],"stateMutability":"nonpayable","type":"constructor"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"grantRole","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"hasRole","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"isOperation","outputs":[{"internalType":"bool","name":"pending","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"isOperationDone","outputs":[{"internalType":"bool","name":"done","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"isOperationPending","outputs":[{"internalType":"bool","name":"pending","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"isOperationReady","outputs":[{"internalType":"bool","name":"ready","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"target","type":"address"},{"internalType":"uint256","name":"value","type":"uint256"},{"internalType":"bytes","name":"data","type":"bytes"},{"internalType":"bytes32","name":"predecessor","type":"bytes32"},{"internalType":"bytes32","name":"salt","type":"bytes32"}],"name":"execute","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"address","name":"target","type":"address"},{"internalType":"uint256","name":"value","type":"uint256"},{"internalType":"bytes","name":"data","type":"bytes"},{"internalType":"bytes32","name":"predecessor","type":"bytes32"},{"internalType":"bytes32","name":"salt","type":"bytes32"},{"internalType":"uint256","name":"delay","type":"uint256"}],"name":"schedule","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"cancel","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"getMinDelay","outputs":[{"internalType":"uint256","name":"duration","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"getTimestamp","outputs":[{"internalType":"uint256","name":"timestamp","type":"uint256"}],"stateMutability":"view","type":"function"}]`,
			"owner":       "0x0000000000000000000000000000000000000000",
			"description": "OpenZeppelin TimelockController contract for time-delayed execution of governance proposals.",
			"is_shared":   true,
		},
	}

	for _, abi := range sharedABIs {
		if err := h.db.WithContext(ctx).Table("abis").Create(abi).Error; err != nil {
			logger.Error("Failed to insert shared ABI", err, "name", abi["name"])
			return fmt.Errorf("failed to insert shared ABI %s: %w", abi["name"], err)
		}
	}

	logger.Info("Inserted all shared ABIs successfully")
	return nil
}

// GetMigrationStatus 获取迁移状态（用于监控和调试）
func GetMigrationStatus(db *gorm.DB) ([]Migration, error) {
	var migrations []Migration
	err := db.Order("created_at ASC").Find(&migrations).Error
	return migrations, err
}

// ResetDangerous 危险的重置函数 - 仅用于开发环境
// 此函数会删除所有数据，请谨慎使用
func ResetDangerous(db *gorm.DB) error {
	ctx := context.Background()
	logger.Warn("DANGEROUS: Starting database reset - ALL DATA WILL BE LOST!")

	// 删除所有表（逆序删除以避免外键约束问题）
	tables := []string{
		"compound_timelocks",
		"openzeppelin_timelocks",
		"user_assets",
		"abis",
		"sponsors",
		"support_chains",
		"users",
		"schema_migrations",
	}

	for _, table := range tables {
		if err := db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)).Error; err != nil {
			logger.Error("Failed to drop table", err, "table", table)
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	logger.Warn("Database reset completed - all tables dropped")
	return nil
}

// insertDefaultSponsors 插入默认赞助方数据（v1.0.4）
func (h *MigrationHandler) insertDefaultSponsors(ctx context.Context) error {
	logger.Info("Inserting default sponsors data...")

	// 检查是否已有数据
	var count int64
	h.db.WithContext(ctx).Table("sponsors").Count(&count)
	if count > 0 {
		logger.Info("Sponsors data already exists, skipping insertion")
		return nil
	}

	// 赞助方数据
	sponsors := []map[string]interface{}{
		{
			"name":        "AAVE",
			"logo_url":    "https://raw.githubusercontent.com/timelock-labs/assets/main/sponsors/AAVE.png",
			"link":        "https://aave.com",
			"description": "Decentralized lending and borrowing protocol.",
			"type":        "sponsor",
			"sort_order":  100,
			"is_active":   true,
		},
		{
			"name":        "Lido",
			"logo_url":    "https://raw.githubusercontent.com/timelock-labs/assets/main/sponsors/Lido.jpg",
			"link":        "https://lido.fi",
			"description": "Liquid staking solution for Ethereum.",
			"type":        "sponsor",
			"sort_order":  90,
			"is_active":   true,
		},
		{
			"name":        "EigenLayer",
			"logo_url":    "https://raw.githubusercontent.com/timelock-labs/assets/main/sponsors/EigenLayer.jpg",
			"link":        "https://www.eigenlayer.xyz",
			"description": "Restaking protocol for Ethereum.",
			"type":        "sponsor",
			"sort_order":  80,
			"is_active":   true,
		},
	}

	// 生态伙伴数据
	partners := []map[string]interface{}{
		{
			"name":        "Ethena",
			"logo_url":    "https://raw.githubusercontent.com/timelock-labs/assets/main/sponsors/Ethena.png",
			"link":        "https://ethena.fi",
			"description": "Synthetic dollar protocol.",
			"type":        "partner",
			"sort_order":  100,
			"is_active":   true,
		},
		{
			"name":        "Uniswap",
			"logo_url":    "https://raw.githubusercontent.com/timelock-labs/assets/main/sponsors/Uniswap.jpg",
			"link":        "https://uniswap.org",
			"description": "Decentralized exchange protocol.",
			"type":        "partner",
			"sort_order":  90,
			"is_active":   true,
		},
		{
			"name":        "Compound",
			"logo_url":    "https://raw.githubusercontent.com/timelock-labs/assets/main/sponsors/Compound.png",
			"link":        "https://compound.finance",
			"description": "Decentralized lending protocol.",
			"type":        "partner",
			"sort_order":  50,
			"is_active":   true,
		},
		{
			"name":        "OpenZeppelin",
			"logo_url":    "https://raw.githubusercontent.com/timelock-labs/assets/main/sponsors/OpenZeppelin.png",
			"link":        "https://openzeppelin.com",
			"description": "Smart contract development platform.",
			"type":        "partner",
			"sort_order":  40,
			"is_active":   true,
		},
	}

	// 插入赞助方数据
	for _, sponsor := range sponsors {
		if err := h.db.WithContext(ctx).Table("sponsors").Create(sponsor).Error; err != nil {
			logger.Error("Failed to insert sponsor", err, "name", sponsor["name"])
			return fmt.Errorf("failed to insert sponsor %s: %w", sponsor["name"], err)
		}
	}

	// 插入生态伙伴数据
	for _, partner := range partners {
		if err := h.db.WithContext(ctx).Table("sponsors").Create(partner).Error; err != nil {
			logger.Error("Failed to insert partner", err, "name", partner["name"])
			return fmt.Errorf("failed to insert partner %s: %w", partner["name"], err)
		}
	}

	logger.Info("Inserted all sponsors and partners successfully")
	return nil
}
