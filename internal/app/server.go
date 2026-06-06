package app

// app 包提供 cc-call 和 cc-console 服务的应用层组装逻辑。
// 负责创建 HTTP 服务器、注册路由、初始化依赖和提供统一的启动入口。

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	authdomain "yunshu/internal/domain/auth"
	"yunshu/internal/domain/esl"
	operatedomain "yunshu/internal/domain/operate"
	"yunshu/internal/infra/business"
	"yunshu/internal/infra/config"
	"yunshu/internal/infra/eslgateway"
	"yunshu/internal/infra/events"
	"yunshu/internal/infra/extensionstatus"
	"yunshu/internal/infra/installer"
	"yunshu/internal/infra/logging"
	"yunshu/internal/infra/merchant"
	redisinfra "yunshu/internal/infra/redis"
	"yunshu/internal/infra/resource"
	"yunshu/internal/infra/security"
	selectioninfra "yunshu/internal/infra/selection"
	"yunshu/internal/infra/system"
	"yunshu/internal/infra/telephony"
	"yunshu/internal/observability"
	httpcti "yunshu/internal/transport/http/call/cti"
	httpesl "yunshu/internal/transport/http/call/esl"
	httpoperate "yunshu/internal/transport/http/console/operate"
)

// Server 封装 Gin HTTP 服务器和应用依赖。
// - Name 标识服务名称，用于路由注册和日志区分
// - gin 是 Gin HTTP 引擎，注册了健康检查、契约发现和领域路由
// - callRuntime 仅 cc-call 服务特有，聚合 CTI 和 ESL 业务服务
type Server struct {
	Name        contracts.ServiceName
	gin         *gin.Engine
	callRuntime *CallRuntime
	console     *ConsoleRuntime
	worker      *WorkerRuntime
	installer   *installer.Installer
	cfg         config.Config
	ctx         context.Context
	cancel      context.CancelFunc
}

// ConsoleRuntime 聚合 cc-console 管理端运行时依赖。
// 运营端接口优先读写 兼容数据库；没有 MySQL DSN 时使用配置文件节点作为本地兜底。
type ConsoleRuntime struct {
	Auth             *authdomain.AuthService
	RoutePermissions authdomain.RoutePermissionResolver
	Permissions      *system.PermissionRepository
	Account          *operatedomain.AccountManagementService
	Channel          *operatedomain.ChannelManagementService
	Blacklist        *operatedomain.BlacklistManagementService
	FreeSwitch       *operatedomain.FreeSwitchManagementService
	Merchant         *operatedomain.MerchantManagementService
	Rate             *operatedomain.RateManagementService
	Whitelist        *operatedomain.WhitelistManagementService
	Billing          *operatedomain.BillingManagementService
	BatchTask        *operatedomain.BatchTaskManagementService
	Department       *operatedomain.DepartmentManagementService
	CallRecord       *operatedomain.CallRecordManagementService
	AIFlow           *operatedomain.AIModelFlowManagementService
	AIConfig         *operatedomain.AIModelConfigManagementService
	Gateway          *operatedomain.GatewayManagementService
	Rtpengine        *operatedomain.RtpengineManagementService
	Dispatcher       *operatedomain.DispatcherManagementService
	Extension        *operatedomain.ExtensionManagementService
	Pool             *operatedomain.PoolManagementService
	PoolPhone        *operatedomain.PoolPhoneManagementService
	PhoneGroup       *operatedomain.PhoneGroupManagementService
	SkillGroup       *operatedomain.SkillGroupManagementService
	RiskControl      *operatedomain.RiskControlManagementService
	PhoneAttribution *operatedomain.PhoneAttributionManagementService
	ProxyConfig      *operatedomain.ProxyConfigManagementService
	AreaCode         operatedomain.AreaCodeRepository
	License          *operatedomain.LicenseService
	IPBlock          *operatedomain.IPBlockManagementService
	DB               *gorm.DB
}

// NewServer 负责创建单个服务进程的 Gin 引擎，并注册通用探活、契约发现和领域路由。
// 这里保持装配逻辑集中，避免各个 cmd 入口散落不同的中间件和基础路由。
func NewServer(name contracts.ServiceName) (*Server, error) {
	return NewServerWithConfig(name, config.Config{})
}

// NewServerWithConfig 按配置创建服务。
// cc-call 可以通过配置切换内存事件总线或 Redis Stream 事件总线。
func NewServerWithConfig(name contracts.ServiceName, cfg config.Config) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{Name: name, gin: gin.New(), cfg: cfg, ctx: ctx, cancel: cancel}
	if name == contracts.ServiceCall {
		var err error
		s.callRuntime, err = NewCallRuntimeWithConfig(ctx, cfg, buildEventBus(cfg), slog.Default())
		if err != nil {
			cancel()
			return nil, err
		}
	}
	if name == contracts.ServiceConsole {
		var err error
		s.console, err = NewConsoleRuntimeWithConfig(cfg, slog.Default())
		if err != nil {
			cancel()
			return nil, err
		}
		s.installer = installer.NewInstaller(slog.Default())
	}
	if name == contracts.ServiceWorker {
		var err error
		s.worker, err = NewWorkerRuntimeWithConfig(cfg, slog.Default())
		if err != nil {
			cancel()
			return nil, err
		}
		s.worker.Dispatcher.Events = buildEventBus(cfg)
		s.worker.Start(s.ctx, cfg.Worker.Outbox.Interval)
	}
	s.routes()
	return s, nil
}

