package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// Client Redis客户端封装
type Client struct {
	client *redis.Client
	config *config.RedisConfig
}

// NewClient 创建Redis客户端
func NewClient(cfg *config.RedisConfig) (*Client, error) {
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
		logger.Error("Failed to connect to redis", err)
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	logger.Info("Redis connection established", "host", cfg.Host, "port", cfg.Port, "db", cfg.DB)

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// GetClient 获取原始Redis客户端
func (c *Client) GetClient() *redis.Client {
	return c.client
}

// Close 关闭Redis连接
func (c *Client) Close() error {
	return c.client.Close()
}

// Ping 测试连接
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx).Result()
	return err
}

// Set 设置键值对
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Get 获取值
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	result := c.client.Get(ctx, key)
	if result.Err() != nil {
		if errors.Is(result.Err(), redis.Nil) {
			return "", nil
		}
		return "", result.Err()
	}
	return result.Val(), nil
}

// Del 删除键
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists 检查键是否存在
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Exists(ctx, keys...).Result()
}

// Expire 设置键的过期时间
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, key, expiration).Err()
}

// TTL 获取键的剩余生存时间
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}
