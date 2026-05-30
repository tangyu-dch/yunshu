package esl_test

import (
	"context"
	"testing"

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
