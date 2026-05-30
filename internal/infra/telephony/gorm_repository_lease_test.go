package telephony

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGormRegistryClaimsAndReleasesLease(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&FreeswitchModel{}, &FreeswitchEventLeaseModel{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&FreeswitchModel{ID: 1, Address: "10.0.0.1", ESLPort: 8021, Enable: true, DelFlag: false, UpdatedTime: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}

	registry := NewGormRegistry(db)
	ctx := context.Background()
	node, err := registry.ClaimEvents(ctx, "10.0.0.1:8021", "esl-a", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if node.EventOwner != "esl-a" {
		t.Fatalf("unexpected owner: %+v", node)
	}
	if _, err := registry.ClaimEvents(ctx, "10.0.0.1:8021", "esl-b", time.Minute); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("expected lease held, got %v", err)
	}
	if err := registry.ReleaseEvents(ctx, "10.0.0.1:8021", "esl-a"); err != nil {
		t.Fatal(err)
	}
	node, err = registry.ClaimEvents(ctx, "10.0.0.1:8021", "esl-b", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if node.EventOwner != "esl-b" {
		t.Fatalf("unexpected owner after release: %+v", node)
	}
}

func TestFreeswitchEventLeaseModelTableName(t *testing.T) {
	t.Parallel()

	if (FreeswitchEventLeaseModel{}).TableName() != "cc_tel_fs_lease" {
		t.Fatalf("unexpected lease table name")
	}
}
