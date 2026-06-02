package telephony

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DummyDispatcherModel 用于在测试中模拟 cc_tel_freeswitch 表结构，避免循环依赖
type DummyDispatcherModel struct {
	ID          int       `gorm:"column:id;primaryKey"`
	SetID       int       `gorm:"column:set_id"`
	Destination string    `gorm:"column:destination"`
	Flags       int       `gorm:"column:flags"`
	Priority    int       `gorm:"column:priority"`
	Attrs       string    `gorm:"column:attrs"`
	Description string    `gorm:"column:description"`
	Enable      bool      `gorm:"column:enable"`
	DelFlag     bool      `gorm:"column:del_flag"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

func (DummyDispatcherModel) TableName() string {
	return "cc_res_freeswitch"
}

func TestFreeswitchModelAutoSyncToKamailioDispatcher(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// 1. 迁移 freeswitch 和 cc_res_freeswitch 模拟表
	if err := db.AutoMigrate(&FreeswitchModel{}, &DummyDispatcherModel{}); err != nil {
		t.Fatal(err)
	}

	registry := NewGormRegistry(db)
	ctx := context.Background()

	// 2. 测试新增 FreeSWITCH 节点时的自动同步
	node := Node{
		ID:           101,
		Address:      "192.168.1.10",
		LocalAddress: "192.168.1.10",
		ESLPort:      8021,
		SIPPort:      5060,
		Password:     "ClueCon",
		SetID:        1,
		Weight:       100,
		CC:           50,
		Enable:       true,
	}

	err = registry.Upsert(ctx, node)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// 验证 cc_res_freeswitch 是否成功创建了对应记录
	var disp DummyDispatcherModel
	err = db.Where("description = ?", "FS-Node:101").First(&disp).Error
	if err != nil {
		t.Fatalf("failed to find synced dispatcher: %v", err)
	}

	if disp.Destination != "sip:192.168.1.10:5060" || disp.SetID != 1 || disp.Priority != 100 || disp.Attrs != "max-concurrency=50" || disp.Flags != 0 || !disp.Enable || disp.DelFlag {
		t.Fatalf("unexpected synced dispatcher fields: %+v", disp)
	}

	// 3. 测试更新 FreeSWITCH 节点时的自动同步 (例如更改权重、并发以及启用状态)
	node.Weight = 200
	node.CC = 150
	node.Enable = false // 禁用

	err = registry.Upsert(ctx, node)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	err = db.Where("description = ?", "FS-Node:101").First(&disp).Error
	if err != nil {
		t.Fatal(err)
	}

	if disp.Priority != 200 || disp.Attrs != "max-concurrency=150" || disp.Flags != 1 || disp.Enable {
		t.Fatalf("unexpected updated fields: %+v", disp)
	}

	// 4. 测试逻辑删除 FreeSWITCH 节点时的自动同步
	err = registry.Delete(ctx, 101)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	err = db.Where("description = ?", "FS-Node:101").First(&disp).Error
	if err != nil {
		t.Fatal(err)
	}

	if !disp.DelFlag || disp.Enable || disp.Flags != 1 {
		t.Fatalf("unexpected deleted fields: %+v", disp)
	}
}

func TestFreeswitchModelSyncNoTableSafety(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// 仅迁移 freeswitch 表，故意不迁移 cc_tel_freeswitch
	if err := db.AutoMigrate(&FreeswitchModel{}); err != nil {
		t.Fatal(err)
	}

	registry := NewGormRegistry(db)
	ctx := context.Background()

	node := Node{
		ID:           102,
		Address:      "192.168.1.11",
		LocalAddress: "192.168.1.11",
		ESLPort:      8021,
		SIPPort:      5060,
		Password:     "ClueCon",
		SetID:        1,
		Weight:       100,
		CC:           50,
		Enable:       true,
	}

	// 应当成功执行，由于检测到没有 cc_res_freeswitch 表，直接安全返回 nil 从而不报错
	err = registry.Upsert(ctx, node)
	if err != nil {
		t.Fatalf("Upsert should be safe and succeed even if cc_res_freeswitch is missing: %v", err)
	}
}
