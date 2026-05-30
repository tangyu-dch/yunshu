package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidPoolPhone 表示号码参数无效。
	ErrInvalidPoolPhone = errors.New("invalid pool phone")
	// ErrPoolPhoneNotFound 表示号码不存在。
	ErrPoolPhoneNotFound = errors.New("pool phone not found")
	// ErrPoolPhoneConflict 表示号码冲突。
	ErrPoolPhoneConflict = errors.New("pool phone conflict")
)

// PoolPhone 表示  兼容 `pool_phone` 表中的号码配置。
type PoolPhone struct {
	ID          int    `json:"id,omitempty"`
	PoolID      int    `json:"poolId"`
	Phone       string `json:"phone"`
	Province    string `json:"province,omitempty"`
	City        string `json:"city,omitempty"`
	Concurrency int    `json:"concurrency"`
	Remark      string `json:"remark,omitempty"`
	CallLimit   int    `json:"callLimit"`
	Enable      bool   `json:"enable"`
}

type PoolPhonePageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	PoolID     int    `json:"poolId,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Enable     *bool  `json:"enable,omitempty"`
}

type PoolPhonePageResult struct {
	PageNumber int         `json:"pageNumber"`
	PageSize   int         `json:"pageSize"`
	Total      int64       `json:"total"`
	Records    []PoolPhone `json:"records"`
}

type PoolPhoneRepository interface {
	Page(ctx context.Context, req PoolPhonePageRequest) (PoolPhonePageResult, error)
	GetByID(ctx context.Context, id int) (PoolPhone, error)
	ExistsPhone(ctx context.Context, phone string, excludeID int) (bool, error)
	Save(ctx context.Context, phone PoolPhone) (PoolPhone, error)
	Delete(ctx context.Context, ids []int) error
	SetEnable(ctx context.Context, id int, enable bool) (PoolPhone, error)
	SetPool(ctx context.Context, ids []int, poolID int) error
}

type PoolPhoneManagementService struct {
	Repository PoolPhoneRepository
	Logger     *slog.Logger
}

func (s *PoolPhoneManagementService) Page(ctx context.Context, req PoolPhonePageRequest) (PoolPhonePageResult, error) {
	logger := s.logger()
	req = normalizePoolPhonePage(req)
	logger.Info("运营端开始分页查询号码", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "poolId", req.PoolID, "phone", req.Phone)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询号码失败", "error", err.Error())
		return PoolPhonePageResult{}, err
	}
	logger.Info("运营端分页查询号码完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

func (s *PoolPhoneManagementService) Save(ctx context.Context, phone PoolPhone) (PoolPhone, error) {
	logger := s.logger()
	normalized, err := normalizePoolPhoneForSave(phone)
	if err != nil {
		logger.Warn("运营端保存号码参数无效", "id", phone.ID, "phone", phone.Phone, "error", err.Error())
		return PoolPhone{}, err
	}
	exists, err := s.Repository.ExistsPhone(ctx, normalized.Phone, normalized.ID)
	if err != nil {
		logger.Error("运营端校验号码唯一性失败", "id", normalized.ID, "phone", normalized.Phone, "error", err.Error())
		return PoolPhone{}, err
	}
	if exists {
		logger.Warn("运营端保存号码冲突", "id", normalized.ID, "phone", normalized.Phone)
		return PoolPhone{}, ErrPoolPhoneConflict
	}
	logger.Info("运营端开始保存号码", "id", normalized.ID, "phone", normalized.Phone, "poolId", normalized.PoolID, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存号码失败", "id", normalized.ID, "phone", normalized.Phone, "error", err.Error())
		return PoolPhone{}, err
	}
	logger.Info("运营端保存号码完成", "id", saved.ID, "phone", saved.Phone, "enable", saved.Enable)
	return saved, nil
}

func (s *PoolPhoneManagementService) Delete(ctx context.Context, phones []PoolPhone) error {
	logger := s.logger()
	ids := filterPositivePhoneIDs(phones)
	if len(ids) == 0 {
		return ErrInvalidPoolPhone
	}
	logger.Info("运营端开始删除号码", "phoneCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端删除号码失败", "phoneCount", len(ids), "error", err.Error())
		return err
	}
	logger.Info("运营端删除号码完成", "phoneCount", len(ids))
	return nil
}

func (s *PoolPhoneManagementService) SetEnable(ctx context.Context, id int, enable bool) (PoolPhone, error) {
	logger := s.logger()
	logger.Info("运营端开始切换号码启用状态", "id", id, "enable", enable)
	phone, err := s.Repository.SetEnable(ctx, id, enable)
	if err != nil {
		logger.Error("运营端切换号码启用状态失败", "id", id, "enable", enable, "error", err.Error())
		return PoolPhone{}, err
	}
	logger.Info("运营端切换号码启用状态完成", "id", id, "phone", phone.Phone, "enable", phone.Enable)
	return phone, nil
}

func (s *PoolPhoneManagementService) SetPool(ctx context.Context, ids []int, poolID int) error {
	logger := s.logger()
	ids = filterPositiveIDs(ids)
	if len(ids) == 0 {
		return ErrInvalidPoolPhone
	}
	logger.Info("运营端开始批量移动号码池", "phoneCount", len(ids), "poolId", poolID)
	if err := s.Repository.SetPool(ctx, ids, poolID); err != nil {
		logger.Error("运营端批量移动号码池失败", "phoneCount", len(ids), "poolId", poolID, "error", err.Error())
		return err
	}
	logger.Info("运营端批量移动号码池完成", "phoneCount", len(ids), "poolId", poolID)
	return nil
}

func (s *PoolPhoneManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizePoolPhonePage(req PoolPhonePageRequest) PoolPhonePageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Phone = strings.TrimSpace(req.Phone)
	return req
}

func normalizePoolPhoneForSave(phone PoolPhone) (PoolPhone, error) {
	phone.Phone = strings.TrimSpace(phone.Phone)
	phone.Province = strings.TrimSpace(phone.Province)
	phone.City = strings.TrimSpace(phone.City)
	phone.Remark = strings.TrimSpace(phone.Remark)
	if phone.PoolID < 0 || phone.Phone == "" {
		return PoolPhone{}, ErrInvalidPoolPhone
	}
	if phone.Concurrency < 0 || phone.CallLimit < 0 {
		return PoolPhone{}, ErrInvalidPoolPhone
	}
	return phone, nil
}

func filterPositivePhoneIDs(phones []PoolPhone) []int {
	ids := make([]int, 0, len(phones))
	for _, phone := range phones {
		if phone.ID > 0 {
			ids = append(ids, phone.ID)
		}
	}
	return ids
}
