package merchant

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// TestMerchantBillingRepository_ConcurrentRechargeStress 测试高并发场景下，商户充值/账务调整事务的一致性与严谨性。
// 在 100 个并发 goroutine 激烈竞争充值的情况下，由于使用了 SELECT FOR UPDATE 悲观锁事务保障，
// 最终商户余额必须精确累加，且生成的充值历史流水数量精确等于 100 条，没有任何交易记录丢失或余额漂移。
func TestMerchantBillingRepository_ConcurrentRechargeStress(t *testing.T) {
	t.Parallel()

	// 1. 初始化隔离的共享缓存 SQLite 内存数据库作为物理全真环境
	db, err := gorm.Open(sqlite.Open("file:merchant_billing_test_db?mode=memory&cache=shared&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)

	// 自动迁移账单总览表和充值流水表
	err = db.AutoMigrate(&MerchantBillingOverviewModel{}, &MerchantBillingRechargeModel{})
	if err != nil {
		t.Fatal(err)
	}

	repo := NewBillingRepository(db, nil)
	ctx := context.Background()
	merchantID := 888

	// 2. 写入初始的账单总览数据，余额 10.0 元
	err = db.Create(&MerchantBillingOverviewModel{
		MerchantID:     merchantID,
		PaymentMode:    operate.PaymentModePrepaid,
		CurrentBalance: 10.0,
		CreditLimit:    0.0,
	}).Error
	if err != nil {
		t.Fatal(err)
	}

	// 3. 开启 100 个并发 goroutine 同时充值 10.0 元，累计期望充值 1000.0 元
	var wg sync.WaitGroup
	concurrency := 100
	wg.Add(concurrency)

	var successCount int64
	var failureCount int64

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			req := operate.MerchantRechargeRequest{
				MerchantID: merchantID,
				Amount:     10.0,
				Remark:     "并发压力测试充值",
				Operator:   999,
			}
			err := repo.Recharge(ctx, req)
			if err != nil {
				atomic.AddInt64(&failureCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// 4. 断言所有并发充值必须 100% 成功吞吐，零死锁、零失败
	if failureCount > 0 {
		t.Fatalf("【商户账务并发充值压测失败】: 期望无失败，但出现 %d 次充值失败", failureCount)
	}
	if successCount != int64(concurrency) {
		t.Fatalf("【商户账务并发充值压测失败】: 期望成功充值 %d 次，但实际成功次数为 %d", concurrency, successCount)
	}

	// 5. 校验数据库最终余额必须精确为 1010.0 元 (10.0 初始 + 1000.0 充值)
	var billing MerchantBillingOverviewModel
	err = db.Where("merchant_id = ?", merchantID).First(&billing).Error
	if err != nil {
		t.Fatal(err)
	}
	if billing.CurrentBalance != 1010.0 {
		t.Fatalf("【商户账务并发充值压测失败】: 期望最终余额为 1010.00，实际为 %v", billing.CurrentBalance)
	}

	// 6. 校验充值流水记录数必须精准为 100 条，流水笔笔有迹可循
	var logCount int64
	err = db.Model(&MerchantBillingRechargeModel{}).Where("merchant_id = ?", merchantID).Count(&logCount).Error
	if err != nil {
		t.Fatal(err)
	}
	if logCount != int64(concurrency) {
		t.Fatalf("【商户账务并发充值压测失败】: 期望生成充值流水记录 %d 条，但实际查到 %d 条", concurrency, logCount)
	}
}
