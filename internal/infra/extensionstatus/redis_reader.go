// Package extensionstatus 提供  兼容的分机状态读取 adapter。
package extensionstatus

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
)

const (
	//  Constant.EXTENSION_STATUS_KEY，hash field 为分机号，value 为 ExtensionStatus.status。
	redisExtensionStatusKey = contracts.KeyExtensionStatus

	// extensionAliveKeyPrefix 伴生活跃标记前缀。每个非离线状态的分机会对应一个 extension:alive:{ext} key。
	// 该 key 带有 TTL，若 SIP 终端非正常断开（进程崩溃、网络中断），alive key 过期后由后台清理协程
	// 自动将对应的 Hash field 重置为离线，避免幽灵在线状态。
	extensionAliveKeyPrefix = "extension:alive:"

	// extensionAliveTTL 伴生活跃标记的 TTL，默认 5 分钟。
	// SIP 注册通常每 60-120 秒续期一次，5 分钟提供充足的安全余量。
	extensionAliveTTL = 5 * time.Minute
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

// SetExtensionStatus 写入分机状态，同时维护伴生活跃标记。
//
// 对于非离线状态，在 pipeline 中同时写入 Hash field 和一个带 TTL 的 alive key；
// 对于离线状态，同时删除 alive key。所有操作在单个 pipeline 中完成以减少 RTT。
func (r *RedisReader) SetExtensionStatus(ctx context.Context, extension string, status esl.ExtensionStatus) error {
	aliveKey := extensionAliveKeyPrefix + extension
	pipe := r.Client.Pipeline()

	pipe.HSet(ctx, redisExtensionStatusKey, extension, int(status))

	if status == esl.ExtensionStatusOffline {
		pipe.Del(ctx, aliveKey)
	} else {
		pipe.Set(ctx, aliveKey, "1", extensionAliveTTL)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// StartStatusCleaner 启动后台清理协程，周期扫描 extension:status Hash，
// 将已失去 alive key 的陈旧分机状态重置为离线。
//
// 当 SIP 终端非正常断开时（进程崩溃、网络中断），alive key 在 TTL 到期后自动消失，
// 清理协程随之检测到并将 Hash field 设为 -1（离线），避免幽灵在线状态。
func (r *RedisReader) StartStatusCleaner(ctx context.Context, interval time.Duration) {
	if r.Client == nil {
		return
	}
	if interval <= 0 {
		interval = 60 * time.Second
	}
	logger := slog.Default()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.cleanStaleStatuses(ctx, logger)
		}
	}
}

// cleanStaleStatuses 执行一次清理扫描。
func (r *RedisReader) cleanStaleStatuses(ctx context.Context, logger *slog.Logger) {
	fields, err := r.Client.HGetAll(ctx, redisExtensionStatusKey).Result()
	if err != nil {
		logger.Error("分机状态清理协程读取 Hash 失败", "error", err.Error())
		return
	}

	cleaned := 0
	for ext, raw := range fields {
		status, err := strconv.Atoi(raw)
		if err != nil {
			continue
		}
		// 只检查非离线状态
		if esl.ExtensionStatus(status) == esl.ExtensionStatusOffline {
			continue
		}

		aliveKey := extensionAliveKeyPrefix + ext
		exists, err := r.Client.Exists(ctx, aliveKey).Result()
		if err != nil {
			logger.Error("分机状态清理协程检查 alive key 失败", "extension", ext, "error", err.Error())
			continue
		}

		if exists == 0 {
			// alive key 已过期，重置为离线
			r.Client.HSet(ctx, redisExtensionStatusKey, ext, int(esl.ExtensionStatusOffline))
			r.Client.Del(ctx, aliveKey)
			cleaned++
			logger.Info("分机状态清理协程重置幽灵在线状态", "extension", ext, "previousStatus", status)
		}
	}

	if cleaned > 0 {
		logger.Info("分机状态清理协程完成扫描", "total", len(fields), "cleaned", cleaned)
	}
}
