package operate

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNodeNotFound 表示请求的 FreeSWITCH 节点在注册表中不存在。
	ErrNodeNotFound = errors.New("freeswitch node not found")
	// ErrLeaseHeld 表示该节点的事件消费租约已被其他实例持有。
	ErrLeaseHeld = errors.New("freeswitch event lease is held by another owner")
)

// NodeStatus 表示 FreeSWITCH 节点的运行状态枚举。
type NodeStatus string

// 节点状态常量：未知、活跃、排空中、不可用。
const (
	NodeUnknown     NodeStatus = "unknown"
	NodeActive      NodeStatus = "active"
	NodeDraining    NodeStatus = "draining"
	NodeUnavailable NodeStatus = "unavailable"
)

// Node 表示 FreeSWITCH 节点在注册表中的完整视图。
type Node struct {
	ID           int        `json:"id,omitempty"`
	FSAddr       string     `json:"fsAddr"`
	Name         string     `json:"name"`
	Address      string     `json:"address,omitempty"`
	LocalAddress string     `json:"localAddress,omitempty"`
	ESLPort      int        `json:"eslPort,omitempty"`
	SIPPort      int        `json:"sipPort,omitempty"`
	CmdPort      int        `json:"cmdPort,omitempty"`
	Password     string     `json:"password,omitempty"`
	SetID        int        `json:"setId,omitempty"`
	Weight       int        `json:"weight,omitempty"`
	RWeight      int        `json:"rweight,omitempty"`
	CC           int        `json:"cc,omitempty"`
	Canary       bool       `json:"canary,omitempty"`
	Enable       bool       `json:"enable"`
	Status       NodeStatus `json:"status"`
	CommandURL   string     `json:"commandUrl"`
	MaxChannels  int        `json:"maxChannels"`
	ActiveCalls  int        `json:"activeCalls"`
	EventOwner   string     `json:"eventOwner,omitempty"`
	LeaseExpires time.Time  `json:"leaseExpires,omitempty"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// Registry 定义 FS 节点注册表能力。
type Registry interface {
	Upsert(ctx context.Context, node Node) error
	Get(ctx context.Context, fsAddr string) (Node, error)
	GetByID(ctx context.Context, id int) (Node, error)
	List(ctx context.Context) ([]Node, error)
	ListEnabled(ctx context.Context) ([]Node, error)
	Delete(ctx context.Context, id int) error
	ClaimEvents(ctx context.Context, fsAddr, owner string, ttl time.Duration) (Node, error)
	ReleaseEvents(ctx context.Context, fsAddr, owner string) error
}
