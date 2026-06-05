package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"yunshu/internal/contracts"
)

type rateLimiter struct {
	mu         sync.Mutex
	lastRefill time.Time
	tokens     float64
	capacity   float64
	refillRate float64 // 每秒生成的令牌数
}

func newRateLimiter(capacity float64, refillRate float64) *rateLimiter {
	return &rateLimiter{
		lastRefill: time.Now(),
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
	}
}

func (l *rateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.lastRefill = now
	l.tokens += elapsed * l.refillRate
	if l.tokens > l.capacity {
		l.tokens = l.capacity
	}
	if l.tokens >= 1.0 {
		l.tokens -= 1.0
		return true
	}
	return false
}

// ipLimiters 按 IP 隔离的令牌桶限制器集合
type ipLimiters struct {
	mu         sync.Mutex
	limiters   map[string]*rateLimiter
	capacity   float64
	refillRate float64
}

func newIPLimiters(capacity float64, refillRate float64) *ipLimiters {
	return &ipLimiters{
		limiters:   make(map[string]*rateLimiter),
		capacity:   capacity,
		refillRate: refillRate,
	}
}

func (i *ipLimiters) allow(ip string) bool {
	i.mu.Lock()
	lim, exists := i.limiters[ip]
	if !exists {
		lim = newRateLimiter(i.capacity, i.refillRate)
		i.limiters[ip] = lim
	}
	i.mu.Unlock()
	return lim.allow()
}

// RateLimitMiddleware 基于 IP 的速率限制拦截器。
// - capacity: 最大并发/突发容量
// - refillRate: 每秒恢复的令牌速率（如 0.2 表示每 5 秒生成一个令牌，相当于每 5 秒只允许 1 次访问）
func RateLimitMiddleware(capacity float64, refillRate float64) gin.HandlerFunc {
	lim := newIPLimiters(capacity, refillRate)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !lim.allow(ip) {
			c.JSON(http.StatusTooManyRequests, contracts.Fail(429, "请求过于频繁，请稍后再试"))
			c.Abort()
			return
		}
		c.Next()
	}
}
