package cti

import "testing"

func TestTaskStateMachineHappyPath(t *testing.T) {
	t.Parallel()

	machine := NewTaskStateMachine(TaskCreated)
	steps := []struct {
		event TaskEvent
		want  TaskState
	}{
		{EventConfirm, TaskConfirmed},
		{EventStart, TaskRunning},
		{EventPause, TaskPaused},
		{EventResume, TaskRunning},
		{EventSettle, TaskSettling},
		{EventFinish, TaskFinished},
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
