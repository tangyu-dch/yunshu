package selection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/cti"
	"yunshu/internal/domain/operate"
)

// RuntimeSelectionMarker 负责在呼叫中心选号逻辑执行前，对号码候选者（NumberCandidate）补充安全和风控标记。
// 它通过高并发本地缓存（sync.Map）和数据库查询，对候选号码进行商户白名单、网关黑名单、
// 全局风控黑名单（等级过滤）、外呼区域盲区过滤以及被叫呼叫频次控流检查。
// 遵循高并发及多实例设计规范，热路径查询结果具备 TTL 缓存加速。
type RuntimeSelectionMarker struct {
	DB       *gorm.DB      // 关系型数据库连接句柄，用于回源查询配置和频次统计
	Logger   *slog.Logger  // 结构化日志输出工具
	CacheTTL time.Duration // 缓存生存时间，控制本地 sync.Map 中热数据的有效期
	cache    sync.Map      // 内存 TTL 缓存，加速白名单/黑名单/风控等重复热点查询
}

// riskControlModel 映射数据库的 `risk_control` 风控规则配置表。
// 用于描述对特定商户或通道施加的呼叫限制规则，包括黑名单过滤级别、盲区行政区划限制、防高频骚扰频次限制。
type riskControlModel struct {
	ID                  int    `gorm:"column:id"`                    // 规则主键ID
	BlackLevelFlag      bool   `gorm:"column:black_level_flag"`      // 是否启用黑名单等级过滤
	BlackLevel          string `gorm:"column:black_level"`           // 过滤等级 (LEVEL_1/LEVEL_2/LEVEL_3)
	BlindAreaFlag       bool   `gorm:"column:blind_area_flag"`       // 是否启用盲区拦截
	BlindArea           string `gorm:"column:blind_area"`            // 盲区行政区划编码 CSV (如: "440300,440100")
	CalleeFrequencyFlag bool   `gorm:"column:callee_frequency_flag"` // 是否启用被叫高频呼叫拦截
	CalleeFrequency     string `gorm:"column:callee_frequency"`      // 被叫频次拦截 JSON 配置 (如: [{"day":1,"count":3,"type":"CONNECTED"}])
	DelFlag             bool   `gorm:"column:del_flag"`              // 逻辑删除标志
}

// TableName 指定 riskControlModel 对应的物理表名为 `cc_sec_risk_control`。
func (riskControlModel) TableName() string {
	return "cc_sec_risk_control"
}

// riskControlMerchantModel 映射数据库的 `risk_control_merchant` 风控规则与商户关系表。
// 在多商户体系下，定义了单个商户当前绑定并生效的风控规则策略。
type riskControlMerchantModel struct {
	RiskID     int  `gorm:"column:risk_id"`     // 风控规则ID
	MerchantID int  `gorm:"column:merchant_id"` // 绑定的商户ID
	Enable     bool `gorm:"column:enable"`      // 绑定关系是否启用
}

// TableName 指定 riskControlMerchantModel 对应的物理表名为 `cc_sec_risk_merchant`。
func (riskControlMerchantModel) TableName() string {
	return "cc_sec_risk_merchant"
}

// phoneAttributionModel 映射数据库的 `phone_attribution` 号码归属地索引表。
// 通过号码的前 7 位（号段）查询其对应的省份和城市区划编码，用于盲区风控策略判断。
type phoneAttributionModel struct {
	AreaCode string `gorm:"column:area_code;primaryKey"` // 号码前7位 (例如 "1380013")
	ProvCode string `gorm:"column:prov_code"`            // 省份行政区划代码 (例如 "440000")
	CityCode string `gorm:"column:city_code"`            // 城市行政区划代码 (例如 "440300")
}

// TableName 指定 phoneAttributionModel 对应的物理表名为 `cc_sys_attribution`。
func (phoneAttributionModel) TableName() string {
	return "cc_sys_attribution"
}

