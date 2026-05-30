package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	ErrInvalidPhoneAttribution  = errors.New("invalid phone attribution")
	ErrPhoneAttributionNotFound = errors.New("phone attribution not found")
	ErrPhoneAttributionConflict = errors.New("phone attribution conflict")
)

// PhoneAttribution 描述号段前7位（如1380013）与被叫省市行政区划的映射关系，
// 用于盲区风控策略判断和外呼路径的选择过滤。
type PhoneAttribution struct {
	AreaCode string `json:"areaCode"` // 号码前7位号段，主键 (例如 "1380013")
	ProvCode string `json:"provCode"` // 省份行政区划代码 (例如 "440000")
	CityCode string `json:"cityCode"` // 城市行政区划代码 (例如 "440300")
}

type PhoneAttributionPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	AreaCode   string `json:"areaCode,omitempty"`
	ProvCode   string `json:"provCode,omitempty"`
	CityCode   string `json:"cityCode,omitempty"`
}

type PhoneAttributionPageResult struct {
	PageNumber int                `json:"pageNumber"`
	PageSize   int                `json:"pageSize"`
	Total      int64              `json:"total"`
	Records    []PhoneAttribution `json:"records"`
}

type PhoneAttributionRepository interface {
	Page(ctx context.Context, req PhoneAttributionPageRequest) (PhoneAttributionPageResult, error)
	GetByAreaCode(ctx context.Context, areaCode string) (PhoneAttribution, bool, error)
	Save(ctx context.Context, attr PhoneAttribution) (PhoneAttribution, error)
	Delete(ctx context.Context, areaCodes []string) error
}

type PhoneAttributionManagementService struct {
	Repository PhoneAttributionRepository
	Logger     *slog.Logger
}

func (s *PhoneAttributionManagementService) Page(ctx context.Context, req PhoneAttributionPageRequest) (PhoneAttributionPageResult, error) {
	logger := s.logger()
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.AreaCode = strings.TrimSpace(req.AreaCode)
	req.ProvCode = strings.TrimSpace(req.ProvCode)
	req.CityCode = strings.TrimSpace(req.CityCode)

	logger.Info("开始分页查询号码归属地数据", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "areaCode", req.AreaCode)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("分页查询号码归属地数据失败", "error", err.Error())
		return PhoneAttributionPageResult{}, err
	}
	return page, nil
}

func (s *PhoneAttributionManagementService) Save(ctx context.Context, attr PhoneAttribution) (PhoneAttribution, error) {
	logger := s.logger()
	attr.AreaCode = strings.TrimSpace(attr.AreaCode)
	attr.ProvCode = strings.TrimSpace(attr.ProvCode)
	attr.CityCode = strings.TrimSpace(attr.CityCode)

	if attr.AreaCode == "" || len(attr.AreaCode) < 7 {
		logger.Warn("保存号码归属地校验失败：号段无效", "areaCode", attr.AreaCode)
		return PhoneAttribution{}, ErrInvalidPhoneAttribution
	}

	logger.Info("开始保存号码归属地映射", "areaCode", attr.AreaCode, "provCode", attr.ProvCode, "cityCode", attr.CityCode)
	saved, err := s.Repository.Save(ctx, attr)
	if err != nil {
		logger.Error("保存号码归属地映射失败", "areaCode", attr.AreaCode, "error", err.Error())
		return PhoneAttribution{}, err
	}
	return saved, nil
}

func (s *PhoneAttributionManagementService) Delete(ctx context.Context, areaCodes []string) error {
	logger := s.logger()
	var validCodes []string
	for _, code := range areaCodes {
		trimmed := strings.TrimSpace(code)
		if trimmed != "" {
			validCodes = append(validCodes, trimmed)
		}
	}
	if len(validCodes) == 0 {
		return ErrInvalidPhoneAttribution
	}

	logger.Info("开始批量删除号码归属地映射", "count", len(validCodes))
	if err := s.Repository.Delete(ctx, validCodes); err != nil {
		logger.Error("删除号码归属地映射失败", "error", err.Error())
		return err
	}
	return nil
}

func (s *PhoneAttributionManagementService) Lookup(ctx context.Context, phone string) (PhoneAttribution, bool, error) {
	phone = strings.TrimSpace(phone)
	if len(phone) < 7 {
		return PhoneAttribution{}, false, nil
	}
	areaCode := phone[:7]
	return s.Repository.GetByAreaCode(ctx, areaCode)
}

func (s *PhoneAttributionManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}
