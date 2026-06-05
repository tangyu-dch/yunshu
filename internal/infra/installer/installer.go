package installer

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	operatedomain "yunshu/internal/domain/operate"
	"yunshu/internal/infra/business"
	"yunshu/internal/infra/merchant"
	"yunshu/internal/infra/resource"
	"yunshu/internal/infra/security"
	"yunshu/internal/infra/system"
	"yunshu/internal/infra/telephony"
)

// SetupParams 定义一键初始化设置所需要的全部表单数据。
type SetupParams struct {
	MySQLHost         string `json:"mysqlHost"`
	MySQLPort         int    `json:"mysqlPort"`
	MySQLUser         string `json:"mysqlUser"`
	MySQLPassword     string `json:"mysqlPassword"`
	MySQLDatabase     string `json:"mysqlDatabase"`
	MySQLUseDocker    bool   `json:"mysqlUseDocker"`
	RedisHost         string `json:"redisHost"`
	RedisPort         int    `json:"redisPort"`
	RedisUseDocker    bool   `json:"redisUseDocker"`
	ExternalIP        string `json:"externalIp"`
	SipPort           int    `json:"sipPort"`
	WsPort            int    `json:"wsPort"`
	RtpStartPort      int    `json:"rtpStartPort"`
	RtpEndPort        int    `json:"rtpEndPort"`
	TenantMode        string `json:"tenantMode"`        // "single" | "multi"
	DefaultMerchantID int    `json:"defaultMerchantId"` // 默认 1001
}

// PortCheck 记录端口占用扫描的状态。
type PortCheck struct {
	Port     int    `json:"port"`
	Name     string `json:"name"`
	Occupied bool   `json:"occupied"`
}

// EnvStatus 汇总宿主机 Docker 运行时环境与端口预检状态。
type EnvStatus struct {
	Installed        bool        `json:"installed"`
	DockerInstalled  bool        `json:"dockerInstalled"`
	ComposeInstalled bool        `json:"composeInstalled"`
	DockerVersion    string      `json:"dockerVersion"`
	ComposeVersion   string      `json:"composeVersion"`
	Ports            []PortCheck `json:"ports"`
}

// DeployStatus 汇总后台 Docker Compose 容器拉起部署的状态及日志。
type DeployStatus struct {
	Status   string   `json:"status"` // "idle", "deploying", "success", "failed"
	Progress int      `json:"progress"`
	Logs     []string `json:"logs"`
	ErrorMsg string   `json:"errorMsg,omitempty"`
}

// Installer 管理系统一键引导安装的整体生命周期。
type Installer struct {
	mu             sync.Mutex
	status         string // "idle", "deploying", "success", "failed"
	logs           []string
	maxLogs        int
	logger         *slog.Logger
	ConfigFilePath string
	ComposePath    string
}

// NewInstaller 创建初始化安装器实例。
func NewInstaller(logger *slog.Logger) *Installer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Installer{
		status:         "idle",
		logs:           make([]string, 0),
		maxLogs:        1000,
		logger:         logger,
		ConfigFilePath: "configs/default.yaml",
		ComposePath:    "docker-compose.yml",
	}
}

// IsInstalled 检查系统当前是否已经完成初始化部署（configs/default.yaml 是否存在且非空）。
func (in *Installer) IsInstalled() bool {
	info, err := os.Stat(in.ConfigFilePath)
	if err != nil {
		return false
	}
	return info.Size() > 0
}

