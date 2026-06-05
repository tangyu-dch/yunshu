package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	// ErrInvalidDepartment 表示部门参数无效。
	ErrInvalidDepartment = errors.New("invalid department")
	// ErrDepartmentNotFound 表示部门不存在。
	ErrDepartmentNotFound = errors.New("department not found")
	// ErrDepartmentReferenced 表示部门仍被账号或活动外呼任务引用。
	ErrDepartmentReferenced = errors.New("department referenced")
)

// Department 表示商户组织架构中的部门。
type Department struct {
	ID          int    `json:"id,omitempty"`
	MerchantID  int    `json:"merchantId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enable      bool   `json:"enable"`
}

// DepartmentPageRequest 表示部门分页查询请求。
type DepartmentPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
	MerchantID int    `json:"merchantId,omitempty"`
	Enable     *bool  `json:"enable,omitempty"`
}

// DepartmentPageResult 表示部门分页查询结果。
type DepartmentPageResult struct {
	PageNumber int          `json:"pageNumber"`
	PageSize   int          `json:"pageSize"`
	Total      int64        `json:"total"`
	Records    []Department `json:"records"`
}

// DepartmentRepository 定义商户部门管理的仓储接口。
type DepartmentRepository interface {
	Page(ctx context.Context, req DepartmentPageRequest) (DepartmentPageResult, error)
	GetByID(ctx context.Context, id int) (Department, error)
	Save(ctx context.Context, dept Department) (Department, error)
	Delete(ctx context.Context, ids []int) error
	ListAll(ctx context.Context, merchantID int) ([]Department, error)
	HasBindings(ctx context.Context, ids []int) (bool, error)
}

// DepartmentManagementService 承载部门管理的业务逻辑。
type DepartmentManagementService struct {
	Repository DepartmentRepository
	Logger     *slog.Logger
}

// Page 返回部门的分页列表。
func (s *DepartmentManagementService) Page(ctx context.Context, req DepartmentPageRequest) (DepartmentPageResult, error) {
	logger := s.logger()
	req = normalizeDepartmentPage(req)
	logger.Info("商户端开始分页查询部门", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name, "merchantId", req.MerchantID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("商户端分页查询部门失败", "error", err.Error())
		return DepartmentPageResult{}, err
	}
	logger.Info("商户端分页查询部门完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Save 新增或更新部门。
func (s *DepartmentManagementService) Save(ctx context.Context, dept Department) (Department, error) {
	logger := s.logger()
	normalized, err := normalizeDepartmentForSave(dept)
	if err != nil {
		logger.Warn("商户端保存部门参数无效", "id", dept.ID, "name", dept.Name, "error", err.Error())
		return Department{}, err
	}
	logger.Info("商户端开始保存部门信息", "id", normalized.ID, "name", normalized.Name, "merchantId", normalized.MerchantID, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("商户端保存部门失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return Department{}, err
	}
	logger.Info("商户端保存部门完成", "id", saved.ID, "name", saved.Name)
	return saved, nil
}

// Delete 逻辑删除部门。
func (s *DepartmentManagementService) Delete(ctx context.Context, ids []int) error {
	logger := s.logger()
	ids = filterPositiveIDs(ids)
	if len(ids) == 0 {
		return ErrInvalidDepartment
	}
	hasBindings, err := s.Repository.HasBindings(ctx, ids)
	if err != nil {
		logger.Error("商户端删除部门前检查绑定失败", "deptCount", len(ids), "error", err.Error())
		return err
	}
	if hasBindings {
		logger.Warn("商户端删除部门失败，部门仍被控制台账号或活动外呼任务引用", "deptCount", len(ids))
		return ErrDepartmentReferenced
	}
	logger.Info("商户端开始删除部门", "deptCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("商户端删除部门失败", "deptCount", len(ids), "error", err.Error())
		return err
	}
	logger.Info("商户端删除部门完成", "deptCount", len(ids))
	return nil
}

// ListAll 获取商户下的所有可用部门列表。
func (s *DepartmentManagementService) ListAll(ctx context.Context, merchantID int) ([]Department, error) {
	logger := s.logger()
	logger.Info("商户端获取所有可用部门列表", "merchantId", merchantID)
	return s.Repository.ListAll(ctx, merchantID)
}

func (s *DepartmentManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeDepartmentPage(req DepartmentPageRequest) DepartmentPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizeDepartmentForSave(dept Department) (Department, error) {
	dept.Name = strings.TrimSpace(dept.Name)
	dept.Description = strings.TrimSpace(dept.Description)
	if dept.Name == "" || dept.MerchantID <= 0 {
		return Department{}, ErrInvalidDepartment
	}
	return dept, nil
}
