package selection

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/domain/cti"
)

// RedisCandidateSource 通过 Redis 读穿缓存 CTI 选号候选号码。
//
// 命中缓存时直接返回，未命中时读取下层 Source 并写入 Redis。生产上应配合变更事件
// 或定时刷新投影，让缓存保持可用且可重建。
type RedisCandidateSource struct {
	Client *goredis.Client
	Source cti.CandidateSource
	TTL    time.Duration
	Logger *slog.Logger
}

// CandidatesForUser 先读 Redis，未命中则回源并回填缓存。
func (s *RedisCandidateSource) CandidatesForUser(ctx context.Context, userID int) ([]cti.NumberCandidate, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id")
	}
	if cached, ok, err := s.read(ctx, userID); err != nil {
		return nil, err
	} else if ok {
		return cached, nil
	}
	if s.Source == nil {
		return nil, fmt.Errorf("candidate source is nil")
	}
	candidates, err := s.Source.CandidatesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.write(ctx, userID, candidates); err != nil {
		s.logger().Warn("写入 CTI 选号候选缓存失败", "userId", userID, "error", err.Error())
	}
	return candidates, nil
}

func (s *RedisCandidateSource) read(ctx context.Context, userID int) ([]cti.NumberCandidate, bool, error) {
	if s.Client == nil {
		return nil, false, nil
	}
	raw, err := s.Client.Get(ctx, candidateCacheKey(userID)).Result()
	if err == goredis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var candidates []cti.NumberCandidate
	if err := json.Unmarshal([]byte(raw), &candidates); err != nil {
		return nil, false, err
	}
	return candidates, true, nil
}

func (s *RedisCandidateSource) write(ctx context.Context, userID int, candidates []cti.NumberCandidate) error {
	if s.Client == nil {
		return nil
	}
	ttl := s.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	raw, err := json.Marshal(candidates)
	if err != nil {
		return err
	}
	return s.Client.Set(ctx, candidateCacheKey(userID), raw, ttl).Err()
}

func (s *RedisCandidateSource) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func candidateCacheKey(userID int) string {
	return fmt.Sprintf("cti:phone_resource:user:%d", userID)
}
