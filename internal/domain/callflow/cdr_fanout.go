package callflow

import (
	"fmt"
	"time"

	"yunshu/internal/domain/outbox"
)

const (
	DestinationCDRBilling          = "cti_cdr_billing"
	DestinationCDRRecording        = "cti_cdr_recording"
	DestinationCDRRecordingOSS     = "cti_cdr_recording_oss"
	DestinationCDRReportProjection = "cti_cdr_report_projection"
	DestinationCDRDownstreamPush   = "cti_cdr_downstream_push"
)

// BuildCDRFanoutEntries 根据已落库 CDR outbox 构造后续流程节点。
//
// CDR 是通话收口事实，后续计费、录音、报表和外部 CDR 推送都应该从这里继续，
// 不再回头读取易丢失的会话内存态。
func BuildCDRFanoutEntries(cdrEntry outbox.Entry, now time.Time) []outbox.Entry {
	callID := fmt.Sprint(cdrEntry.Payload["callId"])
	if callID == "" || callID == "<nil>" {
		callID = cdrEntry.AggregateID
	}
	entries := []outbox.Entry{
		cdrFanoutEntry(cdrEntry, callID, DestinationCDRBilling, "billing", now),
		cdrFanoutEntry(cdrEntry, callID, DestinationCDRRecording, "recording", now),
		cdrFanoutEntry(cdrEntry, callID, DestinationCDRReportProjection, "report", now),
		cdrFanoutEntry(cdrEntry, callID, DestinationCDRDownstreamPush, "downstream", now),
	}
	// 添加录音 OSS 上传任务
	entries = append(entries, cdrFanoutEntry(cdrEntry, callID, DestinationCDRRecordingOSS, "recording_oss", now))
	return entries
}

func cdrFanoutEntry(cdrEntry outbox.Entry, callID, destination, suffix string, now time.Time) outbox.Entry {
	payload := make(map[string]any, len(cdrEntry.Payload)+2)
	for key, value := range cdrEntry.Payload {
		payload[key] = value
	}
	payload["sourceOutboxId"] = cdrEntry.ID
	payload["eventType"] = "cdr_persisted"
	return outbox.Entry{
		ID:             "cdr:" + suffix + ":" + callID,
		AggregateType:  "call_cdr_record",
		AggregateID:    callID,
		Destination:    destination,
		IdempotencyKey: "cdr:" + suffix + ":" + callID,
		Payload:        payload,
		NextAttemptAt:  now.UTC(),
	}
}