// NewConsoleRuntimeWithConfig 创建 cc-console 管理端运行时。
// 这里复用 cc-call 的数据库和 FreeSWITCH registry 装配规则，保证管理端和呼叫端看到同一套配置真相。
func NewConsoleRuntimeWithConfig(cfg config.Config, logger *slog.Logger) (*ConsoleRuntime, error) {
	if logger == nil {
		logger = slog.Default()
	}
	gormDB, err := openRuntimeDB(cfg, logger)
	if err != nil {
		return nil, err
	}
	var redisClient *goredis.Client
	if len(cfg.Redis.Addrs) > 0 {
		redisClient = redisinfra.NewClient(cfg.Redis)
		logger.Info("cc-console Redis 会话与缓存已启用", "redisAddr", cfg.Redis.Addrs[0])
	} else {
		logger.Warn("未配置 Redis 地址，cc-console 使用本地内存会话兜底", "impact", "多实例登录态不会共享")
	}
	registry := buildFSRegistry(cfg, gormDB, logger)
	var authStore authdomain.SessionStore = authdomain.NewMemorySessionStore()
	if redisClient != nil {
		authStore = redisinfra.NewRedisSessionStore(redisClient, "")
		logger.Info("管理端会话将写入 Redis", "keyPrefix", contracts.KeyConsoleAuthSessionPrefix)
	}
	authService := &authdomain.AuthService{Store: authStore, TTL: 12 * time.Hour, Logger: logger}
	var routePermissionResolver authdomain.RoutePermissionResolver
	var permissionRepository *system.PermissionRepository
	accountRepository := operatedomain.AccountRepository(system.NewMemoryAccountRepository())
	var memoryAccountRepository *system.MemoryAccountRepository
	if repo, ok := accountRepository.(*system.MemoryAccountRepository); ok {
		memoryAccountRepository = repo
		seedMemoryAccounts(memoryAccountRepository, logger)
		authService.IdentityResolver = memoryAccountRepository
	}
	if gormDB != nil {
		accountRepo := system.NewConsoleAccountRepository(gormDB, logger)
		if err := accountRepo.EnsureDefaults(context.Background()); err != nil {
			logger.Error("控制台默认账号初始化失败", "error", err.Error())
		} else {
			logger.Info("控制台默认账号已完成初始化", "table", "console_account", "accounts", "admin,operator,merchant")
		}
		permissionRepository = system.NewPermissionRepository(gormDB, logger)
		if err := permissionRepository.EnsureDefaults(context.Background()); err != nil {
			logger.Error("管理端权限初始化失败", "error", err.Error())
		} else {
			logger.Info("管理端默认角色和权限已完成初始化", "tables", "console_role,console_permission,console_role_permission,console_route_permission")
		}

		accountRepository = accountRepo
		authService.IdentityResolver = accountRepo
		authService.Permissions = permissionRepository
		routePermissionResolver = permissionRepository
		logger.Info("管理端权限配置将从数据库读取", "tables", "console_permission,console_role_permission,console_route_permission")
	}

	var statuses esl.ExtensionStatusReader
	if redisClient != nil {
		statuses = extensionstatus.NewRedisReader(redisClient)
	}

	// 初始化商户仓储，使用重构后的 merchant 包以对齐规范
	merchantRepository := operatedomain.MerchantRepository(merchant.NewMemoryMerchantRepository())
	if gormDB != nil {
		merchantRepository = merchant.NewMerchantRepository(gormDB, statuses, logger)
		logger.Info("运营端商户配置将从数据库读取", "table", "merchant")
		seedDatabaseMerchant(gormDB, logger)
	} else {
		if memoryMerchantRepository, ok := merchantRepository.(*merchant.MemoryMerchantRepository); ok && memoryAccountRepository != nil {
			seedMemoryMerchants(memoryMerchantRepository, memoryAccountRepository, logger)
		}
		logger.Warn("未配置 MySQL DSN，运营端商户管理使用本地内存兜底", "impact", "生产环境必须配置 merchant 表仓储")
	}
	batchTaskRepository := operatedomain.BatchTaskRepository(business.NewMemoryBatchTaskRepository())
	if gormDB != nil {
		batchTaskRepository = business.NewBatchRepository(gormDB, statuses, logger)
		logger.Info("运营端批量外呼任务配置将从数据库读取", "table", "merchant_batch_call_task")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端批量外呼任务管理使用本地内存兜底", "impact", "生产环境必须配置 merchant_batch_call_task 表仓储")
	}
	var callRecordService *operatedomain.CallRecordManagementService
	var callRecordRepository operatedomain.CallRecordRepository
	if gormDB != nil {
		callRecordRepository = business.NewQueryRepository(gormDB)
		callRecordService = &operatedomain.CallRecordManagementService{
			Repository:  callRecordRepository,
			RedisClient: redisClient,
			Logger:      logger,
		}
		logger.Info("运营端呼叫记录将从数据库读取", "table", "call_cdr_record")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端呼叫记录管理未启用", "impact", "生产环境必须配置 call_cdr_record 表仓储")
	}
	aiFlowRepository := operatedomain.AIModelFlowRepository(operatedomain.NewMemoryAIModelFlowRepository())
	if gormDB != nil {
		aiFlowRepository = business.NewAIModelFlowRepository(gormDB, logger)
		logger.Info("运营端 AI 流程将从数据库读取", "table", "merchant_ai_model_flow")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端 AI 流程管理使用本地内存兜底", "impact", "生产环境必须配置 merchant_ai_model_flow 表仓储")
	}
	aiFlowService := &operatedomain.AIModelFlowManagementService{Repository: aiFlowRepository, Logger: logger}

	aiConfigRepository := operatedomain.AIModelConfigRepository(business.NewMemoryAIModelConfigRepository())
	if gormDB != nil {
		aiConfigRepository = business.NewAIModelConfigRepository(gormDB, logger)
		logger.Info("运营端 AI 模型配置将从数据库读取", "table", "merchant_ai_model_config")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端 AI 模型配置使用本地内存兜底")
	}
	aiConfigService := &operatedomain.AIModelConfigManagementService{Repository: aiConfigRepository, Logger: logger}
	// 初始化渠道仓储，使用重构后的 resource 包以对齐物理重组规范
	channelRepository := operatedomain.ChannelRepository(resource.NewMemoryChannelRepository())
	if gormDB != nil {
		channelRepository = resource.NewChannelRepository(gormDB, logger)
		logger.Info("运营端渠道配置将从数据库读取", "table", "channel")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端渠道管理使用本地内存兜底", "impact", "生产环境必须配置 channel 表仓储")
	}
	// 初始化号码池仓储，使用重构后的 resource 包以对齐物理重组规范
	poolRepository := operatedomain.PoolRepository(resource.NewMemoryPoolRepository())
	if gormDB != nil {
		poolRepository = resource.NewPoolRepository(gormDB, logger)
		logger.Info("运营端号码池配置将从数据库读取", "table", "pool")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端号码池管理使用本地内存兜底", "impact", "生产环境必须配置 pool 表仓储")
	}
	phoneRepository := operatedomain.PoolPhoneRepository(resource.NewMemoryPoolPhoneRepository())
	if gormDB != nil {
		phoneRepository = resource.NewPoolPhoneRepository(gormDB, logger)
		logger.Info("运营端号码配置将从数据库读取", "table", "pool_phone")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端号码管理使用本地内存兜底", "impact", "生产环境必须配置 pool_phone 表仓储")
	}
	extensionRepository := operatedomain.ExtensionManagementRepository(resource.NewMemoryExtensionManagementRepository())
	if gormDB != nil {
		extensionRepository = resource.NewExtensionRepository(gormDB, logger)
		logger.Info("运营端分机配置将从数据库读取", "table", "extension")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端分机管理使用本地内存兜底", "impact", "生产环境必须配置 extension 表仓储")
	}
	phoneGroupRepository := operatedomain.PhoneGroupRepository(resource.NewMemoryPhoneGroupRepository())
	if gormDB != nil {
		phoneGroupRepository = resource.NewPhoneGroupRepository(gormDB, logger)
		logger.Info("商户端号码组配置将从数据库读取", "table", "merchant_phone_group")
	} else {
		logger.Warn("未配置 MySQL DSN，商户端号码组管理使用本地内存兜底", "impact", "生产环境必须配置 merchant_phone_group 表仓储")
	}
	skillGroupRepository := operatedomain.SkillGroupRepository(resource.NewMemorySkillGroupRepository())
	if gormDB != nil {
		skillGroupRepository = resource.NewSkillGroupRepository(gormDB, logger)
		logger.Info("商户端技能组配置将从数据库读取", "table", "skill_group")
	} else {
		logger.Warn("未配置 MySQL DSN，商户端技能组管理使用本地内存兜底", "impact", "生产环境必须配置 skill_group 表仓储")
	}
	departmentRepository := operatedomain.DepartmentRepository(resource.NewMemoryDepartmentRepository())
	if gormDB != nil {
		departmentRepository = resource.NewDepartmentRepository(gormDB, logger)
		logger.Info("运营端部门配置将从数据库读取", "table", "cc_res_department")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端部门管理使用本地内存兜底", "impact", "生产环境必须配置 cc_res_department 表仓储")
	}
	gatewayRepository := operatedomain.GatewayRepository(telephony.NewMemoryGatewayRepository())
	if gormDB != nil {
		gatewayRepository = telephony.NewGatewayRepository(gormDB, logger)
		logger.Info("运营端网关配置将从数据库读取", "table", "gateway")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端网关管理使用本地内存兜底", "impact", "生产环境必须配置 gateway 表仓储")
	}
	rateRepository := operatedomain.RateRepository(merchant.NewMemoryRateRepository())
	if gormDB != nil {
		rateRepository = merchant.NewRateRepository(gormDB, logger)
		logger.Info("运营端费率配置将从数据库读取", "tables", "call_rate,call_rate_merchant")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端费率管理使用本地内存兜底", "impact", "生产环境必须配置 call_rate 和 call_rate_merchant 表仓储")
	}
	blacklistRepository := operatedomain.BlacklistRepository(security.NewMemoryBlacklistRepository())
	if gormDB != nil {
		blacklistRepository = security.NewBlacklistRepository(gormDB, logger)
		logger.Info("运营端黑名单配置将从数据库读取", "tables", "blacklist,blacklist_gateway")
		// 预热风控验证通道动态配置缓存
		if list, err := blacklistRepository.ListChannels(context.Background()); err == nil {
			operatedomain.LoadAllChannelsToCache(list)
			logger.Info("三方风控验证通道快速缓存预热成功", "count", len(list))
		} else {
			logger.Error("三方风控验证通道快速缓存预热失败", "error", err.Error())
		}
	} else {
		logger.Warn("未配置 MySQL DSN，运营端黑名单管理使用本地内存兜底", "impact", "生产环境必须配置 blacklist 和 blacklist_gateway 表仓储")
	}
	whitelistRepository := operatedomain.WhitelistRepository(security.NewMemoryWhitelistRepository())
	if gormDB != nil {
		whitelistRepository = security.NewWhitelistRepository(gormDB, logger)
		logger.Info("运营端白名单配置将从数据库读取", "tables", "whitelist_data,whitelist_data_merchant")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端白名单管理使用本地内存兜底", "impact", "生产环境必须配置 whitelist_data 和 whitelist_data_merchant 表仓储")
	}
	billingRepository := operatedomain.BillingRepository(merchant.NewMemoryBillingRepository())
	if gormDB != nil {
		billingRepository = merchant.NewBillingRepository(gormDB, logger)
		logger.Info("运营端商户账务将从数据库读取", "tables", "merchant_billing_overview,merchant_billing_recharge")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端商户账务管理使用本地内存兜底", "impact", "生产环境必须配置 merchant_billing_overview 和 merchant_billing_recharge 表仓储")
	}
	var gatewayCacheInvalidator operatedomain.GatewayCacheInvalidator
	if redisClient != nil {
		gatewayCacheInvalidator = &selectioninfra.RedisCandidateCacheInvalidator{Client: redisClient, Logger: logger}
	}
	var gatewaySynchronizer operatedomain.GatewaySynchronizer
	if cfg.Console.CallBaseURL != "" {
		gatewaySynchronizer = &eslgateway.Synchronizer{BaseURL: cfg.Console.CallBaseURL, Timeout: cfg.FreeSwitch.CommandTimeout, Logger: logger}
		logger.Info("运营端网关运行时同步已启用", "callBaseURL", cfg.Console.CallBaseURL)
	} else {
		logger.Warn("运营端网关运行时同步未启用", "impact", "生产环境应配置 CC_CALL_BASE_URL")
	}
	logger.Info("cc-console FreeSWITCH 管理能力已启用", "databaseConfigured", gormDB != nil)
	rtpengineRepository := operatedomain.RtpengineRepository(telephony.NewMemoryRtpengineRepository())
	dispatcherRepository := operatedomain.DispatcherRepository(telephony.NewMemoryDispatcherRepository())
	if gormDB != nil {
		rtpengineRepository = telephony.NewRtpengineRepository(gormDB, logger)
		logger.Info("运营端 Kamailio rtpengine 配置将从数据库读取", "table", "cc_res_rtpengine")
		dispatcherRepository = telephony.NewGormDispatcherRepository(gormDB, logger)
		logger.Info("运营端 Kamailio dispatcher 配置将从数据库读取", "table", "cc_res_freeswitch")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端 Kamailio rtpengine 管理使用本地内存兜底", "impact", "生产环境必须配置 cc_res_rtpengine 表仓储")
		logger.Warn("未配置 MySQL DSN，运营端 Kamailio dispatcher 管理使用本地内存兜底", "impact", "生产环境必须配置 cc_res_freeswitch 表仓储")
	}
	riskControlRepository := operatedomain.RiskControlRepository(security.NewMemoryRiskControlRepository())
	if gormDB != nil {
		riskControlRepository = security.NewRiskControlRepository(gormDB, logger)
		logger.Info("运营端风控配置将从数据库读取", "tables", "risk_control,risk_control_merchant")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端风控管理使用本地内存兜底", "impact", "生产环境必须配置 risk_control 和 risk_control_merchant 表仓储")
	}
	phoneAttributionRepository := operatedomain.PhoneAttributionRepository(system.NewMemoryPhoneAttributionRepository())
	if gormDB != nil {
		phoneAttributionRepository = system.NewPhoneAttributionGormRepository(gormDB, logger)
		logger.Info("运营端号码归属地配置将从数据库读取", "table", "phone_attribution")
	} else {
		logger.Warn("未配置 MySQL DSN，运营端号码归属地管理使用本地内存兜底", "impact", "生产环境必须配置 phone_attribution 表仓储")
	}
	phoneAttributionRepository = system.NewCachedPhoneAttributionRepository(phoneAttributionRepository)

	var areaCodeRepository operatedomain.AreaCodeRepository
	if gormDB != nil {
		areaCodeRepository = system.NewAreaCodeGormRepository(gormDB)
		logger.Info("运营端行政区划配置将从数据库读取", "table", "area_code")
	} else {
		areaCodeRepository = system.NewMemoryAreaCodeRepository()
		logger.Warn("未配置 MySQL DSN，运营端行政区划使用内存兜底")
	}

	authCacheInvalidator := buildAuthCacheInvalidator(redisClient, logger)

	var proxyConfigService *operatedomain.ProxyConfigManagementService
	var dispReloader operatedomain.DispatcherReloadPort
	rtpengineService := &operatedomain.RtpengineManagementService{Repository: rtpengineRepository, Logger: logger}
	dispatcherService := &operatedomain.DispatcherManagementService{Repository: dispatcherRepository, Logger: logger}

	licenseService := operatedomain.NewLicenseService(nil, cfg.Console.LicensePath, logger)
	var ipBlockService *operatedomain.IPBlockManagementService

	if gormDB != nil {
		proxyConfigRepo := system.NewProxyConfigRepository(gormDB, logger)
		if err := proxyConfigRepo.EnsureDefaults(context.Background()); err != nil {
			logger.Error("代理默认配置初始化失败", "error", err.Error())
		} else {
			logger.Info("代理默认配置已完成初始化", "table", "proxy_config")
		}

		// 同步 YAML 配置中的租户隔离模式至数据库参数表，以防出现配置不一致导致前端逻辑异常
		if cfg.Tenant.Mode != "" {
			if err := proxyConfigRepo.Set(context.Background(), "tenant.mode", cfg.Tenant.Mode, "云枢隔离部署模式(single/multi)"); err != nil {
				logger.Error("同步 YAML 租户模式配置至数据库失败", "error", err.Error())
			} else {
				logger.Info("已成功同步 YAML 租户模式配置至数据库", "mode", cfg.Tenant.Mode)
			}
		}

		reloader := &kamailioRtpengineReloader{repo: proxyConfigRepo, logger: logger}
		rtpengineService.Reloader = reloader
		dispReloader = telephony.NewKamailioDispatcherReloader(proxyConfigRepo, logger)
		dispatcherService.Reloader = dispReloader
		proxyConfigService = operatedomain.NewProxyConfigManagementService(proxyConfigRepo, reloader, redisClient, logger)
		if err := proxyConfigService.SyncToRedis(context.Background()); err != nil {
			logger.Error("系统启动同步代理配置至 Redis 失败", "error", err.Error())
		}

		// 绑定真正的数据库仓储
		licenseService = operatedomain.NewLicenseService(proxyConfigRepo, cfg.Console.LicensePath, logger)
		// 1. 自动计算唯一部署 ID 并输出醒目的中文启动日志
		depID, err := licenseService.GetDeploymentID()
		if err == nil {
			logger.Info("【云枢授权】系统启动成功", "部署唯一标识(Deployment ID)", depID, "运行状态", "正常运行")
		} else {
			logger.Error("【云枢授权】计算部署唯一标识失败", "error", err.Error())
		}
		// 2. 自动执行首次 License 验证，若发现已失效（超出宽限期），在日志中打印中文警告
		_, err = licenseService.LoadAndVerify(context.Background())
		if err != nil {
			logger.Warn("【云枢授权】未激活可用的私有化授权或授权已失效", "error", err.Error())
		}

		// 初始化 IP 拦截服务
		ipBlockLogRepo := system.NewGormIPBlockLogRepository(gormDB, logger)
		ipBlockService = operatedomain.NewIPBlockManagementService(ipBlockLogRepo, proxyConfigRepo, redisClient, logger)
		if err := ipBlockService.SyncToRedis(context.Background()); err != nil {
			logger.Error("系统启动同步 IP 拦截配置至 Redis 失败", "error", err.Error())
		}
	} else {
		logger.Warn("未配置 MySQL DSN，系统参数配置与 RTPEngine/Dispatcher 热刷新未启用")
		ipBlockService = operatedomain.NewIPBlockManagementService(system.NewMemoryIPBlockLogRepository(), nil, redisClient, logger)
	}

	return &ConsoleRuntime{
		Auth:             authService,
		RoutePermissions: routePermissionResolver,
		Permissions:      permissionRepository,
		Account:          &operatedomain.AccountManagementService{Repository: accountRepository, Logger: logger},
		Channel:          &operatedomain.ChannelManagementService{Repository: channelRepository, Logger: logger},
		Blacklist:        &operatedomain.BlacklistManagementService{Repository: blacklistRepository, Logger: logger},
		FreeSwitch:       &operatedomain.FreeSwitchManagementService{Registry: registry, Reloader: dispReloader, Logger: logger},
		Merchant:         &operatedomain.MerchantManagementService{Repository: merchantRepository, ExtensionRepo: extensionRepository, Cache: authCacheInvalidator, Logger: logger},
		Rate:             &operatedomain.RateManagementService{Repository: rateRepository, Logger: logger},
		Whitelist:        &operatedomain.WhitelistManagementService{Repository: whitelistRepository, Logger: logger},
		Billing:          &operatedomain.BillingManagementService{Repository: billingRepository, Logger: logger},
		BatchTask:        &operatedomain.BatchTaskManagementService{Repository: batchTaskRepository, Logger: logger},
		Department:       &operatedomain.DepartmentManagementService{Repository: departmentRepository, Logger: logger},
		CallRecord:       callRecordService,
		AIFlow:           aiFlowService,
		AIConfig:         aiConfigService,
		Gateway:          &operatedomain.GatewayManagementService{Repository: gatewayRepository, Synchronizer: gatewaySynchronizer, Cache: gatewayCacheInvalidator, Logger: logger},
		Rtpengine:        rtpengineService,
		Dispatcher:       dispatcherService,
		Extension:        &operatedomain.ExtensionManagementService{Repository: extensionRepository, MerchantRepo: merchantRepository, Cache: authCacheInvalidator, License: licenseService, Logger: logger},
		Pool:             &operatedomain.PoolManagementService{Repository: poolRepository, Logger: logger},
		PoolPhone:        &operatedomain.PoolPhoneManagementService{Repository: phoneRepository, Logger: logger},
		PhoneGroup:       &operatedomain.PhoneGroupManagementService{Repository: phoneGroupRepository, Logger: logger},
		SkillGroup:       &operatedomain.SkillGroupManagementService{Repository: skillGroupRepository, Logger: logger},
		RiskControl:      &operatedomain.RiskControlManagementService{Repository: riskControlRepository, Logger: logger},
		PhoneAttribution: &operatedomain.PhoneAttributionManagementService{Repository: phoneAttributionRepository, Logger: logger},
		ProxyConfig:      proxyConfigService,
		AreaCode:         areaCodeRepository,
		License:          licenseService,
		IPBlock:          ipBlockService,
		DB:               gormDB,
	}, nil
}

