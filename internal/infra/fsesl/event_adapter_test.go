package fsesl

import (
	"net/textproto"
	"testing"

	"github.com/percipia/eslgo"

	"yunshu/internal/contracts"
)

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
