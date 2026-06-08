package callflow

import (
	"context"
	"testing"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/cti"
	"yunshu/internal/domain/esl"
	"yunshu/internal/infra/business"
	"yunshu/internal/infra/events"
	"yunshu/pkg/idempotency"
	"yunshu/pkg/workflow"
)

func TestConsumersAdvanceAPIOutboundWorkflows(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), nil)
	RegisterConsumers(context.Background(), bus, ctiRunner, eslRunner, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-1", contracts.EventAPICallRequested, "idem-1", "call", "call-1", contracts.ServiceCall, map[string]any{"callId": "call-1"})); err != nil {
		t.Fatal(err)
	}
	ctiInstance, err := ctiRunner.Store.Get(ctx, cti.WorkflowAPIOutbound, "call-1")
	if err != nil {
		t.Fatal(err)
	}
	if ctiInstance.State != "number_selected" {
		t.Fatalf("cti state got %s", ctiInstance.State)
	}

	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-2", contracts.EventESLCommandSent, "idem-2", "call", "call-1", contracts.ServiceCall, map[string]any{"callId": "call-1"})); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-3", contracts.EventFSApplied, "idem-3", "call", "call-1", contracts.ServiceCall, map[string]any{"callId": "call-1", "eventName": "CHANNEL_CREATE"})); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-4", contracts.EventFSApplied, "idem-4", "call", "call-1", contracts.ServiceCall, map[string]any{"callId": "call-1", "eventName": "CHANNEL_HANGUP_COMPLETE"})); err != nil {
		t.Fatal(err)
	}
	eslInstance, err := eslRunner.Store.Get(ctx, esl.WorkflowESLAPIOutbound, "call-1")
	if err != nil {
		t.Fatal(err)
	}
	if eslInstance.State != "complete" {
		t.Fatalf("esl state got %s", eslInstance.State)
	}
}

func TestConsumersAdvanceBatchOutboundWorkflows(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), nil)
	RegisterConsumers(context.Background(), bus, ctiRunner, eslRunner, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-batch-1", contracts.EventBatchCallRequested, "idem-batch-1", "call", "batch-call-1", contracts.ServiceCall, map[string]any{"callId": "batch-call-1", "batchTaskId": 10, "batchCallTelId": 20})); err != nil {
		t.Fatal(err)
	}
	ctiInstance, err := ctiRunner.Store.Get(ctx, cti.WorkflowBatchOutbound, "batch-call-1")
	if err != nil {
		t.Fatal(err)
	}
	if ctiInstance.State != "originating" {
		t.Fatalf("cti batch state got %s", ctiInstance.State)
	}

	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-batch-2", contracts.EventESLCommandSent, "idem-batch-2", "call", "batch-call-1", contracts.ServiceCall, map[string]any{"callId": "batch-call-1", "profile": string(contracts.CallFlowBatchOutbound), "batchTaskId": 10})); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-batch-3", contracts.EventFSApplied, "idem-batch-3", "call", "batch-call-1", contracts.ServiceCall, map[string]any{"callId": "batch-call-1", "profile": string(contracts.CallFlowBatchOutbound), "eventName": "CHANNEL_CREATE"})); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-batch-4", contracts.EventFSApplied, "idem-batch-4", "call", "batch-call-1", contracts.ServiceCall, map[string]any{"callId": "batch-call-1", "profile": string(contracts.CallFlowBatchOutbound), "eventName": "CHANNEL_HANGUP_COMPLETE"})); err != nil {
		t.Fatal(err)
	}
	eslInstance, err := eslRunner.Store.Get(ctx, esl.WorkflowESLBatchOutbound, "batch-call-1")
	if err != nil {
		t.Fatal(err)
	}
	if eslInstance.State != "complete" {
		t.Fatalf("esl batch state got %s", eslInstance.State)
	}
}

func TestBatchConsumersHandleTerminalAndDispatchNext(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	repo := &terminalBatchRepository{
		task: cti.BatchTaskSnapshot{ID: 10, MerchantID: 88, UserID: 7, ExtensionNumber: "1001"},
		tel:  cti.BatchTelSnapshot{ID: 21, TaskID: 10, MerchantID: 88, UserID: 7, Tel: "13800138001"},
	}
	eslClient := &terminalBatchESLClient{}
	scheduler := &cti.BatchSchedulerService{
		Repository: repo,
		ESL:        eslClient,
		Events:     bus,
		NewCallID:  func(cti.BatchTaskSnapshot, cti.BatchTelSnapshot) string { return "batch-next-1" },
	}
	RegisterBatchConsumers(bus, ctiRunner, scheduler, nil)

	ctx := context.Background()
	if _, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, "batch-call-1", workflow.Event{Name: "acquire_slot", Payload: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, "batch-call-1", workflow.Event{Name: "select_number", Payload: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, "batch-call-1", workflow.Event{Name: "dispatch_originate", Payload: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-terminal-1", contracts.EventFSApplied, "idem-terminal-1", "call", "batch-call-1", contracts.ServiceCall, map[string]any{
		"callId":         "batch-call-1",
		"profile":        string(contracts.CallFlowBatchOutbound),
		"eventName":      "CHANNEL_HANGUP_COMPLETE",
		"batchTaskId":    10,
		"batchCallTelId": 20,
	})); err != nil {
		t.Fatal(err)
	}
	instance, err := ctiRunner.Store.Get(ctx, cti.WorkflowBatchOutbound, "batch-call-1")
	if err != nil {
		t.Fatal(err)
	}
	if instance.State != "list_finished" {
		t.Fatalf("batch workflow state got %s", instance.State)
	}
	if !repo.completed || eslClient.callID != "batch-next-1" {
		t.Fatalf("expected completion and next dispatch, completed=%v next=%s", repo.completed, eslClient.callID)
	}
}

