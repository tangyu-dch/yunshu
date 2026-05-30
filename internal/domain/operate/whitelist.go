package operate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

var (
	// ErrInvalidWhitelist 表示白名单参数错误。
	ErrInvalidWhitelist = errors.New("invalid whitelist")
	// ErrWhitelistNotFound 表示白名单记录不存在。
	ErrWhitelistNotFound = errors.New("whitelist not found")
)

const (
	// WhiteNumberTypeCaller 表示主叫白名单。
	WhiteNumberTypeCaller = "CALLER"
	// WhiteNumberTypeCallee 表示被叫白名单。
	WhiteNumberTypeCallee = "CALLEE"
)

// WhitelistRecord 表示 `whitelist_data` 与 `whitelist_data_merchant` 聚合后的管理视图。
type WhitelistRecord struct {
	ID            int      `json:"id,omitempty"`
	Phone         string   `json:"phone"`
	NumberType    string   `json:"numberType"`
	MerchantIDs   []int    `json:"merchantIds,omitempty"`
	MerchantNames []string `json:"merchantNames,omitempty"`
}

// WhitelistPageRequest 表示白名单分页查询条件。
type WhitelistPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Number     string `json:"number,omitempty"`
	MerchantID int    `json:"merchantId,omitempty"`
}

// WhitelistPageResult 表示白名单分页结果。
type WhitelistPageResult struct {
	PageNumber int               `json:"pageNumber"`
	PageSize   int               `json:"pageSize"`
	Total      int64             `json:"total"`
	Records    []WhitelistRecord `json:"records"`
}

// AddWhitelistRequest 表示批量添加白名单请求。
type AddWhitelistRequest struct {
	Phones      []string `json:"phones"`
	NumberType  string   `json:"numberType"`
	MerchantIDs []int    `json:"merchantIds,omitempty"`
}

// UpdateWhitelistRequest 表示编辑白名单请求。
type UpdateWhitelistRequest struct {
	ID          int    `json:"id"`
	NumberType  string `json:"numberType"`
	MerchantIDs []int  `json:"merchantIds,omitempty"`
}

// WhitelistRepository 定义白名单管理仓储能力。
type WhitelistRepository interface {
	Page(ctx context.Context, req WhitelistPageRequest) (WhitelistPageResult, error)
	FindExistingPhones(ctx context.Context, phones []string) ([]string, error)
	CreateBatch(ctx context.Context, phones []string, numberType string, merchantIDs []int) error
	GetByID(ctx context.Context, id int) (WhitelistRecord, error)
	Update(ctx context.Context, req UpdateWhitelistRequest) error
	Delete(ctx context.Context, ids []int) error
}

// WhitelistManagementService 承载运营端白名单管理业务。
type WhitelistManagementService struct {
	Repository WhitelistRepository
	Logger     *slog.Logger
}

