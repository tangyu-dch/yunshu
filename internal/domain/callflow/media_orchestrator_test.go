package callflow

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	"yunshu/pkg/idempotency"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

func newTestCommandService() *esl.CommandService {
	return esl.NewCommandService(idempotency.NewMemoryStore(), &esl.MemoryCommandExecutor{}, nil)
}

func newTestSessionStore() esl.SessionStore {
	return esl.NewMemorySessionStore()
}

func TestMediaOrchestrator_StartAndStop(t *testing.T) {
	ctx := context.Background()
	cmdService := newTestCommandService()
	store := newTestSessionStore()

	// 预创建会话，供 stopPlayback 回写
	_ = store.Save(ctx, esl.CallSession{CallID: "call-1", Metadata: map[string]any{"supplementRingPlaying": true}})

	orch := &MediaOrchestrator{
		CallID:         "call-1",
		CommandService: cmdService,
		SessionStore:   store,
		Logger:         testLogger,
	}

	// 启动补铃音（不设 broadcastTime）
	err := orch.Start(ctx, "ring.wav", "uuid-agent", "fs-addr", 0, contracts.LegRoleAgent, contracts.CallFlowAPIOutbound)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if orch.Phase() != MediaPhaseSupplementRing {
		t.Errorf("expected phase SupplementRing, got %v", orch.Phase())
	}

	// 幂等：再次 Start 不应报错
	err = orch.Start(ctx, "ring.wav", "uuid-agent", "fs-addr", 0, contracts.LegRoleAgent, contracts.CallFlowAPIOutbound)
	if err != nil {
		t.Fatalf("Idempotent Start failed: %v", err)
	}

	// 停止
	orch.Stop(ctx)
	if orch.Phase() != MediaPhaseComplete {
		t.Errorf("expected phase Complete after Stop, got %v", orch.Phase())
	}

	// 幂等 Stop
	orch.Stop(ctx)
	if orch.Phase() != MediaPhaseComplete {
		t.Errorf("expected phase Complete after double Stop, got %v", orch.Phase())
	}
}

func TestMediaOrchestrator_BroadcastTimeTimer(t *testing.T) {
	ctx := context.Background()
	cmdService := newTestCommandService()
	store := newTestSessionStore()

	_ = store.Save(ctx, esl.CallSession{CallID: "call-timer", Metadata: map[string]any{"supplementRingPlaying": true}})

	orch := &MediaOrchestrator{
		CallID:         "call-timer",
		CommandService: cmdService,
		SessionStore:   store,
		Logger:         testLogger,
	}

	// 启动补铃音，broadcastTime = 100ms
	err := orch.Start(ctx, "ring.wav", "uuid-agent", "fs-addr", 100, contracts.LegRoleAgent, contracts.CallFlowAPIOutbound)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if orch.Phase() != MediaPhaseSupplementRing {
		t.Errorf("expected SupplementRing, got %v", orch.Phase())
	}

	// 等待定时器到期（100ms + 余量）
	time.Sleep(250 * time.Millisecond)

	// 定时器应已自动截断播放
	orch.mu.Lock()
	playing := orch.playing
	timerStarted := orch.timerStarted
	orch.mu.Unlock()

	if playing {
		t.Error("expected playing=false after broadcastTime expired")
	}
	if timerStarted {
		t.Error("expected timerStarted=false after broadcastTime expired")
	}

	// 会话元数据应反映截断
	session, _ := store.Get(ctx, "call-timer")
	if val, _ := session.Metadata["supplementRingPlaying"].(bool); val {
		t.Error("expected supplementRingPlaying=false in metadata after timer")
	}
	if val, _ := session.Metadata["broadcastTimeExpired"].(bool); !val {
		t.Error("expected broadcastTimeExpired=true in metadata")
	}
}

