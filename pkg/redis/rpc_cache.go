package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// RPCMetadata RPC元数据结构
type RPCMetadata struct {
	URL                  string    `json:"url"`
	ChainID              int       `json:"chain_id"`
	IsHealthy            bool      `json:"is_healthy"`
	MaxSafeRange         int       `json:"max_safe_range"`
	LastCheckedAt        time.Time `json:"last_checked_at"`
	ResponseTimeMs       int64     `json:"response_time_ms"`
	ErrorCount           int       `json:"error_count"`
	LastError            *string   `json:"last_error"`
	CapabilityLastTested time.Time `json:"capability_last_tested"`
}

// RPCCache RPC缓存管理器
type RPCCache struct {
	client *Client
}

// NewRPCCache 创建RPC缓存管理器
func NewRPCCache(client *Client) *RPCCache {
	return &RPCCache{
		client: client,
	}
}

// GetRPCMetadataKey 获取RPC元数据的Redis键
func (rc *RPCCache) GetRPCMetadataKey(chainID int, rpcURL string) string {
	return fmt.Sprintf("rpc:metadata:%d:%s", chainID, rpcURL)
}

// GetRPCQueueKey 获取RPC优先队列的Redis键
func (rc *RPCCache) GetRPCQueueKey(chainID int) string {
	return fmt.Sprintf("rpc:queue:%d", chainID)
}

// FIFO 队列键（使用Redis List实现）
func (rc *RPCCache) GetRPCFIFOQueueKey(chainID int) string {
	return fmt.Sprintf("rpc:queue_fifo:%d", chainID)
}

// SetRPCMetadata 设置RPC元数据
func (rc *RPCCache) SetRPCMetadata(ctx context.Context, metadata *RPCMetadata) error {
	key := rc.GetRPCMetadataKey(metadata.ChainID, metadata.URL)

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal RPC metadata: %w", err)
	}

	// 设置24小时过期时间
	return rc.client.Set(ctx, key, data, 24*time.Hour)
}

// GetRPCMetadata 获取RPC元数据
func (rc *RPCCache) GetRPCMetadata(ctx context.Context, chainID int, rpcURL string) (*RPCMetadata, error) {
	key := rc.GetRPCMetadataKey(chainID, rpcURL)

	data, err := rc.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get RPC metadata: %w", err)
	}

	if data == "" {
		return nil, nil // 不存在
	}

	var metadata RPCMetadata
	if err := json.Unmarshal([]byte(data), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal RPC metadata: %w", err)
	}

	return &metadata, nil
}

// DeleteRPCMetadata 删除RPC元数据
func (rc *RPCCache) DeleteRPCMetadata(ctx context.Context, chainID int, rpcURL string) error {
	key := rc.GetRPCMetadataKey(chainID, rpcURL)
	return rc.client.Del(ctx, key)
}

// PushRPCToFIFOQueue 将RPC添加到FIFO队列尾部
func (rc *RPCCache) PushRPCToFIFOQueue(ctx context.Context, chainID int, rpcURL string) error {
	key := rc.GetRPCFIFOQueueKey(chainID)
	return rc.client.client.RPush(ctx, key, rpcURL).Err()
}

// PopRPCFromFIFOQueue 从FIFO队列头部弹出一个RPC
func (rc *RPCCache) PopRPCFromFIFOQueue(ctx context.Context, chainID int) (string, error) {
	key := rc.GetRPCFIFOQueueKey(chainID)
	res := rc.client.client.LPop(ctx, key)
	if res.Err() != nil {
		return "", res.Err()
	}
	val := res.Val()
	if val == "" {
		return "", fmt.Errorf("no RPC available in FIFO queue for chain %d", chainID)
	}
	return val, nil
}

// RemoveFromFIFOQueue 从FIFO队列中移除指定RPC的所有出现
func (rc *RPCCache) RemoveFromFIFOQueue(ctx context.Context, chainID int, rpcURL string) error {
	key := rc.GetRPCFIFOQueueKey(chainID)
	// LREM key 0 value 移除所有匹配的元素
	return rc.client.client.LRem(ctx, key, 0, rpcURL).Err()
}