// Precheck 执行宿主机环境的全部预检项（Docker、Compose 版本及核心端口占用）。
func (in *Installer) Precheck(ctx context.Context) EnvStatus {
	status := EnvStatus{
		Installed: in.IsInstalled(),
	}

	// 1. 检测 Docker
	cmd := exec.CommandContext(ctx, "docker", "--version")
	if out, err := cmd.Output(); err == nil {
		status.DockerInstalled = true
		status.DockerVersion = strings.TrimSpace(string(out))
	}

	// 2. 检测 Docker Compose
	cmd = exec.CommandContext(ctx, "docker", "compose", "version")
	if out, err := cmd.Output(); err == nil {
		status.ComposeInstalled = true
		status.ComposeVersion = strings.TrimSpace(string(out))
	} else {
		// 备用检测旧版 docker-compose
		cmd = exec.CommandContext(ctx, "docker-compose", "--version")
		if out, err := cmd.Output(); err == nil {
			status.ComposeInstalled = true
			status.ComposeVersion = strings.TrimSpace(string(out))
		}
	}

	// 3. 扫描核心端口
	portsToCheck := []struct {
		port int
		name string
	}{
		{3306, "MySQL 数据库服务"},
		{63790, "Redis 缓存服务"},
		{2223, "RTPEngine 控制命令端口"},
		{5060, "Kamailio SIP 信令端口"},
		{5066, "Kamailio WebSocket 端口"},
		{5080, "FreeSWITCH 内网端口"},
		{8021, "FreeSWITCH ESL 控制接口"},
		{8080, "Yunshu 控制台主服务"},
	}

	status.Ports = make([]PortCheck, 0, len(portsToCheck))
	for _, pt := range portsToCheck {
		occupied := checkPortOccupied("127.0.0.1", pt.port)
		status.Ports = append(status.Ports, PortCheck{
			Port:     pt.port,
			Name:     pt.name,
			Occupied: occupied,
		})
	}

	return status
}

// GenerateConfigs 根据参数动态生成主配置文件 configs/default.yaml 与 docker-compose.yml。
func (in *Installer) GenerateConfigs(params SetupParams) error {
	in.logger.Info("开始动态生成 Yunshu 初始化配置文件", "tenantMode", params.TenantMode)

	// 确保 configs 目录存在
	dir := filepath.Dir(in.ConfigFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 1. 动态生成 default.yaml 配置结构
	mysqlDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		params.MySQLUser, params.MySQLPassword, params.MySQLHost, params.MySQLPort, params.MySQLDatabase)
	redisAddr := fmt.Sprintf("%s:%d", params.RedisHost, params.RedisPort)

	defaultYamlMap := map[string]any{
		"service": map[string]any{
			"name": "cc-call",
			"addr": ":8080",
		},
		"logging": map[string]any{
			"level":  "info",
			"format": "json",
		},
		"mysql": map[string]any{
			"dsn":             mysqlDSN,
			"maxIdleConns":    10,
			"maxOpenConns":    100,
			"connMaxLifetime": "1h",
		},
		"redis": map[string]any{
			"addrs":        []string{redisAddr},
			"db":           0,
			"readTimeout":  "3s",
			"writeTimeout": "3s",
			"stream": map[string]any{
				"enabled":        false,
				"stream":         "yunshu:events",
				"group":          "cc-call",
				"consumer":       "cc-call-local",
				"block":          "5s",
				"batchSize":      16,
				"claimMinIdle":   "1m",
				"startFromFirst": false,
			},
		},
		"rabbitmq": map[string]any{
			"url": "amqp://guest:guest@127.0.0.1:5672/",
		},
		"console": map[string]any{
			"callBaseURL": "",
		},
		"worker": map[string]any{
			"outbox": map[string]any{
				"interval":   "5s",
				"batchSize":  100,
				"retryDelay": "1m",
				"lease":      "30s",
				"workerId":   "cc-worker-local",
			},
			"callback": map[string]any{
				"url":     "",
				"secret":  "",
				"timeout": "5s",
			},
			"downstream": map[string]any{
				"url":     "",
				"secret":  "",
				"timeout": "5s",
			},
			"recording": map[string]any{
				"url":     "",
				"secret":  "",
				"timeout": "5s",
			},
			"billing": map[string]any{
				"defaultRatePerMin": 0,
			},
		},
		"freeswitch": map[string]any{
			"eventLeaseTTL":  "30s",
			"commandTimeout": "5s",
			"reconnect": map[string]any{
				"interval":    "5s",
				"maxAttempts": 30,
			},
			"nodes": []any{},
		},
		"tenant": map[string]any{
			"mode":              params.TenantMode,
			"defaultMerchantId": params.DefaultMerchantID,
		},
	}

	yamlData, err := yaml.Marshal(defaultYamlMap)
	if err != nil {
		return err
	}

	if err := os.WriteFile(in.ConfigFilePath, yamlData, 0644); err != nil {
		return err
	}
	in.logger.Info("成功写入初始化配置文件", "path", in.ConfigFilePath)

	// 2. 动态生成 docker-compose.yml 文件
	if err := in.writeComposeFile(params); err != nil {
		return err
	}
	in.logger.Info("成功写入 Docker-Compose 部署文件", "path", in.ComposePath)

	return nil
}

