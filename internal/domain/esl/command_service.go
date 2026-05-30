package esl

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/infra/logging"
	"yunshu/pkg/idempotency"
)

var (
	ErrInvalidCommand   = errors.New("invalid telephony command")
	ErrDuplicateCommand = errors.New("duplicate telephony command")
)

type CommandExecutor interface {
	Execute(ctx context.Context, command contracts.TelephonyCommand) error
}

// CommandService 负责 ESL 控制命令的统一入口。
// 它先校验追踪字段，再做命令幂等，最后委托具体执行器连接 FreeSWITCH。
// 执行失败会释放幂等占位，允许上游按 commandId 进行安全重试。
type CommandService struct {
	Validator   CommandValidator
	Idempotency idempotency.Store
	Executor    CommandExecutor
	Logger      *slog.Logger
}

// NewCommandService 创建命令服务。executor 可以是真实 ESL 连接，也可以是测试替身。
func NewCommandService(store idempotency.Store, executor CommandExecutor, logger *slog.Logger) *CommandService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CommandService{
		Validator:   CommandValidator{},
		Idempotency: store,
		Executor:    executor,
		Logger:      logger,
	}
}

// Execute 执行一条可追踪、可幂等的通话控制命令。
func (s *CommandService) Execute(ctx context.Context, cmd contracts.TelephonyCommand) error {
	if !s.Validator.Validate(cmd) {
		s.Logger.Warn("拒绝缺少追踪字段的通话控制命令", logging.TelephonyAttrs(cmd)...)
		return ErrInvalidCommand
	}
	claimed, err := s.Idempotency.Claim(ctx, "esl:command:"+cmd.CommandID, 10*time.Minute)
	if err != nil {
		return err
	}
	if !claimed {
		s.Logger.Info("跳过重复通话控制命令", logging.TelephonyAttrs(cmd)...)
		return ErrDuplicateCommand
	}
	if err := s.Executor.Execute(ctx, cmd); err != nil {
		_ = s.Idempotency.Release(ctx, "esl:command:"+cmd.CommandID)
		s.Logger.Error("通话控制命令执行失败", append(logging.TelephonyAttrs(cmd), slog.String("error", err.Error()))...)
		return err
	}
	s.Logger.Info("通话控制命令执行成功", logging.TelephonyAttrs(cmd)...)
	return nil
}
