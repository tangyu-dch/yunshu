package operate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	// ErrInvalidBlacklist 表示黑名单参数不合法。
	ErrInvalidBlacklist = errors.New("invalid blacklist")
	// ErrBlacklistNotFound 表示黑名单不存在。
	ErrBlacklistNotFound = errors.New("blacklist not found")
	// ErrBlacklistConflict 表示黑名单名称冲突。
	ErrBlacklistConflict = errors.New("blacklist conflict")
)

const (
	// BlacklistVerificationChannelDongXin 对齐  东信易通黑名单渠道。
	BlacklistVerificationChannelDongXin = 1
	// BlacklistVerificationChannelYuLe 对齐  羽乐黑名单渠道。
	BlacklistVerificationChannelYuLe = 2
)

var blacklistVerificationChannelNames = map[int]string{
	BlacklistVerificationChannelDongXin: "东信易通黑名单",
	BlacklistVerificationChannelYuLe:    "羽乐黑名单",
}

// BlacklistChannel 表示风控风控防骚扰或投诉的三方黑名单验证通道
type BlacklistChannel struct {
	Code            int    `json:"code"`
	Name            string `json:"name"`
	Vendor          string `json:"vendor"`
	Remark          string `json:"remark"`
	Enable          bool   `json:"enable"`
	APIUrl          string `json:"apiUrl"`
	AppID           string `json:"appId"`
	AppSecret       string `json:"appSecret"`
	ReqTemplate     string `json:"reqTemplate"`
	RespExtractPath string `json:"respExtractPath"`
	RespMatchValue  string `json:"respMatchValue"`
	TimeoutMs       int    `json:"timeoutMs"`
}

// BlacklistValidator 定义三方外部黑名单验证接口，用于提供后续定制开发的强扩展能力
type BlacklistValidator interface {
	Validate(ctx context.Context, phone string) (bool, error)
}

// defaultMockValidator 默认风控验证通道模拟实现
type defaultMockValidator struct {
	vendor string
}

func (d *defaultMockValidator) Validate(ctx context.Context, phone string) (bool, error) {
	slog.Info("执行默认外部黑名单风控通道模拟验证", "vendor", d.vendor, "phone", phone)
	return false, nil
}

// DynamicHTTPValidator 动态通用三方风控 HTTP 验证执行引擎
type DynamicHTTPValidator struct {
	Channel BlacklistChannel
}

// Validate 执行实际的第三方 HTTP 接口风控调用，支持完整的请求格式匹配、结果路径提取与布尔值对比
func (v *DynamicHTTPValidator) Validate(ctx context.Context, phone string) (bool, error) {
	if v.Channel.APIUrl == "" {
		return false, nil
	}

	// 1. 优先从内存缓存中获取防骚扰风控校验结果，防止并发爆表
	if hit, ok := getPhoneVerificationCache(v.Channel.Code, phone); ok {
		slog.Debug("三方风控号码校验命中内存热缓存", "channel", v.Channel.Code, "phone", phone, "hit", hit)
		return hit, nil
	}

	// 2. 占位符替换，动态构建请求体 payload
	bodyStr := v.Channel.ReqTemplate
	if bodyStr == "" {
		bodyStr = `{"phone":"{phone}"}`
	}
	bodyStr = strings.ReplaceAll(bodyStr, "{phone}", phone)
	bodyStr = strings.ReplaceAll(bodyStr, "{app_id}", v.Channel.AppID)
	bodyStr = strings.ReplaceAll(bodyStr, "{app_secret}", v.Channel.AppSecret)

	timeout := time.Duration(v.Channel.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}

	client := &http.Client{
		Timeout: timeout,
	}

	slog.Info("开始向外部三方通道发起风控校验请求", "url", v.Channel.APIUrl, "payload", bodyStr)
	req, err := http.NewRequestWithContext(ctx, "POST", v.Channel.APIUrl, strings.NewReader(bodyStr))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("外部三方风控通道连接或超时异常", "url", v.Channel.APIUrl, "error", err.Error())
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("外部三方风控通道返回非200异常状态码", "url", v.Channel.APIUrl, "status", resp.Status)
		return false, fmt.Errorf("http status code %d", resp.StatusCode)
	}

	var respData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		slog.Error("外部三方风控通道响应反序列化失败", "url", v.Channel.APIUrl, "error", err.Error())
		return false, err
	}

	// 3. 按配置的 RespExtractPath (如 data.hit) 提取目标结果进行比对
	extractedVal := extractJSONPath(respData, v.Channel.RespExtractPath)
	extractedStr := strings.TrimSpace(fmt.Sprintf("%v", extractedVal))
	expectedStr := strings.TrimSpace(v.Channel.RespMatchValue)

	hit := strings.EqualFold(extractedStr, expectedStr)
	slog.Info("外部三方风控通道校验执行完成", "phone", phone, "extracted", extractedStr, "expected", expectedStr, "hit", hit)

	// 4. 将结果持久化进内存缓存（TTL = 5分钟）防止同一批次号码多次调用接口
	setPhoneVerificationCache(v.Channel.Code, phone, hit)

	return hit, nil
}

