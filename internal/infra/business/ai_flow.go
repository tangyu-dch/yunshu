package business

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	operatedomain "yunshu/internal/domain/operate"
)

// AIModelFlowModel 映射 Go-native `merchant_ai_model_flow` 表。
//
// 该表用于管理端保存 AI 流程草稿、预检查结果和发布状态，不参与呼叫热路径。
type AIModelFlowModel struct {
	ID            int       `gorm:"column:id;primaryKey"`
	Name          string    `gorm:"column:name"`
	Prompt        string    `gorm:"column:prompt"`
	CustomReplies string    `gorm:"column:custom_replies;type:text"` // 新增自定义回复与按键编排链 JSON 文本
	FlowGraph     string    `gorm:"column:flow_graph;type:text"`     // 新增可视化网格流程图拓扑 JSON 文本
	Description   string    `gorm:"column:description"`
	Published     bool      `gorm:"column:published"`
	Prechecked    bool      `gorm:"column:prechecked"`
	DelFlag       bool      `gorm:"column:del_flag"`
	CreatedTime   time.Time `gorm:"column:created_time"`
	UpdatedTime   time.Time `gorm:"column:updated_time"`
}

// TableName 返回 AI 流程管理表名。
func (AIModelFlowModel) TableName() string {
	return "cc_biz_ai_flow"
}

// AIModelFlowRepository 基于 GORM 的 AI 流程管理仓储。
type AIModelFlowRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewAIModelFlowRepository 创建 AI 流程管理仓储。
func NewAIModelFlowRepository(db *gorm.DB, logger *slog.Logger) *AIModelFlowRepository {
	return &AIModelFlowRepository{DB: db, Logger: logger}
}

// Page 返回 AI 流程分页结果。
func (r *AIModelFlowRepository) Page(ctx context.Context, req operatedomain.AIModelFlowPageRequest) (operatedomain.AIModelFlowPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&AIModelFlowModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Published != nil {
		query = query.Where("published = ?", *req.Published)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operatedomain.AIModelFlowPageResult{}, err
	}
	var models []AIModelFlowModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operatedomain.AIModelFlowPageResult{}, err
	}
	records := make([]operatedomain.AIModelFlow, 0, len(models))
	for _, model := range models {
		records = append(records, aiModelFlowFromModel(model))
	}
	return operatedomain.AIModelFlowPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 读取单个未删除 AI 流程。
func (r *AIModelFlowRepository) GetByID(ctx context.Context, id int) (operatedomain.AIModelFlow, error) {
	var model AIModelFlowModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operatedomain.AIModelFlow{}, operatedomain.ErrAIModelFlowNotFound
	}
	return aiModelFlowFromModel(model), err
}

// Save 新增或更新 AI 流程。
func (r *AIModelFlowRepository) Save(ctx context.Context, flow operatedomain.AIModelFlow) (operatedomain.AIModelFlow, error) {
	r.logger().Info("开始保存 AI 话术流程配置", "id", flow.ID, "name", flow.Name)
	model := aiModelFlowToModel(flow)
	now := time.Now().UTC()
	model.UpdatedTime = now
	if model.ID == 0 {
		model.CreatedTime = now
	}
	tx := r.DB.WithContext(ctx)
	if model.ID != 0 {
		tx = tx.Omit("created_time")
	}
	if err := tx.Save(&model).Error; err != nil {
		r.logger().Error("保存 AI 话术流程配置失败", "name", flow.Name, "error", err.Error())
		return operatedomain.AIModelFlow{}, err
	}
	r.logger().Info("保存 AI 话术流程配置成功", "id", model.ID, "name", model.Name)
	return aiModelFlowFromModel(model), nil
}

