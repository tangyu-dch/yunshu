package business

import "math"

// RatingResult 表示一次计费估算结果。
type RatingResult struct {
	Amount     float64
	RatePerMin float64
	Note       string
}

// EstimateByMinute 使用默认分钟费率做基础估算。
//
// 当前仅作为可审计的计费计算节点，不做余额扣减； 费率、套餐、优惠规则迁移后
// 应替换为完整 rater，但仍保持“计算”和“扣款”两个节点分离。
func EstimateByMinute(durationSec int, ratePerMin float64) RatingResult {
	if durationSec <= 0 || ratePerMin <= 0 {
		return RatingResult{Amount: 0, RatePerMin: ratePerMin, Note: "zero_duration_or_rate"}
	}
	minutes := math.Ceil(float64(durationSec) / 60)
	amount := math.Round(minutes*ratePerMin*100) / 100
	return RatingResult{Amount: amount, RatePerMin: ratePerMin, Note: "estimated_by_default_minute_rate"}
}
