package httpesl

// httpesl 包提供 cc-call 进程的 ESL HTTP 路由注册。
// 暴露  兼容的 HTTP API 端点，将请求委托给 esl 领域服务执行。
// 支持通话发起、媒体控制、会话管理、FreeSWITCH 节点生命周期管理和状态机转换。

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	"yunshu/internal/infra/fsesl"
	"yunshu/internal/infra/resource"
	fsregistry "yunshu/internal/infra/telephony"
	"yunshu/pkg/telephony"
)

// RegisterRoutes 注册 cc-call 进程中的 ESL HTTP 入口。
// 这里暴露  兼容路径，领域逻辑仍通过 esl.OriginateService 和 esl.CommandService 执行。
func RegisterRoutes(r gin.IRoutes, originate *esl.OriginateService, command *esl.CommandService, session *esl.SessionService, gatewaySync *esl.GatewayConfigService, registry fsregistry.Registry, pool *fsesl.ConnectionPool, gormDB *gorm.DB) {
	ownership := esl.NewEventOwnershipService()
	validator := esl.CommandValidator{}

	r.POST("/esl/call/start", func(c *gin.Context) {
		var req contracts.ApiCallReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		callID := c.Query("callId")
		if callID == "" {
			callID = c.GetHeader("X-Call-Id")
		}
		if err := originate.StartAPIOutbound(c.Request.Context(), esl.OriginateRequest{
			Version: c.GetHeader("X-Backend-Version"),
			CallID:  callID,
			Request: req,
		}); err != nil {
			if errors.Is(err, esl.ErrInvalidCommand) {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "ESL 起呼失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.POST("/esl/batch/call/start", func(c *gin.Context) {
		var req contracts.BatchCallReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		callID := c.Query("callId")
		if callID == "" {
			callID = c.GetHeader("X-Call-Id")
		}
		if err := originate.StartBatchOutbound(c.Request.Context(), esl.BatchOriginateRequest{
			Version: c.GetHeader("X-Backend-Version"),
			CallID:  callID,
			Request: req,
		}); err != nil {
			if errors.Is(err, esl.ErrInvalidCommand) {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
				return
			}
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "ESL 批量外呼起呼失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	checkMiddleware := checkAppCredentials(gormDB)

	for path, cmdName := range map[string]string{
		"/esl/control/playback":      "playback",
		"/esl/control/stop-playback": "stop-playback",
		"/esl/control/eavesdrop":     "eavesdrop",
		"/esl/control/audio":         "audio",
		"/esl/control/transfer":      "transfer",
		"/esl/control/hangup":        "hangup",
		"/esl/control/break":         "break",
		"/esl/control/bridge":        "bridge",
		"/esl/control/audio-stream":  "audio-stream",
		"/esl/control/dtmf":          "dtmf",
	} {
		commandName := cmdName
		r.POST(path, checkMiddleware, func(c *gin.Context) {
			var req contracts.CallControlReq
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
				return
			}

			// 对带有商户凭证的请求进行商户和通话归属越权校验
			if tenant, ok := contracts.TenantFromContext(c.Request.Context()); ok && tenant.MerchantID != "" {
				sess, err := session.Store.Get(c.Request.Context(), req.CallID)
				if err == nil {
					mID := getMerchantIDFromMetadata(sess.Metadata)
					if mID != "" && mID != tenant.MerchantID {
						c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "越权操作：目标通话会话不属于您所属的商户"))
						return
					}
				} else if errors.Is(err, esl.ErrSessionNotFound) && gormDB != nil {
					// 内存会话不存在时，尝试从 DB 话单中校验归属
					var cdr struct {
						MerchantID int
					}
					dbErr := gormDB.WithContext(c.Request.Context()).Table("call_cdr_record").
						Select("merchant_id").
						Where("call_id = ? AND del_flag = ?", req.CallID, false).
						First(&cdr).Error
					if dbErr == nil && strconv.Itoa(cdr.MerchantID) != tenant.MerchantID {
						c.JSON(http.StatusForbidden, contracts.Fail(contracts.CodeForbidden, "越权操作：目标通话历史不属于您所属的商户"))
						return
					}
				}
			}

			if req.CommandID == "" {
				req.CommandID = commandName + ":" + req.CallID + ":" + req.UUID + ":" + req.UUID1 + ":" + req.UUID2
			}
			if req.UUID == "" {
				req.UUID = req.UUID1
			}
			if req.FSAddr == "" {
				req.FSAddr = "default"
			}
			if req.LegRole == "" {
				req.LegRole = contracts.LegRoleUnknown
			}
			cmd := telephony.NewCommand(req.CommandID, commandName, req.CallID, req.UUID, req.FSAddr, req.LegRole, contracts.CallFlowAPIOutbound, controlPayload(req))
			if err := command.Execute(c.Request.Context(), cmd); err != nil {
				if errors.Is(err, esl.ErrInvalidCommand) {
					c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "控制命令缺少可追踪字段"))
					return
				}
				if errors.Is(err, esl.ErrDuplicateCommand) {
					c.JSON(http.StatusOK, contracts.Fail(contracts.CodeDuplicateCommand, "重复命令"))
					return
				}
				c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "通话控制命令执行失败"))
				return
			}
			c.JSON(http.StatusOK, contracts.OK(nil))
		})
	}

	r.POST("/esl/control/validate", func(c *gin.Context) {
		var cmd contracts.TelephonyCommand
		if err := c.ShouldBindJSON(&cmd); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if !validator.Validate(cmd) {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "控制命令缺少可追踪字段"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]bool{"valid": true}))
	})

	r.POST("/esl/call/asr-detect", func(c *gin.Context) {
		var req struct {
			CallID string `json:"callId" binding:"required"`
			Text   string `json:"text" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}

		if originate.Events == nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "事件总线未初始化"))
			return
		}

		err := originate.Events.Publish(c.Request.Context(), contracts.NewEventEnvelope(
			"asr-detect:"+req.CallID+":"+strconv.FormatInt(time.Now().UnixNano(), 10),
			"asr_speech_detected",
			req.CallID,
			"call",
			req.CallID,
			contracts.ServiceCall,
			map[string]any{
				"callId": req.CallID,
				"text":   req.Text,
			},
		))

		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "发布 ASR 事件失败: "+err.Error()))
			return
		}

		c.JSON(http.StatusOK, contracts.OK(map[string]bool{"dispatched": true}))
	})

	r.POST("/esl/call/dtmf-detect", func(c *gin.Context) {
		var req struct {
			CallID string `json:"callId" binding:"required"`
			Digit  string `json:"digit" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}

		if originate.Events == nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "事件总线未初始化"))
			return
		}

		err := originate.Events.Publish(c.Request.Context(), contracts.NewEventEnvelope(
			"dtmf-detect:"+req.CallID+":"+strconv.FormatInt(time.Now().UnixNano(), 10),
			"dtmf_detected",
			req.CallID,
			"call",
			req.CallID,
			contracts.ServiceCall,
			map[string]any{
				"callId": req.CallID,
				"digit":  req.Digit,
			},
		))

		if err != nil {
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "发布 DTMF 事件失败: "+err.Error()))
			return
		}

		c.JSON(http.StatusOK, contracts.OK(map[string]bool{"dispatched": true}))
	})

	r.POST("/esl/events/apply", func(c *gin.Context) {
		var event contracts.TelephonyEvent
		if err := c.ShouldBindJSON(&event); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		updated, err := session.ApplyEvent(c.Request.Context(), event)
		if err != nil {
			if errors.Is(err, esl.ErrDuplicateEvent) {
				c.JSON(http.StatusOK, contracts.Fail(contracts.CodeDuplicateCommand, "重复事件"))
				return
			}
			if errors.Is(err, esl.ErrSessionNotFound) {
				c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "通话会话不存在"))
				return
			}
			c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, "FS 事件状态迁移失败"))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(updated))
	})

	r.POST("/esl/gateway", func(c *gin.Context) {
		gatewayID, err := strconv.Atoi(c.Query("gatewayId"))
		if err != nil || gatewayID <= 0 {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "网关 ID 错误"))
			return
		}
		result, err := gatewaySync.Sync(c.Request.Context(), esl.GatewaySyncRequest{Action: esl.GatewaySyncCreate, GatewayID: gatewayID})
		writeGatewaySyncResult(c, result, err)
	})

	r.PUT("/esl/gateway", func(c *gin.Context) {
		gatewayID, err := strconv.Atoi(c.Query("gatewayId"))
		if err != nil || gatewayID <= 0 {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "网关 ID 错误"))
			return
		}
		result, err := gatewaySync.Sync(c.Request.Context(), esl.GatewaySyncRequest{Action: esl.GatewaySyncUpdate, GatewayID: gatewayID})
		writeGatewaySyncResult(c, result, err)
	})

	r.DELETE("/esl/gateway", func(c *gin.Context) {
		gatewayName := c.Query("gatewayName")
		if gatewayName == "" {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "网关名称错误"))
			return
		}
		result, err := gatewaySync.Sync(c.Request.Context(), esl.GatewaySyncRequest{Action: esl.GatewaySyncDelete, GatewayName: gatewayName})
		writeGatewaySyncResult(c, result, err)
	})

	r.POST("/esl/freeswitch/ownership/claim", func(c *gin.Context) {
		var req struct {
			FSAddr string `json:"fsAddr"`
			Owner  string `json:"owner"`
			TTLMS  int64  `json:"ttlMs"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		if req.TTLMS <= 0 {
			req.TTLMS = 30_000
		}
		claimed := ownership.Claim(c.Request.Context(), req.FSAddr, req.Owner, time.Duration(req.TTLMS)*time.Millisecond)
		c.JSON(http.StatusOK, contracts.OK(map[string]bool{"claimed": claimed}))
	})

	r.GET("/esl/freeswitch/list", func(c *gin.Context) {
		slog.Info("查询 FreeSWITCH 节点列表")
		if registry == nil {
			c.JSON(http.StatusOK, contracts.OK([]fsregistry.Node{}))
			return
		}
		nodes, err := registry.List(c.Request.Context())
		if err != nil {
			slog.Error("查询 FreeSWITCH 节点列表失败", "error", err.Error())
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "查询 FreeSWITCH 节点失败"))
			return
		}
		slog.Info("查询 FreeSWITCH 节点列表完成", "nodeCount", len(nodes))
		c.JSON(http.StatusOK, contracts.OK(nodes))
	})

	r.GET("/esl/freeswitch/status", func(c *gin.Context) {
		slog.Info("查询 FreeSWITCH 连接池状态")
		if pool == nil {
			c.JSON(http.StatusOK, contracts.OK([]fsesl.NodeRuntimeStatus{}))
			return
		}
		statuses := pool.Status()
		slog.Info("查询 FreeSWITCH 连接池状态完成", "nodeCount", len(statuses))
		c.JSON(http.StatusOK, contracts.OK(statuses))
	})

	r.POST("/esl/freeswitch/reload", func(c *gin.Context) {
		slog.Info("开始从数据库刷新 FreeSWITCH 节点")
		if registry == nil || pool == nil {
			c.JSON(http.StatusOK, contracts.OK(map[string]int{"loaded": 0}))
			return
		}
		nodes, err := registry.ListEnabled(c.Request.Context())
		if err != nil {
			slog.Error("从数据库刷新 FreeSWITCH 节点失败", "error", err.Error())
			c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "刷新 FreeSWITCH 节点失败"))
			return
		}
		for _, node := range nodes {
			pool.UpsertNode(fsesl.NodeConfig{ID: node.ID, Addr: node.FSAddr, Password: node.Password, SetID: node.SetID, Weight: node.Weight, Enabled: node.Enable})
		}
		slog.Info("从数据库刷新 FreeSWITCH 节点完成", "loaded", len(nodes))
		c.JSON(http.StatusOK, contracts.OK(map[string]int{"loaded": len(nodes)}))
	})

	r.POST("/esl/freeswitch/load/:id", func(c *gin.Context) {
		slog.Info("开始动态加载 FreeSWITCH 节点", "id", c.Param("id"))
		if registry == nil || pool == nil {
			c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "FreeSWITCH 节点管理未启用"))
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "节点 ID 错误"))
			return
		}
		node, err := registry.GetByID(c.Request.Context(), id)
		if err != nil {
			slog.Warn("动态加载 FreeSWITCH 节点失败，节点不存在", "id", id, "error", err.Error())
			c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "FreeSWITCH 节点不存在"))
			return
		}
		pool.UpsertNode(fsesl.NodeConfig{ID: node.ID, Addr: node.FSAddr, Password: node.Password, SetID: node.SetID, Weight: node.Weight, Enabled: node.Enable})
		slog.Info("动态加载 FreeSWITCH 节点完成", "id", id, "fsAddr", node.FSAddr, "enabled", node.Enable)
		c.JSON(http.StatusOK, contracts.OK(node))
	})

	r.POST("/esl/freeswitch/remove/:id", func(c *gin.Context) {
		slog.Info("开始动态移除 FreeSWITCH 节点", "id", c.Param("id"))
		if registry == nil || pool == nil {
			c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "FreeSWITCH 节点管理未启用"))
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "节点 ID 错误"))
			return
		}
		node, err := registry.GetByID(c.Request.Context(), id)
		if err != nil {
			slog.Warn("动态移除 FreeSWITCH 节点失败，节点不存在", "id", id, "error", err.Error())
			c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "FreeSWITCH 节点不存在"))
			return
		}
		pool.RemoveNode(node.FSAddr)
		slog.Info("动态移除 FreeSWITCH 节点完成", "id", id, "fsAddr", node.FSAddr)
		c.JSON(http.StatusOK, contracts.OK(nil))
	})

	r.POST("/esl/lifecycle/apply", func(c *gin.Context) {
		var req struct {
			Initial esl.CallState `json:"initial"`
			Event   esl.CallEvent `json:"event"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
			return
		}
		machine := esl.NewCallLifecycle(req.Initial)
		next, err := machine.Apply(req.Event)
		if err != nil {
			c.JSON(http.StatusConflict, contracts.Fail(contracts.CodeConflict, err.Error()))
			return
		}
		c.JSON(http.StatusOK, contracts.OK(map[string]esl.CallState{"state": next}))
	})
}

