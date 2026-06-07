package rag

import (
	"context"
)

// Embedding 是向量嵌入的类型别名
type Embedding []float32

// Embedder 文本嵌入生成器接口
// 负责将文本转换为向量表示
type Embedder interface {
	// Embed 将一段或多段文本转换为向量嵌入
	Embed(ctx context.Context, texts []string) ([]Embedding, error)

	// Dim 返回嵌入的向量维度
	Dim() int

	// Name 返回嵌入模型的名称
	Name() string
}

// SearchResult 向量搜索结果
type SearchResult struct {
	ID       string                 // 文档 ID
	Vector   Embedding              // 文档向量
	Score    float64                // 相似度分数 (0-1)
	Text     string                 // 原始文本内容
	Metadata map[string]interface{} // 元数据
}

// VectorStore 向量存储接口
// 负责向量的存储、检索和管理
type VectorStore interface {
	// Upsert 插入或更新向量
	Upsert(ctx context.Context, id string, vector Embedding, metadata map[string]interface{}) error

	// UpsertBatch 批量插入或更新
	UpsertBatch(ctx context.Context, items []VectorItem) error

	// Search 向量相似度搜索
	Search(ctx context.Context, query Embedding, topK int, filters map[string]interface{}) ([]SearchResult, error)

	// Delete 删除向量
	Delete(ctx context.Context, id string) error

	// DeleteBatch 批量删除
	DeleteBatch(ctx context.Context, ids []string) error

	// Get 获取单个向量
	Get(ctx context.Context, id string) (*VectorItem, error)

	// List 列出向量（分页）
	List(ctx context.Context, offset, limit int) ([]VectorItem, int, error)

	// EnsureCollection 确保集合/索引存在
	EnsureCollection(ctx context.Context, dim int) error

	// Close 关闭连接
	Close() error
}

// VectorItem 向量项目
type VectorItem struct {
	ID       string                 // 项目 ID
	Vector   Embedding              // 向量
	Text     string                 // 原始文本（可选）
	Metadata map[string]interface{} // 元数据
}
