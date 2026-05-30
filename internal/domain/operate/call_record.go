package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

var (
	// ErrInvalidCallRecord 表示呼叫记录参数无效。
	ErrInvalidCallRecord = errors.New("invalid call record")
	// ErrCallRecordNotFound 表示呼叫记录不存在。
	ErrCallRecordNotFound = errors.New("call record not found")
)

// CallRecord 表示商户侧呼叫记录查询结果。
type CallRecord struct {
	CallID      string    `json:"callId"`
	UUID        string    `json:"uuid,omitempty"`
	FSAddr      string    `json:"fsAddr,omitempty"`
	Profile     string    `json:"profile,omitempty"`
	MerchantID  int       `json:"merchantId"`
	UserID      int       `json:"userId"`
	BatchTaskID int       `json:"batchTaskId,omitempty"`
	BatchTelID  int       `json:"batchCallTelId,omitempty"`
	Caller      string    `json:"caller,omitempty"`
	Callee      string    `json:"callee,omitempty"`
	DurationSec int       `json:"durationSec,omitempty"`
	HangupCause string    `json:"hangupCause,omitempty"`
	FinalState  string    `json:"finalState,omitempty"`
	RecordFile  string    `json:"recordFilePath,omitempty"`
	CompletedAt time.Time `json:"completedAt,omitempty"`

	// 新增高级通话统计与分析报表字段
	Billsec     int    `json:"billsec"`               // 实际通话时长（秒）
	Ringsec     int    `json:"ringsec"`               // 振铃时长（秒）
	BillingSec  int    `json:"billingSec"`            // 计费时长（秒）
	GatewayName string `json:"gatewayName,omitempty"` // 走了哪个网关
	Extension   string `json:"extension,omitempty"`   // 用的是哪个分机
}

// CallRecordPageRequest 表示呼叫记录分页查询条件。
type CallRecordPageRequest struct {
	PageNumber  int    `json:"pageNumber"`
	PageSize    int    `json:"pageSize"`
	CallID      string `json:"callId,omitempty"`
	MerchantID  int    `json:"merchantId,omitempty"`
	UserID      int    `json:"userId,omitempty"`
	BatchTaskID int    `json:"batchTaskId,omitempty"`

	// 新增的查询过滤条件
	MinDuration int    `json:"minDuration,omitempty"` // 通话时长大于等于（秒）
	GatewayID   string `json:"gatewayId,omitempty"`   // 网关名称或ID
	Profile     string `json:"profile,omitempty"`     // 渠道类型 (api_outbound/batch_outbound/api_direct/inbound)
	Extension   string `json:"extension,omitempty"`   // 分机号
	StartTime   string `json:"startTime,omitempty"`   // 呼叫时间范围起点 (RFC3339)
	EndTime     string `json:"endTime,omitempty"`     // 呼叫时间范围终点 (RFC3339)
}

// CallRecordPageResult 表示呼叫记录分页结果。
type CallRecordPageResult struct {
	PageNumber int          `json:"pageNumber"`
	PageSize   int          `json:"pageSize"`
	Total      int64        `json:"total"`
	Records    []CallRecord `json:"records"`
}

// CallRecordRepository 定义呼叫记录查询能力。
type CallRecordRepository interface {
	Page(ctx context.Context, req CallRecordPageRequest) (CallRecordPageResult, error)
	GetByCallID(ctx context.Context, callID string) (CallRecord, error)
}

// CallRecordManagementService 承载商户呼叫记录查询。
type CallRecordManagementService struct {
	Repository CallRecordRepository
	Logger     *slog.Logger
}

// Page 返回呼叫记录分页结果。
func (s *CallRecordManagementService) Page(ctx context.Context, req CallRecordPageRequest) (CallRecordPageResult, error) {
	logger := s.logger()
	req = normalizeCallRecordPage(req)
	logger.Info("商户端开始查询呼叫记录", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "callId", req.CallID, "merchantId", req.MerchantID, "userId", req.UserID, "batchTaskId", req.BatchTaskID)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("商户端查询呼叫记录失败", "error", err.Error())
		return CallRecordPageResult{}, err
	}
	logger.Info("商户端查询呼叫记录完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Detail 返回单条呼叫记录。
func (s *CallRecordManagementService) Detail(ctx context.Context, callID string) (CallRecord, error) {
	logger := s.logger()
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return CallRecord{}, ErrInvalidCallRecord
	}
	logger.Info("商户端开始查询呼叫记录详情", "callId", callID)
	record, err := s.Repository.GetByCallID(ctx, callID)
	if err != nil {
		logger.Error("商户端查询呼叫记录详情失败", "callId", callID, "error", err.Error())
		return CallRecord{}, err
	}
	logger.Info("商户端查询呼叫记录详情完成", "callId", callID)
	return record, nil
}

func (s *CallRecordManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeCallRecordPage(req CallRecordPageRequest) CallRecordPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	return req
}
