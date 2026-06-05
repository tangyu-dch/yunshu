package system

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// IPBlockLogModel 映射 IP 拦截日志表。
type IPBlockLogModel struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	IP          string    `gorm:"column:ip;type:varchar(50);not null;index"`
	CountryCode string    `gorm:"column:country_code;type:varchar(10);not null"`
	CallID      string    `gorm:"column:call_id;type:varchar(100)"`
	Method      string    `gorm:"column:method;type:varchar(20)"`
	BlockedAt   time.Time `gorm:"column:blocked_at;not null;index"`
}

// TableName 返回 cc_sys_ip_block_log 表名。
func (IPBlockLogModel) TableName() string {
	return "cc_sys_ip_block_log"
}

// GormIPBlockLogRepository 基于 GORM 的 IP 拦截日志仓储实现。
type GormIPBlockLogRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewGormIPBlockLogRepository 创建 GORM 拦截日志仓储。
func NewGormIPBlockLogRepository(db *gorm.DB, logger *slog.Logger) *GormIPBlockLogRepository {
	return &GormIPBlockLogRepository{DB: db, Logger: logger}
}

// Page 分页及按条件过滤查询 IP 拦截流水日志
func (r *GormIPBlockLogRepository) Page(ctx context.Context, req operate.IPBlockLogPageRequest) (operate.IPBlockLogPageResult, error) {
	var models []IPBlockLogModel
	var total int64

	query := r.DB.WithContext(ctx).Model(&IPBlockLogModel{})

	if req.IP != "" {
		query = query.Where("ip LIKE ?", "%"+req.IP+"%")
	}
	if req.CountryCode != "" {
		query = query.Where("country_code = ?", req.CountryCode)
	}
	if !req.StartTime.IsZero() {
		query = query.Where("blocked_at >= ?", req.StartTime)
	}
	if !req.EndTime.IsZero() {
		query = query.Where("blocked_at <= ?", req.EndTime)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return operate.IPBlockLogPageResult{}, err
	}

	// 排序与分页
	offset := (req.PageNumber - 1) * req.PageSize
	err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error
	if err != nil {
		return operate.IPBlockLogPageResult{}, err
	}

	records := make([]operate.IPBlockLog, 0, len(models))
	for _, m := range models {
		records = append(records, operate.IPBlockLog{
			ID:          m.ID,
			IP:          m.IP,
			CountryCode: m.CountryCode,
			CallID:      m.CallID,
			Method:      m.Method,
			BlockedAt:   m.BlockedAt,
		})
	}

	return operate.IPBlockLogPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    records,
	}, nil
}

// Save 保存拦截流水记录到数据库中
func (r *GormIPBlockLogRepository) Save(ctx context.Context, log operate.IPBlockLog) (operate.IPBlockLog, error) {
	model := IPBlockLogModel{
		IP:          log.IP,
		CountryCode: log.CountryCode,
		CallID:      log.CallID,
		Method:      log.Method,
		BlockedAt:   log.BlockedAt,
	}

	if err := r.DB.WithContext(ctx).Create(&model).Error; err != nil {
		return operate.IPBlockLog{}, err
	}

	return operate.IPBlockLog{
		ID:          model.ID,
		IP:          model.IP,
		CountryCode: model.CountryCode,
		CallID:      model.CallID,
		Method:      model.Method,
		BlockedAt:   model.BlockedAt,
	}, nil
}

// MemoryIPBlockLogRepository 基于内存的 IP 拦截日志仓储。
type MemoryIPBlockLogRepository struct {
	logs []operate.IPBlockLog
	mu   sync.RWMutex
}

// NewMemoryIPBlockLogRepository 创建内存拦截日志仓储。
func NewMemoryIPBlockLogRepository() *MemoryIPBlockLogRepository {
	return &MemoryIPBlockLogRepository{
		logs: make([]operate.IPBlockLog, 0),
	}
}

// Page 分页查询内存中的 IP 拦截日志
func (r *MemoryIPBlockLogRepository) Page(ctx context.Context, req operate.IPBlockLogPageRequest) (operate.IPBlockLogPageResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []operate.IPBlockLog
	for _, log := range r.logs {
		if req.IP != "" && !strings.Contains(log.IP, req.IP) {
			continue
		}
		if req.CountryCode != "" && log.CountryCode != req.CountryCode {
			continue
		}
		if !req.StartTime.IsZero() && log.BlockedAt.Before(req.StartTime) {
			continue
		}
		if !req.EndTime.IsZero() && log.BlockedAt.After(req.EndTime) {
			continue
		}
		filtered = append(filtered, log)
	}

	// 按 ID 降序（最新在前）
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	total := int64(len(filtered))
	offset := (req.PageNumber - 1) * req.PageSize
	if offset < 0 {
		offset = 0
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + req.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	records := filtered[offset:end]
	return operate.IPBlockLogPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    records,
	}, nil
}

// Save 保存拦截记录至内存
func (r *MemoryIPBlockLogRepository) Save(ctx context.Context, log operate.IPBlockLog) (operate.IPBlockLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.ID = int64(len(r.logs) + 1)
	if log.BlockedAt.IsZero() {
		log.BlockedAt = time.Now()
	}
	r.logs = append(r.logs, log)
	return log, nil
}
