package resource

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"yunshu/internal/domain/esl"
)

// InboundAgentResolver 通过 DID 号码查找可用的空闲坐席。
// 查询链路: DID(cc_res_pool_phone) → 号码技能组(cc_res_pool_phone_skill_group)
//
//	→ 用户技能组(cc_res_user_skill_group) → 商户用户(cc_res_mch_user)
//	→ 分机(cc_res_extension)
//
// 过滤条件: 用户启用、分机启用、分机状态为 IDLE(Redis)。
type InboundAgentResolver struct {
	DB           *gorm.DB
	StatusReader esl.ExtensionStatusReader
	Logger       *slog.Logger
}

// NewInboundAgentResolver 创建呼入坐席分配器。
func NewInboundAgentResolver(db *gorm.DB, statusReader esl.ExtensionStatusReader, logger *slog.Logger) *InboundAgentResolver {
	return &InboundAgentResolver{DB: db, StatusReader: statusReader, Logger: logger}
}

// inboundAgentRow 查询坐席候选的中间结果。
type inboundAgentRow struct {
	UserID          int    `gorm:"column:user_id"`
	MerchantID      int    `gorm:"column:merchant_id"`
	SeatNumber      string `gorm:"column:seat_number"`
	ExtensionNumber string `gorm:"column:extension_number"`
	SipDomain       string `gorm:"column:sip_domain"`
	SkillGroupID    int    `gorm:"column:skill_group_id"`
}

// ResolveForDID 根据呼入 DID 号码找到一个可用的空闲坐席。
// 返回坐席的 userId、merchantId、extension 分机号。
// 如果没有找到可用坐席，返回 nil, nil。
func (r *InboundAgentResolver) ResolveForDID(ctx context.Context, did string, merchantID int) (*esl.Extension, error) {
	// 查询 DID 号码关联的技能组下的所有坐席
	var rows []inboundAgentRow
	err := r.DB.WithContext(ctx).Raw(`
		SELECT
			u.id AS user_id,
			u.merchant_id,
			u.seat_number,
			e.extension_number,
			e.sip_domain,
			sg.id AS skill_group_id
		FROM cc_res_pool_phone pp
		INNER JOIN cc_res_pool_phone_skill_group ppsg ON ppsg.pool_phone_id = pp.id
		INNER JOIN cc_res_skill_group sg ON sg.id = ppsg.skill_group_id AND sg.enable = 1 AND sg.del_flag = 0
		INNER JOIN cc_res_user_skill_group usg ON usg.skill_group_id = sg.id
		INNER JOIN cc_res_mch_user u ON u.id = usg.user_id AND u.enable = 1 AND u.del_flag = 0
		INNER JOIN cc_res_extension e ON e.user_id = u.id AND e.enable = 1 AND e.del_flag = 0
		WHERE pp.phone = ? AND pp.enable = 1 AND pp.del_flag = 0 AND u.merchant_id = ?
		ORDER BY u.id ASC
	`, did, merchantID).Scan(&rows).Error
	if err != nil {
		r.logger().Error("查询 DID 关联坐席失败", "did", did, "merchantId", merchantID, "error", err.Error())
		return nil, err
	}

	if len(rows) == 0 {
		r.logger().Warn("DID 未关联任何坐席", "did", did, "merchantId", merchantID)
		return nil, nil
	}

	// 遍历候选坐席，找到第一个 IDLE 的
	for _, row := range rows {
		status, found, err := r.StatusReader.GetExtensionStatus(ctx, row.ExtensionNumber)
		if err != nil {
			r.logger().Debug("读取分机状态失败，跳过", "extension", row.ExtensionNumber, "error", err.Error())
			continue
		}
		if !found || status != esl.ExtensionStatusIdle {
			r.logger().Debug("坐席非空闲，跳过", "extension", row.ExtensionNumber, "status", int(status), "found", found)
			continue
		}
		ext := &esl.Extension{
			UserID:          row.UserID,
			MerchantID:      row.MerchantID,
			ExtensionNumber: row.ExtensionNumber,
			SipDomain:       row.SipDomain,
			SkillGroupID:    row.SkillGroupID,
		}
		r.logger().Info("呼入坐席分配成功",
			"did", did,
			"merchantId", merchantID,
			"userId", ext.UserID,
			"extension", ext.ExtensionNumber)
		return ext, nil
	}

	skillGroupID := rows[0].SkillGroupID
	r.logger().Warn("DID 关联坐席均非空闲，返回技能组用于排队", "did", did, "merchantId", merchantID, "skillGroupId", skillGroupID, "candidateCount", len(rows))
	return &esl.Extension{MerchantID: merchantID, SkillGroupID: skillGroupID}, nil
}

func (r *InboundAgentResolver) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
