package operate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var (
	ErrInvalidIPBlock = errors.New("无效的 IP 拦截记录")
)

const (
	KeyBlockedCountries      = "ipblock.blocked_countries"
	RedisKeyBlockedCountries = "ipblock:blocked_countries"
	KeyOnlyAllowCN           = "ipblock.only_allow_cn"
	RedisKeyOnlyAllowCN      = "ipblock:only_allow_cn"
)

// IPBlockLog 描述被内核防火墙（iptables/ipset）拦截的境外 IP 审计流水记录。
type IPBlockLog struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	IP          string    `gorm:"size:50;not null;index" json:"ip"`    // 拦截的源 IP 地址
	CountryCode string    `gorm:"size:10;not null" json:"countryCode"` // 拦截的源 IP 所属国家/地区代码（如 "US", "DE"）
	CallID      string    `gorm:"size:100" json:"callId"`              // 匹配到的 SIP 呼叫 Call-ID (可选)
	Method      string    `gorm:"size:20" json:"method"`               // 匹配到的 SIP 方法/协议 (可选，例如 "REGISTER" 或 "INVITE")
	BlockedAt   time.Time `gorm:"not null" json:"blockedAt"`           // 拦截发生的时间
}

// IPBlockLogPageRequest 拦截流水日志分页查询请求
type IPBlockLogPageRequest struct {
	PageNumber  int       `json:"pageNumber"`
	PageSize    int       `json:"pageSize"`
	IP          string    `json:"ip,omitempty"`          // 源 IP 过滤条件
	CountryCode string    `json:"countryCode,omitempty"` // 国家代码过滤条件
	StartTime   time.Time `json:"startTime,omitempty"`   // 开始时间
	EndTime     time.Time `json:"endTime,omitempty"`     // 结束时间
}

// IPBlockLogPageResult 拦截流水日志分页查询响应
type IPBlockLogPageResult struct {
	PageNumber int          `json:"pageNumber"`
	PageSize   int          `json:"pageSize"`
	Total      int64        `json:"total"`
	Records    []IPBlockLog `json:"records"`
}

// IPBlockLogRepository 描述拦截日志在基础设施持久化层的 CRUD 契约
type IPBlockLogRepository interface {
	Page(ctx context.Context, req IPBlockLogPageRequest) (IPBlockLogPageResult, error)
	Save(ctx context.Context, log IPBlockLog) (IPBlockLog, error)
}

// ConfigRepository 描述系统基础配置读取与修改接口的简单子集
type ConfigRepository interface {
	Get(ctx context.Context, key string) (ProxyConfigItem, error)
	Set(ctx context.Context, key, value, desc string) error
}

// IPBlockManagementService 运营管理平台防盗打/IP 地理拦截管理服务。
// 统一协调黑名单配置的持久化、以及内核日志所触发的拦截审计入库逻辑。
type IPBlockManagementService struct {
	LogRepo     IPBlockLogRepository
	ConfigRepo  ConfigRepository
	RedisClient *goredis.Client
	Logger      *slog.Logger
}

func NewIPBlockManagementService(logRepo IPBlockLogRepository, configRepo ConfigRepository, redisClient *goredis.Client, logger *slog.Logger) *IPBlockManagementService {
	return &IPBlockManagementService{
		LogRepo:     logRepo,
		ConfigRepo:  configRepo,
		RedisClient: redisClient,
		Logger:      logger,
	}
}

// Page 分页查询已被 iptables 拦截丢弃的外部扫描记录流水
func (s *IPBlockManagementService) Page(ctx context.Context, req IPBlockLogPageRequest) (IPBlockLogPageResult, error) {
	logger := s.logger()
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.IP = strings.TrimSpace(req.IP)
	req.CountryCode = strings.TrimSpace(req.CountryCode)

	logger.Info("开始分页查询 IP 拦截流水日志", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "ip", req.IP, "countryCode", req.CountryCode)
	res, err := s.LogRepo.Page(ctx, req)
	if err != nil {
		logger.Error("分页查询 IP 拦截流水日志失败", "error", err.Error())
		return IPBlockLogPageResult{}, err
	}
	return res, nil
}

