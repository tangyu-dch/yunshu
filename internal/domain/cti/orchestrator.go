package cti

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"yunshu/internal/domain/outbox"
	"yunshu/internal/infra/limit"
	"yunshu/internal/infra/logging"
	"yunshu/pkg/idempotency"
)

var ErrDuplicateAllocation = errors.New("duplicate allocation command")

type AllocationRequest struct {
	CommandID  string
	CallID     string
	MerchantID string
	Callee     string
	SkillGroup string
	Candidates []NumberCandidate
}

type AllocationResult struct {
	Selection SelectionResult
	OutboxID  string
}

// AllocationService 编排 CTI 起呼前的资源分配流程。
// 当前包含命令幂等、选号、商户并发槽位和 outbox 投递；后续接入 Redis/DB
// 时保持这个用例入口不变，只替换底层 Store/ limiter/outbox 实现。
type AllocationService struct {
	Selector    Selector
	Idempotency idempotency.Store
	Limiter     *limit.ShardedLimiter
	Outbox      outbox.Store
	Now         func() time.Time
	Logger      *slog.Logger
}

// NewAllocationService 创建资源分配服务。
func NewAllocationService(idem idempotency.Store, limiter *limit.ShardedLimiter, outboxStore outbox.Store) *AllocationService {
	return &AllocationService{
		Selector:    Selector{},
		Idempotency: idem,
		Limiter:     limiter,
		Outbox:      outboxStore,
		Now:         time.Now,
		Logger:      slog.Default(),
	}
}

// Allocate 为一次外呼分配主叫号码和并发槽位，并写入待投递的 ESL 起呼 outbox。
// 业务失败会释放幂等占位，便于修正资源后重试；已成功的 commandId 会被拒绝重复执行。
func (s *AllocationService) Allocate(ctx context.Context, req AllocationRequest) (AllocationResult, error) {
	attrs := logging.AllocationAttrs(req.CommandID, req.CallID, req.MerchantID, req.SkillGroup)
	s.Logger.Info("开始 CTI 外呼资源分配", attrs...)
	claimed, err := s.Idempotency.Claim(ctx, "cti:allocation:"+req.CommandID, 10*time.Minute)
	if err != nil {
		s.Logger.Error("CTI 外呼资源分配幂等占位失败", append(attrs, slog.String("error", err.Error()))...)
		return AllocationResult{}, err
	}
	if !claimed {
		s.Logger.Info("跳过重复 CTI 外呼资源分配命令", attrs...)
		return AllocationResult{}, ErrDuplicateAllocation
	}

	result := s.Selector.Select(ctx, SelectionRequest{
		CallID:     req.CallID,
		MerchantID: req.MerchantID,
		Callee:     req.Callee,
		SkillGroup: req.SkillGroup,
		Candidates: req.Candidates,
	})
	if !result.Success {
		_ = s.Idempotency.Release(ctx, "cti:allocation:"+req.CommandID)
		s.Logger.Warn("CTI 选号失败，释放幂等占位等待后续重试", append(attrs, slog.String("reason", result.Reason), slog.Int("traceCount", len(result.Trace)))...)
		return AllocationResult{Selection: result}, ErrNoAvailableNumber
	}
	s.Logger.Info("CTI 选号成功", append(attrs, slog.String("gatewayId", result.Caller.GatewayID), slog.Int("traceCount", len(result.Trace)))...)

	limitKey := "merchant:" + req.MerchantID
	if !s.Limiter.Acquire(ctx, limitKey, 1) {
		_ = s.Idempotency.Release(ctx, "cti:allocation:"+req.CommandID)
		s.Logger.Warn("CTI 商户并发槽位不足，释放幂等占位", append(attrs, slog.String("limitKey", limitKey))...)
		return AllocationResult{Selection: result}, fmt.Errorf("%w: %s", ErrNoAvailableNumber, "merchant concurrency exhausted")
	}
	s.Logger.Info("CTI 商户并发槽位占用成功", append(attrs, slog.String("limitKey", limitKey), slog.Int("usedSlots", s.Limiter.Used(ctx, limitKey)))...)

	outboxID := "originate:" + req.CallID
	if err := s.Outbox.Append(ctx, outbox.Entry{
		ID:             outboxID,
		AggregateType:  "call",
		AggregateID:    req.CallID,
		Destination:    "cc-call.esl.originate",
		IdempotencyKey: req.CommandID,
		Payload: map[string]any{
			"callId":     req.CallID,
			"merchantId": req.MerchantID,
			"callee":     req.Callee,
			"caller":     result.Caller.Phone,
			"gatewayId":  result.Caller.GatewayID,
		},
		NextAttemptAt: s.Now().UTC(),
	}); err != nil {
		s.Limiter.Release(ctx, limitKey, 1)
		_ = s.Idempotency.Release(ctx, "cti:allocation:"+req.CommandID)
		s.Logger.Error("CTI 起呼 outbox 写入失败，已释放并发槽位和幂等占位", append(attrs, slog.String("outboxId", outboxID), slog.String("error", err.Error()))...)
		return AllocationResult{}, err
	}
	s.Logger.Info("CTI 起呼 outbox 写入成功", append(attrs, slog.String("outboxId", outboxID))...)

	return AllocationResult{Selection: result, OutboxID: outboxID}, nil
}
