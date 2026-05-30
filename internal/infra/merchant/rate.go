package merchant

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

// CallRateModel 映射  `call_rate` 表。
type CallRateModel struct {
	ID           int       `gorm:"column:id;primaryKey"`
	RateName     string    `gorm:"column:rate_name"`
	BillingPrice float64   `gorm:"column:billing_price"`
	BillingCycle int       `gorm:"column:billing_cycle"`
	Remark       string    `gorm:"column:remark"`
	Enable       bool      `gorm:"column:enable"`
	DelFlag      bool      `gorm:"column:del_flag"`
	CreatedTime  time.Time `gorm:"column:created_time"`
	UpdatedTime  time.Time `gorm:"column:updated_time"`
}

func (CallRateModel) TableName() string {
	return "cc_mch_rate"
}

// CallRateMerchantModel 映射  `call_rate_merchant` 表。
type CallRateMerchantModel struct {
	RateID     int `gorm:"column:rate_id;primaryKey"`
	MerchantID int `gorm:"column:merchant_id;primaryKey"`
}

func (CallRateMerchantModel) TableName() string {
	return "cc_mch_rate_ref"
}

// RateRepository 基于 GORM 的费率管理仓储。
// 负责费率的分页查询、新增/更新、逻辑删除和引用检查。
type RateRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewRateRepository 创建费率仓储。
func NewRateRepository(db *gorm.DB, logger *slog.Logger) *RateRepository {
	return &RateRepository{DB: db, Logger: logger}
}

