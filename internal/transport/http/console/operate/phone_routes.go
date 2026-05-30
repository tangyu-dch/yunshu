package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterPoolPhoneRoutes 注册运营端号码管理接口。
func RegisterPoolPhoneRoutes(r gin.IRoutes, service *operatedomain.PoolPhoneManagementService) {
	r.POST("/operate/pool-phone/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码管理未启用"))
			return
		}
		var req operatedomain.PoolPhonePageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询号码失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/pool-phone/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writePoolPhoneError(c, err, "号码不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.PUT("/operate/pool-phone/add", savePoolPhone(service))
	r.POST("/operate/pool-phone/update", savePoolPhone(service))
	r.POST("/operate/pool-phone/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码管理未启用"))
			return
		}
		var req []operatedomain.PoolPhone
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), req); err != nil {
			writePoolPhoneError(c, err, "删除号码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(req)}))
	})

	r.POST("/operate/pool-phone/enable/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.SetEnable(c.Request.Context(), id, true)
		if err != nil {
			writePoolPhoneError(c, err, "启用号码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.POST("/operate/pool-phone/disable/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.SetEnable(c.Request.Context(), id, false)
		if err != nil {
			writePoolPhoneError(c, err, "停用号码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.POST("/operate/pool-phone/batch-move", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码管理未启用"))
			return
		}
		var req struct {
			PhoneIDs []int `json:"phoneIds"`
			PoolID   int   `json:"poolId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.SetPool(c.Request.Context(), req.PhoneIDs, req.PoolID); err != nil {
			writePoolPhoneError(c, err, "批量移动号码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"moved": len(req.PhoneIDs)}))
	})

	r.GET("/operate/pool-phone", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码管理未启用"))
			return
		}
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		page, err := service.Page(c.Request.Context(), operatedomain.PoolPhonePageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			PoolID:     parsePositiveInt(c.Query("poolId"), 0),
			Phone:      c.Query("phone"),
			Enable:     enablePtr(enable),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询号码失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func savePoolPhone(service *operatedomain.PoolPhoneManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码管理未启用"))
			return
		}
		var req operatedomain.PoolPhone
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writePoolPhoneError(c, err, "保存号码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func writePoolPhoneError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidPoolPhone):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "号码参数错误"))
	case errors.Is(err, operatedomain.ErrPoolPhoneConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "号码已存在"))
	case errors.Is(err, operatedomain.ErrPoolPhoneNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "号码不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
