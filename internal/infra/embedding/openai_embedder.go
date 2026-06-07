package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"yunshu/internal/domain/rag"
)

// OpenAIEmbedder 是 OpenAI 嵌入生成器的实现
type OpenAIEmbedder struct {
	apiKey     string
	endpoint   string
	model      string
	httpClient *http.Client
}

// NewOpenAIEmbedder 创建一个 OpenAI 嵌入生成器
func NewOpenAIEmbedder(apiKey, model string) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		apiKey:     apiKey,
		endpoint:   "https://api.openai.com/v1/embeddings",
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewOpenAIEmbedderWithEndpoint 创建一个自定义端点的 OpenAI 嵌入生成器
// 适用于兼容 OpenAI 协议的第三方（如 DeepSeek）
func NewOpenAIEmbedderWithEndpoint(apiKey, endpoint, model string) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		apiKey:     apiKey,
		endpoint:   endpoint,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Embed 实现 Embedder 接口
func (o *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([]rag.Embedding, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 构建请求
	reqBody := embeddingRequest{
		Model: o.model,
		Input: texts,
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	// 发送请求
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(respBodyBytes))
	}

	// 解析响应
	var respData embeddingResponse
	if err := json.Unmarshal(respBodyBytes, &respData); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	// 转换为我们的 Embedding 类型
	embeddings := make([]rag.Embedding, len(respData.Data))
	for i, item := range respData.Data {
		embeddings[i] = item.Embedding
	}

	return embeddings, nil
}

// Dim 返回 OpenAI 嵌入的维度
func (o *OpenAIEmbedder) Dim() int {
	switch o.model {
	case "text-embedding-3-small":
		return 1536
	case "text-embedding-3-large":
		return 3072
	case "text-embedding-ada-002":
		return 1536
	default:
		return 1536 // 默认假设是小模型
	}
}

// Name 返回嵌入模型名称
func (o *OpenAIEmbedder) Name() string {
	return o.model
}

// embeddingRequest OpenAI 嵌入请求
type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingResponse OpenAI 嵌入响应
type embeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string          `json:"object"`
		Index     int             `json:"index"`
		Embedding rag.Embedding   `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}
