package business

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClientDeliverPostsDownstreamCDR(t *testing.T) {
	t.Parallel()

	var gotTarget string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTarget = r.Header.Get("X-Downstream-Target")
		if r.Header.Get("X-Signature-SHA256") == "" {
			t.Fatalf("missing signature")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["callId"] != "call-1" {
			t.Fatalf("unexpected body: %+v", body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewDownstreamHTTPClient(server.URL, "secret", time.Second, nil)
	err := client.Deliver(context.Background(), Entry{ID: "outbox-1", IdempotencyKey: "idem-1", Payload: map[string]any{"callId": "call-1"}}, PushJob{ID: "job-1", CallID: "call-1", Target: "ods"})
	if err != nil {
		t.Fatal(err)
	}
	if gotTarget != "ods" {
		t.Fatalf("unexpected target header: %s", gotTarget)
	}
}

func TestHTTPClientDeliverRetriesOnNonSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewDownstreamHTTPClient(server.URL, "", time.Second, nil)
	if err := client.Deliver(context.Background(), Entry{ID: "outbox-1"}, PushJob{ID: "job-1", CallID: "call-1"}); err == nil {
		t.Fatalf("expected retryable error")
	}
}
