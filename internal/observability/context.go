package observability

// observability 包负责分布式追踪、可观测性及全局请求标识的注入与提取。
// 当前文件提供了 HTTP 通信上下文（net/http）的统一封装，
// 从请求 Header 中抽取关键呼叫及租户标识写入 Go Context 中，以打通全链路异步调用日志关联。

import (
	"context"
	"net/http"
)

// ContextKey 为可观测性上下文内部存储键的专有字符串类型，防止与其他包中注入的 Context Key 命名冲突。
type ContextKey string

const (
	RequestIDKey  ContextKey = "request_id"  // 全局一次 API 调用的唯一请求标识（RequestId）
	TraceIDKey    ContextKey = "trace_id"    // 分布式链路追踪标识（TraceId）
	MerchantIDKey ContextKey = "merchant_id" // 租户/商户标识（MerchantId）
	UserIDKey     ContextKey = "user_id"     // 用户唯一标识（UserId）
	CallIDKey     ContextKey = "call_id"     // 话务唯一标识，用于在 cc-call、cc-worker 和 ESL 中串联同一呼叫（CallId）
)

// WithRequestContext 提取传入 HTTP 请求 Header 中的通信追踪元数据，并注入到返回的 Context 中。
// 满足多实例和分布式高并发追踪需求。
func WithRequestContext(ctx context.Context, r *http.Request) context.Context {
	ctx = withHeader(ctx, RequestIDKey, r, "X-Request-Id")
	ctx = withHeader(ctx, TraceIDKey, r, "X-Trace-Id")
	ctx = withHeader(ctx, MerchantIDKey, r, "X-Merchant-Id")
	ctx = withHeader(ctx, UserIDKey, r, "X-User-Id")
	ctx = withHeader(ctx, CallIDKey, r, "X-Call-Id")
	return ctx
}

// Value 从 Context 中安全提取可观测性变量值，若不存在则返回空字符串。
func Value(ctx context.Context, key ContextKey) string {
	v, _ := ctx.Value(key).(string)
	return v
}

// withHeader 辅助函数：如果 HTTP 请求包含特定请求头，将其注入 Context 中；否则保持原 Context 不变。
func withHeader(ctx context.Context, key ContextKey, r *http.Request, header string) context.Context {
	if value := r.Header.Get(header); value != "" {
		return context.WithValue(ctx, key, value)
	}
	return ctx
}
