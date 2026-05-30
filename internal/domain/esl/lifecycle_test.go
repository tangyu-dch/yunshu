package esl

import (
	"testing"

	"yunshu/internal/contracts"
)

func TestCallLifecycleCompletesOnHangupComplete(t *testing.T) {
	t.Parallel()

	machine := NewCallLifecycle(CallNew)
	steps := []struct {
		event CallEvent
		want  CallState
	}{
		{EventChannelCreate, CallCreated},
		{EventChannelProgressMedia, CallProgressMedia},
		{EventChannelAnswer, CallAnswered},
		{EventChannelBridge, CallBridged},
		{EventChannelHangup, CallHangup},
		{EventChannelHangupComplete, CallComplete},
	}

	for _, step := range steps {
		got, err := machine.Apply(step.event)
		if err != nil {
			t.Fatalf("apply %s: %v", step.event, err)
		}
		if got != step.want {
			t.Fatalf("apply %s got %s want %s", step.event, got, step.want)
		}
	}
}

func TestCommandValidatorRequiresTraceFields(t *testing.T) {
	t.Parallel()

	valid := contracts.TelephonyCommand{
		CommandID: "cmd-1",
		Command:   "hangup",
		CallID:    "call-1",
		UUID:      "uuid-1",
		FSAddr:    "10.0.0.1:8021",
		LegRole:   contracts.LegRoleCustomer,
	}
	if !(CommandValidator{}).Validate(valid) {
		t.Fatal("expected command to be valid")
	}

	valid.UUID = ""
	if (CommandValidator{}).Validate(valid) {
		t.Fatal("expected command without uuid to be invalid")
	}
}
