package cti

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/infra/events"
)

var (
	ErrInvalidBatchTask = errors.New("invalid batch task")
	ErrNoBatchTel       = errors.New("no pending batch tel")
)

// BatchTaskSnapshot 是批量任务调度需要的最小领域快照。
//
// 领域层只关心调度决策所需字段，不直接依赖 GORM Model 或  表结构。
type BatchTaskSnapshot struct {
	ID              int
	MerchantID      int
	UserID          int
	State           int
	AIFlag          bool
	Extra           string
	ExtensionID     int
	ExtensionNumber string
	UserName        string
}

// BatchTelSnapshot 是单个待拨号码的领域快照。
type BatchTelSnapshot struct {
	ID           int
	TaskID       int
	MerchantID   int
	UserID       int
	CustomerName string
	Tel          string
	Extra        string
}

// BatchTaskStats 是批量任务完成收口时的统计快照。
type BatchTaskStats struct {
	TaskID         int
	MerchantID     int
	UserID         int
	TotalCount     int
	CalledCount    int
	PendingCount   int
	CallingCount   int
	CompletedCount int
	ConnectedCount int
}

// BatchTaskStatsProvider 提供批量任务统计快照，用于完成事件与投影节点。
type BatchTaskStatsProvider interface {
	GetBatchTaskStats(ctx context.Context, taskID int) (BatchTaskStats, error)
}

// BatchTaskRepository 定义批量调度访问任务和号码清单的端口。
type BatchTaskRepository interface {
	GetRunnableBatchTask(ctx context.Context, taskID int) (BatchTaskSnapshot, error)
	ClaimNextPendingBatchTel(ctx context.Context, taskID int, now time.Time) (BatchTelSnapshot, error)
	CompleteBatchTel(ctx context.Context, taskID, telID int, connected bool, now time.Time) error
	ReleaseBatchTel(ctx context.Context, taskID, telID int, now time.Time) error
	CompleteBatchTaskIfDrained(ctx context.Context, taskID int, now time.Time) (bool, error)
}

// BatchESLClient 是批量调度调用 ESL 起呼能力的端口。
type BatchESLClient interface {
	StartBatchOutbound(ctx context.Context, version, callID string, req contracts.BatchCallReq) error
}

// BatchCallIDFactory 生成批量外呼 callID。
type BatchCallIDFactory func(task BatchTaskSnapshot, tel BatchTelSnapshot) string

// BatchSchedulerService 负责任务扫描、号码 CAS 占用、事件发布和 ESL 下发。
//
// 该服务是批量外呼的流程编排入口之一：生产环境可以由 Redis Stream worker、
// 定时器或 HTTP 管理接口触发，但业务逻辑统一留在这里，避免散落 if/else。
type BatchSchedulerService struct {
	Repository BatchTaskRepository
	ESL        BatchESLClient
	Events     events.Bus
	Now        func() time.Time
	NewCallID  BatchCallIDFactory
	Logger     *slog.Logger
}

