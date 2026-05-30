package esl

import (
	"context"
	"errors"
	"testing"

	"yunshu/internal/contracts"
	"yunshu/pkg/idempotency"
)

type fakeExecutor struct {
	calls int
	err   error
}

func (f *fakeExecutor) Execute(context.Context, contracts.TelephonyCommand) error {
	f.calls++
	return f.err
}

func TestCommandServiceExecutesOnce(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	service := NewCommandService(idempotency.NewMemoryStore(), exec, nil)
	cmd := validCommand()

	if err := service.Execute(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	if err := service.Execute(context.Background(), cmd); !errors.Is(err, ErrDuplicateCommand) {
		t.Fatalf("expected duplicate command, got %v", err)
	}
	if exec.calls != 1 {
		t.Fatalf("executor calls got %d", exec.calls)
	}
}

func TestCommandServiceReleasesIdempotencyOnExecutorFailure(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{err: errors.New("fs down")}
	service := NewCommandService(idempotency.NewMemoryStore(), exec, nil)
	cmd := validCommand()

	if err := service.Execute(context.Background(), cmd); err == nil {
		t.Fatal("expected executor error")
	}
	exec.err = nil
	if err := service.Execute(context.Background(), cmd); err != nil {
		t.Fatalf("expected retry after executor failure: %v", err)
	}
}

func TestCommandServiceRejectsInvalidCommand(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	service := NewCommandService(idempotency.NewMemoryStore(), exec, nil)
	cmd := validCommand()
	cmd.CallID = ""

	if err := service.Execute(context.Background(), cmd); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("expected invalid command, got %v", err)
	}
	if exec.calls != 0 {
		t.Fatal("invalid command should not execute")
	}
}

func validCommand() contracts.TelephonyCommand {
	return contracts.TelephonyCommand{
		CommandID: "cmd-1",
		Command:   "hangup",
		CallID:    "call-1",
		UUID:      "uuid-1",
		FSAddr:    "10.0.0.1:8021",
		LegRole:   contracts.LegRoleCustomer,
		Profile:   contracts.CallFlowAPIOutbound,
	}
}
