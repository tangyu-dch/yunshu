package esl

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"yunshu/internal/contracts"
)

var (
	defaultKamailioDomain = "127.0.0.1:5060"
)

// SetDefaultKamailioAddr 设置 Kamailio SIP 代理地址，应在服务启动时从配置注入。
func SetDefaultKamailioAddr(addr string) {
	if addr != "" {
		defaultKamailioDomain = addr
	}
}

// OriginatePlan 是业务起呼命令到 FreeSWITCH originate 命令之间的中间态。
//
// 结构对齐  OriginatePlan：业务编排在领域层完成，FS adapter 只负责把计划翻译为
// ESL 命令，避免连接层再根据业务场景写分支。
type OriginatePlan struct {
	CallID             string                  `json:"callId"`
	FSAddr             string                  `json:"fsAddr"`
	OriginateMode      contracts.OriginateMode `json:"originateMode"`
	AgentID            string                  `json:"agentId,omitempty"`
	AgentUUID          string                  `json:"agentUuid,omitempty"`
	CustomerUUID       string                  `json:"customerUuid,omitempty"`
	Destination        string                  `json:"destination"`
	DomainOrGateway    string                  `json:"domainOrGateway"`
	Register           bool                    `json:"register"`
	SupplementRing     bool                    `json:"supplementRing"`
	SupplementRingFile string                  `json:"supplementRingFile,omitempty"`
	BroadcastTime      int64                   `json:"broadcastTime,omitempty"`
	BroadcastTimeFlag  bool                    `json:"broadcastTimeFlag,omitempty"`
	Options            map[string]any          `json:"options,omitempty"`
}

// BuildAPIOutboundPlan 构建 API 外呼的 AGENT_FIRST originate 计划。
//
//	API 外呼会先呼坐席分机，再由后续流程选择客户线路；这里保持相同方向。
func BuildAPIOutboundPlan(callID, version, fsAddr string, req contracts.ApiCallReq, extensionNumber, sipDomain string, logger *slog.Logger) OriginatePlan {
	if logger == nil {
		logger = slog.Default()
	}
	extra := parseExtra(req.Extra)
	extension := contracts.FirstNonEmpty(extensionNumber, extra["extension"], extra["extensionNumber"], extra["agentId"], strconv.Itoa(req.UserID))
	if extension == strconv.Itoa(req.UserID) {
		logger.Warn("API 外呼未解析到分机号，临时使用 userId 作为分机占位", "callId", callID, "userId", req.UserID, "impact", "生产环境必须配置 extension 数据库仓储")
	}
	agentUUID := newDeterministicUUID("agent", callID)
	customerUUID := newDeterministicUUID("customer", callID)
	displayNumber := maskPhone(req.Callee)
	supplementRing := extraBool(extra, "supplementRing", "supplement_ring")
	supplementRingFile := contracts.FirstNonEmpty(extra["supplementRingFile"], extra["supplement_ring_file"], extra["ringbackFile"], extra["ringback_file"], extra["yunshuRingbackFile"], extra["yunshu_ringback_file"])
	broadcastTime := extraInt64(extra, "broadcastTime", "broadcast_time")
	broadcastTimeFlag := extraBool(extra, "broadcastTimeFlag", "broadcast_time_flag")

	domainOrGateway := defaultKamailioDomain
	paiDomain := domainHost(defaultKamailioDomain)
	if sipDomain != "" {
		domainOrGateway = fmt.Sprintf("%s;fs_path=sip:%s", sipDomain, defaultKamailioDomain)
		paiDomain = sipDomain
	}

	options := map[string]any{
		"Call-ID":                      callID,
		"yunshu_call_id":               callID,
		"Caller-Username":              displayNumber,
		"Caller-Caller-ID-Number":      displayNumber,
		"origination_caller_id_name":   displayNumber,
		"origination_caller_id_number": displayNumber,
		"effective_caller_id_name":     displayNumber,
		"effective_caller_id_number":   displayNumber,
		"origination_callee_id_name":   displayNumber,
		"origination_callee_id_number": displayNumber,
		"origination_uuid":             agentUUID,
		"hangup_after_bridge":          false,
		"bridge_early_media":           true,
		"ignore_early_media":           false,
		"sip_from_user":                displayNumber,
		"sip_from_display":             displayNumber,
		"sip_h_P-Asserted-Identity":    fmt.Sprintf("\"%s\"<sip:%s@%s>", displayNumber, displayNumber, paiDomain),
		"sip_h_Remote-Party-ID":        fmt.Sprintf("\"%s\"<sip:%s@%s>", displayNumber, displayNumber, paiDomain),
		"sip_h_X-Internal-Call":        true,
		"sip_h_X-S-C-I":                callID,
		"sip_h_X-S-C-T":                0,
		"variable_customer_number":     req.Callee,
		"variable_route_version":       version,
		"variable_api_user_id":         req.UserID,
	}
	if supplementRingFile != "" {
		options["ringback"] = supplementRingFile
		options["variable_yunshu_ringback_file"] = supplementRingFile
	}
	if supplementRing {
		options["variable_yunshu_supplement_ring"] = true
	}
	if broadcastTime > 0 {
		options["variable_yunshu_broadcast_time"] = broadcastTime
	}
	if broadcastTimeFlag {
		options["variable_yunshu_broadcast_time_flag"] = true
	}
	logger.Info("已构建 API 外呼 AGENT_FIRST 起呼计划", "callId", callID, "fsAddr", fsAddr, "extension", extension, "sipDomain", sipDomain, "mode", contracts.OriginateModeAgentFirst)
	return OriginatePlan{
		CallID:             callID,
		FSAddr:             fsAddr,
		OriginateMode:      contracts.OriginateModeAgentFirst,
		AgentID:            extension,
		AgentUUID:          agentUUID,
		CustomerUUID:       customerUUID,
		Destination:        extension,
		DomainOrGateway:    domainOrGateway,
		Register:           false,
		SupplementRing:     supplementRing,
		SupplementRingFile: supplementRingFile,
		BroadcastTime:      broadcastTime,
		BroadcastTimeFlag:  broadcastTimeFlag,
		Options:            options,
	}
}

