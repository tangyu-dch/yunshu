package business

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JSONMap 是 outbox payload 的 GORM JSON 字段封装。
type JSONMap map[string]any

// Value 将 payload 序列化为数据库 JSON 字符串。
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(map[string]any(m))
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

// Scan 从数据库 JSON 字段反序列化 payload。
func (m *JSONMap) Scan(value any) error {
	if m == nil {
		return errors.New("nil JSONMap receiver")
	}
	switch typed := value.(type) {
	case nil:
		*m = JSONMap{}
		return nil
	case []byte:
		return json.Unmarshal(typed, m)
	case string:
		return json.Unmarshal([]byte(typed), m)
	default:
		return fmt.Errorf("unsupported JSONMap value %T", value)
	}
}

// MessageOutboxModel 映射 Go 生产 outbox 表。
//
// 这是 Go 原生可靠投递表，不直接复用  队列表。 兼容事件仍通过 payload 和
// destination 保持，后续可由 worker 投递到 RabbitMQ、Redis Stream、WebSocket 或回调。
type MessageOutboxModel struct {
	ID             string     `gorm:"column:id;primaryKey;size:128"`
	AggregateType  string     `gorm:"column:aggregate_type;size:64;index:idx_message_outbox_aggregate"`
	AggregateID    string     `gorm:"column:aggregate_id;size:128;index:idx_message_outbox_aggregate"`
	Destination    string     `gorm:"column:destination;size:128;index:idx_message_outbox_pending"`
	IdempotencyKey string     `gorm:"column:idempotency_key;size:160;uniqueIndex"`
	Payload        JSONMap    `gorm:"column:payload;type:json"`
	Status         Status     `gorm:"column:status;size:24;index:idx_message_outbox_pending"`
	Attempts       int        `gorm:"column:attempts"`
	NextAttemptAt  time.Time  `gorm:"column:next_attempt_at;index:idx_message_outbox_pending"`
	LockedBy       string     `gorm:"column:locked_by;size:128;index:idx_message_outbox_lease"`
	LockedUntil    *time.Time `gorm:"column:locked_until;index:idx_message_outbox_lease"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

// TableName 返回 Go 原生 outbox 表名。
func (MessageOutboxModel) TableName() string {
	return "cc_biz_outbox"
}

// GormStore 使用数据库持久化 outbox，支持多实例 worker 扫描和失败重试。
type OutboxGormStore struct {
	DB     *gorm.DB
	Now    func() time.Time
	Logger *slog.Logger
}

// NewGormStore 创建 GORM outbox 存储。
func NewOutboxGormStore(db *gorm.DB, logger *slog.Logger) *OutboxGormStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &OutboxGormStore{DB: db, Now: time.Now, Logger: logger}
}

// Append 幂等写入 outbox 记录。
func (s *OutboxGormStore) Append(ctx context.Context, entry Entry) error {
	now := s.now()
	if entry.Status == "" {
		entry.Status = Pending
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = now
	}
	if entry.NextAttemptAt.IsZero() {
		entry.NextAttemptAt = now
	}
	model := modelFromEntry(entry)
	err := s.DB.WithContext(ctx).Create(&model).Error
	if err != nil {
		if isDuplicateError(err) {
			s.Logger.Info("outbox 记录已存在，按幂等写入处理", "outboxId", entry.ID, "destination", entry.Destination)
			return ErrDuplicateEntry
		}
		s.Logger.Error("outbox 记录写入失败", "outboxId", entry.ID, "destination", entry.Destination, "error", err.Error())
		return err
	}
	s.Logger.Info("outbox 记录写入成功", "outboxId", entry.ID, "destination", entry.Destination, "aggregateType", entry.AggregateType, "aggregateId", entry.AggregateID)
	return nil
}

// MarkPublished 标记 outbox 记录投递成功。
func (s *OutboxGormStore) MarkPublished(ctx context.Context, id string) error {
	result := s.DB.WithContext(ctx).Model(&MessageOutboxModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": Published, "locked_by": "", "locked_until": nil, "updated_at": s.now()})
	if result.Error != nil {
		s.Logger.Error("outbox 标记投递成功失败", "outboxId", id, "error", result.Error.Error())
		return result.Error
	}
	s.Logger.Info("outbox 已标记投递成功", "outboxId", id, "rowsAffected", result.RowsAffected)
	return nil
}

// MarkFailed 标记 outbox 记录投递失败，并设置下次重试时间。
func (s *OutboxGormStore) MarkFailed(ctx context.Context, id string, nextAttemptAt time.Time) error {
	result := s.DB.WithContext(ctx).Model(&MessageOutboxModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": Failed, "attempts": gorm.Expr("attempts + ?", 1), "next_attempt_at": nextAttemptAt.UTC(), "locked_by": "", "locked_until": nil, "updated_at": s.now()})
	if result.Error != nil {
		s.Logger.Error("outbox 标记投递失败失败", "outboxId", id, "error", result.Error.Error())
		return result.Error
	}
	s.Logger.Warn("outbox 已标记投递失败，等待重试", "outboxId", id, "nextAttemptAt", nextAttemptAt.UTC(), "rowsAffected", result.RowsAffected)
	return nil
}

// Pending 查询待投递或到期重试的 outbox 记录。
func (s *OutboxGormStore) Pending(ctx context.Context, limit int, now time.Time) ([]Entry, error) {
	if limit <= 0 {
		limit = 100
	}
	var models []MessageOutboxModel
	err := s.DB.WithContext(ctx).
		Where("status = ? OR (status = ? AND next_attempt_at <= ?)", Pending, Failed, now.UTC()).
		Order("next_attempt_at ASC, created_at ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		s.Logger.Error("outbox 待投递记录查询失败", "limit", limit, "error", err.Error())
		return nil, err
	}
	entries := make([]Entry, 0, len(models))
	for _, model := range models {
		entries = append(entries, entryFromModel(model))
	}
	s.Logger.Info("outbox 待投递记录查询完成", "limit", limit, "count", len(entries))
	return entries, nil
}

// ClaimDue 领取到期 outbox 记录并写入 worker 租约。
//
// MySQL 8 生产环境会生成 `FOR UPDATE SKIP LOCKED`，多个 cc-worker 并发扫描时不会领取到
// 同一批记录。processing 记录的租约过期后允许再次领取，用于恢复 worker 崩溃导致的悬挂
// 投递。SQLite 或不支持 SKIP LOCKED 的测试方言仍通过事务保持单进程语义。
func (s *OutboxGormStore) ClaimDue(ctx context.Context, workerID string, limit int, now time.Time, lease time.Duration) ([]Entry, error) {
	if limit <= 0 {
		limit = 100
	}
	if lease <= 0 {
		lease = time.Minute
	}
	now = now.UTC()
	lockedUntil := now.Add(lease).UTC()
	var claimed []MessageOutboxModel
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var models []MessageOutboxModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? OR (status = ? AND next_attempt_at <= ?) OR (status = ? AND locked_until <= ?)", Pending, Failed, now, Processing, now).
			Order("next_attempt_at ASC, created_at ASC").
			Limit(limit).
			Find(&models).Error; err != nil {
			return err
		}
		if len(models) == 0 {
			return nil
		}
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		if err := tx.Model(&MessageOutboxModel{}).
			Where("id IN ?", ids).
			Updates(map[string]any{"status": Processing, "locked_by": workerID, "locked_until": lockedUntil, "updated_at": s.now()}).Error; err != nil {
			return err
		}
		if err := tx.Where("id IN ?", ids).Order("next_attempt_at ASC, created_at ASC").Find(&claimed).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		s.Logger.Error("outbox 领取租约失败", "workerId", workerID, "limit", limit, "error", err.Error())
		return nil, err
	}
	entries := make([]Entry, 0, len(claimed))
	for _, model := range claimed {
		entries = append(entries, entryFromModel(model))
	}
	s.Logger.Info("outbox 领取租约完成", "workerId", workerID, "limit", limit, "count", len(entries), "leaseUntil", lockedUntil)
	return entries, nil
}

func (s *OutboxGormStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func modelFromEntry(entry Entry) MessageOutboxModel {
	return MessageOutboxModel{
		ID:             entry.ID,
		AggregateType:  entry.AggregateType,
		AggregateID:    entry.AggregateID,
		Destination:    entry.Destination,
		IdempotencyKey: entry.IdempotencyKey,
		Payload:        JSONMap(entry.Payload),
		Status:         entry.Status,
		Attempts:       entry.Attempts,
		NextAttemptAt:  entry.NextAttemptAt.UTC(),
		LockedBy:       entry.LockedBy,
		LockedUntil:    timePtr(entry.LockedUntil),
		CreatedAt:      entry.CreatedAt.UTC(),
		UpdatedAt:      entry.UpdatedAt.UTC(),
	}
}

func entryFromModel(model MessageOutboxModel) Entry {
	return Entry{
		ID:             model.ID,
		AggregateType:  model.AggregateType,
		AggregateID:    model.AggregateID,
		Destination:    model.Destination,
		IdempotencyKey: model.IdempotencyKey,
		Payload:        map[string]any(model.Payload),
		Status:         model.Status,
		Attempts:       model.Attempts,
		NextAttemptAt:  model.NextAttemptAt,
		LockedBy:       model.LockedBy,
		LockedUntil:    timeValue(model.LockedUntil),
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}

func isDuplicateError(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey)
}
