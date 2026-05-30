package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterGatewayRoutes 注册运营端网关管理接口。
//
// 同时支持 Go 统一前缀 `/operate/gateway` 和  原始路径中的动作名语义：
// add、update、delete、page。写操作返回 syncRequired，提示运行时需要刷新 ESL/CTI 网关缓存。
func RegisterGatewayRoutes(r gin.IRoutes, service *operatedomain.GatewayManagementService) {
	r.POST("/operate/gateway/page", func(c *gin.Context) {
		var req operatedomain.GatewayPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询网关失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/gateway/detail/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		gateway, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeGatewayError(c, err, "网关不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(gateway))
	})

	r.PUT("/operate/gateway/add", saveGateway(service))
	r.POST("/operate/gateway/update", saveGateway(service))
	r.POST("/operate/gateway/delete", func(c *gin.Context) {
		var req []operatedomain.Gateway
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Delete(c.Request.Context(), req)
		if err != nil {
			writeGatewayError(c, err, "网关不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST("/operate/gateway/sync/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		result, err := service.Sync(c.Request.Context(), id)
		if err != nil {
			writeGatewayError(c, err, "同步网关失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.GET("/operate/gateway/encode", func(c *gin.Context) {
		c.JSON(http.StatusOK, contracts.OK([]map[string]any{
			{"id": "PCMU", "name": "PCMU"},
			{"id": "PCMA", "name": "PCMA"},
			{"id": "G722", "name": "G722"},
			{"id": "G729", "name": "G729"},
		}))
	})

	r.GET("/operate/gateway", func(c *gin.Context) {
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		page, err := service.Page(c.Request.Context(), operatedomain.GatewayPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Name:       c.Query("name"),
			Enable:     enablePtr(enable),
			ChannelID:  parsePositiveInt(c.Query("channelId"), 0),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询网关失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func saveGateway(service *operatedomain.GatewayManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req operatedomain.Gateway
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeGatewayError(c, err, "保存网关失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	}
}

func writeGatewayError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidGateway):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "网关参数错误"))
	case errors.Is(err, operatedomain.ErrGatewayConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "网关名称或网关描述已存在"))
	case errors.Is(err, operatedomain.ErrGatewayNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "网关不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}

func parsePositiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func parseEnableQuery(raw string) (bool, func(bool) *bool) {
	if raw == "" {
		return false, func(bool) *bool { return nil }
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, func(bool) *bool { return nil }
	}
	return parsed, func(value bool) *bool { return &value }
}