// DispatchNext 派发指定任务的下一个待拨号码。
func (s *BatchSchedulerService) DispatchNext(ctx context.Context, version string, taskID int) (contracts.BatchCallReq, string, error) {
	logger := s.logger()
	if taskID <= 0 || s.Repository == nil || s.ESL == nil {
		logger.Warn("批量外呼调度参数不完整", "taskId", taskID, "hasRepository", s.Repository != nil, "hasESL", s.ESL != nil)
		return contracts.BatchCallReq{}, "", ErrInvalidBatchTask
	}
	now := s.now()
	logger.Info("开始批量外呼调度", "taskId", taskID, "version", version)
	task, err := s.Repository.GetRunnableBatchTask(ctx, taskID)
	if err != nil {
		logger.Error("读取可运行批量任务失败", "taskId", taskID, "error", err.Error())
		return contracts.BatchCallReq{}, "", err
	}
	tel, err := s.Repository.ClaimNextPendingBatchTel(ctx, taskID, now)
	if err != nil {
		logger.Warn("批量外呼未获取到可派发号码", "taskId", taskID, "error", err.Error())
		return contracts.BatchCallReq{}, "", ErrNoBatchTel
	}
	callID := s.callID(task, tel)
	req := contracts.BatchCallReq{
		UserID:         firstNonZero(tel.UserID, task.UserID),
		BatchTaskID:    task.ID,
		CallTaskState:  contracts.BatchTaskRunning,
		BatchCallTelID: tel.ID,
		Phone:          tel.Tel,
		MerchantID:     firstNonZero(tel.MerchantID, task.MerchantID),
		UserName:       firstNonEmpty(tel.CustomerName, task.UserName),
		Extension:      task.ExtensionNumber,
		ExtensionID:    task.ExtensionID,
		AIFlag:         task.AIFlag,
		Push:           true,
		Extra:          firstNonEmpty(tel.Extra, task.Extra),
	}
	if s.Events != nil {
		if err := s.Events.Publish(ctx, contracts.NewEventEnvelope(
			"batch-call-requested:"+callID,
			contracts.EventBatchCallRequested,
			"batch-call:"+callID,
			"call",
			callID,
			contracts.ServiceCall,
			map[string]any{
				"callId":         callID,
				"batchTaskId":    req.BatchTaskID,
				"batchCallTelId": req.BatchCallTelID,
				"userId":         req.UserID,
				"merchantId":     req.MerchantID,
				"phone":          req.Phone,
			},
		)); err != nil {
			if releaseErr := s.Repository.ReleaseBatchTel(ctx, taskID, tel.ID, now); releaseErr != nil {
				logger.Error("批量外呼请求事件发布失败且号码释放失败", "callId", callID, "taskId", taskID, "telId", tel.ID, "releaseError", releaseErr.Error())
				return contracts.BatchCallReq{}, "", releaseErr
			}
			logger.Error("批量外呼请求事件发布失败", "callId", callID, "taskId", taskID, "telId", tel.ID, "error", err.Error())
			return contracts.BatchCallReq{}, "", err
		}
	}
	if err := s.ESL.StartBatchOutbound(ctx, version, callID, req); err != nil {
		if releaseErr := s.Repository.ReleaseBatchTel(ctx, taskID, tel.ID, now); releaseErr != nil {
			logger.Error("批量外呼调用 ESL 起呼失败且号码释放失败", "callId", callID, "taskId", taskID, "telId", tel.ID, "releaseError", releaseErr.Error())
			return contracts.BatchCallReq{}, "", releaseErr
		}
		logger.Error("批量外呼调用 ESL 起呼失败", "callId", callID, "taskId", taskID, "telId", tel.ID, "error", err.Error())
		return contracts.BatchCallReq{}, "", err
	}
	logger.Info("批量外呼号码派发完成", "callId", callID, "taskId", taskID, "telId", tel.ID, "phone", tel.Tel)
	return req, callID, nil
}

