package operate

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterRtpengineRoutes 注册运营端 Kamailio RTPEngine 管理接口。
func RegisterRtpengineRoutes(r gin.IRoutes, service *operatedomain.RtpengineManagementService) {
	r.POST("/operate/kamailio/rtpengine/page", func(c *gin.Context) {
		var req operatedomain.RtpenginePageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询 RTPEngine 失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/operate/kamailio/rtpengine/detail/:id", func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		engine, err := service.Repository.GetByID(c.Request.Context(), id)
		if err != nil {
			writeRtpengineError(c, err, "RTPEngine 不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(engine))
	})

	r.PUT("/operate/kamailio/rtpengine/add", saveRtpengine(service))
	r.POST("/operate/kamailio/rtpengine/update", saveRtpengine(service))

	r.POST("/operate/kamailio/rtpengine/delete", func(c *gin.Context) {
		var req []operatedomain.Rtpengine
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Delete(c.Request.Context(), req)
		if err != nil {
			writeRtpengineError(c, err, "删除 RTPEngine 失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST("/operate/kamailio/rtpengine/reload", func(c *gin.Context) {
		result, err := service.Reload(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "刷新 RTPEngine 失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.GET("/operate/kamailio/rtpengine", func(c *gin.Context) {
		disabled, disabledPtr := parseDisabledQuery(c.Query("disabled"))
		page, err := service.Page(c.Request.Context(), operatedomain.RtpenginePageRequest{
			PageNumber:    parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:      parsePositiveInt(c.Query("pageSize"), 20),
			SetID:         parsePositiveInt(c.Query("setId"), 0),
			RtpengineSock: c.Query("rtpengineSock"),
			Disabled:      disabledPtr(disabled),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询 RTPEngine 失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func saveRtpengine(service *operatedomain.RtpengineManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req operatedomain.Rtpengine
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		result, err := service.Save(c.Request.Context(), req)
		if err != nil {
			writeRtpengineError(c, err, "保存 RTPEngine 失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	}
}

func writeRtpengineError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, operatedomain.ErrInvalidRtpengine):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "RTPEngine 参数错误"))
	case errors.Is(err, operatedomain.ErrRtpengineConflict):
		c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "RTPEngine 连接串已存在"))
	case errors.Is(err, operatedomain.ErrRtpengineNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "RTPEngine 记录不存在"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
	}
}

func parseDisabledQuery(query string) (bool, func(bool) *bool) {
	ptr := func(v bool) *bool { return &v }
	if query == "true" || query == "1" {
		return true, ptr
	}
	if query == "false" || query == "0" {
		return false, ptr
	}
	return false, func(bool) *bool { return nil }
}
