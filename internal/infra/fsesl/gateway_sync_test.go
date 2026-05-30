package fsesl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yunshu/internal/domain/esl"
)

func TestGatewayHTTPExecutorAppliesCreate(t *testing.T) {
	t.Parallel()

	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := &GatewayHTTPExecutor{Client: server.Client(), Timeout: time.Second}
	err := executor.ApplyGatewayConfig(context.Background(), esl.GatewaySyncRequest{
		Action:    esl.GatewaySyncCreate,
		GatewayID: 7,
	}, esl.GatewaySyncNode{FSAddr: "10.0.0.1:8021", CommandURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/esl/createGateway" || gotQuery != "gatewayId=7" {
		t.Fatalf("unexpected request: path=%s query=%s", gotPath, gotQuery)
	}
}

func TestGatewayHTTPExecutorEscapesDeleteName(t *testing.T) {
	t.Parallel()

	rawURL, err := gatewaySyncURL(esl.GatewaySyncRequest{Action: esl.GatewaySyncDelete, GatewayName: "gw 上海"}, "127.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
	if rawURL != "http://127.0.0.1:8080/esl/deleteGateway?gatewayName=gw+%E4%B8%8A%E6%B5%B7" {
		t.Fatalf("unexpected url: %s", rawURL)
	}
}

func TestGatewayHTTPExecutorSkipsMissingCommandURL(t *testing.T) {
	t.Parallel()

	executor := &GatewayHTTPExecutor{Timeout: time.Second}
	err := executor.ApplyGatewayConfig(context.Background(), esl.GatewaySyncRequest{
		Action:    esl.GatewaySyncUpdate,
		GatewayID: 7,
	}, esl.GatewaySyncNode{FSAddr: "10.0.0.1:8021"})
	if err != nil {
		t.Fatalf("expected skip without error, got %v", err)
	}
}
