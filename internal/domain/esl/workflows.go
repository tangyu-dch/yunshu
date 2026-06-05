package esl

// workflows 包定义 ESL 命令执行工作流。
// 工作流编排 FreeSWITCH 命令的执行路径，包括 API 外呼命令流程和批量外呼命令流程。
// 每个工作流定义初始状态、状态转换规则和命令执行处理逻辑。

import (
	"context"
	"log/slog"
	"strconv"

	"yunshu/pkg/workflow"
)

// ESL 工作流标识符常量。
const (
	WorkflowESLAPIOutbound     = "esl_api_outbound"     // ESL API 外呼命令工作流，处理单次起呼命令
	WorkflowESLBatchOutbound   = "esl_batch_outbound"   // ESL 批量外呼命令工作流，处理批量起呼命令
	WorkflowESLDialpadDirect   = "esl_dialpad_direct"   // ESL 拨号盘直呼工作流
	WorkflowESLInbound         = "esl_inbound"          // ESL 客户呼入工作流
	WorkflowESLBatchPredictive = "esl_batch_predictive" // ESL 预测批量外呼工作流
	WorkflowESLBatchSynergy    = "esl_batch_synergy"    // ESL 协同批量外呼工作流
	stepCaptureMediaPhase      = "capture_media_phase"
)

