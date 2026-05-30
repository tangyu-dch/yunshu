package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

var (
	// ErrInvalidBatchTask 表示批量任务参数无效。
	ErrInvalidBatchTask = errors.New("invalid batch task")
	// ErrBatchTaskNotFound 表示批量任务不存在。
	ErrBatchTaskNotFound = errors.New("batch task not found")
)

// BatchTask 表示商户侧批量外呼任务。
type BatchTask struct {
	ID                  int        `json:"id,omitempty"`
	MerchantID          int        `json:"merchantId"`
	UserID              int        `json:"userId"`
	Name                string     `json:"name"`
	State               int        `json:"state"`
	LastStartTime       *time.Time `json:"lastStartTime,omitempty"`
	ConnectedInterval   int        `json:"connectedInterval"`
	UnconnectedInterval int        `json:"unconnectedInterval"`
	CallTimePeriod      string     `json:"callTimePeriod,omitempty"`
	TerminationTime     *time.Time `json:"terminationTime,omitempty"`
	TotalCount          int        `json:"totalCount"`
	CalledCount         int        `json:"calledCount"`
	ConnectedCount      int        `json:"connectedCount"`
	AIFlag              bool       `json:"aiFlag"`
	Extra               string     `json:"extra,omitempty"`
	PausedReason        string     `json:"pausedReason,omitempty"`
	Enable              bool       `json:"enable"`
}

// BatchTaskPageRequest 表示批量任务分页查询条件。
type BatchTaskPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
	MerchantID int    `json:"merchantId,omitempty"`
	UserID     int    `json:"userId,omitempty"`
	Enable     *bool  `json:"enable,omitempty"`
}

// BatchTaskPageResult 表示批量任务分页结果。
type BatchTaskPageResult struct {
	PageNumber int         `json:"pageNumber"`
	PageSize   int         `json:"pageSize"`
	Total      int64       `json:"total"`
	Records    []BatchTask `json:"records"`
}

// BatchTaskDetail 表示批量外呼号码拨打明细。
type BatchTaskDetail struct {
	ID            int        `json:"id"`
	TaskID        int        `json:"taskId"`
	MerchantID    int        `json:"merchantId"`
	UserID        int        `json:"userId"`
	CustomerName  string     `json:"customerName"`
	Tel           string     `json:"tel"`
	TelView       string     `json:"telView"`
	CallTime      *time.Time `json:"callTime"`
	CallStatus    int        `json:"callStatus"`
	ConnectStatus *bool      `json:"connectStatus"`
	Extra         string     `json:"extra"`
	DurationSec   int        `json:"durationSec"`
}

// BatchTaskRepository 定义商户批量任务管理仓储能力。
type BatchTaskRepository interface {
	Page(ctx context.Context, req BatchTaskPageRequest) (BatchTaskPageResult, error)
	GetByID(ctx context.Context, id int) (BatchTask, error)
	Save(ctx context.Context, task BatchTask) (BatchTask, error)
	Delete(ctx context.Context, ids []int) error
	SetEnable(ctx context.Context, id int, enable bool, pausedReason string) (BatchTask, error)
	ImportTels(ctx context.Context, taskID int, merchantID int, userID int, tels []string) error
	GetDetails(ctx context.Context, taskID int) ([]BatchTaskDetail, error)
}

// BatchTaskManagementService 承载批量任务管理。
type BatchTaskManagementService struct {
	Repository BatchTaskRepository
	Logger     *slog.Logger
}

// Page 返回批量任务分页结果。
func (s *BatchTaskManagementService) Page(ctx context.Context, req BatchTaskPageRequest) (BatchTaskPageResult, error) {
	logger := s.logger()
	req = normalizeBatchTaskPage(req)
	logger.Info("商户端开始查询批量外呼任务", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name, "merchantId", req.MerchantID, "userId", req.UserID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("商户端查询批量外呼任务失败", "error", err.Error())
		return BatchTaskPageResult{}, err
	}
	logger.Info("商户端查询批量外呼任务完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Save 保存批量外呼任务。
func (s *BatchTaskManagementService) Save(ctx context.Context, task BatchTask) (BatchTask, error) {
	logger := s.logger()
	normalized, err := normalizeBatchTaskForSave(task)
	if err != nil {
		logger.Warn("商户端保存批量外呼任务参数无效", "id", task.ID, "name", task.Name, "error", err.Error())
		return BatchTask{}, err
	}
	logger.Info("商户端开始保存批量外呼任务", "id", normalized.ID, "name", normalized.Name, "merchantId", normalized.MerchantID, "userId", normalized.UserID, "enable", normalized.Enable)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("商户端保存批量外呼任务失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return BatchTask{}, err
	}
	logger.Info("商户端保存批量外呼任务完成", "id", saved.ID, "name", saved.Name, "enable", saved.Enable)
	return saved, nil
}

// Delete 逻辑删除批量外呼任务。
func (s *BatchTaskManagementService) Delete(ctx context.Context, ids []int) error {
	logger := s.logger()
	ids = filterPositiveIDs(ids)
	if len(ids) == 0 {
		return ErrInvalidBatchTask
	}
	logger.Info("商户端开始删除批量外呼任务", "taskCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("商户端删除批量外呼任务失败", "taskCount", len(ids), "error", err.Error())
		return err
	}
	logger.Info("商户端删除批量外呼任务完成", "taskCount", len(ids))
	return nil
}

// SetEnable 切换批量外呼任务启用状态。
func (s *BatchTaskManagementService) SetEnable(ctx context.Context, id int, enable bool, pausedReason string) (BatchTask, error) {
	logger := s.logger()
	logger.Info("商户端开始切换批量外呼任务启用状态", "id", id, "enable", enable)
	task, err := s.Repository.SetEnable(ctx, id, enable, strings.TrimSpace(pausedReason))
	if err != nil {
		logger.Error("商户端切换批量外呼任务启用状态失败", "id", id, "enable", enable, "error", err.Error())
		return BatchTask{}, err
	}
	logger.Info("商户端切换批量外呼任务启用状态完成", "id", id, "enable", task.Enable, "pausedReason", task.PausedReason)
	return task, nil
}

// ImportTels 批量导入电话号码。
func (s *BatchTaskManagementService) ImportTels(ctx context.Context, taskID int, merchantID int, userID int, tels []string) error {
	s.logger().Info("商户端开始导入批量外呼号码", "taskId", taskID, "telsCount", len(tels))
	return s.Repository.ImportTels(ctx, taskID, merchantID, userID, tels)
}

// GetDetails 查询拨打明细。
func (s *BatchTaskManagementService) GetDetails(ctx context.Context, taskID int) ([]BatchTaskDetail, error) {
	s.logger().Info("商户端查询号码拨打明细", "taskId", taskID)
	return s.Repository.GetDetails(ctx, taskID)
}

func (s *BatchTaskManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeBatchTaskPage(req BatchTaskPageRequest) BatchTaskPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizeBatchTaskForSave(task BatchTask) (BatchTask, error) {
	task.Name = strings.TrimSpace(task.Name)
	if task.Name == "" || task.MerchantID <= 0 || task.UserID <= 0 {
		return BatchTask{}, ErrInvalidBatchTask
	}
	if task.State < 0 {
		task.State = 0
	}
	return task, nil
}

func filterPositiveIDs(ids []int) []int {
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			out = append(out, id)
		}
	}
	return out
}
