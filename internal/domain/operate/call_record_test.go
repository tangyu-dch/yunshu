package operate_test

import (
	"context"
	"errors"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestCallRecordManagementService(t *testing.T) {
	t.Parallel()

	repo := newFakeCallRecordRepository()
	service := &operate.CallRecordManagementService{Repository: repo}

	// 准备数据
	repo.records["call-123"] = operate.CallRecord{
		CallID:     "call-123",
		MerchantID: 1001,
		Caller:     "13800000000",
		Callee:     "13900000000",
	}

	// 1. 分页查询
	page, err := service.Page(context.Background(), operate.CallRecordPageRequest{
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}

	// 2. 详情查询
	detail, err := service.Detail(context.Background(), "call-123")
	if err != nil {
		t.Fatal(err)
	}
	if detail.CallID != "call-123" {
		t.Fatalf("expected call-123, got %s", detail.CallID)
	}

	// 3. 空 ID 校验
	_, err = service.Detail(context.Background(), "")
	if !errors.Is(err, operate.ErrInvalidCallRecord) {
		t.Fatalf("expected ErrInvalidCallRecord for empty ID, got %v", err)
	}

	// 4. 不存在记录校验
	_, err = service.Detail(context.Background(), "call-nonexistent")
	if !errors.Is(err, operate.ErrCallRecordNotFound) {
		t.Fatalf("expected ErrCallRecordNotFound, got %v", err)
	}
}

// fakeCallRecordRepository
type fakeCallRecordRepository struct {
	records map[string]operate.CallRecord
}

func newFakeCallRecordRepository() *fakeCallRecordRepository {
	return &fakeCallRecordRepository{records: make(map[string]operate.CallRecord)}
}

func (r *fakeCallRecordRepository) Page(_ context.Context, req operate.CallRecordPageRequest) (operate.CallRecordPageResult, error) {
	list := make([]operate.CallRecord, 0, len(r.records))
	for _, v := range r.records {
		list = append(list, v)
	}
	return operate.CallRecordPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(list)),
		Records:    list,
	}, nil
}

func (r *fakeCallRecordRepository) GetByCallID(_ context.Context, callID string) (operate.CallRecord, error) {
	record, ok := r.records[callID]
	if !ok {
		return operate.CallRecord{}, operate.ErrCallRecordNotFound
	}
	return record, nil
}