func TestMediaOrchestrator_CarrierEarlyMediaPreventsRing(t *testing.T) {
	ctx := context.Background()
	cmdService := newTestCommandService()
	store := newTestSessionStore()

	orch := &MediaOrchestrator{
		CallID:         "call-em",
		CommandService: cmdService,
		SessionStore:   store,
		Logger:         testLogger,
	}

	// 运营商早期媒体先到
	orch.MarkCarrierEarlyMedia()
	if orch.Phase() != MediaPhaseCarrierEarlyMedia {
		t.Errorf("expected CarrierEarlyMedia, got %v", orch.Phase())
	}
	if !orch.HasCarrierEarlyMedia() {
		t.Error("expected HasCarrierEarlyMedia=true")
	}

	// 补铃音应被跳过
	err := orch.Start(ctx, "ring.wav", "uuid-agent", "fs-addr", 0, contracts.LegRoleAgent, contracts.CallFlowAPIOutbound)
	if err != nil {
		t.Fatalf("Start should not error when early media active: %v", err)
	}

	// 不应进入 SupplementRing 相位
	if orch.Phase() != MediaPhaseCarrierEarlyMedia {
		t.Errorf("expected CarrierEarlyMedia unchanged, got %v", orch.Phase())
	}
}

func TestMediaOrchestrator_StopCancelsTimer(t *testing.T) {
	ctx := context.Background()
	cmdService := newTestCommandService()
	store := newTestSessionStore()

	_ = store.Save(ctx, esl.CallSession{CallID: "call-cancel", Metadata: map[string]any{"supplementRingPlaying": true}})

	orch := &MediaOrchestrator{
		CallID:         "call-cancel",
		CommandService: cmdService,
		SessionStore:   store,
		Logger:         testLogger,
	}

	// 启动补铃音 + 长定时器
	_ = orch.Start(ctx, "ring.wav", "uuid-agent", "fs-addr", 5000, contracts.LegRoleAgent, contracts.CallFlowAPIOutbound)

	// 立即停止（应取消定时器）
	orch.Stop(ctx)

	orch.mu.Lock()
	hasTimer := orch.timer != nil
	orch.mu.Unlock()

	if hasTimer {
		t.Error("expected timer to be nil after Stop")
	}
	if orch.Phase() != MediaPhaseComplete {
		t.Errorf("expected Complete, got %v", orch.Phase())
	}

	// 等待原定时器时间（5s），确认不会触发 panic 或副作用
	time.Sleep(100 * time.Millisecond)
}

func TestMediaRegistry_GetOrCreateAndRemove(t *testing.T) {
	ctx := context.Background()
	registry := NewMediaRegistry()
	cmdService := newTestCommandService()
	store := newTestSessionStore()

	_ = store.Save(ctx, esl.CallSession{CallID: "reg-1", Metadata: map[string]any{"supplementRingPlaying": true}})

	// GetOrCreate 创建新实例
	orch1 := registry.GetOrCreate("reg-1", cmdService, store, nil)
	if orch1 == nil {
		t.Fatal("expected non-nil orchestrator")
	}

	// GetOrCreate 返回相同实例
	orch2 := registry.GetOrCreate("reg-1", cmdService, store, nil)
	if orch1 != orch2 {
		t.Error("expected same orchestrator instance")
	}

	// Get 返回已创建的实例
	orch3 := registry.Get("reg-1")
	if orch3 != orch1 {
		t.Error("expected Get to return same orchestrator")
	}

	// 启动编排器
	_ = orch1.Start(ctx, "ring.wav", "uuid", "fs", 5000, contracts.LegRoleAgent, contracts.CallFlowAPIOutbound)

	// Remove 应停止定时器并移除
	registry.Remove("reg-1")

	// Get 应返回 nil
	if orch := registry.Get("reg-1"); orch != nil {
		t.Error("expected nil after Remove")
	}

	// 幂等 Remove
	registry.Remove("reg-1") // should not panic
}

func TestMediaRegistry_GetNonExistent(t *testing.T) {
	registry := NewMediaRegistry()
	if orch := registry.Get("non-existent"); orch != nil {
		t.Error("expected nil for non-existent callID")
	}
}

func TestMediaOrchestrator_SetBroadcastTime(t *testing.T) {
	orch := &MediaOrchestrator{}
	orch.SetBroadcastTime(3000)
	orch.mu.Lock()
	val := orch.broadcastMs
	orch.mu.Unlock()
	if val != 3000 {
		t.Errorf("expected broadcastMs=3000, got %d", val)
	}
}
