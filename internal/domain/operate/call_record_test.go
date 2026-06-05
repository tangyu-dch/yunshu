package operate_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"yunshu/internal/domain/operate"
)

func TestCallRecordManagementService(t *testing.T) {
	t.Parallel()

	repo := newFakeCallRecordRepository()
	service := &operate.CallRecordManagementService{Repository: repo}

	// 准备数据
	repo.records["call-123"] = operate.CallRecord{
		CallID:     "call-123",
		MerchantID: 1001,
		Caller:     "13800000000",
		Callee:     "13900000000",
	}

	// 1. 分页查询
	page, err := service.Page(context.Background(), operate.CallRecordPageRequest{
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Fatalf("unexpected page result: %+v", page)
	}

	// 2. 详情查询
	detail, err := service.Detail(context.Background(), "call-123")
	if err != nil {
		t.Fatal(err)
	}
	if detail.CallID != "call-123" {
		t.Fatalf("expected call-123, got %s", detail.CallID)
	}

	// 3. 空 ID 校验
	_, err = service.Detail(context.Background(), "")
	if !errors.Is(err, operate.ErrInvalidCallRecord) {
		t.Fatalf("expected ErrInvalidCallRecord for empty ID, got %v", err)
	}

	// 4. 不存在记录校验
	_, err = service.Detail(context.Background(), "call-nonexistent")
	if !errors.Is(err, operate.ErrCallRecordNotFound) {
		t.Fatalf("expected ErrCallRecordNotFound, got %v", err)
	}
}

// fakeCallRecordRepository
type fakeCallRecordRepository struct {
	records map[string]operate.CallRecord
}

func newFakeCallRecordRepository() *fakeCallRecordRepository {
	return &fakeCallRecordRepository{records: make(map[string]operate.CallRecord)}
}

func (r *fakeCallRecordRepository) Page(_ context.Context, req operate.CallRecordPageRequest) (operate.CallRecordPageResult, error) {
	list := make([]operate.CallRecord, 0, len(r.records))
	for _, v := range r.records {
		list = append(list, v)
	}
	return operate.CallRecordPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      int64(len(list)),
		Records:    list,
	}, nil
}

func (r *fakeCallRecordRepository) GetByCallID(_ context.Context, callID string) (operate.CallRecord, error) {
	record, ok := r.records[callID]
	if !ok {
		return operate.CallRecord{}, operate.ErrCallRecordNotFound
	}
	return record, nil
}

func TestCallRecordSipTrace(t *testing.T) {
	t.Parallel()

	// 1. 启动 miniredis 模拟 Redis 实例
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	// 2. 创建 redis 客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	// 3. 创建服务
	repo := newFakeCallRecordRepository()
	service := &operate.CallRecordManagementService{
		Repository:  repo,
		RedisClient: redisClient,
	}

	// 准备呼叫记录
	repo.records["call-trace-123"] = operate.CallRecord{
		CallID:     "call-trace-123",
		UUID:       "uuid-trace-456",
		MerchantID: 1001,
	}

	// 4. 播种 Redis 模拟信令数据
	// 格式: timestamp###method###status###from_ip:port###to_ip:port###rawMsg
	traceData := []string{
		"1780630821.100000###INVITE###0###192.168.1.10:5060###192.168.1.20:5060###INVITE sip:alice@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n",
		"1780630821.200000###100###100 Trying###192.168.1.20:5060###192.168.1.10:5060###SIP/2.0 100 Trying\r\n\r\n",
		"1780630821.300000###200###200 OK###192.168.1.20:5060###192.168.1.10:5060###SIP/2.0 200 OK\r\nContent-Length: 12\r\n\r\nSDP Body Here",
	}

	for _, item := range traceData {
		if err := redisClient.RPush(context.Background(), "sip_trace:call-trace-123", item).Err(); err != nil {
			t.Fatal(err)
		}
	}

	// 5. 调用接口并验证解析正确性
	res, err := service.SipTrace(context.Background(), "call-trace-123")
	if err != nil {
		t.Fatal(err)
	}

	if res.CallID != "call-trace-123" {
		t.Fatalf("expected CallID call-trace-123, got %s", res.CallID)
	}

	// 校验去重后的节点 IP
	if len(res.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %+v", len(res.Nodes), res.Nodes)
	}

	// 校验信令条数
	if len(res.Trace) != 3 {
		t.Fatalf("expected 3 trace items, got %d", len(res.Trace))
	}

	// 首条 INVITE 断言 (status 应为空串)
	first := res.Trace[0]
	if first.Method != "INVITE" || first.Status != "" || first.FromIP != "192.168.1.10:5060" || first.ToIP != "192.168.1.20:5060" {
		t.Fatalf("unexpected first trace item: %+v", first)
	}
	if first.RawMsg != "INVITE sip:alice@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n" {
		t.Fatalf("unexpected RawMsg: %q", first.RawMsg)
	}

	// 第三条 200 OK 断言 (status 为 200 OK，包含 SDP)
	third := res.Trace[2]
	if third.Method != "200" || third.Status != "200 OK" || third.FromIP != "192.168.1.20:5060" || third.ToIP != "192.168.1.10:5060" {
		t.Fatalf("unexpected third trace item: %+v", third)
	}
	if third.RawMsg != "SIP/2.0 200 OK\r\nContent-Length: 12\r\n\r\nSDP Body Here" {
		t.Fatalf("unexpected RawMsg: %q", third.RawMsg)
	}

	// 6. 校验 UUID 降级/回退逻辑
	for _, item := range traceData {
		if err := redisClient.RPush(context.Background(), "sip_trace:uuid-trace-456", item).Err(); err != nil {
			t.Fatal(err)
		}
	}

	repo.records["call-uuid-only"] = operate.CallRecord{
		CallID:     "call-uuid-only",
		UUID:       "uuid-trace-456",
		MerchantID: 1001,
	}

	resFallback, err := service.SipTrace(context.Background(), "call-uuid-only")
	if err != nil {
		t.Fatal(err)
	}
	if len(resFallback.Trace) != 3 {
		t.Fatalf("expected 3 trace items from UUID fallback, got %d", len(resFallback.Trace))
	}
}
