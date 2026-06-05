// Package redis 提供 Redis 客户端创建和健康检查能力。
package redis

import (
	"context"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/infra/config"
)

// NewClient 创建 Redis 客户端。
// 当前先使用单节点客户端；如果后续生产使用 Cluster，可以在这里根据 addrs 数量和配置切换。
func NewClient(cfg config.RedisConfig) *goredis.Client {
	addr := "127.0.0.1:6379"
	if len(cfg.Addrs) > 0 && cfg.Addrs[0] != "" {
		addr = cfg.Addrs[0]
	}
	if len(cfg.Addrs) > 1 {
		slog.Warn("Redis 配置了多个地址但仅使用第一个（单节点模式）", "addr", addr, "allAddrs", cfg.Addrs)
	}
	readTimeout := cfg.ReadTimeout
	if readTimeout == 0 {
		readTimeout = 3 * time.Second
	}
	writeTimeout := cfg.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = 3 * time.Second
	}
	return goredis.NewClient(&goredis.Options{
		Addr:            addr,
		DB:              cfg.DB,
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		DialTimeout:     5 * time.Second,
		PoolSize:        20,                       // 连接池大小
		MinIdleConns:    5,                        // 最小空闲连接
		ConnMaxIdleTime: 5 * time.Minute,          // 空闲连接最大存活时间
		ConnMaxLifetime: 30 * time.Minute,         // 连接最大生命周期
		MaxRetries:      3,                        // 最大重试次数
		MinRetryBackoff: 8 * time.Millisecond,     // 重试最小退避
		MaxRetryBackoff: 512 * time.Millisecond,   // 重试最大退避
	})
}

// Ping 用于启动期或 readiness 检查确认 Redis 可用。
func Ping(ctx context.Context, client *goredis.Client) error {
	return client.Ping(ctx).Err()
}
