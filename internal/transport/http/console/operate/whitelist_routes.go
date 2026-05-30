package operate

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterWhitelistRoutes 注册运营端白名单管理接口。
func RegisterWhitelistRoutes(r gin.IRoutes, service *operatedomain.WhitelistManagementService) {
	r.POST("/operate/whitelist/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "白名单管理未启用"))
			return
		}
		var req operatedomain.WhitelistPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询白名单失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.PUT("/operate/whitelist/add", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "白名单管理未启用"))
			return
		}
		var req operatedomain.AddWhitelistRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		message, err := service.Add(c.Request.Context(), req)
		if err != nil {
			writeWhitelistError(c, err, "添加白名单失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(message))
	})

	r.POST("/operate/whitelist/update", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "白名单管理未启用"))
			return
		}
		var req operatedomain.UpdateWhitelistRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Update(c.Request.Context(), req); err != nil {
			writeWhitelistError(c, err, "编辑白名单失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.GET("/operate/whitelist/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "白名单管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.Detail(c.Request.Context(), id)
		if err != nil {
			writeWhitelistError(c, err, "白名单不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.POST("/operate/whitelist/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "白名单管理未启用"))
			return
		}
		ids := parseWhitelistDeleteIDs(c.Query("whiteIds"))
		if len(ids) == 0 {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), ids); err != nil {
			writeWhitelistError(c, err, "删除白名单失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.GET("/operate/whitelist", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "白名单管理未启用"))
			return
		}
		page, err := service.Page(c.Request.Context(), operatedomain.WhitelistPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Number:     c.Query("number"),
			MerchantID: parsePositiveInt(c.Query("merchantId"), 0),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询白名单失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func writeWhitelistError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidWhitelist):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "白名单参数错误"))
	case errors.Is(err, operatedomain.ErrWhitelistNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "白名单不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}

func parseWhitelistDeleteIDs(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}