func TestAPIOutboundBridgesOnlyAfterCustomer183OrAnswer(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), nil)
	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	originate := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Events:         bus,
		Extensions: apiOutboundTestExtensionResolver{extension: esl.Extension{
			ID:              11,
			UserID:          7,
			MerchantID:      88,
			ExtensionNumber: "2002",
		}},
	}
	candidateSource := apiOutboundTestCandidateSource{candidates: []cti.NumberCandidate{{
		Phone:         "13900000000",
		GatewayID:     "1",
		GatewayName:   "gw1",
		GatewayRegion: "10.0.0.1:5060",
		Model:         0,
		Available:     true,
		RiskAllowed:   true,
		Concurrency:   1,
		WhitelistHit:  true,
	}}}
	runtimeSelector := &cti.RuntimeSelector{Allocator: apiOutboundRuntimeAllocator{}}
	RegisterConsumers(context.Background(), bus, ctiRunner, eslRunner, session, originate, runtimeSelector, candidateSource, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()
	if err := originate.StartAPIOutbound(ctx, esl.OriginateRequest{
		Version: "v1",
		CallID:  "api-bridge-1",
		Request: contracts.ApiCallReq{UserID: 7, Callee: "13800138000"},
	}); err != nil {
		t.Fatal(err)
	}
	if got := executor.Count(); got != 1 {
		t.Fatalf("expected one agent originate command, got %d", got)
	}
	sessionState, err := session.Store.Get(ctx, "api-bridge-1")
	if err != nil {
		t.Fatal(err)
	}
	if sessionState.Metadata["routeVersion"] != "v1" {
		t.Fatalf("expected route version to be persisted, got %+v", sessionState.Metadata)
	}

	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-0", contracts.EventFSApplied, "idem-api-0", "call", "api-bridge-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-bridge-1",
		"eventName": string(esl.EventChannelCreate),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-1", contracts.EventFSApplied, "idem-api-1", "call", "api-bridge-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-bridge-1",
		"eventName": string(esl.EventChannelProgress),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}
	if got := executor.Count(); got != 2 {
		t.Fatalf("expected customer originate after agent progress, got %d", got)
	}

	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-2", contracts.EventFSApplied, "idem-api-2", "call", "api-bridge-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-bridge-1",
		"eventName": string(esl.EventChannelAnswer),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}
	if got := executor.Count(); got != 2 {
		t.Fatalf("expected no bridge before customer ready, got %d", got)
	}

	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-3", contracts.EventFSApplied, "idem-api-3", "call", "api-bridge-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-bridge-1",
		"eventName": string(esl.EventChannelProgress),
		"legRole":   string(contracts.LegRoleCustomer),
	})); err != nil {
		t.Fatal(err)
	}
	if got := executor.Count(); got != 2 {
		t.Fatalf("expected 180 on customer leg not to bridge, got %d", got)
	}

	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-4", contracts.EventFSApplied, "idem-api-4", "call", "api-bridge-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-bridge-1",
		"eventName": string(esl.EventChannelProgressMedia),
		"legRole":   string(contracts.LegRoleCustomer),
	})); err != nil {
		t.Fatal(err)
	}
	if got := executor.Count(); got != 3 {
		t.Fatalf("expected bridge after customer 183, got %d", got)
	}
	bridge := executor.Commands[2]
	if bridge.Command != "bridge" {
		t.Fatalf("expected bridge command, got %+v", bridge)
	}
}

type terminalBatchRepository struct {
	task          cti.BatchTaskSnapshot
	tel           cti.BatchTelSnapshot
	completed     bool
	taskCompleted bool
}

func (r *terminalBatchRepository) GetRunnableBatchTask(context.Context, int) (cti.BatchTaskSnapshot, error) {
	return r.task, nil
}

func (r *terminalBatchRepository) ClaimNextPendingBatchTel(context.Context, int, time.Time) (cti.BatchTelSnapshot, error) {
	return r.tel, nil
}

func (r *terminalBatchRepository) CompleteBatchTel(context.Context, int, int, bool, time.Time) error {
	r.completed = true
	return nil
}

func (r *terminalBatchRepository) ReleaseBatchTel(context.Context, int, int, time.Time) error {
	return nil
}

func (r *terminalBatchRepository) CompleteBatchTaskIfDrained(context.Context, int, time.Time) (bool, error) {
	r.taskCompleted = true
	return true, nil
}

func (r *terminalBatchRepository) GetIdleAgentFromSkillGroup(context.Context, int) (int, string, error) {
	return 0, "", nil
}

func (r *terminalBatchRepository) GetOnlineAgents(context.Context, int) ([]int, error) {
	return []int{1}, nil
}

func (r *terminalBatchRepository) GetActiveCallCount(context.Context, int) (int, error) {
	return 0, nil
}

func (r *terminalBatchRepository) GetAgentSkillGroups(context.Context, int) ([]int, error) {
	return []int{1}, nil
}

type terminalBatchESLClient struct {
	callID string
}

