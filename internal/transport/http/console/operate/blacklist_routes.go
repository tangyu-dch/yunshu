package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterBlacklistRoutes 注册运营端黑名单管理接口。
func RegisterBlacklistRoutes(r gin.IRoutes, service *operatedomain.BlacklistManagementService) {
	r.POST("/operate/blacklist/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		var req operatedomain.BlacklistPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询黑名单失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/blacklist/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		item, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeBlacklistError(c, err, "黑名单不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(item))
	})

	r.PUT("/operate/blacklist/add", saveBlacklist(service))
	r.POST("/operate/blacklist/update", saveBlacklist(service))
	r.POST("/operate/blacklist/delete/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		if err := service.Delete(c.Request.Context(), id); err != nil {
			writeBlacklistError(c, err, "删除黑名单失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.GET("/operate/blacklist", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		page, err := service.Page(c.Request.Context(), operatedomain.BlacklistPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Name:       c.Query("name"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询黑名单失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.POST("/operate/blacklist/numbers/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		var req operatedomain.BlacklistNumberPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.PageNumbers(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询黑名单号码失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.POST("/operate/blacklist/numbers/save", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		var req operatedomain.BlacklistNumber
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		num, err := service.SaveNumber(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(num))
	})

	r.POST("/operate/blacklist/numbers/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		var req struct {
			Phones []string `json:"phones"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.DeleteNumbers(c.Request.Context(), req.Phones); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除黑名单号码失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.GET("/operate/blacklist/channels", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		list, err := service.ListChannels(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取三方验证通道失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	})

	r.POST("/operate/blacklist/channels/save", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		var req operatedomain.BlacklistChannel
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.SaveChannel(c.Request.Context(), req); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.DELETE("/operate/blacklist/channels/:code", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		var req struct {
			Code int `uri:"code" binding:"required"`
		}
		if err := c.ShouldBindUri(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "参数错误"))
			return
		}
		if err := service.DeleteChannel(c.Request.Context(), req.Code); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "删除通道失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})
}

func saveBlacklist(service *operatedomain.BlacklistManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "黑名单管理未启用"))
			return
		}
		var req operatedomain.Blacklist
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeBlacklistError(c, err, "保存黑名单失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func writeBlacklistError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidBlacklist):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "黑名单参数错误"))
	case errors.Is(err, operatedomain.ErrBlacklistConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "黑名单名称已存在"))
	case errors.Is(err, operatedomain.ErrBlacklistNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "黑名单不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
