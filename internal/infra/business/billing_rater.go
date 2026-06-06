package business

import (
	"fmt"
	"math"
)

// RatingResult 表示一次计费估算结果。
type RatingResult struct {
	Amount     float64
	RatePerMin float64
	Note       string
}

// EstimateByGranularity 按可配置计费颗粒度估算话费。
//
// cycleSec 为计费周期秒数（如 6、10、60），pricePerCycle 为每个周期费率。
// 不足一个周期一律按一个周期计费（进位模式，类似电信费率计费模型）。
// 精度为 0.0001 元。
func EstimateByGranularity(durationSec, cycleSec int, pricePerCycle float64) RatingResult {
	if durationSec <= 0 || cycleSec <= 0 || pricePerCycle <= 0 {
		return RatingResult{
			Amount:     0,
			RatePerMin: 0,
			Note:       "zero_duration_or_rate",
		}
	}
	units := int(math.Ceil(float64(durationSec) / float64(cycleSec)))
	// 金额精度射入到 0.0001 元，避免浮点小数縯加导致计费偏差
	amount := math.Round(float64(units)*pricePerCycle*10000) / 10000
	// 展示用分钟费率：将单位费率换算成每分钟价格方便展示
	ratePerMin := pricePerCycle / float64(cycleSec) * 60
	return RatingResult{
		Amount:     amount,
		RatePerMin: math.Round(ratePerMin*100000) / 100000,
		Note: fmt.Sprintf(
			"granularity_billing|cycle=%ds|units=%d|price_per_unit=%.6f|total=%.4f",
			cycleSec, units, pricePerCycle, amount,
		),
	}
}

// EstimateByMinute 使用默认分钟费率做局座億底估算。
//
// 已过时：业务应优先使用 EstimateByGranularity + GormRateResolver。
// 保留此函数为当 RateResolver 不可用时的局座億底。
func EstimateByMinute(durationSec int, ratePerMin float64) RatingResult {
	if durationSec <= 0 || ratePerMin <= 0 {
		return RatingResult{Amount: 0, RatePerMin: ratePerMin, Note: "zero_duration_or_rate"}
	}
	minutes := math.Ceil(float64(durationSec) / 60)
	amount := math.Round(minutes*ratePerMin*100) / 100
	return RatingResult{Amount: amount, RatePerMin: ratePerMin, Note: "estimated_by_default_minute_rate"}
}
