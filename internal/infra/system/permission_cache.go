// Package system 提供管理端权限系统的持久化与缓存支持。
//
// CachedPermissionRepository 在 PermissionRepository 基础上添加了
// Redis + 本地内存二级缓存，用于加速路由权限解析，减少数据库查询压力。
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	authdomain "yunshu/internal/domain/auth"
)

const (
	// KeyConsolePermInvalidate 是权限变更通知的 Pub/Sub 通道名。
	KeyConsolePermInvalidate = "cc:perm:invalidate"
	// KeyConsoleRoutePermCachePrefix 是路由权限缓存的 Redis Key 前缀。
	KeyConsoleRoutePermCachePrefix = "cc:perm:route:"
)

// CachedPermissionRepository 在 PermissionRepository 基础上添加了 Redis + 本地内存二级缓存。
//
// 缓存策略：
//  1. 本地内存缓存（sync.Map）：缓存最近解析结果，TTL 默认 5 秒
//  2. Redis Hash 缓存：按 HTTP Method 分组缓存路由规则，TTL 默认 10 分钟
//  3. 失效通知：权限变更时通过 Redis Pub/Sub 通知其他实例刷新本地缓存
type CachedPermissionRepository struct {
	DB                *gorm.DB
	Redis             *redis.Client
	Logger            *slog.Logger
	Inner             *PermissionRepository
	LocalCacheTTL     time.Duration
	RedisCacheTTL     time.Duration

	// 本地内存缓存
	localCacheMu sync.RWMutex
	localCache   map[string]localCacheEntry

	// 订阅取消函数
	subCancel context.CancelFunc
}

type localCacheEntry struct {
	permission contracts.PermissionCode
	found      bool
	expiresAt  time.Time
}

// NewCachedPermissionRepository 创建带缓存的权限仓储。
func NewCachedPermissionRepository(db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) *CachedPermissionRepository {
	repo := &CachedPermissionRepository{
		DB:            db,
		Redis:         redisClient,
		Logger:        logger,
		Inner:         NewPermissionRepository(db, logger),
		LocalCacheTTL: 5 * time.Second,
		RedisCacheTTL: 10 * time.Minute,
		localCache:    make(map[string]localCacheEntry),
	}
	return repo
}

// Start 启动权限变更订阅。
func (r *CachedPermissionRepository) Start(ctx context.Context) {
	if r.Redis == nil {
		return
	}
	// 订阅权限变更通知
	subCtx, cancel := context.WithCancel(ctx)
	r.subCancel = cancel
	go func() {
		pubsub := r.Redis.Subscribe(subCtx, KeyConsolePermInvalidate)
		defer pubsub.Close()
		ch := pubsub.Channel()
		r.logger().Info("权限变更通知订阅已启动", "channel", KeyConsolePermInvalidate)
		for {
			select {
			case <-subCtx.Done():
				r.logger().Info("权限变更通知订阅已停止")
				return
			case msg := <-ch:
				if msg != nil {
					r.logger().Info("收到权限变更通知，清空本地缓存", "message", msg.Payload)
					r.ClearLocalCache()
				}
			}
		}
	}()
}

// Stop 停止权限变更订阅。
func (r *CachedPermissionRepository) Stop() {
	if r.subCancel != nil {
		r.subCancel()
	}
}

// EnsureDefaults 确保权限系统默认数据存在。
func (r *CachedPermissionRepository) EnsureDefaults(ctx context.Context) error {
	return r.Inner.EnsureDefaults(ctx)
}

// ResolveLoginPermissions 按角色解析登录权限。
func (r *CachedPermissionRepository) ResolveLoginPermissions(ctx context.Context, req authdomain.LoginRequest) ([]string, bool, error) {
	return r.Inner.ResolveLoginPermissions(ctx, req)
}

