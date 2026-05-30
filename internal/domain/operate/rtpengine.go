package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidRtpengine 表示运营端提交的 RTPEngine 配置无效或缺少字段。
	ErrInvalidRtpengine = errors.New("invalid rtpengine")
	// ErrRtpengineNotFound 表示请求的 RTPEngine 记录不存在或已被删除。
	ErrRtpengineNotFound = errors.New("rtpengine not found")
	// ErrRtpengineConflict 表示 RTPEngine 套接字连接串已存在。
	ErrRtpengineConflict = errors.New("rtpengine conflict")
)

// Rtpengine 表示 Kamailio RTPEngine 节点的运营配置。
type Rtpengine struct {
	ID            int    `json:"id,omitempty"`
	SetID         int    `json:"setId"`
	RtpengineSock string `json:"rtpengineSock"`
	Disabled      bool   `json:"disabled"`
	Weight        int    `json:"weight"`
	Description   string `json:"description"`
}

// RtpenginePageRequest 表示 RTPEngine 节点的查询条件。
type RtpenginePageRequest struct {
	PageNumber    int    `json:"pageNumber"`
	PageSize      int    `json:"pageSize"`
	SetID         int    `json:"setId,omitempty"`
	RtpengineSock string `json:"rtpengineSock,omitempty"`
	Disabled      *bool  `json:"disabled,omitempty"`
}

// RtpenginePageResult 是分页查询结果。
type RtpenginePageResult struct {
	PageNumber int         `json:"pageNumber"`
	PageSize   int         `json:"pageSize"`
	Total      int64       `json:"total"`
	Records    []Rtpengine `json:"records"`
}

// RtpengineMutationResult 描述修改 RTPEngine 配置后的结果。
type RtpengineMutationResult struct {
	Rtpengine        Rtpengine `json:"rtpengine,omitempty"`
	ReloadRequired   bool      `json:"reloadRequired"`
	ReloadDispatched bool      `json:"reloadDispatched"`
}

// RtpengineRepository 定义 RTPEngine 配置的仓储操作。
type RtpengineRepository interface {
	Page(ctx context.Context, req RtpenginePageRequest) (RtpenginePageResult, error)
	GetByID(ctx context.Context, id int) (Rtpengine, error)
	ExistsSock(ctx context.Context, rtpengineSock string, excludeID int) (bool, error)
	Save(ctx context.Context, engine Rtpengine) (Rtpengine, error)
	Delete(ctx context.Context, ids []int) error
}

// RtpengineReloadPort 定义触发 Kamailio RTPEngine 配置热重载的接口。
type RtpengineReloadPort interface {
	ReloadRtpengine(ctx context.Context) error
}

// RtpengineManagementService 承载 Kamailio RTPEngine 管理业务。
type RtpengineManagementService struct {
	Repository RtpengineRepository
	Reloader   RtpengineReloadPort
	Logger     *slog.Logger
}

// Page 返回分页查询结果。
func (s *RtpengineManagementService) Page(ctx context.Context, req RtpenginePageRequest) (RtpenginePageResult, error) {
	logger := s.logger()
	req = normalizeRtpenginePage(req)
	logger.Info("运营端开始分页查询 Kamailio RTPEngine 节点", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "setId", req.SetID, "rtpengineSock", req.RtpengineSock)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询 Kamailio RTPEngine 节点失败", "error", err.Error())
		return RtpenginePageResult{}, err
	}
	logger.Info("运营端分页查询 Kamailio RTPEngine 节点完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Save 保存（新增或更新）节点。
func (s *RtpengineManagementService) Save(ctx context.Context, engine Rtpengine) (RtpengineMutationResult, error) {
	logger := s.logger()
	normalized, err := normalizeRtpengineForSave(engine)
	if err != nil {
		logger.Warn("运营端保存 RTPEngine 节点参数无效", "id", engine.ID, "rtpengineSock", engine.RtpengineSock, "error", err.Error())
		return RtpengineMutationResult{}, err
	}
	exists, err := s.Repository.ExistsSock(ctx, normalized.RtpengineSock, normalized.ID)
	if err != nil {
		logger.Error("运营端校验 RTPEngine 唯一性失败", "id", normalized.ID, "rtpengineSock", normalized.RtpengineSock, "error", err.Error())
		return RtpengineMutationResult{}, err
	}
	if exists {
		logger.Warn("运营端保存 RTPEngine 冲突", "id", normalized.ID, "rtpengineSock", normalized.RtpengineSock)
		return RtpengineMutationResult{}, ErrRtpengineConflict
	}

	action := "create"
	if normalized.ID > 0 {
		action = "update"
	}
	logger.Info("运营端开始保存 RTPEngine 节点", "id", normalized.ID, "setId", normalized.SetID, "rtpengineSock", normalized.RtpengineSock, "action", action, "disabled", normalized.Disabled)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存 RTPEngine 节点失败", "id", normalized.ID, "rtpengineSock", normalized.RtpengineSock, "error", err.Error())
		return RtpengineMutationResult{}, err
	}
	reloadDispatched := s.reload(ctx, action, saved)
	logger.Info("运营端保存 RTPEngine 节点完成", "id", saved.ID, "rtpengineSock", saved.RtpengineSock, "reloadRequired", true, "reloadDispatched", reloadDispatched)
	return RtpengineMutationResult{Rtpengine: saved, ReloadRequired: true, ReloadDispatched: reloadDispatched}, nil
}

