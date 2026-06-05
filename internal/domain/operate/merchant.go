package operate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"yunshu/internal/contracts"
)

var (
	// ErrInvalidMerchant 表示运营端提交的商户配置缺少生产必需字段。
	ErrInvalidMerchant = errors.New("invalid merchant")
	// ErrMerchantNotFound 表示请求的商户不存在或已逻辑删除。
	ErrMerchantNotFound = errors.New("merchant not found")
	// ErrMerchantConflict 表示商户名称或账号与现有未删除记录冲突。
	ErrMerchantConflict = errors.New("merchant conflict")
)

var (
	domainPattern = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	ipv4Pattern   = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
)

// Merchant 表示  兼容 `merchant` 表中的商户主体。
//
// 管理端通过该结构维护商户的启停和有效期。外呼准入仍通过数据库读这张表。
type Merchant struct {
	ID               int        `json:"id,omitempty"`
	Name             string     `json:"name"`
	Account          string     `json:"account"`
	ExpiredTime      *time.Time `json:"expiredTime,omitempty"`
	RateID           int        `json:"rateId,omitempty"`
	WhitelistDomains string     `json:"whitelistDomains,omitempty"`
	SipDomain        string     `json:"sipDomain,omitempty"`
	Enable           bool       `json:"enable"`
	AppKey           string     `json:"appKey,omitempty"`
	AppSecret        string     `json:"appSecret,omitempty"`
	MaxAgents        int        `json:"maxAgents"`
}

// MerchantPageRequest 表示商户分页查询条件。
type MerchantPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
	Account    string `json:"account,omitempty"`
	Enable     *bool  `json:"enable,omitempty"`
}

// MerchantPageResult 是分页查询结果。
type MerchantPageResult struct {
	PageNumber int        `json:"pageNumber"`
	PageSize   int        `json:"pageSize"`
	Total      int64      `json:"total"`
	Records    []Merchant `json:"records"`
}

// MerchantMutationResult 描述商户写入后的刷新语义。
type MerchantMutationResult struct {
	Merchant Merchant `json:"merchant,omitempty"`
}

// MerchantRepository 定义商户管理仓储能力。
type MerchantRepository interface {
	Page(ctx context.Context, req MerchantPageRequest) (MerchantPageResult, error)
	GetByID(ctx context.Context, id int) (Merchant, error)
	ExistsNameOrAccount(ctx context.Context, name, account string, excludeID int) (bool, error)
	RateExists(ctx context.Context, rateID int) (bool, error)
	Save(ctx context.Context, merchant Merchant) (Merchant, error)
	Delete(ctx context.Context, ids []int) error
}

// MerchantManagementService 承载商户管理业务。
type MerchantManagementService struct {
	Repository    MerchantRepository
	ExtensionRepo ExtensionManagementRepository
	Cache         AuthCacheInvalidator
	Logger        *slog.Logger
}

