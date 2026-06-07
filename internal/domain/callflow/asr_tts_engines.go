package callflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"yunshu/internal/contracts"
)

// ============================================================================
// 1. ASR 引擎实现
// ============================================================================

// MockASREngine Mock ASR 引擎（用于测试）
type MockASREngine struct {
}

// NewMockASREngine 创建 Mock ASR 引擎
func NewMockASREngine() *MockASREngine {
	return &MockASREngine{}
}

// Transcribe 转写音频
func (e *MockASREngine) Transcribe(ctx context.Context, audioData []byte, format string, config map[string]any) (string, error) {
	// 模拟处理延迟
	time.Sleep(100 * time.Millisecond)
	
	// 返回模拟的识别结果
	if len(audioData) > 0 {
		return "你好，我想查一下话费", nil
	}
	return "", fmt.Errorf("empty audio data")
}

// VolcASREngine 火山引擎 ASR 引擎
type VolcASREngine struct {
	config contracts.ASRProviderConfig
	client HTTPClient
}

// NewVolcASREngine 创建火山引擎 ASR 引擎
func NewVolcASREngine(config contracts.ASRProviderConfig) *VolcASREngine {
	return &VolcASREngine{
		config: config,
		client: GlobalHTTPClient,
	}
}

// Transcribe 转写音频
func (e *VolcASREngine) Transcribe(ctx context.Context, audioData []byte, format string, config map[string]any) (string, error) {
	endpoint := e.config.Endpoint
	if endpoint == "" {
		endpoint = "https://openspeech.bytedance.com/api/v1/tts/audio/query"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	// 添加音频文件
	part, err := writer.CreateFormFile("audio", "audio.pcm")
	if err != nil {
		return "", fmt.Errorf("create form file failed: %w", err)
	}
	part.Write(audioData)
	
	// 添加其他参数
	writer.WriteField("app_id", e.config.ID)
	writer.WriteField("format", format)
	writer.WriteField("sample_rate", "16000")
	writer.WriteField("language", "zh-CN")
	
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close writer failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, body)
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ASR API failed, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// 简化的响应解析
	var result struct {
		Result struct {
			Text string `json:"text"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response failed: %w", err)
	}

	return result.Result.Text, nil
}

// ============================================================================
// 2. ASR 管道管理器
// ============================================================================

// ASRPipelineManager ASR 管道管理器
type ASRPipelineManager struct {
	engines map[string]ASREngine
	mu      sync.RWMutex
}

var (
	globalASRPipelineManager *ASRPipelineManager
	asrPipelineOnce          sync.Once
)

// GetASRPipelineManager 获取 ASR 管道管理器单例
func GetASRPipelineManager() *ASRPipelineManager {
	asrPipelineOnce.Do(func() {
		globalASRPipelineManager = &ASRPipelineManager{
			engines: make(map[string]ASREngine),
		}
	})
	return globalASRPipelineManager
}

// RegisterEngine 注册引擎
func (m *ASRPipelineManager) RegisterEngine(id string, engine ASREngine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.engines[id] = engine
}

// GetEngine 获取引擎
func (m *ASRPipelineManager) GetEngine(id string) (ASREngine, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	engine, ok := m.engines[id]
	return engine, ok
}

// LoadFromConfig 从配置加载引擎
func (m *ASRPipelineManager) LoadFromConfig(config contracts.AIStreamPipelineConfig) error {
	for id, providerConfig := range config.ASRProviders {
		if !providerConfig.Enabled {
			continue
		}
		engine, err := CreateASREngineFromConfig(providerConfig)
		if err != nil {
			continue
		}
		m.RegisterEngine(id, engine)
	}
	return nil
}

// CreateASREngineFromConfig 根据配置创建 ASR 引擎
func CreateASREngineFromConfig(config contracts.ASRProviderConfig) (ASREngine, error) {
	switch strings.ToLower(config.Provider) {
	case "volc", "volcengine":
		return NewVolcASREngine(config), nil
	case "mock", "test":
		return NewMockASREngine(), nil
	default:
		return NewMockASREngine(), nil // 默认使用 Mock
	}
}

// RegisterDefaultASREngines 注册默认的 ASR 引擎
func RegisterDefaultASREngines() {
	mockEngine := NewMockASREngine()
	RegisterASREngine("mock", mockEngine)
	RegisterASREngine("test", mockEngine)
}

// ============================================================================
// 3. TTS 引擎实现
// ============================================================================

// MockTTSEngine Mock TTS 引擎
type MockTTSEngine struct {
}

// NewMockTTSEngine 创建 Mock TTS 引擎
func NewMockTTSEngine() *MockTTSEngine {
	return &MockTTSEngine{}
}

// Synthesize 合成音频
func (e *MockTTSEngine) Synthesize(ctx context.Context, text string, config map[string]any) ([]byte, error) {
	// 模拟合成延迟
	time.Sleep(200 * time.Millisecond)
	
	// 返回模拟的音频数据
	return []byte("mock audio data: " + text), nil
}

// VolcTTSEngine 火山引擎 TTS 引擎
type VolcTTSEngine struct {
	config contracts.TTSProviderConfig
	client HTTPClient
}

// NewVolcTTSEngine 创建火山引擎 TTS 引擎
func NewVolcTTSEngine(config contracts.TTSProviderConfig) *VolcTTSEngine {
	return &VolcTTSEngine{
		config: config,
		client: GlobalHTTPClient,
	}
}

// Synthesize 合成音频
func (e *VolcTTSEngine) Synthesize(ctx context.Context, text string, config map[string]any) ([]byte, error) {
	endpoint := e.config.Endpoint
	if endpoint == "" {
		endpoint = "https://openspeech.bytedance.com/api/v1/tts"
	}

	voiceType := e.config.VoiceType
	if config != nil && config["volcVoiceType"] != nil {
		if v, ok := config["volcVoiceType"].(string); ok && v != "" {
			voiceType = v
		}
	}

	reqBody := map[string]interface{}{
		"app_id":       e.config.ID,
		"text":         text,
		"voice_type":   voiceType,
		"encoding":     "mp3",
		"sample_rate":  16000,
		"speed_ratio":  e.config.Speed,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS API failed, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

// ============================================================================
// 4. TTS 管道管理器
// ============================================================================

// TTSPipelineManager TTS 管道管理器
type TTSPipelineManager struct {
	engines map[string]TTSEngine
	mu      sync.RWMutex
}

var (
	globalTTSPipelineManager *TTSPipelineManager
	ttsPipelineOnce          sync.Once
)

// GetTTSPipelineManager 获取 TTS 管道管理器单例
func GetTTSPipelineManager() *TTSPipelineManager {
	ttsPipelineOnce.Do(func() {
		globalTTSPipelineManager = &TTSPipelineManager{
			engines: make(map[string]TTSEngine),
		}
	})
	return globalTTSPipelineManager
}

// RegisterEngine 注册引擎
func (m *TTSPipelineManager) RegisterEngine(id string, engine TTSEngine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.engines[id] = engine
}

// GetEngine 获取引擎
func (m *TTSPipelineManager) GetEngine(id string) (TTSEngine, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	engine, ok := m.engines[id]
	return engine, ok
}

// LoadFromConfig 从配置加载引擎
func (m *TTSPipelineManager) LoadFromConfig(config contracts.AIStreamPipelineConfig) error {
	for id, providerConfig := range config.TTSProviders {
		if !providerConfig.Enabled {
			continue
		}
		engine, err := CreateTTSEngineFromConfig(providerConfig)
		if err != nil {
			continue
		}
		m.RegisterEngine(id, engine)
	}
	return nil
}

// CreateTTSEngineFromConfig 根据配置创建 TTS 引擎
func CreateTTSEngineFromConfig(config contracts.TTSProviderConfig) (TTSEngine, error) {
	switch strings.ToLower(config.Provider) {
	case "volc", "volcengine":
		return NewVolcTTSEngine(config), nil
	case "mock", "test":
		return NewMockTTSEngine(), nil
	default:
		return NewMockTTSEngine(), nil
	}
}

// RegisterDefaultTTSEngines 注册默认的 TTS 引擎
func RegisterDefaultTTSEngines() {
	mockEngine := NewMockTTSEngine()
	RegisterTTSEngine("mock", mockEngine)
	RegisterTTSEngine("test", mockEngine)
}

// ============================================================================
// 4. LLM Mock 引擎实现
// ============================================================================

// MockLLMEngine Mock LLM 引擎
type MockLLMEngine struct {
}

// NewMockLLMEngine 创建 Mock LLM 引擎
func NewMockLLMEngine() *MockLLMEngine {
	return &MockLLMEngine{}
}

// GenerateReply 生成回复
func (e *MockLLMEngine) GenerateReply(ctx context.Context, systemPrompt, userMessage string, config map[string]any) (string, error) {
	// 简单的模拟回复逻辑
	if strings.Contains(userMessage, "查") || strings.Contains(userMessage, "话费") || strings.Contains(userMessage, "余额") {
		return "好的，请问您的手机号是多少？我来帮您查询话费余额。", nil
	} else if strings.Contains(userMessage, "人工") || strings.Contains(userMessage, "客服") || strings.Contains(userMessage, "转接") {
		return "好的，我现在为您转接人工客服，请稍等。", nil
	} else {
		return "您好，这里是云枢AI助手，请问有什么可以帮您的？", nil
	}
}

// RegisterDefaultLLMEngines 注册默认的 LLM 引擎
func RegisterDefaultLLMEngines() {
	mockEngine := NewMockLLMEngine()
	RegisterLLMEngine("mock", mockEngine)
	RegisterLLMEngine("test", mockEngine)
}