// calleeFeatureModel 映射数据库的 `callee_feature` 被叫号码外呼特征表。
// 按天统计特定商户下某个被叫号码的呼叫总次数与接通总次数，是执行高频拦截控制的直接计数依据。
type calleeFeatureModel struct {
	CalledNumber     string    `gorm:"column:called_number"`      // 被叫电话号码
	MerchantID       int       `gorm:"column:merchant_id"`        // 关联的商户ID
	ChannelID        string    `gorm:"column:channel_id"`         // 关联的外呼通道/网关ID
	StatDate         time.Time `gorm:"column:stat_date"`          // 统计日期
	CallDialCount    int       `gorm:"column:call_dial_count"`    // 当日累计拨打次数
	CallConnectCount int       `gorm:"column:call_connect_count"` // 当日累计接通次数
}

// TableName 指定 calleeFeatureModel 对应的物理表名为 `cc_sec_callee_feature`。
func (calleeFeatureModel) TableName() string {
	return "cc_sec_callee_feature"
}

// frequencyConfig 描述单条被叫高频呼叫限制的配置结构。
type frequencyConfig struct {
	Day   int `json:"day"`   // 计数时间跨度（天数）
	Count int `json:"count"` // 允许的最大呼叫次数上限值
	Type  any `json:"type"`  // 控制类型：2 / "CONNECTED" 表示只统计接通数，1 / "DIAL" 或其他表示统计呼叫拨打数
}

// markerCacheEntry 表示 RuntimeSelectionMarker 内存缓存中存放的包含 TTL 过期时间的缓存记录项。
type markerCacheEntry struct {
	ExpiresAt time.Time // 记录失效的UTC绝对时间点
	Value     any       // 存储的真实数据
}

// whitelistCacheValue 缓存白名单查询结果，避免重复检索库表导致话务处理延迟。
type whitelistCacheValue struct {
	CalleeHit    bool     // 被叫号码本身是否属于商户全局白名单
	CallerPhones []string // 该商户配置的特许主叫号码列表（若主叫命中则不执行风控阻断）
}

// MarkCandidates 根据商户白名单、系统黑名单、风控限制和通道策略，对给定的候选号码（NumberCandidate）数组进行分析，补齐风控和可用性标记。
// - 白名单命中的候选号码享有最高路由优先级（WhitelistHit = true），并跳过所有风控和黑名单拦截。
// - 触发全局风控黑名单、网关绑定黑名单、盲区限制或高频频次超限的号码，其可用性或允许状态将被降级（BlacklistHit = true 或 RiskAllowed = false）。
func (m *RuntimeSelectionMarker) MarkCandidates(ctx context.Context, req cti.SelectionRequest, candidates []cti.NumberCandidate) ([]cti.NumberCandidate, error) {
	if m == nil || m.DB == nil || len(candidates) == 0 {
		return candidates, nil
	}
	if req.MerchantID == "" {
		return candidates, nil
	}
	merchantID, err := strconv.Atoi(req.MerchantID)
	if err != nil || merchantID <= 0 {
		return nil, fmt.Errorf("invalid merchant id")
	}

	// 1. 加载白名单：查询商户的加白规则（包含特定被叫加白，或白名单主叫列表）
	whitelistHit, callerPhones, err := m.loadWhitelistPhones(ctx, merchantID, req.Callee)
	if err != nil {
		return nil, err
	}

	// 2. 加载风控黑名单：根据商户绑定的风控规则等级（LEVEL_1 到 LEVEL_3），校验被叫是否命中系统防骚扰库
	globalBlacklistHit, err := m.loadRiskBlacklistHit(ctx, merchantID, req.RiskID, req.Callee)
	if err != nil {
		return nil, err
	}

	// 3. 加载网关黑名单：校验被叫号码是否存在于限制路由此被叫的特定网关黑名单中
	blacklistHit, blockedGateways, err := m.loadBlockedGateways(ctx, req.Callee)
	if err != nil {
		return nil, err
	}

	// 4. 初步标记黑白名单属性
	for i := range candidates {
		// 如果被叫在商户白名单中，或者当前候选号码的主叫电话已被加白：
		if whitelistHit || containsString(callerPhones, candidates[i].Phone) {
			candidates[i].WhitelistHit = true  // 强制标记白名单命中
			candidates[i].BlacklistHit = false // 免疫黑名单
			candidates[i].RiskAllowed = true   // 免疫风控阻断
			continue
		}

		// 如果该候选在此前流程中已被标记为不可用，跳过判断
		if !candidates[i].RiskAllowed {
			continue
		}

		// 命中系统全局风控黑名单
		if globalBlacklistHit {
			candidates[i].BlacklistHit = true
			continue
		}

		// 命中网关特定黑名单映射
		if blacklistHit && containsString(blockedGateways, candidates[i].GatewayID) {
			candidates[i].BlacklistHit = true
		}
	}

	// 5. 风控规则二次校验：校验被叫号码是否触发盲区限制（BlindArea）和高频限制（Frequency）
	riskBlocked, err := m.loadRiskBlocks(ctx, merchantID, req.RiskID, req.Callee)
	if err != nil {
		return nil, err
	}

	// 如果触发风控规则限制，则将所有非白名单候选号码的 RiskAllowed 设为 false
	if riskBlocked {
		for i := range candidates {
			if candidates[i].WhitelistHit {
				continue
			}
			candidates[i].RiskAllowed = false
		}
	}

	// 6. 物理通道限制校验：对剩余处于允许状态的非白名单候选，根据其绑定的物理通道（Channel）规则（如特定线路盲区或线路高频控制）执行最终检查
	for i := range candidates {
		if candidates[i].WhitelistHit || !candidates[i].RiskAllowed {
			continue
		}
		blocked, err := m.loadChannelBlocks(ctx, candidates[i].ChannelID, req.Callee)
		if err != nil {
			return nil, err
		}
		if blocked {
			candidates[i].RiskAllowed = false
		}
	}

	return candidates, nil
}

