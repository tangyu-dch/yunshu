package operate

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// MemoryRegistry 是基于内存的 FreeSWITCH 节点注册表实现。
// 适用于测试和本地开发。
type MemoryRegistry struct {
	mu    sync.Mutex
	nodes map[string]Node
	now   func() time.Time
}

// NewMemoryRegistry 创建内存注册表实例。
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{nodes: map[string]Node{}, now: time.Now}
}

// Upsert 保存或更新节点配置，会自动规范化 FS 地址格式。
func (r *MemoryRegistry) Upsert(_ context.Context, node Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	node.FSAddr = normalizeFSAddr(node)
	if node.CommandURL == "" {
		node.CommandURL = normalizeCommandURL(node)
	}
	node.UpdatedAt = r.now().UTC()
	if node.Status == "" {
		node.Status = NodeUnknown
	}
	r.nodes[node.FSAddr] = node
	return nil
}

func (r *MemoryRegistry) Get(_ context.Context, fsAddr string) (Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	node, ok := r.nodes[fsAddr]
	if !ok {
		return Node{}, ErrNodeNotFound
	}
	return node, nil
}

func (r *MemoryRegistry) GetByID(_ context.Context, id int) (Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, node := range r.nodes {
		if node.ID == id {
			return node, nil
		}
	}
	return Node{}, ErrNodeNotFound
}

// List 返回所有节点。
func (r *MemoryRegistry) List(_ context.Context) ([]Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	nodes := make([]Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (r *MemoryRegistry) ListEnabled(_ context.Context) ([]Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	nodes := make([]Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		if node.Enable {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (r *MemoryRegistry) Delete(_ context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for fsAddr, node := range r.nodes {
		if node.ID == id {
			delete(r.nodes, fsAddr)
			return nil
		}
	}
	return ErrNodeNotFound
}

// ClaimEvents 尝试获取指定节点的事件消费租约。
func (r *MemoryRegistry) ClaimEvents(_ context.Context, fsAddr, owner string, ttl time.Duration) (Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	node, ok := r.nodes[fsAddr]
	if !ok {
		return Node{}, ErrNodeNotFound
	}
	now := r.now().UTC()
	if node.EventOwner != "" && node.EventOwner != owner && node.LeaseExpires.After(now) {
		return node, ErrLeaseHeld
	}
	node.EventOwner = owner
	node.LeaseExpires = now.Add(ttl)
	node.UpdatedAt = now
	r.nodes[fsAddr] = node
	return node, nil
}

func (r *MemoryRegistry) ReleaseEvents(_ context.Context, fsAddr, owner string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	node, ok := r.nodes[fsAddr]
	if !ok {
		return ErrNodeNotFound
	}
	if node.EventOwner == owner {
		node.EventOwner = ""
		node.LeaseExpires = time.Time{}
		node.UpdatedAt = r.now().UTC()
		r.nodes[fsAddr] = node
	}
	return nil
}

// normalizeFSAddr 规范化节点地址格式。
func normalizeFSAddr(node Node) string {
	if node.FSAddr != "" {
		return node.FSAddr
	}
	if node.Address != "" && node.ESLPort > 0 {
		return node.Address + ":" + strconv.Itoa(node.ESLPort)
	}
	return node.FSAddr
}

func normalizeCommandURL(node Node) string {
	if node.CmdPort <= 0 {
		return ""
	}
	address := node.Address
	if address == "" {
		address, _, _ = splitAddressPort(node.FSAddr)
	}
	if address == "" {
		return ""
	}
	return address + ":" + strconv.Itoa(node.CmdPort)
}

func splitAddressPort(value string) (string, int, bool) {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] != ':' {
			continue
		}
		port, err := strconv.Atoi(value[i+1:])
		if err != nil {
			return "", 0, false
		}
		return value[:i], port, true
	}
	return "", 0, false
}
