// Package config 负责加载进程配置。
//
// 配置层只返回普通结构体，让服务装配层自由组合 Gin、GORM、Redis、RabbitMQ 等官方
// adapter，同时避免配置解析逻辑进入领域包。
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 顶层配置结构，包含服务发现、日志、数据库、缓存、消息队列、FreeSWITCH 和 AI/RAG 相关配置。
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
	AI         AIConfig         `yaml:"ai"`
}

// AIConfig 定义 AI 相关配置，包括 RAG、嵌入、大语言模型等。
type AIConfig struct {
	// Enabled 是否启用 AI 功能
	Enabled bool `yaml:"enabled"`
	// Embedder 嵌入模型配置
	Embedder EmbedderConfig `yaml:"embedder"`
	// VectorDB 向量数据库配置
	VectorDB VectorDBConfig `yaml:"vectorDB"`
	// RAG 检索增强生成配置
	RAG RAGConfig `yaml:"rag"`
}

// EmbedderConfig 嵌入模型配置
type EmbedderConfig struct {
	// Provider 嵌入服务提供商: "openai", "deepseek"
	Provider string `yaml:"provider"`
	// APIKey API 密钥
	APIKey string `yaml:"apiKey"`
	// Endpoint API 端点（可选，默认使用官方地址）
	Endpoint string `yaml:"endpoint"`
	// Model 嵌入模型名称
	Model string `yaml:"model"`
}

// VectorDBConfig 向量数据库配置
type VectorDBConfig struct {
	// Type 向量数据库类型: "memory", "qdrant"
	Type string `yaml:"type"`
	// Address 向量数据库地址（Qdrant 等）
	Address string `yaml:"address"`
	// Collection 集合/索引名称
	Collection string `yaml:"collection"`
}

// RAGConfig 检索增强生成配置
type RAGConfig struct {
	// TopK 返回最相关的 K 个文档
	TopK int `yaml:"topK"`
	// ScoreThreshold 相似度阈值（0-1，低于这个值的不会被使用）
	ScoreThreshold float64 `yaml:"scoreThreshold"`
	// MaxTokens 上下文最大 token 数
	MaxTokens int `yaml:"maxTokens"`
}

// ServiceConfig 定义服务自身的基本信息，包括服务名称和监听地址。
type ServiceConfig struct {
	Name       string `yaml:"name"`
	Addr       string `yaml:"addr"`
	InstanceID string `yaml:"instanceId"`
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
	CallBaseURL string              `yaml:"callBaseURL"`
	Dialpad     DialpadUpdateConfig `yaml:"dialpad"`
	LicensePath string              `yaml:"licensePath"`
	Perm        ConsolePermConfig   `yaml:"perm"`
}

// ConsolePermConfig 定义管理端权限缓存参数。
type ConsolePermConfig struct {
	// RouteCacheTTL 路由权限规则在 Redis 中的缓存 TTL，默认 10 分钟。
	RouteCacheTTL time.Duration `yaml:"routeCacheTTL"`
	// LocalCacheTTL 单进程内本地内存缓存的 TTL，默认 5 秒。减少对 Redis 的跳次。
	LocalCacheTTL time.Duration `yaml:"localCacheTTL"`
}

// DialpadUpdateConfig 定义桌面拨号盘客户端更新配置。
type DialpadUpdateConfig struct {
	Version     string              `yaml:"version"`
	ForceUpdate bool                `yaml:"forceUpdate"`
	Changelog   string              `yaml:"changelog"`
	RustFS      DialpadRustFSConfig `yaml:"rustfs"`
}

// DialpadRustFSConfig 定义 S3 兼容的 RustFS 存储连接信息。
type DialpadRustFSConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
	Bucket    string `yaml:"bucket"`
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
	OSS     OSSConfig     `yaml:"oss"`
}

// OSSConfig 定义 S3 兼容对象存储的录音上传参数。
// 支持 MinIO、RustFS、阿里云 OSS 等 S3 兼容接口。
type OSSConfig struct {
	// Endpoint S3 服务地址，如 http://minio:9000。为空时跳过 OSS 上传。
	Endpoint string `yaml:"endpoint"`
	// AccessKey S3 Access Key ID。
	AccessKey string `yaml:"accessKey"`
	// SecretKey S3 Secret Access Key。
	SecretKey string `yaml:"secretKey"`
	// Bucket 录音文件所在 bucket 名称，默认 recordings。
	Bucket string `yaml:"bucket"`
	// BaseDir FreeSWITCH 录音文件在本地文件系统的挂载根目录。
	// Worker 读取录音文件时以此为根路径拼接相对路径。
	BaseDir string `yaml:"baseDir"`
	// CDNBaseURL OSS 对象的公网访问前缀，如 https://cdn.example.com/recordings。
	// 上传成功后 record_url 将设为 CDNBaseURL + "/" + objectKey。
	CDNBaseURL string `yaml:"cdnBaseURL"`
}

// WorkerBillingConfig 定义 worker 计费估算参数。
type WorkerBillingConfig struct {
	DefaultRatePerMin float64 `yaml:"defaultRatePerMin"`
}

