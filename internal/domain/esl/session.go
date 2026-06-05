package esl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/outbox"
	"yunshu/internal/infra/events"
	"yunshu/internal/infra/logging"
)

var (
	ErrSessionNotFound = errors.New("call session not found")
	ErrDuplicateEvent  = errors.New("duplicate freeswitch event")
)

// CallSession 保存 ESL 侧通话运行态。
// 生产环境需要持久化到 Redis/DB，内存实现只用于本地开发和单元测试。
type CallSession struct {
	CallID      string                       `json:"callId"`
	Profile     contracts.CallFlowProfile    `json:"profile"`
	State       CallState                    `json:"state"`
	UUIDs       map[string]contracts.LegRole `json:"uuids"`
	FSAddr      string                       `json:"fsAddr"`
	Metadata    map[string]any               `json:"metadata,omitempty"`
	LastEventID string                       `json:"lastEventId,omitempty"`
	CreatedAt   time.Time                    `json:"createdAt"`
	UpdatedAt   time.Time                    `json:"updatedAt"`
	CompletedAt time.Time                    `json:"completedAt,omitempty"`
}

// SessionStore 定义通话会话存储能力。
type SessionStore interface {
	Save(ctx context.Context, session CallSession) error
	Get(ctx context.Context, callID string) (CallSession, error)
	CountActive(ctx context.Context) (int, error)
}

// MemorySessionStore 是测试和本地开发使用的会话存储。
type MemorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]CallSession
}

// NewMemorySessionStore 创建内存通话会话存储。
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: map[string]CallSession{}}
}

// Save 保存或覆盖通话会话。
func (s *MemorySessionStore) Save(_ context.Context, session CallSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.CallID] = session
	return nil
}

// Get 读取通话会话。
func (s *MemorySessionStore) Get(_ context.Context, callID string) (CallSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[callID]
	if !ok {
		return CallSession{}, ErrSessionNotFound
	}
	return session, nil
}

// CountActive 统计当前处于活动状态的会话数。
func (s *MemorySessionStore) CountActive(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, sess := range s.sessions {
		if sess.CompletedAt.IsZero() && sess.State != CallComplete {
			count++
		}
	}
	return count, nil
}

// SessionSniffer 用于在物理起呼事件中识别分机与 DID。
type SessionSniffer interface {
	// IsExtension 判断该号码是否为已注册的坐席分机。
	IsExtension(ctx context.Context, number string) (bool, *Extension, error)
	// IsMerchantDID 判断该号码是否为已注册的商户公网呼入 DID，并返回商户 ID。
	IsMerchantDID(ctx context.Context, number string) (bool, int, error)
}

// SessionService 处理 ESL 通话会话和 FreeSWITCH 事件。
type SessionService struct {
	Store   SessionStore
	Outbox  outbox.Store
	Events  events.Bus
	Sniffer SessionSniffer
	Now     func() time.Time
	Logger  *slog.Logger
}

// NewSessionService 创建通话会话服务。
func NewSessionService(store SessionStore, outboxStore outbox.Store, logger *slog.Logger) *SessionService {
	if logger == nil {
		logger = slog.Default()
	}
	return &SessionService{Store: store, Outbox: outboxStore, Now: time.Now, Logger: logger}
}

