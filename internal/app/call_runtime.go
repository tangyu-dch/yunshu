package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/callflow"
	"yunshu/internal/domain/cti"
	"yunshu/internal/domain/esl"
	"yunshu/internal/domain/operate"
	"yunshu/internal/infra/business"
	"yunshu/internal/infra/config"
	"yunshu/internal/infra/db"
	"yunshu/internal/infra/events"
	"yunshu/internal/infra/extensionstatus"
	"yunshu/internal/infra/fsesl"
	infrahttp "yunshu/internal/infra/http"
	"yunshu/internal/infra/kamailioauth"
	"yunshu/internal/infra/merchant"
	redisinfra "yunshu/internal/infra/redis"
	"yunshu/internal/infra/resource"
	"yunshu/internal/infra/security"
	selectioninfra "yunshu/internal/infra/selection"
	"yunshu/internal/infra/storage"
	"yunshu/internal/infra/system"
	"yunshu/internal/infra/telephony"
	wsinfra "yunshu/internal/infra/websocket"
	"yunshu/pkg/idempotency"
	"yunshu/pkg/workflow"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	dbMigrationOnce sync.Once
	dbMigrationErr  error
)

// CallRuntime 聚合 cc-call 进程内的 CTI 和 ESL 业务服务。
// 先以内存方式打通主链路，后续替换为 Redis/DB/FS/RabbitMQ adapter 时保持 transport 不变。
type CallRuntime struct {
	APICall        *cti.APICallService
	BatchScheduler *cti.BatchSchedulerService
	Originate      *esl.OriginateService
	Command        *esl.CommandService
	Session        *esl.SessionService
	GatewaySync    *esl.GatewayConfigService
	Events         events.Bus
	CTIFlow        *workflow.Runner
	ESLFlow        *workflow.Runner
	Executor       esl.CommandExecutor
	FSPool         *fsesl.ConnectionPool
	FSNodes        telephony.Registry
	DB             *gorm.DB
	Selector       *cti.RuntimeSelector
	Candidates     cti.CandidateSource
	Marker         cti.CandidateMarker
	WSHub          *wsinfra.Hub
	WSHubCancel    context.CancelFunc
	ASRServer      *wsinfra.ASRServer
	CallControl    *cti.CallControlService
}

// NewCallRuntime 创建 cc-call 运行时依赖。
func NewCallRuntime(logger *slog.Logger) (*CallRuntime, error) {
	return NewCallRuntimeWithConfig(context.Background(), config.Config{}, nil, logger)
}

// NewCallRuntimeWithEventBus 创建可注入事件总线的 cc-call 运行时。
// 生产环境注入 Redis Stream bus，本地开发和单元测试默认使用内存 bus。
func NewCallRuntimeWithEventBus(bus events.Bus, logger *slog.Logger) (*CallRuntime, error) {
	return NewCallRuntimeWithConfig(context.Background(), config.Config{}, bus, logger)
}

