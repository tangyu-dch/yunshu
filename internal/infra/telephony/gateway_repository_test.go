package telephony

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// DummyBlacklistGatewayModel matches the structure of cc_sec_blacklist_gateway for testing.
type DummyBlacklistGatewayModel struct {
	BlacklistID int `gorm:"column:blacklist_id;primaryKey"`
	GatewayID   int `gorm:"column:gateway_id;primaryKey"`
}

func (DummyBlacklistGatewayModel) TableName() string {
	return "cc_sec_blacklist_gateway"
}

func TestGatewayRepositoryDeleteCascadeBlacklist(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Migrate both tables
	if err := db.AutoMigrate(&GatewayModel{}, &DummyBlacklistGatewayModel{}); err != nil {
		t.Fatal(err)
	}

	repo := NewGatewayRepository(db, nil)
	ctx := context.Background()

	// 1. Create a gateway
	gw, err := repo.Save(ctx, operate.Gateway{
		Name:        "测试网关",
		Description: "测试描述",
		ChannelID:   1,
		Concurrency: 10,
		Realm:       "127.0.0.1",
		Port:        "5060",
		Priority:    1,
		Enable:      true,
	})
	if err != nil {
		t.Fatalf("Failed to save gateway: %v", err)
	}

	// 2. Create a blacklist mapping referencing the gateway ID
	blacklistMapping := DummyBlacklistGatewayModel{
		BlacklistID: 100,
		GatewayID:   gw.ID,
	}
	if err := db.Create(&blacklistMapping).Error; err != nil {
		t.Fatalf("Failed to create blacklist mapping: %v", err)
	}

	// 3. Verify blacklist mapping exists
	var count int64
	db.Model(&DummyBlacklistGatewayModel{}).Where("gateway_id = ?", gw.ID).Count(&count)
	if count != 1 {
		t.Fatalf("Expected 1 blacklist mapping, got %d", count)
	}

	// 4. Delete the gateway
	err = repo.Delete(ctx, []int{gw.ID})
	if err != nil {
		t.Fatalf("Failed to delete gateway: %v", err)
	}

	// 5. Verify gateway is logically deleted (del_flag = true)
	var deletedGw GatewayModel
	if err := db.Where("id = ?", gw.ID).First(&deletedGw).Error; err != nil {
		t.Fatalf("Failed to retrieve gateway: %v", err)
	}
	if !deletedGw.DelFlag {
		t.Fatalf("Expected gateway del_flag to be true, got false")
	}

	// 6. Verify blacklist mapping is physically deleted
	db.Model(&DummyBlacklistGatewayModel{}).Where("gateway_id = ?", gw.ID).Count(&count)
	if count != 0 {
		t.Fatalf("Expected 0 blacklist mappings after cascade deletion, got %d", count)
	}
}
