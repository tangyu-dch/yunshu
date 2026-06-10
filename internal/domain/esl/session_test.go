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

func TestSessionServiceWritesCDRForAllYunshuProfiles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	profiles := []contracts.CallFlowProfile{
		contracts.CallFlowAPIOutbound,
		contracts.CallFlowAPIDirect,
		contracts.CallFlowInbound,
		contracts.CallFlowBatchOutbound,
		contracts.CallFlowBatchPredictive,
		contracts.CallFlowBatchSynergy,
	}
	for _, profile := range profiles {
		profile := profile
		t.Run(string(profile), func(t *testing.T) {
			t.Parallel()
			outboxStore := business.NewOutboxMemoryStore()
			service := esl.NewSessionService(esl.NewMemorySessionStore(), outboxStore, nil)
			callID := "cdr-required-" + string(profile)
			cmd := telephony.NewCommand("cmd-"+callID, "originate", callID, "uuid-"+callID, "default", contracts.LegRoleCustomer, profile, map[string]any{"merchantId": 88, "userId": 99})
			if err := service.CreateFromOriginate(ctx, cmd); err != nil {
				t.Fatal(err)
			}
			if _, err := service.ApplyEvent(ctx, contracts.TelephonyEvent{
				EventID:   "create-" + callID,
				EventName: string(esl.EventChannelCreate),
				CallID:    callID,
				UUID:      "uuid-" + callID,
				FSAddr:    "10.0.0.1:8021",
				LegRole:   contracts.LegRoleCustomer,
				Profile:   profile,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := service.ApplyEvent(ctx, contracts.TelephonyEvent{
				EventID:   "hangup-" + callID,
				EventName: string(esl.EventChannelHangupComplete),
				CallID:    callID,
				UUID:      "uuid-" + callID,
				FSAddr:    "10.0.0.1:8021",
				LegRole:   contracts.LegRoleCustomer,
				Profile:   profile,
				Headers:   map[string]any{"hangupCause": "NORMAL_CLEARING"},
			}); err != nil {
				t.Fatal(err)
			}
			pending, err := outboxStore.Pending(ctx, 10, service.Now())
			if err != nil {
				t.Fatal(err)
			}
			if len(pending) != 1 {
				t.Fatalf("expected one CDR outbox for profile %s, got %d", profile, len(pending))
			}
			if pending[0].Destination != "call_center_cdr_queue" || pending[0].AggregateID != callID {
				t.Fatalf("unexpected CDR outbox entry: %+v", pending[0])
			}
			if pending[0].Payload["profile"] != profile {
				t.Fatalf("expected profile %s in payload, got %+v", profile, pending[0].Payload)
			}
		})
	}
}

func TestSessionServiceWritesCDRForSniffedPhysicalCalls(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cases := []struct {
		name      string
		callID    string
		caller    string
		callee    string
		wantProf  contracts.CallFlowProfile
		wantMerID int
	}{
		{name: "dialpad direct", callID: "sniff-direct-1", caller: "2002", callee: "13800138000", wantProf: contracts.CallFlowAPIDirect, wantMerID: 88},
		{name: "customer inbound", callID: "sniff-inbound-1", caller: "13800138000", callee: "01088886666", wantProf: contracts.CallFlowInbound, wantMerID: 1001},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outboxStore := business.NewOutboxMemoryStore()
			service := esl.NewSessionService(esl.NewMemorySessionStore(), outboxStore, nil)
			service.Sniffer = cdrSniffer{}
			if _, err := service.ApplyEvent(ctx, contracts.TelephonyEvent{
				EventID:   "create-" + tc.callID,
				EventName: string(esl.EventChannelCreate),
				CallID:    tc.callID,
				UUID:      "uuid-" + tc.callID,
				FSAddr:    "10.0.0.1:8021",
				LegRole:   contracts.LegRoleCustomer,
				Headers: map[string]any{
					"callerNumber": tc.caller,
					"calleeNumber": tc.callee,
				},
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := service.ApplyEvent(ctx, contracts.TelephonyEvent{
				EventID:   "hangup-" + tc.callID,
				EventName: string(esl.EventChannelHangupComplete),
				CallID:    tc.callID,
				UUID:      "uuid-" + tc.callID,
				FSAddr:    "10.0.0.1:8021",
				LegRole:   contracts.LegRoleCustomer,
				Headers:   map[string]any{"hangupCause": "NORMAL_CLEARING"},
			}); err != nil {
				t.Fatal(err)
			}
			pending, err := outboxStore.Pending(ctx, 10, service.Now())
			if err != nil {
				t.Fatal(err)
			}
			if len(pending) != 1 {
				t.Fatalf("expected one sniffed-call CDR, got %d", len(pending))
			}
			if pending[0].Payload["profile"] != tc.wantProf || pending[0].Payload["merchantId"] != tc.wantMerID {
				t.Fatalf("unexpected CDR payload: %+v", pending[0].Payload)
			}
		})
	}
}

type cdrSniffer struct{}

func (cdrSniffer) IsExtension(_ context.Context, number string) (bool, *esl.Extension, error) {
	if number == "2002" {
		return true, &esl.Extension{UserID: 99, MerchantID: 88, ExtensionNumber: "2002"}, nil
	}
	return false, nil, nil
}

func (cdrSniffer) IsMerchantDID(_ context.Context, number string) (bool, int, error) {
	if number == "01088886666" {
		return true, 1001, nil
	}
	return false, 0, nil
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