func (c *terminalBatchESLClient) StartBatchOutbound(_ context.Context, _ string, callID string, _ contracts.BatchCallReq) error {
	c.callID = callID
	return nil
}

type apiOutboundTestCandidateSource struct {
	candidates []cti.NumberCandidate
}

func (s apiOutboundTestCandidateSource) CandidatesForUser(context.Context, int) ([]cti.NumberCandidate, error) {
	return append([]cti.NumberCandidate(nil), s.candidates...), nil
}

type apiOutboundTestExtensionResolver struct {
	extension esl.Extension
}

func (r apiOutboundTestExtensionResolver) GetByUserID(context.Context, int) (esl.Extension, error) {
	return r.extension, nil
}

type apiOutboundRuntimeAllocator struct{}

func (apiOutboundRuntimeAllocator) Claim(_ context.Context, req cti.SelectionRequest, candidates []cti.NumberCandidate) (cti.RuntimeAllocation, error) {
	if len(candidates) == 0 {
		return cti.RuntimeAllocation{}, cti.ErrNoAvailableNumber
	}
	candidate := candidates[0]
	return cti.RuntimeAllocation{
		CallID:     req.CallID,
		MerchantID: req.MerchantID,
		Caller:     candidate.Phone,
		GatewayID:  candidate.GatewayID,
		ClaimKey:   "claim:" + req.CallID,
	}, nil
}

func (apiOutboundRuntimeAllocator) Release(context.Context, cti.RuntimeAllocation) error {
	return nil
}

func TestAPIOutboundPlaysSupplementaryRingOnCustomerProgress(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), nil)
	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	originate := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Events:         bus,
		Extensions: apiOutboundTestExtensionResolver{extension: esl.Extension{
			ID:              11,
			UserID:          7,
			MerchantID:      88,
			ExtensionNumber: "2002",
		}},
	}
	candidateSource := apiOutboundTestCandidateSource{candidates: []cti.NumberCandidate{{
		Phone:              "13900000000",
		GatewayID:          "1",
		GatewayName:        "gw1",
		GatewayRegion:      "10.0.0.1:5060",
		Model:              0,
		Available:          true,
		RiskAllowed:        true,
		Concurrency:        1,
		WhitelistHit:       true,
		SupplementRing:     true,
		SupplementRingFile: "/tmp/ring.wav",
	}}}
	runtimeSelector := &cti.RuntimeSelector{Allocator: apiOutboundRuntimeAllocator{}}
	RegisterConsumers(context.Background(), bus, ctiRunner, eslRunner, session, originate, runtimeSelector, candidateSource, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()
	// Start API Outbound (Agent first)
	if err := originate.StartAPIOutbound(ctx, esl.OriginateRequest{
		Version: "v1",
		CallID:  "api-ring-1",
		Request: contracts.ApiCallReq{UserID: 7, Callee: "13800138000"},
	}); err != nil {
		t.Fatal(err)
	}

	// 1. Agent leg created
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-0", contracts.EventFSApplied, "idem-api-0", "call", "api-ring-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-ring-1",
		"eventName": string(esl.EventChannelCreate),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}

	// 2. Agent leg progress -> triggers customer originate
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-1", contracts.EventFSApplied, "idem-api-1", "call", "api-ring-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-ring-1",
		"eventName": string(esl.EventChannelProgress),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}

	// 3. Agent leg answers
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-2", contracts.EventFSApplied, "idem-api-2", "call", "api-ring-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-ring-1",
		"eventName": string(esl.EventChannelAnswer),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}

	// Make sure we have originated the customer leg
	if executor.Count() != 2 {
		t.Fatalf("expected 2 commands (agent originate + customer originate), got %d", executor.Count())
	}

	// Force session state to match progress (as the test bypasses ApplyEvent)
	sess, err := session.Store.Get(ctx, "api-ring-1")
	if err != nil {
		t.Fatal(err)
	}
	sess.State = esl.CallProgress
	if err := session.Store.Save(ctx, sess); err != nil {
		t.Fatal(err)
	}

	// 4. Customer leg progress (180 Ringing) -> triggers supplement ring playback
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-3", contracts.EventFSApplied, "idem-api-3", "call", "api-ring-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-ring-1",
		"eventName": string(esl.EventChannelProgress),
		"legRole":   string(contracts.LegRoleCustomer),
	})); err != nil {
		t.Fatal(err)
	}

	// Check if playback command was sent to agent
	if got := executor.Count(); got != 3 {
		t.Fatalf("expected 3 commands (originate, originate, playback), got %d", got)
	}
	playCmd := executor.Commands[2]
	if playCmd.Command != "playback" || playCmd.UUID != sess.Metadata["agentUuid"] || playCmd.Payload["file"] != "/tmp/ring.wav" {
		t.Fatalf("unexpected playback command details: %+v", playCmd)
	}

	// 5. Customer leg answers -> stops playback and bridges
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-4", contracts.EventFSApplied, "idem-api-4", "call", "api-ring-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-ring-1",
		"eventName": string(esl.EventChannelAnswer),
		"legRole":   string(contracts.LegRoleCustomer),
	})); err != nil {
		t.Fatal(err)
	}

	if got := executor.Count(); got != 5 {
		t.Fatalf("expected 5 commands (originate, originate, playback, break, bridge), got %d", got)
	}
	breakCmd := executor.Commands[3]
	if breakCmd.Command != "break" || breakCmd.UUID != sess.Metadata["agentUuid"] {
		t.Fatalf("expected break command, got %+v", breakCmd)
	}
	bridgeCmd := executor.Commands[4]
	if bridgeCmd.Command != "bridge" {
		t.Fatalf("expected bridge command, got %+v", bridgeCmd)
	}
}