// loadWhitelistPhones 读取商户绑定的白名单主叫号码列表，并判断目标被叫本身是否属于白名单。采用本地内存缓存降低 QPS 热点库负载。
func (m *RuntimeSelectionMarker) loadWhitelistPhones(ctx context.Context, merchantID int, callee string) (bool, []string, error) {
	cacheKey := fmt.Sprintf("whitelist:%d:%s", merchantID, callee)
	if cached, ok := m.getCache(cacheKey); ok {
		if value, good := cached.(whitelistCacheValue); good {
			return value.CalleeHit, append([]string(nil), value.CallerPhones...), nil
		}
	}

	// 双表关联：查询白名单号码及其商户关联绑定关系
	var rows []struct {
		Phone      string `gorm:"column:phone"`
		NumberType string `gorm:"column:number_type"`
	}
	err := m.DB.WithContext(ctx).
		Table("cc_sec_whitelist AS wd").
		Select("wd.phone, wd.number_type").
		Joins("INNER JOIN cc_sec_whitelist_merchant AS wdm ON wdm.white_id = wd.id").
		Where("wdm.merchant_id = ? AND wd.del_flag = ? AND wd.enable = ?", merchantID, false, true).
		Find(&rows).Error
	if err != nil {
		return false, nil, err
	}

	calleeHit := false
	callers := make([]string, 0, len(rows))
	for _, row := range rows {
		switch strings.ToUpper(strings.TrimSpace(row.NumberType)) {
		case "CALLEE":
			if row.Phone == callee {
				calleeHit = true
			}
		case "CALLER":
			callers = append(callers, row.Phone)
		}
	}

	m.setCache(cacheKey, whitelistCacheValue{CalleeHit: calleeHit, CallerPhones: append([]string(nil), callers...)})
	return calleeHit, callers, nil
}