// LogBlockEvent 记录一次 IP 拦截事件审计流水 (通常由内核日志抓取进程调用)
func (s *IPBlockManagementService) LogBlockEvent(ctx context.Context, ip, countryCode, callID, method string) (IPBlockLog, error) {
	logger := s.logger()
	ip = strings.TrimSpace(ip)
	countryCode = strings.TrimSpace(countryCode)
	if ip == "" || countryCode == "" {
		logger.Warn("写入 IP 拦截日志失败：参数不全", "ip", ip, "countryCode", countryCode)
		return IPBlockLog{}, ErrInvalidIPBlock
	}

	blockedLog := IPBlockLog{
		IP:          ip,
		CountryCode: strings.ToUpper(countryCode),
		CallID:      strings.TrimSpace(callID),
		Method:      strings.TrimSpace(method),
		BlockedAt:   time.Now(),
	}

	logger.Info("开始写入内核 IP 拦截审计日志", "ip", ip, "countryCode", countryCode, "method", method)
	saved, err := s.LogRepo.Save(ctx, blockedLog)
	if err != nil {
		logger.Error("写入内核 IP 拦截审计日志失败", "ip", ip, "error", err.Error())
		return IPBlockLog{}, err
	}
	return saved, nil
}

// GetBlockedCountries 读取当前黑名单国家列表 (以逗号分隔，如 "US,DE")
func (s *IPBlockManagementService) GetBlockedCountries(ctx context.Context) (string, error) {
	if s.ConfigRepo == nil {
		return "", nil
	}
	item, err := s.ConfigRepo.Get(ctx, KeyBlockedCountries)
	if err != nil {
		if errors.Is(err, ErrConfigNotFound) {
			return "", nil
		}
		return "", err
	}
	return item.Value, nil
}

