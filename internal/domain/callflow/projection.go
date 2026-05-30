package callflow

import (
	"context"
	"fmt"
	"log/slog"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/outbox"
	"yunshu/internal/infra/events"
)

const (
	DestinationBatchTelProjection  = "cti_batch_tel_projection"
	DestinationBatchTaskProjection = "cti_batch_task_projection"
	DestinationBatchCallback       = "cti_batch_callback"
)

// RegisterProjectionConsumers 注册批量外呼投影/推送流程节点。
//
// WebSocket、客户回调、报表投影等最终消息推送不能在终结消费者里直接执行。
// 这里先把投影任务写入 outbox，后续 worker 可以按 destination 做可靠推送、重试和补偿。
func RegisterProjectionConsumers(bus events.Bus, store outbox.Store, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	if store == nil {
		logger.Warn("批量外呼投影消费者未注册，outbox 为空")
		return
	}
	bus.Subscribe(contracts.EventBatchCallTelCompleted, func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		taskID := fmt.Sprint(event.Payload["batchTaskId"])
		telID := fmt.Sprint(event.Payload["batchCallTelId"])
		entry := outbox.Entry{
			ID:             "batch-tel-projection:" + taskID + ":" + telID,
			AggregateType:  "batch_call_tel",
			AggregateID:    taskID + ":" + telID,
			Destination:    DestinationBatchTelProjection,
			IdempotencyKey: "batch-tel-projection:" + taskID + ":" + telID,
			Payload:        event.Payload,
			NextAttemptAt:  event.OccurredAt,
		}
		if err := store.Append(ctx, entry); err != nil {
			logger.Error("批量外呼号码完成投影任务写入失败", "eventId", event.EventID, "taskId", taskID, "telId", telID, "error", err.Error())
			return err
		}
		callbackEntry := outbox.Entry{
			ID:             "batch-tel-callback:" + taskID + ":" + telID,
			AggregateType:  "batch_call_tel",
			AggregateID:    taskID + ":" + telID,
			Destination:    DestinationBatchCallback,
			IdempotencyKey: "batch-tel-callback:" + taskID + ":" + telID,
			Payload:        callbackPayload("batch_tel_completed", event.Payload),
			NextAttemptAt:  event.OccurredAt,
		}
		if err := store.Append(ctx, callbackEntry); err != nil {
			logger.Error("批量外呼号码完成回调任务写入失败", "eventId", event.EventID, "taskId", taskID, "telId", telID, "error", err.Error())
			return err
		}
		logger.Info("批量外呼号码完成投影任务已写入", "eventId", event.EventID, "taskId", taskID, "telId", telID, "destination", entry.Destination)
		return nil
	})
	bus.Subscribe(contracts.EventBatchCallTaskCompleted, func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		taskID := fmt.Sprint(event.Payload["batchTaskId"])
		entry := outbox.Entry{
			ID:             "batch-task-projection:" + taskID,
			AggregateType:  "batch_call_task",
			AggregateID:    taskID,
			Destination:    DestinationBatchTaskProjection,
			IdempotencyKey: "batch-task-projection:" + taskID,
			Payload:        event.Payload,
			NextAttemptAt:  event.OccurredAt,
		}
		if err := store.Append(ctx, entry); err != nil {
			logger.Error("批量外呼任务完成投影任务写入失败", "eventId", event.EventID, "taskId", taskID, "error", err.Error())
			return err
		}
		callbackEntry := outbox.Entry{
			ID:             "batch-task-callback:" + taskID,
			AggregateType:  "batch_call_task",
			AggregateID:    taskID,
			Destination:    DestinationBatchCallback,
			IdempotencyKey: "batch-task-callback:" + taskID,
			Payload:        callbackPayload("batch_task_completed", event.Payload),
			NextAttemptAt:  event.OccurredAt,
		}
		if err := store.Append(ctx, callbackEntry); err != nil {
			logger.Error("批量外呼任务完成回调任务写入失败", "eventId", event.EventID, "taskId", taskID, "error", err.Error())
			return err
		}
		logger.Info("批量外呼任务完成投影任务已写入", "eventId", event.EventID, "taskId", taskID, "destination", entry.Destination)
		return nil
	})
}

func callbackPayload(eventType string, payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		out[key] = value
	}
	out["eventType"] = eventType
	return out
}
