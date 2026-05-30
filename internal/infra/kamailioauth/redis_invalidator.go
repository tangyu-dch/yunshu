package kamailioauth

import (
	"context"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
)

// RedisAuthCacheInvalidator 负责清理 Kamailio auth 缓存。
type RedisAuthCacheInvalidator struct {
	Client *goredis.Client
	Logger *slog.Logger
}

// InvalidateAuthCache 清理 `kamailio:auth:*` 缓存键。
func (i *RedisAuthCacheInvalidator) InvalidateAuthCache(ctx context.Context) error {
	if i == nil || i.Client == nil {
		return nil
	}
	logger := i.logger()
	var cursor uint64
	var deleted int64
	for {
		keys, next, err := i.Client.Scan(ctx, cursor, contracts.KeyKamailioAuthPrefix, 100).Result()
		if err != nil {
			logger.Error("清理 Kamailio auth 缓存失败", "error", err.Error())
			return err
		}
		if len(keys) > 0 {
			if err := i.Client.Del(ctx, keys...).Err(); err != nil {
				logger.Error("删除 Kamailio auth 缓存键失败", "error", err.Error())
				return err
			}
			deleted += int64(len(keys))
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	logger.Info("Kamailio auth 缓存已清理", "deleted", deleted)
	return nil
}

func (i *RedisAuthCacheInvalidator) logger() *slog.Logger {
	if i != nil && i.Logger != nil {
		return i.Logger
	}
	return slog.Default()
}
