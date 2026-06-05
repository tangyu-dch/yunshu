package operate_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"yunshu/internal/domain/operate"
)

func TestIPBlockManagementService(t *testing.T) {
	t.Parallel()

	fakeLogRepo := &fakeIPBlockLogRepository{}
	fakeConfigRepo := &fakeConfigRepository{configs: make(map[string]string)}

	// 初始化服务
	service := operate.NewIPBlockManagementService(fakeLogRepo, fakeConfigRepo, nil, nil)

	// 1. 测试初始国家列表为空
	countries, err := service.GetBlockedCountries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if countries != "" {
		t.Fatalf("expected empty countries, got: %s", countries)
	}

	// 2. 测试保存拦截国家列表
	err = service.SaveBlockedCountries(context.Background(), "US,de,  gb ")
	if err != nil {
		t.Fatal(err)
	}

	countries, err = service.GetBlockedCountries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if countries != "US,DE,GB" {
		t.Fatalf("expected US,DE,GB, got: %s", countries)
	}

	// 3. 测试记录拦截日志
	log1, err := service.LogBlockEvent(context.Background(), "192.0.2.1", "US", "CallID-1", "INVITE")
	if err != nil {
		t.Fatal(err)
	}
	if log1.IP != "192.0.2.1" || log1.CountryCode != "US" || log1.CallID != "CallID-1" || log1.Method != "INVITE" {
		t.Fatalf("unexpected log: %+v", log1)
	}

	// 4. 再记录一条拦截日志
	_, err = service.LogBlockEvent(context.Background(), "192.0.2.2", "DE", "CallID-2", "REGISTER")
	if err != nil {
		t.Fatal(err)
	}

	// 5. 测试分页查询所有日志
	pageResult, err := service.Page(context.Background(), operate.IPBlockLogPageRequest{
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pageResult.Total != 2 {
		t.Fatalf("expected total 2, got: %d", pageResult.Total)
	}
	if len(pageResult.Records) != 2 {
		t.Fatalf("expected 2 records, got: %d", len(pageResult.Records))
	}

	// 6. 测试根据国家条件过滤
	filteredResult, err := service.Page(context.Background(), operate.IPBlockLogPageRequest{
		PageNumber:  1,
		PageSize:    10,
		CountryCode: "DE",
	})
	if err != nil {
		t.Fatal(err)
	}
	if filteredResult.Total != 1 || filteredResult.Records[0].IP != "192.0.2.2" {
		t.Fatalf("expected 1 record from DE with IP 192.0.2.2, got: %+v", filteredResult)
	}

	// 7. 测试无效输入校验
	_, err = service.LogBlockEvent(context.Background(), "", "US", "", "")
	if !errors.Is(err, operate.ErrInvalidIPBlock) {
		t.Fatalf("expected ErrInvalidIPBlock, got: %v", err)
	}
}

// fakeIPBlockLogRepository 模拟 IP 拦截审计日志仓储
type fakeIPBlockLogRepository struct {
	mu   sync.RWMutex
	logs []operate.IPBlockLog
}

func (r *fakeIPBlockLogRepository) Page(_ context.Context, req operate.IPBlockLogPageRequest) (operate.IPBlockLogPageResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []operate.IPBlockLog
	for _, l := range r.logs {
		if req.IP != "" && !strings.Contains(l.IP, req.IP) {
			continue
		}
		if req.CountryCode != "" && l.CountryCode != req.CountryCode {
			continue
		}
		matched = append(matched, l)
	}

	total := int64(len(matched))
	start := (req.PageNumber - 1) * req.PageSize
	if start < 0 {
		start = 0
	}
	if start > len(matched) {
		start = len(matched)
	}
	end := start + req.PageSize
	if end > len(matched) {
		end = len(matched)
	}

	return operate.IPBlockLogPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    matched[start:end],
	}, nil
}

func (r *fakeIPBlockLogRepository) Save(_ context.Context, log operate.IPBlockLog) (operate.IPBlockLog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.ID = int64(len(r.logs) + 1)
	if log.BlockedAt.IsZero() {
		log.BlockedAt = time.Now()
	}
	r.logs = append(r.logs, log)
	return log, nil
}

// fakeConfigRepository 模拟系统配置仓储
type fakeConfigRepository struct {
	mu      sync.RWMutex
	configs map[string]string
}

func (c *fakeConfigRepository) Get(_ context.Context, key string) (operate.ProxyConfigItem, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.configs[key]
	if !ok {
		return operate.ProxyConfigItem{}, operate.ErrConfigNotFound
	}
	return operate.ProxyConfigItem{
		Key:   key,
		Value: val,
	}, nil
}

func (c *fakeConfigRepository) Set(_ context.Context, key, value, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.configs[key] = value
	return nil
}
