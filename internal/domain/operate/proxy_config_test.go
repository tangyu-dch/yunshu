package operate_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"yunshu/internal/domain/operate"
)

type fakeProxyConfigRepository struct {
	mu      sync.Mutex
	configs map[string]operate.ProxyConfigItem
}

func newFakeProxyConfigRepository() *fakeProxyConfigRepository {
	return &fakeProxyConfigRepository{configs: make(map[string]operate.ProxyConfigItem)}
}

func (r *fakeProxyConfigRepository) Get(_ context.Context, key string) (operate.ProxyConfigItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.configs[key]
	if !ok {
		return operate.ProxyConfigItem{}, errors.New("not found")
	}
	return item, nil
}

func (r *fakeProxyConfigRepository) Set(_ context.Context, key, value, description string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[key] = operate.ProxyConfigItem{
		Key:         key,
		Value:       value,
		Description: description,
		UpdatedTime: time.Now(),
	}
	return nil
}

func (r *fakeProxyConfigRepository) List(_ context.Context) ([]operate.ProxyConfigItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := make([]operate.ProxyConfigItem, 0, len(r.configs))
	for _, v := range r.configs {
		list = append(list, v)
	}
	return list, nil
}

func (r *fakeProxyConfigRepository) EnsureDefaults(_ context.Context) error {
	return nil
}

type fakeProxyConfigRtpengineReloadPort struct {
	calls int
	mu    sync.Mutex
}

func (f *fakeProxyConfigRtpengineReloadPort) ReloadRtpengine(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return nil
}

func TestProxyConfigManagementService_GetConfig(t *testing.T) {
	t.Parallel()

	repo := newFakeProxyConfigRepository()
	reloader := &fakeProxyConfigRtpengineReloadPort{}
	service := operate.NewProxyConfigManagementService(repo, reloader, nil, nil)

	ctx := context.Background()

	// 1. 测试没有配置时的默认值回退
	cfg, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KamailioSipPort != 5060 || cfg.RtpengineStartPort != 30000 || cfg.KamailioUdpIp != "0.0.0.0" {
		t.Fatalf("expected defaults, got %+v", cfg)
	}

	// 2. 播种部分配置，验证其正确覆盖
	repo.Set(ctx, operate.KeyKamailioSipPort, "5090", "")
	repo.Set(ctx, operate.KeyRtpengineStartPort, "40000", "")
	repo.Set(ctx, operate.KeyKamailioUdpIp, "192.168.1.100", "")

	cfg2, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.KamailioSipPort != 5090 || cfg2.RtpengineStartPort != 40000 || cfg2.KamailioUdpIp != "192.168.1.100" {
		t.Fatalf("expected overridden values, got %+v", cfg2)
	}
}

func TestProxyConfigManagementService_SaveConfig_Validation(t *testing.T) {
	t.Parallel()

	repo := newFakeProxyConfigRepository()
	reloader := &fakeProxyConfigRtpengineReloadPort{}
	service := operate.NewProxyConfigManagementService(repo, reloader, nil, nil)

	ctx := context.Background()

	// 1. 无效的 RTP 端口范围：结束端口小于起始端口
	err := service.SaveConfig(ctx, operate.ProxyConfig{
		KamailioUdpIp:      "0.0.0.0",
		KamailioTcpIp:      "0.0.0.0",
		KamailioSipPort:    5060,
		KamailioWsPort:     5066,
		KamailioExternalIp: "127.0.0.1",
		RtpengineStartPort: 40000,
		RtpengineEndPort:   30000, // Invalid
	})
	if err == nil || !strings.Contains(err.Error(), "RTP 端口范围") {
		t.Fatalf("expected RTP port range invalid error, got %v", err)
	}

	// 2. 无效的 Kamailio 端口（小于等于 0）
	err = service.SaveConfig(ctx, operate.ProxyConfig{
		KamailioUdpIp:      "0.0.0.0",
		KamailioTcpIp:      "0.0.0.0",
		KamailioSipPort:    0, // Invalid
		KamailioWsPort:     5066,
		KamailioExternalIp: "127.0.0.1",
		RtpengineStartPort: 30000,
		RtpengineEndPort:   30100,
	})
	if err == nil || !strings.Contains(err.Error(), "Kamailio 端口号") {
		t.Fatalf("expected Kamailio port invalid error, got %v", err)
	}

	// 3. 正常保存配置，验证数据库是否写入成功
	err = service.SaveConfig(ctx, operate.ProxyConfig{
		KamailioUdpIp:       "1.1.1.1",
		KamailioTcpIp:       "2.2.2.2",
		KamailioSipPort:     5070,
		KamailioWsPort:      5077,
		KamailioExternalIp:  "3.3.3.3",
		RtpengineInternalIp: "4.4.4.4",
		RtpengineSdpIp:      "5.5.5.5",
		RtpengineStartPort:  32000,
		RtpengineEndPort:    32500,
	})
	if err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	// 验证值在数据库中正确保存
	item, _ := repo.Get(ctx, operate.KeyKamailioExternalIp)
	if item.Value != "3.3.3.3" {
		t.Fatalf("expected 3.3.3.3, got %s", item.Value)
	}

	item2, _ := repo.Get(ctx, operate.KeyRtpengineStartPort)
	if item2.Value != "32000" {
		t.Fatalf("expected 32000, got %s", item2.Value)
	}
}

