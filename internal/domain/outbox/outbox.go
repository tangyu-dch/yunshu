package outbox

import (
	"context"
	"time"
)

type Status string

const (
	Pending    Status = "pending"
	Processing Status = "processing"
	Published  Status = "published"
	Failed     Status = "failed"
)

type Entry struct {
	ID             string         `json:"id"`
	AggregateType  string         `json:"aggregateType"`
	AggregateID    string         `json:"aggregateId"`
	Destination    string         `json:"destination"`
	IdempotencyKey string         `json:"idempotencyKey"`
	Payload        map[string]any `json:"payload"`
	Status         Status         `json:"status"`
	Attempts       int            `json:"attempts"`
	NextAttemptAt  time.Time      `json:"nextAttemptAt"`
	LockedBy       string         `json:"lockedBy"`
	LockedUntil    time.Time      `json:"lockedUntil"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type Store interface {
	Append(ctx context.Context, entry Entry) error
	MarkPublished(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, nextAttemptAt time.Time) error
	Pending(ctx context.Context, limit int, now time.Time) ([]Entry, error)
}

// LeaseStore 是支持多 worker 领取租约的 outbox 存储接口。
type LeaseStore interface {
	Store
	ClaimDue(ctx context.Context, workerID string, limit int, now time.Time, lease time.Duration) ([]Entry, error)
}
