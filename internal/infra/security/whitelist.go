package security

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
	"yunshu/internal/infra/resource"
)

// WhitelistDataModel 映射  `whitelist_data` 表。
type WhitelistDataModel struct {
	ID          int       `gorm:"column:id;primaryKey"`
	Phone       string    `gorm:"column:phone"`
	NumberType  string    `gorm:"column:number_type"`
	Enable      bool      `gorm:"column:enable"`
	DelFlag     bool      `gorm:"column:del_flag"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

func (WhitelistDataModel) TableName() string {
	return "cc_sec_whitelist"
}

// WhitelistDataMerchantModel 映射  `whitelist_data_merchant` 表。
type WhitelistDataMerchantModel struct {
	WhiteID    int `gorm:"column:white_id;primaryKey"`
	MerchantID int `gorm:"column:merchant_id;primaryKey"`
}

func (WhitelistDataMerchantModel) TableName() string {
	return "cc_sec_whitelist_merchant"
}

// WhitelistRepository 基于 GORM 的白名单管理仓储。
type WhitelistRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewWhitelistRepository 创建白名单仓储。
func NewWhitelistRepository(db *gorm.DB, logger *slog.Logger) *WhitelistRepository {
	return &WhitelistRepository{DB: db, Logger: logger}
}

// Page 分页查询白名单。
func (r *WhitelistRepository) Page(ctx context.Context, req operate.WhitelistPageRequest) (operate.WhitelistPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&WhitelistDataModel{}).Where("del_flag = ?", false)
	if req.Number != "" {
		query = query.Where("phone LIKE ?", "%"+req.Number+"%")
	}
	if req.MerchantID > 0 {
		sub := r.DB.WithContext(ctx).Model(&WhitelistDataMerchantModel{}).Select("distinct white_id").Where("merchant_id = ?", req.MerchantID)
		query = query.Where("id IN (?)", sub)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.WhitelistPageResult{}, err
	}
	var models []WhitelistDataModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("created_time DESC, id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.WhitelistPageResult{}, err
	}
	ids := make([]int, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	bindings, err := r.loadMerchantBindings(ctx, ids)
	if err != nil {
		return operate.WhitelistPageResult{}, err
	}
	names, err := r.loadMerchantNames(ctx, flattenMerchantBindings(bindings))
	if err != nil {
		return operate.WhitelistPageResult{}, err
	}
	records := make([]operate.WhitelistRecord, 0, len(models))
	for _, model := range models {
		records = append(records, whitelistFromModel(model, bindings[model.ID], names))
	}
	return operate.WhitelistPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// FindExistingPhones 查询已存在白名单号码。
func (r *WhitelistRepository) FindExistingPhones(ctx context.Context, phones []string) ([]string, error) {
	var rows []struct {
		Phone string `gorm:"column:phone"`
	}
	if err := r.DB.WithContext(ctx).Model(&WhitelistDataModel{}).
		Select("phone").
		Where("phone IN ? AND del_flag = ?", phones, false).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.Phone)
	}
	slices.Sort(result)
	return result, nil
}

// CreateBatch 批量创建白名单及商户绑定。
func (r *WhitelistRepository) CreateBatch(ctx context.Context, phones []string, numberType string, merchantIDs []int) error {
	if len(phones) == 0 {
		return nil
	}
	r.logger().Info("开始批量导入白名单数据", "phoneCount", len(phones), "numberType", numberType, "merchantCount", len(merchantIDs))
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		models := make([]WhitelistDataModel, 0, len(phones))
		for _, phone := range phones {
			models = append(models, WhitelistDataModel{
				Phone:       phone,
				NumberType:  numberType,
				Enable:      true,
				DelFlag:     false,
				CreatedTime: now,
				UpdatedTime: now,
			})
		}
		if err := tx.Create(&models).Error; err != nil {
			r.logger().Error("批量写入白名单数据失败", "error", err.Error())
			return err
		}
		if len(merchantIDs) == 0 {
			return nil
		}
		refs := make([]WhitelistDataMerchantModel, 0, len(models)*len(merchantIDs))
		for _, model := range models {
			for _, merchantID := range merchantIDs {
				refs = append(refs, WhitelistDataMerchantModel{WhiteID: model.ID, MerchantID: merchantID})
			}
		}
		if err := tx.Create(&refs).Error; err != nil {
			r.logger().Error("批量写入白名单商户关联失败", "error", err.Error())
			return err
		}
		return nil
	})
	if err != nil {
		r.logger().Error("批量导入白名单事务异常", "error", err.Error())
		return err
	}
	r.logger().Info("批量导入白名单成功", "phoneCount", len(phones))
	return nil
}

// GetByID 查询白名单详情。
func (r *WhitelistRepository) GetByID(ctx context.Context, id int) (operate.WhitelistRecord, error) {
	var model WhitelistDataModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.WhitelistRecord{}, operate.ErrWhitelistNotFound
	}
	if err != nil {
		return operate.WhitelistRecord{}, err
	}
	bindings, err := r.loadMerchantBindings(ctx, []int{id})
	if err != nil {
		return operate.WhitelistRecord{}, err
	}
	names, err := r.loadMerchantNames(ctx, bindings[id])
	if err != nil {
		return operate.WhitelistRecord{}, err
	}
	return whitelistFromModel(model, bindings[id], names), nil
}

// Update 更新白名单类型和商户绑定。
func (r *WhitelistRepository) Update(ctx context.Context, req operate.UpdateWhitelistRequest) error {
	r.logger().Info("开始更新白名单数据", "id", req.ID, "numberType", req.NumberType, "merchantCount", len(req.MerchantIDs))
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&WhitelistDataModel{}).
			Where("id = ? AND del_flag = ?", req.ID, false).
			Updates(map[string]any{"number_type": req.NumberType, "updated_time": time.Now().UTC()})
		if result.Error != nil {
			r.logger().Error("更新白名单基本数据失败", "id", req.ID, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			r.logger().Warn("更新白名单失败：未找到有效记录", "id", req.ID)
			return operate.ErrWhitelistNotFound
		}
		if err := tx.Where("white_id = ?", req.ID).Delete(&WhitelistDataMerchantModel{}).Error; err != nil {
			r.logger().Error("清除历史白名单商户关联失败", "id", req.ID, "error", err.Error())
			return err
		}
		if len(req.MerchantIDs) == 0 {
			return nil
		}
		refs := make([]WhitelistDataMerchantModel, 0, len(req.MerchantIDs))
		for _, merchantID := range req.MerchantIDs {
			refs = append(refs, WhitelistDataMerchantModel{WhiteID: req.ID, MerchantID: merchantID})
		}
		if err := tx.Create(&refs).Error; err != nil {
			r.logger().Error("创建白名单新商户关联记录失败", "id", req.ID, "error", err.Error())
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	r.logger().Info("更新白名单数据成功", "id", req.ID)
	return nil
}

// Delete 逻辑删除白名单并清理关系。
func (r *WhitelistRepository) Delete(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	r.logger().Info("开始批量逻辑删除白名单", "ids", ids)
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&WhitelistDataModel{}).
			Where("id IN ?", ids).
			Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
		if result.Error != nil {
			r.logger().Error("批量逻辑删除白名单基础记录失败", "ids", ids, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			r.logger().Warn("批量逻辑删除白名单失败：未匹配到有效记录", "ids", ids)
			return operate.ErrWhitelistNotFound
		}
		if err := tx.Where("white_id IN ?", ids).Delete(&WhitelistDataMerchantModel{}).Error; err != nil {
			r.logger().Error("批量物理删除白名单商户映射关系失败", "ids", ids, "error", err.Error())
			return err
		}
		r.logger().Info("批量逻辑删除白名单成功", "ids", ids, "rowsAffected", result.RowsAffected)
		return nil
	})
}

func (r *WhitelistRepository) loadMerchantBindings(ctx context.Context, whiteIDs []int) (map[int][]int, error) {
	result := make(map[int][]int, len(whiteIDs))
	if len(whiteIDs) == 0 {
		return result, nil
	}
	var refs []WhitelistDataMerchantModel
	if err := r.DB.WithContext(ctx).Where("white_id IN ?", whiteIDs).Find(&refs).Error; err != nil {
		return nil, err
	}
	for _, ref := range refs {
		result[ref.WhiteID] = append(result[ref.WhiteID], ref.MerchantID)
	}
	for id := range result {
		slices.Sort(result[id])
	}
	return result, nil
}

func (r *WhitelistRepository) loadMerchantNames(ctx context.Context, merchantIDs []int) (map[int]string, error) {
	result := make(map[int]string, len(merchantIDs))
	if len(merchantIDs) == 0 {
		return result, nil
	}
	var rows []resource.MerchantModel
	if err := r.DB.WithContext(ctx).Where("id IN ? AND del_flag = ?", merchantIDs, false).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row.Name
	}
	return result, nil
}

// MemoryWhitelistRepository 供本地开发和测试使用。
type MemoryWhitelistRepository struct {
	mu      sync.Mutex
	nextID  int
	records map[int]operate.WhitelistRecord
}

// NewMemoryWhitelistRepository 创建内存白名单仓储。
func NewMemoryWhitelistRepository() *MemoryWhitelistRepository {
	return &MemoryWhitelistRepository{nextID: 1, records: map[int]operate.WhitelistRecord{}}
}

func (r *MemoryWhitelistRepository) Page(_ context.Context, req operate.WhitelistPageRequest) (operate.WhitelistPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.WhitelistRecord, 0, len(r.records))
	for _, record := range r.records {
		if req.Number != "" && !strings.Contains(record.Phone, req.Number) {
			continue
		}
		if req.MerchantID > 0 && !slices.Contains(record.MerchantIDs, req.MerchantID) {
			continue
		}
		records = append(records, record)
	}
	return operate.WhitelistPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: int64(len(records)), Records: records}, nil
}

func (r *MemoryWhitelistRepository) FindExistingPhones(_ context.Context, phones []string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]string, 0)
	for _, phone := range phones {
		for _, record := range r.records {
			if record.Phone == phone {
				result = append(result, phone)
				break
			}
		}
	}
	slices.Sort(result)
	return result, nil
}

func (r *MemoryWhitelistRepository) CreateBatch(_ context.Context, phones []string, numberType string, merchantIDs []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, phone := range phones {
		r.records[r.nextID] = operate.WhitelistRecord{ID: r.nextID, Phone: phone, NumberType: numberType, MerchantIDs: merchantIDs}
		r.nextID++
	}
	return nil
}

func (r *MemoryWhitelistRepository) GetByID(_ context.Context, id int) (operate.WhitelistRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.records[id]
	if !ok {
		return operate.WhitelistRecord{}, operate.ErrWhitelistNotFound
	}
	return record, nil
}

func (r *MemoryWhitelistRepository) Update(_ context.Context, req operate.UpdateWhitelistRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.records[req.ID]
	if !ok {
		return operate.ErrWhitelistNotFound
	}
	record.NumberType = req.NumberType
	record.MerchantIDs = req.MerchantIDs
	r.records[req.ID] = record
	return nil
}

func (r *MemoryWhitelistRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		delete(r.records, id)
	}
	return nil
}

func whitelistFromModel(model WhitelistDataModel, merchantIDs []int, merchantNames map[int]string) operate.WhitelistRecord {
	names := make([]string, 0, len(merchantIDs))
	for _, merchantID := range merchantIDs {
		if name, ok := merchantNames[merchantID]; ok && name != "" {
			names = append(names, name)
		}
	}
	return operate.WhitelistRecord{
		ID:            model.ID,
		Phone:         model.Phone,
		NumberType:    model.NumberType,
		MerchantIDs:   merchantIDs,
		MerchantNames: names,
	}
}

func flattenMerchantBindings(bindings map[int][]int) []int {
	seen := make(map[int]struct{})
	result := make([]int, 0)
	for _, merchantIDs := range bindings {
		for _, merchantID := range merchantIDs {
			if merchantID <= 0 {
				continue
			}
			if _, ok := seen[merchantID]; ok {
				continue
			}
			seen[merchantID] = struct{}{}
			result = append(result, merchantID)
		}
	}
	slices.Sort(result)
	return result
}

func (r *WhitelistRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
