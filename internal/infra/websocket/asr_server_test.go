package websocket

import (
	"context"
	"log/slog"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	"yunshu/internal/infra/events"
)

func TestASRServerVADAndMockRecognition(t *testing.T) {
	logger := slogNewTestLogger()
	bus := events.NewMemoryBus(logger)
	store := esl.NewMemorySessionStore()

	// 1. 初始化 ASR 接收端服务并监听随机高端口
	server := NewASRServer("127.0.0.1:0", bus, store, logger)
	err := server.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start ASRServer: %v", err)
	}
	defer server.Stop()

	// 获取分配的真实 TCP 监听端口
	addr := server.listener.Addr().String()

	// 2. 预存一个正在通话的 IVR 流程会话
	flowJSON := `{
		"flowGraph": {
			"nodes": [
				{"id": "node-start", "type": "start", "label": "开始", "metadata": {"asrEnabled": true}},
				{"id": "node-intent", "type": "intent", "label": "意图分支"},
				{"id": "node-bill", "type": "reply", "label": "查话费", "metadata": {"text": "查话费完毕"}},
				{"id": "node-transfer", "type": "transfer", "label": "转接人工"}
			],
			"edges": [
				{"id": "edge-1", "source": "node-intent", "target": "node-bill", "sourceHandle": "我要查话费"},
				{"id": "edge-2", "source": "node-intent", "target": "node-transfer", "sourceHandle": "我要转人工"}
			]
		}
	}`

	session := esl.CallSession{
		CallID:  "test-asr-call-100",
		Profile: contracts.CallFlowInbound,
		State:   esl.CallAnswered,
		UUIDs:   map[string]contracts.LegRole{"uuid-cust": contracts.LegRoleCustomer},
		FSAddr:  "127.0.0.1:5060",
		Metadata: map[string]any{
			"customerUuid":  "uuid-cust",
			"aiEnabled":     true,
			"aiCurrentNode": "node-intent",
			"aiFlowData":    flowJSON,
		},
	}
	_ = store.Save(context.Background(), session)

	// 3. 开启订阅通道，等待 ASR 识别事件发布
	eventChan := make(chan contracts.EventEnvelope[map[string]any], 1)
	bus.Subscribe("asr_speech_detected", func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		eventChan <- event
		return nil
	})

	// 4. WebSocket 客户端拨号连接
	u := url.URL{Scheme: "ws", Host: addr, Path: "/asr"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	defer conn.Close()

	// 发送首帧 Metadata
	metaJSON := `{"callId": "test-asr-call-100", "uuid": "uuid-cust"}`
	err = conn.WriteMessage(websocket.TextMessage, []byte(metaJSON))
	if err != nil {
		t.Fatalf("Failed to write metadata: %v", err)
	}

	// 5. 模拟 VAD 录音包输入
	// A) 发送 10 帧纯静音包 (0)
	silenceFrame := make([]byte, 320)
	for i := 0; i < 10; i++ {
		_ = conn.WriteMessage(websocket.BinaryMessage, silenceFrame)
		time.Sleep(5 * time.Millisecond)
	}

	// B) 发送 10 帧高能人声包 (声波振幅较大，RMS 远超阈值)
	voiceFrame := make([]byte, 320)
	for i := 0; i < len(voiceFrame)/2; i++ {
		// 填充 16bit 符号交替大振幅 (如 10000/-10000)
		val := int16(10000)
		if i%2 == 0 {
			val = -10000
		}
		voiceFrame[i*2] = byte(val & 0xFF)
		voiceFrame[i*2+1] = byte((val >> 8) & 0xFF)
	}

	for i := 0; i < 10; i++ {
		_ = conn.WriteMessage(websocket.BinaryMessage, voiceFrame)
		time.Sleep(5 * time.Millisecond)
	}

	// C) 说话结束，发送 60 帧静音包触发断句检测
	for i := 0; i < 60; i++ {
		_ = conn.WriteMessage(websocket.BinaryMessage, silenceFrame)
		time.Sleep(5 * time.Millisecond)
	}

	// 6. 验证是否发布了 asr_speech_detected 领域事件
	select {
	case evt := <-eventChan:
		t.Logf("Success! Event received: %+v", evt)
		if evt.AggregateID != "test-asr-call-100" {
			t.Errorf("Expected callId test-asr-call-100, got %s", evt.AggregateID)
		}
		text, _ := evt.Payload["text"].(string)
		if text != "我要查话费" && text != "我要转人工" {
			t.Errorf("Expected simulated search handle to match flow edge, got %q", text)
		}
		t.Logf("Transcribed ASR Text (Self-Driving): %s", text)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for ASR speech detected event")
	}
}

// slogNewTestLogger 提供一个内部空 logger。
func slogNewTestLogger() *slog.Logger {
	return slog.Default()
}