// Delete 批量逻辑删除节点。
func (s *RtpengineManagementService) Delete(ctx context.Context, engines []Rtpengine) (RtpengineMutationResult, error) {
	logger := s.logger()
	ids := make([]int, 0, len(engines))
	for _, engine := range engines {
		if engine.ID > 0 {
			ids = append(ids, engine.ID)
		}
	}
	if len(ids) == 0 {
		return RtpengineMutationResult{}, ErrInvalidRtpengine
	}
	logger.Info("运营端开始逻辑删除 RTPEngine 节点", "count", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端逻辑删除 RTPEngine 节点失败", "count", len(ids), "error", err.Error())
		return RtpengineMutationResult{}, err
	}
	reloadDispatched := false
	for _, engine := range engines {
		reloadDispatched = s.reload(ctx, "delete", engine) || reloadDispatched
	}
	logger.Info("运营端逻辑删除 RTPEngine 节点完成", "count", len(ids), "reloadRequired", true, "reloadDispatched", reloadDispatched)
	return RtpengineMutationResult{ReloadRequired: true, ReloadDispatched: reloadDispatched}, nil
}

// Reload 手动触发热刷新配置。
func (s *RtpengineManagementService) Reload(ctx context.Context) (RtpengineMutationResult, error) {
	logger := s.logger()
	logger.Info("运营端手工触发刷新 Kamailio RTPEngine")
	if s.Reloader == nil {
		logger.Warn("运营端 Kamailio RTPEngine 刷新接口未配置")
		return RtpengineMutationResult{ReloadRequired: true}, nil
	}
	if err := s.Reloader.ReloadRtpengine(ctx); err != nil {
		logger.Error("运营端手工刷新 Kamailio RTPEngine 失败", "error", err.Error())
		return RtpengineMutationResult{ReloadRequired: true}, err
	}
	logger.Info("运营端手工刷新 Kamailio RTPEngine 完成")
	return RtpengineMutationResult{ReloadRequired: true, ReloadDispatched: true}, nil
}

func (s *RtpengineManagementService) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *RtpengineManagementService) reload(ctx context.Context, action string, engine Rtpengine) bool {
	if s.Reloader == nil {
		s.logger().Warn("运营端 Kamailio RTPEngine 刷新接口未配置", "id", engine.ID, "rtpengineSock", engine.RtpengineSock, "action", action)
		return false
	}
	if err := s.Reloader.ReloadRtpengine(ctx); err != nil {
		s.logger().Warn("运营端 Kamailio RTPEngine 刷新失败", "id", engine.ID, "rtpengineSock", engine.RtpengineSock, "action", action, "error", err.Error())
		return false
	}
	return true
}

func normalizeRtpenginePage(req RtpenginePageRequest) RtpenginePageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.RtpengineSock = strings.TrimSpace(req.RtpengineSock)
	return req
}

func normalizeRtpengineForSave(engine Rtpengine) (Rtpengine, error) {
	engine.RtpengineSock = strings.TrimSpace(engine.RtpengineSock)
	engine.Description = strings.TrimSpace(engine.Description)
	if engine.SetID <= 0 || engine.RtpengineSock == "" || engine.Description == "" {
		return Rtpengine{}, ErrInvalidRtpengine
	}
	if engine.Weight <= 0 {
		engine.Weight = 1
	}
	return engine, nil
}
