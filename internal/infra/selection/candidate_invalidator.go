package selection

import (
	"context"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"
)

// RedisCandidateCacheInvalidator 清理 CTI 选号候选缓存。
type RedisCandidateCacheInvalidator struct {
	Client *goredis.Client
	Logger *slog.Logger
}

// InvalidateCandidateCache 删除所有候选缓存键。
func (i *RedisCandidateCacheInvalidator) InvalidateCandidateCache(ctx context.Context) error {
	if i.Client == nil {
		return nil
	}
	var cursor uint64
	var deleted int64
	for {
		keys, next, err := i.Client.Scan(ctx, cursor, "cti:phone_resource:user:*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			deleted += int64(len(keys))
			if err := i.Client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	i.logger().Info("CTI 选号候选缓存已失效", "pattern", "cti:phone_resource:user:*", "deleted", deleted)
	return nil
}

func (i *RedisCandidateCacheInvalidator) logger() *slog.Logger {
	if i.Logger != nil {
		return i.Logger
	}
	return slog.Default()
}