// SaveBlockedCountries 保存拦截国家配置并同步至 Redis 缓存
func (s *IPBlockManagementService) SaveBlockedCountries(ctx context.Context, countries string) error {
	logger := s.logger()
	if s.ConfigRepo == nil {
		return errors.New("配置存储未初始化")
	}

	countries = strings.TrimSpace(countries)
	parts := strings.Split(countries, ",")
	var cleaned []string
	for _, p := range parts {
		trimmed := strings.ToUpper(strings.TrimSpace(p))
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	val := strings.Join(cleaned, ",")

	logger.Info("保存被拦截的国家列表配置", "countries", val)
	err := s.ConfigRepo.Set(ctx, KeyBlockedCountries, val, "被拦截的外部 IP 国家代码列表 (逗号分隔，如 US,DE)")
	if err != nil {
		logger.Error("保存被拦截的国家列表配置失败", "error", err.Error())
		return err
	}

	// 同步到 Redis 触发 ipset/iptables 守护进程动态加载
	if s.RedisClient != nil {
		if err := s.RedisClient.Set(ctx, RedisKeyBlockedCountries, val, 0).Err(); err != nil {
			logger.Error("同步被拦截国家列表至 Redis 失败", "error", err.Error())
		} else {
			logger.Info("同步被拦截国家列表至 Redis 成功", "value", val)
		}
	}
	return nil
}

// GetOnlyAllowCN 读取是否仅放行中国大陆 IP
func (s *IPBlockManagementService) GetOnlyAllowCN(ctx context.Context) (bool, error) {
	if s.ConfigRepo == nil {
		return false, nil
	}
	item, err := s.ConfigRepo.Get(ctx, KeyOnlyAllowCN)
	if err != nil {
		if errors.Is(err, ErrConfigNotFound) {
			return false, nil
		}
		return false, err
	}
	return item.Value == "true", nil
}

// SaveOnlyAllowCN 保存是否仅放行中国大陆 IP 的开关并同步至 Redis 缓存
func (s *IPBlockManagementService) SaveOnlyAllowCN(ctx context.Context, onlyAllow bool) error {
	logger := s.logger()
	if s.ConfigRepo == nil {
		return errors.New("配置存储未初始化")
	}

	val := "false"
	if onlyAllow {
		val = "true"
	}

	logger.Info("保存仅放行中国大陆 IP 状态", "onlyAllowCn", val)
	err := s.ConfigRepo.Set(ctx, KeyOnlyAllowCN, val, "是否仅允许中国大陆的 IP 访问 (true/false)")
	if err != nil {
		logger.Error("保存仅放行中国大陆 IP 状态失败", "error", err.Error())
		return err
	}

	if s.RedisClient != nil {
		if err := s.RedisClient.Set(ctx, RedisKeyOnlyAllowCN, val, 0).Err(); err != nil {
			logger.Error("同步仅放行中国大陆 IP 状态至 Redis 失败", "error", err.Error())
		} else {
			logger.Info("同步仅放行中国大陆 IP 状态至 Redis 成功", "value", val)
		}
	}
	return nil
}

// SyncToRedis 在系统启动时将数据库中的配置刷新到 Redis 共享缓存
func (s *IPBlockManagementService) SyncToRedis(ctx context.Context) error {
	logger := s.logger()
	if s.RedisClient == nil {
		return nil
	}
	val, err := s.GetBlockedCountries(ctx)
	if err != nil {
		return err
	}
	if err := s.RedisClient.Set(ctx, RedisKeyBlockedCountries, val, 0).Err(); err != nil {
		logger.Error("系统启动同步被拦截国家至 Redis 失败", "error", err.Error())
		return err
	}

	onlyAllow, err := s.GetOnlyAllowCN(ctx)
	if err != nil {
		return err
	}
	onlyAllowVal := "false"
	if onlyAllow {
		onlyAllowVal = "true"
	}
	if err := s.RedisClient.Set(ctx, RedisKeyOnlyAllowCN, onlyAllowVal, 0).Err(); err != nil {
		logger.Error("系统启动同步仅放行中国大陆 IP 状态至 Redis 失败", "error", err.Error())
		return err
	}

	logger.Info("系统启动同步仅放行中国大陆 IP 状态及国家拦截列表至 Redis 成功", "value", val, "onlyAllowCn", onlyAllowVal)
	return nil
}

func (s *IPBlockManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

type ipLookupEntry struct {
	ipNet *net.IPNet
	cc    string
}

var (
	lookupCache []ipLookupEntry
	lookupOnce  sync.Once
	lookupMu    sync.RWMutex
)

// LookupIP 查询 IP 归属国家/地区代码
func (s *IPBlockManagementService) LookupIP(ctx context.Context, ipStr string) (string, error) {
	logger := s.logger()
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return "", errors.New("无效的 IP 地址")
	}

	var initErr error
	lookupOnce.Do(func() {
		logger.Info("开始首次懒加载全局国家 IP 地理位置索引...")
		start := time.Now()
		files, err := filepath.Glob("configs/ipblocks/*.zone")
		if err != nil {
			initErr = err
			return
		}

		var tempCache []ipLookupEntry
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			cc := strings.ToUpper(strings.TrimSuffix(filepath.Base(file), ".zone"))
			scanner := bufio.NewScanner(bytes.NewReader(data))
			for scanner.Scan() {
				cidr := strings.TrimSpace(scanner.Text())
				if cidr == "" || strings.HasPrefix(cidr, "#") {
					continue
				}
				_, ipNet, err := net.ParseCIDR(cidr)
				if err != nil {
					continue
				}
				tempCache = append(tempCache, ipLookupEntry{
					ipNet: ipNet,
					cc:    cc,
				})
			}
		}

		lookupMu.Lock()
		lookupCache = tempCache
		lookupMu.Unlock()
		logger.Info("全局国家 IP 地理位置索引加载完成", "count", len(lookupCache), "elapsed", time.Since(start).String())
	})

	if initErr != nil {
		return "", initErr
	}

	lookupMu.RLock()
	defer lookupMu.RUnlock()

	for _, entry := range lookupCache {
		if entry.ipNet.Contains(ip) {
			return entry.cc, nil
		}
	}

	return "UNKNOWN", nil
}
