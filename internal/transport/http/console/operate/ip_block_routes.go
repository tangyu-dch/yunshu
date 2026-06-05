package operate

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterIPBlockRoutes 注册运营端 IP 拦截与审计管理接口。
func RegisterIPBlockRoutes(r gin.IRoutes, service *operatedomain.IPBlockManagementService) {
	// 获取拦截国家配置
	r.GET("/operate/ip-block/config", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "IP 拦截服务未初始化"))
			return
		}
		countries, err := service.GetBlockedCountries(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取拦截配置失败"))
			return
		}
		onlyAllowCn, err := service.GetOnlyAllowCN(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取仅放行配置失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]interface{}{
			"countries":   countries,
			"onlyAllowCn": onlyAllowCn,
		}))
	})

	// 保存拦截国家配置
	r.POST("/operate/ip-block/config", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "IP 拦截服务未初始化"))
			return
		}
		var req struct {
			Countries   string `json:"countries"`
			OnlyAllowCn bool   `json:"onlyAllowCn"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.SaveBlockedCountries(c.Request.Context(), req.Countries); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		if err := service.SaveOnlyAllowCN(c.Request.Context(), req.OnlyAllowCn); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK("拦截国家与放行配置保存成功"))
	})

	// 分页查询拦截审计日志
	r.GET("/operate/ip-block/logs", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "IP 拦截服务未初始化"))
			return
		}

		var startTime time.Time
		if st := c.Query("startTime"); st != "" {
			if parsed, err := time.Parse(time.RFC3339, st); err == nil {
				startTime = parsed
			}
		}
		var endTime time.Time
		if et := c.Query("endTime"); et != "" {
			if parsed, err := time.Parse(time.RFC3339, et); err == nil {
				endTime = parsed
			}
		}

		page, err := service.Page(c.Request.Context(), operatedomain.IPBlockLogPageRequest{
			PageNumber:  parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:    parsePositiveInt(c.Query("pageSize"), 20),
			IP:          c.Query("ip"),
			CountryCode: c.Query("countryCode"),
			StartTime:   startTime,
			EndTime:     endTime,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询拦截日志失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	// 查询单条 IP 所属国家/地区
	r.GET("/operate/ip-block/lookup", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "IP 拦截服务未初始化"))
			return
		}
		ipStr := c.Query("ip")
		if ipStr == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "IP 参数不能为空"))
			return
		}
		countryCode, err := service.LookupIP(c.Request.Context(), ipStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]interface{}{
			"ip":          ipStr,
			"countryCode": countryCode,
		}))
	})

	// CTI 内网日志记录 Webhook (主要供 Kamailio 或防火墙抓取守护进程调用)
	r.POST("/cti/kamailio/ipblock/log", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "IP 拦截服务未初始化"))
			return
		}
		var req struct {
			IP          string `json:"ip"`
			CountryCode string `json:"countryCode"`
			CallID      string `json:"callId"`
			Method      string `json:"method"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		saved, err := service.LogBlockEvent(c.Request.Context(), req.IP, req.CountryCode, req.CallID, req.Method)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(saved))
	})
}
