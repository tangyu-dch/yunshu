// Package fsesl 提供 FreeSWITCH ESL 连接池和真实命令执行 adapter。
//
// 设计对齐  yunshu-starter-esl：
// 1. 节点配置先加载到内存注册表；
// 2. 每个 FS 地址维护独立连接和断线重连；
// 3. 事件消费还要结合上层 ownership 租约，避免多实例重复处理同一节点事件；
// 4. 命令网关只负责发送已经成型的 ESL 命令，不做业务决策。
package fsesl

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/percipia/eslgo"
	"github.com/percipia/eslgo/command"

	"yunshu/internal/contracts"
	"yunshu/internal/infra/logging"
	fsregistry "yunshu/internal/infra/telephony"
)

// ErrFSNodeNotConfigured 表示尝试连接的 FreeSWITCH 节点地址未在连接池中配置。
var ErrFSNodeNotConfigured = errors.New("freeswitch node not configured")

// NodeConfig 是 FreeSWITCH 节点连接配置，包含 ESL 地址、认证密码和负载均衡权重。
type NodeConfig struct {
	ID       int
	Addr     string
	Password string
	SetID    int
	Weight   int
	Enabled  bool
}

// NodeRuntimeStatus 描述连接池内某个 FreeSWITCH 节点的运行状态快照。
// 用于运维监控和管理接口展示，包含连接状态、权重、是否启用等信息。
type NodeRuntimeStatus struct {
	ID        int    `json:"id,omitempty"`
	FSAddr    string `json:"fsAddr"`
	SetID     int    `json:"setId,omitempty"`
	Weight    int    `json:"weight,omitempty"`
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
}

// ConnectionPool 管理多个 FreeSWITCH ESL inbound 连接池。
// 每个 FS 地址维护独立的 eslgo.Conn 连接，支持断线重连和动态节点配置更新。
// 连接成功后会注册事件监听器，将事件通过 OnEvent 回调通知上层。
type ConnectionPool struct {
	ctx               context.Context
	mu                sync.RWMutex
	nodes             map[string]NodeConfig
	conns             map[string]*eslgo.Conn
	leaseCancels      map[string]context.CancelFunc
	eventCancels      map[string]context.CancelFunc
	logger            *slog.Logger
	reconnectInterval time.Duration
	maxReconnect      int
	LeaseRegistry     fsregistry.Registry
	LeaseOwner        string
	LeaseTTL          time.Duration
	dial              func(addr, password string, onDisconnect func()) (*eslgo.Conn, error)
	OnEvent           func(context.Context, contracts.TelephonyEvent)
	OnSofiaEvent      func(ctx context.Context, eventSubclass string, extension string)
}

// NewConnectionPool 创建 ESL 连接池。
func NewConnectionPool(ctx context.Context, nodes []NodeConfig, reconnectInterval time.Duration, maxReconnect int, logger *slog.Logger) *ConnectionPool {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	if reconnectInterval == 0 {
		reconnectInterval = 5 * time.Second
	}
	if maxReconnect == 0 {
		maxReconnect = 30
	}
	pool := &ConnectionPool{
		ctx:               ctx,
		nodes:             map[string]NodeConfig{},
		conns:             map[string]*eslgo.Conn{},
		leaseCancels:      map[string]context.CancelFunc{},
		eventCancels:      map[string]context.CancelFunc{},
		logger:            logger,
		reconnectInterval: reconnectInterval,
		maxReconnect:      maxReconnect,
		dial:              eslgo.Dial,
		LeaseTTL:          30 * time.Second,
	}
	for _, node := range nodes {
		if node.Enabled && node.Addr != "" {
			pool.nodes[node.Addr] = node
		}
	}
	return pool
}

// ConnectAll 连接所有已启用 FS 节点。
func (p *ConnectionPool) ConnectAll(ctx context.Context) error {
	for _, node := range p.SnapshotNodes() {
		if _, err := p.Connect(ctx, node.Addr); err != nil {
			if errors.Is(err, fsregistry.ErrLeaseHeld) {
				p.logger.Warn("FreeSWITCH 事件租约已被其他实例持有，跳过该节点", "fsAddr", node.Addr)
				continue
			}
			p.logger.Error("FreeSWITCH 节点连接失败", "fsAddr", node.Addr, "error", err.Error())
			return err
		}
	}
	return nil
}

