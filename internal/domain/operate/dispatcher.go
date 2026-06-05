package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidDispatcher 表示运营端提交的 Dispatcher 配置无效或缺少字段。
	ErrInvalidDispatcher = errors.New("invalid dispatcher")
	// ErrDispatcherNotFound 表示请求的 Dispatcher 记录不存在或已被删除。
	ErrDispatcherNotFound = errors.New("dispatcher not found")
	// ErrDispatcherConflict 表示 Dispatcher 目的地址已存在。
	ErrDispatcherConflict = errors.New("dispatcher conflict")
)

// Dispatcher 表示 Kamailio Dispatcher 网关探测节点的运营配置。
type Dispatcher struct {
	ID          int    `json:"id,omitempty"`
	SetID       int    `json:"setId"`
	Destination string `json:"destination"`
	Flags       int    `json:"flags"`
	Priority    int    `json:"priority"`
	Attrs       string `json:"attrs"`
	Description string `json:"description"`
	Enable      bool   `json:"enable"`
}

// DispatcherPageRequest 表示 Dispatcher 节点的查询条件。
type DispatcherPageRequest struct {
	PageNumber int   `json:"pageNumber"`
	PageSize   int   `json:"pageSize"`
	SetID      int   `json:"setId,omitempty"`
	Enable     *bool `json:"enable,omitempty"`
}

// DispatcherPageResult 是分页查询结果。
type DispatcherPageResult struct {
	PageNumber int          `json:"pageNumber"`
	PageSize   int          `json:"pageSize"`
	Total      int64        `json:"total"`
	Records    []Dispatcher `json:"records"`
}

// DispatcherMutationResult 描述修改 Dispatcher 配置后的结果。
type DispatcherMutationResult struct {
	Dispatcher       Dispatcher `json:"dispatcher,omitempty"`
	ReloadRequired   bool       `json:"reloadRequired"`
	ReloadDispatched bool       `json:"reloadDispatched"`
}

// DispatcherRepository 定义 Dispatcher 配置的仓储操作。
type DispatcherRepository interface {
	Page(ctx context.Context, req DispatcherPageRequest) (DispatcherPageResult, error)
	GetByID(ctx context.Context, id int) (Dispatcher, error)
	ExistsDestination(ctx context.Context, destination string, excludeID int) (bool, error)
	Save(ctx context.Context, disp Dispatcher) (Dispatcher, error)
	Delete(ctx context.Context, ids []int) error
}

// DispatcherReloadPort 定义触发 Kamailio Dispatcher 配置热重载的接口。
type DispatcherReloadPort interface {
	ReloadDispatcher(ctx context.Context) error
}

// DispatcherManagementService 承载 Kamailio Dispatcher 管理业务。
type DispatcherManagementService struct {
	Repository DispatcherRepository
	Reloader   DispatcherReloadPort
	Logger     *slog.Logger
}

