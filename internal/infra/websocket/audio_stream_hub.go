package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	goredis "github.com/redis/go-redis/v9"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/callflow"
)

// ============================================================================
// 1. 音频流 WebSocket Hub
// ============================================================================

// AudioStreamHub 音频流 WebSocket Hub
type AudioStreamHub struct {
	pipeline callflow.AIStreamPipeline
	redis    *goredis.Client
	logger   *slog.Logger
	upgrader websocket.Upgrader

	clients        map[string]*AudioStreamClient // key: callID
	sessionToCall  map[string]string             // key: sessionID -> callID
	mu             sync.RWMutex
}

// AudioStreamClient 单个音频流客户端
type AudioStreamClient struct {
	conn      *websocket.Conn
	sessionID string
	callID    string
	sendChan  chan []byte
	hub       *AudioStreamHub
}

// NewAudioStreamHub 创建音频流 Hub
func NewAudioStreamHub(pipeline callflow.AIStreamPipeline, redis *goredis.Client, logger *slog.Logger) *AudioStreamHub {
	if logger == nil {
		logger = slog.Default()
	}

	hub := &AudioStreamHub{
		pipeline:      pipeline,
		redis:         redis,
		logger:        logger,
		clients:       make(map[string]*AudioStreamClient),
		sessionToCall: make(map[string]string),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true // 无 Origin 头的请求（同源请求、服务端调用）
				}
				allowedOrigins := os.Getenv("WS_ALLOWED_ORIGINS")
				if allowedOrigins == "" {
					return true // 未配置时允许所有（开发环境兜底）
				}
				for _, allowed := range strings.Split(allowedOrigins, ",") {
					if strings.TrimSpace(allowed) == origin {
						return true
					}
				}
				return false
			},
			ReadBufferSize:  8192,
			WriteBufferSize: 8192,
		},
	}

	// 启动事件监听
	go hub.listenPipelineEvents()

	return hub
}

// ServeHTTP 处理 WebSocket 连接
func (h *AudioStreamHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// --- Token 认证 ---
	token := r.URL.Query().Get("token")
	if token == "" {
		h.logger.Warn("音频流 WebSocket 连接拒绝升级：缺少 token 凭证", "remoteAddr", r.RemoteAddr)
		http.Error(w, "缺少 token 凭证", http.StatusUnauthorized)
		return
	}

	if h.redis == nil {
		h.logger.Error("音频流 WebSocket 内部错误：Redis 客户端未配置")
		http.Error(w, "运行时错误", http.StatusInternalServerError)
		return
	}

	sessionKey := "console:auth:session:" + token
	raw, err := h.redis.Get(r.Context(), sessionKey).Result()
	if err != nil {
		h.logger.Warn("从 Redis 读取 Token 会话失败，拒绝 WebSocket 连接", "error", err.Error())
		http.Error(w, "无效的 token", http.StatusUnauthorized)
		return
	}

	var ticket struct {
		Tenant struct {
			MerchantID string `json:"merchantId"`
		} `json:"tenant"`
	}
	if err := json.Unmarshal([]byte(raw), &ticket); err != nil {
		h.logger.Warn("解析 Redis Token 会话 JSON 失败", "error", err.Error())
		http.Error(w, "会话格式错误", http.StatusUnauthorized)
		return
	}

	// --- 参数校验 ---
	callID := r.URL.Query().Get("callId")
	if callID == "" {
		http.Error(w, "missing callId", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("WebSocket 升级失败", "error", err)
		return
	}

	client := &AudioStreamClient{
		conn:     conn,
		callID:   callID,
		sendChan: make(chan []byte, 256),
		hub:      h,
	}

	h.mu.Lock()
	h.clients[callID] = client
	h.mu.Unlock()

	h.logger.Info("音频流客户端连接", "callID", callID, "merchantId", ticket.Tenant.MerchantID)

	// 启动会话
	ctx := context.Background()
	session, err := h.pipeline.StartSession(
		callID,
		r.URL.Query().Get("customerUUID"),
		r.URL.Query().Get("fsAddr"),
		map[string]interface{}{
			"systemPrompt": "你是云枢呼叫中心的AI助手，友好、专业地帮助用户解决问题。",
		},
	)
	if err != nil {
		h.logger.Error("启动会话失败", "error", err)
		conn.Close()
		h.mu.Lock()
		delete(h.clients, callID)
		h.mu.Unlock()
		return
	}

	client.sessionID = session.SessionID

	// 注册 sessionID -> callID 映射，用于事件过滤
	h.mu.Lock()
	h.sessionToCall[session.SessionID] = callID
	h.mu.Unlock()

	// 启动读写循环
	go client.readPump(ctx)
	go client.writePump()
}

// listenPipelineEvents 监听管道事件
func (h *AudioStreamHub) listenPipelineEvents() {
	eventChan := h.pipeline.GetEventChannel()

	for event := range eventChan {
		// 从事件 Data 中提取 sessionID，用于定位目标客户端
		targetCallID := h.extractCallIDFromEvent(event)

		data := h.serializeEvent(event)

		h.mu.RLock()
		if targetCallID != "" {
			// 精确投递给单个客户端
			if client, ok := h.clients[targetCallID]; ok {
				select {
				case client.sendChan <- data:
				default:
					h.logger.Warn("客户端发送队列已满", "callID", targetCallID)
				}
			}
		} else {
			// 无法提取 sessionID 时退化为全量广播（兼容无 sessionID 的事件）
			for callID, client := range h.clients {
				select {
				case client.sendChan <- data:
				default:
					h.logger.Warn("客户端发送队列已满", "callID", callID)
				}
			}
		}
		h.mu.RUnlock()
	}
}

// extractCallIDFromEvent 从事件数据中提取 sessionID 并映射到 callID
func (h *AudioStreamHub) extractCallIDFromEvent(event contracts.StreamEvent) string {
	sessionID := ""
	switch d := event.Data.(type) {
	case map[string]interface{}:
		if v, ok := d["sessionID"].(string); ok {
			sessionID = v
		}
	}
	if sessionID == "" {
		return ""
	}
	h.mu.RLock()
	callID := h.sessionToCall[sessionID]
	h.mu.RUnlock()
	return callID
}

// serializeEvent 序列化事件
func (h *AudioStreamHub) serializeEvent(event contracts.StreamEvent) []byte {
	msg := callflow.WSMessage{
		Type:    event.Type,
		Data:    json.RawMessage(h.serializeData(event.Data)),
		Session: "",
	}

	data, _ := json.Marshal(msg)
	return data
}

// serializeData 序列化数据
func (h *AudioStreamHub) serializeData(data interface{}) []byte {
	if data == nil {
		return []byte("null")
	}
	result, _ := json.Marshal(data)
	return result
}

// ============================================================================
// 2. 客户端读写循环
// ============================================================================

// readPump 读取客户端消息
func (c *AudioStreamClient) readPump(ctx context.Context) {
	defer func() {
		c.hub.mu.Lock()
		delete(c.hub.clients, c.callID)
		delete(c.hub.sessionToCall, c.sessionID)
		c.hub.mu.Unlock()
		c.conn.Close()
		c.hub.pipeline.StopSession(c.sessionID)
		close(c.sendChan)
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("WebSocket 读取错误", "callID", c.callID, "error", err)
			}
			break
		}

		// 解析消息
		var wsMsg callflow.WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			c.hub.logger.Warn("消息解析失败", "callID", c.callID, "error", err)
			continue
		}

		// 处理消息
		if err := c.handleMessage(ctx, wsMsg); err != nil {
			c.hub.logger.Error("消息处理失败", "callID", c.callID, "error", err)
		}
	}
}

