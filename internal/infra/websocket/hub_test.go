package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gorilla "github.com/gorilla/websocket"
	goredis "github.com/redis/go-redis/v9"
)

func TestHubBroadcastsProjectionFromRedis(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	hub := NewHub(client, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub.Start(ctx)

	httpServer := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer httpServer.Close()
	url := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "?merchantId=88"
	conn, _, err := gorilla.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := client.HSet(ctx, "batch:10:summary", map[string]any{"taskId": "10", "merchantId": "88", "status": "completed"}).Err(); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"type": "batch_task_completed", "taskId": "10", "merchantId": "88", "projectionKey": "batch:10:summary"})
	if err := client.Publish(ctx, PushTopic, raw).Err(); err != nil {
		t.Fatal(err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var message struct {
		Type       string            `json:"type"`
		TaskID     string            `json:"taskId"`
		Projection map[string]string `json:"projection"`
	}
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	if message.Type != "batch_task_completed" || message.Projection["status"] != "completed" {
		t.Fatalf("unexpected websocket message: %+v", message)
	}
}

func TestHubRejectsConnectionWithoutMerchantScope(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	hub := NewHub(client, nil)
	httpServer := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer httpServer.Close()

	url := "ws" + strings.TrimPrefix(httpServer.URL, "http")
	conn, resp, err := gorilla.DefaultDialer.Dial(url, nil)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatalf("expected missing merchant scope to be rejected")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 response, got %#v err=%v", resp, err)
	}
}

func TestHubFiltersProjectionBySubscription(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	hub := NewHub(client, nil)
	ctx := context.Background()

	httpServer := httptest.NewServer(http.HandlerFunc(hub.ServeHTTP))
	defer httpServer.Close()
	baseURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")
	matched, _, err := gorilla.DefaultDialer.Dial(baseURL+"?merchantId=88&taskId=10", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer matched.Close()
	mismatched, _, err := gorilla.DefaultDialer.Dial(baseURL+"?merchantId=99&taskId=10", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mismatched.Close()

	if err := client.HSet(ctx, "batch:10:summary", map[string]any{"taskId": "10", "merchantId": "88", "status": "completed"}).Err(); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"type": "batch_task_completed", "taskId": "10", "merchantId": "88", "projectionKey": "batch:10:summary"})
	if err := hub.handlePush(ctx, raw); err != nil {
		t.Fatal(err)
	}

	if err := matched.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var message map[string]any
	if err := matched.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	if message["merchantId"] != "88" {
		t.Fatalf("unexpected matched message: %+v", message)
	}

	if err := mismatched.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := mismatched.ReadJSON(&message); err == nil {
		t.Fatalf("mismatched client should not receive message: %+v", message)
	}
}