// ListenAndServe 启动 HTTP 服务。在 context 被取消时触发优雅停机。
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           s.gin,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		slog.Info("服务已启动", "service", s.Name, "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
		close(errChan)
	}()

	select {
	case <-ctx.Done():
		slog.Info("收到关闭通知，开始优雅关闭...", "service", s.Name)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		s.Shutdown(shutdownCtx)
		return server.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

// Shutdown 执行 Server 内部相关资源的释放工作，防止 goroutine 泄漏。
func (s *Server) Shutdown(ctx context.Context) {
	if s.cancel != nil {
		s.cancel()
	}
	if s.callRuntime != nil {
		if s.callRuntime.WSHubCancel != nil {
			s.callRuntime.WSHubCancel()
		}
		if s.callRuntime.ASRServer != nil {
			s.callRuntime.ASRServer.Stop()
		}
		if s.callRuntime.FSPool != nil {
			s.callRuntime.FSPool.CloseAll()
		}
	}
}

// Run 是所有 cmd 服务入口共用的启动函数，保证参数解析、日志和退出语义一致。
func Run(name contracts.ServiceName) {
	addr := flag.String("addr", env("ADDR", ":8080"), "listen address")
	configPath := flag.String("config", env("CONFIG", ""), "config file path")
	flag.Parse()
	if *configPath == "" {
		if _, err := os.Stat("configs/default.yaml"); err == nil {
			*configPath = "configs/default.yaml"
		}
	}
	cfg := config.Config{}
	if *configPath != "" {
		loaded, err := config.Load(*configPath)
		if err != nil {
			slog.Error("配置加载失败", "path", *configPath, "error", err)
			os.Exit(1)
		}
		cfg = loaded
		if cfg.Service.Addr != "" && *addr == env("ADDR", ":8080") {
			*addr = cfg.Service.Addr
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s, err := NewServerWithConfig(name, cfg)
	if err != nil {
		slog.Error("服务初始化失败", "service", name, "error", err)
		os.Exit(1)
	}

	if err := s.ListenAndServe(ctx, *addr); err != nil {
		slog.Error("服务异常停止", "service", name, "error", err)
		os.Exit(1)
	}
}

// buildEventBus 根据配置构建事件总线。
// 当 Redis Stream 未启用时返回 nil，cc-call 将使用内存事件总线。
// 启用时创建 Redis Stream 消费者 goroutine，消费失败会记录错误但不阻塞主流程。
func buildEventBus(cfg config.Config) events.Bus {
	if !cfg.Redis.Stream.Enabled {
		return nil
	}
	client := redisinfra.NewClient(cfg.Redis)
	bus := events.NewRedisStreamBus(client, events.RedisStreamConfig{
		Stream:   cfg.Redis.Stream.Stream,
		Group:    cfg.Redis.Stream.Group,
		Consumer: cfg.Redis.Stream.Consumer,
		Block:    cfg.Redis.Stream.Block,
		Count:    cfg.Redis.Stream.BatchSize,
	}, slog.Default())
	go func() {
		if err := bus.RunConsumer(context.Background()); err != nil {
			slog.Error("Redis Stream 消费循环停止", "error", err)
		}
	}()
	return bus
}

// routes 注册所有 HTTP 路由，包括通用探活、契约发现和领域路由。
// - 通用路由：/healthz、/contracts/* 用于服务发现和监控
// - cc-call 路由：CTI 和 ESL 领域路由由对应 RegisterRoutes 函数注册
// - 其他服务路由：通过 registerCompatibilityRoutes 注册兼容占位端点
func (s *Server) routes() {
	s.gin.Use(s.requestMiddleware(), gin.Recovery(), s.consoleAccessMiddleware(), s.consoleTenantMiddleware())
	s.gin.GET("/healthz", s.handleHealth)
	s.gin.GET("/contracts/routes", s.handleRoutes)
	s.gin.GET("/contracts/redis", s.handleRedis)
	s.gin.GET("/contracts/mq", s.handleMQ)

	switch s.Name {
	case contracts.ServiceCall:
		var redisClient *goredis.Client
		if s.callRuntime.WSHub != nil {
			redisClient = s.callRuntime.WSHub.Client
		}
		httpcti.RegisterRoutes(s.gin, s.callRuntime.APICall, s.callRuntime.Selector, s.callRuntime.BatchScheduler, s.callRuntime.Candidates, s.callRuntime.Marker, s.callRuntime.WSHub, s.callRuntime.DB, redisClient, s.callRuntime.FSPool, s.callRuntime.CallControl)
		httpesl.RegisterRoutes(s.gin, s.callRuntime.Originate, s.callRuntime.Command, s.callRuntime.Session, s.callRuntime.GatewaySync, s.callRuntime.FSNodes, s.callRuntime.FSPool, s.callRuntime.DB)
	case contracts.ServiceConsole:
		httpoperate.RegisterAuthRoutes(s.gin, s.console.Auth)
		httpoperate.RegisterDialpadCompatRoutes(s.gin, s.console.Auth, s.console.CallRecord, s.console.Extension, s.console.DB, s.cfg.Console.Dialpad)
		httpoperate.RegisterPermissionRoutes(s.gin, s.console.Permissions)
		httpoperate.RegisterAccountRoutes(s.gin, s.console.Account)
		httpoperate.RegisterFreeSwitchRoutes(s.gin, s.console.FreeSwitch)
		httpoperate.RegisterChannelRoutes(s.gin, s.console.Channel)
		httpoperate.RegisterBlacklistRoutes(s.gin, s.console.Blacklist)
		httpoperate.RegisterWhitelistRoutes(s.gin, s.console.Whitelist)
		httpoperate.RegisterBillingRoutes(s.gin, s.console.Billing)
		httpoperate.RegisterExtensionRoutes(s.gin, s.console.Extension)
		httpoperate.RegisterPoolRoutes(s.gin, s.console.Pool)
		httpoperate.RegisterPoolPhoneRoutes(s.gin, s.console.PoolPhone, s.console.Pool)
		httpoperate.RegisterMerchantRoutes(s.gin, s.console.Merchant)
		httpoperate.RegisterRateRoutes(s.gin, s.console.Rate)
		httpoperate.RegisterBatchTaskRoutes(s.gin, s.console.BatchTask)
		httpoperate.RegisterBatchDialpadRoutes(s.gin, s.console.BatchTask)
		httpoperate.RegisterDepartmentRoutes(s.gin, s.console.Department)
		httpoperate.RegisterCallRecordRoutes(s.gin, s.console.CallRecord)
		httpoperate.RegisterAIModelFlowRoutes(s.gin, s.console.AIFlow)
		httpoperate.RegisterAIModelConfigRoutes(s.gin, s.console.AIConfig)
		httpoperate.RegisterPhoneGroupRoutes(s.gin, s.console.PhoneGroup)
		httpoperate.RegisterSkillGroupRoutes(s.gin, s.console.SkillGroup)
		httpoperate.RegisterGatewayRoutes(s.gin, s.console.Gateway)
		httpoperate.RegisterRtpengineRoutes(s.gin, s.console.Rtpengine)
		httpoperate.RegisterDispatcherRoutes(s.gin, s.console.Dispatcher)
		httpoperate.RegisterRiskControlRoutes(s.gin, s.console.RiskControl)
		httpoperate.RegisterPhoneAttributionRoutes(s.gin, s.console.PhoneAttribution)
		httpoperate.RegisterProxyConfigRoutes(s.gin, s.console.ProxyConfig)
		httpoperate.RegisterAreaCodeRoutes(s.gin, s.console.AreaCode)
		httpoperate.RegisterInstallerRoutes(s.gin, s.installer)
		httpoperate.RegisterLicenseRoutes(s.gin, s.console.License)
		httpoperate.RegisterIPBlockRoutes(s.gin, s.console.IPBlock)
		s.registerCompatibilityRoutes("/operate/auth", "/operate/account", "/operate/freeswitch", "/operate/channel", "/operate/blacklist", "/operate/whitelist", "/operate/billing", "/operate/extension", "/operate/pool", "/operate/pool-phone", "/operate/merchant", "/operate/rate", "/operate/gateway", "/operate/kamailio/rtpengine", "/operate/kamailio/dispatcher", "/operate/risk-control", "/operate/phone-attribution", "/operate/proxy-config", "/operate/area-code", "/operate/ai-model-config", "/operate/license", "/operate/ip-block", "/merchant/auth", "/merchant/account", "/merchant/batch-call-task", "/merchant/batch-call-dialpad", "/merchant/call-record", "/merchant/ai-model-flow", "/merchant/ai-model-config", "/merchant/phone-group", "/merchant/skill-group", "/merchant/detail")
	default:
		s.registerCompatibilityRoutes()
	}
}

// consoleAccessMiddleware 强制管理端业务接口在登录后访问。
// 登录、注销、token 查询、健康检查和契约查询保持公开，其余 operate/merchant 路由必须携带有效 token。
func (s *Server) consoleAccessMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.Name != contracts.ServiceConsole {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if isPublicConsolePath(path) {
			c.Next()
			return
		}
		if !isProtectedConsolePath(path) {
			c.Next()
			return
		}
		if s.cfg.Tenant.Mode == "single" {
			c.Next()
			return
		}
		if s.console == nil || s.console.Auth == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "管理端认证未启用"))
			c.Abort()
			return
		}
		token := requestToken(c.Request)
		if token == "" {
			c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "请先登录"))
			c.Abort()
			return
		}
		ticket, ok := s.console.Auth.Token(c.Request.Context(), token)
		if !ok {
			c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "token 无效或已过期"))
			c.Abort()
			return
		}

		// 联动实时校验账号的数据库状态（启用和删除状态）
		if s.console.Account != nil {
			accID, _ := strconv.Atoi(ticket.Tenant.UserID)
			if accID > 0 {
				acc, err := s.console.Account.Repository.GetByID(c.Request.Context(), accID)
				if err != nil || !acc.Enable {
					c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "账号已被停用或删除"))
					c.Abort()
					return
				}
			}
		}

		// 联动实时校验所属商户的数据库状态（启用、删除以及服务时限）
		if s.console.Merchant != nil && ticket.Tenant.MerchantID != "" {
			mchID, err := strconv.Atoi(strings.TrimSpace(ticket.Tenant.MerchantID))
			if err == nil && mchID > 0 {
				mch, err := s.console.Merchant.Repository.GetByID(c.Request.Context(), mchID)
				if err != nil || !mch.Enable {
					c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "商户主体已被停用或删除"))
					c.Abort()
					return
				}
				if mch.ExpiredTime != nil && mch.ExpiredTime.Before(time.Now().UTC()) {
					c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "商户服务已过期"))
					c.Abort()
					return
				}
			}
		}
		// 特殊豁免：授权指纹获取、状态拉取和证书上传替换在离线锁定下只要登录即可执行（不校验具体操作权限）
		// 但 tenant-mode 切换依然强绑定运营端内部管理员身份
		if strings.HasPrefix(path, "/operate/license") {
			if path == "/operate/license/tenant-mode" && !ticket.Tenant.Internal {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "没有权限修改系统架构模式"))
				c.Abort()
				return
			}
			c.Request = c.Request.WithContext(contracts.WithTenant(c.Request.Context(), ticket.Tenant))
			c.Next()
			return
		}

		required, found, err := s.requiredConsolePermission(c.Request.Context(), path, c.Request.Method)
		if err != nil {
			slog.Error("读取管理端路由权限失败", "path", path, "method", c.Request.Method, "error", err.Error())
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "读取权限配置失败"))
			c.Abort()
			return
		}
		if !found {
			c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "功能权限未配置"))
			c.Abort()
			return
		}
		if !ticket.Tenant.HasPermission(string(required)) {
			c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权限访问"))
			c.Abort()
			return
		}
		c.Request = c.Request.WithContext(contracts.WithTenant(c.Request.Context(), ticket.Tenant))
		c.Next()
	}
}