// Page 分页查询白名单。
func (s *WhitelistManagementService) Page(ctx context.Context, req WhitelistPageRequest) (WhitelistPageResult, error) {
	logger := s.logger()
	req = normalizeWhitelistPage(req)
	logger.Info("运营端开始分页查询白名单", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "number", req.Number, "merchantId", req.MerchantID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询白名单失败", "error", err.Error())
		return WhitelistPageResult{}, err
	}
	logger.Info("运营端分页查询白名单完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Add 批量新增白名单，并返回与  兼容的中文结果摘要。
func (s *WhitelistManagementService) Add(ctx context.Context, req AddWhitelistRequest) (string, error) {
	logger := s.logger()
	normalized, err := normalizeAddWhitelistRequest(req)
	if err != nil {
		logger.Warn("运营端添加白名单参数无效", "error", err.Error())
		return "", err
	}
	repeated, err := s.Repository.FindExistingPhones(ctx, normalized.Phones)
	if err != nil {
		logger.Error("运营端查询重复白名单号码失败", "error", err.Error())
		return "", err
	}
	repeatedSet := make(map[string]struct{}, len(repeated))
	for _, phone := range repeated {
		repeatedSet[phone] = struct{}{}
	}
	createdPhones := make([]string, 0, len(normalized.Phones))
	for _, phone := range normalized.Phones {
		if _, ok := repeatedSet[phone]; !ok {
			createdPhones = append(createdPhones, phone)
		}
	}
	if len(createdPhones) == 0 {
		logger.Info("运营端添加白名单完成，无新增号码", "repeatCount", len(repeated))
		return fmt.Sprintf("添加成功 0个, 失败%d个 失败号码有%s", len(repeated), strings.Join(repeated, ", ")), nil
	}
	logger.Info("运营端开始批量添加白名单", "phoneCount", len(createdPhones), "numberType", normalized.NumberType, "merchantCount", len(normalized.MerchantIDs))
	if err := s.Repository.CreateBatch(ctx, createdPhones, normalized.NumberType, normalized.MerchantIDs); err != nil {
		logger.Error("运营端批量添加白名单失败", "phoneCount", len(createdPhones), "error", err.Error())
		return "", err
	}
	logger.Info("运营端批量添加白名单完成", "createdCount", len(createdPhones), "repeatCount", len(repeated))
	if len(repeated) == 0 {
		return "添加白名单成功", nil
	}
	return fmt.Sprintf("添加成功 %d个, 失败%d个 失败号码有%s", len(createdPhones), len(repeated), strings.Join(repeated, ", ")), nil
}

// Detail 查询白名单详情。
func (s *WhitelistManagementService) Detail(ctx context.Context, id int) (WhitelistRecord, error) {
	if id <= 0 {
		return WhitelistRecord{}, ErrInvalidWhitelist
	}
	return s.Repository.GetByID(ctx, id)
}

// Update 编辑白名单类型和商户绑定。
func (s *WhitelistManagementService) Update(ctx context.Context, req UpdateWhitelistRequest) error {
	logger := s.logger()
	normalized, err := normalizeUpdateWhitelistRequest(req)
	if err != nil {
		logger.Warn("运营端编辑白名单参数无效", "id", req.ID, "error", err.Error())
		return err
	}
	logger.Info("运营端开始编辑白名单", "id", normalized.ID, "numberType", normalized.NumberType, "merchantCount", len(normalized.MerchantIDs))
	if err := s.Repository.Update(ctx, normalized); err != nil {
		logger.Error("运营端编辑白名单失败", "id", normalized.ID, "error", err.Error())
		return err
	}
	logger.Info("运营端编辑白名单完成", "id", normalized.ID, "merchantCount", len(normalized.MerchantIDs))
	return nil
}

// Delete 删除白名单。
func (s *WhitelistManagementService) Delete(ctx context.Context, ids []int) error {
	logger := s.logger()
	normalized := normalizeWhitelistIDs(ids)
	if len(normalized) == 0 {
		return ErrInvalidWhitelist
	}
	logger.Info("运营端开始删除白名单", "whiteCount", len(normalized))
	if err := s.Repository.Delete(ctx, normalized); err != nil {
		logger.Error("运营端删除白名单失败", "whiteCount", len(normalized), "error", err.Error())
		return err
	}
	logger.Info("运营端删除白名单完成", "whiteCount", len(normalized))
	return nil
}

func (s *WhitelistManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeWhitelistPage(req WhitelistPageRequest) WhitelistPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Number = strings.TrimSpace(req.Number)
	return req
}

func normalizeAddWhitelistRequest(req AddWhitelistRequest) (AddWhitelistRequest, error) {
	numberType := strings.ToUpper(strings.TrimSpace(req.NumberType))
	if numberType != WhiteNumberTypeCaller && numberType != WhiteNumberTypeCallee {
		return AddWhitelistRequest{}, ErrInvalidWhitelist
	}
	set := make(map[string]struct{}, len(req.Phones))
	phones := make([]string, 0, len(req.Phones))
	for _, phone := range req.Phones {
		trimmed := strings.TrimSpace(phone)
		if trimmed == "" {
			continue
		}
		if _, ok := set[trimmed]; ok {
			continue
		}
		set[trimmed] = struct{}{}
		phones = append(phones, trimmed)
	}
	if len(phones) == 0 {
		return AddWhitelistRequest{}, ErrInvalidWhitelist
	}
	merchantIDs := normalizeWhitelistIDs(req.MerchantIDs)
	return AddWhitelistRequest{Phones: phones, NumberType: numberType, MerchantIDs: merchantIDs}, nil
}

func normalizeUpdateWhitelistRequest(req UpdateWhitelistRequest) (UpdateWhitelistRequest, error) {
	if req.ID <= 0 {
		return UpdateWhitelistRequest{}, ErrInvalidWhitelist
	}
	numberType := strings.ToUpper(strings.TrimSpace(req.NumberType))
	if numberType != WhiteNumberTypeCaller && numberType != WhiteNumberTypeCallee {
		return UpdateWhitelistRequest{}, ErrInvalidWhitelist
	}
	return UpdateWhitelistRequest{
		ID:          req.ID,
		NumberType:  numberType,
		MerchantIDs: normalizeWhitelistIDs(req.MerchantIDs),
	}, nil
}

func normalizeWhitelistIDs(ids []int) []int {
	set := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := set[id]; ok {
			continue
		}
		set[id] = struct{}{}
		normalized = append(normalized, id)
	}
	sort.Ints(normalized)
	return normalized
}
