package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterAIModelFlowRoutes 注册商户 AI 流程管理接口。
func RegisterAIModelFlowRoutes(r gin.IRoutes, service *operatedomain.AIModelFlowManagementService) {
	r.POST("/merchant/ai-model-flow/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		var req operatedomain.AIModelFlowPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询 AI 流程失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/merchant/ai-model-flow/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		flow, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeAIModelFlowError(c, err, "AI 流程不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(flow))
	})

	r.PUT("/merchant/ai-model-flow/add", saveAIModelFlow(service))
	r.POST("/merchant/ai-model-flow/update", saveAIModelFlow(service))
	r.POST("/merchant/ai-model-flow/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		ids, ok := parseIDs(c)
		if !ok {
			return
		}
		if err := service.Delete(c.Request.Context(), ids); err != nil {
			writeAIModelFlowError(c, err, "删除 AI 流程失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(ids)}))
	})

	r.POST("/merchant/ai-model-flow/precheck", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		var req operatedomain.AIModelFlow
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		flow, err := service.Precheck(c.Request.Context(), req)
		if err != nil {
			writeAIModelFlowError(c, err, "AI 流程预检查失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(flow))
	})

	r.POST("/merchant/ai-model-flow/publish/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		flow, err := service.Publish(c.Request.Context(), id)
		if err != nil {
			writeAIModelFlowError(c, err, "AI 流程发布失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(flow))
	})
}

func saveAIModelFlow(service *operatedomain.AIModelFlowManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 流程管理未启用"))
			return
		}
		var req operatedomain.AIModelFlow
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeAIModelFlowError(c, err, "保存 AI 流程失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	}
}

func writeAIModelFlowError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrAIModelFlowNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "AI 流程不存在"))
	case errors.Is(err, operatedomain.ErrInvalidAIModelFlow):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "AI 流程参数错误"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
