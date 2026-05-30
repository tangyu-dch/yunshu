package esl

import (
	"testing"

	"yunshu/internal/contracts"
)

func TestBuildAPIOutboundPlanUsesAgentFirst(t *testing.T) {
	t.Parallel()

	plan := BuildAPIOutboundPlan("call-1", "v1", "10.0.0.1:8021", contracts.ApiCallReq{
		UserID: 7,
		Callee: "13800138000",
		Extra:  `{"extension":"1001"}`,
	}, "", nil)

	if plan.OriginateMode != contracts.OriginateModeAgentFirst {
		t.Fatalf("unexpected mode: %s", plan.OriginateMode)
	}
	if plan.Destination != "1001" || plan.AgentID != "1001" {
		t.Fatalf("unexpected agent destination: %+v", plan)
	}
	if plan.Options["origination_caller_id_number"] != "138****8000" {
		t.Fatalf("expected masked caller id, got %v", plan.Options["origination_caller_id_number"])
	}
	if plan.AgentUUID == "" || plan.CustomerUUID == "" || plan.AgentUUID == plan.CustomerUUID {
		t.Fatalf("unexpected uuids: agent=%s customer=%s", plan.AgentUUID, plan.CustomerUUID)
	}
}

func TestBuildAPIOutboundPlanPrefersResolvedExtension(t *testing.T) {
	t.Parallel()

	plan := BuildAPIOutboundPlan("call-1", "v1", "10.0.0.1:8021", contracts.ApiCallReq{
		UserID: 7,
		Callee: "13800138000",
		Extra:  `{"extension":"1001"}`,
	}, "2002", nil)

	if plan.Destination != "2002" {
		t.Fatalf("expected resolved extension, got %s", plan.Destination)
	}
}

func TestBuildBatchOutboundPlanUsesCustomerFirst(t *testing.T) {
	t.Parallel()

	plan := BuildBatchOutboundPlan("call-1", "v1", "10.0.0.1:8021", contracts.BatchCallReq{
		UserID:         7,
		BatchTaskID:    10,
		BatchCallTelID: 20,
		Phone:          "13800138000",
		Extension:      "1001",
		Extra:          `{"gatewayName":"gw-sh"}`,
	}, nil)

	if plan.OriginateMode != contracts.OriginateModeCustomerFirst {
		t.Fatalf("unexpected mode: %s", plan.OriginateMode)
	}
	if plan.Destination != "13800138000" || plan.DomainOrGateway != "gw-sh" {
		t.Fatalf("unexpected batch plan: %+v", plan)
	}
	if plan.CustomerUUID == "" || plan.AgentUUID == "" || plan.CustomerUUID == plan.AgentUUID {
		t.Fatalf("unexpected uuids: %+v", plan)
	}
}
