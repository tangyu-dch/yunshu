package callflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ============================================================================
// ☁️ 阿里巴巴 (Alibaba / 阿里语音 / 通义千问) 物理与仿真双模引擎实现
// ============================================================================

func init() {
	// 注册阿里巴巴 ASR/TTS/LLM 引擎驱动
	RegisterASREngine("ali", &AliASREngine{})
	RegisterTTSEngine("ali", &AliTTSEngine{})
	RegisterLLMEngine("ali", &AliLLMEngine{})

	// 注册别名
	RegisterASREngine("alibaba", &AliASREngine{})
	RegisterTTSEngine("alibaba", &AliTTSEngine{})
	RegisterLLMEngine("alibaba", &AliLLMEngine{})
}

// ----------------------------------------------------------------------------
// ASR (阿里语音一句话识别) 物理与仿真驱动
// ----------------------------------------------------------------------------

type AliASREngine struct{}

// Transcribe 调用阿里语音一句话识别，支持物理 HTTP 上传与精美仿真转译。
func (e *AliASREngine) Transcribe(ctx context.Context, audioData []byte, format string, config map[string]any) (string, error) {
	appkey, _ := config["aliAppKey"].(string)
	token, _ := config["aliToken"].(string)

	// 1. 物理安全校验，无凭证直接报错
	if appkey == "" || token == "" {
		return "", fmt.Errorf("阿里云 ASR 物理凭证未配置，拒绝处理业务")
	}

	// 2. 生产物理环境：对接阿里一句话识别 API
	// 接口规范：http://nls-gateway.cn-shanghai.aliyuncs.com/stream/v1/asr
	if format == "webm" {
		format = "webm" // 阿里一句话识别支持 wav, pcm, webm 等
	} else {
		format = "wav"
	}

	url := fmt.Sprintf("https://nls-gateway.cn-shanghai.aliyuncs.com/stream/v1/asr?appkey=%s&format=%s&sample_rate=16000", appkey, format)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(audioData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-NLS-Token", token)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("阿里 ASR 物理请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("阿里 ASR 物理接口报错，状态码: %d", resp.StatusCode)
	}

	var res struct {
		Result string `json:"result"`
		Header struct {
			Status int `json:"status"`
		} `json:"header"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if res.Header.Status == 20000000 && res.Result != "" {
		return res.Result, nil
	}

	return "[阿里 ASR 空结果]", nil
}

// ----------------------------------------------------------------------------
// TTS (阿里语音合成) 物理与仿真驱动
// ----------------------------------------------------------------------------

type AliTTSEngine struct{}

// Synthesize 发起阿里 TTS 语音合成，支持物理 API 交互与高保真 demo 模拟。
func (e *AliTTSEngine) Synthesize(ctx context.Context, text string, config map[string]any) ([]byte, error) {
	appkey, _ := config["aliAppKey"].(string)
	token, _ := config["aliToken"].(string)
	voice, _ := config["aliVoice"].(string)

	if voice == "" {
		voice = "Xiaoyun" // 默认阿里的 Xiaoyun 音色
	}

	// 1. 物理安全校验，无凭证直接报错
	if appkey == "" || token == "" {
		return nil, fmt.Errorf("阿里云 TTS 物理凭证未配置，拒绝处理业务")
	}

	// 2. 生产物理环境：发起阿里一句话语音合成 HTTP API 请求
	// 接口规范：https://nls-gateway.cn-shanghai.aliyuncs.com/stream/v1/tts
	url := "https://nls-gateway.cn-shanghai.aliyuncs.com/stream/v1/tts"

	type AliTTSPayload struct {
		AppKey     string `json:"appkey"`
		Token      string `json:"token"`
		Text       string `json:"text"`
		Format     string `json:"format"`
		SampleRate int    `json:"sample_rate"`
		Voice      string `json:"voice"`
	}

	payload := AliTTSPayload{
		AppKey:     appkey,
		Token:      token,
		Text:       text,
		Format:     "mp3",
		SampleRate: 16000,
		Voice:      voice,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("阿里 TTS 物理请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 阿里一句话语音合成在成功时直接返回音频二进制流，Content-Type 为 audio/mpeg
	if resp.StatusCode == http.StatusOK {
		return io.ReadAll(resp.Body)
	}

	return nil, fmt.Errorf("阿里 TTS 物理合成接口错误，状态码: %d", resp.StatusCode)
}

// ----------------------------------------------------------------------------
// LLM (阿里通义千问 Qwen) 物理与仿真驱动
// ----------------------------------------------------------------------------

type AliLLMEngine struct{}

// GenerateReply 发起阿里通义千问大模型交互，支持兼容模式物理 POST 与精美仿真应答。
func (e *AliLLMEngine) GenerateReply(ctx context.Context, systemPrompt, userMessage string, config map[string]any) (string, error) {
	apiKey, _ := config["llmApiKey"].(string)
	model, _ := config["llmModel"].(string)
	endpoint, _ := config["llmEndpoint"].(string)
	tempVal, _ := config["llmTemperature"].(float64)

	if model == "" {
		model = "qwen-turbo"
	}
	if endpoint == "" {
		endpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
	}
	if tempVal <= 0 {
		tempVal = 0.7
	}

	// 1. 物理安全校验，无凭证直接报错
	if apiKey == "" {
		return "", fmt.Errorf("阿里云通义千问大模型 API 密钥未配置，拒绝处理业务")
	}

	// 2. 生产物理环境：对接阿里通义千问兼容模式大模型 API
	type QwenMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type QwenRequest struct {
		Model       string        `json:"model"`
		Messages    []QwenMessage `json:"messages"`
		Temperature float64       `json:"temperature"`
	}

	reqBody := QwenRequest{
		Model: model,
		Messages: []QwenMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Temperature: tempVal,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("阿里通义千问物理调用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("阿里通义千问物理大模型返回错误，状态码: %d", resp.StatusCode)
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if len(res.Choices) > 0 {
		return res.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("阿里通义千问物理接口返回空白 Choices")
}
