// Package business 提供 RateResolver 接口和 RateDecision 数据结构。
//
// RateResolver 是计费 workflow 节点的费率来源抽象，调用方无需感知
// 费率是来自数据库绑定、缓存还是默认配置。
package business

import (
	"context"
	"fmt"
)

// RateDecision 描述一次计费决策的完整上下文，用于审计与追溯。
//
// 所有字段都会写入 cc_biz_ledger.rating_note，运营端可通过该字段
// 追溯每笔话单使用了哪个费率模板、颗粒度和匹配原因。
type RateDecision struct {
	// RateTemplateID 匹配到的费率模板 ID（cc_mch_rate 表主键）；
	// 0 表示使用系统默认费率，未绑定商户专属模板。
	RateTemplateID int
	// RateName 费率模板名称，仅用于日志和审计展示。
	RateName string
	// BillingCycle 计费颗粒度（秒），如 6 表示每 6 秒计一次费。
	// 不足一个周期按一个周期进位计费。
	BillingCycle int
	// BillingPrice 每个计费单元的费率金额（元）。
	BillingPrice float64
	// MatchRule 费率匹配规则说明，用于审计。
	// 示例："merchant_rate|id=3|cycle=6s" / "system_default_rate|cycle=60s"
	MatchRule string
}

// AuditNote 生成写入 rating_note 的可读审计字符串。
func (d RateDecision) AuditNote(durationSec, units int, amount float64) string {
	return fmt.Sprintf(
		"rate_id=%d|rate=%s|cycle=%ds|units=%d|price=%.6f|amount=%.4f|rule=%s",
		d.RateTemplateID, d.RateName, d.BillingCycle, units, d.BillingPrice, amount, d.MatchRule,
	)
}

// RateResolver 按商户 ID 解析最优适用费率，返回含颗粒度信息的计费决策。
//
// 实现优先级：商户专属绑定费率 > 系统默认费率（来自配置）。
// 调用方使用 RateDecision 中的 BillingCycle 和 BillingPrice 调用
// EstimateByGranularity 完成计费，不依赖硬编码的分钟费率。
type RateResolver interface {
	// Resolve 按商户 ID 返回费率决策。
	// 返回的 RateDecision 必须包含非零的 BillingCycle 和 BillingPrice。
	// 任何情况下都不应返回 error（兜底到默认费率），仅在严重故障时返回 error。
	Resolve(ctx context.Context, merchantID int) (RateDecision, error)
}

// DefaultRateResolver 是无数据库时的兜底费率解析器。
//
// 使用全局配置的 defaultRatePerMin 换算为 60 秒颗粒度（传统按分钟计费）。
// 生产环境必须替换为 GormRateResolver 以支持商户级费率配置。
type DefaultRateResolver struct {
	// DefaultRatePerMin 系统默认按分钟费率。
	// 对应配置项 worker.billing.defaultRatePerMin 或环境变量 WORKER_BILLING_DEFAULT_RATE_PER_MIN。
	DefaultRatePerMin float64
}

// Resolve 返回基于默认分钟费率换算的 60 秒颗粒度费率决策。
func (r *DefaultRateResolver) Resolve(_ context.Context, _ int) (RateDecision, error) {
	return RateDecision{
		RateTemplateID: 0,
		RateName:       "system_default",
		BillingCycle:   60,
		BillingPrice:   r.DefaultRatePerMin,
		MatchRule:      "system_default_rate|cycle=60s",
	}, nil
}