// NewCallRuntimeWithConfig 按配置创建 cc-call 运行时。
// 有 FS 节点配置时使用真实 ESL 连接池；没有节点时使用内存执行器便于本地开发。
func NewCallRuntimeWithConfig(ctx context.Context, cfg config.Config, bus events.Bus, logger *slog.Logger) (*CallRuntime, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// 注入基础设施层的 HTTP 客户端与 TTS 缓存存储实现，解耦 domain 依赖
	callflow.SetHTTPClient(infrahttp.NewDefaultHTTPClient(15 * time.Second))
	callflow.SetTTSCacheStore(storage.NewLocalTTSCacheStore(""))
	if bus == nil {
		bus = events.NewMemoryBus(logger)
	}
	ctiEngine, err := workflow.NewEngine(cti.WorkflowDefinitions()...)
	if err != nil {
		logger.Error("CTI 工作流引擎初始化失败", "error", err.Error())
		return nil, fmt.Errorf("cti workflow engine init: %w", err)
	}
	eslEngine, err := workflow.NewEngine(esl.WorkflowDefinitions()...)
	if err != nil {
		logger.Error("ESL 工作流引擎初始化失败", "error", err.Error())
		return nil, fmt.Errorf("esl workflow engine init: %w", err)
	}
	ctiRunner := workflow.NewRunner(ctiEngine, workflow.NewMemoryInstanceStore(), logger)
	eslRunner := workflow.NewRunner(eslEngine, workflow.NewMemoryInstanceStore(), logger)

	gormDB, err := openRuntimeDB(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("open runtime db: %w", err)
	}
	redisClient := redisinfra.NewClient(cfg.Redis)
	var wsHub *wsinfra.Hub
	var wsCancel context.CancelFunc
	if len(cfg.Redis.Addrs) > 0 {
		wsHub = wsinfra.NewHub(redisClient, logger)
		var wsCtx context.Context
		wsCtx, wsCancel = context.WithCancel(ctx)
		wsHub.Start(wsCtx)
		logger.Info("CTI WebSocket Hub 已启用", "topic", wsinfra.PushTopic)
	} else {
		logger.Warn("未配置 Redis 地址，CTI WebSocket Hub 未启用", "impact", "无法跨实例推送批量投影刷新")
	}
	nodeRegistry := buildFSRegistry(cfg, gormDB, logger)
	extensionResolver := buildExtensionResolver(gormDB, logger)
	outboundGuard := buildOutboundGuard(gormDB, extensionstatus.NewRedisReader(redisClient), logger)
	nodes, err := nodeRegistry.ListEnabled(context.Background())
	if err != nil {
		logger.Error("读取 FreeSWITCH 节点配置失败", "error", err.Error())
		if wsCancel != nil {
			wsCancel()
		}
		return nil, fmt.Errorf("list FreeSWITCH nodes: %w", err)
	}
	fsNodes := fsNodeConfigs(nodes)
	executor := esl.CommandExecutor(&esl.MemoryCommandExecutor{Logger: logger})
	reliableOutbox := buildOutboxStore(gormDB, logger)
	session := esl.NewSessionService(esl.NewMemorySessionStore(), reliableOutbox, logger)
	session.Events = bus
	callflow.RegisterProjectionConsumers(bus, reliableOutbox, logger)
	var pool *fsesl.ConnectionPool
	if len(fsNodes) > 0 {
		pool = fsesl.NewConnectionPool(ctx, fsNodes, cfg.FreeSwitch.Reconnect.Interval, cfg.FreeSwitch.Reconnect.MaxAttempts, logger)
		pool.LeaseRegistry = nodeRegistry
		pool.LeaseOwner = buildFSLeaseOwner(cfg.Service.Name)
		pool.LeaseTTL = cfg.FreeSwitch.EventLeaseTTL
		pool.OnEvent = func(ctx context.Context, event contracts.TelephonyEvent) {
			if _, err := session.ApplyEvent(ctx, event); err != nil {
				logger.Warn("FreeSWITCH 事件写入会话失败", "eventId", event.EventID, "eventName", event.EventName, "callId", event.CallID, "uuid", event.UUID, "fsAddr", event.FSAddr, "error", err.Error())
			}
		}
		pool.OnSofiaEvent = func(ctx context.Context, subclass string, extension string) {
			if redisClient == nil {
				return
			}
			status := "-1" // 默认注销/到期 = Offline (-1)
			if subclass == "sofia::register" {
				status = "1" // 注册成功 = Idle (1)
			}
			err := redisClient.HSet(ctx, contracts.KeyExtensionStatus, extension, status).Err()
			if err != nil {
				logger.Error("实时同步分机注册状态到 Redis 失败", "extension", extension, "subclass", subclass, "status", status, "error", err.Error())
			} else {
				logger.Info("已实时同步分机注册状态到 Redis", "extension", extension, "subclass", subclass, "status", status)
			}
		}
		executor = &fsesl.ESLCommandExecutor{Pool: pool, Timeout: cfg.FreeSwitch.CommandTimeout, Logger: logger}
		logger.Info("cc-call 已启用真实 FreeSWITCH ESL 执行器", "nodeCount", len(fsNodes))
		go startFSNodeLeaseClaimer(ctx, pool, nodeRegistry, logger)
	} else {
		logger.Warn("cc-call 未配置 FreeSWITCH 节点，使用内存 ESL 执行器", "impact", "不会向真实 FreeSWITCH 发送命令")
	}
	command := esl.NewCommandService(idempotency.NewMemoryStore(), executor, logger)
	candidateSource := buildCandidateSource(gormDB, redisClient, logger)
	candidateMarker := buildCandidateMarker(gormDB, logger)
	runtimeSelector := &cti.RuntimeSelector{
		RuleSelector: cti.Selector{},
		Allocator:    selectioninfra.NewRedisAllocator(redisClient, 30*time.Minute),
		Marker:       candidateMarker,
		Logger:       logger,
	}
	gatewayRepository := buildGatewayRepository(gormDB, logger)
	gatewaySync := &esl.GatewayConfigService{Gateways: gatewayRepository, Nodes: registryGatewaySyncNodeLister{registry: nodeRegistry}, Executor: &fsesl.GatewayHTTPExecutor{Timeout: cfg.FreeSwitch.CommandTimeout, Logger: logger}, Logger: logger}
	var licenseService *operate.LicenseService
	if gormDB != nil {
		proxyConfigRepo := system.NewProxyConfigRepository(gormDB, logger)
		licenseService = operate.NewLicenseService(proxyConfigRepo, cfg.Console.LicensePath, logger)
		_, _ = licenseService.LoadAndVerify(context.Background())
	} else {
		licenseService = operate.NewLicenseService(nil, cfg.Console.LicensePath, logger)
		_, _ = licenseService.LoadAndVerify(context.Background())
	}

	originate := &esl.OriginateService{
		CommandService: command,
		SessionService: session,
		NodeSelector:   registryNodeSelector{registry: nodeRegistry},
		Extensions:     extensionResolver,
		Guard:          outboundGuard,
		Events:         bus,
		Limiter:        licenseService,
		Logger:         logger,
	}
	apiCall := &cti.APICallService{ESL: inProcessESLClient{originate: originate}, Events: bus, Logger: logger}
	var batchRepo cti.BatchTaskRepository
	if gormDB != nil {
		batchRepo = business.NewBatchRepository(gormDB, extensionstatus.NewRedisReader(redisClient), logger)
	}
	var callQueue cti.CallQueue
	if redisClient != nil {
		callQueue = selectioninfra.NewRedisCallQueue(redisClient)
	}

	callflow.RegisterConsumers(ctx, bus, ctiRunner, eslRunner, session, originate, runtimeSelector, candidateSource, extensionstatus.NewRedisReader(redisClient), batchRepo, callQueue, logger)
	var batchScheduler *cti.BatchSchedulerService
	if gormDB != nil {
		batchScheduler = &cti.BatchSchedulerService{
			Repository: batchRepo,
			Queue:      callQueue,
			ESL:        inProcessESLClient{originate: originate},
			Events:     bus,
			Logger:     logger,
		}
		callflow.RegisterBatchConsumers(bus, ctiRunner, batchScheduler, logger)
		logger.Info("批量外呼调度器已启用", "tables", "merchant_batch_call_task,merchant_batch_call_task_list")
	} else {
		logger.Warn("未配置 MySQL DSN，批量外呼调度器未启用", "impact", "生产环境必须配置批量任务表仓储")
	}

	var asrServer *wsinfra.ASRServer
	if !runningUnderGoTest() {
		asrServer = wsinfra.NewASRServer(":9002", bus, session.Store, logger)
		if err := asrServer.Start(ctx); err != nil {
			logger.Warn("云枢 ASR 旁路推流 WebSocket 服务启动失败，端口可能已被占用", "error", err.Error())
		} else {
			logger.Info("云枢 ASR 旁路推流 WebSocket 服务已在端口 9002 启动")
		}
	}

	if gormDB != nil && redisClient != nil {
		go startOfflineExtensionUnbinder(ctx, gormDB, redisClient, logger)
	}

	callControl := cti.NewCallControlService(session.Store, command, extensionResolver, logger)

	return &CallRuntime{APICall: apiCall, BatchScheduler: batchScheduler, Originate: originate, Command: command, Session: session, GatewaySync: gatewaySync, Events: bus, CTIFlow: ctiRunner, ESLFlow: eslRunner, Executor: executor, FSPool: pool, FSNodes: nodeRegistry, DB: gormDB, Selector: runtimeSelector, Candidates: candidateSource, Marker: candidateMarker, WSHub: wsHub, WSHubCancel: wsCancel, ASRServer: asrServer, CallControl: callControl}, nil
}

