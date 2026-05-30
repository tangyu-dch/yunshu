package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterExtensionRoutes 注册运营端分机管理接口。
func RegisterExtensionRoutes(r gin.IRoutes, service *operatedomain.ExtensionManagementService) {
	r.POST("/operate/extension/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "分机管理未启用"))
			return
		}
		var req operatedomain.ExtensionPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询分机失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/extension/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "分机管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeExtensionError(c, err, "分机不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.PUT("/operate/extension/add", saveExtension(service))
	r.POST("/operate/extension/update", saveExtension(service))
	r.POST("/operate/extension/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "分机管理未启用"))
			return
		}
		var req []operatedomain.Extension
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), req); err != nil {
			writeExtensionError(c, err, "删除分机失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(req)}))
	})

	r.POST("/operate/extension/enable/:id", toggleExtension(service, true))
	r.POST("/operate/extension/disable/:id", toggleExtension(service, false))
	r.POST("/operate/extension/dynamic-bind", dynamicBindExtension(service))
	r.POST("/merchant/extension/dynamic-bind", dynamicBindExtension(service))

	r.GET("/operate/extension", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "分机管理未启用"))
			return
		}
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		merchantID, _ := strconv.Atoi(c.Query("merchantId"))
		userID, _ := strconv.Atoi(c.Query("userId"))
		page, err := service.Page(c.Request.Context(), operatedomain.ExtensionPageRequest{
			PageNumber:      parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:        parsePositiveInt(c.Query("pageSize"), 20),
			ExtensionNumber: c.Query("extensionNumber"),
			MerchantID:      merchantID,
			UserID:          userID,
			Enable:          enablePtr(enable),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询分机失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func saveExtension(service *operatedomain.ExtensionManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "分机管理未启用"))
			return
		}
		var req operatedomain.Extension
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeExtensionError(c, err, "保存分机失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func toggleExtension(service *operatedomain.ExtensionManagementService, enable bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "分机管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.SetEnable(c.Request.Context(), id, enable)
		if err != nil {
			writeExtensionError(c, err, "切换分机状态失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func writeExtensionError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidExtension):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "分机参数错误"))
	case errors.Is(err, operatedomain.ErrExtensionConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "分机号已存在"))
	case errors.Is(err, operatedomain.ErrExtensionNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "分机不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}

type DynamicBindReq struct {
	ExtensionNumber string `json:"extensionNumber" binding:"required"`
	UserID          int    `json:"userId" binding:"required"`
	MerchantID      int    `json:"merchantId" binding:"required"`
}

func dynamicBindExtension(service *operatedomain.ExtensionManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "分机管理未启用"))
			return
		}
		var req DynamicBindReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		err := service.DynamicBind(c.Request.Context(), req.ExtensionNumber, req.UserID, req.MerchantID)
		if err != nil {
			if errors.Is(err, operatedomain.ErrExtensionNotFound) {
				c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "分机不存在"))
				return
			}
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	}
}
