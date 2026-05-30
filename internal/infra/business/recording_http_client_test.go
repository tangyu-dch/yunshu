package business

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClientUploadPostsRecordingRecordingJob(t *testing.T) {
	t.Parallel()

	var gotOutbox string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOutbox = r.Header.Get("X-Outbox-Id")
		if r.Header.Get("X-Signature-SHA256") == "" {
			t.Fatalf("missing signature")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["recordFilePath"] != "/record/call-1.wav" {
			t.Fatalf("unexpected body: %+v", body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewRecordingHTTPClient(server.URL, "secret", time.Second, nil)
	err := client.Upload(context.Background(), Entry{ID: "outbox-1", IdempotencyKey: "idem-1"}, RecordingJob{ID: "job-1", CallID: "call-1", RecordFile: "/record/call-1.wav"})
	if err != nil {
		t.Fatal(err)
	}
	if gotOutbox != "outbox-1" {
		t.Fatalf("unexpected outbox header: %s", gotOutbox)
	}
}

func TestHTTPClientUploadRetriesOnNonSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewRecordingHTTPClient(server.URL, "", time.Second, nil)
	if err := client.Upload(context.Background(), Entry{ID: "outbox-1"}, RecordingJob{ID: "job-1", CallID: "call-1", RecordFile: "/record/call-1.wav"}); err == nil {
		t.Fatalf("expected retryable error")
	}
}
