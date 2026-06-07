package business

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"yunshu/internal/infra/merchant"
)

// GormRateResolver 使用数据库和 Redis 缓存实现商户费率解析。
//
// 查询优先级：
//  1. Redis 缓存（TTL 5 分钟）
//  2. 数据库：cc_mch_rate_ref JOIN cc_mch_rate 查商户绑定的专属费率
//  3. 兜底：DefaultRatePerMin 换算为 60 秒颗粒度
//
// 管理端变更商户费率绑定后需调用 InvalidateMerchantRate 主动失效缓存。
type GormRateResolver struct {
	DB                *gorm.DB
	Redis             *redis.Client
	DefaultRatePerMin float64
	CacheTTL          time.Duration
	Logger            *slog.Logger
}

// rateCache 是存储在 Redis 中的费率缓存结构体。
type rateCache struct {
	RateTemplateID int     `json:"rateTemplateId"`
	RateName       string  `json:"rateName"`
	BillingCycle   int     `json:"billingCycle"`
	BillingPrice   float64 `json:"billingPrice"`
	MatchRule      string  `json:"matchRule"`
}

// Resolve 按商户 ID 解析最优费率，优先读取 Redis 缓存，缓存缺失时查数据库并写回缓存。
func (r *GormRateResolver) Resolve(ctx context.Context, merchantID int) (RateDecision, error) {
	logger := r.logger()

	// 1. 优先读 Redis 缓存
	if r.Redis != nil {
		cacheKey := fmt.Sprintf("cc:rate:merchant:%d", merchantID)
		cached, err := r.Redis.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var rc rateCache
			if jsonErr := json.Unmarshal(cached, &rc); jsonErr == nil {
				logger.Debug("命中商户费率缓存", "merchantId", merchantID, "rateId", rc.RateTemplateID, "cycle", rc.BillingCycle)
				return RateDecision{
					RateTemplateID: rc.RateTemplateID,
					RateName:       rc.RateName,
					BillingCycle:   rc.BillingCycle,
					BillingPrice:   rc.BillingPrice,
					MatchRule:      rc.MatchRule,
				}, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			logger.Warn("读取商户费率缓存失败，降级查询数据库", "merchantId", merchantID, "error", err.Error())
		}
	}

	// 2. 数据库查商户绑定的专属费率：cc_mch_rate_ref JOIN cc_mch_rate
	if r.DB != nil {
		decision, found, err := r.resolveFromDB(ctx, merchantID)
		if err != nil {
			logger.Error("数据库查询商户费率失败，降级使用系统默认费率", "merchantId", merchantID, "error", err.Error())
		} else if found {
			r.writeRateCache(context.Background(), merchantID, decision)
			logger.Info("商户费率已从数据库加载", "merchantId", merchantID, "rateId", decision.RateTemplateID, "cycle", decision.BillingCycle, "price", decision.BillingPrice)
			return decision, nil
		} else {
			logger.Info("商户未绑定专属费率，使用系统默认费率", "merchantId", merchantID)
		}
	}

	// 3. 兜底：系统默认费率
	return r.defaultDecision(), nil
}

// InvalidateMerchantRate 主动失效指定商户的费率缓存。
// 管理端修改商户费率绑定后应立即调用此方法。
func (r *GormRateResolver) InvalidateMerchantRate(ctx context.Context, merchantID int) error {
	if r.Redis == nil {
		return nil
	}
	cacheKey := fmt.Sprintf("cc:rate:merchant:%d", merchantID)
	if err := r.Redis.Del(ctx, cacheKey).Err(); err != nil {
		r.logger().Warn("删除商户费率缓存失败", "merchantId", merchantID, "error", err.Error())
		return err
	}
	r.logger().Info("商户费率缓存已主动失效", "merchantId", merchantID)
	return nil
}

// resolveFromDB 查询 cc_mch_rate_ref JOIN cc_mch_rate 获取商户绑定费率。
func (r *GormRateResolver) resolveFromDB(ctx context.Context, merchantID int) (RateDecision, bool, error) {
	// 通过关联表查询商户绑定的费率模板
	var rate merchant.CallRateModel
	err := r.DB.WithContext(ctx).
		Table("cc_mch_rate r").
		Joins("INNER JOIN cc_mch_rate_ref ref ON ref.rate_id = r.id").
		Where("ref.merchant_id = ? AND r.enable = ? AND r.del_flag = ?", merchantID, true, false).
		Order("r.id DESC"). // 取最新绑定的费率
		First(&rate).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RateDecision{}, false, nil
		}
		return RateDecision{}, false, err
	}

	// 校验费率配置完整性
	if rate.BillingCycle <= 0 || rate.BillingPrice <= 0 {
		r.logger().Warn("商户费率模板配置无效，回退系统默认费率",
			"merchantId", merchantID, "rateId", rate.ID,
			"billingCycle", rate.BillingCycle, "billingPrice", rate.BillingPrice)
		return RateDecision{}, false, nil
	}

	decision := RateDecision{
		RateTemplateID: rate.ID,
		RateName:       rate.RateName,
		BillingCycle:   rate.BillingCycle,
		BillingPrice:   rate.BillingPrice,
		MatchRule:      fmt.Sprintf("merchant_rate|id=%d|cycle=%ds", rate.ID, rate.BillingCycle),
	}
	return decision, true, nil
}

// writeRateCache 将费率决策写入 Redis 缓存。
func (r *GormRateResolver) writeRateCache(ctx context.Context, merchantID int, decision RateDecision) {
	if r.Redis == nil {
		return
	}
	rc := rateCache{
		RateTemplateID: decision.RateTemplateID,
		RateName:       decision.RateName,
		BillingCycle:   decision.BillingCycle,
		BillingPrice:   decision.BillingPrice,
		MatchRule:      decision.MatchRule,
	}
	data, err := json.Marshal(rc)
	if err != nil {
		r.logger().Warn("序列化费率缓存失败", "merchantId", merchantID, "error", err.Error())
		return
	}
	ttl := r.CacheTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	cacheKey := fmt.Sprintf("cc:rate:merchant:%d", merchantID)
	if err := r.Redis.Set(ctx, cacheKey, data, ttl).Err(); err != nil {
		r.logger().Warn("写入商户费率缓存失败", "merchantId", merchantID, "error", err.Error())
	}
}

// defaultDecision 返回基于系统默认分钟费率的 60 秒颗粒度决策。
func (r *GormRateResolver) defaultDecision() RateDecision {
	return RateDecision{
		RateTemplateID: 0,
		RateName:       "system_default",
		BillingCycle:   60,
		BillingPrice:   r.DefaultRatePerMin,
		MatchRule:      fmt.Sprintf("system_default_rate|cycle=60s|rate_per_min=%.6f", r.DefaultRatePerMin),
	}
}

// NewGormRateResolver 创建 Gorm 实现的费率解析器。
func NewGormRateResolver(db *gorm.DB, redis *redis.Client, defaultRatePerMin float64, cacheTTL time.Duration, logger *slog.Logger) *GormRateResolver {
	return &GormRateResolver{
		DB:                db,
		Redis:             redis,
		DefaultRatePerMin: defaultRatePerMin,
		CacheTTL:          cacheTTL,
		Logger:            logger,
	}
}

func (r *GormRateResolver) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
