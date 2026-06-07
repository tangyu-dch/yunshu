package callflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"yunshu/internal/contracts"
)

// ============================================================================
// AI 流处理管道接口定义
// ============================================================================

// AIStreamPipeline AI 流处理管道接口
type AIStreamPipeline interface {
	StartSession(callID, customerUUID, fsAddr string, config map[string]interface{}) (*contracts.AudioStreamSession, error)
	StopSession(sessionID string) error
	GetEventChannel() <-chan contracts.StreamEvent
	ProcessAudioChunk(ctx context.Context, sessionID string, chunk contracts.AudioChunk) error
	ProcessASRText(ctx context.Context, sessionID string, text string, isFinal bool) error
	SwitchLLMProvider(sessionID, llmID string) error
	SwitchASRProvider(sessionID, asrID string) error
	SwitchTTSProvider(sessionID, ttsID string) error
}

// MockPipeline Mock AI 流处理管道
type MockPipeline struct {
	eventChan chan contracts.StreamEvent
}

// NewMockPipeline 创建 Mock 管道
func NewMockPipeline() *MockPipeline {
	return &MockPipeline{
		eventChan: make(chan contracts.StreamEvent, 100),
	}
}

// StartSession 启动会话
func (p *MockPipeline) StartSession(callID, customerUUID, fsAddr string, config map[string]interface{}) (*contracts.AudioStreamSession, error) {
	session := &contracts.AudioStreamSession{
		SessionID:    fmt.Sprintf("session-%s-%d", callID, time.Now().Unix()),
		CallID:      callID,
		CustomerUUID: customerUUID,
		Status:      "streaming",
		StartTime:   time.Now(),
		Metadata:    config,
	}
	return session, nil
}

// StopSession 停止会话
func (p *MockPipeline) StopSession(sessionID string) error {
	return nil
}

// GetEventChannel 获取事件通道
func (p *MockPipeline) GetEventChannel() <-chan contracts.StreamEvent {
	return p.eventChan
}

// ProcessAudioChunk 处理音频数据块
func (p *MockPipeline) ProcessAudioChunk(ctx context.Context, sessionID string, chunk contracts.AudioChunk) error {
	// Mock 实现：发布音频处理事件
	event := contracts.StreamEvent{
		Type:      "audio_processed",
		Data:      map[string]interface{}{"sessionID": sessionID},
		Timestamp: time.Now(),
	}
	p.PublishEvent(event)
	return nil
}

// ProcessASRText 处理 ASR 文本
func (p *MockPipeline) ProcessASRText(ctx context.Context, sessionID string, text string, isFinal bool) error {
	// Mock 实现：发布 ASR 结果事件
	event := contracts.StreamEvent{
		Type: "asr_result",
		Data: map[string]interface{}{
			"sessionID": sessionID,
			"text":      text,
			"isFinal":   isFinal,
		},
		Timestamp: time.Now(),
	}
	p.PublishEvent(event)
	return nil
}

// SwitchLLMProvider 切换 LLM 提供商
func (p *MockPipeline) SwitchLLMProvider(sessionID, llmID string) error {
	// Mock 实现：发布切换事件
	event := contracts.StreamEvent{
		Type: "llm_switched",
		Data: map[string]interface{}{
			"sessionID": sessionID,
			"llmID":    llmID,
		},
		Timestamp: time.Now(),
	}
	p.PublishEvent(event)
	return nil
}

// SwitchASRProvider 切换 ASR 提供商
func (p *MockPipeline) SwitchASRProvider(sessionID, asrID string) error {
	// Mock 实现：发布切换事件
	event := contracts.StreamEvent{
		Type: "asr_switched",
		Data: map[string]interface{}{
			"sessionID": sessionID,
			"asrID":    asrID,
		},
		Timestamp: time.Now(),
	}
	p.PublishEvent(event)
	return nil
}

// SwitchTTSProvider 切换 TTS 提供商
func (p *MockPipeline) SwitchTTSProvider(sessionID, ttsID string) error {
	// Mock 实现：发布切换事件
	event := contracts.StreamEvent{
		Type: "tts_switched",
		Data: map[string]interface{}{
			"sessionID": sessionID,
			"ttsID":    ttsID,
		},
		Timestamp: time.Now(),
	}
	p.PublishEvent(event)
	return nil
}

// PublishEvent 发布事件
func (p *MockPipeline) PublishEvent(event contracts.StreamEvent) {
	select {
	case p.eventChan <- event:
	default:
		// 通道满，跳过
	}
}

// ============================================================================
// WebSocket 消息类型定义
// ============================================================================

// WSMessage WebSocket 消息结构
type WSMessage struct {
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data"`
	Session string          `json:"session,omitempty"`
}

// WSAudioData WebSocket 音频数据
type WSAudioData struct {
	Data       []byte `json:"data"`
	Format     string `json:"format"`
	SampleRate int    `json:"sampleRate"`
}

// WSTextData WebSocket 文本数据
type WSTextData struct {
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

// ============================================================================
// AI 流程管理器
// ============================================================================

var (
	globalPipeline     AIStreamPipeline
	globalPipelineOnce sync.Once
	globalPipelineMu   sync.RWMutex
)

// SetGlobalPipeline 设置全局 AI 流程管道
func SetGlobalPipeline(pipeline AIStreamPipeline) {
	globalPipelineMu.Lock()
	globalPipeline = pipeline
	globalPipelineMu.Unlock()
}

// GetGlobalPipeline 获取全局 AI 流程管道
func GetGlobalPipeline() AIStreamPipeline {
	globalPipelineMu.RLock()
	p := globalPipeline
	globalPipelineMu.RUnlock()
	if p != nil {
		return p
	}
	globalPipelineOnce.Do(func() {
		globalPipelineMu.Lock()
		if globalPipeline == nil {
			globalPipeline = NewMockPipeline()
		}
		globalPipelineMu.Unlock()
	})
	globalPipelineMu.RLock()
	p = globalPipeline
	globalPipelineMu.RUnlock()
	return p
}

// InitAIPipeline 初始化 AI 流程管道
func InitAIPipeline() error {
	// 创建 Mock 管道作为默认实现
	pipeline := NewMockPipeline()
	SetGlobalPipeline(pipeline)
	return nil
}
