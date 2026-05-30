package system

import (
	"context"
	"gorm.io/gorm"
	"yunshu/internal/domain/operate"
)

// AreaCodeModel 映射数据库中的 `area_code` 表。
type AreaCodeModel struct {
	Code       string `gorm:"column:code;primaryKey;type:varchar(20)"`
	Name       string `gorm:"column:name;type:varchar(100);index"`
	ParentCode string `gorm:"column:parent_code;type:varchar(20);index"`
	Level      int    `gorm:"column:level;type:int"`
}

// TableName 指定 AreaCodeModel 对应的物理表名为 `cc_sys_area`。
func (AreaCodeModel) TableName() string {
	return "cc_sys_area"
}

// AreaCodeGormRepository 提供基于 GORM 的行政区划仓储实现。
type AreaCodeGormRepository struct {
	DB *gorm.DB
}

// NewAreaCodeGormRepository 实例化 AreaCode 数据库仓储。
func NewAreaCodeGormRepository(db *gorm.DB) *AreaCodeGormRepository {
	return &AreaCodeGormRepository{DB: db}
}

// ListAll 查询数据库中所有的行政区划数据并按代码排序。
func (r *AreaCodeGormRepository) ListAll(ctx context.Context) ([]operate.AreaCode, error) {
	var models []AreaCodeModel
	if err := r.DB.WithContext(ctx).Order("code ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]operate.AreaCode, len(models))
	for i, m := range models {
		res[i] = operate.AreaCode{
			Code:       m.Code,
			Name:       m.Name,
			ParentCode: m.ParentCode,
			Level:      m.Level,
		}
	}
	return res, nil
}

// SaveBatch 批量插入或更新行政区划数据（在单次事务中执行以保证极速建库）。
func (r *AreaCodeGormRepository) SaveBatch(ctx context.Context, list []operate.AreaCode) error {
	if len(list) == 0 {
		return nil
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range list {
			m := AreaCodeModel{
				Code:       item.Code,
				Name:       item.Name,
				ParentCode: item.ParentCode,
				Level:      item.Level,
			}
			if err := tx.Save(&m).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// MemoryAreaCodeRepository 提供用于单元测试或本地内存模式的行政区划仓储。
type MemoryAreaCodeRepository struct {
	list []operate.AreaCode
}

// NewMemoryAreaCodeRepository 实例化内存版仓储。
func NewMemoryAreaCodeRepository() *MemoryAreaCodeRepository {
	return &MemoryAreaCodeRepository{list: []operate.AreaCode{}}
}

// ListAll 获取内存中的所有数据。
func (r *MemoryAreaCodeRepository) ListAll(_ context.Context) ([]operate.AreaCode, error) {
	return append([]operate.AreaCode(nil), r.list...), nil
}

// SaveBatch 批量写入内存。
func (r *MemoryAreaCodeRepository) SaveBatch(_ context.Context, list []operate.AreaCode) error {
	r.list = append([]operate.AreaCode(nil), list...)
	return nil
}
