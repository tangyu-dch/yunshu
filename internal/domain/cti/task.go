package cti

// task 包定义 CTI 任务状态机和相关类型。
// 任务状态机管理外呼任务的生命周期，包括创建、确认、运行、暂停、结算和完成等状态转换。

import "yunshu/pkg/state"

// TaskState 表示 CTI 任务所处的工作阶段。
type TaskState string

// TaskEvent 触发任务状态转换的事件。
type TaskEvent string

// CTI 任务状态常量定义任务从创建到结束的所有可能状态。
const (
	TaskCreated   TaskState = "created"   // 任务已创建，等待资源分配
	TaskConfirmed TaskState = "confirmed" // 任务已确认，资源就绪
	TaskRunning   TaskState = "running"   // 任务正在执行外呼
	TaskPaused    TaskState = "paused"    // 任务因资源不足等原因暂停
	TaskSettling  TaskState = "settling"  // 任务进入结算流程
	TaskFinished  TaskState = "finished"  // 任务正常完成
	TaskFailed    TaskState = "failed"    // 任务执行失败
)

// CTI 任务事件常量定义所有可能导致状态迁移的事件。
const (
	EventConfirm TaskEvent = "confirm" // 确认任务，可以开始执行
	EventStart   TaskEvent = "start"   // 开始执行任务
	EventPause   TaskEvent = "pause"   // 暂停任务
	EventResume  TaskEvent = "resume"  // 恢复暂停的任务
	EventSettle  TaskEvent = "settle"  // 进入结算流程
	EventFinish  TaskEvent = "finish"  // 任务正常结束
	EventFail    TaskEvent = "fail"    // 任务执行失败
)

// NewTaskStateMachine 创建 CTI 任务状态机实例。
// 状态转换规则：
// - created -> confirmed（确认后可执行），-> failed（失败则终止）
// - confirmed -> running（启动执行），-> failed（失败则终止）
// - running -> paused（暂停），-> settling（进入结算），-> failed（失败则终止）
// - paused -> running（恢复），-> settling（进入结算），-> failed（失败则终止）
// - settling -> finished（完成结算），-> failed（结算失败）
func NewTaskStateMachine(initial TaskState) *state.Machine[TaskState, TaskEvent] {
	return state.NewMachine(initial, map[TaskState]map[TaskEvent]TaskState{
		TaskCreated:   {EventConfirm: TaskConfirmed, EventFail: TaskFailed},
		TaskConfirmed: {EventStart: TaskRunning, EventFail: TaskFailed},
		TaskRunning:   {EventPause: TaskPaused, EventSettle: TaskSettling, EventFail: TaskFailed},
		TaskPaused:    {EventResume: TaskRunning, EventSettle: TaskSettling, EventFail: TaskFailed},
		TaskSettling:  {EventFinish: TaskFinished, EventFail: TaskFailed},
	})
}
