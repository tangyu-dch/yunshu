package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestMerchantManagementSavePageAndDelete(t *testing.T) {
	t.Parallel()

	service := &operate.MerchantManagementService{Repository: newFakeMerchantRepository()}
	saved, err := service.Save(context.Background(), operate.Merchant{
		Name:             "商户A",
		Account:          "merchant-a",
		WhitelistDomains: "example.com, 192.168.1.1, example.com",
		Enable:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Merchant.ID == 0 {
		t.Fatalf("expected merchant id")
	}
	if saved.Merchant.WhitelistDomains != "example.com,192.168.1.1" {
		t.Fatalf("unexpected whitelist domains: %s", saved.Merchant.WhitelistDomains)
	}
	page, err := service.Page(context.Background(), operate.MerchantPageRequest{PageNumber: 1, PageSize: 10, Name: "商户"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}
	result, err := service.Delete(context.Background(), []operate.Merchant{{ID: saved.Merchant.ID, Name: saved.Merchant.Name}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Merchant.ID != 0 {
		t.Fatalf("unexpected delete result: %+v", result)
	}
}

func TestMerchantManagementRejectsConflict(t *testing.T) {
	t.Parallel()

	service := &operate.MerchantManagementService{Repository: newFakeMerchantRepository()}
	merchant := operate.Merchant{Name: "商户A", Account: "merchant-a"}
	if _, err := service.Save(context.Background(), merchant); err != nil {
		t.Fatal(err)
	}
	_, err := service.Save(context.Background(), merchant)
	if !errors.Is(err, operate.ErrMerchantConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestMerchantManagementRejectsInvalidWhitelistDomains(t *testing.T) {
	t.Parallel()

	service := &operate.MerchantManagementService{Repository: newFakeMerchantRepository()}
	_, err := service.Save(context.Background(), operate.Merchant{
		Name:             "商户A",
		Account:          "merchant-a",
		WhitelistDomains: "bad domain,example.com",
	})
	if !errors.Is(err, operate.ErrInvalidMerchant) {
		t.Fatalf("expected invalid merchant, got %v", err)
	}
}

type fakeMerchantRepository struct {
	nextID    int
	merchants map[int]operate.Merchant
}

func newFakeMerchantRepository() *fakeMerchantRepository {
	return &fakeMerchantRepository{nextID: 1, merchants: map[int]operate.Merchant{}}
}

func (r *fakeMerchantRepository) Page(_ context.Context, req operate.MerchantPageRequest) (operate.MerchantPageResult, error) {
	records := make([]operate.Merchant, 0, len(r.merchants))
	for _, merchant := range r.merchants {
		if req.Name != "" && !strings.Contains(merchant.Name, req.Name) {
			continue
		}
		if req.Account != "" && !strings.Contains(merchant.Account, req.Account) {
			continue
		}
		records = append(records, merchant)
	}
	return operate.MerchantPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: int64(len(records)), Records: records}, nil
}

func (r *fakeMerchantRepository) GetByID(_ context.Context, id int) (operate.Merchant, error) {
	merchant, ok := r.merchants[id]
	if !ok {
		return operate.Merchant{}, operate.ErrMerchantNotFound
	}
	return merchant, nil
}

func (r *fakeMerchantRepository) ExistsNameOrAccount(_ context.Context, name, account string, excludeID int) (bool, error) {
	for id, merchant := range r.merchants {
		if id == excludeID {
			continue
		}
		if merchant.Name == name || merchant.Account == account {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeMerchantRepository) Save(_ context.Context, merchant operate.Merchant) (operate.Merchant, error) {
	if merchant.ID == 0 {
		merchant.ID = r.nextID
		r.nextID++
	}
	r.merchants[merchant.ID] = merchant
	return merchant, nil
}

func (r *fakeMerchantRepository) RateExists(_ context.Context, rateID int) (bool, error) {
	return rateID >= 0, nil
}

func (r *fakeMerchantRepository) Delete(_ context.Context, ids []int) error {
	for _, id := range ids {
		if _, ok := r.merchants[id]; !ok {
			return operate.ErrMerchantNotFound
		}
		delete(r.merchants, id)
	}
	return nil
}

func TestMerchantSipDomainCascade(t *testing.T) {
	t.Parallel()

	mchRepo := newFakeMerchantRepository()
	extRepo := newFakeExtensionRepository()
	cache := &fakeAuthCacheInvalidator{}
	service := &operate.MerchantManagementService{
		Repository:    mchRepo,
		ExtensionRepo: extRepo,
		Cache:         cache,
	}

	// 1. 创建商户，初始域名为 sip.old.com
	mch, err := mchRepo.Save(context.Background(), operate.Merchant{
		Name:      "测试商户",
		Account:   "test-mch",
		SipDomain: "sip.old.com",
		Enable:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. 为该商户创建分机
	ext1, err := extRepo.Save(context.Background(), operate.Extension{
		ExtensionNumber: "1001",
		Password:        "12345678",
		MerchantID:      mch.ID,
		SipDomain:       "sip.old.com",
		HA1:             "old-ha1",
		HA1b:            "old-ha1b",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. 修改商户的 SipDomain 为 sip.new.com
	mch.SipDomain = "sip.new.com"
	_, err = service.Save(context.Background(), mch)
	if err != nil {
		t.Fatal(err)
	}

	// 4. 验证分机 SipDomain 与 HA 值是否已被更新
	updatedExt, err := extRepo.GetByID(context.Background(), ext1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedExt.SipDomain != "sip.new.com" {
		t.Errorf("expected SipDomain to be sip.new.com, got %s", updatedExt.SipDomain)
	}
	if updatedExt.HA1 == "old-ha1" || updatedExt.HA1b == "old-ha1b" {
		t.Errorf("expected HA values to be recalculated and changed")
	}
	if cache.calls != 1 {
		t.Errorf("expected 1 auth cache invalidation, got %d", cache.calls)
	}
}
