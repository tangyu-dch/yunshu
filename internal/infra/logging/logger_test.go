package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNewJSONLoggerAddsService(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := New(Config{Service: "cc-test", Level: "debug", Format: FormatJSON, Writer: &buf})
	logger.Debug("hello", slog.String("requestId", "req-1"))

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["service"] != "cc-test" {
		t.Fatalf("missing service field: %+v", payload)
	}
	if payload["requestId"] != "req-1" {
		t.Fatalf("missing request id: %+v", payload)
	}
}

func TestHTTPAttrs(t *testing.T) {
	t.Parallel()

	attrs := HTTPAttrs("GET", "/healthz", "req-1", "trace-1", 200, "1ms")
	if len(attrs) != 6 {
		t.Fatalf("unexpected attr count: %d", len(attrs))
	}
}
