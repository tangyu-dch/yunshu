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
	CallID         string `json:"callId"`
	MerchantID     string `json:"merchantId"`
	Caller         string `json:"caller"`
	GatewayID      string `json:"gatewayId"`
	ClaimKey       string `json:"claimKey"`
	CandidateIndex int    `json:"candidateIndex"` // 批量原子试选中成功占用的候选位置（0-based）
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
	// 所有合格候选号码打包进单次 Redis 原子试选调用，消除 N-1 次网络往返。
	// Redis Lua 脚本内部按排序优先级逐个试选，首个成功占用的候选立即返回。
	allocation, err := s.Allocator.Claim(ctx, req, eligible)
	if err == nil {
		// 从批量 Lua 返回的索引获取成功占用的候选号码
		var caller NumberCandidate
		if allocation.CandidateIndex >= 0 && allocation.CandidateIndex < len(eligible) {
			caller = eligible[allocation.CandidateIndex]
		} else {
			// 兜底：通过 Caller 和 GatewayID 匹配
			for _, c := range eligible {
				if c.Phone == allocation.Caller && c.GatewayID == allocation.GatewayID {
					caller = c
					break
				}
			}
		}
		result := SelectionResult{Success: true, Caller: &caller, Trace: trace}
		logger.Info("CTI 运行时号码占用成功", "callId", req.CallID, "merchantId", req.MerchantID, "caller", allocation.Caller, "gatewayId", allocation.GatewayID, "claimKey", allocation.ClaimKey, "candidateIndex", allocation.CandidateIndex, "totalCandidates", len(eligible))
		return result, &allocation, nil
	}
	if errors.Is(err, ErrRuntimeConcurrencyExhausted) {
		logger.Warn("CTI 所有候选号码运行时并发已满，选号失败", "callId", req.CallID, "merchantId", req.MerchantID, "totalCandidates", len(eligible))
		return SelectionResult{Success: false, Reason: ErrNoAvailableNumber.Error(), Trace: trace}, nil, ErrNoAvailableNumber
	}
	// 非并发错误（如 Redis 不可达），中止选号
	logger.Warn("CTI 运行时号码占用失败", "callId", req.CallID, "merchantId", req.MerchantID, "error", err.Error())
	return SelectionResult{Success: false, Reason: err.Error(), Trace: trace}, nil, err
}
