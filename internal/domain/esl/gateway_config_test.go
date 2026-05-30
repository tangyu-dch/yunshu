package esl

import (
	"context"
	"errors"
	"testing"
)

func TestGatewayConfigServiceSyncCreate(t *testing.T) {
	t.Parallel()

	executor := &fakeGatewayConfigExecutor{}
	service := &GatewayConfigService{
		Gateways: fakeGatewayResolver{names: map[int]string{7: "gw-test"}},
		Nodes:    fakeGatewayNodeLister{nodes: []GatewaySyncNode{{ID: 1, FSAddr: "10.0.0.1:8021"}}},
		Executor: executor,
	}
	result, err := service.Sync(context.Background(), GatewaySyncRequest{Action: GatewaySyncCreate, GatewayID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if result.GatewayName != "gw-test" || result.TargetCount != 1 || !result.Applied {
		t.Fatalf("unexpected result: %+v", result)
	}
	if executor.count != 1 {
		t.Fatalf("expected one sync apply, got %d", executor.count)
	}
}

func TestGatewayConfigServiceRejectsMissingGateway(t *testing.T) {
	t.Parallel()

	service := &GatewayConfigService{
		Gateways: fakeGatewayResolver{names: map[int]string{}},
		Nodes:    fakeGatewayNodeLister{nodes: []GatewaySyncNode{{ID: 1, FSAddr: "10.0.0.1:8021"}}},
	}
	_, err := service.Sync(context.Background(), GatewaySyncRequest{Action: GatewaySyncUpdate, GatewayID: 8})
	if !errors.Is(err, ErrGatewayConfigNotFound) {
		t.Fatalf("expected missing gateway, got %v", err)
	}
}

func TestGatewayConfigServiceRejectsMissingTargets(t *testing.T) {
	t.Parallel()

	service := &GatewayConfigService{
		Gateways: fakeGatewayResolver{names: map[int]string{7: "gw-test"}},
		Nodes:    fakeGatewayNodeLister{},
	}
	_, err := service.Sync(context.Background(), GatewaySyncRequest{Action: GatewaySyncCreate, GatewayID: 7})
	if !errors.Is(err, ErrGatewaySyncTargetMissing) {
		t.Fatalf("expected missing targets, got %v", err)
	}
}

type fakeGatewayResolver struct {
	names map[int]string
}

func (r fakeGatewayResolver) GetGatewayNameByID(_ context.Context, id int) (string, error) {
	name, ok := r.names[id]
	if !ok {
		return "", ErrGatewayConfigNotFound
	}
	return name, nil
}

type fakeGatewayNodeLister struct {
	nodes []GatewaySyncNode
}

func (l fakeGatewayNodeLister) ListGatewaySyncNodes(context.Context) ([]GatewaySyncNode, error) {
	return l.nodes, nil
}

type fakeGatewayConfigExecutor struct {
	count int
}

func (e *fakeGatewayConfigExecutor) ApplyGatewayConfig(context.Context, GatewaySyncRequest, GatewaySyncNode) error {
	e.count++
	return nil
}
