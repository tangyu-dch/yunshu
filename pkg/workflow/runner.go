// Package workflow 提供轻量级事件驱动流程编排内核。
//
// 领域模块通过声明状态、事件、步骤和处理器来推进业务，避免把长流程写成分散在
// Controller、Consumer、Scheduler 里的大段 if/else。
package workflow

import (
	"context"
	"errors"
	"log/slog"

	"yunshu/internal/infra/logging"
)

// Runner 把流程引擎和实例存储组合成可消费事件的运行器。
type Runner struct {
	Engine *Engine
	Store  InstanceStore
	Logger *slog.Logger
}

// NewRunner 创建流程运行器。
func NewRunner(engine *Engine, store InstanceStore, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{Engine: engine, Store: store, Logger: logger}
}

// Start 创建并保存流程实例。
func (r *Runner) Start(ctx context.Context, workflowID, instanceID string) (Instance, error) {
	instance, err := r.Engine.Start(workflowID, instanceID)
	if err != nil {
		return Instance{}, err
	}
	if err := r.Store.Save(ctx, instance); err != nil {
		return Instance{}, err
	}
	r.Logger.Info("流程实例已保存", logging.WorkflowAttrs(workflowID, instanceID, string(instance.State), "start")...)
	return instance, nil
}

// Apply 读取流程实例、应用事件并保存最新状态。
func (r *Runner) Apply(ctx context.Context, workflowID, instanceID string, event Event) (Instance, error) {
	instance, err := r.Store.Get(ctx, workflowID, instanceID)
	if errors.Is(err, ErrInstanceNotFound) {
		instance, err = r.Start(ctx, workflowID, instanceID)
	}
	if err != nil {
		return Instance{}, err
	}
	if err := r.Engine.Apply(ctx, &instance, event); err != nil {
		return instance, err
	}
	if err := r.Store.Save(ctx, instance); err != nil {
		return instance, err
	}
	r.Logger.Info("流程实例状态已保存", logging.WorkflowAttrs(workflowID, instanceID, string(instance.State), string(event.Name))...)
	return instance, nil
}
