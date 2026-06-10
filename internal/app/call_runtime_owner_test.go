package app

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestBuildFSLeaseOwnerUsesInstanceID(t *testing.T) {
	t.Parallel()
	owner := buildFSLeaseOwner("cc-call", "instance-a")
	if !strings.Contains(owner, "cc-call-") || !strings.HasSuffix(owner, "-instance-a") {
		t.Fatalf("unexpected owner: %s", owner)
	}
}

func TestBuildFSLeaseOwnerFallsBackToPID(t *testing.T) {
	t.Parallel()
	owner := buildFSLeaseOwner("cc-call", "")
	if !strings.HasSuffix(owner, "-"+strconv.Itoa(getpidForTest())) {
		t.Fatalf("expected owner to include pid fallback, got %s", owner)
	}
}

func getpidForTest() int {
	return os.Getpid()
}
