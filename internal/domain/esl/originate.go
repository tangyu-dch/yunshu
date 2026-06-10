package esl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"yunshu/internal/contracts"
	"yunshu/internal/infra/events"
	"yunshu/internal/infra/logging"
	"yunshu/pkg/telephony"
)

var ErrExtensionNotFound = errors.New("extension not found")

var (
	ErrMerchantUserNotFound = errors.New("merchant user not found")
	ErrMerchantNotFound     = errors.New("merchant not found")
	ErrOutboundRejected     = errors.New("outbound request rejected")
)

// FSNodeSelector 为起呼选择可用 FreeSWITCH 节点。
//
// 领域层只依赖这个小接口，生产实现可以从 DB/Redis 节点注册表读取健康节点。
type FSNodeSelector interface {
	SelectAPIOutbound(ctx context.Context, req OriginateRequest) (string, error)
}

// Extension 保存 API 外呼所需的坐席分机信息。
type Extension struct {
	ID              int
	UserID          int
	MerchantID      int
	ExtensionNumber string
	SipDomain       string
	SkillGroupID    int
}

// ExtensionResolver 按用户读取已绑定分机。
//
//	ApiCallService 会通过 ExtensionService.getByUserId 查询分机；Go 侧使用小接口
//
// 保持领域层不依赖 GORM。
type ExtensionResolver interface {
	GetByUserID(ctx context.Context, userID int) (Extension, error)
}

type ExtensionStatus int

const (
	ExtensionStatusOffline ExtensionStatus = -1
	ExtensionStatusBusy    ExtensionStatus = 0
	ExtensionStatusIdle    ExtensionStatus = 1
	ExtensionStatusPreRing ExtensionStatus = 2
	ExtensionStatusRinging ExtensionStatus = 3
	ExtensionStatusTalking ExtensionStatus = 4
)

// ExtensionStatusReader 读取 Redis 中的分机在线/忙闲状态。
type ExtensionStatusReader interface {
	GetExtensionStatus(ctx context.Context, extension string) (ExtensionStatus, bool, error)
}

// ExtensionStatusWriter 写入 Redis 中的分机在线/忙闲状态。
type ExtensionStatusWriter interface {
	SetExtensionStatus(ctx context.Context, extension string, status ExtensionStatus) error
}

// OutboundGuard 对齐  OutboundRequestGuard 的 API 外呼兜底校验。
//
// 正常路径 CTI 会先做校验；ESL 内部入口仍需要兜底，避免绕过用户、商户、余额和坐席约束。
type OutboundGuard interface {
	ValidateAPICall(ctx context.Context, req contracts.ApiCallReq, extension Extension) error
}

// OriginateRequest 表示 ESL 收到的一次起呼请求。
type OriginateRequest struct {
	Version string
	CallID  string
	Request contracts.ApiCallReq
}

// BatchOriginateRequest 表示 CTI 调度器下发的一次批量号码起呼。
type BatchOriginateRequest struct {
	Version string
	CallID  string
	Request contracts.BatchCallReq
}

// APICustomerOriginateRequest 表示 API 外呼在坐席腿接通后发起的客户腿起呼请求。
type APICustomerOriginateRequest struct {
	Version   string
	CallID    string
	Selection contracts.SelectPhoneResp
}

// APIBridgeRequest 表示 API 外呼在两腿准备完成后发起的桥接请求。
type APIBridgeRequest struct {
	CallID       string
	AgentUUID    string
	CustomerUUID string
	FSAddr       string
}

// ConcurrencyLimiter 定义系统级通话并发限制与功能模块限制校验能力。
type ConcurrencyLimiter interface {
	CheckConcurrencyLimit(ctx context.Context, activeCount int) error
	CheckFeatureLimit(ctx context.Context, feature string) error
}

// OriginateService 把 API/批量外呼请求转换成可追踪的 ESL originate 命令。
type OriginateService struct {
	CommandService *CommandService
	SessionService *SessionService
	NodeSelector   FSNodeSelector
	Extensions     ExtensionResolver
	Guard          OutboundGuard
	Events         events.Bus
	Limiter        ConcurrencyLimiter // 系统级别限制器
	Logger         *slog.Logger
}

