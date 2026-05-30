package callflow

import (
	"context"
	"log/slog"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/outbox"
	"yunshu/internal/infra/events"
)

// OutboxHandler 处理一个 outbox destination 的实际投递。
type OutboxHandler func(context.Context, outbox.Entry) error

// OutboxDispatcher 扫描 outbox 并按 destination 投递。
//
// worker 进程可以周期性调用 DispatchOnce；每个 handler 内部再执行 WebSocket、HTTP
// 回调、报表投影或 MQ 发布。成功后才标记 published，失败则设置下一次重试时间。
type OutboxDispatcher struct {
	Store      outbox.Store
	Handlers   map[string]OutboxHandler
	WorkerID   string
	RetryDelay time.Duration
	Lease      time.Duration
	BatchSize  int
	Now        func() time.Time
	Events     events.Bus
	Logger     *slog.Logger
}

// DispatchOnce 执行一次 outbox 扫描和投递。
func (d *OutboxDispatcher) DispatchOnce(ctx context.Context) (int, error) {
	logger := d.logger()
	if d.Store == nil {
		logger.Warn("outbox 投递器缺少存储，跳过本轮扫描")
		return 0, nil
	}
	now := d.now()
	entries, err := d.claim(ctx, now)
	if err != nil {
		logger.Error("outbox 投递器扫描失败", "error", err.Error())
		return 0, err
	}
	dispatched := 0
	for _, entry := range entries {
		handler := d.Handlers[entry.Destination]
		if handler == nil {
			logger.Warn("outbox 未找到 destination handler，等待后续重试或人工修复", "outboxId", entry.ID, "destination", entry.Destination)
			if err := d.Store.MarkFailed(ctx, entry.ID, now.Add(d.retryDelay())); err != nil {
				return dispatched, err
			}
			continue
		}
		logger.Info("开始投递 outbox", "outboxId", entry.ID, "destination", entry.Destination, "aggregateType", entry.AggregateType, "aggregateId", entry.AggregateID)
		if err := handler(ctx, entry); err != nil {
			logger.Error("outbox 投递失败，等待重试", "outboxId", entry.ID, "destination", entry.Destination, "error", err.Error())
			if markErr := d.Store.MarkFailed(ctx, entry.ID, now.Add(d.retryDelay())); markErr != nil {
				return dispatched, markErr
			}
			continue
		}
		if err := d.Store.MarkPublished(ctx, entry.ID); err != nil {
			return dispatched, err
		}
		d.publishCompletionEvent(ctx, entry)
		dispatched++
		logger.Info("outbox 投递完成", "outboxId", entry.ID, "destination", entry.Destination)
	}
	return dispatched, nil
}

func (d *OutboxDispatcher) publishCompletionEvent(ctx context.Context, entry outbox.Entry) {
	if d.Events == nil {
		return
	}
	var eventType string
	switch entry.Destination {
	case "call_center_cdr_queue":
		eventType = "cdr_persisted"
	case "cti_billing_settlement":
		eventType = "billing_completed"
	case "cti_cdr_recording":
		eventType = "recording_completed"
	case "cti_cdr_downstream_push":
		eventType = "push_completed"
	case "cti_batch_callback":
		eventType = "callback_completed"
	}
	if eventType == "" {
		return
	}

	payload := make(map[string]any, len(entry.Payload)+1)
	for k, v := range entry.Payload {
		payload[k] = v
	}
	payload["outboxId"] = entry.ID

	evt := contracts.NewEventEnvelope(
		"evt-"+eventType+"-"+entry.AggregateID,
		eventType,
		"idem-"+eventType+"-"+entry.AggregateID,
		entry.AggregateType,
		entry.AggregateID,
		contracts.ServiceWorker,
		payload,
	)
	if err := d.Events.Publish(ctx, evt); err != nil {
		d.logger().Error("outbox 投递成功后发布流程事件失败", "eventType", eventType, "callId", entry.AggregateID, "error", err.Error())
	} else {
		d.logger().Info("outbox 投递成功后已发布流程事件", "eventType", eventType, "callId", entry.AggregateID)
	}
}

func (d *OutboxDispatcher) claim(ctx context.Context, now time.Time) ([]outbox.Entry, error) {
	if store, ok := d.Store.(outbox.LeaseStore); ok {
		return store.ClaimDue(ctx, d.workerID(), d.batchSize(), now, d.lease())
	}
	return d.Store.Pending(ctx, d.batchSize(), now)
}

func (d *OutboxDispatcher) logger() *slog.Logger {
	if d.Logger != nil {
		return d.Logger
	}
	return slog.Default()
}

func (d *OutboxDispatcher) now() time.Time {
	if d.Now != nil {
		return d.Now().UTC()
	}
	return time.Now().UTC()
}

func (d *OutboxDispatcher) batchSize() int {
	if d.BatchSize > 0 {
		return d.BatchSize
	}
	return 100
}

func (d *OutboxDispatcher) workerID() string {
	if d.WorkerID != "" {
		return d.WorkerID
	}
	return "cc-worker-local"
}

func (d *OutboxDispatcher) lease() time.Duration {
	if d.Lease > 0 {
		return d.Lease
	}
	return 30 * time.Second
}

func (d *OutboxDispatcher) retryDelay() time.Duration {
	if d.RetryDelay > 0 {
		return d.RetryDelay
	}
	return time.Minute
}