// openRuntimeDB 打开数据库连接。
// 必须配置并成功连接 MySQL，否则返回 error。彻底移除了 SQLite 开发库和 Fallback 兜底逻辑。
func openRuntimeDB(cfg config.Config, logger *slog.Logger) (*gorm.DB, error) {
	if runningUnderGoTest() && cfg.MySQL.DSN == "" {
		logger.Info("测试环境下未配置 MySQL DSN，跳过数据库连接")
		return nil, nil
	}
	if cfg.MySQL.DSN == "" {
		logger.Error("数据库配置错误：MySQL DSN 不能为空！彻底移除了 SQLite 内存兜底。")
		return nil, fmt.Errorf("MySQL DSN is empty")
	}
	gormDB, err := db.OpenMySQL(db.Config{
		DSN:             cfg.MySQL.DSN,
		MaxIdleConns:    cfg.MySQL.MaxIdleConns,
		MaxOpenConns:    cfg.MySQL.MaxOpenConns,
		ConnMaxLifetime: cfg.MySQL.ConnMaxLifetime,
	})
	if err != nil {
		logger.Error("无法连接 MySQL 数据库，程序直接退出（已移除 SQLite 内存兜底）", "error", err.Error())
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	logger.Info("MySQL 数据库连接已建立")

	// 执行自动迁移
	dbMigrationOnce.Do(func() {
		dbMigrationErr = gormDB.AutoMigrate(
			&system.ProxyConfigModel{},
			&system.ConsoleAccountModel{},
			&system.ConsoleRoleModel{},
			&system.ConsolePermissionModel{},
			&system.ConsoleRolePermissionModel{},
			&system.ConsoleRoutePermissionModel{},
			&resource.MerchantModel{},
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
			&business.AIModelConfigModel{},
			&security.BlacklistModel{},
			&security.BlacklistGatewayModel{},
			&security.BlacklistDataModel{},
			&security.BlacklistChannelModel{},
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
			&system.PhoneAttributionModel{},
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
			&system.IPBlockLogModel{},
		)
	})
	if dbMigrationErr != nil {
		logger.Error("MySQL 数据库自动迁移失败", "error", dbMigrationErr.Error())
		return nil, fmt.Errorf("auto migrate: %w", dbMigrationErr)
	} else {
		logger.Info("MySQL 数据库自动迁移处理完成")
	}
	return gormDB, nil
}

func runningUnderGoTest() bool {
	return strings.HasSuffix(filepath.Base(os.Args[0]), ".test")
}

// buildFSRegistry 构建 FreeSWITCH 节点注册表。
// 优先使用 GORM 从数据库 table=freeswitch 读取节点配置；
// 若无数据库则使用内存注册表，仅包含 YAML 中定义的节点（仅用于本地开发）。
func buildFSRegistry(cfg config.Config, gormDB *gorm.DB, logger *slog.Logger) telephony.Registry {
	if gormDB != nil {
		logger.Info("FreeSWITCH 节点配置将从数据库读取", "table", "freeswitch")
		return telephony.NewGormRegistry(gormDB)
	}
	registry := telephony.NewMemoryRegistry()
	for _, node := range cfg.FreeSwitch.Nodes {
		_ = registry.Upsert(context.Background(), telephony.Node{
			ID:       node.ID,
			FSAddr:   node.Addr,
			Password: node.Password,
			SetID:    node.SetID,
			Weight:   node.Weight,
			CmdPort:  node.CmdPort,
			Enable:   node.Enabled,
			Status:   telephony.NodeActive,
		})
	}
	logger.Warn("未配置 MySQL DSN，FreeSWITCH 节点使用本地内存兜底", "nodeCount", len(cfg.FreeSwitch.Nodes))
	return registry
}

// buildExtensionResolver 构建分机解析器。
// 优先使用数据库 table=extension 解析 API 外呼的分机号；
// 若无数据库则返回 nil，OriginateService 会从请求 extra 中兜底解析。
func buildExtensionResolver(gormDB *gorm.DB, logger *slog.Logger) esl.ExtensionResolver {
	if gormDB == nil {
		logger.Warn("未配置 MySQL DSN，API 外呼分机将仅从请求 extra 兜底解析", "impact", "生产环境必须配置 extension 表仓储")
		return nil
	}
	logger.Info("API 外呼分机配置将从数据库读取", "table", "extension")
	return resource.NewExtensionRepository(gormDB, logger)
}

// buildOutboundGuard 构建 API 外呼出站校验器。
// 优先使用数据库 tables=merchant_user,merchant,merchant_billing_overview 和 Redis 缓存校验：
// - 商户用户存在且状态正常
// - 商户状态正常且未过期
// - 预付费余额充足
// statuses 参数提供 Redis 分机状态缓存加速校验。若无数据库则返回 nil，OriginateService 仅执行基础请求校验。
func buildOutboundGuard(gormDB *gorm.DB, statuses esl.ExtensionStatusReader, logger *slog.Logger) esl.OutboundGuard {
	if gormDB == nil {
		logger.Warn("未配置 MySQL DSN，ESL API 外呼兜底校验仅执行请求基础校验", "impact", "生产环境必须配置用户、商户和账单仓储")
		return nil
	}
	logger.Info("ESL API 外呼兜底校验将从数据库和 Redis 读取", "tables", "merchant_user,merchant,merchant_billing_overview", "redisKey", contracts.KeyExtensionStatus)
	return resource.NewOutboundGuard(gormDB, statuses, logger)
}

// buildGatewayRepository 构建网关配置仓储。
// cc-call 的 `/esl/gateway` 同步入口需要读取  `gateway` 表确认网关存在；
// 没有数据库时使用空内存仓储，仅支持本地测试，生产必须配置 MySQL。
func buildGatewayRepository(gormDB *gorm.DB, logger *slog.Logger) esl.GatewayNameResolver {
	if gormDB == nil {
		logger.Warn("未配置 MySQL DSN，ESL 网关同步无法读取生产 gateway 表", "impact", "生产环境必须配置 gateway 表仓储")
		return telephony.NewMemoryGatewayRepository()
	}
	logger.Info("ESL 网关同步将从数据库读取网关配置", "table", "gateway")
	return telephony.NewGatewayRepository(gormDB, logger)
}

// buildAuthCacheInvalidator 构建 Kamailio auth 缓存失效器。
func buildAuthCacheInvalidator(redisClient *goredis.Client, logger *slog.Logger) operate.AuthCacheInvalidator {
	if redisClient == nil {
		logger.Warn("未配置 Redis，Kamailio auth 缓存无法自动失效", "impact", "生产环境应配置 Redis 以删除 kamailio:auth:*")
		return nil
	}
	return &kamailioauth.RedisAuthCacheInvalidator{Client: redisClient, Logger: logger}
}

// buildCandidateSource 构建 CTI 选号候选源。
// 生产候选来自  兼容 gateway/pool/pool_phone/skill_group/user_skill_group 关系；
// 无数据库时返回 nil，HTTP 层保留本地占位候选用于开发调试。
func buildCandidateSource(gormDB *gorm.DB, redisClient *goredis.Client, logger *slog.Logger) cti.CandidateSource {
	if gormDB == nil {
		logger.Warn("未配置 MySQL DSN，CTI 选号候选源使用本地占位", "impact", "生产环境必须配置号码池相关表仓储")
		return nil
	}
	logger.Info("CTI 选号候选源将从数据库读取", "tables", "gateway,pool,pool_phone,pool_phone_skill_group,skill_group,user_skill_group")
	source := cti.CandidateSource(resource.NewPhoneResourceRepository(gormDB, logger))
	if redisClient != nil {
		logger.Info("CTI 选号候选缓存已启用", "keyPattern", "cti:phone_resource:user:*")
		source = &selectioninfra.RedisCandidateSource{Client: redisClient, Source: source, TTL: 15 * time.Minute, Logger: logger}
	}
	return source
}

// buildCandidateMarker 构建选号黑白名单标记器。
func buildCandidateMarker(gormDB *gorm.DB, logger *slog.Logger) cti.CandidateMarker {
	if gormDB == nil {
		logger.Warn("未配置 MySQL DSN，CTI 黑白名单标记使用空实现", "impact", "白名单和黑名单不会影响选号结果")
		return cti.NoopCandidateMarker{}
	}
	logger.Info("CTI 黑白名单标记将从数据库读取", "tables", "whitelist_data,whitelist_data_merchant,blacklist_data,blacklist_gateway")
	return &selectioninfra.RuntimeSelectionMarker{DB: gormDB, Logger: logger}
}

// buildOutboxStore 构建可靠投递 outbox 存储。
// 生产环境使用数据库 `message_outbox`，本地无 MySQL 时使用内存兜底。
func buildOutboxStore(gormDB *gorm.DB, logger *slog.Logger) business.OutboxStore {
	if gormDB == nil {
		logger.Warn("未配置 MySQL DSN，outbox 使用内存存储", "impact", "服务重启会丢失待投递消息")
		return business.NewOutboxMemoryStore()
	}
	logger.Info("outbox 将使用数据库持久化", "table", "message_outbox")
	return business.NewOutboxGormStore(gormDB, logger)
}

// fsNodeConfigs 将注册表节点转换为 ESL 连接池配置。
// 仅包含地址有效且已启用的节点，空地址或禁用的节点会被过滤。
func fsNodeConfigs(nodes []telephony.Node) []fsesl.NodeConfig {
	configs := make([]fsesl.NodeConfig, 0, len(nodes))
	for _, node := range nodes {
		if node.FSAddr == "" || !node.Enable {
			continue
		}
		configs = append(configs, fsesl.NodeConfig{
			ID:       node.ID,
			Addr:     node.FSAddr,
			Password: node.Password,
			SetID:    node.SetID,
			Weight:   node.Weight,
			Enabled:  node.Enable,
		})
	}
	return configs
}

func buildFSLeaseOwner(serviceName string) string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		if serviceName != "" {
			return serviceName + "-local"
		}
		return "cc-call-local"
	}
	if serviceName == "" {
		return hostname
	}
	return serviceName + "-" + hostname
}

