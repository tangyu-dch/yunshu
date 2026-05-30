package state

import "testing"

func TestMachineApply(t *testing.T) {
	t.Parallel()

	machine := NewMachine("new", map[string]map[string]string{
		"new": {"start": "running"},
	})

	next, err := machine.Apply("start")
	if err != nil {
		t.Fatal(err)
	}
	if next != "running" || machine.State() != "running" {
		t.Fatalf("unexpected state: %s", next)
	}
}

func TestMachineRejectsInvalidEvent(t *testing.T) {
	t.Parallel()

	machine := NewMachine("new", map[string]map[string]string{
		"new": {"start": "running"},
	})
	if _, err := machine.Apply("pause"); err == nil {
		t.Fatal("expected invalid event error")
	}
}
