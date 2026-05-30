package callflow

import (
	"testing"
	"time"

	"yunshu/internal/domain/outbox"
)

func TestBuildCDRFanoutEntries(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	entries := BuildCDRFanoutEntries(outbox.Entry{
		ID:          "cdr:call-1",
		AggregateID: "call-1",
		Payload:     map[string]any{"callId": "call-1", "uuid": "uuid-1"},
	}, now)
	if len(entries) != 4 {
		t.Fatalf("expected four fanout entries, got %d", len(entries))
	}
	got := map[string]bool{}
	for _, entry := range entries {
		got[entry.Destination] = true
		if entry.AggregateID != "call-1" || entry.Payload["sourceOutboxId"] != "cdr:call-1" {
			t.Fatalf("unexpected fanout entry: %+v", entry)
		}
	}
	if !got[DestinationCDRBilling] || !got[DestinationCDRRecording] || !got[DestinationCDRReportProjection] || !got[DestinationCDRDownstreamPush] {
		t.Fatalf("unexpected fanout destinations: %+v", entries)
	}
}