// extractJSONPath 基于 '.' 拆分简单提取多层嵌套 map 中的值
func extractJSONPath(data map[string]any, path string) any {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	var current any = data
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if m, ok := current.(map[string]any); ok {
			current = m[part]
		} else {
			return nil
		}
	}
	return current
}

// PhoneVerificationCache 线程安全的风控号码拦截二级缓存
var phoneVerificationCache = struct {
	sync.RWMutex
	items map[string]struct {
		hit       bool
		expiresAt time.Time
	}
}{
	items: make(map[string]struct {
		hit       bool
		expiresAt time.Time
	}),
}

func getPhoneVerificationCache(channelCode int, phone string) (bool, bool) {
	phoneVerificationCache.RLock()
	defer phoneVerificationCache.RUnlock()
	key := fmt.Sprintf("%d:%s", channelCode, phone)
	entry, ok := phoneVerificationCache.items[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return false, false
	}
	return entry.hit, true
}

func setPhoneVerificationCache(channelCode int, phone string, hit bool) {
	phoneVerificationCache.Lock()
	defer phoneVerificationCache.Unlock()
	key := fmt.Sprintf("%d:%s", channelCode, phone)
	phoneVerificationCache.items[key] = struct {
		hit       bool
		expiresAt time.Time
	}{
		hit:       hit,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
}

// ChannelCache 线程安全的三方通道快速缓存，避免在Originate呼叫热路径上查询数据库
type ChannelCache struct {
	mu    sync.RWMutex
	items map[int]BlacklistChannel
}

var globalChannelCache = &ChannelCache{
	items: make(map[int]BlacklistChannel),
}

// Set 更新单条缓存
func (c *ChannelCache) Set(code int, ch BlacklistChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[code] = ch
}

// Delete 清理单条缓存
func (c *ChannelCache) Delete(code int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, code)
}

// Get 获取单条缓存，提供极快的读取速度
func (c *ChannelCache) Get(code int) (BlacklistChannel, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ch, ok := c.items[code]
	return ch, ok
}

// LoadAll 批量重新加载缓存
func (c *ChannelCache) LoadAll(channels []BlacklistChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[int]BlacklistChannel)
	for _, ch := range channels {
		c.items[ch.Code] = ch
	}
}

// GetChannelFromCache 获取全局风控验证通道缓存数据
func GetChannelFromCache(code int) (BlacklistChannel, bool) {
	return globalChannelCache.Get(code)
}

// validatorRegistry 用于保存所有外部或自建定制开发的黑名单风控验证引擎
var validatorRegistry = struct {
	sync.RWMutex
	validators map[string]BlacklistValidator
}{
	validators: make(map[string]BlacklistValidator),
}

