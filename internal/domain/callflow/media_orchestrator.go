package callflow

// media_orchestrator 包实现呼叫早期媒体与补铃音的主动编排控制。
// 职责：
//  1. 跟踪媒体相位（无 → 载波早期媒体 → 补铃音 → 完成）
//  2. 强制执行 broadcastTime 定时器，到时自动截断补铃音播放
//  3. 区分 CHANNEL_PROGRESS（180 振铃）与 CHANNEL_PROGRESS_MEDIA（183 早期媒体）
//  4. 在通话释放时安全清理定时器，防止 goroutine 泄漏

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	"yunshu/pkg/telephony"
)

// MediaPhase 表示通话在桥接前所处的媒体相位。
type MediaPhase int

const (
	MediaPhaseNone             MediaPhase = iota // 无媒体活动
	MediaPhaseCarrierEarlyMedia                  // 运营商早期媒体（183 Session Progress + SDP）
	MediaPhaseSupplementRing                     // 应用侧补铃音播放中
	MediaPhaseComplete                           // 媒体阶段完成（应答或挂断）
)

func (p MediaPhase) String() string {
	switch p {
	case MediaPhaseNone:
		return "none"
	case MediaPhaseCarrierEarlyMedia:
		return "carrier_early_media"
	case MediaPhaseSupplementRing:
		return "supplement_ring"
	case MediaPhaseComplete:
		return "complete"
	default:
		return "unknown"
	}
}

// MediaOrchestrator 管理单个通话的早期媒体与补铃音编排。
// 每个需要补铃音或早期媒体控制的通话对应一个实例，通过 MediaRegistry 按 callID 管理。
type MediaOrchestrator struct {
	CallID         string
	CommandService *esl.CommandService
	SessionStore   esl.SessionStore
	Logger         *slog.Logger

	mu           sync.Mutex
	phase        MediaPhase
	broadcastMs  int64
	timer        *time.Timer
	timerStarted bool
	playing      bool
}

// MediaRegistry 按 callID 管理活跃的 MediaOrchestrator 实例。
// 线程安全，支持并发读写。
type MediaRegistry struct {
	mu            sync.Mutex
	orchestrators map[string]*MediaOrchestrator
}

// NewMediaRegistry 创建媒体编排注册表。
func NewMediaRegistry() *MediaRegistry {
	return &MediaRegistry{orchestrators: map[string]*MediaOrchestrator{}}
}

// GetOrCreate 获取或创建指定 callID 的媒体编排器。
func (r *MediaRegistry) GetOrCreate(callID string, cmdService *esl.CommandService, store esl.SessionStore, logger *slog.Logger) *MediaOrchestrator {
	r.mu.Lock()
	defer r.mu.Unlock()
	if o, ok := r.orchestrators[callID]; ok {
		return o
	}
	if logger == nil {
		logger = slog.Default()
	}
	o := &MediaOrchestrator{
		CallID:         callID,
		CommandService: cmdService,
		SessionStore:   store,
		Logger:         logger,
	}
	r.orchestrators[callID] = o
	return o
}

// Get 获取指定 callID 的媒体编排器，不存在时返回 nil。
func (r *MediaRegistry) Get(callID string) *MediaOrchestrator {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.orchestrators[callID]
}

// Remove 停止编排器并移除注册。在通话挂断时调用，确保定时器清理。
func (r *MediaRegistry) Remove(callID string) {
	r.mu.Lock()
	o, ok := r.orchestrators[callID]
	if ok {
		delete(r.orchestrators, callID)
	}
	r.mu.Unlock()
	if ok && o != nil {
		o.Stop(context.Background())
	}
}

// ---------------------------------------------------------------------------
// MediaOrchestrator 方法
// ---------------------------------------------------------------------------

// SetBroadcastTime 设置补铃音最大播放时长（毫秒）。
// 必须在 Start 之前调用。
func (o *MediaOrchestrator) SetBroadcastTime(ms int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.broadcastMs = ms
}

// Phase 返回当前媒体相位。
func (o *MediaOrchestrator) Phase() MediaPhase {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.phase
}

// MarkCarrierEarlyMedia 标记运营商早期媒体（183 Session Progress + SDP）已到达。
// 此标记用于防止后续 CHANNEL_PROGRESS 再触发冗余补铃音播放。
func (o *MediaOrchestrator) MarkCarrierEarlyMedia() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.phase == MediaPhaseNone || o.phase == MediaPhaseCarrierEarlyMedia {
		o.phase = MediaPhaseCarrierEarlyMedia
	}
}