func TestBatchOutboundSymmetricFlow(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), nil)
	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	originate := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Events:         bus,
	}
	RegisterConsumers(context.Background(), bus, ctiRunner, eslRunner, session, originate, nil, nil, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()
	// Start Batch Outbound (Customer first)
	err = originate.StartBatchOutbound(ctx, esl.BatchOriginateRequest{
		Version: "v1",
		CallID:  "batch-sym-1",
		Request: contracts.BatchCallReq{
			UserID:         7,
			BatchTaskID:    10,
			BatchCallTelID: 20,
			Phone:          "13800138000",
			Extension:      "1001",
			MerchantID:     88,
			Extra:          `{"supplement_ring":true,"supplement_ring_file":"/tmp/ring.wav"}`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if executor.Count() != 1 {
		t.Fatalf("expected one command (customer originate), got %d", executor.Count())
	}

	// 1. Customer leg created
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-b-0", contracts.EventFSApplied, "idem-b-0", "call", "batch-sym-1", contracts.ServiceCall, map[string]any{
		"callId":    "batch-sym-1",
		"profile":   string(contracts.CallFlowBatchOutbound),
		"eventName": string(esl.EventChannelCreate),
		"legRole":   string(contracts.LegRoleCustomer),
	})); err != nil {
		t.Fatal(err)
	}

	// 2. Customer leg answers -> triggers agent originate
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-b-1", contracts.EventFSApplied, "idem-b-1", "call", "batch-sym-1", contracts.ServiceCall, map[string]any{
		"callId":    "batch-sym-1",
		"profile":   string(contracts.CallFlowBatchOutbound),
		"eventName": string(esl.EventChannelAnswer),
		"legRole":   string(contracts.LegRoleCustomer),
	})); err != nil {
		t.Fatal(err)
	}

	if executor.Count() != 2 {
		t.Fatalf("expected customer originate + agent originate, got %d", executor.Count())
	}
	agentOrigCmd := executor.Commands[1]
	if agentOrigCmd.Command != "originate" || agentOrigCmd.LegRole != contracts.LegRoleAgent {
		t.Fatalf("unexpected agent originate command: %+v", agentOrigCmd)
	}

	sess, err := session.Store.Get(ctx, "batch-sym-1")
	if err != nil {
		t.Fatal(err)
	}

	// 3. Agent leg created
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-b-2", contracts.EventFSApplied, "idem-b-2", "call", "batch-sym-1", contracts.ServiceCall, map[string]any{
		"callId":    "batch-sym-1",
		"profile":   string(contracts.CallFlowBatchOutbound),
		"eventName": string(esl.EventChannelCreate),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}

	// 4. Agent leg progress (180 Ringing) -> triggers supplement ring playback on customer leg
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-b-3", contracts.EventFSApplied, "idem-b-3", "call", "batch-sym-1", contracts.ServiceCall, map[string]any{
		"callId":    "batch-sym-1",
		"profile":   string(contracts.CallFlowBatchOutbound),
		"eventName": string(esl.EventChannelProgress),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}

	if got := executor.Count(); got != 3 {
		t.Fatalf("expected 3 commands (customer originate, agent originate, playback), got %d", got)
	}
	playCmd := executor.Commands[2]
	if playCmd.Command != "playback" || playCmd.UUID != sess.Metadata["customerUuid"] || playCmd.Payload["file"] != "/tmp/ring.wav" {
		t.Fatalf("unexpected playback command details: %+v", playCmd)
	}

	// 5. Agent leg answers -> stops playback and bridges
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-b-4", contracts.EventFSApplied, "idem-b-4", "call", "batch-sym-1", contracts.ServiceCall, map[string]any{
		"callId":    "batch-sym-1",
		"profile":   string(contracts.CallFlowBatchOutbound),
		"eventName": string(esl.EventChannelAnswer),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}

	if got := executor.Count(); got != 5 {
		t.Fatalf("expected 5 commands (customer originate, agent originate, playback, break, bridge), got %d", got)
	}
	breakCmd := executor.Commands[3]
	if breakCmd.Command != "break" || breakCmd.UUID != sess.Metadata["customerUuid"] {
		t.Fatalf("expected break command, got %+v", breakCmd)
	}
	bridgeCmd := executor.Commands[4]
	if bridgeCmd.Command != "bridge" {
		t.Fatalf("expected bridge command, got %+v", bridgeCmd)
	}
}

