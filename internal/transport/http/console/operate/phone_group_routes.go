package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterPhoneGroupRoutes 注册商户号码组管理接口。
func RegisterPhoneGroupRoutes(r gin.IRoutes, service *operatedomain.PhoneGroupManagementService) {
	r.POST("/merchant/phone-group/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码组管理未启用"))
			return
		}
		var req operatedomain.PhoneGroupPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询号码组失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/merchant/phone-group/detail/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		record, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writePhoneGroupError(c, err, "号码组不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.PUT("/merchant/phone-group/add", savePhoneGroup(service))
	r.POST("/merchant/phone-group/update", savePhoneGroup(service))
	r.POST("/merchant/phone-group/delete", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码组管理未启用"))
			return
		}
		var req []operatedomain.PhoneGroup
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.Delete(c.Request.Context(), req); err != nil {
			writePhoneGroupError(c, err, "删除号码组失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"deleted": len(req)}))
	})

	r.POST("/merchant/phone-group/phones/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			MerchantID int   `json:"merchantId"`
			PhoneIDs   []int `json:"phoneIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.ReplacePhones(c.Request.Context(), id, req.MerchantID, req.PhoneIDs); err != nil {
			writePhoneGroupError(c, err, "保存号码组号码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"phoneGroupId": id, "phoneCount": len(req.PhoneIDs)}))
	})

	r.GET("/merchant/phone-group/phones/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		ids, err := service.PhonesByGroup(c.Request.Context(), id)
		if err != nil {
			writePhoneGroupError(c, err, "查询号码组号码失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"phoneGroupId": id, "phoneIds": ids}))
	})

	r.POST("/merchant/phone-group/skill-groups/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			MerchantID    int   `json:"merchantId"`
			SkillGroupIDs []int `json:"skillGroupIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if err := service.ReplaceSkillGroups(c.Request.Context(), id, req.MerchantID, req.SkillGroupIDs); err != nil {
			writePhoneGroupError(c, err, "保存号码组技能组失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"phoneGroupId": id, "skillGroupCount": len(req.SkillGroupIDs)}))
	})

	r.GET("/merchant/phone-group/skill-groups/:id", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码组管理未启用"))
			return
		}
		id, ok := parseID(c)
		if !ok {
			return
		}
		ids, err := service.SkillGroupsByGroup(c.Request.Context(), id)
		if err != nil {
			writePhoneGroupError(c, err, "查询号码组技能组失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"phoneGroupId": id, "skillGroupIds": ids}))
	})

	r.GET("/merchant/phone-group", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码组管理未启用"))
			return
		}
		enable, enablePtr := parseEnableQuery(c.Query("enable"))
		merchantID, _ := strconv.Atoi(c.Query("merchantId"))
		page, err := service.Page(c.Request.Context(), operatedomain.PhoneGroupPageRequest{
			PageNumber: parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:   parsePositiveInt(c.Query("pageSize"), 20),
			Name:       c.Query("name"),
			MerchantID: merchantID,
			Enable:     enablePtr(enable),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询号码组失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func savePhoneGroup(service *operatedomain.PhoneGroupManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "号码组管理未启用"))
			return
		}
		var req operatedomain.PhoneGroup
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		record, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writePhoneGroupError(c, err, "保存号码组失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	}
}

func writePhoneGroupError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidPhoneGroup):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "号码组参数错误"))
	case errors.Is(err, operatedomain.ErrPhoneGroupConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "号码组名称已存在"))
	case errors.Is(err, operatedomain.ErrPhoneGroupNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "号码组不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}