// RegisterValidator 允许外部插件或定制开发者注册黑名单验证逻辑
func RegisterValidator(vendor string, validator BlacklistValidator) {
	validatorRegistry.Lock()
	defer validatorRegistry.Unlock()
	validatorRegistry.validators[vendor] = validator
	slog.Info("成功注册自定义外部黑名单风控验证引擎", "vendor", vendor)
}

// GetValidator 获取特定厂商的黑名单风控验证逻辑
func GetValidator(vendor string) (BlacklistValidator, bool) {
	validatorRegistry.RLock()
	defer validatorRegistry.RUnlock()
	v, ok := validatorRegistry.validators[vendor]
	return v, ok
}

func init() {
	// 初始化注册默认三方通道
	RegisterValidator("DONG_XIN", &defaultMockValidator{vendor: "DONG_XIN"})
	RegisterValidator("YU_LE", &defaultMockValidator{vendor: "YU_LE"})
}

// Blacklist表示  兼容 `blacklist` 与 `blacklist_gateway` 组合的运营配置。
type Blacklist struct {
	ID                      int    `json:"id,omitempty"`
	Name                    string `json:"name"`
	VerificationChannel     int    `json:"verificationChannel"`
	VerificationChannelName string `json:"verificationChannelName,omitempty"`
	GatewayIDs              []int  `json:"gatewayIds,omitempty"`
	Gateways                string `json:"gateways,omitempty"`
	Remark                  string `json:"remark,omitempty"`
}

// BlacklistNumber 表示黑名单中的具体手机号及拦截级别。
type BlacklistNumber struct {
	Phone      string `json:"phone"`
	BlackLevel string `json:"blackLevel"`
	Remark     string `json:"remark"`
}

// BlacklistPageRequest 表示黑名单分页查询条件。
type BlacklistPageRequest struct {
	PageNumber           int    `json:"pageNumber"`
	PageSize             int    `json:"pageSize"`
	Name                 string `json:"name,omitempty"`
	VerificationChannels []int  `json:"verificationChannels,omitempty"`
	Gateways             []int  `json:"gateways,omitempty"`
}

// BlacklistNumberPageRequest 表示黑名单号码的分页查询条件。
type BlacklistNumberPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Phone      string `json:"phone,omitempty"`
	BlackLevel string `json:"blackLevel,omitempty"`
}

// BlacklistPageResult 表示黑名单分页结果。
type BlacklistPageResult struct {
	PageNumber int         `json:"pageNumber"`
	PageSize   int         `json:"pageSize"`
	Total      int64       `json:"total"`
	Records    []Blacklist `json:"records"`
}

// BlacklistNumberPageResult 表示黑名单号码的分页查询结果。
type BlacklistNumberPageResult struct {
	PageNumber int               `json:"pageNumber"`
	PageSize   int               `json:"pageSize"`
	Total      int64             `json:"total"`
	Records    []BlacklistNumber `json:"records"`
}

// BlacklistRepository 定义黑名单管理仓储能力。
type BlacklistRepository interface {
	Page(ctx context.Context, req BlacklistPageRequest) (BlacklistPageResult, error)
	GetByID(ctx context.Context, id int) (Blacklist, error)
	ExistsName(ctx context.Context, name string, excludeID int) (bool, error)
	Save(ctx context.Context, blacklist Blacklist) (Blacklist, error)
	Delete(ctx context.Context, id int) error

	// 黑名单具体号码管理
	PageNumbers(ctx context.Context, req BlacklistNumberPageRequest) (BlacklistNumberPageResult, error)
	SaveNumber(ctx context.Context, num BlacklistNumber) (BlacklistNumber, error)
	DeleteNumbers(ctx context.Context, phones []string) error

	// 三方验证通道动态管理能力
	ListChannels(ctx context.Context) ([]BlacklistChannel, error)
	SaveChannel(ctx context.Context, channel BlacklistChannel) error
	DeleteChannel(ctx context.Context, code int) error
}

// BlacklistManagementService 承载运营端黑名单管理业务。
type BlacklistManagementService struct {
	Repository BlacklistRepository
	Logger     *slog.Logger
}

