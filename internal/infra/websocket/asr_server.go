package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/callflow"
	"yunshu/internal/domain/esl"
	operate "yunshu/internal/domain/operate"
	"yunshu/internal/infra/events"
)

// ASRServer 实现了基于 WebSocket 旁路推流的 ASR 语音识别与仿真服务。
// 接收 FreeSWITCH mod_audio_stream 二进制 PCM 数据，物理进行能量检测 (VAD)，
// 并在说话断句后自动结合可视化 IVR 流图分支状态，进行智能寻路文本生成与事件投递。
type ASRServer struct {
	Addr         string
	Events       events.Bus
	SessionStore esl.SessionStore
	Logger       *slog.Logger

	upgrader websocket.Upgrader
	listener net.Listener
	server   *http.Server
	wg       sync.WaitGroup

	mu      sync.Mutex
	running bool
}

// NewASRServer 创建 ASR WebSocket 仿真与处理服务。
func NewASRServer(addr string, bus events.Bus, store esl.SessionStore, logger *slog.Logger) *ASRServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &ASRServer{
		Addr:         addr,
		Events:       bus,
		SessionStore: store,
		Logger:       logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Start 启动 ASR 服务，监听指定端口的 WebSocket 推流。
func (s *ASRServer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("ASR 服务端口监听失败: %w", err)
	}
	s.listener = ln
	s.running = true

	mux := http.NewServeMux()
	mux.HandleFunc("/asr", s.handleASRStream)
	mux.HandleFunc("/asr/", s.handleASRStream)

	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.mu.Unlock()
	s.Logger.Info("云枢 ASR 旁路语音推流 WebSocket 接收服务启动成功", "addr", s.Addr)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.Serve(ln); err != nil && !strings.Contains(err.Error(), "closed") {
			s.Logger.Error("云枢 ASR 接收服务异常退出", "error", err.Error())
		}
	}()

	return nil
}

// Stop 停止 ASR 接收服务。
func (s *ASRServer) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	s.Logger.Info("正在停止云枢 ASR 旁路语音接收服务...")
	if s.server != nil {
		_ = s.server.Shutdown(context.Background())
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.wg.Wait()
	s.Logger.Info("云枢 ASR 旁路语音接收服务已停止。")
}

// handleASRStream 处理 FreeSWITCH 侧的 Webhook/WebSocket 音频旁路连接。
func (s *ASRServer) handleASRStream(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.Logger.Error("云枢 ASR 接收：WebSocket 升级失败", "error", err.Error())
		return
	}
	defer conn.Close()

	s.Logger.Info("云枢 ASR 接收：已成功握手 FreeSWITCH 语音推流连接", "remote", r.RemoteAddr, "path", r.URL.Path)

	// 尝试从路径或参数提取 callId
	callID := r.URL.Query().Get("callId")
	if callID == "" {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 2 && parts[2] != "" {
			callID = parts[2]
		}
	}

	var packetCount int
	var totalBytes int64
	var hasMetadata bool
	var speechActive bool
	var speechStartFrame int
	var silenceFrames int
	var audioBuffer []byte // 协程局部音频字节流收集

	// PCM 采样通常为 20ms 一包，16kHz/16bit mono 帧大小为 640 字节，8kHz/16bit mono 为 320 字节。
	// 这里设定 RMS 能量检测 VAD 阈值。
	const (
		rmsThreshold   = 800.0 // 音量检测 VAD 阈值
		silenceLimit   = 50    // 连续 50 帧低能量判定为说话结束（约 1.0 秒静音）
		minSpeechLimit = 5     // 说话最少需要 5 帧高能量（约 100ms），避免呼吸或环境噪声误判
	)

	utteranceCount := 0

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.Logger.Info("云枢 ASR 接收：FreeSWITCH 正常关闭推流连接", "callId", callID)
			} else {
				s.Logger.Warn("云枢 ASR 接收：音频连接断开", "callId", callID, "error", err.Error())
			}
			break
		}

		if msgType == websocket.TextMessage {
			// 首帧通常为 Metadata 文本
			s.Logger.Info("云枢 ASR 接收：收到文本元数据帧", "callId", callID, "payload", string(data))
			var meta struct {
				CallID string `json:"callId"`
				UUID   string `json:"uuid"`
			}
			if err := json.Unmarshal(data, &meta); err == nil && meta.CallID != "" {
				callID = meta.CallID
			}
			hasMetadata = true
			continue
		}

		if msgType == websocket.BinaryMessage {
			packetCount++
			totalBytes += int64(len(data))

			if callID == "" {
				callID = "unknown-session"
			}

			// 能量计算 VAD
			rms := s.calculateRMS(data)
			if rms > rmsThreshold {
				if !speechActive {
					speechStartFrame++
					if speechStartFrame >= minSpeechLimit {
						speechActive = true
						s.Logger.Info("云枢 ASR 接收：检测到用户开始说话 (Speech Start)", "callId", callID, "rms", fmt.Sprintf("%.2f", rms))
						audioBuffer = append(audioBuffer, data...)
					}
				} else {
					audioBuffer = append(audioBuffer, data...)
				}
				silenceFrames = 0
			} else {
				if speechActive {
					audioBuffer = append(audioBuffer, data...)
					silenceFrames++
					if silenceFrames >= silenceLimit {
						// 触发断句物理与仿真 ASR 转换
						s.Logger.Info("云枢 ASR 接收：检测到用户说话结束 (Speech End)，启动断句寻路", "callId", callID, "audioBytes", len(audioBuffer))
						utteranceCount++

						// 拷贝当前音频 buffer，防止异步并发冲突
						currentAudio := make([]byte, len(audioBuffer))
						copy(currentAudio, audioBuffer)

						s.transcribeAndPublish(callID, utteranceCount, currentAudio)

						speechActive = false
						speechStartFrame = 0
						silenceFrames = 0
						audioBuffer = nil
					}
				} else {
					speechStartFrame = 0
				}
			}
		}
	}

	s.Logger.Info("云枢 ASR 接收：连接已安全关闭", "callId", callID, "totalFrames", packetCount, "totalBytes", totalBytes, "metadataParsed", hasMetadata)
}

