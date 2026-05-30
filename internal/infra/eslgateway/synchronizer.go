// Package eslgateway 提供 cc-console 调用 cc-call ESL 管理接口的客户端。
package eslgateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yunshu/internal/domain/operate"
)

// Synchronizer 通过 cc-call `/esl/gateway` 兼容接口同步网关配置。
//
//	operate 服务通过 Feign 调用 ESL 服务。Go 重写保持相同边界：管理端只写
//	配置表，运行时副作用交给 cc-call 处理。
type Synchronizer struct {
	BaseURL string
	Client  *http.Client
	Timeout time.Duration
	Logger  *slog.Logger
}

// SyncGatewayConfig 调用 cc-call 的  兼容网关同步入口。
func (s *Synchronizer) SyncGatewayConfig(ctx context.Context, action string, gateway operate.Gateway) error {
	rawURL, method, err := s.request(action, gateway)
	if err != nil {
		return err
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, method, rawURL, nil)
	if err != nil {
		return err
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	s.logger().Info("运营端调用 cc-call 同步网关配置", "gatewayId", gateway.ID, "name", gateway.Name, "syncAction", action, "url", rawURL)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cc-call gateway sync http status %d", resp.StatusCode)
	}
	s.logger().Info("运营端调用 cc-call 同步网关配置成功", "gatewayId", gateway.ID, "name", gateway.Name, "syncAction", action, "status", resp.StatusCode)
	return nil
}

func (s *Synchronizer) request(action string, gateway operate.Gateway) (string, string, error) {
	baseURL := strings.TrimRight(s.BaseURL, "/")
	if baseURL == "" {
		return "", "", fmt.Errorf("cc-call base url is empty")
	}
	switch action {
	case "create":
		return baseURL + "/esl/gateway?gatewayId=" + url.QueryEscape(fmt.Sprintf("%d", gateway.ID)), http.MethodPost, nil
	case "update":
		return baseURL + "/esl/gateway?gatewayId=" + url.QueryEscape(fmt.Sprintf("%d", gateway.ID)), http.MethodPut, nil
	case "delete":
		return baseURL + "/esl/gateway?gatewayName=" + url.QueryEscape(gateway.Name), http.MethodDelete, nil
	default:
		return "", "", fmt.Errorf("unsupported gateway sync action %s", action)
	}
}

func (s *Synchronizer) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}
