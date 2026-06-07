package contracts

import "time"

// ============================================================================
// 1. AI 流式处理配置模型 (支持多 LLM 动态配置)
// ============================================================================

// LLMProviderConfig 单个 LLM 提供商的配置
type LLMProviderConfig struct {
	ID          string                 `json:"id"`          // 唯一标识，如 "openai-gpt4"、"volc-doubao-pro"
	Name        string                 `json:"name"`        // 显示名称
	Provider    string                 `json:"provider"`    // 提供商类型: openai, volc, deepseek, qwen, custom
	Enabled     bool                   `json:"enabled"`     // 是否启用
	APIKey      string                 `json:"apiKey"`      // API Key (敏感信息)
	Endpoint    string                 `json:"endpoint"`    // 自定义 API 端点
	Model       string                 `json:"model"`       // 模型名称
	Temperature float64                `json:"temperature"` // 默认温度
	MaxTokens   int                    `json:"maxTokens"`   // 最大 token 数
	Extra       map[string]interface{} `json:"extra"`       // 其他自定义配置
}

// ASRProviderConfig 单个 ASR 提供商的配置
type ASRProviderConfig struct {
	ID       string                 `json:"id"`       // 唯一标识
	Name     string                 `json:"name"`     // 显示名称
	Provider string                 `json:"provider"` // 提供商类型: volc, openai, whisper, custom
	Enabled  bool                   `json:"enabled"`  // 是否启用
	APIKey   string                 `json:"apiKey"`   // API Key
	Endpoint string                 `json:"endpoint"` // API 端点
	Language string                 `json:"language"` // 默认语言: zh-CN, en-US
	Extra    map[string]interface{} `json:"extra"`    // 其他配置
}

// TTSProviderConfig 单个 TTS 提供商的配置
type TTSProviderConfig struct {
	ID        string                 `json:"id"`        // 唯一标识
	Name      string                 `json:"name"`      // 显示名称
	Provider  string                 `json:"provider"`  // 提供商类型: volc, openai, elevenlabs, custom
	Enabled   bool                   `json:"enabled"`   // 是否启用
	APIKey    string                 `json:"apiKey"`    // API Key
	Endpoint  string                 `json:"endpoint"`  // API 端点
	VoiceType string                 `json:"voiceType"` // 默认音色
	Speed     float64                `json:"speed"`     // 语速
	Extra     map[string]interface{} `json:"extra"`     // 其他配置
}

// AIStreamPipelineConfig 完整的 AI 流处理管道配置
type AIStreamPipelineConfig struct {
	// LLM 配置
	LLMProviders map[string]LLMProviderConfig `json:"llmProviders"` // 多个 LLM 提供商
	DefaultLLMID string                      `json:"defaultLLMId"` // 默认使用的 LLM

	// ASR 配置
	ASRProviders map[string]ASRProviderConfig `json:"asrProviders"` // 多个 ASR 提供商
	DefaultASRID string                      `json:"defaultASRId"` // 默认使用的 ASR

	// TTS 配置
	TTSProviders map[string]TTSProviderConfig `json:"ttsProviders"` // 多个 TTS 提供商
	DefaultTTSID string                      `json:"defaultTTSId"` // 默认使用的 TTS

	// 流处理配置
	EnableStream bool `json:"enableStream"` // 是否启用流式输出
	BufferSize   int  `json:"bufferSize"`   // 音频缓冲区大小
}

// ============================================================================
// 2. 流处理管道状态和事件
// ============================================================================

