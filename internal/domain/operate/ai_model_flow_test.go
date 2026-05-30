package operate_test

import (
	"context"
	"errors"
	"testing"

	"yunshu/internal/domain/operate"
)

func TestAIModelFlowManagementService(t *testing.T) {
	t.Parallel()

	repo := operate.NewMemoryAIModelFlowRepository()
	service := &operate.AIModelFlowManagementService{Repository: repo}

	// 1. 正常保存 AI 流程
	flow, err := service.Save(context.Background(), operate.AIModelFlow{
		Name:   "客服外呼流程",
		Prompt: "你是一个专业的客服助理...",
	})
	if err != nil {
		t.Fatal(err)
	}
	if flow.ID == 0 {
		t.Fatalf("expected positive flow ID")
	}

	// 2. 参数验证校验
	_, err = service.Save(context.Background(), operate.AIModelFlow{
		Name:   "", // 无效名称
		Prompt: "Prompt",
	})
	if !errors.Is(err, operate.ErrInvalidAIModelFlow) {
		t.Fatalf("expected ErrInvalidAIModelFlow, got %v", err)
	}

	// 3. 预检查
	checked, err := service.Precheck(context.Background(), flow)
	if err != nil {
		t.Fatal(err)
	}
	if !checked.Prechecked {
		t.Fatalf("expected prechecked to be true")
	}

	// 4. 发布
	published, err := service.Publish(context.Background(), flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !published.Published {
		t.Fatalf("expected published to be true")
	}

	// 5. 分页查询
	page, err := service.Page(context.Background(), operate.AIModelFlowPageRequest{
		PageNumber: 1,
		PageSize:   10,
		Name:       "客服",
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page records: %+v", page)
	}

	// 6. 批量删除
	err = service.Delete(context.Background(), []int{flow.ID})
	if err != nil {
		t.Fatal(err)
	}

	// 验证删除
	_, err = repo.GetByID(context.Background(), flow.ID)
	if !errors.Is(err, operate.ErrAIModelFlowNotFound) {
		t.Fatalf("expected ErrAIModelFlowNotFound, got %v", err)
	}

	// 无效删除校验
	err = service.Delete(context.Background(), []int{})
	if !errors.Is(err, operate.ErrInvalidAIModelFlow) {
		t.Fatalf("expected ErrInvalidAIModelFlow for empty delete, got %v", err)
	}
}
