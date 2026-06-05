package operate

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yunshu/internal/contracts"
	operatedomain "yunshu/internal/domain/operate"
)

// RegisterCallRecordRoutes 注册商户呼叫记录查询接口。
func RegisterCallRecordRoutes(r gin.IRoutes, service *operatedomain.CallRecordManagementService) {
	r.POST("/merchant/call-record/page", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "呼叫记录查询未启用"))
			return
		}
		var req operatedomain.CallRecordPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		page, err := service.Page(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询呼叫记录失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})

	r.GET("/merchant/call-record/detail/:callId", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "呼叫记录查询未启用"))
			return
		}
		record, err := service.Detail(c.Request.Context(), c.Param("callId"))
		if err != nil {
			writeCallRecordError(c, err, "呼叫记录不存在")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(record))
	})

	r.GET("/merchant/call-record/sip-trace/:callId", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "呼叫记录查询未启用"))
			return
		}
		trace, err := service.SipTrace(c.Request.Context(), c.Param("callId"))
		if err != nil {
			writeCallRecordError(c, err, "获取呼叫信令失败")
			return
		}
		c.JSON(http.StatusOK, contracts.OK(trace))
	})

	r.GET("/merchant/call-record", func(c *gin.Context) {
		if service == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "呼叫记录查询未启用"))
			return
		}
		merchantID, _ := strconv.Atoi(c.Query("merchantId"))
		userID, _ := strconv.Atoi(c.Query("userId"))
		batchTaskID, _ := strconv.Atoi(c.Query("batchTaskId"))
		minDuration, _ := strconv.Atoi(c.Query("minDuration"))
		page, err := service.Page(c.Request.Context(), operatedomain.CallRecordPageRequest{
			PageNumber:  parsePositiveInt(c.Query("pageNumber"), 1),
			PageSize:    parsePositiveInt(c.Query("pageSize"), 20),
			CallID:      c.Query("callId"),
			MerchantID:  merchantID,
			UserID:      userID,
			BatchTaskID: batchTaskID,
			MinDuration: minDuration,
			GatewayID:   c.Query("gatewayId"),
			Profile:     c.Query("profile"),
			Extension:   c.Query("extension"),
			Phone:       c.Query("phone"),
			StartTime:   c.Query("startTime"),
			EndTime:     c.Query("endTime"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询呼叫记录失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(page))
	})
}

func writeCallRecordError(c *gin.Context, err error, fallback string) {
	if errors.Is(err, operatedomain.ErrCallRecordNotFound) {
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "呼叫记录不存在"))
		return
	}
	if errors.Is(err, operatedomain.ErrInvalidCallRecord) {
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "呼叫记录参数错误"))
		return
	}
	c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, fallback))
}
