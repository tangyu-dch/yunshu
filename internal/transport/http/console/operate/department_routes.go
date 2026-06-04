package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterDepartmentRoutes 注册商户部门管理路由接口。
func RegisterDepartmentRoutes(r gin.IRoutes, service *operatedomain.DepartmentManagementService) {
	r.POST("/merchant/department/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "部门管理服务未启用"))
			return
		}
		var req operatedomain.DepartmentPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		// 数据隔离校验
		tenant, ok := contracts.TenantFromContext(c.Request.Context())
		if ok {
			if !tenant.Internal {
				parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
				req.MerchantID = parsedMerchantID
			} else {
				if req.MerchantID <= 0 {
					if ctxMerchantID := c.GetHeader("X-Merchant-Id"); ctxMerchantID != "" {
						if parsed, err := strconv.Atoi(ctxMerchantID); err == nil {
							req.MerchantID = parsed
						}
					}
				}
			}
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "分页查询部门失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/merchant/department/list", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "部门管理服务未启用"))
			return
		}
		var merchantID int
		tenant, ok := contracts.TenantFromContext(c.Request.Context())
		if ok && !tenant.Internal {
			merchantID, _ = strconv.Atoi(tenant.MerchantID)
		} else {
			mIDStr := c.Query("merchantId")
			if mIDStr != "" {
				merchantID, _ = strconv.Atoi(mIDStr)
			}
			if merchantID <= 0 {
				if ctxMerchantID := c.GetHeader("X-Merchant-Id"); ctxMerchantID != "" {
					if parsed, err := strconv.Atoi(ctxMerchantID); err == nil {
						merchantID = parsed
					}
				}
			}
		}
		if merchantID <= 0 {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "商户 ID 错误"))
			return
		}
		list, err := service.ListAll(c.Request.Context(), merchantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取部门列表失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	})

	r.POST("/merchant/department/save", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "部门管理服务未启用"))
			return
		}
		var dept operatedomain.Department
		if err := c.ShouldBindJSON(&dept); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		// 数据隔离校验与强制重写
		tenant, ok := contracts.TenantFromContext(c.Request.Context())
		if ok && !tenant.Internal {
			parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
			dept.MerchantID = parsedMerchantID
		}
		if dept.MerchantID <= 0 {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "商户 ID 错误"))
			return
		}
		saved, err := service.Save(c.Request.Context(), dept)
		if err != nil {
			if errors.Is(err, operatedomain.ErrInvalidDepartment) {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "部门参数验证失败"))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存部门失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(saved))
	})

	r.POST("/merchant/department/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "部门管理服务未启用"))
			return
		}
		var req []struct {
			ID int `json:"id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		ids := make([]int, 0, len(req))
		for _, item := range req {
			if item.ID > 0 {
				ids = append(ids, item.ID)
			}
		}
		if len(ids) == 0 {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "未指定要删除的部门"))
			return
		}
		// 数据隔离：普通商户只能删自己的部门
		tenant, ok := contracts.TenantFromContext(c.Request.Context())
		if ok && !tenant.Internal {
			parsedMerchantID, _ := strconv.Atoi(tenant.MerchantID)
			for _, id := range ids {
				existing, err := service.Repository.GetByID(c.Request.Context(), id)
				if err == nil && existing.MerchantID != parsedMerchantID {
					c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权删除其他商户的部门"))
					return
				}
			}
		}
		if err := service.Delete(c.Request.Context(), ids); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除部门失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})
}
