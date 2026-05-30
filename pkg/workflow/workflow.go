// Package workflow 提供轻量级事件驱动流程编排内核。
//
// 领域模块通过声明状态、事件、步骤和处理器来推进业务，避免把长流程写成分散在
// Controller、Consumer、Scheduler 里的大段 if/else。
package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"yunshu/internal/infra/logging"
)

var (
	ErrWorkflowNotFound  = errors.New("workflow not found")
	ErrTransitionMissing = errors.New("workflow transition missing")
	ErrHandlerMissing    = errors.New("workflow handler missing")
)

type State string
type EventName string
type StepName string

type Event struct {
	Name    EventName
	Payload map[string]any
}

// Instance 表示一个正在运行的流程实例。
// 生产环境需要把实例状态持久化，避免服务重启后丢失长流程进度。
type Instance struct {
	ID         string
	WorkflowID string
	State      State
	Variables  map[string]any
}

type Handler func(context.Context, *Instance, Event) error

type Transition struct {
	From State
	On   EventName
	To   State
	Step StepName
}

type Definition struct {
	ID          string
	Initial     State
	Transitions []Transition
	Handlers    map[StepName]Handler
}

// Engine 保存流程定义并负责根据事件推进实例状态。
type Engine struct {
	definitions map[string]Definition
}

// NewEngine 创建流程引擎。流程定义在启动时注册，运行期只按 workflowID 查找。
func NewEngine(definitions ...Definition) (*Engine, error) {
	engine := &Engine{definitions: map[string]Definition{}}
	for _, definition := range definitions {
		if definition.ID == "" {
			return nil, fmt.Errorf("workflow id is required")
		}
		engine.definitions[definition.ID] = definition
	}
	return engine, nil
}

// Start 创建流程实例，并使用定义中的初始状态。
func (e *Engine) Start(workflowID, instanceID string) (Instance, error) {
	definition, ok := e.definitions[workflowID]
	if !ok {
		slog.Error("流程启动失败，流程定义不存在", "workflowId", workflowID, "workflowInstanceId", instanceID)
		return Instance{}, ErrWorkflowNotFound
	}
	slog.Info("流程实例启动", logging.WorkflowAttrs(workflowID, instanceID, string(definition.Initial), "start")...)
	return Instance{ID: instanceID, WorkflowID: workflowID, State: definition.Initial, Variables: map[string]any{}}, nil
}

// Apply 根据事件推进流程实例。
// 如果 transition 绑定了 Step，会先执行 Step，成功后再变更状态。
func (e *Engine) Apply(ctx context.Context, instance *Instance, event Event) error {
	definition, ok := e.definitions[instance.WorkflowID]
	if !ok {
		slog.Error("流程事件处理失败，流程定义不存在", logging.WorkflowAttrs(instance.WorkflowID, instance.ID, string(instance.State), string(event.Name))...)
		return ErrWorkflowNotFound
	}
	for _, transition := range definition.Transitions {
		if transition.From == instance.State && transition.On == event.Name {
			slog.Info("开始处理流程事件", logging.WorkflowAttrs(instance.WorkflowID, instance.ID, string(instance.State), string(event.Name))...)
			if transition.Step != "" {
				handler, ok := definition.Handlers[transition.Step]
				if !ok {
					slog.Error("流程事件处理失败，步骤处理器不存在", append(logging.WorkflowAttrs(instance.WorkflowID, instance.ID, string(instance.State), string(event.Name)), slog.String("step", string(transition.Step)))...)
					return ErrHandlerMissing
				}
				if err := handler(ctx, instance, event); err != nil {
					slog.Error("流程步骤执行失败", append(logging.WorkflowAttrs(instance.WorkflowID, instance.ID, string(instance.State), string(event.Name)), slog.String("step", string(transition.Step)), slog.String("error", err.Error()))...)
					return err
				}
			}
			instance.State = transition.To
			slog.Info("流程事件处理完成", logging.WorkflowAttrs(instance.WorkflowID, instance.ID, string(instance.State), string(event.Name))...)
			return nil
		}
	}
	slog.Warn("流程事件没有匹配的状态迁移", logging.WorkflowAttrs(instance.WorkflowID, instance.ID, string(instance.State), string(event.Name))...)
	return ErrTransitionMissing
}
