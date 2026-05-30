package contracts

const (
	// EventAPICallRequested 表示 CTI 已接收到 API 外呼请求，需要推进 CTI 外呼流程。
	EventAPICallRequested = "cti.api_call.requested"
	// EventBatchCallRequested 表示 CTI 已下发单个批量号码，需要推进批量外呼流程。
	EventBatchCallRequested = "cti.batch_call.requested"
	// EventBatchCallTelCompleted 表示批量外呼单个号码已完成，需要推进统计、推送和下一号码调度。
	EventBatchCallTelCompleted = "cti.batch_call.tel_completed"
	// EventBatchCallTaskCompleted 表示批量外呼任务已无待拨/拨打中号码，需要推进任务收口投影。
	EventBatchCallTaskCompleted = "cti.batch_call.task_completed"
	// EventESLCommandSent 表示 ESL 起呼命令已提交，需要推进 ESL 起呼流程。
	EventESLCommandSent = "esl.command.sent"
	// EventFSApplied 表示 FreeSWITCH 事件已被 ESL 会话服务处理，需要推进 ESL 事件流程。
	EventFSApplied = "esl.fs_event.applied"
)
