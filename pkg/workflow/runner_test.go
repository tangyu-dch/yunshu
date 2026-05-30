package workflow

import (
	"context"
	"testing"
)

func TestRunnerStartsMissingInstanceAndAppliesEvent(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(Definition{
		ID:      "wf",
		Initial: "new",
		Transitions: []Transition{
			{From: "new", On: "start", To: "running"},
		},
		Handlers: map[StepName]Handler{},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(engine, NewMemoryInstanceStore(), nil)
	instance, err := runner.Apply(context.Background(), "wf", "1", Event{Name: "start"})
	if err != nil {
		t.Fatal(err)
	}
	if instance.State != "running" {
		t.Fatalf("state got %s", instance.State)
	}
	saved, err := runner.Store.Get(context.Background(), "wf", "1")
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != "running" {
		t.Fatalf("saved state got %s", saved.State)
	}
}
