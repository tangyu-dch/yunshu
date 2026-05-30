package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

var (
	// ErrInvalidBilling 表示账务参数错误。
	ErrInvalidBilling = errors.New("invalid billing")
	// ErrBillingNotFound 表示商户账务总览不存在。
	ErrBillingNotFound = errors.New("billing not found")
)

const (
	// PaymentModePrepaid 表示预充值模式。
	PaymentModePrepaid = 1
	// PaymentModePostpaid 表示后付费模式。
	PaymentModePostpaid = 2
)

var billingPaymentModeNames = map[int]string{
	PaymentModePrepaid:  "预充值模式",
	PaymentModePostpaid: "后付费模式",
}

// MerchantBillingOverview 表示商户账务总览。
type MerchantBillingOverview struct {
	ID                 int     `json:"id,omitempty"`
	MerchantID         int     `json:"merchantId"`
	Merchant           string  `json:"merchant,omitempty"`
	PaymentModeCode    int     `json:"paymentModeCode"`
	PaymentMode        string  `json:"paymentMode,omitempty"`
	CurrentBalance     float64 `json:"currentBalance"`
	DailyTotalAmount   float64 `json:"dailyTotalAmount"`
	MonthlyTotalAmount float64 `json:"monthlyTotalAmount"`
	CreditLimit        float64 `json:"creditLimit"`
	FeeDate            int     `json:"feeDate,omitempty"`
	FeeMonth           int     `json:"feeMonth,omitempty"`
}

// BillingOverviewPageRequest 表示账单总览分页查询条件。
type BillingOverviewPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Merchant   string `json:"merchant,omitempty"`
}

// BillingOverviewPageResult 表示账单总览分页结果。
type BillingOverviewPageResult struct {
	PageNumber int                       `json:"pageNumber"`
	PageSize   int                       `json:"pageSize"`
	Total      int64                     `json:"total"`
	Records    []MerchantBillingOverview `json:"records"`
}

// BillingOverviewSaveRequest 表示保存商户账务配置。
type BillingOverviewSaveRequest struct {
	MerchantID      int     `json:"merchantId"`
	PaymentModeCode int     `json:"paymentModeCode"`
	CreditLimit     float64 `json:"creditLimit"`
}

// MerchantRechargeRecord 表示充值记录。
type MerchantRechargeRecord struct {
	ID         int       `json:"id,omitempty"`
	MerchantID int       `json:"merchantId"`
	Merchant   string    `json:"merchant,omitempty"`
	Amount     float64   `json:"amount"`
	Remark     string    `json:"remark,omitempty"`
	Operator   int       `json:"operator,omitempty"`
	CreatedAt  time.Time `json:"createdTime"`
}

// MerchantRechargeRequest 表示账务调整请求。
type MerchantRechargeRequest struct {
	MerchantID int     `json:"merchantId"`
	Amount     float64 `json:"amount"`
	Remark     string  `json:"remark,omitempty"`
	Operator   int     `json:"operator,omitempty"`
}

// MerchantRechargePageRequest 表示充值记录分页查询条件。
type MerchantRechargePageRequest struct {
	PageNumber        int    `json:"pageNumber"`
	PageSize          int    `json:"pageSize"`
	Merchant          string `json:"merchant,omitempty"`
	RechargeTimeStart int64  `json:"rechargeTimeStart,omitempty"`
	RechargeTimeEnd   int64  `json:"rechargeTimeEnd,omitempty"`
}

// MerchantRechargePageResult 表示充值记录分页结果。
type MerchantRechargePageResult struct {
	PageNumber int                      `json:"pageNumber"`
	PageSize   int                      `json:"pageSize"`
	Total      int64                    `json:"total"`
	Records    []MerchantRechargeRecord `json:"records"`
}

// BillingRepository 定义商户账务管理仓储能力。
type BillingRepository interface {
	PageOverview(ctx context.Context, req BillingOverviewPageRequest) (BillingOverviewPageResult, error)
	SaveOverview(ctx context.Context, req BillingOverviewSaveRequest) (MerchantBillingOverview, error)
	Recharge(ctx context.Context, req MerchantRechargeRequest) error
	PageRechargeRecords(ctx context.Context, req MerchantRechargePageRequest) (MerchantRechargePageResult, error)
}

// BillingManagementService 承载运营端商户账务管理业务。
type BillingManagementService struct {
	Repository BillingRepository
	Logger     *slog.Logger
}

