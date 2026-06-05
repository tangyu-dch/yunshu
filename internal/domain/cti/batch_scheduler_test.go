package cti

import (
	"context"
	"errors"
	"testing"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/infra/events"
)

type fakeBatchRepository struct {
	task          BatchTaskSnapshot
	tel           BatchTelSnapshot
	stats         BatchTaskStats
	err           error
	noPending     bool
	completed     bool
	released      bool
	taskCompleted bool
}

func (r fakeBatchRepository) GetRunnableBatchTask(context.Context, int) (BatchTaskSnapshot, error) {
	if r.err != nil {
		return BatchTaskSnapshot{}, r.err
	}
	return r.task, nil
}

func (r fakeBatchRepository) ClaimNextPendingBatchTel(context.Context, int, time.Time) (BatchTelSnapshot, error) {
	if r.err != nil {
		return BatchTelSnapshot{}, r.err
	}
	if r.noPending {
		return BatchTelSnapshot{}, ErrNoBatchTel
	}
	return r.tel, nil
}

func (r *fakeBatchRepository) CompleteBatchTel(context.Context, int, int, bool, time.Time) error {
	if r.err != nil {
		return r.err
	}
	r.completed = true
	return nil
}

func (r *fakeBatchRepository) ReleaseBatchTel(context.Context, int, int, time.Time) error {
	if r.err != nil {
		return r.err
	}
	r.released = true
	return nil
}

func (r *fakeBatchRepository) CompleteBatchTaskIfDrained(context.Context, int, time.Time) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	r.taskCompleted = true
	return true, nil
}

func (r *fakeBatchRepository) GetBatchTaskStats(context.Context, int) (BatchTaskStats, error) {
	if r.stats.TaskID != 0 {
		return r.stats, nil
	}
	return BatchTaskStats{TaskID: r.task.ID, MerchantID: r.task.MerchantID, UserID: r.task.UserID, TotalCount: 1, CalledCount: 1, CompletedCount: 1}, nil
}

func (r *fakeBatchRepository) GetIdleAgentFromSkillGroup(context.Context, int) (int, string, error) {
	return 0, "", nil
}

func (r *fakeBatchRepository) GetOnlineAgents(context.Context, int) ([]int, error) {
	return []int{1}, nil
}

func (r *fakeBatchRepository) GetActiveCallCount(context.Context, int) (int, error) {
	return 0, nil
}

func (r *fakeBatchRepository) GetAgentSkillGroups(context.Context, int) ([]int, error) {
	return []int{1}, nil
}

type fakeBatchESLClient struct {
	callID string
	req    contracts.BatchCallReq
}

func (c *fakeBatchESLClient) StartAPIOutbound(context.Context, string, string, contracts.ApiCallReq) error {
	return nil
}

func (c *fakeBatchESLClient) StartBatchOutbound(_ context.Context, _ string, callID string, req contracts.BatchCallReq) error {
	c.callID = callID
	c.req = req
	return nil
}

type failingBatchESLClient struct{}

func (f *failingBatchESLClient) StartAPIOutbound(context.Context, string, string, contracts.ApiCallReq) error {
	return nil
}

func (f *failingBatchESLClient) StartBatchOutbound(context.Context, string, string, contracts.BatchCallReq) error {
	return errors.New("batch originate failed")
}

func TestBatchSchedulerDispatchNextPublishesEventAndStartsESL(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	var consumed contracts.EventEnvelope[map[string]any]
	bus.Subscribe(contracts.EventBatchCallRequested, func(_ context.Context, event contracts.EventEnvelope[map[string]any]) error {
		consumed = event
		return nil
	})
	eslClient := &fakeBatchESLClient{}
	scheduler := &BatchSchedulerService{
		Repository: &fakeBatchRepository{
			task: BatchTaskSnapshot{ID: 10, MerchantID: 88, UserID: 7, AIFlag: true, Extra: `{"gatewayName":"gw-sh"}`, ExtensionNumber: "1001", ExtensionID: 5},
			tel:  BatchTelSnapshot{ID: 20, TaskID: 10, MerchantID: 88, UserID: 7, CustomerName: "张三", Tel: "13800138000"},
		},
		ESL:       eslClient,
		Events:    bus,
		NewCallID: func(BatchTaskSnapshot, BatchTelSnapshot) string { return "call-batch-1" },
	}

	req, callID, err := scheduler.DispatchNext(context.Background(), "v1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if callID != "call-batch-1" || eslClient.callID != callID {
		t.Fatalf("unexpected call id: %s client=%s", callID, eslClient.callID)
	}
	if req.BatchTaskID != 10 || req.BatchCallTelID != 20 || req.Extension != "1001" || !req.AIFlag {
		t.Fatalf("unexpected request: %+v", req)
	}
	if consumed.EventType != contracts.EventBatchCallRequested || consumed.AggregateID != callID {
		t.Fatalf("unexpected event: %+v", consumed)
	}
}

