package business

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/infra/merchant"
)

func TestSettlementMemorySettlementStoreDebitAndMarkSettled(t *testing.T) {
	t.Parallel()

	store := NewSettlementMemoryStore()
	store.Balance[88] = 100
	job, err := store.SaveFromOutbox(context.Background(), Entry{
		ID:          "billing:settlement:call-1",
		AggregateID: "call-1",
		Payload: map[string]any{
			"callId":     "call-1",
			"merchantId": 88,
			"amount":     12.5,
			"ratePerMin": 0.3,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	before, after, err := store.DebitBalance(context.Background(), 88, job.Amount)
	if err != nil {
		t.Fatal(err)
	}
	if before != 100 || after != 87.5 {
		t.Fatalf("unexpected balance change: %v -> %v", before, after)
	}
	if err := store.MarkSettled(context.Background(), job.ID, before, after, time.Now()); err != nil {
		t.Fatal(err)
	}
	got := store.SettlementJobs["call-1"]
	if got.Status != StatusSettled || got.BalanceAfter != 87.5 {
		t.Fatalf("unexpected settlement job: %+v", got)
	}
}

func TestSettlementMemoryStoreDebitBalanceReturnsNoOpWhenOverviewNotFound(t *testing.T) {
	t.Parallel()

	store := NewSettlementMemoryStore()
	// Do not populate store.Balance[88] to simulate missing overview record

	job, err := store.SaveFromOutbox(context.Background(), Entry{
		ID:          "billing:settlement:call-2",
		AggregateID: "call-2",
		Payload: map[string]any{
			"callId":     "call-2",
			"merchantId": 88,
			"amount":     10.0,
			"ratePerMin": 0.2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = store.DebitBalance(context.Background(), 88, job.Amount)
	if !errors.Is(err, ErrBillingOverviewNotFound) {
		t.Fatalf("expected ErrBillingOverviewNotFound, got: %v", err)
	}

	err = store.MarkNoOp(context.Background(), job.ID, "merchant billing overview not found")
	if err != nil {
		t.Fatal(err)
	}

	got := store.SettlementJobs["call-2"]
	if got.Status != StatusNoOp {
		t.Fatalf("expected status %s, got: %s", StatusNoOp, got.Status)
	}
	if got.LastError != "merchant billing overview not found" {
		t.Fatalf("expected last error to store reason, got: %s", got.LastError)
	}
}

func TestSettlementSettlementJobModelTableName(t *testing.T) {
	t.Parallel()

	if (SettlementJobModel{}).TableName() != "cc_biz_settlement" {
		t.Fatalf("unexpected table name")
	}
}

func TestSettlementGormStoreDebitBalanceAndMarkNoOp(t *testing.T) {
	t.Parallel()

	// Setup SQLite in-memory DB for GORM testing
	db, err := gorm.Open(sqlite.Open("file:settlement_noop_test_db?mode=memory&cache=shared&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Run migration for necessary models
	err = db.AutoMigrate(&SettlementJobModel{}, &merchant.MerchantBillingOverviewModel{})
	if err != nil {
		t.Fatal(err)
	}

	store := NewSettlementGormStore(db, nil)
	ctx := context.Background()

	// 1. Save settlement job
	job, err := store.SaveFromOutbox(ctx, Entry{
		ID:          "billing:settlement:call-3",
		AggregateID: "call-3",
		Payload: map[string]any{
			"callId":     "call-3",
			"merchantId": 99,
			"amount":     5.5,
			"ratePerMin": 0.15,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Debit balance when billing overview record does not exist
	_, _, err = store.DebitBalance(ctx, 99, job.Amount)
	if !errors.Is(err, ErrBillingOverviewNotFound) {
		t.Fatalf("expected ErrBillingOverviewNotFound when overview does not exist, got: %v", err)
	}

	// 3. Mark job as no-op
	err = store.MarkNoOp(ctx, job.ID, "merchant billing overview not found")
	if err != nil {
		t.Fatal(err)
	}

	// Check job status in database
	var dbJob SettlementJobModel
	err = db.Where("id = ?", job.ID).First(&dbJob).Error
	if err != nil {
		t.Fatal(err)
	}
	if dbJob.Status != StatusNoOp {
		t.Fatalf("expected status in DB to be %s, got %s", StatusNoOp, dbJob.Status)
	}
	if dbJob.LastError != "merchant billing overview not found" {
		t.Fatalf("expected DB last error to store reason, got: %s", dbJob.LastError)
	}

	// 4. Test normal flow when billing overview DOES exist
	err = db.Create(&merchant.MerchantBillingOverviewModel{
		MerchantID:     99,
		CurrentBalance: 100.0,
		CreditLimit:    20.0,
	}).Error
	if err != nil {
		t.Fatal(err)
	}

	before, after, err := store.DebitBalance(ctx, 99, job.Amount)
	if err != nil {
		t.Fatal(err)
	}
	if before != 100.0 || after != 94.5 {
		t.Fatalf("unexpected balance change: %v -> %v", before, after)
	}

	// Check if updated in database
	var dbOverview merchant.MerchantBillingOverviewModel
	err = db.Where("merchant_id = ?", 99).First(&dbOverview).Error
	if err != nil {
		t.Fatal(err)
	}
	if dbOverview.CurrentBalance != 94.5 {
		t.Fatalf("expected CurrentBalance in DB to be 94.5, got %v", dbOverview.CurrentBalance)
	}
}

// TestSettlementGormStore_ConcurrentDebitBalanceStress 测试高并发压力下计费余额的扣减防超扣强一致性。
// 在 100 个并发 goroutine 激烈竞争扣款的情况下，利用 SELECT FOR UPDATE 事务排他锁机制，
// 最终扣除的总成功次数必须精确等于 200 次，且商户余额必须完美扣光归零。
func TestSettlementGormStore_ConcurrentDebitBalanceStress(t *testing.T) {
	t.Parallel()

	// 1. 初始化 SQLite 内存数据库做隔离环境
	db, err := gorm.Open(sqlite.Open("file:settlement_stress_test_db?mode=memory&cache=shared&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	err = db.AutoMigrate(&SettlementJobModel{}, &merchant.MerchantBillingOverviewModel{})
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)

	store := NewSettlementGormStore(db, nil)
	ctx := context.Background()

	// 2. 创建商户，初始余额 100.0 元，CreditLimit 为 0
	merchantID := 1001
	err = db.Create(&merchant.MerchantBillingOverviewModel{
		MerchantID:     merchantID,
		CurrentBalance: 100.0,
		CreditLimit:    0.0,
	}).Error
	if err != nil {
		t.Fatal(err)
	}

	// 3. 启动 100 个 goroutine 并发扣费，每次扣 0.5 元，每个协程循环尝试 3 次，总计尝试 300 次扣费。
	// 由于总额只有 100 元，理应只有 200 次成功，其余 100 次报错余额不足。
	var wg sync.WaitGroup
	concurrency := 100
	wg.Add(concurrency)

	var successCount int64
	var failureCount int64

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				_, _, err := store.DebitBalance(ctx, merchantID, 0.5)
				if err != nil {
					atomic.AddInt64(&failureCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	wg.Wait()

	// 4. 强力断言：
	// - 成功扣除次数正好为 200 次 (100 / 0.5)
	// - 失败扣除次数正好为 100 次 (300 - 200)
	if successCount != 200 {
		t.Fatalf("【并发扣减防超扣压测失败】: 期望成功扣减 200 次，但实际成功次数为 %d", successCount)
	}
	if failureCount != 100 {
		t.Fatalf("【并发扣减防超扣压测失败】: 期望因余额不足失败 100 次，但实际失败次数为 %d", failureCount)
	}

	// 5. 校验数据库最终余额必须精确为 0.0 元
	var billing merchant.MerchantBillingOverviewModel
	err = db.Where("merchant_id = ?", merchantID).First(&billing).Error
	if err != nil {
		t.Fatal(err)
	}
	if billing.CurrentBalance != 0.0 {
		t.Fatalf("【并发扣减防超扣压测失败】: 数据库最终剩余余额期望精准为 0.00，实际剩余为 %v", billing.CurrentBalance)
	}
}

// TestSettlementGormStore_CompleteFlow 模拟完整的计费结算生命周期端到端闭环验证。
// 流程包括：生成 SettlementJob -> 余额校验与悲观锁划扣 -> 标记 settled 归档快照，
// 以及商户 overview 不存在时平滑回退为 no-op 归档，并验证数据持久性状态。
func TestSettlementGormStore_CompleteFlow(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:settlement_flow_test_db?mode=memory&cache=shared&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	err = db.AutoMigrate(&SettlementJobModel{}, &merchant.MerchantBillingOverviewModel{})
	if err != nil {
		t.Fatal(err)
	}

	store := NewSettlementGormStore(db, nil)
	ctx := context.Background()

	// ---- 第一阶段：有余额记录的商户，全流程全链路绿灯结算流程 ----
	mchID := 2001
	callID := "call-flow-success"
	amount := 3.6
	ratePerMin := 0.12

	// 1. 初始化商户余额 10.0 元
	err = db.Create(&merchant.MerchantBillingOverviewModel{
		MerchantID:     mchID,
		CurrentBalance: 10.0,
		CreditLimit:    5.0,
	}).Error
	if err != nil {
		t.Fatal(err)
	}

	// 2. 收到 CTI CDR 扣费事件，幂等落库生成 SettlementJob
	job, err := store.SaveFromOutbox(ctx, Entry{
		ID:          "billing:settlement:" + callID,
		AggregateID: callID,
		Payload: map[string]any{
			"callId":     callID,
			"merchantId": mchID,
			"amount":     amount,
			"ratePerMin": ratePerMin,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 检查落库后初始状态为 pending
	var initialJob SettlementJobModel
	err = db.Where("call_id = ?", callID).First(&initialJob).Error
	if err != nil {
		t.Fatal(err)
	}
	if initialJob.Status != StatusPending {
		t.Fatalf("expected initial status to be pending, got: %s", initialJob.Status)
	}

	// 3. 执行账户余额扣减（悲观锁）
	before, after, err := store.DebitBalance(ctx, mchID, amount)
	if err != nil {
		t.Fatal(err)
	}
	if before != 10.0 || after != 6.4 {
		t.Fatalf("unexpected balance change: %v -> %v", before, after)
	}

	// 4. 结算成功后标记 settled 状态并记录余额快照
	err = store.MarkSettled(ctx, job.ID, before, after, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// 校验数据库中的结算任务状态已被置为 settled
	var finalizedJob SettlementJobModel
	err = db.Where("call_id = ?", callID).First(&finalizedJob).Error
	if err != nil {
		t.Fatal(err)
	}
	if finalizedJob.Status != StatusSettled {
		t.Fatalf("expected finalized status to be settled, got: %s", finalizedJob.Status)
	}
	if finalizedJob.BalanceBefore != 10.0 || finalizedJob.BalanceAfter != 6.4 {
		t.Fatalf("unexpected snapshot balance: %+v", finalizedJob)
	}

	// ---- 第二阶段：没有账费总览记录的商户，平滑触发 NoOp 回退与标记机制 ----
	noMchID := 9999
	noCallID := "call-flow-noop"

	// 1. 幂等生成结算任务
	noopJob, err := store.SaveFromOutbox(ctx, Entry{
		ID:          "billing:settlement:" + noCallID,
		AggregateID: noCallID,
		Payload: map[string]any{
			"callId":     noCallID,
			"merchantId": noMchID,
			"amount":     1.5,
			"ratePerMin": 0.08,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. 扣除余额，期望因为 Overview 不存在而返回特定哨兵错误 ErrBillingOverviewNotFound
	_, _, err = store.DebitBalance(ctx, noMchID, noopJob.Amount)
	if !errors.Is(err, ErrBillingOverviewNotFound) {
		t.Fatalf("expected ErrBillingOverviewNotFound, got: %v", err)
	}

	// 3. 平滑进入 no-op 回归处理，标记结算任务为 StatusNoOp，保持计费 settlement 独立链不断掉
	err = store.MarkNoOp(ctx, noopJob.ID, "merchant billing overview not found")
	if err != nil {
		t.Fatal(err)
	}

	// 4. 校验数据库，状态应标记为 no_op
	var dbNoopJob SettlementJobModel
	err = db.Where("call_id = ?", noCallID).First(&dbNoopJob).Error
	if err != nil {
		t.Fatal(err)
	}
	if dbNoopJob.Status != StatusNoOp {
		t.Fatalf("expected status to be no_op, got: %s", dbNoopJob.Status)
	}
	if dbNoopJob.LastError != "merchant billing overview not found" {
		t.Fatalf("expected last_error to be logged, got: %s", dbNoopJob.LastError)
	}
}