// HasCarrierEarlyMedia 返回是否已收到运营商早期媒体。
func (o *MediaOrchestrator) HasCarrierEarlyMedia() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.phase == MediaPhaseCarrierEarlyMedia || o.phase == MediaPhaseComplete
}

// Start 开始补铃音播放并启动 broadcastTime 定时器。
// targetUUID 和 fsAddr 分别标识播放目标通道和 FS 节点地址。
// profile 用于命令追踪。若 broadcastMs > 0，定时器将在到时后自动发送 break 截断播放。
func (o *MediaOrchestrator) Start(ctx context.Context, file, targetUUID, fsAddr string, broadcastMs int64, legRole contracts.LegRole, profile contracts.CallFlowProfile) error {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	o.mu.Lock()

	// 幂等：已在播放中则跳过
	if o.playing {
		o.mu.Unlock()
		return nil
	}
	// 运营商早期媒体已在播放，跳过应用侧补铃音
	if o.phase == MediaPhaseCarrierEarlyMedia {
		o.mu.Unlock()
		o.Logger.Info("运营商早期媒体已激活，跳过补铃音播放", "callId", o.CallID)
		return nil
	}

	o.playing = true
	o.phase = MediaPhaseSupplementRing
	o.broadcastMs = broadcastMs

	// 清除旧定时器
	if o.timer != nil {
		o.timer.Stop()
		o.timer = nil
		o.timerStarted = false
	}
	o.mu.Unlock()

	// 发送 playback 命令
	cmd := telephony.NewCommand(
		"playback:"+o.CallID+":supplement_ring",
		"playback",
		o.CallID,
		targetUUID,
		fsAddr,
		legRole,
		profile,
		map[string]any{
			"file": file,
			"both": "aleg",
		},
	)
	if err := o.CommandService.Execute(ctx, cmd); err != nil {
		o.mu.Lock()
		o.playing = false
		o.mu.Unlock()
		o.Logger.Error("发送补铃音命令失败", "callId", o.CallID, "error", err.Error())
		return err
	}

	o.Logger.Info("补铃音播放已启动", "callId", o.CallID, "file", file, "broadcastMs", broadcastMs)

	// 启动 broadcastTime 定时器
	if broadcastMs > 0 {
		o.mu.Lock()
		capturedCallID := o.CallID
		o.timer = time.AfterFunc(time.Duration(broadcastMs)*time.Millisecond, func() {
			o.Logger.Info("broadcastTime 定时器到期，自动截断补铃音", "callId", capturedCallID, "broadcastMs", broadcastMs)
			o.stopPlayback(context.Background(), targetUUID, fsAddr, legRole, profile)
		})
		o.timerStarted = true
		o.mu.Unlock()
	}

	return nil
}

// Stop 停止补铃音播放并取消 broadcastTime 定时器。
// 幂等：多次调用安全。
func (o *MediaOrchestrator) Stop(ctx context.Context) {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	o.mu.Lock()
	wasPlaying := o.playing
	o.playing = false
	o.phase = MediaPhaseComplete

	if o.timer != nil {
		o.timer.Stop()
		o.timer = nil
		o.timerStarted = false
	}
	o.mu.Unlock()

	if wasPlaying {
		o.Logger.Info("补铃音编排器已停止", "callId", o.CallID)
	}
}

// stopPlayback 内部方法：发送 break 命令停止播放，并清理定时器状态。
func (o *MediaOrchestrator) stopPlayback(ctx context.Context, targetUUID, fsAddr string, legRole contracts.LegRole, profile contracts.CallFlowProfile) {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	o.mu.Lock()
	o.playing = false
	if o.timer != nil {
		o.timer.Stop()
		o.timer = nil
		o.timerStarted = false
	}
	o.mu.Unlock()

	breakCmd := telephony.NewCommand(
		"break:"+o.CallID+":broadcast_timeout",
		"break",
		o.CallID,
		targetUUID,
		fsAddr,
		legRole,
		profile,
		map[string]any{},
	)
	if err := o.CommandService.Execute(ctx, breakCmd); err != nil {
		o.Logger.Error("broadcastTime 截断播放失败", "callId", o.CallID, "error", err.Error())
	} else {
		o.Logger.Info("broadcastTime 已截断补铃音", "callId", o.CallID)
	}

	// 更新会话元数据
	if o.SessionStore != nil {
		session, err := o.SessionStore.Get(ctx, o.CallID)
		if err == nil && session.Metadata != nil {
			session.Metadata["supplementRingPlaying"] = false
			session.Metadata["broadcastTimeExpired"] = true
			_ = o.SessionStore.Save(ctx, session)
		}
	}
}