func TestCTIPostCallWorkflowTransitions(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)

	RegisterConsumers(context.Background(), bus, ctiRunner, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()

	t.Run("API Outbound post-call flow", func(t *testing.T) {
		callID := "api-post-call-1"
		//received -> validate -> validated -> select_number -> number_selected -> dispatch_originate -> originating -> terminal_event -> finished
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowAPIOutbound, callID, workflow.Event{Name: "validate", Payload: map[string]any{}}); err != nil {
			t.Fatal(err)
		}
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowAPIOutbound, callID, workflow.Event{Name: "select_number", Payload: map[string]any{}}); err != nil {
			t.Fatal(err)
		}
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowAPIOutbound, callID, workflow.Event{Name: "dispatch_originate", Payload: map[string]any{}}); err != nil {
			t.Fatal(err)
		}
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowAPIOutbound, callID, workflow.Event{Name: "terminal_event", Payload: map[string]any{}}); err != nil {
			t.Fatal(err)
		}

		inst, err := ctiRunner.Store.Get(ctx, cti.WorkflowAPIOutbound, callID)
		if err != nil {
			t.Fatal(err)
		}
		if inst.State != "finished" {
			t.Fatalf("expected state finished, got %s", inst.State)
		}

		// 1. cdr_persisted -> cdr_finalized
		if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-1", "cdr_persisted", "idem-1", "call", callID, contracts.ServiceWorker, map[string]any{})); err != nil {
			t.Fatal(err)
		}
		inst, _ = ctiRunner.Store.Get(ctx, cti.WorkflowAPIOutbound, callID)
		if inst.State != "cdr_finalized" {
			t.Fatalf("expected cdr_finalized, got %s", inst.State)
		}

		// 2. billing_completed -> billing_done
		if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-2", "billing_completed", "idem-2", "call", callID, contracts.ServiceWorker, map[string]any{})); err != nil {
			t.Fatal(err)
		}
		inst, _ = ctiRunner.Store.Get(ctx, cti.WorkflowAPIOutbound, callID)
		if inst.State != "billing_done" {
			t.Fatalf("expected billing_done, got %s", inst.State)
		}

		// 3. recording_completed -> recording_done
		if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-3", "recording_completed", "idem-3", "call", callID, contracts.ServiceWorker, map[string]any{})); err != nil {
			t.Fatal(err)
		}
		inst, _ = ctiRunner.Store.Get(ctx, cti.WorkflowAPIOutbound, callID)
		if inst.State != "recording_done" {
			t.Fatalf("expected recording_done, got %s", inst.State)
		}

		// 4. push_completed -> ended
		if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-4", "push_completed", "idem-4", "call", callID, contracts.ServiceWorker, map[string]any{})); err != nil {
			t.Fatal(err)
		}
		inst, _ = ctiRunner.Store.Get(ctx, cti.WorkflowAPIOutbound, callID)
		if inst.State != "ended" {
			t.Fatalf("expected ended, got %s", inst.State)
		}
	})

	t.Run("Batch Outbound post-call flow", func(t *testing.T) {
		callID := "batch-post-call-1"
		payload := map[string]any{"batchTaskId": 123}
		// task_ready -> acquire_slot -> slot_acquired -> select_number -> number_selected -> dispatch_originate -> originating -> terminal_event -> list_finished
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, callID, workflow.Event{Name: "acquire_slot", Payload: payload}); err != nil {
			t.Fatal(err)
		}
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, callID, workflow.Event{Name: "select_number", Payload: payload}); err != nil {
			t.Fatal(err)
		}
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, callID, workflow.Event{Name: "dispatch_originate", Payload: payload}); err != nil {
			t.Fatal(err)
		}
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, callID, workflow.Event{Name: "terminal_event", Payload: payload}); err != nil {
			t.Fatal(err)
		}

		inst, err := ctiRunner.Store.Get(ctx, cti.WorkflowBatchOutbound, callID)
		if err != nil {
			t.Fatal(err)
		}
		if inst.State != "list_finished" {
			t.Fatalf("expected state list_finished, got %s", inst.State)
		}

		// 1. cdr_persisted -> cdr_finalized
		if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-5", "cdr_persisted", "idem-5", "call", callID, contracts.ServiceWorker, payload)); err != nil {
			t.Fatal(err)
		}
		inst, _ = ctiRunner.Store.Get(ctx, cti.WorkflowBatchOutbound, callID)
		if inst.State != "cdr_finalized" {
			t.Fatalf("expected cdr_finalized, got %s", inst.State)
		}

		// 2. billing_completed -> billing_done
		if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-6", "billing_completed", "idem-6", "call", callID, contracts.ServiceWorker, payload)); err != nil {
			t.Fatal(err)
		}
		inst, _ = ctiRunner.Store.Get(ctx, cti.WorkflowBatchOutbound, callID)
		if inst.State != "billing_done" {
			t.Fatalf("expected billing_done, got %s", inst.State)
		}

		// 3. recording_completed -> recording_done
		if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-7", "recording_completed", "idem-7", "call", callID, contracts.ServiceWorker, payload)); err != nil {
			t.Fatal(err)
		}
		inst, _ = ctiRunner.Store.Get(ctx, cti.WorkflowBatchOutbound, callID)
		if inst.State != "recording_done" {
			t.Fatalf("expected recording_done, got %s", inst.State)
		}

		// 4. callback_completed -> finished
		if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-8", "callback_completed", "idem-8", "call", callID, contracts.ServiceWorker, payload)); err != nil {
			t.Fatal(err)
		}
		inst, _ = ctiRunner.Store.Get(ctx, cti.WorkflowBatchOutbound, callID)
		if inst.State != "finished" {
			t.Fatalf("expected finished, got %s", inst.State)
		}
	})
}

type testSessionSniffer struct {
	extension     esl.Extension
	didMerchantID int
	didMatch      bool
}

func (s testSessionSniffer) IsExtension(ctx context.Context, number string) (bool, *esl.Extension, error) {
	if number == s.extension.ExtensionNumber {
		return true, &s.extension, nil
	}
	return false, nil, nil
}