// calculateRMS 计算 PCM 16bit 小端单声道音频的 Root Mean Square 能量。
func (s *ASRServer) calculateRMS(data []byte) float64 {
	if len(data) < 2 {
		return 0
	}
	var sum float64
	samples := len(data) / 2
	for i := 0; i < samples; i++ {
		sample := int16(data[i*2]) | (int16(data[i*2+1]) << 8)
		sum += float64(sample) * float64(sample)
	}
	return math.Sqrt(sum / float64(samples))
}

// transcribeAndPublish 根据会话所处的可视化 IVR 卡点节点，智能提取并物理转写匹配的 ASR 文字，
// 发送 asr_speech_detected 事件到事件总线，从而推动整个流图自走运转。
func (s *ASRServer) transcribeAndPublish(callID string, count int, pcmData []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := s.SessionStore.Get(ctx, callID)
	if err != nil {
		s.Logger.Warn("云枢 ASR 接收：无法获取活跃通话会话", "callId", callID, "error", err.Error())
		return
	}

	// 1. 检查会话是否配置了智能 AI 流程
	aiEnabled, _ := session.Metadata["aiEnabled"].(bool)
	if !aiEnabled {
		s.Logger.Warn("云枢 ASR 接收：该会话未启用智能 AI 可视化 IVR 流程，无需 ASR 转换", "callId", callID)
		return
	}

	var flow operate.AIModelFlow
	if flowJSON, ok := session.Metadata["aiFlowData"].(string); ok && flowJSON != "" {
		_ = json.Unmarshal([]byte(flowJSON), &flow)
	}

	if flow.FlowGraph == nil {
		s.Logger.Warn("云枢 ASR 接收：会话中缺少 AI 流程拓扑图数据", "callId", callID)
		return
	}

	currentNodeID, _ := session.Metadata["aiCurrentNode"].(string)
	if currentNodeID == "" {
		currentNodeID = "node-intent" // 默认退回意图节点
	}

	var transcribedText string
	var currentLabel string

	var currentNode *operate.AIFlowNode
	for i := range flow.FlowGraph.Nodes {
		if flow.FlowGraph.Nodes[i].ID == currentNodeID {
			currentNode = &flow.FlowGraph.Nodes[i]
			break
		}
	}

	if currentNode != nil {
		currentLabel = currentNode.Label
	}

	// 2. 物理 ASR 识别机制（火山引擎语音识别）
	var startNode *operate.AIFlowNode
	for i := range flow.FlowGraph.Nodes {
		if flow.FlowGraph.Nodes[i].Type == "start" {
			startNode = &flow.FlowGraph.Nodes[i]
			break
		}
	}

	if len(pcmData) > 0 && startNode != nil && startNode.Metadata != nil {
		volcAppId, _ := startNode.Metadata["volcAppId"].(string)
		volcToken, _ := startNode.Metadata["volcToken"].(string)
		volcCluster, _ := startNode.Metadata["volcCluster"].(string)

		// 兼容大模型 Key
		if volcAppId == "" {
			provider, _ := startNode.Metadata["llmProvider"].(string)
			apiKey, _ := startNode.Metadata["llmApiKey"].(string)
			if (strings.Contains(provider, "豆包") || strings.Contains(provider, "火山")) && apiKey != "" {
				volcToken = apiKey
				volcAppId = "cloudshu_demo"
			}
		}

		if volcAppId != "" && volcToken != "" {
			asrProvider, _ := startNode.Metadata["asrProvider"].(string)
			if asrProvider == "" {
				asrProvider = "volc"
			}

			s.Logger.Info("云枢 ASR 接收：检测到语音识别凭证，通过通用 ASR 引擎转写...", "callId", callID, "provider", asrProvider)
			wavData := pcmToWav(pcmData, 16000, 1, 16)

			configMap := map[string]any{
				"volcAppId":   volcAppId,
				"volcToken":   volcToken,
				"volcCluster": volcCluster,
			}
			asrEng := callflow.GetASREngine(asrProvider)
			text, err := asrEng.Transcribe(ctx, wavData, "wav", configMap)
			if err == nil && text != "" {
				transcribedText = text
				s.Logger.Info("云枢 ASR 接收：物理 ASR 转译成功！", "callId", callID, "text", transcribedText)
			} else {
				s.Logger.Error("云枢 ASR 接收：物理 ASR 接口转译失败，将降级使用仿真寻路", "callId", callID, "error", err)
			}
		}
	}

	// 如果没有火山 key 或者物理调用失败，采用自驱动仿真匹配
	if transcribedText == "" {
		if currentNode != nil && currentNode.Type == "intent" {
			var edges []operate.AIFlowEdge
			for i := range flow.FlowGraph.Edges {
				if flow.FlowGraph.Edges[i].Source == currentNode.ID {
					edges = append(edges, flow.FlowGraph.Edges[i])
				}
			}

			if len(edges) > 0 {
				index := (count - 1) % len(edges)
				edge := edges[index]
				if edge.SourceHandle != "" {
					transcribedText = edge.SourceHandle
					s.Logger.Info("云枢 ASR 接收：智能流图寻路匹配！自动生成符合意图的 ASR 仿真文本", "callId", callID, "currentNode", currentLabel, "handleText", transcribedText)
				}
			}
		}

		if transcribedText == "" {
			switch count {
			case 1:
				transcribedText = "你好，请问你是谁？"
			case 2:
				transcribedText = "我想咨询一下你们有什么功能"
			default:
				transcribedText = "谢谢，我没有其他问题了，再见。"
			}
			s.Logger.Info("云枢 ASR 接收：未找到匹配的分支，使用仿真拟真通用话术", "callId", callID, "text", transcribedText)
		}
	}

	// 3. 发布 asr_speech_detected 事件到总线
	if s.Events != nil {
		envelope := contracts.NewEventEnvelope(
			"asr-vad-detect:"+callID+":"+strconv.FormatInt(time.Now().UnixNano(), 10),
			"asr_speech_detected",
			callID,
			"call",
			callID,
			contracts.ServiceCall,
			map[string]any{
				"callId": callID,
				"text":   transcribedText,
			},
		)
		err := s.Events.Publish(ctx, envelope)
		if err != nil {
			s.Logger.Error("云枢 ASR 接收：发布 asr_speech_detected 事件失败", "callId", callID, "error", err.Error())
		} else {
			s.Logger.Info("云枢 ASR 接收：已成功向系统发布 asr_speech_detected 领域事件", "callId", callID, "text", transcribedText)
		}
	}
}

