package contracts

import "time"

// CDRTask 是 ESL 在通话最终收口时发布给 CTI 的话单任务。
// 它对应  里的 `call_center_cdr_queue` 语义，后续由 CTI 做入库、计费、录音和回调。
type CDRTask struct {
	CallID       string          `json:"callId"`
	UUID         string          `json:"uuid"`
	FSAddr       string          `json:"fsAddr"`
	Profile      CallFlowProfile `json:"profile"`
	HangupCause  string          `json:"hangupCause,omitempty"`
	FinalState   string          `json:"finalState"`
	CompletedAt  time.Time       `json:"completedAt"`
	EventID      string          `json:"eventId"`
	EventVersion int             `json:"eventVersion"`
}
