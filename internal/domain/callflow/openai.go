package callflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// ============================================================================
// 🌐 OpenAI (ChatGPT / Whisper / OpenAI TTS) 物理与仿真双模引擎实现
// ============================================================================

func init() {
	// 注册 OpenAI ASR/TTS/LLM 引擎驱动
	RegisterASREngine("openai", &OpenAIASREngine{})
	RegisterTTSEngine("openai", &OpenAITTSEngine{})
	RegisterLLMEngine("openai", &OpenAILLMEngine{})

	// 注册别名
	RegisterASREngine("whisper", &OpenAIASREngine{})
	RegisterTTSEngine("whisper", &OpenAITTSEngine{})
	RegisterLLMEngine("chatgpt", &OpenAILLMEngine{})
}

// ----------------------------------------------------------------------------
// ASR (OpenAI Whisper 语音识别) 物理与仿真驱动
// ----------------------------------------------------------------------------

type OpenAIASREngine struct{}

// Transcribe 调用 OpenAI Whisper 接口，物理封装 multipart/form-data 表单并执行上传。
func (e *OpenAIASREngine) Transcribe(ctx context.Context, audioData []byte, format string, config map[string]any) (string, error) {
	apiKey, _ := config["llmApiKey"].(string)

	// 1. 物理安全校验，无凭证直接报错
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI Whisper 物理凭证未配置，拒绝处理业务")
	}

	// 2. 生产物理环境：对接 OpenAI Whisper 识别接口
	// Whisper ASR 接口要求 multipart/form-data 上传，端点：https://api.openai.com/v1/audio/transcriptions
	url := "https://api.openai.com/v1/audio/transcriptions"

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// 添加 model 字段
	if err := bodyWriter.WriteField("model", "whisper-1"); err != nil {
		return "", err
	}

	// 写入音频文件二进制
	fileName := "audio.wav"
	if format == "webm" {
		fileName = "audio.webm"
	}
	filePart, err := bodyWriter.CreateFormFile("file", fileName)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(filePart, bytes.NewReader(audioData)); err != nil {
		return "", err
	}

	if err := bodyWriter.Close(); err != nil {
		return "", err
	}

	reqCtx, reqCancel := context.WithTimeout(ctx, 12*time.Second)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", url, bodyBuf)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := GlobalHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI Whisper 物理 ASR 接口请求失败: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI Whisper ASR 接口返回错误，状态码: %d", resp.StatusCode)
	}

	var res struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return res.Text, nil
}

// ----------------------------------------------------------------------------
// TTS (OpenAI TTS 语音合成) 物理与仿真驱动
// ----------------------------------------------------------------------------

type OpenAITTSEngine struct{}

// Synthesize 发起 OpenAI TTS 合成，支持高保真物理 API 请求与仿真退化。
func (e *OpenAITTSEngine) Synthesize(ctx context.Context, text string, config map[string]any) ([]byte, error) {
	apiKey, _ := config["llmApiKey"].(string)
	voice, _ := config["openaiVoice"].(string)

	if voice == "" {
		voice = "alloy" // OpenAI TTS 默认声音
	}

	// 1. 物理安全校验，无凭证直接报错
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI TTS 物理凭证未配置，拒绝处理业务")
	}

	// 2. 生产物理环境：发起 OpenAI TTS 语音合成物理 API 请求
	// 端点：https://api.openai.com/v1/audio/speech
	url := "https://api.openai.com/v1/audio/speech"

	type OpenAITTSPayload struct {
		Model          string `json:"model"`
		Input          string `json:"input"`
		Voice          string `json:"voice"`
		ResponseFormat string `json:"response_format"`
	}

	payload := OpenAITTSPayload{
		Model:          "tts-1",
		Input:          text,
		Voice:          voice,
		ResponseFormat: "mp3",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqCtx, reqCancel := context.WithTimeout(ctx, 10*time.Second)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := GlobalHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI TTS 物理合成接口请求失败: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		return io.ReadAll(resp.Body)
	}

	return nil, fmt.Errorf("OpenAI TTS 物理合成接口返回错误，状态码: %d", resp.StatusCode)
}

// ----------------------------------------------------------------------------
// LLM (OpenAI ChatGPT) 物理与仿真驱动
// ----------------------------------------------------------------------------

type OpenAILLMEngine struct{}

// GenerateReply 发起 ChatGPT 物理调用，支持自定义 Endpoint 与高保真仿真回复。
func (e *OpenAILLMEngine) GenerateReply(ctx context.Context, systemPrompt, userMessage string, config map[string]any) (string, error) {
	apiKey, _ := config["llmApiKey"].(string)
	model, _ := config["llmModel"].(string)
	endpoint, _ := config["llmEndpoint"].(string)
	tempVal, _ := config["llmTemperature"].(float64)

	if model == "" {
		model = "gpt-4o"
	}
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/chat/completions"
	}
	if tempVal <= 0 {
		tempVal = 0.7
	}

	// 1. 物理安全校验，无凭证直接报错
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI ChatGPT API 密钥未配置，拒绝处理业务")
	}

	// 2. 生产物理环境：对接 OpenAI 兼容的大模型 API
	type ChatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type ChatRequest struct {
		Model       string        `json:"model"`
		Messages    []ChatMessage `json:"messages"`
		Temperature float64       `json:"temperature"`
	}

	reqBody := ChatRequest{
		Model: model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Temperature: tempVal,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	reqCtx, reqCancel := context.WithTimeout(ctx, 15*time.Second)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := GlobalHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI 物理 API 调用失败: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI 物理大模型接口报错，状态码: %d", resp.StatusCode)
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

	return "", fmt.Errorf("OpenAI 物理接口返回空白 Choices")
}
