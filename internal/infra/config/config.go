// Package config 负责加载进程配置。
//
// 配置层只返回普通结构体，让服务装配层自由组合 Gin、GORM、Redis、RabbitMQ 等官方
// adapter，同时避免配置解析逻辑进入领域包。
package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 顶层配置结构，包含服务发现、日志、数据库、缓存、消息队列和 FreeSWITCH 相关配置。
// 各子配置可通过环境变量覆盖，用于容器部署和密钥注入。
type Config struct {
	Service    ServiceConfig    `yaml:"service"`
	Logging    LoggingConfig    `yaml:"logging"`
	MySQL      MySQLConfig      `yaml:"mysql"`
	Redis      RedisConfig      `yaml:"redis"`
	RabbitMQ   RabbitMQConfig   `yaml:"rabbitmq"`
	FreeSwitch FreeSwitchConfig `yaml:"freeswitch"`
	Console    ConsoleConfig    `yaml:"console"`
	Worker     WorkerConfig     `yaml:"worker"`
	Tenant     TenantConfig     `yaml:"tenant"`
}

// ServiceConfig 定义服务自身的基本信息，包括服务名称和监听地址。
type ServiceConfig struct {
	Name string `yaml:"name"`
	Addr string `yaml:"addr"`
}

// LoggingConfig 定义日志级别和输出格式，格式支持 json 和 text。
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// MySQLConfig 定义 MySQL 连接参数，包含 DSN、连接池大小 and 连接生命周期。
type MySQLConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxIdleConns    int           `yaml:"maxIdleConns"`
	MaxOpenConns    int           `yaml:"maxOpenConns"`
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime"`
}

// RedisConfig 定义 Redis 连接参数，支持多地址（集群模式）和 Stream 消费配置。
type RedisConfig struct {
	Addrs        []string          `yaml:"addrs"`
	DB           int               `yaml:"db"`
	Stream       RedisStreamConfig `yaml:"stream"`
	ReadTimeout  time.Duration     `yaml:"readTimeout"`
	WriteTimeout time.Duration     `yaml:"writeTimeout"`
}

// RedisStreamConfig 定义 Redis Stream 消费者组的配置参数。
// 用于事件总线消费，包含流名称、消费组、消费者实例名、拉取间隔和批量大小。
type RedisStreamConfig struct {
	Enabled        bool          `yaml:"enabled"`
	Stream         string        `yaml:"stream"`
	Group          string        `yaml:"group"`
	Consumer       string        `yaml:"consumer"`
	Block          time.Duration `yaml:"block"`
	BatchSize      int64         `yaml:"batchSize"`
	ClaimMinIdle   time.Duration `yaml:"claimMinIdle"`
	StartFromFirst bool          `yaml:"startFromFirst"`
}

// RabbitMQConfig 定义 RabbitMQ 连接 URL，用于消息队列发布和消费。
type RabbitMQConfig struct {
	URL string `yaml:"url"`
}

// ConsoleConfig 定义管理端调用其他内部服务的配置。
type ConsoleConfig struct {
	CallBaseURL string `yaml:"callBaseURL"`
}

// WorkerConfig 定义后台 worker 流程节点配置。
type WorkerConfig struct {
	Outbox     WorkerOutboxConfig     `yaml:"outbox"`
	Callback   WorkerCallbackConfig   `yaml:"callback"`
	Downstream WorkerDownstreamConfig `yaml:"downstream"`
	Recording  WorkerRecordingConfig  `yaml:"recording"`
	Billing    WorkerBillingConfig    `yaml:"billing"`
}

// WorkerOutboxConfig 定义 outbox 投递循环参数。
type WorkerOutboxConfig struct {
	Interval   time.Duration `yaml:"interval"`
	BatchSize  int           `yaml:"batchSize"`
	RetryDelay time.Duration `yaml:"retryDelay"`
	Lease      time.Duration `yaml:"lease"`
	WorkerID   string        `yaml:"workerId"`
}

// WorkerCallbackConfig 定义 worker 对外客户回调投递参数。
type WorkerCallbackConfig struct {
	URL     string        `yaml:"url"`
	Secret  string        `yaml:"secret"`
	Timeout time.Duration `yaml:"timeout"`
}

// WorkerDownstreamConfig 定义 CDR 下游 HTTP 投递参数。
type WorkerDownstreamConfig struct {
	URL     string        `yaml:"url"`
	Secret  string        `yaml:"secret"`
	Timeout time.Duration `yaml:"timeout"`
}

