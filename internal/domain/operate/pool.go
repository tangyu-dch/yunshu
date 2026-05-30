package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidPool 表示号码池参数无效。
	ErrInvalidPool = errors.New("invalid pool")
	// ErrPoolNotFound 表示号码池不存在。
	ErrPoolNotFound = errors.New("pool not found")
	// ErrPoolConflict 表示号码池名称冲突。
	ErrPoolConflict = errors.New("pool conflict")
)

// Pool 表示  兼容 `pool` 表中的号码池配置。
type Pool struct {
	ID                int    `json:"id,omitempty"`
	Name              string `json:"name"`
	Remark            string `json:"remark,omitempty"`
	Type              int    `json:"type"`
	GatewayID         int    `json:"gatewayId,omitempty"`
	Enable            bool   `json:"enable"`
	SelectionStrategy string `json:"selectionStrategy,omitempty"`
}

type PoolPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
	GatewayID  int    `json:"gatewayId,omitempty"`
	Enable     *bool  `json:"enable,omitempty"`
}

type PoolPageResult struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Total      int64  `json:"total"`
	Records    []Pool `json:"records"`
}

type PoolRepository interface {
	Page(ctx context.Context, req PoolPageRequest) (PoolPageResult, error)
	GetByID(ctx context.Context, id int) (Pool, error)
	ExistsName(ctx context.Context, name string, excludeID int) (bool, error)
	Save(ctx context.Context, pool Pool) (Pool, error)
	Delete(ctx context.Context, ids []int) error
	ListByGateway(ctx context.Context, gatewayID int) ([]Pool, error)
	ListAll(ctx context.Context) ([]Pool, error)
}

type PoolManagementService struct {
	Repository PoolRepository
	Logger     *slog.Logger
}

func (s *PoolManagementService) Page(ctx context.Context, req PoolPageRequest) (PoolPageResult, error) {
	logger := s.logger()
	req = normalizePoolPage(req)
	logger.Info("运营端开始分页查询号码池", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name, "gatewayId", req.GatewayID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("运营端分页查询号码池失败", "error", err.Error())
		return PoolPageResult{}, err
	}
	logger.Info("运营端分页查询号码池完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

func (s *PoolManagementService) Save(ctx context.Context, pool Pool) (Pool, error) {
	logger := s.logger()
	normalized, err := normalizePoolForSave(pool)
	if err != nil {
		logger.Warn("运营端保存号码池参数无效", "id", pool.ID, "name", pool.Name, "error", err.Error())
		return Pool{}, err
	}
	exists, err := s.Repository.ExistsName(ctx, normalized.Name, normalized.ID)
	if err != nil {
		logger.Error("运营端校验号码池唯一性失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return Pool{}, err
	}
	if exists {
		logger.Warn("运营端保存号码池冲突", "id", normalized.ID, "name", normalized.Name)
		return Pool{}, ErrPoolConflict
	}
	logger.Info("运营端开始保存号码池", "id", normalized.ID, "name", normalized.Name, "gatewayId", normalized.GatewayID, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("运营端保存号码池失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return Pool{}, err
	}
	logger.Info("运营端保存号码池完成", "id", saved.ID, "name", saved.Name, "enable", saved.Enable)
	return saved, nil
}

func (s *PoolManagementService) Delete(ctx context.Context, pools []Pool) error {
	logger := s.logger()
	ids := filterPositivePoolIDs(pools)
	if len(ids) == 0 {
		return ErrInvalidPool
	}
	logger.Info("运营端开始删除号码池", "poolCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("运营端删除号码池失败", "poolCount", len(ids), "error", err.Error())
		return err
	}
	logger.Info("运营端删除号码池完成", "poolCount", len(ids))
	return nil
}

func (s *PoolManagementService) ListByGateway(ctx context.Context, gatewayID int) ([]Pool, error) {
	logger := s.logger()
	logger.Info("运营端开始按网关查询号码池", "gatewayId", gatewayID)
	records, err := s.Repository.ListByGateway(ctx, gatewayID)
	if err != nil {
		logger.Error("运营端按网关查询号码池失败", "gatewayId", gatewayID, "error", err.Error())
		return nil, err
	}
	logger.Info("运营端按网关查询号码池完成", "gatewayId", gatewayID, "recordCount", len(records))
	return records, nil
}

func (s *PoolManagementService) ListAll(ctx context.Context) ([]Pool, error) {
	logger := s.logger()
	logger.Info("运营端开始查询全部号码池")
	records, err := s.Repository.ListAll(ctx)
	if err != nil {
		logger.Error("运营端查询全部号码池失败", "error", err.Error())
		return nil, err
	}
	logger.Info("运营端查询全部号码池完成", "recordCount", len(records))
	return records, nil
}

func (s *PoolManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizePoolPage(req PoolPageRequest) PoolPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizePoolForSave(pool Pool) (Pool, error) {
	pool.Name = strings.TrimSpace(pool.Name)
	pool.Remark = strings.TrimSpace(pool.Remark)
	pool.SelectionStrategy = strings.TrimSpace(pool.SelectionStrategy)
	if pool.SelectionStrategy == "" {
		pool.SelectionStrategy = "RANDOM"
	}
	if pool.Name == "" || pool.Type <= 0 {
		return Pool{}, ErrInvalidPool
	}
	return pool, nil
}

func filterPositivePoolIDs(pools []Pool) []int {
	ids := make([]int, 0, len(pools))
	for _, pool := range pools {
		if pool.ID > 0 {
			ids = append(ids, pool.ID)
		}
	}
	return ids
}
