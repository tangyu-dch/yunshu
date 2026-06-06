package cti

// cti 包提供云枢呼叫中心系统的 CTI 控制服务。
//
// 呼叫控制服务通过 SessionStore 匹配活跃通话，
// 并由 GORM 反查坐席注册分机，使用专属 options 构造 FreeSWITCH eavesdrop/hangup 控制，
// 实现监听（Spy）、强插/插话（Whisper/Barge）以及强拆（Hangup）。

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	"yunshu/pkg/telephony"
)

var (
	ErrSessionNotFound   = errors.New("活跃通话会话不存在")
	ErrPermissionDenied  = errors.New("越权操作：通话或用户不属于发起商户")
	ErrExtensionNotFound = errors.New("未找到绑定且启用的主管分机")
	ErrNoActiveChannel   = errors.New("未找到可监听的活跃通道")
)

// ExtensionResolver 定义了反查坐席分机的接口能力，避免 Domain 直接依赖 DB 仓储实现。
type ExtensionResolver interface {
	GetByUserID(ctx context.Context, userID int) (esl.Extension, error)
}

// CallControlService 提供呼叫的监听、强插和强拆控制能力。
type CallControlService struct {
	SessionStore esl.SessionStore
	Command      *esl.CommandService
	Resolver     ExtensionResolver
	Logger       *slog.Logger
}

// NewCallControlService 创建呼叫控制服务实例。
func NewCallControlService(store esl.SessionStore, cmd *esl.CommandService, resolver ExtensionResolver, logger *slog.Logger) *CallControlService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CallControlService{
		SessionStore: store,
		Command:      cmd,
		Resolver:     resolver,
		Logger:       logger,
	}
}

// Hangup 对目标活跃呼叫或特定通道执行强行挂断（强拆）。
func (s *CallControlService) Hangup(ctx context.Context, merchantID int, req contracts.CallHangupReq) error {
	s.Logger.Info("接收到云枢 CTI 强拆请求", "merchantId", merchantID, "callId", req.CallID, "uuid", req.UUID)

	session, err := s.SessionStore.Get(ctx, req.CallID)
	if err != nil {
		s.Logger.Warn("强拆失败，未找到活跃通话会话", "callId", req.CallID, "error", err.Error())
		return ErrSessionNotFound
	}

	// 越权拦截校验：确保通话属于发起请求的商户
	mIDStr, _ := session.Metadata["merchantId"].(string)
	if mIDStr == "" {
		if mIDFloat, ok := session.Metadata["merchantId"].(float64); ok {
			mIDStr = strconv.Itoa(int(mIDFloat))
		} else if mIDInt, ok := session.Metadata["merchantId"].(int); ok {
			mIDStr = strconv.Itoa(mIDInt)
		}
	}
	if mIDStr != strconv.Itoa(merchantID) {
		s.Logger.Warn("强拆越权拦截", "callId", req.CallID, "sessionMerchantId", mIDStr, "reqMerchantId", merchantID)
		return ErrPermissionDenied
	}

	// 确定挂断的 UUID 列表
	var uuidsToKill []string
	if req.UUID != "" {
		if _, ok := session.UUIDs[req.UUID]; !ok {
			return fmt.Errorf("通道 %s 不在当前通话会话中", req.UUID)
		}
		uuidsToKill = append(uuidsToKill, req.UUID)
	} else {
		for uuid := range session.UUIDs {
			uuidsToKill = append(uuidsToKill, uuid)
		}
	}

	if len(uuidsToKill) == 0 {
		return errors.New("无活跃通道可执行挂断")
	}

	for _, uuid := range uuidsToKill {
		cmdID := fmt.Sprintf("cti:hangup:%s:%d", uuid, time.Now().UnixNano())
		cmd := telephony.NewCommand(
			cmdID,
			"hangup",
			req.CallID,
			uuid,
			session.FSAddr,
			contracts.LegRoleUnknown,
			contracts.CallFlowAPIOutbound,
			map[string]any{"reasonCode": "NORMAL_CLEARING"},
		)
		if err := s.Command.Execute(ctx, cmd); err != nil {
			s.Logger.Error("下发强拆挂断指令失败", "callId", req.CallID, "uuid", uuid, "error", err.Error())
			return err
		}
	}

	s.Logger.Info("云枢强拆挂断指令执行成功", "callId", req.CallID, "uuids", uuidsToKill)
	return nil
}