// WorkflowDefinitions 返回所有 ESL 命令执行工作流的定义。
// 工作流采用事件驱动模型，通过接收 FreeSWITCH 事件驱动状态迁移：
// - ESLAPIOutbound：命令接收 -> 校验 -> 执行起呼 -> 等待通道事件 -> 通话结束
// - ESLBatchOutbound：命令接收 -> 校验 -> 执行起呼 -> 等待早期媒体/应答 -> 通话结束
// 状态转换由 FreeSWITCH 事件（CHANNEL_CREATE、CHANNEL_ANSWER、CHANNEL_HANGUP_COMPLETE 等）驱动。
func WorkflowDefinitions() []workflow.Definition {
	return []workflow.Definition{
		{
			ID:      WorkflowESLAPIOutbound,
			Initial: "command_received",
			Transitions: []workflow.Transition{
				{From: "command_received", On: "validate_command", To: "validated"},
				{From: "validated", On: "execute_originate", To: "originating"},
				{From: "originating", On: "CHANNEL_CREATE", To: "created"},
				{From: "created", On: "CHANNEL_PROGRESS", To: "progress"},
				{From: "created", On: "CHANNEL_PROGRESS_MEDIA", To: "ringback", Step: stepCaptureMediaPhase},
				{From: "progress", On: "CHANNEL_PROGRESS_MEDIA", To: "ringback", Step: stepCaptureMediaPhase},
				{From: "progress", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "progress", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "ringback", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "ringback", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "created", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "created", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "answered", On: "CHANNEL_PROGRESS", To: "progress"},
				{From: "answered", On: "CHANNEL_PROGRESS_MEDIA", To: "ringback", Step: stepCaptureMediaPhase},
				{From: "answered", On: "CHANNEL_BRIDGE", To: "bridged"},
				{From: "answered", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "bridged", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{
				stepCaptureMediaPhase: captureMediaPhaseHandler("api"),
			},
		},
		{
			ID:      WorkflowESLBatchOutbound,
			Initial: "command_received",
			Transitions: []workflow.Transition{
				{From: "command_received", On: "validate_command", To: "validated"},
				{From: "validated", On: "execute_originate", To: "originating"},
				{From: "originating", On: "CHANNEL_CREATE", To: "created"},
				{From: "created", On: "CHANNEL_PROGRESS", To: "progress"},
				{From: "created", On: "CHANNEL_PROGRESS_MEDIA", To: "early_media", Step: stepCaptureMediaPhase},
				{From: "progress", On: "CHANNEL_PROGRESS_MEDIA", To: "early_media", Step: stepCaptureMediaPhase},
				{From: "progress", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "progress", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "created", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "created", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "early_media", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "early_media", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "answered", On: "CHANNEL_PROGRESS", To: "progress"},
				{From: "answered", On: "CHANNEL_PROGRESS_MEDIA", To: "early_media", Step: stepCaptureMediaPhase},
				{From: "answered", On: "CHANNEL_BRIDGE", To: "bridged"},
				{From: "answered", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "bridged", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{
				stepCaptureMediaPhase: captureMediaPhaseHandler("batch"),
			},
		},
		{
			ID:      WorkflowESLBatchPredictive,
			Initial: "command_received",
			Transitions: []workflow.Transition{
				{From: "command_received", On: "validate_command", To: "validated"},
				{From: "validated", On: "execute_originate", To: "originating"},
				{From: "originating", On: "CHANNEL_CREATE", To: "created"},
				{From: "created", On: "CHANNEL_PROGRESS", To: "progress"},
				{From: "created", On: "CHANNEL_PROGRESS_MEDIA", To: "early_media", Step: stepCaptureMediaPhase},
				{From: "progress", On: "CHANNEL_PROGRESS_MEDIA", To: "early_media", Step: stepCaptureMediaPhase},
				{From: "progress", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "progress", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "created", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "created", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "early_media", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "early_media", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "answered", On: "CHANNEL_PROGRESS", To: "progress"},
				{From: "answered", On: "CHANNEL_PROGRESS_MEDIA", To: "early_media", Step: stepCaptureMediaPhase},
				{From: "answered", On: "CHANNEL_BRIDGE", To: "bridged"},
				{From: "answered", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "bridged", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{
				stepCaptureMediaPhase: captureMediaPhaseHandler("batch_predictive"),
			},
		},
		{
			ID:      WorkflowESLBatchSynergy,
			Initial: "command_received",
			Transitions: []workflow.Transition{
				{From: "command_received", On: "validate_command", To: "validated"},
				{From: "validated", On: "execute_originate", To: "originating"},
				{From: "originating", On: "CHANNEL_CREATE", To: "created"},
				{From: "created", On: "CHANNEL_PROGRESS", To: "progress"},
				{From: "created", On: "CHANNEL_PROGRESS_MEDIA", To: "early_media", Step: stepCaptureMediaPhase},
				{From: "progress", On: "CHANNEL_PROGRESS_MEDIA", To: "early_media", Step: stepCaptureMediaPhase},
				{From: "progress", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "progress", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "created", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "created", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "early_media", On: "CHANNEL_ANSWER", To: "answered"},
				{From: "early_media", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "answered", On: "CHANNEL_PROGRESS", To: "progress"},
				{From: "answered", On: "CHANNEL_PROGRESS_MEDIA", To: "early_media", Step: stepCaptureMediaPhase},
				{From: "answered", On: "CHANNEL_BRIDGE", To: "bridged"},
				{From: "answered", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "bridged", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{
				stepCaptureMediaPhase: captureMediaPhaseHandler("batch_synergy"),
			},
		},
		{
			ID:      WorkflowESLDialpadDirect,
			Initial: "init",
			Transitions: []workflow.Transition{
				{From: "init", On: "CHANNEL_CREATE", To: "agent_created"},
				{From: "agent_created", On: "CHANNEL_PROGRESS", To: "agent_progress"},
				{From: "agent_created", On: "CHANNEL_PROGRESS_MEDIA", To: "agent_progress_media"},
				{From: "agent_progress", On: "CHANNEL_PROGRESS_MEDIA", To: "agent_progress_media"},
				{From: "agent_created", On: "CHANNEL_ANSWER", To: "agent_answered"},
				{From: "agent_progress", On: "CHANNEL_ANSWER", To: "agent_answered"},
				{From: "agent_progress_media", On: "CHANNEL_ANSWER", To: "agent_answered"},
				{From: "agent_answered", On: "validate_command", To: "customer_validated"},
				{From: "customer_validated", On: "execute_originate", To: "customer_originating"},
				{From: "customer_originating", On: "CHANNEL_CREATE", To: "customer_created"},
				{From: "customer_created", On: "CHANNEL_PROGRESS", To: "customer_progress"},
				{From: "customer_created", On: "CHANNEL_PROGRESS_MEDIA", To: "customer_early_media", Step: stepCaptureMediaPhase},
				{From: "customer_progress", On: "CHANNEL_PROGRESS_MEDIA", To: "customer_early_media", Step: stepCaptureMediaPhase},
				{From: "customer_created", On: "CHANNEL_ANSWER", To: "customer_answered"},
				{From: "customer_progress", On: "CHANNEL_ANSWER", To: "customer_answered"},
				{From: "customer_early_media", On: "CHANNEL_ANSWER", To: "customer_answered"},
				{From: "customer_progress", On: "CHANNEL_BRIDGE", To: "bridged"},
				{From: "customer_early_media", On: "CHANNEL_BRIDGE", To: "bridged"},
				{From: "customer_answered", On: "CHANNEL_BRIDGE", To: "bridged"},
				{From: "bridged", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				// 异常挂断分支
				{From: "agent_created", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "agent_progress", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "agent_progress_media", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "agent_answered", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "customer_originating", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "customer_created", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "customer_progress", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "customer_early_media", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "customer_answered", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{
				stepCaptureMediaPhase: captureMediaPhaseHandler("api_direct"),
			},
		},
		{
			ID:      WorkflowESLInbound,
			Initial: "init",
			Transitions: []workflow.Transition{
				{From: "init", On: "CHANNEL_CREATE", To: "customer_created"},
				{From: "customer_created", On: "CHANNEL_PROGRESS", To: "customer_progress"},
				{From: "customer_created", On: "CHANNEL_PROGRESS_MEDIA", To: "customer_early_media", Step: stepCaptureMediaPhase},
				{From: "customer_progress", On: "CHANNEL_PROGRESS_MEDIA", To: "customer_early_media", Step: stepCaptureMediaPhase},
				{From: "customer_created", On: "CHANNEL_ANSWER", To: "customer_answered"},
				{From: "customer_progress", On: "CHANNEL_ANSWER", To: "customer_answered"},
				{From: "customer_early_media", On: "CHANNEL_ANSWER", To: "customer_answered"},
				{From: "customer_created", On: "validate_command", To: "agent_validated"},
				{From: "customer_progress", On: "validate_command", To: "agent_validated"},
				{From: "customer_early_media", On: "validate_command", To: "agent_validated"},
				{From: "customer_answered", On: "validate_command", To: "agent_validated"},
				{From: "agent_validated", On: "execute_originate", To: "agent_originating"},
				{From: "agent_originating", On: "CHANNEL_CREATE", To: "agent_created"},
				{From: "agent_created", On: "CHANNEL_PROGRESS", To: "agent_progress"},
				{From: "agent_created", On: "CHANNEL_PROGRESS_MEDIA", To: "agent_early_media"},
				{From: "agent_progress", On: "CHANNEL_PROGRESS_MEDIA", To: "agent_early_media"},
				{From: "agent_created", On: "CHANNEL_ANSWER", To: "agent_answered"},
				{From: "agent_progress", On: "CHANNEL_ANSWER", To: "agent_answered"},
				{From: "agent_early_media", On: "CHANNEL_ANSWER", To: "agent_answered"},
				{From: "agent_answered", On: "CHANNEL_BRIDGE", To: "bridged"},
				{From: "bridged", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				// 异常挂断分支
				{From: "customer_created", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "customer_progress", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "customer_early_media", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "customer_answered", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "agent_originating", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "agent_created", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "agent_progress", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "agent_early_media", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
				{From: "agent_answered", On: "CHANNEL_HANGUP_COMPLETE", To: "complete"},
			},
			Handlers: map[workflow.StepName]workflow.Handler{
				stepCaptureMediaPhase: captureMediaPhaseHandler("inbound"),
			},
		},
	}
}

func captureMediaPhaseHandler(profile string) workflow.Handler {
	return func(_ context.Context, instance *workflow.Instance, event workflow.Event) error {
		if instance.Variables == nil {
			instance.Variables = map[string]any{}
		}
		instance.Variables["mediaPhase"] = "ringback"
		instance.Variables["workflowProfile"] = profile
		if value := stringFromPayload(event.Payload, "playbackFile"); value != "" {
			instance.Variables["playbackFile"] = value
		}
		if value := stringFromPayload(event.Payload, "supplementRingFile"); value != "" {
			instance.Variables["supplementRingFile"] = value
		}
		if value, ok := boolFromPayload(event.Payload, "supplementRing"); ok {
			instance.Variables["supplementRing"] = value
		}
		if value, ok := int64FromPayload(event.Payload, "broadcastTime"); ok && value > 0 {
			instance.Variables["broadcastTime"] = value
		}
		slog.Info("ESL 振铃/早期媒体步骤已捕获", "workflowId", instance.WorkflowID, "instanceId", instance.ID, "profile", profile, "mediaPhase", instance.Variables["mediaPhase"], "playbackFile", instance.Variables["playbackFile"])
		return nil
	}
}

func stringFromPayload(payload map[string]any, key string) string {
	if value, ok := payload[key]; ok && value != nil {
		switch typed := value.(type) {
		case string:
			return typed
		default:
			return ""
		}
	}
	return ""
}

func boolFromPayload(payload map[string]any, key string) (bool, bool) {
	value, ok := payload[key]
	if !ok || value == nil {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		return typed == "true" || typed == "1" || typed == "yes", true
	default:
		return false, false
	}
}

func int64FromPayload(payload map[string]any, key string) (int64, bool) {
	value, ok := payload[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}
