
package operate

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/operate"
)

// RegisterCustomerProfileRoutes 注册客户画像相关路由
func RegisterCustomerProfileRoutes(group *gin.RouterGroup, db *gorm.DB) {
	service := operate.NewCustomerProfileService(db)
	
	routes := group.Group("/customer-profile")
	{
		// 客户画像路由
		routes.POST("/create", createProfileHandler(service))
		routes.POST("/update", updateProfileHandler(service))
		routes.GET("/detail/:id", getProfileHandler(service))
		routes.POST("/query", queryProfilesHandler(service))
		routes.POST("/batch-update", batchUpdateProfilesHandler(service))
		routes.POST("/delete", deleteProfileHandler(service))
		routes.GET("/statistics", getProfileStatisticsHandler(service))
		
		// 标签管理路由
		routes.POST("/tag/create", createTagHandler(service))
		routes.POST("/tag/update", updateTagHandler(service))
		routes.POST("/tag/delete", deleteTagHandler(service))
		routes.GET("/tags", listTagsHandler(service))
		routes.POST("/tag/batch-operation", batchTagOperationHandler(service))
		
		// 画像编排流程路由
		routes.POST("/workflow/create", createWorkflowHandler(service))
		routes.POST("/workflow/update", updateWorkflowHandler(service))
		routes.POST("/workflow/delete", deleteWorkflowHandler(service))
		routes.GET("/workflows", listWorkflowsHandler(service))
		routes.POST("/workflow/execute", executeWorkflowHandler(service))
		routes.GET("/workflow/executions/:id", listWorkflowExecutionsHandler(service))
		
		// 向量匹配路由
		routes.POST("/vector/search", vectorSimilaritySearchHandler(service))
		routes.POST("/vector/update-embedding", updateProfileEmbeddingHandler(service))
		routes.POST("/vector/batch-update-embeddings", batchUpdateEmbeddingsHandler(service))
	}
}

// 创建客户画像
func createProfileHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var profile operate.CustomerProfile
		if err := c.ShouldBindJSON(&profile); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.CreateProfile(&profile); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, profile)
	}
}

// 更新客户画像
func updateProfileHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var profile operate.CustomerProfile
		if err := c.ShouldBindJSON(&profile); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.UpdateProfile(&profile); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, profile)
	}
}

// 获取客户画像详情
func getProfileHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, "无效的ID")
			return
		}

		profile, err := service.GetProfileByID(id)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusNotFound, "客户画像不存在")
			return
		}

		contracts.SuccessJSON(c, profile)
	}
}

// 查询客户画像列表
func queryProfilesHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req operate.ProfileQueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if req.Page <= 0 {
			req.Page = 1
		}
		if req.PageSize <= 0 {
			req.PageSize = 20
		}

		profiles, total, err := service.QueryProfiles(req)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, gin.H{
			"records": profiles,
			"total":   total,
			"page":    req.Page,
			"pageSize": req.PageSize,
		})
	}
}

// 批量更新客户画像
func batchUpdateProfilesHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			operate.ProfileBatchUpdateRequest
			MerchantID uint64 `json:"merchantId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.BatchUpdateProfiles(req.MerchantID, req.ProfileBatchUpdateRequest); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, nil)
	}
}

// 删除客户画像
func deleteProfileHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID         uint64 `json:"id"`
			MerchantID uint64 `json:"merchantId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.DeleteProfile(req.ID, req.MerchantID); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, nil)
	}
}

// 获取画像统计信息
func getProfileStatisticsHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		merchantIDStr := c.Query("merchantId")
		merchantID, _ := strconv.ParseUint(merchantIDStr, 10, 64)
		if merchantID == 0 {
			merchantID = 1
		}

		stats, err := service.GetProfileStatistics(merchantID)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, stats)
	}
}

// 创建标签
func createTagHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tag operate.CustomerProfileTag
		if err := c.ShouldBindJSON(&tag); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.CreateTag(&tag); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, tag)
	}
}

