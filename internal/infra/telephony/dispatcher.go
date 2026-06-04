package telephony

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// DispatcherModel 映射 Kamailio dispatcher 信令网关探测表。
type DispatcherModel struct {
	ID          int       `gorm:"column:id;primaryKey"`
	SetID       int       `gorm:"column:set_id"`
	Destination string    `gorm:"column:destination"`
	Flags       int       `gorm:"column:flags"`
	Priority    int       `gorm:"column:priority"`
	Attrs       string    `gorm:"column:attrs"`
	Description string    `gorm:"column:description"`
	Enable      bool      `gorm:"column:enable"`
	DelFlag     bool      `gorm:"column:del_flag"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

// TableName 返回 cc_res_freeswitch 物理表名。
func (DispatcherModel) TableName() string {
	return "cc_res_freeswitch"
}

// GormDispatcherRepository 基于 GORM 的 Dispatcher 仓储实现。
type GormDispatcherRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewGormDispatcherRepository 创建 GORM Dispatcher 仓储。
func NewGormDispatcherRepository(db *gorm.DB, logger *slog.Logger) *GormDispatcherRepository {
	return &GormDispatcherRepository{DB: db, Logger: logger}
}

// Page 分页查询未删除 dispatcher 节点。
func (r *GormDispatcherRepository) Page(ctx context.Context, req operate.DispatcherPageRequest) (operate.DispatcherPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&DispatcherModel{}).Where("del_flag = ?", false)
	if req.SetID > 0 {
		query = query.Where("set_id = ?", req.SetID)
	}
	if req.Enable != nil {
		query = query.Where("enable = ?", *req.Enable)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.DispatcherPageResult{}, err
	}

	var models []DispatcherModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("set_id ASC, priority DESC, id ASC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.DispatcherPageResult{}, err
	}

	records := make([]operate.Dispatcher, 0, len(models))
	for _, model := range models {
		records = append(records, dispatcherFromModel(model))
	}

	return operate.DispatcherPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    records,
	}, nil
}

// GetByID 读取 dispatcher 节点。
func (r *GormDispatcherRepository) GetByID(ctx context.Context, id int) (operate.Dispatcher, error) {
	var model DispatcherModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Dispatcher{}, operate.ErrDispatcherNotFound
	}
	return dispatcherFromModel(model), err
}

// ExistsDestination 校验 destination 唯一性。
func (r *GormDispatcherRepository) ExistsDestination(ctx context.Context, destination string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&DispatcherModel{}).
		Where("del_flag = ? AND destination = ?", false, destination)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新 dispatcher 节点。
func (r *GormDispatcherRepository) Save(ctx context.Context, disp operate.Dispatcher) (operate.Dispatcher, error) {
	r.logger().Info("开始保存 Kamailio Dispatcher 配置", "destination", disp.Destination, "description", disp.Description)
	model := dispatcherToModel(disp)
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
		r.logger().Error("保存 Kamailio Dispatcher 配置失败", "destination", disp.Destination, "error", err.Error())
		return operate.Dispatcher{}, err
	}
	r.logger().Info("保存 Kamailio Dispatcher 配置成功", "id", model.ID, "destination", model.Destination)
	return dispatcherFromModel(model), nil
}

// Delete 逻辑删除 dispatcher 节点。
func (r *GormDispatcherRepository) Delete(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	r.logger().Info("开始批量逻辑删除 Kamailio Dispatcher 记录", "ids", ids)
	result := r.DB.WithContext(ctx).Model(&DispatcherModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"del_flag":     true,
			"enable":       false,
			"flags":        1,
			"updated_time": time.Now().UTC(),
		})
	if result.Error != nil {
		r.logger().Error("批量逻辑删除 Kamailio Dispatcher 记录失败", "ids", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("批量逻辑删除 Kamailio Dispatcher 记录未匹配到有效记录", "ids", ids)
		return operate.ErrDispatcherNotFound
	}
	r.logger().Info("批量逻辑删除 Kamailio Dispatcher 记录成功", "ids", ids, "rowsAffected", result.RowsAffected)
	return nil
}

func dispatcherToModel(disp operate.Dispatcher) DispatcherModel {
	flags := disp.Flags
	if !disp.Enable {
		flags = 1
	}
	return DispatcherModel{
		ID:          disp.ID,
		SetID:       disp.SetID,
		Destination: disp.Destination,
		Flags:       flags,
		Priority:    disp.Priority,
		Attrs:       disp.Attrs,
		Description: disp.Description,
		Enable:      disp.Enable,
		DelFlag:     false,
	}
}

func dispatcherFromModel(model DispatcherModel) operate.Dispatcher {
	return operate.Dispatcher{
		ID:          model.ID,
		SetID:       model.SetID,
		Destination: model.Destination,
		Flags:       model.Flags,
		Priority:    model.Priority,
		Attrs:       model.Attrs,
		Description: model.Description,
		Enable:      model.Enable,
	}
}

func (r *GormDispatcherRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

// MemoryDispatcherRepository 供本地开发和测试使用。
type MemoryDispatcherRepository struct {
	mu          sync.Mutex
	nextID      int
	dispatchers map[int]operate.Dispatcher
}

// NewMemoryDispatcherRepository 创建 dispatcher 内存仓储。
func NewMemoryDispatcherRepository() *MemoryDispatcherRepository {
	return &MemoryDispatcherRepository{nextID: 1, dispatchers: map[int]operate.Dispatcher{}}
}

func (r *MemoryDispatcherRepository) Page(_ context.Context, req operate.DispatcherPageRequest) (operate.DispatcherPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Dispatcher, 0, len(r.dispatchers))
	for _, disp := range r.dispatchers {
		if req.SetID > 0 && disp.SetID != req.SetID {
			continue
		}
		if req.Enable != nil && disp.Enable != *req.Enable {
			continue
		}
		records = append(records, disp)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Dispatcher{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.DispatcherPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryDispatcherRepository) GetByID(_ context.Context, id int) (operate.Dispatcher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	disp, ok := r.dispatchers[id]
	if !ok {
		return operate.Dispatcher{}, operate.ErrDispatcherNotFound
	}
	return disp, nil
}

func (r *MemoryDispatcherRepository) ExistsDestination(_ context.Context, destination string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, disp := range r.dispatchers {
		if id == excludeID {
			continue
		}
		if disp.Destination == destination {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryDispatcherRepository) Save(_ context.Context, disp operate.Dispatcher) (operate.Dispatcher, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if disp.ID == 0 {
		disp.ID = r.nextID
		r.nextID++
	}
	r.dispatchers[disp.ID] = disp
	return disp, nil
}

func (r *MemoryDispatcherRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for _, id := range ids {
		if _, ok := r.dispatchers[id]; ok {
			delete(r.dispatchers, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrDispatcherNotFound
	}
	return nil
}

// KamailioDispatcherReloader HTTP 触发 Kamailio dispatcher 刷新。
type KamailioDispatcherReloader struct {
	ConfigRepo operate.ProxyConfigRepository
	Logger     *slog.Logger
}

// NewKamailioDispatcherReloader 创建 KamailioDispatcherReloader 实例。
func NewKamailioDispatcherReloader(configRepo operate.ProxyConfigRepository, logger *slog.Logger) *KamailioDispatcherReloader {
	if logger == nil {
		logger = slog.Default()
	}
	return &KamailioDispatcherReloader{ConfigRepo: configRepo, Logger: logger}
}

// ReloadDispatcher HTTP GET 触发 kamailio 热刷新。
func (r *KamailioDispatcherReloader) ReloadDispatcher(ctx context.Context) error {
	wsPort := 5066
	if r.ConfigRepo != nil {
		if item, err := r.ConfigRepo.Get(ctx, operate.KeyKamailioWsPort); err == nil {
			if port, err := strconv.Atoi(item.Value); err == nil && port > 0 {
				wsPort = port
			}
		}
	}

	url1 := fmt.Sprintf("http://127.0.0.1:%d/dispatcher/reload", wsPort)
	r.Logger.Info("开始触发 Kamailio Dispatcher 热重载", "url", url1)

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url1, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			r.Logger.Info("Kamailio Dispatcher 热重载成功", "status", resp.Status)
			return nil
		}
		r.Logger.Warn("Kamailio Dispatcher 热重载接口返回非 200 状态码", "status", resp.Status)
	} else {
		r.Logger.Warn("尝试连接 localhost 失败，尝试容器内网络重试", "error", err.Error())
	}

	// 尝试通过 Docker 容器服务名重试
	url2 := "http://kamailio:5066/dispatcher/reload"
	r.Logger.Info("尝试使用 Docker 容器服务名重载 Dispatcher", "url", url2)
	req2, err := http.NewRequestWithContext(ctx, "GET", url2, nil)
	if err != nil {
		return err
	}
	resp2, err := client.Do(req2)
	if err == nil {
		defer resp2.Body.Close()
		if resp2.StatusCode == http.StatusOK {
			r.Logger.Info("Kamailio Dispatcher 容器内网络热重载成功", "status", resp2.Status)
			return nil
		}
		return fmt.Errorf("kamailio reload returned status %s", resp2.Status)
	}

	return fmt.Errorf("failed to reload dispatcher: %w", err)
}
