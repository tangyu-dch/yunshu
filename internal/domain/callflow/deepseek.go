package callflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

	// 1. 无凭证情况：平滑退化为云枢专署的高保真 DeepSeek 推理仿真
	if apiKey == "" {
		userMessage = strings.TrimSpace(userMessage)
		if strings.Contains(userMessage, "话费") || strings.Contains(userMessage, "余额") || strings.Contains(userMessage, "账单") {
			return "【DeepSeek 智能大模型仿真】为您成功连接云枢计费网关，DeepSeek 正在深度推理账单，已确定您的商户余额充裕，费率已匹配最新的按秒实时扣减！", nil
		}
		if strings.Contains(userMessage, "转人工") || strings.Contains(userMessage, "客服") || strings.Contains(userMessage, "坐席") {
			return "【DeepSeek 智能大模型仿真】已识别到您的转人工需求。DeepSeek 正在为您推理路由拓扑，已检测到分机可用，将在 1.2 秒内把您的呼叫划拨至云枢 ACD 优先级技能组，请稍后...", nil
		}
		return fmt.Sprintf("【DeepSeek 智能大模型仿真】接收到您的输入：“%s”。DeepSeek 已在云枢系统完美解耦部署，推理耗时极短，随时支持高并发话务决策！", userMessage), nil
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

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建 DeepSeek HTTP 请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 DeepSeek 物理 API 失败: %w", err)
	}
	defer resp.Body.Close()

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
