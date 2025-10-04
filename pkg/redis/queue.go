package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// QueueManager Redis队列管理器
type QueueManager struct {
	client *Client
}

// NewQueueManager 创建队列管理器
func NewQueueManager(client *Client) *QueueManager {
	return &QueueManager{
		client: client,
	}
}

// GetLogQueueKey 获取日志队列的Redis键
func (qm *QueueManager) GetLogQueueKey(chainID int) string {
	return fmt.Sprintf("chain:%d:logs", chainID)
}

// PushLog 推送日志到队列尾部
func (qm *QueueManager) PushLog(ctx context.Context, chainID int, logData interface{}) error {
	key := qm.GetLogQueueKey(chainID)

	data, err := json.Marshal(logData)
	if err != nil {
		return fmt.Errorf("failed to marshal log data: %w", err)
	}

	return qm.client.client.RPush(ctx, key, data).Err()
}

// PushLogs 批量推送日志到队列尾部
func (qm *QueueManager) PushLogs(ctx context.Context, chainID int, logsData []interface{}) error {
	if len(logsData) == 0 {
		return nil
	}

	key := qm.GetLogQueueKey(chainID)

	// 序列化所有日志数据
	serializedLogs := make([]interface{}, len(logsData))
	for i, logData := range logsData {
		data, err := json.Marshal(logData)
		if err != nil {
			return fmt.Errorf("failed to marshal log data at index %d: %w", i, err)
		}
		serializedLogs[i] = data
	}

	return qm.client.client.RPush(ctx, key, serializedLogs...).Err()
}

// PopLog 从队列头部弹出一个日志
func (qm *QueueManager) PopLog(ctx context.Context, chainID int) (string, error) {
	key := qm.GetLogQueueKey(chainID)

	result := qm.client.client.LPop(ctx, key)
	if result.Err() != nil {
		return "", result.Err()
	}

	return result.Val(), nil
}

// PopLogs 从队列头部批量弹出日志
func (qm *QueueManager) PopLogs(ctx context.Context, chainID int, count int64) ([]string, error) {
	key := qm.GetLogQueueKey(chainID)

	// 使用 LPOP 的批量版本 (Redis 6.2+)
	result := qm.client.client.LPopCount(ctx, key, int(count))
	if result.Err() != nil {
		return nil, result.Err()
	}

	return result.Val(), nil
}

// GetQueueLength 获取队列长度
func (qm *QueueManager) GetQueueLength(ctx context.Context, chainID int) (int64, error) {
	key := qm.GetLogQueueKey(chainID)
	return qm.client.client.LLen(ctx, key).Result()
}

// PeekLogs 查看队列头部的日志(不弹出)
func (qm *QueueManager) PeekLogs(ctx context.Context, chainID int, count int64) ([]string, error) {
	key := qm.GetLogQueueKey(chainID)

	// 使用 LRANGE 查看队列头部数据
	result := qm.client.client.LRange(ctx, key, 0, count-1)
	if result.Err() != nil {
		return nil, result.Err()
	}

	return result.Val(), nil
}

// ClearQueue 清空队列
func (qm *QueueManager) ClearQueue(ctx context.Context, chainID int) error {
	key := qm.GetLogQueueKey(chainID)
	return qm.client.client.Del(ctx, key).Err()
}

// BlockingPopLog 阻塞式弹出日志 (如果队列为空则等待)
func (qm *QueueManager) BlockingPopLog(ctx context.Context, chainID int, timeout time.Duration) (string, error) {
	key := qm.GetLogQueueKey(chainID)

	result := qm.client.client.BLPop(ctx, timeout, key)
	if result.Err() != nil {
		return "", result.Err()
	}

	// BLPop 返回 [key, value] 的数组，我们需要第二个元素
	values := result.Val()
	if len(values) < 2 {
		return "", fmt.Errorf("unexpected BLPop result format")
	}

	return values[1], nil
}

// GetAllQueueStats 获取所有队列的统计信息
func (qm *QueueManager) GetAllQueueStats(ctx context.Context, chainIDs []int) (map[int]int64, error) {
	stats := make(map[int]int64)

	for _, chainID := range chainIDs {
		length, err := qm.GetQueueLength(ctx, chainID)
		if err != nil {
			return nil, fmt.Errorf("failed to get queue length for chain %d: %w", chainID, err)
		}
		stats[chainID] = length
	}

	return stats, nil
}