func (s *Server) requiredConsolePermission(ctx context.Context, path, method string) (contracts.PermissionCode, bool, error) {
	if s.console != nil && s.console.RoutePermissions != nil {
		permission, ok, err := s.console.RoutePermissions.RequiredPermissionForRequest(ctx, path, method)
		if err != nil {
			return "", false, err
		}
		if ok {
			return permission, true, nil
		}
	}
	permission, ok := contracts.RequiredPermissionForRequest(path, method)
	return permission, ok, nil
}

func isPublicConsolePath(path string) bool {
	return path == "/healthz" ||
		strings.HasPrefix(path, "/contracts/") ||
		strings.HasPrefix(path, "/operate/auth") ||
		strings.HasPrefix(path, "/merchant/auth") ||
		strings.HasPrefix(path, "/api/install")
}

func isProtectedConsolePath(path string) bool {
	return strings.HasPrefix(path, "/operate/") || strings.HasPrefix(path, "/merchant/")
}

// registerCompatibilityRoutes 为非 cc-call 服务注册  兼容路由占位端点。
// 这些路由在 Go 实现迁移前返回 501，便于前端和网关识别哪些接口已登记但待实现。
func (s *Server) registerCompatibilityRoutes(excludePrefixes ...string) {
	for _, route := range contracts.RoutesFor(s.Name) {
		prefix := route.PathPrefix
		if routePrefixExcluded(prefix, excludePrefixes) {
			continue
		}
		s.gin.Any(prefix, compatibilityHandler)
		s.gin.Any(prefix+"/*path", compatibilityHandler)
	}
}