// inProcessESLClient 是进程内 ESL 客户端的简单封装。
// 用于 CTI 层直接调用同进程内的 OriginateService，避免跨进程通信开销。
// 当 CTI 工作流需要触发 ESL 外呼时，通过此接口在进程内完成调用。
type inProcessESLClient struct {
	originate *esl.OriginateService
}

// StartAPIOutbound 将 API 外呼请求委托给同进程的 OriginateService。
// version 参数用于追踪后端版本，callID 是呼叫唯一标识。
// 失败时会返回 esl 领域层的错误，调用方需根据错误类型进行响应。
func (c inProcessESLClient) StartAPIOutbound(ctx context.Context, version, callID string, req contracts.ApiCallReq) error {
	return c.originate.StartAPIOutbound(ctx, esl.OriginateRequest{Version: version, CallID: callID, Request: req})
}

// StartBatchOutbound 将批量外呼请求委托给同进程的 OriginateService。
func (c inProcessESLClient) StartBatchOutbound(ctx context.Context, version, callID string, req contracts.BatchCallReq) error {
	return c.originate.StartBatchOutbound(ctx, esl.BatchOriginateRequest{Version: version, CallID: callID, Request: req})
}

// registryNodeSelector 通过 FreeSWITCH 节点注册表实现节点选择。
// 支持按 setID 分组、权重负载均衡和并发控制。
// 当 setID 无法匹配时，fallback 为返回首个可用节点。
type registryNodeSelector struct {
	registry telephony.Registry
}