// StartDeployment 启动后台部署流程，异步拉起 Docker 容器。
func (in *Installer) StartDeployment() error {
	in.mu.Lock()
	if in.status == "deploying" {
		in.mu.Unlock()
		return errors.New("部署已在进行中")
	}
	in.status = "deploying"
	in.logs = make([]string, 0)
	in.mu.Unlock()

	in.appendLog(">>> 开始进行一键容器部署编排 ...")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		var cmd *exec.Cmd
		// 优先执行 docker compose
		cmd = exec.CommandContext(ctx, "docker", "compose", "up", "-d")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			in.markFailed(err.Error())
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			in.markFailed(err.Error())
			return
		}

		if err := cmd.Start(); err != nil {
			// 备用执行旧版 docker-compose
			in.appendLog(">>> 'docker compose' 不可用，尝试使用旧版 'docker-compose' ...")
			cmd = exec.CommandContext(ctx, "docker-compose", "up", "-d")
			stdout, err = cmd.StdoutPipe()
			if err != nil {
				in.markFailed(err.Error())
				return
			}
			stderr, err = cmd.StderrPipe()
			if err != nil {
				in.markFailed(err.Error())
				return
			}
			if err := cmd.Start(); err != nil {
				in.markFailed("无法拉起 Docker-Compose，请检查宿主机环境并确保障碍已排除: " + err.Error())
				return
			}
		}

		// 合并输出日志
		var wg sync.WaitGroup
		wg.Add(2)
		go in.readLogs(stdout, &wg)
		go in.readLogs(stderr, &wg)

		wg.Wait()
		if err := cmd.Wait(); err != nil {
			in.markFailed("部署指令执行失败: " + err.Error())
			return
		}

		in.mu.Lock()
		in.status = "success"
		in.appendLog(">>> 容器一键拉起部署完成！所有依赖容器运行中。")
		in.mu.Unlock()
	}()

	return nil
}

// DeployStatus 获取一键部署流程当前的状态和终端输出日志。
func (in *Installer) DeployStatus() DeployStatus {
	in.mu.Lock()
	defer in.mu.Unlock()

	logsCopy := make([]string, len(in.logs))
	copy(logsCopy, in.logs)

	progress := 0
	switch in.status {
	case "deploying":
		progress = 40
		if len(in.logs) > 5 {
			progress = 70
		}
	case "success":
		progress = 100
	case "failed":
		progress = 100
	}

	msg := ""
	if in.status == "failed" && len(in.logs) > 0 {
		msg = in.logs[len(in.logs)-1]
	}

	return DeployStatus{
		Status:   in.status,
		Progress: progress,
		Logs:     logsCopy,
		ErrorMsg: msg,
	}
}

func (in *Installer) readLogs(reader io.ReadCloser, wg *sync.WaitGroup) {
	defer wg.Done()
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		in.appendLog(scanner.Text())
	}
}

func (in *Installer) appendLog(line string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.logs = append(in.logs, line)
	if len(in.logs) > in.maxLogs {
		in.logs = in.logs[len(in.logs)-in.maxLogs:]
	}
}

func (in *Installer) markFailed(msg string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.status = "failed"
	in.appendLog(">>> [部署失败]: " + msg)
}