func routePrefixExcluded(prefix string, excludes []string) bool {
	for _, excluded := range excludes {
		if prefix == excluded {
			return true
		}
	}
	return false
}

// requestMiddleware 是 HTTP 请求中间件，实现以下功能：
// 1. 注入 request ID 和 trace ID 到 context，用于链路追踪
// 2. 记录 HTTP 请求日志，包含方法、路径、状态码和耗时
// 3. 捕获 panic 并通过 gin.Recovery 统一处理
func (s *Server) requestMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		ctx := observability.WithRequestContext(c.Request.Context(), c.Request)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		slog.Info("HTTP 请求完成", logging.HTTPAttrs(
			c.Request.Method,
			c.Request.URL.Path,
			observability.Value(ctx, observability.RequestIDKey),
			observability.Value(ctx, observability.TraceIDKey),
			c.Writer.Status(),
			time.Since(start).String(),
		)...)
	}
}

// consoleTenantMiddleware 把管理端 token 中携带的租户上下文写回 request context。
// 这样后续管理端 handler 或服务层可以直接读取统一的 tenant 上下文，而不必重复解析 token。
func (s *Server) consoleTenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		injected := false
		if s.console != nil && s.console.Auth != nil {
			if token := requestToken(c.Request); token != "" {
				if ticket, ok := s.console.Auth.Token(c.Request.Context(), token); ok {
					c.Request = c.Request.WithContext(contracts.WithTenant(c.Request.Context(), ticket.Tenant))
					injected = true
				}
			}
		}
		if !injected && s.cfg.Tenant.Mode == "single" {
			defaultMerchantID := s.cfg.Tenant.DefaultMerchantID
			if defaultMerchantID <= 0 {
				defaultMerchantID = 1001
			}
			defaultTenant := contracts.TenantContext{
				MerchantID:  strconv.Itoa(defaultMerchantID),
				UserID:      "2001",
				RoleID:      "super_admin",
				Internal:    true,
				Permissions: []string{"*"},
			}
			c.Request = c.Request.WithContext(contracts.WithTenant(c.Request.Context(), defaultTenant))
		}
		c.Next()
	}
}

