package operate

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"math/big"
	"strings"
	"time"
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
	ID              int        `json:"id,omitempty"`
	ExtensionNumber string     `json:"extensionNumber"`
	Password        string     `json:"password,omitempty"`
	MerchantID      int        `json:"merchantId"`
	UserID          int        `json:"userId"`
	Enable          bool       `json:"enable"`
	BindType        int        `json:"bindType"`
	SipDomain       string     `json:"sipDomain,omitempty"`
	HA1             string     `json:"ha1,omitempty"`
	HA1b            string     `json:"ha1b,omitempty"`
	OfflineAt       *time.Time `json:"offlineAt,omitempty"`
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

	// 编辑时：如果前端未传密码（留空），从数据库保留旧密码
	// Kamailio 的 subscriber 表要求 ha1 / ha1b 始终与 password 保持一致
	if normalized.ID > 0 && normalized.Password == "" {
		existing, err := s.Repository.GetByID(ctx, normalized.ID)
		if err == nil && existing.Password != "" {
			normalized.Password = existing.Password
		}
	}

	// 始终重算 HA1 / HA1b（参考 Kamailio auth_db 模块）：
	//   ha1  = MD5(username : realm : password)   — 标准 RFC 2617 Digest
	//   ha1b = MD5(username@realm : realm : password) — Kamailio 扩展格式
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

// RecalculateAllHA 批量重算所有分机的密码、SipDomain、HA1、HA1b，并将 bindType 统一设为动态释放。
//
// 遍历全部分机记录，对每条记录：
//  1. 生成新的 8 位随机密码（替换旧的弱密码如 "123456"）；
//  2. 从关联商户解析 SipDomain（若无则使用默认值）；
//  3. 根据新密码重新计算 HA1 / HA1b（Kamailio auth_db 格式）；
//  4. 将 bindType 设为 2（动态释放 / 自动回收）；
//  5. 将更新后的记录写回数据库。
//
// 返回成功更新的记录数。
func (s *ExtensionManagementService) RecalculateAllHA(ctx context.Context) (int, error) {
	logger := s.logger()
	logger.Info("开始批量重算全部分机密码/HA1/HA1b 并统一 bindType 为动态释放")

	updated := 0
	pageNum := 1
	const batchSize = 200

	for {
		page, err := s.Repository.Page(ctx, ExtensionPageRequest{PageNumber: pageNum, PageSize: batchSize})
		if err != nil {
			logger.Error("批量重算 HA 读取分机失败", "page", pageNum, "error", err.Error())
			return updated, err
		}
		if len(page.Records) == 0 {
			break
		}

		for _, ext := range page.Records {
			// 为每条分机生成新的随机 8 位密码
			ext.Password = GenerateRandomPassword(8)

			// 解析 SipDomain
			if s.MerchantRepo != nil {
				mch, err := s.MerchantRepo.GetByID(ctx, ext.MerchantID)
				if err == nil && mch.SipDomain != "" {
					ext.SipDomain = mch.SipDomain
				}
			}
			if ext.SipDomain == "" {
				ext.SipDomain = "sip.yunshu.local"
			}

			// 根据新密码重算 Kamailio HA1 / HA1b
			ext.HA1 = calculateHA1(ext.ExtensionNumber, ext.SipDomain, ext.Password)
			ext.HA1b = calculateHA1b(ext.ExtensionNumber, ext.SipDomain, ext.Password)

			// 统一设为动态释放（自动回收）
			ext.BindType = BindTypeDynamic

			if _, err := s.Repository.Save(ctx, ext); err != nil {
				logger.Warn("批量重算 HA 保存分机失败", "id", ext.ID, "extension", ext.ExtensionNumber, "error", err.Error())
				continue
			}
			updated++
		}

		if int64(pageNum*batchSize) >= page.Total {
			break
		}
		pageNum++
	}

	if s.Cache != nil {
		if err := s.Cache.InvalidateAuthCache(ctx); err != nil {
			logger.Error("批量重算 HA 后清理 Kamailio auth 缓存失败", "error", err.Error())
		}
	}
	logger.Info("批量重算全部分机密码/HA1/HA1b 完成", "updated", updated)
	return updated, nil
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
	if extension.ExtensionNumber == "" || extension.MerchantID <= 0 {
		return Extension{}, ErrInvalidExtension
	}
	if extension.UserID < 0 {
		extension.UserID = 0
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

// GenerateRandomPassword 生成指定长度的随机密码。
// 字符集排除了易混淆的字符（0/O、1/l/I），与前端 generateRandomPassword 保持一致。
func GenerateRandomPassword(length int) string {
	const charset = "ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
	result := make([]byte, length)
	for i := range result {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// crypto/rand 失败时使用后备方案
			result[i] = charset[i%len(charset)]
			continue
		}
		result[i] = charset[idx.Int64()]
	}
	return string(result)
}