func TestProxyConfigManagementService_ApplyAndRestart(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	mockYamlPath := filepath.Join(tempDir, "default.yaml")
	mockComposePath := filepath.Join(tempDir, "docker-compose.yml")

	// 写入初始的 mock default.yaml
	initialYaml := `
mysql:
  dsn: "root:Password123!@tcp(mysql:3306)/yunshu?charset=utf8mb4"
`
	if err := os.WriteFile(mockYamlPath, []byte(initialYaml), 0644); err != nil {
		t.Fatal(err)
	}

	// 写入初始的 mock docker-compose.yml
	initialCompose := `services:
  rtpengine:
    environment:
      - RTP_START_PORT=30000
      - RTP_END_PORT=30100
    ports:
      - "30000-30100:30000-30100/udp"
  kamailio:
    ports:
      - "5060:5060/udp"
      - "5060:5060/tcp"
      - "5066:5066/tcp"
    entrypoint:
      - --substdef
      - '!DEFAULT_EXTERNAL_IP!127.0.0.1!g'`
	if err := os.WriteFile(mockComposePath, []byte(initialCompose), 0644); err != nil {
		t.Fatal(err)
	}

	repo := newFakeProxyConfigRepository()
	ctx := context.Background()

	// 保存需要写入的系统配置
	_ = repo.Set(ctx, operate.KeyKamailioUdpIp, "0.0.0.0", "")
	_ = repo.Set(ctx, operate.KeyKamailioTcpIp, "0.0.0.0", "")
	_ = repo.Set(ctx, operate.KeyKamailioSipPort, "5080", "")
	_ = repo.Set(ctx, operate.KeyKamailioWsPort, "5088", "")
	_ = repo.Set(ctx, operate.KeyKamailioExternalIp, "198.51.100.10", "")
	_ = repo.Set(ctx, operate.KeyRtpengineInternalIp, "0.0.0.0", "")
	_ = repo.Set(ctx, operate.KeyRtpengineSdpIp, "198.51.100.10", "")
	_ = repo.Set(ctx, operate.KeyRtpengineStartPort, "35000", "")
	_ = repo.Set(ctx, operate.KeyRtpengineEndPort, "35200", "")

	reloader := &fakeProxyConfigRtpengineReloadPort{}
	service := operate.NewProxyConfigManagementService(repo, reloader, nil, nil)
	service.ConfigFilePath = mockYamlPath
	service.ComposePath = mockComposePath

	// 执行配置应用和重写
	err := service.ApplyAndRestart(ctx)
	if err != nil {
		t.Fatalf("ApplyAndRestart failed: %v", err)
	}

	// 1. 验证 default.yaml 是否可解析且结构正确
	yamlBytes, err := os.ReadFile(mockYamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(yamlBytes), "dsn:") {
		t.Fatalf("expected yaml to contain dsn, got: %s", string(yamlBytes))
	}

	// 2. 验证 docker-compose.yml 里的端口映射及 IP 变量是否被正确改写
	composeBytes, err := os.ReadFile(mockComposePath)
	if err != nil {
		t.Fatal(err)
	}
	composeStr := string(composeBytes)

	if !strings.Contains(composeStr, "RTP_START_PORT=35000") {
		t.Errorf("expected RTP_START_PORT=35000, got: %s", composeStr)
	}
	if !strings.Contains(composeStr, "RTP_END_PORT=35200") {
		t.Errorf("expected RTP_END_PORT=35200, got: %s", composeStr)
	}
	if !strings.Contains(composeStr, `- "35000-35200:35000-35200/udp"`) {
		t.Errorf("expected RTP port mapping replaced, got: %s", composeStr)
	}
	if !strings.Contains(composeStr, `- "5080:5060/udp"`) {
		t.Errorf("expected Kamailio UDP port mapping replaced, got: %s", composeStr)
	}
	if !strings.Contains(composeStr, `- "5080:5060/tcp"`) {
		t.Errorf("expected Kamailio TCP port mapping replaced, got: %s", composeStr)
	}
	if !strings.Contains(composeStr, `- "5088:5066/tcp"`) {
		t.Errorf("expected Kamailio WebSocket port mapping replaced, got: %s", composeStr)
	}
	if !strings.Contains(composeStr, "DEFAULT_EXTERNAL_IP!198.51.100.10!g") {
		t.Errorf("expected DEFAULT_EXTERNAL_IP substdef replaced, got: %s", composeStr)
	}

	// 验证即使文件不存在或为只读时的容错
	service.ConfigFilePath = filepath.Join(tempDir, "non_existent_file.yaml")
	service.ComposePath = filepath.Join(tempDir, "non_existent_compose.yml")
	err = service.ApplyAndRestart(ctx)
	if err != nil {
		t.Fatalf("expected no-op success on non-existent configs, got: %v", err)
	}
}

func TestProxyConfigManagementService_ApplyAndRestart_PermissionErr(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	mockYamlPath := filepath.Join(tempDir, "readonly.yaml")

	if err := os.WriteFile(mockYamlPath, []byte("mysql: dsn"), 0400); err != nil {
		t.Fatal(err)
	}

	repo := newFakeProxyConfigRepository()
	ctx := context.Background()
	_ = repo.Set(ctx, operate.KeyKamailioSipPort, "5080", "")

	reloader := &fakeProxyConfigRtpengineReloadPort{}
	service := operate.NewProxyConfigManagementService(repo, reloader, nil, nil)
	service.ConfigFilePath = mockYamlPath
	service.ComposePath = filepath.Join(tempDir, "non_existent.yml")

	// 尝试向只读文件写入，预期应该触发写入权限错误
	err := service.ApplyAndRestart(ctx)
	if err == nil {
		t.Fatal("expected write error on readonly file, got nil")
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("expected path error, got %T: %v", err, err)
	}
}
