package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterMerchantRoutes 注册运营端商户管理接口。
//
// 路由对齐  兼容前缀 `/operate/merchant`，用于维护商户主体和启停状态。
func RegisterMerchantRoutes(r gin.IRoutes, service *operatedomain.MerchantManagementService) {
	r.POST("/operate/merchant/page", func(c *gin.Context) {
		var req operatedomain.MerchantPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询商户失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	detailHandler := func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		tenant, _ := contracts.TenantFromContext(c.Request.Context())
		if !tenant.Internal && tenant.MerchantID != "" {
			var mID int
			if parsed, err := strconv.Atoi(tenant.MerchantID); err == nil {
				mID = parsed
			}
			if mID != id {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权访问此商户详情"))
				return
			}
		}
		merchant, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeMerchantError(c, err, "商户不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(merchant))
	}
	r.GET("/operate/merchant/detail/:id", detailHandler)
	r.GET("/merchant/detail/:id", detailHandler)

	r.PUT("/operate/merchant/add", saveMerchant(service))
	r.POST("/operate/merchant/update", saveMerchant(service))
	r.POST("/operate/merchant/delete", func(c *gin.Context) {
		var req []operatedomain.Merchant
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Delete(c.Request.Context(), req)
		if err != nil {
			writeMerchantError(c, err, "删除商户失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST("/operate/merchant/enable/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		result, err := service.Enable(c.Request.Context(), id, true)
		if err != nil {
			writeMerchantError(c, err, "启用商户失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST("/operate/merchant/disable/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		result, err := service.Enable(c.Request.Context(), id, false)
		if err != nil {
			writeMerchantError(c, err, "停用商户失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.GET("/operate/merchant", func(c *gin.Context) {
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		page, err := service.Page(c.Request.Context(), operatedomain.MerchantPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Name:       c.Query("name"),
			Account:    c.Query("account"),
			Enable:     enablePtr(enable),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询商户失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func saveMerchant(service *operatedomain.MerchantManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req operatedomain.Merchant
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeMerchantError(c, err, "保存商户失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	}
}

func writeMerchantError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidMerchant):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "商户参数错误"))
	case errors.Is(err, operatedomain.ErrMerchantConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "商户名称或账号已存在"))
	case errors.Is(err, operatedomain.ErrMerchantNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "商户不存在"))
	case errors.Is(err, operatedomain.ErrRateNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "费率不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