func (s testSessionSniffer) IsMerchantDID(ctx context.Context, number string) (bool, int, error) {
	if s.didMatch {
		return true, s.didMerchantID, nil
	}
	return false, 0, nil
}

func TestDialpadDirectCallSymmetricFlow(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), nil)

	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	session.Events = bus
	sniffer := testSessionSniffer{
		extension: esl.Extension{
			ID:              11,
			UserID:          7,
			MerchantID:      88,
			ExtensionNumber: "2002",
		},
	}
	session.Sniffer = sniffer

	originate := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Events:         bus,
		Extensions:     apiOutboundTestExtensionResolver{extension: sniffer.extension},
	}
	candidateSource := apiOutboundTestCandidateSource{candidates: []cti.NumberCandidate{{
		Phone:         "13900000000",
		GatewayID:     "1",
		GatewayName:   "gw1",
		GatewayRegion: "10.0.0.1:5060",
		Available:     true,
		RiskAllowed:   true,
		Concurrency:   1,
		WhitelistHit:  true,
	}}}
	runtimeSelector := &cti.RuntimeSelector{Allocator: apiOutboundRuntimeAllocator{}}

	RegisterConsumers(context.Background(), bus, ctiRunner, eslRunner, session, originate, runtimeSelector, candidateSource, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()

	// 1. Physical agent call created (CHANNEL_CREATE)
	evt1 := contracts.TelephonyEvent{
		EventID:   "evt-1",
		EventName: "CHANNEL_CREATE",
		CallID:    "direct-call-1",
		UUID:      "agent-uuid-1",
		LegRole:   contracts.LegRoleAgent,
		FSAddr:    "127.0.0.1:8021",
		Headers: map[string]any{
			"callerNumber": "2002",
			"calleeNumber": "13800138000",
		},
	}
	sess, err := session.ApplyEvent(ctx, evt1)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Profile != contracts.CallFlowAPIDirect {
		t.Fatalf("expected profile api_direct, got %s", sess.Profile)
	}

	// 2. Physical agent answers (CHANNEL_ANSWER) -> triggers customer originate
	evt2 := contracts.TelephonyEvent{
		EventID:   "evt-2",
		EventName: "CHANNEL_ANSWER",
		CallID:    "direct-call-1",
		UUID:      "agent-uuid-1",
		LegRole:   contracts.LegRoleAgent,
		FSAddr:    "127.0.0.1:8021",
	}
	_, err = session.ApplyEvent(ctx, evt2)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that customer originate was triggered
	if executor.Count() != 1 {
		t.Fatalf("expected customer originate command, got %d", executor.Count())
	}
	origCmd := executor.Commands[0]
	if origCmd.Command != "originate" || origCmd.LegRole != contracts.LegRoleCustomer {
		t.Fatalf("unexpected command details: %+v", origCmd)
	}

	// For helper synchronization in tests, retrieve customer leg uuid
	sess, _ = session.Store.Get(ctx, "direct-call-1")
	customerUUID := sess.Metadata["customerUuid"].(string)

	// 3. Physical customer leg created (CHANNEL_CREATE)
	evt3 := contracts.TelephonyEvent{
		EventID:   "evt-3",
		EventName: "CHANNEL_CREATE",
		CallID:    "direct-call-1",
		UUID:      customerUUID,
		LegRole:   contracts.LegRoleCustomer,
		FSAddr:    "127.0.0.1:8021",
	}
	_, err = session.ApplyEvent(ctx, evt3)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Physical customer answers (CHANNEL_ANSWER) -> triggers bridge
	evt4 := contracts.TelephonyEvent{
		EventID:   "evt-4",
		EventName: "CHANNEL_ANSWER",
		CallID:    "direct-call-1",
		UUID:      customerUUID,
		LegRole:   contracts.LegRoleCustomer,
		FSAddr:    "127.0.0.1:8021",
	}
	_, err = session.ApplyEvent(ctx, evt4)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that bridge command was executed
	if executor.Count() != 2 {
		t.Fatalf("expected bridge command, got %d", executor.Count())
	}
	bridgeCmd := executor.Commands[1]
	if bridgeCmd.Command != "bridge" {
		t.Fatalf("expected bridge command, got %+v", bridgeCmd)
	}

	// 5. Physical channel hangup (CHANNEL_HANGUP_COMPLETE) -> completes ESL flow
	evt5 := contracts.TelephonyEvent{
		EventID:   "evt-5",
		EventName: "CHANNEL_HANGUP_COMPLETE",
		CallID:    "direct-call-1",
		UUID:      customerUUID,
		LegRole:   contracts.LegRoleCustomer,
		FSAddr:    "127.0.0.1:8021",
	}
	_, err = session.ApplyEvent(ctx, evt5)
	if err != nil {
		t.Fatal(err)
	}

	eslInst, err := eslRunner.Store.Get(ctx, esl.WorkflowESLDialpadDirect, "direct-call-1")
	if err != nil {
		t.Fatal(err)
	}
	if eslInst.State != "complete" {
		t.Fatalf("expected complete state, got %s", eslInst.State)
	}

	// 6. Verify CTI finalization flow
	payload := map[string]any{"profile": string(contracts.CallFlowAPIDirect)}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-c-1", "cdr_persisted", "idem-c-1", "call", "direct-call-1", contracts.ServiceWorker, payload)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-c-2", "billing_completed", "idem-c-2", "call", "direct-call-1", contracts.ServiceWorker, payload)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-c-3", "recording_completed", "idem-c-3", "call", "direct-call-1", contracts.ServiceWorker, payload)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-c-4", "push_completed", "idem-c-4", "call", "direct-call-1", contracts.ServiceWorker, payload)); err != nil {
		t.Fatal(err)
	}

	ctiInst, err := ctiRunner.Store.Get(ctx, cti.WorkflowDialpadDirect, "direct-call-1")
	if err != nil {
		t.Fatal(err)
	}
	if ctiInst.State != "ended" {
		t.Fatalf("expected CTI state ended, got %s", ctiInst.State)
	}
}

