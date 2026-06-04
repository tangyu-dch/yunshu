// Package operate 承载运营管理端的业务能力。
//
// 管理端负责修改会影响运行时的配置，例如 FreeSWITCH 节点、网关、线路和
// Kamailio 配置。这里保持领域服务只依赖小接口，HTTP 兼容、数据库模型和缓存
// 刷新等外部细节由 transport/infra/app 层处理。
package operate

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// ErrInvalidFreeSwitchNode 表示管理端提交的 FreeSWITCH 节点配置缺少生产必需字段。
	ErrInvalidFreeSwitchNode = errors.New("invalid freeswitch node")
)

// FreeSwitchManagementService 管理  兼容 `freeswitch` 表中的节点配置。
//
// 修改节点配置只更新配置真相；多实例运行时刷新必须通过后续的 MQ/事件通知或
// cc-call 的 reload 接口完成，避免管理进程直接持有呼叫进程内部连接池。
type FreeSwitchManagementService struct {
	Registry Registry
	Reloader DispatcherReloadPort
	Logger   *slog.Logger
}

// List 返回所有未删除节点，包含禁用节点，并并发执行物理 ESL 在线健康检测。
func (s *FreeSwitchManagementService) List(ctx context.Context) ([]Node, error) {
	logger := s.logger()
	logger.Info("运营端开始查询 FreeSWITCH 节点列表")
	nodes, err := s.Registry.List(ctx)
	if err != nil {
		logger.Error("运营端查询 FreeSWITCH 节点列表失败", "error", err.Error())
		return nil, err
	}

	// 极速并发执行物理在线健康检测
	var wg sync.WaitGroup
	for i := range nodes {
		if !nodes[i].Enable {
			nodes[i].Status = NodeUnavailable
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			nodes[idx].Status = pingFreeswitch(nodes[idx].Address, nodes[idx].ESLPort)
		}(i)
	}
	wg.Wait()

	logger.Info("运营端查询 FreeSWITCH 节点列表完成", "nodeCount", len(nodes))
	return nodes, nil
}

// Save 创建或更新 FreeSWITCH 节点。
//
// enable=false 表示只落库不参与运行时选路；调用方仍需要触发刷新才能让 cc-call
// 连接池感知变更。密码为空时允许保存，便于本地环境或旧数据补录，但生产启用前
// 应由运维流程补齐。
func (s *FreeSwitchManagementService) Save(ctx context.Context, node Node) (Node, error) {
	logger := s.logger()
	normalized, err := normalizeNodeForSave(node)
	if err != nil {
		logger.Warn("运营端保存 FreeSWITCH 节点参数无效", "id", node.ID, "fsAddr", node.FSAddr, "address", node.Address, "eslPort", node.ESLPort, "error", err.Error())
		return Node{}, err
	}
	logger.Info("运营端开始保存 FreeSWITCH 节点", "id", normalized.ID, "fsAddr", normalized.FSAddr, "setId", normalized.SetID, "enable", normalized.Enable)
	if err := s.Registry.Upsert(ctx, normalized); err != nil {
		logger.Error("运营端保存 FreeSWITCH 节点失败", "id", normalized.ID, "fsAddr", normalized.FSAddr, "error", err.Error())
		return Node{}, err
	}
	if s.Reloader != nil {
		if err := s.Reloader.ReloadDispatcher(ctx); err != nil {
			logger.Warn("保存 FreeSWITCH 节点后热刷新 Kamailio Dispatcher 失败", "error", err.Error())
		} else {
			logger.Info("保存 FreeSWITCH 节点后热刷新 Kamailio Dispatcher 成功")
		}
	}
	if normalized.ID > 0 {
		saved, err := s.Registry.GetByID(ctx, normalized.ID)
		if err == nil {
			normalized = saved
		}
	}
	logger.Info("运营端保存 FreeSWITCH 节点完成", "id", normalized.ID, "fsAddr", normalized.FSAddr, "enable", normalized.Enable, "refreshRequired", true)
	return normalized, nil
}

