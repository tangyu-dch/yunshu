package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"yunshu/internal/domain/operate"
)

func TestBillingManagementService_PageOverview(t *testing.T) {
	t.Parallel()
	repo := newFakeBillingRepository()
	service := &operate.BillingManagementService{Repository: repo}

	// 准备数据
	_, _ = repo.SaveOverview(context.Background(), operate.BillingOverviewSaveRequest{
		MerchantID:      1001,
		PaymentModeCode: operate.PaymentModePrepaid,
		CreditLimit:     500,
	})

	// 测试查询
	res, err := service.PageOverview(context.Background(), operate.BillingOverviewPageRequest{
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || len(res.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", res)
	}
	if res.Records[0].MerchantID != 1001 {
		t.Fatalf("expected merchant 1001, got %d", res.Records[0].MerchantID)
	}
}

func TestBillingManagementService_SaveOverview(t *testing.T) {
	t.Parallel()
	repo := newFakeBillingRepository()
	service := &operate.BillingManagementService{Repository: repo}

	// 正常保存
	saved, err := service.SaveOverview(context.Background(), operate.BillingOverviewSaveRequest{
		MerchantID:      1002,
		PaymentModeCode: operate.PaymentModePostpaid,
		CreditLimit:     1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.MerchantID != 1002 || saved.PaymentModeCode != operate.PaymentModePostpaid || saved.CreditLimit != 1000 {
		t.Fatalf("unexpected saved data: %+v", saved)
	}

	// 参数无效校验
	_, err = service.SaveOverview(context.Background(), operate.BillingOverviewSaveRequest{
		MerchantID:      0, // 无效
		PaymentModeCode: operate.PaymentModePostpaid,
		CreditLimit:     1000,
	})
	if !errors.Is(err, operate.ErrInvalidBilling) {
		t.Fatalf("expected ErrInvalidBilling, got %v", err)
	}
}

func TestBillingManagementService_Recharge(t *testing.T) {
	t.Parallel()
	repo := newFakeBillingRepository()
	service := &operate.BillingManagementService{Repository: repo}

	// 正常充值
	err := service.Recharge(context.Background(), operate.MerchantRechargeRequest{
		MerchantID: 1001,
		Amount:     200,
		Remark:     "首次充值",
		Operator:   1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 校验余额与充值记录
	overviewPage, _ := repo.PageOverview(context.Background(), operate.BillingOverviewPageRequest{})
	if len(overviewPage.Records) != 1 || overviewPage.Records[0].CurrentBalance != 200 {
		t.Fatalf("expected balance to be 200, got %+v", overviewPage.Records)
	}

	rechargePage, err := service.PageRechargeRecords(context.Background(), operate.MerchantRechargePageRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if rechargePage.Total != 1 || rechargePage.Records[0].Amount != 200 {
		t.Fatalf("expected recharge record, got %+v", rechargePage)
	}

	// 参数无效校验
	err = service.Recharge(context.Background(), operate.MerchantRechargeRequest{
		MerchantID: 1001,
		Amount:     0, // 无效金额
	})
	if !errors.Is(err, operate.ErrInvalidBilling) {
		t.Fatalf("expected ErrInvalidBilling, got %v", err)
	}
}

// fakeBillingRepository
type fakeBillingRepository struct {
	overviews map[int]operate.MerchantBillingOverview
	recharges []operate.MerchantRechargeRecord
	nextID    int
}

func newFakeBillingRepository() *fakeBillingRepository {
	return &fakeBillingRepository{
		overviews: make(map[int]operate.MerchantBillingOverview),
		recharges: make([]operate.MerchantRechargeRecord, 0),
		nextID:    1,
	}
}

func (r *fakeBillingRepository) PageOverview(_ context.Context, req operate.BillingOverviewPageRequest) (operate.BillingOverviewPageResult, error) {
	records := make([]operate.MerchantBillingOverview, 0)
	for _, v := range r.overviews {
		if req.Merchant != "" && !strings.Contains(v.Merchant, req.Merchant) {
			continue
		}
		records = append(records, v)
	}
	return operate.BillingOverviewPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(records)),
		Records:    records,
	}, nil
}

func (r *fakeBillingRepository) SaveOverview(_ context.Context, req operate.BillingOverviewSaveRequest) (operate.MerchantBillingOverview, error) {
	v, ok := r.overviews[req.MerchantID]
	if !ok {
		v = operate.MerchantBillingOverview{
			ID:         r.nextID,
			MerchantID: req.MerchantID,
			Merchant:   "商户" + string(rune(req.MerchantID)),
		}
		r.nextID++
	}
	v.PaymentModeCode = req.PaymentModeCode
	v.CreditLimit = req.CreditLimit
	r.overviews[req.MerchantID] = v
	return v, nil
}

func (r *fakeBillingRepository) Recharge(_ context.Context, req operate.MerchantRechargeRequest) error {
	v, ok := r.overviews[req.MerchantID]
	if !ok {
		v = operate.MerchantBillingOverview{
			ID:         r.nextID,
			MerchantID: req.MerchantID,
			Merchant:   "商户",
		}
		r.nextID++
	}
	v.CurrentBalance += req.Amount
	r.overviews[req.MerchantID] = v

	r.recharges = append(r.recharges, operate.MerchantRechargeRecord{
		ID:         len(r.recharges) + 1,
		MerchantID: req.MerchantID,
		Merchant:   v.Merchant,
		Amount:     req.Amount,
		Remark:     req.Remark,
		Operator:   req.Operator,
		CreatedAt:  time.Now(),
	})
	return nil
}

func (r *fakeBillingRepository) PageRechargeRecords(_ context.Context, req operate.MerchantRechargePageRequest) (operate.MerchantRechargePageResult, error) {
	records := make([]operate.MerchantRechargeRecord, 0)
	for _, v := range r.recharges {
		if req.Merchant != "" && !strings.Contains(v.Merchant, req.Merchant) {
			continue
		}
		records = append(records, v)
	}
	return operate.MerchantRechargePageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(records)),
		Records:    records,
	}, nil
}
