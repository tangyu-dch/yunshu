package telephony

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// RtpengineModel 映射 Kamailio rtpengine 媒体代理表。
type RtpengineModel struct {
	ID            int       `gorm:"column:id;primaryKey"`
	SetID         int       `gorm:"column:set_id"`
	RtpengineSock string    `gorm:"column:rtpengine_sock"`
	Disabled      bool      `gorm:"column:disabled"`
	Weight        int       `gorm:"column:weight"`
	Description   string    `gorm:"column:description"`
	DelFlag       bool      `gorm:"column:del_flag"`
	CreatedTime   time.Time `gorm:"column:created_time"`
	UpdatedTime   time.Time `gorm:"column:updated_time"`
}

// TableName 返回 rtpengine 表名。
func (RtpengineModel) TableName() string {
	return "cc_tel_rtpengine"
}

// RtpengineRepository 基于 GORM 的 rtpengine 仓储实现。
type RtpengineRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewRtpengineRepository 创建 GORM rtpengine 仓储。
func NewRtpengineRepository(db *gorm.DB, logger *slog.Logger) *RtpengineRepository {
	return &RtpengineRepository{DB: db, Logger: logger}
}

// Page 分页查询未删除 rtpengine 节点。
func (r *RtpengineRepository) Page(ctx context.Context, req operate.RtpenginePageRequest) (operate.RtpenginePageResult, error) {
	query := r.DB.WithContext(ctx).Model(&RtpengineModel{}).Where("del_flag = ?", false)
	if req.SetID > 0 {
		query = query.Where("set_id = ?", req.SetID)
	}
	if req.RtpengineSock != "" {
		query = query.Where("rtpengine_sock LIKE ?", "%"+req.RtpengineSock+"%")
	}
	if req.Disabled != nil {
		query = query.Where("disabled = ?", *req.Disabled)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.RtpenginePageResult{}, err
	}

	var models []RtpengineModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("set_id ASC, weight DESC, id ASC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.RtpenginePageResult{}, err
	}

	records := make([]operate.Rtpengine, 0, len(models))
	for _, model := range models {
		records = append(records, rtpengineFromModel(model))
	}

	return operate.RtpenginePageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    records,
	}, nil
}

// GetByID 读取 rtpengine 节点。
func (r *RtpengineRepository) GetByID(ctx context.Context, id int) (operate.Rtpengine, error) {
	var model RtpengineModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Rtpengine{}, operate.ErrRtpengineNotFound
	}
	return rtpengineFromModel(model), err
}

// ExistsSock 校验 rtpengine_sock 唯一性。
func (r *RtpengineRepository) ExistsSock(ctx context.Context, rtpengineSock string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&RtpengineModel{}).
		Where("del_flag = ? AND rtpengine_sock = ?", false, rtpengineSock)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新 rtpengine 节点。
func (r *RtpengineRepository) Save(ctx context.Context, engine operate.Rtpengine) (operate.Rtpengine, error) {
	r.logger().Info("开始保存 Kamailio RTPEngine 配置", "rtpengineSock", engine.RtpengineSock, "description", engine.Description)
	model := rtpengineToModel(engine)
	now := time.Now().UTC()
	model.UpdatedTime = now
	if model.ID == 0 {
		model.CreatedTime = now
	}
	tx := r.DB.WithContext(ctx)
	if model.ID != 0 {
		tx = tx.Omit("created_time")
	}
	if err := tx.Save(&model).Error; err != nil {
		r.logger().Error("保存 Kamailio RTPEngine 配置失败", "rtpengineSock", engine.RtpengineSock, "error", err.Error())
		return operate.Rtpengine{}, err
	}
	r.logger().Info("保存 Kamailio RTPEngine 配置成功", "id", model.ID, "rtpengineSock", model.RtpengineSock)
	return rtpengineFromModel(model), nil
}

// Delete 逻辑删除 rtpengine 节点。
func (r *RtpengineRepository) Delete(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	r.logger().Info("开始批量逻辑删除 Kamailio RTPEngine 记录", "ids", ids)
	result := r.DB.WithContext(ctx).Model(&RtpengineModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"del_flag": true, "disabled": true, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("批量逻辑删除 Kamailio RTPEngine 记录失败", "ids", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("批量逻辑删除 Kamailio RTPEngine 记录未匹配到有效记录", "ids", ids)
		return operate.ErrRtpengineNotFound
	}
	r.logger().Info("批量逻辑删除 Kamailio RTPEngine 记录成功", "ids", ids, "rowsAffected", result.RowsAffected)
	return nil
}

func rtpengineToModel(engine operate.Rtpengine) RtpengineModel {
	return RtpengineModel{
		ID:            engine.ID,
		SetID:         engine.SetID,
		RtpengineSock: engine.RtpengineSock,
		Disabled:      engine.Disabled,
		Weight:        engine.Weight,
		Description:   engine.Description,
		DelFlag:       false,
	}
}

func rtpengineFromModel(model RtpengineModel) operate.Rtpengine {
	return operate.Rtpengine{
		ID:            model.ID,
		SetID:         model.SetID,
		RtpengineSock: model.RtpengineSock,
		Disabled:      model.Disabled,
		Weight:        model.Weight,
		Description:   model.Description,
	}
}

func (r *RtpengineRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

// MemoryRtpengineRepository 供本地开发和测试使用。
type MemoryRtpengineRepository struct {
	mu      sync.Mutex
	nextID  int
	engines map[int]operate.Rtpengine
}

// NewMemoryRtpengineRepository 创建 rtpengine 内存仓储。
func NewMemoryRtpengineRepository() *MemoryRtpengineRepository {
	return &MemoryRtpengineRepository{nextID: 1, engines: map[int]operate.Rtpengine{}}
}

func (r *MemoryRtpengineRepository) Page(_ context.Context, req operate.RtpenginePageRequest) (operate.RtpenginePageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Rtpengine, 0, len(r.engines))
	for _, engine := range r.engines {
		if req.SetID > 0 && engine.SetID != req.SetID {
			continue
		}
		if req.RtpengineSock != "" && !strings.Contains(engine.RtpengineSock, req.RtpengineSock) {
			continue
		}
		if req.Disabled != nil && engine.Disabled != *req.Disabled {
			continue
		}
		records = append(records, engine)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Rtpengine{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.RtpenginePageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryRtpengineRepository) GetByID(_ context.Context, id int) (operate.Rtpengine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	engine, ok := r.engines[id]
	if !ok {
		return operate.Rtpengine{}, operate.ErrRtpengineNotFound
	}
	return engine, nil
}

func (r *MemoryRtpengineRepository) ExistsSock(_ context.Context, rtpengineSock string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, engine := range r.engines {
		if id == excludeID {
			continue
		}
		if engine.RtpengineSock == rtpengineSock {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryRtpengineRepository) Save(_ context.Context, engine operate.Rtpengine) (operate.Rtpengine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if engine.ID == 0 {
		engine.ID = r.nextID
		r.nextID++
	}
	r.engines[engine.ID] = engine
	return engine, nil
}

func (r *MemoryRtpengineRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for _, id := range ids {
		if _, ok := r.engines[id]; ok {
			delete(r.engines, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrRtpengineNotFound
	}
	return nil
}