// Enable 切换 FreeSWITCH 节点启用状态。
//
// 禁用节点后，运行时刷新应从连接池移除或停止选择该节点；启用节点后，运行时刷新
// 应重新加载节点连接配置。
func (s *FreeSwitchManagementService) Enable(ctx context.Context, id int, enable bool) (Node, error) {
	logger := s.logger()
	logger.Info("运营端开始切换 FreeSWITCH 节点启用状态", "id", id, "enable", enable)
	node, err := s.Registry.GetByID(ctx, id)
	if err != nil {
		logger.Warn("运营端切换 FreeSWITCH 节点启用状态失败，节点不存在", "id", id, "error", err.Error())
		return Node{}, err
	}
	node.Enable = enable
	if enable {
		node.Status = NodeActive
	} else {
		node.Status = NodeUnavailable
	}
	saved, err := s.Save(ctx, node)
	if err != nil {
		return Node{}, err
	}
	logger.Info("运营端切换 FreeSWITCH 节点启用状态完成", "id", id, "fsAddr", saved.FSAddr, "enable", enable, "refreshRequired", true)
	return saved, nil
}

// Delete 按  兼容语义逻辑删除 FreeSWITCH 节点。
//
// 删除后需要刷新 cc-call，确保连接池释放对应 ESL 连接且新呼叫不再路由到该节点。
func (s *FreeSwitchManagementService) Delete(ctx context.Context, id int) error {
	logger := s.logger()
	logger.Info("运营端开始删除 FreeSWITCH 节点", "id", id)
	if err := s.Registry.Delete(ctx, id); err != nil {
		logger.Warn("运营端删除 FreeSWITCH 节点失败", "id", id, "error", err.Error())
		return err
	}
	if s.Reloader != nil {
		if err := s.Reloader.ReloadDispatcher(ctx); err != nil {
			logger.Warn("删除 FreeSWITCH 节点后热刷新 Kamailio Dispatcher 失败", "id", id, "error", err.Error())
		} else {
			logger.Info("删除 FreeSWITCH 节点后热刷新 Kamailio Dispatcher 成功", "id", id)
		}
	}
	logger.Info("运营端删除 FreeSWITCH 节点完成", "id", id, "refreshRequired", true)
	return nil
}

func (s *FreeSwitchManagementService) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeNodeForSave(node Node) (Node, error) {
	if node.Address == "" && node.FSAddr != "" {
		address, port, ok := splitFSAddr(node.FSAddr)
		if ok {
			node.Address = address
			node.ESLPort = port
		}
	}
	if node.Address == "" || node.ESLPort <= 0 {
		return Node{}, ErrInvalidFreeSwitchNode
	}
	if node.FSAddr == "" {
		node.FSAddr = node.Address + ":" + strconv.Itoa(node.ESLPort)
	}
	if node.SetID <= 0 {
		node.SetID = 1
	}
	if node.Weight <= 0 {
		node.Weight = 50
	}
	if node.RWeight <= 0 {
		node.RWeight = node.Weight
	}
	if node.CC <= 0 {
		node.CC = 1
	}
	if node.Status == "" {
		if node.Enable {
			node.Status = NodeActive
		} else {
			node.Status = NodeUnavailable
		}
	}
	return node, nil
}

func splitFSAddr(fsAddr string) (string, int, bool) {
	index := strings.LastIndex(fsAddr, ":")
	if index <= 0 || index == len(fsAddr)-1 {
		return "", 0, false
	}
	port, err := strconv.Atoi(fsAddr[index+1:])
	if err != nil {
		return "", 0, false
	}
	return fsAddr[:index], port, true
}

// pingFreeswitch 极速物理 ESL 状态心跳探测
func pingFreeswitch(address string, eslPort int) NodeStatus {
	if eslPort <= 0 {
		eslPort = 8021
	}
	addr := net.JoinHostPort(address, strconv.Itoa(eslPort))

	// 设置超短的 150ms 拨号超时
	conn, err := net.DialTimeout("tcp", addr, 150*time.Millisecond)
	if err != nil {
		return NodeUnavailable
	}
	conn.Close()
	return NodeActive
}