// Eavesdrop 呼叫主管分机并对其执行监听、单向插话或三方强插。
func (s *CallControlService) Eavesdrop(ctx context.Context, merchantID int, req contracts.CallEavesdropReq) error {
	s.Logger.Info("接收到云枢 CTI 监听/强插请求", "merchantId", merchantID, "targetCallId", req.TargetCallID, "mode", req.Mode)

	// 1. 获取目标活跃会话
	session, err := s.SessionStore.Get(ctx, req.TargetCallID)
	if err != nil {
		s.Logger.Warn("监听失败，未找到活跃通话会话", "targetCallId", req.TargetCallID, "error", err.Error())
		return ErrSessionNotFound
	}

	// 越权拦截校验：确保目标通话属于发起请求的商户
	mIDStr, _ := session.Metadata["merchantId"].(string)
	if mIDStr == "" {
		if mIDFloat, ok := session.Metadata["merchantId"].(float64); ok {
			mIDStr = strconv.Itoa(int(mIDFloat))
		} else if mIDInt, ok := session.Metadata["merchantId"].(int); ok {
			mIDStr = strconv.Itoa(mIDInt)
		}
	}
	if mIDStr != strconv.Itoa(merchantID) {
		s.Logger.Warn("监听越权拦截", "targetCallId", req.TargetCallID, "sessionMerchantId", mIDStr, "reqMerchantId", merchantID)
		return ErrPermissionDenied
	}

	// 2. 匹配被监听的目标 UUID
	var targetUuid string
	if req.TargetUUID != "" {
		if _, ok := session.UUIDs[req.TargetUUID]; ok {
			targetUuid = req.TargetUUID
		}
	}
	if targetUuid == "" {
		// 智能优先选择 Agent 腿通道
		for uuid, role := range session.UUIDs {
			if role == contracts.LegRoleAgent {
				targetUuid = uuid
				break
			}
		}
		// 次选 Customer 腿通道
		if targetUuid == "" {
			for uuid, role := range session.UUIDs {
				if role == contracts.LegRoleCustomer {
					targetUuid = uuid
					break
				}
			}
		}
		// 兜底选择
		if targetUuid == "" {
			for uuid := range session.UUIDs {
				targetUuid = uuid
				break
			}
		}
	}
	if targetUuid == "" {
		return ErrNoActiveChannel
	}

	// 3. 查询主管绑定的分机与 SIP 域
	ext, err := s.Resolver.GetByUserID(ctx, req.UserID)
	if err != nil {
		s.Logger.Warn("未查询到主管绑定的可用分机", "userId", req.UserID, "error", err.Error())
		return ErrExtensionNotFound
	}
	if ext.MerchantID != merchantID {
		s.Logger.Warn("主管所属商户不匹配，越权拒绝", "userId", req.UserID, "extMerchantID", ext.MerchantID, "reqMerchantID", merchantID)
		return ErrPermissionDenied
	}

	// 4. 组装 originate 多租户拨号串，Kamailio 默认 127.0.0.1:5060 转发路径
	defaultKamailioDomain := "127.0.0.1:5060"
	domainOrGateway := defaultKamailioDomain
	if ext.SipDomain != "" {
		domainOrGateway = fmt.Sprintf("%s;fs_path=sip:%s", ext.SipDomain, defaultKamailioDomain)
	}

	// 监听独立 CallID 与 UUID，防止与被监听通话会话冲突
	spyCallID := "spy-" + req.TargetCallID
	spyUUID := esl.NewDeterministicUUID("spy-agent", spyCallID)

	options := map[string]any{
		"origination_uuid":             spyUUID,
		"yunshu_call_id":               spyCallID,
		"sip_h_X-S-C-I":                req.TargetCallID,
		"hangup_after_bridge":          "false",
		"bridge_early_media":           "true",
		"ignore_early_media":           "false",
		"origination_caller_id_name":   "云枢监控端监听",
		"origination_caller_id_number": "9999",
	}

	// 根据模式配置 eavesdrop 变量
	switch req.Mode {
	case "whisper":
		options["eavesdrop_whisper"] = "true"
		options["eavesdrop_whisper_aleg"] = "true"
		options["eavesdrop_whisper_bleg"] = "false"
	case "barge":
		options["eavesdrop_whisper"] = "true"
		options["eavesdrop_whisper_aleg"] = "true"
		options["eavesdrop_whisper_bleg"] = "true"
	default: // "spy"
		options["eavesdrop_whisper"] = "false"
	}

	payload := map[string]any{
		"originateMode":   contracts.OriginateModeAgentFirst,
		"destination":     ext.ExtensionNumber,
		"domainOrGateway": domainOrGateway,
		"register":        false,
		"executeApp":      fmt.Sprintf("eavesdrop(%s)", targetUuid),
		"options":         options,
	}

	cmdID := fmt.Sprintf("cti:eavesdrop:%s:%d", req.TargetCallID, time.Now().UnixNano())
	cmd := telephony.NewCommand(
		cmdID,
		"originate",
		spyCallID,
		spyUUID,
		session.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowAPIOutbound,
		payload,
	)

	s.Logger.Info("下发云枢监听/强插 originate 指令呼叫主管", "spyCallId", spyCallID, "spyUUID", spyUUID, "fsAddr", session.FSAddr, "targetUuid", targetUuid, "mode", req.Mode)
	if err := s.Command.Execute(ctx, cmd); err != nil {
		s.Logger.Error("下发监听 originate 失败", "targetCallId", req.TargetCallID, "error", err.Error())
		return err
	}

	s.Logger.Info("云枢监听/强插 originate 指令下发成功", "targetCallId", req.TargetCallID, "spyCallId", spyCallID)
	return nil
}
