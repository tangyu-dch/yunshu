package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

// TestDepartmentManagementSaveAndPage 验证部门的保存、分页查询与逻辑校验。
func TestDepartmentManagementSaveAndPage(t *testing.T) {
	t.Parallel()

	repo := newFakeDepartmentRepository()
	service := &operate.DepartmentManagementService{Repository: repo}
	ctx := context.Background()

	// 1. 成功保存部门
	saved, err := service.Save(ctx, operate.Department{
		Name:        " 研发部 ", // 包含首尾空格以验证 normalize
		Description: "核心研发团队",
		MerchantID:  1001,
		Enable:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 {
		t.Fatalf("期望自动生成ID，得到 0")
	}
	if saved.Name != "研发部" {
		t.Fatalf("期望首尾空格被清理，得到: '%s'", saved.Name)
	}
	if saved.MerchantID != 1001 {
		t.Fatalf("期望 MerchantID 1001，得到 %d", saved.MerchantID)
	}

	// 2. 分页过滤查询
	page, err := service.Page(ctx, operate.DepartmentPageRequest{PageNumber: 1, PageSize: 10, Name: "研发"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("期望分页结果为 1 条记录，实际得到: %d", page.Total)
	}
}

// TestDepartmentManagementDeleteAndBindings 验证部门删除校验与绑定阻断。
func TestDepartmentManagementDeleteAndBindings(t *testing.T) {
	t.Parallel()

	repo := newFakeDepartmentRepository()
	service := &operate.DepartmentManagementService{Repository: repo}
	ctx := context.Background()

	// 1. 创建测试部门
	dept, err := service.Save(ctx, operate.Department{Name: "销售部", MerchantID: 1001, Enable: true})
	if err != nil {
		t.Fatal(err)
	}

	// 2. 当存在关联绑定时，删除应该失败
	repo.MockBindings = true
	err = service.Delete(ctx, []int{dept.ID})
	if !errors.Is(err, operate.ErrDepartmentReferenced) {
		t.Fatalf("期望由于引用关系导致删除失败，得到: %v", err)
	}

	// 3. 消除绑定后，删除应该成功
	repo.MockBindings = false
	err = service.Delete(ctx, []int{dept.ID})
	if err != nil {
		t.Fatal(err)
	}

	// 4. 查询已删除部门应报错
	_, err = repo.GetByID(ctx, dept.ID)
	if !errors.Is(err, operate.ErrDepartmentNotFound) {
		t.Fatalf("期望已被删除，得到: %v", err)
	}
}

// fakeDepartmentRepository 实现 operate.DepartmentRepository 接口，用于纯内存单元测试。
type fakeDepartmentRepository struct {
	nextID       int
	depts        map[int]operate.Department
	MockBindings bool
}

func newFakeDepartmentRepository() *fakeDepartmentRepository {
	return &fakeDepartmentRepository{
		nextID:       1,
		depts:        make(map[int]operate.Department),
		MockBindings: false,
	}
}

func (r *fakeDepartmentRepository) Page(_ context.Context, req operate.DepartmentPageRequest) (operate.DepartmentPageResult, error) {
	var records []operate.Department
	for _, d := range r.depts {
		if req.Name != "" && !strings.Contains(d.Name, req.Name) {
			continue
		}
		if req.MerchantID > 0 && d.MerchantID != req.MerchantID {
			continue
		}
		records = append(records, d)
	}
	return operate.DepartmentPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(records)),
		Records:    records,
	}, nil
}

func (r *fakeDepartmentRepository) GetByID(_ context.Context, id int) (operate.Department, error) {
	d, ok := r.depts[id]
	if !ok {
		return operate.Department{}, operate.ErrDepartmentNotFound
	}
	return d, nil
}

func (r *fakeDepartmentRepository) Save(_ context.Context, dept operate.Department) (operate.Department, error) {
	if dept.ID == 0 {
		dept.ID = r.nextID
		r.nextID++
	}
	r.depts[dept.ID] = dept
	return dept, nil
}

func (r *fakeDepartmentRepository) Delete(_ context.Context, ids []int) error {
	for _, id := range ids {
		if _, ok := r.depts[id]; !ok {
			return operate.ErrDepartmentNotFound
		}
		delete(r.depts, id)
	}
	return nil
}

func (r *fakeDepartmentRepository) ListAll(_ context.Context, merchantID int) ([]operate.Department, error) {
	var out []operate.Department
	for _, d := range r.depts {
		if d.MerchantID == merchantID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r *fakeDepartmentRepository) HasBindings(_ context.Context, ids []int) (bool, error) {
	return r.MockBindings, nil
}
