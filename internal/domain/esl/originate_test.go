package esl_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	"yunshu/internal/infra/business"
	"yunshu/pkg/idempotency"
)

type fakeExtensionResolver struct {
	extension esl.Extension
	err       error
}

func (r fakeExtensionResolver) GetByUserID(context.Context, int) (esl.Extension, error) {
	return r.extension, r.err
}

type fakeOutboundGuard struct {
	calls int
	err   error
}

func (g *fakeOutboundGuard) ValidateAPICall(context.Context, contracts.ApiCallReq, esl.Extension) error {
	g.calls++
	return g.err
}

func TestStartAPIOutboundUsesResolvedExtension(t *testing.T) {
	t.Parallel()

	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	service := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Extensions: fakeExtensionResolver{extension: esl.Extension{
			ID:              11,
			UserID:          7,
			MerchantID:      88,
			ExtensionNumber: "2002",
		}},
	}

	err := service.StartAPIOutbound(context.Background(), esl.OriginateRequest{
		Version: "v1",
		CallID:  "call-1",
		Request: contracts.ApiCallReq{UserID: 7, Callee: "13800138000", Extra: `{"extension":"1001"}`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if executor.Count() != 1 {
		t.Fatalf("expected one command, got %d", executor.Count())
	}
	if got := executor.Commands[0].Payload["destination"]; got != "2002" {
		t.Fatalf("expected db extension destination, got %v", got)
	}
}

func TestStartAPIOutboundRunsGuard(t *testing.T) {
	t.Parallel()

	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	guard := &fakeOutboundGuard{}
	service := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Extensions: fakeExtensionResolver{extension: esl.Extension{
			ID:              11,
			UserID:          7,
			MerchantID:      88,
			ExtensionNumber: "2002",
		}},
		Guard: guard,
	}

	err := service.StartAPIOutbound(context.Background(), esl.OriginateRequest{
		Version: "v1",
		CallID:  "call-1",
		Request: contracts.ApiCallReq{UserID: 7, Callee: "13800138000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if guard.calls != 1 {
		t.Fatalf("expected guard called once, got %d", guard.calls)
	}
}

func TestBuildAPIOutboundPlanInjectsRingbackMetadata(t *testing.T) {
	t.Parallel()

	plan := esl.BuildAPIOutboundPlan("call-1", "v1", "10.0.0.1:8021", contracts.ApiCallReq{
		UserID: 7,
		Callee: "13800138000",
		Extra:  `{"extension":"1001","supplementRing":true,"supplementRingFile":"/tmp/ring.wav","broadcastTime":30,"broadcastTimeFlag":true}`,
	}, "", nil)

	if !plan.SupplementRing || plan.SupplementRingFile != "/tmp/ring.wav" {
		t.Fatalf("unexpected ringback metadata: %+v", plan)
	}
	if plan.Options["ringback"] != "/tmp/ring.wav" || plan.Options["variable_yunshu_ringback_file"] != "/tmp/ring.wav" {
		t.Fatalf("expected ringback options, got %+v", plan.Options)
	}
	if plan.Options["variable_yunshu_broadcast_time"] != int64(30) || plan.Options["variable_yunshu_broadcast_time_flag"] != true {
		t.Fatalf("expected broadcast options, got %+v", plan.Options)
	}
}

func TestStartBatchOutboundCreatesCustomerFirstCommand(t *testing.T) {
	t.Parallel()

	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	service := &esl.OriginateService{CommandService: command, SessionService: session}

	err := service.StartBatchOutbound(context.Background(), esl.BatchOriginateRequest{
		Version: "v1",
		CallID:  "call-batch-1",
		Request: contracts.BatchCallReq{
			UserID:         7,
			BatchTaskID:    10,
			BatchCallTelID: 20,
			Phone:          "13800138000",
			Extension:      "1001",
			MerchantID:     88,
			Extra:          `{"gatewayName":"gw-sh"}`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if executor.Count() != 1 {
		t.Fatalf("expected one command, got %d", executor.Count())
	}
	cmd := executor.Commands[0]
	if cmd.Profile != contracts.CallFlowBatchOutbound || cmd.LegRole != contracts.LegRoleCustomer {
		t.Fatalf("unexpected command: %+v", cmd)
	}
	if got := cmd.Payload["originateMode"]; got != contracts.OriginateModeCustomerFirst {
		t.Fatalf("unexpected mode: %v", got)
	}
	sessionState, err := session.Store.Get(context.Background(), "call-batch-1")
	if err != nil {
		t.Fatal(err)
	}
	if sessionState.Metadata["batchTaskId"] != 10 || sessionState.Metadata["batchCallTelId"] != 20 {
		t.Fatalf("unexpected session metadata: %+v", sessionState.Metadata)
	}
}

func TestBuildBatchOutboundPlanInjectsRingbackMetadata(t *testing.T) {
	t.Parallel()

	plan := esl.BuildBatchOutboundPlan("call-batch-1", "v1", "10.0.0.1:8021", contracts.BatchCallReq{
		UserID:         7,
		BatchTaskID:    10,
		BatchCallTelID: 20,
		Phone:          "13800138000",
		Extension:      "1001",
		MerchantID:     88,
		Extra:          `{"gatewayName":"gw-sh","supplement_ring":true,"supplement_ring_file":"/tmp/ring.wav"}`,
	}, nil)

	if !plan.SupplementRing || plan.SupplementRingFile != "/tmp/ring.wav" {
		t.Fatalf("unexpected ringback metadata: %+v", plan)
	}
	if plan.Options["ringback"] != "/tmp/ring.wav" || plan.Options["variable_yunshu_ringback_file"] != "/tmp/ring.wav" {
		t.Fatalf("expected ringback options, got %+v", plan.Options)
	}
}

type fakeConcurrencyLimiter struct {
	maxCalls int
}

func (l *fakeConcurrencyLimiter) CheckConcurrencyLimit(ctx context.Context, activeCount int) error {
	if activeCount >= l.maxCalls {
		return fmt.Errorf("concurrency limit exceeded: max %d, active %d", l.maxCalls, activeCount)
	}
	return nil
}

func (l *fakeConcurrencyLimiter) CheckFeatureLimit(ctx context.Context, feature string) error {
	return nil
}

func TestOriginateServiceConcurrencyLimiter(t *testing.T) {
	t.Parallel()

	// 1. Setup limiter with max 2 concurrent calls
	limiter := &fakeConcurrencyLimiter{maxCalls: 2}

	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	sessionStore := esl.NewMemorySessionStore()
	session := esl.NewSessionService(sessionStore, business.NewOutboxMemoryStore(), nil)

	service := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Limiter:        limiter,
		Extensions: fakeExtensionResolver{extension: esl.Extension{
			ID:              11,
			UserID:          7,
			MerchantID:      88,
			ExtensionNumber: "2002",
		}},
	}

	// 2. Add 2 active call sessions to the store
	err := sessionStore.Save(context.Background(), esl.CallSession{
		CallID: "active-call-1",
		State:  esl.CallAnswered,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = sessionStore.Save(context.Background(), esl.CallSession{
		CallID: "active-call-2",
		State:  esl.CallBridged,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Active count is now 2. Starting a new call should be rejected since max is 2.
	err = service.StartAPIOutbound(context.Background(), esl.OriginateRequest{
		Version: "v1",
		CallID:  "new-call-3",
		Request: contracts.ApiCallReq{UserID: 7, Callee: "13800138000"},
	})
	if err == nil {
		t.Fatal("expected error due to concurrency limit, got nil")
	}

	// 3. Mark one call completed. Active count becomes 1.
	err = sessionStore.Save(context.Background(), esl.CallSession{
		CallID:      "active-call-1",
		State:       esl.CallComplete,
		CompletedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Starting a new call should now succeed.
	err = service.StartAPIOutbound(context.Background(), esl.OriginateRequest{
		Version: "v1",
		CallID:  "new-call-3",
		Request: contracts.ApiCallReq{UserID: 7, Callee: "13800138000"},
	})
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
}

func TestOriginateServiceConcurrencyLimiterDiscountSelf(t *testing.T) {
	t.Parallel()

	// Concurrency limit = 2.
	limiter := &fakeConcurrencyLimiter{maxCalls: 2}

	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	sessionStore := esl.NewMemorySessionStore()
	session := esl.NewSessionService(sessionStore, business.NewOutboxMemoryStore(), nil)

	service := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Limiter:        limiter,
	}

	// Add 1 unrelated active call
	err := sessionStore.Save(context.Background(), esl.CallSession{
		CallID: "active-call-unrelated",
		State:  esl.CallAnswered,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add current call session as active (e.g., dialpad agent answered)
	err = sessionStore.Save(context.Background(), esl.CallSession{
		CallID: "current-call",
		State:  esl.CallAnswered,
		Metadata: map[string]any{
			"agentUuid":    "agent-uuid",
			"customerUuid": "customer-uuid",
			"callee":       "13800138000",
			"userId":       7,
			"merchantId":   88,
			"extension":    "1001",
		},
		FSAddr: "default",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Total active calls in store = 2 (unrelated + current-call).
	// But since "current-call" is the call we are originating the customer leg for,
	// it should be discounted by 1.
	// Thus, verified active count = 1 < 2, so it should succeed.
	err = service.StartDialpadCustomerOutbound(context.Background(), esl.APICustomerOriginateRequest{
		Version: "v1",
		CallID:  "current-call",
		Selection: contracts.SelectPhoneResp{
			Phone:         "13800000000",
			GatewayID:     1,
			GatewayName:   "gw-1",
			GatewayRegion: "sh",
			Model:         1,
		},
	})
	if err != nil {
		t.Fatalf("expected success with self-discount, got err: %v", err)
	}
}
