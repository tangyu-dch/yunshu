package resource

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/cti"
)

// PoolPhoneModel 映射  `pool_phone` 表。
type PoolPhoneModel struct {
	ID          int       `gorm:"column:id;primaryKey"`
	PoolID      int       `gorm:"column:pool_id"`
	Phone       string    `gorm:"column:phone"`
	Province    string    `gorm:"column:province"`
	City        string    `gorm:"column:city"`
	Concurrency int       `gorm:"column:concurrency"`
	Remark      string    `gorm:"column:remark"`
	CallLimit   int       `gorm:"column:call_limit"`
	Enable      bool      `gorm:"column:enable"`
	DelFlag     bool      `gorm:"column:del_flag"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的号码表名。
func (PoolPhoneModel) TableName() string {
	return "cc_res_pool_phone"
}

// PoolPhoneSkillGroupModel 映射  `pool_phone_skill_group` 表。
type PoolPhoneSkillGroupModel struct {
	PoolPhoneID  int `gorm:"column:pool_phone_id"`
	SkillGroupID int `gorm:"column:skill_group_id"`
}

// TableName 返回  生产库中的号码技能组关系表名。
func (PoolPhoneSkillGroupModel) TableName() string {
	return "cc_res_pool_phone_skill_group"
}

// SkillGroupModel 映射  `skill_group` 表中选号需要的字段。
type SkillGroupModel struct {
	ID          int       `gorm:"column:id;primaryKey"`
	Name        string    `gorm:"column:name"`
	MerchantID  int       `gorm:"column:merchant_id"`
	Description string    `gorm:"column:description"`
	Enable      bool      `gorm:"column:enable"`
	DelFlag     bool      `gorm:"column:del_flag"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的技能组表名。
func (SkillGroupModel) TableName() string {
	return "cc_res_skill_group"
}

// UserSkillGroupModel 映射  `user_skill_group` 表。
type UserSkillGroupModel struct {
	UserID       int       `gorm:"column:user_id"`
	SkillGroupID int       `gorm:"column:skill_group_id"`
	CreatedTime  time.Time `gorm:"column:created_time"`
	UpdatedTime  time.Time `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的用户技能组关系表名。
func (UserSkillGroupModel) TableName() string {
	return "cc_res_user_skill_group"
}

// PhoneResourceRepository 从  兼容表读取 CTI 选号候选号码。
//
// 该查询用于迁移期和低压验证；生产高并发呼叫路径应把结果预热到 Redis/物化投影，
// 再用原子计数完成并发占用。
type PhoneResourceRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewPhoneResourceRepository 创建 CTI 选号候选仓储。
func NewPhoneResourceRepository(db *gorm.DB, logger *slog.Logger) *PhoneResourceRepository {
	return &PhoneResourceRepository{DB: db, Logger: logger}
}

// CandidatesForUser 按  PhoneResourceMapper SQL 读取用户可用号码资源。
//
// 业务逻辑与规则链：
// 1. 查询坐席（User）所在的技能组（Skill Group）；
// 2. 匹配该技能组下绑定的所有号码池号码（Pool Phone）；
// 3. 关联号码所在的号码池（Pool）及呼叫网关（Gateway）；
// 4. 网关、号码池、号码、技能组必须全部处于启用（enable=1）且未删除（del_flag=0）状态；
// 5. 按网关优先级（priority ASC）、网关ID、号码ID依次升序排列，作为选号候选队列；
// 6. 返回的候选数据将作为并发计数、规则链过滤和并发扣减的依据。
func (r *PhoneResourceRepository) CandidatesForUser(ctx context.Context, userID int) ([]cti.NumberCandidate, error) {
	r.logger().Info("开始为坐席查询选号呼叫候选号码资源", "userId", userID)
	var rows []phoneResourceRow
	err := r.DB.WithContext(ctx).Raw(`
SELECT
	sg.id AS skill_group_id,
	gw.id AS gateway_id,
	gw.model,
	pp.phone,
	pp.concurrency AS phone_concurrency,
	pp.call_limit,
	pp.province,
	pp.city,
	gw.concurrency AS gateway_concurrency,
	gw.priority,
	p.id AS pool_id,
	p.selection_strategy,
	gw.supplement_ring,
	gw.supplement_ring_file,
	gw.channel_id,
	gw.name AS gateway_name,
	gw.callee_prefix,
	gw.caller_prefix,
	gw.callee_rewrite_rule,
	gw.caller_rewrite_rule,
	gw.codec_prefs,
	gw.callee_number_limit,
	gw.callee_number_limit_type,
	gw.broadcast_time,
	gw.broadcast_time_flag,
	CONCAT(gw.realm, ':', gw.port) AS gateway_region
FROM gateway gw
INNER JOIN pool p ON gw.id = p.gateway_id AND p.enable = 1 AND p.del_flag = 0
INNER JOIN cc_res_pool_phone pp ON p.id = pp.pool_id AND pp.enable = 1 AND pp.del_flag = 0
INNER JOIN cc_res_pool_phone_skill_group ppsg ON ppsg.pool_phone_id = pp.id
INNER JOIN cc_res_skill_group sg ON sg.id = ppsg.skill_group_id AND sg.enable = 1 AND sg.del_flag = 0
INNER JOIN cc_res_user_skill_group usg ON usg.skill_group_id = sg.id
WHERE usg.user_id = ? AND gw.enable = 1 AND gw.del_flag = 0
ORDER BY gw.priority ASC, gw.id ASC, pp.id ASC
`, userID).Scan(&rows).Error
	if err != nil {
		r.logger().Error("为坐席查询选号呼叫候选号码资源失败", "userId", userID, "error", err.Error())
		return nil, err
	}
	candidates := make([]cti.NumberCandidate, 0, len(rows))
	for _, row := range rows {
		concurrency := row.PhoneConcurrency
		if concurrency <= 0 {
			concurrency = row.GatewayConcurrency
		}
		candidates = append(candidates, cti.NumberCandidate{
			Phone:              row.Phone,
			GatewayID:          strconv.Itoa(row.GatewayID),
			SkillGroupID:       row.SkillGroupID,
			ChannelID:          row.ChannelID,
			GatewayName:        row.GatewayName,
			GatewayRegion:      row.GatewayRegion,
			Model:              row.Model,
			CallerPrefix:       row.CallerPrefix,
			CalleePrefix:       row.CalleePrefix,
			CallerRewriteRule:  row.CallerRewriteRule,
			CalleeRewriteRule:  row.CalleeRewriteRule,
			SupplementRing:     row.SupplementRing,
			SupplementRingFile: row.SupplementRingFile,
			Province:           row.Province,
			City:               row.City,
			PoolID:             row.PoolID,
			CodecPrefs:         row.CodecPrefs,
			BroadcastTime:      int64(row.BroadcastTime),
			BroadcastTimeFlag:  row.BroadcastTimeFlag,
			Concurrency:        concurrency,
			GatewayConcurrency: row.GatewayConcurrency,
			Available:          true,
			RiskAllowed:        true,
			Priority:           row.Priority,
			SelectionStrategy:  row.SelectionStrategy,
		})
	}
	r.logger().Info("为坐席查询选号呼叫候选号码资源成功", "userId", userID, "candidateCount", len(candidates))
	return candidates, nil
}

type phoneResourceRow struct {
	SkillGroupID          int    `gorm:"column:skill_group_id"`
	GatewayID             int    `gorm:"column:gateway_id"`
	Model                 int    `gorm:"column:model"`
	Phone                 string `gorm:"column:phone"`
	PhoneConcurrency      int    `gorm:"column:phone_concurrency"`
	CallLimit             int    `gorm:"column:call_limit"`
	Province              string `gorm:"column:province"`
	City                  string `gorm:"column:city"`
	GatewayConcurrency    int    `gorm:"column:gateway_concurrency"`
	Priority              int    `gorm:"column:priority"`
	PoolID                int    `gorm:"column:pool_id"`
	SelectionStrategy     string `gorm:"column:selection_strategy"`
	SupplementRing        bool   `gorm:"column:supplement_ring"`
	SupplementRingFile    string `gorm:"column:supplement_ring_file"`
	ChannelID             int    `gorm:"column:channel_id"`
	GatewayName           string `gorm:"column:gateway_name"`
	CalleePrefix          string `gorm:"column:callee_prefix"`
	CallerPrefix          string `gorm:"column:caller_prefix"`
	CalleeRewriteRule     string `gorm:"column:callee_rewrite_rule"`
	CallerRewriteRule     string `gorm:"column:caller_rewrite_rule"`
	CodecPrefs            string `gorm:"column:codec_prefs"`
	CalleeNumberLimit     bool   `gorm:"column:callee_number_limit"`
	CalleeNumberLimitType string `gorm:"column:callee_number_limit_type"`
	BroadcastTime         int    `gorm:"column:broadcast_time"`
	BroadcastTimeFlag     bool   `gorm:"column:broadcast_time_flag"`
	GatewayRegion         string `gorm:"column:gateway_region"`
}

func (r *PhoneResourceRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