func TestCustomerInboundCallSymmetricFlow(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), nil)

	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	session.Events = bus
	sniffer := testSessionSniffer{
		didMatch:      true,
		didMerchantID: 88,
	}
	session.Sniffer = sniffer

	originate := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Events:         bus,
	}

	RegisterConsumers(context.Background(), bus, ctiRunner, eslRunner, session, originate, nil, nil, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()

	// 1. Physical customer calls DID (CHANNEL_CREATE)
	evt1 := contracts.TelephonyEvent{
		EventID:   "evt-1",
		EventName: "CHANNEL_CREATE",
		CallID:    "inbound-call-1",
		UUID:      "customer-uuid-1",
		LegRole:   contracts.LegRoleCustomer,
		FSAddr:    "127.0.0.1:8021",
		Headers: map[string]any{
			"callerNumber": "13800138000",
			"calleeNumber": "88888888",
		},
	}
	sess, err := session.ApplyEvent(ctx, evt1)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Profile != contracts.CallFlowInbound {
		t.Fatalf("expected profile inbound, got %s", sess.Profile)
	}

	// 2. Physical customer answers (CHANNEL_ANSWER) -> triggers agent originate
	evt2 := contracts.TelephonyEvent{
		EventID:   "evt-2",
		EventName: "CHANNEL_ANSWER",
		CallID:    "inbound-call-1",
		UUID:      "customer-uuid-1",
		LegRole:   contracts.LegRoleCustomer,
		FSAddr:    "127.0.0.1:8021",
	}
	_, err = session.ApplyEvent(ctx, evt2)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that agent originate was triggered
	if executor.Count() != 1 {
		t.Fatalf("expected agent originate command, got %d", executor.Count())
	}
	origCmd := executor.Commands[0]
	if origCmd.Command != "originate" || origCmd.LegRole != contracts.LegRoleAgent {
		t.Fatalf("unexpected command details: %+v", origCmd)
	}

	sess, _ = session.Store.Get(ctx, "inbound-call-1")
	agentUUID := sess.Metadata["agentUuid"].(string)

	// 3. Physical agent leg created (CHANNEL_CREATE)
	evt3 := contracts.TelephonyEvent{
		EventID:   "evt-3",
		EventName: "CHANNEL_CREATE",
		CallID:    "inbound-call-1",
		UUID:      agentUUID,
		LegRole:   contracts.LegRoleAgent,
		FSAddr:    "127.0.0.1:8021",
	}
	_, err = session.ApplyEvent(ctx, evt3)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Physical agent answers (CHANNEL_ANSWER) -> triggers bridge
	evt4 := contracts.TelephonyEvent{
		EventID:   "evt-4",
		EventName: "CHANNEL_ANSWER",
		CallID:    "inbound-call-1",
		UUID:      agentUUID,
		LegRole:   contracts.LegRoleAgent,
		FSAddr:    "127.0.0.1:8021",
	}
	_, err = session.ApplyEvent(ctx, evt4)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that bridge command was executed
	if executor.Count() != 2 {
		t.Fatalf("expected bridge command, got %d", executor.Count())
	}
	bridgeCmd := executor.Commands[1]
	if bridgeCmd.Command != "bridge" {
		t.Fatalf("expected bridge command, got %+v", bridgeCmd)
	}

	// 5. Physical channel hangup (CHANNEL_HANGUP_COMPLETE) -> completes ESL flow
	evt5 := contracts.TelephonyEvent{
		EventID:   "evt-5",
		EventName: "CHANNEL_HANGUP_COMPLETE",
		CallID:    "inbound-call-1",
		UUID:      agentUUID,
		LegRole:   contracts.LegRoleAgent,
		FSAddr:    "127.0.0.1:8021",
	}
	_, err = session.ApplyEvent(ctx, evt5)
	if err != nil {
		t.Fatal(err)
	}

	eslInst, err := eslRunner.Store.Get(ctx, esl.WorkflowESLInbound, "inbound-call-1")
	if err != nil {
		t.Fatal(err)
	}
	if eslInst.State != "complete" {
		t.Fatalf("expected complete state, got %s", eslInst.State)
	}

	// 6. Verify CTI finalization flow
	payload := map[string]any{"profile": string(contracts.CallFlowInbound)}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-c-1", "cdr_persisted", "idem-c-1", "call", "inbound-call-1", contracts.ServiceWorker, payload)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-c-2", "billing_completed", "idem-c-2", "call", "inbound-call-1", contracts.ServiceWorker, payload)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-c-3", "recording_completed", "idem-c-3", "call", "inbound-call-1", contracts.ServiceWorker, payload)); err != nil {
		t.Fatal(err)
	}
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-c-4", "push_completed", "idem-c-4", "call", "inbound-call-1", contracts.ServiceWorker, payload)); err != nil {
		t.Fatal(err)
	}

	ctiInst, err := ctiRunner.Store.Get(ctx, cti.WorkflowInbound, "inbound-call-1")
	if err != nil {
		t.Fatal(err)
	}
	if ctiInst.State != "ended" {
		t.Fatalf("expected CTI state ended, got %s", ctiInst.State)
	}
}

