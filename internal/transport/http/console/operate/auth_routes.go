package operate

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	authdomain "yunshu/internal/domain/auth"
)

// RegisterAuthRoutes 注册管理端登录和 token 查询接口。
func RegisterAuthRoutes(r gin.IRoutes, service *authdomain.AuthService) {
	registerLoginRoutes(r, service, "/operate/auth", true)
	registerLoginRoutes(r, service, "/merchant/auth", false)
}

func registerLoginRoutes(r gin.IRoutes, service *authdomain.AuthService, prefix string, internal bool) {
	r.POST(prefix+"/login", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "认证服务未启用"))
			return
		}
		var req contracts.AuthLoginReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		req.Internal = internal
		ticket, err := service.Login(c.Request.Context(), authdomain.LoginRequest{
			Username:    req.Username,
			Password:    req.Password,
			MerchantID:  req.MerchantID,
			UserID:      req.UserID,
			RoleID:      req.RoleID,
			DataScope:   req.DataScope,
			Permissions: req.Permissions,
			Internal:    req.Internal,
		})
		if err != nil {
			if strings.EqualFold(err.Error(), authdomain.ErrInvalidLogin.Error()) {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "登录参数错误"))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "登录失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(contracts.AuthLoginResp{
			Token:     ticket.Token,
			ExpiresAt: ticket.ExpiresAt,
			Tenant:    ticket.Tenant,
		}))
	})

	r.POST(prefix+"/logout", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "认证服务未启用"))
			return
		}
		token := tokenFromRequest(c)
		if err := service.Logout(c.Request.Context(), token); err != nil {
			if strings.EqualFold(err.Error(), authdomain.ErrInvalidLogin.Error()) {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "token 错误"))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "退出失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"logout": true}))
	})

	r.GET(prefix+"/token", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "认证服务未启用"))
			return
		}
		token := tokenFromRequest(c)
		ticket, ok := service.Token(c.Request.Context(), token)
		if !ok {
			c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "token 无效或已过期"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(contracts.AuthLoginResp{
			Token:     ticket.Token,
			ExpiresAt: ticket.ExpiresAt,
			Tenant:    ticket.Tenant,
		}))
	})
}

func tokenFromRequest(c *gin.Context) string {
	if token := c.Query("token"); token != "" {
		return token
	}
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return strings.TrimSpace(c.GetHeader("X-Token"))
}
