package operate

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterPhoneAttributionRoutes 注册号码归属地相关的运营管理 HTTP 路由。
func RegisterPhoneAttributionRoutes(r gin.IRoutes, service *operatedomain.PhoneAttributionManagementService) {
	r.POST("/operate/phone-attribution/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码归属地管理未启用"))
			return
		}
		var req operatedomain.PhoneAttributionPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询号码归属地数据失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.PUT("/operate/phone-attribution/add", savePhoneAttribution(service))
	r.POST("/operate/phone-attribution/update", savePhoneAttribution(service))

	r.POST("/operate/phone-attribution/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码归属地管理未启用"))
			return
		}
		var req []string
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), req); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除号码归属地映射失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(req)}))
	})

	r.GET("/operate/phone-attribution/lookup", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码归属地管理未启用"))
			return
		}
		phone := strings.TrimSpace(c.Query("phone"))
		if phone == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "号码不能为空"))
			return
		}
		attr, found, err := service.Lookup(c.Request.Context(), phone)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询归属地失败"))
			return
		}
		if !found {
			c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "归属地未找到"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(attr))
	})
}

func savePhoneAttribution(service *operatedomain.PhoneAttributionManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码归属地管理未启用"))
			return
		}
		var req operatedomain.PhoneAttribution
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			if err == operatedomain.ErrInvalidPhoneAttribution {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "归属地数据不合法"))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存号码归属地映射失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}
