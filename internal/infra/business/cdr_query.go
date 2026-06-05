package business

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// QueryRepository 读取 CDR 记录供商户管理端查询。
type QueryRepository struct {
	DB *gorm.DB
}

// NewQueryRepository 创建 CDR 查询仓储。
func NewQueryRepository(db *gorm.DB) *QueryRepository {
	return &QueryRepository{DB: db}
}

// Page 返回 CDR 记录分页结果。
func (r *QueryRepository) Page(ctx context.Context, req operate.CallRecordPageRequest) (operate.CallRecordPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&RecordModel{})
	if req.CallID != "" {
		query = query.Where("call_id LIKE ?", "%"+strings.TrimSpace(req.CallID)+"%")
	}
	if req.MerchantID > 0 {
		query = query.Where("merchant_id = ?", req.MerchantID)
	}
	if req.UserID > 0 {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.BatchTaskID > 0 {
		query = query.Where("batch_task_id = ?", req.BatchTaskID)
	}
	if req.MinDuration > 0 {
		query = query.Where("duration_sec >= ?", req.MinDuration)
	}
	if req.Profile != "" {
		query = query.Where("profile = ?", req.Profile)
	}
	if req.Extension != "" {
		query = query.Where("extension = ? OR caller = ? OR raw_payload LIKE ?", req.Extension, req.Extension, "%"+req.Extension+"%")
	}
	if req.Phone != "" {
		phone := strings.TrimSpace(req.Phone)
		query = query.Where("caller = ? OR callee = ?", phone, phone)
	}
	if req.GatewayID != "" {
		query = query.Where("fs_addr LIKE ? OR raw_payload LIKE ?", "%"+req.GatewayID+"%", "%"+req.GatewayID+"%")
	}
	if req.StartTime != "" {
		query = query.Where("completed_at >= ?", req.StartTime)
	}
	if req.EndTime != "" {
		query = query.Where("completed_at <= ?", req.EndTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.CallRecordPageResult{}, err
	}
	var models []RecordModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("completed_at DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.CallRecordPageResult{}, err
	}
	records := make([]operate.CallRecord, 0, len(models))
	for _, model := range models {
		records = append(records, callRecordFromModel(model))
	}
	return operate.CallRecordPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByCallID 读取单条 CDR 记录。
func (r *QueryRepository) GetByCallID(ctx context.Context, callID string) (operate.CallRecord, error) {
	var model RecordModel
	err := r.DB.WithContext(ctx).Where("call_id = ?", strings.TrimSpace(callID)).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.CallRecord{}, operate.ErrCallRecordNotFound
	}
	return callRecordFromModel(model), err
}

func callRecordFromModel(model RecordModel) operate.CallRecord {
	payload := model.RawPayload

	// 提取实际通话时长 (billsec)
	billsec := 0
	if val, ok := payload["billsec"]; ok {
		billsec = intValue(val)
	} else if val, ok := payload["variable_billsec"]; ok {
		billsec = intValue(val)
	}

	// 提取总时长
	durationSec := model.DurationSec
	if durationSec <= 0 {
		if val, ok := payload["durationSec"]; ok {
			durationSec = intValue(val)
		}
	}

	// 振铃时长 = 总时长 - 实际通话时长
	ringsec := durationSec - billsec
	if ringsec < 0 {
		ringsec = 0
	}

	// 计费时长为了保留最真实的秒数，以实际 billsec 决定
	billingSec := billsec

	// 走了哪个网关
	gatewayName := ""
	if val, ok := payload["selectedGatewayName"]; ok && val != nil {
		gatewayName = fmt.Sprint(val)
	} else if val, ok := payload["selectedGatewayId"]; ok && val != nil {
		gatewayName = "网关 " + fmt.Sprint(val)
	}

	// 分机号 (优先读取数据库物理列，如果无值则从 raw_payload 提取以确保历史数据兼容)
	extension := model.Extension
	if extension == "" {
		if val, ok := payload["extension"]; ok && val != nil && fmt.Sprint(val) != "" {
			extension = fmt.Sprint(val)
		} else if val, ok := payload["variable_agent_extension"]; ok && val != nil && fmt.Sprint(val) != "" {
			extension = fmt.Sprint(val)
		} else if val, ok := payload["agent_extension"]; ok && val != nil && fmt.Sprint(val) != "" {
			extension = fmt.Sprint(val)
		}
	}

	// 挂断方向细节 (同理，优先读取数据库物理列进行兼容)
	sipHangupDisposition := model.SipHangupDisposition
	if sipHangupDisposition == "" {
		if val, ok := payload["sipHangupDisposition"]; ok && val != nil {
			sipHangupDisposition = fmt.Sprint(val)
		}
	}

	return operate.CallRecord{
		CallID:               model.CallID,
		UUID:                 model.UUID,
		FSAddr:               model.FSAddr,
		Profile:              model.Profile,
		MerchantID:           model.MerchantID,
		UserID:               model.UserID,
		BatchTaskID:          model.BatchTaskID,
		BatchTelID:           model.BatchTelID,
		Caller:               model.Caller,
		Callee:               model.Callee,
		DurationSec:          durationSec,
		HangupCause:          model.HangupCause,
		FinalState:           model.FinalState,
		RecordFile:           model.RecordFile,
		CompletedAt:          model.CompletedAt,
		Billsec:              billsec,
		Ringsec:              ringsec,
		BillingSec:           billingSec,
		GatewayName:          gatewayName,
		Extension:            extension,
		SipHangupDisposition: sipHangupDisposition,
	}
}