// writePump 写入消息到客户端
func (c *AudioStreamClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case message, ok := <-c.sendChan:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// 通道已关闭
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				return
			}

			// 写入队列中的其他消息
			n := len(c.sendChan)
			for i := 0; i < n; i++ {
				if err := c.conn.WriteMessage(websocket.BinaryMessage, <-c.sendChan); err != nil {
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理消息
func (c *AudioStreamClient) handleMessage(ctx context.Context, msg callflow.WSMessage) error {
	switch msg.Type {
	case "audio":
		// 处理音频数据
		var audioData callflow.WSAudioData
		if err := json.Unmarshal(msg.Data, &audioData); err != nil {
			return err
		}

		chunk := contracts.AudioChunk{
			Data:       audioData.Data,
			Format:     audioData.Format,
			SampleRate: audioData.SampleRate,
			Timestamp:  time.Now(),
		}

		return c.hub.pipeline.ProcessAudioChunk(ctx, c.sessionID, chunk)

	case "asr_text":
		// 处理 ASR 文本（当 ASR 在客户端处理时使用）
		var textData callflow.WSTextData
		if err := json.Unmarshal(msg.Data, &textData); err != nil {
			return err
		}

		return c.hub.pipeline.ProcessASRText(ctx, c.sessionID, textData.Text, true)

	case "switch_llm":
		// 切换 LLM 提供商
		var data struct {
			LLMID string `json:"llmId"`
		}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return err
		}

		return c.hub.pipeline.SwitchLLMProvider(c.sessionID, data.LLMID)

	case "switch_asr":
		// 切换 ASR 提供商
		var data struct {
			ASRID string `json:"asrId"`
		}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return err
		}

		return c.hub.pipeline.SwitchASRProvider(c.sessionID, data.ASRID)

	case "switch_tts":
		// 切换 TTS 提供商
		var data struct {
			TTSID string `json:"ttsId"`
		}
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			return err
		}

		return c.hub.pipeline.SwitchTTSProvider(c.sessionID, data.TTSID)

	case "ping":
		// 心跳
		c.sendChan <- c.createPongMessage()
		return nil
	}

	return nil
}

// createPongMessage 创建 pong 消息
func (c *AudioStreamClient) createPongMessage() []byte {
	msg := callflow.WSMessage{
		Type: "pong",
		Data: []byte("{}"),
	}
	data, _ := json.Marshal(msg)
	return data
}

// ============================================================================
// 3. 配置和初始化
// ============================================================================

// AudioStreamServerConfig 音频流服务器配置
type AudioStreamServerConfig struct {
	Addr     string
	Path     string
	Pipeline callflow.AIStreamPipeline
	Redis    *goredis.Client
	Logger   *slog.Logger
}

// StartAudioStreamServer 启动音频流服务器
func StartAudioStreamServer(config AudioStreamServerConfig) (*http.Server, error) {
	if config.Pipeline == nil {
		return nil, fmt.Errorf("pipeline is required")
	}
	if config.Redis == nil {
		return nil, fmt.Errorf("redis client is required")
	}

	if config.Addr == "" {
		config.Addr = ":8081"
	}

	if config.Path == "" {
		config.Path = "/ws/audio"
	}

	hub := NewAudioStreamHub(config.Pipeline, config.Redis, config.Logger)

	mux := http.NewServeMux()
	mux.Handle(config.Path, hub)

	server := &http.Server{
		Addr:    config.Addr,
		Handler: mux,
	}

	go func() {
		config.Logger.Info("音频流 WebSocket 服务器启动", "addr", config.Addr, "path", config.Path)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			config.Logger.Error("音频流服务器启动失败", "error", err)
		}
	}()

	return server, nil
}