// UpsertNode 新增或更新一个 FS 节点配置。
//
// 动态管理接口从数据库读取节点后调用这里，使生产环境不依赖静态 YAML。
func (p *ConnectionPool) UpsertNode(node NodeConfig) {
	if node.Addr == "" {
		return
	}
	if !node.Enabled {
		p.mu.Lock()
		delete(p.nodes, node.Addr)
		conn := p.conns[node.Addr]
		delete(p.conns, node.Addr)
		p.stopLeaseRenewalLocked(node.Addr)
		if cancel := p.eventCancels[node.Addr]; cancel != nil {
			cancel()
			delete(p.eventCancels, node.Addr)
		}
		p.mu.Unlock()
		p.releaseLease(node.Addr)
		if conn != nil {
			conn.ExitAndClose()
		}
		p.logger.Info("FreeSWITCH 节点已从连接池禁用", "fsAddr", node.Addr, "id", node.ID)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nodes[node.Addr] = node
	p.logger.Info("FreeSWITCH 节点已写入连接池", "fsAddr", node.Addr, "id", node.ID, "setId", node.SetID, "weight", node.Weight)
}

// RemoveNode 从连接池中移除指定节点并关闭其 ESL 连接。
func (p *ConnectionPool) RemoveNode(fsAddr string) {
	p.mu.Lock()
	delete(p.nodes, fsAddr)
	conn := p.conns[fsAddr]
	delete(p.conns, fsAddr)
	p.stopLeaseRenewalLocked(fsAddr)
	if cancel := p.eventCancels[fsAddr]; cancel != nil {
		cancel()
		delete(p.eventCancels, fsAddr)
	}
	p.mu.Unlock()
	p.releaseLease(fsAddr)
	if conn != nil {
		conn.ExitAndClose()
	}
	p.logger.Info("FreeSWITCH 节点已从连接池移除", "fsAddr", fsAddr)
}

// Status 返回连接池内所有节点的当前运行状态快照。
func (p *ConnectionPool) Status() []NodeRuntimeStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	statuses := make([]NodeRuntimeStatus, 0, len(p.nodes))
	for fsAddr, node := range p.nodes {
		statuses = append(statuses, NodeRuntimeStatus{
			ID:        node.ID,
			FSAddr:    fsAddr,
			SetID:     node.SetID,
			Weight:    node.Weight,
			Enabled:   node.Enabled,
			Connected: p.conns[fsAddr] != nil,
		})
	}
	return statuses
}

