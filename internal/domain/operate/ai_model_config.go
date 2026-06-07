package operate

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

var (
	// ErrAIModelConfigNotFound 表示 AI 模型配置不存在。
	ErrAIModelConfigNotFound = errors.New("ai model config not found")
)

// AIModelConfig 表示 AI 大模型商户配置实体。
type AIModelConfig struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`           // 配置显示名称，如 "DeepSeek 官方客服"
	Provider       string    `json:"provider"`       // 服务商: "DeepSeek" | "OpenAI" | "Cloud枢私有大模型"
	ModelName      string    `json:"modelName"`      // 模型名称，如 "deepseek-chat", "gpt-4o"
	Endpoint       string    `json:"endpoint"`       // API 网关代理地址
	ApiKey         string    `json:"apiKey"`         // 密钥 API Key
	Temperature    float64   `json:"temperature"`    // 温度参数 (0.0 - 1.5)
	SystemPrompt   string    `json:"systemPrompt"`   // 全局角色设定 Prompt
	Description    string    `json:"description"`    // 备注描述
	VolcAppId      string    `json:"volcAppId"`      // 火山语音 AppId
	VolcToken      string    `json:"volcToken"`      // 火山语音 Token
	VolcCluster    string    `json:"volcCluster"`    // 火山语音 Cluster
	VolcVoiceType  string    `json:"volcVoiceType"`  // 火山语音 TTS 发音人音色
	VolcSpeedRatio float64   `json:"volcSpeedRatio"` // 火山语音 TTS 语速
	UpdatedAt      time.Time `json:"updatedAt"`
}

// AIModelConfigRepository 定义 AI 大模型配置持久化仓储接口。
type AIModelConfigRepository interface {
	List(ctx context.Context) ([]AIModelConfig, error)
	GetByID(ctx context.Context, id int) (AIModelConfig, error)
	Save(ctx context.Context, config AIModelConfig) (AIModelConfig, error)
	Delete(ctx context.Context, ids []int) error
}

// AIModelConfigManagementService 承载 AI 大模型配置的业务流程管理。
type AIModelConfigManagementService struct {
	Repository AIModelConfigRepository
	Logger     *slog.Logger
}

// List 返回所有未删除的 AI 模型配置。
func (s *AIModelConfigManagementService) List(ctx context.Context) ([]AIModelConfig, error) {
	return s.Repository.List(ctx)
}

// GetByID 根据 ID 查询单个配置。
func (s *AIModelConfigManagementService) GetByID(ctx context.Context, id int) (AIModelConfig, error) {
	return s.Repository.GetByID(ctx, id)
}

// Save 新增或更新 AI 模型配置。
func (s *AIModelConfigManagementService) Save(ctx context.Context, config AIModelConfig) (AIModelConfig, error) {
	if config.Name == "" {
		return AIModelConfig{}, errors.New("config name cannot be empty")
	}
	if config.Provider == "" {
		return AIModelConfig{}, errors.New("provider cannot be empty")
	}
	if config.ModelName == "" {
		if config.Provider == "DeepSeek" {
			config.ModelName = "deepseek-chat"
		} else if config.Provider == "OpenAI" {
			config.ModelName = "gpt-4o"
		} else if config.Provider == "Zhipu" || config.Provider == "智谱" || config.Provider == "glm" {
			config.ModelName = "glm-4"
			if config.Endpoint == "" {
				config.Endpoint = "https://open.bigmodel.cn/api/paas/v4/chat/completions"
			}
		} else {
			config.ModelName = "cloudshu-v1"
		}
	}
	if config.Temperature <= 0 {
		config.Temperature = 0.7
	}
	return s.Repository.Save(ctx, config)
}

// Delete 批量逻辑删除配置。
func (s *AIModelConfigManagementService) Delete(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return s.Repository.Delete(ctx, ids)
}
