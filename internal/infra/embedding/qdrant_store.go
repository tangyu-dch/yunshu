package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
	"yunshu/internal/domain/rag"
)

// QdrantVectorStore 是 Qdrant 向量存储实现
type QdrantVectorStore struct {
	address    string
	collection string
	httpClient *http.Client
}

// NewQdrantVectorStore 创建一个 Qdrant 向量存储
func NewQdrantVectorStore(address, collection string) *QdrantVectorStore {
	return &QdrantVectorStore{
		address:    address,
		collection: collection,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// EnsureCollection 确保集合存在
func (q *QdrantVectorStore) EnsureCollection(ctx context.Context, dim int) error {
	// 先检查集合是否存在
	url := fmt.Sprintf("%s/collections/%s", q.address, q.collection)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := q.httpClient.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		return nil // 集合已存在
	}
	if resp != nil {
		resp.Body.Close()
	}

	// 创建集合
	createReq := createCollectionRequest{
		Vectors: vectorParams{
			Size:     dim,
			Distance: "Cosine",
		},
	}

	createURL := fmt.Sprintf("%s/collections/%s", q.address, q.collection)
	reqBody, _ := json.Marshal(createReq)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPut, createURL, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err = q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create collection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create collection failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// Upsert 插入或更新向量
func (q *QdrantVectorStore) Upsert(ctx context.Context, id string, vector rag.Embedding, metadata map[string]interface{}) error {
	return q.UpsertBatch(ctx, []rag.VectorItem{{
		ID:       id,
		Vector:   vector,
		Metadata: metadata,
	}})
}

// UpsertBatch 批量插入
func (q *QdrantVectorStore) UpsertBatch(ctx context.Context, items []rag.VectorItem) error {
	points := make([]point, len(items))
	for i, item := range items {
		points[i] = point{
			ID:      item.ID,
			Vector:  item.Vector,
			Payload: item.Metadata,
		}
	}

	req := upsertRequest{
		Points: points,
	}

	url := fmt.Sprintf("%s/collections/%s/points?wait=true", q.address, q.collection)
	reqBody, _ := json.Marshal(req)
	httpreq, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(reqBody))
	httpreq.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(httpreq)
	if err != nil {
		return fmt.Errorf("upsert failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upsert failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// Search 向量搜索
func (q *QdrantVectorStore) Search(ctx context.Context, query rag.Embedding, topK int, filters map[string]interface{}) ([]rag.SearchResult, error) {
	req := searchRequest{
		Vector: query,
		Limit:  topK,
		WithPayload: true,
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", q.address, q.collection)
	reqBody, _ := json.Marshal(req)
	httpreq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	httpreq.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(httpreq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var respData searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("decode search response failed: %w", err)
	}

	results := make([]rag.SearchResult, len(respData.Result))
	for i, res := range respData.Result {
		text := ""
		if t, ok := res.Payload["text"].(string); ok {
			text = t
		}
		results[i] = rag.SearchResult{
			ID:       res.ID,
			Score:    res.Score,
			Text:     text,
			Metadata: res.Payload,
		}
	}

	return results, nil
}

// Delete 删除向量
func (q *QdrantVectorStore) Delete(ctx context.Context, id string) error {
	return q.DeleteBatch(ctx, []string{id})
}

// DeleteBatch 批量删除
func (q *QdrantVectorStore) DeleteBatch(ctx context.Context, ids []string) error {
	req := deleteRequest{
		Points: ids,
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete?wait=true", q.address, q.collection)
	reqBody, _ := json.Marshal(req)
	httpreq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	httpreq.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(httpreq)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// Get 获取单个向量
func (q *QdrantVectorStore) Get(ctx context.Context, id string) (*rag.VectorItem, error) {
	url := fmt.Sprintf("%s/collections/%s/points/%s", q.address, q.collection, id)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var respData getPointResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("decode get response failed: %w", err)
	}

	return &rag.VectorItem{
		ID:       respData.Result.ID,
		Vector:   respData.Result.Vector,
		Metadata: respData.Result.Payload,
	}, nil
}

// List 列出向量（分页）
func (q *QdrantVectorStore) List(ctx context.Context, offset, limit int) ([]rag.VectorItem, int, error) {
	// Qdrant 的 scroll 接口
	req := scrollRequest{
		Limit: uint32(limit),
		WithPayload: true,
		WithVector: true,
	}
	if offset > 0 {
		req.Offset = strconv.Itoa(offset) // 简化实现，实际项目可能需要更完整的游标逻辑
	}

	url := fmt.Sprintf("%s/collections/%s/points/scroll", q.address, q.collection)
	reqBody, _ := json.Marshal(req)
	httpreq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	httpreq.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(httpreq)
	if err != nil {
		return nil, 0, fmt.Errorf("list failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("list failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var respData scrollResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, 0, fmt.Errorf("decode list response failed: %w", err)
	}

	items := make([]rag.VectorItem, len(respData.Result.Points))
	for i, p := range respData.Result.Points {
		items[i] = rag.VectorItem{
			ID:       p.ID,
			Vector:   p.Vector,
			Metadata: p.Payload,
		}
	}

	// 简化处理，total 暂时返回 len(items)
	return items, len(items), nil
}

// Close 关闭连接（Qdrant 使用 HTTP，无需特殊操作）
func (q *QdrantVectorStore) Close() error {
	return nil
}

// Qdrant 请求/响应类型
type createCollectionRequest struct {
	Vectors vectorParams `json:"vectors"`
}

type vectorParams struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

type point struct {
	ID      string                 `json:"id"`
	Vector  rag.Embedding          `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type upsertRequest struct {
	Points []point `json:"points"`
}

type searchRequest struct {
	Vector      rag.Embedding `json:"vector"`
	Limit       int           `json:"limit"`
	WithPayload bool          `json:"with_payload"`
}

type searchResponse struct {
	Result []searchResultItem `json:"result"`
}

type searchResultItem struct {
	ID      string                 `json:"id"`
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

type deleteRequest struct {
	Points []string `json:"points"`
}

type getPointResponse struct {
	Result point `json:"result"`
}

type scrollRequest struct {
	Offset      string `json:"offset,omitempty"`
	Limit       uint32 `json:"limit"`
	WithPayload bool   `json:"with_payload"`
	WithVector  bool   `json:"with_vector"`
}

type scrollResponse struct {
	Result struct {
		Points []point `json:"points"`
		NextPageOffset string `json:"next_page_offset"`
	} `json:"result"`
}