// Page 返回分页查询结果。
func (s *DispatcherManagementService) Page(ctx context.Context, req DispatcherPageRequest) (DispatcherPageResult, error) {
	logger := s.logger()
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	logger.Info("运营端开始分页查询 Kamailio Dispatcher 节点", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "setId", req.SetID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询 Kamailio Dispatcher 节点失败", "error", err.Error())
		return DispatcherPageResult{}, err
	}
	logger.Info("运营端分页查询 Kamailio Dispatcher 节点完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Save 保存（新增或更新）节点。
func (s *DispatcherManagementService) Save(ctx context.Context, disp Dispatcher) (DispatcherMutationResult, error) {
	logger := s.logger()
	disp.Destination = strings.TrimSpace(disp.Destination)
	disp.Description = strings.TrimSpace(disp.Description)
	disp.Attrs = strings.TrimSpace(disp.Attrs)
	if disp.SetID <= 0 || disp.Destination == "" || disp.Description == "" {
		return DispatcherMutationResult{}, ErrInvalidDispatcher
	}

	exists, err := s.Repository.ExistsDestination(ctx, disp.Destination, disp.ID)
	if err != nil {
		logger.Error("运营端校验 Dispatcher 唯一性失败", "id", disp.ID, "destination", disp.Destination, "error", err.Error())
		return DispatcherMutationResult{}, err
	}
	if exists {
		logger.Warn("运营端保存 Dispatcher 冲突", "id", disp.ID, "destination", disp.Destination)
		return DispatcherMutationResult{}, ErrDispatcherConflict
	}

	action := "create"
	if disp.ID > 0 {
		action = "update"
	}
	logger.Info("运营端开始保存 Kamailio Dispatcher 节点", "id", disp.ID, "setId", disp.SetID, "destination", disp.Destination, "action", action)
	saved, err := s.Repository.Save(ctx, disp)
	if err != nil {
		logger.Error("运营端保存 Kamailio Dispatcher 节点失败", "id", disp.ID, "destination", disp.Destination, "error", err.Error())
		return DispatcherMutationResult{}, err
	}
	reloadDispatched := s.reload(ctx, action, saved)
	logger.Info("运营端保存 Kamailio Dispatcher 节点完成", "id", saved.ID, "destination", saved.Destination, "reloadRequired", true, "reloadDispatched", reloadDispatched)
	return DispatcherMutationResult{Dispatcher: saved, ReloadRequired: true, ReloadDispatched: reloadDispatched}, nil
}

// Delete 批量逻辑删除节点。
func (s *DispatcherManagementService) Delete(ctx context.Context, dispatchers []Dispatcher) (DispatcherMutationResult, error) {
	logger := s.logger()
	ids := make([]int, 0, len(dispatchers))
	for _, disp := range dispatchers {
		if disp.ID > 0 {
			ids = append(ids, disp.ID)
		}
	}
	if len(ids) == 0 {
		return DispatcherMutationResult{}, ErrInvalidDispatcher
	}
	logger.Info("运营端开始逻辑删除 Kamailio Dispatcher 节点", "count", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端逻辑删除 Kamailio Dispatcher 节点失败", "count", len(ids), "error", err.Error())
		return DispatcherMutationResult{}, err
	}
	reloadDispatched := false
	for _, disp := range dispatchers {
		reloadDispatched = s.reload(ctx, "delete", disp) || reloadDispatched
	}
	logger.Info("运营端逻辑删除 Kamailio Dispatcher 节点完成", "count", len(ids), "reloadRequired", true, "reloadDispatched", reloadDispatched)
	return DispatcherMutationResult{ReloadRequired: true, ReloadDispatched: reloadDispatched}, nil
}

// Reload 手动触发热刷新配置。
func (s *DispatcherManagementService) Reload(ctx context.Context) (DispatcherMutationResult, error) {
	logger := s.logger()
	logger.Info("运营端手工触发刷新 Kamailio Dispatcher")
	if s.Reloader == nil {
		logger.Warn("运营端 Kamailio Dispatcher 刷新接口未配置")
		return DispatcherMutationResult{ReloadRequired: true}, nil
	}
	if err := s.Reloader.ReloadDispatcher(ctx); err != nil {
		logger.Error("运营端手工刷新 Kamailio Dispatcher 失败", "error", err.Error())
		return DispatcherMutationResult{ReloadRequired: true}, err
	}
	logger.Info("运营端手工刷新 Kamailio Dispatcher 完成")
	return DispatcherMutationResult{ReloadRequired: true, ReloadDispatched: true}, nil
}

func (s *DispatcherManagementService) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *DispatcherManagementService) reload(ctx context.Context, action string, disp Dispatcher) bool {
	if s.Reloader == nil {
		s.logger().Warn("运营端 Kamailio Dispatcher 刷新接口未配置", "id", disp.ID, "destination", disp.Destination, "action", action)
		return false
	}
	if err := s.Reloader.ReloadDispatcher(ctx); err != nil {
		s.logger().Warn("运营端 Kamailio Dispatcher 刷新失败", "id", disp.ID, "destination", disp.Destination, "action", action, "error", err.Error())
		return false
	}
	return true
}
