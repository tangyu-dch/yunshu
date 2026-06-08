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
	SkillGroupID    int
	DepartmentID    int
	CallMode        int
	CallRatio       float64
	QueueEnable     bool
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
	GetIdleAgentFromSkillGroup(ctx context.Context, merchantID, skillGroupID int) (int, string, error)
	GetOnlineAgents(ctx context.Context, skillGroupID int) ([]int, error)
	GetActiveCallCount(ctx context.Context, taskID int) (int, error)
	GetAgentSkillGroups(ctx context.Context, userID int) ([]int, error)
}

// CallQueue 定义排队功能需要的队列操作端口。
// 所有方法均携带 merchantID 以实现多租户 Redis Key 前缀隔离，
// 确保不同商户的排队数据完全独立，避免跨商户互相污染。
type CallQueue interface {
	// Push 将呼叫 ID 推入对应商户与技能组的排队队列。
	Push(ctx context.Context, merchantID, skillGroupID int, callID string) error
	// Pop 从队列中弹出一个等待呼叫 ID（FIFO 语义，空队列返回空字符串）。
	Pop(ctx context.Context, merchantID, skillGroupID int) (string, error)
	// Len 获取当前队列的排队人数。
	Len(ctx context.Context, merchantID, skillGroupID int) (int, error)
	// Remove 原子地从队列中移除指定呼叫 ID（用于超时退出和客户主动挂机清理）。
	// 返回实际被移除的数量（0 表示已被坐席接听或已挂断，>0 表示成功清理）。
	Remove(ctx context.Context, merchantID, skillGroupID int, callID string) (int64, error)
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
	Queue      CallQueue
	ESL        BatchESLClient
	Events     events.Bus
	Candidates CandidateSource  // CTI 选号候选源（Redis 缓存穿透）
	Selector   *RuntimeSelector // CTI 运行时选号器（规则链 + Redis 原子占用）
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

	var assignedUserID int
	var assignedExtension string
	if task.SkillGroupID > 0 && !task.AIFlag {
		if task.CallMode == 1 { // 预测模式
			onlineAgents, err := s.Repository.GetOnlineAgents(ctx, task.SkillGroupID)
			if err != nil {
				logger.Error("获取技能组在线坐席失败", "taskId", taskID, "skillGroupId", task.SkillGroupID, "error", err.Error())
				return contracts.BatchCallReq{}, "", err
			}
			if len(onlineAgents) == 0 {
				logger.Warn("当前技能组内无在线坐席，本次调度跳过号码起呼", "taskId", taskID, "skillGroupId", task.SkillGroupID)
				return contracts.BatchCallReq{}, "", fmt.Errorf("no online agent available in skill group %d", task.SkillGroupID)
			}

			activeCalls, err := s.Repository.GetActiveCallCount(ctx, taskID)
			if err != nil {
				logger.Error("获取活动呼叫数量失败", "taskId", taskID, "error", err.Error())
				return contracts.BatchCallReq{}, "", err
			}
			ratio := task.CallRatio
			if ratio <= 0 {
				ratio = 1.0
			}
			maxConcurrentCalls := float64(len(onlineAgents)) * ratio
			if float64(activeCalls) >= maxConcurrentCalls {
				logger.Info("批量外呼并发数达到比例上限，暂不起呼", "taskId", taskID, "activeCalls", activeCalls, "maxConcurrent", maxConcurrentCalls, "onlineCount", len(onlineAgents), "ratio", ratio)
				return contracts.BatchCallReq{}, "", fmt.Errorf("concurrency limit reached (active: %d, max: %.2f)", activeCalls, maxConcurrentCalls)
			}
		} else { // 协同模式
			var getAgentErr error
			assignedUserID, assignedExtension, getAgentErr = s.Repository.GetIdleAgentFromSkillGroup(ctx, task.MerchantID, task.SkillGroupID)
			if getAgentErr != nil {
				logger.Error("获取技能组空闲坐席失败", "taskId", taskID, "skillGroupId", task.SkillGroupID, "error", getAgentErr.Error())
				return contracts.BatchCallReq{}, "", getAgentErr
			}
			if assignedUserID == 0 || assignedExtension == "" {
				logger.Warn("当前技能组内无可用空闲坐席，本次调度跳过号码起呼", "taskId", taskID, "skillGroupId", task.SkillGroupID)
				return contracts.BatchCallReq{}, "", fmt.Errorf("no idle agent available in skill group %d", task.SkillGroupID)
			}
		}
	}

	tel, err := s.Repository.ClaimNextPendingBatchTel(ctx, taskID, now)
	if err != nil {
		logger.Warn("批量外呼未获取到可派发号码", "taskId", taskID, "error", err.Error())
		return contracts.BatchCallReq{}, "", ErrNoBatchTel
	}

	// 运行时选号：为批量外呼分配主叫号码和网关（高并发原子占用）
	var selectedCaller string
	var selectedGatewayID string
	var gatewayRegister bool = true
	var allocation *RuntimeAllocation
	effectiveUserID := firstNonZero(assignedUserID, tel.UserID, task.UserID)
	if s.Selector != nil && s.Candidates != nil && effectiveUserID > 0 {
		candidates, candErr := s.Candidates.CandidatesForUser(ctx, effectiveUserID)
		if candErr != nil {
			logger.Warn("批量外呼加载选号候选失败，将使用默认路由", "taskId", taskID, "userId", effectiveUserID, "error", candErr.Error())
		} else if len(candidates) > 0 {
			callID := s.callID(task, tel)
			selReq := SelectionRequest{
				CallID:     callID,
				MerchantID: fmt.Sprint(firstNonZero(tel.MerchantID, task.MerchantID)),
				Callee:     tel.Tel,
				UserID:     effectiveUserID,
				Candidates: candidates,
			}
			selResult, allocation, selErr := s.Selector.SelectAndClaim(ctx, selReq)
			if selErr != nil {
				logger.Error("批量外呼选号失败，释放号码并中止调度", "taskId", taskID, "telId", tel.ID, "callee", tel.Tel, "error", selErr.Error())
				_ = s.Repository.ReleaseBatchTel(ctx, taskID, tel.ID, now)
				return contracts.BatchCallReq{}, "", fmt.Errorf("批量选号失败: %w", selErr)
			}
			selectedCaller = allocation.Caller
			selectedGatewayID = allocation.GatewayID
			if selResult.Caller != nil && selResult.Caller.Model == 2 {
				gatewayRegister = false
				if selResult.Caller.GatewayRegion != "" {
					selectedGatewayID = selResult.Caller.GatewayRegion
				}
			}
			logger.Info("批量外呼运行时选号成功", "taskId", taskID, "telId", tel.ID, "caller", selectedCaller, "gatewayId", selectedGatewayID, "candidateIndex", allocation.CandidateIndex)
		}
	}

	callID := s.callID(task, tel)
	req := contracts.BatchCallReq{
		UserID:          effectiveUserID,
		BatchTaskID:     task.ID,
		CallTaskState:   contracts.BatchTaskRunning,
		BatchCallTelID:  tel.ID,
		Phone:           tel.Tel,
		MerchantID:      firstNonZero(tel.MerchantID, task.MerchantID),
		UserName:        contracts.FirstNonEmpty(tel.CustomerName, task.UserName),
		Extension:       contracts.FirstNonEmpty(assignedExtension, task.ExtensionNumber),
		ExtensionID:     task.ExtensionID,
		AIFlag:          task.AIFlag,
		Push:            true,
		Extra:           contracts.FirstNonEmpty(tel.Extra, task.Extra),
		CallMode:        task.CallMode,
		CallRatio:       task.CallRatio,
		QueueEnable:     task.QueueEnable,
		CallerNumber:    selectedCaller,
		CallerGatewayID: selectedGatewayID,
		GatewayRegister:   gatewayRegister,
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
		if selectedCaller != "" && s.Selector != nil && s.Selector.Allocator != nil {
			if relErr := s.Selector.Allocator.Release(ctx, *allocation); relErr != nil {
				logger.Error("批量外呼 ESL 失败后释放选号槽失败", "callId", callID, "error", relErr.Error())
			}
		}
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
