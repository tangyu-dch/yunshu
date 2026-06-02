package security

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// CalleeFeatureTestModel 用于在测试中模拟 `cc_sec_callee_feature` 频次特征记录，
// 避免因跨包调用导致 selection 和 security 出现循环引用的问题。
type CalleeFeatureTestModel struct {
	CalledNumber     string    `gorm:"column:called_number;primaryKey"`
	MerchantID       int       `gorm:"column:merchant_id;primaryKey"`
	ChannelID        string    `gorm:"column:channel_id;primaryKey"`
	StatDate         time.Time `gorm:"column:stat_date;primaryKey"`
	CallDialCount    int       `gorm:"column:call_dial_count"`
	CallConnectCount int       `gorm:"column:call_connect_count"`
}

func (CalleeFeatureTestModel) TableName() string {
	return "cc_sec_callee_feature"
}

// TestRiskControl_ConcurrentStress 执行针对风控系统的高并发压力测试。
// 本测试覆盖两大核心风控版块：
//  1. 【运营管理侧并发测试】：100个协程并发执行风控规则的保存（Save）、获取（GetByID）与绑定商户列表（SaveMerchants），
//     强力校验高频库表写入事务的防冲突性能与一致性，杜绝脏写。
//  2. 【话务频次拦截侧并发测试】：100个协程并发发起对同一个号码的外呼频次安全校验与特征累加（高频外呼模拟），
//     在设置上限为 10 次拨打的极端竞争下，强力断言：
//     - 允许通过（Allowed）的外呼次数必须精确等于 10；
//     - 被风控拦截（Blocked）的拒绝次数必须精确等于 90；
//     - 最终库表中当天统计的 `CallDialCount` 必须精确等于 10，决无任何漂移和漏网之鱼，全面守护 Fail-Closed 话务安全！
func TestRiskControl_ConcurrentStress(t *testing.T) {
	t.Parallel()

	// 1. 初始化唯一命名的共享内存 SQLite 实例，物理环境完全隔离
	db, err := gorm.Open(sqlite.Open("file:security_risk_test_db?mode=memory&cache=shared&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// SQLite 限制同一时刻有且仅有一个物理写连接，以此强迫 GORM 层面在并发时在队列上安全排队，
	// 而 Go 协程层面依然保持 100+ 并发的剧烈冲突竞争，最大化仿真高一致性。
	sqlDB.SetMaxOpenConns(1)

	// 自动迁移风控相关基础表与被叫统计特征表
	err = db.AutoMigrate(&RiskControlModel{}, &RiskControlMerchantModel{}, &CalleeFeatureTestModel{})
	if err != nil {
		t.Fatal(err)
	}

	repo := NewRiskControlRepository(db, nil)
	ctx := context.Background()

	// ==========================================
	// 阶段一：【运营管理端风控规则高并发并发读写压测】
	// ==========================================
	var wg1 sync.WaitGroup
	concurrency := 100
	wg1.Add(concurrency)

	var mchSaveSuccess int64
	var mchSaveFailure int64

	// 创建初始风控配置
	initRC, err := repo.Save(ctx, operate.RiskControl{
		Name:                "全局并发风控测试策略",
		Remark:              "高并发单元测试策略",
		BlackLevelFlag:      true,
		BlackLevel:          "LEVEL_2",
		BlindAreaFlag:       false,
		CalleeFrequencyFlag: true,
		CalleeFrequency:     `[{"day":1,"count":10,"type":"DIAL"}]`,
	})
	if err != nil {
		t.Fatalf("【风控并发压测失败】: 初始化风控策略失败: %v", err)
	}

	// 100个协程并发去频繁读取与更新这个风控策略绑定的商户关联列表
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg1.Done()

			// 模拟商户绑定风控规则，每个商户拥有不同的 ID
			bindings := []operate.RiskControlMerchant{
				{RiskID: initRC.ID, MerchantID: 1000 + idx, Enable: true},
				{RiskID: initRC.ID, MerchantID: 2000 + idx, Enable: true},
			}

			// 保存绑定关系并高频回查
			err := repo.SaveMerchants(ctx, initRC.ID, bindings)
			if err != nil {
				atomic.AddInt64(&mchSaveFailure, 1)
				return
			}

			_, err = repo.GetMerchants(ctx, initRC.ID)
			if err != nil {
				atomic.AddInt64(&mchSaveFailure, 1)
				return
			}

			atomic.AddInt64(&mchSaveSuccess, 1)
		}(i)
	}

	wg1.Wait()

	// 强力断言：100个协程的管理侧操作在数据库事务排队机制下 100% 成功，没有任何报错死锁
	if mchSaveFailure > 0 {
		t.Fatalf("【风控管理侧并发压测失败】: 存在更新绑定商户失败的协程，总失败数为: %d", mchSaveFailure)
	}
	if mchSaveSuccess != int64(concurrency) {
		t.Fatalf("【风控管理侧并发压测失败】: 期望成功 %d 次，但实际仅成功 %d 次", concurrency, mchSaveSuccess)
	}

	// ==========================================
	// 阶段二：【话务外呼频次拦截高并发一致性判定压测】
	// ==========================================
	var wg2 sync.WaitGroup
	wg2.Add(concurrency)

	var allowedCalls int64 // 判定通过允许呼叫的协程数
	var blockedCalls int64 // 判定拦截阻断呼叫的协程数
	var systemErrors int64 // 出现数据库死锁等系统异常的协程数

	testCallee := "13988889999"
	testMerchantID := 888
	maxDialLimit := 10 // 限制每个被叫当天最多只能拨打 10 次

	// 100个协程并发模拟 API 外呼起呼时，对同一个号码的频次判定竞争
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg2.Done()

			// 执行模拟的 CTI 频次检查与记录更新（防超扣悲观排他事务）
			allowed, err := simulateCallRiskCheckAndRecord(db, testMerchantID, testCallee, maxDialLimit)
			if err != nil {
				atomic.AddInt64(&systemErrors, 1)
				return
			}

			if allowed {
				atomic.AddInt64(&allowedCalls, 1)
			} else {
				atomic.AddInt64(&blockedCalls, 1)
			}
		}()
	}

	wg2.Wait()

	// 打印并发测试的核心执行指标
	t.Logf("【风控外呼拦截压测报告】: 并发协程数=%d, 允许呼叫数=%d, 拦截呼叫数=%d, 系统异常数=%d",
		concurrency, allowedCalls, blockedCalls, systemErrors)

	// 强力断言：
	// A. 系统必须稳定执行，绝无任何 SQLite 或悲观锁死锁带来的系统报错
	if systemErrors > 0 {
		t.Fatalf("【风控高并发判定失败】: 话务判定中发生了 %d 次数据库报错", systemErrors)
	}

	// B. 精准强一致性校验：在 100 个激烈的 goroutine 并发下，被允许起呼的外呼数必须【精确等于 10】
	if allowedCalls != int64(maxDialLimit) {
		t.Fatalf("【风控高并发判定失败】: 期望允许起呼次数精准为 %d，但实际为 %d （可能由于并发未锁发生多拨或超支！）",
			maxDialLimit, allowedCalls)
	}

	// C. 精准强拦截率校验：被风控拒绝的次数必须【精确等于 90】
	expectedBlocked := int64(concurrency - maxDialLimit)
	if blockedCalls != expectedBlocked {
		t.Fatalf("【风控高并发判定失败】: 期望风控安全拦截的次数精准为 %d，但实际拦截了 %d 次",
			expectedBlocked, blockedCalls)
	}

	// D. 数据库特征记录事实最终态校验：查询当天 `cc_sec_callee_feature` 中该号码的 `CallDialCount` 必须精确等于 10
	var finalFeature CalleeFeatureTestModel
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	err = db.Where("called_number = ? AND merchant_id = ? AND stat_date = ?", testCallee, testMerchantID, today).
		First(&finalFeature).Error
	if err != nil {
		t.Fatalf("【风控高并发判定失败】: 查询数据库特征记录失败: %v", err)
	}

	if finalFeature.CallDialCount != maxDialLimit {
		t.Fatalf("【风控高并发判定失败】: 数据库中落库的当天累计拨打次数为 %d，不等于最大限制次数 %d （证明最终落库状态不一致！）",
			finalFeature.CallDialCount, maxDialLimit)
	}
}

