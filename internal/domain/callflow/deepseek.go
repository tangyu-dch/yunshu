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
// 🐳 DeepSeek 智能大模型物理与仿真双模引擎驱动
// ============================================================================

func init() {
	// 注册 DeepSeek 引擎驱动
	RegisterLLMEngine("deepseek", &DeepSeekLLMEngine{})
	RegisterLLMEngine("deepseek-ai", &DeepSeekLLMEngine{})
}

type DeepSeekLLMEngine struct{}

// GenerateReply 发起 DeepSeek 大模型会话，支持真实物理调用与平滑高保真仿真退化。
func (e *DeepSeekLLMEngine) GenerateReply(ctx context.Context, systemPrompt, userMessage string, config map[string]any) (string, error) {
	apiKey, _ := config["llmApiKey"].(string)
	model, _ := config["llmModel"].(string)
	endpoint, _ := config["llmEndpoint"].(string)
	tempVal, _ := config["llmTemperature"].(float64)

	if model == "" {
		model = "deepseek-chat"
	}
	if endpoint == "" {
		endpoint = "https://api.deepseek.com/v1/chat/completions"
	}
	if tempVal <= 0 {
		tempVal = 0.7
	}

	// 1. 物理安全校验，无凭证直接报错
	if apiKey == "" {
		return "", fmt.Errorf("DeepSeek API 密钥未配置，拒绝处理业务")
	}

	// 2. 物理调用情况：发起真实的 DeepSeek API 调用
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
		return "", fmt.Errorf("DeepSeek 请求序列化失败: %w", err)
	}

	reqCtx, reqCancel := context.WithTimeout(ctx, 15*time.Second)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建 DeepSeek HTTP 请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := GlobalHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 DeepSeek 物理 API 失败: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepSeek 物理大模型接口响应错误: HTTP 状态码 %d", resp.StatusCode)
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("解析 DeepSeek 物理响应 JSON 失败: %w", err)
	}

	if len(res.Choices) > 0 {
		return res.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("DeepSeek 大语言模型物理接口返回空白 Choices")
}
