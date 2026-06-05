// contracts 包定义了呼叫中心系统的对外契约，包括 HTTP API、Redis、MQ、错误码和共享类型。
// 所有跨服务通信都必须遵循本包定义的接口和数据结构，任何修改都是兼容性契约变更。
package contracts

import "time"

// LegRole 表示通话通道的角色身份，用于区分同一通呼叫中不同参与方的语义。
// customer=被叫方/客户，agent=坐席分机，ai=AI对话通道，unknown=角色未知或未确定。
type LegRole string

const (
	LegRoleCustomer LegRole = "customer" // 被叫方/客户通道
	LegRoleAgent    LegRole = "agent"    // 坐席分机通道
	LegRoleAI       LegRole = "ai"       // AI对话通道
	LegRoleUnknown  LegRole = "unknown"  // 未知角色
)

// CallFlowProfile 描述通话的业务流程类型，决定了 CTI 和 ESL 的协作模式。
// api_outbound=API外呼流程，batch_outbound=批量外呼流程，api_direct=拨号盘直呼流程，inbound=客户呼入流程。
type CallFlowProfile string

const (
	CallFlowAPIOutbound     CallFlowProfile = "api_outbound"     // API单次外呼流程
	CallFlowBatchOutbound   CallFlowProfile = "batch_outbound"   // 批量外呼任务流程
	CallFlowAPIDirect       CallFlowProfile = "api_direct"       // 拨号盘直呼流程
	CallFlowInbound         CallFlowProfile = "inbound"          // 客户呼入流程
	CallFlowBatchPredictive CallFlowProfile = "batch_predictive" // 预测批量外呼流程
	CallFlowBatchSynergy    CallFlowProfile = "batch_synergy"    // 协同批量外呼流程
)

// OriginateMode 定义了双通道通话的起呼顺序模式。
// AGENT_FIRST=先呼叫坐席再呼叫客户，CUSTOMER_FIRST=先呼叫客户再呼叫坐席。
type OriginateMode string

const (
	OriginateModeAgentFirst    OriginateMode = "AGENT_FIRST"    // 先呼叫坐席
	OriginateModeCustomerFirst OriginateMode = "CUSTOMER_FIRST" // 先呼叫客户
)

// TelephonyCommand 是 ES L发往 FreeSWITCH 的控制命令结构。
// CommandID 用于幂等去重，Command 是 FreeSWITCH 命令名称，
// CallID/UUID/FSAddr 标识目标通话和节点，LegRole 指定命令作用的通道角色，
// Profile 描述业务类型，Payload 携带命令参数，CreatedAt 记录命令发起时间。
type TelephonyCommand struct {
	CommandID string          `json:"commandId"`
	Command   string          `json:"command"`
	CallID    string          `json:"callId"`
	UUID      string          `json:"uuid"`
	FSAddr    string          `json:"fsAddr"`
	LegRole   LegRole         `json:"legRole"`
	Profile   CallFlowProfile `json:"profile"`
	Payload   map[string]any  `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

// TelephonyEvent 是 FreeSWITCH 上报的通话终端事件结构。
// EventID 是事件的唯一标识，EventName 是 FreeSWITCH 事件名称，
// CallID/UUID/FSAddr 标识所属通话和节点，LegRole 指示事件发生的通道角色，
// Profile 描述业务类型，At 是事件发生时间（UTC），Headers 包含 SIP 和通话相关元数据。
type TelephonyEvent struct {
	EventID   string          `json:"eventId"`
	EventName string          `json:"eventName"`
	CallID    string          `json:"callId"`
	UUID      string          `json:"uuid"`
	FSAddr    string          `json:"fsAddr"`
	LegRole   LegRole         `json:"legRole"`
	Profile   CallFlowProfile `json:"profile"`
	At        time.Time       `json:"at"`
	Headers   map[string]any  `json:"headers,omitempty"`
}
