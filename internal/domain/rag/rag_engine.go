package rag

import (
	"context"
	"fmt"
	"strings"

	"yunshu/internal/domain/callflow"
)

// RAGEngine 是检索增强生成引擎
type RAGEngine struct {
	embedder   Embedder
	vectorStore VectorStore
	llm        callflow.LLMEngine
	config     RAGConfig
}

// RAGConfig RAG 配置
type RAGConfig struct {
	TopK          int     // 返回最相关的 K 个文档
	ScoreThreshold float64 // 相似度阈值（低于这个值的不会被使用）
	MaxTokens     int     // 上下文最大 token 数
}

// DefaultRAGConfig 是默认 RAG 配置
func DefaultRAGConfig() RAGConfig {
	return RAGConfig{
		TopK:          5,
		ScoreThreshold: 0.7,
		MaxTokens:     4000,
	}
}

// NewRAGEngine 创建一个新的 RAG 引擎
func NewRAGEngine(embedder Embedder, vectorStore VectorStore, llm callflow.LLMEngine, config RAGConfig) *RAGEngine {
	if config.TopK <= 0 {
		config = DefaultRAGConfig()
	}
	return &RAGEngine{
		embedder:    embedder,
		vectorStore: vectorStore,
		llm:         llm,
		config:      config,
	}
}

// Query 查询 RAG 系统并生成答案
func (r *RAGEngine) Query(ctx context.Context, question string) (string, error) {
	// 1. 将问题嵌入为向量
	embeddings, err := r.embedder.Embed(ctx, []string{question})
	if err != nil {
		return "", fmt.Errorf("embedding question failed: %w", err)
	}
	if len(embeddings) == 0 {
		return "", fmt.Errorf("no embedding returned")
	}

	// 2. 在向量库中搜索相关文档
	searchResults, err := r.vectorStore.Search(ctx, embeddings[0], r.config.TopK, nil)
	if err != nil {
		return "", fmt.Errorf("vector search failed: %w", err)
	}

	// 3. 过滤低于阈值的结果
	filteredResults := r.filterLowScores(searchResults)
	if len(filteredResults) == 0 {
		return "", nil // 没有找到相关文档，返回空，让 LLM 直接回答
	}

	// 4. 构建上下文
	context := r.buildContext(filteredResults)

	// 5. 使用 LLM 生成答案
	prompt := r.buildPrompt(question, context)
	answer, err := r.llm.GenerateReply(ctx, "", prompt, nil)
	if err != nil {
		return "", fmt.Errorf("llm generation failed: %w", err)
	}

	return answer, nil
}

// AddDocument 添加文档到向量库
func (r *RAGEngine) AddDocument(ctx context.Context, id, text string, metadata map[string]interface{}) error {
	embeddings, err := r.embedder.Embed(ctx, []string{text})
	if err != nil {
		return fmt.Errorf("embedding document failed: %w", err)
	}
	if len(embeddings) == 0 {
		return fmt.Errorf("no embedding returned")
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["text"] = text

	return r.vectorStore.Upsert(ctx, id, embeddings[0], metadata)
}

// AddDocumentBatch 批量添加文档
func (r *RAGEngine) AddDocumentBatch(ctx context.Context, items []DocumentItem) error {
	// 收集所有文本进行批量嵌入
	texts := make([]string, len(items))
	for i, item := range items {
		texts[i] = item.Text
	}

	// 批量嵌入
	embeddings, err := r.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("batch embedding failed: %w", err)
	}

	// 构建向量项目
	vectorItems := make([]VectorItem, len(items))
	for i, item := range items {
		if i < len(embeddings) {
			vectorItems[i] = VectorItem{
				ID:     item.ID,
				Vector: embeddings[i],
				Text:   item.Text,
				Metadata: item.Metadata,
			}
			if vectorItems[i].Metadata == nil {
				vectorItems[i].Metadata = make(map[string]interface{})
			}
			vectorItems[i].Metadata["text"] = item.Text
		}
	}

	return r.vectorStore.UpsertBatch(ctx, vectorItems)
}

// DeleteDocument 删除文档
func (r *RAGEngine) DeleteDocument(ctx context.Context, id string) error {
	return r.vectorStore.Delete(ctx, id)
}

// filterLowScores 过滤掉低于阈值的搜索结果
func (r *RAGEngine) filterLowScores(results []SearchResult) []SearchResult {
	var filtered []SearchResult
	for _, result := range results {
		if result.Score >= r.config.ScoreThreshold {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// buildContext 从搜索结果构建上下文字符串
func (r *RAGEngine) buildContext(results []SearchResult) string {
	var sb strings.Builder
	sb.WriteString("以下是相关知识库内容：\n\n")
	for i, result := range results {
		sb.WriteString(fmt.Sprintf("[%d] (相似度: %.2f)\n", i+1, result.Score))
		sb.WriteString(r.extractText(result))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// extractText 从搜索结果中提取文本
func (r *RAGEngine) extractText(result SearchResult) string {
	if result.Text != "" {
		return result.Text
	}
	if txt, ok := result.Metadata["text"].(string); ok && txt != "" {
		return txt
	}
	return fmt.Sprintf("文档 %s (无文本)", result.ID)
}

// buildPrompt 构建提示词
func (r *RAGEngine) buildPrompt(question, context string) string {
	return fmt.Sprintf(`你是一个专业的客服助手。请根据以下知识库内容回答用户的问题。

知识库内容：
%s

用户问题：%s

请基于知识库内容准确回答问题。如果知识库中没有相关信息，请说"抱歉，我没有找到相关信息"。`,
		context, question)
}

// DocumentItem 文档项目（用于批量添加）
type DocumentItem struct {
	ID       string                 // 文档 ID
	Text     string                 // 文档文本
	Metadata map[string]interface{} // 元数据
}
