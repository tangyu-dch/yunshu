package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterBillingRoutes 注册运营端商户账务接口。
func RegisterBillingRoutes(r gin.IRoutes, service *operatedomain.BillingManagementService) {
	r.POST("/operate/billing/overview/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "商户账务管理未启用"))
			return
		}
		var req operatedomain.BillingOverviewPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.PageOverview(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询商户账务总览失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.POST("/operate/billing/overview/save", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "商户账务管理未启用"))
			return
		}
		var req operatedomain.BillingOverviewSaveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.SaveOverview(c.Request.Context(), req)
		if err != nil {
			writeBillingError(c, err, "保存商户账务配置失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST("/operate/billing/recharge", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "商户账务管理未启用"))
			return
		}
		var req operatedomain.MerchantRechargeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Recharge(c.Request.Context(), req); err != nil {
			writeBillingError(c, err, "调整商户余额失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.POST("/operate/billing/recharge-records", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "商户账务管理未启用"))
			return
		}
		var req operatedomain.MerchantRechargePageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.PageRechargeRecords(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询充值记录失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func writeBillingError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidBilling):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "商户账务参数错误"))
	case errors.Is(err, operatedomain.ErrBillingNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "商户账务不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
