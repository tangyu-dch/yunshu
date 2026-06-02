package operate

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidExtension 表示分机参数无效。
	ErrInvalidExtension = errors.New("invalid extension")
	// ErrExtensionNotFound 表示分机不存在。
	ErrExtensionNotFound = errors.New("extension not found")
	// ErrExtensionConflict 表示分机号已存在。
	ErrExtensionConflict = errors.New("extension conflict")
)

// 绑定类型常量。
const (
	BindTypeManual  = 1 // 手动绑定（离线不自动释放）
	BindTypeDynamic = 2 // 动态绑定（离线自动解绑）
)

// Extension 表示  兼容 `cc_res_extension` 表中的分机配置。
type Extension struct {
	ID              int    `json:"id,omitempty"`
	ExtensionNumber string `json:"extensionNumber"`
	Password        string `json:"password,omitempty"`
	MerchantID      int    `json:"merchantId"`
	UserID          int    `json:"userId"`
	Enable          bool   `json:"enable"`
	BindType        int    `json:"bindType"`
	SipDomain       string `json:"sipDomain,omitempty"`
	HA1             string `json:"ha1,omitempty"`
	HA1b            string `json:"ha1b,omitempty"`
}

type ExtensionPageRequest struct {
	PageNumber      int    `json:"pageNumber"`
	PageSize        int    `json:"pageSize"`
	ExtensionNumber string `json:"extensionNumber,omitempty"`
	MerchantID      int    `json:"merchantId,omitempty"`
	UserID          int    `json:"userId,omitempty"`
	Enable          *bool  `json:"enable,omitempty"`
}

type ExtensionPageResult struct {
	PageNumber int         `json:"pageNumber"`
	PageSize   int         `json:"pageSize"`
	Total      int64       `json:"total"`
	Records    []Extension `json:"records"`
}

type ExtensionManagementRepository interface {
	Page(ctx context.Context, req ExtensionPageRequest) (ExtensionPageResult, error)
	GetByID(ctx context.Context, id int) (Extension, error)
	ExistsNumber(ctx context.Context, extensionNumber string, merchantID int, excludeID int) (bool, error)
	Save(ctx context.Context, extension Extension) (Extension, error)
	Delete(ctx context.Context, ids []int) error
	SetEnable(ctx context.Context, id int, enable bool) (Extension, error)
	DynamicBind(ctx context.Context, extensionNumber string, userID int, merchantID int) error
}

type AuthCacheInvalidator interface {
	InvalidateAuthCache(ctx context.Context) error
}

type ExtensionManagementService struct {
	Repository   ExtensionManagementRepository
	MerchantRepo MerchantRepository
	Cache        AuthCacheInvalidator
	Logger       *slog.Logger
}

func (s *ExtensionManagementService) DynamicBind(ctx context.Context, extensionNumber string, userID int, merchantID int) error {
	logger := s.logger()
	logger.Info("开始动态绑定分机", "extension", extensionNumber, "userId", userID, "merchantId", merchantID)
	err := s.Repository.DynamicBind(ctx, extensionNumber, userID, merchantID)
	if err != nil {
		logger.Warn("动态绑定分机失败", "extension", extensionNumber, "userId", userID, "error", err.Error())
		return err
	}
	logger.Info("动态绑定分机成功", "extension", extensionNumber, "userId", userID)
	return nil
}