// pcmToWav 在内存中将 PCM 二进制数据拼装 44-byte 标准 WAV 头。
func pcmToWav(pcm []byte, sampleRate, channels, bits int) []byte {
	size := len(pcm)
	wav := make([]byte, 44+size)

	// RIFF
	copy(wav[0:4], []byte("RIFF"))
	totalSize := uint32(36 + size)
	wav[4] = byte(totalSize)
	wav[5] = byte(totalSize >> 8)
	wav[6] = byte(totalSize >> 16)
	wav[7] = byte(totalSize >> 24)
	copy(wav[8:12], []byte("WAVE"))

	// fmt
	copy(wav[12:16], []byte("fmt "))
	subChunk1Size := uint32(16)
	wav[16] = byte(subChunk1Size)
	wav[17] = byte(subChunk1Size >> 8)
	wav[18] = byte(subChunk1Size >> 16)
	wav[19] = byte(subChunk1Size >> 24)

	audioFormat := uint16(1)
	wav[20] = byte(audioFormat)
	wav[21] = byte(audioFormat >> 8)

	numChannels := uint16(channels)
	wav[22] = byte(numChannels)
	wav[23] = byte(numChannels >> 8)

	sRate := uint32(sampleRate)
	wav[24] = byte(sRate)
	wav[25] = byte(sRate >> 8)
	wav[26] = byte(sRate >> 16)
	wav[27] = byte(sRate >> 24)

	byteRate := uint32(sampleRate * channels * bits / 8)
	wav[28] = byte(byteRate)
	wav[29] = byte(byteRate >> 8)
	wav[30] = byte(byteRate >> 16)
	wav[31] = byte(byteRate >> 24)

	blockAlign := uint16(channels * bits / 8)
	wav[32] = byte(blockAlign)
	wav[33] = byte(blockAlign >> 8)

	bitsPerS := uint16(bits)
	wav[34] = byte(bitsPerS)
	wav[35] = byte(bitsPerS >> 8)

	// data
	copy(wav[36:40], []byte("data"))
	subChunk2Size := uint32(size)
	wav[40] = byte(subChunk2Size)
	wav[41] = byte(subChunk2Size >> 8)
	wav[42] = byte(subChunk2Size >> 16)
	wav[43] = byte(subChunk2Size >> 24)

	copy(wav[44:], pcm)
	return wav
}
