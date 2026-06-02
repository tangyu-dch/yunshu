package cti

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
)

var ErrNoAvailableNumber = errors.New("no available caller number")

type NumberCandidate struct {
	Phone              string `json:"phone"`
	GatewayID          string `json:"gatewayId"`
	SkillGroupID       int    `json:"skillGroupId,omitempty"`
	ChannelID          int    `json:"channelId,omitempty"`
	GatewayName        string `json:"gatewayName,omitempty"`
	GatewayRegion      string `json:"gatewayRegion,omitempty"`
	Model              int    `json:"model,omitempty"`
	CallerPrefix       string `json:"callerPrefix,omitempty"`
	CalleePrefix       string `json:"calleePrefix,omitempty"`
	CallerRewriteRule  string `json:"callerRewriteRule,omitempty"`
	CalleeRewriteRule  string `json:"calleeRewriteRule,omitempty"`
	SupplementRing     bool   `json:"supplementRing,omitempty"`
	SupplementRingFile string `json:"supplementRingFile,omitempty"`
	Province           string `json:"province,omitempty"`
	City               string `json:"city,omitempty"`
	PoolID             int    `json:"poolId,omitempty"`
	CodecPrefs         string `json:"codecPrefs,omitempty"`
	BroadcastTime      int64  `json:"broadcastTime,omitempty"`
	BroadcastTimeFlag  bool   `json:"broadcastTimeFlag,omitempty"`
	Concurrency        int    `json:"concurrency"`
	GatewayConcurrency int    `json:"gatewayConcurrency,omitempty"` // 网关全局物理并发上限限制，用于运行时与号码并发进行双重级联限制校验
	Available          bool   `json:"available"`
	RiskAllowed        bool   `json:"riskAllowed"`
	WhitelistHit       bool   `json:"whitelistHit"`
	BlacklistHit       bool   `json:"blacklistHit"`
	Priority           int    `json:"priority,omitempty"`
	SelectionStrategy  string `json:"selectionStrategy,omitempty"`
}

type SelectionRequest struct {
	CallID            string            `json:"callId"`
	MerchantID        string            `json:"merchantId"`
	RiskID            int               `json:"riskId,omitempty"`
	UserID            int               `json:"userId,omitempty"`
	Callee            string            `json:"callee"`
	SkillGroup        string            `json:"skillGroup"`
	Candidates        []NumberCandidate `json:"candidates"`
	SelectionStrategy string            `json:"selectionStrategy,omitempty"`
}

type SelectionResult struct {
	Success bool             `json:"success"`
	Caller  *NumberCandidate `json:"caller,omitempty"`
	Reason  string           `json:"reason,omitempty"`
	Trace   []SelectionTrace `json:"trace"`
}

