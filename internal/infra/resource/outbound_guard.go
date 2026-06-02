// Package directory 提供组织、用户、分机等基础资料的数据库 adapter。
// 这些 adapter 与  侧表结构保持一致，确保 Go 重写后外部接口行为不变。
package resource

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
)

// paymentModePrepaid 表示商户采用预付费计费模式。
const (
	paymentModePrepaid = 1
)

// MerchantUserModel 映射  `merchant_user` 表。
// 该表存储商户下的坐席用户信息，包含用户状态、逻辑删除标记和坐席绑定信息。
type MerchantUserModel struct {
	ID                  int       `gorm:"column:id;primaryKey"`
	MerchantID          int       `gorm:"column:merchant_id"`
	OrganizationID      int       `gorm:"column:organization_id"`
	Username            string    `gorm:"column:username"`
	SeatNumber          string    `gorm:"column:seat_number"`
	CallExtensionEnable bool      `gorm:"column:call_extension_enable"`
	Enable              bool      `gorm:"column:enable"`
	DelFlag             bool      `gorm:"column:del_flag"`
	CreatedTime         time.Time `gorm:"column:created_time"`
	UpdatedTime         time.Time `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的商户用户表名。
func (MerchantUserModel) TableName() string {
	return "cc_res_mch_user"
}

// MerchantModel 映射  `merchant` 表。
// 该表存储商户主体信息，包含账户状态、有效期 and 计费模式。
type MerchantModel struct {
	ID               int        `gorm:"column:id;primaryKey"`
	Name             string     `gorm:"column:name"`
	Account          string     `gorm:"column:account"`
	ExpiredTime      *time.Time `gorm:"column:expired_time"`
	WhitelistDomains string     `gorm:"column:whitelist_domains"`
	SipDomain        string     `gorm:"column:sip_domain;type:varchar(128);uniqueIndex"`
	Enable           bool       `gorm:"column:enable"`
	DelFlag          bool       `gorm:"column:del_flag"`
	AppKey           string     `gorm:"column:app_key"`
	AppSecret        string     `gorm:"column:app_secret"`
	MaxAgents        int        `gorm:"column:max_agents"`
	CreatedTime      time.Time  `gorm:"column:created_time"`
	UpdatedTime      time.Time  `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的商户表名。
func (MerchantModel) TableName() string {
	return "cc_mch_info"
}

// MerchantBillingOverviewModel 映射  `merchant_billing_overview` 表。
// 该表存储商户的计费余额信息，包括预付费余额和信用额度。
type MerchantBillingOverviewModel struct {
	ID                 int       `gorm:"column:id;primaryKey"`
	MerchantID         int       `gorm:"column:merchant_id"`
	PaymentMode        int       `gorm:"column:payment_mode"`
	CurrentBalance     float64   `gorm:"column:current_balance"`
	DailyTotalAmount   float64   `gorm:"column:daily_total_amount"`
	FeeDate            int       `gorm:"column:fee_date"`
	FeeMonth           int       `gorm:"column:fee_month"`
	MonthlyTotalAmount float64   `gorm:"column:monthly_total_amount"`
	CreditLimit        float64   `gorm:"column:credit_limit"`
	CreatedTime        time.Time `gorm:"column:created_time"`
	UpdatedTime        time.Time `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的商户账单总览表名。
func (MerchantBillingOverviewModel) TableName() string {
	return "cc_mch_billing_overview"
}

// OutboundGuard 是 API 外呼请求的兜底校验器。
// 它对齐  OutboundRequestGuard 的数据库校验逻辑，在发起外呼前验证用户、商户和余额状态。
type OutboundGuard struct {
	DB       *gorm.DB
	Statuses esl.ExtensionStatusReader
	Logger   *slog.Logger
	Now      func() time.Time
}

// NewOutboundGuard 创建 API 外呼兜底校验器。
// statuses 参数用于验证分机 SIP 注册状态，如果为 nil 则跳过状态检查。
func NewOutboundGuard(db *gorm.DB, statuses esl.ExtensionStatusReader, logger *slog.Logger) *OutboundGuard {
	return &OutboundGuard{DB: db, Statuses: statuses, Logger: logger, Now: time.Now}
}

// ValidateAPICall 对齐  OutboundRequestGuard.validateApiCall 的数据库校验部分。
func (g *OutboundGuard) ValidateAPICall(ctx context.Context, req contracts.ApiCallReq, extension esl.Extension) error {
	logger := g.logger()
	logger.Info("开始执行出站外呼兜底安全校验", "userId", req.UserID, "callee", req.Callee, "extensionNumber", extension.ExtensionNumber)

	if req.UserID == 0 || req.Callee == "" {
		logger.Warn("出站外呼安全校验失败：外呼请求参数不完整", "userId", req.UserID, "callee", req.Callee)
		return fmt.Errorf("%w: API 外呼请求参数不完整", esl.ErrOutboundRejected)
	}
	user, err := g.loadUser(ctx, req.UserID)
	if err != nil {
		logger.Warn("出站外呼安全校验失败：加载用户记录失败", "userId", req.UserID, "error", err.Error())
		return err
	}
	if !user.Enable {
		logger.Warn("出站外呼安全校验失败：用户已被停用", "userId", req.UserID)
		return fmt.Errorf("%w: 用户未启用", esl.ErrOutboundRejected)
	}
	if extension.ID == 0 || extension.ExtensionNumber == "" {
		logger.Warn("出站外呼安全校验失败：用户未绑定坐席分机", "userId", req.UserID)
		return fmt.Errorf("%w: 用户未绑定坐席", esl.ErrOutboundRejected)
	}
	if extension.UserID != 0 && extension.UserID != req.UserID {
		logger.Warn("出站外呼安全校验失败：坐席分机不属于当前用户", "userId", req.UserID, "extensionUserId", extension.UserID, "extensionNumber", extension.ExtensionNumber)
		return fmt.Errorf("%w: 坐席不属于当前用户", esl.ErrOutboundRejected)
	}
	if err := g.validateExtensionStatus(ctx, extension.ExtensionNumber); err != nil {
		logger.Warn("出站外呼安全校验失败：分机状态无效", "userId", req.UserID, "extensionNumber", extension.ExtensionNumber, "error", err.Error())
		return err
	}
	merchantID := user.MerchantID
	if merchantID == 0 {
		merchantID = extension.MerchantID
	}
	merchant, err := g.loadMerchant(ctx, merchantID)
	if err != nil {
		logger.Warn("出站外呼安全校验失败：加载商户记录失败", "userId", req.UserID, "merchantId", merchantID, "error", err.Error())
		return err
	}
	if !merchant.Enable {
		logger.Warn("出站外呼安全校验失败：商户已被停用", "userId", req.UserID, "merchantId", merchantID, "merchantName", merchant.Name)
		return fmt.Errorf("%w: 商户账号已停用", esl.ErrOutboundRejected)
	}
	if merchant.ExpiredTime != nil && merchant.ExpiredTime.Before(g.Now()) {
		logger.Warn("出站外呼安全校验失败：商户账号已过期", "userId", req.UserID, "merchantId", merchantID, "merchantName", merchant.Name, "expiredTime", merchant.ExpiredTime)
		return fmt.Errorf("%w: 商户账号已过期", esl.ErrOutboundRejected)
	}
	if err := g.validateBilling(ctx, merchantID); err != nil {
		logger.Warn("出站外呼安全校验失败：商户账务欠费", "userId", req.UserID, "merchantId", merchantID, "merchantName", merchant.Name, "error", err.Error())
		return err
	}

	logger.Info("出站外呼兜底安全校验通过", "userId", req.UserID, "callee", req.Callee, "merchantId", merchantID, "extensionNumber", extension.ExtensionNumber)
	return nil
}

func (g *OutboundGuard) validateExtensionStatus(ctx context.Context, extension string) error {
	if g.Statuses == nil {
		return nil
	}
	status, ok, err := g.Statuses.GetExtensionStatus(ctx, extension)
	if err != nil {
		return err
	}
	if !ok || status == esl.ExtensionStatusOffline {
		return fmt.Errorf("%w: SIP未注册", esl.ErrOutboundRejected)
	}
	switch status {
	case esl.ExtensionStatusPreRing, esl.ExtensionStatusRinging, esl.ExtensionStatusTalking:
		return fmt.Errorf("%w: 当前正在通话中，请稍候重试", esl.ErrOutboundRejected)
	default:
		return nil
	}
}

// loadUser 根据用户 ID 加载未删除的商户用户记录。
// 返回 esl.ErrMerchantUserNotFound 如果用户不存在或已删除。
func (g *OutboundGuard) loadUser(ctx context.Context, userID int) (MerchantUserModel, error) {
	var user MerchantUserModel
	err := g.DB.WithContext(ctx).
		Where("id = ? AND del_flag = ?", userID, false).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MerchantUserModel{}, esl.ErrMerchantUserNotFound
	}
	return user, err
}

func (g *OutboundGuard) loadMerchant(ctx context.Context, merchantID int) (MerchantModel, error) {
	var merchant MerchantModel
	err := g.DB.WithContext(ctx).
		Where("id = ? AND del_flag = ?", merchantID, false).
		First(&merchant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MerchantModel{}, esl.ErrMerchantNotFound
	}
	return merchant, err
}

func (g *OutboundGuard) validateBilling(ctx context.Context, merchantID int) error {
	var billing MerchantBillingOverviewModel
	err := g.DB.WithContext(ctx).
		Where("merchant_id = ?", merchantID).
		First(&billing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if billing.PaymentMode == paymentModePrepaid && billing.CurrentBalance+billing.CreditLimit <= 0 {
		return fmt.Errorf("%w: 商户已欠费", esl.ErrOutboundRejected)
	}
	return nil
}

func (g *OutboundGuard) logger() *slog.Logger {
	if g.Logger != nil {
		return g.Logger
	}
	return slog.Default()
}
