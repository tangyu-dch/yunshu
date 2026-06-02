package security

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// TestBlacklistRepository_ConcurrentStress 测试高并发场景下黑名单号码的导入、查询和过滤判定的一致性。
// 在 100 个并发 goroutine 激烈竞争写入黑名单号码（SaveNumber）和读取（PageNumbers）时，
// 最终在数据库中落库的所有黑名单号码条数应精准无误，系统无任何状态丢失或脏数据。
func TestBlacklistRepository_ConcurrentStress(t *testing.T) {
	t.Parallel()

	// 1. 初始化隔离的共享缓存 SQLite 内存数据库作为物理全真环境
	db, err := gorm.Open(sqlite.Open("file:security_blacklist_test_db?mode=memory&cache=shared&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)

	// 自动迁移黑名单号码表
	err = db.AutoMigrate(&BlacklistDataModel{})
	if err != nil {
		t.Fatal(err)
	}

	repo := NewBlacklistRepository(db, nil)
	ctx := context.Background()

	// 2. 启动 100 个并发 goroutine 竞争导入黑名单手机号，且互相不会冲突报错
	var wg sync.WaitGroup
	concurrency := 100
	wg.Add(concurrency)

	var successCount int64
	var failureCount int64

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			num := operate.BlacklistNumber{
				Phone:      "1380000" + fmtString(idx, 4),
				BlackLevel: "LEVEL_1",
				Remark:     "高并发压力导入",
			}
			_, err := repo.SaveNumber(ctx, num)
			if err != nil {
				atomic.AddInt64(&failureCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// 3. 强力断言：
	// - 并发写入 100 个独立号码，全部成功，没有失败
	if failureCount > 0 {
		t.Fatalf("【黑名单并发压测失败】: 期望无写入失败，实际失败数为 %d", failureCount)
	}
	if successCount != int64(concurrency) {
		t.Fatalf("【黑名单并发压测失败】: 期望成功 %d 次，但实际成功次数为 %d", concurrency, successCount)
	}

	// 4. 校验数据库最终写入的黑名单号码条数必须精确等于 100
	var count int64
	err = db.Model(&BlacklistDataModel{}).Count(&count).Error
	if err != nil {
		t.Fatal(err)
	}
	if count != int64(concurrency) {
		t.Fatalf("【黑名单并发压测失败】: 期望数据库存在 %d 条黑名单数据，但实际为 %d 条", concurrency, count)
	}

	// 5. 并发查询黑名单号码，模拟 100 个协程高吞吐匹配被叫号码拦截
	var readWg sync.WaitGroup
	readWg.Add(concurrency)
	var readSuccess int64

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer readWg.Done()
			phoneToFind := "1380000" + fmtString(idx, 4)
			res, err := repo.PageNumbers(ctx, operate.BlacklistNumberPageRequest{
				Phone:      phoneToFind,
				PageNumber: 1,
				PageSize:   10,
			})
			if err == nil && len(res.Records) == 1 && res.Records[0].Phone == phoneToFind {
				atomic.AddInt64(&readSuccess, 1)
			}
		}(i)
	}

	readWg.Wait()

	if readSuccess != int64(concurrency) {
		t.Fatalf("【黑名单并发匹配压测失败】: 期望成功命中匹配并拦截 %d 次，实际匹配成功 %d 次", concurrency, readSuccess)
	}
}

func fmtString(v int, width int) string {
	s := strconv.Itoa(v)
	for len(s) < width {
		s = "0" + s
	}
	return s
}
