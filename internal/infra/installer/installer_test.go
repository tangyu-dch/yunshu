package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"yunshu/internal/infra/security"
	"yunshu/internal/infra/system"
)

func TestInstaller_IsInstalled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "default.yaml")

	inst := NewInstaller(nil)
	inst.ConfigFilePath = configPath

	// 1. 配置文件不存在
	if inst.IsInstalled() {
		t.Fatal("expected IsInstalled to be false when file does not exist")
	}

	// 2. 配置文件存在但为空
	err := os.WriteFile(configPath, []byte(""), 0644)
	if err != nil {
		t.Fatal(err)
	}
	if inst.IsInstalled() {
		t.Fatal("expected IsInstalled to be false when file is empty")
	}

	// 3. 配置文件存在且有内容
	err = os.WriteFile(configPath, []byte("service:\n  name: cc-call"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	if !inst.IsInstalled() {
		t.Fatal("expected IsInstalled to be true when file has content")
	}
}

func TestInstaller_GenerateConfigs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "default.yaml")
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	inst := NewInstaller(nil)
	inst.ConfigFilePath = configPath
	inst.ComposePath = composePath

	params := SetupParams{
		MySQLHost:         "127.0.0.1",
		MySQLPort:         3306,
		MySQLUser:         "root",
		MySQLPassword:     "root123",
		MySQLDatabase:     "yunshu_test",
		MySQLUseDocker:    true,
		RedisHost:         "127.0.0.1",
		RedisPort:         6379,
		RedisUseDocker:    true,
		ExternalIP:        "1.2.3.4",
		SipPort:           5060,
		WsPort:            5066,
		RtpStartPort:      30000,
		RtpEndPort:        30100,
		TenantMode:        "single",
		DefaultMerchantID: 1001,
	}

	err := inst.GenerateConfigs(params)
	if err != nil {
		t.Fatalf("failed to generate configs: %v", err)
	}

	// 1. 验证 default.yaml 生成
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("expected default.yaml to be created")
	}
	yamlData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	yamlStr := string(yamlData)
	if !strings.Contains(yamlStr, "dsn: root:root123@tcp(127.0.0.1:3306)/yunshu_test") {
		t.Errorf("yaml missing dsn, got: %s", yamlStr)
	}
	if !strings.Contains(yamlStr, "127.0.0.1:6379") {
		t.Errorf("yaml missing redis address, got: %s", yamlStr)
	}

	// 2. 验证 docker-compose.yml 生成
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		t.Fatal("expected docker-compose.yml to be created")
	}
	composeData, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	composeStr := string(composeData)
	if !strings.Contains(composeStr, "MYSQL_ROOT_PASSWORD: root123") {
		t.Errorf("compose missing mysql password, got: %s", composeStr)
	}
	if !strings.Contains(composeStr, "MYSQL_DATABASE: yunshu_test") {
		t.Errorf("compose missing mysql database name, got: %s", composeStr)
	}
	if !strings.Contains(composeStr, "30000-30100:30000-30100/udp") {
		t.Errorf("compose missing rtp ports range, got: %s", composeStr)
	}
	if !strings.Contains(composeStr, "!DEFAULT_EXTERNAL_IP!1.2.3.4!g") {
		t.Errorf("compose missing external ip substdef mapping, got: %s", composeStr)
	}
}

func TestInstaller_Precheck(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "default.yaml")

	inst := NewInstaller(nil)
	inst.ConfigFilePath = configPath

	status := inst.Precheck(context.Background())
	// 无论 docker 是否安装，都应该能无损返回状态而不崩溃
	if status.Installed {
		t.Fatal("expected Installed status to be false initially")
	}
	if len(status.Ports) == 0 {
		t.Fatal("expected ports status check to be populated")
	}
}

// 模拟验证 InitializeDatabase 在 Repository 层的数据播种机制
func TestInstaller_InitializeDatabaseSeeds(t *testing.T) {
	t.Parallel()

	// 我们直接利用 directory 包现有的 openPermissionTestDB 逻辑，通过本地 SQLite 内存表来验证
	// 我们的默认种子数据是否能成功插入。
	db := openTestDB(t)

	ctx := context.Background()

	// 1. 代理默认配置播种验证
	proxyConfigRepo := system.NewProxyConfigRepository(db, nil)
	if err := proxyConfigRepo.EnsureDefaults(ctx); err != nil {
		t.Fatalf("failed to seed proxy configs: %v", err)
	}

	var configCount int64
	if err := db.Model(&system.ProxyConfigModel{}).Count(&configCount).Error; err != nil {
		t.Fatal(err)
	}
	if configCount == 0 {
		t.Fatal("expected proxy configs to be seeded")
	}

	// 2. 系统默认账号播种验证
	accountRepo := system.NewConsoleAccountRepository(db, nil)
	if err := accountRepo.EnsureDefaults(ctx); err != nil {
		t.Fatalf("failed to seed console accounts: %v", err)
	}

	var accountCount int64
	if err := db.Model(&system.ConsoleAccountModel{}).Count(&accountCount).Error; err != nil {
		t.Fatal(err)
	}
	if accountCount == 0 {
		t.Fatal("expected console accounts to be seeded")
	}

	// 3. 系统默认角色和权限播种验证
	permissionRepo := system.NewPermissionRepository(db, nil)
	if err := permissionRepo.EnsureDefaults(ctx); err != nil {
		t.Fatalf("failed to seed permissions: %v", err)
	}

	var permissionCount int64
	if err := db.Model(&system.ConsolePermissionModel{}).Count(&permissionCount).Error; err != nil {
		t.Fatal(err)
	}
	if permissionCount == 0 {
		t.Fatal("expected permissions to be seeded")
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	err = db.AutoMigrate(
		&system.ProxyConfigModel{},
		&system.ConsoleAccountModel{},
		&system.ConsoleRoleModel{},
		&system.ConsolePermissionModel{},
		&system.ConsoleRolePermissionModel{},
		&system.ConsoleRoutePermissionModel{},
		&security.RiskControlModel{},
		&security.RiskControlMerchantModel{},
		&system.AreaCodeModel{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
