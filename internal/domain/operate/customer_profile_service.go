
package operate

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// CustomerProfileService 客户画像服务
type CustomerProfileService struct {
	db *gorm.DB
}

// NewCustomerProfileService 创建客户画像服务
func NewCustomerProfileService(db *gorm.DB) *CustomerProfileService {
	return &CustomerProfileService{db: db}
}

// CreateProfile 创建客户画像
func (s *CustomerProfileService) CreateProfile(profile *CustomerProfile) error {
	return s.db.Create(profile).Error
}

// UpdateProfile 更新客户画像
func (s *CustomerProfileService) UpdateProfile(profile *CustomerProfile) error {
	return s.db.Save(profile).Error
}

// GetProfileByID 根据ID获取客户画像
func (s *CustomerProfileService) GetProfileByID(id uint64) (*CustomerProfile, error) {
	var profile CustomerProfile
	err := s.db.First(&profile, id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetProfileByPhone 根据手机号获取客户画像
func (s *CustomerProfileService) GetProfileByPhone(merchantID uint64, phone string) (*CustomerProfile, error) {
	var profile CustomerProfile
	err := s.db.Where("merchant_id = ? AND phone_number = ?", merchantID, phone).First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// QueryProfiles 查询客户画像列表
func (s *CustomerProfileService) QueryProfiles(req ProfileQueryRequest) ([]CustomerProfile, int64, error) {
	var profiles []CustomerProfile
	var total int64

	query := s.db.Model(&CustomerProfile{}).Where("merchant_id = ?", req.MerchantID)

	if req.PhoneNumber != "" {
		query = query.Where("phone_number LIKE ?", "%"+req.PhoneNumber+"%")
	}
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if len(req.Tags) > 0 {
		for _, tag := range req.Tags {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		}
	}

	// 统计总数
	query.Count(&total)

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&profiles).Error
	if err != nil {
		return nil, 0, err
	}

	return profiles, total, nil
}

// BatchUpdateProfiles 批量更新客户画像
func (s *CustomerProfileService) BatchUpdateProfiles(merchantID uint64, req ProfileBatchUpdateRequest) error {
	if len(req.IDs) == 0 {
		return nil
	}

	updateData := make(map[string]interface{})
	if req.Status != "" {
		updateData["status"] = req.Status
	}
	if req.CustomFields != nil {
		updateData["custom_fields"] = req.CustomFields
	}
	if len(req.Tags) > 0 {
		tagsJSON, _ := json.Marshal(req.Tags)
		updateData["tags"] = string(tagsJSON)
	}

	return s.db.Model(&CustomerProfile{}).
		Where("merchant_id = ? AND id IN ?", merchantID, req.IDs).
		Updates(updateData).Error
}

// DeleteProfile 删除客户画像
func (s *CustomerProfileService) DeleteProfile(id uint64, merchantID uint64) error {
	return s.db.Where("id = ? AND merchant_id = ?", id, merchantID).Delete(&CustomerProfile{}).Error
}

// CreateTag 创建客户标签
func (s *CustomerProfileService) CreateTag(tag *CustomerProfileTag) error {
	return s.db.Create(tag).Error
}

// UpdateTag 更新客户标签
func (s *CustomerProfileService) UpdateTag(tag *CustomerProfileTag) error {
	return s.db.Save(tag).Error
}

// DeleteTag 删除客户标签
func (s *CustomerProfileService) DeleteTag(id uint64, merchantID uint64) error {
	return s.db.Where("id = ? AND merchant_id = ?", id, merchantID).Delete(&CustomerProfileTag{}).Error
}

// ListTags 列出客户标签
func (s *CustomerProfileService) ListTags(merchantID uint64) ([]CustomerProfileTag, error) {
	var tags []CustomerProfileTag
	err := s.db.Where("merchant_id = ?", merchantID).Order("created_at DESC").Find(&tags).Error
	return tags, err
}

// GetTagByID 根据ID获取标签
func (s *CustomerProfileService) GetTagByID(id uint64) (*CustomerProfileTag, error) {
	var tag CustomerProfileTag
	err := s.db.First(&tag, id).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// BatchTagOperation 批量标签操作
func (s *CustomerProfileService) BatchTagOperation(merchantID uint64, req ProfileTagBatchRequest) error {
	if len(req.CustomerIDs) == 0 || len(req.TagIDs) == 0 {
		return nil
	}

	// 获取标签信息
	var tags []CustomerProfileTag
	if err := s.db.Where("id IN ? AND merchant_id = ?", req.TagIDs, merchantID).Find(&tags).Error; err != nil {
		return err
	}

	if len(tags) == 0 {
		return fmt.Errorf("没有找到有效的标签")
	}

	// 获取客户画像
	var profiles []CustomerProfile
	if err := s.db.Where("id IN ? AND merchant_id = ?", req.CustomerIDs, merchantID).Find(&profiles).Error; err != nil {
		return err
	}

	// 批量更新
	for _, profile := range profiles {
		var currentTags []string
		if profile.Tags != "" {
			if err := json.Unmarshal([]byte(profile.Tags), &currentTags); err != nil {
				currentTags = []string{}
			}
		}

		tagMap := make(map[string]bool)
		for _, t := range currentTags {
			tagMap[t] = true
		}

		if req.Operation == "add" {
			for _, tag := range tags {
				tagMap[tag.Name] = true
			}
		} else if req.Operation == "remove" {
			for _, tag := range tags {
				delete(tagMap, tag.Name)
			}
		}

		newTags := make([]string, 0, len(tagMap))
		for t := range tagMap {
			newTags = append(newTags, t)
		}

		tagsJSON, _ := json.Marshal(newTags)
		profile.Tags = string(tagsJSON)
		profile.UpdatedAt = time.Now()

		if err := s.db.Save(&profile).Error; err != nil {
			return err
		}
	}

	return nil
}

// CreateWorkflow 创建画像编排流程
func (s *CustomerProfileService) CreateWorkflow(workflow *ProfileWorkflow) error {
	return s.db.Create(workflow).Error
}

// UpdateWorkflow 更新画像编排流程
func (s *CustomerProfileService) UpdateWorkflow(workflow *ProfileWorkflow) error {
	return s.db.Save(workflow).Error
}

// DeleteWorkflow 删除画像编排流程
func (s *CustomerProfileService) DeleteWorkflow(id uint64, merchantID uint64) error {
	return s.db.Where("id = ? AND merchant_id = ?", id, merchantID).Delete(&ProfileWorkflow{}).Error
}

// ListWorkflows 列出画像编排流程
func (s *CustomerProfileService) ListWorkflows(merchantID uint64) ([]ProfileWorkflow, error) {
	var workflows []ProfileWorkflow
	err := s.db.Where("merchant_id = ?", merchantID).Order("created_at DESC").Find(&workflows).Error
	return workflows, err
}

// GetWorkflowByID 根据ID获取流程
func (s *CustomerProfileService) GetWorkflowByID(id uint64) (*ProfileWorkflow, error) {
	var workflow ProfileWorkflow
	err := s.db.First(&workflow, id).Error
	if err != nil {
		return nil, err
	}
	return &workflow, nil
}

// ExecuteWorkflow 执行画像编排流程
func (s *CustomerProfileService) ExecuteWorkflow(workflowID uint64, customerIDs []uint64, merchantID uint64) error {
	// 获取流程信息
	workflow, err := s.GetWorkflowByID(workflowID)
	if err != nil {
		return err
	}

	// 获取客户画像
	var profiles []CustomerProfile
	if err := s.db.Where("id IN ? AND merchant_id = ?", customerIDs, merchantID).Find(&profiles).Error; err != nil {
		return err
	}

	// 创建执行记录
	now := time.Now()
	for _, profile := range profiles {
		execution := &ProfileWorkflowExecution{
			WorkflowID: workflowID,
			CustomerID: profile.ID,
			MerchantID: merchantID,
			Status:     "running",
			StartTime:  &now,
		}
		if err := s.db.Create(execution).Error; err != nil {
			continue
		}

		// 模拟执行流程（实际项目中应该根据流程配置执行）
		// 这里简化为直接更新执行状态
		result := map[string]interface{}{
			"status": "completed",
			"message": "流程执行成功",
		}
		resultJSON, _ := json.Marshal(result)
		
		endTime := time.Now()
		execution.Status = "success"
		execution.ExecutionResult = string(resultJSON)
		execution.EndTime = &endTime
		
		s.db.Save(execution)
	}

	return nil
}

// ListWorkflowExecutions 列出流程执行记录
func (s *CustomerProfileService) ListWorkflowExecutions(workflowID uint64, page, pageSize int) ([]ProfileWorkflowExecution, int64, error) {
	var executions []ProfileWorkflowExecution
	var total int64

	query := s.db.Model(&ProfileWorkflowExecution{}).Where("workflow_id = ?", workflowID)
	
	query.Count(&total)
	
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&executions).Error
	
	return executions, total, err
}

// UpdateProfileCallStats 更新客户画像通话统计
func (s *CustomerProfileService) UpdateProfileCallStats(merchantID uint64, phone string, connected bool, duration int) error {
	var profile CustomerProfile
	err := s.db.Where("merchant_id = ? AND phone_number = ?", merchantID, phone).First(&profile).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 客户画像不存在，创建新的
			now := time.Now()
			profile = CustomerProfile{
				MerchantID:     merchantID,
				PhoneNumber:    phone,
				TotalCalls:     1,
				FirstContact:   &now,
				LastContact:    &now,
				Status:         "active",
			}
			if connected {
				profile.ConnectedCalls = 1
				profile.AvgDuration = duration
			}
			return s.db.Create(&profile).Error
		}
		return err
	}

	// 更新统计信息
	profile.TotalCalls++
	if connected {
		profile.ConnectedCalls++
		// 计算新的平均通话时长
		totalDuration := profile.AvgDuration * (profile.ConnectedCalls - 1)
		profile.AvgDuration = (totalDuration + duration) / profile.ConnectedCalls
	}
	now := time.Now()
	profile.LastContact = &now
	profile.UpdatedAt = now

	return s.db.Save(&profile).Error
}

