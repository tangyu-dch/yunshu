package rag

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// 向量知识库管理
// ============================================================================

// KnowledgeBase 知识库
type KnowledgeBase struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// KnowledgeBaseDocument 知识库文档
type KnowledgeBaseDocument struct {
	ID          string                 `json:"id"`
	KBID        string                 `json:"kbId"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Embedding   Embedding              `json:"embedding,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// KnowledgeBaseManager 知识库管理器
type KnowledgeBaseManager struct {
	knowledgeBases map[string]*KnowledgeBase
	documents      map[string][]*KnowledgeBaseDocument
	embedder       Embedder
	vectorStore    VectorStore
	mu             sync.RWMutex
}

// NewKnowledgeBaseManager 创建知识库管理器
func NewKnowledgeBaseManager(embedder Embedder, vectorStore VectorStore) *KnowledgeBaseManager {
	return &KnowledgeBaseManager{
		knowledgeBases: make(map[string]*KnowledgeBase),
		documents:      make(map[string][]*KnowledgeBaseDocument),
		embedder:       embedder,
		vectorStore:    vectorStore,
	}
}

// CreateKnowledgeBase 创建知识库
func (m *KnowledgeBaseManager) CreateKnowledgeBase(ctx context.Context, name, description string, metadata map[string]interface{}) (*KnowledgeBase, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("kb-%d", time.Now().Unix())
	kb := &KnowledgeBase{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    metadata,
	}

	m.knowledgeBases[id] = kb
	m.documents[id] = []*KnowledgeBaseDocument{}

	// 确保向量存储集合存在
	if m.vectorStore != nil {
		if err := m.vectorStore.EnsureCollection(ctx, m.embedder.Dim()); err != nil {
			return nil, fmt.Errorf("ensure collection failed: %w", err)
		}
	}

	return kb, nil
}

// ListKnowledgeBases 列出所有知识库
func (m *KnowledgeBaseManager) ListKnowledgeBases() []*KnowledgeBase {
	m.mu.RLock()
	defer m.mu.RUnlock()

	kbs := make([]*KnowledgeBase, 0, len(m.knowledgeBases))
	for _, kb := range m.knowledgeBases {
		kbs = append(kbs, kb)
	}
	return kbs
}

// GetKnowledgeBase 获取知识库
func (m *KnowledgeBaseManager) GetKnowledgeBase(id string) (*KnowledgeBase, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	kb, exists := m.knowledgeBases[id]
	return kb, exists
}

// DeleteKnowledgeBase 删除知识库
func (m *KnowledgeBaseManager) DeleteKnowledgeBase(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 删除所有文档的向量
	if docs, exists := m.documents[id]; exists && m.vectorStore != nil {
		docIDs := make([]string, len(docs))
		for i, doc := range docs {
			docIDs[i] = doc.ID
		}
		if err := m.vectorStore.DeleteBatch(ctx, docIDs); err != nil {
			return fmt.Errorf("delete vectors failed: %w", err)
		}
	}

	delete(m.knowledgeBases, id)
	delete(m.documents, id)
	return nil
}

// AddDocument 添加文档到知识库
func (m *KnowledgeBaseManager) AddDocument(ctx context.Context, kbID, title, content string, metadata map[string]interface{}) (*KnowledgeBaseDocument, error) {
	// 在锁外校验知识库是否存在
	m.mu.RLock()
	if _, exists := m.knowledgeBases[kbID]; !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("knowledge base not found: %s", kbID)
	}
	m.mu.RUnlock()

	id := fmt.Sprintf("doc-%d", time.Now().Unix())
	doc := &KnowledgeBaseDocument{
		ID:        id,
		KBID:      kbID,
		Title:     title,
		Content:   content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  metadata,
	}

	// 生成嵌入向量（外部调用，不持锁）
	if m.embedder != nil {
		embeddings, err := m.embedder.Embed(ctx, []string{content})
		if err != nil {
			return nil, fmt.Errorf("embed failed: %w", err)
		}
		if len(embeddings) > 0 {
			doc.Embedding = embeddings[0]
		}
	}

	// 存储到向量数据库（外部调用，不持锁）
	if m.vectorStore != nil && doc.Embedding != nil {
		payload := make(map[string]interface{})
		if metadata != nil {
			for k, v := range metadata {
				payload[k] = v
			}
		}
		payload["text"] = content
		payload["title"] = title
		payload["kbId"] = kbID

		if err := m.vectorStore.Upsert(ctx, id, doc.Embedding, payload); err != nil {
			return nil, fmt.Errorf("upsert vector failed: %w", err)
		}
	}

	// 仅在内存 map 变更时加锁
	m.mu.Lock()
	m.documents[kbID] = append(m.documents[kbID], doc)
	if kb, exists := m.knowledgeBases[kbID]; exists {
		kb.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	return doc, nil
}

// AddDocumentsBatch 批量添加文档
func (m *KnowledgeBaseManager) AddDocumentsBatch(ctx context.Context, kbID string, docs []struct{ Title, Content string }) ([]*KnowledgeBaseDocument, error) {
	// 在锁外校验知识库是否存在
	m.mu.RLock()
	if _, exists := m.knowledgeBases[kbID]; !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("knowledge base not found: %s", kbID)
	}
	m.mu.RUnlock()

	// 准备文档
	now := time.Now()
	newDocs := make([]*KnowledgeBaseDocument, len(docs))
	contents := make([]string, len(docs))
	for i, doc := range docs {
		id := fmt.Sprintf("doc-%d-%d", now.Unix(), i)
		newDocs[i] = &KnowledgeBaseDocument{
			ID:        id,
			KBID:      kbID,
			Title:     doc.Title,
			Content:   doc.Content,
			CreatedAt: now,
			UpdatedAt: now,
		}
		contents[i] = doc.Content
	}

	// 批量生成嵌入（外部调用，不持锁）
	if m.embedder != nil {
		embeddings, err := m.embedder.Embed(ctx, contents)
		if err == nil {
			for i := range newDocs {
				if i < len(embeddings) {
					newDocs[i].Embedding = embeddings[i]
				}
			}
		}
	}

	// 批量存储到向量数据库（外部调用，不持锁）
	if m.vectorStore != nil {
		vectorItems := make([]VectorItem, 0, len(newDocs))
		for _, doc := range newDocs {
			if doc.Embedding != nil {
				payload := map[string]interface{}{
					"text":  doc.Content,
					"title": doc.Title,
					"kbId":  kbID,
				}
				vectorItems = append(vectorItems, VectorItem{
					ID:       doc.ID,
					Vector:   doc.Embedding,
					Text:     doc.Content,
					Metadata: payload,
				})
			}
		}
		if len(vectorItems) > 0 {
			if err := m.vectorStore.UpsertBatch(ctx, vectorItems); err != nil {
				return nil, fmt.Errorf("upsert batch failed: %w", err)
			}
		}
	}

	// 仅在内存 map 变更时加锁
	m.mu.Lock()
	m.documents[kbID] = append(m.documents[kbID], newDocs...)
	if kb, exists := m.knowledgeBases[kbID]; exists {
		kb.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	return newDocs, nil
}

// ListDocuments 列出知识库文档
func (m *KnowledgeBaseManager) ListDocuments(kbID string) ([]*KnowledgeBaseDocument, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.knowledgeBases[kbID]; !exists {
		return nil, fmt.Errorf("knowledge base not found: %s", kbID)
	}

	return append([]*KnowledgeBaseDocument{}, m.documents[kbID]...), nil
}

// DeleteDocument 删除文档
func (m *KnowledgeBaseManager) DeleteDocument(ctx context.Context, kbID, docID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.knowledgeBases[kbID]; !exists {
		return fmt.Errorf("knowledge base not found: %s", kbID)
	}

	// 从向量数据库删除
	if m.vectorStore != nil {
		if err := m.vectorStore.Delete(ctx, docID); err != nil {
			return fmt.Errorf("delete vector failed: %w", err)
		}
	}

	// 从内存删除
	docs := m.documents[kbID]
	for i, doc := range docs {
		if doc.ID == docID {
			m.documents[kbID] = append(docs[:i], docs[i+1:]...)
			break
		}
	}

	// 更新知识库时间
	if kb, exists := m.knowledgeBases[kbID]; exists {
		kb.UpdatedAt = time.Now()
	}

	return nil
}

// Search 在知识库中搜索
func (m *KnowledgeBaseManager) Search(ctx context.Context, kbID, query string, topK int, scoreThreshold float64) ([]*SearchResult, error) {
	if m.vectorStore == nil || m.embedder == nil {
		return nil, fmt.Errorf("vector store or embedder not initialized")
	}

	// 生成查询向量
	embeddings, err := m.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query failed: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding generated")
	}

	// 搜索
	results, err := m.vectorStore.Search(ctx, embeddings[0], topK, nil)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 过滤并按知识库筛选
	filtered := make([]*SearchResult, 0, len(results))
	for i := range results {
		result := &results[i]
		if result.Score >= scoreThreshold {
			// 如果指定了知识库，只返回该知识库的结果
			if kbID != "" {
				if result.Metadata != nil {
					if resultKBID, ok := result.Metadata["kbId"].(string); ok && resultKBID == kbID {
						filtered = append(filtered, result)
					}
				}
			} else {
				filtered = append(filtered, result)
			}
		}
	}

	return filtered, nil
}