type SelectionTrace struct {
	Phone  string `json:"phone"`
	Rule   string `json:"rule"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

// CandidateSource 提供 CTI 选号候选号码。
// 生产实现应从  兼容表或其缓存投影读取，呼叫热路径后续会切到 Redis/物化结构。
type CandidateSource interface {
	CandidatesForUser(ctx context.Context, userID int) ([]NumberCandidate, error)
}

// CandidateMarker 在进入选号规则链之前补充黑白名单等运行时标记。
type CandidateMarker interface {
	MarkCandidates(ctx context.Context, req SelectionRequest, candidates []NumberCandidate) ([]NumberCandidate, error)
}

// NoopCandidateMarker 是默认空实现。
type NoopCandidateMarker struct{}

// MarkCandidates 直接返回原始候选。
func (NoopCandidateMarker) MarkCandidates(_ context.Context, _ SelectionRequest, candidates []NumberCandidate) ([]NumberCandidate, error) {
	return candidates, nil
}

type Selector struct{}

// Select 执行选号规则链。
// 选号失败不是系统异常，而是批量任务和 API 外呼都必须显式处理的业务路径。
func (Selector) Select(_ context.Context, req SelectionRequest) SelectionResult {
	slog.Info("开始执行 CTI 选号规则链", "callId", req.CallID, "merchantId", req.MerchantID, "skillGroup", req.SkillGroup, "candidateCount", len(req.Candidates))
	eligible, trace := eligibleCandidates(req)
	if len(eligible) > 0 {
		chosen := eligible[0]
		slog.Info("CTI 选号规则链执行成功", "callId", req.CallID, "merchantId", req.MerchantID, "gatewayId", chosen.GatewayID, "traceCount", len(trace))
		return SelectionResult{Success: true, Caller: &chosen, Trace: trace}
	}
	slog.Warn("CTI 选号规则链执行失败", "callId", req.CallID, "merchantId", req.MerchantID, "traceCount", len(trace), "reason", ErrNoAvailableNumber.Error())
	return SelectionResult{Success: false, Reason: ErrNoAvailableNumber.Error(), Trace: trace}
}

func eligibleCandidates(req SelectionRequest) ([]NumberCandidate, []SelectionTrace) {
	trace := make([]SelectionTrace, 0, len(req.Candidates)*4)
	eligible := make([]NumberCandidate, 0, len(req.Candidates))
	for _, candidate := range req.Candidates {
		if !candidate.Available {
			slog.Debug("候选号不可用", "callId", req.CallID, "gatewayId", candidate.GatewayID, "rule", "availability")
			trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "availability", Passed: false, Reason: "号码不可用"})
			continue
		}
		trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "availability", Passed: true})
		if candidate.WhitelistHit {
			trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "whitelist", Passed: true})
		} else {
			trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "whitelist", Passed: false, Reason: "未命中白名单"})
		}
		if candidate.WhitelistHit {
			if candidate.Concurrency <= 0 {
				slog.Debug("白名单候选号并发不足", "callId", req.CallID, "gatewayId", candidate.GatewayID, "rule", "concurrency")
				trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "concurrency", Passed: false, Reason: "并发不足"})
				continue
			}
			trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "concurrency", Passed: true})
			eligible = append(eligible, candidate)
			continue
		}
		if candidate.BlacklistHit {
			slog.Debug("候选号命中黑名单", "callId", req.CallID, "gatewayId", candidate.GatewayID, "rule", "blacklist")
			trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "blacklist", Passed: false, Reason: "命中黑名单"})
			continue
		}
		trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "blacklist", Passed: true})
		if !candidate.RiskAllowed {
			slog.Debug("候选号被风控拒绝", "callId", req.CallID, "gatewayId", candidate.GatewayID, "rule", "risk")
			trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "risk", Passed: false, Reason: "风控拒绝"})
			continue
		}
		trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "risk", Passed: true})
		if candidate.Concurrency <= 0 {
			slog.Debug("候选号并发不足", "callId", req.CallID, "gatewayId", candidate.GatewayID, "rule", "concurrency")
			trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "concurrency", Passed: false, Reason: "并发不足"})
			continue
		}
		trace = append(trace, SelectionTrace{Phone: candidate.Phone, Rule: "concurrency", Passed: true})
		eligible = append(eligible, candidate)
	}
	strategy := req.SelectionStrategy
	if strategy == "" && len(eligible) > 0 {
		strategy = eligible[0].SelectionStrategy
	}
	if strategy == "" {
		strategy = "CONCURRENCY"
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].WhitelistHit != eligible[j].WhitelistHit {
			return eligible[i].WhitelistHit
		}
		switch strings.ToUpper(strategy) {
		case "RANDOM":
			hI := fnvHash(eligible[i].Phone + req.CallID)
			hJ := fnvHash(eligible[j].Phone + req.CallID)
			if hI != hJ {
				return hI < hJ
			}
		case "PRIORITY":
			if eligible[i].Priority != eligible[j].Priority {
				return eligible[i].Priority < eligible[j].Priority // 优先级升序（数值越小优先级越高）
			}
		case "CONCURRENCY":
			if eligible[i].Concurrency != eligible[j].Concurrency {
				return eligible[i].Concurrency > eligible[j].Concurrency
			}
		}
		return eligible[i].Phone < eligible[j].Phone
	})
	return eligible, trace
}

func fnvHash(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	return h
}
