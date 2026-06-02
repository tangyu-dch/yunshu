package merchant

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
	"yunshu/internal/infra/telephony"
)

// TestRateRepository_ConcurrentStress 测试高并发场景下费率数据的查询、更新与引用关系校验的并发一致性与鲁棒性。
// 在 100 个 goroutine 并发执行 Save、GetByID 与 HasBindings 引用排他性校验时，
// 期望所有请求均安全吞吐，不报任何死锁或脏状态，费率引用保护 Fail-Closed 机制表现完全正确。
func TestRateRepository_ConcurrentStress(t *testing.T) {
	t.Parallel()

	// 1. 初始化隔离的共享缓存 SQLite 内存数据库作为物理全真环境
	db, err := gorm.Open(sqlite.Open("file:merchant_rate_test_db?mode=memory&cache=shared&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)

	// 自动迁移费率表、商户费率关系表和网关表
	err = db.AutoMigrate(&CallRateModel{}, &CallRateMerchantModel{}, &telephony.GatewayModel{})
	if err != nil {
		t.Fatal(err)
	}

	repo := NewRateRepository(db, nil)
	ctx := context.Background()

	// 2. 写入测试费率数据
	rateID := 101
	err = db.Create(&CallRateModel{
		ID:           rateID,
		RateName:     "并发压力测试费率",
		BillingPrice: 0.12,
		BillingCycle: 60,
		Enable:       true,
		DelFlag:      false,
	}).Error
	if err != nil {
		t.Fatal(err)
	}

	// 3. 写入网关绑定关系，网关 rate_id 绑定 101，验证引用拦截有效性
	err = db.Create(&telephony.GatewayModel{
		ID:       55,
		Name:     "网关1",
		RateID:   rateID,
		Enable:   true,
		DelFlag:  false,
		Priority: 1,
	}).Error
	if err != nil {
		t.Fatal(err)
	}

	// 4. 启动 100 个并发 goroutine 竞争读写与引用关系判断
	var wg sync.WaitGroup
	concurrency := 100
	wg.Add(concurrency)

	var readSuccess int64
	var saveSuccess int64
	var bindSuccess int64

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			// A. 并发读取
			_, err := repo.GetByID(ctx, rateID)
			if err == nil {
				atomic.AddInt64(&readSuccess, 1)
			}

			// B. 并发校验引用，由于已绑定网关 55，应当 100% 成功返回 hasBinding = true
			hasBinding, err := repo.HasBindings(ctx, []int{rateID})
			if err == nil && hasBinding {
				atomic.AddInt64(&bindSuccess, 1)
			}

			// C. 并发更新名称，检验事务隔离与锁排队下的 Save 并发性
			_, err = repo.Save(ctx, operate.Rate{
				ID:           rateID,
				RateName:     "并发压力测试费率-新",
				BillingPrice: 0.12,
				BillingCycle: 60,
			})
			if err == nil {
				atomic.AddInt64(&saveSuccess, 1)
			}
		}(i)
	}

	wg.Wait()

	// 5. 强力断言 100 次高频竞争下，各项功能读写吞吐完好、数据强一致
	if readSuccess != int64(concurrency) {
		t.Fatalf("【费率并发压测失败】: 并发读取期望成功 %d 次，但实际为 %d", concurrency, readSuccess)
	}
	if bindSuccess != int64(concurrency) {
		t.Fatalf("【费率并发压测失败】: 并发绑定引用检测期望全命中，但成功命中次数为 %d", bindSuccess)
	}
	if saveSuccess != int64(concurrency) {
		t.Fatalf("【费率并发压测失败】: 并发更新期望成功 %d 次，但实际为 %d", concurrency, saveSuccess)
	}

	// 6. 校验数据库最终更新的名称一致性
	var updated CallRateModel
	err = db.Where("id = ?", rateID).First(&updated).Error
	if err != nil {
		t.Fatal(err)
	}
	if updated.RateName != "并发压力测试费率-新" {
		t.Fatalf("【费率并发压测失败】: 期望最终费率名称更新成功，但实际名称为: %s", updated.RateName)
	}
}
