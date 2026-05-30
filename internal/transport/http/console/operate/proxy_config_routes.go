package operate

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterProxyConfigRoutes 注册运营端代理与网络配置管理接口。
func RegisterProxyConfigRoutes(r gin.IRoutes, service *operatedomain.ProxyConfigManagementService) {
	r.GET("/operate/proxy-config", func(c *gin.Context) {
		cfg, err := service.GetConfig(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取系统配置失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(cfg))
	})

	r.POST("/operate/proxy-config/save", func(c *gin.Context) {
		var req operatedomain.ProxyConfig
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.SaveConfig(c.Request.Context(), req); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK("配置保存成功"))
	})

	r.POST("/operate/proxy-config/apply", func(c *gin.Context) {
		if err := service.ApplyAndRestart(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK("配置应用成功，正在重新拉起话务代理服务容器，请稍后刷新监控"))
	})

	r.POST("/operate/proxy-config/reload-rtp", func(c *gin.Context) {
		if service.RtpReloader == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "重载接口未配置"))
			return
		}
		if err := service.RtpReloader.ReloadRtpengine(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "触发 RTPEngine 热刷新失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK("Kamailio 媒体代理配置热刷新成功"))
	})
}