// logger 返回注入的 Logger，未注入时回退到 slog.Default()。
func (r *RateRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

// Page 分页查询未删除费率。
func (r *RateRepository) Page(ctx context.Context, req operate.RatePageRequest) (operate.RatePageResult, error) {
	query := r.DB.WithContext(ctx).Model(&CallRateModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("rate_name LIKE ?", "%"+req.Name+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.RatePageResult{}, err
	}
	var models []CallRateModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("updated_time DESC, id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.RatePageResult{}, err
	}
	records := make([]operate.Rate, 0, len(models))
	for _, model := range models {
		records = append(records, rateFromModel(model))
	}
	return operate.RatePageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 查询单个未删除费率。
func (r *RateRepository) GetByID(ctx context.Context, id int) (operate.Rate, error) {
	var model CallRateModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Rate{}, operate.ErrRateNotFound
	}
	return rateFromModel(model), err
}

// ExistsName 校验费率名称唯一性。
func (r *RateRepository) ExistsName(ctx context.Context, rateName string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&CallRateModel{}).
		Where("rate_name = ? AND del_flag = ?", rateName, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新费率。
// 新增时 ID==0，由数据库自增生成；更新时保留原 ID。
func (r *RateRepository) Save(ctx context.Context, rate operate.Rate) (operate.Rate, error) {
	model := rateToModel(rate)
	now := time.Now().UTC()
	model.UpdatedTime = now
	isCreate := model.ID == 0
	if isCreate {
		model.CreatedTime = now
	}
	tx := r.DB.WithContext(ctx)
	if model.ID != 0 {
		tx = tx.Omit("created_time")
	}
	if err := tx.Save(&model).Error; err != nil {
		r.logger().Error("费率保存失败", "rateId", rate.ID, "rateName", rate.RateName, "isCreate", isCreate, "error", err.Error())
		return operate.Rate{}, err
	}
	if isCreate {
		r.logger().Info("费率新增成功", "rateId", model.ID, "rateName", model.RateName)
	} else {
		r.logger().Info("费率更新成功", "rateId", model.ID, "rateName", model.RateName)
	}
	return rateFromModel(model), nil
}

// Delete 逻辑删除费率。
// 调用方应先通过 HasBindings 校验费率是否仍被网关或商户引用，被引用的费率不允许删除。
func (r *RateRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除费率", "ids", ids)
	result := r.DB.WithContext(ctx).Model(&CallRateModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("费率逻辑删除失败", "ids", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("费率逻辑删除未匹配到记录", "ids", ids)
		return operate.ErrRateNotFound
	}
	r.logger().Info("费率逻辑删除成功", "ids", ids, "rowsAffected", result.RowsAffected)
	return nil
}

// HasBindings 检查费率是否仍被网关或商户引用。
// 分别查询 gateway 表和 call_rate_merchant 表，任一存在关联则返回 true。
func (r *RateRepository) HasBindings(ctx context.Context, ids []int) (bool, error) {
	var gatewayCount int64
	if err := r.DB.WithContext(ctx).Model(&GatewayModel{}).
		Where("rate_id IN ? AND del_flag = ?", ids, false).
		Count(&gatewayCount).Error; err != nil {
		r.logger().Error("检查费率网关引用失败", "ids", ids, "error", err.Error())
		return false, err
	}
	if gatewayCount > 0 {
		r.logger().Info("费率仍被网关引用，不允许删除", "ids", ids, "gatewayCount", gatewayCount)
		return true, nil
	}
	var merchantCount int64
	if err := r.DB.WithContext(ctx).Model(&CallRateMerchantModel{}).
		Where("rate_id IN ?", ids).
		Count(&merchantCount).Error; err != nil {
		r.logger().Error("检查费率商户引用失败", "ids", ids, "error", err.Error())
		return false, err
	}
	if merchantCount > 0 {
		r.logger().Info("费率仍被商户引用，不允许删除", "ids", ids, "merchantCount", merchantCount)
	}
	return merchantCount > 0, nil
}

// MemoryRateRepository 供本地开发和测试使用的内存费率仓储。
// 不持久化数据，进程重启后丢失。
type MemoryRateRepository struct {
	mu       sync.Mutex
	nextID   int
	rates    map[int]operate.Rate
	bindings map[int]bool
}

// NewMemoryRateRepository 创建内存费率仓储。
func NewMemoryRateRepository() *MemoryRateRepository {
	return &MemoryRateRepository{
		nextID:   1,
		rates:    map[int]operate.Rate{},
		bindings: map[int]bool{},
	}
}

// Page 按名称模糊筛选并分页返回费率。
func (r *MemoryRateRepository) Page(_ context.Context, req operate.RatePageRequest) (operate.RatePageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Rate, 0, len(r.rates))
	for _, rate := range r.rates {
		if req.Name != "" && !strings.Contains(rate.RateName, req.Name) {
			continue
		}
		records = append(records, rate)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Rate{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.RatePageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 按 ID 查找费率。
func (r *MemoryRateRepository) GetByID(_ context.Context, id int) (operate.Rate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rate, ok := r.rates[id]
	if !ok {
		return operate.Rate{}, operate.ErrRateNotFound
	}
	return rate, nil
}

// ExistsName 检查费率名称是否已被占用。
func (r *MemoryRateRepository) ExistsName(_ context.Context, rateName string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, rate := range r.rates {
		if id == excludeID {
			continue
		}
		if rate.RateName == rateName {
			return true, nil
		}
	}
	return false, nil
}

// Save 保存费率到内存。
func (r *MemoryRateRepository) Save(_ context.Context, rate operate.Rate) (operate.Rate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rate.ID == 0 {
		rate.ID = r.nextID
		r.nextID++
	}
	r.rates[rate.ID] = rate
	return rate, nil
}

// Delete 从内存中删除费率。
func (r *MemoryRateRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for _, id := range ids {
		if _, ok := r.rates[id]; ok {
			delete(r.rates, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrRateNotFound
	}
	return nil
}

// HasBindings 检查费率是否存在模拟引用关系。
func (r *MemoryRateRepository) HasBindings(_ context.Context, ids []int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if r.bindings[id] {
			return true, nil
		}
	}
	return false, nil
}

// BindingsForTest 仅供测试构造“费率已被引用”的场景。
func (r *MemoryRateRepository) BindingsForTest(rateID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings[rateID] = true
}

// rateToModel 将领域费率对象转换为数据库模型。
func rateToModel(rate operate.Rate) CallRateModel {
	return CallRateModel{
		ID:           rate.ID,
		RateName:     rate.RateName,
		BillingPrice: rate.BillingPrice,
		BillingCycle: rate.BillingCycle,
		Remark:       rate.Remark,
		Enable:       true,
		DelFlag:      false,
	}
}

// rateFromModel 将数据库模型转换为领域费率对象。
func rateFromModel(model CallRateModel) operate.Rate {
	return operate.Rate{
		ID:           model.ID,
		RateName:     model.RateName,
		BillingPrice: model.BillingPrice,
		BillingCycle: model.BillingCycle,
		Remark:       model.Remark,
	}
}
