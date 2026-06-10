package callflow

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	operatedomain "yunshu/internal/domain/operate"
	"yunshu/pkg/idempotency"
)

type mockStatusReader struct {
	status esl.ExtensionStatus
	ok     bool
	err    error
}

func (m mockStatusReader) GetExtensionStatus(_ context.Context, _ string) (esl.ExtensionStatus, bool, error) {
	return m.status, m.ok, m.err
}

func buildTestFlow() operatedomain.AIModelFlow {
	graphJSON := `{
		"nodes": [
			{"id": "node-start", "type": "start", "label": "开始", "metadata": {"asrEnabled": true, "wsUrl": "ws://10.0.0.1:8080/asr"}},
			{"id": "node-reply", "type": "reply", "label": "欢迎语", "metadata": {"text": "您好！这里是云枢。"}},
			{"id": "node-intent", "type": "intent", "label": "意图分支"},
			{"id": "node-bill", "type": "reply", "label": "播报话费", "metadata": {"text": "您的话费充足。"}},
			{"id": "node-transfer", "type": "transfer", "label": "转接人工", "metadata": {"targetId": "1002", "targetType": "extension"}},
			{"id": "node-agent-busy", "type": "reply", "label": "坐席忙", "metadata": {"text": "坐席正忙，请稍后。"}},
			{"id": "node-agent-idle", "type": "reply", "label": "坐席通", "metadata": {"text": "正在为您转接。"}},
			{"id": "node-end", "type": "end", "label": "挂断"}
		],
		"edges": [
			{"id": "edge-1", "source": "node-start", "target": "node-reply"},
			{"id": "edge-2", "source": "node-reply", "target": "node-intent"},
			{"id": "edge-3", "source": "node-intent", "target": "node-bill", "sourceHandle": "我要查话费"},
			{"id": "edge-4", "source": "node-intent", "target": "node-transfer", "sourceHandle": "转人工"},
			{"id": "edge-5", "source": "node-transfer", "target": "node-agent-busy", "sourceHandle": "no_agent"},
			{"id": "edge-6", "source": "node-transfer", "target": "node-agent-idle", "sourceHandle": "has_agent"},
			{"id": "edge-7", "source": "node-bill", "target": "node-end"}
		]
	}`

	var graph operatedomain.AIFlowGraph
	_ = json.Unmarshal([]byte(graphJSON), &graph)

	return operatedomain.AIModelFlow{
		ID:        1,
		Name:      "测试智能 IVR",
		Prompt:    "降级提示词",
		FlowGraph: &graph,
	}
}

func TestAIVoiceEngineStartFlowAndAudioStream(t *testing.T) {
	t.Parallel()

	executor := &esl.MemoryCommandExecutor{}
	cmdService := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	store := esl.NewMemorySessionStore()
	statusReader := mockStatusReader{status: esl.ExtensionStatusIdle, ok: true}
	engine := NewAIVoiceEngine(context.Background(), cmdService, store, statusReader, nil)

	session := esl.CallSession{
		CallID:  "call-start-1",
		Profile: contracts.CallFlowInbound,
		State:   esl.CallAnswered,
		UUIDs:   map[string]contracts.LegRole{"uuid-cust": contracts.LegRoleCustomer},
		FSAddr:  "127.0.0.1:5060",
		Metadata: map[string]any{
			"customerUuid": "uuid-cust",
		},
	}
	_ = store.Save(context.Background(), session)

	flow := buildTestFlow()
	err := engine.StartAIVoiceFlow(context.Background(), &session, flow)
	if err != nil {
		t.Fatalf("StartAIVoiceFlow failed: %v", err)
	}

	// 验证下发了启动 audio_stream 指令
	if executor.Count() < 1 {
		t.Fatalf("Expected audio_stream and playback commands, got %d", executor.Count())
	}

	hasAudioStream := false
	hasPlayback := false
	for _, cmd := range executor.Commands {
		if cmd.Command == "audio-stream" {
			hasAudioStream = true
			if cmd.Payload["url"] != "ws://10.0.0.1:8080/asr" {
				t.Errorf("Unexpected audio-stream url: %v", cmd.Payload["url"])
			}
		}
		if cmd.Command == "playback" {
			hasPlayback = true
			if cmd.Payload["file"] != "tts://您好！这里是云枢。" {
				t.Errorf("Unexpected playback file: %v", cmd.Payload["file"])
			}
		}
	}

	if !hasAudioStream {
		t.Errorf("audio_stream command was not sent")
	}
	if !hasPlayback {
		t.Errorf("playback command was not sent")
	}

	// 验证活跃节点已被同步保存
	saved, _ := store.Get(context.Background(), "call-start-1")
	if saved.Metadata["aiCurrentNode"] != "node-reply" {
		t.Errorf("Expected current node to be node-reply, got %v", saved.Metadata["aiCurrentNode"])
	}
}