// AudioStreamSession 单个音频流会话的状态
type AudioStreamSession struct {
	SessionID    string                 `json:"sessionId"`    // 会话 ID
	CallID       string                 `json:"callId"`       // 呼叫 ID
	CustomerUUID string                 `json:"customerUUID"` // 客户侧 UUID
	Status       string                 `json:"status"`       // 会话状态: connecting, streaming, paused, stopped
	StartTime    time.Time              `json:"startTime"`    // 开始时间
	LLMConfigID  string                 `json:"llmConfigId"`  // 当前使用的 LLM 配置 ID
	ASRConfigID  string                 `json:"asrConfigId"`  // 当前使用的 ASR 配置 ID
	TTSConfigID  string                 `json:"ttsConfigId"`  // 当前使用的 TTS 配置 ID
	Metadata     map[string]interface{} `json:"metadata"`     // 附加元数据
}

// AudioChunk 接收到的音频数据块
type AudioChunk struct {
	Data       []byte    `json:"data"`       // 音频数据
	Format     string    `json:"format"`     // 格式: pcm, mp3, wav
	SampleRate int       `json:"sampleRate"` // 采样率
	Channels   int       `json:"channels"`   // 声道数
	Timestamp  time.Time `json:"timestamp"`  // 时间戳
}

// ASRResult ASR 识别结果
type ASRResult struct {
	Text      string    `json:"text"`      // 识别文本
	Confidence float64   `json:"confidence"` // 置信度
	IsFinal   bool      `json:"isFinal"`  // 是否为最终结果
	Timestamp time.Time `json:"timestamp"` // 时间戳
}

// LLMMessage LLM 对话消息
type LLMMessage struct {
	Role      string    `json:"role"`      // 角色: system, user, assistant
	Content   string    `json:"content"`   // 内容
	Timestamp time.Time `json:"timestamp"` // 时间戳
}

// LLMResponse LLM 响应
type LLMResponse struct {
	Content    string                 `json:"content"`    // 响应内容
	FinishReason string               `json:"finishReason"` // 结束原因
	Usage      map[string]int         `json:"usage"`      // Token 使用情况
	Extra      map[string]interface{} `json:"extra"`      // 附加信息
}

// StreamEvent 流处理事件
type StreamEvent struct {
	Type      string                 `json:"type"`      // 事件类型: audio, asr, llm, tts, error
	Data      interface{}            `json:"data"`      // 事件数据
	Timestamp time.Time             `json:"timestamp"` // 时间戳
}

// ============================================================================
// 3. 对话历史管理
// ============================================================================

// ConversationHistory 对话历史
type ConversationHistory struct {
	CallID    string       `json:"callId"`    // 呼叫 ID
	Messages  []LLMMessage `json:"messages"`  // 消息列表
	CreatedAt time.Time    `json:"createdAt"` // 创建时间
	UpdatedAt time.Time    `json:"updatedAt"` // 更新时间
}

// AddUserMessage 添加用户消息
func (ch *ConversationHistory) AddUserMessage(content string) {
	ch.Messages = append(ch.Messages, LLMMessage{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})
	ch.UpdatedAt = time.Now()
}

// AddAssistantMessage 添加助手消息
func (ch *ConversationHistory) AddAssistantMessage(content string) {
	ch.Messages = append(ch.Messages, LLMMessage{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	})
	ch.UpdatedAt = time.Now()
}

// AddSystemMessage 添加系统消息
func (ch *ConversationHistory) AddSystemMessage(content string) {
	ch.Messages = append(ch.Messages, LLMMessage{
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	})
	ch.UpdatedAt = time.Now()
}

// Truncate 截断历史消息
func (ch *ConversationHistory) Truncate(maxMessages int) {
	if maxMessages <= 0 {
		ch.Messages = []LLMMessage{}
		return
	}
	if len(ch.Messages) > maxMessages {
		// 保留 system 消息和最近的消息
		var result []LLMMessage
		for _, msg := range ch.Messages {
			if msg.Role == "system" {
				result = append(result, msg)
			}
		}
		remaining := maxMessages - len(result)
		if remaining > 0 && len(ch.Messages) > len(result) {
			start := len(ch.Messages) - remaining
			if start < len(result) {
				start = len(result)
			}
			result = append(result, ch.Messages[start:]...)
		}
		ch.Messages = result
	}
}