// FreeSwitchConfig 定义 FreeSWITCH ESL 连接和事件租约相关配置。
type FreeSwitchConfig struct {
	EventLeaseTTL  time.Duration   `yaml:"eventLeaseTTL"`
	CommandTimeout time.Duration   `yaml:"commandTimeout"`
	KamailioAddr   string          `yaml:"kamailioAddr"`   // Kamailio SIP 代理地址
	KamailioWSPort int             `yaml:"kamailioWSPort"` // Kamailio WebSocket 端口
	SipDomain      string          `yaml:"sipDomain"`      // 默认 SIP 域名
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
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate 检查配置合法性，在加载和环境变量覆盖之后调用。
func (c *Config) Validate() error {
	if c.Service.Addr == "" {
		return fmt.Errorf("config: service.addr is required")
	}
	if c.MySQL.DSN == "" {
		return fmt.Errorf("config: mysql.dsn is required")
	}
	if len(c.Redis.Addrs) == 0 {
		return fmt.Errorf("config: redis.addrs is required")
	}
	if c.FreeSwitch.EventLeaseTTL > 0 && c.FreeSwitch.EventLeaseTTL < time.Second {
		return fmt.Errorf("config: freeswitch.eventLeaseTTL must be at least 1s")
	}
	if c.AI.Enabled {
		if c.AI.Embedder.Provider != "" && c.AI.Embedder.APIKey == "" {
			return fmt.Errorf("config: ai.embedder.apiKey is required when embedder provider is set")
		}
	}
	return nil
}

// applyEnv 应用环境变量覆盖 YAML 配置中的对应字段。
// 环境变量优先级高于配置文件，用于容器化部署时的动态配置注入。
func applyEnv(cfg *Config) {
	if value := os.Getenv("ADDR"); value != "" {
		cfg.Service.Addr = value
	}
	if value := os.Getenv("SERVICE_INSTANCE_ID"); value != "" {
		cfg.Service.InstanceID = value
	} else if value := os.Getenv("CC_CALL_INSTANCE_ID"); value != "" {
		cfg.Service.InstanceID = value
	} else if value := os.Getenv("POD_NAME"); value != "" {
		cfg.Service.InstanceID = value
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
	if value := os.Getenv("LICENSE_PATH"); value != "" {
		cfg.Console.LicensePath = value
	}
	if value := os.Getenv("DIALPAD_VERSION"); value != "" {
		cfg.Console.Dialpad.Version = value
	}
	if value := os.Getenv("DIALPAD_FORCE_UPDATE"); value == "true" {
		cfg.Console.Dialpad.ForceUpdate = true
	}
	if value := os.Getenv("DIALPAD_CHANGELOG"); value != "" {
		cfg.Console.Dialpad.Changelog = value
	}
	if value := os.Getenv("DIALPAD_RUSTFS_ENDPOINT"); value != "" {
		cfg.Console.Dialpad.RustFS.Endpoint = value
	}
	if value := os.Getenv("DIALPAD_RUSTFS_ACCESS_KEY"); value != "" {
		cfg.Console.Dialpad.RustFS.AccessKey = value
	}
	if value := os.Getenv("DIALPAD_RUSTFS_SECRET_KEY"); value != "" {
		cfg.Console.Dialpad.RustFS.SecretKey = value
	}
	if value := os.Getenv("DIALPAD_RUSTFS_BUCKET"); value != "" {
		cfg.Console.Dialpad.RustFS.Bucket = value
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
	if value := os.Getenv("RECORDING_OSS_ENDPOINT"); value != "" {
		cfg.Worker.Recording.OSS.Endpoint = value
	}
	if value := os.Getenv("RECORDING_OSS_ACCESS_KEY"); value != "" {
		cfg.Worker.Recording.OSS.AccessKey = value
	}
	if value := os.Getenv("RECORDING_OSS_SECRET_KEY"); value != "" {
		cfg.Worker.Recording.OSS.SecretKey = value
	}
	if value := os.Getenv("RECORDING_OSS_BUCKET"); value != "" {
		cfg.Worker.Recording.OSS.Bucket = value
	}
	if value := os.Getenv("RECORDING_LOCAL_BASE_DIR"); value != "" {
		cfg.Worker.Recording.OSS.BaseDir = value
	}
	if value := os.Getenv("RECORDING_CDN_BASE_URL"); value != "" {
		cfg.Worker.Recording.OSS.CDNBaseURL = value
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
	// AI 配置
	if value := os.Getenv("AI_ENABLED"); value == "true" {
		cfg.AI.Enabled = true
	}
	if value := os.Getenv("AI_EMBEDDER_PROVIDER"); value != "" {
		cfg.AI.Embedder.Provider = value
	}
	if value := os.Getenv("AI_EMBEDDER_API_KEY"); value != "" {
		cfg.AI.Embedder.APIKey = value
	}
	if value := os.Getenv("AI_EMBEDDER_ENDPOINT"); value != "" {
		cfg.AI.Embedder.Endpoint = value
	}
	if value := os.Getenv("AI_EMBEDDER_MODEL"); value != "" {
		cfg.AI.Embedder.Model = value
	}
	if value := os.Getenv("AI_VECTORDB_TYPE"); value != "" {
		cfg.AI.VectorDB.Type = value
	}
	if value := os.Getenv("AI_VECTORDB_ADDRESS"); value != "" {
		cfg.AI.VectorDB.Address = value
	}
	if value := os.Getenv("AI_VECTORDB_COLLECTION"); value != "" {
		cfg.AI.VectorDB.Collection = value
	}
	if value := os.Getenv("AI_RAG_TOP_K"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			cfg.AI.RAG.TopK = parsed
		}
	}
	if value := os.Getenv("AI_RAG_SCORE_THRESHOLD"); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			cfg.AI.RAG.ScoreThreshold = parsed
		}
	}
	if value := os.Getenv("AI_RAG_MAX_TOKENS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			cfg.AI.RAG.MaxTokens = parsed
		}
	}
}
