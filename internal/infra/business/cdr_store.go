// Package cdr 提供 CDR 最终收口的持久化适配器。
//
// ESL 在 hangup complete 后只写 CDR outbox；cc-worker 领取 outbox 后调用这里落库，
// 让计费、录音、报表和外部推送都可以基于已持久化的话单继续编排。
package business

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Record 是 Go-native CDR 收口记录。
type Record struct {
	CallID               string
	UUID                 string
	FSAddr               string
	Profile              string
	MerchantID           int
	UserID               int
	BatchTaskID          int
	BatchTelID           int
	Caller               string
	Callee               string
	DurationSec          int
	HangupCause          string
	FinalState           string
	RecordFile           string
	CompletedAt          time.Time
	EventID              string
	EventVersion         int
	OutboxID             string
	Extension            string
	SipHangupDisposition string
	RawPayload           map[string]any
}

// Store 定义 CDR 记录落库能力。
type CdrStore interface {
	SaveFromOutbox(ctx context.Context, entry Entry) error
}

// MemoryStore 是 CDR 内存仓储，用于测试和本地无数据库兜底。
type CdrMemoryStore struct {
	mu      sync.Mutex
	Records map[string]Record
}

// NewMemoryStore 创建内存 CDR 仓储。
func NewCdrMemoryStore() *CdrMemoryStore {
	return &CdrMemoryStore{Records: map[string]Record{}}
}

// SaveFromOutbox 幂等保存 CDR 记录。
func (s *CdrMemoryStore) SaveFromOutbox(_ context.Context, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := recordFromOutbox(entry)
	if record.CallID == "" {
		return fmt.Errorf("cdr missing callId")
	}
	s.Records[record.CallID] = record
	return nil
}

func recordFromOutbox(entry Entry) Record {
	payload := entry.Payload
	profile := stringValue(payload["profile"])

	var caller, callee string

	// 判断是否为批量外呼（包括预测、协同等）
	isBatchOutbound := profile == "batch_outbound" || profile == "batch_predictive" || profile == "batch_synergy"

	if isBatchOutbound {
		// 批量外呼是 Customer First：FS 起呼的第一条腿是客户，第二条腿是坐席。
		// 业务语义上：
		// - 主叫（Caller）应该是外呼所使用的显示号码（selectedCaller / callerNumber）或者坐席分机（extension）。
		// - 被叫（Callee）应该是客户手机号码（callee / calleeNumber）。
		callee = firstString(payload["calleeNumber"], payload["callee"], payload["variable_customer_number"])
		caller = firstString(payload["selectedCaller"], payload["callerNumber"], payload["variable_agent_extension"], payload["extension"])
	} else if profile == "inbound" {
		// 呼入流程：客户打进来。
		// 业务语义上：
		// - 主叫（Caller）应该是客户手机号码（callerNumber / callerDestination）。
		// - 被叫（Callee）应该是接入的分机或网关。
		caller = firstString(payload["callerNumber"], payload["callerDestination"])
		callee = firstString(payload["calleeNumber"], payload["callee"])
	} else {
		// 默认或 API 外呼、拨号盘直呼（Agent First）：
		// 第一条腿是坐席（主叫），第二条腿是客户（被叫）。
		// FreeSWITCH 原生的 callerDestination / callerNumber 能够对应。
		caller = firstString(payload["selectedCaller"], payload["callerNumber"], payload["extension"])
		if caller == "" {
			caller = firstString(payload["callerNumber"], payload["callerDestination"])
		}
		callee = firstString(payload["calleeNumber"], payload["callee"])
	}

	// 提取分机号
	extension := ""
	if val, ok := payload["extension"]; ok && val != nil && stringValue(val) != "" {
		extension = stringValue(val)
	} else if val, ok := payload["variable_agent_extension"]; ok && val != nil && stringValue(val) != "" {
		extension = stringValue(val)
	} else if val, ok := payload["agent_extension"]; ok && val != nil && stringValue(val) != "" {
		extension = stringValue(val)
	}

	// 提取挂断细节
	sipHangupDisposition := ""
	if val, ok := payload["sipHangupDisposition"]; ok && val != nil {
		sipHangupDisposition = stringValue(val)
	}

	return Record{
		CallID:               stringValue(payload["callId"]),
		UUID:                 stringValue(payload["uuid"]),
		FSAddr:               stringValue(payload["fsAddr"]),
		Profile:              profile,
		MerchantID:           intValue(payload["merchantId"]),
		UserID:               intValue(payload["userId"]),
		BatchTaskID:          intValue(payload["batchTaskId"]),
		BatchTelID:           intValue(payload["batchCallTelId"]),
		Caller:               caller,
		Callee:               callee,
		DurationSec:          intValue(payload["durationSec"]),
		HangupCause:          stringValue(payload["hangupCause"]),
		FinalState:           stringValue(payload["finalState"]),
		RecordFile:           stringValue(payload["recordFilePath"]),
		CompletedAt:          timeValue(payload["completedAt"]),
		EventID:              stringValue(payload["eventId"]),
		EventVersion:         intValue(payload["eventVersion"]),
		OutboxID:             entry.ID,
		Extension:            extension,
		SipHangupDisposition: sipHangupDisposition,
		RawPayload:           payload,
	}
}