// handleHealth 返回服务健康检查响应。
// 用于 Kubernetes/负载均衡探活，返回服务名称、UP 状态和 UTC 时间戳。
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, contracts.OK(map[string]any{
		"service": s.Name,
		"status":  "UP",
		"time":    time.Now().UTC(),
	}))
}

// handleRoutes 返回当前服务的 HTTP 路由契约列表。
// 用于服务发现 and 契约验证，列出所有已注册的 API 端点及其路径前缀。
func (s *Server) handleRoutes(c *gin.Context) {
	c.JSON(http.StatusOK, contracts.OK(contracts.RoutesFor(s.Name)))
}

// handleRedis 返回当前服务的 Redis 契约信息。
// 列出服务使用的 Redis Key 模式、用途和数据结构类型。
func (s *Server) handleRedis(c *gin.Context) {
	c.JSON(http.StatusOK, contracts.OK(contracts.RedisContracts))
}

// handleMQ 返回当前服务的 MQ（消息队列）契约信息。
// 列出服务使用的 MQ 队列/主题名称、消息类型和消费者配置。
func (s *Server) handleMQ(c *gin.Context) {
	c.JSON(http.StatusOK, contracts.OK(contracts.MQContracts))
}

func requestToken(r *http.Request) string {
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-Token"))
}

