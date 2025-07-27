package migrations

import (
	"context"
	"fmt"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

// InitTables 初始化数据库表和数据
func InitTables(db *gorm.DB) error {
	ctx := context.Background()
	logger.Info("Starting database initialization...")

	// 第一步：逆序删除表（按依赖关系）
	if err := dropTables(db, ctx); err != nil {
		logger.Error("Failed to drop tables: ", err)
		return fmt.Errorf("failed to drop tables: %w", err)
	}

	// 第二步：创建表结构
	if err := createTables(db, ctx); err != nil {
		logger.Error("Failed to create tables: ", err)
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// 第三步：创建索引
	if err := createIndexes(db, ctx); err != nil {
		logger.Error("Failed to create indexes: ", err)
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	// 第四步：插入初始数据
	if err := insertInitialData(db, ctx); err != nil {
		logger.Error("Failed to insert initial data: ", err)
		return fmt.Errorf("failed to insert initial data: %w", err)
	}

	logger.Info("Database initialization completed successfully")
	return nil
}

// dropTables 逆序删除表
func dropTables(db *gorm.DB, ctx context.Context) error {
	logger.Info("Dropping existing tables...")

	tables := []string{
		"emergency_notifications",
		"email_send_logs",
		"email_notifications",
		"transactions",
		"compound_timelocks",
		"openzeppelin_timelocks",
		"user_assets",
		"abis",
		"support_chains",
		"users",
	}

	for _, table := range tables {
		if err := db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)).Error; err != nil {
			logger.Error("Failed to drop table", err, "table", table)
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	logger.Info("Dropped tables successfully")
	return nil
}

// createTables 创建表结构
func createTables(db *gorm.DB, ctx context.Context) error {
	logger.Info("Creating database tables...")

	// 1. 用户表
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

	if err := db.WithContext(ctx).Exec(createUsersTable).Error; err != nil {
		logger.Error("Failed to create users table: ", err)
		return fmt.Errorf("failed to create users table: %w", err)
	}
	logger.Info("Created table: users")

	// 2. 支持的区块链表（重构版）
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

	if err := db.WithContext(ctx).Exec(createSupportChainsTable).Error; err != nil {
		logger.Error("Failed to create support_chains table: ", err)
		return fmt.Errorf("failed to create support_chains table: %w", err)
	}
	logger.Info("Created table: support_chains")

	// 3. 用户资产表
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

	if err := db.WithContext(ctx).Exec(createUserAssetsTable).Error; err != nil {
		logger.Error("Failed to create user_assets table: ", err)
		return fmt.Errorf("failed to create user_assets table: %w", err)
	}
	logger.Info("Created table: user_assets")

	// 4. ABI库表
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

	if err := db.WithContext(ctx).Exec(createABIsTable).Error; err != nil {
		logger.Error("Failed to create abis table: ", err)
		return fmt.Errorf("failed to create abis table: %w", err)
	}
	logger.Info("Created table: abis")

	// 5. Compound标准Timelock合约表
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
		emergency_mode BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(chain_id, contract_address)
	)`

	if err := db.WithContext(ctx).Exec(createCompoundTimelocksTable).Error; err != nil {
		logger.Error("Failed to create compound_timelocks table: ", err)
		return fmt.Errorf("failed to create compound_timelocks table: %w", err)
	}
	logger.Info("Created table: compound_timelocks")

	// 6. OpenZeppelin标准Timelock合约表
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
		emergency_mode BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(chain_id, contract_address)
	)`

	if err := db.WithContext(ctx).Exec(createOpenzeppelinTimelocksTable).Error; err != nil {
		logger.Error("Failed to create openzeppelin_timelocks table: ", err)
		return fmt.Errorf("failed to create openzeppelin_timelocks table: %w", err)
	}
	logger.Info("Created table: openzeppelin_timelocks")

	// 7. 交易记录表
	createTransactionsTable := `
	CREATE TABLE transactions (
		id BIGSERIAL PRIMARY KEY,
		creator_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE,
		chain_id INTEGER NOT NULL,
		chain_name VARCHAR(50) NOT NULL,
		timelock_address VARCHAR(42) NOT NULL,
		timelock_standard VARCHAR(20) NOT NULL CHECK (timelock_standard IN ('compound', 'openzeppelin')),
		tx_hash VARCHAR(66) NOT NULL UNIQUE,
		tx_data TEXT NOT NULL,
		target VARCHAR(42) NOT NULL,
		value VARCHAR(100) NOT NULL DEFAULT '0',
		function_sig VARCHAR(200),
		eta BIGINT NOT NULL,
		queued_at TIMESTAMP WITH TIME ZONE,
		executed_at TIMESTAMP WITH TIME ZONE,
		canceled_at TIMESTAMP WITH TIME ZONE,
		status VARCHAR(20) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'ready', 'executed', 'expired', 'canceled')),
		description VARCHAR(500) DEFAULT '',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`

	if err := db.WithContext(ctx).Exec(createTransactionsTable).Error; err != nil {
		logger.Error("Failed to create transactions table: ", err)
		return fmt.Errorf("failed to create transactions table: %w", err)
	}
	logger.Info("Created table: transactions")

	// 8. 邮件通知配置表
	createEmailNotificationsTable := `
	CREATE TABLE email_notifications (
		id BIGSERIAL PRIMARY KEY,
		wallet_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE,
		email VARCHAR(255) NOT NULL,
		email_remark VARCHAR(200) DEFAULT '',
		timelock_contracts TEXT NOT NULL DEFAULT '[]',
		is_verified BOOLEAN NOT NULL DEFAULT false,
		verification_code VARCHAR(6),
		verification_expires_at TIMESTAMP WITH TIME ZONE,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(wallet_address, email)
	)`

	if err := db.WithContext(ctx).Exec(createEmailNotificationsTable).Error; err != nil {
		logger.Error("Failed to create email_notifications table: ", err)
		return fmt.Errorf("failed to create email_notifications table: %w", err)
	}
	logger.Info("Created table: email_notifications")

	// 9. 邮件发送记录表
	createEmailSendLogsTable := `
	CREATE TABLE email_send_logs (
		id BIGSERIAL PRIMARY KEY,
		email_notification_id BIGINT NOT NULL REFERENCES email_notifications(id) ON DELETE CASCADE,
		email VARCHAR(255) NOT NULL,
		timelock_address VARCHAR(42) NOT NULL,
		transaction_hash VARCHAR(66),
		event_type VARCHAR(50) NOT NULL,
		subject VARCHAR(500) NOT NULL,
		content TEXT NOT NULL,
		is_emergency BOOLEAN NOT NULL DEFAULT false,
		emergency_reply_token VARCHAR(64),
		is_replied BOOLEAN NOT NULL DEFAULT false,
		replied_at TIMESTAMP WITH TIME ZONE,
		send_status VARCHAR(20) NOT NULL DEFAULT 'pending',
		send_attempts INTEGER NOT NULL DEFAULT 0,
		error_message TEXT,
		sent_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`

	if err := db.WithContext(ctx).Exec(createEmailSendLogsTable).Error; err != nil {
		logger.Error("Failed to create email_send_logs table: ", err)
		return fmt.Errorf("failed to create email_send_logs table: %w", err)
	}
	logger.Info("Created table: email_send_logs")

	// 10. 应急通知追踪表
	createEmergencyNotificationsTable := `
	CREATE TABLE emergency_notifications (
		id BIGSERIAL PRIMARY KEY,
		timelock_address VARCHAR(42) NOT NULL,
		transaction_hash VARCHAR(66) NOT NULL,
		event_type VARCHAR(50) NOT NULL,
		replied_emails INTEGER NOT NULL DEFAULT 0,
		is_completed BOOLEAN NOT NULL DEFAULT false,
		next_send_at TIMESTAMP WITH TIME ZONE,
		send_count INTEGER NOT NULL DEFAULT 1,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(timelock_address, transaction_hash, event_type)
	)`

	if err := db.WithContext(ctx).Exec(createEmergencyNotificationsTable).Error; err != nil {
		logger.Error("Failed to create emergency_notifications table: ", err)
		return fmt.Errorf("failed to create emergency_notifications table: %w", err)
	}
	logger.Info("Created table: emergency_notifications")

	return nil
}

// createIndexes 创建索引
func createIndexes(db *gorm.DB, ctx context.Context) error {
	logger.Info("Creating database indexes...")

	indexes := []string{
		// 用户表索引
		"CREATE INDEX idx_users_wallet_address ON users(wallet_address)",
		"CREATE INDEX idx_users_chain_id ON users(chain_id)",

		// 支持链表索引
		"CREATE INDEX idx_support_chains_chain_name ON support_chains(chain_name)",
		"CREATE INDEX idx_support_chains_chain_id ON support_chains(chain_id)",
		"CREATE INDEX idx_support_chains_is_active ON support_chains(is_active)",
		"CREATE INDEX idx_support_chains_is_testnet ON support_chains(is_testnet)",

		// 用户资产表索引
		"CREATE INDEX idx_user_assets_wallet_address ON user_assets(wallet_address)",
		"CREATE INDEX idx_user_assets_chain_name ON user_assets(chain_name)",
		"CREATE INDEX idx_user_assets_usd_value ON user_assets(usd_value DESC)",

		// ABI表索引
		"CREATE INDEX idx_abis_owner ON abis(owner)",
		"CREATE INDEX idx_abis_name ON abis(name)",
		"CREATE INDEX idx_abis_is_shared ON abis(is_shared)",
		"CREATE INDEX idx_abis_created_at ON abis(created_at DESC)",

		// Compound timelock索引
		"CREATE INDEX idx_compound_timelocks_creator_address ON compound_timelocks(creator_address)",
		"CREATE INDEX idx_compound_timelocks_chain_id ON compound_timelocks(chain_id)",
		"CREATE INDEX idx_compound_timelocks_chain_name ON compound_timelocks(chain_name)",
		"CREATE INDEX idx_compound_timelocks_contract_address ON compound_timelocks(contract_address)",
		"CREATE INDEX idx_compound_timelocks_admin ON compound_timelocks(admin)",
		"CREATE INDEX idx_compound_timelocks_pending_admin ON compound_timelocks(pending_admin)",
		"CREATE INDEX idx_compound_timelocks_status ON compound_timelocks(status)",
		"CREATE INDEX idx_compound_timelocks_emergency_mode ON compound_timelocks(emergency_mode)",

		// OpenZeppelin timelock索引
		"CREATE INDEX idx_openzeppelin_timelocks_creator_address ON openzeppelin_timelocks(creator_address)",
		"CREATE INDEX idx_openzeppelin_timelocks_chain_id ON openzeppelin_timelocks(chain_id)",
		"CREATE INDEX idx_openzeppelin_timelocks_chain_name ON openzeppelin_timelocks(chain_name)",
		"CREATE INDEX idx_openzeppelin_timelocks_contract_address ON openzeppelin_timelocks(contract_address)",
		"CREATE INDEX idx_openzeppelin_timelocks_status ON openzeppelin_timelocks(status)",
		"CREATE INDEX idx_openzeppelin_timelocks_emergency_mode ON openzeppelin_timelocks(emergency_mode)",

		// 交易记录表索引
		"CREATE INDEX idx_transactions_creator_address ON transactions(creator_address)",
		"CREATE INDEX idx_transactions_chain_id ON transactions(chain_id)",
		"CREATE INDEX idx_transactions_timelock_address ON transactions(timelock_address)",
		"CREATE INDEX idx_transactions_timelock_standard ON transactions(timelock_standard)",
		"CREATE INDEX idx_transactions_tx_hash ON transactions(tx_hash)",
		"CREATE INDEX idx_transactions_status ON transactions(status)",
		"CREATE INDEX idx_transactions_eta ON transactions(eta)",
		"CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC)",
		"CREATE INDEX idx_transactions_updated_at ON transactions(updated_at DESC)",

		// 邮件通知配置表索引
		"CREATE INDEX idx_email_notifications_wallet_address ON email_notifications(wallet_address)",
		"CREATE INDEX idx_email_notifications_email ON email_notifications(email)",
		"CREATE INDEX idx_email_notifications_is_verified ON email_notifications(is_verified)",
		"CREATE INDEX idx_email_notifications_is_active ON email_notifications(is_active)",

		// 邮件发送记录表索引
		"CREATE INDEX idx_email_send_logs_email_notification_id ON email_send_logs(email_notification_id)",
		"CREATE INDEX idx_email_send_logs_timelock_address ON email_send_logs(timelock_address)",
		"CREATE INDEX idx_email_send_logs_transaction_hash ON email_send_logs(transaction_hash)",
		"CREATE INDEX idx_email_send_logs_event_type ON email_send_logs(event_type)",
		"CREATE INDEX idx_email_send_logs_is_emergency ON email_send_logs(is_emergency)",
		"CREATE INDEX idx_email_send_logs_is_replied ON email_send_logs(is_replied)",
		"CREATE INDEX idx_email_send_logs_send_status ON email_send_logs(send_status)",
		"CREATE INDEX idx_email_send_logs_sent_at ON email_send_logs(sent_at DESC)",

		// 应急通知追踪表索引
		"CREATE INDEX idx_emergency_notifications_timelock_address ON emergency_notifications(timelock_address)",
		"CREATE INDEX idx_emergency_notifications_transaction_hash ON emergency_notifications(transaction_hash)",
		"CREATE INDEX idx_emergency_notifications_is_completed ON emergency_notifications(is_completed)",
		"CREATE INDEX idx_emergency_notifications_next_send_at ON emergency_notifications(next_send_at)",
	}

	for _, indexSQL := range indexes {
		if err := db.WithContext(ctx).Exec(indexSQL).Error; err != nil {
			logger.Error("Failed to create index", err, "sql", indexSQL)
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	logger.Info("Created all indexes successfully")
	return nil
}

// insertInitialData 插入初始数据
func insertInitialData(db *gorm.DB, ctx context.Context) error {
	logger.Info("Inserting initial data...")

	// 插入支持的链数据
	if err := insertSupportedChains(db, ctx); err != nil {
		logger.Error("Failed to insert supported chains: ", err)
		return fmt.Errorf("failed to insert supported chains: %w", err)
	}

	// 插入共享ABI数据
	if err := insertSharedABIs(db, ctx); err != nil {
		logger.Error("Failed to insert shared ABIs: ", err)
		return fmt.Errorf("failed to insert shared ABIs: %w", err)
	}

	return nil
}

// insertSupportedChains 插入支持的链数据
func insertSupportedChains(db *gorm.DB, ctx context.Context) error {
	logger.Info("Inserting supported chains data...")

	// 主网数据
	mainnets := []map[string]interface{}{
		{
			"chain_name":               "eth-mainnet",
			"display_name":             "Ethereum Mainnet",
			"chain_id":                 1,
			"native_currency_name":     "Ether",
			"native_currency_symbol":   "ETH",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/ethereum/info/logo.png",
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
			"logo_url":                 "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/smartchain/info/logo.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://bnb-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://bsc-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://bsc-dataseed.binance.org","https://rpc.ankr.com/bsc"]`,
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
			"logo_url":                 "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/polygon/info/logo.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://polygon-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://polygon-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://polygon-rpc.com","https://rpc.ankr.com/polygon"]`,
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
			"logo_url":                 "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/arbitrum/info/logo.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://arb-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://arbitrum-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://arb1.arbitrum.io/rpc","https://rpc.ankr.com/arbitrum"]`,
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
			"logo_url":                 "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/optimism/info/logo.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://opt-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://optimism-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://mainnet.optimism.io","https://rpc.ankr.com/optimism"]`,
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
			"logo_url":                 "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/base/info/logo.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://base-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://base-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://mainnet.base.org","https://rpc.ankr.com/base"]`,
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
			"logo_url":                 "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/avalanchec/info/logo.png",
			"is_testnet":               false,
			"is_active":                true,
			"alchemy_rpc_template":     "https://avax-mainnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://avalanche-mainnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://api.avax.network/ext/bc/C/rpc","https://rpc.ankr.com/avalanche"]`,
			"block_explorer_urls":      `["https://snowtrace.io"]`,
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
			"logo_url":                 "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/ethereum/info/logo.png",
			"is_testnet":               true,
			"is_active":                true,
			"alchemy_rpc_template":     "https://eth-sepolia.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://sepolia.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://ethereum-sepolia.publicnode.com","https://rpc.ankr.com/eth_sepolia"]`,
			"block_explorer_urls":      `["https://sepolia.etherscan.io"]`,
			"rpc_enabled":              true,
		},
		{
			"chain_name":               "bnb-testnet",
			"display_name":             "BNB Smart Chain Testnet",
			"chain_id":                 97,
			"native_currency_name":     "Test BNB",
			"native_currency_symbol":   "BNB",
			"native_currency_decimals": 18,
			"logo_url":                 "https://raw.githubusercontent.com/trustwallet/assets/master/blockchains/smartchain/info/logo.png",
			"is_testnet":               true,
			"is_active":                true,
			"alchemy_rpc_template":     "https://bnb-testnet.g.alchemy.com/v2/{API_KEY}",
			"infura_rpc_template":      "https://bsc-testnet.infura.io/v3/{API_KEY}",
			"official_rpc_urls":        `["https://data-seed-prebsc-1-s1.binance.org:8545","https://rpc.ankr.com/bsc_testnet_chapel"]`,
			"block_explorer_urls":      `["https://testnet.bscscan.com"]`,
			"rpc_enabled":              true,
		},
	}

	// 插入主网数据
	for _, chain := range mainnets {
		if err := db.WithContext(ctx).Table("support_chains").Create(chain).Error; err != nil {
			logger.Error("Failed to insert mainnet chain", err, "chain_name", chain["chain_name"])
			return fmt.Errorf("failed to insert mainnet chain %s: %w", chain["chain_name"], err)
		}
	}

	// 插入测试网数据
	for _, chain := range testnets {
		if err := db.WithContext(ctx).Table("support_chains").Create(chain).Error; err != nil {
			logger.Error("Failed to insert testnet chain", err, "chain_name", chain["chain_name"])
			return fmt.Errorf("failed to insert testnet chain %s: %w", chain["chain_name"], err)
		}
	}

	logger.Info("Inserted all supported chains successfully")
	return nil
}

// insertSharedABIs 插入共享ABI数据
func insertSharedABIs(db *gorm.DB, ctx context.Context) error {
	logger.Info("Inserting shared ABIs data...")

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
		if err := db.WithContext(ctx).Table("abis").Create(abi).Error; err != nil {
			logger.Error("Failed to insert shared ABI", err, "name", abi["name"])
			return fmt.Errorf("failed to insert shared ABI %s: %w", abi["name"], err)
		}
	}

	logger.Info("Inserted all shared ABIs successfully")
	return nil
}