// Delete 逻辑删除 AI 流程。
func (r *AIModelFlowRepository) Delete(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	r.logger().Info("开始批量逻辑删除 AI 话术流程", "ids", ids)
	result := r.DB.WithContext(ctx).Model(&AIModelFlowModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("批量逻辑删除 AI 话术流程失败", "ids", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("批量逻辑删除 AI 话术流程未匹配到有效记录", "ids", ids)
		return operatedomain.ErrAIModelFlowNotFound
	}
	r.logger().Info("批量逻辑删除 AI 话术流程成功", "ids", ids, "rowsAffected", result.RowsAffected)
	return nil
}

// Precheck 标记 AI 流程已预检查。
func (r *AIModelFlowRepository) Precheck(ctx context.Context, flow operatedomain.AIModelFlow) (operatedomain.AIModelFlow, error) {
	r.logger().Info("开始预检 AI 话术流程", "id", flow.ID, "name", flow.Name)
	flow.Prechecked = true
	res, err := r.Save(ctx, flow)
	if err != nil {
		r.logger().Error("预检 AI 话术流程状态保存失败", "id", flow.ID, "error", err.Error())
		return operatedomain.AIModelFlow{}, err
	}
	r.logger().Info("预检 AI 话术流程状态保存成功", "id", flow.ID)
	return res, nil
}

// Publish 标记 AI 流程已发布。
func (r *AIModelFlowRepository) Publish(ctx context.Context, id int) (operatedomain.AIModelFlow, error) {
	r.logger().Info("开始发布 AI 话术流程", "id", id)
	flow, err := r.GetByID(ctx, id)
	if err != nil {
		r.logger().Warn("发布 AI 话术流程失败：获取记录失败", "id", id, "error", err.Error())
		return operatedomain.AIModelFlow{}, err
	}
	flow.Published = true
	res, err := r.Save(ctx, flow)
	if err != nil {
		r.logger().Error("发布 AI 话术流程状态保存失败", "id", id, "error", err.Error())
		return operatedomain.AIModelFlow{}, err
	}
	r.logger().Info("发布 AI 话术流程状态保存成功", "id", id)
	return res, nil
}

// MemoryAIModelFlowRepository 是本地开发和测试使用的 AI 流程仓储。
type MemoryAIModelFlowRepository struct {
	mu     sync.Mutex
	nextID int
	flows  map[int]operatedomain.AIModelFlow
}

// NewMemoryAIModelFlowRepository 创建内存 AI 流程仓储。
func NewMemoryAIModelFlowRepository() *MemoryAIModelFlowRepository {
	return &MemoryAIModelFlowRepository{nextID: 1, flows: map[int]operatedomain.AIModelFlow{}}
}

func (r *MemoryAIModelFlowRepository) Page(_ context.Context, req operatedomain.AIModelFlowPageRequest) (operatedomain.AIModelFlowPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operatedomain.AIModelFlow, 0, len(r.flows))
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
		records = []operatedomain.AIModelFlow{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operatedomain.AIModelFlowPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryAIModelFlowRepository) GetByID(_ context.Context, id int) (operatedomain.AIModelFlow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	flow, ok := r.flows[id]
	if !ok {
		return operatedomain.AIModelFlow{}, operatedomain.ErrAIModelFlowNotFound
	}
	return flow, nil
}

func (r *MemoryAIModelFlowRepository) Save(_ context.Context, flow operatedomain.AIModelFlow) (operatedomain.AIModelFlow, error) {
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
		return operatedomain.ErrAIModelFlowNotFound
	}
	return nil
}

func (r *MemoryAIModelFlowRepository) Precheck(ctx context.Context, flow operatedomain.AIModelFlow) (operatedomain.AIModelFlow, error) {
	flow.Prechecked = true
	return r.Save(ctx, flow)
}

func (r *MemoryAIModelFlowRepository) Publish(_ context.Context, id int) (operatedomain.AIModelFlow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	flow, ok := r.flows[id]
	if !ok {
		return operatedomain.AIModelFlow{}, operatedomain.ErrAIModelFlowNotFound
	}
	flow.Published = true
	flow.UpdatedAt = time.Now().UTC()
	r.flows[id] = flow
	return flow, nil
}

func aiModelFlowToModel(flow operatedomain.AIModelFlow) AIModelFlowModel {
	customRepliesJSON, err := json.Marshal(flow.CustomReplies)
	if err != nil {
		slog.Warn("AI 话术流自定义回复序列化失败", "name", flow.Name, "error", err.Error())
		customRepliesJSON = []byte("[]")
	}

	var flowGraphJSON []byte
	if flow.FlowGraph != nil {
		var err error
		flowGraphJSON, err = json.Marshal(flow.FlowGraph)
		if err != nil {
			slog.Warn("AI 话术流可视化拓扑图序列化失败", "name", flow.Name, "error", err.Error())
			flowGraphJSON = []byte("{}")
		}
	} else {
		flowGraphJSON = []byte("{}")
	}

	return AIModelFlowModel{
		ID:            flow.ID,
		Name:          flow.Name,
		Prompt:        flow.Prompt,
		CustomReplies: string(customRepliesJSON),
		FlowGraph:     string(flowGraphJSON),
		Description:   flow.Description,
		Published:     flow.Published,
		Prechecked:    flow.Prechecked,
	}
}

func aiModelFlowFromModel(model AIModelFlowModel) operatedomain.AIModelFlow {
	var customReplies []operatedomain.CustomReplyRule
	if model.CustomReplies != "" {
		if err := json.Unmarshal([]byte(model.CustomReplies), &customReplies); err != nil {
			slog.Error("AI 话术流自定义回复反序列化失败", "id", model.ID, "error", err.Error())
			customReplies = []operatedomain.CustomReplyRule{}
		}
	} else {
		customReplies = []operatedomain.CustomReplyRule{}
	}

	var flowGraph *operatedomain.AIFlowGraph
	if model.FlowGraph != "" && model.FlowGraph != "{}" {
		var graph operatedomain.AIFlowGraph
		if err := json.Unmarshal([]byte(model.FlowGraph), &graph); err != nil {
			slog.Error("AI 话术流可视化拓扑图反序列化失败", "id", model.ID, "error", err.Error())
			flowGraph = nil
		} else {
			flowGraph = &graph
		}
	}

	return operatedomain.AIModelFlow{
		ID:            model.ID,
		Name:          model.Name,
		Prompt:        model.Prompt,
		CustomReplies: customReplies,
		FlowGraph:     flowGraph,
		Published:     model.Published,
		Prechecked:    model.Prechecked,
		Description:   model.Description,
		UpdatedAt:     model.UpdatedTime,
	}
}

func (r *AIModelFlowRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