// RequiredPermissionForRequest 按请求路径和方法解析所需权限，优先使用缓存。
func (r *CachedPermissionRepository) RequiredPermissionForRequest(ctx context.Context, path, method string) (contracts.PermissionCode, bool, error) {
	// 1. 本地内存缓存查询
	cacheKey := fmt.Sprintf("%s:%s", method, path)
	if perm, found := r.getFromLocalCache(cacheKey); found {
		r.logger().Debug("路由权限命中本地缓存", "path", path, "method", method, "permission", perm)
		return perm, true, nil
	}

	// 2. Redis 缓存查询
	if r.Redis != nil {
		redisKey := KeyConsoleRoutePermCachePrefix + method
		cachedPerm, err := r.Redis.HGet(ctx, redisKey, path).Result()
		if err == nil && cachedPerm != "" {
			var result struct {
				Permission string `json:"permission"`
				Found      bool   `json:"found"`
			}
			if jsonErr := json.Unmarshal([]byte(cachedPerm), &result); jsonErr == nil {
				perm := contracts.PermissionCode(result.Permission)
				r.setLocalCache(cacheKey, perm, result.Found)
				r.logger().Debug("路由权限命中 Redis 缓存", "path", path, "method", method, "permission", perm)
				return perm, result.Found, nil
			}
		} else if err != nil && err != redis.Nil {
			r.logger().Warn("读取 Redis 路由权限缓存失败，降级查询数据库", "error", err.Error())
		}
	}

	// 3. 数据库查询
	permission, found, err := r.Inner.RequiredPermissionForRequest(ctx, path, method)
	if err != nil {
		return "", false, err
	}

	// 4. 回写缓存
	if r.Redis != nil {
		r.writeRouteCache(ctx, method, path, permission, found)
	}
	r.setLocalCache(cacheKey, permission, found)

	return permission, found, nil
}

// WarmRouteCache 启动时预热路由权限缓存到 Redis。
func (r *CachedPermissionRepository) WarmRouteCache(ctx context.Context) error {
	if r.Redis == nil || r.DB == nil {
		return nil
	}

	r.logger().Info("开始预热路由权限缓存")

	// 查询所有启用的路由规则
	var rules []ConsoleRoutePermissionModel
	err := r.DB.WithContext(ctx).
		Where("enable = ? AND del_flag = ?", true, false).
		Find(&rules).Error
	if err != nil {
		r.logger().Error("预热路由权限缓存时查询数据库失败", "error", err.Error())
		return err
	}

	// 按 Method 分组写入 Redis Hash
	rulesByMethod := make(map[string][]ConsoleRoutePermissionModel)
	for _, rule := range rules {
		method := rule.Method
		if method == "" {
			method = "*"
		}
		rulesByMethod[method] = append(rulesByMethod[method], rule)
	}

	// 写入 Redis 缓存
	for method, methodRules := range rulesByMethod {
		redisKey := KeyConsoleRoutePermCachePrefix + method
		pipe := r.Redis.Pipeline()
		for _, rule := range methodRules {
			entry, err := json.Marshal(map[string]any{
				"permission": rule.PermissionCode,
				"found":      true,
			})
			if err != nil {
				continue
			}
			pipe.HSet(ctx, redisKey, rule.PathPrefix, entry)
		}
		pipe.Expire(ctx, redisKey, r.RedisCacheTTL)
		if _, err := pipe.Exec(ctx); err != nil {
			r.logger().Warn("预热路由权限缓存到 Redis 失败", "method", method, "error", err.Error())
		}
	}

	r.logger().Info("路由权限缓存预热完成", "methods", len(rulesByMethod), "totalRules", len(rules))
	return nil
}

// InvalidateRouteCache 清除路由权限缓存（Redis + 本地）。
func (r *CachedPermissionRepository) InvalidateRouteCache(ctx context.Context) error {
	r.ClearLocalCache()

	if r.Redis == nil {
		return nil
	}

	// 清除 Redis 中的所有路由权限缓存（使用 SCAN 避免 KEYS 阻塞）
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = r.Redis.Scan(ctx, cursor, KeyConsoleRoutePermCachePrefix+"*", 100).Result()
		if err != nil && err != redis.Nil {
			r.logger().Warn("SCAN Redis 路由权限缓存键失败", "error", err.Error())
			return err
		}
		if len(keys) > 0 {
			if err := r.Redis.Del(ctx, keys...).Err(); err != nil {
				r.logger().Warn("删除 Redis 路由权限缓存失败", "error", err.Error())
				return err
			}
		}
		if cursor == 0 {
			break
		}
	}

	// 发布失效通知
	if err := r.Redis.Publish(ctx, KeyConsolePermInvalidate, "refresh").Err(); err != nil {
		r.logger().Warn("发布权限变更通知失败", "error", err.Error())
	}

	r.logger().Info("路由权限缓存已清除")
	return nil
}