// GetProfileStatistics 获取画像统计信息
func (s *CustomerProfileService) GetProfileStatistics(merchantID uint64) (map[string]interface{}, error) {
	var total int64
	var active int64
	var inactive int64
	var blacklist int64

	s.db.Model(&CustomerProfile{}).Where("merchant_id = ?", merchantID).Count(&total)
	s.db.Model(&CustomerProfile{}).Where("merchant_id = ? AND status = 'active'", merchantID).Count(&active)
	s.db.Model(&CustomerProfile{}).Where("merchant_id = ? AND status = 'inactive'", merchantID).Count(&inactive)
	s.db.Model(&CustomerProfile{}).Where("merchant_id = ? AND status = 'blacklist'", merchantID).Count(&blacklist)

	return map[string]interface{}{
		"total":      total,
		"active":     active,
		"inactive":   inactive,
		"blacklist":  blacklist,
	}, nil
}

// GenerateProfileSummary 生成客户画像摘要文本
func (s *CustomerProfileService) GenerateProfileSummary(profile *CustomerProfile) string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("客户姓名：%s，", profile.Name))
	summary.WriteString(fmt.Sprintf("手机号：%s，", profile.PhoneNumber))

	if profile.Gender != "" {
		genderText := map[string]string{
			"male":   "男",
			"female": "女",
			"unknown": "未知",
		}[profile.Gender]
		summary.WriteString(fmt.Sprintf("性别：%s，", genderText))
	}

	if profile.Age > 0 {
		summary.WriteString(fmt.Sprintf("年龄：%d岁，", profile.Age))
	}

	if profile.Province != "" {
		summary.WriteString(fmt.Sprintf("省份：%s，", profile.Province))
	}
	if profile.City != "" {
		summary.WriteString(fmt.Sprintf("城市：%s，", profile.City))
	}

	if profile.Source != "" {
		summary.WriteString(fmt.Sprintf("来源：%s，", profile.Source))
	}

	summary.WriteString(fmt.Sprintf("总呼叫次数：%d次，接通次数：%d次，", profile.TotalCalls, profile.ConnectedCalls))
	if profile.AvgDuration > 0 {
		summary.WriteString(fmt.Sprintf("平均通话时长：%d秒，", profile.AvgDuration))
	}

	if profile.Tags != "" {
		var tags []string
		if json.Unmarshal([]byte(profile.Tags), &tags) == nil {
			summary.WriteString(fmt.Sprintf("标签：%s，", strings.Join(tags, "、")))
		}
	}

	if profile.CustomFields != nil && len(profile.CustomFields) > 0 {
		summary.WriteString("自定义信息：")
		customParts := make([]string, 0, len(profile.CustomFields))
		for k, v := range profile.CustomFields {
			customParts = append(customParts, fmt.Sprintf("%s=%v", k, v))
		}
		summary.WriteString(strings.Join(customParts, "，"))
	}

	return summary.String()
}