// PageOverview 分页查询商户账单总览。
func (s *BillingManagementService) PageOverview(ctx context.Context, req BillingOverviewPageRequest) (BillingOverviewPageResult, error) {
	logger := s.logger()
	req = normalizeBillingOverviewPage(req)
	logger.Info("运营端开始分页查询商户账务总览", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "merchant", req.Merchant)
	page, err := s.Repository.PageOverview(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询商户账务总览失败", "error", err.Error())
		return BillingOverviewPageResult{}, err
	}
	logger.Info("运营端分页查询商户账务总览完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// SaveOverview 保存商户支付方式和信用额度配置。
func (s *BillingManagementService) SaveOverview(ctx context.Context, req BillingOverviewSaveRequest) (MerchantBillingOverview, error) {
	logger := s.logger()
	normalized, err := normalizeBillingOverviewSave(req)
	if err != nil {
		logger.Warn("运营端保存商户账务配置参数无效", "merchantId", req.MerchantID, "error", err.Error())
		return MerchantBillingOverview{}, err
	}
	logger.Info("运营端开始保存商户账务配置", "merchantId", normalized.MerchantID, "paymentModeCode", normalized.PaymentModeCode, "creditLimit", normalized.CreditLimit)
	overview, err := s.Repository.SaveOverview(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存商户账务配置失败", "merchantId", normalized.MerchantID, "error", err.Error())
		return MerchantBillingOverview{}, err
	}
	logger.Info("运营端保存商户账务配置完成", "merchantId", overview.MerchantID, "paymentModeCode", overview.PaymentModeCode, "creditLimit", overview.CreditLimit)
	return overview, nil
}

// Recharge 调整商户余额并写入充值记录。
func (s *BillingManagementService) Recharge(ctx context.Context, req MerchantRechargeRequest) error {
	logger := s.logger()
	normalized, err := normalizeMerchantRecharge(req)
	if err != nil {
		logger.Warn("运营端调整商户余额参数无效", "merchantId", req.MerchantID, "error", err.Error())
		return err
	}
	logger.Info("运营端开始调整商户余额", "merchantId", normalized.MerchantID, "amount", normalized.Amount, "operator", normalized.Operator)
	if err := s.Repository.Recharge(ctx, normalized); err != nil {
		logger.Error("运营端调整商户余额失败", "merchantId", normalized.MerchantID, "amount", normalized.Amount, "error", err.Error())
		return err
	}
	logger.Info("运营端调整商户余额完成", "merchantId", normalized.MerchantID, "amount", normalized.Amount)
	return nil
}

// PageRechargeRecords 分页查询充值记录。
func (s *BillingManagementService) PageRechargeRecords(ctx context.Context, req MerchantRechargePageRequest) (MerchantRechargePageResult, error) {
	logger := s.logger()
	req = normalizeMerchantRechargePage(req)
	logger.Info("运营端开始分页查询充值记录", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "merchant", req.Merchant)
	page, err := s.Repository.PageRechargeRecords(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询充值记录失败", "error", err.Error())
		return MerchantRechargePageResult{}, err
	}
	logger.Info("运营端分页查询充值记录完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

func (s *BillingManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeBillingOverviewPage(req BillingOverviewPageRequest) BillingOverviewPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Merchant = strings.TrimSpace(req.Merchant)
	return req
}

func normalizeBillingOverviewSave(req BillingOverviewSaveRequest) (BillingOverviewSaveRequest, error) {
	if req.MerchantID <= 0 || req.CreditLimit < 0 {
		return BillingOverviewSaveRequest{}, ErrInvalidBilling
	}
	if _, ok := billingPaymentModeNames[req.PaymentModeCode]; !ok {
		return BillingOverviewSaveRequest{}, ErrInvalidBilling
	}
	return req, nil
}

func normalizeMerchantRecharge(req MerchantRechargeRequest) (MerchantRechargeRequest, error) {
	req.Remark = strings.TrimSpace(req.Remark)
	if req.MerchantID <= 0 || req.Amount == 0 {
		return MerchantRechargeRequest{}, ErrInvalidBilling
	}
	return req, nil
}

func normalizeMerchantRechargePage(req MerchantRechargePageRequest) MerchantRechargePageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Merchant = strings.TrimSpace(req.Merchant)
	return req
}
