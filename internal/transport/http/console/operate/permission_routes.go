package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
	"yunshu/internal/infra/system"
)

// RegisterPermissionRoutes 注册运营端角色权限管理接口以及商户端套餐绑定接口。
func RegisterPermissionRoutes(r gin.IRoutes, repo *system.PermissionRepository, merchantService *operatedomain.MerchantManagementService) {
	if repo == nil {
		return
	}

	// ==========================================
	// 运营端角色与权限配置 API
	// ==========================================

	// 分页查询角色
	r.POST("/operate/role/page", func(c *gin.Context) {
		var req system.RolePageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := repo.PageRoles(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询角色列表失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	// 保存角色 (新增/更新)
	saveRoleHandler := func(c *gin.Context) {
		var req system.ConsoleRoleModel
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		saved, err := repo.SaveRole(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(saved))
	}
	r.PUT("/operate/role/add", saveRoleHandler)
	r.POST("/operate/role/update", saveRoleHandler)

	// 删除角色
	r.POST("/operate/role/delete", func(c *gin.Context) {
		var req []system.ConsoleRoleModel
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		codes := make([]string, 0, len(req))
		for _, role := range req {
			if role.Code != "" {
				codes = append(codes, role.Code)
			}
		}
		if err := repo.DeleteRoles(c.Request.Context(), codes); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(codes)}))
	})

	// 启用角色
	r.POST("/operate/role/enable/:code", func(c *gin.Context) {
		code := c.Param("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "角色编码不能为空"))
			return
		}
		role := system.ConsoleRoleModel{Code: code, Enable: true}
		saved, err := repo.SaveRole(c.Request.Context(), role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(saved))
	})

	// 停用角色
	r.POST("/operate/role/disable/:code", func(c *gin.Context) {
		code := c.Param("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "角色编码不能为空"))
			return
		}
		role := system.ConsoleRoleModel{Code: code, Enable: false}
		saved, err := repo.SaveRole(c.Request.Context(), role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(saved))
	})

	// 获取所有权限定义
	r.GET("/operate/permission", func(c *gin.Context) {
		list, err := repo.ListPermissions(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取权限目录失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(list))
	})

	// 获取角色绑定的权限列表
	r.GET("/operate/role/permissions/:code", func(c *gin.Context) {
		code := c.Param("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "角色编码不能为空"))
			return
		}
		permissions, err := repo.GetRolePermissions(c.Request.Context(), code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "获取角色关联权限失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(permissions))
	})

	// 保存角色关联的权限列表
	type SaveRolePermissionsReq struct {
		RoleCode        string   `json:"roleCode"`
		PermissionCodes []string `json:"permissionCodes"`
	}
	r.POST("/operate/role/permissions/save", func(c *gin.Context) {
		var req SaveRolePermissionsReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		err := repo.SaveRolePermissions(c.Request.Context(), req.RoleCode, req.PermissionCodes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	// ==========================================
	// 商户端套餐自主选择与绑定 API
	// ==========================================
	type MerchantRateBindReq struct {
		RateID int `json:"rateId"`
	}
	r.POST("/merchant/billing/rate/bind", func(c *gin.Context) {
		if merchantService == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "商户管理服务未启用"))
			return
		}
		var req MerchantRateBindReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		tenant, _ := contracts.TenantFromContext(c.Request.Context())
		if tenant.MerchantID == "" {
			c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "仅商户租户允许绑定套餐"))
			return
		}
		mID, err := strconv.Atoi(tenant.MerchantID)
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "无效的商户身份"))
			return
		}

		// 获取商户实体
		merchant, err := merchantService.Repository.GetByID(c.Request.Context(), mID)
		if err != nil {
			if errors.Is(err, operatedomain.ErrMerchantNotFound) {
				c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "商户未找到"))
			} else {
				c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询商户失败"))
			}
			return
		}

		// 检查费率是否存在
		rateExists, err := merchantService.Repository.RateExists(c.Request.Context(), req.RateID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "验证费率状态失败"))
			return
		}
		if !rateExists {
			c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "选择的套餐费率不存在"))
			return
		}

		// 更新商户绑定的 rateId 并落库
		merchant.RateID = req.RateID
		_, err = merchantService.Repository.Save(c.Request.Context(), merchant)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "更新套餐绑定关系失败"))
			return
		}

		c.JSON(http.StatusOK, contracts.OK(nil))
	})
}
