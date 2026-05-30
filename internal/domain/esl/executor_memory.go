package esl

import (
	"context"
	"log/slog"
	"sync"

	"yunshu/internal/contracts"
	"yunshu/internal/infra/logging"
)

// MemoryCommandExecutor 是本地开发和单元测试使用的命令执行器。
// 它不会连接真实 FreeSWITCH，只记录命令；真实 adapter 后续实现 CommandExecutor 即可替换。
type MemoryCommandExecutor struct {
	mu       sync.Mutex
	Commands []contracts.TelephonyCommand
	Logger   *slog.Logger
}

// Execute 记录一条已通过校验和幂等的 ESL 控制命令。
func (e *MemoryCommandExecutor) Execute(_ context.Context, command contracts.TelephonyCommand) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Commands = append(e.Commands, command)
	logger := e.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("内存执行器记录通话控制命令", logging.TelephonyAttrs(command)...)
	return nil
}

// Count 返回已记录命令数量。
func (e *MemoryCommandExecutor) Count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.Commands)
}
