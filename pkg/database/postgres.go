package database

import (
	"errors"
	"fmt"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/internal/types"
	tl_logger "timelocker-backend/pkg/logger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewPostgresConnection 创建PostgreSQL数据库连接
func NewPostgresConnection(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		tl_logger.Error("NewPostgresConnection Error: ", errors.New("failed to connect to database"), "error: ", err)
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 获取底层的sql.DB对象进行连接池配置
	sqlDB, err := db.DB()
	if err != nil {
		tl_logger.Error("NewPostgresConnection Error: ", errors.New("failed to get underlying sql.DB"), "error: ", err)
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	tl_logger.Info("NewPostgresConnection: ", "host: ", cfg.Host, "port: ", cfg.Port, "user: ", cfg.User, "dbname: ", cfg.DBName, "sslmode: ", cfg.SSLMode)
	return db, nil
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate(db *gorm.DB) error {
	// 先尝试修复可能存在的约束冲突
	if err := fixConstraintConflicts(db); err != nil {
		tl_logger.Error("AutoMigrate Warning: ", errors.New("failed to fix constraint conflicts"), "error: ", err)
		// 不返回错误，继续尝试迁移
	}

	// 自动迁移所有表（已禁用外键约束，可以一次性迁移）
	err := db.AutoMigrate(
		&types.User{},
		&types.SupportToken{},
		&types.SupportChain{},
		&types.ChainToken{},
		&types.UserAsset{},
	)
	if err != nil {
		tl_logger.Error("AutoMigrate Error: ", errors.New("failed to migrate database"), "error: ", err)
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	tl_logger.Info("AutoMigrate: ", "database migration completed successfully")
	return nil
}

// fixConstraintConflicts 修复约束冲突
func fixConstraintConflicts(db *gorm.DB) error {
	// 彻底清理user_assets表的所有约束和索引
	if err := dropAllUserAssetsConstraints(db); err != nil {
		tl_logger.Error("fixConstraintConflicts Warning: ", errors.New("failed to drop user_assets constraints"), "error: ", err)
	}

	// 增强的约束清理函数，支持模糊匹配
	cleanupConstraintsAdvanced := func(tableName string, constraintPatterns []string) {
		// 首先查询所有现有约束
		var constraints []string
		query := `SELECT constraint_name FROM information_schema.table_constraints 
				  WHERE table_name = ? AND table_schema = 'public'`

		if err := db.Raw(query, tableName).Pluck("constraint_name", &constraints).Error; err != nil {
			tl_logger.Error("fixConstraintConflicts Warning: ", errors.New("failed to query constraints"), "error: ", err, "table: ", tableName)
			return
		}

		// 删除匹配的约束
		for _, constraint := range constraints {
			for _, pattern := range constraintPatterns {
				if constraint == pattern {
					dropQuery := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", tableName, constraint)
					if err := db.Exec(dropQuery).Error; err != nil {
						tl_logger.Error("fixConstraintConflicts Warning: ", errors.New("failed to drop constraint"), "error: ", err, "table: ", tableName, "constraint: ", constraint)
					} else {
						tl_logger.Info("fixConstraintConflicts: ", "dropped constraint", "table: ", tableName, "constraint: ", constraint)
					}
					break
				}
			}
		}
	}

	// 删除所有现有的索引，避免约束冲突
	dropIndexes := func(tableName string, indexPatterns []string) {
		for _, indexName := range indexPatterns {
			dropQuery := fmt.Sprintf("DROP INDEX IF EXISTS %s", indexName)
			if err := db.Exec(dropQuery).Error; err != nil {
				tl_logger.Error("fixConstraintConflicts Warning: ", errors.New("failed to drop index"), "error: ", err, "table: ", tableName, "index: ", indexName)
			} else {
				tl_logger.Info("fixConstraintConflicts: ", "dropped index", "table: ", tableName, "index: ", indexName)
			}
		}
	}

	// 清理各表的约束和索引
	cleanupConstraintsAdvanced("users", []string{
		"uni_users_wallet_address",
		"idx_users_wallet_address",
		"users_wallet_address_key",
	})

	cleanupConstraintsAdvanced("support_tokens", []string{
		"uni_support_tokens_symbol",
		"uni_support_tokens_coingecko_id",
		"idx_support_tokens_symbol",
		"idx_support_tokens_coingecko_id",
		"support_tokens_symbol_key",
		"support_tokens_coingecko_id_key",
	})

	cleanupConstraintsAdvanced("support_chains", []string{
		"uni_support_chains_chain_id",
		"idx_support_chains_chain_id",
		"support_chains_chain_id_key",
	})

	cleanupConstraintsAdvanced("chain_tokens", []string{
		"uni_chain_tokens_chain_id_token_id",
		"idx_chain_tokens_unique",
		"chain_tokens_chain_id_token_id_key",
	})

	cleanupConstraintsAdvanced("user_assets", []string{
		"uni_user_assets_user_id_chain_id_token_id",
		"idx_user_assets_unique",
		"user_assets_user_id_chain_id_token_id_key",
	})

	// 删除可能冲突的索引
	dropIndexes("users", []string{
		"idx_users_wallet_address",
		"uni_users_wallet_address",
	})

	dropIndexes("support_tokens", []string{
		"idx_support_tokens_symbol",
		"idx_support_tokens_coingecko_id",
		"uni_support_tokens_symbol",
		"uni_support_tokens_coingecko_id",
	})

	dropIndexes("support_chains", []string{
		"idx_support_chains_chain_id",
		"uni_support_chains_chain_id",
	})

	dropIndexes("chain_tokens", []string{
		"idx_chain_tokens_unique",
		"uni_chain_tokens_chain_id_token_id",
	})

	dropIndexes("user_assets", []string{
		"idx_user_assets_unique",
		"uni_user_assets_user_id_chain_id_token_id",
	})

	return nil
}

// dropAllUserAssetsConstraints 彻底删除user_assets表的所有约束和索引
func dropAllUserAssetsConstraints(db *gorm.DB) error {
	// 查询user_assets表的所有约束
	var constraints []string
	constraintsQuery := `
		SELECT constraint_name 
		FROM information_schema.table_constraints 
		WHERE table_name = 'user_assets' AND table_schema = 'public'
		AND constraint_type IN ('UNIQUE', 'CHECK')`

	if err := db.Raw(constraintsQuery).Pluck("constraint_name", &constraints).Error; err != nil {
		tl_logger.Error("dropAllUserAssetsConstraints Warning: ", errors.New("failed to query constraints"), "error: ", err)
	}

	// 删除所有约束
	for _, constraint := range constraints {
		dropQuery := fmt.Sprintf("ALTER TABLE user_assets DROP CONSTRAINT IF EXISTS %s", constraint)
		if err := db.Exec(dropQuery).Error; err != nil {
			tl_logger.Error("dropAllUserAssetsConstraints Warning: ", errors.New("failed to drop constraint"), "constraint: ", constraint, "error: ", err)
		} else {
			tl_logger.Info("dropAllUserAssetsConstraints: ", "dropped constraint", "constraint: ", constraint)
		}
	}

	// 查询user_assets表的所有索引（除了主键）
	var indexes []string
	indexesQuery := `
		SELECT indexname 
		FROM pg_indexes 
		WHERE tablename = 'user_assets' AND schemaname = 'public'
		AND indexname NOT LIKE '%_pkey'`

	if err := db.Raw(indexesQuery).Pluck("indexname", &indexes).Error; err != nil {
		tl_logger.Error("dropAllUserAssetsConstraints Warning: ", errors.New("failed to query indexes"), "error: ", err)
	}

	// 删除所有索引（除了主键）
	for _, index := range indexes {
		dropQuery := fmt.Sprintf("DROP INDEX IF EXISTS %s", index)
		if err := db.Exec(dropQuery).Error; err != nil {
			tl_logger.Error("dropAllUserAssetsConstraints Warning: ", errors.New("failed to drop index"), "index: ", index, "error: ", err)
		} else {
			tl_logger.Info("dropAllUserAssetsConstraints: ", "dropped index", "index: ", index)
		}
	}

	return nil
}

// CreateIndexes 创建额外的数据库索引
func CreateIndexes(db *gorm.DB) error {
	// 为用户表创建索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_wallet_address ON users(wallet_address)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create wallet_address index"), "error: ", err)
		return fmt.Errorf("failed to create wallet_address index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_chain_id ON users(chain_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create chain_id index"), "error: ", err)
		return fmt.Errorf("failed to create chain_id index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create created_at index"), "error: ", err)
		return fmt.Errorf("failed to create created_at index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_status ON users(status)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create status index"), "error: ", err)
		return fmt.Errorf("failed to create status index: %w", err)
	}

	// 为支持代币表创建索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_support_tokens_symbol ON support_tokens(symbol)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create support_tokens symbol index"), "error: ", err)
		return fmt.Errorf("failed to create support_tokens symbol index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_support_tokens_coingecko_id ON support_tokens(coingecko_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create support_tokens coingecko_id index"), "error: ", err)
		return fmt.Errorf("failed to create support_tokens coingecko_id index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_support_tokens_is_active ON support_tokens(is_active)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create support_tokens is_active index"), "error: ", err)
		return fmt.Errorf("failed to create support_tokens is_active index: %w", err)
	}

	// 为支持链表创建索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_support_chains_chain_id ON support_chains(chain_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create support_chains chain_id index"), "error: ", err)
		return fmt.Errorf("failed to create support_chains chain_id index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_support_chains_is_active ON support_chains(is_active)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create support_chains is_active index"), "error: ", err)
		return fmt.Errorf("failed to create support_chains is_active index: %w", err)
	}

	// 为链代币关联表创建索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_chain_tokens_chain_id ON chain_tokens(chain_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create chain_tokens chain_id index"), "error: ", err)
		return fmt.Errorf("failed to create chain_tokens chain_id index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_chain_tokens_token_id ON chain_tokens(token_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create chain_tokens token_id index"), "error: ", err)
		return fmt.Errorf("failed to create chain_tokens token_id index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_chain_tokens_contract_address ON chain_tokens(contract_address)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create chain_tokens contract_address index"), "error: ", err)
		return fmt.Errorf("failed to create chain_tokens contract_address index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_chain_tokens_is_active ON chain_tokens(is_active)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create chain_tokens is_active index"), "error: ", err)
		return fmt.Errorf("failed to create chain_tokens is_active index: %w", err)
	}

	// 为用户资产表创建索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_assets_user_id ON user_assets(user_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create user_assets user_id index"), "error: ", err)
		return fmt.Errorf("failed to create user_assets user_id index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_assets_wallet_address ON user_assets(wallet_address)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create user_assets wallet_address index"), "error: ", err)
		return fmt.Errorf("failed to create user_assets wallet_address index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_assets_chain_id ON user_assets(chain_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create user_assets chain_id index"), "error: ", err)
		return fmt.Errorf("failed to create user_assets chain_id index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_assets_token_id ON user_assets(token_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create user_assets token_id index"), "error: ", err)
		return fmt.Errorf("failed to create user_assets token_id index: %w", err)
	}

	// 创建复合索引但不是唯一索引，避免重复数据冲突
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_assets_composite ON user_assets(user_id, chain_id, token_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create user_assets composite index"), "error: ", err)
		return fmt.Errorf("failed to create user_assets composite index: %w", err)
	}

	// 创建钱包地址复合索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_user_assets_wallet_composite ON user_assets(wallet_address, chain_id, token_id)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create user_assets wallet composite index"), "error: ", err)
		return fmt.Errorf("failed to create user_assets wallet composite index: %w", err)
	}

	tl_logger.Info("CreateIndexes: ", "database indexes created successfully")
	return nil
}

