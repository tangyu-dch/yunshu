package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
	"yunshu/internal/infra/merchant"
)

// RegisterRateRoutes 注册运营端费率管理接口。
func RegisterRateRoutes(r gin.IRoutes, service *operatedomain.RateManagementService) {
	r.POST("/operate/rate/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "费率管理未启用"))
			return
		}
		var req operatedomain.RatePageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询费率失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/rate/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "费率管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		tenant, _ := contracts.TenantFromContext(c.Request.Context())
		if !tenant.Internal && tenant.MerchantID != "" {
			dbRepo, ok := service.Repository.(*merchant.RateRepository)
			if ok {
				var count int64
				err := dbRepo.DB.WithContext(c.Request.Context()).Table("cc_mch_rate_ref").
					Where("rate_id = ? AND merchant_id = ?", id, tenant.MerchantID).Count(&count).Error
				if err != nil || count == 0 {
					c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权访问此费率套餐详情"))
					return
				}
			}
		}
		record, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeRateError(c, err, "费率不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.PUT("/operate/rate/add", saveRate(service))
	r.POST("/operate/rate/update", saveRate(service))
	r.POST("/operate/rate/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "费率管理未启用"))
			return
		}
		var req []operatedomain.Rate
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), req); err != nil {
			writeRateError(c, err, "删除费率失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(req)}))
	})

	// 获取公开可用费率（商户端自主绑定使用）
	r.GET("/operate/rate/list-active", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "费率管理未启用"))
			return
		}
		dbRepo, ok := service.Repository.(*merchant.RateRepository)
		if ok {
			var models []merchant.CallRateModel
			if err := dbRepo.DB.WithContext(c.Request.Context()).Where("del_flag = ? AND enable = ?", false, true).Order("rate_name ASC").Find(&models).Error; err != nil {
				c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取可用费率失败"))
				return
			}
			records := make([]operatedomain.Rate, len(models))
			for i, m := range models {
				records[i] = operatedomain.Rate{
					ID:           m.ID,
					RateName:     m.RateName,
					BillingPrice: m.BillingPrice,
					BillingCycle: m.BillingCycle,
					Remark:       m.Remark,
				}
			}
			c.JSON(http.StatusOK, contracts.OK(records))
			return
		}

		// 内存降级 fallback
		page, err := service.Page(c.Request.Context(), operatedomain.RatePageRequest{PageNumber: 1, PageSize: 1000})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取可用费率失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page.Records))
	})

	r.GET("/operate/rate", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "费率管理未启用"))
			return
		}
		tenant, _ := contracts.TenantFromContext(c.Request.Context())
		if !tenant.Internal && tenant.MerchantID != "" {
			dbRepo, ok := service.Repository.(*merchant.RateRepository)
			if ok {
				var boundRate struct {
					RateID int `gorm:"column:rate_id"`
				}
				err := dbRepo.DB.WithContext(c.Request.Context()).Table("cc_mch_rate_ref").
					Where("merchant_id = ?", tenant.MerchantID).First(&boundRate).Error
				if err != nil {
					// 未绑定费率，返回空分页
					c.JSON(http.StatusOK, contracts.OK(operatedomain.RatePageResult{
						PageNumber: 1, PageSize: 20, Total: 0, Records: []operatedomain.Rate{},
					}))
					return
				}
				record, err := service.Repository.GetByID(c.Request.Context(), boundRate.RateID)
				if err != nil {
					c.JSON(http.StatusOK, contracts.OK(operatedomain.RatePageResult{
						PageNumber: 1, PageSize: 20, Total: 0, Records: []operatedomain.Rate{},
					}))
					return
				}
				c.JSON(http.StatusOK, contracts.OK(operatedomain.RatePageResult{
					PageNumber: 1,
					PageSize:   20,
					Total:      1,
					Records:    []operatedomain.Rate{record},
				}))
				return
			}
		}

		page, err := service.Page(c.Request.Context(), operatedomain.RatePageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Name:       c.Query("name"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询费率失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func saveRate(service *operatedomain.RateManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "费率管理未启用"))
			return
		}
		var req operatedomain.Rate
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeRateError(c, err, "保存费率失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func writeRateError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidRate):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "费率参数错误"))
	case errors.Is(err, operatedomain.ErrRateConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "费率名称已存在"))
	case errors.Is(err, operatedomain.ErrRateReferenced):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "费率已被引用，不能删除"))
	case errors.Is(err, operatedomain.ErrRateNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "费率不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