// BuildBatchOutboundPlan 构建批量外呼 CUSTOMER_FIRST 起呼计划。
//
//	批量外呼由 CTI 调度器打包任务、号码、分机和商户上下文，ESL 不再反查业务库；
//
// Go 侧保持这个边界，先呼客户号码，再由事件流程处理桥接和下一号码调度。
func BuildBatchOutboundPlan(callID, version, fsAddr string, req contracts.BatchCallReq, logger *slog.Logger) OriginatePlan {
	if logger == nil {
		logger = slog.Default()
	}
	extra := parseExtra(req.Extra)
	// 网关选择：优先使用 CTI 运行时选号分配的网关，否则从 extra 中获取
	gateway := req.CallerGatewayID
	if gateway == "" {
		gateway = contracts.FirstNonEmpty(extra["gatewayName"], extra["gatewayRegion"], extra["gateway"], "default")
	}
	supplementRing := extraBool(extra, "supplementRing", "supplement_ring")
	supplementRingFile := contracts.FirstNonEmpty(extra["supplementRingFile"], extra["supplement_ring_file"], extra["ringbackFile"], extra["ringback_file"], extra["yunshuRingbackFile"], extra["yunshu_ringback_file"])
	broadcastTime := extraInt64(extra, "broadcastTime", "broadcast_time")
	broadcastTimeFlag := extraBool(extra, "broadcastTimeFlag", "broadcast_time_flag")
	customerUUID := newDeterministicUUID("batch-customer", callID)
	agentUUID := newDeterministicUUID("batch-agent", callID)
	options := map[string]any{
		"Call-ID":                      callID,
		"yunshu_call_id":               callID,
		"origination_uuid":             customerUUID,
		"hangup_after_bridge":          false,
		"bridge_early_media":           true,
		"ignore_early_media":           false,
		"sip_h_X-S-C-I":                callID,
		"sip_h_X-S-C-T":                1,
		"variable_route_version":       version,
		"variable_batch_task_id":       req.BatchTaskID,
		"variable_batch_call_tel_id":   req.BatchCallTelID,
		"variable_customer_number":     req.Phone,
		"variable_agent_extension":     req.Extension,
		"variable_api_user_id":         req.UserID,
		"origination_caller_id_number": maskPhone(req.Phone),
	}
	// CTI 运行时选号分配的主叫号码透传到 SIP INVITE From 头
	if req.CallerNumber != "" {
		options["origination_caller_id_number"] = req.CallerNumber
		options["variable_origination_caller_id_number"] = req.CallerNumber
		options["variable_yunshu_selected_caller"] = req.CallerNumber
	}
	if supplementRingFile != "" {
		options["ringback"] = supplementRingFile
		options["variable_yunshu_ringback_file"] = supplementRingFile
	}
	if supplementRing {
		options["variable_yunshu_supplement_ring"] = true
	}
	if broadcastTime > 0 {
		options["variable_yunshu_broadcast_time"] = broadcastTime
	}
	if broadcastTimeFlag {
		options["variable_yunshu_broadcast_time_flag"] = true
	}
	logger.Info("已构建批量外呼 CUSTOMER_FIRST 起呼计划", "callId", callID, "taskId", req.BatchTaskID, "telId", req.BatchCallTelID, "fsAddr", fsAddr, "gateway", gateway)
	return OriginatePlan{
		CallID:             callID,
		FSAddr:             fsAddr,
		OriginateMode:      contracts.OriginateModeCustomerFirst,
		AgentID:            req.Extension,
		AgentUUID:          agentUUID,
		CustomerUUID:       customerUUID,
		Destination:        req.Phone,
		DomainOrGateway:    gateway,
		Register:           req.GatewayRegister,
		SupplementRing:     supplementRing,
		SupplementRingFile: supplementRingFile,
		BroadcastTime:      broadcastTime,
		BroadcastTimeFlag:  broadcastTimeFlag,
		Options:            options,
	}
}

func parseExtra(raw string) map[string]string {
	if raw == "" {
		return map[string]string{}
	}
	values := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, val := range values {
		if val != nil {
			out[key] = fmt.Sprint(val)
		}
	}
	return out
}

func extraBool(extra map[string]string, keys ...string) bool {
	for _, key := range keys {
		if value := strings.TrimSpace(extra[key]); value != "" {
			return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
		}
	}
	return false
}

func extraInt64(extra map[string]string, keys ...string) int64 {
	for _, key := range keys {
		if value := strings.TrimSpace(extra[key]); value != "" {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func domainHost(domain string) string {
	for i := 0; i < len(domain); i++ {
		if domain[i] == ':' {
			return domain[:i]
		}
	}
	return domain
}

func maskPhone(phone string) string {
	if len(phone) < 8 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

func NewDeterministicUUID(role, callID string) string {
	return newDeterministicUUID(role, callID)
}

func newDeterministicUUID(role, callID string) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", hash(role+callID)&0xffffffff, hash(callID)&0xffff, hash(role)&0xffff, hash(role+":"+callID)&0xffff, hash(callID+role)&0xffffffffffff)
}

func hash(value string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(value); i++ {
		h ^= uint64(value[i])
		h *= 1099511628211
	}
	return h
}
