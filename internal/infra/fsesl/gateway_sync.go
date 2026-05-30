package fsesl

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"yunshu/internal/domain/esl"
)

// GatewayHTTPExecutor 通过 FreeSWITCH 节点的 cmd HTTP 地址同步网关配置。
//
//	GatewayConfigService 使用 GET 调用 `esl/createGateway`、`esl/updateGateway`
//
// 和 `esl/deleteGateway`。这里保持相同外部副作用，便于 Go 管理端和旧 FS cmd
// 服务共存。节点缺少 commandUrl 时只记录并跳过，兼容  的跳过语义。
type GatewayHTTPExecutor struct {
	Client  *http.Client
	Timeout time.Duration
	Logger  *slog.Logger
}

// ApplyGatewayConfig 对单个 FreeSWITCH 节点执行网关配置同步。
func (e *GatewayHTTPExecutor) ApplyGatewayConfig(ctx context.Context, req esl.GatewaySyncRequest, node esl.GatewaySyncNode) error {
	logger := e.logger()
	if node.CommandURL == "" {
		logger.Warn("跳过 FreeSWITCH 网关配置同步，节点 cmd 地址为空", "gatewayId", req.GatewayID, "gatewayName", req.GatewayName, "action", req.Action, "fsAddr", node.FSAddr)
		return nil
	}
	rawURL, err := gatewaySyncURL(req, node.CommandURL)
	if err != nil {
		return err
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	client := e.Client
	if client == nil {
		client = http.DefaultClient
	}
	logger.Info("调用 FreeSWITCH cmd 同步网关配置", "gatewayId", req.GatewayID, "gatewayName", req.GatewayName, "action", req.Action, "fsAddr", node.FSAddr, "url", rawURL)
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("freeswitch gateway sync http status %d", resp.StatusCode)
	}
	logger.Info("FreeSWITCH cmd 网关配置同步成功", "gatewayId", req.GatewayID, "gatewayName", req.GatewayName, "action", req.Action, "fsAddr", node.FSAddr, "status", resp.StatusCode)
	return nil
}

func (e *GatewayHTTPExecutor) logger() *slog.Logger {
	if e.Logger != nil {
		return e.Logger
	}
	return slog.Default()
}

func gatewaySyncURL(req esl.GatewaySyncRequest, commandURL string) (string, error) {
	base := strings.TrimRight(commandURL, "/")
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	switch req.Action {
	case esl.GatewaySyncCreate:
		return base + "/esl/createGateway?gatewayId=" + url.QueryEscape(fmt.Sprintf("%d", req.GatewayID)), nil
	case esl.GatewaySyncUpdate:
		return base + "/esl/updateGateway?gatewayId=" + url.QueryEscape(fmt.Sprintf("%d", req.GatewayID)), nil
	case esl.GatewaySyncDelete:
		return base + "/esl/deleteGateway?gatewayName=" + url.QueryEscape(req.GatewayName), nil
	default:
		return "", esl.ErrInvalidCommand
	}
}
