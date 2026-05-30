package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

// TestPhoneGroupManagementSaveAndPage 验证号码组的保存、校验、分页查询及冲突防御逻辑。
func TestPhoneGroupManagementSaveAndPage(t *testing.T) {
	t.Parallel()

	repo := newFakePhoneGroupRepository()
	service := &operate.PhoneGroupManagementService{Repository: repo}
	ctx := context.Background()

	// 1. 成功保存 normalized 号码组记录
	saved, err := service.Save(ctx, operate.PhoneGroup{
		Name:       " 深圳业务线路组 ", // 包含首尾空格以验证 normalize
		Remark:     "核心主叫线路",
		MerchantID: 1001,
		Enable:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 {
		t.Fatalf("期望自动生成ID，得到 0")
	}
	if saved.Name != "深圳业务线路组" {
		t.Fatalf("期望首尾空格被清理，得到: '%s'", saved.Name)
	}

	// 2. 分页过滤查询
	page, err := service.Page(ctx, operate.PhoneGroupPageRequest{PageNumber: 1, PageSize: 10, Name: "深圳"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("期望分页结果为 1 条记录，实际得到: %d", page.Total)
	}

	// 3. 校验同商户下的名字冲突限制
	_, err = service.Save(ctx, operate.PhoneGroup{Name: "深圳业务线路组", MerchantID: 1001})
	if !errors.Is(err, operate.ErrPhoneGroupConflict) {
		t.Fatalf("期望发生名字冲突错误，实际得到 %v", err)
	}

	// 4. 不同商户允许同名号码组
	_, err = service.Save(ctx, operate.PhoneGroup{Name: "深圳业务线路组", MerchantID: 1002})
	if err != nil {
		t.Fatalf("不同商户应允许同名，但发生了错误: %v", err)
	}
}

// TestPhoneGroupManagementDeleteAndAssociations 验证号码组逻辑删除及号码、技能组级联绑定关系的替换与读取。
func TestPhoneGroupManagementDeleteAndAssociations(t *testing.T) {
	t.Parallel()

	repo := newFakePhoneGroupRepository()
	service := &operate.PhoneGroupManagementService{Repository: repo}
	ctx := context.Background()

	// 1. 创建测试号码组
	group, err := service.Save(ctx, operate.PhoneGroup{Name: "广州专线组", MerchantID: 1001, Enable: true})
	if err != nil {
		t.Fatal(err)
	}

	// 2. 替换并查询号码关联
	err = service.ReplacePhones(ctx, group.ID, 1001, []int{301, 302, -1}) // -1 应该被过滤掉
	if err != nil {
		t.Fatal(err)
	}
	phones, err := service.PhonesByGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(phones) != 2 || phones[0] != 301 || phones[1] != 302 {
		t.Fatalf("号码绑定映射错误，得到: %v", phones)
	}

	// 3. 替换并查询技能组关联
	err = service.ReplaceSkillGroups(ctx, group.ID, 1001, []int{401, 402})
	if err != nil {
		t.Fatal(err)
	}
	skillGroups, err := service.SkillGroupsByGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(skillGroups) != 2 || skillGroups[0] != 401 || skillGroups[1] != 402 {
		t.Fatalf("技能组绑定映射错误，得到: %v", skillGroups)
	}

	// 4. 逻辑删除号码组
	err = service.Delete(ctx, []operate.PhoneGroup{group})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.GetByID(ctx, group.ID)
	if !errors.Is(err, operate.ErrPhoneGroupNotFound) {
		t.Fatalf("期望已被删除，得到: %v", err)
	}
}

// fakePhoneGroupRepository 实现 operate.PhoneGroupRepository 接口，用于纯内存单元测试。
type fakePhoneGroupRepository struct {
	nextID      int
	phoneGroups map[int]operate.PhoneGroup
	phoneRefs   map[int][]int
	skillRefs   map[int][]int
}

func newFakePhoneGroupRepository() *fakePhoneGroupRepository {
	return &fakePhoneGroupRepository{
		nextID:      1,
		phoneGroups: make(map[int]operate.PhoneGroup),
		phoneRefs:   make(map[int][]int),
		skillRefs:   make(map[int][]int),
	}
}

func (r *fakePhoneGroupRepository) Page(_ context.Context, req operate.PhoneGroupPageRequest) (operate.PhoneGroupPageResult, error) {
	var records []operate.PhoneGroup
	for _, pg := range r.phoneGroups {
		if req.Name != "" && !strings.Contains(pg.Name, req.Name) {
			continue
		}
		if req.MerchantID > 0 && pg.MerchantID != req.MerchantID {
			continue
		}
		records = append(records, pg)
	}
	return operate.PhoneGroupPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(records)),
		Records:    records,
	}, nil
}

func (r *fakePhoneGroupRepository) GetByID(_ context.Context, id int) (operate.PhoneGroup, error) {
	pg, ok := r.phoneGroups[id]
	if !ok {
		return operate.PhoneGroup{}, operate.ErrPhoneGroupNotFound
	}
	return pg, nil
}

func (r *fakePhoneGroupRepository) ExistsName(_ context.Context, name string, merchantID int, excludeID int) (bool, error) {
	for id, pg := range r.phoneGroups {
		if id == excludeID {
			continue
		}
		if pg.Name == name && pg.MerchantID == merchantID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakePhoneGroupRepository) Save(_ context.Context, group operate.PhoneGroup) (operate.PhoneGroup, error) {
	if group.ID == 0 {
		group.ID = r.nextID
		r.nextID++
	}
	r.phoneGroups[group.ID] = group
	return group, nil
}

func (r *fakePhoneGroupRepository) Delete(_ context.Context, ids []int) error {
	for _, id := range ids {
		delete(r.phoneGroups, id)
		delete(r.phoneRefs, id)
		delete(r.skillRefs, id)
	}
	return nil
}

func (r *fakePhoneGroupRepository) ReplacePhones(_ context.Context, groupID int, _ int, phoneIDs []int) error {
	r.phoneRefs[groupID] = phoneIDs
	return nil
}

func (r *fakePhoneGroupRepository) ReplaceSkillGroups(_ context.Context, groupID int, _ int, skillGroupIDs []int) error {
	r.skillRefs[groupID] = skillGroupIDs
	return nil
}

func (r *fakePhoneGroupRepository) PhonesByGroup(_ context.Context, groupID int) ([]int, error) {
	return r.phoneRefs[groupID], nil
}

func (r *fakePhoneGroupRepository) SkillGroupsByGroup(_ context.Context, groupID int) ([]int, error) {
	return r.skillRefs[groupID], nil
}
