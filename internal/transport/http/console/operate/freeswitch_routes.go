// Package operate 注册 cc-console 运营端 HTTP 路由。
//
// transport 层只负责  兼容路径、请求绑定和 Result 响应，业务语义交给
// internal/domain/operate，避免控制器直接操作数据库或运行时连接池。
package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
	fsregistry "yunshu/internal/infra/telephony"
)

// RegisterFreeSwitchRoutes 注册运营端 FreeSWITCH 节点管理接口。
//
// 路由前缀对齐  兼容契约 `/operate/freeswitch`；保存、启停和删除只修改
// 配置真相，并在响应中标记 refreshRequired，提醒调用方刷新 cc-call 运行时。
func RegisterFreeSwitchRoutes(r gin.IRoutes, service *operatedomain.FreeSwitchManagementService) {
	r.GET("/operate/freeswitch/list", func(c *gin.Context) {
		nodes, err := service.List(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询 FreeSWITCH 节点失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nodes))
	})

	r.GET("/operate/freeswitch/detail/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		node, err := service.Registry.GetByID(c.Request.Context(), id)
		if err != nil {
			writeRegistryError(c, err, "FreeSWITCH 节点不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(node))
	})

	r.POST("/operate/freeswitch", saveFreeSwitchNode(service))
	r.PUT("/operate/freeswitch/:id", saveFreeSwitchNode(service))

	r.POST("/operate/freeswitch/:id/enable", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		node, err := service.Enable(c.Request.Context(), id, true)
		if err != nil {
			writeRegistryError(c, err, "FreeSWITCH 节点不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"node": node, "refreshRequired": true}))
	})

	r.POST("/operate/freeswitch/:id/disable", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		node, err := service.Enable(c.Request.Context(), id, false)
		if err != nil {
			writeRegistryError(c, err, "FreeSWITCH 节点不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"node": node, "refreshRequired": true}))
	})

	r.DELETE("/operate/freeswitch/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		if err := service.Delete(c.Request.Context(), id); err != nil {
			writeRegistryError(c, err, "FreeSWITCH 节点不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"refreshRequired": true}))
	})
}

func saveFreeSwitchNode(service *operatedomain.FreeSwitchManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req fsregistry.Node
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if idParam := c.Param("id"); idParam != "" {
			id, err := strconv.Atoi(idParam)
			if err != nil || id <= 0 {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "节点 ID 错误"))
				return
			}
			req.ID = id
		}
		node, err := service.Save(c.Request.Context(), req)
		if err != nil {
			if errors.Is(err, operatedomain.ErrInvalidFreeSwitchNode) {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "FreeSWITCH 节点参数错误"))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "保存 FreeSWITCH 节点失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"node": node, "refreshRequired": true}))
	}
}

func parseID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "节点 ID 错误"))
		return 0, false
	}
	return id, true
}

func writeRegistryError(c *gin.Context, err error, notFoundMessage string) {
	if errors.Is(err, fsregistry.ErrNodeNotFound) {
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, notFoundMessage))
		return
	}
	c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "FreeSWITCH 节点管理失败"))
}
