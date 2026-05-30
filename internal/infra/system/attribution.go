package system

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

type PhoneAttributionModel struct {
	AreaCode string `gorm:"column:area_code;primaryKey"` // 号码前7位 (例如 "1380013")
	ProvCode string `gorm:"column:prov_code"`            // 省份行政区划代码 (例如 "440000")
	CityCode string `gorm:"column:city_code"`            // 城市行政区划代码 (例如 "440300")
}

func (PhoneAttributionModel) TableName() string {
	return "cc_sys_attribution"
}

type PhoneAttributionGormRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

func NewPhoneAttributionGormRepository(db *gorm.DB, logger *slog.Logger) *PhoneAttributionGormRepository {
	return &PhoneAttributionGormRepository{DB: db, Logger: logger}
}

func (r *PhoneAttributionGormRepository) Page(ctx context.Context, req operate.PhoneAttributionPageRequest) (operate.PhoneAttributionPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&PhoneAttributionModel{})
	if req.AreaCode != "" {
		query = query.Where("area_code LIKE ?", "%"+req.AreaCode+"%")
	}
	if req.ProvCode != "" {
		query = query.Where("prov_code = ?", req.ProvCode)
	}
	if req.CityCode != "" {
		query = query.Where("city_code = ?", req.CityCode)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.PhoneAttributionPageResult{}, err
	}

	var models []PhoneAttributionModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("area_code ASC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.PhoneAttributionPageResult{}, err
	}

	records := make([]operate.PhoneAttribution, 0, len(models))
	for _, m := range models {
		records = append(records, operate.PhoneAttribution{
			AreaCode: m.AreaCode,
			ProvCode: m.ProvCode,
			CityCode: m.CityCode,
		})
	}

	return operate.PhoneAttributionPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    records,
	}, nil
}

func (r *PhoneAttributionGormRepository) GetByAreaCode(ctx context.Context, areaCode string) (operate.PhoneAttribution, bool, error) {
	var m PhoneAttributionModel
	err := r.DB.WithContext(ctx).Where("area_code = ?", areaCode).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.PhoneAttribution{}, false, nil
	}
	if err != nil {
		return operate.PhoneAttribution{}, false, err
	}
	return operate.PhoneAttribution{
		AreaCode: m.AreaCode,
		ProvCode: m.ProvCode,
		CityCode: m.CityCode,
	}, true, nil
}

func (r *PhoneAttributionGormRepository) Save(ctx context.Context, attr operate.PhoneAttribution) (operate.PhoneAttribution, error) {
	model := PhoneAttributionModel{
		AreaCode: attr.AreaCode,
		ProvCode: attr.ProvCode,
		CityCode: attr.CityCode,
	}
	if err := r.DB.WithContext(ctx).Save(&model).Error; err != nil {
		return operate.PhoneAttribution{}, err
	}
	return attr, nil
}

func (r *PhoneAttributionGormRepository) Delete(ctx context.Context, areaCodes []string) error {
	if len(areaCodes) == 0 {
		return nil
	}
	return r.DB.WithContext(ctx).Where("area_code IN ?", areaCodes).Delete(&PhoneAttributionModel{}).Error
}

// MemoryPhoneAttributionRepository 内存版的号码归属地仓储。
type MemoryPhoneAttributionRepository struct {
	mu           sync.Mutex
	attributions map[string]operate.PhoneAttribution
}

func NewMemoryPhoneAttributionRepository() *MemoryPhoneAttributionRepository {
	return &MemoryPhoneAttributionRepository{
		attributions: make(map[string]operate.PhoneAttribution),
	}
}

func (r *MemoryPhoneAttributionRepository) Page(_ context.Context, req operate.PhoneAttributionPageRequest) (operate.PhoneAttributionPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var matched []operate.PhoneAttribution
	for _, a := range r.attributions {
		if req.AreaCode != "" && a.AreaCode != req.AreaCode {
			continue
		}
		if req.ProvCode != "" && a.ProvCode != req.ProvCode {
			continue
		}
		if req.CityCode != "" && a.CityCode != req.CityCode {
			continue
		}
		matched = append(matched, a)
	}

	total := int64(len(matched))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(matched) {
		matched = []operate.PhoneAttribution{}
	} else {
		end := start + req.PageSize
		if end > len(matched) {
			end = len(matched)
		}
		matched = matched[start:end]
	}

	return operate.PhoneAttributionPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    matched,
	}, nil
}

func (r *MemoryPhoneAttributionRepository) GetByAreaCode(_ context.Context, areaCode string) (operate.PhoneAttribution, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.attributions[areaCode]
	return a, ok, nil
}

func (r *MemoryPhoneAttributionRepository) Save(_ context.Context, attr operate.PhoneAttribution) (operate.PhoneAttribution, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attributions[attr.AreaCode] = attr
	return attr, nil
}

func (r *MemoryPhoneAttributionRepository) Delete(_ context.Context, areaCodes []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, code := range areaCodes {
		delete(r.attributions, code)
	}
	return nil
}
