// Package redis 提供 Redis 客户端创建和健康检查能力。
package redis

import (
	"context"
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
	readTimeout := cfg.ReadTimeout
	if readTimeout == 0 {
		readTimeout = 3 * time.Second
	}
	writeTimeout := cfg.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = 3 * time.Second
	}
	return goredis.NewClient(&goredis.Options{
		Addr:         addr,
		DB:           cfg.DB,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})
}

// Ping 用于启动期或 readiness 检查确认 Redis 可用。
func Ping(ctx context.Context, client *goredis.Client) error {
	return client.Ping(ctx).Err()
}