func TestAIVoiceEngineProcessASRTextRoutesCorrectly(t *testing.T) {
	t.Parallel()

	executor := &esl.MemoryCommandExecutor{}
	cmdService := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	store := esl.NewMemorySessionStore()
	statusReader := mockStatusReader{status: esl.ExtensionStatusIdle, ok: true}
	engine := NewAIVoiceEngine(context.Background(), cmdService, store, statusReader, nil)

	session := esl.CallSession{
		CallID:  "call-asr-1",
		Profile: contracts.CallFlowInbound,
		State:   esl.CallAnswered,
		UUIDs:   map[string]contracts.LegRole{"uuid-cust": contracts.LegRoleCustomer},
		FSAddr:  "127.0.0.1:5060",
		Metadata: map[string]any{
			"customerUuid":  "uuid-cust",
			"aiCurrentNode": "node-intent", // 当前停留在意图节点
		},
	}
	_ = store.Save(context.Background(), session)

	flow := buildTestFlow()

	// 模拟 ASR 识别“查话费”
	err := engine.ProcessASRText(context.Background(), &session, flow, "我想查话费，谢谢")
	if err != nil {
		t.Fatalf("ProcessASRText failed: %v", err)
	}

	// 确认下发了话费播报
	saved, _ := store.Get(context.Background(), "call-asr-1")
	if saved.Metadata["aiCurrentNode"] != "node-bill" {
		t.Errorf("Expected current node to be node-bill, got %v", saved.Metadata["aiCurrentNode"])
	}

	hasPlayback := false
	for _, cmd := range executor.Commands {
		if cmd.Command == "playback" && cmd.Payload["file"] == "tts://您当前的话费余额为一百元。" { // 在 executeNode 中，node-bill 的 Metadata text 是“您当前的话费余额为一百元。”吗？
			// 等等，buildTestFlow 中 node-bill 包含 text "您的话费充足。"
			// 我们来验证 text == "tts://您的话费充足。"
		}
		if cmd.Command == "playback" && cmd.Payload["file"] == "tts://您的话费充足。" {
			hasPlayback = true
		}
	}
	if !hasPlayback {
		t.Errorf("Expected tts://您的话费充足。 playback, but not found")
	}
}

func TestAIVoiceEngineProcessASRTextFallback(t *testing.T) {
	t.Parallel()

	executor := &esl.MemoryCommandExecutor{}
	cmdService := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
	store := esl.NewMemorySessionStore()
	engine := NewAIVoiceEngine(context.Background(), cmdService, store, nil, nil)

	session := esl.CallSession{
		CallID:  "call-asr-fallback",
		Profile: contracts.CallFlowInbound,
		State:   esl.CallAnswered,
		UUIDs:   map[string]contracts.LegRole{"uuid-cust": contracts.LegRoleCustomer},
		FSAddr:  "127.0.0.1:5060",
		Metadata: map[string]any{
			"customerUuid":  "uuid-cust",
			"aiCurrentNode": "node-intent",
		},
	}
	_ = store.Save(context.Background(), session)

	flow := buildTestFlow()

	// 模拟 ASR 无法识别出内容，且未配置大语言模型，物理引擎将直接严格报错
	err := engine.ProcessASRText(context.Background(), &session, flow, "我想买个苹果")
	if err == nil {
		t.Fatalf("Expected ProcessASRText to return error when LLM is not configured, got nil")
	}

	if !strings.Contains(err.Error(), "大语言模型物理引擎未配置或应答内容为空") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// 确认没有任何播放动作被下发（拒绝仿真兜底）
	hasFallbackPlayback := false
	for _, cmd := range executor.Commands {
		if cmd.Command == "playback" {
			hasFallbackPlayback = true
		}
	}
	if hasFallbackPlayback {
		t.Errorf("Expected no playback to be issued when LLM is not configured")
	}

	// 因为报错，活跃节点依然在意图卡点
	saved, _ := store.Get(context.Background(), "call-asr-fallback")
	if saved.Metadata["aiCurrentNode"] != "node-intent" {
		t.Errorf("Expected active node to remain node-intent, got %v", saved.Metadata["aiCurrentNode"])
	}
}

