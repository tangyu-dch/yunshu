package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterAIModelConfigRoutes 注册大模型厂商与 API 配置接口。
func RegisterAIModelConfigRoutes(r gin.IRoutes, service *operatedomain.AIModelConfigManagementService) {
	// 获取模型配置列表 (商户侧 & 运营侧路由兼容)
	listHandler := func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 模型配置服务未启用"))
			return
		}
		list, err := service.List(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取 AI 模型配置列表失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	}

	r.GET("/merchant/ai-model-config/list", listHandler)
	r.GET("/operate/ai-model-config/list", listHandler)

	// 新增/保存配置
	saveHandler := func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 模型配置服务未启用"))
			return
		}
		var req operatedomain.AIModelConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		res, err := service.Save(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存 AI 模型配置失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(res))
	}

	r.PUT("/merchant/ai-model-config/add", saveHandler)
	r.POST("/merchant/ai-model-config/save", saveHandler)

	// 删除配置
	r.POST("/merchant/ai-model-config/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "AI 模型配置服务未启用"))
			return
		}
		ids, ok := parseIDs(c)
		if !ok {
			return
		}
		if err := service.Delete(c.Request.Context(), ids); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除 AI 模型配置失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(ids)}))
	})
}

func writeAIModelConfigError(c *gin.Context, err error, fallback string) {
	if errors.Is(err, operatedomain.ErrAIModelConfigNotFound) {
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "AI 模型配置不存在"))
		return
	}
	c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
}