// loadBlockedGateways 查询当前被叫是否命中了任何本地或三方动态配置的黑名单风控通道，并返回被拦截的物理网关 ID 列表。
func (m *RuntimeSelectionMarker) loadBlockedGateways(ctx context.Context, callee string) (bool, []string, error) {
	cacheKey := "blacklist:gateways:" + callee
	if cached, ok := m.getCache(cacheKey); ok {
		if value, good := cached.([]string); good {
			return len(value) > 0, append([]string(nil), value...), nil
		}
	}

	blockedGatewaysMap := make(map[string]struct{})

	// 1. 加载所有启用的黑名单库配置
	var activeBlacklists []struct {
		ID                  int `gorm:"column:id"`
		VerificationChannel int `gorm:"column:verification_channel"`
	}
	if err := m.DB.WithContext(ctx).
		Table("cc_sec_blacklist").
		Select("id, verification_channel").
		Where("enable = ? AND del_flag = ?", true, false).
		Find(&activeBlacklists).Error; err == nil {

		for _, b := range activeBlacklists {
			hit := false
			// 2. 检查是否有动态配置的三方风控验证通道缓存
			if ch, ok := operate.GetChannelFromCache(b.VerificationChannel); ok && ch.Enable && ch.APIUrl != "" {
				// 通过动态配置发起实时 HTTP 请求进行外部校验
				validator := &operate.DynamicHTTPValidator{Channel: ch}
				if apiHit, err := validator.Validate(ctx, callee); err == nil && apiHit {
					hit = true
					slog.Info("三方风控通道实时动态请求拦截命中！", "channel", ch.Code, "phone", callee)
				}
			}

			// 3. 如果三方没有命中，则以本地 cc_sec_blacklist_data 兜底
			if !hit {
				var localCount int64
				if err := m.DB.WithContext(ctx).
					Table("cc_sec_blacklist_data").
					Where("phone = ?", callee).
					Count(&localCount).Error; err == nil && localCount > 0 {
					hit = true
				}
			}

			// 4. 只要命中该黑名单配置，就找出该黑名单配置所绑定的所有物理网关并加入拦截列表
			if hit {
				var gatewayRefs []struct {
					GatewayID int `gorm:"column:gateway_id"`
				}
				if err := m.DB.WithContext(ctx).
					Table("cc_sec_blacklist_gateway").
					Select("gateway_id").
					Where("blacklist_id = ?", b.ID).
					Find(&gatewayRefs).Error; err == nil {
					for _, ref := range gatewayRefs {
						blockedGatewaysMap[strconv.Itoa(ref.GatewayID)] = struct{}{}
					}
				}
			}
		}
	}

	gateways := make([]string, 0, len(blockedGatewaysMap))
	for gwID := range blockedGatewaysMap {
		gateways = append(gateways, gwID)
	}

	m.setCache(cacheKey, gateways)
	return len(gateways) > 0, gateways, nil
}

// loadRiskBlacklistHit 校验被叫是否命中了商户绑定的特定风控黑名单等级配置（如 LEVEL_1 仅拦截高危，LEVEL_3 拦截所有黑名单等级）。
func (m *RuntimeSelectionMarker) loadRiskBlacklistHit(ctx context.Context, merchantID int, riskID int, callee string) (bool, error) {
	cacheKey := fmt.Sprintf("risk:blacklist:%d:%d:%s", merchantID, riskID, callee)
	if cached, ok := m.getCache(cacheKey); ok {
		if hit, good := cached.(bool); good {
			return hit, nil
		}
	}

	// 读取当前商户的风控配置
	risk, ok, err := m.loadRiskControl(ctx, merchantID, riskID)
	if err != nil || !ok {
		return false, err
	}
	if !risk.BlackLevelFlag {
		return false, nil
	}

	// 转换为契约对应的黑名单级别列表
	levels := blackLevelsFor(risk.BlackLevel)
	if len(levels) == 0 {
		return false, nil
	}

	var count int64
	if err := m.DB.WithContext(ctx).
		Table("cc_sec_blacklist_data").
		Where("phone = ? AND black_level IN ?", callee, levels).
		Count(&count).Error; err != nil {
		return false, err
	}

	hit := count > 0
	m.setCache(cacheKey, hit)
	return hit, nil
}

