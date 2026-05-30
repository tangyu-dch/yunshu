package operate

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

func TestFreeSwitchManagementServiceSavesNormalizedNode(t *testing.T) {
	t.Parallel()

	service := &FreeSwitchManagementService{Registry: NewMemoryRegistry(), Logger: slog.Default()}
	node, err := service.Save(context.Background(), Node{ID: 1, FSAddr: "10.0.0.1:8021", Enable: true})
	if err != nil {
		t.Fatal(err)
	}

	if node.Address != "10.0.0.1" || node.ESLPort != 8021 {
		t.Fatalf("expected address and port normalized, got %+v", node)
	}
	if node.SetID != 1 || node.Weight != 50 || node.RWeight != 50 || node.CC != 1 {
		t.Fatalf("expected default routing fields, got %+v", node)
	}
	if node.Status != NodeActive {
		t.Fatalf("expected active status, got %s", node.Status)
	}
}

func TestFreeSwitchManagementServiceRejectsInvalidNode(t *testing.T) {
	t.Parallel()

	service := &FreeSwitchManagementService{Registry: NewMemoryRegistry(), Logger: slog.Default()}
	_, err := service.Save(context.Background(), Node{ID: 1, Address: "10.0.0.1"})
	if !errors.Is(err, ErrInvalidFreeSwitchNode) {
		t.Fatalf("expected invalid node error, got %v", err)
	}
}

func TestFreeSwitchManagementServiceEnableAndDelete(t *testing.T) {
	t.Parallel()

	service := &FreeSwitchManagementService{Registry: NewMemoryRegistry(), Logger: slog.Default()}
	if _, err := service.Save(context.Background(), Node{ID: 2, FSAddr: "10.0.0.2:8021", Enable: true}); err != nil {
		t.Fatal(err)
	}

	disabled, err := service.Enable(context.Background(), 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Enable || disabled.Status != NodeUnavailable {
		t.Fatalf("expected disabled unavailable node, got %+v", disabled)
	}

	if err := service.Delete(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Registry.GetByID(context.Background(), 2); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected deleted node missing, got %v", err)
	}
}
