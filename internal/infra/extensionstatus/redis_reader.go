// Package extensionstatus 提供  兼容的分机状态读取 adapter。
package extensionstatus

import (
	"context"
	"strconv"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
)

const (
	//  Constant.EXTENSION_STATUS_KEY，hash field 为分机号，value 为 ExtensionStatus.status。
	redisExtensionStatusKey = contracts.KeyExtensionStatus
)

// RedisReader 是从 Redis hash 读取分机在线/忙闲状态的数据访问对象。
// 用于 API 外呼前验证分机是否已 SIP 注册。
type RedisReader struct {
	Client *goredis.Client
}

// NewRedisReader 创建 Redis 分机状态读取器。
// client 参数为已初始化的 go-redis 客户端实例。
func NewRedisReader(client *goredis.Client) *RedisReader {
	return &RedisReader{Client: client}
}

// GetExtensionStatus 读取分机状态。
//
// 返回 ok=false 表示 Redis 中没有该分机状态，按  逻辑等价于离线。
func (r *RedisReader) GetExtensionStatus(ctx context.Context, extension string) (esl.ExtensionStatus, bool, error) {
	raw, err := r.Client.HGet(ctx, redisExtensionStatusKey, extension).Result()
	if err == goredis.Nil {
		return esl.ExtensionStatusOffline, false, nil
	}
	if err != nil {
		return esl.ExtensionStatusOffline, false, err
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return esl.ExtensionStatusOffline, false, err
	}
	return esl.ExtensionStatus(value), true, nil
}

// SetExtensionStatus 写入分机状态。
//
// TODO: 当前实现未设置 TTL。如果分机下线时未能更新此 Hash（如进程崩溃、网络中断），
// 对应的 field 将永久保留在 Redis 中，导致后续查询误认为该分机仍在线。
// 建议后续增加 HSET + EXPIRE 组合，或在写入侧定期清理过期分机状态。
func (r *RedisReader) SetExtensionStatus(ctx context.Context, extension string, status esl.ExtensionStatus) error {
	return r.Client.HSet(ctx, redisExtensionStatusKey, extension, int(status)).Err()
}