// CreateFromOriginate 在起呼命令提交后创建通话会话。
func (s *SessionService) CreateFromOriginate(ctx context.Context, cmd contracts.TelephonyCommand) error {
	now := s.Now().UTC()
	session := CallSession{
		CallID:    cmd.CallID,
		Profile:   cmd.Profile,
		State:     CallNew,
		UUIDs:     map[string]contracts.LegRole{cmd.UUID: cmd.LegRole},
		FSAddr:    cmd.FSAddr,
		Metadata:  sessionMetadata(cmd),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing, err := s.Store.Get(ctx, cmd.CallID); err == nil {
		session = existing
		if session.UUIDs == nil {
			session.UUIDs = map[string]contracts.LegRole{}
		}
		if session.Metadata == nil {
			session.Metadata = map[string]any{}
		}
		session.Profile = cmd.Profile
		session.FSAddr = cmd.FSAddr
		session.State = existing.State
		session.UpdatedAt = now
	} else if !errors.Is(err, ErrSessionNotFound) {
		return err
	}
	for key, value := range sessionMetadata(cmd) {
		if session.Metadata == nil {
			session.Metadata = map[string]any{}
		}
		session.Metadata[key] = value
	}
	session.UUIDs[cmd.UUID] = cmd.LegRole
	s.Logger.Info("创建 ESL 通话会话", logging.TelephonyAttrs(cmd)...)
	return s.Store.Save(ctx, session)
}

// ApplyEvent 应用 FreeSWITCH 事件并在最终收口时写入 CDR outbox。
func (s *SessionService) ApplyEvent(ctx context.Context, event contracts.TelephonyEvent) (CallSession, error) {
	attrs := logging.TelephonyEventAttrs(event)
	session, err := s.Store.Get(ctx, event.CallID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) && event.EventName == string(EventChannelCreate) && s.Sniffer != nil {
			callerNumber, _ := event.Headers["callerNumber"].(string)
			if callerNumber == "" {
				callerNumber, _ = event.Headers["variable_caller_id_number"].(string)
			}
			calleeNumber, _ := event.Headers["calleeNumber"].(string)
			if calleeNumber == "" {
				calleeNumber, _ = event.Headers["variable_callee_id_number"].(string)
			}
			if calleeNumber == "" {
				calleeNumber, _ = event.Headers["callerDestination"].(string)
			}

			// Sniff caller for extension (api_direct)
			isExt, ext, serr := s.Sniffer.IsExtension(ctx, callerNumber)
			if serr == nil && isExt && ext != nil {
				// Initialize as api_direct (坐席直呼)
				now := s.Now().UTC()
				session = CallSession{
					CallID:  event.CallID,
					Profile: contracts.CallFlowAPIDirect,
					State:   CallNew,
					UUIDs:   map[string]contracts.LegRole{event.UUID: contracts.LegRoleAgent},
					FSAddr:  event.FSAddr,
					Metadata: map[string]any{
						"userId":     ext.UserID,
						"merchantId": ext.MerchantID,
						"extension":  ext.ExtensionNumber,
						"agentUuid":  event.UUID,
						"caller":     callerNumber,
						"callee":     calleeNumber,
					},
					CreatedAt: now,
					UpdatedAt: now,
				}
				s.Logger.Info("自动捕获坐席直呼会话(api_direct)", "callId", event.CallID, "extension", ext.ExtensionNumber, "merchantId", ext.MerchantID)
				if err := s.Store.Save(ctx, session); err != nil {
					return CallSession{}, err
				}
				err = nil // clear error so it proceeds
			} else {
				// Sniff callee for DID (inbound)
				isDID, merchantID, serr := s.Sniffer.IsMerchantDID(ctx, calleeNumber)
				if serr == nil && isDID {
					// Initialize as inbound (客户呼入)
					now := s.Now().UTC()
					session = CallSession{
						CallID:  event.CallID,
						Profile: contracts.CallFlowInbound,
						State:   CallNew,
						UUIDs:   map[string]contracts.LegRole{event.UUID: contracts.LegRoleCustomer},
						FSAddr:  event.FSAddr,
						Metadata: map[string]any{
							"merchantId":   merchantID,
							"customerUuid": event.UUID,
							"caller":       callerNumber,
							"callee":       calleeNumber,
						},
						CreatedAt: now,
						UpdatedAt: now,
					}
					s.Logger.Info("自动捕获客户呼入会话(inbound)", "callId", event.CallID, "did", calleeNumber, "merchantId", merchantID)
					if err := s.Store.Save(ctx, session); err != nil {
						return CallSession{}, err
					}
					err = nil // clear error so it proceeds
				}
			}
		}

		if err != nil {
			s.Logger.Error("处理 FS 事件失败，通话会话不存在", append(attrs, slog.String("error", err.Error()))...)
			return CallSession{}, err
		}
	}
	if session.LastEventID == event.EventID {
		s.Logger.Info("跳过重复 FS 事件", attrs...)
		return session, ErrDuplicateEvent
	}
	machine := NewCallLifecycle(session.State)
	next, err := machine.Apply(CallEvent(event.EventName))
	if err != nil {
		s.Logger.Warn("FS 事件状态迁移失败", append(attrs, slog.String("state", string(session.State)), slog.String("error", err.Error()))...)
		return session, err
	}
	session.State = next
	session.LastEventID = event.EventID
	session.UpdatedAt = s.Now().UTC()
	if event.UUID != "" {
		session.UUIDs[event.UUID] = event.LegRole
	}
	if event.FSAddr != "" {
		session.FSAddr = event.FSAddr
	}
	if next == CallComplete {
		session.CompletedAt = session.UpdatedAt
		if err := s.appendCDRTask(ctx, session, event); err != nil {
			s.Logger.Error("FS 事件最终收口写入 CDR outbox 失败", append(attrs, slog.String("error", err.Error()))...)
			return session, err
		}
	}
	if err := s.Store.Save(ctx, session); err != nil {
		s.Logger.Error("保存 ESL 通话会话失败", append(attrs, slog.String("error", err.Error()))...)
		return session, err
	}
	if s.Events != nil {
		payload := map[string]any{"callId": event.CallID, "eventName": event.EventName, "uuid": event.UUID, "fsAddr": event.FSAddr, "profile": string(session.Profile)}
		if event.LegRole != "" {
			payload["legRole"] = string(event.LegRole)
		}
		for key, value := range session.Metadata {
			payload[key] = value
		}
		for _, key := range []string{"playbackFile", "supplementRing", "supplementRingFile", "broadcastTime", "broadcastTimeFlag"} {
			if value, ok := event.Headers[key]; ok {
				payload[key] = value
			}
		}
		if err := s.Events.Publish(ctx, contracts.NewEventEnvelope(
			"fs-event-applied:"+event.EventID,
			contracts.EventFSApplied,
			event.EventID,
			"call",
			event.CallID,
			contracts.ServiceCall,
			payload,
		)); err != nil {
			s.Logger.Error("FS 事件消费后发布流程事件失败", append(attrs, slog.String("error", err.Error()))...)
			return session, err
		}
	}
	s.Logger.Info("FS 事件处理完成", append(attrs, slog.String("state", string(session.State)))...)
	return session, nil
}
func sessionMetadata(cmd contracts.TelephonyCommand) map[string]any {
	metadata := map[string]any{}
	for _, key := range []string{"batchTaskId", "batchCallTelId", "userId", "merchantId", "callee", "routeVersion", "agentUuid", "customerUuid", "extension", "callMode", "callRatio", "queueEnable"} {
		if value, ok := cmd.Payload[key]; ok {
			metadata[key] = value
		}
	}
	if options, ok := cmd.Payload["options"].(map[string]any); ok {
		if value, ok := options["ringback"]; ok {
			metadata["playbackFile"] = value
		}
		if value, ok := options["variable_yunshu_ringback_file"]; ok {
			metadata["supplementRingFile"] = value
		}
		if value, ok := options["variable_yunshu_supplement_ring"]; ok {
			metadata["supplementRing"] = value
		}
		if value, ok := options["variable_yunshu_broadcast_time"]; ok {
			metadata["broadcastTime"] = value
		}
		if value, ok := options["variable_yunshu_broadcast_time_flag"]; ok {
			metadata["broadcastTimeFlag"] = value
		}
	}
	return metadata
}