// loadRiskBlocks 校验被叫号码是否同时触发了盲区风控策略（盲区拦截）和被叫频次限制策略（高频拦截）。
func (m *RuntimeSelectionMarker) loadRiskBlocks(ctx context.Context, merchantID int, riskID int, callee string) (bool, error) {
	cacheKey := fmt.Sprintf("risk:blocks:%d:%d:%s", merchantID, riskID, callee)
	if cached, ok := m.getCache(cacheKey); ok {
		if hit, good := cached.(bool); good {
			return hit, nil
		}
	}

	// 获取风控实体策略描述
	risk, ok, err := m.loadRiskControl(ctx, merchantID, riskID)
	if err != nil || !ok {
		return false, err
	}

	// A. 盲区拦截校验
	blindBlocked, err := m.loadBlindSpotHit(ctx, risk, callee)
	if err != nil {
		return false, err
	}
	if blindBlocked {
		m.setCache(cacheKey, true)
		return true, nil
	}

	// B. 频次拦截校验
	hit, err := m.loadCalleeFrequencyHit(ctx, merchantID, risk, callee)
	if err == nil {
		m.setCache(cacheKey, hit)
	}
	return hit, err
}

// loadBlindSpotHit 判断被叫号码的物理归属地省份/城市行政编码，是否被配置包含在盲区黑名单中。
func (m *RuntimeSelectionMarker) loadBlindSpotHit(ctx context.Context, risk riskControlModel, callee string) (bool, error) {
	cacheKey := fmt.Sprintf("risk:blind:%d:%s:%s", risk.ID, risk.BlindArea, callee)
	if cached, ok := m.getCache(cacheKey); ok {
		if hit, good := cached.(bool); good {
			return hit, nil
		}
	}
	if !risk.BlindAreaFlag || strings.TrimSpace(risk.BlindArea) == "" {
		return false, nil
	}

	// 截取号段前7位进行归属地查询
	areaCode := calleeAreaCode(callee)
	if areaCode == "" {
		return false, nil
	}
	attr, ok, err := m.loadPhoneAttribution(ctx, areaCode)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	// 解析 CSV 盲区限制代码列表进行包含判定
	blindAreas := splitCSV(risk.BlindArea)
	hit := containsString(blindAreas, attr.CityCode) || containsString(blindAreas, attr.ProvCode)
	m.setCache(cacheKey, hit)
	return hit, nil
}

// loadCalleeFrequencyHit 查询 `callee_feature` 表，统计时间窗口内已产生的拨打次数或接通次数是否超限。
func (m *RuntimeSelectionMarker) loadCalleeFrequencyHit(ctx context.Context, merchantID int, risk riskControlModel, callee string) (bool, error) {
	cacheKey := fmt.Sprintf("risk:freq:%d:%d:%s", merchantID, risk.ID, callee)
	if cached, ok := m.getCache(cacheKey); ok {
		if hit, good := cached.(bool); good {
			return hit, nil
		}
	}
	if !risk.CalleeFrequencyFlag || strings.TrimSpace(risk.CalleeFrequency) == "" {
		return false, nil
	}

	// 反序列化频次限制配置数组
	var configs []frequencyConfig
	if err := json.Unmarshal([]byte(risk.CalleeFrequency), &configs); err != nil {
		return false, nil
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// 逐条策略规则进行时间区段滑动累加并超限对比
	for _, cfg := range configs {
		if cfg.Day <= 0 || cfg.Count <= 0 {
			continue
		}
		// 计算统计的起始日期
		start := today.AddDate(0, 0, -(cfg.Day - 1))
		end := now
		field := "call_dial_count"
		if strings.EqualFold(fmt.Sprint(cfg.Type), "2") || strings.EqualFold(fmt.Sprint(cfg.Type), "CONNECTED") {
			field = "call_connect_count"
		}

		var rows []calleeFeatureModel
		if err := m.DB.WithContext(ctx).
			Where("called_number = ? AND merchant_id = ? AND stat_date >= ? AND stat_date <= ?", callee, merchantID, start, end).
			Find(&rows).Error; err != nil {
			return false, err
		}

		total := 0
		for _, row := range rows {
			if field == "call_connect_count" {
				total += row.CallConnectCount
			} else {
				total += row.CallDialCount
			}
		}

		// 触发频次硬阻断
		if total >= cfg.Count {
			m.setCache(cacheKey, true)
			return true, nil
		}
	}

	m.setCache(cacheKey, false)
	return false, nil
}

// loadRiskControl 获取特定商户的风控配置。如果指定了 `riskID`，则以其为主；否则查询商户绑定的默认有效策略关系。
func (m *RuntimeSelectionMarker) loadRiskControl(ctx context.Context, merchantID int, riskID int) (riskControlModel, bool, error) {
	if riskID > 0 {
		var model riskControlModel
		err := m.DB.WithContext(ctx).
			Where("id = ? AND del_flag = ?", riskID, false).
			First(&model).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return riskControlModel{}, false, nil
		}
		if err != nil {
			return riskControlModel{}, false, err
		}
		return model, true, nil
	}

	var binding riskControlMerchantModel
	err := m.DB.WithContext(ctx).
		Where("merchant_id = ? AND enable = ?", merchantID, true).
		First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return riskControlModel{}, false, nil
	}
	if err != nil {
		return riskControlModel{}, false, err
	}

	var model riskControlModel
	err = m.DB.WithContext(ctx).
		Where("id = ? AND del_flag = ?", binding.RiskID, false).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return riskControlModel{}, false, nil
	}
	if err != nil {
		return riskControlModel{}, false, err
	}
	return model, true, nil
}

