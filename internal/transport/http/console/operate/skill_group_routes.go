package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterSkillGroupRoutes 注册商户技能组管理接口。
func RegisterSkillGroupRoutes(r gin.IRoutes, service *operatedomain.SkillGroupManagementService) {
	r.POST("/merchant/skill-group/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "技能组管理未启用"))
			return
		}
		var req operatedomain.SkillGroupPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询技能组失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/merchant/skill-group/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "技能组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeSkillGroupError(c, err, "技能组不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.PUT("/merchant/skill-group/add", saveSkillGroup(service))
	r.POST("/merchant/skill-group/update", saveSkillGroup(service))
	r.POST("/merchant/skill-group/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "技能组管理未启用"))
			return
		}
		var req []operatedomain.SkillGroup
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), req); err != nil {
			writeSkillGroupError(c, err, "删除技能组失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(req)}))
	})

	r.POST("/merchant/skill-group/users/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "技能组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			UserIDs []int `json:"userIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.ReplaceUsers(c.Request.Context(), id, req.UserIDs); err != nil {
			writeSkillGroupError(c, err, "保存技能组用户失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"skillGroupId": id, "userCount": len(req.UserIDs)}))
	})

	r.GET("/merchant/skill-group/users/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "技能组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		ids, err := service.UsersBySkillGroup(c.Request.Context(), id)
		if err != nil {
			writeSkillGroupError(c, err, "查询技能组用户失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"skillGroupId": id, "userIds": ids}))
	})

	r.POST("/merchant/skill-group/phones/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "技能组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			PhoneIDs []int `json:"phoneIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.ReplacePhones(c.Request.Context(), id, req.PhoneIDs); err != nil {
			writeSkillGroupError(c, err, "保存技能组号码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"skillGroupId": id, "phoneCount": len(req.PhoneIDs)}))
	})

	r.GET("/merchant/skill-group/phones/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "技能组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		ids, err := service.PhonesBySkillGroup(c.Request.Context(), id)
		if err != nil {
			writeSkillGroupError(c, err, "查询技能组号码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"skillGroupId": id, "phoneIds": ids}))
	})

	r.GET("/merchant/skill-group", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "技能组管理未启用"))
			return
		}
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		page, err := service.Page(c.Request.Context(), operatedomain.SkillGroupPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Name:       c.Query("name"),
			MerchantID: parsePositiveInt(c.Query("merchantId"), 0),
			Enable:     enablePtr(enable),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询技能组失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func saveSkillGroup(service *operatedomain.SkillGroupManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "技能组管理未启用"))
			return
		}
		var req operatedomain.SkillGroup
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeSkillGroupError(c, err, "保存技能组失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func writeSkillGroupError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidSkillGroup):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "技能组参数错误"))
	case errors.Is(err, operatedomain.ErrSkillGroupConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "技能组名称已存在"))
	case errors.Is(err, operatedomain.ErrSkillGroupNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "技能组不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