// Connect 建立或复用指定 FS 地址的 ESL 连接。
// 使用 double-check locking 避免并发 Connect 同时建立连接的竞态条件。
func (p *ConnectionPool) Connect(ctx context.Context, fsAddr string) (*eslgo.Conn, error) {
	// 快速路径：已有连接时直接返回
	p.mu.RLock()
	if conn := p.conns[fsAddr]; conn != nil {
		p.mu.RUnlock()
		return conn, nil
	}
	p.mu.RUnlock()

	// 获取写锁，double-check 后建立连接
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check: 可能在等锁期间被其他 goroutine 建立
	if conn := p.conns[fsAddr]; conn != nil {
		return conn, nil
	}

	node, ok := p.nodes[fsAddr]
	if !ok {
		return nil, ErrFSNodeNotConfigured
	}

	disconnect := func() {
		p.mu.Lock()
		delete(p.conns, fsAddr)
		if cancel := p.eventCancels[fsAddr]; cancel != nil {
			cancel()
			delete(p.eventCancels, fsAddr)
		}
		p.stopLeaseRenewalLocked(fsAddr)
		p.mu.Unlock()
		p.releaseLease(fsAddr)
		p.logger.Warn("FreeSWITCH ESL 连接断开，准备重连", "fsAddr", fsAddr)
		go p.reconnect(p.ctx, node)
	}

	conn, err := p.dial(node.Addr, node.Password, disconnect)
	if err != nil {
		return nil, err
	}
	if err := conn.EnableEvents(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	if err := p.claimLease(ctx, fsAddr); err != nil {
		conn.Close()
		return nil, err
	}

	// 事件监听使用独立的 context，不受 Connect 调用方 context 生命周期影响
	eventCtx, eventCancel := context.WithCancel(context.Background())
	if existing := p.eventCancels[fsAddr]; existing != nil {
		existing()
	}
	p.eventCancels[fsAddr] = eventCancel

	conn.RegisterEventListener(eslgo.EventListenAll, func(event *eslgo.Event) {
		eventName := event.Headers.Get("Event-Name")
		if eventName == "CUSTOM" {
			subclass := event.Headers.Get("Event-Subclass")
			if subclass == "sofia::register" || subclass == "sofia::unregister" || subclass == "sofia::expire" {
				ext := event.Headers.Get("user")
				if ext == "" {
					ext = event.Headers.Get("from-user")
				}
				if ext != "" {
					p.logger.Info("收到 FreeSWITCH Sofia 注册事件", "fsAddr", fsAddr, "subclass", subclass, "extension", ext)
					if p.OnSofiaEvent != nil {
						p.OnSofiaEvent(eventCtx, subclass, ext)
					}
				}
			}
		}

		domainEvent := EventFromESL(fsAddr, event)
		if domainEvent.CallID != "" {
			p.logger.Info("收到 FreeSWITCH 事件", "eventId", domainEvent.EventID, "eventName", domainEvent.EventName, "callId", domainEvent.CallID, "uuid", domainEvent.UUID, "fsAddr", domainEvent.FSAddr, "legRole", domainEvent.LegRole)
			if p.OnEvent != nil {
				p.OnEvent(eventCtx, domainEvent)
			}
		}
	})
	p.conns[fsAddr] = conn
	p.startLeaseRenewal(fsAddr)
	p.logger.Info("FreeSWITCH ESL 连接已成功并启用事件", "fsAddr", fsAddr)
	return conn, nil
}

// SendAPI 向指定 FS 节点发送 api/bgapi 命令。
func (p *ConnectionPool) SendAPI(ctx context.Context, fsAddr, commandName, args string, background bool) error {
	conn, err := p.Connect(ctx, fsAddr)
	if err != nil {
		return err
	}
	p.logger.Info("发送 FreeSWITCH API 命令", "fsAddr", fsAddr, "command", commandName, "background", background)
	_, err = conn.SendCommand(ctx, command.API{Command: commandName, Arguments: args, Background: background})
	if err != nil {
		p.logger.Error("FreeSWITCH API 命令发送失败", "fsAddr", fsAddr, "command", commandName, "error", err.Error())
		return err
	}
	p.logger.Info("FreeSWITCH API 命令发送成功", "fsAddr", fsAddr, "command", commandName)
	return nil
}

// QueryChannels 向指定的 FreeSWITCH 节点发送 "show channels" 命令查询所有活跃通道。
// 返回命令执行后的原始文本 Body 响应。
func (p *ConnectionPool) QueryChannels(ctx context.Context, fsAddr string) (string, error) {
	conn, err := p.Connect(ctx, fsAddr)
	if err != nil {
		return "", err
	}
	p.logger.Info("向 FreeSWITCH 查询活跃通道", "fsAddr", fsAddr)
	resp, err := conn.SendCommand(ctx, command.API{Command: "show", Arguments: "channels", Background: false})
	if err != nil {
		p.logger.Error("向 FreeSWITCH 查询活跃通道失败", "fsAddr", fsAddr, "error", err.Error())
		return "", err
	}
	return string(resp.Body), nil
}

// CloseAll 关闭所有 ESL 连接。
func (p *ConnectionPool) CloseAll() {
	p.mu.Lock()
	conns := make(map[string]*eslgo.Conn, len(p.conns))
	for fsAddr, conn := range p.conns {
		conns[fsAddr] = conn
		delete(p.conns, fsAddr)
		p.stopLeaseRenewalLocked(fsAddr)
		if cancel := p.eventCancels[fsAddr]; cancel != nil {
			cancel()
			delete(p.eventCancels, fsAddr)
		}
	}
	p.mu.Unlock()
	for fsAddr, conn := range conns {
		p.logger.Info("关闭 FreeSWITCH ESL 连接", "fsAddr", fsAddr)
		p.releaseLease(fsAddr)
		conn.ExitAndClose()
	}
}

// reconnect 实现指定节点的指数退避重连逻辑。
// 重连成功或达到最大重试次数后退出。当前实现为线性重试。
func (p *ConnectionPool) reconnect(ctx context.Context, node NodeConfig) {
	for attempt := 1; attempt <= p.maxReconnect; attempt++ {
		select {
		case <-ctx.Done():
			p.logger.Info("FreeSWITCH ESL 重连停止", "fsAddr", node.Addr, "reason", ctx.Err().Error())
			return
		case <-time.After(p.reconnectInterval):
		}
		p.logger.Info("尝试重连 FreeSWITCH ESL", "fsAddr", node.Addr, "attempt", attempt)
		if _, err := p.Connect(ctx, node.Addr); err == nil {
			p.logger.Info("FreeSWITCH ESL 重连成功", "fsAddr", node.Addr, "attempt", attempt)
			return
		} else if errors.Is(err, fsregistry.ErrLeaseHeld) {
			p.logger.Warn("FreeSWITCH 事件租约仍被其他实例持有，停止当前重连轮次", "fsAddr", node.Addr, "attempt", attempt)
			return
		}
	}
	p.logger.Error("FreeSWITCH ESL 达到最大重连次数", "fsAddr", node.Addr, "maxAttempts", p.maxReconnect)
}

// SnapshotNodes 返回当前连接池中的所有节点配置快照。
func (p *ConnectionPool) SnapshotNodes() []NodeConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	nodes := make([]NodeConfig, 0, len(p.nodes))
	for _, node := range p.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (p *ConnectionPool) claimLease(ctx context.Context, fsAddr string) error {
	if p.LeaseRegistry == nil {
		return nil
	}
	owner := p.leaseOwner()
	ttl := p.leaseTTL()
	node, err := p.LeaseRegistry.ClaimEvents(ctx, fsAddr, owner, ttl)
	if err != nil {
		return err
	}
	p.logger.Info("FreeSWITCH 事件租约声明成功", "fsAddr", fsAddr, "owner", owner, "leaseExpires", node.LeaseExpires)
	return nil
}

func (p *ConnectionPool) startLeaseRenewal(fsAddr string) {
	if p.LeaseRegistry == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	if existing := p.leaseCancels[fsAddr]; existing != nil {
		existing()
	}
	p.leaseCancels[fsAddr] = cancel
	p.mu.Unlock()
	interval := p.leaseRenewInterval()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := p.claimLease(context.Background(), fsAddr); err != nil {
					p.logger.Error("FreeSWITCH 事件租约续约失败，关闭事件连接避免重复消费", "fsAddr", fsAddr, "owner", p.leaseOwner(), "error", err.Error())
					p.closeForLeaseFailure(fsAddr)
					return
				}
			}
		}
	}()
}

