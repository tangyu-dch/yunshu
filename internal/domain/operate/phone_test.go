package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestPoolPhoneManagementService(t *testing.T) {
	t.Parallel()

	repo := newFakePoolPhoneRepository()
	service := &operate.PoolPhoneManagementService{Repository: repo}

	// 1. 正常保存号码
	phone, err := service.Save(context.Background(), operate.PoolPhone{
		PoolID:      1,
		Phone:       "13800000001",
		Province:    "广东",
		City:        "深圳",
		Concurrency: 10,
		CallLimit:   100,
		Enable:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if phone.ID == 0 {
		t.Fatalf("expected positive phone ID")
	}

	// 2. 号码唯一性冲突校验
	_, err = service.Save(context.Background(), operate.PoolPhone{
		PoolID: 1,
		Phone:  "13800000001",
	})
	if !errors.Is(err, operate.ErrPoolPhoneConflict) {
		t.Fatalf("expected ErrPoolPhoneConflict, got %v", err)
	}

	// 3. 切换启用状态
	toggled, err := service.SetEnable(context.Background(), phone.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if toggled.Enable {
		t.Fatalf("expected disabled phone")
	}

	// 4. 批量移动号码池
	err = service.SetPool(context.Background(), []int{phone.ID}, 2)
	if err != nil {
		t.Fatal(err)
	}
	moved, _ := repo.GetByID(context.Background(), phone.ID)
	if moved.PoolID != 2 {
		t.Fatalf("expected PoolID to be 2, got %d", moved.PoolID)
	}

	// 5. 分页查询
	page, err := service.Page(context.Background(), operate.PoolPhonePageRequest{
		PageNumber: 1,
		PageSize:   10,
		Phone:      "138",
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}

	// 6. 删除
	err = service.Delete(context.Background(), []operate.PoolPhone{phone})
	if err != nil {
		t.Fatal(err)
	}

	// 验证删除
	_, err = repo.GetByID(context.Background(), phone.ID)
	if !errors.Is(err, operate.ErrPoolPhoneNotFound) {
		t.Fatalf("expected ErrPoolPhoneNotFound, got %v", err)
	}

	// 无效删除校验
	err = service.Delete(context.Background(), []operate.PoolPhone{})
	if !errors.Is(err, operate.ErrInvalidPoolPhone) {
		t.Fatalf("expected ErrInvalidPoolPhone, got %v", err)
	}
}

// fakePoolPhoneRepository
type fakePoolPhoneRepository struct {
	phones map[int]operate.PoolPhone
	nextID int
}

func newFakePoolPhoneRepository() *fakePoolPhoneRepository {
	return &fakePoolPhoneRepository{phones: make(map[int]operate.PoolPhone), nextID: 1}
}

func (r *fakePoolPhoneRepository) Page(_ context.Context, req operate.PoolPhonePageRequest) (operate.PoolPhonePageResult, error) {
	list := make([]operate.PoolPhone, 0)
	for _, v := range r.phones {
		if req.Phone != "" && !strings.Contains(v.Phone, req.Phone) {
			continue
		}
		if req.PoolID > 0 && v.PoolID != req.PoolID {
			continue
		}
		list = append(list, v)
	}
	return operate.PoolPhonePageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(list)),
		Records:    list,
	}, nil
}

func (r *fakePoolPhoneRepository) GetByID(_ context.Context, id int) (operate.PoolPhone, error) {
	p, ok := r.phones[id]
	if !ok {
		return operate.PoolPhone{}, operate.ErrPoolPhoneNotFound
	}
	return p, nil
}

func (r *fakePoolPhoneRepository) ExistsPhone(_ context.Context, phone string, excludeID int) (bool, error) {
	for k, v := range r.phones {
		if k == excludeID {
			continue
		}
		if v.Phone == phone {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakePoolPhoneRepository) Save(_ context.Context, phone operate.PoolPhone) (operate.PoolPhone, error) {
	if phone.ID == 0 {
		phone.ID = r.nextID
		r.nextID++
	}
	r.phones[phone.ID] = phone
	return phone, nil
}

func (r *fakePoolPhoneRepository) Delete(_ context.Context, ids []int) error {
	removed := 0
	for _, id := range ids {
		if _, ok := r.phones[id]; ok {
			delete(r.phones, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrPoolPhoneNotFound
	}
	return nil
}

func (r *fakePoolPhoneRepository) SetEnable(_ context.Context, id int, enable bool) (operate.PoolPhone, error) {
	p, ok := r.phones[id]
	if !ok {
		return operate.PoolPhone{}, operate.ErrPoolPhoneNotFound
	}
	p.Enable = enable
	r.phones[id] = p
	return p, nil
}

func (r *fakePoolPhoneRepository) SetPool(_ context.Context, ids []int, poolID int) error {
	for _, id := range ids {
		if p, ok := r.phones[id]; ok {
			p.PoolID = poolID
			r.phones[id] = p
		}
	}
	return nil
}
