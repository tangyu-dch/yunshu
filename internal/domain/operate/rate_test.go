package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestRateManagementSavePageAndDelete(t *testing.T) {
	t.Parallel()

	service := &operate.RateManagementService{Repository: newFakeRateRepository()}
	saved, err := service.Save(context.Background(), operate.Rate{
		RateName:     "标准费率",
		BillingPrice: 0.3,
		BillingCycle: 60,
		Remark:       "按分钟计费",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 {
		t.Fatalf("expected rate id")
	}
	page, err := service.Page(context.Background(), operate.RatePageRequest{PageNumber: 1, PageSize: 10, Name: "标准"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}
	if err := service.Delete(context.Background(), []operate.Rate{{ID: saved.ID}}); err != nil {
		t.Fatal(err)
	}
}

func TestRateManagementRejectsConflict(t *testing.T) {
	t.Parallel()

	service := &operate.RateManagementService{Repository: newFakeRateRepository()}
	rate := operate.Rate{RateName: "标准费率", BillingPrice: 0.3, BillingCycle: 60}
	if _, err := service.Save(context.Background(), rate); err != nil {
		t.Fatal(err)
	}
	_, err := service.Save(context.Background(), rate)
	if !errors.Is(err, operate.ErrRateConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestRateManagementRejectsReferencedDelete(t *testing.T) {
	t.Parallel()

	repo := newFakeRateRepository()
	service := &operate.RateManagementService{Repository: repo}
	saved, err := service.Save(context.Background(), operate.Rate{RateName: "标准费率", BillingPrice: 0.3, BillingCycle: 60})
	if err != nil {
		t.Fatal(err)
	}
	repo.bindings[saved.ID] = true
	err = service.Delete(context.Background(), []operate.Rate{{ID: saved.ID}})
	if !errors.Is(err, operate.ErrRateReferenced) {
		t.Fatalf("expected referenced error, got %v", err)
	}
}

type fakeRateRepository struct {
	nextID   int
	rates    map[int]operate.Rate
	bindings map[int]bool
}

func newFakeRateRepository() *fakeRateRepository {
	return &fakeRateRepository{
		nextID:   1,
		rates:    map[int]operate.Rate{},
		bindings: map[int]bool{},
	}
}

func (r *fakeRateRepository) Page(_ context.Context, req operate.RatePageRequest) (operate.RatePageResult, error) {
	records := make([]operate.Rate, 0, len(r.rates))
	for _, rate := range r.rates {
		if req.Name != "" && !strings.Contains(rate.RateName, req.Name) {
			continue
		}
		records = append(records, rate)
	}
	return operate.RatePageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: int64(len(records)), Records: records}, nil
}

func (r *fakeRateRepository) GetByID(_ context.Context, id int) (operate.Rate, error) {
	rate, ok := r.rates[id]
	if !ok {
		return operate.Rate{}, operate.ErrRateNotFound
	}
	return rate, nil
}

func (r *fakeRateRepository) ExistsName(_ context.Context, rateName string, excludeID int) (bool, error) {
	for id, rate := range r.rates {
		if id == excludeID {
			continue
		}
		if rate.RateName == rateName {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRateRepository) Save(_ context.Context, rate operate.Rate) (operate.Rate, error) {
	if rate.ID == 0 {
		rate.ID = r.nextID
		r.nextID++
	}
	r.rates[rate.ID] = rate
	return rate, nil
}

func (r *fakeRateRepository) Delete(_ context.Context, ids []int) error {
	removed := 0
	for _, id := range ids {
		if _, ok := r.rates[id]; ok {
			delete(r.rates, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrRateNotFound
	}
	return nil
}

func (r *fakeRateRepository) HasBindings(_ context.Context, ids []int) (bool, error) {
	for _, id := range ids {
		if r.bindings[id] {
			return true, nil
		}
	}
	return false, nil
}