func (p *ConnectionPool) stopLeaseRenewalLocked(fsAddr string) {
	if cancel := p.leaseCancels[fsAddr]; cancel != nil {
		cancel()
		delete(p.leaseCancels, fsAddr)
	}
}

func (p *ConnectionPool) closeForLeaseFailure(fsAddr string) {
	p.mu.Lock()
	conn := p.conns[fsAddr]
	delete(p.conns, fsAddr)
	p.stopLeaseRenewalLocked(fsAddr)
	if cancel := p.eventCancels[fsAddr]; cancel != nil {
		cancel()
		delete(p.eventCancels, fsAddr)
	}
	p.mu.Unlock()
	if conn != nil {
		conn.ExitAndClose()
	}
}

func (p *ConnectionPool) releaseLease(fsAddr string) {
	if p.LeaseRegistry == nil {
		return
	}
	owner := p.leaseOwner()
	if err := p.LeaseRegistry.ReleaseEvents(context.Background(), fsAddr, owner); err != nil {
		p.logger.Warn("FreeSWITCH 事件租约释放失败", "fsAddr", fsAddr, "owner", owner, "error", err.Error())
		return
	}
	p.logger.Info("FreeSWITCH 事件租约已释放", "fsAddr", fsAddr, "owner", owner)
}

func (p *ConnectionPool) leaseOwner() string {
	if p.LeaseOwner != "" {
		return p.LeaseOwner
	}
	return "cc-call-local"
}

func (p *ConnectionPool) leaseTTL() time.Duration {
	if p.LeaseTTL > 0 {
		return p.LeaseTTL
	}
	return 30 * time.Second
}

func (p *ConnectionPool) leaseRenewInterval() time.Duration {
	ttl := p.leaseTTL()
	if ttl <= 2*time.Second {
		return time.Second
	}
	return ttl / 2
}

// ESLCommandExecutor 是 FreeSWITCH 命令执行器。
// 它将领域层 TelephonyCommand 通过 command_builder 转换为 ESL 命令并通过连接池发送。
type ESLCommandExecutor struct {
	Pool    *ConnectionPool
	Timeout time.Duration
	Logger  *slog.Logger
}

// Execute 将通话控制命令转换为 ESL 命令并发送到对应 FS 节点。
// 命令应已通过领域层幂等性校验和业务状态验证。
func (e *ESLCommandExecutor) Execute(ctx context.Context, cmd contracts.TelephonyCommand) error {
	timeout := e.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	logger := e.Logger
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	apiName, args, background := BuildAPICommand(cmd)
	logger.Info("准备执行真实 FreeSWITCH 命令", append(logging.TelephonyAttrs(cmd), slog.String("apiCommand", apiName), slog.Bool("background", background))...)
	return e.Pool.SendAPI(ctx, cmd.FSAddr, apiName, args, background)
}