// controlPayload 将 CallControlReq 中的字段映射为 FreeSWITCH 命令的 payload。
// 统一字段命名（如 UUID1/UUID2、Digit/Digits）并过滤空值后传递给 ESL 命令执行器。
func controlPayload(req contracts.CallControlReq) map[string]any {
	payload := map[string]any{}
	for key, value := range req.Payload {
		payload[key] = value
	}
	if req.UUID1 != "" {
		payload["uuid1"] = req.UUID1
	}
	if req.UUID2 != "" {
		payload["uuid2"] = req.UUID2
	}
	if req.Digit != "" {
		payload["digits"] = req.Digit
	}
	if req.Digits != "" {
		payload["digits"] = req.Digits
	}
	if req.Destination != "" {
		payload["destination"] = req.Destination
	}
	if req.ReasonCode != "" {
		payload["reasonCode"] = req.ReasonCode
	}
	if req.CustomCause != "" {
		payload["customCause"] = req.CustomCause
	}
	return payload
}

func writeGatewaySyncResult(c *gin.Context, result esl.GatewaySyncResult, err error) {
	if err == nil {
		c.JSON(http.StatusOK, contracts.OK(result))
		return
	}
	switch {
	case errors.Is(err, esl.ErrInvalidCommand):
		c.JSON(http.StatusBadRequest, contracts.Fail(contracts.CodeBadRequest, "请求参数错误"))
	case errors.Is(err, esl.ErrGatewayConfigNotFound):
		c.JSON(http.StatusNotFound, contracts.Fail(contracts.CodeNotFound, "网关不存在"))
	case errors.Is(err, esl.ErrGatewaySyncTargetMissing):
		c.JSON(http.StatusOK, contracts.Fail(contracts.CodeResourceUnavailable, "没有可同步的 FreeSWITCH 节点"))
	default:
		c.JSON(http.StatusInternalServerError, contracts.Fail(contracts.CodeInternal, "网关配置同步失败"))
	}
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

func getMerchantIDFromMetadata(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	val, ok := metadata["merchantId"]
	if !ok {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return fmt.Sprintf("%v", val)
}
