package httpcti

// httpcti 包提供 cc-call 进程的 CTI HTTP 路由注册。
// 暴露  兼容和 Go 调试两套 HTTP API 端点，支持 API 外呼、号码选择和任务状态机转换。
// 灰度期间保留双路径以验证 Go 实现与  行为一致。

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/cti"
	"yunshu/internal/domain/esl"
	"yunshu/internal/infra/extensionstatus"
	"yunshu/internal/infra/fsesl"
	"yunshu/internal/infra/resource"
	"yunshu/internal/transport/http/middleware"
)

// WebSocketHub 是 CTI WebSocket 推送节点的最小 transport 端口。
type WebSocketHub interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// RegisterRoutes 注册 cc-call 进程中的 CTI HTTP 入口。
// 兼容  现有路径，同时保留早期 Go 调试路径，方便灰度期间双写验证。
func RegisterRoutes(
	r gin.IRoutes,
	apiCall *cti.APICallService,
	runtimeSelector *cti.RuntimeSelector,
	batchScheduler *cti.BatchSchedulerService,
	candidateSource cti.CandidateSource,
	candidateMarker cti.CandidateMarker,
	wsHub WebSocketHub,
	gormDB *gorm.DB,
	redisClient *goredis.Client,
	pool *fsesl.ConnectionPool,
	callControl *cti.CallControlService,
) {
	// Selector 根据呼叫上下文和选号规则从候选号码中选择最优号码。
	// 选号逻辑考虑号码可用性、并发限制、风险等级等因素。
	selector := cti.Selector{}
	statusWriter := extensionstatus.NewRedisReader(redisClient)
	r.POST("/cti/callTask/call", checkAppCredentials(gormDB), middleware.RateLimitMiddleware(10, 2.0), func(c *gin.Context) {
		var req contracts.ApiCallReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if gormDB != nil {
			if tenant, ok := contracts.TenantFromContext(c.Request.Context()); ok && tenant.MerchantID != "" {
				// 二次校验请求体 UserID 的商户归属，防止水平越权
				var user resource.MerchantUserModel
				err := gormDB.WithContext(c.Request.Context()).
					Where("id = ? AND del_flag = ?", req.UserID, false).
					First(&user).Error
				if err != nil {
					c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "越权操作：请求的用户 ID 不存在"))
					return
				}
				if strconv.Itoa(user.MerchantID) != tenant.MerchantID {
					c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "越权操作：请求的用户 ID 不属于您所属的商户"))
					return
				}
			}
		}
		callID := c.Query("callId")
		if callID == "" {
			callID = c.GetHeader("X-Call-Id")
		}
		if err := apiCall.Run(c.Request.Context(), c.GetHeader("X-Backend-Version"), callID, req); err != nil {
			if errors.Is(err, cti.ErrInvalidApiCall) {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "API 外呼处理失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.POST("/cti/select/number/rule", func(c *gin.Context) {
		var req contracts.SelectRuleReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		callID := c.Query("callId")
		candidates := fallbackCandidates()
		if candidateSource != nil && req.UserID > 0 {
			loaded, err := candidateSource.CandidatesForUser(c.Request.Context(), req.UserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "读取选号候选失败"))
				return
			}
			if len(loaded) > 0 {
				candidates = loaded
			}
		}
		if candidateMarker != nil {
			marked, err := candidateMarker.MarkCandidates(c.Request.Context(), cti.SelectionRequest{
				MerchantID: strconv.Itoa(req.MerchantID),
				RiskID:     req.RiskID,
				Callee:     req.Callee,
				UserID:     req.UserID,
			}, candidates)
			if err != nil {
				c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "标记选号候选失败"))
				return
			}
			candidates = marked
		}
		selectionReq := cti.SelectionRequest{
			CallID:     callID,
			MerchantID: strconv.Itoa(req.MerchantID),
			RiskID:     req.RiskID,
			Callee:     req.Callee,
			Candidates: candidates,
		}
		result := selector.Select(c.Request.Context(), selectionReq)
		if runtimeSelector != nil {
			claimed, allocation, err := runtimeSelector.SelectAndClaim(c.Request.Context(), selectionReq)
			result = claimed
			if err == nil && allocation != nil && result.Caller != nil {
				result.Caller.Phone = allocation.Caller
				result.Caller.GatewayID = allocation.GatewayID
			}
		}
		if !result.Success {
			c.JSON(http.StatusOK, contracts.Fail(contracts.CodeSelectionFailed, "选号失败"))
			return
		}
		gatewayID, _ := strconv.Atoi(result.Caller.GatewayID)
		c.JSON(http.StatusOK, contracts.OK(contracts.SelectPhoneResp{
			Phone:              result.Caller.Phone,
			GatewayID:          gatewayID,
			SkillGroupID:       result.Caller.SkillGroupID,
			ChannelID:          result.Caller.ChannelID,
			GatewayName:        result.Caller.GatewayName,
			GatewayRegion:      result.Caller.GatewayRegion,
			Model:              result.Caller.Model,
			CallerPrefix:       result.Caller.CallerPrefix,
			CalleePrefix:       result.Caller.CalleePrefix,
			CallerRewriteRule:  result.Caller.CallerRewriteRule,
			CalleeRewriteRule:  result.Caller.CalleeRewriteRule,
			SupplementRing:     result.Caller.SupplementRing,
			SupplementRingFile: result.Caller.SupplementRingFile,
			Province:           result.Caller.Province,
			City:               result.Caller.City,
			PoolID:             result.Caller.PoolID,
			CodecPrefs:         result.Caller.CodecPrefs,
			BroadcastTime:      result.Caller.BroadcastTime,
			BroadcastTimeFlag:  result.Caller.BroadcastTimeFlag,
		}))
	})

	r.GET("/cti/select/number/rule/release", func(c *gin.Context) {
		callID := c.Query("callId")
		if callID == "" {
			callID = c.Query("callID")
		}
		merchantID := c.Query("merchantId")
		if merchantID == "" {
			merchantID = c.Query("merchantID")
		}
		caller := c.Query("caller")
		gatewayID := c.Query("gatewayId")
		if gatewayID == "" {
			gatewayID = c.Query("gatewayID")
		}
		claimKey := c.Query("claimKey")

		if callID == "" || caller == "" || gatewayID == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "缺失必填释放参数 (callId, caller, gatewayId)"))
			return
		}

		if runtimeSelector == nil || runtimeSelector.Allocator == nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "运行时选号分配器未配置"))
			return
		}

		alloc := cti.RuntimeAllocation{
			CallID:     callID,
			MerchantID: merchantID,
			Caller:     caller,
			GatewayID:  gatewayID,
			ClaimKey:   claimKey,
		}

		if err := runtimeSelector.Allocator.Release(c.Request.Context(), alloc); err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "并发槽位原子释放失败: "+err.Error()))
			return
		}

		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.POST("/cti/select-number", func(c *gin.Context) {
		var req cti.SelectionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if len(req.Candidates) == 0 && candidateSource != nil && req.UserID > 0 {
			loaded, err := candidateSource.CandidatesForUser(c.Request.Context(), req.UserID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "读取选号候选失败"))
				return
			}
			if len(loaded) > 0 {
				req.Candidates = loaded
			}
		}
		if candidateMarker != nil {
			marked, err := candidateMarker.MarkCandidates(c.Request.Context(), req, req.Candidates)
			if err != nil {
				c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "标记选号候选失败"))
				return
			}
			req.Candidates = marked
		}
		result := selector.Select(c.Request.Context(), req)
		if !result.Success {
			c.JSON(http.StatusOK, contracts.Fail(contracts.CodeSelectionFailed, "选号失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(result))
	})

	r.POST("/cti/batch-call-task/state", func(c *gin.Context) {
		var req struct {
			Initial cti.TaskState `json:"initial"`
			Event   cti.TaskEvent `json:"event"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		machine := cti.NewTaskStateMachine(req.Initial)
		next, err := machine.Apply(req.Event)
		if err != nil {
			c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]cti.TaskState{"state": next}))
	})

	r.POST("/cti/batch-call-task/dispatch", func(c *gin.Context) {
		if batchScheduler == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "批量外呼调度器未启用"))
			return
		}
		var req struct {
			TaskID int `json:"taskId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		dispatched, callID, err := batchScheduler.DispatchNext(c.Request.Context(), c.GetHeader("X-Backend-Version"), req.TaskID)
		if err != nil {
			if errors.Is(err, cti.ErrInvalidBatchTask) {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "批量任务参数错误"))
				return
			}
			if errors.Is(err, cti.ErrNoBatchTel) {
				c.JSON(http.StatusOK, contracts.Fail(contracts.CodeNotFound, "没有待拨号码"))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "批量外呼调度失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"callId": callID, "request": dispatched}))
	})

	r.GET("/cti/extension/active-state/:extension", checkAppCredentials(gormDB), func(c *gin.Context) {
		ext := strings.TrimSpace(c.Param("extension"))
		if ext == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "分机号不能为空"))
			return
		}

		// 检查商户与分机的绑定归属关系，防止越权查询
		if tenant, ok := contracts.TenantFromContext(c.Request.Context()); ok && tenant.MerchantID != "" {
			if gormDB != nil {
				var extModel resource.ExtensionModel
				err := gormDB.WithContext(c.Request.Context()).
					Where("extension_number = ? AND del_flag = ?", ext, false).
					First(&extModel).Error
				if err != nil {
					c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "越权操作：请求的分机号不存在"))
					return
				}
				if strconv.Itoa(extModel.MerchantID) != tenant.MerchantID {
					c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "越权操作：请求的分机号不属于您所属的商户"))
					return
				}
			}
		}

		if pool == nil || redisClient == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "CTI 状态回查服务未启用"))
			return
		}
		ctx := c.Request.Context()
		active := false
		var matchedNode string
		nodes := pool.Status()
		for _, node := range nodes {
			if !node.Connected {
				continue
			}
			channelsText, err := pool.QueryChannels(ctx, node.FSAddr)
			if err != nil {
				slog.Warn("回查 FS 活跃通道失败", "fsAddr", node.FSAddr, "error", err.Error())
				continue
			}
			if strings.Contains(channelsText, ext) {
				active = true
				matchedNode = node.FSAddr
				break
			}
		}

		statusRaw, err := redisClient.HGet(ctx, contracts.KeyExtensionStatus, ext).Result()
		hasStuckState := false
		var oldStatus string
		if err == nil {
			statusInt, _ := strconv.Atoi(statusRaw)
			// PreRing=2, Ringing=3, Talking=4
			if statusInt >= 2 && statusInt <= 4 {
				oldStatus = statusRaw
				if !active {
					hasStuckState = true
					// 强行清理重置为 Idle (1)
					_ = statusWriter.SetExtensionStatus(ctx, ext, esl.ExtensionStatusIdle)
				}
			}
		}

		c.JSON(http.StatusOK, contracts.OK(map[string]any{
			"extension":     ext,
			"fsActive":      active,
			"fsAddr":        matchedNode,
			"redisStatus":   statusRaw,
			"hasStuckState": hasStuckState,
			"oldStatus":     oldStatus,
			"action":        "sync",
		}))
	})

	r.GET("/cti/ws", func(c *gin.Context) {
		if wsHub == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "WebSocket 推送节点未启用"))
			return
		}
		wsHub.ServeHTTP(c.Writer, c.Request)
	})

	// --- Kamailio Webhook 端点：使用共享密钥鉴权，防止未授权请求枚举 SIP 域名和分机 ---
	kamailioGroup := r.(*gin.RouterGroup).Group("", middleware.KamailioWebhookAuth())

	kamailioGroup.POST("/cti/kamailio/subscriber/register-status", func(c *gin.Context) {
		var req struct {
			Extension string `json:"extension"`
			Event     string `json:"event"` // register, unregister, expire
			Contact   string `json:"contact,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		ext := strings.TrimSpace(req.Extension)
		event := strings.TrimSpace(req.Event)
		if ext == "" || event == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "extension 和 event 不能为空"))
			return
		}

		if redisClient == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "Redis 未启用，状态同步不可用"))
			return
		}

		esStatus := esl.ExtensionStatusOffline // unregister, expire 默认离线
		if event == "register" {
			esStatus = esl.ExtensionStatusIdle
		}

		ctx := c.Request.Context()
		err := statusWriter.SetExtensionStatus(ctx, ext, esStatus)
		if err != nil {
			slog.Error("收到 Kamailio SIP 注册状态回调更新 Redis 失败", "extension", ext, "event", event, "error", err.Error())
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "更新 Redis 状态失败"))
			return
		}

		slog.Info("收到 Kamailio SIP 注册状态回调 Webhook", "extension", ext, "event", event, "contact", req.Contact, "status", int(esStatus))
		c.JSON(http.StatusOK, contracts.OK(map[string]any{"extension": ext, "event": event, "status": int(esStatus)}))
	})

	kamailioGroup.POST("/cti/kamailio/auth", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			IP       string `json:"ip"`
			Domain   string `json:"domain"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, map[string]any{"code": 400, "message": "请求参数错误"})
			return
		}
		slog.Info("收到 Kamailio 业务鉴权回调", "username", req.Username, "ip", req.IP, "domain", req.Domain)

		if gormDB == nil {
			c.JSON(http.StatusOK, map[string]any{"code": 200, "message": "成功"})
			return
		}

		ctx := c.Request.Context()
		var merchantID int
		if req.Domain != "" {
			var m struct {
				ID     int
				Enable bool
			}
			err := gormDB.WithContext(ctx).Table("cc_mch_info").
				Select("id, enable").
				Where("sip_domain = ? AND del_flag = ?", req.Domain, false).
				First(&m).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.JSON(http.StatusOK, map[string]any{"code": 403, "message": "SIP域名对应的商户不存在"})
					return
				}
				c.JSON(http.StatusOK, map[string]any{"code": 500, "message": "系统查询商户失败"})
				return
			}
			if !m.Enable {
				c.JSON(http.StatusOK, map[string]any{"code": 403, "message": "商户已被禁用"})
				return
			}
			merchantID = m.ID
		} else {
			merchantID = 1
		}

		var ext struct {
			ID     int
			Enable bool
		}
		err := gormDB.WithContext(ctx).Table("cc_res_extension").
			Select("id, enable").
			Where("extension_number = ? AND merchant_id = ? AND del_flag = ?", req.Username, merchantID, false).
			First(&ext).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusOK, map[string]any{"code": 403, "message": "商户下无此分机配置"})
				return
			}
			c.JSON(http.StatusOK, map[string]any{"code": 500, "message": "系统查询分机失败"})
			return
		}
		if !ext.Enable {
			c.JSON(http.StatusOK, map[string]any{"code": 403, "message": "分机已被禁用"})
			return
		}

		c.JSON(http.StatusOK, map[string]any{"code": 200, "message": "成功"})
	})

	kamailioGroup.POST("/cti/kamailio/auth/register", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			IP       string `json:"ip"`
			Port     string `json:"port"`
			Domain   string `json:"domain"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if req.Username == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "username 不能为空"))
			return
		}
		if redisClient == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "Redis 未启用，状态同步不可用"))
			return
		}

		ctx := c.Request.Context()
		reqJSON, _ := json.Marshal(req)
		err := redisClient.HSet(ctx, "kamailio:auth:"+req.Domain, req.Username, string(reqJSON)).Err()
		if err != nil {
			slog.Error("保存 Kamailio 注册详情到 Redis 失败", "username", req.Username, "domain", req.Domain, "error", err.Error())
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "更新 Redis 状态失败"))
			return
		}

		err = statusWriter.SetExtensionStatus(ctx, req.Username, esl.ExtensionStatusIdle)
		if err != nil {
			slog.Error("更新分机状态为 IDLE 失败", "username", req.Username, "error", err.Error())
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "更新分机状态失败"))
			return
		}

		slog.Info("收到 Kamailio 注册成功回调，状态同步成功", "username", req.Username, "ip", req.IP, "domain", req.Domain)
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	kamailioGroup.POST("/cti/kamailio/auth/unregister", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			IP       string `json:"ip"`
			Port     string `json:"port"`
			Domain   string `json:"domain"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if req.Username == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "username 不能为空"))
			return
		}
		if redisClient == nil {
			c.JSON(http.StatusServiceUnavailable, contracts.Fail(contracts.CodeInternal, "Redis 未启用，状态同步不可用"))
			return
		}

		ctx := c.Request.Context()
		err := redisClient.HDel(ctx, "kamailio:auth:"+req.Domain, req.Username).Err()
		if err != nil {
			slog.Error("从 Redis 删除 Kamailio 注册详情失败", "username", req.Username, "domain", req.Domain, "error", err.Error())
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "更新 Redis 状态失败"))
			return
		}

		err = statusWriter.SetExtensionStatus(ctx, req.Username, esl.ExtensionStatusOffline)
		if err != nil {
			slog.Error("更新分机状态为 OFFLINE 失败", "username", req.Username, "error", err.Error())
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "更新分机状态失败"))
			return
		}

		slog.Info("收到 Kamailio 注销成功回调，状态同步成功", "username", req.Username, "ip", req.IP, "domain", req.Domain)
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.POST("/cti/call/eavesdrop", checkAppCredentials(gormDB), func(c *gin.Context) {
		var req contracts.CallEavesdropReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}

		merchantID := 0
		if tenant, ok := contracts.TenantFromContext(c.Request.Context()); ok && tenant.MerchantID != "" {
			merchantID, _ = strconv.Atoi(tenant.MerchantID)
		} else {
			c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "商户未登录或对接密钥无效"))
			return
		}

		// 安全加固：防水平越权校验
		if gormDB != nil {
			var user resource.MerchantUserModel
			err := gormDB.WithContext(c.Request.Context()).
				Where("id = ? AND del_flag = ?", req.UserID, false).
				First(&user).Error
			if err != nil {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "越权操作：请求的主管用户 ID 不存在"))
				return
			}
			if user.MerchantID != merchantID {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "越权操作：请求的主管用户 ID 不属于您所属的商户"))
				return
			}
		}

		if err := callControl.Eavesdrop(c.Request.Context(), merchantID, req); err != nil {
			if errors.Is(err, cti.ErrSessionNotFound) {
				c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, err.Error()))
				return
			}
			if errors.Is(err, cti.ErrPermissionDenied) {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, err.Error()))
				return
			}
			if errors.Is(err, cti.ErrExtensionNotFound) {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, err.Error()))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "监听操作失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.POST("/cti/call/hangup", checkAppCredentials(gormDB), func(c *gin.Context) {
		var req contracts.CallHangupReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}

		merchantID := 0
		if tenant, ok := contracts.TenantFromContext(c.Request.Context()); ok && tenant.MerchantID != "" {
			merchantID, _ = strconv.Atoi(tenant.MerchantID)
		} else {
			c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "商户未登录或对接密钥无效"))
			return
		}

		if err := callControl.Hangup(c.Request.Context(), merchantID, req); err != nil {
			if errors.Is(err, cti.ErrSessionNotFound) {
				c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, err.Error()))
				return
			}
			if errors.Is(err, cti.ErrPermissionDenied) {
				c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, err.Error()))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "强拆操作失败: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

}