// UpdateProfileEmbedding 更新客户画像向量（简化版，实际项目应集成真实的Embedding服务）
func (s *CustomerProfileService) UpdateProfileEmbedding(profile *CustomerProfile) error {
	// 生成画像摘要
	summary := s.GenerateProfileSummary(profile)
	profile.ProfileSummary = summary

	// 简化版：生成模拟的向量嵌入（实际项目中应调用Embedding API）
	// 这里用一个简单的哈希方法生成固定长度的向量，仅用于演示
	embedding := s.generateMockEmbedding(summary)
	profile.ProfileEmbedding = embedding
	profile.EmbeddingModel = "mock-embedding-v1"
	now := time.Now()
	profile.EmbeddingUpdatedAt = &now

	return s.db.Save(profile).Error
}

// generateMockEmbedding 生成模拟向量（仅用于演示）
func (s *CustomerProfileService) generateMockEmbedding(text string) []float32 {
	// 生成一个128维的模拟向量
	dim := 128
	embedding := make([]float32, dim)
	
	// 使用简单的哈希方法生成伪随机向量
	hash := 0
	for _, c := range text {
		hash = (hash*31 + int(c)) % 1000000
	}
	
	for i := 0; i < dim; i++ {
		// 生成范围在[-1, 1]之间的浮点数
		seed := (hash + i*12345) % 1000000
		val := float32(math.Sin(float64(seed))*0.5 + 0.5)
		embedding[i] = val*2 - 1
	}
	
	// L2归一化
	norm := float32(0)
	for _, v := range embedding {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}
	
	return embedding
}