// Page 分页查询黑名单。
func (s *BlacklistManagementService) Page(ctx context.Context, req BlacklistPageRequest) (BlacklistPageResult, error) {
	logger := s.logger()
	req = normalizeBlacklistPage(req)
	logger.Info("运营端开始分页查询黑名单", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name, "verificationChannelCount", len(req.VerificationChannels), "gatewayCount", len(req.Gateways))
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询黑名单失败", "error", err.Error())
		return BlacklistPageResult{}, err
	}
	logger.Info("运营端分页查询黑名单完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// PageNumbers 分页查询具体黑名单号码。
func (s *BlacklistManagementService) PageNumbers(ctx context.Context, req BlacklistNumberPageRequest) (BlacklistNumberPageResult, error) {
	logger := s.logger()
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	logger.Info("运营端开始分页查询黑名单号码", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "phone", req.Phone, "blackLevel", req.BlackLevel)
	page, err := s.Repository.PageNumbers(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询黑名单号码失败", "error", err.Error())
		return BlacklistNumberPageResult{}, err
	}
	logger.Info("运营端分页查询黑名单号码完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// SaveNumber 保存（新增或更新）具体黑名单号码。
func (s *BlacklistManagementService) SaveNumber(ctx context.Context, num BlacklistNumber) (BlacklistNumber, error) {
	logger := s.logger()
	num.Phone = strings.TrimSpace(num.Phone)
	num.BlackLevel = strings.TrimSpace(num.BlackLevel)
	if num.Phone == "" {
		return BlacklistNumber{}, errors.New("手机号不能为空")
	}
	if num.BlackLevel == "" {
		num.BlackLevel = "LEVEL_1"
	}
	logger.Info("运营端开始保存黑名单号码", "phone", num.Phone, "blackLevel", num.BlackLevel)
	saved, err := s.Repository.SaveNumber(ctx, num)
	if err != nil {
		logger.Error("运营端保存黑名单号码失败", "phone", num.Phone, "error", err.Error())
		return BlacklistNumber{}, err
	}
	logger.Info("运营端保存黑名单号码成功", "phone", saved.Phone)
	return saved, nil
}

// DeleteNumbers 批量删除具体黑名单号码。
func (s *BlacklistManagementService) DeleteNumbers(ctx context.Context, phones []string) error {
	logger := s.logger()
	if len(phones) == 0 {
		return nil
	}
	logger.Info("运营端开始批量删除黑名单号码", "count", len(phones))
	if err := s.Repository.DeleteNumbers(ctx, phones); err != nil {
		logger.Error("运营端批量删除黑名单号码失败", "error", err.Error())
		return err
	}
	logger.Info("运营端批量删除黑名单号码成功", "count", len(phones))
	return nil
}

// Save 新增或更新黑名单。
func (s *BlacklistManagementService) Save(ctx context.Context, blacklist Blacklist) (Blacklist, error) {
	logger := s.logger()
	blacklist.Name = strings.TrimSpace(blacklist.Name)
	blacklist.Remark = strings.TrimSpace(blacklist.Remark)
	if blacklist.Name == "" {
		logger.Warn("运营端保存黑名单参数无效：库名称为空", "id", blacklist.ID)
		return Blacklist{}, ErrInvalidBlacklist
	}

	// 动态从数据库/仓储检索所有注册的三方通道，实现动态校验
	channels, err := s.Repository.ListChannels(ctx)
	if err != nil {
		logger.Error("运营端保存黑名单时动态查询三方通道失败", "error", err.Error())
		return Blacklist{}, err
	}

	var matchedChannel *BlacklistChannel
	for i := range channels {
		if channels[i].Code == blacklist.VerificationChannel {
			matchedChannel = &channels[i]
			break
		}
	}

	if matchedChannel == nil {
		logger.Warn("运营端保存黑名单失败：通道代码不存在于动态配置中", "channel", blacklist.VerificationChannel)
		return Blacklist{}, ErrInvalidBlacklist
	}
	if !matchedChannel.Enable {
		logger.Warn("运营端保存黑名单失败：动态配置中该通道已停用", "channel", blacklist.VerificationChannel)
		return Blacklist{}, ErrInvalidBlacklist
	}

	blacklist.VerificationChannelName = matchedChannel.Name

	exists, err := s.Repository.ExistsName(ctx, blacklist.Name, blacklist.ID)
	if err != nil {
		logger.Error("运营端校验黑名单唯一性失败", "id", blacklist.ID, "name", blacklist.Name, "error", err.Error())
		return Blacklist{}, err
	}
	if exists {
		logger.Warn("运营端保存黑名单冲突", "id", blacklist.ID, "name", blacklist.Name)
		return Blacklist{}, ErrBlacklistConflict
	}
	logger.Info("运营端开始保存黑名单", "id", blacklist.ID, "name", blacklist.Name, "verificationChannel", blacklist.VerificationChannel, "gatewayCount", len(blacklist.GatewayIDs))
	saved, err := s.Repository.Save(ctx, blacklist)
	if err != nil {
		logger.Error("运营端保存黑名单失败", "id", blacklist.ID, "name", blacklist.Name, "error", err.Error())
		return Blacklist{}, err
	}
	logger.Info("运营端保存黑名单完成", "id", saved.ID, "name", saved.Name, "verificationChannel", saved.VerificationChannel, "gatewayCount", len(saved.GatewayIDs))
	return saved, nil
}

// Delete 删除黑名单。
func (s *BlacklistManagementService) Delete(ctx context.Context, id int) error {
	logger := s.logger()
	if id <= 0 {
		return ErrInvalidBlacklist
	}
	logger.Info("运营端开始删除黑名单", "id", id)
	if err := s.Repository.Delete(ctx, id); err != nil {
		logger.Error("运营端删除黑名单失败", "id", id, "error", err.Error())
		return err
	}
	logger.Info("运营端删除黑名单完成", "id", id)
	return nil
}

// ListChannels 查询所有三方风控验证通道。
func (s *BlacklistManagementService) ListChannels(ctx context.Context) ([]BlacklistChannel, error) {
	s.logger().Info("运营端开始查询动态三方验证通道列表")
	return s.Repository.ListChannels(ctx)
}

// SaveChannel 保存或修改三方风控验证通道。
func (s *BlacklistManagementService) SaveChannel(ctx context.Context, channel BlacklistChannel) error {
	s.logger().Info("运营端开始保存动态三方验证通道", "code", channel.Code, "name", channel.Name, "vendor", channel.Vendor)
	channel.Name = strings.TrimSpace(channel.Name)
	channel.Vendor = strings.TrimSpace(channel.Vendor)
	if channel.Code <= 0 {
		return errors.New("通道唯一识别码必须大于0")
	}
	if channel.Name == "" {
		return errors.New("通道名称不能为空")
	}
	if channel.Vendor == "" {
		return errors.New("技术标识 Vendor ID 不能为空")
	}
	if err := s.Repository.SaveChannel(ctx, channel); err != nil {
		return err
	}
	globalChannelCache.Set(channel.Code, channel)
	s.logger().Info("动态刷新三方验证通道配置缓存成功", "code", channel.Code)
	return nil
}

// DeleteChannel 删除三方风控验证通道。
func (s *BlacklistManagementService) DeleteChannel(ctx context.Context, code int) error {
	s.logger().Info("运营端开始删除动态三方验证通道", "code", code)
	if code <= 0 {
		return errors.New("通道唯一识别码错误")
	}
	if err := s.Repository.DeleteChannel(ctx, code); err != nil {
		return err
	}
	globalChannelCache.Delete(code)
	s.logger().Info("动态从缓存中移除三方验证通道成功", "code", code)
	return nil
}

// LoadAllChannelsToCache 批量加载通道至缓存
func LoadAllChannelsToCache(channels []BlacklistChannel) {
	globalChannelCache.LoadAll(channels)
}

func (s *BlacklistManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeBlacklistPage(req BlacklistPageRequest) BlacklistPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}
