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
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			last_login TIMESTAMP WITH TIME ZONE,
			status INTEGER DEFAULT 1,
			is_safe_wallet BOOLEAN DEFAULT FALSE,
			safe_threshold INTEGER,
			safe_owners TEXT
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
			chain_name VARCHAR(100) NOT NULL UNIQUE,
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
			subgraph_url TEXT,  -- Goldsky subgraph URL
			compound_webhook_secret TEXT,  -- Goldsky Compound Timelock Transaction webhook secret
			oz_webhook_secret TEXT,  -- Goldsky OpenZeppelin Timelock Transaction webhook secret
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`

		if err := h.db.WithContext(ctx).Exec(createSupportChainsTable).Error; err != nil {
			return fmt.Errorf("failed to create support_chains table: %w", err)
		}
		logger.Info("Created table: support_chains")
	}

	// 3. ABI库表
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

	// 4. Compound标准Timelock合约表
	if !h.db.Migrator().HasTable("compound_timelocks") {
		createCompoundTimelocksTable := `
		CREATE TABLE compound_timelocks (
			id BIGSERIAL PRIMARY KEY,
			creator_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE,
			chain_id INTEGER NOT NULL,
			chain_name VARCHAR(100) NOT NULL,
			contract_address VARCHAR(42) NOT NULL,
			delay BIGINT NOT NULL,
			admin VARCHAR(42) NOT NULL,
			pending_admin VARCHAR(42),
			grace_period BIGINT NOT NULL,
			minimum_delay BIGINT NOT NULL,
			maximum_delay BIGINT NOT NULL,
			remark VARCHAR(500) DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'deleted')),
			is_imported BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(creator_address, chain_id, contract_address)
		)`

		if err := h.db.WithContext(ctx).Exec(createCompoundTimelocksTable).Error; err != nil {
			return fmt.Errorf("failed to create compound_timelocks table: %w", err)
		}
		logger.Info("Created table: compound_timelocks")
	}

	// 5. OpenZeppelin标准Timelock合约表
	if !h.db.Migrator().HasTable("openzeppelin_timelocks") {
		createOpenzeppelinTimelocksTable := `
		CREATE TABLE openzeppelin_timelocks (
			id BIGSERIAL PRIMARY KEY,
			creator_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE,
			chain_id INTEGER NOT NULL,
			chain_name VARCHAR(100) NOT NULL,
			contract_address VARCHAR(42) NOT NULL,
			delay BIGINT NOT NULL,
			admin VARCHAR(42),
			proposers TEXT NOT NULL,
			executors TEXT NOT NULL,
			remark VARCHAR(500) DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'deleted')),
			is_imported BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(creator_address, chain_id, contract_address)
		)`

		if err := h.db.WithContext(ctx).Exec(createOpenzeppelinTimelocksTable).Error; err != nil {
			return fmt.Errorf("failed to create openzeppelin_timelocks table: %w", err)
		}
		logger.Info("Created table: openzeppelin_timelocks")
	}

	// 6. emails 表
	if !h.db.Migrator().HasTable("emails") {
		sql := `
        CREATE TABLE emails (
            id BIGSERIAL PRIMARY KEY,
            email VARCHAR(200) NOT NULL UNIQUE,
            is_deliverable BOOLEAN NOT NULL DEFAULT TRUE,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW()
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create emails table: %w", err)
		}
		logger.Info("Created table: emails")
	}

	// 7. error_logs 表 - 用于记录 logger.error 的错误详情
	if !h.db.Migrator().HasTable("error_logs") {
		sql := `
        CREATE TABLE error_logs (
            id BIGSERIAL PRIMARY KEY,
            timestamp TIMESTAMPTZ DEFAULT NOW(),          -- 时间戳
            caller VARCHAR(255) NOT NULL,                 -- 调用者 (文件:行号)
            function VARCHAR(255) NOT NULL,               -- 函数名
            message TEXT NOT NULL,                        -- 日志消息
            error TEXT,                                   -- 错误信息
            context TEXT                                  -- 上下文信息 (JSON字符串)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create error_logs table: %w", err)
		}
		logger.Info("Created table: error_logs")
	}

	// 8. user_emails 表
	if !h.db.Migrator().HasTable("user_emails") {
		sql := `
        CREATE TABLE user_emails (
            id BIGSERIAL PRIMARY KEY,
            user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            email_id BIGINT NOT NULL REFERENCES emails(id) ON DELETE CASCADE,
            remark VARCHAR(500),
            is_verified BOOLEAN NOT NULL DEFAULT FALSE,
            last_verified_at TIMESTAMPTZ,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE(user_id, email_id)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create user_emails table: %w", err)
		}
		logger.Info("Created table: user_emails")
	}

	// 9. email_verification_codes 表
	if !h.db.Migrator().HasTable("email_verification_codes") {
		sql := `
        CREATE TABLE email_verification_codes (
            id BIGSERIAL PRIMARY KEY,
            user_email_id BIGINT NOT NULL REFERENCES user_emails(id) ON DELETE CASCADE,
            code VARCHAR(16) NOT NULL,
            expires_at TIMESTAMPTZ NOT NULL,
            sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            attempt_count INTEGER NOT NULL DEFAULT 0,
            is_used BOOLEAN NOT NULL DEFAULT FALSE
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create email_verification_codes table: %w", err)
		}
		logger.Info("Created table: email_verification_codes")
	}

	// 10. email_send_logs 表（按邮箱去重）
	if !h.db.Migrator().HasTable("email_send_logs") {
		sql := `
        CREATE TABLE email_send_logs (
            id BIGSERIAL PRIMARY KEY,
            email_id BIGINT NOT NULL REFERENCES emails(id) ON DELETE CASCADE,
            flow_id VARCHAR(128) NOT NULL,
            timelock_standard VARCHAR(20) NOT NULL,
            chain_id INTEGER NOT NULL,
            contract_address VARCHAR(42) NOT NULL,
            status_from VARCHAR(20),
            status_to VARCHAR(20) NOT NULL,
            tx_hash VARCHAR(66),
            send_status VARCHAR(20) NOT NULL CHECK (send_status IN ('success','failed')),
            error_message TEXT,
            retry_count INTEGER NOT NULL DEFAULT 0,
            sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            UNIQUE(email_id, flow_id, status_to)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create email_send_logs table: %w", err)
		}
		logger.Info("Created table: email_send_logs")
	}

	// 11. safe_wallets 表
	if !h.db.Migrator().HasTable("safe_wallets") {
		sql := `
        CREATE TABLE safe_wallets (
            id BIGSERIAL PRIMARY KEY,
            safe_address VARCHAR(42) NOT NULL,
            chain_id INTEGER NOT NULL,
            chain_name VARCHAR(50) NOT NULL,
            threshold INTEGER NOT NULL,
            owners TEXT NOT NULL,
            version VARCHAR(20),
            status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE(safe_address, chain_id)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create safe_wallets table: %w", err)
		}
		logger.Info("Created table: safe_wallets")
	}

	// 12. auth_nonces 表
	if !h.db.Migrator().HasTable("auth_nonces") {
		sql := `
        CREATE TABLE auth_nonces (
            id BIGSERIAL PRIMARY KEY,
            wallet_address VARCHAR(42) NOT NULL,
            nonce VARCHAR(128) NOT NULL,
            message TEXT NOT NULL,
            expires_at TIMESTAMPTZ NOT NULL,
            is_used BOOLEAN NOT NULL DEFAULT FALSE,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            UNIQUE(wallet_address, nonce)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create auth_nonces table: %w", err)
		}
		logger.Info("Created table: auth_nonces")
	}

	// 13. telegram_configs 表
	if !h.db.Migrator().HasTable("telegram_configs") {
		sql := `
        CREATE TABLE telegram_configs (
            id BIGSERIAL PRIMARY KEY,
            user_address VARCHAR(42) NOT NULL,
			name VARCHAR(100) NOT NULL,
            bot_token VARCHAR(500) NOT NULL,
            chat_id VARCHAR(100) NOT NULL,
            is_active BOOLEAN NOT NULL DEFAULT TRUE,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(user_address, name)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create telegram_configs table: %w", err)
		}
		logger.Info("Created table: telegram_configs")
	}

	// 14. lark_configs 表
	if !h.db.Migrator().HasTable("lark_configs") {
		sql := `
        CREATE TABLE lark_configs (
            id BIGSERIAL PRIMARY KEY,
            user_address VARCHAR(42) NOT NULL,
			name VARCHAR(100) NOT NULL,
            webhook_url VARCHAR(1000) NOT NULL,
            secret VARCHAR(500) DEFAULT '',
            is_active BOOLEAN NOT NULL DEFAULT TRUE,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(user_address, name)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create lark_configs table: %w", err)
		}
		logger.Info("Created table: lark_configs")
	}

	// 15. feishu_configs 表
	if !h.db.Migrator().HasTable("feishu_configs") {
		sql := `
        CREATE TABLE feishu_configs (
            id BIGSERIAL PRIMARY KEY,
            user_address VARCHAR(42) NOT NULL,
			name VARCHAR(100) NOT NULL,
            webhook_url VARCHAR(1000) NOT NULL,
            secret VARCHAR(500) DEFAULT '',
            is_active BOOLEAN NOT NULL DEFAULT TRUE,
            created_at TIMESTAMPTZ DEFAULT NOW(),
            updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(user_address, name)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create feishu_configs table: %w", err)
		}
		logger.Info("Created table: feishu_configs")
	}

	// 16. notification_logs 表
	if !h.db.Migrator().HasTable("notification_logs") {
		sql := `
        CREATE TABLE notification_logs (
            id BIGSERIAL PRIMARY KEY,
            user_address VARCHAR(42) NOT NULL,
			channel VARCHAR(20) NOT NULL,
            config_id BIGINT NOT NULL,
            flow_id VARCHAR(128) NOT NULL,
            timelock_standard VARCHAR(20) NOT NULL,
            chain_id INTEGER NOT NULL,
            contract_address VARCHAR(42) NOT NULL,
            status_from VARCHAR(20),
            status_to VARCHAR(20) NOT NULL,
            tx_hash VARCHAR(66),
			send_status VARCHAR(20) NOT NULL CHECK (send_status IN ('success','failed')),
            error_message TEXT,
			sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(channel, config_id, flow_id, status_to)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create notification_logs table: %w", err)
		}
		logger.Info("Created table: notification_logs")
	}

	// 17. compound_timelock_flows 表
	if !h.db.Migrator().HasTable("compound_timelock_flows") {
		sql := `
        CREATE TABLE compound_timelock_flows (
            id BIGSERIAL PRIMARY KEY,
            flow_id VARCHAR(128) NOT NULL,
            timelock_standard VARCHAR(20) NOT NULL DEFAULT 'compound',
            chain_id INTEGER NOT NULL,
            contract_address VARCHAR(42) NOT NULL,
            status VARCHAR(20) NOT NULL DEFAULT 'waiting',
            
            -- 关联的交易哈希
            queue_tx_hash VARCHAR(66),
            execute_tx_hash VARCHAR(66),
            cancel_tx_hash VARCHAR(66),
            
            -- Flow 信息
            initiator_address VARCHAR(42),
            target_address VARCHAR(42),
            value DECIMAL(78,0) NOT NULL DEFAULT 0,
            call_data BYTEA,
            function_signature TEXT,
            
            -- 时间信息
            queued_at TIMESTAMPTZ,
            eta TIMESTAMPTZ,
            grace_period BIGINT,
            expired_at TIMESTAMPTZ,
            executed_at TIMESTAMPTZ,
            cancelled_at TIMESTAMPTZ,
            
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            
            UNIQUE(flow_id, chain_id, contract_address)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create compound_timelock_flows table: %w", err)
		}
		logger.Info("Created table: compound_timelock_flows")
	}

	// 18. openzeppelin_timelock_flows 表
	if !h.db.Migrator().HasTable("openzeppelin_timelock_flows") {
		sql := `
        CREATE TABLE openzeppelin_timelock_flows (
            id BIGSERIAL PRIMARY KEY,
            flow_id VARCHAR(128) NOT NULL,
            timelock_standard VARCHAR(20) NOT NULL DEFAULT 'openzeppelin',
            chain_id INTEGER NOT NULL,
            contract_address VARCHAR(42) NOT NULL,
            status VARCHAR(20) NOT NULL DEFAULT 'waiting',
            
            -- 关联的交易哈希
            schedule_tx_hash VARCHAR(66),
            execute_tx_hash VARCHAR(66),
            cancel_tx_hash VARCHAR(66),
            
            -- Flow 信息
            initiator_address VARCHAR(42),
            target_address VARCHAR(42),
            value DECIMAL(78,0) NOT NULL DEFAULT 0,
            call_data BYTEA,
            
            -- 时间信息
            queued_at TIMESTAMPTZ,
            delay BIGINT,
            eta TIMESTAMPTZ,
            executed_at TIMESTAMPTZ,
            cancelled_at TIMESTAMPTZ,
            
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            
            UNIQUE(flow_id, chain_id, contract_address)
        )`
		if err := h.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create openzeppelin_timelock_flows table: %w", err)
		}
		logger.Info("Created table: openzeppelin_timelock_flows")
	}

	logger.Info("All tables created successfully")
	return nil
}

// createIndexes 创建索引（v1.0.1）
func (h *MigrationHandler) createIndexes(ctx context.Context) error {
	logger.Info("Creating database indexes...")

	indexes := []string{
		// Users
		`CREATE INDEX IF NOT EXISTS idx_users_status ON users(status)`,

		// Support Chains
		`CREATE INDEX IF NOT EXISTS idx_support_chains_chain_id ON support_chains(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_support_chains_active_rpc ON support_chains(is_active, rpc_enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_support_chains_testnet ON support_chains(is_testnet)`,

		// ABIs
		`CREATE INDEX IF NOT EXISTS idx_abis_owner ON abis(owner)`,
		`CREATE INDEX IF NOT EXISTS idx_abis_is_shared ON abis(is_shared)`,

		// Compound Timelocks
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_creator ON compound_timelocks(creator_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_admin ON compound_timelocks(admin)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_pending_admin ON compound_timelocks(pending_admin)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_status ON compound_timelocks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_chain_address ON compound_timelocks(chain_id, contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_timelocks_creator_chain_address ON compound_timelocks(creator_address, chain_id, contract_address)`,

		// OpenZeppelin Timelocks
		`CREATE INDEX IF NOT EXISTS idx_oz_timelocks_creator ON openzeppelin_timelocks(creator_address)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_timelocks_admin ON openzeppelin_timelocks(admin)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_timelocks_status ON openzeppelin_timelocks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_timelocks_chain_address ON openzeppelin_timelocks(chain_id, contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_timelocks_creator_chain_address ON openzeppelin_timelocks(creator_address, chain_id, contract_address)`,

		// Email & Notification
		`CREATE INDEX IF NOT EXISTS idx_emails_email ON emails(email)`,
		`CREATE INDEX IF NOT EXISTS idx_user_emails_user ON user_emails(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_emails_email ON user_emails(email_id)`,
		`CREATE INDEX IF NOT EXISTS idx_verification_codes_user_email ON email_verification_codes(user_email_id, is_used)`,
		`CREATE INDEX IF NOT EXISTS idx_verification_codes_expires ON email_verification_codes(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_send_logs_flow_status ON email_send_logs(flow_id, status_to)`,
		`CREATE INDEX IF NOT EXISTS idx_send_logs_contract ON email_send_logs(timelock_standard, chain_id, contract_address)`,

		// Safe Wallets
		`CREATE INDEX IF NOT EXISTS idx_safe_wallets_address ON safe_wallets(safe_address)`,
		`CREATE INDEX IF NOT EXISTS idx_safe_wallets_chain ON safe_wallets(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_safe_wallets_status ON safe_wallets(status)`,
		`CREATE INDEX IF NOT EXISTS idx_safe_wallets_address_chain ON safe_wallets(safe_address, chain_id)`,

		// Auth Nonces
		`CREATE INDEX IF NOT EXISTS idx_auth_nonces_wallet ON auth_nonces(wallet_address)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_nonces_expires ON auth_nonces(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_nonces_used ON auth_nonces(is_used)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_nonces_wallet_nonce ON auth_nonces(wallet_address, nonce)`,

		// Error Logs
		`CREATE INDEX IF NOT EXISTS idx_error_logs_timestamp ON error_logs(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_error_logs_caller ON error_logs(caller)`,
		`CREATE INDEX IF NOT EXISTS idx_error_logs_function ON error_logs(function)`,
		`CREATE INDEX IF NOT EXISTS idx_error_logs_message_gin ON error_logs USING GIN (to_tsvector('english', message))`,

		// Notification Channels
		`CREATE INDEX IF NOT EXISTS idx_telegram_configs_user ON telegram_configs(user_address)`,
		`CREATE INDEX IF NOT EXISTS idx_telegram_configs_active ON telegram_configs(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_telegram_configs_user_name ON telegram_configs(user_address, name)`,
		`CREATE INDEX IF NOT EXISTS idx_lark_configs_user ON lark_configs(user_address)`,
		`CREATE INDEX IF NOT EXISTS idx_lark_configs_active ON lark_configs(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_lark_configs_user_name ON lark_configs(user_address, name)`,
		`CREATE INDEX IF NOT EXISTS idx_feishu_configs_user ON feishu_configs(user_address)`,
		`CREATE INDEX IF NOT EXISTS idx_feishu_configs_active ON feishu_configs(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_feishu_configs_user_name ON feishu_configs(user_address, name)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_user ON notification_logs(user_address)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_flow ON notification_logs(flow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_channel ON notification_logs(channel)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_config ON notification_logs(config_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_send_status ON notification_logs(send_status)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_sent_at ON notification_logs(sent_at)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_contract ON notification_logs(timelock_standard, chain_id, contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_flow_status ON notification_logs(flow_id, status_to)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_logs_unique_check ON notification_logs(channel, config_id, flow_id, status_to)`,

		// Compound Timelock Flows
		`CREATE INDEX IF NOT EXISTS idx_compound_flows_flow_id ON compound_timelock_flows(flow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_flows_chain_contract ON compound_timelock_flows(chain_id, contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_flows_status ON compound_timelock_flows(status)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_flows_initiator ON compound_timelock_flows(initiator_address)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_flows_eta ON compound_timelock_flows(eta)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_flows_expired_at ON compound_timelock_flows(expired_at)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_flows_status_eta ON compound_timelock_flows(status, eta)`,
		`CREATE INDEX IF NOT EXISTS idx_compound_flows_queue_tx ON compound_timelock_flows(queue_tx_hash)`,

		// OpenZeppelin Timelock Flows
		`CREATE INDEX IF NOT EXISTS idx_oz_flows_flow_id ON openzeppelin_timelock_flows(flow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_flows_chain_contract ON openzeppelin_timelock_flows(chain_id, contract_address)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_flows_status ON openzeppelin_timelock_flows(status)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_flows_initiator ON openzeppelin_timelock_flows(initiator_address)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_flows_eta ON openzeppelin_timelock_flows(eta)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_flows_status_eta ON openzeppelin_timelock_flows(status, eta)`,
		`CREATE INDEX IF NOT EXISTS idx_oz_flows_schedule_tx ON openzeppelin_timelock_flows(schedule_tx_hash)`,
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
			"display_name":             "Ethereum",
			"chain_id":                 1,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/eth-mainnet.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://mainnet.infura.io/v3/",
			"infura_rpc_template":      "",
			"official_rpc_urls":        `["https://ethereum.publicnode.com","https://rpc.ankr.com/eth"]`,
			"block_explorer_urls":      `["https://etherscan.io"]`,
			"rpc_enabled":              true,
			"subgraph_url":             "https://api.goldsky.com/api/public/project_cmisnqs37gdlo01y7f60oa25h/subgraphs/timelock-eth-mainnet/1.0.0/gn",
		},
		// {
		// 	"chain_name":               "bsc-mainnet",
		// 	"display_name":             "BNB Chain",
		// 	"chain_id":                 56,
		// 	"native_currency_name":     "BNB",
		// 	"native_currency_symbol":   "BNB",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/bsc-mainnet.png",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://binance.llamarpc.com",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://bsc.drpc.org", "https://bsc.blockrazor.xyz"]`,
		// 	"block_explorer_urls":      `["https://bscscan.com"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "arbitrum-mainnet",
		// 	"display_name":             "Arbitrum",
		// 	"chain_id":                 42161,
		// 	"native_currency_name":     "Ether",
		// 	"native_currency_symbol":   "ETH",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/arbitrum-mainnet.png",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://arb1.arbitrum.io/rpc",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://arbitrum.drpc.org", "https://arb-pokt.nodies.app"]`,
		// 	"block_explorer_urls":      `["https://arbiscan.io"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "optimism-mainnet",
		// 	"display_name":             "Optimism",
		// 	"chain_id":                 10,
		// 	"native_currency_name":     "Ether",
		// 	"native_currency_symbol":   "ETH",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/optimism-mainnet.png",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://mainnet.optimism.io",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://mainnet.optimism.io","https://optimism.drpc.org"]`,
		// 	"block_explorer_urls":      `["https://optimistic.etherscan.io"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "base-mainnet",
		// 	"display_name":             "Base",
		// 	"chain_id":                 8453,
		// 	"native_currency_name":     "Ether",
		// 	"native_currency_symbol":   "ETH",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/base-mainnet.png",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://mainnet.base.org",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://mainnet.base.org","https://base.llamarpc.com"]`,
		// 	"block_explorer_urls":      `["https://basescan.org"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "linea-mainnet",
		// 	"display_name":             "Linea",
		// 	"chain_id":                 59144,
		// 	"native_currency_name":     "Ether",
		// 	"native_currency_symbol":   "ETH",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/linea-mainnet.png",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.linea.build",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.linea.build", "https://linea.drpc.org"]`,
		// 	"block_explorer_urls":      `["https://lineascan.build"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "scroll-mainnet",
		// 	"display_name":             "Scroll",
		// 	"chain_id":                 534352,
		// 	"native_currency_name":     "SCROLL",
		// 	"native_currency_symbol":   "SCROLL",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/scroll-mainnet.png",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.scroll.io",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.scroll.io"]`,
		// 	"block_explorer_urls":      `["https://scrollscan.com"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "xlayer-mainnet",
		// 	"display_name":             "X Layer",
		// 	"chain_id":                 196,
		// 	"native_currency_name":     "OKB",
		// 	"native_currency_symbol":   "OKB",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/xlayer-mainnet.png",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.xlayer.tech",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.xlayer.tech", "https://xlayerrpc.okx.com"]`,
		// 	"block_explorer_urls":      `["https://web3.okx.com/explorer/x-layer"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "bitlayer-mainnet",
		// 	"display_name":             "Bitlayer",
		// 	"chain_id":                 200901,
		// 	"native_currency_name":     "BTC",
		// 	"native_currency_symbol":   "BTC",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/bitlayer-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.ankr.com/bitlayer",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.ankr.com/bitlayer", "https://rpc-bitlayer.rockx.com"]`,
		// 	"block_explorer_urls":      `["https://www.btrscan.com"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "mode-mainnet",
		// 	"display_name":             "Mode",
		// 	"chain_id":                 34443,
		// 	"native_currency_name":     "Ether",
		// 	"native_currency_symbol":   "ETH",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/mode-mainnet.png",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://mainnet.mode.network",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://mainnet.mode.network", "https://mode.drpc.org"]`,
		// 	"block_explorer_urls":      `["https://explorer.mode.network"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "plume-mainnet",
		// 	"display_name":             "Plume",
		// 	"chain_id":                 98866,
		// 	"native_currency_name":     "Plume",
		// 	"native_currency_symbol":   "PLUME",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/plume-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.plume.org",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.plume.org"]`,
		// 	"block_explorer_urls":      `["https://explorer.plumenetwork.xyz"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "core-mainnet",
		// 	"display_name":             "Core",
		// 	"chain_id":                 1116,
		// 	"native_currency_name":     "Core",
		// 	"native_currency_symbol":   "CORE",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/core-mainnet.png",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.ankr.com/core",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.ankr.com/core", "https://core.drpc.org"]`,
		// 	"block_explorer_urls":      `["https://scan.coredao.org"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "hemi-mainnet",
		// 	"display_name":             "Hemi",
		// 	"chain_id":                 43111,
		// 	"native_currency_name":     "Ether",
		// 	"native_currency_symbol":   "ETH",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/hemi-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.hemi.network/rpc",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.hemi.network/rpc", "https://hemi.drpc.org"]`,
		// 	"block_explorer_urls":      `["https://explorer.hemi.xyz"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "goat-mainnet",
		// 	"display_name":             "GOAT",
		// 	"chain_id":                 2345,
		// 	"native_currency_name":     "BTC",
		// 	"native_currency_symbol":   "BTC",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/goat-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.ankr.com/goat_mainnet",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.ankr.com/goat_mainnet", "https://rpc.goat.network"]`,
		// 	"block_explorer_urls":      `["https://explorer.goat.network"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "b2-mainnet",
		// 	"display_name":             "B2",
		// 	"chain_id":                 223,
		// 	"native_currency_name":     "BTC",
		// 	"native_currency_symbol":   "BTC",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/b2-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.ankr.com/b2",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.bsquared.network", "https://rpc.ankr.com/b2"]`,
		// 	"block_explorer_urls":      `["https://explorer.bsquared.network"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "ailayer-mainnet",
		// 	"display_name":             "AILayer",
		// 	"chain_id":                 2649,
		// 	"native_currency_name":     "BTC",
		// 	"native_currency_symbol":   "BTC",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/ailayer-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://mainnet-rpc.ailayer.xyz",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://mainnet-rpc.ailayer.xyz"]`,
		// 	"block_explorer_urls":      `["https://mainnet-explorer.ailayer.xyz"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "zklink-mainnet",
		// 	"display_name":             "zkLink",
		// 	"chain_id":                 810180,
		// 	"native_currency_name":     "Ether",
		// 	"native_currency_symbol":   "ETH",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/zklink-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.zklink.io",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.zklink.io"]`,
		// 	"block_explorer_urls":      `["https://explorer.zklink.io"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "merlin-mainnet",
		// 	"display_name":             "Merlin",
		// 	"chain_id":                 4200,
		// 	"native_currency_name":     "BTC",
		// 	"native_currency_symbol":   "BTC",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/merlin-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc.merlinchain.io",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://rpc.merlinchain.io", "https://merlin.drpc.org"]`,
		// 	"block_explorer_urls":      `["https://scan.merlinchain.io"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "exsat-mainnet",
		// 	"display_name":             "exSat",
		// 	"chain_id":                 7200,
		// 	"native_currency_name":     "BTC",
		// 	"native_currency_symbol":   "BTC",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/exsat-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://rpc-sg.exsat.network",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://evm.exsat.network", "https://rpc-sg.exsat.network", "https://rpc-us.exsat.network"]`,
		// 	"block_explorer_urls":      `["https://scan.exsat.network"]`,
		// 	"rpc_enabled":              true,
		// },
		// {
		// 	"chain_name":               "hashkey-mainnet",
		// 	"display_name":             "HashKey",
		// 	"chain_id":                 177,
		// 	"native_currency_name":     "HSK",
		// 	"native_currency_symbol":   "HSK",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/hashkey-mainnet.jpg",
		// 	"is_testnet":               false,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://mainnet.hsk.xyz",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://mainnet.hsk.xyz", "https://hashkey.drpc.org"]`,
		// 	"block_explorer_urls":      `["https://hashkey.blockscout.com"]`,
		// 	"rpc_enabled":              true,
		// },
	}

	// 测试网数据
	testnets := []map[string]interface{}{
		{
			"chain_name":               "eth-sepolia",
			"display_name":             "Sepolia",
			"chain_id":                 11155111,
			"native_currency_name":     "Sepolia Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/eth-sepolia.png",
			"is_testnet":               true,
			"is_active":                true,
			"alchemy_rpc_template":     "https://sepolia.infura.io/v3/",
			"infura_rpc_template":      "",
			"official_rpc_urls":        `["https://ethereum-sepolia-rpc.publicnode.com","https://1rpc.io/sepolia"]`,
			"block_explorer_urls":      `["https://sepolia.etherscan.io"]`,
			"rpc_enabled":              true,
			"subgraph_url":             "https://api.goldsky.com/api/public/project_cmisnqs37gdlo01y7f60oa25h/subgraphs/timelock-eth-sepolia/1.0.0/gn",
		},
		// {
		// 	"chain_name":               "bsc-testnet",
		// 	"display_name":             "BNB Testnet",
		// 	"chain_id":                 97,
		// 	"native_currency_name":     "Test BNB",
		// 	"native_currency_symbol":   "BNB",
		// 	"native_currency_decimals": 18,
		// 	"logo_url":                 "https://raw.githubusercontent.com/timelock-labs/assets/main/chains/bsc-testnet.png",
		// 	"is_testnet":               true,
		// 	"is_active":                true,
		// 	"alchemy_rpc_template":     "https://api.zan.top/bsc-testnet",
		// 	"infura_rpc_template":      "",
		// 	"official_rpc_urls":        `["https://bsc-testnet-rpc.publicnode.com","https://bsc-testnet.drpc.org"]`,
		// 	"block_explorer_urls":      `["https://testnet.bscscan.com"]`,
		// 	"rpc_enabled":              true,
		// },
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
			"name":        "Compound Timelock",
			"abi_content": `[{"inputs":[{"internalType":"address","name":"admin_","type":"address"},{"internalType":"uint256","name":"delay_","type":"uint256"}],"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"txHash","type":"bytes32"},{"indexed":true,"internalType":"address","name":"target","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"},{"indexed":false,"internalType":"string","name":"signature","type":"string"},{"indexed":false,"internalType":"bytes","name":"data","type":"bytes"},{"indexed":false,"internalType":"uint256","name":"eta","type":"uint256"}],"name":"CancelTransaction","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"txHash","type":"bytes32"},{"indexed":true,"internalType":"address","name":"target","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"},{"indexed":false,"internalType":"string","name":"signature","type":"string"},{"indexed":false,"internalType":"bytes","name":"data","type":"bytes"},{"indexed":false,"internalType":"uint256","name":"eta","type":"uint256"}],"name":"ExecuteTransaction","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"newAdmin","type":"address"}],"name":"NewAdmin","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint256","name":"newDelay","type":"uint256"}],"name":"NewDelay","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"newPendingAdmin","type":"address"}],"name":"NewPendingAdmin","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes32","name":"txHash","type":"bytes32"},{"indexed":true,"internalType":"address","name":"target","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"},{"indexed":false,"internalType":"string","name":"signature","type":"string"},{"indexed":false,"internalType":"bytes","name":"data","type":"bytes"},{"indexed":false,"internalType":"uint256","name":"eta","type":"uint256"}],"name":"QueueTransaction","type":"event"},{"stateMutability":"payable","type":"fallback"},{"inputs":[],"name":"GRACE_PERIOD","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"MAXIMUM_DELAY","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"MINIMUM_DELAY","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"acceptAdmin","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"admin","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"target","type":"address"},{"internalType":"uint256","name":"value","type":"uint256"},{"internalType":"string","name":"signature","type":"string"},{"internalType":"bytes","name":"data","type":"bytes"},{"internalType":"uint256","name":"eta","type":"uint256"}],"name":"cancelTransaction","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"delay","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"target","type":"address"},{"internalType":"uint256","name":"value","type":"uint256"},{"internalType":"string","name":"signature","type":"string"},{"internalType":"bytes","name":"data","type":"bytes"},{"internalType":"uint256","name":"eta","type":"uint256"}],"name":"executeTransaction","outputs":[{"internalType":"bytes","name":"","type":"bytes"}],"stateMutability":"payable","type":"function"},{"inputs":[],"name":"pendingAdmin","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"target","type":"address"},{"internalType":"uint256","name":"value","type":"uint256"},{"internalType":"string","name":"signature","type":"string"},{"internalType":"bytes","name":"data","type":"bytes"},{"internalType":"uint256","name":"eta","type":"uint256"}],"name":"queueTransaction","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"name":"queuedTransactions","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"delay_","type":"uint256"}],"name":"setDelay","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"pendingAdmin_","type":"address"}],"name":"setPendingAdmin","outputs":[],"stateMutability":"nonpayable","type":"function"}]`,
			"owner":       "0x0000000000000000000000000000000000000000",
			"description": "Compound Timelock contract interface for managing timelock transactions.",
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