func (s *SessionService) appendCDRTask(ctx context.Context, session CallSession, event contracts.TelephonyEvent) error {
	task := contracts.CDRTask{
		CallID:       session.CallID,
		UUID:         event.UUID,
		FSAddr:       session.FSAddr,
		Profile:      session.Profile,
		FinalState:   string(session.State),
		CompletedAt:  session.CompletedAt,
		EventID:      event.EventID,
		EventVersion: 1,
	}
	if customCause, ok := event.Headers["customHangupCause"].(string); ok && customCause != "" {
		task.HangupCause = customCause
	} else if cause, ok := event.Headers["hangupCause"].(string); ok {
		task.HangupCause = cause
	}
	payload := map[string]any{
		"callId":       task.CallID,
		"uuid":         task.UUID,
		"fsAddr":       task.FSAddr,
		"profile":      task.Profile,
		"hangupCause":  task.HangupCause,
		"finalState":   task.FinalState,
		"completedAt":  task.CompletedAt,
		"eventId":      task.EventID,
		"eventVersion": task.EventVersion,
	}
	for key, value := range session.Metadata {
		payload[key] = value
	}
	for _, key := range []string{
		"recordFilePath", "callerNumber", "calleeNumber", "callerDestination",
		"duration", "billsec", "variable_duration", "variable_billsec",
		"sipHangupDisposition",
	} {
		if value, ok := event.Headers[key]; ok {
			payload[key] = value
		}
	}

	// Calculate and populate durationSec
	durationSec := 0
	if val, ok := event.Headers["variable_billsec"]; ok {
		durationSec = intValue(val)
	} else if val, ok := event.Headers["billsec"]; ok {
		durationSec = intValue(val)
	} else if val, ok := event.Headers["variable_duration"]; ok {
		durationSec = intValue(val)
	} else if val, ok := event.Headers["duration"]; ok {
		durationSec = intValue(val)
	} else if !session.CompletedAt.IsZero() && !session.CreatedAt.IsZero() {
		durationSec = int(session.CompletedAt.Sub(session.CreatedAt).Seconds())
	}
	payload["durationSec"] = durationSec

	return s.Outbox.Append(ctx, outbox.Entry{
		ID:             "cdr:" + session.CallID,
		AggregateType:  "call",
		AggregateID:    session.CallID,
		Destination:    "call_center_cdr_queue",
		IdempotencyKey: "cdr:" + session.CallID,
		Payload:        payload,
		NextAttemptAt:  s.Now().UTC(),
	})
}

func intValue(value any) int {
	if value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var parsed int
		_, _ = fmt.Sscanf(typed, "%d", &parsed)
		return parsed
	default:
		return 0
	}
}