func seedMemoryAccounts(repo *system.MemoryAccountRepository, logger *slog.Logger) {
	if repo == nil {
		return
	}
	for _, account := range authdomain.DefaultLoginAccounts() {
		accountType := operatedomain.AccountTypeMerchantAdmin
		dataScope := account.DataScope
		switch account.RoleID {
		case "super_admin":
			accountType = operatedomain.AccountTypeSuperAdmin
			dataScope = operatedomain.DataScopeGlobal
		case "operate_lead", "operate_staff":
			accountType = operatedomain.AccountTypeOperate
			dataScope = operatedomain.DataScopeGlobal
		case "merchant_admin":
			accountType = operatedomain.AccountTypeMerchantAdmin
			dataScope = operatedomain.DataScopeMerchant
		default:
			accountType = operatedomain.AccountTypeMerchantUser
			dataScope = operatedomain.DataScopeMerchant
		}
		id, _ := strconv.Atoi(account.UserID)
		if _, err := repo.Save(context.Background(), operatedomain.Account{
			ID:          id,
			Username:    account.Username,
			Password:    account.Password,
			MerchantID:  account.MerchantID,
			UserID:      account.UserID,
			RoleID:      account.RoleID,
			AccountType: accountType,
			DataScope:   dataScope,
			Enable:      true,
			CreatedBy:   "system",
			UpdatedBy:   "system",
		}); err != nil {
			logger.Error("控制台内存默认账号初始化失败", "username", account.Username, "error", err.Error())
		}
	}
	logger.Info("控制台内存默认账号已初始化", "accounts", "admin,operator,merchant")
}

