package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	goredis "github.com/redis/go-redis/v9"
)

const (
	PushTopic = "cti_websocket_push_event"
)

// Hub 负责 CTI WebSocket 连接管理和 Redis 投影事件广播。
//
// Pub/Sub 消息只包含刷新提示；Hub 收到后必须读取 projectionKey 对应的 Redis hash，
// 再把投影内容推给客户端，避免把 Pub/Sub 当作最终业务真相。
type Hub struct {
	Client   *goredis.Client
	Topic    string
	Logger   *slog.Logger
	upgrader websocket.Upgrader

	mu      sync.Mutex
	clients map[*websocket.Conn]subscription
}

type subscription struct {
	MerchantID string
	TaskID     string
}

// NewHub 创建 WebSocket hub。
func NewHub(client *goredis.Client, logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		Client:  client,
		Topic:   PushTopic,
		Logger:  logger,
		clients: map[*websocket.Conn]subscription{},
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true // 允许无 Origin 的请求（同源请求、服务端调用）
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
		},
	}
}

// Start 启动 Redis Pub/Sub 消费循环。
func (h *Hub) Start(ctx context.Context) {
	if h == nil || h.Client == nil {
		return
	}
	topic := h.topic()
	pubsub := h.Client.Subscribe(ctx, topic)
	go func() {
		defer pubsub.Close()
		h.Logger.Info("CTI WebSocket Hub 已订阅 Redis 推送事件", "topic", topic)
		for {
			message, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					h.Logger.Info("CTI WebSocket Hub 推送循环退出", "reason", ctx.Err().Error())
					return
				}
				h.Logger.Error("CTI WebSocket Hub 读取 Redis 推送事件失败", "topic", topic, "error", err.Error())
				time.Sleep(time.Second)
				continue
			}
			if err := h.handlePush(ctx, []byte(message.Payload)); err != nil {
				h.Logger.Error("CTI WebSocket Hub 处理推送事件失败", "topic", topic, "payload", message.Payload, "error", err.Error())
			}
		}
	}()
}

// ServeHTTP 升级 WebSocket 连接并注册到 Hub。
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.Logger.Warn("CTI WebSocket 连接拒绝升级：缺少 token 凭证", "remoteAddr", r.RemoteAddr)
		http.Error(w, "缺少 token 凭证", http.StatusUnauthorized)
		return
	}

	if h.Client == nil {
		h.Logger.Error("CTI WebSocket 内部错误：Redis 客户端未配置")
		http.Error(w, "运行时错误", http.StatusInternalServerError)
		return
	}

	key := "console:auth:session:" + token
	raw, err := h.Client.Get(r.Context(), key).Result()
	if err != nil {
		h.Logger.Warn("从 Redis 读取 Token 会话失败，拒绝 WebSocket 连接", "token", token, "error", err.Error())
		http.Error(w, "无效的 token", http.StatusUnauthorized)
		return
	}

	var ticket struct {
		Tenant struct {
			MerchantID string `json:"merchantId"`
			Internal   bool   `json:"internal"`
			RoleID     string `json:"roleId"`
		} `json:"tenant"`
	}
	if err := json.Unmarshal([]byte(raw), &ticket); err != nil {
		h.Logger.Warn("解析 Redis Token 会话 JSON 失败", "token", token, "error", err.Error())
		http.Error(w, "会话格式错误", http.StatusUnauthorized)
		return
	}

	sub := subscription{
		TaskID: r.URL.Query().Get("taskId"),
	}

	// 仅从解析后的 TenantContext 提取 MerchantID；非管理员/内部角色不允许越权越界订阅其它商户
	if ticket.Tenant.Internal {
		// 内部管理员允许在 query 参数中指定任意 merchantId 进行订阅监控
		reqMerchantID := r.URL.Query().Get("merchantId")
		if reqMerchantID != "" {
			sub.MerchantID = reqMerchantID
		} else {
			sub.MerchantID = ticket.Tenant.MerchantID
		}
	} else {
		// 普通商户角色强制绑定其自身的 MerchantID，无视任何用户传入的 merchantId 过滤，防止越权越界
		sub.MerchantID = ticket.Tenant.MerchantID
	}

	if sub.MerchantID == "" {
		h.Logger.Warn("CTI WebSocket 连接缺少商户订阅范围，拒绝升级", "remoteAddr", r.RemoteAddr, "taskId", sub.TaskID)
		http.Error(w, "缺少 merchantId", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.Logger.Error("CTI WebSocket 连接升级失败", "error", err.Error())
		return
	}
	h.add(conn, sub)
	h.Logger.Info("CTI WebSocket 客户端已安全连接并注册", "remoteAddr", r.RemoteAddr, "merchantId", sub.MerchantID, "taskId", sub.TaskID, "internal", ticket.Tenant.Internal)
	defer func() {
		h.remove(conn)
		_ = conn.Close()
		h.Logger.Info("CTI WebSocket 客户端已断开", "remoteAddr", r.RemoteAddr)
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) handlePush(ctx context.Context, raw []byte) error {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	projectionKey, _ := payload["projectionKey"].(string)
	if projectionKey == "" {
		h.Logger.Warn("CTI WebSocket 推送事件缺少 projectionKey", "payload", payload)
		return nil
	}
	projection, err := h.Client.HGetAll(ctx, projectionKey).Result()
	if err != nil {
		return err
	}
	merchantID := firstValue(payload["merchantId"], projection["merchantId"])
	if stringify(merchantID) == "" {
		h.Logger.Warn("CTI WebSocket 推送事件缺少商户范围，已跳过广播", "projectionKey", projectionKey, "payload", payload)
		return nil
	}
	message := map[string]any{
		"type":          payload["type"],
		"taskId":        payload["taskId"],
		"telId":         payload["telId"],
		"merchantId":    merchantID,
		"userId":        firstValue(payload["userId"], projection["userId"]),
		"projectionKey": projectionKey,
		"projection":    projection,
	}
	h.broadcast(message)
	h.Logger.Info("CTI WebSocket 投影刷新已广播", "projectionKey", projectionKey, "clientCount", h.count())
	return nil
}

func (h *Hub) add(conn *websocket.Conn, sub subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = sub
}

func (h *Hub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, conn)
}

func (h *Hub) broadcast(message map[string]any) {
	h.mu.Lock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for conn, sub := range h.clients {
		if !sub.matches(message) {
			continue
		}
		clients = append(clients, conn)
	}
	h.mu.Unlock()
	h.writeMu.Lock()
	for _, conn := range clients {
		_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if err := conn.WriteJSON(message); err != nil {
			h.Logger.Warn("CTI WebSocket 投影刷新写入客户端失败", "error", err.Error())
			h.remove(conn)
			_ = conn.Close()
		}
	}
	h.writeMu.Unlock()
}

func (s subscription) matches(message map[string]any) bool {
	if s.MerchantID != "" && s.MerchantID != stringify(message["merchantId"]) {
		return false
	}
	if s.TaskID != "" && s.TaskID != stringify(message["taskId"]) {
		return false
	}
	return true
}

func stringify(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float32:
		if typed == float32(int(typed)) {
			return strconv.Itoa(int(typed))
		}
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return jsonNumberString(typed)
	}
}

func jsonNumberString(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func firstValue(values ...any) any {
	for _, value := range values {
		if stringify(value) != "" && stringify(value) != "<nil>" {
			return value
		}
	}
	return ""
}

func (h *Hub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *Hub) topic() string {
	if h.Topic != "" {
		return h.Topic
	}
	return PushTopic
}
