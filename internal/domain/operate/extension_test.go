package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestExtensionManagementService(t *testing.T) {
	t.Parallel()

	repo := newFakeExtensionRepository()
	cache := &fakeAuthCacheInvalidator{}
	service := &operate.ExtensionManagementService{Repository: repo, Cache: cache}

	// 1. 正常保存分机
	ext, err := service.Save(context.Background(), operate.Extension{
		ExtensionNumber: "8001",
		Password:        "pass123",
		MerchantID:      1001,
		UserID:          2001,
		Enable:          true,
		BindType:        operate.BindTypeManual,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ext.ID == 0 {
		t.Fatalf("expected positive extension ID")
	}
	if cache.calls != 1 {
		t.Fatalf("expected 1 auth cache invalidation, got %d", cache.calls)
	}

	// 2. 分机号唯一性冲突校验
	_, err = service.Save(context.Background(), operate.Extension{
		ExtensionNumber: "8001",
		MerchantID:      1001,
		UserID:          2002,
	})
	if !errors.Is(err, operate.ErrExtensionConflict) {
		t.Fatalf("expected ErrExtensionConflict, got %v", err)
	}

	// 3. 动态绑定分机
	err = service.DynamicBind(context.Background(), "8001", 2002, 1001)
	if err != nil {
		t.Fatal(err)
	}

	// 4. 分页查询
	page, err := service.Page(context.Background(), operate.ExtensionPageRequest{
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}

	// 5. 切换启用状态
	toggled, err := service.SetEnable(context.Background(), ext.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if toggled.Enable {
		t.Fatalf("expected disabled extension")
	}

	// 6. 删除
	err = service.Delete(context.Background(), []operate.Extension{ext})
	if err != nil {
		t.Fatal(err)
	}

	// 验证删除
	_, err = repo.GetByID(context.Background(), ext.ID)
	if !errors.Is(err, operate.ErrExtensionNotFound) {
		t.Fatalf("expected ErrExtensionNotFound, got %v", err)
	}

	// 无效删除校验
	err = service.Delete(context.Background(), []operate.Extension{})
	if !errors.Is(err, operate.ErrInvalidExtension) {
		t.Fatalf("expected ErrInvalidExtension, got %v", err)
	}
}

// fakeExtensionRepository
type fakeExtensionRepository struct {
	extensions map[int]operate.Extension
	nextID     int
}

func newFakeExtensionRepository() *fakeExtensionRepository {
	return &fakeExtensionRepository{extensions: make(map[int]operate.Extension), nextID: 1}
}

func (r *fakeExtensionRepository) Page(_ context.Context, req operate.ExtensionPageRequest) (operate.ExtensionPageResult, error) {
	list := make([]operate.Extension, 0)
	for _, v := range r.extensions {
		if req.ExtensionNumber != "" && !strings.Contains(v.ExtensionNumber, req.ExtensionNumber) {
			continue
		}
		if req.MerchantID > 0 && v.MerchantID != req.MerchantID {
			continue
		}
		list = append(list, v)
	}
	return operate.ExtensionPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(list)),
		Records:    list,
	}, nil
}

func (r *fakeExtensionRepository) GetByID(_ context.Context, id int) (operate.Extension, error) {
	ext, ok := r.extensions[id]
	if !ok {
		return operate.Extension{}, operate.ErrExtensionNotFound
	}
	return ext, nil
}

func (r *fakeExtensionRepository) ExistsNumber(_ context.Context, num string, merchantID int, excludeID int) (bool, error) {
	for k, v := range r.extensions {
		if k == excludeID {
			continue
		}
		if v.ExtensionNumber == num && v.MerchantID == merchantID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeExtensionRepository) Save(_ context.Context, ext operate.Extension) (operate.Extension, error) {
	if ext.ID == 0 {
		ext.ID = r.nextID
		r.nextID++
	}
	r.extensions[ext.ID] = ext
	return ext, nil
}

func (r *fakeExtensionRepository) Delete(_ context.Context, ids []int) error {
	removed := 0
	for _, id := range ids {
		if _, ok := r.extensions[id]; ok {
			delete(r.extensions, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrExtensionNotFound
	}
	return nil
}

func (r *fakeExtensionRepository) SetEnable(_ context.Context, id int, enable bool) (operate.Extension, error) {
	ext, ok := r.extensions[id]
	if !ok {
		return operate.Extension{}, operate.ErrExtensionNotFound
	}
	ext.Enable = enable
	r.extensions[id] = ext
	return ext, nil
}

func (r *fakeExtensionRepository) DynamicBind(_ context.Context, num string, userID int, merchantID int) error {
	for k, v := range r.extensions {
		if v.ExtensionNumber == num && v.MerchantID == merchantID {
			v.UserID = userID
			v.BindType = operate.BindTypeDynamic
			r.extensions[k] = v
			return nil
		}
	}
	return operate.ErrExtensionNotFound
}

// fakeAuthCacheInvalidator
type fakeAuthCacheInvalidator struct {
	calls int
}

func (f *fakeAuthCacheInvalidator) InvalidateAuthCache(_ context.Context) error {
	f.calls++
	return nil
}