// blackLevelsFor 根据过滤等级字符，转换为黑名单的枚举判断判定级数组。
func blackLevelsFor(level string) []string {
	switch strings.TrimSpace(strings.ToUpper(level)) {
	case "LEVEL_1":
		return []string{"LEVEL_1"}
	case "LEVEL_2":
		return []string{"LEVEL_1", "LEVEL_2"}
	case "LEVEL_3":
		return []string{"LEVEL_1", "LEVEL_2", "LEVEL_3"}
	default:
		return nil
	}
}

// loadChannelBlocks 对单个通信物理通道进行线路级的盲区拦截和线路高频拦截判断。
func (m *RuntimeSelectionMarker) loadChannelBlocks(ctx context.Context, channelID int, callee string) (bool, error) {
	cacheKey := fmt.Sprintf("channel:block:%d:%s", channelID, callee)
	if cached, ok := m.getCache(cacheKey); ok {
		if hit, good := cached.(bool); good {
			return hit, nil
		}
	}
	if channelID <= 0 {
		return false, nil
	}

	var row struct {
		Config    string `gorm:"column:config"`
		BlindArea string `gorm:"column:blind_area"`
	}
	err := m.DB.WithContext(ctx).
		Table("cc_tel_channel").
		Select("config, blind_area").
		Where("id = ? AND del_flag = ? AND enable = ?", channelID, false, true).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// 校验线路配置的特定盲区
	blindBlocked, err := m.hitChannelBlindArea(ctx, row.BlindArea, callee)
	if err != nil {
		return false, err
	}
	if blindBlocked {
		m.setCache(cacheKey, true)
		return true, nil
	}

	// 校验线路配置的特定防高频限流 config
	hit, err := m.hitChannelFrequency(ctx, channelID, row.Config, callee)
	if err == nil {
		m.setCache(cacheKey, hit)
	}
	return hit, err
}

// hitChannelBlindArea 判断被叫号归属地是否处于特定物理线路禁运盲区内。
func (m *RuntimeSelectionMarker) hitChannelBlindArea(ctx context.Context, raw string, callee string) (bool, error) {
	cacheKey := "channel:blind:" + raw + ":" + callee
	if cached, ok := m.getCache(cacheKey); ok {
		if hit, good := cached.(bool); good {
			return hit, nil
		}
	}
	if strings.TrimSpace(raw) == "" {
		return false, nil
	}
	areaCode := calleeAreaCode(callee)
	if areaCode == "" {
		return false, nil
	}
	attr, ok, err := m.loadPhoneAttribution(ctx, areaCode)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	blindAreas := splitCSV(raw)
	hit := containsString(blindAreas, attr.CityCode) || containsString(blindAreas, attr.ProvCode)
	m.setCache(cacheKey, hit)
	return hit, nil
}