func fallbackCandidates() []cti.NumberCandidate {
	return []cti.NumberCandidate{{
		Phone:       "10086",
		GatewayID:   "1",
		Available:   true,
		RiskAllowed: true,
		Concurrency: 1,
	}}
}

// checkAppCredentials 拦截并校验请求头中的 X-App-Key 和 X-App-Secret
func checkAppCredentials(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.Next()
			return
		}
		appKey := c.GetHeader("X-App-Key")
		appSecret := c.GetHeader("X-App-Secret")
		if appKey == "" || appSecret == "" {
			c.Next()
			return
		}
		var merchant resource.MerchantModel
		err := db.WithContext(c.Request.Context()).
			Where("app_key = ? AND app_secret = ? AND enable = ? AND del_flag = ?", appKey, appSecret, true, false).
			First(&merchant).Error
		if err != nil {
			c.JSON(http.StatusUnauthorized, contracts.Fail(contracts.CodeUnauthorized, "商户对接密钥对(X-App-Key/X-App-Secret)无效或商户已被禁用"))
			c.Abort()
			return
		}
		tenant := contracts.TenantContext{
			MerchantID:  strconv.Itoa(merchant.ID),
			RoleID:      "merchant_admin",
			DataScope:   "merchant",
			Permissions: []string{"*"},
		}
		c.Request = c.Request.WithContext(contracts.WithTenant(c.Request.Context(), tenant))
		c.Next()
	}
}
