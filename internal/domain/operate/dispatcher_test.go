package operate_test

import (
	"context"
	"errors"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestDispatcherManagementService(t *testing.T) {
	t.Parallel()

	repo := newFakeDispatcherRepository()
	reloader := &fakeDispatcherReloadPort{}
	service := &operate.DispatcherManagementService{Repository: repo, Reloader: reloader}

	// 1. 正常保存节点
	disp := operate.Dispatcher{
		SetID:       1,
		Destination: "sip:127.0.0.1:5060",
		Flags:       0,
		Priority:    100,
		Attrs:       "max-concurrency=1000",
		Description: "北京核心网关",
		Enable:      true,
	}
	res, err := service.Save(context.Background(), disp)
	if err != nil {
		t.Fatal(err)
	}
	savedDisp := res.Dispatcher
	if savedDisp.ID == 0 {
		t.Fatalf("expected positive ID")
	}
	if !res.ReloadRequired || !res.ReloadDispatched {
		t.Fatalf("expected hot reload trigger, got %+v", res)
	}
	if reloader.calls != 1 {
		t.Fatalf("expected 1 reload, got %d", reloader.calls)
	}

	// 2. 目的地址冲突校验
	_, err = service.Save(context.Background(), operate.Dispatcher{
		SetID:       1,
		Destination: "sip:127.0.0.1:5060",
		Description: "冲突节点",
	})
	if !errors.Is(err, operate.ErrDispatcherConflict) {
		t.Fatalf("expected ErrDispatcherConflict, got %v", err)
	}

	// 3. 空参数校验
	_, err = service.Save(context.Background(), operate.Dispatcher{
		Destination: "",
	})
	if !errors.Is(err, operate.ErrInvalidDispatcher) {
		t.Fatalf("expected ErrInvalidDispatcher, got %v", err)
	}

	// 4. 手动热刷新配置
	res, err = service.Reload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.ReloadRequired || !res.ReloadDispatched {
		t.Fatalf("expected manual reload success, got %+v", res)
	}

	// 5. 分页查询
	page, err := service.Page(context.Background(), operate.DispatcherPageRequest{
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}

	// 6. 批量删除
	res, err = service.Delete(context.Background(), []operate.Dispatcher{savedDisp})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ReloadRequired || !res.ReloadDispatched {
		t.Fatalf("expected hot reload on delete, got %+v", res)
	}

	// 验证删除
	_, err = repo.GetByID(context.Background(), savedDisp.ID)
	if !errors.Is(err, operate.ErrDispatcherNotFound) {
		t.Fatalf("expected ErrDispatcherNotFound, got %v", err)
	}

	// 无效删除校验
	_, err = service.Delete(context.Background(), []operate.Dispatcher{})
	if !errors.Is(err, operate.ErrInvalidDispatcher) {
		t.Fatalf("expected ErrInvalidDispatcher, got %v", err)
	}
}

// fakeDispatcherRepository
type fakeDispatcherRepository struct {
	dispatchers map[int]operate.Dispatcher
	nextID      int
}

func newFakeDispatcherRepository() *fakeDispatcherRepository {
	return &fakeDispatcherRepository{dispatchers: make(map[int]operate.Dispatcher), nextID: 1}
}

func (r *fakeDispatcherRepository) Page(_ context.Context, req operate.DispatcherPageRequest) (operate.DispatcherPageResult, error) {
	list := make([]operate.Dispatcher, 0)
	for _, v := range r.dispatchers {
		if req.SetID > 0 && v.SetID != req.SetID {
			continue
		}
		if req.Enable != nil && v.Enable != *req.Enable {
			continue
		}
		list = append(list, v)
	}
	return operate.DispatcherPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(list)),
		Records:    list,
	}, nil
}

func (r *fakeDispatcherRepository) GetByID(_ context.Context, id int) (operate.Dispatcher, error) {
	d, ok := r.dispatchers[id]
	if !ok {
		return operate.Dispatcher{}, operate.ErrDispatcherNotFound
	}
	return d, nil
}

func (r *fakeDispatcherRepository) ExistsDestination(_ context.Context, dest string, excludeID int) (bool, error) {
	for k, v := range r.dispatchers {
		if k == excludeID {
			continue
		}
		if v.Destination == dest {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeDispatcherRepository) Save(_ context.Context, d operate.Dispatcher) (operate.Dispatcher, error) {
	if d.ID == 0 {
		d.ID = r.nextID
		r.nextID++
	}
	r.dispatchers[d.ID] = d
	return d, nil
}

func (r *fakeDispatcherRepository) Delete(_ context.Context, ids []int) error {
	removed := 0
	for _, id := range ids {
		if _, ok := r.dispatchers[id]; ok {
			delete(r.dispatchers, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrDispatcherNotFound
	}
	return nil
}

// fakeDispatcherReloadPort
type fakeDispatcherReloadPort struct {
	calls int
}

func (f *fakeDispatcherReloadPort) ReloadDispatcher(_ context.Context) error {
	f.calls++
	return nil
}
