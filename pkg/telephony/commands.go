// Package telephony 提供 FreeSWITCH 电话命令和事件的构造工具。
//
// 用于 ESL 层与 CTI 层之间的命令下发和事件上报，确保命令和事件携带完整的
// 调用链路上下文（callId、uuid、fsAddr、role、profile）。
package telephony

import (
	"time"

	"yunshu/internal/contracts"
)

// NewCommand 构造一个电话命令结构，用于向 FreeSWITCH 下发操作指令。
// 常见命令包括：originate、uuid_answer、uuid_bridge、uuid_transfer 等。
// commandID 用于幂等追踪，callID/uuid 标识通话，role 标识呼叫侧/被叫侧。
func NewCommand(commandID, command, callID, uuid, fsAddr string, role contracts.LegRole, profile contracts.CallFlowProfile, payload map[string]any) contracts.TelephonyCommand {
	return contracts.TelephonyCommand{
		CommandID: commandID,
		Command:   command,
		CallID:    callID,
		UUID:      uuid,
		FSAddr:    fsAddr,
		LegRole:   role,
		Profile:   profile,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
}

// NewEvent 构造一个电话事件结构，用于将 FreeSWITCH 事件适配上报给上层。
// 常见事件包括：CHANNEL_ANSWER、CHANNEL_HANGUP、CHANNEL_BRIDGE、DTMF 等。
// eventID 用于事件去重追踪，headers 携带 FreeSWITCH 原始事件头信息。
func NewEvent(eventID, eventName, callID, uuid, fsAddr string, role contracts.LegRole, profile contracts.CallFlowProfile, headers map[string]any) contracts.TelephonyEvent {
	return contracts.TelephonyEvent{
		EventID:   eventID,
		EventName: eventName,
		CallID:    callID,
		UUID:      uuid,
		FSAddr:    fsAddr,
		LegRole:   role,
		Profile:   profile,
		At:        time.Now().UTC(),
		Headers:   headers,
	}
}