// 更新标签
func updateTagHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tag operate.CustomerProfileTag
		if err := c.ShouldBindJSON(&tag); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.UpdateTag(&tag); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, tag)
	}
}

// 删除标签
func deleteTagHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID         uint64 `json:"id"`
			MerchantID uint64 `json:"merchantId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.DeleteTag(req.ID, req.MerchantID); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, nil)
	}
}

// 列出标签
func listTagsHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		merchantIDStr := c.Query("merchantId")
		merchantID, _ := strconv.ParseUint(merchantIDStr, 10, 64)
		if merchantID == 0 {
			merchantID = 1
		}

		tags, err := service.ListTags(merchantID)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, tags)
	}
}

// 批量标签操作
func batchTagOperationHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			operate.ProfileTagBatchRequest
			MerchantID uint64 `json:"merchantId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.BatchTagOperation(req.MerchantID, req.ProfileTagBatchRequest); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, nil)
	}
}

// 创建画像编排流程
func createWorkflowHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var workflow operate.ProfileWorkflow
		if err := c.ShouldBindJSON(&workflow); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.CreateWorkflow(&workflow); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, workflow)
	}
}

// 更新画像编排流程
func updateWorkflowHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var workflow operate.ProfileWorkflow
		if err := c.ShouldBindJSON(&workflow); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.UpdateWorkflow(&workflow); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, workflow)
	}
}

// 删除画像编排流程
func deleteWorkflowHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID         uint64 `json:"id"`
			MerchantID uint64 `json:"merchantId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.DeleteWorkflow(req.ID, req.MerchantID); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, nil)
	}
}

// 列出画像编排流程
func listWorkflowsHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		merchantIDStr := c.Query("merchantId")
		merchantID, _ := strconv.ParseUint(merchantIDStr, 10, 64)
		if merchantID == 0 {
			merchantID = 1
		}

		workflows, err := service.ListWorkflows(merchantID)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, workflows)
	}
}

// 执行画像编排流程
func executeWorkflowHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			WorkflowID  uint64   `json:"workflowId"`
			CustomerIDs []uint64 `json:"customerIds"`
			MerchantID  uint64   `json:"merchantId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := service.ExecuteWorkflow(req.WorkflowID, req.CustomerIDs, req.MerchantID); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, nil)
	}
}

// 列出流程执行记录
func listWorkflowExecutionsHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, "无效的ID")
			return
		}

		page, _ := strconv.Atoi(c.Query("page"))
		pageSize, _ := strconv.Atoi(c.Query("pageSize"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = 20
		}

		executions, total, err := service.ListWorkflowExecutions(id, page, pageSize)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, gin.H{
			"records": executions,
			"total":   total,
			"page":    page,
			"pageSize": pageSize,
		})
	}
}

// 向量相似度搜索
func vectorSimilaritySearchHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req operate.VectorSimilarityRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if req.MerchantID == 0 {
			req.MerchantID = 1
		}

		result, err := service.VectorSimilaritySearch(req)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, result)
	}
}

// 更新单个客户画像的向量
func updateProfileEmbeddingHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ProfileID  uint64 `json:"profileId"`
			MerchantID uint64 `json:"merchantId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if req.MerchantID == 0 {
			req.MerchantID = 1
		}

		profile, err := service.GetProfileByID(req.ProfileID)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusNotFound, "客户画像不存在")
			return
		}

		if err := service.UpdateProfileEmbedding(profile); err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, profile)
	}
}

// 批量更新客户画像向量
func batchUpdateEmbeddingsHandler(service *operate.CustomerProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req operate.UpdateProfileEmbeddingRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			contracts.ErrorJSON(c, http.StatusBadRequest, err.Error())
			return
		}

		if req.MerchantID == 0 {
			req.MerchantID = 1
		}

		successCount, err := service.BatchUpdateEmbeddings(req)
		if err != nil {
			contracts.ErrorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}

		contracts.SuccessJSON(c, gin.H{
			"successCount": successCount,
		})
	}
}
