package operate_test

import (
	"context"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestWhitelistManagementAddPageDetailUpdateDelete(t *testing.T) {
	t.Parallel()

	service := &operate.WhitelistManagementService{Repository: newFakeWhitelistRepository()}
	message, err := service.Add(context.Background(), operate.AddWhitelistRequest{
		Phones:      []string{"13800000000", "13800000001"},
		NumberType:  operate.WhiteNumberTypeCaller,
		MerchantIDs: []int{1, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "添加白名单成功") {
		t.Fatalf("unexpected message: %s", message)
	}
	page, err := service.Page(context.Background(), operate.WhitelistPageRequest{PageNumber: 1, PageSize: 10, Number: "138"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Records) != 2 {
		t.Fatalf("unexpected page result: %+v", page)
	}
	detail, err := service.Detail(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ID != 1 || detail.NumberType != operate.WhiteNumberTypeCaller {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if err := service.Update(context.Background(), operate.UpdateWhitelistRequest{ID: 1, NumberType: operate.WhiteNumberTypeCallee, MerchantIDs: []int{3}}); err != nil {
		t.Fatal(err)
	}
	detail, err = service.Detail(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if detail.NumberType != operate.WhiteNumberTypeCallee || len(detail.MerchantIDs) != 1 || detail.MerchantIDs[0] != 3 {
		t.Fatalf("unexpected updated detail: %+v", detail)
	}
	if err := service.Delete(context.Background(), []int{1, 2}); err != nil {
		t.Fatal(err)
	}
}

type fakeWhitelistRepository struct {
	nextID  int
	records map[int]operate.WhitelistRecord
}

func newFakeWhitelistRepository() *fakeWhitelistRepository {
	return &fakeWhitelistRepository{nextID: 1, records: map[int]operate.WhitelistRecord{}}
}

func (r *fakeWhitelistRepository) Page(_ context.Context, req operate.WhitelistPageRequest) (operate.WhitelistPageResult, error) {
	records := make([]operate.WhitelistRecord, 0, len(r.records))
	for _, record := range r.records {
		if req.Number != "" && !strings.Contains(record.Phone, req.Number) {
			continue
		}
		if req.MerchantID > 0 {
			found := false
			for _, merchantID := range record.MerchantIDs {
				if merchantID == req.MerchantID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		records = append(records, record)
	}
	return operate.WhitelistPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: int64(len(records)), Records: records}, nil
}

func (r *fakeWhitelistRepository) FindExistingPhones(_ context.Context, phones []string) ([]string, error) {
	existing := make([]string, 0)
	for _, phone := range phones {
		for _, record := range r.records {
			if record.Phone == phone {
				existing = append(existing, phone)
				break
			}
		}
	}
	return existing, nil
}

func (r *fakeWhitelistRepository) CreateBatch(_ context.Context, phones []string, numberType string, merchantIDs []int) error {
	for _, phone := range phones {
		r.records[r.nextID] = operate.WhitelistRecord{ID: r.nextID, Phone: phone, NumberType: numberType, MerchantIDs: merchantIDs}
		r.nextID++
	}
	return nil
}

func (r *fakeWhitelistRepository) GetByID(_ context.Context, id int) (operate.WhitelistRecord, error) {
	record, ok := r.records[id]
	if !ok {
		return operate.WhitelistRecord{}, operate.ErrWhitelistNotFound
	}
	return record, nil
}

func (r *fakeWhitelistRepository) Update(_ context.Context, req operate.UpdateWhitelistRequest) error {
	record, ok := r.records[req.ID]
	if !ok {
		return operate.ErrWhitelistNotFound
	}
	record.NumberType = req.NumberType
	record.MerchantIDs = req.MerchantIDs
	r.records[req.ID] = record
	return nil
}

func (r *fakeWhitelistRepository) Delete(_ context.Context, ids []int) error {
	for _, id := range ids {
		delete(r.records, id)
	}
	return nil
}
