package esl

import (
	"context"
	"errors"
	"log/slog"
)

var (
	// ErrGatewayConfigNotFound 表示同步请求引用的网关不存在。
	ErrGatewayConfigNotFound = errors.New("gateway config not found")
	// ErrGatewaySyncTargetMissing 表示没有可同步的 FreeSWITCH 节点。
	ErrGatewaySyncTargetMissing = errors.New("gateway sync target missing")
)

// GatewaySyncAction 表示  `/esl/gateway` 兼容接口中的同步动作。
type GatewaySyncAction string

const (
	GatewaySyncCreate GatewaySyncAction = "create"
	GatewaySyncUpdate GatewaySyncAction = "update"
	GatewaySyncDelete GatewaySyncAction = "delete"
)

// GatewaySyncNode 表示需要接收网关配置同步的 FreeSWITCH 节点。
type GatewaySyncNode struct {
	ID         int    `json:"id,omitempty"`
	FSAddr     string `json:"fsAddr"`
	CommandURL string `json:"commandUrl,omitempty"`
}

// GatewaySyncRequest 表示一次网关配置同步请求。
type GatewaySyncRequest struct {
	Action      GatewaySyncAction `json:"action"`
	GatewayID   int               `json:"gatewayId,omitempty"`
	GatewayName string            `json:"gatewayName,omitempty"`
}

// GatewaySyncResult 表示网关同步目标和执行结果。
type GatewaySyncResult struct {
	Action      GatewaySyncAction `json:"action"`
	GatewayID   int               `json:"gatewayId,omitempty"`
	GatewayName string            `json:"gatewayName,omitempty"`
	TargetCount int               `json:"targetCount"`
	Targets     []GatewaySyncNode `json:"targets,omitempty"`
	Applied     bool              `json:"applied"`
}

// GatewayNameResolver 按 ID 解析  `gateway` 表中的网关名称。
type GatewayNameResolver interface {
	GetGatewayNameByID(ctx context.Context, id int) (string, error)
}

// GatewaySyncNodeLister 返回所有启用且未删除的 FreeSWITCH 同步目标。
type GatewaySyncNodeLister interface {
	ListGatewaySyncNodes(ctx context.Context) ([]GatewaySyncNode, error)
}

// GatewayConfigSyncExecutor 执行对单个 FreeSWITCH 节点的网关配置同步副作用。
type GatewayConfigSyncExecutor interface {
	ApplyGatewayConfig(ctx context.Context, req GatewaySyncRequest, node GatewaySyncNode) error
}

// GatewayConfigService 处理  兼容 `/esl/gateway` 网关配置同步入口。
//
//	实现通过每个 FreeSWITCH 节点的 cmd HTTP 地址执行 create/update/delete。
//
// Go 领域层先固定校验、目标枚举、日志和错误语义，具体 FS cmd 调用由 executor adapter 接入。
type GatewayConfigService struct {
	Gateways GatewayNameResolver
	Nodes    GatewaySyncNodeLister
	Executor GatewayConfigSyncExecutor
	Logger   *slog.Logger
}

// Sync 执行网关配置同步。
func (s *GatewayConfigService) Sync(ctx context.Context, req GatewaySyncRequest) (GatewaySyncResult, error) {
	logger := s.logger()
	if req.Action != GatewaySyncCreate && req.Action != GatewaySyncUpdate && req.Action != GatewaySyncDelete {
		return GatewaySyncResult{}, ErrInvalidCommand
	}
	if req.Action != GatewaySyncDelete {
		if req.GatewayID <= 0 {
			return GatewaySyncResult{}, ErrInvalidCommand
		}
		name, err := s.Gateways.GetGatewayNameByID(ctx, req.GatewayID)
		if err != nil {
			logger.Warn("网关配置同步跳过，网关不存在", "gatewayId", req.GatewayID, "action", req.Action, "error", err.Error())
			return GatewaySyncResult{}, ErrGatewayConfigNotFound
		}
		req.GatewayName = name
	}
	if req.Action == GatewaySyncDelete && req.GatewayName == "" {
		return GatewaySyncResult{}, ErrInvalidCommand
	}
	nodes, err := s.Nodes.ListGatewaySyncNodes(ctx)
	if err != nil {
		logger.Error("网关配置同步读取 FreeSWITCH 节点失败", "gatewayId", req.GatewayID, "gatewayName", req.GatewayName, "action", req.Action, "error", err.Error())
		return GatewaySyncResult{}, err
	}
	if len(nodes) == 0 {
		logger.Warn("网关配置同步跳过，未找到可用 FreeSWITCH 节点", "gatewayId", req.GatewayID, "gatewayName", req.GatewayName, "action", req.Action)
		return GatewaySyncResult{}, ErrGatewaySyncTargetMissing
	}
	for _, node := range nodes {
		logger.Info("开始同步 FreeSWITCH 网关配置", "gatewayId", req.GatewayID, "gatewayName", req.GatewayName, "action", req.Action, "fsAddr", node.FSAddr, "commandUrl", node.CommandURL)
		if s.Executor != nil {
			if err := s.Executor.ApplyGatewayConfig(ctx, req, node); err != nil {
				logger.Error("同步 FreeSWITCH 网关配置失败", "gatewayId", req.GatewayID, "gatewayName", req.GatewayName, "action", req.Action, "fsAddr", node.FSAddr, "error", err.Error())
				return GatewaySyncResult{}, err
			}
		}
	}
	logger.Info("同步 FreeSWITCH 网关配置完成", "gatewayId", req.GatewayID, "gatewayName", req.GatewayName, "action", req.Action, "targetCount", len(nodes), "applied", s.Executor != nil)
	return GatewaySyncResult{Action: req.Action, GatewayID: req.GatewayID, GatewayName: req.GatewayName, TargetCount: len(nodes), Targets: nodes, Applied: s.Executor != nil}, nil
}

func (s *GatewayConfigService) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}
