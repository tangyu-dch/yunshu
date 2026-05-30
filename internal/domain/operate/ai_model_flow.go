package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
)

var (
	// ErrInvalidAIModelFlow 表示 AI 流程配置无效。
	ErrInvalidAIModelFlow = errors.New("invalid ai model flow")
	// ErrAIModelFlowNotFound 表示 AI 流程配置不存在。
	ErrAIModelFlowNotFound = errors.New("ai model flow not found")
)

// AIModelFlow 表示商户侧 AI 流程配置。
type AIModelFlow struct {
	ID          int       `json:"id,omitempty"`
	Name        string    `json:"name"`
	Prompt      string    `json:"prompt"`
	Published   bool      `json:"published"`
	Prechecked  bool      `json:"prechecked"`
	Description string    `json:"description,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

// AIModelFlowPageRequest 表示 AI 流程分页查询条件。
type AIModelFlowPageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
	Published  *bool  `json:"published,omitempty"`
}

// AIModelFlowPageResult 表示 AI 流程分页结果。
type AIModelFlowPageResult struct {
	PageNumber int           `json:"pageNumber"`
	PageSize   int           `json:"pageSize"`
	Total      int64         `json:"total"`
	Records    []AIModelFlow `json:"records"`
}

// AIModelFlowRepository 定义 AI 流程管理仓储。
type AIModelFlowRepository interface {
	Page(ctx context.Context, req AIModelFlowPageRequest) (AIModelFlowPageResult, error)
	GetByID(ctx context.Context, id int) (AIModelFlow, error)
	Save(ctx context.Context, flow AIModelFlow) (AIModelFlow, error)
	Delete(ctx context.Context, ids []int) error
	Precheck(ctx context.Context, flow AIModelFlow) (AIModelFlow, error)
	Publish(ctx context.Context, id int) (AIModelFlow, error)
}

// AIModelFlowManagementService 承载 AI 流程管理。
type AIModelFlowManagementService struct {
	Repository AIModelFlowRepository
	Logger     *slog.Logger
}

// Page 返回 AI 流程分页结果。
func (s *AIModelFlowManagementService) Page(ctx context.Context, req AIModelFlowPageRequest) (AIModelFlowPageResult, error) {
	logger := s.logger()
	req = normalizeAIModelFlowPage(req)
	logger.Info("商户端开始查询 AI 流程", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "name", req.Name)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("商户端查询 AI 流程失败", "error", err.Error())
		return AIModelFlowPageResult{}, err
	}
	logger.Info("商户端查询 AI 流程完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Save 保存 AI 流程配置。
func (s *AIModelFlowManagementService) Save(ctx context.Context, flow AIModelFlow) (AIModelFlow, error) {
	logger := s.logger()
	normalized, err := normalizeAIModelFlowForSave(flow)
	if err != nil {
		logger.Warn("商户端保存 AI 流程参数无效", "id", flow.ID, "name", flow.Name, "error", err.Error())
		return AIModelFlow{}, err
	}
	logger.Info("商户端开始保存 AI 流程", "id", normalized.ID, "name", normalized.Name, "published", normalized.Published)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("商户端保存 AI 流程失败", "id", normalized.ID, "name", normalized.Name, "error", err.Error())
		return AIModelFlow{}, err
	}
	logger.Info("商户端保存 AI 流程完成", "id", saved.ID, "name", saved.Name, "published", saved.Published)
	return saved, nil
}

// Delete 删除 AI 流程配置。
func (s *AIModelFlowManagementService) Delete(ctx context.Context, ids []int) error {
	logger := s.logger()
	ids = filterPositiveIDs(ids)
	if len(ids) == 0 {
		return ErrInvalidAIModelFlow
	}
	logger.Info("商户端开始删除 AI 流程", "flowCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("商户端删除 AI 流程失败", "flowCount", len(ids), "error", err.Error())
		return err
	}
	logger.Info("商户端删除 AI 流程完成", "flowCount", len(ids))
	return nil
}

// Precheck 预检查 AI 流程配置。
func (s *AIModelFlowManagementService) Precheck(ctx context.Context, flow AIModelFlow) (AIModelFlow, error) {
	logger := s.logger()
	normalized, err := normalizeAIModelFlowForSave(flow)
	if err != nil {
		logger.Warn("商户端预检查 AI 流程参数无效", "error", err.Error())
		return AIModelFlow{}, err
	}
	checked, err := s.Repository.Precheck(ctx, normalized)
	if err != nil {
		logger.Error("商户端预检查 AI 流程失败", "id", normalized.ID, "error", err.Error())
		return AIModelFlow{}, err
	}
	logger.Info("商户端预检查 AI 流程完成", "id", checked.ID, "prechecked", checked.Prechecked)
	return checked, nil
}

// Publish 发布 AI 流程配置。
func (s *AIModelFlowManagementService) Publish(ctx context.Context, id int) (AIModelFlow, error) {
	logger := s.logger()
	logger.Info("商户端开始发布 AI 流程", "id", id)
	flow, err := s.Repository.Publish(ctx, id)
	if err != nil {
		logger.Error("商户端发布 AI 流程失败", "id", id, "error", err.Error())
		return AIModelFlow{}, err
	}
	logger.Info("商户端发布 AI 流程完成", "id", id, "published", flow.Published)
	return flow, nil
}

func (s *AIModelFlowManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeAIModelFlowPage(req AIModelFlowPageRequest) AIModelFlowPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Name = strings.TrimSpace(req.Name)
	return req
}

func normalizeAIModelFlowForSave(flow AIModelFlow) (AIModelFlow, error) {
	flow.Name = strings.TrimSpace(flow.Name)
	flow.Prompt = strings.TrimSpace(flow.Prompt)
	if flow.Name == "" || flow.Prompt == "" {
		return AIModelFlow{}, ErrInvalidAIModelFlow
	}
	return flow, nil
}

// MemoryAIModelFlowRepository 是本地开发和测试使用的 AI 流程仓储。
type MemoryAIModelFlowRepository struct {
	mu     sync.Mutex
	nextID int
	flows  map[int]AIModelFlow
}

// NewMemoryAIModelFlowRepository 创建内存 AI 流程仓储。
func NewMemoryAIModelFlowRepository() *MemoryAIModelFlowRepository {
	return &MemoryAIModelFlowRepository{nextID: 1, flows: map[int]AIModelFlow{}}
}

func (r *MemoryAIModelFlowRepository) Page(_ context.Context, req AIModelFlowPageRequest) (AIModelFlowPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]AIModelFlow, 0, len(r.flows))
	for _, flow := range r.flows {
		if req.Name != "" && !strings.Contains(flow.Name, req.Name) {
			continue
		}
		if req.Published != nil && flow.Published != *req.Published {
			continue
		}
		records = append(records, flow)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []AIModelFlow{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return AIModelFlowPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryAIModelFlowRepository) GetByID(_ context.Context, id int) (AIModelFlow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	flow, ok := r.flows[id]
	if !ok {
		return AIModelFlow{}, ErrAIModelFlowNotFound
	}
	return flow, nil
}

func (r *MemoryAIModelFlowRepository) Save(_ context.Context, flow AIModelFlow) (AIModelFlow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if flow.ID == 0 {
		flow.ID = r.nextID
		r.nextID++
	}
	flow.UpdatedAt = time.Now().UTC()
	r.flows[flow.ID] = flow
	return flow, nil
}

func (r *MemoryAIModelFlowRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for _, id := range ids {
		if _, ok := r.flows[id]; ok {
			delete(r.flows, id)
			removed++
		}
	}
	if removed == 0 {
		return ErrAIModelFlowNotFound
	}
	return nil
}

func (r *MemoryAIModelFlowRepository) Precheck(ctx context.Context, flow AIModelFlow) (AIModelFlow, error) {
	flow.Prechecked = true
	return r.Save(ctx, flow)
}

func (r *MemoryAIModelFlowRepository) Publish(_ context.Context, id int) (AIModelFlow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	flow, ok := r.flows[id]
	if !ok {
		return AIModelFlow{}, ErrAIModelFlowNotFound
	}
	flow.Published = true
	flow.UpdatedAt = time.Now().UTC()
	r.flows[id] = flow
	return flow, nil
}
