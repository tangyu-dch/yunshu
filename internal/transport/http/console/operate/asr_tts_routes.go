package operate

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterASRTTSRoutes 注册ASR/TTS配置路由
func RegisterASRTTSRoutes(r gin.IRoutes, service *operatedomain.ASRTTSManagementService) {
	// ASR配置列表
	r.GET("/merchant/asr-config/list", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "ASR/TTS服务未启用"))
			return
		}
		list, err := service.ListASRConfigs(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取ASR配置列表失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	})

	// 新增/保存ASR配置
	saveASRHandler := func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "ASR/TTS服务未启用"))
			return
		}
		var req operatedomain.ASRConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		res, err := service.SaveASRConfig(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存ASR配置失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(res))
	}
	r.PUT("/merchant/asr-config/add", saveASRHandler)
	r.POST("/merchant/asr-config/update", saveASRHandler)

	// 删除ASR配置
	r.POST("/merchant/asr-config/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "ASR/TTS服务未启用"))
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		if err := service.DeleteASRConfig(c.Request.Context(), []string{req.ID}); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除ASR配置失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": true}))
	})

	// TTS配置列表
	r.GET("/merchant/tts-config/list", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "ASR/TTS服务未启用"))
			return
		}
		list, err := service.ListTTSConfigs(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取TTS配置列表失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	})

	// 新增/保存TTS配置
	saveTTSHandler := func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "ASR/TTS服务未启用"))
			return
		}
		var req operatedomain.TTSConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		res, err := service.SaveTTSConfig(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存TTS配置失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(res))
	}
	r.PUT("/merchant/tts-config/add", saveTTSHandler)
	r.POST("/merchant/tts-config/update", saveTTSHandler)

	// 删除TTS配置
	r.POST("/merchant/tts-config/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "ASR/TTS服务未启用"))
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数验证失败: "+err.Error()))
			return
		}
		if err := service.DeleteTTSConfig(c.Request.Context(), []string{req.ID}); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除TTS配置失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": true}))
	})
}
