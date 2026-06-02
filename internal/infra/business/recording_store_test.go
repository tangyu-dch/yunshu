package business

import (
	"context"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRecordingMemoryRecordingStoreSaveFromOutbox(t *testing.T) {
	t.Parallel()

	store := NewRecordingMemoryStore()
	job, err := store.SaveFromOutbox(context.Background(), Entry{
		ID:          "cdr:recording:call-1",
		AggregateID: "call-1",
		Payload: map[string]any{
			"callId":         "call-1",
			"merchantId":     88,
			"recordFilePath": "/record/call-1.wav",
			"sourceOutboxId": "cdr:call-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.MerchantID != 88 || job.RecordFile != "/record/call-1.wav" || job.Status != StatusPending {
		t.Fatalf("unexpected recording job: %+v", job)
	}
}

func TestRecordingMemoryRecordingStoreSkipsMissingRecordFile(t *testing.T) {
	t.Parallel()

	store := NewRecordingMemoryStore()
	if _, err := store.SaveFromOutbox(context.Background(), Entry{AggregateID: "call-1", Payload: map[string]any{"callId": "call-1"}}); err != nil {
		t.Fatal(err)
	}
	if store.RecordingJobs["call-1"].Status != StatusSkipped {
		t.Fatalf("expected skipped job: %+v", store.RecordingJobs["call-1"])
	}
}

func TestRecordingMemoryRecordingStoreMarksUploadState(t *testing.T) {
	t.Parallel()

	store := NewRecordingMemoryStore()
	job, err := store.SaveFromOutbox(context.Background(), Entry{AggregateID: "call-1", Payload: map[string]any{"callId": "call-1", "recordFilePath": "/record/call-1.wav"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkFailed(context.Background(), job.ID, "upload timeout"); err != nil {
		t.Fatal(err)
	}
	if store.RecordingJobs["call-1"].Status != StatusFailed || store.RecordingJobs["call-1"].Attempts != 1 {
		t.Fatalf("expected failed job: %+v", store.RecordingJobs["call-1"])
	}
	if err := store.MarkUploaded(context.Background(), job.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if store.RecordingJobs["call-1"].Status != StatusUploaded || store.RecordingJobs["call-1"].UploadedAt.IsZero() {
		t.Fatalf("expected uploaded job: %+v", store.RecordingJobs["call-1"])
	}
}

func TestRecordingRecordingJobModelTableName(t *testing.T) {
	t.Parallel()

	if (RecordingJobModel{}).TableName() != "cc_biz_recording" {
		t.Fatalf("unexpected table name")
	}
}

// TestRecordingGormStore_ConcurrentIdempotency 测试高并发场景下录音任务幂等落库的一致性与健壮性。
// 通过 50 个 goroutine 并发用相同的 CallID 激烈竞争 SaveFromOutbox，
// 期望最终在数据库中仅保留唯一的、不产生任何脏数据的一条录音记录，完美模拟了高并发消息重放时的落库表现。
func TestRecordingGormStore_ConcurrentIdempotency(t *testing.T) {
	t.Parallel()

	// 1. 初始化 SQLite 内存数据库
	db, err := gorm.Open(sqlite.Open("file:recording_test_db?mode=memory&cache=shared&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	err = db.AutoMigrate(&RecordingJobModel{})
	if err != nil {
		t.Fatal(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)

	store := NewRecordingGormStore(db, nil)
	ctx := context.Background()
	callID := "concurrent-recording-call-id"

	// 2. 模拟 50 个并发 goroutine 竞争保存相同的录音文件 Entry
	var wg sync.WaitGroup
	concurrency := 50
	wg.Add(concurrency)

	errChan := make(chan error, concurrency)

	entry := Entry{
		ID:          "cdr:recording:" + callID,
		AggregateID: callID,
		Payload: map[string]any{
			"callId":         callID,
			"merchantId":     88,
			"recordFilePath": "/record/stress-test.wav",
			"sourceOutboxId": "cdr:outbox:stress",
		},
	}

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_, err := store.SaveFromOutbox(ctx, entry)
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// 3. 校验并断言所有并发请求都未产生致命报错，数据库成功执行 OnConflict 更新
	for err := range errChan {
		t.Fatalf("【录音高并发落库压测失败】: 期望幂等跳过/更新无报错，但出现错误: %v", err)
	}

	// 4. 从数据库查询，断言必须仅有一条该 CallID 的数据
	var count int64
	err = db.Model(&RecordingJobModel{}).Where("call_id = ?", callID).Count(&count).Error
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("【录音高并发落库压测失败】: 期望在数据库中只落库一条记录，但实际查到 %d 条记录", count)
	}

	// 5. 校验录音记录字段
	var model RecordingJobModel
	err = db.Where("call_id = ?", callID).First(&model).Error
	if err != nil {
		t.Fatal(err)
	}
	if model.Status != StatusPending || model.RecordFile != "/record/stress-test.wav" {
		t.Fatalf("【录音高并发落库压测失败】: 录音记录状态或路径不匹配期望: %+v", model)
	}

	// 6. 进行上传成功和失败的方法测试，确保状态流转正常
	err = store.MarkFailed(ctx, model.ID, "temporary connection reset")
	if err != nil {
		t.Fatal(err)
	}

	err = db.Where("call_id = ?", callID).First(&model).Error
	if err != nil {
		t.Fatal(err)
	}
	if model.Status != StatusFailed || model.Attempts != 1 || model.LastError != "temporary connection reset" {
		t.Fatalf("【录音状态流转测试失败】: 期望失败状态及重试次数增加，实际为: %+v", model)
	}

	err = store.MarkUploaded(ctx, model.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	err = db.Where("call_id = ?", callID).First(&model).Error
	if err != nil {
		t.Fatal(err)
	}
	if model.Status != StatusUploaded || model.UploadedAt == nil {
		t.Fatalf("【录音状态流转测试失败】: 期望更新为上传成功状态且记录时间戳，实际为: %+v", model)
	}
}
