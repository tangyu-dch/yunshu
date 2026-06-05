package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterDispatcherRoutes 注册运营端 Kamailio Dispatcher 管理接口。
func RegisterDispatcherRoutes(r gin.IRoutes, service *operatedomain.DispatcherManagementService) {
	r.POST("/operate/kamailio/dispatcher/page", func(c *gin.Context) {
		var req operatedomain.DispatcherPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询 Dispatcher 失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/kamailio/dispatcher/detail/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		disp, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeDispatcherError(c, err, "Dispatcher 记录不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(disp))
	})

	r.PUT("/operate/kamailio/dispatcher/add", saveDispatcher(service))
	r.POST("/operate/kamailio/dispatcher/add", saveDispatcher(service))
	r.POST("/operate/kamailio/dispatcher/update", saveDispatcher(service))

	r.POST("/operate/kamailio/dispatcher/delete", func(c *gin.Context) {
		var req []operatedomain.Dispatcher
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Delete(c.Request.Context(), req)
		if err != nil {
			writeDispatcherError(c, err, "删除 Dispatcher 失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST("/operate/kamailio/dispatcher/reload", func(c *gin.Context) {
		result, err := service.Reload(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "刷新 Dispatcher 失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.GET("/operate/kamailio/dispatcher", func(c *gin.Context) {
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		page, err := service.Page(c.Request.Context(), operatedomain.DispatcherPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			SetID:      parsePositiveInt(c.Query("setId"), 0),
			Enable:     enablePtr(enable),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询 Dispatcher 失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func saveDispatcher(service *operatedomain.DispatcherManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req operatedomain.Dispatcher
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeDispatcherError(c, err, "保存 Dispatcher 失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	}
}

func writeDispatcherError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidDispatcher):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "Dispatcher 参数错误"))
	case errors.Is(err, operatedomain.ErrDispatcherConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "Dispatcher 目的地址已存在"))
	case errors.Is(err, operatedomain.ErrDispatcherNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "Dispatcher 记录不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
