// Package events 提供进程内事件总线和消费者框架。
//
// 这是 RabbitMQ/Redis Stream 接入前的正式业务抽象：HTTP、定时任务和外部 adapter
// 都发布 contracts.EventEnvelope，消费者按 eventType 处理并推进 workflow。
package events

import (
	"context"
	"log/slog"
	"sync"

	"yunshu/internal/contracts"
)

// Handler 是事件消费者函数类型。
// 接收上下文和事件信封，返回 nil 表示消费成功可以安全确认；返回错误表示处理失败应触发重试。
type Handler func(context.Context, contracts.EventEnvelope[map[string]any]) error

// Bus 定义事件发布和订阅能力接口。
// 发布者通过 Publish 发布事件，消费者通过 Subscribe 注册指定事件类型的处理器。
type Bus interface {
	Publish(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error
	Subscribe(eventType string, handler Handler)
}

// MemoryBus 是进程内同步事件总线。
// 它用于本地开发和单元测试；生产环境应替换为 MQ/Stream 消费者，但保持 Handler 语义不变。
type MemoryBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	logger   *slog.Logger
}

// NewMemoryBus 创建内存事件总线。
func NewMemoryBus(logger *slog.Logger) *MemoryBus {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemoryBus{handlers: map[string][]Handler{}, logger: logger}
}

// Subscribe 注册指定事件类型的消费者。
func (b *MemoryBus) Subscribe(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
	b.logger.Info("注册事件消费者", "eventType", eventType, "handlerCount", len(b.handlers[eventType]))
}

// Publish 发布事件并同步执行消费者。
// 如果任一消费者失败，调用方会收到错误，未来 MQ adapter 应据此决定不 ack 并触发重试。
func (b *MemoryBus) Publish(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
	b.mu.RLock()
	handlers := append([]Handler(nil), b.handlers[event.EventType]...)
	b.mu.RUnlock()
	b.logger.Info("发布领域事件", "eventType", event.EventType, "eventId", event.EventID, "aggregateId", event.AggregateID, "handlerCount", len(handlers))
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			b.logger.Error("领域事件消费失败", "eventType", event.EventType, "eventId", event.EventID, "aggregateId", event.AggregateID, "error", err.Error())
			return err
		}
	}
	b.logger.Info("领域事件消费完成", "eventType", event.EventType, "eventId", event.EventID, "aggregateId", event.AggregateID)
	return nil
}
