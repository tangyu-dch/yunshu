package rag

import (
	"context"
	"fmt"
	"math"
	"sync"
)

// MemoryVectorStore 是一个内存向量存储实现，适合测试和小规模应用
type MemoryVectorStore struct {
	items map[string]VectorItem
	mu    sync.RWMutex
}

// NewMemoryVectorStore 创建一个内存向量存储
func NewMemoryVectorStore() *MemoryVectorStore {
	return &MemoryVectorStore{
		items: make(map[string]VectorItem),
	}
}

// Upsert 插入或更新向量
func (m *MemoryVectorStore) Upsert(ctx context.Context, id string, vector Embedding, metadata map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	text, _ := metadata["text"].(string)
	m.items[id] = VectorItem{
		ID:       id,
		Vector:   vector,
		Text:     text,
		Metadata: metadata,
	}
	return nil
}

// UpsertBatch 批量插入
func (m *MemoryVectorStore) UpsertBatch(ctx context.Context, items []VectorItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range items {
		m.items[item.ID] = item
	}
	return nil
}

// Search 向量相似度搜索
func (m *MemoryVectorStore) Search(ctx context.Context, query Embedding, topK int, filters map[string]interface{}) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type scoredItem struct {
		item  VectorItem
		score float64
	}

	var scored []scoredItem

	for _, item := range m.items {
		score := cosineSimilarity(query, item.Vector)
		scored = append(scored, scoredItem{item: item, score: score})
	}

	// 按分数降序排序
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// 取前 K 个
	k := topK
	if k > len(scored) {
		k = len(scored)
	}

	results := make([]SearchResult, k)
	for i := 0; i < k; i++ {
		si := scored[i]
		results[i] = SearchResult{
			ID:       si.item.ID,
			Vector:   si.item.Vector,
			Score:    si.score,
			Text:     si.item.Text,
			Metadata: si.item.Metadata,
		}
	}

	return results, nil
}

// Delete 删除向量
func (m *MemoryVectorStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, id)
	return nil
}

// DeleteBatch 批量删除
func (m *MemoryVectorStore) DeleteBatch(ctx context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		delete(m.items, id)
	}
	return nil
}

// Get 获取单个向量
func (m *MemoryVectorStore) Get(ctx context.Context, id string) (*VectorItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.items[id]
	if !ok {
		return nil, fmt.Errorf("item not found: %s", id)
	}
	return &item, nil
}

// List 列出向量
func (m *MemoryVectorStore) List(ctx context.Context, offset, limit int) ([]VectorItem, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allItems := make([]VectorItem, 0, len(m.items))
	for _, item := range m.items {
		allItems = append(allItems, item)
	}

	start := offset
	if start > len(allItems) {
		start = len(allItems)
	}
	end := start + limit
	if end > len(allItems) {
		end = len(allItems)
	}

	return allItems[start:end], len(allItems), nil
}

// EnsureCollection 确保集合存在（内存存储不需要此操作）
func (m *MemoryVectorStore) EnsureCollection(ctx context.Context, dim int) error {
	return nil
}

// Close 关闭（内存存储不需要此操作）
func (m *MemoryVectorStore) Close() error {
	return nil
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b Embedding) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	dotProduct := 0.0
	normA := 0.0
	normB := 0.0

	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