func (s *OriginateService) checkLicenseConcurrency(ctx context.Context, currentCallID string) error {
	if s.Limiter == nil {
		return nil
	}
	// 1. 校验“外呼/CTI”模块是否已被授权启用
	if err := s.Limiter.CheckFeatureLimit(ctx, "outbound"); err != nil {
		return err
	}

	if s.SessionService == nil || s.SessionService.Store == nil {
		return nil
	}
	activeCount, err := s.SessionService.Store.CountActive(ctx)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Error("【云枢授权】获取活动会话数失败", "error", err.Error())
		}
		return err
	}
	if currentCallID != "" {
		if sess, errGet := s.SessionService.Store.Get(ctx, currentCallID); errGet == nil && sess.CompletedAt.IsZero() && sess.State != CallComplete {
			activeCount--
		}
	}
	return s.Limiter.CheckConcurrencyLimit(ctx, activeCount)
}

// StartAPIOutbound 启动一次 API 外呼。
// API 外呼遵循  AGENT_FIRST 语义：先呼坐席分机，客户腿由后续流程继续编排。
func (s *OriginateService) StartAPIOutbound(ctx context.Context, req OriginateRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("开始 ESL API 外呼起呼编排", "callId", req.CallID, "userId", req.Request.UserID, "version", req.Version)
	if err := s.checkLicenseConcurrency(ctx, req.CallID); err != nil {
		logger.Warn("ESL API 外呼因系统授权并发超限被拦截", "callId", req.CallID, "error", err.Error())
		return err
	}
	fsAddr := "default"
	if s.NodeSelector != nil {
		selected, err := s.NodeSelector.SelectAPIOutbound(ctx, req)
		if err != nil {
			logger.Error("ESL API 外呼选择 FreeSWITCH 节点失败", "callId", req.CallID, "userId", req.Request.UserID, "error", err.Error())
			return err
		}
		fsAddr = selected
	}
	extensionNumber := ""
	sipDomain := ""
	if s.Extensions != nil {
		extension, err := s.Extensions.GetByUserID(ctx, req.Request.UserID)
		if err != nil {
			logger.Error("ESL API 外呼读取坐席分机失败", "callId", req.CallID, "userId", req.Request.UserID, "error", err.Error())
			return err
		}
		extensionNumber = extension.ExtensionNumber
		sipDomain = extension.SipDomain
		logger.Info("ESL API 外呼已读取坐席分机", "callId", req.CallID, "userId", req.Request.UserID, "extensionId", extension.ID, "extension", extension.ExtensionNumber, "merchantId", extension.MerchantID)
		if s.Guard != nil {
			if err := s.Guard.ValidateAPICall(ctx, req.Request, extension); err != nil {
				logger.Warn("ESL API 外呼兜底校验未通过", "callId", req.CallID, "userId", req.Request.UserID, "extensionId", extension.ID, "merchantId", extension.MerchantID, "error", err.Error())
				return err
			}
			logger.Info("ESL API 外呼兜底校验通过", "callId", req.CallID, "userId", req.Request.UserID, "extensionId", extension.ID, "merchantId", extension.MerchantID)
		}
	}
	plan := BuildAPIOutboundPlan(req.CallID, req.Version, fsAddr, req.Request, extensionNumber, sipDomain, logger)
	cmd := telephony.NewCommand(
		"originate:"+req.CallID,
		"originate",
		req.CallID,
		plan.AgentUUID,
		plan.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowAPIOutbound,
		map[string]any{
			"originateMode":   plan.OriginateMode,
			"agentId":         plan.AgentID,
			"agentUuid":       plan.AgentUUID,
			"customerUuid":    plan.CustomerUUID,
			"destination":     plan.Destination,
			"domainOrGateway": plan.DomainOrGateway,
			"register":        plan.Register,
			"options":         plan.Options,
			"routeVersion":    req.Version,
			"callee":          req.Request.Callee,
			"extra":           req.Request.Extra,
			"userId":          req.Request.UserID,
		},
	)
	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("ESL API 外呼起呼命令执行失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
		return err
	}
	if s.SessionService != nil {
		if err := s.SessionService.CreateFromOriginate(ctx, cmd); err != nil {
			logger.Error("ESL API 外呼会话创建失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
			return err
		}
	}
	if s.Events != nil {
		if err := s.Events.Publish(ctx, contracts.NewEventEnvelope(
			"esl-command-sent:"+req.CallID,
			contracts.EventESLCommandSent,
			cmd.CommandID,
			"call",
			req.CallID,
			contracts.ServiceCall,
			map[string]any{"callId": req.CallID, "commandId": cmd.CommandID, "command": cmd.Command, "profile": string(cmd.Profile)},
		)); err != nil {
			logger.Error("ESL 起呼命令事件发布失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
			return err
		}
	}
	logger.Info("ESL API 外呼起呼命令已提交", logging.TelephonyAttrs(cmd)...)
	return nil
}

// StartBatchOutbound 启动一次批量外呼号码起呼。
// 批量外呼遵循  CUSTOMER_FIRST 语义：先呼客户号码，后续由流程处理坐席桥接和收口。
func (s *OriginateService) StartBatchOutbound(ctx context.Context, req BatchOriginateRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("开始 ESL 批量外呼起呼编排", "callId", req.CallID, "taskId", req.Request.BatchTaskID, "telId", req.Request.BatchCallTelID, "userId", req.Request.UserID, "version", req.Version)
	if err := s.checkLicenseConcurrency(ctx, req.CallID); err != nil {
		logger.Warn("ESL 批量外呼因系统授权限制被拦截", "callId", req.CallID, "taskId", req.Request.BatchTaskID, "error", err.Error())
		return err
	}
	// 如果是 AI 自动外呼，且未被授权使用 AI 调度器模块，则进行安全拦截
	if req.Request.AIFlag && s.Limiter != nil {
		if err := s.Limiter.CheckFeatureLimit(ctx, "ai_scheduler"); err != nil {
			logger.Warn("ESL 批量外呼因 AI 调度器模块未获授权被拦截", "callId", req.CallID, "taskId", req.Request.BatchTaskID, "error", err.Error())
			return err
		}
	}
	if req.Request.Extension == "" && s.Extensions != nil {
		extension, err := s.Extensions.GetByUserID(ctx, req.Request.UserID)
		if err != nil {
			logger.Error("ESL 批量外呼读取坐席分机失败", "callId", req.CallID, "taskId", req.Request.BatchTaskID, "userId", req.Request.UserID, "error", err.Error())
			return err
		}
		req.Request.Extension = extension.ExtensionNumber
		req.Request.ExtensionID = extension.ID
		logger.Info("ESL 批量外呼已读取坐席分机", "callId", req.CallID, "taskId", req.Request.BatchTaskID, "userId", req.Request.UserID, "extensionId", extension.ID, "extension", extension.ExtensionNumber, "merchantId", extension.MerchantID)
	}
	fsAddr := "default"
	if s.NodeSelector != nil {
		selected, err := s.NodeSelector.SelectAPIOutbound(ctx, OriginateRequest{
			Version: req.Version,
			CallID:  req.CallID,
			Request: contracts.ApiCallReq{UserID: req.Request.UserID, Callee: req.Request.Phone, Extra: req.Request.Extra},
		})
		if err != nil {
			logger.Error("ESL 批量外呼选择 FreeSWITCH 节点失败", "callId", req.CallID, "taskId", req.Request.BatchTaskID, "error", err.Error())
			return err
		}
		fsAddr = selected
	}
	profile := contracts.CallFlowBatchOutbound
	if req.Request.CallMode == 1 {
		profile = contracts.CallFlowBatchPredictive
	} else if req.Request.CallMode == 2 {
		profile = contracts.CallFlowBatchSynergy
	}

	plan := BuildBatchOutboundPlan(req.CallID, req.Version, fsAddr, req.Request, logger)
	cmd := telephony.NewCommand(
		"batch-originate:"+req.CallID,
		"originate",
		req.CallID,
		plan.CustomerUUID,
		plan.FSAddr,
		contracts.LegRoleCustomer,
		profile,
		map[string]any{
			"originateMode":   plan.OriginateMode,
			"agentId":         plan.AgentID,
			"agentUuid":       plan.AgentUUID,
			"customerUuid":    plan.CustomerUUID,
			"destination":     plan.Destination,
			"domainOrGateway": plan.DomainOrGateway,
			"register":        plan.Register,
			"options":         plan.Options,
			"routeVersion":    req.Version,
			"batchTaskId":     req.Request.BatchTaskID,
			"batchCallTelId":  req.Request.BatchCallTelID,
			"callee":          req.Request.Phone,
			"extension":       req.Request.Extension,
			"extra":           req.Request.Extra,
			"userId":          req.Request.UserID,
			"merchantId":      req.Request.MerchantID,
			"callMode":        req.Request.CallMode,
			"callRatio":       req.Request.CallRatio,
			"queueEnable":     req.Request.QueueEnable,
		},
	)
	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("ESL 批量外呼起呼命令执行失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
		return err
	}
	if s.SessionService != nil {
		if err := s.SessionService.CreateFromOriginate(ctx, cmd); err != nil {
			logger.Error("ESL 批量外呼会话创建失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
			return err
		}
	}
	if s.Events != nil {
		if err := s.Events.Publish(ctx, contracts.NewEventEnvelope(
			"batch-command-sent:"+req.CallID,
			contracts.EventESLCommandSent,
			cmd.CommandID,
			"call",
			req.CallID,
			contracts.ServiceCall,
			map[string]any{"callId": req.CallID, "commandId": cmd.CommandID, "command": cmd.Command, "profile": string(cmd.Profile), "batchTaskId": req.Request.BatchTaskID, "batchCallTelId": req.Request.BatchCallTelID},
		)); err != nil {
			logger.Error("ESL 批量外呼命令事件发布失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
			return err
		}
	}
	logger.Info("ESL 批量外呼起呼命令已提交", logging.TelephonyAttrs(cmd)...)
	return nil
}

// StartAPICustomerOutbound 在 API 外呼中为客户腿发起 originate。
//
// 该步骤在坐席腿返回 180/183 后触发：先从会话中读取 agent/customer UUID 和呼叫上下文，
// 再结合 CTI 选号结果发起客户腿，最后把选择结果和客户腿标记回写到会话元数据。
func (s *OriginateService) StartAPICustomerOutbound(ctx context.Context, req APICustomerOriginateRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if s.SessionService == nil {
		return ErrInvalidCommand
	}
	session, err := s.SessionService.Store.Get(ctx, req.CallID)
	if err != nil {
		logger.Error("API 外呼客户腿起呼前读取会话失败", "callId", req.CallID, "error", err.Error())
		return err
	}
	agentUUID, _ := session.Metadata["agentUuid"].(string)
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	callee, _ := session.Metadata["callee"].(string)
	userID := intFromMetadata(session.Metadata, "userId")
	merchantID := intFromMetadata(session.Metadata, "merchantId")
	extension, _ := session.Metadata["extension"].(string)
	if agentUUID == "" || customerUUID == "" || callee == "" {
		logger.Warn("API 外呼客户腿起呼上下文不完整", "callId", req.CallID, "agentUuid", agentUUID, "customerUuid", customerUUID, "callee", callee)
		return ErrInvalidCommand
	}
	register := req.Selection.Model == 1
	domainOrGateway := req.Selection.GatewayRegion
	if register && req.Selection.GatewayName != "" {
		domainOrGateway = req.Selection.GatewayName
	}
	options := map[string]any{
		"Call-ID":                      req.CallID,
		"yunshu_call_id":               req.CallID,
		"origination_uuid":             customerUUID,
		"hangup_after_bridge":          false,
		"bridge_early_media":           true,
		"ignore_early_media":           false,
		"origination_caller_id_number": req.Selection.Phone,
		"caller_id_number":             req.Selection.Phone,
		"variable_customer_number":     callee,
		"variable_route_version":       req.Version,
		"variable_api_user_id":         userID,
		"variable_merchant_id":         merchantID,
		"variable_agent_uuid":          agentUUID,
		"variable_customer_uuid":       customerUUID,
	}
	if req.Selection.CallerPrefix != "" {
		options["caller_prefix"] = req.Selection.CallerPrefix
	}
	if req.Selection.CalleePrefix != "" {
		options["callee_prefix"] = req.Selection.CalleePrefix
	}
	if req.Selection.CallerRewriteRule != "" {
		options["caller_rewrite_rule"] = req.Selection.CallerRewriteRule
	}
	if req.Selection.CalleeRewriteRule != "" {
		options["callee_rewrite_rule"] = req.Selection.CalleeRewriteRule
	}
	if req.Selection.SupplementRingFile != "" {
		options["ringback"] = req.Selection.SupplementRingFile
		options["variable_yunshu_ringback_file"] = req.Selection.SupplementRingFile
	}
	if req.Selection.SupplementRing {
		options["variable_yunshu_supplement_ring"] = true
	}
	if req.Selection.BroadcastTime > 0 {
		options["variable_yunshu_broadcast_time"] = req.Selection.BroadcastTime
	}
	if req.Selection.BroadcastTimeFlag {
		options["variable_yunshu_broadcast_time_flag"] = true
	}
	cmd := telephony.NewCommand(
		"api-customer-originate:"+req.CallID,
		"originate",
		req.CallID,
		customerUUID,
		session.FSAddr,
		contracts.LegRoleCustomer,
		contracts.CallFlowAPIOutbound,
		map[string]any{
			"originateMode":         contracts.OriginateModeCustomerFirst,
			"agentId":               extension,
			"agentUuid":             agentUUID,
			"customerUuid":          customerUUID,
			"destination":           callee,
			"domainOrGateway":       domainOrGateway,
			"register":              register,
			"caller":                req.Selection.Phone,
			"callee":                callee,
			"gatewayId":             req.Selection.GatewayID,
			"gatewayName":           req.Selection.GatewayName,
			"gatewayRegion":         req.Selection.GatewayRegion,
			"callerPrefix":          req.Selection.CallerPrefix,
			"calleePrefix":          req.Selection.CalleePrefix,
			"callerRewriteRule":     req.Selection.CallerRewriteRule,
			"calleeRewriteRule":     req.Selection.CalleeRewriteRule,
			"supplementRing":        req.Selection.SupplementRing,
			"supplementRingFile":    req.Selection.SupplementRingFile,
			"broadcastTime":         req.Selection.BroadcastTime,
			"broadcastTimeFlag":     req.Selection.BroadcastTimeFlag,
			"options":               options,
			"userId":                userID,
			"merchantId":            merchantID,
			"extension":             extension,
			"customerOriginateSent": true,
			"selectedCaller":        req.Selection.Phone,
			"selectedGatewayId":     req.Selection.GatewayID,
			"selectedGatewayName":   req.Selection.GatewayName,
			"selectedGatewayRegion": req.Selection.GatewayRegion,
		},
	)
	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("API 外呼客户腿起呼命令执行失败", "callId", req.CallID, "agentUuid", agentUUID, "customerUuid", customerUUID, "error", err.Error())
		return err
	}
	if err := s.SessionService.CreateFromOriginate(ctx, cmd); err != nil {
		logger.Error("API 外呼客户腿会话回写失败", "callId", req.CallID, "error", err.Error())
		return err
	}
	session.Metadata["customerOriginateSent"] = true
	session.Metadata["selectedCaller"] = req.Selection.Phone
	session.Metadata["selectedGatewayId"] = req.Selection.GatewayID
	session.Metadata["selectedGatewayName"] = req.Selection.GatewayName
	session.Metadata["selectedGatewayRegion"] = req.Selection.GatewayRegion
	session.Metadata["selectedModel"] = req.Selection.Model
	if err := s.SessionService.Store.Save(ctx, session); err != nil {
		logger.Error("API 外呼客户腿会话状态保存失败", "callId", req.CallID, "error", err.Error())
		return err
	}
	logger.Info("API 外呼客户腿起呼命令已提交", "callId", req.CallID, "caller", req.Selection.Phone, "gatewayId", req.Selection.GatewayID, "gatewayName", req.Selection.GatewayName, "gatewayRegion", req.Selection.GatewayRegion)
	return nil
}

// BridgeAPIOutbound 将 API 外呼的坐席腿与客户腿桥接。
func (s *OriginateService) BridgeAPIOutbound(ctx context.Context, req APIBridgeRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if s.CommandService == nil {
		return ErrInvalidCommand
	}
	cmd := telephony.NewCommand(
		"api-bridge:"+req.CallID+":"+req.AgentUUID+":"+req.CustomerUUID,
		"bridge",
		req.CallID,
		req.AgentUUID,
		req.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowAPIOutbound,
		map[string]any{
			"uuid1": req.AgentUUID,
			"uuid2": req.CustomerUUID,
		},
	)
	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("API 外呼桥接命令执行失败", "callId", req.CallID, "agentUuid", req.AgentUUID, "customerUuid", req.CustomerUUID, "error", err.Error())
		return err
	}
	logger.Info("API 外呼两腿桥接命令已提交", "callId", req.CallID, "agentUuid", req.AgentUUID, "customerUuid", req.CustomerUUID)
	return nil
}

func intFromMetadata(metadata map[string]any, key string) int {
	value, ok := metadata[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func agentDomainOrGateway(sipDomain string) string {
	if sipDomain != "" {
		return fmt.Sprintf("%s;fs_path=sip:%s", sipDomain, defaultKamailioDomain)
	}
	return defaultKamailioDomain
}

// BatchAgentOriginateRequest 表示批量外呼中为坐席腿发起 originate 的请求。
type BatchAgentOriginateRequest struct {
	Version      string
	CallID       string
	Extension    string
	SipDomain    string
	AgentUUID    string
	CustomerUUID string
	FSAddr       string
	UserID       int
	MerchantID   int
}

// StartBatchAgentOutbound 在批量外呼中客户应答后为坐席腿发起 originate。
func (s *OriginateService) StartBatchAgentOutbound(ctx context.Context, req BatchAgentOriginateRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("批量外呼客户应答，开始为坐席腿发起 originate", "callId", req.CallID, "extension", req.Extension)

	options := map[string]any{
		"Call-ID":                req.CallID,
		"yunshu_call_id":         req.CallID,
		"origination_uuid":       req.AgentUUID,
		"hangup_after_bridge":    false,
		"bridge_early_media":     true,
		"ignore_early_media":     false,
		"variable_agent_id":      req.Extension,
		"variable_route_version": req.Version,
		"variable_api_user_id":   req.UserID,
		"variable_merchant_id":   req.MerchantID,
		"variable_agent_uuid":    req.AgentUUID,
		"variable_customer_uuid": req.CustomerUUID,
		"sip_h_X-Internal-Call":  true,
	}

	cmd := telephony.NewCommand(
		"batch-agent-originate:"+req.CallID,
		"originate",
		req.CallID,
		req.AgentUUID,
		req.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowBatchOutbound,
		map[string]any{
			"originateMode":   contracts.OriginateModeCustomerFirst,
			"agentId":         req.Extension,
			"agentUuid":       req.AgentUUID,
			"customerUuid":    req.CustomerUUID,
			"destination":     req.Extension,
			"domainOrGateway": agentDomainOrGateway(req.SipDomain),
			"register":        false,
			"options":         options,
			"userId":          req.UserID,
			"merchantId":      req.MerchantID,
		},
	)

	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("批量外呼坐席腿起呼命令执行失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
		return err
	}
	if s.SessionService != nil {
		if err := s.SessionService.CreateFromOriginate(ctx, cmd); err != nil {
			logger.Error("批量外呼坐席腿会话回写失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
			return err
		}
	}
	logger.Info("批量外呼坐席腿起呼命令已提交", logging.TelephonyAttrs(cmd)...)
	return nil
}

// BridgeBatchOutbound 将批量外呼的客户腿与坐席腿桥接。
func (s *OriginateService) BridgeBatchOutbound(ctx context.Context, req APIBridgeRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if s.CommandService == nil {
		return ErrInvalidCommand
	}
	cmd := telephony.NewCommand(
		"batch-bridge:"+req.CallID+":"+req.AgentUUID+":"+req.CustomerUUID,
		"bridge",
		req.CallID,
		req.AgentUUID,
		req.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowBatchOutbound,
		map[string]any{
			"uuid1": req.AgentUUID,
			"uuid2": req.CustomerUUID,
		},
	)
	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("批量外呼桥接命令执行失败", "callId", req.CallID, "agentUuid", req.AgentUUID, "customerUuid", req.CustomerUUID, "error", err.Error())
		return err
	}
	logger.Info("批量外呼两腿桥接命令已提交", "callId", req.CallID, "agentUuid", req.AgentUUID, "customerUuid", req.CustomerUUID)
	return nil
}

// StartDialpadCustomerOutbound 在拨号盘直呼中为客户腿发起 originate。
// 注意：本函数只执行 originate 命令和发布事件，不做 session load/save。
// Session 生命周期由调用方 handleDialpadAgentAnswer 统一管理，避免重复 load-modify-save 写覆盖。
func (s *OriginateService) StartDialpadCustomerOutbound(ctx context.Context, req APICustomerOriginateRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if err := s.checkLicenseConcurrency(ctx, req.CallID); err != nil {
		logger.Warn("拨号盘直呼因系统授权并发超限被拦截", "callId", req.CallID, "error", err.Error())
		return err
	}
	if s.SessionService == nil {
		return ErrInvalidCommand
	}

	// 获取会话仅用于读取必要的上下文信息，不做任何修改
	session, err := s.SessionService.Store.Get(ctx, req.CallID)
	if err != nil {
		logger.Error("拨号盘直呼客户腿起呼前读取会话失败", "callId", req.CallID, "error", err.Error())
		return err
	}

	// 仅读取必要的上下文，不修改 Metadata
	agentUUID, _ := session.Metadata["agentUuid"].(string)
	customerUUID := NewDeterministicUUID("customer", req.CallID)
	callee, _ := session.Metadata["callee"].(string)
	userID := intFromMetadata(session.Metadata, "userId")
	merchantID := intFromMetadata(session.Metadata, "merchantId")
	extension, _ := session.Metadata["extension"].(string)

	if agentUUID == "" || callee == "" {
		logger.Warn("拨号盘直呼客户腿起呼上下文不完整", "callId", req.CallID, "agentUuid", agentUUID, "callee", callee)
		return ErrInvalidCommand
	}

	register := req.Selection.Model == 1
	domainOrGateway := req.Selection.GatewayRegion
	if register && req.Selection.GatewayName != "" {
		domainOrGateway = req.Selection.GatewayName
	}

	options := map[string]any{
		"Call-ID":                      req.CallID,
		"yunshu_call_id":               req.CallID,
		"origination_uuid":             customerUUID,
		"hangup_after_bridge":          false,
		"bridge_early_media":           true,
		"ignore_early_media":           false,
		"origination_caller_id_number": req.Selection.Phone,
		"caller_id_number":             req.Selection.Phone,
		"variable_customer_number":     callee,
		"variable_route_version":       req.Version,
		"variable_api_user_id":         userID,
		"variable_merchant_id":         merchantID,
		"variable_agent_uuid":          agentUUID,
		"variable_customer_uuid":       customerUUID,
	}

	if req.Selection.CallerPrefix != "" {
		options["caller_prefix"] = req.Selection.CallerPrefix
	}
	if req.Selection.CalleePrefix != "" {
		options["callee_prefix"] = req.Selection.CalleePrefix
	}
	if req.Selection.CallerRewriteRule != "" {
		options["caller_rewrite_rule"] = req.Selection.CallerRewriteRule
	}
	if req.Selection.CalleeRewriteRule != "" {
		options["callee_rewrite_rule"] = req.Selection.CalleeRewriteRule
	}
	if req.Selection.SupplementRingFile != "" {
		options["ringback"] = req.Selection.SupplementRingFile
		options["variable_yunshu_ringback_file"] = req.Selection.SupplementRingFile
	}
	if req.Selection.SupplementRing {
		options["variable_yunshu_supplement_ring"] = true
	}
	if req.Selection.BroadcastTime > 0 {
		options["variable_yunshu_broadcast_time"] = req.Selection.BroadcastTime
	}
	if req.Selection.BroadcastTimeFlag {
		options["variable_yunshu_broadcast_time_flag"] = true
	}

	cmd := telephony.NewCommand(
		"api-customer-originate:"+req.CallID,
		"originate",
		req.CallID,
		customerUUID,
		session.FSAddr,
		contracts.LegRoleCustomer,
		contracts.CallFlowAPIDirect,
		map[string]any{
			"originateMode":         contracts.OriginateModeCustomerFirst,
			"agentId":               extension,
			"agentUuid":             agentUUID,
			"customerUuid":          customerUUID,
			"destination":           callee,
			"domainOrGateway":       domainOrGateway,
			"register":              register,
			"caller":                req.Selection.Phone,
			"callee":                callee,
			"gatewayId":             req.Selection.GatewayID,
			"gatewayName":           req.Selection.GatewayName,
			"gatewayRegion":         req.Selection.GatewayRegion,
			"callerPrefix":          req.Selection.CallerPrefix,
			"calleePrefix":          req.Selection.CalleePrefix,
			"callerRewriteRule":     req.Selection.CallerRewriteRule,
			"calleeRewriteRule":     req.Selection.CalleeRewriteRule,
			"supplementRing":        req.Selection.SupplementRing,
			"supplementRingFile":    req.Selection.SupplementRingFile,
			"broadcastTime":         req.Selection.BroadcastTime,
			"broadcastTimeFlag":     req.Selection.BroadcastTimeFlag,
			"options":               options,
			"userId":                userID,
			"merchantId":            merchantID,
			"extension":             extension,
			"selectedCaller":        req.Selection.Phone,
			"selectedGatewayId":     req.Selection.GatewayID,
			"selectedGatewayName":   req.Selection.GatewayName,
			"selectedGatewayRegion": req.Selection.GatewayRegion,
		},
	)

	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("拨号盘直呼客户腿起呼命令执行失败", "callId", req.CallID, "agentUuid", agentUUID, "customerUuid", customerUUID, "error", err.Error())
		return err
	}

	// 仅注册会话到 Store，不修改 Metadata（session 生命周期由调用方统一管理）
	if err := s.SessionService.CreateFromOriginate(ctx, cmd); err != nil {
		logger.Error("拨号盘直呼客户腿会话UUID注册失败", "callId", req.CallID, "error", err.Error())
		return err
	}

	if s.Events != nil {
		if err := s.Events.Publish(ctx, contracts.NewEventEnvelope(
			"direct-command-sent:"+req.CallID,
			contracts.EventESLCommandSent,
			cmd.CommandID,
			"call",
			req.CallID,
			contracts.ServiceCall,
			map[string]any{"callId": req.CallID, "commandId": cmd.CommandID, "command": cmd.Command, "profile": string(cmd.Profile)},
		)); err != nil {
			logger.Error("ESL 起呼命令事件发布失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
			return err
		}
	}

	logger.Info("拨号盘直呼客户腿起呼命令已提交", "callId", req.CallID, "caller", req.Selection.Phone, "gatewayId", req.Selection.GatewayID, "gatewayName", req.Selection.GatewayName, "gatewayRegion", req.Selection.GatewayRegion)
	return nil
}

// StartInboundAgentOutbound 在客户呼入中呼起坐席分机腿。
func (s *OriginateService) StartInboundAgentOutbound(ctx context.Context, req BatchAgentOriginateRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("客户呼入已接听，开始为坐席腿发起 originate", "callId", req.CallID, "extension", req.Extension)
	if err := s.checkLicenseConcurrency(ctx, req.CallID); err != nil {
		logger.Warn("客户呼入坐席腿起呼因系统授权并发超限被拦截", "callId", req.CallID, "extension", req.Extension, "error", err.Error())
		return err
	}

	options := map[string]any{
		"Call-ID":                req.CallID,
		"yunshu_call_id":         req.CallID,
		"origination_uuid":       req.AgentUUID,
		"hangup_after_bridge":    false,
		"bridge_early_media":     true,
		"ignore_early_media":     false,
		"variable_agent_id":      req.Extension,
		"variable_route_version": req.Version,
		"variable_api_user_id":   req.UserID,
		"variable_merchant_id":   req.MerchantID,
		"variable_agent_uuid":    req.AgentUUID,
		"variable_customer_uuid": req.CustomerUUID,
		"sip_h_X-Internal-Call":  true,
	}

	cmd := telephony.NewCommand(
		"inbound-agent-originate:"+req.CallID,
		"originate",
		req.CallID,
		req.AgentUUID,
		req.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowInbound,
		map[string]any{
			"originateMode":   contracts.OriginateModeCustomerFirst,
			"agentId":         req.Extension,
			"agentUuid":       req.AgentUUID,
			"customerUuid":    req.CustomerUUID,
			"destination":     req.Extension,
			"domainOrGateway": agentDomainOrGateway(req.SipDomain),
			"register":        false,
			"options":         options,
			"userId":          req.UserID,
			"merchantId":      req.MerchantID,
		},
	)

	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("客户呼入坐席腿起呼命令执行失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
		return err
	}
	if s.SessionService != nil {
		if err := s.SessionService.CreateFromOriginate(ctx, cmd); err != nil {
			logger.Error("客户呼入坐席腿会话回写失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
			return err
		}
	}
	if s.Events != nil {
		if err := s.Events.Publish(ctx, contracts.NewEventEnvelope(
			"inbound-command-sent:"+req.CallID,
			contracts.EventESLCommandSent,
			cmd.CommandID,
			"call",
			req.CallID,
			contracts.ServiceCall,
			map[string]any{"callId": req.CallID, "commandId": cmd.CommandID, "command": cmd.Command, "profile": string(cmd.Profile)},
		)); err != nil {
			logger.Error("ESL 起呼命令事件发布失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
			return err
		}
	}
	logger.Info("客户呼入坐席腿起呼命令已提交", logging.TelephonyAttrs(cmd)...)
	return nil
}

// BridgeDialpadDirect 将拨号盘直呼的坐席腿与客户腿桥接。
func (s *OriginateService) BridgeDialpadDirect(ctx context.Context, req APIBridgeRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if s.CommandService == nil {
		return ErrInvalidCommand
	}
	cmd := telephony.NewCommand(
		"api-direct-bridge:"+req.CallID+":"+req.AgentUUID+":"+req.CustomerUUID,
		"bridge",
		req.CallID,
		req.AgentUUID,
		req.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowAPIDirect,
		map[string]any{
			"uuid1": req.AgentUUID,
			"uuid2": req.CustomerUUID,
		},
	)
	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("拨号盘直呼桥接命令执行失败", "callId", req.CallID, "agentUuid", req.AgentUUID, "customerUuid", req.CustomerUUID, "error", err.Error())
		return err
	}
	logger.Info("拨号盘直呼两腿桥接命令已提交", "callId", req.CallID, "agentUuid", req.AgentUUID, "customerUuid", req.CustomerUUID)
	return nil
}

// BridgeInbound 将客户呼入的客户腿与坐席腿桥接。
func (s *OriginateService) BridgeInbound(ctx context.Context, req APIBridgeRequest) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if s.CommandService == nil {
		return ErrInvalidCommand
	}
	cmd := telephony.NewCommand(
		"inbound-bridge:"+req.CallID+":"+req.AgentUUID+":"+req.CustomerUUID,
		"bridge",
		req.CallID,
		req.AgentUUID,
		req.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowInbound,
		map[string]any{
			"uuid1": req.AgentUUID,
			"uuid2": req.CustomerUUID,
		},
	)
	if err := s.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("客户呼入桥接命令执行失败", "callId", req.CallID, "agentUuid", req.AgentUUID, "customerUuid", req.CustomerUUID, "error", err.Error())
		return err
	}
	logger.Info("客户呼入两腿桥接命令已提交", "callId", req.CallID, "agentUuid", req.AgentUUID, "customerUuid", req.CustomerUUID)
	return nil
}
