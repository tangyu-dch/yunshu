// Package fsesl 提供 FreeSWITCH ESL 命令构建和事件适配能力。
//
// 该包负责将领域层的 TelephonyCommand 转换为 FreeSWITCH ESL 可执行的 API/bgapi 命令，
// 并将 ESL 原始事件转换为领域 TelephonyEvent，隔离 FreeSWITCH 协议细节与业务逻辑。
package fsesl

import (
	"fmt"
	"strings"

	"yunshu/internal/contracts"
)

// BuildAPICommand 把领域命令转换为 FreeSWITCH API/bgapi 命令名称、参数和是否后台执行。
// 命令格式参考  DefaultEslCommandGateway，返回的命令名和参数可直接发送给 eslgo.Conn.SendCommand。
// 支持的命令包括：originate、bridge、hangup、break、dtmf、transfer、playback、stop-playback、audio 等。
func BuildAPICommand(cmd contracts.TelephonyCommand) (name string, args string, background bool) {
	switch cmd.Command {
	case "originate":
		return "originate", BuildOriginateArgs(cmd), true
	case "bridge":
		return "uuid_bridge", fmt.Sprintf("%v %v", value(cmd, "uuid1", cmd.UUID), value(cmd, "uuid2", "")), true
	case "hangup":
		reason := stringValue(cmd, "reasonCode", "NORMAL_CLEARING")
		return "uuid_kill", fmt.Sprintf("%s %s", cmd.UUID, reason), true
	case "break":
		return "uuid_break", cmd.UUID + " all", false
	case "dtmf":
		return "uuid_send_dtmf", fmt.Sprintf("%s %s", cmd.UUID, stringValue(cmd, "digits", "")), true
	case "transfer":
		return "transfer", fmt.Sprintf("%s %s %s %s", cmd.UUID, stringValue(cmd, "destination", ""), stringValue(cmd, "dialplan", "XML"), stringValue(cmd, "context", "default")), true
	case "playback":
		return "uuid_broadcast", fmt.Sprintf("%s %s %s", cmd.UUID, stringValue(cmd, "file", ""), stringValue(cmd, "both", "aleg")), true
	case "stop-playback":
		return "uuid_break", cmd.UUID + " all", false
	case "audio":
		option := stringValue(cmd, "option", "start")
		direction := stringValue(cmd, "direction", "both")
		return "uuid_audio", fmt.Sprintf("%s %s %s", cmd.UUID, option, direction), true
	case "audio-stream":
		return "uuid_audio_stream", fmt.Sprintf("%s %s %s", cmd.UUID, stringValue(cmd, "control", "start"), stringValue(cmd, "url", "")), true
	default:
		return cmd.Command, strings.TrimSpace(fmt.Sprint(cmd.Payload["args"])), true
	}
}

// BuildOriginateArgs 构造 originate 参数。
// API 外呼默认 AGENT_FIRST：先呼坐席/分机，后续由流程事件触发选号和客户腿。
func BuildOriginateArgs(cmd contracts.TelephonyCommand) string {
	options := optionsFromPayload(cmd)
	if _, ok := options["origination_uuid"]; !ok {
		options["origination_uuid"] = cmd.UUID
	}
	if _, ok := options["yunshu_call_id"]; !ok {
		options["yunshu_call_id"] = cmd.CallID
	}
	if _, ok := options["sip_h_X-S-C-I"]; !ok {
		options["sip_h_X-S-C-I"] = cmd.CallID
	}
	optionText := buildOptions(options)

	mode := contracts.OriginateMode(stringValue(cmd, "originateMode", string(contracts.OriginateModeCustomerFirst)))
	destination := stringValue(cmd, "destination", "")
	if destination == "" {
		destination = stringValue(cmd, "extension", "")
	}
	if destination == "" {
		destination = stringValue(cmd, "callee", "")
	}
	domainOrGateway := stringValue(cmd, "domainOrGateway", stringValue(cmd, "domain", "default"))
	if mode == contracts.OriginateModeAgentFirst {
		// 如果显式要求使用 user 协议，或者分机长度为 4~6 位且未指定外置网关 IP，则优先使用 user/ 协议以支持多端同振
		if boolValue(cmd, "useUserProtocol", false) || (len(destination) >= 4 && len(destination) <= 6 && !strings.Contains(domainOrGateway, ":") && domainOrGateway != "default") {
			return fmt.Sprintf("{%s}user/%s &park()", optionText, destination)
		}
		return fmt.Sprintf("{%s}sofia/external/%s@%s &park()", optionText, destination, domainOrGateway)
	}
	if boolValue(cmd, "register", true) {
		return fmt.Sprintf("{%s}sofia/gateway/%s/%s &park()", optionText, domainOrGateway, destination)
	}
	return fmt.Sprintf("{%s}sofia/external/%s@%s &park()", optionText, destination, domainOrGateway)
}

func optionsFromPayload(cmd contracts.TelephonyCommand) map[string]any {
	options := map[string]any{
		"hangup_after_bridge": "false",
		"bridge_early_media":  "true",
		"ignore_early_media":  "false",
	}
	if raw, ok := cmd.Payload["options"].(map[string]any); ok {
		for key, val := range raw {
			options[key] = val
		}
	}
	for key, val := range cmd.Payload {
		if strings.HasPrefix(key, "var_") {
			options[strings.TrimPrefix(key, "var_")] = val
		}
	}
	return options
}

func buildOptions(options map[string]any) string {
	parts := make([]string, 0, len(options))
	for key, val := range options {
		if key == "" || val == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", key, val))
	}
	return strings.Join(parts, ",")
}

func stringValue(cmd contracts.TelephonyCommand, key string, fallback string) string {
	if val, ok := cmd.Payload[key]; ok && val != nil {
		text := fmt.Sprint(val)
		if text != "" {
			return text
		}
	}
	return fallback
}

func value(cmd contracts.TelephonyCommand, key string, fallback string) any {
	if val, ok := cmd.Payload[key]; ok && val != nil {
		return val
	}
	return fallback
}

func boolValue(cmd contracts.TelephonyCommand, key string, fallback bool) bool {
	if val, ok := cmd.Payload[key]; ok && val != nil {
		switch typed := val.(type) {
		case bool:
			return typed
		case string:
			return strings.EqualFold(typed, "true")
		}
	}
	return fallback
}
