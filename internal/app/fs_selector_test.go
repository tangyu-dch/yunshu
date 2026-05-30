package app

import (
	"context"
	"testing"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	fsregistry "yunshu/internal/infra/telephony"
)

func TestRegistryNodeSelectorUsesRequestedSetID(t *testing.T) {
	t.Parallel()

	registry := fsregistry.NewMemoryRegistry()
	ctx := context.Background()
	_ = registry.Upsert(ctx, fsregistry.Node{ID: 1, FSAddr: "10.0.0.1:8021", SetID: 1, Weight: 100, RWeight: 100, CC: 1, Enable: true})
	_ = registry.Upsert(ctx, fsregistry.Node{ID: 2, FSAddr: "10.0.0.2:8021", SetID: 2, Weight: 100, RWeight: 100, CC: 1, Enable: true})

	selector := registryNodeSelector{registry: registry}
	fsAddr, err := selector.SelectAPIOutbound(ctx, esl.OriginateRequest{
		CallID:  "call-1",
		Request: contracts.ApiCallReq{Extra: `{"setid":2}`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fsAddr != "10.0.0.2:8021" {
		t.Fatalf("unexpected fs addr: %s", fsAddr)
	}
}

func TestEffectiveWeightUsesRWeightWhenCongestionControlEnabled(t *testing.T) {
	t.Parallel()

	node := fsregistry.Node{Weight: 100, RWeight: 20, CC: 1}
	if got := effectiveWeight(node); got != 20 {
		t.Fatalf("unexpected effective weight: %d", got)
	}
	node.CC = 2
	if got := effectiveWeight(node); got != 100 {
		t.Fatalf("unexpected effective weight without cc: %d", got)
	}
}