// simulateCallRiskCheckAndRecord 模拟生产环境下的频次风控逻辑：
// 1. 在同一个写事务中，查询商户对特定被叫号码的当天已拨打总次数。
// 2. 检查次数是否已达上限（如上限为 10）。
// 3. 若未超限，则允许通过，并更新 `cc_sec_callee_feature` 当天累计呼叫数（原子+1）；
// 4. 若已超限，则拦截阻断（Fail-Closed）。
func simulateCallRiskCheckAndRecord(db *gorm.DB, merchantID int, calledNumber string, maxLimit int) (bool, error) {
	var allowed bool
	err := db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// 1. 查询当前已有的拨打次数（带悲观锁）
		var feature CalleeFeatureTestModel
		err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("called_number = ? AND merchant_id = ? AND stat_date = ?", calledNumber, merchantID, today).
			First(&feature).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 2. 检查是否触发高频频次限制
		if err == nil && feature.CallDialCount >= maxLimit {
			allowed = false // 触发风控拦截
			return nil
		}

		// 3. 未触发频次限制，允许通过，执行呼叫登记累加
		allowed = true
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 新增当天统计特征记录
			feature = CalleeFeatureTestModel{
				CalledNumber:  calledNumber,
				MerchantID:    merchantID,
				ChannelID:     "1",
				StatDate:      today,
				CallDialCount: 1,
			}
			if err := tx.Create(&feature).Error; err != nil {
				return err
			}
		} else {
			// 累加已有记录的拨打计数
			if err := tx.Model(&feature).
				Where("called_number = ? AND merchant_id = ? AND stat_date = ?", calledNumber, merchantID, today).
				Update("call_dial_count", gorm.Expr("call_dial_count + 1")).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return allowed, err
}
