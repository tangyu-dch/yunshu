
package operate

import (
	"time"
)

// CustomerProfile 客户画像模型
type CustomerProfile struct {
	ID            uint64                 `json:"id" gorm:"primaryKey;autoIncrement"`
	PhoneNumber   string                 `json:"phoneNumber" gorm:"index;size:20;comment:客户手机号"`
	MerchantID    uint64                 `json:"merchantId" gorm:"index;comment:商户ID"`
	Name          string                 `json:"name" gorm:"size:100;comment:客户姓名"`
	Gender        string                 `json:"gender" gorm:"size:10;comment:性别:male,female,unknown"`
	Age           int                    `json:"age" gorm:"comment:年龄"`
	Province      string                 `json:"province" gorm:"size:50;comment:省份"`
	City          string                 `json:"city" gorm:"size:50;comment:城市"`
	Tags          string                 `json:"tags" gorm:"type:text;comment:客户标签,JSON数组"`
	CustomFields  map[string]interface{} `json:"customFields" gorm:"type:json;comment:自定义字段"`
	Source        string                 `json:"source" gorm:"size:50;comment:来源渠道"`
	FirstContact  *time.Time             `json:"firstContact" gorm:"comment:首次联系时间"`
	LastContact   *time.Time             `json:"lastContact" gorm:"comment:最近联系时间"`
	TotalCalls    int                    `json:"totalCalls" gorm:"default:0;comment:总呼叫次数"`
	ConnectedCalls int                   `json:"connectedCalls" gorm:"default:0;comment:接通次数"`
	AvgDuration   int                    `json:"avgDuration" gorm:"default:0;comment:平均通话时长(秒)"`
	Status        string                 `json:"status" gorm:"size:20;default:'active';comment:状态:active,inactive,blacklist"`
	// 向量相关字段
	ProfileSummary string                `json:"profileSummary" gorm:"type:text;comment:客户画像摘要文本，用于生成向量"`
	ProfileEmbedding []float32           `json:"profileEmbedding" gorm:"type:json;comment:客户画像向量嵌入"`
	EmbeddingModel string               `json:"embeddingModel" gorm:"size:100;comment:向量模型名称"`
	EmbeddingUpdatedAt *time.Time        `json:"embeddingUpdatedAt" gorm:"comment:向量更新时间"`
	CreatedAt     time.Time              `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt     time.Time              `json:"updatedAt" gorm:"autoUpdateTime"`
}

// CustomerProfileTag 客户标签库
type CustomerProfileTag struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	MerchantID  uint64    `json:"merchantId" gorm:"index;comment:商户ID"`
	Name        string    `json:"name" gorm:"size:100;comment:标签名称"`
	Color       string    `json:"color" gorm:"size:20;default:'#1890ff';comment:标签颜色"`
	Description string    `json:"description" gorm:"size:500;comment:标签描述"`
	Category    string    `json:"category" gorm:"size:50;comment:标签分类"`
	Enable      bool      `json:"enable" gorm:"default:true;comment:是否启用"`
	CreatedAt   time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

// ProfileWorkflow 画像编排流程
type ProfileWorkflow struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	MerchantID  uint64    `json:"merchantId" gorm:"index;comment:商户ID"`
	Name        string    `json:"name" gorm:"size:200;comment:流程名称"`
	Description string    `json:"description" gorm:"size:500;comment:流程描述"`
	Config      string    `json:"config" gorm:"type:json;comment:流程配置,JSON"`
	Status      string    `json:"status" gorm:"size:20;default:'active';comment:状态:active,inactive"`
	CreatedAt   time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

// ProfileWorkflowExecution 画像编排执行记录
type ProfileWorkflowExecution struct {
	ID             uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	WorkflowID     uint64    `json:"workflowId" gorm:"index;comment:流程ID"`
	CustomerID     uint64    `json:"customerId" gorm:"index;comment:客户ID"`
	MerchantID     uint64    `json:"merchantId" gorm:"index;comment:商户ID"`
	Status         string    `json:"status" gorm:"size:20;comment:执行状态:pending,running,success,failed"`
	ExecutionResult string  `json:"executionResult" gorm:"type:text;comment:执行结果,JSON"`
	ErrorMessage   string    `json:"errorMessage" gorm:"type:text;comment:错误信息"`
	StartTime      *time.Time `json:"startTime" gorm:"comment:开始时间"`
	EndTime        *time.Time `json:"endTime" gorm:"comment:结束时间"`
	CreatedAt      time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

// ProfileQueryRequest 客户画像查询请求
type ProfileQueryRequest struct {
	MerchantID uint64   `json:"merchantId"`
	PhoneNumber string `json:"phoneNumber"`
	Name       string `json:"name"`
	Tags       []string `json:"tags"`
	Status     string `json:"status"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
}

// ProfileBatchUpdateRequest 批量更新请求
type ProfileBatchUpdateRequest struct {
	IDs         []uint64               `json:"ids"`
	Tags        []string               `json:"tags"`
	Status      string                 `json:"status"`
	CustomFields map[string]interface{} `json:"customFields"`
}

// ProfileTagBatchRequest 标签批量操作请求
type ProfileTagBatchRequest struct {
	TagIDs       []uint64 `json:"tagIds"`
	CustomerIDs  []uint64 `json:"customerIds"`
	Operation    string   `json:"operation"` // add, remove
}

// VectorSimilarityRequest 向量相似度查询请求
type VectorSimilarityRequest struct {
	MerchantID      uint64   `json:"merchantId"`
	QueryText       string   `json:"queryText"`       // 查询文本（将被转换为向量）
	QueryEmbedding  []float32 `json:"queryEmbedding"`  // 直接提供查询向量（可选）
	TopK            int      `json:"topK"`            // 返回最相似的前K个结果
	MinScore        float32  `json:"minScore"`        // 最小相似度分数阈值
	Tags            []string `json:"tags"`            // 标签过滤（可选）
	Status          string   `json:"status"`          // 状态过滤（可选）
}

// VectorSimilarityResult 向量相似度查询结果
type VectorSimilarityResult struct {
	ProfileID       uint64  `json:"profileId"`
	Profile         CustomerProfile `json:"profile"`
	SimilarityScore float32 `json:"similarityScore"`
}

// VectorSimilarityResponse 向量相似度查询响应
type VectorSimilarityResponse struct {
	Results []VectorSimilarityResult `json:"results"`
	Total   int                      `json:"total"`
}

// UpdateProfileEmbeddingRequest 更新客户画像向量请求
type UpdateProfileEmbeddingRequest struct {
	ProfileIDs []uint64 `json:"profileIds"` // 客户画像ID列表，为空则更新所有
	MerchantID uint64   `json:"merchantId"`
}