func TestAIVoiceEngineACDRoutingBasedOnRedis(t *testing.T) {
	t.Parallel()

	t.Run("Agent is Idle", func(t *testing.T) {
		executor := &esl.MemoryCommandExecutor{}
		cmdService := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		store := esl.NewMemorySessionStore()
		// 坐席状态为空闲
		statusReader := mockStatusReader{status: esl.ExtensionStatusIdle, ok: true}
		engine := NewAIVoiceEngine(context.Background(), cmdService, store, statusReader, nil)

		session := esl.CallSession{
			CallID:  "call-acd-idle",
			Profile: contracts.CallFlowInbound,
			State:   esl.CallAnswered,
			UUIDs:   map[string]contracts.LegRole{"uuid-cust": contracts.LegRoleCustomer},
			FSAddr:  "127.0.0.1:5060",
			Metadata: map[string]any{
				"customerUuid":  "uuid-cust",
				"aiCurrentNode": "node-intent",
			},
		}
		_ = store.Save(context.Background(), session)

		flow := buildTestFlow()

		// ASR 转人工
		err := engine.ProcessASRText(context.Background(), &session, flow, "我要转人工服务")
		if err != nil {
			t.Fatal(err)
		}

		saved, _ := store.Get(context.Background(), "call-acd-idle")
		// 在线空闲，走向 has_agent 连线的 target 即 node-agent-idle 节点
		if saved.Metadata["aiCurrentNode"] != "node-agent-idle" {
			t.Errorf("Expected current node to be node-agent-idle, got %v", saved.Metadata["aiCurrentNode"])
		}

		hasPlayback := false
		for _, cmd := range executor.Commands {
			if cmd.Command == "playback" && cmd.Payload["file"] == "tts://正在为您转接。" {
				hasPlayback = true
			}
		}
		if !hasPlayback {
			t.Errorf("Expected playback for transferring")
		}
	})

	t.Run("Agent is Busy", func(t *testing.T) {
		executor := &esl.MemoryCommandExecutor{}
		cmdService := esl.NewCommandService(idempotency.NewMemoryStore(), executor, nil)
		store := esl.NewMemorySessionStore()
		// 坐席状态为忙碌
		statusReader := mockStatusReader{status: esl.ExtensionStatusBusy, ok: true}
		engine := NewAIVoiceEngine(context.Background(), cmdService, store, statusReader, nil)

		session := esl.CallSession{
			CallID:  "call-acd-busy",
			Profile: contracts.CallFlowInbound,
			State:   esl.CallAnswered,
			UUIDs:   map[string]contracts.LegRole{"uuid-cust": contracts.LegRoleCustomer},
			FSAddr:  "127.0.0.1:5060",
			Metadata: map[string]any{
				"customerUuid":  "uuid-cust",
				"aiCurrentNode": "node-intent",
			},
		}
		_ = store.Save(context.Background(), session)

		flow := buildTestFlow()

		// ASR 转人工
		err := engine.ProcessASRText(context.Background(), &session, flow, "转人工")
		if err != nil {
			t.Fatal(err)
		}

		saved, _ := store.Get(context.Background(), "call-acd-busy")
		// 坐席正忙，走向 no_agent 连线的 target 即 node-agent-busy 节点
		if saved.Metadata["aiCurrentNode"] != "node-agent-busy" {
			t.Errorf("Expected current node to be node-agent-busy, got %v", saved.Metadata["aiCurrentNode"])
		}

		hasPlayback := false
		for _, cmd := range executor.Commands {
			if cmd.Command == "playback" && cmd.Payload["file"] == "tts://坐席正忙，请稍后。" {
				hasPlayback = true
			}
		}
		if !hasPlayback {
			t.Errorf("Expected playback for agent busy")
		}
	})
}