func TestBatchSchedulerDispatchNextReleasesTelWhenESLFails(t *testing.T) {
	t.Parallel()

	repo := &fakeBatchRepository{
		task: BatchTaskSnapshot{ID: 10, MerchantID: 88, UserID: 7, AIFlag: true, Extra: `{"gatewayName":"gw-sh"}`, ExtensionNumber: "1001", ExtensionID: 5},
		tel:  BatchTelSnapshot{ID: 20, TaskID: 10, MerchantID: 88, UserID: 7, CustomerName: "张三", Tel: "13800138000"},
	}
	scheduler := &BatchSchedulerService{
		Repository: repo,
		ESL:        &failingBatchESLClient{},
		NewCallID:  func(BatchTaskSnapshot, BatchTelSnapshot) string { return "call-batch-1" },
	}

	_, _, err := scheduler.DispatchNext(context.Background(), "v1", 10)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !repo.released {
		t.Fatalf("expected tel to be released after originate failure")
	}
}

func TestBatchSchedulerDispatchNextRejectsInvalidTask(t *testing.T) {
	t.Parallel()

	scheduler := &BatchSchedulerService{}
	_, _, err := scheduler.DispatchNext(context.Background(), "v1", 0)
	if !errors.Is(err, ErrInvalidBatchTask) {
		t.Fatalf("expected invalid task, got %v", err)
	}
}

func TestBatchSchedulerHandleTerminalCompletesTelAndDispatchesNext(t *testing.T) {
	t.Parallel()

	repo := &fakeBatchRepository{
		task: BatchTaskSnapshot{ID: 10, MerchantID: 88, UserID: 7, ExtensionNumber: "1001"},
		tel:  BatchTelSnapshot{ID: 21, TaskID: 10, MerchantID: 88, UserID: 7, Tel: "13800138001"},
	}
	eslClient := &fakeBatchESLClient{}
	bus := events.NewMemoryBus(nil)
	var completedEvent contracts.EventEnvelope[map[string]any]
	bus.Subscribe(contracts.EventBatchCallTelCompleted, func(_ context.Context, event contracts.EventEnvelope[map[string]any]) error {
		completedEvent = event
		return nil
	})
	scheduler := &BatchSchedulerService{
		Repository: repo,
		ESL:        eslClient,
		Events:     bus,
		NewCallID:  func(BatchTaskSnapshot, BatchTelSnapshot) string { return "batch-next-1" },
	}

	err := scheduler.HandleTerminal(context.Background(), map[string]any{"callId": "batch-call-1", "batchTaskId": 10, "batchCallTelId": 20, "connected": true})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.completed {
		t.Fatalf("expected tel completed")
	}
	if completedEvent.EventType != contracts.EventBatchCallTelCompleted {
		t.Fatalf("unexpected completed event: %+v", completedEvent)
	}
	if eslClient.callID != "batch-next-1" {
		t.Fatalf("expected next dispatch, got %s", eslClient.callID)
	}
}

func TestBatchSchedulerHandleTerminalCompletesTaskWhenNoPendingTel(t *testing.T) {
	t.Parallel()

	repo := &fakeBatchRepository{
		noPending: true,
		stats: BatchTaskStats{
			TaskID:         10,
			MerchantID:     88,
			UserID:         7,
			TotalCount:     5,
			CalledCount:    5,
			PendingCount:   0,
			CallingCount:   0,
			CompletedCount: 5,
			ConnectedCount: 3,
		},
	}
	bus := events.NewMemoryBus(nil)
	var taskCompleted contracts.EventEnvelope[map[string]any]
	bus.Subscribe(contracts.EventBatchCallTaskCompleted, func(_ context.Context, event contracts.EventEnvelope[map[string]any]) error {
		taskCompleted = event
		return nil
	})
	scheduler := &BatchSchedulerService{
		Repository: repo,
		ESL:        &fakeBatchESLClient{},
		Events:     bus,
	}

	err := scheduler.HandleTerminal(context.Background(), map[string]any{"callId": "batch-call-1", "batchTaskId": 10, "batchCallTelId": 20})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.completed || !repo.taskCompleted {
		t.Fatalf("expected tel and task completed, tel=%v task=%v", repo.completed, repo.taskCompleted)
	}
	if taskCompleted.EventType != contracts.EventBatchCallTaskCompleted {
		t.Fatalf("unexpected task completed event: %+v", taskCompleted)
	}
	if taskCompleted.Payload["totalCount"] != 5 || taskCompleted.Payload["connectedCount"] != 3 {
		t.Fatalf("expected stats in task completed payload: %+v", taskCompleted.Payload)
	}
}