type registryGatewaySyncNodeLister struct {
	registry telephony.Registry
}

func (l registryGatewaySyncNodeLister) ListGatewaySyncNodes(ctx context.Context) ([]esl.GatewaySyncNode, error) {
	nodes, err := l.registry.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]esl.GatewaySyncNode, 0, len(nodes))
	for _, node := range nodes {
		if node.FSAddr == "" {
			continue
		}
		targets = append(targets, esl.GatewaySyncNode{ID: node.ID, FSAddr: node.FSAddr, CommandURL: node.CommandURL})
	}
	return targets, nil
}

// SelectAPIOutbound 为 API 外呼选择最优 FreeSWITCH 节点。
// 1. 从注册表获取所有启用节点
// 2. 根据请求 extra 中的 setID 过滤节点（支持 freeswitchSetid/setid/setId 字段）
// 3. 按权重负载均衡选择节点，若无匹配则 fallback 返回首个启用节点
// 返回 fsAddr 字符串，失败时返回错误
func (s registryNodeSelector) SelectAPIOutbound(ctx context.Context, req esl.OriginateRequest) (string, error) {
	nodes, err := s.registry.ListEnabled(ctx)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "default", nil
	}
	setID := resolveSetID(req.Request.Extra)
	candidates := make([]telephony.Node, 0, len(nodes))
	for _, node := range nodes {
		if node.FSAddr == "" || normalizeSetID(node.SetID) != setID || effectiveWeight(node) <= 0 {
			continue
		}
		candidates = append(candidates, node)
	}
	if len(candidates) == 0 {
		slog.Warn("未匹配到目的地分组可用 FreeSWITCH 节点，回退使用首个启用节点", "callId", req.CallID, "setId", setID, "enabledCount", len(nodes))
		return nodes[0].FSAddr, nil
	}
	selected := selectNodeByWeight(candidates, req.CallID)
	slog.Info("API 外呼已选择 FreeSWITCH 节点", "callId", req.CallID, "fsAddr", selected.FSAddr, "setId", normalizeSetID(selected.SetID), "weight", selected.Weight, "rweight", selected.RWeight, "cc", selected.CC, "effectiveWeight", effectiveWeight(selected))
	return selected.FSAddr, nil
}

