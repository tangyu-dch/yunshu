package selection

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RedisCallQueue 是基于 Redis List 实现的多租户排队队列。
// Key 格式为: cti:merchant:{merchantID}:queue:skill_group:{skillGroupID}
// 多租户前缀隔离确保不同商户的排队队列完全独立，避免跨商户数据污染。
type RedisCallQueue struct {
	Client *goredis.Client
}

// NewRedisCallQueue 创建一个 RedisCallQueue 实例。
func NewRedisCallQueue(client *goredis.Client) *RedisCallQueue {
	return &RedisCallQueue{Client: client}
}

// key 生成多租户隔离的队列 Redis Key。
// 格式: cti:merchant:{merchantID}:queue:skill_group:{skillGroupID}
func (q *RedisCallQueue) key(merchantID, skillGroupID int) string {
	return fmt.Sprintf("cti:merchant:%d:queue:skill_group:%d", merchantID, skillGroupID)
}

// Push 将呼叫 ID 推入排队队列（从队尾插入，FIFO 语义）。
// 同时设置 2 小时过期时间，防止死通道残留导致内存泄露。
func (q *RedisCallQueue) Push(ctx context.Context, merchantID, skillGroupID int, callID string) error {
	k := q.key(merchantID, skillGroupID)
	// 使用 RPush 从尾部入队，Pop 从头部出队，保证先入先出公平调度
	err := q.Client.RPush(ctx, k, callID).Err()
	if err != nil {
		return fmt.Errorf("redis rpush failed (merchant=%d, skillGroup=%d): %w", merchantID, skillGroupID, err)
	}
	// 设置 2 小时过期时间，防止死通道残留导致泄露
	q.Client.Expire(ctx, k, 2*time.Hour)
	return nil
}

// Pop 从排队队列中弹出一个呼叫 ID (LPop，先入先出)。
// 如果队列为空，返回空字符串和 nil error。
func (q *RedisCallQueue) Pop(ctx context.Context, merchantID, skillGroupID int) (string, error) {
	k := q.key(merchantID, skillGroupID)
	val, err := q.Client.LPop(ctx, k).Result()
	if err == goredis.Nil {
		// 队列为空，正常情况，返回空字符串
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("redis lpop failed (merchant=%d, skillGroup=%d): %w", merchantID, skillGroupID, err)
	}
	return val, nil
}

// Len 获取当前队列的排队人数。
func (q *RedisCallQueue) Len(ctx context.Context, merchantID, skillGroupID int) (int, error) {
	k := q.key(merchantID, skillGroupID)
	length, err := q.Client.LLen(ctx, k).Result()
	if err != nil {
		return 0, fmt.Errorf("redis llen failed (merchant=%d, skillGroup=%d): %w", merchantID, skillGroupID, err)
	}
	return int(length), nil
}

// Remove 原子地从队列中移除指定呼叫 ID（利用 LRem 命令）。
// 该方法用于两种场景：
//  1. 排队超时：30 秒超时协程调用 Remove，如果返回 removed > 0，说明客户仍在队列中等待，触发挂断。
//  2. 客户中途挂机：监听到客户腿挂机事件时，原子清理队列以避免坐席接单空指针起呼。
//
// count=0 表示移除所有匹配元素（通常队列中每个 callID 最多一条）。
func (q *RedisCallQueue) Remove(ctx context.Context, merchantID, skillGroupID int, callID string) (int64, error) {
	k := q.key(merchantID, skillGroupID)
	// LRem count=0 表示移除所有匹配，返回实际被移除的数量
	removed, err := q.Client.LRem(ctx, k, 0, callID).Result()
	if err != nil {
		return 0, fmt.Errorf("redis lrem failed (merchant=%d, skillGroup=%d, callId=%s): %w", merchantID, skillGroupID, callID, err)
	}
	return removed, nil
}
