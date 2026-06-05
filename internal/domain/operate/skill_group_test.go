package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/operate"
)

// TestSkillGroupManagementSaveAndPage 验证技能组保存、分页查询与逻辑校验。
func TestSkillGroupManagementSaveAndPage(t *testing.T) {
	t.Parallel()

	repo := newFakeSkillGroupRepository()
	service := &operate.SkillGroupManagementService{Repository: repo}

	// 1. 成功保存 normalized 技能组
	ctx := contracts.WithTenant(context.Background(), contracts.TenantContext{MerchantID: "1001"})
	saved, err := service.Save(ctx, operate.SkillGroup{
		Name:        " 售前咨询组 ", // 包含首尾空格以验证 normalize
		Description: "售前服务",
		Enable:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 {
		t.Fatalf("期望自动生成ID，得到 0")
	}
	if saved.Name != "售前咨询组" {
		t.Fatalf("期望首尾空格被清理，得到: '%s'", saved.Name)
	}
	if saved.MerchantID != 1001 {
		t.Fatalf("期望从 Context 中自动获取 MerchantID 1001，得到 %d", saved.MerchantID)
	}

	// 2. 分页过滤查询
	page, err := service.Page(ctx, operate.SkillGroupPageRequest{PageNumber: 1, PageSize: 10, Name: "售前"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("期望分页结果为 1 条记录，实际得到: %d", page.Total)
	}

	// 3. 校验重名冲突限制
	_, err = service.Save(ctx, operate.SkillGroup{Name: "售前咨询组", MerchantID: 1001})
	if !errors.Is(err, operate.ErrSkillGroupConflict) {
		t.Fatalf("期望发生重名冲突，得到 %v", err)
	}
}

// TestSkillGroupManagementDeleteAndAssociations 验证技能组删除、技能组下用户/号码关联的替换与查询。
func TestSkillGroupManagementDeleteAndAssociations(t *testing.T) {
	t.Parallel()

	repo := newFakeSkillGroupRepository()
	service := &operate.SkillGroupManagementService{Repository: repo}
	ctx := context.Background()

	// 1. 创建测试技能组
	group, err := service.Save(ctx, operate.SkillGroup{Name: "技术支持组", MerchantID: 1001, Enable: true})
	if err != nil {
		t.Fatal(err)
	}

	// 2. 替换并查询技能组用户关联
	err = service.ReplaceUsers(ctx, group.ID, []int{101, 102, -1}) // -1 应该被过滤掉
	if err != nil {
		t.Fatal(err)
	}
	users, err := service.UsersBySkillGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0] != 101 || users[1] != 102 {
		t.Fatalf("用户关联错误，得到: %v", users)
	}

	// 3. 替换并查询技能组号码关联
	err = service.ReplacePhones(ctx, group.ID, []int{501, 502})
	if err != nil {
		t.Fatal(err)
	}
	phones, err := service.PhonesBySkillGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(phones) != 2 || phones[0] != 501 || phones[1] != 502 {
		t.Fatalf("号码关联错误，得到: %v", phones)
	}

	// 4. 当存在活动任务引用时，删除应该失败
	repo.MockActiveTasks = true
	err = service.Delete(ctx, []operate.SkillGroup{group})
	if !errors.Is(err, operate.ErrSkillGroupReferenced) {
		t.Fatalf("期望由于活动任务引用删除失败，得到: %v", err)
	}

	// 5. 消除引用后，删除成功
	repo.MockActiveTasks = false
	err = service.Delete(ctx, []operate.SkillGroup{group})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.GetByID(ctx, group.ID)
	if !errors.Is(err, operate.ErrSkillGroupNotFound) {
		t.Fatalf("期望已被删除，得到: %v", err)
	}
}

// fakeSkillGroupRepository 实现 operate.SkillGroupRepository 接口，用于纯内存单元测试。
type fakeSkillGroupRepository struct {
	nextID          int
	skillGroups     map[int]operate.SkillGroup
	usersMap        map[int][]int
	phonesMap       map[int][]int
	MockActiveTasks bool
}

func newFakeSkillGroupRepository() *fakeSkillGroupRepository {
	return &fakeSkillGroupRepository{
		nextID:          1,
		skillGroups:     make(map[int]operate.SkillGroup),
		usersMap:        make(map[int][]int),
		phonesMap:       make(map[int][]int),
		MockActiveTasks: false,
	}
}

func (r *fakeSkillGroupRepository) Page(_ context.Context, req operate.SkillGroupPageRequest) (operate.SkillGroupPageResult, error) {
	var records []operate.SkillGroup
	for _, sg := range r.skillGroups {
		if req.Name != "" && !strings.Contains(sg.Name, req.Name) {
			continue
		}
		if req.MerchantID > 0 && sg.MerchantID != req.MerchantID {
			continue
		}
		records = append(records, sg)
	}
	return operate.SkillGroupPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(records)),
		Records:    records,
	}, nil
}

func (r *fakeSkillGroupRepository) GetByID(_ context.Context, id int) (operate.SkillGroup, error) {
	sg, ok := r.skillGroups[id]
	if !ok {
		return operate.SkillGroup{}, operate.ErrSkillGroupNotFound
	}
	return sg, nil
}

func (r *fakeSkillGroupRepository) ExistsName(_ context.Context, name string, merchantID int, excludeID int) (bool, error) {
	for id, sg := range r.skillGroups {
		if id == excludeID {
			continue
		}
		if sg.Name == name && sg.MerchantID == merchantID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeSkillGroupRepository) Save(_ context.Context, skillGroup operate.SkillGroup) (operate.SkillGroup, error) {
	if skillGroup.ID == 0 {
		skillGroup.ID = r.nextID
		r.nextID++
	}
	r.skillGroups[skillGroup.ID] = skillGroup
	return skillGroup, nil
}

func (r *fakeSkillGroupRepository) Delete(_ context.Context, ids []int) error {
	for _, id := range ids {
		delete(r.skillGroups, id)
		delete(r.usersMap, id)
		delete(r.phonesMap, id)
	}
	return nil
}

func (r *fakeSkillGroupRepository) ReplaceUsers(_ context.Context, skillGroupID int, userIDs []int) error {
	r.usersMap[skillGroupID] = userIDs
	return nil
}

func (r *fakeSkillGroupRepository) ReplacePhones(_ context.Context, skillGroupID int, phoneIDs []int) error {
	r.phonesMap[skillGroupID] = phoneIDs
	return nil
}

func (r *fakeSkillGroupRepository) UsersBySkillGroup(_ context.Context, skillGroupID int) ([]int, error) {
	return r.usersMap[skillGroupID], nil
}

func (r *fakeSkillGroupRepository) PhonesBySkillGroup(_ context.Context, skillGroupID int) ([]int, error) {
	return r.phonesMap[skillGroupID], nil
}

func (r *fakeSkillGroupRepository) HasActiveTasks(_ context.Context, ids []int) (bool, error) {
	return r.MockActiveTasks, nil
}