// WorkerRecordingConfig 定义录音上传 HTTP 投递参数。
type WorkerRecordingConfig struct {
	URL     string        `yaml:"url"`
	Secret  string        `yaml:"secret"`
	Timeout time.Duration `yaml:"timeout"`
}

// WorkerBillingConfig 定义 worker 计费估算参数。
type WorkerBillingConfig struct {
	DefaultRatePerMin float64 `yaml:"defaultRatePerMin"`
}

// FreeSwitchConfig 定义 FreeSWITCH ESL 连接和事件租约相关配置。
type FreeSwitchConfig struct {
	EventLeaseTTL  time.Duration   `yaml:"eventLeaseTTL"`
	CommandTimeout time.Duration   `yaml:"commandTimeout"`
	Nodes          []FSNodeConfig  `yaml:"nodes"`
	Reconnect      ReconnectConfig `yaml:"reconnect"`
}

// FSNodeConfig 定义单个 FreeSWITCH 节点的连接参数。
// ID 为数据库主键，Addr 为 ESL 地址，Password 为 ESL 认证密码，SetID 用于分组，Weight 影响负载均衡。
type FSNodeConfig struct {
	ID       int    `yaml:"id"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	SetID    int    `yaml:"setId"`
	Weight   int    `yaml:"weight"`
	CmdPort  int    `yaml:"cmdPort"`
	Enabled  bool   `yaml:"enabled"`
}

// ReconnectConfig 定义 FreeSWITCH ESL 断线重连策略。
type ReconnectConfig struct {
	Interval    time.Duration `yaml:"interval"`
	MaxAttempts int           `yaml:"maxAttempts"`
}

// TenantConfig 定义商户租户运作模式配置。
type TenantConfig struct {
	Mode              string `yaml:"mode"`              // "single" | "multi"
	DefaultMerchantID int    `yaml:"defaultMerchantId"` // 默认商户ID
}

// Load 从 YAML 文件加载配置，并应用环境变量覆盖。
// 环境变量用于容器部署和密钥注入，提交到仓库的 YAML 只能保存安全默认值。
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	applyEnv(&cfg)
	return cfg, nil
}

// applyEnv 应用环境变量覆盖 YAML 配置中的对应字段。
// 环境变量优先级高于配置文件，用于容器化部署时的动态配置注入。
func applyEnv(cfg *Config) {
	if value := os.Getenv("ADDR"); value != "" {
		cfg.Service.Addr = value
	}
	if value := os.Getenv("MYSQL_DSN"); value != "" {
		cfg.MySQL.DSN = value
	}
	if value := os.Getenv("RABBITMQ_URL"); value != "" {
		cfg.RabbitMQ.URL = value
	}
	if value := os.Getenv("CC_CALL_BASE_URL"); value != "" {
		cfg.Console.CallBaseURL = value
	}
	if value := os.Getenv("REDIS_STREAM_ENABLED"); value == "true" {
		cfg.Redis.Stream.Enabled = true
	}
	if value := os.Getenv("REDIS_STREAM_CONSUMER"); value != "" {
		cfg.Redis.Stream.Consumer = value
	}
	if value := os.Getenv("WORKER_ID"); value != "" {
		cfg.Worker.Outbox.WorkerID = value
	}
	if value := os.Getenv("CALLBACK_URL"); value != "" {
		cfg.Worker.Callback.URL = value
	}
	if value := os.Getenv("CALLBACK_SECRET"); value != "" {
		cfg.Worker.Callback.Secret = value
	}
	if value := os.Getenv("DOWNSTREAM_CDR_URL"); value != "" {
		cfg.Worker.Downstream.URL = value
	}
	if value := os.Getenv("DOWNSTREAM_CDR_SECRET"); value != "" {
		cfg.Worker.Downstream.Secret = value
	}
	if value := os.Getenv("RECORDING_UPLOAD_URL"); value != "" {
		cfg.Worker.Recording.URL = value
	}
	if value := os.Getenv("RECORDING_UPLOAD_SECRET"); value != "" {
		cfg.Worker.Recording.Secret = value
	}
	if value := os.Getenv("WORKER_BILLING_DEFAULT_RATE_PER_MIN"); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			cfg.Worker.Billing.DefaultRatePerMin = parsed
		}
	}
	if value := os.Getenv("TENANT_MODE"); value != "" {
		cfg.Tenant.Mode = value
	}
	if value := os.Getenv("TENANT_DEFAULT_MERCHANT_ID"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			cfg.Tenant.DefaultMerchantID = parsed
		}
	}
}
