package cti

// workflows 包定义 CTI 业务工作流。
// 工作流编排外呼任务的执行路径，包括 API 外呼流程和批量外呼流程。
// 每个工作流定义初始状态、状态转换规则和业务处理逻辑。

import "yunshu/pkg/workflow"

// CTI 工作流标识符常量。
const (
	WorkflowAPIOutbound   = "cti_api_outbound"   // API 外呼工作流，处理单个外呼请求
	WorkflowBatchOutbound = "cti_batch_outbound" // 批量外呼工作流，处理批量任务
	WorkflowDialpadDirect = "cti_dialpad_direct" // 拨号盘直呼工作流，处理物理坐席直呼
	WorkflowInbound       = "cti_inbound"        // 客户呼入工作流，处理物理客户呼入
)

// WorkflowDefinitions 返回所有 CTI 业务工作流的定义。
// 工作流采用事件驱动模型，通过接收外部事件驱动状态迁移：
// - APIOutbound：从接收请求 -> 校验 -> 选号 -> 发起呼叫 -> 结束
// - BatchOutbound：从任务就绪 -> 获取槽位 -> 选号 -> 发起呼叫 -> 任务结束
func WorkflowDefinitions() []workflow.Definition {
	return []workflow.Definition{
		{
			ID:      WorkflowAPIOutbound,
			Initial: "received",
			Transitions: []workflow.Transition{
				{From: "received", On: "validate", To: "validated"},
				{From: "validated", On: "select_number", To: "number_selected"},
				{From: "validated", On: "selection_failed", To: "failed"},
				{From: "number_selected", On: "dispatch_originate", To: "originating"},
				{From: "originating", On: "terminal_event", To: "finished"},
				{From: "finished", On: "cdr_persisted", To: "cdr_finalized"},
				{From: "cdr_finalized", On: "billing_completed", To: "billing_done"},
				{From: "billing_done", On: "recording_completed", To: "recording_done"},
				{From: "recording_done", On: "push_completed", To: "ended"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{},
		},
		{
			ID:      WorkflowBatchOutbound,
			Initial: "task_ready",
			Transitions: []workflow.Transition{
				{From: "task_ready", On: "acquire_slot", To: "slot_acquired"},
				{From: "slot_acquired", On: "select_number", To: "number_selected"},
				{From: "slot_acquired", On: "selection_failed", To: "list_failed"},
				{From: "number_selected", On: "dispatch_originate", To: "originating"},
				{From: "originating", On: "terminal_event", To: "list_finished"},
				{From: "list_finished", On: "cdr_persisted", To: "cdr_finalized"},
				{From: "cdr_finalized", On: "billing_completed", To: "billing_done"},
				{From: "billing_done", On: "recording_completed", To: "recording_done"},
				{From: "recording_done", On: "callback_completed", To: "finished"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{},
		},
		{
			ID:      WorkflowDialpadDirect,
			Initial: "finished",
			Transitions: []workflow.Transition{
				{From: "finished", On: "cdr_persisted", To: "cdr_finalized"},
				{From: "cdr_finalized", On: "billing_completed", To: "billing_done"},
				{From: "billing_done", On: "recording_completed", To: "recording_done"},
				{From: "recording_done", On: "push_completed", To: "ended"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{},
		},
		{
			ID:      WorkflowInbound,
			Initial: "finished",
			Transitions: []workflow.Transition{
				{From: "finished", On: "cdr_persisted", To: "cdr_finalized"},
				{From: "cdr_finalized", On: "billing_completed", To: "billing_done"},
				{From: "billing_done", On: "recording_completed", To: "recording_done"},
				{From: "recording_done", On: "push_completed", To: "ended"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{},
		},
	}
}
