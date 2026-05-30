package eslgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yunshu/internal/domain/operate"
)

func TestSynchronizerCallsCreate(t *testing.T) {
	t.Parallel()

	var gotMethod string
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	syncer := &Synchronizer{BaseURL: server.URL, Client: server.Client(), Timeout: time.Second}
	err := syncer.SyncGatewayConfig(context.Background(), "create", operate.Gateway{ID: 7, Name: "gw-test"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/esl/gateway" || gotQuery != "gatewayId=7" {
		t.Fatalf("unexpected request: method=%s path=%s query=%s", gotMethod, gotPath, gotQuery)
	}
}

func TestSynchronizerCallsDeleteWithEscapedName(t *testing.T) {
	t.Parallel()

	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	syncer := &Synchronizer{BaseURL: server.URL, Client: server.Client(), Timeout: time.Second}
	err := syncer.SyncGatewayConfig(context.Background(), "delete", operate.Gateway{ID: 7, Name: "gw 上海"})
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "gatewayName=gw+%E4%B8%8A%E6%B5%B7" {
		t.Fatalf("unexpected query: %s", gotQuery)
	}
}

func TestSynchronizerRejectsHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	syncer := &Synchronizer{BaseURL: server.URL, Client: server.Client(), Timeout: time.Second}
	if err := syncer.SyncGatewayConfig(context.Background(), "update", operate.Gateway{ID: 7, Name: "gw-test"}); err == nil {
		t.Fatal("expected http error")
	}
}
