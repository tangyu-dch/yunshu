package workflow

import (
	"context"
	"errors"
	"testing"
)

func TestWorkflowEngineAppliesTransitionAndStep(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(Definition{
		ID:      "api_outbound",
		Initial: "new",
		Transitions: []Transition{
			{From: "new", On: "start", To: "selecting", Step: "select_number"},
			{From: "selecting", On: "selected", To: "originating"},
		},
		Handlers: map[StepName]Handler{
			"select_number": func(_ context.Context, instance *Instance, event Event) error {
				instance.Variables["callId"] = event.Payload["callId"]
				return nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	instance, err := engine.Start("api_outbound", "call-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(context.Background(), &instance, Event{Name: "start", Payload: map[string]any{"callId": "call-1"}}); err != nil {
		t.Fatal(err)
	}
	if instance.State != "selecting" {
		t.Fatalf("state got %s", instance.State)
	}
	if instance.Variables["callId"] != "call-1" {
		t.Fatalf("step did not store variable")
	}
}

func TestWorkflowEngineRejectsMissingTransition(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(Definition{ID: "wf", Initial: "new"})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := engine.Start("wf", "1")
	if err != nil {
		t.Fatal(err)
	}
	err = engine.Apply(context.Background(), &instance, Event{Name: "unknown"})
	if !errors.Is(err, ErrTransitionMissing) {
		t.Fatalf("expected transition missing, got %v", err)
	}
}
