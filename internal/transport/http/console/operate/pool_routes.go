package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterPoolRoutes 注册运营端号码池管理接口。
func RegisterPoolRoutes(r gin.IRoutes, service *operatedomain.PoolManagementService) {
	r.POST("/operate/pool/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码池管理未启用"))
			return
		}
		var req operatedomain.PoolPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询号码池失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/pool/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码池管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writePoolError(c, err, "号码池不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.PUT("/operate/pool/add", savePool(service))
	r.POST("/operate/pool/update", savePool(service))
	r.POST("/operate/pool/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码池管理未启用"))
			return
		}
		var req []operatedomain.Pool
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), req); err != nil {
			writePoolError(c, err, "删除号码池失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(req)}))
	})

	r.GET("/operate/pool/list/:gatewayId", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码池管理未启用"))
			return
		}
		gatewayID, err := strconv.Atoi(c.Param("gatewayId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "网关 ID 错误"))
			return
		}
		records, err := service.ListByGateway(c.Request.Context(), gatewayID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询号码池失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(records))
	})

	r.GET("/operate/pool/list", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码池管理未启用"))
			return
		}
		records, err := service.ListAll(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询号码池失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(records))
	})

	r.GET("/operate/pool", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码池管理未启用"))
			return
		}
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		gatewayID, _ := strconv.Atoi(c.Query("gatewayId"))
		page, err := service.Page(c.Request.Context(), operatedomain.PoolPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Name:       c.Query("name"),
			GatewayID:  gatewayID,
			Enable:     enablePtr(enable),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询号码池失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func savePool(service *operatedomain.PoolManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码池管理未启用"))
			return
		}
		var req operatedomain.Pool
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writePoolError(c, err, "保存号码池失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func writePoolError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidPool):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "号码池参数错误"))
	case errors.Is(err, operatedomain.ErrPoolConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "号码池名称已存在"))
	case errors.Is(err, operatedomain.ErrPoolNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "号码池不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
