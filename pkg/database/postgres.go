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
		Logger: logger.Default.LogMode(logger.Info),
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
	err := db.AutoMigrate(
		&types.User{},
		// 未来会添加更多模型
	)
	if err != nil {
		tl_logger.Error("AutoMigrate Error: ", errors.New("failed to migrate database"), "error: ", err)
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	tl_logger.Info("AutoMigrate: ", "database migration completed successfully")
	return nil
}

// CreateIndexes 创建额外的数据库索引
func CreateIndexes(db *gorm.DB) error {
	// 为用户表创建索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_wallet_address ON users(wallet_address)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create wallet_address index"), "error: ", err)
		return fmt.Errorf("failed to create wallet_address index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create created_at index"), "error: ", err)
		return fmt.Errorf("failed to create created_at index: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_status ON users(status)").Error; err != nil {
		tl_logger.Error("CreateIndexes Error: ", errors.New("failed to create status index"), "error: ", err)
		return fmt.Errorf("failed to create status index: %w", err)
	}

	tl_logger.Info("CreateIndexes: ", "database indexes created successfully")
	return nil
}