// CosineSimilarity 计算余弦相似度
func (s *CustomerProfileService) CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	
	dotProduct := float32(0)
	normA := float32(0)
	normB := float32(0)
	
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// VectorSimilaritySearch 向量相似度搜索
func (s *CustomerProfileService) VectorSimilaritySearch(req VectorSimilarityRequest) (*VectorSimilarityResponse, error) {
	// 构建查询
	query := s.db.Model(&CustomerProfile{}).Where("merchant_id = ?", req.MerchantID)
	
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	
	if len(req.Tags) > 0 {
		for _, tag := range req.Tags {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		}
	}
	
	// 获取有向量数据的客户画像
	var profiles []CustomerProfile
	query = query.Where("profile_embedding IS NOT NULL AND profile_embedding != '[]'")
	if err := query.Find(&profiles).Error; err != nil {
		return nil, err
	}
	
	// 生成查询向量
	var queryEmbedding []float32
	if len(req.QueryEmbedding) > 0 {
		queryEmbedding = req.QueryEmbedding
	} else if req.QueryText != "" {
		queryEmbedding = s.generateMockEmbedding(req.QueryText)
	} else {
		return nil, fmt.Errorf("必须提供查询文本或查询向量")
	}
	
	// 计算相似度
	results := make([]VectorSimilarityResult, 0, len(profiles))
	for _, profile := range profiles {
		if len(profile.ProfileEmbedding) == 0 {
			continue
		}
		
		similarity := s.CosineSimilarity(queryEmbedding, profile.ProfileEmbedding)
		
		if req.MinScore > 0 && similarity < req.MinScore {
			continue
		}
		
		results = append(results, VectorSimilarityResult{
			ProfileID:       profile.ID,
			Profile:         profile,
			SimilarityScore: similarity,
		})
	}
	
	// 按相似度排序（简单的冒泡排序，数据量小时适用）
	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[j].SimilarityScore > results[i].SimilarityScore {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	
	// 取Top K
	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}
	if topK > len(results) {
		topK = len(results)
	}
	
	return &VectorSimilarityResponse{
		Results: results[:topK],
		Total:   len(results),
	}, nil
}

// BatchUpdateEmbeddings 批量更新向量
func (s *CustomerProfileService) BatchUpdateEmbeddings(req UpdateProfileEmbeddingRequest) (int, error) {
	query := s.db.Model(&CustomerProfile{}).Where("merchant_id = ?", req.MerchantID)
	
	if len(req.ProfileIDs) > 0 {
		query = query.Where("id IN ?", req.ProfileIDs)
	}
	
	var profiles []CustomerProfile
	if err := query.Find(&profiles).Error; err != nil {
		return 0, err
	}
	
	successCount := 0
	for _, profile := range profiles {
		if err := s.UpdateProfileEmbedding(&profile); err == nil {
			successCount++
		}
	}
	
	return successCount, nil
}


// escapeLike 转义 LIKE 通配符 % 和 _，防止通配符注入。
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
