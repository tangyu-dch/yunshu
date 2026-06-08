package callflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/rag"
)

// ============================================================================
// 完整的 AI 流处理管道实现
// ============================================================================

// AIPipelineConfig AI 流处理管道配置
type AIPipelineConfig struct {
	LLMConfig  contracts.AIStreamPipelineConfig
	RAGEnabled bool
	RAGConfig  rag.RAGConfig
}

// AIPipeline 完整的 AI 流处理管道
type AIPipeline struct {
	config          AIPipelineConfig
	sessions        map[string]*AISession
	sessionsMu      sync.RWMutex
	ragEngine       *rag.RAGEngine
	eventChan       chan contracts.StreamEvent
	conversationMgr *ConversationManager
	logger          *slog.Logger
}

// AISession AI 会话状态，自带互斥锁保护并发访问
type AISession struct {
	mu           sync.Mutex
	sessionID    string
	callID       string
	customerUUID string
	fsAddr       string
	status       string
	startTime    time.Time
	llmID        string
	asrID        string
	ttsID        string
	conversation *contracts.ConversationHistory
	metadata     map[string]interface{}
	ragEnabled   bool
	ragThreshold float64
}

// NewAIPipeline 创建新的 AI 流处理管道
func NewAIPipeline(config AIPipelineConfig) *AIPipeline {
	return &AIPipeline{
		config:          config,
		sessions:        make(map[string]*AISession),
		eventChan:       make(chan contracts.StreamEvent, 1000),
		conversationMgr: GetConversationManager(),
		logger:          slog.Default(),
	}
}

// SetRAGEngine 设置 RAG 引擎
func (p *AIPipeline) SetRAGEngine(engine *rag.RAGEngine) {
	p.ragEngine = engine
}