func (s *ExtensionManagementService) Page(ctx context.Context, req ExtensionPageRequest) (ExtensionPageResult, error) {
	logger := s.logger()
	req = normalizeExtensionPage(req)
	logger.Info("运营端开始分页查询分机", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "extension", req.ExtensionNumber, "merchantId", req.MerchantID, "userId", req.UserID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询分机失败", "error", err.Error())
		return ExtensionPageResult{}, err
	}
	logger.Info("运营端分页查询分机完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}


func (s *ExtensionManagementService) Save(ctx context.Context, extension Extension) (Extension, error) {
	logger := s.logger()
	normalized, err := normalizeExtensionForSave(extension)
	if err != nil {
		logger.Warn("运营端保存分机参数无效", "id", extension.ID, "extension", extension.ExtensionNumber, "error", err.Error())
		return Extension{}, err
	}
	exists, err := s.Repository.ExistsNumber(ctx, normalized.ExtensionNumber, normalized.MerchantID, normalized.ID)
	if err != nil {
		logger.Error("运营端校验分机唯一性失败", "id", normalized.ID, "extension", normalized.ExtensionNumber, "error", err.Error())
		return Extension{}, err
	}
	if exists {
		logger.Warn("运营端保存分机冲突", "id", normalized.ID, "extension", normalized.ExtensionNumber)
		return Extension{}, ErrExtensionConflict
	}

	// 动态解析租户域 (SipDomain) 并重新计算 HA1 和 HA1b 哈希密码
	if s.MerchantRepo != nil {
		mch, err := s.MerchantRepo.GetByID(ctx, normalized.MerchantID)
		if err == nil && mch.SipDomain != "" {
			normalized.SipDomain = mch.SipDomain
		}
	}
	if normalized.SipDomain == "" {
		normalized.SipDomain = "sip.yunshu.local" // 默认域名退避
	}
	if normalized.Password != "" {
		normalized.HA1 = calculateHA1(normalized.ExtensionNumber, normalized.SipDomain, normalized.Password)
		normalized.HA1b = calculateHA1b(normalized.ExtensionNumber, normalized.SipDomain, normalized.Password)
	}

	logger.Info("运营端开始保存分机", "id", normalized.ID, "extension", normalized.ExtensionNumber, "merchantId", normalized.MerchantID, "userId", normalized.UserID, "sipDomain", normalized.SipDomain, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存分机失败", "id", normalized.ID, "extension", normalized.ExtensionNumber, "error", err.Error())
		return Extension{}, err
	}
	if s.Cache != nil {
		if err := s.Cache.InvalidateAuthCache(ctx); err != nil {
			logger.Error("保存分机清理 Kamailio auth 缓存失败", "error", err.Error())
		}
	}
	logger.Info("运营端保存分机完成", "id", saved.ID, "extension", saved.ExtensionNumber, "enable", saved.Enable)
	return saved, nil
}

func (s *ExtensionManagementService) Delete(ctx context.Context, extensions []Extension) error {
	logger := s.logger()
	ids := filterPositiveExtensionIDs(extensions)
	if len(ids) == 0 {
		return ErrInvalidExtension
	}
	logger.Info("运营端开始删除分机", "extensionCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端删除分机失败", "extensionCount", len(ids), "error", err.Error())
		return err
	}
	if s.Cache != nil {
		if err := s.Cache.InvalidateAuthCache(ctx); err != nil {
			logger.Error("删除分机清理 Kamailio auth 缓存失败", "error", err.Error())
		}
	}
	logger.Info("运营端删除分机完成", "extensionCount", len(ids))
	return nil
}

func (s *ExtensionManagementService) SetEnable(ctx context.Context, id int, enable bool) (Extension, error) {
	logger := s.logger()
	logger.Info("运营端开始切换分机启用状态", "id", id, "enable", enable)
	extension, err := s.Repository.SetEnable(ctx, id, enable)
	if err != nil {
		logger.Error("运营端切换分机启用状态失败", "id", id, "enable", enable, "error", err.Error())
		return Extension{}, err
	}
	if s.Cache != nil {
		if err := s.Cache.InvalidateAuthCache(ctx); err != nil {
			logger.Error("切换分机启用状态清理 Kamailio auth 缓存失败", "error", err.Error())
		}
	}
	logger.Info("运营端切换分机启用状态完成", "id", id, "extension", extension.ExtensionNumber, "enable", extension.Enable)
	return extension, nil
}

func (s *ExtensionManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeExtensionPage(req ExtensionPageRequest) ExtensionPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.ExtensionNumber = strings.TrimSpace(req.ExtensionNumber)
	return req
}

func normalizeExtensionForSave(extension Extension) (Extension, error) {
	extension.ExtensionNumber = strings.TrimSpace(extension.ExtensionNumber)
	extension.Password = strings.TrimSpace(extension.Password)
	if extension.ExtensionNumber == "" || extension.MerchantID <= 0 || extension.UserID <= 0 {
		return Extension{}, ErrInvalidExtension
	}
	return extension, nil
}

func filterPositiveExtensionIDs(extensions []Extension) []int {
	ids := make([]int, 0, len(extensions))
	for _, extension := range extensions {
		if extension.ID > 0 {
			ids = append(ids, extension.ID)
		}
	}
	return ids
}

func calculateMD5(val string) string {
	h := md5.New()
	h.Write([]byte(val))
	return hex.EncodeToString(h.Sum(nil))
}

func calculateHA1(username, realm, password string) string {
	return calculateMD5(username + ":" + realm + ":" + password)
}

func calculateHA1b(username, realm, password string) string {
	return calculateMD5(username + "@" + realm + ":" + realm + ":" + password)
}