// Page 返回商户分页结果。
func (s *MerchantManagementService) Page(ctx context.Context, req MerchantPageRequest) (MerchantPageResult, error) {
	logger := s.logger()
	req = normalizeMerchantPage(req)

	tenant, _ := contracts.TenantFromContext(ctx)
	if !tenant.Internal && tenant.MerchantID != "" {
		var mID int
		if parsed, err := strconv.Atoi(tenant.MerchantID); err == nil {
			mID = parsed
		}
		logger.Info("商户端查询限制：只返回当前商户数据", "merchantId", mID)
		merchant, err := s.Repository.GetByID(ctx, mID)
		if err != nil {
			logger.Warn("商户查询自身数据但商户不存在", "merchantId", mID, "error", err.Error())
			return MerchantPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: 0, Records: []Merchant{}}, nil
		}
		return MerchantPageResult{
			PageNumber: req.PageNumber,
			PageSize:   req.PageSize,
			Total:      1,
			Records:    []Merchant{merchant},
		}, nil
	}

	logger.Info("运营端开始分页查询商户", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name, "account", req.Account)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询商户失败", "error", err.Error())
		return MerchantPageResult{}, err
	}
	logger.Info("运营端分页查询商户完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Save 新增或更新商户。
func (s *MerchantManagementService) Save(ctx context.Context, merchant Merchant) (MerchantMutationResult, error) {
	logger := s.logger()
	normalized, err := normalizeMerchantForSave(merchant)
	if err != nil {
		logger.Warn("运营端保存商户参数无效", "id", merchant.ID, "name", merchant.Name, "error", err.Error())
		return MerchantMutationResult{}, err
	}

	var oldSipDomain string
	var sipDomainChanged bool
	if normalized.ID > 0 {
		oldMch, err := s.Repository.GetByID(ctx, normalized.ID)
		if err == nil {
			oldSipDomain = oldMch.SipDomain
			if oldSipDomain != normalized.SipDomain {
				sipDomainChanged = true
			}
		}
	}

	exists, err := s.Repository.ExistsNameOrAccount(ctx, normalized.Name, normalized.Account, normalized.ID)
	if err != nil {
		logger.Error("运营端校验商户唯一性失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return MerchantMutationResult{}, err
	}
	if exists {
		logger.Warn("运营端保存商户冲突", "id", normalized.ID, "name", normalized.Name, "account", normalized.Account)
		return MerchantMutationResult{}, ErrMerchantConflict
	}
	if normalized.RateID > 0 {
		rateExists, err := s.Repository.RateExists(ctx, normalized.RateID)
		if err != nil {
			logger.Error("运营端校验商户费率失败", "id", normalized.ID, "rateId", normalized.RateID, "error", err.Error())
			return MerchantMutationResult{}, err
		}
		if !rateExists {
			logger.Warn("运营端保存商户失败，费率不存在", "id", normalized.ID, "rateId", normalized.RateID)
			return MerchantMutationResult{}, ErrRateNotFound
		}
	}
	action := "create"
	if normalized.ID > 0 {
		action = "update"
	}
	logger.Info("运营端开始保存商户", "id", normalized.ID, "name", normalized.Name, "account", normalized.Account, "action", action, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存商户失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return MerchantMutationResult{}, err
	}
	logger.Info("运营端保存商户完成", "id", saved.ID, "name", saved.Name, "enable", saved.Enable)

	// 如果 SIP 域名发生变更，并且配置了 ExtensionRepo，我们需要级联更新该商户下所有分机的 SipDomain 以及重新计算 HA1 和 HA1b
	if sipDomainChanged && s.ExtensionRepo != nil {
		logger.Info("商户 SIP 域名变更，开始级联更新下属所有分机的 SipDomain 与 HA 哈希", "merchantId", saved.ID, "oldSipDomain", oldSipDomain, "newSipDomain", saved.SipDomain)
		pageNum := 1
		const batchSize = 100
		updatedCount := 0
		for {
			page, err := s.ExtensionRepo.Page(ctx, ExtensionPageRequest{
				PageNumber: pageNum,
				PageSize:   batchSize,
				MerchantID: saved.ID,
			})
			if err != nil {
				logger.Error("商户 SIP 域名级联更新分机读取失败", "merchantId", saved.ID, "page", pageNum, "error", err.Error())
				break
			}
			if len(page.Records) == 0 {
				break
			}

			for _, ext := range page.Records {
				ext.SipDomain = saved.SipDomain
				if ext.Password != "" {
					ext.HA1 = calculateHA1(ext.ExtensionNumber, ext.SipDomain, ext.Password)
					ext.HA1b = calculateHA1b(ext.ExtensionNumber, ext.SipDomain, ext.Password)
				}
				if _, err := s.ExtensionRepo.Save(ctx, ext); err != nil {
					logger.Error("商户 SIP 域名级联更新分机保存失败", "id", ext.ID, "extension", ext.ExtensionNumber, "error", err.Error())
				} else {
					updatedCount++
				}
			}

			if int64(pageNum*batchSize) >= page.Total {
				break
			}
			pageNum++
		}
		logger.Info("商户 SIP 域名级联更新分机完成", "merchantId", saved.ID, "updatedCount", updatedCount)

		// 清理 Kamailio 鉴权缓存
		if s.Cache != nil {
			if err := s.Cache.InvalidateAuthCache(ctx); err != nil {
				logger.Error("商户 SIP 域名级联更新分机清理 Kamailio auth 缓存失败", "error", err.Error())
			}
		}
	}

	return MerchantMutationResult{Merchant: saved}, nil
}

// Delete 逻辑删除商户。
func (s *MerchantManagementService) Delete(ctx context.Context, merchants []Merchant) (MerchantMutationResult, error) {
	logger := s.logger()
	ids := make([]int, 0, len(merchants))
	for _, merchant := range merchants {
		if merchant.ID > 0 {
			ids = append(ids, merchant.ID)
		}
	}
	if len(ids) == 0 {
		return MerchantMutationResult{}, ErrInvalidMerchant
	}
	logger.Info("运营端开始删除商户", "merchantCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端删除商户失败", "merchantCount", len(ids), "error", err.Error())
		return MerchantMutationResult{}, err
	}
	logger.Info("运营端删除商户完成", "merchantCount", len(ids))
	return MerchantMutationResult{}, nil
}

// Enable 切换商户启用状态。
func (s *MerchantManagementService) Enable(ctx context.Context, id int, enable bool) (MerchantMutationResult, error) {
	logger := s.logger()
	logger.Info("运营端开始切换商户启用状态", "id", id, "enable", enable)
	merchant, err := s.Repository.GetByID(ctx, id)
	if err != nil {
		logger.Warn("运营端切换商户启用状态失败，商户不存在", "id", id, "error", err.Error())
		return MerchantMutationResult{}, err
	}
	merchant.Enable = enable
	saved, err := s.Repository.Save(ctx, merchant)
	if err != nil {
		logger.Error("运营端切换商户启用状态失败", "id", id, "error", err.Error())
		return MerchantMutationResult{}, err
	}
	logger.Info("运营端切换商户启用状态完成", "id", saved.ID, "name", saved.Name, "enable", saved.Enable)
	return MerchantMutationResult{Merchant: saved}, nil
}

// ResetAPIKeys 重新生成并持久化商户的 AppKey 与 AppSecret
func (s *MerchantManagementService) ResetAPIKeys(ctx context.Context, id int) (MerchantMutationResult, error) {
	logger := s.logger()
	logger.Info("商户端开始重置 API 密钥对", "id", id)

	tenant, _ := contracts.TenantFromContext(ctx)
	if !tenant.Internal && tenant.MerchantID != "" {
		mID, err := strconv.Atoi(tenant.MerchantID)
		if err != nil || mID != id {
			logger.Warn("非法的越权密钥重置请求", "requestMchId", id, "tokenMchId", tenant.MerchantID)
			return MerchantMutationResult{}, errors.New("forbidden")
		}
	}

	merchant, err := s.Repository.GetByID(ctx, id)
	if err != nil {
		logger.Warn("密钥重置失败，商户不存在", "id", id, "error", err.Error())
		return MerchantMutationResult{}, err
	}

	merchant.AppKey, merchant.AppSecret = GenerateAppKeyPair()
	saved, err := s.Repository.Save(ctx, merchant)
	if err != nil {
		logger.Error("密钥重置并持久化失败", "id", id, "error", err.Error())
		return MerchantMutationResult{}, err
	}

	logger.Info("密钥重置并持久化完成", "id", saved.ID, "appKey", saved.AppKey)
	return MerchantMutationResult{Merchant: saved}, nil
}

func (s *MerchantManagementService) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeMerchantPage(req MerchantPageRequest) MerchantPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Account = strings.TrimSpace(req.Account)
	return req
}

