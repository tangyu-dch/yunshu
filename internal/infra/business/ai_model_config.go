package business

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	operatedomain "yunshu/internal/domain/operate"
)

// AIModelConfigModel 映射 `cc_biz_ai_model_config` 数据库表。
type AIModelConfigModel struct {
	ID           int       `gorm:"column:id;primaryKey"`
	Name         string    `gorm:"column:name"`
	Provider     string    `gorm:"column:provider"`
	ModelName    string    `gorm:"column:model_name"`
	Endpoint     string    `gorm:"column:endpoint"`
	ApiKey       string    `gorm:"column:api_key"`
	Temperature  float64   `gorm:"column:temperature"`
	SystemPrompt string    `gorm:"column:system_prompt;type:text"`
	Description  string    `gorm:"column:description"`
	DelFlag      bool      `gorm:"column:del_flag"`
	CreatedTime  time.Time `gorm:"column:created_time"`
	UpdatedTime  time.Time `gorm:"column:updated_time"`
}

// TableName 返回 AI 模型配置表名。
func (AIModelConfigModel) TableName() string {
	return "cc_biz_ai_model_config"
}

// AIModelConfigRepository 基于 GORM 的 AI 模型配置仓储。
type AIModelConfigRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewAIModelConfigRepository 创建 AI 模型配置仓储。
func NewAIModelConfigRepository(db *gorm.DB, logger *slog.Logger) *AIModelConfigRepository {
	return &AIModelConfigRepository{DB: db, Logger: logger}
}

// List 返回所有未删除的 AI 模型配置。
func (r *AIModelConfigRepository) List(ctx context.Context) ([]operatedomain.AIModelConfig, error) {
	var models []AIModelConfigModel
	err := r.DB.WithContext(ctx).Where("del_flag = ?", false).Order("id DESC").Find(&models).Error
	if err != nil {
		return nil, err
	}
	configs := make([]operatedomain.AIModelConfig, 0, len(models))
	for _, m := range models {
		configs = append(configs, aiModelConfigFromModel(m))
	}
	return configs, nil
}

// GetByID 根据 ID 查询配置。
func (r *AIModelConfigRepository) GetByID(ctx context.Context, id int) (operatedomain.AIModelConfig, error) {
	var model AIModelConfigModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operatedomain.AIModelConfig{}, operatedomain.ErrAIModelConfigNotFound
	}
	return aiModelConfigFromModel(model), err
}

// Save 新增或更新 AI 模型配置。
func (r *AIModelConfigRepository) Save(ctx context.Context, config operatedomain.AIModelConfig) (operatedomain.AIModelConfig, error) {
	model := aiModelConfigToModel(config)
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
		return operatedomain.AIModelConfig{}, err
	}
	return aiModelConfigFromModel(model), nil
}

// Delete 批量逻辑删除配置。
func (r *AIModelConfigRepository) Delete(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	err := r.DB.WithContext(ctx).Model(&AIModelConfigModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()}).Error
	return err
}

// MemoryAIModelConfigRepository 内存 AI 模型配置仓储（用于本地测试或未连接 DB 时）。
type MemoryAIModelConfigRepository struct {
	mu      sync.Mutex
	nextID  int
	configs map[int]operatedomain.AIModelConfig
}

// NewMemoryAIModelConfigRepository 创建内存 AI 模型配置仓储。
func NewMemoryAIModelConfigRepository() *MemoryAIModelConfigRepository {
	repo := &MemoryAIModelConfigRepository{nextID: 1, configs: map[int]operatedomain.AIModelConfig{}}
	// 预设默认的 AI 厂商模板，方便开箱即用
	repo.configs[1] = operatedomain.AIModelConfig{
		ID:           1,
		Name:         "云枢私有大模型 (官方默认)",
		Provider:     "Cloud枢私有大模型",
		ModelName:    "cloudshu-v1",
		Temperature:  0.7,
		SystemPrompt: "您是云枢智能客服话务员，请根据用户的咨询礼貌作答。",
		Description:  "系统预设官方默认模型，可开箱即用",
	}
	repo.nextID = 2
	return repo
}

func (r *MemoryAIModelConfigRepository) List(_ context.Context) ([]operatedomain.AIModelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := make([]operatedomain.AIModelConfig, 0, len(r.configs))
	for _, c := range r.configs {
		list = append(list, c)
	}
	return list, nil
}

func (r *MemoryAIModelConfigRepository) GetByID(_ context.Context, id int) (operatedomain.AIModelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.configs[id]
	if !ok {
		return operatedomain.AIModelConfig{}, operatedomain.ErrAIModelConfigNotFound
	}
	return c, nil
}

func (r *MemoryAIModelConfigRepository) Save(_ context.Context, config operatedomain.AIModelConfig) (operatedomain.AIModelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if config.ID == 0 {
		config.ID = r.nextID
		r.nextID++
	}
	config.UpdatedAt = time.Now().UTC()
	r.configs[config.ID] = config
	return config, nil
}

func (r *MemoryAIModelConfigRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		delete(r.configs, id)
	}
	return nil
}

func aiModelConfigFromModel(m AIModelConfigModel) operatedomain.AIModelConfig {
	return operatedomain.AIModelConfig{
		ID:           m.ID,
		Name:         m.Name,
		Provider:     m.Provider,
		ModelName:    m.ModelName,
		Endpoint:     m.Endpoint,
		ApiKey:       m.ApiKey,
		Temperature:  m.Temperature,
		SystemPrompt: m.SystemPrompt,
		Description:  m.Description,
		UpdatedAt:    m.UpdatedTime,
	}
}

func aiModelConfigToModel(c operatedomain.AIModelConfig) AIModelConfigModel {
	return AIModelConfigModel{
		ID:           c.ID,
		Name:         c.Name,
		Provider:     c.Provider,
		ModelName:    c.ModelName,
		Endpoint:     c.Endpoint,
		ApiKey:       c.ApiKey,
		Temperature:  c.Temperature,
		SystemPrompt: c.SystemPrompt,
		Description:  c.Description,
	}
}
