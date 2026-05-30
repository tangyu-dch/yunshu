// Package logging 统一运行期日志字段。
//
// 通话命令、FS 事件、补偿任务和 HTTP 请求都必须使用这里的字段构造函数，
// 这样排障时才能用 callId、uuid、fsAddr、commandId 等稳定字段串起完整链路。
package logging

import (
	"log/slog"

	"yunshu/internal/contracts"
)

// TelephonyAttrs 返回通话控制命令的标准日志字段。
// 字段名保持英文，日志消息使用中文，避免日志平台索引频繁变化。
func TelephonyAttrs(cmd contracts.TelephonyCommand) []any {
	return []any{
		slog.String("callId", cmd.CallID),
		slog.String("uuid", cmd.UUID),
		slog.String("fsAddr", cmd.FSAddr),
		slog.String("legRole", string(cmd.LegRole)),
		slog.String("commandId", cmd.CommandID),
		slog.String("command", cmd.Command),
	}
}

// TelephonyEventAttrs 返回 FreeSWITCH 事件的标准日志字段。
func TelephonyEventAttrs(event contracts.TelephonyEvent) []any {
	return []any{
		slog.String("callId", event.CallID),
		slog.String("uuid", event.UUID),
		slog.String("fsAddr", event.FSAddr),
		slog.String("legRole", string(event.LegRole)),
		slog.String("eventId", event.EventID),
		slog.String("eventName", event.EventName),
	}
}

// HTTPAttrs 返回 HTTP 访问日志的标准字段。
// requestId 和 traceId 来自入口 header，后续需要继续透传到 MQ 和 workflow 事件。
func HTTPAttrs(method, path, requestID, traceID string, status int, duration string) []any {
	return []any{
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", status),
		slog.String("duration", duration),
		slog.String("requestId", requestID),
		slog.String("traceId", traceID),
	}
}

// WorkflowAttrs 返回流程编排日志的标准字段。
func WorkflowAttrs(workflowID, instanceID, state, event string) []any {
	return []any{
		slog.String("workflowId", workflowID),
		slog.String("workflowInstanceId", instanceID),
		slog.String("workflowState", state),
		slog.String("workflowEvent", event),
	}
}

// AllocationAttrs 返回 CTI 资源分配日志的标准字段。
func AllocationAttrs(commandID, callID, merchantID, skillGroup string) []any {
	return []any{
		slog.String("commandId", commandID),
		slog.String("callId", callID),
		slog.String("merchantId", merchantID),
		slog.String("skillGroup", skillGroup),
	}
}

// FSOwnershipAttrs 返回 FS 节点事件消费租约日志的标准字段。
func FSOwnershipAttrs(fsAddr, owner string) []any {
	return []any{
		slog.String("fsAddr", fsAddr),
		slog.String("owner", owner),
	}
}
