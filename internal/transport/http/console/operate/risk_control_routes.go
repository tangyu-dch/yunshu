package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterRiskControlRoutes 注册运营端风控配置管理接口。
func RegisterRiskControlRoutes(r gin.IRoutes, service *operatedomain.RiskControlManagementService) {
	r.POST("/operate/risk-control/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "风控管理服务不可用"))
			return
		}
		var req operatedomain.RiskControlPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数格式错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "分页查询风控策略失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/risk-control/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "风控管理服务不可用"))
			return
		}
		id, ok := parseRiskControlID(c)
		if !ok {
			return
		}
		record, err := service.GetByID(c.Request.Context(), id)
		if err != nil {
			writeRiskControlError(c, err, "风控策略详情获取失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.PUT("/operate/risk-control/add", saveRiskControl(service))
	r.POST("/operate/risk-control/update", saveRiskControl(service))

	r.POST("/operate/risk-control/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "风控管理服务不可用"))
			return
		}
		var req []operatedomain.RiskControl
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), req); err != nil {
			writeRiskControlError(c, err, "删除风控策略失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(req)}))
	})

	r.GET("/operate/risk-control/merchants/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "风控管理服务不可用"))
			return
		}
		id, ok := parseRiskControlID(c)
		if !ok {
			return
		}
		bindings, err := service.GetMerchants(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询关联商户关系失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(bindings))
	})

	r.POST("/operate/risk-control/merchants/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "风控管理服务不可用"))
			return
		}
		id, ok := parseRiskControlID(c)
		if !ok {
			return
		}
		var req []operatedomain.RiskControlMerchant
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.SaveMerchants(c.Request.Context(), id, req); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存商户关联关系失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})
}

func saveRiskControl(service *operatedomain.RiskControlManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "风控管理服务不可用"))
			return
		}
		var req operatedomain.RiskControl
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数格式错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeRiskControlError(c, err, "保存风控配置失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func parseRiskControlID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "策略ID解析错误"))
		return 0, false
	}
	return id, true
}

func writeRiskControlError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidRiskControl):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "风控参数配置无效"))
	case errors.Is(err, operatedomain.ErrRiskControlConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "风控配置名称已冲突"))
	case errors.Is(err, operatedomain.ErrRiskControlNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "未找到目标风控配置"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
