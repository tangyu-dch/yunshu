package esl

// lifecycle 包定义 ESL 通话生命周期状态机。
// 状态机管理单个通话从创建到结束的全过程，包括振铃、通话、桥接和挂断等状态。
// 事件来自 FreeSWITCH ESL 事件流，经过适配后驱动状态迁移。

import (
	"yunshu/internal/contracts"
	"yunshu/pkg/state"
)

// CallState 表示通话通道当前所处的工作阶段。
type CallState string

// CallEvent 触发通话状态迁移的 FreeSWITCH 事件名称。
type CallEvent string

// ESL 通话状态常量，定义通话通道从创建到释放的所有可能状态。
const (
	CallNew            CallState = "new"             // 通道刚创建，等待 FS 事件确认
	CallCreated        CallState = "created"         // 通道已创建，等待振铃或应答
	CallProgress       CallState = "progress"        // 通道收到振铃信号
	CallProgressMedia  CallState = "progress_media"  // 通道收到早期媒体（如彩铃）
	CallAnswered       CallState = "answered"        // 通道已被应答
	CallBridged        CallState = "bridged"         // 坐席腿与客户腿已桥接
	CallUnbridged      CallState = "unbridged"       // 桥接已解除
	CallHangup         CallState = "hangup"          // 收到挂断请求
	CallComplete       CallState = "complete"        // 通道完全释放，生命周期结束
	CallRecordFinished CallState = "record_finished" // 录音文件已生成
)

// ESL 通话事件常量，映射 FreeSWITCH ESL 事件名称。
const (
	EventChannelCreate         CallEvent = "CHANNEL_CREATE"          // 通道创建事件
	EventChannelProgress       CallEvent = "CHANNEL_PROGRESS"        // 通道振铃事件
	EventChannelProgressMedia  CallEvent = "CHANNEL_PROGRESS_MEDIA"  // 通道早期媒体事件
	EventChannelAnswer         CallEvent = "CHANNEL_ANSWER"          // 通道应答事件
	EventChannelBridge         CallEvent = "CHANNEL_BRIDGE"          // 通道桥接事件
	EventChannelUnbridge       CallEvent = "CHANNEL_UNBRIDGE"        // 通道解除桥接事件
	EventChannelHangup         CallEvent = "CHANNEL_HANGUP"          // 通道挂断请求事件
	EventChannelHangupComplete CallEvent = "CHANNEL_HANGUP_COMPLETE" // 通道完全释放事件
	EventRecordStop            CallEvent = "RECORD_STOP"             // 录音停止事件
)

// NewCallLifecycle 创建通话生命周期状态机实例。
// 状态转换规则涵盖通话正常流程和异常分支：
// - new -> created（FS 确认通道创建）或 -> complete（立即挂断）
// - created -> progress/progress_media/answered/hangup/complete
// - progress -> progress_media/answered/hangup/complete
// - progress_media -> answered/bridged/hangup/complete（可能在早期媒体阶段桥接）
// - answered -> bridged/hangup/complete/record_finished
// - bridged -> unbridged/hangup/complete/record_finished
// - unbridged -> hangup/complete/record_finished
// - hangup -> complete/record_finished（可能先完成录音再释放）
// - record_finished -> complete
func NewCallLifecycle(initial CallState) *state.Machine[CallState, CallEvent] {
	return state.NewMachine(initial, map[CallState]map[CallEvent]CallState{
		CallNew:            {EventChannelCreate: CallCreated, EventChannelHangupComplete: CallComplete},
		CallCreated:        {EventChannelCreate: CallCreated, EventChannelProgress: CallProgress, EventChannelProgressMedia: CallProgressMedia, EventChannelAnswer: CallAnswered, EventChannelHangup: CallHangup, EventChannelHangupComplete: CallComplete},
		CallProgress:       {EventChannelCreate: CallProgress, EventChannelProgressMedia: CallProgressMedia, EventChannelAnswer: CallAnswered, EventChannelHangup: CallHangup, EventChannelHangupComplete: CallComplete},
		CallProgressMedia:  {EventChannelCreate: CallProgressMedia, EventChannelAnswer: CallAnswered, EventChannelBridge: CallBridged, EventChannelHangup: CallHangup, EventChannelHangupComplete: CallComplete},
		CallAnswered:       {EventChannelCreate: CallAnswered, EventChannelProgress: CallProgress, EventChannelProgressMedia: CallProgressMedia, EventChannelAnswer: CallAnswered, EventChannelBridge: CallBridged, EventChannelHangup: CallHangup, EventChannelHangupComplete: CallComplete, EventRecordStop: CallRecordFinished},
		CallBridged:        {EventChannelCreate: CallBridged, EventChannelUnbridge: CallUnbridged, EventChannelHangup: CallHangup, EventChannelHangupComplete: CallComplete, EventRecordStop: CallRecordFinished, EventChannelAnswer: CallBridged},
		CallUnbridged:      {EventChannelHangup: CallHangup, EventChannelHangupComplete: CallComplete, EventRecordStop: CallRecordFinished},
		CallHangup:         {EventChannelHangupComplete: CallComplete, EventRecordStop: CallRecordFinished},
		CallRecordFinished: {EventChannelHangupComplete: CallComplete},
	})
}

// CommandValidator 校验 ESL 控制命令是否包含完整的追踪字段。
// 追踪字段用于日志关联和故障排查，缺少任一字段的命令将被拒绝执行。
type CommandValidator struct{}

// Validate 检查命令是否包含 CallID、UUID、FSAddr、LegRole 和 Command 等必要字段。
// 该校验在幂等检查之前执行，确保所有被记录的命令都是可追踪的。
func (CommandValidator) Validate(cmd contracts.TelephonyCommand) bool {
	return cmd.CommandID != "" && cmd.CallID != "" && cmd.UUID != "" && cmd.FSAddr != "" && cmd.LegRole != "" && cmd.Command != ""
}
