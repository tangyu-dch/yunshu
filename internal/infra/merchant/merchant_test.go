package merchant

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

func TestMerchantRepository_ExtensionCascades(t *testing.T) {
	t.Parallel()

	// 1. 初始化 SQLite 内存数据库进行隔离测试
	db, err := gorm.Open(sqlite.Open("file:merchant_test_db?mode=memory&cache=shared&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)

	// 自动迁移商户表、分机表、账单总览表、费率绑定表
	err = db.AutoMigrate(&MerchantModel{}, &ExtensionModel{}, &MerchantBillingOverviewModel{}, &CallRateMerchantModel{})
	if err != nil {
		t.Fatal(err)
	}

	repo := NewMerchantRepository(db, nil, nil)
	ctx := context.Background()

	// 2. 新增商户，设置 sipDomain = "test.yunshu.com", maxAgents = 2
	mch, err := repo.Save(ctx, operate.Merchant{
		Name:      "测试级联商户",
		Account:   "test_cascade",
		SipDomain: "test.yunshu.com",
		MaxAgents: 2,
		Enable:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 验证是否级联自动生成了 2 个分机
	var exts []ExtensionModel
	err = db.Where("merchant_id = ? AND del_flag = ?", mch.ID, false).Find(&exts).Error
	if err != nil {
		t.Fatal(err)
	}
	if len(exts) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(exts))
	}

	// 验证分机信息和哈希密码是否自动计算
	for _, ext := range exts {
		if ext.SipDomain != "test.yunshu.com" {
			t.Errorf("extension %s has incorrect domain: %s", ext.ExtensionNumber, ext.SipDomain)
		}
		expectedHA1 := calculateHA1(ext.ExtensionNumber, "test.yunshu.com", ext.Password)
		expectedHA1b := calculateHA1b(ext.ExtensionNumber, "test.yunshu.com", ext.Password)
		if ext.HA1 != expectedHA1 || ext.HA1b != expectedHA1b {
			t.Errorf("extension %s has incorrect hashes, ha1: %s, ha1b: %s", ext.ExtensionNumber, ext.HA1, ext.HA1b)
		}
	}

	// 3. 更新商户，将 sipDomain 更改为 "prod.yunshu.com"
	mch.SipDomain = "prod.yunshu.com"
	_, err = repo.Save(ctx, mch)
	if err != nil {
		t.Fatal(err)
	}

	// 再次验证分机域名与哈希是否级联重算
	var updatedExts []ExtensionModel
	err = db.Where("merchant_id = ? AND del_flag = ?", mch.ID, false).Find(&updatedExts).Error
	if err != nil {
		t.Fatal(err)
	}

	for _, ext := range updatedExts {
		if ext.SipDomain != "prod.yunshu.com" {
			t.Errorf("updated extension %s has incorrect domain: %s", ext.ExtensionNumber, ext.SipDomain)
		}
		expectedHA1 := calculateHA1(ext.ExtensionNumber, "prod.yunshu.com", ext.Password)
		expectedHA1b := calculateHA1b(ext.ExtensionNumber, "prod.yunshu.com", ext.Password)
		if ext.HA1 != expectedHA1 || ext.HA1b != expectedHA1b {
			t.Errorf("updated extension %s has incorrect hashes, ha1: %s, ha1b: %s", ext.ExtensionNumber, ext.HA1, ext.HA1b)
		}
	}
}
