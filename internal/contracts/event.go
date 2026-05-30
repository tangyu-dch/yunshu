package contracts

import "time"

// EventEnvelope 是跨服务事件的稳定信封。
//
// 领域事件、控制命令、流程事件和补偿事件都应使用这个外壳；Payload 可以按版本演进，
// 但信封字段必须保持向后兼容，保证排障、幂等、回放和灰度期间都能识别同一条业务事实。
type EventEnvelope[T any] struct {
	EventID        string            `json:"eventId"`
	EventType      string            `json:"eventType"`
	EventVersion   int               `json:"eventVersion"`
	SourceService  ServiceName       `json:"sourceService"`
	TraceID        string            `json:"traceId,omitempty"`
	RequestID      string            `json:"requestId,omitempty"`
	IdempotencyKey string            `json:"idempotencyKey"`
	AggregateType  string            `json:"aggregateType"`
	AggregateID    string            `json:"aggregateId"`
	OccurredAt     time.Time         `json:"occurredAt"`
	Headers        map[string]string `json:"headers,omitempty"`
	Payload        T                 `json:"payload"`
}

// NewEventEnvelope 创建默认版本为 1 的事件信封。
// 事件创建时间统一使用 UTC，避免多服务部署时出现时区歧义。
func NewEventEnvelope[T any](eventID, eventType, idempotencyKey, aggregateType, aggregateID string, source ServiceName, payload T) EventEnvelope[T] {
	return EventEnvelope[T]{
		EventID:        eventID,
		EventType:      eventType,
		EventVersion:   1,
		SourceService:  source,
		IdempotencyKey: idempotencyKey,
		AggregateType:  aggregateType,
		AggregateID:    aggregateID,
		OccurredAt:     time.Now().UTC(),
		Headers:        map[string]string{},
		Payload:        payload,
	}
}
