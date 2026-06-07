package operate

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterKnowledgeBaseRoutes 注册知识库管理路由
func RegisterKnowledgeBaseRoutes(r gin.IRoutes, service *operatedomain.KnowledgeBaseManagementService) {
	// 知识库列表
	r.GET("/merchant/knowledge-base/list", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "知识库服务未启用"))
			return
		}
		list, err := service.List(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取知识库列表失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	})

	// 新增/保存知识库
	saveKBHandler := func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "知识库服务未启用"))
			return
		}
		var req operatedomain.KnowledgeBase
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		res, err := service.Save(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存知识库失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(res))
	}
	r.PUT("/merchant/knowledge-base/add", saveKBHandler)
	r.POST("/merchant/knowledge-base/update", saveKBHandler)

	// 删除知识库
	r.POST("/merchant/knowledge-base/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "知识库服务未启用"))
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		if err := service.Delete(c.Request.Context(), []string{req.ID}); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除知识库失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": true}))
	})

	// 知识库文档列表
	r.GET("/merchant/knowledge-base/:kbId/documents", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "知识库服务未启用"))
			return
		}
		kbID := c.Param("kbId")
		docs, err := service.ListDocuments(c.Request.Context(), kbID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取文档列表失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(docs))
	})

	// 新增/保存文档
	saveDocHandler := func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "知识库服务未启用"))
			return
		}
		var req operatedomain.KnowledgeBaseDocument
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		res, err := service.SaveDocument(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存文档失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(res))
	}
	r.PUT("/merchant/knowledge-base/document/add", saveDocHandler)
	r.POST("/merchant/knowledge-base/document/update", saveDocHandler)

	// 删除文档
	r.POST("/merchant/knowledge-base/document/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "知识库服务未启用"))
			return
		}
		var req struct {
			KbID  string `json:"kbId"`
			DocID string `json:"docId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		if err := service.DeleteDocument(c.Request.Context(), req.KbID, req.DocID); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除文档失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": true}))
	})

	// 搜索知识库
	r.POST("/merchant/knowledge-base/search", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "知识库服务未启用"))
			return
		}
		var req struct {
			KbID           string  `json:"kbId"`
			Query          string  `json:"query"`
			TopK           int     `json:"topK"`
			ScoreThreshold float64 `json:"scoreThreshold"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		results, err := service.Search(c.Request.Context(), req.KbID, req.Query, req.TopK, req.ScoreThreshold)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "搜索失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(results))
	})
}
