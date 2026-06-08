package resource

import (
	"context"
	"errors"
	"log/slog"

	"gorm.io/gorm"

	"yunshu/internal/domain/esl"
)

// GormSessionSniffer 使用 GORM 从数据库识别分机与 DID 的生产实现。
// 当 FreeSWITCH CHANNEL_CREATE 事件到达时，SessionService 通过此嗅探器
// 自动判断来电是否为坐席直呼或客户呼入，并创建对应的通话会话。
type GormSessionSniffer struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewGormSessionSniffer 创建基于 GORM 的会话嗅探器。
func NewGormSessionSniffer(db *gorm.DB, logger *slog.Logger) *GormSessionSniffer {
	return &GormSessionSniffer{DB: db, Logger: logger}
}

// IsExtension 判断该号码是否为已注册的坐席分机。
// 查询 cc_res_extension 表，匹配 extension_number 并返回完整的分机信息。
func (s *GormSessionSniffer) IsExtension(ctx context.Context, number string) (bool, *esl.Extension, error) {
	if number == "" {
		return false, nil, nil
	}

	var model ExtensionModel
	err := s.DB.WithContext(ctx).
		Where("extension_number = ? AND del_flag = ? AND enable = ?", number, false, true).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, nil
		}
		s.logger().Error("查询分机信息失败", "extension", number, "error", err.Error())
		return false, nil, err
	}

	ext := &esl.Extension{
		ID:              model.ID,
		UserID:          model.UserID,
		MerchantID:      model.MerchantID,
		ExtensionNumber: model.ExtensionNumber,
		SipDomain:       model.SipDomain,
	}
	return true, ext, nil
}

// IsMerchantDID 判断该号码是否为已注册的商户公网呼入 DID，并返回商户 ID。
// 查询链路: cc_res_pool_phone → cc_tel_pool → 获取 merchant_id。
func (s *GormSessionSniffer) IsMerchantDID(ctx context.Context, number string) (bool, int, error) {
	if number == "" {
		return false, 0, nil
	}

	var result struct {
		MerchantID int `gorm:"column:merchant_id"`
	}
	err := s.DB.WithContext(ctx).
		Table("cc_res_pool_phone pp").
		Select("p.merchant_id").
		Joins("INNER JOIN cc_tel_pool p ON p.id = pp.pool_id AND p.enable = 1 AND p.del_flag = 0").
		Where("pp.phone = ? AND pp.enable = 1 AND pp.del_flag = 0", number).
		Scan(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, 0, nil
		}
		s.logger().Error("查询 DID 信息失败", "did", number, "error", err.Error())
		return false, 0, err
	}

	if result.MerchantID == 0 {
		return false, 0, nil
	}

	s.logger().Info("识别到商户 DID 呼入", "did", number, "merchantId", result.MerchantID)
	return true, result.MerchantID, nil
}

func (s *GormSessionSniffer) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}
