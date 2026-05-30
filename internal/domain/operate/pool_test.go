package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

// TestPoolManagementSaveAndPage 验证号码池（Pool）的保存、参数规范化、名字排重以及分页查询。
func TestPoolManagementSaveAndPage(t *testing.T) {
	t.Parallel()

	repo := newFakePoolRepository()
	service := &operate.PoolManagementService{Repository: repo}
	ctx := context.Background()

	// 1. 成功保存号码池并默认分配选号策略 RANDOM
	saved, err := service.Save(ctx, operate.Pool{
		Name:      "  深圳首选主叫号码池 ", // 首尾空格以测试 normalize
		Type:      1,
		GatewayID: 5,
		Enable:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 {
		t.Fatalf("期望号码池 ID 自动生成，得到 0")
	}
	if saved.Name != "深圳首选主叫号码池" {
		t.Fatalf("期望首尾空格被截断，实际为 '%s'", saved.Name)
	}
	if saved.SelectionStrategy != "RANDOM" {
		t.Fatalf("期望未指定选号策略时默认采用 RANDOM，实际得到: %s", saved.SelectionStrategy)
	}

	// 2. 带特定选号策略保存
	saved2, err := service.Save(ctx, operate.Pool{
		Name:              "并发线数控制池",
		Type:              1,
		GatewayID:         5,
		Enable:            true,
		SelectionStrategy: "CONCURRENCY",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved2.SelectionStrategy != "CONCURRENCY" {
		t.Fatalf("期望选号策略为 CONCURRENCY，实际为: %s", saved2.SelectionStrategy)
	}

	// 3. 分页查询
	page, err := service.Page(ctx, operate.PoolPageRequest{PageNumber: 1, PageSize: 10, Name: "首选"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("期望分页命为 1 条记录，实际得到: %d", page.Total)
	}

	// 4. 重名防重冲突拦截
	_, err = service.Save(ctx, operate.Pool{Name: "深圳首选主叫号码池", Type: 1})
	if !errors.Is(err, operate.ErrPoolConflict) {
		t.Fatalf("期望发生重名冲突错误，实际为: %v", err)
	}
}

// fakePoolRepository 实现 operate.PoolRepository 接口，用于纯内存单元测试。
type fakePoolRepository struct {
	nextID int
	pools  map[int]operate.Pool
}

func newFakePoolRepository() *fakePoolRepository {
	return &fakePoolRepository{
		nextID: 1,
		pools:  make(map[int]operate.Pool),
	}
}

func (r *fakePoolRepository) Page(_ context.Context, req operate.PoolPageRequest) (operate.PoolPageResult, error) {
	var records []operate.Pool
	for _, p := range r.pools {
		if req.Name != "" && !strings.Contains(p.Name, req.Name) {
			continue
		}
		if req.GatewayID > 0 && p.GatewayID != req.GatewayID {
			continue
		}
		records = append(records, p)
	}
	return operate.PoolPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(records)),
		Records:    records,
	}, nil
}

func (r *fakePoolRepository) GetByID(_ context.Context, id int) (operate.Pool, error) {
	p, ok := r.pools[id]
	if !ok {
		return operate.Pool{}, operate.ErrPoolNotFound
	}
	return p, nil
}

func (r *fakePoolRepository) ExistsName(_ context.Context, name string, excludeID int) (bool, error) {
	for id, p := range r.pools {
		if id == excludeID {
			continue
		}
		if p.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakePoolRepository) Save(_ context.Context, pool operate.Pool) (operate.Pool, error) {
	if pool.ID == 0 {
		pool.ID = r.nextID
		r.nextID++
	}
	r.pools[pool.ID] = pool
	return pool, nil
}

func (r *fakePoolRepository) Delete(_ context.Context, ids []int) error {
	for _, id := range ids {
		delete(r.pools, id)
	}
	return nil
}

func (r *fakePoolRepository) ListByGateway(_ context.Context, gatewayID int) ([]operate.Pool, error) {
	var out []operate.Pool
	for _, p := range r.pools {
		if p.GatewayID == gatewayID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fakePoolRepository) ListAll(_ context.Context) ([]operate.Pool, error) {
	var out []operate.Pool
	for _, p := range r.pools {
		out = append(out, p)
	}
	return out, nil
}
