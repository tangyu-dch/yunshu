package merchant

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/operate"
)

// MerchantBillingRechargeModel 映射  `merchant_billing_recharge` 表。
type MerchantBillingRechargeModel struct {
	ID          int       `gorm:"column:id;primaryKey"`
	MerchantID  int       `gorm:"column:merchant_id;index"`
	Amount      float64   `gorm:"column:amount"`
	Remark      string    `gorm:"column:remark"`
	Operator    int       `gorm:"column:operator"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

func (MerchantBillingRechargeModel) TableName() string {
	return "cc_mch_billing_recharge"
}

// BillingRepository 基于 GORM 的商户账务管理仓储。
type BillingRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewBillingRepository 创建商户账务仓储。
func NewBillingRepository(db *gorm.DB, logger *slog.Logger) *BillingRepository {
	return &BillingRepository{DB: db, Logger: logger}
}

// PageOverview 分页查询商户账单总览。
func (r *BillingRepository) PageOverview(ctx context.Context, req operate.BillingOverviewPageRequest) (operate.BillingOverviewPageResult, error) {
	// Auto-initialize billing overview for active merchants that don't have one
	var activeMerchantIDs []int
	if err := r.DB.WithContext(ctx).Model(&MerchantModel{}).Where("del_flag = ?", false).Pluck("id", &activeMerchantIDs).Error; err == nil && len(activeMerchantIDs) > 0 {
		var existingMerchantIDs []int
		if err := r.DB.WithContext(ctx).Model(&MerchantBillingOverviewModel{}).Where("merchant_id IN ?", activeMerchantIDs).Pluck("merchant_id", &existingMerchantIDs).Error; err == nil {
			existingSet := make(map[int]bool)
			for _, id := range existingMerchantIDs {
				existingSet[id] = true
			}
			var missingIDs []int
			for _, id := range activeMerchantIDs {
				if !existingSet[id] {
					missingIDs = append(missingIDs, id)
				}
			}
			if len(missingIDs) > 0 {
				now := time.Now().UTC()
				for _, mID := range missingIDs {
					overview := MerchantBillingOverviewModel{
						MerchantID:         mID,
						PaymentMode:        1, // Prepaid
						CurrentBalance:     0,
						CreditLimit:        0,
						DailyTotalAmount:   0,
						MonthlyTotalAmount: 0,
						CreatedTime:        now,
						UpdatedTime:        now,
					}
					_ = r.DB.WithContext(ctx).Create(&overview) // ignore conflicts/errors gracefully
				}
			}
		}
	}

	query := r.DB.WithContext(ctx).Model(&MerchantBillingOverviewModel{}).Joins("LEFT JOIN cc_mch_info ON cc_mch_info.id = cc_mch_billing_overview.merchant_id AND cc_mch_info.del_flag = ?", false)
	tenant, _ := contracts.TenantFromContext(ctx)
	if !tenant.Internal && tenant.MerchantID != "" {
		if id, err := strconv.Atoi(tenant.MerchantID); err == nil {
			query = query.Where("cc_mch_billing_overview.merchant_id = ?", id)
		}
	} else if req.Merchant != "" {
		query = query.Where("cc_mch_info.name LIKE ?", "%"+req.Merchant+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.BillingOverviewPageResult{}, err
	}

	var rows []struct {
		MerchantBillingOverviewModel
		Merchant string `gorm:"column:merchant"`
	}
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Select("cc_mch_billing_overview.*, cc_mch_info.name AS merchant").
		Order("cc_mch_billing_overview.updated_time DESC").
		Offset(offset).Limit(req.PageSize).Scan(&rows).Error; err != nil {
		return operate.BillingOverviewPageResult{}, err
	}
	records := make([]operate.MerchantBillingOverview, 0, len(rows))
	for _, row := range rows {
		records = append(records, billingOverviewFromModel(row.MerchantBillingOverviewModel, row.Merchant))
	}
	return operate.BillingOverviewPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// SaveOverview 保存商户支付方式和信用额度配置，不影响当前余额与消费累计。
func (r *BillingRepository) SaveOverview(ctx context.Context, req operate.BillingOverviewSaveRequest) (operate.MerchantBillingOverview, error) {
	r.logger().Info("开始保存商户账务总览配置", "merchantId", req.MerchantID, "paymentMode", req.PaymentModeCode, "creditLimit", req.CreditLimit)
	now := time.Now().UTC()
	model := MerchantBillingOverviewModel{
		MerchantID:         req.MerchantID,
		PaymentMode:        req.PaymentModeCode,
		CreditLimit:        req.CreditLimit,
		CurrentBalance:     0,
		DailyTotalAmount:   0,
		MonthlyTotalAmount: 0,
		CreatedTime:        now,
		UpdatedTime:        now,
	}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "merchant_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"payment_mode", "credit_limit", "updated_time"}),
	}).Create(&model).Error; err != nil {
		r.logger().Error("保存商户账务总览配置失败", "merchantId", req.MerchantID, "error", err.Error())
		return operate.MerchantBillingOverview{}, err
	}
	var saved MerchantBillingOverviewModel
	if err := r.DB.WithContext(ctx).Where("merchant_id = ?", req.MerchantID).First(&saved).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger().Warn("保存商户账务总览后加载失败：记录未找到", "merchantId", req.MerchantID)
			return operate.MerchantBillingOverview{}, operate.ErrBillingNotFound
		}
		r.logger().Error("保存商户账务总览后加载异常", "merchantId", req.MerchantID, "error", err.Error())
		return operate.MerchantBillingOverview{}, err
	}
	var merchant MerchantModel
	_ = r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", req.MerchantID, false).First(&merchant).Error
	r.logger().Info("保存商户账务总览配置成功", "merchantId", req.MerchantID)
	return billingOverviewFromModel(saved, merchant.Name), nil
}

// Recharge 调整商户余额并落充值记录。
func (r *BillingRepository) Recharge(ctx context.Context, req operate.MerchantRechargeRequest) error {
	r.logger().Info("开始对商户进行充值/账务调整", "merchantId", req.MerchantID, "amount", req.Amount, "operator", req.Operator)
	now := time.Now().UTC()
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		recharge := MerchantBillingRechargeModel{
			MerchantID:  req.MerchantID,
			Amount:      req.Amount,
			Remark:      req.Remark,
			Operator:    req.Operator,
			CreatedTime: now,
			UpdatedTime: now,
		}
		if err := tx.Create(&recharge).Error; err != nil {
			r.logger().Error("创建充值流水记录失败", "merchantId", req.MerchantID, "error", err.Error())
			return err
		}
		var overview MerchantBillingOverviewModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("merchant_id = ?", req.MerchantID).
			First(&overview).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			overview = MerchantBillingOverviewModel{
				MerchantID:         req.MerchantID,
				PaymentMode:        operate.PaymentModePrepaid,
				CurrentBalance:     req.Amount,
				CreditLimit:        0,
				DailyTotalAmount:   0,
				MonthlyTotalAmount: 0,
				CreatedTime:        now,
				UpdatedTime:        now,
			}
			if err := tx.Create(&overview).Error; err != nil {
				r.logger().Error("创建新商户账务总览记录失败", "merchantId", req.MerchantID, "error", err.Error())
				return err
			}
			r.logger().Info("未找到历史账务记录，成功新建商户账务总览并设置初始金额", "merchantId", req.MerchantID, "amount", req.Amount)
			return nil
		}
		if err != nil {
			r.logger().Error("排他锁查询商户账务总览异常", "merchantId", req.MerchantID, "error", err.Error())
			return err
		}
		oldBalance := overview.CurrentBalance
		overview.CurrentBalance += req.Amount
		overview.UpdatedTime = now
		if err := tx.Model(&MerchantBillingOverviewModel{}).
			Where("id = ?", overview.ID).
			Updates(map[string]any{"current_balance": overview.CurrentBalance, "updated_time": now}).Error; err != nil {
			r.logger().Error("更新商户账务余额失败", "merchantId", req.MerchantID, "error", err.Error())
			return err
		}
		r.logger().Info("商户账务余额更新成功", "merchantId", req.MerchantID, "oldBalance", oldBalance, "newBalance", overview.CurrentBalance)
		return nil
	})
	if err != nil {
		r.logger().Error("商户充值事务执行失败", "merchantId", req.MerchantID, "amount", req.Amount, "error", err.Error())
		return err
	}
	r.logger().Info("商户充值事务执行成功", "merchantId", req.MerchantID, "amount", req.Amount)
	return nil
}

// PageRechargeRecords 分页查询充值记录。
func (r *BillingRepository) PageRechargeRecords(ctx context.Context, req operate.MerchantRechargePageRequest) (operate.MerchantRechargePageResult, error) {
	query := r.DB.WithContext(ctx).Model(&MerchantBillingRechargeModel{}).Joins("LEFT JOIN cc_mch_info ON cc_mch_info.id = cc_mch_billing_recharge.merchant_id AND cc_mch_info.del_flag = ?", false)
	tenant, _ := contracts.TenantFromContext(ctx)
	if !tenant.Internal && tenant.MerchantID != "" {
		if id, err := strconv.Atoi(tenant.MerchantID); err == nil {
			query = query.Where("cc_mch_billing_recharge.merchant_id = ?", id)
		}
	} else {
		if req.Merchant != "" {
			query = query.Where("cc_mch_info.name LIKE ?", "%"+req.Merchant+"%")
		}
	}
	if req.RechargeTimeStart > 0 {
		query = query.Where("cc_mch_billing_recharge.created_time >= ?", time.UnixMilli(req.RechargeTimeStart).UTC())
	}
	if req.RechargeTimeEnd > 0 {
		query = query.Where("cc_mch_billing_recharge.created_time <= ?", time.UnixMilli(req.RechargeTimeEnd).UTC())
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.MerchantRechargePageResult{}, err
	}
	var rows []struct {
		MerchantBillingRechargeModel
		Merchant string `gorm:"column:merchant"`
	}
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Select("cc_mch_billing_recharge.*, cc_mch_info.name AS merchant").
		Order("cc_mch_billing_recharge.id DESC").
		Offset(offset).Limit(req.PageSize).Scan(&rows).Error; err != nil {
		return operate.MerchantRechargePageResult{}, err
	}
	records := make([]operate.MerchantRechargeRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, operate.MerchantRechargeRecord{
			ID:         row.ID,
			MerchantID: row.MerchantID,
			Merchant:   row.Merchant,
			Amount:     row.Amount,
			Remark:     row.Remark,
			Operator:   row.Operator,
			CreatedAt:  row.CreatedTime,
		})
	}
	return operate.MerchantRechargePageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// MemoryBillingRepository 供本地开发和测试使用。
type MemoryBillingRepository struct {
	mu          sync.Mutex
	nextID      int
	nextLogID   int
	overviews   map[int]operate.MerchantBillingOverview
	rechargeLog []operate.MerchantRechargeRecord
}

// NewMemoryBillingRepository 创建内存账务仓储。
func NewMemoryBillingRepository() *MemoryBillingRepository {
	return &MemoryBillingRepository{nextID: 1, nextLogID: 1, overviews: map[int]operate.MerchantBillingOverview{}}
}

func (r *MemoryBillingRepository) PageOverview(ctx context.Context, req operate.BillingOverviewPageRequest) (operate.BillingOverviewPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, _ := contracts.TenantFromContext(ctx)
	var mID int
	if !tenant.Internal && tenant.MerchantID != "" {
		if id, err := strconv.Atoi(tenant.MerchantID); err == nil {
			mID = id
		}
	}
	records := make([]operate.MerchantBillingOverview, 0, len(r.overviews))
	for _, item := range r.overviews {
		if mID > 0 && item.MerchantID != mID {
			continue
		}
		if mID == 0 && req.Merchant != "" && !strings.Contains(item.Merchant, req.Merchant) {
			continue
		}
		records = append(records, item)
	}
	return operate.BillingOverviewPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: int64(len(records)), Records: records}, nil
}

func (r *MemoryBillingRepository) SaveOverview(_ context.Context, req operate.BillingOverviewSaveRequest) (operate.MerchantBillingOverview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.overviews[req.MerchantID]
	if !ok {
		item = operate.MerchantBillingOverview{ID: r.nextID, MerchantID: req.MerchantID}
		r.nextID++
	}
	item.PaymentModeCode = req.PaymentModeCode
	item.PaymentMode = billingPaymentModeName(req.PaymentModeCode)
	item.CreditLimit = req.CreditLimit
	r.overviews[req.MerchantID] = item
	return item, nil
}

func (r *MemoryBillingRepository) Recharge(_ context.Context, req operate.MerchantRechargeRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.overviews[req.MerchantID]
	if !ok {
		item = operate.MerchantBillingOverview{
			ID:              r.nextID,
			MerchantID:      req.MerchantID,
			PaymentModeCode: operate.PaymentModePrepaid,
			PaymentMode:     billingPaymentModeName(operate.PaymentModePrepaid),
		}
		r.nextID++
	}
	item.CurrentBalance += req.Amount
	r.overviews[req.MerchantID] = item
	r.rechargeLog = append(r.rechargeLog, operate.MerchantRechargeRecord{
		ID:         r.nextLogID,
		MerchantID: req.MerchantID,
		Amount:     req.Amount,
		Remark:     req.Remark,
		Operator:   req.Operator,
		CreatedAt:  time.Now().UTC(),
	})
	r.nextLogID++
	return nil
}

func (r *MemoryBillingRepository) PageRechargeRecords(ctx context.Context, req operate.MerchantRechargePageRequest) (operate.MerchantRechargePageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, _ := contracts.TenantFromContext(ctx)
	var mID int
	if !tenant.Internal && tenant.MerchantID != "" {
		if id, err := strconv.Atoi(tenant.MerchantID); err == nil {
			mID = id
		}
	}
	records := make([]operate.MerchantRechargeRecord, 0, len(r.rechargeLog))
	for _, item := range r.rechargeLog {
		if mID > 0 && item.MerchantID != mID {
			continue
		}
		records = append(records, item)
	}
	return operate.MerchantRechargePageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: int64(len(records)), Records: records}, nil
}

func billingOverviewFromModel(model MerchantBillingOverviewModel, merchant string) operate.MerchantBillingOverview {
	return operate.MerchantBillingOverview{
		ID:                 model.ID,
		MerchantID:         model.MerchantID,
		Merchant:           merchant,
		PaymentModeCode:    model.PaymentMode,
		PaymentMode:        billingPaymentModeName(model.PaymentMode),
		CurrentBalance:     model.CurrentBalance,
		DailyTotalAmount:   model.DailyTotalAmount,
		MonthlyTotalAmount: model.MonthlyTotalAmount,
		CreditLimit:        model.CreditLimit,
		FeeDate:            model.FeeDate,
		FeeMonth:           model.FeeMonth,
	}
}

func billingPaymentModeName(code int) string {
	switch code {
	case operate.PaymentModePrepaid:
		return "预充值模式"
	case operate.PaymentModePostpaid:
		return "后付费模式"
	default:
		return ""
	}
}

func (r *BillingRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
