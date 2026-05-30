package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestRiskControlManagementService(t *testing.T) {
	t.Parallel()

	repo := newFakeRiskControlRepository()
	service := &operate.RiskControlManagementService{Repository: repo}

	// 1. 正常保存风控策略
	rc, err := service.Save(context.Background(), operate.RiskControl{
		Name:                "高频外呼风控",
		Remark:              "重点风控",
		BlackLevelFlag:      true,
		BlackLevel:          "LEVEL_3",
		BlindAreaFlag:       true,
		BlindArea:           "广东",
		CalleeFrequencyFlag: true,
		CalleeFrequency:     "10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rc.ID == 0 {
		t.Fatalf("expected positive risk ID")
	}

	// 2. 风控策略重名冲突校验
	_, err = service.Save(context.Background(), operate.RiskControl{
		Name: "高频外呼风控",
	})
	if !errors.Is(err, operate.ErrRiskControlConflict) {
		t.Fatalf("expected ErrRiskControlConflict, got %v", err)
	}

	// 3. 空参数校验
	_, err = service.Save(context.Background(), operate.RiskControl{
		Name: "",
	})
	if !errors.Is(err, operate.ErrInvalidRiskControl) {
		t.Fatalf("expected ErrInvalidRiskControl, got %v", err)
	}

	// 4. 获取策略详情
	detail, err := service.GetByID(context.Background(), rc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Name != "高频外呼风控" {
		t.Fatalf("expected name to be 高频外呼风控, got %s", detail.Name)
	}

	// 5. 保存绑定的商户列表
	bindings := []operate.RiskControlMerchant{
		{RiskID: rc.ID, MerchantID: 1001, Enable: true},
	}
	err = service.SaveMerchants(context.Background(), rc.ID, bindings)
	if err != nil {
		t.Fatal(err)
	}

	// 6. 获取绑定的商户列表
	retrieved, err := service.GetMerchants(context.Background(), rc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(retrieved) != 1 || retrieved[0].MerchantID != 1001 {
		t.Fatalf("unexpected merchant bindings: %+v", retrieved)
	}

	// 7. 绑定商户错误风控ID校验
	err = service.SaveMerchants(context.Background(), rc.ID, []operate.RiskControlMerchant{
		{RiskID: rc.ID + 1, MerchantID: 1001},
	})
	if !errors.Is(err, operate.ErrInvalidRiskControl) {
		t.Fatalf("expected ErrInvalidRiskControl, got %v", err)
	}

	// 8. 分页查询
	page, err := service.Page(context.Background(), operate.RiskControlPageRequest{
		PageNumber: 1,
		PageSize:   10,
		Name:       "风控",
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}

	// 9. 删除风控策略
	err = service.Delete(context.Background(), []operate.RiskControl{rc})
	if err != nil {
		t.Fatal(err)
	}

	// 验证删除
	_, err = repo.GetByID(context.Background(), rc.ID)
	if !errors.Is(err, operate.ErrRiskControlNotFound) {
		t.Fatalf("expected ErrRiskControlNotFound, got %v", err)
	}

	// 无效删除校验
	err = service.Delete(context.Background(), []operate.RiskControl{})
	if !errors.Is(err, operate.ErrInvalidRiskControl) {
		t.Fatalf("expected ErrInvalidRiskControl, got %v", err)
	}
}

// fakeRiskControlRepository
type fakeRiskControlRepository struct {
	rcs       map[int]operate.RiskControl
	merchants map[int][]operate.RiskControlMerchant
	nextID    int
}

func newFakeRiskControlRepository() *fakeRiskControlRepository {
	return &fakeRiskControlRepository{
		rcs:       make(map[int]operate.RiskControl),
		merchants: make(map[int][]operate.RiskControlMerchant),
		nextID:    1,
	}
}

func (r *fakeRiskControlRepository) Page(_ context.Context, req operate.RiskControlPageRequest) (operate.RiskControlPageResult, error) {
	list := make([]operate.RiskControl, 0)
	for _, v := range r.rcs {
		if req.Name != "" && !strings.Contains(v.Name, req.Name) {
			continue
		}
		list = append(list, v)
	}
	return operate.RiskControlPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(list)),
		Records:    list,
	}, nil
}

func (r *fakeRiskControlRepository) GetByID(_ context.Context, id int) (operate.RiskControl, error) {
	rc, ok := r.rcs[id]
	if !ok {
		return operate.RiskControl{}, operate.ErrRiskControlNotFound
	}
	return rc, nil
}

func (r *fakeRiskControlRepository) ExistsName(_ context.Context, name string, excludeID int) (bool, error) {
	for k, v := range r.rcs {
		if k == excludeID {
			continue
		}
		if v.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRiskControlRepository) Save(_ context.Context, rc operate.RiskControl) (operate.RiskControl, error) {
	if rc.ID == 0 {
		rc.ID = r.nextID
		r.nextID++
	}
	r.rcs[rc.ID] = rc
	return rc, nil
}

func (r *fakeRiskControlRepository) Delete(_ context.Context, ids []int) error {
	removed := 0
	for _, id := range ids {
		if _, ok := r.rcs[id]; ok {
			delete(r.rcs, id)
			delete(r.merchants, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrRiskControlNotFound
	}
	return nil
}

func (r *fakeRiskControlRepository) GetMerchants(_ context.Context, riskID int) ([]operate.RiskControlMerchant, error) {
	return r.merchants[riskID], nil
}

func (r *fakeRiskControlRepository) SaveMerchants(_ context.Context, riskID int, bindings []operate.RiskControlMerchant) error {
	r.merchants[riskID] = bindings
	return nil
}