func checkPortOccupied(host string, port int) bool {
	timeout := 500 * time.Millisecond
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (in *Installer) writeComposeFile(params SetupParams) error {
	// 构建基本的 compose 文件内容
	mysqlPortMap := "3306:3306"
	if params.MySQLPort != 3306 {
		mysqlPortMap = fmt.Sprintf("%d:3306", params.MySQLPort)
	}

	// 动态端口映射范围定义
	rtpPorts := fmt.Sprintf("%d-%d:%d-%d/udp", params.RtpStartPort, params.RtpEndPort, params.RtpStartPort, params.RtpEndPort)

	composeTmpl := `services:
  # =========================================================================
  # 1. MySQL 数据库服务：存储呼叫配置、分机账号及网关节点
  # =========================================================================
  mysql:
    image: mysql:8.0
    container_name: cc-mysql
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: %s
      MYSQL_DATABASE: %s
    ports:
      - "%s"
    volumes:
      - ./docker/mysql/init.sql:/docker-entrypoint-initdb.d/init.sql
      - mysql_data:/var/lib/mysql
    networks:
      - callcenter_net
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-u", "root", "-p%s"]
      interval: 5s
      timeout: 5s
      retries: 10

  # =========================================================================
  # 2. Redis 缓存服务：存储坐席分机实时状态、选号资源锁与话务临时数据
  # =========================================================================
  redis:
    image: redis:7.0-alpine
    container_name: cc-redis
    restart: always
    ports:
      - "%d:6379"
    volumes:
      - redis_data:/data
    networks:
      - callcenter_net
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 10

  # =========================================================================
  # 3. RTPEngine 媒体代理服务：自适应 WebRTC 与 传统 SIP 音频转码桥接
  # =========================================================================
  rtpengine:
    image: jambonz/rtpengine:latest
    container_name: cc-rtpengine
    platform: linux/amd64
    restart: always
    environment:
      - RTP_START_PORT=%d
      - RTP_END_PORT=%d
      - LOGLEVEL=6
    command:
      - rtpengine
      - --listen-ng=2223
    ports:
      - "2223:2223/udp"
      - "%s"
    networks:
      - callcenter_net
    depends_on:
      - mysql
      - redis

  # =========================================================================
  # 4. FreeSWITCH 媒体节点服务：处理具体呼叫路由与 IVR 音频逻辑
  # =========================================================================
  freeswitch:
    image: bytedesk/freeswitch:latest
    container_name: cc-freeswitch
    restart: always
    ports:
      - "5080:5080/udp"
      - "8021:8021"
    environment:
      - ESL_PASSWORD=ClueCon
    networks:
      - callcenter_net
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_healthy

  # =========================================================================
  # 5. Kamailio 信令网关与注册服务：处理坐席注册、动态均衡分发与 NAT 穿透
  # =========================================================================
  kamailio:
    image: ghcr.io/kamailio/kamailio:6.1.2-bookworm
    container_name: cc-kamailio
    platform: linux/amd64
    restart: always
    ports:
      - "%d:5060/udp"
      - "%d:5060/tcp"
      - "%d:5066/tcp"
    volumes:
      - ./configs/kamailio/kamailio.cfg:/etc/kamailio/kamailio.cfg:ro
    entrypoint:
      - kamailio
      - -DD
      - -E
      - -f
      - /etc/kamailio/kamailio.cfg
      - --substdef
      - '!DEFAULT_MYSQL_ADDR!mysql://root:%s@mysql:3306/%s!g'
      - --substdef
      - '!DEFAULT_HTTP_ADDR!host.docker.internal:8082!g'
      - --substdef
      - '!DEFAULT_RTPENGINE_SOCK!udp:rtpengine:2223!g'
      - --substdef
      - '!MY_IP4_ADDR!0.0.0.0!g'
      - --substdef
      - '!DEFAULT_EXTERNAL_IP!%s!g'
    extra_hosts:
      - "host.docker.internal:host-gateway"
    networks:
      - callcenter_net
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_healthy
      rtpengine:
        condition: service_started
      freeswitch:
        condition: service_started

networks:
  callcenter_net:
    name: callcenter_net
    driver: bridge

volumes:
  mysql_data:
    name: cc_mysql_data
  redis_data:
    name: cc_redis_data
`

	content := fmt.Sprintf(composeTmpl,
		params.MySQLPassword,
		params.MySQLDatabase,
		mysqlPortMap,
		params.MySQLPassword,
		params.RedisPort,
		params.RtpStartPort,
		params.RtpEndPort,
		rtpPorts,
		params.SipPort,
		params.SipPort,
		params.WsPort,
		params.MySQLPassword,
		params.MySQLDatabase,
		params.ExternalIP,
	)

	return os.WriteFile(in.ComposePath, []byte(content), 0644)
}

// InitializeDatabase 动态创建数据库、执行表结构迁移并填充初始种子数据。
func (in *Installer) InitializeDatabase(ctx context.Context, params SetupParams) error {
	in.logger.Info("开始执行数据库创建与结构迁移", "host", params.MySQLHost, "port", params.MySQLPort, "database", params.MySQLDatabase)

	// 1. 先连接 MySQL 实例（不指定数据库名），如果数据库不存在则动态创建它
	dsnWithoutDB := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
		params.MySQLUser, params.MySQLPassword, params.MySQLHost, params.MySQLPort)

	var sqlDB *sql.DB
	var err error
	maxAttempts := 15
	for i := 1; i <= maxAttempts; i++ {
		sqlDB, err = sql.Open("mysql", dsnWithoutDB)
		if err == nil {
			err = sqlDB.PingContext(ctx)
		}
		if err == nil {
			break
		}
		in.logger.Warn("等待 MySQL 服务完全启动以进行数据库创建...", "attempt", i, "maxAttempts", maxAttempts, "error", err)
		if sqlDB != nil {
			sqlDB.Close()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if err != nil {
		return fmt.Errorf("连接 MySQL 服务器失败: %w", err)
	}
	defer sqlDB.Close()

	// 执行创建数据库的 SQL
	createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", params.MySQLDatabase)
	if _, err := sqlDB.ExecContext(ctx, createSQL); err != nil {
		in.logger.Error("动态创建数据库失败", "sql", createSQL, "error", err.Error())
		return fmt.Errorf("创建数据库失败: %w", err)
	}
	in.logger.Info("数据库创建/检查成功", "database", params.MySQLDatabase)

	// 2. 连接具体数据库，执行 GORM 自动迁移
	dsnWithDB := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		params.MySQLUser, params.MySQLPassword, params.MySQLHost, params.MySQLPort, params.MySQLDatabase)

	gormDB, err := gorm.Open(mysql.Open(dsnWithDB), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("打开 GORM 数据库连接失败: %w", err)
	}
	sqlConn, _ := gormDB.DB()
	defer sqlConn.Close()

	err = gormDB.AutoMigrate(
		&system.ProxyConfigModel{},
		&system.ConsoleAccountModel{},
		&system.ConsoleRoleModel{},
		&system.ConsolePermissionModel{},
		&system.ConsoleRolePermissionModel{},
		&system.ConsoleRoutePermissionModel{},
		&resource.MerchantModel{},
		&system.PhoneAttributionModel{},
		&resource.MerchantBillingOverviewModel{},
		&merchant.MerchantBillingRechargeModel{},
		&telephony.GatewayModel{},
		&telephony.ChannelModel{},
		&telephony.PoolModel{},
		&resource.PoolPhoneModel{},
		&resource.PoolPhoneSkillGroupModel{},
		&resource.SkillGroupModel{},
		&resource.UserSkillGroupModel{},
		&resource.MerchantUserModel{},
		&resource.ExtensionModel{},
		&resource.PhoneGroupModel{},
		&resource.PhoneGroupPoolPhoneRefModel{},
		&resource.PhoneGroupSkillGroupRefModel{},
		&business.AIModelFlowModel{},
		&security.BlacklistChannelModel{},
		&security.BlacklistModel{},
		&security.BlacklistGatewayModel{},
		&security.BlacklistDataModel{},
		&security.WhitelistDataModel{},
		&security.WhitelistDataMerchantModel{},
		&merchant.CallRateModel{},
		&merchant.CallRateMerchantModel{},
		&business.MerchantBatchCallTaskModel{},
		&business.MerchantBatchCallTaskListModel{},
		&telephony.RtpengineModel{},
		&security.RiskControlModel{},
		&security.RiskControlMerchantModel{},
		&system.AreaCodeModel{},
		&telephony.FreeswitchModel{},
		&telephony.FreeswitchEventLeaseModel{},
		&business.RecordModel{},
		&business.RecordingJobModel{},
		&business.ReportProjectionModel{},
		&business.PushJobModel{},
		&business.SettlementJobModel{},
		&business.MessageOutboxModel{},
		&resource.DialpadVersionModel{},
		&resource.DepartmentModel{},
	)
	if err != nil {
		in.logger.Error("GORM 数据库自动迁移失败", "error", err.Error())
		return fmt.Errorf("自动建表迁移失败: %w", err)
	}
	in.logger.Info("所有数据表结构迁移自动构建完成")

	// 3. 动态填充系统配置默认种子数据
	in.logger.Info("开始动态填充代理与核心参数、账号与配置种子数据...")
	proxyConfigRepo := system.NewProxyConfigRepository(gormDB, in.logger)
	if err := proxyConfigRepo.EnsureDefaults(ctx); err != nil {
		return fmt.Errorf("代理默认配置初始化失败: %w", err)
	}

	accountRepo := system.NewConsoleAccountRepository(gormDB, in.logger)
	if err := accountRepo.EnsureDefaults(ctx); err != nil {
		return fmt.Errorf("系统默认账号初始化失败: %w", err)
	}

	permissionRepo := system.NewPermissionRepository(gormDB, in.logger)
	if err := permissionRepo.EnsureDefaults(ctx); err != nil {
		return fmt.Errorf("系统默认角色和权限初始化失败: %w", err)
	}

	// 动态填充商户种子
	merchantRepo := merchant.NewMerchantRepository(gormDB, nil, in.logger)
	merchantID := params.DefaultMerchantID
	if merchantID <= 0 {
		merchantID = 1001
	}
	// 检查是否已存在
	var existingMerchant merchant.MerchantModel
	errMerchantExist := gormDB.Where("id = ?", merchantID).First(&existingMerchant).Error
	if errors.Is(errMerchantExist, gorm.ErrRecordNotFound) {
		appKey, appSecret := operatedomain.GenerateAppKeyPair()
		_, err = merchantRepo.Save(ctx, operatedomain.Merchant{
			ID:        merchantID,
			Name:      "本地默认商户",
			Account:   "merchant",
			Enable:    true,
			AppKey:    appKey,
			AppSecret: appSecret,
			SipDomain: "sip.yunshu.local",
			MaxAgents: 100,
		})
		if err != nil {
			return fmt.Errorf("保存默认商户种子失败: %w", err)
		}
	}

	// 动态填充全国省市行政区域种子数据
	var areaCount int64
	if err := gormDB.Model(&system.AreaCodeModel{}).Count(&areaCount).Error; err == nil && areaCount == 0 {
		in.logger.Info("检测到行政区域表数据为空，开始播种全国省市行政区划种子...")
		areaSeeds := GetAreaCodeSeeds()
		areaRepo := system.NewAreaCodeGormRepository(gormDB)
		if err := areaRepo.SaveBatch(ctx, areaSeeds); err != nil {
			return fmt.Errorf("播种行政区划种子失败: %w", err)
		}
		in.logger.Info("全国省市行政区划数据种子播种成功！", "records", len(areaSeeds))
	}

	// 动态填充默认的软交换（FreeSWITCH）节点种子数据
	var fsCount int64
	if err := gormDB.Model(&telephony.FreeswitchModel{}).Count(&fsCount).Error; err == nil && fsCount == 0 {
		in.logger.Info("检测到软交换节点表为空，开始自动播种默认软交换（FreeSWITCH）节点配置...")
		fsSeed := telephony.FreeswitchModel{
			ID:           1,
			Address:      "127.0.0.1",
			LocalAddress: "127.0.0.1",
			ESLPort:      8021,
			SIPPort:      5080,
			Password:     "ClueCon",
			SetID:        1,
			Weight:       100,
			RWeight:      100,
			CC:           1000,
			CmdPort:      8080,
			Enable:       true,
			DelFlag:      false,
			CreatedTime:  time.Now().UTC(),
			UpdatedTime:  time.Now().UTC(),
		}
		if err := gormDB.WithContext(ctx).Create(&fsSeed).Error; err != nil {
			return fmt.Errorf("播种默认软交换节点配置失败: %w", err)
		}
		in.logger.Info("默认软交换（FreeSWITCH）节点种子数据播种成功！", "address", fsSeed.Address, "eslPort", fsSeed.ESLPort)
	}

	// 动态填充默认的 RTPEngine 媒体节点种子数据
	var rtpCount int64
	if err := gormDB.Model(&telephony.RtpengineModel{}).Count(&rtpCount).Error; err == nil && rtpCount == 0 {
		in.logger.Info("检测到 RTPEngine 媒体节点表为空，开始自动播种默认媒体节点配置...")
		rtpSeed := telephony.RtpengineModel{
			ID:            1,
			SetID:         1,
			RtpengineSock: "udp:127.0.0.1:2223",
			Disabled:      false,
			Weight:        1,
			Description:   "本地默认媒体代理节点",
			DelFlag:       false,
			CreatedTime:   time.Now().UTC(),
			UpdatedTime:   time.Now().UTC(),
		}
		if err := gormDB.WithContext(ctx).Create(&rtpSeed).Error; err != nil {
			return fmt.Errorf("播种默认 RTPEngine 媒体节点失败: %w", err)
		}
		in.logger.Info("默认 RTPEngine 媒体节点种子数据播种成功！", "socket", rtpSeed.RtpengineSock, "setId", rtpSeed.SetID)
	}

	in.logger.Info("一键数据库迁移及种子数据填充全部圆满完成！")
	return nil
}

// -------------------------------------------------------------------------
// 辅助类型与逻辑 (模拟 GORM / SQL 基础行为，隔离复杂模块依赖)
// -------------------------------------------------------------------------

type gormWrapper struct {
	dbConn *exec.Cmd // 实际上可以通过普通 sql 连接执行 init.sql 或利用已有的 openRuntimeDB 逻辑
}

func openGormConnection(dsn string) (*gormWrapper, error) {
	// 简单的 TCP 握手作为数据库连通性预检测
	// 提取 DSN 中的 host 端口
	idx := strings.Index(dsn, "@tcp(")
	if idx < 0 {
		return nil, errors.New("invalid dsn")
	}
	tail := dsn[idx+5:]
	idxEnd := strings.Index(tail, ")")
	if idxEnd < 0 {
		return nil, errors.New("invalid dsn")
	}
	addr := tail[:idxEnd]
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return nil, err
	}
	conn.Close()
	return &gormWrapper{}, nil
}

func (w *gormWrapper) Close() {}

func (w *gormWrapper) AutoMigrate(logger *slog.Logger) error {
	// 由于我们的 main.go 和 call_runtime.go 在启动连接数据库时会自动执行 AutoMigrate，
	// 我们无需在此处重复实现繁琐的 GORM 反射，只需要让 Go 后端以最新的 config 初始化一次数据库即可。
	// 这里我们通过临时启动一次控制台或核心模块的内部迁移函数进行迁移，或直接返回成功由 main.go 启动时自动触发。
	return nil
}

func (w *gormWrapper) SeedData(defaultMerchantID int, logger *slog.Logger) error {
	// 默认种子会在 openRuntimeDB 中成功执行 seedDatabaseMerchant 进行填充。
	return nil
}

type installerConfig struct {
	MySQL struct {
		DSN string `yaml:"dsn"`
	} `yaml:"mysql"`
}

func loadConfigFromFile(path string) (installerConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return installerConfig{}, err
	}
	var cfg installerConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return installerConfig{}, err
	}
	return cfg, nil
}
