package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// NewRedisConnection 创建Redis连接
func NewRedisConnection(cfg *config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		logger.Error("NewRedisConnection Error: ", errors.New("failed to connect to redis"), "error: ", err)
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	logger.Info("NewRedisConnection: ", "host: ", cfg.Host, "port: ", cfg.Port, "db: ", cfg.DB)
	return client, nil
}
