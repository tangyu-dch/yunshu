package callback

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	outbox "yunshu/internal/infra/business"
)

func TestHTTPClientDeliverPostsCallback(t *testing.T) {
	t.Parallel()

	var gotID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.Header.Get("X-Outbox-Id")
		if r.Header.Get("X-Idempotency-Key") != "idem-1" || r.Header.Get("X-Signature-SHA256") == "" {
			t.Fatalf("missing callback headers")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["aggregateType"] != "batch_call_tel" {
			t.Fatalf("unexpected body: %+v", body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "secret", time.Second, nil)
	err := client.Deliver(context.Background(), outbox.Entry{
		ID:             "outbox-1",
		IdempotencyKey: "idem-1",
		AggregateType:  "batch_call_tel",
		AggregateID:    "10:20",
		Payload:        map[string]any{"eventType": "batch_tel_completed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotID != "outbox-1" {
		t.Fatalf("unexpected outbox id: %s", gotID)
	}
}

func TestHTTPClientDeliverRetriesOnNonSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, "", time.Second, nil)
	if err := client.Deliver(context.Background(), outbox.Entry{ID: "outbox-1"}); err == nil {
		t.Fatalf("expected retryable error")
	}
}