// hitChannelFrequency 根据物理线路的行 config 限制进行统计频次阻断校验。
func (m *RuntimeSelectionMarker) hitChannelFrequency(ctx context.Context, channelID int, raw string, callee string) (bool, error) {
	cacheKey := fmt.Sprintf("channel:freq:%d:%s", channelID, callee)
	if cached, ok := m.getCache(cacheKey); ok {
		if hit, good := cached.(bool); good {
			return hit, nil
		}
	}
	if strings.TrimSpace(raw) == "" {
		return false, nil
	}
	var configs []frequencyConfig
	if err := json.Unmarshal([]byte(raw), &configs); err != nil {
		return false, nil
	}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for _, cfg := range configs {
		if cfg.Day <= 0 || cfg.Count <= 0 {
			continue
		}
		start := today.AddDate(0, 0, -(cfg.Day - 1))
		var rows []calleeFeatureModel
		if err := m.DB.WithContext(ctx).
			Where("called_number = ? AND channel_id = ? AND stat_date >= ? AND stat_date <= ?", callee, strconv.Itoa(channelID), start, now).
			Find(&rows).Error; err != nil {
			return false, err
		}
		total := 0
		for _, row := range rows {
			total += row.CallDialCount
		}
		if total >= cfg.Count {
			m.setCache(cacheKey, true)
			return true, nil
		}
	}
	m.setCache(cacheKey, false)
	return false, nil
}

// calleeAreaCode 截取被叫号码的前7位作为归属地索引。
func calleeAreaCode(callee string) string {
	callee = strings.TrimSpace(callee)
	if len(callee) < 7 {
		return ""
	}
	return callee[:7]
}

// loadPhoneAttribution 查询归属地数据。
func (m *RuntimeSelectionMarker) loadPhoneAttribution(ctx context.Context, areaCode string) (phoneAttributionModel, bool, error) {
	var attr phoneAttributionModel
	err := m.DB.WithContext(ctx).Where("area_code = ?", areaCode).First(&attr).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return phoneAttributionModel{}, false, nil
	}
	if err != nil {
		return phoneAttributionModel{}, false, err
	}
	return attr, true, nil
}

// splitCSV 将逗号分隔文本解析为去重清理后的字符串切片。
func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// getCache 从并发安全的缓存哈希表中按 Key 读取数据，支持时间 TTL 自动失效过期。
func (m *RuntimeSelectionMarker) getCache(key string) (any, bool) {
	if m == nil {
		return nil, false
	}
	raw, ok := m.cache.Load(key)
	if !ok {
		return nil, false
	}
	entry, ok := raw.(markerCacheEntry)
	if !ok {
		m.cache.Delete(key)
		return nil, false
	}
	ttl := m.cacheTTL()
	if ttl > 0 && !entry.ExpiresAt.IsZero() && time.Now().UTC().After(entry.ExpiresAt) {
		m.cache.Delete(key)
		return nil, false
	}
	return entry.Value, true
}

// setCache 写入或覆盖带有过期时间的并发安全内存缓存记录。
func (m *RuntimeSelectionMarker) setCache(key string, value any) {
	if m == nil {
		return
	}
	ttl := m.cacheTTL()
	entry := markerCacheEntry{Value: value}
	if ttl > 0 {
		entry.ExpiresAt = time.Now().UTC().Add(ttl)
	}
	m.cache.Store(key, entry)
}

// cacheTTL 读取有效的全局 TTL 时限，默认值为 5 分钟。
func (m *RuntimeSelectionMarker) cacheTTL() time.Duration {
	if m != nil && m.CacheTTL > 0 {
		return m.CacheTTL
	}
	return 5 * time.Minute
}

// containsString 判断字符串切片中是否包含特定的元素。
func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

// logger 获取配置的 slog 日志工具。
func (m *RuntimeSelectionMarker) logger() *slog.Logger {
	if m != nil && m.Logger != nil {
		return m.Logger
	}
	return slog.Default()
}
