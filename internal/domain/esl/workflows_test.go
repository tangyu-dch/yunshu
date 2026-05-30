package esl

import (
	"context"
	"testing"

	"yunshu/internal/contracts"
	"yunshu/pkg/workflow"
)

func TestWorkflowDefinitionsCaptureRingbackMedia(t *testing.T) {
	t.Parallel()

	engine, err := workflow.NewEngine(WorkflowDefinitions()...)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.Start(WorkflowESLAPIOutbound, "call-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(context.Background(), &instance, workflow.Event{Name: "validate_command"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(context.Background(), &instance, workflow.Event{Name: "execute_originate"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(context.Background(), &instance, workflow.Event{Name: "CHANNEL_CREATE"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(context.Background(), &instance, workflow.Event{Name: "CHANNEL_PROGRESS"}); err != nil {
		t.Fatal(err)
	}
	if instance.State != "progress" {
		t.Fatalf("expected progress state, got %s", instance.State)
	}
	if err := engine.Apply(context.Background(), &instance, workflow.Event{
		Name: "CHANNEL_PROGRESS_MEDIA",
		Payload: map[string]any{
			"playbackFile":       "/tmp/ring.wav",
			"supplementRing":     true,
			"supplementRingFile": "/tmp/ring.wav",
			"broadcastTime":      int64(30),
			"workflowProfile":    string(contracts.CallFlowAPIOutbound),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if instance.State != "ringback" {
		t.Fatalf("expected ringback state, got %s", instance.State)
	}
	if got := instance.Variables["playbackFile"]; got != "/tmp/ring.wav" {
		t.Fatalf("expected playback file captured, got %v", got)
	}
	if got := instance.Variables["supplementRing"]; got != true {
		t.Fatalf("expected supplement ring flag, got %v", got)
	}
}
