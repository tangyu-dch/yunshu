package cti

import (
	"context"
	"errors"
	"log/slog"
)

// ErrRuntimeConcurrencyExhausted 表示运行时选号占用时候选号码并发已满。
var ErrRuntimeConcurrencyExhausted = errors.New("runtime number concurrency exhausted")

// ErrRuntimeAllocatorNotConfigured 表示运行时选号分配器未配置。
var ErrRuntimeAllocatorNotConfigured = errors.New("runtime number allocator not configured")

// RuntimeAllocation 表示运行时已成功占用的主叫号码资源。
type RuntimeAllocation struct {
	CallID     string `json:"callId"`
	MerchantID string `json:"merchantId"`
	Caller     string `json:"caller"`
	GatewayID  string `json:"gatewayId"`
	ClaimKey   string `json:"claimKey"`
}

// RuntimeAllocator 定义高并发选号的运行时原子分配能力。
//
// 数据库可以作为号码、网关、规则的配置源，但热路径必须通过 Redis/Lua 等原子
// 结构完成并发占用和幂等，避免每次呼叫都直接锁表或扫描大表。
type RuntimeAllocator interface {
	Claim(ctx context.Context, req SelectionRequest, candidates []NumberCandidate) (RuntimeAllocation, error)
	Release(ctx context.Context, allocation RuntimeAllocation) error
}

// RuntimeSelector 用规则链过滤候选号，再通过 RuntimeAllocator 原子占用资源。
type RuntimeSelector struct {
	RuleSelector Selector
	Allocator    RuntimeAllocator
	Marker       CandidateMarker
	Logger       *slog.Logger
}

// SelectAndClaim 执行选号过滤和运行时占用。
func (s RuntimeSelector) SelectAndClaim(ctx context.Context, req SelectionRequest) (SelectionResult, *RuntimeAllocation, error) {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if s.Marker != nil {
		marked, err := s.Marker.MarkCandidates(ctx, req, req.Candidates)
		if err != nil {
			logger.Error("运行时选号标记候选失败", "callId", req.CallID, "merchantId", req.MerchantID, "error", err.Error())
			return SelectionResult{Success: false, Reason: "标记选号候选失败: " + err.Error()}, nil, err
		}
		req.Candidates = marked
	}
	eligible, trace := eligibleCandidates(req)
	if len(eligible) == 0 {
		result := SelectionResult{Success: false, Reason: ErrNoAvailableNumber.Error(), Trace: trace}
		return result, nil, ErrNoAvailableNumber
	}
	if s.Allocator == nil {
		logger.Error("CTI 选号未配置运行时 allocator，直接拒绝起呼", "callId", req.CallID, "merchantId", req.MerchantID, "impact", "生产环境必须配置 Redis 原子分配")
		return SelectionResult{Success: false, Reason: ErrRuntimeAllocatorNotConfigured.Error(), Trace: trace}, nil, ErrRuntimeAllocatorNotConfigured
	}
	for i, candidate := range eligible {
		result := SelectionResult{Success: true, Caller: &candidate, Trace: trace}
		allocation, err := s.Allocator.Claim(ctx, req, []NumberCandidate{candidate})
		if err == nil {
			logger.Info("CTI 运行时号码占用成功", "callId", req.CallID, "merchantId", req.MerchantID, "caller", allocation.Caller, "gatewayId", allocation.GatewayID, "claimKey", allocation.ClaimKey, "candidateIndex", i)
			return result, &allocation, nil
		}
		if errors.Is(err, ErrRuntimeConcurrencyExhausted) {
			logger.Warn("CTI 候选号运行时并发已满，继续尝试下一个候选", "callId", req.CallID, "merchantId", req.MerchantID, "candidateIndex", i, "gatewayId", candidate.GatewayID)
			continue
		}
		logger.Warn("CTI 运行时号码占用失败", "callId", req.CallID, "merchantId", req.MerchantID, "candidateIndex", i, "error", err.Error())
		return SelectionResult{Success: false, Reason: err.Error(), Trace: trace}, nil, err
	}
	return SelectionResult{Success: false, Reason: ErrNoAvailableNumber.Error(), Trace: trace}, nil, ErrNoAvailableNumber
}