// PageRoles 分页查询角色。
func (r *CachedPermissionRepository) PageRoles(ctx context.Context, req RolePageRequest) (RolePageResult, error) {
	return r.Inner.PageRoles(ctx, req)
}

// SaveRole 保存角色。
func (r *CachedPermissionRepository) SaveRole(ctx context.Context, role ConsoleRoleModel) (ConsoleRoleModel, error) {
	result, err := r.Inner.SaveRole(ctx, role)
	if err == nil {
		_ = r.InvalidateRouteCache(ctx)
	}
	return result, err
}

// DeleteRoles 逻辑删除角色。
func (r *CachedPermissionRepository) DeleteRoles(ctx context.Context, codes []string) error {
	err := r.Inner.DeleteRoles(ctx, codes)
	if err == nil {
		_ = r.InvalidateRouteCache(ctx)
	}
	return err
}

// ListPermissions 获取所有启用权限。
func (r *CachedPermissionRepository) ListPermissions(ctx context.Context) ([]ConsolePermissionModel, error) {
	return r.Inner.ListPermissions(ctx)
}

// GetRolePermissions 获取角色的权限编码列表。
func (r *CachedPermissionRepository) GetRolePermissions(ctx context.Context, roleCode string) ([]string, error) {
	return r.Inner.GetRolePermissions(ctx, roleCode)
}

// SaveRolePermissions 保存角色的权限编码映射。
func (r *CachedPermissionRepository) SaveRolePermissions(ctx context.Context, roleCode string, permissionCodes []string) error {
	err := r.Inner.SaveRolePermissions(ctx, roleCode, permissionCodes)
	if err == nil {
		_ = r.InvalidateRouteCache(ctx)
	}
	return err
}

// ClearLocalCache 清空本地内存缓存。
func (r *CachedPermissionRepository) ClearLocalCache() {
	r.localCacheMu.Lock()
	defer r.localCacheMu.Unlock()
	r.localCache = make(map[string]localCacheEntry)
}

func (r *CachedPermissionRepository) getFromLocalCache(key string) (contracts.PermissionCode, bool) {
	r.localCacheMu.RLock()
	defer r.localCacheMu.RUnlock()

	entry, exists := r.localCache[key]
	if !exists {
		return "", false
	}

	if time.Now().After(entry.expiresAt) {
		// 缓存已过期
		delete(r.localCache, key)
		return "", false
	}

	return entry.permission, entry.found
}

func (r *CachedPermissionRepository) setLocalCache(key string, permission contracts.PermissionCode, found bool) {
	r.localCacheMu.Lock()
	defer r.localCacheMu.Unlock()

	r.localCache[key] = localCacheEntry{
		permission: permission,
		found:      found,
		expiresAt:  time.Now().Add(r.LocalCacheTTL),
	}
}

func (r *CachedPermissionRepository) writeRouteCache(ctx context.Context, method, path string, permission contracts.PermissionCode, found bool) {
	entry, err := json.Marshal(map[string]any{
		"permission": string(permission),
		"found":      found,
	})
	if err != nil {
		r.logger().Warn("序列化路由权限缓存失败", "error", err.Error())
		return
	}

	redisKey := KeyConsoleRoutePermCachePrefix + method
	pipe := r.Redis.Pipeline()
	pipe.HSet(ctx, redisKey, path, entry)
	pipe.Expire(ctx, redisKey, r.RedisCacheTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		r.logger().Warn("写入路由权限缓存到 Redis 失败", "error", err.Error())
	}
}

func (r *CachedPermissionRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
