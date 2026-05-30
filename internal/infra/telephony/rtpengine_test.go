package telephony

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

func TestRtpengineModelMapsNewTable(t *testing.T) {
	t.Parallel()

	if (RtpengineModel{}).TableName() != "cc_tel_rtpengine" {
		t.Fatalf("unexpected rtpengine table")
	}
}

func TestGormRtpengineRepositoryCRUD(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.AutoMigrate(&RtpengineModel{}); err != nil {
		t.Fatal(err)
	}

	repo := NewRtpengineRepository(db, nil)
	ctx := context.Background()

	// 1. Test Save (Insert)
	engine := operate.Rtpengine{
		SetID:         1,
		RtpengineSock: "udp:127.0.0.1:2223",
		Disabled:      false,
		Weight:        100,
		Description:   "Test proxy",
	}

	saved, err := repo.Save(ctx, engine)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if saved.ID == 0 || saved.RtpengineSock != "udp:127.0.0.1:2223" || saved.Weight != 100 {
		t.Fatalf("unexpected saved engine fields: %+v", saved)
	}

	// 2. Test GetByID
	fetched, err := repo.GetByID(ctx, saved.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.RtpengineSock != "udp:127.0.0.1:2223" {
		t.Fatalf("unexpected fetched engine fields: %+v", fetched)
	}

	// 3. Test ExistsSock
	exists, err := repo.ExistsSock(ctx, "udp:127.0.0.1:2223", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("expected sock to exist")
	}

	exists, err = repo.ExistsSock(ctx, "udp:127.0.0.1:2223", saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatalf("expected sock to be excluded for current ID")
	}

	// 4. Test Page query
	pageResult, err := repo.Page(ctx, operate.RtpenginePageRequest{
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
		t.Fatalf("expected error getting deleted engine")
	}

	// Direct DB check
	var raw RtpengineModel
	if err := db.First(&raw, saved.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !raw.DelFlag || !raw.Disabled {
		t.Fatalf("expected deleted engine to have del_flag = true and disabled = true")
	}
}
