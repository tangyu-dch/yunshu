package selection

import (
	"context"
	"log/slog"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/domain/cti"
	"yunshu/internal/domain/esl"
)

// ClaimWatchdog 看门狗守护协程，周期性地扫描所有活跃的选号占用幂等 Key，
// 对于仍在通话中的活跃会话自动续期其 TTL 租约，防止长通话（超过默认 30 分钟）
// 导致并发计数器过期后被错误地重新占用，引发网关物理并发超限。
//
// 续期策略：
//  1. SCAN 扫描 cti:select:claim:* 键，提取 callId；
//  2. 从幂等值 "phone|gwID|merchantID" 中提取重建 counter Key 所需的信息；
//  3. 校验 SessionStore 中该 callId 是否仍为活跃会话（未进入 complete 状态）；
//  4. 若活跃则调用 RenewClaim 延长 claim/counter/gwCounter 的 TTL。
type ClaimWatchdog struct {
	Client       *goredis.Client
	Allocator    *RedisAllocator
	SessionStore esl.SessionStore
	Logger       *slog.Logger

	// Interval 扫描间隔，默认 5 分钟（TTL 30 分钟的 1/6，留足安全裕量）
	Interval time.Duration
}

// Start 启动看门狗守护协程，在 ctx 取消时自动退出。
func (w *ClaimWatchdog) Start(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	w.logger().Info("选号租约看门狗守护协程已启动", "interval", interval.String())
	for {
		select {
		case <-ctx.Done():
			w.logger().Info("选号租约看门狗守护协程已退出")
			return
		case <-ticker.C:
			renewed, skipped, errors := w.renewActiveClaims(ctx)
			if renewed > 0 || errors > 0 {
				w.logger().Info("选号租约看门狗续期完成", "renewed", renewed, "skipped", skipped, "errors", errors)
			} else {
				w.logger().Debug("选号租约看门狗续期完成", "renewed", renewed, "skipped", skipped, "errors", errors)
			}
		}
	}
}

// renewActiveClaims 扫描所有活跃 claim Key 并续期。
// 返回续期成功数、跳过数（会话已不存在或已过期）和错误数。
func (w *ClaimWatchdog) renewActiveClaims(ctx context.Context) (renewed, skipped, errors int) {
	if w.Client == nil || w.Allocator == nil {
		return
	}

	var cursor uint64
	for {
		keys, next, err := w.Client.Scan(ctx, cursor, "cti:select:claim:*", 100).Result()
		if err != nil {
			w.logger().Error("看门狗 SCAN 扫描 claim Key 失败", "error", err.Error())
			errors++
			return
		}

		for _, key := range keys {
			callID := extractCallID(key)
			if callID == "" {
				skipped++
				continue
			}

			// 读取幂等值 "phone|gwID|merchantID"
			raw, err := w.Client.Get(ctx, key).Result()
			if err != nil {
				errors++
				continue
			}

			phone, gwID, merchantID := parseClaimValue(raw)
			if phone == "" || gwID == "" || merchantID == "" {
				// 旧格式或缺少必要信息，无法续期
				skipped++
				continue
			}

			// 校验该 callId 是否仍为活跃会话
			if !w.isSessionActive(ctx, callID) {
				skipped++
				continue
			}

			// 执行续期
			allocation := cti.RuntimeAllocation{
				CallID:     callID,
				MerchantID: merchantID,
				Caller:     phone,
				GatewayID:  gwID,
				ClaimKey:   key,
			}
			ok, err := w.Allocator.RenewClaim(ctx, allocation)
			if err != nil {
				w.logger().Warn("看门狗续期失败", "callId", callID, "phone", phone, "error", err.Error())
				errors++
				continue
			}
			if ok {
				renewed++
			} else {
				skipped++
			}
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}
	return
}

// isSessionActive 判断指定 callId 的会话是否仍活跃（未进入 complete 状态）。
func (w *ClaimWatchdog) isSessionActive(ctx context.Context, callID string) bool {
	if w.SessionStore == nil {
		// 无 SessionStore 时保守续期，宁可多续不误释放
		return true
	}
	session, err := w.SessionStore.Get(ctx, callID)
	if err != nil {
		// 查询失败时保守续期
		return true
	}
	// session 为零值（未找到）时不续期
	if session.CallID == "" {
		return false
	}
	// 会话存在且未进入 complete 状态 → 续期
	return session.State != "complete"
}

// extractCallID 从 "cti:select:claim:{callID}" 中提取 callID。
func extractCallID(key string) string {
	const prefix = "cti:select:claim:"
	if !strings.HasPrefix(key, prefix) {
		return ""
	}
	return key[len(prefix):]
}

func (w *ClaimWatchdog) logger() *slog.Logger {
	if w.Logger != nil {
		return w.Logger
	}
	return slog.Default()
}