// selectNodeByWeight 基于 callID 哈希实现加权随机节点选择。
// 相同 callID 在有效节点列表中始终选择同一节点，保证重试路由一致。
// total=0 时 fallback 返回首个节点。
func selectNodeByWeight(nodes []telephony.Node, callID string) telephony.Node {
	if len(nodes) == 1 {
		return nodes[0]
	}
	total := 0
	for _, node := range nodes {
		total += effectiveWeight(node)
	}
	if total <= 0 {
		return nodes[0]
	}
	cursor := int(hashString(callID) % uint64(total))
	for _, node := range nodes {
		cursor -= effectiveWeight(node)
		if cursor < 0 {
			return node
		}
	}
	return nodes[len(nodes)-1]
}

// resolveSetID 从请求 extra JSON 字符串中解析 setID 分组标识。
// 支持 freeswitchSetid、setid、setId 三种 key 格式。
// 返回解析出的整数 setID，若均无法解析则默认返回 1。
func resolveSetID(extra string) int {
	for _, key := range []string{"freeswitchSetid", "setid", "setId"} {
		idx := strings.Index(extra, `"`+key+`"`)
		if idx < 0 {
			continue
		}
		tail := extra[idx+len(key)+2:]
		colon := strings.Index(tail, ":")
		if colon < 0 {
			continue
		}
		value := strings.Trim(tail[colon+1:], " \t\r\n\",}")
		if parsed, err := strconv.Atoi(value); err == nil {
			return normalizeSetID(parsed)
		}
	}
	return 1
}

