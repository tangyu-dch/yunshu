package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
	authdomain "yunshu/internal/domain/auth"
)

const redisAuthSessionKeyPrefix = contracts.KeyConsoleAuthSessionPrefix

// RedisSessionStore 是管理端多实例共享的 Redis 会话存储。
// 会话数据会按 TTL 自动过期，注销则直接删除对应 token 键。
type RedisSessionStore struct {
	Client *goredis.Client
	Prefix string
	Now    func() time.Time
}

// NewRedisSessionStore 创建 Redis 会话存储。
func NewRedisSessionStore(client *goredis.Client, prefix string) *RedisSessionStore {
	if prefix == "" {
		prefix = redisAuthSessionKeyPrefix
	}
	return &RedisSessionStore{Client: client, Prefix: prefix, Now: time.Now}
}

// Issue 生成 token 并写入 Redis，保证多实例共享登录态。
func (s *RedisSessionStore) Issue(ctx context.Context, tenant contracts.TenantContext, ttl time.Duration) (authdomain.AuthTicket, error) {
	if s == nil || s.Client == nil {
		return authdomain.AuthTicket{}, authdomain.ErrSessionStoreUnavailable
	}
	token, err := randomToken()
	if err != nil {
		return authdomain.AuthTicket{}, err
	}
	ticket := authdomain.AuthTicket{
		Token:     token,
		Tenant:    tenant,
		ExpiresAt: s.now().UTC().Add(ttl),
	}
	raw, err := json.Marshal(ticket)
	if err != nil {
		return authdomain.AuthTicket{}, err
	}
	if err := s.Client.Set(ctx, s.key(token), raw, ttl).Err(); err != nil {
		return authdomain.AuthTicket{}, err
	}
	return ticket, nil
}

// Get 从 Redis 读取 token 对应的会话票据。
func (s *RedisSessionStore) Get(ctx context.Context, token string) (authdomain.AuthTicket, bool) {
	if s == nil || s.Client == nil {
		return authdomain.AuthTicket{}, false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return authdomain.AuthTicket{}, false
	}
	raw, err := s.Client.Get(ctx, s.key(token)).Result()
	if err == goredis.Nil {
		return authdomain.AuthTicket{}, false
	}
	if err != nil {
		return authdomain.AuthTicket{}, false
	}
	var ticket authdomain.AuthTicket
	if err := json.Unmarshal([]byte(raw), &ticket); err != nil {
		return authdomain.AuthTicket{}, false
	}
	if ticket.Token == "" {
		ticket.Token = token
	}
	if !ticket.ExpiresAt.IsZero() && !ticket.ExpiresAt.After(s.now().UTC()) {
		_ = s.Client.Del(ctx, s.key(token)).Err()
		return authdomain.AuthTicket{}, false
	}
	return ticket, true
}

// Revoke 删除 Redis 中的 token。
func (s *RedisSessionStore) Revoke(ctx context.Context, token string) error {
	if s == nil || s.Client == nil {
		return authdomain.ErrSessionStoreUnavailable
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return s.Client.Del(ctx, s.key(token)).Err()
}

func (s *RedisSessionStore) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *RedisSessionStore) key(token string) string {
	return s.Prefix + token
}

func randomToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