func stringFromMapSafe(m map[string]interface{}, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// StartSession 启动会话
func (p *AIPipeline) StartSession(callID, customerUUID, fsAddr string, config map[string]interface{}) (*contracts.AudioStreamSession, error) {
	sessionID := fmt.Sprintf("session-%s-%d", callID, time.Now().UnixNano())

	llmID, ok := stringFromMapSafe(config, "llmID")
	if !ok || llmID == "" {
		return nil, fmt.Errorf("config missing required field: llmID")
	}
	asrID, ok := stringFromMapSafe(config, "asrID")
	if !ok || asrID == "" {
		return nil, fmt.Errorf("config missing required field: asrID")
	}
	ttsID, ok := stringFromMapSafe(config, "ttsID")
	if !ok || ttsID == "" {
		return nil, fmt.Errorf("config missing required field: ttsID")
	}
	ragEnabled := false
	if v, ok := config["ragEnabled"].(bool); ok {
		ragEnabled = v
	}
	ragThreshold := 0.7
	if v, ok := config["ragThreshold"].(float64); ok {
		ragThreshold = v
	}

	session := &AISession{
		sessionID:    sessionID,
		callID:       callID,
		customerUUID: customerUUID,
		fsAddr:       fsAddr,
		status:       "streaming",
		startTime:    time.Now(),
		llmID:        llmID,
		asrID:        asrID,
		ttsID:        ttsID,
		conversation: &contracts.ConversationHistory{
			CallID:    callID,
			Messages:  []contracts.LLMMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		metadata:     config,
		ragEnabled:   ragEnabled,
		ragThreshold: ragThreshold,
	}

	p.sessionsMu.Lock()
	p.sessions[sessionID] = session
	p.sessionsMu.Unlock()

	p.conversationMgr.InitConversation(callID)

	p.publishEvent(contracts.StreamEvent{
		Type: "session_started",
		Data: map[string]interface{}{
			"sessionID": sessionID,
			"callID":    callID,
		},
		Timestamp: time.Now(),
	})

	return &contracts.AudioStreamSession{
		SessionID:    sessionID,
		CallID:       callID,
		CustomerUUID: customerUUID,
		Status:       "streaming",
		StartTime:    session.startTime,
		LLMConfigID:  llmID,
		ASRConfigID:  asrID,
		TTSConfigID:  ttsID,
		Metadata:     config,
	}, nil
}

// StopSession 停止会话
func (p *AIPipeline) StopSession(sessionID string) error {
	p.sessionsMu.Lock()
	session, exists := p.sessions[sessionID]
	if exists {
		session.mu.Lock()
		session.status = "stopped"
		session.mu.Unlock()
		delete(p.sessions, sessionID)
	}
	p.sessionsMu.Unlock()

	if exists {
		p.publishEvent(contracts.StreamEvent{
			Type: "session_stopped",
			Data: map[string]interface{}{
				"sessionID": sessionID,
				"callID":    session.callID,
			},
			Timestamp: time.Now(),
		})
	}

	return nil
}

// Close 关闭管道，释放资源
func (p *AIPipeline) Close() {
	close(p.eventChan)
}

// GetEventChannel 获取事件通道
func (p *AIPipeline) GetEventChannel() <-chan contracts.StreamEvent {
	return p.eventChan
}

// ProcessAudioChunk 处理音频数据块
func (p *AIPipeline) ProcessAudioChunk(ctx context.Context, sessionID string, chunk contracts.AudioChunk) error {
	session, err := p.getSession(sessionID)
	if err != nil {
		return err
	}

	session.mu.Lock()
		session.mu.Lock()
		_ = session.asrID // TODO: 实现实时音频流 ASR 处理
		session.mu.Unlock()

	return nil
}

// ProcessASRText 处理 ASR 文本
func (p *AIPipeline) ProcessASRText(ctx context.Context, sessionID string, text string, isFinal bool) error {
	_, err := p.getSession(sessionID)
	if err != nil {
		return err
	}

	p.publishEvent(contracts.StreamEvent{
		Type: "asr_result",
		Data: map[string]interface{}{
			"sessionID": sessionID,
			"text":      text,
			"isFinal":   isFinal,
		},
		Timestamp: time.Now(),
	})

	if isFinal {
		// 检查 session 仍存活再启动后台协程
		p.sessionsMu.RLock()
		_, alive := p.sessions[sessionID]
		p.sessionsMu.RUnlock()
		if alive {
			go p.processFinalText(ctx, sessionID, text)
		}
	}

	return nil
}

// processFinalText 处理最终的 ASR 文本，每次操作前重新获取 session 并加锁
func (p *AIPipeline) processFinalText(ctx context.Context, sessionID string, text string) {
	session := p.getSessionSync(sessionID)
	if session == nil {
		return
	}

	session.mu.Lock()
	session.conversation.AddUserMessage(text)
	callID := session.callID
	sessionIDCopy := session.sessionID
	llmID := session.llmID
	ragEnabled := session.ragEnabled
	_ = session.ragThreshold // TODO: 使用 ragThreshold 做相似度过滤
	ttsID := session.ttsID
	session.mu.Unlock()

	p.conversationMgr.AddMessage(callID, "user", text)

	p.publishEvent(contracts.StreamEvent{
		Type: "user_message",
		Data: map[string]interface{}{
			"sessionID": sessionIDCopy,
			"text":      text,
		},
		Timestamp: time.Now(),
	})

	var response string
	var ragContext string

	if ragEnabled && p.ragEngine != nil {
		ragResponse, err := p.ragEngine.Query(ctx, text)
		if err == nil && ragResponse != "" {
			ragContext = ragResponse
			p.publishEvent(contracts.StreamEvent{
				Type: "rag_result",
				Data: map[string]interface{}{
					"sessionID": sessionIDCopy,
					"context":   ragContext,
				},
				Timestamp: time.Now(),
			})
		}
	}

	if llmEngine := GetLLMEngine(llmID); llmEngine != nil {
		systemPrompt := ""
		if ragContext != "" {
			systemPrompt = fmt.Sprintf("请根据以下知识库内容回答用户问题：\n%s", ragContext)
		}

		var err error
		response, err = llmEngine.GenerateReply(ctx, systemPrompt, text, nil)
		if err != nil {
			p.logger.Error("LLM 生成回复失败", "error", err.Error())
			response = "抱歉，我遇到了一些问题，请稍后再试。"
		}
	} else {
		response = "抱歉，AI 服务暂时不可用。"
	}

	session = p.getSessionSync(sessionID)
	if session == nil {
		return
	}
	session.mu.Lock()
	session.conversation.AddAssistantMessage(response)
	session.mu.Unlock()

	p.conversationMgr.AddMessage(callID, "assistant", response)

	p.publishEvent(contracts.StreamEvent{
		Type: "llm_response",
		Data: map[string]interface{}{
			"sessionID": sessionIDCopy,
			"text":      response,
		},
		Timestamp: time.Now(),
	})

	if ttsEngine := GetTTSEngine(ttsID); ttsEngine != nil {
		audioData, err := ttsEngine.Synthesize(ctx, response, nil)
		if err == nil {
			p.publishEvent(contracts.StreamEvent{
				Type: "tts_audio",
				Data: map[string]interface{}{
					"sessionID": sessionIDCopy,
					"audio":     audioData,
					"text":      response,
				},
				Timestamp: time.Now(),
			})
		} else {
			p.logger.Error("TTS 合成失败", "error", err.Error())
		}
	}
}

// SwitchLLMProvider 切换 LLM 提供商
func (p *AIPipeline) SwitchLLMProvider(sessionID, llmID string) error {
	session := p.getSessionSync(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	session.mu.Lock()
	session.llmID = llmID
	session.mu.Unlock()

	p.publishEvent(contracts.StreamEvent{
		Type: "llm_switched",
		Data: map[string]interface{}{
			"sessionID": sessionID,
			"llmID":     llmID,
		},
		Timestamp: time.Now(),
	})

	return nil
}

// SwitchASRProvider 切换 ASR 提供商
func (p *AIPipeline) SwitchASRProvider(sessionID, asrID string) error {
	session := p.getSessionSync(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	session.mu.Lock()
	session.asrID = asrID
	session.mu.Unlock()

	p.publishEvent(contracts.StreamEvent{
		Type: "asr_switched",
		Data: map[string]interface{}{
			"sessionID": sessionID,
			"asrID":     asrID,
		},
		Timestamp: time.Now(),
	})

	return nil
}

// SwitchTTSProvider 切换 TTS 提供商
func (p *AIPipeline) SwitchTTSProvider(sessionID, ttsID string) error {
	session := p.getSessionSync(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	session.mu.Lock()
	session.ttsID = ttsID
	session.mu.Unlock()

	p.publishEvent(contracts.StreamEvent{
		Type: "tts_switched",
		Data: map[string]interface{}{
			"sessionID": sessionID,
			"ttsID":     ttsID,
		},
		Timestamp: time.Now(),
	})

	return nil
}

// getSession 获取 session 指针（读锁保护 map 访问，调用者需自行加锁操作 session 字段）
func (p *AIPipeline) getSession(sessionID string) (*AISession, error) {
	p.sessionsMu.RLock()
	defer p.sessionsMu.RUnlock()

	session, exists := p.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// getSessionSync 获取 session 并检查是否存活
func (p *AIPipeline) getSessionSync(sessionID string) *AISession {
	p.sessionsMu.RLock()
	defer p.sessionsMu.RUnlock()
	return p.sessions[sessionID]
}

func (p *AIPipeline) publishEvent(event contracts.StreamEvent) {
	select {
	case p.eventChan <- event:
	default:
		p.logger.Warn("AI 管道事件通道已满，丢弃事件", "type", event.Type)
	}
}

// ============================================================================
// 对话历史管理器
// ============================================================================

// ConversationManager 对话历史管理器
type ConversationManager struct {
	conversations map[string]*contracts.ConversationHistory
	mu            sync.RWMutex
}

var (
	globalConversationManager *ConversationManager
	conversationMgrOnce       sync.Once
)

// GetConversationManager 获取对话历史管理器单例
func GetConversationManager() *ConversationManager {
	conversationMgrOnce.Do(func() {
		globalConversationManager = &ConversationManager{
			conversations: make(map[string]*contracts.ConversationHistory),
		}
	})
	return globalConversationManager
}

// InitConversation 初始化对话
func (m *ConversationManager) InitConversation(callID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.conversations[callID] = &contracts.ConversationHistory{
		CallID:    callID,
		Messages:  []contracts.LLMMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// AddMessage 添加消息
func (m *ConversationManager) AddMessage(callID, role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conv, exists := m.conversations[callID]; exists {
		if role == "user" {
			conv.AddUserMessage(content)
		} else if role == "assistant" {
			conv.AddAssistantMessage(content)
		} else if role == "system" {
			conv.AddSystemMessage(content)
		}
	}
}

// GetConversation 获取对话
func (m *ConversationManager) GetConversation(callID string) (*contracts.ConversationHistory, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conv, exists := m.conversations[callID]
	return conv, exists
}

// ClearConversation 清除对话
func (m *ConversationManager) ClearConversation(callID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.conversations, callID)
}