// normalizeSetID 标准化 setID，确保有效分组标识为正整数。
// setID <= 0 时统一映射为 1，保证节点分组匹配始终有效。
func normalizeSetID(setID int) int {
	if setID <= 0 {
		return 1
	}
	return setID
}

// normalizeWeight 标准化节点权重值。
// weight=0 时使用 fallback 提供的默认值，weight<0 时映射为 0（表示节点不可用）。
func normalizeWeight(weight int, fallback int) int {
	if weight == 0 {
		return fallback
	}
	if weight < 0 {
		return 0
	}
	return weight
}

// normalizeCC 标准化并发控制值。
// cc=0 时默认映射为 1，保证至少允许单个并发连接。
func normalizeCC(cc int) int {
	if cc == 0 {
		return 1
	}
	return cc
}

// effectiveWeight 计算节点的有效负载权重。
// 当并发控制 cc=1 时使用 RWeight（实时权重），否则使用 Weight（静态权重）。
// 权重为 0 时 fallback 为默认值 50。
func effectiveWeight(node telephony.Node) int {
	if normalizeCC(node.CC) == 1 {
		return normalizeWeight(node.RWeight, 50)
	}
	return normalizeWeight(node.Weight, 50)
}

// hashString 实现 FNV-1a 哈希算法，用于 callID 的确定性哈希。
// 保证相同 callID 在相同节点集合中始终映射到同一节点，实现稳定的加权路由。
func hashString(value string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(value); i++ {
		h ^= uint64(value[i])
		h *= 1099511628211
	}
	return h
}