func TestAPIOutboundHangupAgentOnNumberSelectionFailure(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), nil)
	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	session.Events = bus

	originate := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Events:         bus,
		Extensions: apiOutboundTestExtensionResolver{extension: esl.Extension{
			ID:              11,
			UserID:          7,
			MerchantID:      88,
			ExtensionNumber: "2002",
		}},
	}
	// 没有提供任何可用外呼号码 candidates
	candidateSource := apiOutboundTestCandidateSource{candidates: []cti.NumberCandidate{}}
	runtimeSelector := &cti.RuntimeSelector{Allocator: apiOutboundRuntimeAllocator{}}
	RegisterConsumers(context.Background(), bus, ctiRunner, eslRunner, session, originate, runtimeSelector, candidateSource, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()
	// 启动 API 外呼 (A-leg 坐席发起起呼)
	if err := originate.StartAPIOutbound(ctx, esl.OriginateRequest{
		Version: "v1",
		CallID:  "api-hangup-fail-1",
		Request: contracts.ApiCallReq{UserID: 7, Callee: "13800138000"},
	}); err != nil {
		t.Fatal(err)
	}

	// 1. Agent leg created
	if err := bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-0", contracts.EventFSApplied, "idem-api-0", "call", "api-hangup-fail-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-hangup-fail-1",
		"eventName": string(esl.EventChannelCreate),
		"legRole":   string(contracts.LegRoleAgent),
	})); err != nil {
		t.Fatal(err)
	}

	// 2. Agent leg progress -> 触发选号 -> 由于没有 candidates 导致选号失败
	err = bus.Publish(ctx, contracts.NewEventEnvelope("evt-api-1", contracts.EventFSApplied, "idem-api-1", "call", "api-hangup-fail-1", contracts.ServiceCall, map[string]any{
		"callId":    "api-hangup-fail-1",
		"eventName": string(esl.EventChannelProgress),
		"legRole":   string(contracts.LegRoleAgent),
	}))
	// 应当返回没有可用号码错误
	if err == nil {
		t.Fatal("expected ErrNoAvailableNumber error")
	}

	// 3. 验证是否向 agent 自动下发了挂机 hangup 指令
	foundHangup := false
	for _, cmd := range executor.Commands {
		if cmd.Command == "hangup" && cmd.LegRole == contracts.LegRoleAgent {
			foundHangup = true
			break
		}
	}
	if !foundHangup {
		t.Fatal("expected hangup command sent to agent leg on number selection failure")
	}
}

func TestDialpadDirectHangupAgentOnNumberSelectionFailure(t *testing.T) {
	t.Parallel()

	bus := events.NewMemoryBus(nil)
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), nil)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), nil)
	executor := &esl.MemoryCommandExecutor{}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	session.Events = bus
	sniffer := testSessionSniffer{
		extension: esl.Extension{
			ID:              11,
			UserID:          7,
			MerchantID:      88,
			ExtensionNumber: "2002",
		},
	}
	session.Sniffer = sniffer

	originate := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		Events:         bus,
		Extensions:     apiOutboundTestExtensionResolver{extension: sniffer.extension},
	}
	// 没有提供任何 candidates
	candidateSource := apiOutboundTestCandidateSource{candidates: []cti.NumberCandidate{}}
	runtimeSelector := &cti.RuntimeSelector{Allocator: apiOutboundRuntimeAllocator{}}

	RegisterConsumers(context.Background(), bus, ctiRunner, eslRunner, session, originate, runtimeSelector, candidateSource, nil, nil, nil, nil, nil, nil)

	ctx := context.Background()

	// 1. Physical agent call created (CHANNEL_CREATE)
	evt1 := contracts.TelephonyEvent{
		EventID:   "evt-1",
		EventName: "CHANNEL_CREATE",
		CallID:    "direct-hangup-fail-1",
		UUID:      "agent-uuid-1",
		LegRole:   contracts.LegRoleAgent,
		FSAddr:    "127.0.0.1:8021",
		Headers: map[string]any{
			"callerNumber": "2002",
			"calleeNumber": "13800138000",
		},
	}
	_, err = session.ApplyEvent(ctx, evt1)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Physical agent answers (CHANNEL_ANSWER) -> 触发选号 -> 由于没有 candidates 导致选号失败
	evt2 := contracts.TelephonyEvent{
		EventID:   "evt-2",
		EventName: "CHANNEL_ANSWER",
		CallID:    "direct-hangup-fail-1",
		UUID:      "agent-uuid-1",
		LegRole:   contracts.LegRoleAgent,
		FSAddr:    "127.0.0.1:8021",
	}
	_, err = session.ApplyEvent(ctx, evt2)
	if err == nil {
		t.Fatal("expected ErrNoAvailableNumber error")
	}

	// 3. 验证是否向 agent 自动下发了挂机 hangup 指令
	foundHangup := false
	for _, cmd := range executor.Commands {
		if cmd.Command == "hangup" && cmd.LegRole == contracts.LegRoleAgent {
			foundHangup = true
			break
		}
	}
	if !foundHangup {
		t.Fatal("expected hangup command sent to agent leg on number selection failure in direct dialpad")
	}
}