// seedMemoryMerchants 用于在本地内存模式下填充默认的商户配置，现更正为 merchant 包的 MemoryMerchantRepository
func seedMemoryMerchants(merchantRepo *merchant.MemoryMerchantRepository, accountRepo *system.MemoryAccountRepository, logger *slog.Logger) {
	if merchantRepo == nil || accountRepo == nil {
		return
	}
	appKey, appSecret := operatedomain.GenerateAppKeyPair()
	merchant, err := merchantRepo.Save(context.Background(), operatedomain.Merchant{
		ID:               1001,
		Name:             "本地默认商户",
		Account:          "merchant",
		Enable:           true,
		RateID:           0,
		WhitelistDomains: "",
		AppKey:           appKey,
		AppSecret:        appSecret,
	})
	if err != nil {
		logger.Error("控制台内存默认商户初始化失败", "merchantId", 1001, "error", err.Error())
		return
	}
	accountRepo.SaveMerchantState(merchant)
	logger.Info("控制台内存默认商户已初始化", "merchantId", merchant.ID)
}

func seedDatabaseMerchant(gormDB *gorm.DB, logger *slog.Logger) {
	if gormDB == nil {
		return
	}
	appKey, appSecret := operatedomain.GenerateAppKeyPair()
	sipDomain := "sip.yunshu.local"
	maxAgents := 100
	var expiredTime *time.Time
	// 尝试读取已存在的凭证，避免覆盖已有配置
	var existing resource.MerchantModel
	if err := gormDB.Where("id = ?", 1001).First(&existing).Error; err == nil {
		if existing.AppKey != "" && existing.AppKey != "default_app_key" && existing.AppSecret != "" && existing.AppSecret != "default_app_secret" {
			appKey = existing.AppKey
			appSecret = existing.AppSecret
		}
		if existing.SipDomain != "" {
			sipDomain = existing.SipDomain
		}
		if existing.MaxAgents > 0 {
			maxAgents = existing.MaxAgents
		}
		expiredTime = existing.ExpiredTime
	}
	// 使用重构后的 merchant 包进行默认商户初始化
	repo := merchant.NewMerchantRepository(gormDB, nil, logger)
	merchant, err := repo.Save(context.Background(), operatedomain.Merchant{
		ID:               1001,
		Name:             "本地默认商户",
		Account:          "merchant",
		Enable:           true,
		WhitelistDomains: "",
		AppKey:           appKey,
		AppSecret:        appSecret,
		SipDomain:        sipDomain,
		MaxAgents:        maxAgents,
		ExpiredTime:      expiredTime,
	})
	if err != nil {
		logger.Error("数据库默认商户初始化失败", "merchantId", 1001, "error", err.Error())
		return
	}
	logger.Info("数据库默认商户已初始化", "merchantId", merchant.ID)

	// 初始化默认黑名单号码
	var count int64
	if err := gormDB.Model(&security.BlacklistDataModel{}).Count(&count).Error; err == nil && count == 0 {
		demoNums := []security.BlacklistDataModel{
			{Phone: "13888888888", BlackLevel: "LEVEL_1", Remark: "测试一级严重黑名单 (高危拦截)", CreatedTime: time.Now(), UpdatedTime: time.Now()},
			{Phone: "13999999999", BlackLevel: "LEVEL_2", Remark: "测试二级投诉黑名单 (中危拦截)", CreatedTime: time.Now(), UpdatedTime: time.Now()},
			{Phone: "13777777777", BlackLevel: "LEVEL_3", Remark: "测试三级一般黑名单 (低危拦截)", CreatedTime: time.Now(), UpdatedTime: time.Now()},
		}
		for _, num := range demoNums {
			if err := gormDB.Create(&num).Error; err != nil {
				logger.Error("数据库初始化默认黑名单号码失败", "phone", num.Phone, "error", err.Error())
			}
		}
		logger.Info("数据库默认黑名单号码已初始化")
	}

	// 初始化默认风控验证通道
	var channelCount int64
	if err := gormDB.Model(&security.BlacklistChannelModel{}).Count(&channelCount).Error; err == nil && channelCount == 0 {
		demoChannels := []security.BlacklistChannelModel{
			{Code: 1, Name: "东信易通黑名单", Vendor: "DONG_XIN", Remark: "系统默认东信易通强风控验证通道", Enable: true, CreatedTime: time.Now(), UpdatedTime: time.Now()},
			{Code: 2, Name: "羽乐黑名单", Vendor: "YU_LE", Remark: "系统默认羽乐科技防骚扰拦截通道", Enable: true, CreatedTime: time.Now(), UpdatedTime: time.Now()},
		}
		for _, ch := range demoChannels {
			if err := gormDB.Create(&ch).Error; err != nil {
				logger.Error("数据库初始化默认风控验证通道失败", "code", ch.Code, "error", err.Error())
			}
		}
		logger.Info("数据库默认风控验证通道已初始化完成")
	}
}

type kamailioRtpengineReloader struct {
	repo   operatedomain.ProxyConfigRepository
	logger *slog.Logger
}

func (r *kamailioRtpengineReloader) ReloadRtpengine(ctx context.Context) error {
	wsPort := 5066
	if r.repo != nil {
		if item, err := r.repo.Get(ctx, operatedomain.KeyKamailioWsPort); err == nil {
			if port, err := strconv.Atoi(item.Value); err == nil && port > 0 {
				wsPort = port
			}
		}
	}

	url1 := fmt.Sprintf("http://127.0.0.1:%d/rtpengine/reload", wsPort)
	r.logger.Info("开始触发 Kamailio RTPEngine 热重载", "url", url1)

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url1, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			r.logger.Info("Kamailio RTPEngine 热重载成功", "status", resp.Status)
			return nil
		}
		r.logger.Warn("Kamailio RTPEngine 热重载接口返回非 200 状态码", "status", resp.Status)
	} else {
		r.logger.Warn("尝试连接 localhost 失败，尝试容器内网络重试", "error", err.Error())
	}

	// 尝试通过 Docker 容器服务名重试
	url2 := "http://kamailio:5066/rtpengine/reload"
	r.logger.Info("尝试使用 Docker 容器服务名重载", "url", url2)
	req2, err := http.NewRequestWithContext(ctx, "GET", url2, nil)
	if err != nil {
		return err
	}
	resp2, err := client.Do(req2)
	if err == nil {
		defer resp2.Body.Close()
		if resp2.StatusCode == http.StatusOK {
			r.logger.Info("Kamailio RTPEngine 容器内网络热重载成功", "status", resp2.Status)
			return nil
		}
		return fmt.Errorf("kamailio reload returned status %s", resp2.Status)
	}

	return fmt.Errorf("failed to reload rtpengine: %w", err)
}

// compatibilityHandler 是  兼容路由的占位处理器。
// 返回 501 状态码 and 提示信息，表明该接口已登记但 Go 实现待迁移。
func compatibilityHandler(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, contracts.Fail(contracts.CodeInternal, "接口已登记，Go 实现待迁移"))
}

// env 读取环境变量，若未设置则返回提供的默认值。
// 用于支持通过环境变量覆盖配置文件中的默认值（如监听地址）。
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
