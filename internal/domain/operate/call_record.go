package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
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
	Billsec              int    `json:"billsec"`                        // 实际通话时长（秒）
	Ringsec              int    `json:"ringsec"`                        // 振铃时长（秒）
	BillingSec           int    `json:"billingSec"`                     // 计费时长（秒）
	GatewayName          string `json:"gatewayName,omitempty"`          // 走了哪个网关
	Extension            string `json:"extension,omitempty"`            // 用的是哪个分机
	SipHangupDisposition string `json:"sipHangupDisposition,omitempty"` // 挂断方向/挂断原因细节 (recv_bye, send_bye 等)
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
	Phone       string `json:"phone,omitempty"`       // 客户号码 (主叫或被叫)
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
	Repository  CallRecordRepository
	RedisClient *goredis.Client
	Logger      *slog.Logger
}

// SipTraceItem 描述时序图中的单个 SIP 消息节点
type SipTraceItem struct {
	ID        uint   `json:"id"`
	Timestamp string `json:"timestamp"`
	Method    string `json:"method"` // 动作 (如: INVITE)
	Status    string `json:"status"` // 状态码 (如: 200 OK)
	FromIP    string `json:"fromIp"` // 源节点 (例如: 10.10.0.155:8611)
	ToIP      string `json:"toIp"`   // 目的节点 (例如: 10.10.0.187:5060)
	RawMsg    string `json:"rawMsg"` // 原始报文体
}

type CallSipTraceResult struct {
	CallID string         `json:"callId"`
	Nodes  []string       `json:"nodes"` // 参与呼叫的核心 IP 节点列表（时序图横向坐标）
	Trace  []SipTraceItem `json:"trace"` // 按时间线排序的信令包
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

// SipTrace 从 Redis 加载 SIP 链路交互详情，并返回时序数据结构。
func (s *CallRecordManagementService) SipTrace(ctx context.Context, callID string) (CallSipTraceResult, error) {
	logger := s.logger()
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return CallSipTraceResult{}, ErrInvalidCallRecord
	}
	logger.Info("商户端开始获取呼叫信令 Trace", "callId", callID)

	if s.RedisClient == nil {
		logger.Warn("Redis 客户端未注入，信令链路追踪不可用")
		return CallSipTraceResult{CallID: callID, Nodes: []string{}, Trace: []SipTraceItem{}}, nil
	}

	// 1. 获取 CDR 记录以确认是否存在，顺便拿到 UUID 作为 fallback
	record, err := s.Repository.GetByCallID(ctx, callID)
	if err != nil {
		logger.Warn("未找到呼叫记录，可能已过期或不存在", "callId", callID, "error", err.Error())
		return CallSipTraceResult{CallID: callID, Nodes: []string{}, Trace: []SipTraceItem{}}, nil
	}

	// 2. 从 Redis 读取 trace 列表 (支持以 CallID 和 UUID 作为 Redis Key)
	var rawList []string
	key1 := "sip_trace:" + record.CallID
	rawList, err = s.RedisClient.LRange(ctx, key1, 0, -1).Result()
	if err != nil {
		logger.Error("从 Redis 读取 CallID 信令失败", "key", key1, "error", err.Error())
	}

	if len(rawList) == 0 && record.UUID != "" {
		key2 := "sip_trace:" + record.UUID
		rawList, err = s.RedisClient.LRange(ctx, key2, 0, -1).Result()
		if err != nil {
			logger.Error("从 Redis 读取 UUID 信令失败", "key", key2, "error", err.Error())
		}
	}

	// 3. 解析 Redis 报文数据并收集节点 IP
	traceItems := make([]SipTraceItem, 0, len(rawList))
	nodesMap := make(map[string]struct{})

	for idx, val := range rawList {
		item, ok := parseSipTraceItem(val, idx)
		if !ok {
			continue
		}
		traceItems = append(traceItems, item)

		if item.FromIP != "" && item.FromIP != ":" {
			nodesMap[item.FromIP] = struct{}{}
		}
		if item.ToIP != "" && item.ToIP != ":" {
			nodesMap[item.ToIP] = struct{}{}
		}
	}

	// 4. 将节点 map 转为时序图生存线坐标头部数组（去重）
	nodes := make([]string, 0, len(nodesMap))
	for node := range nodesMap {
		nodes = append(nodes, node)
	}

	logger.Info("商户端获取呼叫信令 Trace 完成", "callId", callID, "traceCount", len(traceItems), "nodeCount", len(nodes))
	return CallSipTraceResult{
		CallID: callID,
		Nodes:  nodes,
		Trace:  traceItems,
	}, nil
}

func parseSipTraceItem(val string, idx int) (SipTraceItem, bool) {
	parts := strings.Split(val, "###")
	if len(parts) < 6 {
		return SipTraceItem{}, false
	}

	// parts[0]: timestamp.microseconds (e.g. 1780630821.123456)
	// parts[1]: method (e.g. INVITE)
	// parts[2]: status (e.g. 200 or 0 for request)
	// parts[3]: from_ip:port
	// parts[4]: to_ip:port
	// parts[5:]: message_body
	rawMsg := strings.Join(parts[5:], "###")

	status := parts[2]
	if status == "0" || status == "" {
		status = ""
	}

	return SipTraceItem{
		ID:        uint(idx + 1),
		Timestamp: parts[0],
		Method:    parts[1],
		Status:    status,
		FromIP:    parts[3],
		ToIP:      parts[4],
		RawMsg:    rawMsg,
	}, true
}