// startOfflineExtensionUnbinder 循环检查离线动态绑定分机并解绑
func startOfflineExtensionUnbinder(ctx context.Context, db *gorm.DB, redisClient *goredis.Client, logger *slog.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	reader := extensionstatus.NewRedisReader(redisClient)
	logger.Info("分机动态绑定离线自动释放守护协程已启动")
	for {
		select {
		case <-ctx.Done():
			logger.Info("分机动态绑定离线自动释放守护协程已退出")
			return
		case <-ticker.C:
			if err := resource.ReleaseOfflineDynamicBindings(ctx, db, reader); err != nil {
				logger.Error("定期释放离线分机绑定失败", "error", err.Error())
			}
		}
	}
}

// startFSNodeLeaseClaimer 循环检测并竞争 FreeSWITCH 节点事件消费租约，支持多实例高可用与节点动态同步
func startFSNodeLeaseClaimer(ctx context.Context, pool *fsesl.ConnectionPool, registry telephony.Registry, logger *slog.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	logger.Info("FreeSWITCH 事件租约与连接守护协程已启动")
	for {
		select {
		case <-ctx.Done():
			logger.Info("FreeSWITCH 事件租约与连接守护协程已退出")
			return
		case <-ticker.C:
			// 1. 获取当前注册表中启用的所有 FreeSWITCH 节点
			nodes, err := registry.ListEnabled(ctx)
			if err != nil {
				logger.Error("事件租约守护协程读取节点配置失败", "error", err.Error())
				continue
			}

			// 2. 获取连接池中的节点配置快照
			poolNodes := pool.SnapshotNodes()
			poolNodeMap := make(map[string]fsesl.NodeConfig)
			for _, pn := range poolNodes {
				poolNodeMap[pn.Addr] = pn
			}

			latestNodeMap := make(map[string]telephony.Node)
			for _, n := range nodes {
				latestNodeMap[n.FSAddr] = n
			}

			// 3. 级联同步新增或修改的节点
			for _, n := range nodes {
				pn, exists := poolNodeMap[n.FSAddr]
				if !exists || pn.ID != n.ID || pn.Password != n.Password || pn.SetID != n.SetID || pn.Weight != n.Weight || !pn.Enabled {
					logger.Info("事件租约守护协程同步新增或更新的节点配置", "fsAddr", n.FSAddr)
					pool.UpsertNode(fsesl.NodeConfig{
						ID:       n.ID,
						Addr:     n.FSAddr,
						Password: n.Password,
						SetID:    n.SetID,
						Weight:   n.Weight,
						Enabled:  n.Enable,
					})
				}
			}

			// 4. 级联同步已禁用或删除的节点
			for _, pn := range poolNodes {
				if _, exists := latestNodeMap[pn.Addr]; !exists {
					logger.Info("事件租约守护协程检测到节点已失效，移出连接池", "fsAddr", pn.Addr)
					pool.RemoveNode(pn.Addr)
				}
			}

			// 5. 扫描连接池状态，为无连接的节点抢占事件监听租约
			statusList := pool.Status()
			for _, status := range statusList {
				if !status.Connected {
					logger.Debug("事件租约守护协程尝试竞争租约并连接 FreeSWITCH", "fsAddr", status.FSAddr)
					if _, err := pool.Connect(ctx, status.FSAddr); err != nil {
						if errors.Is(err, operate.ErrLeaseHeld) {
							logger.Debug("FreeSWITCH 事件租约仍被其他实例持有，跳过连接", "fsAddr", status.FSAddr)
						} else {
							logger.Warn("事件租约守护协程竞争连接 FreeSWITCH 失败", "fsAddr", status.FSAddr, "error", err.Error())
						}
					} else {
						logger.Info("事件租约守护协程成功竞争并连通 FreeSWITCH 节点", "fsAddr", status.FSAddr)
					}
				}
			}
		}
	}
}
