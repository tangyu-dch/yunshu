package fsesl

import (
	"strings"
	"testing"

	"yunshu/internal/contracts"
)

func TestBuildOriginateArgsAgentFirst(t *testing.T) {
	t.Parallel()

	cmd := contracts.TelephonyCommand{
		CommandID: "originate:call-1",
		Command:   "originate",
		CallID:    "call-1",
		UUID:      "uuid-agent",
		FSAddr:    "10.0.0.1:8021",
		LegRole:   contracts.LegRoleAgent,
		Profile:   contracts.CallFlowAPIOutbound,
		Payload: map[string]any{
			"originateMode":   contracts.OriginateModeAgentFirst,
			"destination":     "1001",
			"domainOrGateway": "127.0.0.1:5060",
			"options": map[string]any{
				"origination_caller_id_number": "138****8000",
			},
		},
	}

	args := BuildOriginateArgs(cmd)
	if !strings.Contains(args, "sofia/external/1001@127.0.0.1:5060 &park()") {
		t.Fatalf("unexpected originate args: %s", args)
	}
	if !strings.Contains(args, "origination_caller_id_number=138****8000") {
		t.Fatalf("expected masked caller id in args: %s", args)
	}
}

func TestBuildOriginateArgsCustomerFirstGateway(t *testing.T) {
	t.Parallel()

	cmd := contracts.TelephonyCommand{
		CommandID: "originate:call-1",
		Command:   "originate",
		CallID:    "call-1",
		UUID:      "uuid-customer",
		FSAddr:    "10.0.0.1:8021",
		LegRole:   contracts.LegRoleCustomer,
		Profile:   contracts.CallFlowBatchOutbound,
		Payload: map[string]any{
			"originateMode":   contracts.OriginateModeCustomerFirst,
			"destination":     "13800138000",
			"domainOrGateway": "gw-sh",
			"register":        true,
		},
	}

	args := BuildOriginateArgs(cmd)
	if !strings.Contains(args, "sofia/gateway/gw-sh/13800138000 &park()") {
		t.Fatalf("unexpected originate args: %s", args)
	}
}

func TestBuildOriginateArgsAgentFirstUserProtocol(t *testing.T) {
	t.Parallel()

	// 1. 测试用例：显式要求使用 useUserProtocol
	cmd1 := contracts.TelephonyCommand{
		CommandID: "originate:call-1",
		Command:   "originate",
		Payload: map[string]any{
			"originateMode":   contracts.OriginateModeAgentFirst,
			"destination":     "100001",
			"domainOrGateway": "default",
			"useUserProtocol": true,
		},
	}
	args1 := BuildOriginateArgs(cmd1)
	if !strings.Contains(args1, "user/100001 &park()") {
		t.Fatalf("expected user/100001 protocol, got: %s", args1)
	}

	// 2. 测试用例：满足 4~6 位且未指定外置网关 IP，自动转换
	cmd2 := contracts.TelephonyCommand{
		CommandID: "originate:call-2",
		Command:   "originate",
		Payload: map[string]any{
			"originateMode":   contracts.OriginateModeAgentFirst,
			"destination":     "100002",
			"domainOrGateway": "my-sip-domain",
		},
	}
	args2 := BuildOriginateArgs(cmd2)
	if !strings.Contains(args2, "user/100002 &park()") {
		t.Fatalf("expected user/100002 protocol, got: %s", args2)
	}
}
