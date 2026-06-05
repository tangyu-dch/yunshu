package telephony

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

func TestDispatcherModelMapsNewTable(t *testing.T) {
	t.Parallel()

	if (DispatcherModel{}).TableName() != "cc_res_freeswitch" {
		t.Fatalf("unexpected dispatcher table")
	}
}

func TestGormDispatcherRepositoryCRUD(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.AutoMigrate(&DispatcherModel{}); err != nil {
		t.Fatal(err)
	}

	repo := NewGormDispatcherRepository(db, nil)
	ctx := context.Background()

	// 1. Test Save (Insert)
	disp := operate.Dispatcher{
		SetID:       1,
		Destination: "sip:127.0.0.1:5060",
		Flags:       0,
		Priority:    100,
		Attrs:       "max-concurrency=1000",
		Description: "Test dispatcher node",
		Enable:      true,
	}

	saved, err := repo.Save(ctx, disp)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if saved.ID == 0 || saved.Destination != "sip:127.0.0.1:5060" || saved.Priority != 100 {
		t.Fatalf("unexpected saved dispatcher fields: %+v", saved)
	}

	// 2. Test GetByID
	fetched, err := repo.GetByID(ctx, saved.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.Destination != "sip:127.0.0.1:5060" {
		t.Fatalf("unexpected fetched dispatcher fields: %+v", fetched)
	}

	// 3. Test ExistsDestination
	exists, err := repo.ExistsDestination(ctx, "sip:127.0.0.1:5060", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("expected destination to exist")
	}

	exists, err = repo.ExistsDestination(ctx, "sip:127.0.0.1:5060", saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("expected destination to be excluded for current ID")
	}

	// 4. Test Page query
	pageResult, err := repo.Page(ctx, operate.DispatcherPageRequest{
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatalf("Page failed: %v", err)
	}
	if pageResult.Total != 1 || len(pageResult.Records) != 1 {
		t.Fatalf("expected 1 record, got %d (records: %d)", pageResult.Total, len(pageResult.Records))
	}

	// 5. Test Delete (Logical Delete)
	err = repo.Delete(ctx, []int{saved.ID})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Make sure it's logically deleted (del_flag = true) and doesn't return via GetByID
	_, err = repo.GetByID(ctx, saved.ID)
	if err == nil {
		t.Fatalf("expected error getting deleted dispatcher")
	}

	// Direct DB check
	var raw DispatcherModel
	if err := db.First(&raw, saved.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !raw.DelFlag || raw.Enable || raw.Flags != 1 {
		t.Fatalf("expected deleted dispatcher to have del_flag = true, enable = false and flags = 1")
	}
}
