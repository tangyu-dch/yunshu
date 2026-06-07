package callflow

import (
	"context"
	"testing"
)

func init() {
	// 在测试初始化时注册默认引擎
	RegisterDefaultLLMEngines()
}

// ============================================================================
// 1. Mock LLM 引擎测试
// ============================================================================

func TestMockLLMEngine_GenerateReply(t *testing.T) {
	// Mock 引擎在 ai_engine_providers.go 中已注册
	engine := GetLLMEngine("mock")
	ctx := context.Background()

	tests := []struct {
		name         string
		systemPrompt string
		userMessage  string
	}{
		{
			name:         "Test Hello Message",
			systemPrompt: "",
			userMessage:  "你好",
		},
		{
			name:         "Test Balance Query",
			systemPrompt: "",
			userMessage:  "查一下话费",
		},
		{
			name:         "Test Transfer Request",
			systemPrompt: "",
			userMessage:  "转人工",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.GenerateReply(ctx, tt.systemPrompt, tt.userMessage, nil)
			if err != nil {
				t.Errorf("GenerateReply() error = %v", err)
				return
			}

			if got == "" {
				t.Error("GenerateReply() returned empty string")
				return
			}
		})
	}
}

// ============================================================================
// 2. 引擎创建测试
// ============================================================================

func TestGetLLMEngine(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{
			name:     "Test Mock Provider",
			provider: "mock",
		},
		{
			name:     "Test OpenAI Provider",
			provider: "openai",
		},
		{
			name:     "Test Zhipu Provider",
			provider: "zhipu",
		},
		{
			name:     "Test GLM Provider",
			provider: "glm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := GetLLMEngine(tt.provider)
			if engine == nil {
				t.Errorf("GetLLMEngine() returned nil for provider %s", tt.provider)
			}
		})
	}
}

// ============================================================================
// 3. Zhipu 引擎测试（构造函数测试）
// ============================================================================

func TestZhipuLLMEngine_GenerateReply_RequiresAPIKey(t *testing.T) {
	engine := &ZhipuLLMEngine{}
	ctx := context.Background()

	_, err := engine.GenerateReply(ctx, "system", "hello", map[string]any{})
	if err == nil {
		t.Error("ZhipuLLMEngine.GenerateReply() should require API key")
	}
}