// CheckTablesExist 检查必要的表是否存在
func CheckTablesExist(db *gorm.DB) error {
	requiredTables := []string{
		"users",
		"support_tokens",
		"support_chains",
		"chain_tokens",
		"user_assets",
	}

	for _, tableName := range requiredTables {
		var exists bool
		query := `SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = ?
		)`

		if err := db.Raw(query, tableName).Scan(&exists).Error; err != nil {
			tl_logger.Error("CheckTablesExist Error: ", errors.New("failed to check table existence"), "table: ", tableName, "error: ", err)
			return fmt.Errorf("failed to check table %s existence: %w", tableName, err)
		}

		if !exists {
			tl_logger.Error("CheckTablesExist Error: ", errors.New("required table does not exist"), "table: ", tableName)
			return fmt.Errorf("required table %s does not exist", tableName)
		}
	}

	tl_logger.Info("CheckTablesExist: ", "all required tables exist")
	return nil
}

// InitializePredefinedData 初始化预定义数据
func InitializePredefinedData(db *gorm.DB) error {
	tl_logger.Info("InitializePredefinedData: ", "starting initialization of predefined data")

	// 1. 初始化支持的区块链
	if err := initializeSupportChains(db); err != nil {
		tl_logger.Error("InitializePredefinedData Error: ", errors.New("failed to initialize support chains"), "error: ", err)
		return fmt.Errorf("failed to initialize support chains: %w", err)
	}

	// 2. 初始化支持的代币
	if err := initializeSupportTokens(db); err != nil {
		tl_logger.Error("InitializePredefinedData Error: ", errors.New("failed to initialize support tokens"), "error: ", err)
		return fmt.Errorf("failed to initialize support tokens: %w", err)
	}

	// 3. 初始化链代币关联配置
	if err := initializeChainTokens(db); err != nil {
		tl_logger.Error("InitializePredefinedData Error: ", errors.New("failed to initialize chain tokens"), "error: ", err)
		return fmt.Errorf("failed to initialize chain tokens: %w", err)
	}

	tl_logger.Info("InitializePredefinedData: ", "predefined data initialization completed successfully")
	return nil
}

