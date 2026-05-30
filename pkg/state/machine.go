// Package state 提供泛型有限状态机实现。
//
// 适用于短生命周期、强约束的状态迁移场景，如通话状态机、任务状态机。
// 对于跨服务长流程，应优先使用 pkg/workflow 包。
package state

import "fmt"

// Machine 是一个泛型有限状态机。
// 它用于短生命周期、强约束的状态迁移；跨服务长流程优先使用 pkg/workflow。
type Machine[S comparable, E comparable] struct {
	current     S
	transitions map[S]map[E]S
}

// NewMachine 创建状态机实例。
func NewMachine[S comparable, E comparable](initial S, transitions map[S]map[E]S) *Machine[S, E] {
	return &Machine[S, E]{current: initial, transitions: transitions}
}

// State 返回当前状态。
func (m *Machine[S, E]) State() S {
	return m.current
}

// Apply 应用事件并推进状态；非法迁移会返回错误，调用方应记录并拒绝副作用。
func (m *Machine[S, E]) Apply(event E) (S, error) {
	nextByEvent, ok := m.transitions[m.current]
	if !ok {
		return m.current, fmt.Errorf("state %v has no transitions", m.current)
	}
	next, ok := nextByEvent[event]
	if !ok {
		return m.current, fmt.Errorf("event %v is invalid for state %v", event, m.current)
	}
	m.current = next
	return next, nil
}
