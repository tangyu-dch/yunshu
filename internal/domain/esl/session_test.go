package esl_test

import (
	"context"
	"errors"
	"testing"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	"yunshu/internal/infra/business"
	"yunshu/pkg/telephony"
)

func TestSessionServicePublishesCDROnHangupComplete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	outboxStore := business.NewOutboxMemoryStore()
	service := esl.NewSessionService(esl.NewMemorySessionStore(), outboxStore, nil)
	cmd := telephony.NewCommand("cmd-1", "originate", "call-1", "pending:call-1", "default", contracts.LegRoleCustomer, contracts.CallFlowAPIOutbound, map[string]any{"merchantId": 88, "userId": 99})
	if err := service.CreateFromOriginate(ctx, cmd); err != nil {
		t.Fatal(err)
	}

	if _, err := service.ApplyEvent(ctx, contracts.TelephonyEvent{
		EventID:   "evt-1",
		EventName: string(esl.EventChannelCreate),
		CallID:    "call-1",
		UUID:      "uuid-1",
		FSAddr:    "10.0.0.1:8021",
		LegRole:   contracts.LegRoleCustomer,
		Profile:   contracts.CallFlowAPIOutbound,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyEvent(ctx, contracts.TelephonyEvent{
		EventID:   "evt-2",
		EventName: string(esl.EventChannelHangupComplete),
		CallID:    "call-1",
		UUID:      "uuid-1",
		FSAddr:    "10.0.0.1:8021",
		LegRole:   contracts.LegRoleCustomer,
		Profile:   contracts.CallFlowAPIOutbound,
		Headers:   map[string]any{"hangupCause": "NORMAL_CLEARING", "recordFilePath": "/record/call-1.wav"},
	}); err != nil {
		t.Fatal(err)
	}

	pending, err := outboxStore.Pending(ctx, 10, service.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected one cdr task, got %d", len(pending))
	}
	if pending[0].Destination != "call_center_cdr_queue" {
		t.Fatalf("unexpected destination: %s", pending[0].Destination)
	}
	if pending[0].Payload["merchantId"] != 88 || pending[0].Payload["recordFilePath"] != "/record/call-1.wav" {
		t.Fatalf("expected cdr business context, got %+v", pending[0].Payload)
	}
}

func TestSessionServiceRejectsDuplicateEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := esl.NewSessionService(esl.NewMemorySessionStore(), business.NewOutboxMemoryStore(), nil)
	cmd := telephony.NewCommand("cmd-1", "originate", "call-1", "pending:call-1", "default", contracts.LegRoleCustomer, contracts.CallFlowAPIOutbound, nil)
	if err := service.CreateFromOriginate(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	event := contracts.TelephonyEvent{
		EventID:   "evt-1",
		EventName: string(esl.EventChannelCreate),
		CallID:    "call-1",
		UUID:      "uuid-1",
		FSAddr:    "10.0.0.1:8021",
		LegRole:   contracts.LegRoleCustomer,
		Profile:   contracts.CallFlowAPIOutbound,
	}
	if _, err := service.ApplyEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyEvent(ctx, event); !errors.Is(err, esl.ErrDuplicateEvent) {
		t.Fatalf("expected duplicate event, got %v", err)
	}
}
