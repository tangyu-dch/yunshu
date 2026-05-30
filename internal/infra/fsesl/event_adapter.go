// Package fsesl 提供 FreeSWITCH ESL 事件适配能力。
package fsesl

import (
	"fmt"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/percipia/eslgo"

	"yunshu/internal/contracts"
)

// microsecondTimestampThreshold 用于区分毫秒和微秒精度的时间戳。
// FreeSWITCH 事件时间戳超过此阈值时按微秒处理，否则按毫秒处理。
const microsecondTimestampThreshold = int64(1_000_000_000_000_000)

// EventFromESL 把 eslgo 原始事件适配为领域 TelephonyEvent。
//
// 字段解析规则参考  FsDomainEventAdapter，优先读取 originate 注入的业务变量，
// 再回退到 FreeSWITCH 原生字段，保证事件可以被流程编排和会话 reducer 复用。
func EventFromESL(fsAddr string, event *eslgo.Event) contracts.TelephonyEvent {
	headers := map[string]any{}
	for key, values := range event.Headers {
		if len(values) == 1 {
			headers[key] = values[0]
			continue
		}
		headers[key] = append([]string(nil), values...)
	}
	eventName := get(event, "Event-Name")
	uuid := get(event, "Unique-ID")
	otherUUID := get(event, "Other-Leg-Unique-ID")
	callID := firstNonBlank(
		get(event, "variable_yunshu_call_id"),
		get(event, "Call-ID"),
		get(event, "sip_h_X-S-C-I"),
		get(event, "variable_sip_h_X-S-C-I"),
	)
	return contracts.TelephonyEvent{
		EventID:   resolveEventID(fsAddr, eventName, get(event, "Event-Sequence"), uuid, eventTime(event)),
		EventName: eventName,
		CallID:    callID,
		UUID:      uuid,
		FSAddr:    fsAddr,
		LegRole:   resolveLegRole(event),
		Profile:   contracts.CallFlowAPIOutbound,
		At:        eventTime(event),
		Headers: mergeHeaders(headers, map[string]any{
			"otherUuid":            otherUUID,
			"hangupCause":          get(event, "Hangup-Cause"),
			"customHangupCause":    firstNonBlank(get(event, "custom_hangup_cause"), get(event, "variable_custom_hangup_cause")),
			"q850":                 parseInt(get(event, "variable_hangup_cause_q850")),
			"inviteFailureStatus":  resolveInviteFailureStatus(event),
			"sipHangupDisposition": get(event, "variable_sip_hangup_disposition"),
			"callerDestination":    get(event, "Caller-Destination-Number"),
			"callerCallerIDNumber": get(event, "Caller-Caller-ID-Number"),
			"bridgeId":             resolveBridgeID(get(event, "Bridge-ID"), uuid, otherUUID),
			"playbackFile":         firstNonBlank(get(event, "Playback-File-Path"), get(event, "Playback-File"), get(event, "Application-Data"), get(event, "variable_yunshu_ringback_file")),
			"recordFilePath":       get(event, "Record-File-Path"),
		}),
	}
}

func get(event *eslgo.Event, key string) string {
	return strings.TrimSpace(event.Headers.Get(textproto.CanonicalMIMEHeaderKey(key)))
}

func eventTime(event *eslgo.Event) time.Time {
	raw := get(event, "Event-Date-Timestamp")
	if raw == "" {
		return time.Now().UTC()
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Now().UTC()
	}
	if value >= microsecondTimestampThreshold {
		value /= 1000
	}
	return time.UnixMilli(value).UTC()
}

func resolveEventID(fsAddr, eventName, sequence, uuid string, at time.Time) string {
	if eventName == "" {
		eventName = "UNKNOWN"
	}
	if sequence == "" {
		sequence = strconv.FormatInt(at.UnixMilli(), 10)
	}
	if uuid == "" {
		uuid = "no-uuid"
	}
	return fmt.Sprintf("%s:%s:%s:%s", fsAddr, eventName, sequence, uuid)
}

func resolveLegRole(event *eslgo.Event) contracts.LegRole {
	internalCall := firstNonBlank(get(event, "variable_sip_h_X-Internal-Call"), get(event, "sip_h_X-Internal-Call"))
	if strings.EqualFold(internalCall, "true") {
		return contracts.LegRoleAgent
	}
	destination := get(event, "Caller-Destination-Number")
	if destination != "" && len(destination) <= 6 {
		return contracts.LegRoleAgent
	}
	return contracts.LegRoleCustomer
}

func resolveBridgeID(raw, uuidA, uuidB string) string {
	if raw != "" {
		return raw
	}
	if uuidA == "" {
		uuidA = "missing-a"
	}
	if uuidB == "" {
		uuidB = "missing-b"
	}
	if uuidA <= uuidB {
		return uuidA + ":" + uuidB
	}
	return uuidB + ":" + uuidA
}

func resolveInviteFailureStatus(event *eslgo.Event) int {
	if status := parseInt(get(event, "variable_sip_invite_failure_status")); status > 0 {
		return status
	}
	raw := firstNonBlank(get(event, "variable_last_bridge_proto_specific_hangup_cause"), get(event, "variable_proto_specific_hangup_cause"))
	if raw == "" {
		return 0
	}
	idx := strings.LastIndex(raw, ":")
	if idx >= 0 {
		raw = raw[idx+1:]
	}
	return parseInt(raw)
}

func parseInt(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mergeHeaders(headers map[string]any, extra map[string]any) map[string]any {
	for key, value := range extra {
		if value == nil || value == "" || value == 0 {
			continue
		}
		headers[key] = value
	}
	return headers
}
