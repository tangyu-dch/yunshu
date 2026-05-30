package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestRtpengineManagementService(t *testing.T) {
	t.Parallel()

	repo := newFakeRtpengineRepository()
	reloader := &fakeRtpengineReloadPort{}
	service := &operate.RtpengineManagementService{Repository: repo, Reloader: reloader}

	// 1. 正常保存节点
	engine := operate.Rtpengine{
		SetID:         1,
		RtpengineSock: "udp:127.0.0.1:22222",
		Disabled:      false,
		Weight:        100,
		Description:   "深圳节点",
	}
	res, err := service.Save(context.Background(), engine)
	if err != nil {
		t.Fatal(err)
	}
	savedEngine := res.Rtpengine
	if savedEngine.ID == 0 {
		t.Fatalf("expected positive ID")
	}
	if !res.ReloadRequired || !res.ReloadDispatched {
		t.Fatalf("expected hot reload trigger, got %+v", res)
	}
	if reloader.calls != 1 {
		t.Fatalf("expected 1 reload, got %d", reloader.calls)
	}

	// 2. 套接字冲突校验
	_, err = service.Save(context.Background(), operate.Rtpengine{
		SetID:         1,
		RtpengineSock: "udp:127.0.0.1:22222",
		Description:   "冲突节点",
	})
	if !errors.Is(err, operate.ErrRtpengineConflict) {
		t.Fatalf("expected ErrRtpengineConflict, got %v", err)
	}

	// 3. 空参数校验
	_, err = service.Save(context.Background(), operate.Rtpengine{
		RtpengineSock: "",
	})
	if !errors.Is(err, operate.ErrInvalidRtpengine) {
		t.Fatalf("expected ErrInvalidRtpengine, got %v", err)
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
	page, err := service.Page(context.Background(), operate.RtpenginePageRequest{
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
	res, err = service.Delete(context.Background(), []operate.Rtpengine{savedEngine})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ReloadRequired || !res.ReloadDispatched {
		t.Fatalf("expected hot reload on delete, got %+v", res)
	}

	// 验证删除
	_, err = repo.GetByID(context.Background(), savedEngine.ID)
	if !errors.Is(err, operate.ErrRtpengineNotFound) {
		t.Fatalf("expected ErrRtpengineNotFound, got %v", err)
	}

	// 无效删除校验
	_, err = service.Delete(context.Background(), []operate.Rtpengine{})
	if !errors.Is(err, operate.ErrInvalidRtpengine) {
		t.Fatalf("expected ErrInvalidRtpengine, got %v", err)
	}
}

// fakeRtpengineRepository
type fakeRtpengineRepository struct {
	engines map[int]operate.Rtpengine
	nextID  int
}

func newFakeRtpengineRepository() *fakeRtpengineRepository {
	return &fakeRtpengineRepository{engines: make(map[int]operate.Rtpengine), nextID: 1}
}

func (r *fakeRtpengineRepository) Page(_ context.Context, req operate.RtpenginePageRequest) (operate.RtpenginePageResult, error) {
	list := make([]operate.Rtpengine, 0)
	for _, v := range r.engines {
		if req.RtpengineSock != "" && !strings.Contains(v.RtpengineSock, req.RtpengineSock) {
			continue
		}
		if req.SetID > 0 && v.SetID != req.SetID {
			continue
		}
		list = append(list, v)
	}
	return operate.RtpenginePageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(list)),
		Records:    list,
	}, nil
}

func (r *fakeRtpengineRepository) GetByID(_ context.Context, id int) (operate.Rtpengine, error) {
	e, ok := r.engines[id]
	if !ok {
		return operate.Rtpengine{}, operate.ErrRtpengineNotFound
	}
	return e, nil
}

func (r *fakeRtpengineRepository) ExistsSock(_ context.Context, sock string, excludeID int) (bool, error) {
	for k, v := range r.engines {
		if k == excludeID {
			continue
		}
		if v.RtpengineSock == sock {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRtpengineRepository) Save(_ context.Context, e operate.Rtpengine) (operate.Rtpengine, error) {
	if e.ID == 0 {
		e.ID = r.nextID
		r.nextID++
	}
	r.engines[e.ID] = e
	return e, nil
}

func (r *fakeRtpengineRepository) Delete(_ context.Context, ids []int) error {
	removed := 0
	for _, id := range ids {
		if _, ok := r.engines[id]; ok {
			delete(r.engines, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrRtpengineNotFound
	}
	return nil
}

// fakeRtpengineReloadPort
type fakeRtpengineReloadPort struct {
	calls int
}

func (f *fakeRtpengineReloadPort) ReloadRtpengine(_ context.Context) error {
	f.calls++
	return nil
}