// CleanupDuplicateData 清理重复数据
func CleanupDuplicateData(db *gorm.DB) error {
	tl_logger.Info("CleanupDuplicateData: ", "starting cleanup of duplicate data")

	// 首先检查user_assets表是否存在
	var tableExists bool
	existsQuery := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_name = 'user_assets'
	)`

	if err := db.Raw(existsQuery).Scan(&tableExists).Error; err != nil {
		tl_logger.Error("CleanupDuplicateData Error: ", errors.New("failed to check table existence"), "error: ", err)
		return fmt.Errorf("failed to check table existence: %w", err)
	}

	if !tableExists {
		tl_logger.Info("CleanupDuplicateData: ", "user_assets table does not exist, skipping cleanup")
		return nil
	}

	// 清理user_assets表中的重复数据，保留最新的记录
	cleanupQuery := `
		WITH duplicate_assets AS (
			SELECT id, 
				   ROW_NUMBER() OVER (PARTITION BY user_id, chain_id, token_id ORDER BY last_updated DESC, updated_at DESC, id DESC) as rn
			FROM user_assets
		)
		DELETE FROM user_assets 
		WHERE id IN (
			SELECT id FROM duplicate_assets WHERE rn > 1
		)`

	result := db.Exec(cleanupQuery)
	if result.Error != nil {
		tl_logger.Error("CleanupDuplicateData Error: ", errors.New("failed to cleanup duplicate user_assets"), "error: ", result.Error)
		return fmt.Errorf("failed to cleanup duplicate user_assets: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		tl_logger.Info("CleanupDuplicateData: ", "cleaned up duplicate user_assets", "rows_affected: ", result.RowsAffected)
	}

	// 额外的清理：按钱包地址清理重复数据
	walletCleanupQuery := `
		WITH duplicate_wallet_assets AS (
			SELECT id, 
				   ROW_NUMBER() OVER (PARTITION BY wallet_address, chain_id, token_id ORDER BY last_updated DESC, updated_at DESC, id DESC) as rn
			FROM user_assets
		)
		DELETE FROM user_assets 
		WHERE id IN (
			SELECT id FROM duplicate_wallet_assets WHERE rn > 1
		)`

	result = db.Exec(walletCleanupQuery)
	if result.Error != nil {
		tl_logger.Error("CleanupDuplicateData Warning: ", errors.New("failed to cleanup wallet duplicate user_assets"), "error: ", result.Error)
	} else if result.RowsAffected > 0 {
		tl_logger.Info("CleanupDuplicateData: ", "cleaned up wallet duplicate user_assets", "rows_affected: ", result.RowsAffected)
	}

	// 清理其他表的重复数据
	tables := []struct {
		tableName    string
		uniqueFields string
		orderBy      string
	}{
		{"users", "wallet_address", "created_at DESC, id DESC"},
		{"support_tokens", "symbol", "created_at DESC, id DESC"},
		{"support_chains", "chain_id", "created_at DESC, id DESC"},
		{"chain_tokens", "chain_id, token_id", "created_at DESC, id DESC"},
	}

	for _, table := range tables {
		cleanupQuery := fmt.Sprintf(`
			WITH duplicate_records AS (
				SELECT id, 
					   ROW_NUMBER() OVER (PARTITION BY %s ORDER BY %s) as rn
				FROM %s
			)
			DELETE FROM %s 
			WHERE id IN (
				SELECT id FROM duplicate_records WHERE rn > 1
			)`, table.uniqueFields, table.orderBy, table.tableName, table.tableName)

		result := db.Exec(cleanupQuery)
		if result.Error != nil {
			tl_logger.Error("CleanupDuplicateData Warning: ", errors.New("failed to cleanup duplicates"), "table: ", table.tableName, "error: ", result.Error)
			// 继续处理其他表，不返回错误
		} else if result.RowsAffected > 0 {
			tl_logger.Info("CleanupDuplicateData: ", "cleaned up duplicates", "table: ", table.tableName, "rows_affected: ", result.RowsAffected)
		}
	}

	tl_logger.Info("CleanupDuplicateData: ", "duplicate data cleanup completed")
	return nil
}

// initializeSupportChains 初始化支持的区块链
func initializeSupportChains(db *gorm.DB) error {
	chains := []types.SupportChain{
		{ChainID: 1, Name: "Ethereum", Symbol: "ETH", RpcProvider: "alchemy", IsActive: true},
		{ChainID: 56, Name: "BSC", Symbol: "BNB", RpcProvider: "alchemy", IsActive: true},
		{ChainID: 137, Name: "Polygon", Symbol: "MATIC", RpcProvider: "alchemy", IsActive: true},
		{ChainID: 42161, Name: "Arbitrum One", Symbol: "ETH", RpcProvider: "alchemy", IsActive: true},
	}

	for _, chain := range chains {
		var existingChain types.SupportChain
		if err := db.Where("chain_id = ?", chain.ChainID).First(&existingChain).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 创建新记录
				if err := db.Create(&chain).Error; err != nil {
					tl_logger.Error("initializeSupportChains Error: ", errors.New("failed to create chain"), "chain_id: ", chain.ChainID, "error: ", err)
					return fmt.Errorf("failed to create chain %d: %w", chain.ChainID, err)
				}
				tl_logger.Info("initializeSupportChains: ", "created chain", "chain_id: ", chain.ChainID, "name: ", chain.Name)
			} else {
				tl_logger.Error("initializeSupportChains Error: ", errors.New("failed to query chain"), "chain_id: ", chain.ChainID, "error: ", err)
				return fmt.Errorf("failed to query chain %d: %w", chain.ChainID, err)
			}
		} else {
			// 更新现有记录
			existingChain.Name = chain.Name
			existingChain.Symbol = chain.Symbol
			existingChain.RpcProvider = chain.RpcProvider
			existingChain.IsActive = chain.IsActive
			if err := db.Save(&existingChain).Error; err != nil {
				tl_logger.Error("initializeSupportChains Error: ", errors.New("failed to update chain"), "chain_id: ", chain.ChainID, "error: ", err)
				return fmt.Errorf("failed to update chain %d: %w", chain.ChainID, err)
			}
			tl_logger.Info("initializeSupportChains: ", "updated chain", "chain_id: ", chain.ChainID, "name: ", chain.Name)
		}
	}

	return nil
}

// initializeSupportTokens 初始化支持的代币
func initializeSupportTokens(db *gorm.DB) error {
	tokens := []types.SupportToken{
		{Symbol: "ETH", Name: "Ethereum", CoingeckoID: "ethereum", Decimals: 18, IsActive: true},
		{Symbol: "BNB", Name: "BNB", CoingeckoID: "binancecoin", Decimals: 18, IsActive: true},
		{Symbol: "MATIC", Name: "Polygon", CoingeckoID: "matic-network", Decimals: 18, IsActive: true},
		{Symbol: "USDC", Name: "USD Coin", CoingeckoID: "usd-coin", Decimals: 6, IsActive: true},
		{Symbol: "USDT", Name: "Tether", CoingeckoID: "tether", Decimals: 6, IsActive: true},
		{Symbol: "WETH", Name: "Wrapped Ethereum", CoingeckoID: "weth", Decimals: 18, IsActive: true},
		{Symbol: "DAI", Name: "Dai Stablecoin", CoingeckoID: "dai", Decimals: 18, IsActive: true},
		{Symbol: "UNI", Name: "Uniswap", CoingeckoID: "uniswap", Decimals: 18, IsActive: true},
		{Symbol: "BTC", Name: "Bitcoin", CoingeckoID: "bitcoin", Decimals: 8, IsActive: true},
		{Symbol: "LINK", Name: "Chainlink", CoingeckoID: "chainlink", Decimals: 18, IsActive: true},
		{Symbol: "AAVE", Name: "Aave", CoingeckoID: "aave", Decimals: 18, IsActive: true},
		{Symbol: "CRV", Name: "Curve DAO Token", CoingeckoID: "curve-dao-token", Decimals: 18, IsActive: true},
		{Symbol: "COMP", Name: "Compound", CoingeckoID: "compound-governance-token", Decimals: 18, IsActive: true},
		{Symbol: "MKR", Name: "Maker", CoingeckoID: "maker", Decimals: 18, IsActive: true},
		{Symbol: "SNX", Name: "Synthetix", CoingeckoID: "havven", Decimals: 18, IsActive: true},
	}

	for _, token := range tokens {
		var existingToken types.SupportToken
		if err := db.Where("symbol = ?", token.Symbol).First(&existingToken).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 创建新记录
				if err := db.Create(&token).Error; err != nil {
					tl_logger.Error("initializeSupportTokens Error: ", errors.New("failed to create token"), "symbol: ", token.Symbol, "error: ", err)
					return fmt.Errorf("failed to create token %s: %w", token.Symbol, err)
				}
				tl_logger.Info("initializeSupportTokens: ", "created token", "symbol: ", token.Symbol, "name: ", token.Name)
			} else {
				tl_logger.Error("initializeSupportTokens Error: ", errors.New("failed to query token"), "symbol: ", token.Symbol, "error: ", err)
				return fmt.Errorf("failed to query token %s: %w", token.Symbol, err)
			}
		} else {
			// 更新现有记录
			existingToken.Name = token.Name
			existingToken.CoingeckoID = token.CoingeckoID
			existingToken.Decimals = token.Decimals
			existingToken.IsActive = token.IsActive
			if err := db.Save(&existingToken).Error; err != nil {
				tl_logger.Error("initializeSupportTokens Error: ", errors.New("failed to update token"), "symbol: ", token.Symbol, "error: ", err)
				return fmt.Errorf("failed to update token %s: %w", token.Symbol, err)
			}
			tl_logger.Info("initializeSupportTokens: ", "updated token", "symbol: ", token.Symbol, "name: ", token.Name)
		}
	}

	return nil
}

// initializeChainTokens 初始化链代币关联配置
func initializeChainTokens(db *gorm.DB) error {
	// 定义链代币配置
	chainTokenConfigs := []struct {
		ChainID         int64  // support_chains表的chain_id
		TokenSymbol     string // support_tokens表的symbol
		ContractAddress string
		IsNative        bool
	}{
		// Ethereum 主网 (chain_id = 1)
		{1, "ETH", "", true},
		{1, "USDC", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", false},
		{1, "USDT", "0xdAC17F958D2ee523a2206206994597C13D831ec7", false},
		{1, "DAI", "0x6B175474E89094C44Da98b954EedeAC495271d0F", false},
		{1, "UNI", "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984", false},
		{1, "LINK", "0x514910771AF9Ca656af840dff83E8264EcF986CA", false},
		{1, "AAVE", "0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9", false},
		{1, "CRV", "0xD533a949740bb3306d119CC777fa900bA034cd52", false},
		{1, "COMP", "0xc00e94Cb662C3520282E6f5717214004A7f26888", false},
		{1, "MKR", "0x9f8F72aA9304c8B593d555F12eF6589cC3A579A2", false},
		{1, "SNX", "0xC011a73ee8576Fb46F5E1c5751cA3B9Fe0af2a6F", false},

		// BSC 主网 (chain_id = 56)
		{56, "BNB", "", true},
		{56, "USDC", "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", false},
		{56, "USDT", "0x55d398326f99059fF775485246999027B3197955", false},
		{56, "ETH", "0x2170Ed0880ac9A755fd29B2688956BD959F933F8", false},

		// Polygon 主网 (chain_id = 137)
		{137, "MATIC", "", true},
		{137, "USDC", "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174", false},
		{137, "USDT", "0xc2132D05D31c914a87C6611C10748AEb04B58e8F", false},
		{137, "DAI", "0x8f3Cf7ad23Cd3CaDbD9735AFf958023239c6A063", false},
		{137, "WETH", "0x7ceB23fD6bC0adD59E62ac25578270cFf1b9f619", false},
		{137, "AAVE", "0xD6DF932A45C0f255f85145f286eA0b292B21C90B", false},

		// Arbitrum One (chain_id = 42161)
		{42161, "ETH", "", true},
		{42161, "USDC", "0xFF970A61A04b1cA14834A43f5dE4533eBDDB5CC8", false},
		{42161, "USDT", "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9", false},
		{42161, "DAI", "0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1", false},
		{42161, "UNI", "0xFa7F8980b0f1E64A2062791cc3b0871572f1F7f0", false},
		{42161, "LINK", "0xf97f4df75117a78c1A5a0DBb814Af92458539FB4", false},
	}

	for _, config := range chainTokenConfigs {
		// 获取链信息
		var chain types.SupportChain
		if err := db.Where("chain_id = ?", config.ChainID).First(&chain).Error; err != nil {
			tl_logger.Error("initializeChainTokens Error: ", errors.New("chain not found"), "chain_id: ", config.ChainID, "error: ", err)
			continue // 跳过这个配置，继续处理下一个
		}

		// 获取代币信息
		var token types.SupportToken
		if err := db.Where("symbol = ?", config.TokenSymbol).First(&token).Error; err != nil {
			tl_logger.Error("initializeChainTokens Error: ", errors.New("token not found"), "symbol: ", config.TokenSymbol, "error: ", err)
			continue // 跳过这个配置，继续处理下一个
		}

		// 检查是否已存在
		var existingChainToken types.ChainToken
		if err := db.Where("chain_id = ? AND token_id = ?", chain.ID, token.ID).First(&existingChainToken).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 创建新记录
				chainToken := types.ChainToken{
					ChainID:         chain.ID,
					TokenID:         token.ID,
					ContractAddress: config.ContractAddress,
					IsNative:        config.IsNative,
					IsActive:        true,
				}
				if err := db.Create(&chainToken).Error; err != nil {
					tl_logger.Error("initializeChainTokens Error: ", errors.New("failed to create chain token"), "chain_id: ", config.ChainID, "token_symbol: ", config.TokenSymbol, "error: ", err)
					return fmt.Errorf("failed to create chain token for chain %d, token %s: %w", config.ChainID, config.TokenSymbol, err)
				}
				tl_logger.Info("initializeChainTokens: ", "created chain token", "chain_id: ", config.ChainID, "token_symbol: ", config.TokenSymbol, "contract_address: ", config.ContractAddress)
			} else {
				tl_logger.Error("initializeChainTokens Error: ", errors.New("failed to query chain token"), "chain_id: ", config.ChainID, "token_symbol: ", config.TokenSymbol, "error: ", err)
				return fmt.Errorf("failed to query chain token for chain %d, token %s: %w", config.ChainID, config.TokenSymbol, err)
			}
		} else {
			// 更新现有记录
			existingChainToken.ContractAddress = config.ContractAddress
			existingChainToken.IsNative = config.IsNative
			existingChainToken.IsActive = true
			if err := db.Save(&existingChainToken).Error; err != nil {
				tl_logger.Error("initializeChainTokens Error: ", errors.New("failed to update chain token"), "chain_id: ", config.ChainID, "token_symbol: ", config.TokenSymbol, "error: ", err)
				return fmt.Errorf("failed to update chain token for chain %d, token %s: %w", config.ChainID, config.TokenSymbol, err)
			}
			tl_logger.Info("initializeChainTokens: ", "updated chain token", "chain_id: ", config.ChainID, "token_symbol: ", config.TokenSymbol, "contract_address: ", config.ContractAddress)
		}
	}

	return nil
}
