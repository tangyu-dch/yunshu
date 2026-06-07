package operate

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// KnowledgeBase 知识库定义
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
	ID        string                 `json:"id"`
	KBID      string                 `json:"kbId"`
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Embedding []float32              `json:"embedding,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	ID       string                 `json:"id"`
	Score    float64                `json:"score"`
	Text     string                 `json:"text"`
	Metadata map[string]interface{} `json:"metadata"`
}

// KnowledgeBaseManagementService 知识库管理服务
type KnowledgeBaseManagementService struct {
	knowledgeBases map[string]*KnowledgeBase
	documents      map[string][]*KnowledgeBaseDocument
	mu             sync.RWMutex
}

// NewKnowledgeBaseManagementService 创建知识库管理服务
func NewKnowledgeBaseManagementService() *KnowledgeBaseManagementService {
	return &KnowledgeBaseManagementService{
		knowledgeBases: make(map[string]*KnowledgeBase),
		documents:      make(map[string][]*KnowledgeBaseDocument),
	}
}

// List 列出所有知识库
func (s *KnowledgeBaseManagementService) List(ctx context.Context) ([]*KnowledgeBase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*KnowledgeBase, 0, len(s.knowledgeBases))
	for _, kb := range s.knowledgeBases {
		result = append(result, kb)
	}
	return result, nil
}

// Save 保存知识库
func (s *KnowledgeBaseManagementService) Save(ctx context.Context, kb KnowledgeBase) (*KnowledgeBase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if kb.ID == "" {
		kb.ID = fmt.Sprintf("kb-%d", now.Unix())
		kb.CreatedAt = now
	} else {
		existing, exists := s.knowledgeBases[kb.ID]
		if exists {
			kb.CreatedAt = existing.CreatedAt
		}
	}
	kb.UpdatedAt = now

	s.knowledgeBases[kb.ID] = &kb
	return &kb, nil
}

// Delete 删除知识库
func (s *KnowledgeBaseManagementService) Delete(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		delete(s.knowledgeBases, id)
		delete(s.documents, id)
	}
	return nil
}

// ListDocuments 列出知识库文档
func (s *KnowledgeBaseManagementService) ListDocuments(ctx context.Context, kbID string) ([]*KnowledgeBaseDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs, exists := s.documents[kbID]
	if !exists {
		return []*KnowledgeBaseDocument{}, nil
	}
	return docs, nil
}

// SaveDocument 保存文档
func (s *KnowledgeBaseManagementService) SaveDocument(ctx context.Context, doc KnowledgeBaseDocument) (*KnowledgeBaseDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.knowledgeBases[doc.KBID]; !exists {
		return nil, fmt.Errorf("知识库不存在: %s", doc.KBID)
	}

	now := time.Now()
	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc-%d", now.Unix())
		doc.CreatedAt = now
	} else {
		// 查找现有文档
		docs := s.documents[doc.KBID]
		for _, d := range docs {
			if d.ID == doc.ID {
				doc.CreatedAt = d.CreatedAt
				break
			}
		}
	}
	doc.UpdatedAt = now

	// 更新或添加文档
	docs := s.documents[doc.KBID]
	found := false
	for i, d := range docs {
		if d.ID == doc.ID {
			docs[i] = &doc
			found = true
			break
		}
	}
	if !found {
		docs = append(docs, &doc)
	}
	s.documents[doc.KBID] = docs

	return &doc, nil
}

// DeleteDocument 删除文档
func (s *KnowledgeBaseManagementService) DeleteDocument(ctx context.Context, kbID, docID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	docs := s.documents[kbID]
	for i, d := range docs {
		if d.ID == docID {
			s.documents[kbID] = append(docs[:i], docs[i+1:]...)
			break
		}
	}
	return nil
}

// Search 搜索知识库
func (s *KnowledgeBaseManagementService) Search(ctx context.Context, kbID, query string, topK int, scoreThreshold float64) ([]*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs, exists := s.documents[kbID]
	if !exists {
		return []*SearchResult{}, nil
	}

	// 简单的文本匹配搜索（生产环境应使用向量搜索）
	results := make([]*SearchResult, 0)
	for _, doc := range docs {
		// 简单的包含匹配
		if len(doc.Content) > 0 {
			results = append(results, &SearchResult{
				ID:    doc.ID,
				Score: 0.9, // 模拟分数
				Text:  doc.Content,
				Metadata: map[string]interface{}{
					"title": doc.Title,
					"kbId":  kbID,
				},
			})
		}
	}

	// 限制返回数量
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}
