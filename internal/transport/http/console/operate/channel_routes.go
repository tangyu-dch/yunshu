package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterChannelRoutes 注册运营端渠道管理接口。
func RegisterChannelRoutes(r gin.IRoutes, service *operatedomain.ChannelManagementService) {
	r.POST("/operate/channel/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "渠道管理未启用"))
			return
		}
		var req operatedomain.ChannelPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询渠道失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/channel/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "渠道管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeChannelError(c, err, "渠道不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.PUT("/operate/channel/add", saveChannel(service))
	r.POST("/operate/channel/update", saveChannel(service))
	r.POST("/operate/channel/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "渠道管理未启用"))
			return
		}
		var req []operatedomain.Channel
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), req); err != nil {
			writeChannelError(c, err, "删除渠道失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(req)}))
	})

	r.GET("/operate/channel", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "渠道管理未启用"))
			return
		}
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		page, err := service.Page(c.Request.Context(), operatedomain.ChannelPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Name:       c.Query("name"),
			Enable:     enablePtr(enable),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询渠道失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func saveChannel(service *operatedomain.ChannelManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "渠道管理未启用"))
			return
		}
		var req operatedomain.Channel
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeChannelError(c, err, "保存渠道失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func writeChannelError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidChannel):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "渠道参数错误"))
	case errors.Is(err, operatedomain.ErrChannelConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "渠道名称已存在"))
	case errors.Is(err, operatedomain.ErrChannelNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "渠道不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
