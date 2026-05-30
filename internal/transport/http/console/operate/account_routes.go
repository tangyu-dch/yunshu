package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

type resetPasswordRequest struct {
	Password string `json:"password"`
}

// RegisterAccountRoutes 注册控制台账号体系管理接口。
//
// 运营端路径用于超级管理员和授权运营账号维护平台/商户账号；
// 商户端路径用于商户管理员维护本商户实际使用账号，领域服务会再次校验租户范围。
func RegisterAccountRoutes(r gin.IRoutes, service *operatedomain.AccountManagementService) {
	registerAccountRoutesWithPrefix(r, service, "/operate/account")
	registerAccountRoutesWithPrefix(r, service, "/merchant/account")
}

func registerAccountRoutesWithPrefix(r gin.IRoutes, service *operatedomain.AccountManagementService, prefix string) {
	r.POST(prefix+"/page", func(c *gin.Context) {
		var req operatedomain.AccountPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			writeAccountError(c, err, "查询账号失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET(prefix+"/detail/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		account, err := service.Get(c.Request.Context(), id)
		if err != nil {
			writeAccountError(c, err, "账号不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(account))
	})

	r.PUT(prefix+"/add", saveAccount(service))
	r.POST(prefix+"/update", saveAccount(service))
	r.POST(prefix+"/delete", func(c *gin.Context) {
		var req []operatedomain.Account
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Delete(c.Request.Context(), req)
		if err != nil {
			writeAccountError(c, err, "删除账号失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST(prefix+"/enable/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		result, err := service.Enable(c.Request.Context(), id, true)
		if err != nil {
			writeAccountError(c, err, "启用账号失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST(prefix+"/disable/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		result, err := service.Enable(c.Request.Context(), id, false)
		if err != nil {
			writeAccountError(c, err, "停用账号失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST(prefix+"/reset-password/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req resetPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.ResetPassword(c.Request.Context(), id, req.Password)
		if err != nil {
			writeAccountError(c, err, "重置账号密码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.GET(prefix, func(c *gin.Context) {
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		page, err := service.Page(c.Request.Context(), operatedomain.AccountPageRequest{
			PageNumber:  parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:    parsePositiveInt(c.Query("pageSize"), 20),
			Username:    c.Query("username"),
			MerchantID:  c.Query("merchantId"),
			AccountType: c.Query("accountType"),
			RoleID:      c.Query("roleId"),
			Enable:      enablePtr(enable),
		})
		if err != nil {
			writeAccountError(c, err, "查询账号失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func saveAccount(service *operatedomain.AccountManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req operatedomain.Account
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeAccountError(c, err, "保存账号失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	}
}

func writeAccountError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidAccount):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "账号参数错误"))
	case errors.Is(err, operatedomain.ErrAccountConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "账号名已存在或商户管理员已存在"))
	case errors.Is(err, operatedomain.ErrAccountForbidden):
		c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "无权维护该账号"))
	case errors.Is(err, operatedomain.ErrAccountNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "账号不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
