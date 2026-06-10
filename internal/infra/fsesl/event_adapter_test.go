package fsesl

import (
	"net/textproto"
	"testing"

	"github.com/percipia/eslgo"

	"yunshu/internal/contracts"
)

func TestEventFromESLFallsBackToUniqueIDForPhysicalInbound(t *testing.T) {
	t.Parallel()

	event := &eslgo.Event{Headers: textproto.MIMEHeader{}}
	event.Headers.Set("Event-Name", "CHANNEL_CREATE")
	event.Headers.Set("Event-Sequence", "7")
	event.Headers.Set("Unique-ID", "physical-uuid-1")
	event.Headers.Set("Caller-Destination-Number", "01088886666")

	domainEvent := EventFromESL("10.0.0.1:8021", event)
	if domainEvent.CallID != "physical-uuid-1" {
		t.Fatalf("expected Unique-ID fallback call id, got %+v", domainEvent)
	}
	if domainEvent.UUID != "physical-uuid-1" {
		t.Fatalf("expected event uuid from Unique-ID, got %+v", domainEvent)
	}
}

func TestEventFromESLResolvesBusinessFields(t *testing.T) {
	t.Parallel()

	event := &eslgo.Event{Headers: textproto.MIMEHeader{}}
	event.Headers.Set("Event-Name", "CHANNEL_HANGUP_COMPLETE")
	event.Headers.Set("Event-Sequence", "42")
	event.Headers.Set("Event-Date-Timestamp", "1716357600000000")
	event.Headers.Set("Unique-ID", "uuid-1")
	event.Headers.Set("variable_yunshu_call_id", "call-1")
	event.Headers.Set("variable_sip_h_X-Internal-Call", "true")
	event.Headers.Set("Hangup-Cause", "NORMAL_CLEARING")
	event.Headers.Set("variable_hangup_cause_q850", "16")

	domainEvent := EventFromESL("10.0.0.1:8021", event)
	if domainEvent.EventID != "10.0.0.1:8021:CHANNEL_HANGUP_COMPLETE:42:uuid-1" {
		t.Fatalf("unexpected event id: %s", domainEvent.EventID)
	}
	if domainEvent.CallID != "call-1" || domainEvent.LegRole != contracts.LegRoleAgent {
		t.Fatalf("unexpected event: %+v", domainEvent)
	}
	if domainEvent.Headers["q850"] != 16 {
		t.Fatalf("unexpected q850: %+v", domainEvent.Headers)
	}
}

func TestEventFromESLAddsCallerCalleeAliases(t *testing.T) {
	t.Parallel()

	event := &eslgo.Event{Headers: textproto.MIMEHeader{}}
	event.Headers.Set("Event-Name", "CHANNEL_CREATE")
	event.Headers.Set("Unique-ID", "uuid-alias")
	event.Headers.Set("Caller-Caller-ID-Number", "1001")
	event.Headers.Set("Caller-Destination-Number", "13800003333")

	domainEvent := EventFromESL("10.0.0.1:8021", event)
	if domainEvent.Headers["callerNumber"] != "1001" {
		t.Fatalf("expected callerNumber alias, headers=%+v", domainEvent.Headers)
	}
	if domainEvent.Headers["calleeNumber"] != "13800003333" {
		t.Fatalf("expected calleeNumber alias, headers=%+v", domainEvent.Headers)
	}
}
