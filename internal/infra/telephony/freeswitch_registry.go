package telephony

import (
	"yunshu/internal/domain/operate"
)

type NodeStatus = operate.NodeStatus

const (
	NodeUnknown     NodeStatus = operate.NodeUnknown
	NodeActive      NodeStatus = operate.NodeActive
	NodeDraining    NodeStatus = operate.NodeDraining
	NodeUnavailable NodeStatus = operate.NodeUnavailable
)

type Node = operate.Node
type Registry = operate.Registry

var (
	ErrNodeNotFound = operate.ErrNodeNotFound
	ErrLeaseHeld    = operate.ErrLeaseHeld
)

// MemoryRegistry 是基于内存的 FreeSWITCH 节点注册表实现。
type MemoryRegistry = operate.MemoryRegistry

// NewMemoryRegistry 创建内存注册表实例。
func NewMemoryRegistry() *MemoryRegistry {
	return operate.NewMemoryRegistry()
}
