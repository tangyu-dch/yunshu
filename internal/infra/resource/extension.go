// Package directory 提供组织、用户、分机等基础资料的数据库 adapter。
package resource

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/esl"
)

// ExtensionModel 映射  侧 `extension` 表。
//
// 字段保持与  ExtensionDO 一致，避免 Go 重写创建第二套分机资料。
type ExtensionModel struct {
	ID              int        `gorm:"column:id;primaryKey"`
	ExtensionNumber string     `gorm:"column:extension_number;type:varchar(64);uniqueIndex:idx_extension_merchant"`
	Password        string     `gorm:"column:password"`
	MerchantID      int        `gorm:"column:merchant_id;uniqueIndex:idx_extension_merchant"`
	UserID          int        `gorm:"column:user_id"`
	Enable          bool       `gorm:"column:enable"`
	BindType        int        `gorm:"column:bind_type;default:1"`
	DelFlag         bool       `gorm:"column:del_flag"`
	OfflineAt       *time.Time `gorm:"column:offline_at"`
	CreatedTime     time.Time  `gorm:"column:created_time"`
	UpdatedTime     time.Time  `gorm:"column:updated_time"`
	SipDomain       string     `gorm:"column:sip_domain;type:varchar(64)"`
	HA1             string     `gorm:"column:ha1;type:varchar(64)"`
	HA1b            string     `gorm:"column:ha1b;type:varchar(64)"`
}

// TableName 返回  生产库中的分机表名。
func (ExtensionModel) TableName() string {
	return "cc_res_extension"
}

// ExtensionRepository 使用 GORM 查询坐席分机信息。
// 该仓储提供从数据库读取分机资料的能力，返回的分机对象可用于 ESL 外部呼出。
type ExtensionRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewExtensionRepository 创建分机仓储实例。
// db 参数为已初始化的 GORM 数据库连接。
func NewExtensionRepository(db *gorm.DB, logger *slog.Logger) *ExtensionRepository {
	return &ExtensionRepository{DB: db, Logger: logger}
}

// GetByUserID 对齐  ExtensionService.getByUserId。
//
// Go 侧额外过滤逻辑删除和禁用分机，避免生产起呼使用不可用分机。
func (r *ExtensionRepository) GetByUserID(ctx context.Context, userID int) (esl.Extension, error) {
	var model ExtensionModel
	err := r.DB.WithContext(ctx).
		Where("user_id = ? AND del_flag = ? AND enable = ?", userID, false, true).
		Order("updated_time DESC, id DESC").
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return esl.Extension{}, esl.ErrExtensionNotFound
		}
		return esl.Extension{}, err
	}
	return esl.Extension{
		ID:              model.ID,
		UserID:          model.UserID,
		MerchantID:      model.MerchantID,
		ExtensionNumber: model.ExtensionNumber,
		SipDomain:       model.SipDomain,
	}, nil
}

func (r *ExtensionRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
