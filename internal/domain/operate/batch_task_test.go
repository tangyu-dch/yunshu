package operate_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestBatchTaskManagementService(t *testing.T) {
	t.Parallel()

	repo := newFakeBatchTaskRepository()
	service := &operate.BatchTaskManagementService{Repository: repo}

	// 1. 正常保存批量任务
	task, err := service.Save(context.Background(), operate.BatchTask{
		MerchantID: 1001,
		UserID:     2001,
		Name:       "营销外呼任务",
		Enable:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.ID == 0 {
		t.Fatalf("expected positive task ID")
	}

	// 2. 无效参数校验
	_, err = service.Save(context.Background(), operate.BatchTask{
		Name: "", // 空名称
	})
	if !errors.Is(err, operate.ErrInvalidBatchTask) {
		t.Fatalf("expected ErrInvalidBatchTask, got %v", err)
	}

	// 3. 启用/禁用切换
	toggled, err := service.SetEnable(context.Background(), task.ID, false, "暂停说明")
	if err != nil {
		t.Fatal(err)
	}
	if toggled.Enable || toggled.PausedReason != "暂停说明" {
		t.Fatalf("expected toggled state, got %+v", toggled)
	}

	// 4. 导入电话
	err = service.ImportTels(context.Background(), task.ID, 1001, 2001, []string{"13800000000"})
	if err != nil {
		t.Fatal(err)
	}

	// 5. 查询明细
	details, err := service.GetDetails(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 1 || details[0].Tel != "13800000000" {
		t.Fatalf("unexpected details: %+v", details)
	}

	// 6. 分页查询
	page, err := service.Page(context.Background(), operate.BatchTaskPageRequest{
		PageNumber: 1,
		PageSize:   10,
		Name:       "营销",
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page results: %+v", page)
	}

	// 7. 删除
	err = service.Delete(context.Background(), []int{task.ID})
	if err != nil {
		t.Fatal(err)
	}

	// 验证删除
	_, err = repo.GetByID(context.Background(), task.ID)
	if !errors.Is(err, operate.ErrBatchTaskNotFound) {
		t.Fatalf("expected ErrBatchTaskNotFound, got %v", err)
	}

	// 无效删除校验
	err = service.Delete(context.Background(), []int{})
	if !errors.Is(err, operate.ErrInvalidBatchTask) {
		t.Fatalf("expected ErrInvalidBatchTask for empty delete, got %v", err)
	}
}

// fakeBatchTaskRepository
type fakeBatchTaskRepository struct {
	tasks   map[int]operate.BatchTask
	details map[int][]operate.BatchTaskDetail
	nextID  int
}

func newFakeBatchTaskRepository() *fakeBatchTaskRepository {
	return &fakeBatchTaskRepository{
		tasks:   make(map[int]operate.BatchTask),
		details: make(map[int][]operate.BatchTaskDetail),
		nextID:  1,
	}
}

func (r *fakeBatchTaskRepository) Page(_ context.Context, req operate.BatchTaskPageRequest) (operate.BatchTaskPageResult, error) {
	records := make([]operate.BatchTask, 0)
	for _, v := range r.tasks {
		if req.Name != "" && !strings.Contains(v.Name, req.Name) {
			continue
		}
		if req.MerchantID > 0 && v.MerchantID != req.MerchantID {
			continue
		}
		records = append(records, v)
	}
	return operate.BatchTaskPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(records)),
		Records:    records,
	}, nil
}

func (r *fakeBatchTaskRepository) GetByID(_ context.Context, id int) (operate.BatchTask, error) {
	task, ok := r.tasks[id]
	if !ok {
		return operate.BatchTask{}, operate.ErrBatchTaskNotFound
	}
	return task, nil
}

func (r *fakeBatchTaskRepository) Save(_ context.Context, task operate.BatchTask) (operate.BatchTask, error) {
	if task.ID == 0 {
		task.ID = r.nextID
		r.nextID++
	}
	r.tasks[task.ID] = task
	return task, nil
}

func (r *fakeBatchTaskRepository) Delete(_ context.Context, ids []int) error {
	removed := 0
	for _, id := range ids {
		if _, ok := r.tasks[id]; ok {
			delete(r.tasks, id)
			delete(r.details, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrBatchTaskNotFound
	}
	return nil
}

func (r *fakeBatchTaskRepository) SetEnable(_ context.Context, id int, enable bool, pausedReason string) (operate.BatchTask, error) {
	task, ok := r.tasks[id]
	if !ok {
		return operate.BatchTask{}, operate.ErrBatchTaskNotFound
	}
	task.Enable = enable
	task.PausedReason = pausedReason
	r.tasks[id] = task
	return task, nil
}

func (r *fakeBatchTaskRepository) ImportTels(_ context.Context, taskID int, merchantID int, userID int, tels []string) error {
	list := r.details[taskID]
	for _, tel := range tels {
		list = append(list, operate.BatchTaskDetail{
			ID:         len(list) + 1,
			TaskID:     taskID,
			MerchantID: merchantID,
			UserID:     userID,
			Tel:        tel,
		})
	}
	r.details[taskID] = list
	return nil
}

func (r *fakeBatchTaskRepository) GetDetails(_ context.Context, taskID int) ([]operate.BatchTaskDetail, error) {
	return r.details[taskID], nil
}
