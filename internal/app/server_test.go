package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunshu/internal/contracts"
)

func TestServerHealth(t *testing.T) {
	t.Parallel()

	server, err := NewServer(contracts.ServiceCall)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.gin.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d", rec.Code)
	}

	var result contracts.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != contracts.CodeOK {
		t.Fatalf("unexpected result: %+v", result)
	}
}
