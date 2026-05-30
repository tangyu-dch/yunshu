package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidRate 表示费率参数缺少生产必需字段。
	ErrInvalidRate = errors.New("invalid rate")
	// ErrRateNotFound 表示费率不存在或已删除。
	ErrRateNotFound = errors.New("rate not found")
	// ErrRateConflict 表示费率名称重复。
	ErrRateConflict = errors.New("rate conflict")
	// ErrRateReferenced 表示费率已被网关或商户引用，不能直接删除。
	ErrRateReferenced = errors.New("rate referenced")
)

// Rate 表示  兼容 `call_rate` 表中的费率配置。
//
// 该对象是网关 `rate_id` 和商户 `call_rate_merchant` 绑定的主数据真相；计费 workflow
// 后续会从这里衔接正式费率、周期和套餐规则，而不是继续依赖默认分钟费率。
type Rate struct {
	ID           int     `json:"id,omitempty"`
	RateName     string  `json:"rateName"`
	BillingPrice float64 `json:"billingPrice"`
	BillingCycle int     `json:"billingCycle"`
	Remark       string  `json:"remark,omitempty"`
}

// RatePageRequest 表示费率分页查询条件。
type RatePageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
}

// RatePageResult 表示费率分页结果。
type RatePageResult struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Total      int64  `json:"total"`
	Records    []Rate `json:"records"`
}

// RateRepository 定义费率管理仓储能力。
type RateRepository interface {
	Page(ctx context.Context, req RatePageRequest) (RatePageResult, error)
	GetByID(ctx context.Context, id int) (Rate, error)
	ExistsName(ctx context.Context, rateName string, excludeID int) (bool, error)
	Save(ctx context.Context, rate Rate) (Rate, error)
	Delete(ctx context.Context, ids []int) error
	HasBindings(ctx context.Context, ids []int) (bool, error)
}

// RateManagementService 承载运营端费率管理业务。
type RateManagementService struct {
	Repository RateRepository
	Logger     *slog.Logger
}

// Page 分页查询费率。
func (s *RateManagementService) Page(ctx context.Context, req RatePageRequest) (RatePageResult, error) {
	logger := s.logger()
	req = normalizeRatePage(req)
	logger.Info("运营端开始分页查询费率", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询费率失败", "error", err.Error())
		return RatePageResult{}, err
	}
	logger.Info("运营端分页查询费率完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Save 新增或更新费率。
func (s *RateManagementService) Save(ctx context.Context, rate Rate) (Rate, error) {
	logger := s.logger()
	normalized, err := normalizeRateForSave(rate)
	if err != nil {
		logger.Warn("运营端保存费率参数无效", "id", rate.ID, "rateName", rate.RateName, "error", err.Error())
		return Rate{}, err
	}
	exists, err := s.Repository.ExistsName(ctx, normalized.RateName, normalized.ID)
	if err != nil {
		logger.Error("运营端校验费率唯一性失败", "id", normalized.ID, "rateName", normalized.RateName, "error", err.Error())
		return Rate{}, err
	}
	if exists {
		logger.Warn("运营端保存费率冲突", "id", normalized.ID, "rateName", normalized.RateName)
		return Rate{}, ErrRateConflict
	}
	logger.Info("运营端开始保存费率", "id", normalized.ID, "rateName", normalized.RateName, "billingPrice", normalized.BillingPrice, "billingCycle", normalized.BillingCycle)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存费率失败", "id", normalized.ID, "rateName", normalized.RateName, "error", err.Error())
		return Rate{}, err
	}
	logger.Info("运营端保存费率完成", "id", saved.ID, "rateName", saved.RateName)
	return saved, nil
}

// Delete 删除费率。已被网关或商户绑定的费率必须失败关闭。
func (s *RateManagementService) Delete(ctx context.Context, rates []Rate) error {
	logger := s.logger()
	ids := make([]int, 0, len(rates))
	for _, rate := range rates {
		if rate.ID > 0 {
			ids = append(ids, rate.ID)
		}
	}
	if len(ids) == 0 {
		return ErrInvalidRate
	}
	referenced, err := s.Repository.HasBindings(ctx, ids)
	if err != nil {
		logger.Error("运营端删除费率前检查引用失败", "rateCount", len(ids), "error", err.Error())
		return err
	}
	if referenced {
		logger.Warn("运营端删除费率失败，费率仍被引用", "rateCount", len(ids))
		return ErrRateReferenced
	}
	logger.Info("运营端开始删除费率", "rateCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端删除费率失败", "rateCount", len(ids), "error", err.Error())
		return err
	}
	logger.Info("运营端删除费率完成", "rateCount", len(ids))
	return nil
}

func (s *RateManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeRatePage(req RatePageRequest) RatePageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizeRateForSave(rate Rate) (Rate, error) {
	rate.RateName = strings.TrimSpace(rate.RateName)
	rate.Remark = strings.TrimSpace(rate.Remark)
	if rate.RateName == "" || rate.BillingPrice < 0 || rate.BillingCycle <= 0 {
		return Rate{}, ErrInvalidRate
	}
	return rate, nil
}
