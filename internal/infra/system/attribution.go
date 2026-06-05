package system

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

type PhoneAttributionModel struct {
	AreaCode        string `gorm:"column:area_code;primaryKey"` // 号码前7位 (例如 "1380013")
	Province        string `gorm:"column:province"`             // 省份名称 (例如 "北京")
	City            string `gorm:"column:city"`                 // 城市名称 (例如 "北京")
	ProvCode        string `gorm:"column:prov_code"`            // 省份行政区划代码 (例如 "110000")
	CityCode        string `gorm:"column:city_code"`            // 城市行政区划代码 (例如 "110000")
	ServiceProvider string `gorm:"column:isp"`                  // 运营商 (例如 "中国联通")
}

func (PhoneAttributionModel) TableName() string {
	return "cc_sys_phone_attribution"
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
	if req.Province != "" {
		query = query.Where("province LIKE ?", "%"+req.Province+"%")
	}
	if req.City != "" {
		query = query.Where("city LIKE ?", "%"+req.City+"%")
	}
	if req.ServiceProvider != "" {
		query = query.Where("isp LIKE ?", "%"+req.ServiceProvider+"%")
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
			AreaCode:        m.AreaCode,
			Province:        m.Province,
			City:            m.City,
			ProvCode:        m.ProvCode,
			CityCode:        m.CityCode,
			ServiceProvider: m.ServiceProvider,
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
		AreaCode:        m.AreaCode,
		Province:        m.Province,
		City:            m.City,
		ProvCode:        m.ProvCode,
		CityCode:        m.CityCode,
		ServiceProvider: m.ServiceProvider,
	}, true, nil
}

func (r *PhoneAttributionGormRepository) Save(ctx context.Context, attr operate.PhoneAttribution) (operate.PhoneAttribution, error) {
	model := PhoneAttributionModel{
		AreaCode:        attr.AreaCode,
		Province:        attr.Province,
		City:            attr.City,
		ProvCode:        attr.ProvCode,
		CityCode:        attr.CityCode,
		ServiceProvider: attr.ServiceProvider,
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
		if req.Province != "" && !strings.Contains(a.Province, req.Province) {
			continue
		}
		if req.City != "" && !strings.Contains(a.City, req.City) {
			continue
		}
		if req.ServiceProvider != "" && !strings.Contains(a.ServiceProvider, req.ServiceProvider) {
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

// CachedPhoneAttributionRepository 包装另一个 PhoneAttributionRepository 并加上本地缓存
type CachedPhoneAttributionRepository struct {
	underlying operate.PhoneAttributionRepository
	cache      sync.Map // key: areaCode (string) -> value: any
}

func NewCachedPhoneAttributionRepository(underlying operate.PhoneAttributionRepository) *CachedPhoneAttributionRepository {
	return &CachedPhoneAttributionRepository{
		underlying: underlying,
	}
}

func (r *CachedPhoneAttributionRepository) Page(ctx context.Context, req operate.PhoneAttributionPageRequest) (operate.PhoneAttributionPageResult, error) {
	return r.underlying.Page(ctx, req)
}

func (r *CachedPhoneAttributionRepository) GetByAreaCode(ctx context.Context, areaCode string) (operate.PhoneAttribution, bool, error) {
	if val, ok := r.cache.Load(areaCode); ok {
		if attr, ok := val.(operate.PhoneAttribution); ok {
			return attr, true, nil
		}
		// Negative cache hit (value is false or not operate.PhoneAttribution)
		return operate.PhoneAttribution{}, false, nil
	}

	attr, found, err := r.underlying.GetByAreaCode(ctx, areaCode)
	if err != nil {
		return operate.PhoneAttribution{}, false, err
	}

	if found {
		r.cache.Store(areaCode, attr)
	} else {
		// Cache negative result to prevent cache penetration
		r.cache.Store(areaCode, false)
	}

	return attr, found, nil
}

func (r *CachedPhoneAttributionRepository) Save(ctx context.Context, attr operate.PhoneAttribution) (operate.PhoneAttribution, error) {
	saved, err := r.underlying.Save(ctx, attr)
	if err != nil {
		return operate.PhoneAttribution{}, err
	}
	r.cache.Store(attr.AreaCode, saved)
	return saved, nil
}

func (r *CachedPhoneAttributionRepository) Delete(ctx context.Context, areaCodes []string) error {
	if err := r.underlying.Delete(ctx, areaCodes); err != nil {
		return err
	}
	for _, code := range areaCodes {
		r.cache.Delete(code)
	}
	return nil
}