func normalizeMerchantForSave(merchant Merchant) (Merchant, error) {
	merchant.Name = strings.TrimSpace(merchant.Name)
	merchant.Account = strings.TrimSpace(merchant.Account)
	normalizedWhitelistDomains, err := normalizeWhitelistDomains(merchant.WhitelistDomains)
	if err != nil {
		return Merchant{}, ErrInvalidMerchant
	}
	merchant.WhitelistDomains = normalizedWhitelistDomains
	if merchant.Name == "" || merchant.Account == "" {
		return Merchant{}, ErrInvalidMerchant
	}
	if merchant.RateID < 0 {
		return Merchant{}, ErrInvalidMerchant
	}
	if merchant.AppKey == "" {
		merchant.AppKey, merchant.AppSecret = GenerateAppKeyPair()
	}
	return merchant, nil
}

func normalizeWhitelistDomains(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	domains := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if !isValidWhitelistDomain(trimmed) {
			return "", ErrInvalidMerchant
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		domains = append(domains, trimmed)
	}
	return strings.Join(domains, ","), nil
}

func isValidWhitelistDomain(value string) bool {
	if domainPattern.MatchString(value) {
		return true
	}
	if host, port, ok := strings.Cut(value, ":"); ok {
		if !validPort(port) {
			return false
		}
		return domainPattern.MatchString(host) || ipv4Pattern.MatchString(host)
	}
	return ipv4Pattern.MatchString(value) || domainPattern.MatchString(value)
}

func validPort(port string) bool {
	if port == "" {
		return false
	}
	for _, ch := range port {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	if len(port) > 5 {
		return false
	}
	return true
}

// GenerateAppKeyPair 随机生成用于商户 API 对接的 AppKey 和 AppSecret
func GenerateAppKeyPair() (string, string) {
	var keyBuf [16]byte
	var secBuf [24]byte
	_, _ = rand.Read(keyBuf[:])
	_, _ = rand.Read(secBuf[:])
	return "ak_" + hex.EncodeToString(keyBuf[:]), "as_" + hex.EncodeToString(secBuf[:])
}