// HandleTerminal 处理批量外呼单号码终结事件。
//
// 该方法由流程消费者调用：先推进号码完成状态，再发布完成事件，最后尝试派发下一个号码。
// 没有待拨号码不是错误，后续任务完成统计会作为独立流程节点继续补齐。
func (s *BatchSchedulerService) HandleTerminal(ctx context.Context, payload map[string]any) error {
	logger := s.logger()
	if s.Repository == nil {
		logger.Warn("批量外呼终结事件缺少仓储，跳过处理", "payload", payload)
		return ErrInvalidBatchTask
	}
	taskID := intFromPayload(payload, "batchTaskId")
	telID := intFromPayload(payload, "batchCallTelId")
	merchantID := intFromPayload(payload, "merchantId")
	userID := intFromPayload(payload, "userId")
	callID, _ := payload["callId"].(string)
	if taskID <= 0 || telID <= 0 {
		logger.Warn("批量外呼终结事件缺少任务或号码标识", "callId", callID, "taskId", taskID, "telId", telID)
		return ErrInvalidBatchTask
	}
	connected := boolFromPayload(payload, "connected")
	now := s.now()
	logger.Info("开始处理批量外呼号码终结流程", "callId", callID, "taskId", taskID, "telId", telID, "connected", connected)
	if err := s.Repository.CompleteBatchTel(ctx, taskID, telID, connected, now); err != nil {
		logger.Error("批量外呼号码完成状态写入失败", "callId", callID, "taskId", taskID, "telId", telID, "error", err.Error())
		return err
	}
	if s.Events != nil {
		if err := s.Events.Publish(ctx, contracts.NewEventEnvelope(
			"batch-call-tel-completed:"+callID,
			contracts.EventBatchCallTelCompleted,
			"batch-call-tel:"+callID,
			"batch_call_tel",
			fmt.Sprintf("%d:%d", taskID, telID),
			contracts.ServiceCall,
			map[string]any{"callId": callID, "batchTaskId": taskID, "batchCallTelId": telID, "connected": connected, "merchantId": merchantID, "userId": userID},
		)); err != nil {
			logger.Error("批量外呼号码完成事件发布失败", "callId", callID, "taskId", taskID, "telId", telID, "error", err.Error())
			return err
		}
	}
	if _, nextCallID, err := s.DispatchNext(ctx, "", taskID); err != nil {
		if errors.Is(err, ErrNoBatchTel) {
			completed, completeErr := s.Repository.CompleteBatchTaskIfDrained(ctx, taskID, now)
			if completeErr != nil {
				logger.Error("批量外呼任务完成判定失败", "taskId", taskID, "telId", telID, "error", completeErr.Error())
				return completeErr
			}
			if completed {
				stats, statsErr := s.taskStats(ctx, taskID, merchantID, userID)
				if statsErr != nil {
					logger.Error("批量外呼任务统计读取失败", "taskId", taskID, "error", statsErr.Error())
					return statsErr
				}
				if err := s.publishTaskCompleted(ctx, stats); err != nil {
					return err
				}
				logger.Info("批量外呼任务已完成并发布收口事件", "taskId", taskID)
				return nil
			}
			logger.Info("批量外呼任务暂无待拨号码，但仍存在拨打中号码，等待后续终结事件", "taskId", taskID)
			return nil
		}
		logger.Error("批量外呼终结后派发下一号码失败", "taskId", taskID, "telId", telID, "error", err.Error())
		return err
	} else {
		logger.Info("批量外呼终结后已派发下一号码", "taskId", taskID, "previousTelId", telID, "nextCallId", nextCallID)
	}
	return nil
}

func (s *BatchSchedulerService) publishTaskCompleted(ctx context.Context, stats BatchTaskStats) error {
	if s.Events == nil {
		return nil
	}
	payload := map[string]any{
		"batchTaskId":    stats.TaskID,
		"merchantId":     stats.MerchantID,
		"userId":         stats.UserID,
		"totalCount":     stats.TotalCount,
		"calledCount":    stats.CalledCount,
		"pendingCount":   stats.PendingCount,
		"callingCount":   stats.CallingCount,
		"completedCount": stats.CompletedCount,
		"connectedCount": stats.ConnectedCount,
	}
	if err := s.Events.Publish(ctx, contracts.NewEventEnvelope(
		fmt.Sprintf("batch-call-task-completed:%d", stats.TaskID),
		contracts.EventBatchCallTaskCompleted,
		fmt.Sprintf("batch-call-task:%d", stats.TaskID),
		"batch_call_task",
		fmt.Sprintf("%d", stats.TaskID),
		contracts.ServiceCall,
		payload,
	)); err != nil {
		s.logger().Error("批量外呼任务完成事件发布失败", "taskId", stats.TaskID, "error", err.Error())
		return err
	}
	return nil
}

func (s *BatchSchedulerService) taskStats(ctx context.Context, taskID, merchantID, userID int) (BatchTaskStats, error) {
	if provider, ok := s.Repository.(BatchTaskStatsProvider); ok {
		return provider.GetBatchTaskStats(ctx, taskID)
	}
	return BatchTaskStats{TaskID: taskID, MerchantID: merchantID, UserID: userID}, nil
}

func (s *BatchSchedulerService) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *BatchSchedulerService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *BatchSchedulerService) callID(task BatchTaskSnapshot, tel BatchTelSnapshot) string {
	if s.NewCallID != nil {
		return s.NewCallID(task, tel)
	}
	return fmt.Sprintf("batch-%d-%d", task.ID, tel.ID)
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func intFromPayload(payload map[string]any, key string) int {
	switch value := payload[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}

func boolFromPayload(payload map[string]any, key string) bool {
	switch value := payload[key].(type) {
	case bool:
		return value
	case string:
		return value == "true" || value == "1"
	default:
		return false
	}
}
