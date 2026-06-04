package business

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"yunshu/internal/domain/cti"
	"yunshu/internal/domain/esl"
	operatedomain "yunshu/internal/domain/operate"

	"gorm.io/gorm"
)

const (
	BatchTelNotCalled = 1
	BatchTelCalling   = 2
	BatchTelCompleted = 3

	// BatchTaskStateCompletedDB 是  批量任务完成态的数据库值。
	// 若后续重新设计表结构，应通过兼容 adapter 将新状态映射回旧表。
	BatchTaskStateCompletedDB = 3
)

// MerchantBatchCallTaskModel 映射  `merchant_batch_call_task` 表。
type MerchantBatchCallTaskModel struct {
	ID                  int        `gorm:"column:id;primaryKey"`
	MerchantID          int        `gorm:"column:merchant_id"`
	UserID              int        `gorm:"column:user_id"`
	Name                string     `gorm:"column:name"`
	State               int        `gorm:"column:state"`
	LastStartTime       *time.Time `gorm:"column:last_start_time"`
	ConnectedInterval   int        `gorm:"column:connected_interval"`
	UnconnectedInterval int        `gorm:"column:unconnected_interval"`
	CallTimePeriod      string     `gorm:"column:call_time_period"`
	TerminationTime     *time.Time `gorm:"column:termination_time"`
	TotalCount          int        `gorm:"column:total_count"`
	CalledCount         int        `gorm:"column:called_count"`
	AIFlag              bool       `gorm:"column:ai_flag"`
	Extra               string     `gorm:"column:extra"`
	PausedReason        string     `gorm:"column:paused_reason"`
	Enable              bool       `gorm:"column:enable"`
	DelFlag             bool       `gorm:"column:del_flag"`
	SkillGroupID        int        `gorm:"column:skill_group_id"`
	DepartmentID        int        `gorm:"column:department_id"`
	CallMode            int        `gorm:"column:call_mode"`
	CallRatio           float64    `gorm:"column:call_ratio"`
	QueueEnable         bool       `gorm:"column:queue_enable"`
	CreatedTime         time.Time  `gorm:"column:created_time"`
	UpdatedTime         time.Time  `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的批量任务表名。
func (MerchantBatchCallTaskModel) TableName() string {
	return "cc_biz_task"
}

// MerchantBatchCallTaskListModel 映射  `merchant_batch_call_task_list` 表。
type MerchantBatchCallTaskListModel struct {
	ID            int        `gorm:"column:id;primaryKey"`
	TaskID        int        `gorm:"column:task_id"`
	MerchantID    int        `gorm:"column:merchant_id"`
	UserID        int        `gorm:"column:user_id"`
	CustomerName  string     `gorm:"column:customer_name"`
	Tel           string     `gorm:"column:tel"`
	TelView       string     `gorm:"column:tel_view"`
	CallTime      *time.Time `gorm:"column:call_time"`
	CallStatus    int        `gorm:"column:call_status"`
	ConnectStatus *bool      `gorm:"column:connect_status"`
	Extra         string     `gorm:"column:extra"`
	Enable        bool       `gorm:"column:enable"`
	DelFlag       bool       `gorm:"column:del_flag"`
	CreatedTime   time.Time  `gorm:"column:created_time"`
	UpdatedTime   time.Time  `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的批量号码清单表名。
func (MerchantBatchCallTaskListModel) TableName() string {
	return "cc_biz_task_list"
}

// BatchRepository 读取批量外呼任务和号码清单。
type BatchRepository struct {
	DB       *gorm.DB
	Statuses esl.ExtensionStatusReader
	Logger   *slog.Logger
}

// NewBatchRepository 创建批量任务仓储。
func NewBatchRepository(db *gorm.DB, statuses esl.ExtensionStatusReader, logger *slog.Logger) *BatchRepository {
	return &BatchRepository{DB: db, Statuses: statuses, Logger: logger}
}

// GetRunnableTask 读取可运行的批量任务。
func (r *BatchRepository) GetRunnableTask(ctx context.Context, taskID int) (MerchantBatchCallTaskModel, error) {
	var task MerchantBatchCallTaskModel
	err := r.DB.WithContext(ctx).
		Where("id = ? AND del_flag = ? AND enable = ?", taskID, false, true).
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MerchantBatchCallTaskModel{}, gorm.ErrRecordNotFound
	}
	return task, err
}

// NextPendingTel 读取任务下一个待拨号码。
// 当前只定义最小生产查询，后续调度器会在事务中 CAS 标记号码状态。
func (r *BatchRepository) NextPendingTel(ctx context.Context, taskID int) (MerchantBatchCallTaskListModel, error) {
	var tel MerchantBatchCallTaskListModel
	err := r.DB.WithContext(ctx).
		Where("task_id = ? AND del_flag = ? AND enable = ?", taskID, false, true).
		Order("id ASC").
		First(&tel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MerchantBatchCallTaskListModel{}, gorm.ErrRecordNotFound
	}
	return tel, err
}

// ClaimNextPendingTel 使用事务和条件更新抢占一个待拨号码。
//
// 多个 worker 并发执行时，只有更新条件仍为 NOT_CALLED 的事务能成功，避免同一号码
// 被重复分发到 ESL。
func (r *BatchRepository) ClaimNextPendingTel(ctx context.Context, taskID int, now time.Time) (MerchantBatchCallTaskListModel, error) {
	var claimed MerchantBatchCallTaskListModel
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidate MerchantBatchCallTaskListModel
		if err := tx.
			Where("task_id = ? AND call_status = ? AND del_flag = ? AND enable = ?", taskID, BatchTelNotCalled, false, true).
			Order("id ASC").
			First(&candidate).Error; err != nil {
			return err
		}
		result := tx.Model(&MerchantBatchCallTaskListModel{}).
			Where("id = ? AND call_status = ? AND del_flag = ?", candidate.ID, BatchTelNotCalled, false).
			Updates(map[string]any{"call_status": BatchTelCalling, "call_time": now, "updated_time": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("batch tel claim conflict")
		}
		candidate.CallStatus = BatchTelCalling
		candidate.CallTime = &now
		candidate.UpdatedTime = now
		claimed = candidate
		return nil
	})
	if err != nil {
		r.logger().Warn("批量外呼号码 CAS 占用失败", "taskId", taskID, "error", err.Error())
		return MerchantBatchCallTaskListModel{}, err
	}
	r.logger().Info("批量外呼号码 CAS 占用成功", "taskId", taskID, "telId", claimed.ID)
	return claimed, nil
}

// GetIdleAgentFromSkillGroup 从技能组关联的坐席中寻找一个在线且空闲（ExtensionStatusIdle）的坐席，用于呼叫分配。
func (r *BatchRepository) GetIdleAgentFromSkillGroup(ctx context.Context, skillGroupID int) (int, string, error) {
	r.logger().Info("开始在技能组中查找空闲坐席", "skillGroupId", skillGroupID)
	var agents []struct {
		UserID          int    `gorm:"column:user_id"`
		ExtensionNumber string `gorm:"column:extension_number"`
	}
	err := r.DB.WithContext(ctx).
		Table("cc_res_user_skill_group usg").
		Select("ext.user_id, ext.extension_number").
		Joins("INNER JOIN cc_res_extension ext ON usg.user_id = ext.user_id").
		Where("usg.skill_group_id = ? AND ext.enable = ? AND ext.del_flag = ?", skillGroupID, true, false).
		Order("ext.id ASC").
		Find(&agents).Error
	if err != nil {
		r.logger().Error("在技能组中联查坐席分机失败", "skillGroupId", skillGroupID, "error", err.Error())
		return 0, "", err
	}
	if len(agents) == 0 {
		r.logger().Warn("该技能组下没有绑定任何可用的分机坐席", "skillGroupId", skillGroupID)
		return 0, "", nil
	}

	for _, agent := range agents {
		if r.Statuses == nil {
			r.logger().Warn("未注入 Statuses (ExtensionStatusReader)，跳过 Redis 状态检查", "extension", agent.ExtensionNumber)
			continue
		}
		status, ok, err := r.Statuses.GetExtensionStatus(ctx, agent.ExtensionNumber)
		if err != nil {
			r.logger().Warn("读取分机 Redis 状态失败", "extension", agent.ExtensionNumber, "error", err.Error())
			continue
		}
		if ok && status == esl.ExtensionStatusIdle {
			r.logger().Info("找到空闲坐席", "skillGroupId", skillGroupID, "userId", agent.UserID, "extension", agent.ExtensionNumber)
			return agent.UserID, agent.ExtensionNumber, nil
		}
	}
	r.logger().Warn("技能组内所有在线坐席均处于忙碌或离线状态", "skillGroupId", skillGroupID)
	return 0, "", nil
}

// GetRunnableBatchTask 返回批量调度领域需要的任务快照。
func (r *BatchRepository) GetRunnableBatchTask(ctx context.Context, taskID int) (cti.BatchTaskSnapshot, error) {
	task, err := r.GetRunnableTask(ctx, taskID)
	if err != nil {
		return cti.BatchTaskSnapshot{}, err
	}
	return cti.BatchTaskSnapshot{
		ID:           task.ID,
		MerchantID:   task.MerchantID,
		UserID:       task.UserID,
		State:        task.State,
		AIFlag:       task.AIFlag,
		Extra:        task.Extra,
		SkillGroupID: task.SkillGroupID,
		DepartmentID: task.DepartmentID,
		CallMode:     task.CallMode,
		CallRatio:    task.CallRatio,
		QueueEnable:  task.QueueEnable,
	}, nil
}

// ClaimNextPendingBatchTel 使用事务 CAS 占用号码并返回领域快照。
func (r *BatchRepository) ClaimNextPendingBatchTel(ctx context.Context, taskID int, now time.Time) (cti.BatchTelSnapshot, error) {
	tel, err := r.ClaimNextPendingTel(ctx, taskID, now)
	if err != nil {
		return cti.BatchTelSnapshot{}, err
	}
	return cti.BatchTelSnapshot{
		ID:           tel.ID,
		TaskID:       tel.TaskID,
		MerchantID:   tel.MerchantID,
		UserID:       tel.UserID,
		CustomerName: tel.CustomerName,
		Tel:          tel.Tel,
		Extra:        tel.Extra,
	}, nil
}

// CompleteBatchTel 标记批量号码完成并递增任务已拨统计。
//
// 更新条件要求号码仍处于 CALLING，保证重复终结事件不会重复累计 called_count。
func (r *BatchRepository) CompleteBatchTel(ctx context.Context, taskID, telID int, connected bool, now time.Time) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&MerchantBatchCallTaskListModel{}).
			Where("id = ? AND task_id = ? AND call_status = ? AND del_flag = ?", telID, taskID, BatchTelCalling, false).
			Updates(map[string]any{"call_status": BatchTelCompleted, "connect_status": connected, "updated_time": now})
		if result.Error != nil {
			r.logger().Error("批量外呼号码完成状态更新失败", "taskId", taskID, "telId", telID, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			r.logger().Info("批量外呼号码完成事件已处理或状态不匹配，跳过重复累计", "taskId", taskID, "telId", telID)
			return nil
		}
		if err := tx.Model(&MerchantBatchCallTaskModel{}).
			Where("id = ? AND del_flag = ?", taskID, false).
			Updates(map[string]any{"called_count": gorm.Expr("called_count + ?", 1), "updated_time": now}).Error; err != nil {
			r.logger().Error("批量外呼任务已拨统计更新失败", "taskId", taskID, "telId", telID, "error", err.Error())
			return err
		}
		r.logger().Info("批量外呼号码完成状态更新成功", "taskId", taskID, "telId", telID, "connected", connected)
		return nil
	})
}

// ReleaseBatchTel 将批量号码从 CALLING 回滚到 NOT_CALLED。
//
// 该方法用于批量起呼在进入 FS 之前失败、事件发布失败或外部调用失败后的补偿释放，
// 保证号码不会永久卡在 calling 状态。
func (r *BatchRepository) ReleaseBatchTel(ctx context.Context, taskID, telID int, now time.Time) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&MerchantBatchCallTaskListModel{}).
			Where("id = ? AND task_id = ? AND call_status = ? AND del_flag = ?", telID, taskID, BatchTelCalling, false).
			Updates(map[string]any{"call_status": BatchTelNotCalled, "call_time": nil, "connect_status": nil, "updated_time": now})
		if result.Error != nil {
			r.logger().Error("批量外呼号码释放状态更新失败", "taskId", taskID, "telId", telID, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			r.logger().Info("批量外呼号码释放事件已处理或状态不匹配，跳过重复回滚", "taskId", taskID, "telId", telID)
			return nil
		}
		r.logger().Info("批量外呼号码释放状态更新成功", "taskId", taskID, "telId", telID)
		return nil
	})
}

// GetBatchTaskStats 读取批量任务完成收口所需的统计快照。
func (r *BatchRepository) GetBatchTaskStats(ctx context.Context, taskID int) (cti.BatchTaskStats, error) {
	var task MerchantBatchCallTaskModel
	if err := r.DB.WithContext(ctx).
		Where("id = ? AND del_flag = ?", taskID, false).
		First(&task).Error; err != nil {
		return cti.BatchTaskStats{}, err
	}
	var pendingCount, callingCount, completedCount, connectedCount int64
	if err := r.DB.WithContext(ctx).Model(&MerchantBatchCallTaskListModel{}).
		Where("task_id = ? AND call_status = ? AND del_flag = ? AND enable = ?", taskID, BatchTelNotCalled, false, true).
		Count(&pendingCount).Error; err != nil {
		return cti.BatchTaskStats{}, err
	}
	if err := r.DB.WithContext(ctx).Model(&MerchantBatchCallTaskListModel{}).
		Where("task_id = ? AND call_status = ? AND del_flag = ? AND enable = ?", taskID, BatchTelCalling, false, true).
		Count(&callingCount).Error; err != nil {
		return cti.BatchTaskStats{}, err
	}
	if err := r.DB.WithContext(ctx).Model(&MerchantBatchCallTaskListModel{}).
		Where("task_id = ? AND call_status = ? AND del_flag = ? AND enable = ?", taskID, BatchTelCompleted, false, true).
		Count(&completedCount).Error; err != nil {
		return cti.BatchTaskStats{}, err
	}
	if err := r.DB.WithContext(ctx).Model(&MerchantBatchCallTaskListModel{}).
		Where("task_id = ? AND call_status = ? AND connect_status = ? AND del_flag = ? AND enable = ?", taskID, BatchTelCompleted, true, false, true).
		Count(&connectedCount).Error; err != nil {
		return cti.BatchTaskStats{}, err
	}
	stats := cti.BatchTaskStats{
		TaskID:         task.ID,
		MerchantID:     task.MerchantID,
		UserID:         task.UserID,
		TotalCount:     task.TotalCount,
		CalledCount:    task.CalledCount,
		PendingCount:   int(pendingCount),
		CallingCount:   int(callingCount),
		CompletedCount: int(completedCount),
		ConnectedCount: int(connectedCount),
	}
	r.logger().Info("批量外呼任务统计读取成功", "taskId", taskID, "totalCount", stats.TotalCount, "calledCount", stats.CalledCount, "pendingCount", stats.PendingCount, "callingCount", stats.CallingCount, "completedCount", stats.CompletedCount, "connectedCount", stats.ConnectedCount)
	return stats, nil
}

// CompleteBatchTaskIfDrained 在没有待拨/拨打中号码时完成批量任务。
//
// 该方法把“任务是否完成”的判定放在数据库事务内，避免多个 worker 同时收到最后一批
// FS 终结事件时重复收口。返回 true 表示本次调用完成了任务或任务已处于完成态。
func (r *BatchRepository) CompleteBatchTaskIfDrained(ctx context.Context, taskID int, now time.Time) (bool, error) {
	completed := false
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var activeCount int64
		if err := tx.Model(&MerchantBatchCallTaskListModel{}).
			Where("task_id = ? AND call_status IN ? AND del_flag = ? AND enable = ?", taskID, []int{BatchTelNotCalled, BatchTelCalling}, false, true).
			Count(&activeCount).Error; err != nil {
			r.logger().Error("批量外呼任务完成判定统计号码失败", "taskId", taskID, "error", err.Error())
			return err
		}
		if activeCount > 0 {
			r.logger().Info("批量外呼任务仍有未完成号码，暂不收口", "taskId", taskID, "activeCount", activeCount)
			return nil
		}
		result := tx.Model(&MerchantBatchCallTaskModel{}).
			Where("id = ? AND del_flag = ? AND state <> ?", taskID, false, BatchTaskStateCompletedDB).
			Updates(map[string]any{"state": BatchTaskStateCompletedDB, "termination_time": now, "updated_time": now})
		if result.Error != nil {
			r.logger().Error("批量外呼任务完成状态更新失败", "taskId", taskID, "error", result.Error.Error())
			return result.Error
		}
		completed = true
		r.logger().Info("批量外呼任务完成状态更新成功", "taskId", taskID, "rowsAffected", result.RowsAffected)
		return nil
	})
	return completed, err
}

// ImportTels 批量导入电话号码到任务中。
func (r *BatchRepository) ImportTels(ctx context.Context, taskID int, merchantID int, userID int, tels []string) error {
	r.logger().Info("开始批量导入外呼号码到任务", "taskId", taskID, "merchantId", merchantID, "userId", userID, "count", len(tels))
	if len(tels) == 0 {
		return nil
	}
	now := time.Now().UTC()
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var list []MerchantBatchCallTaskListModel
		for _, tel := range tels {
			list = append(list, MerchantBatchCallTaskListModel{
				TaskID:        taskID,
				MerchantID:    merchantID,
				UserID:        userID,
				Tel:           tel,
				TelView:       tel,
				CallStatus:    BatchTelNotCalled,
				ConnectStatus: nil,
				Enable:        true,
				DelFlag:       false,
				CreatedTime:   now,
				UpdatedTime:   now,
			})
		}
		if err := tx.Create(&list).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&MerchantBatchCallTaskListModel{}).
			Where("task_id = ? AND del_flag = ? AND enable = ?", taskID, false, true).
			Count(&count).Error; err != nil {
			return err
		}
		if err := tx.Model(&MerchantBatchCallTaskModel{}).
			Where("id = ?", taskID).
			Update("total_count", int(count)).Error; err != nil {
			return err
		}
		return nil
	})
	return err
}

// GetDetails 返回指定批量任务的号码拨打明细。
func (r *BatchRepository) GetDetails(ctx context.Context, taskID int) ([]operatedomain.BatchTaskDetail, error) {
	r.logger().Info("开始查询批量任务外呼明细", "taskId", taskID)
	var models []MerchantBatchCallTaskListModel
	err := r.DB.WithContext(ctx).
		Where("task_id = ? AND del_flag = ? AND enable = ?", taskID, false, true).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	var cdrs []struct {
		Callee      string
		DurationSec int
	}
	r.DB.WithContext(ctx).
		Table("cc_biz_cdr").
		Select("callee, MAX(duration_sec) as duration_sec").
		Where("batch_task_id = ?", taskID).
		Group("callee").
		Scan(&cdrs)

	cdrMap := make(map[string]int)
	for _, c := range cdrs {
		cdrMap[c.Callee] = c.DurationSec
	}

	details := make([]operatedomain.BatchTaskDetail, 0, len(models))
	for _, m := range models {
		details = append(details, operatedomain.BatchTaskDetail{
			ID:            m.ID,
			TaskID:        m.TaskID,
			MerchantID:    m.MerchantID,
			UserID:        m.UserID,
			CustomerName:  m.CustomerName,
			Tel:           m.Tel,
			TelView:       m.TelView,
			CallTime:      m.CallTime,
			CallStatus:    m.CallStatus,
			ConnectStatus: m.ConnectStatus,
			Extra:         m.Extra,
			DurationSec:   cdrMap[m.Tel],
		})
	}
	return details, nil
}

// GetOnlineAgents 返回技能组内在线的坐席用户 ID 列表（状态非 -1 离线）。
func (r *BatchRepository) GetOnlineAgents(ctx context.Context, skillGroupID int) ([]int, error) {
	var agents []struct {
		UserID          int    `gorm:"column:user_id"`
		ExtensionNumber string `gorm:"column:extension_number"`
	}
	err := r.DB.WithContext(ctx).
		Table("cc_res_user_skill_group usg").
		Select("ext.user_id, ext.extension_number").
		Joins("INNER JOIN cc_res_extension ext ON usg.user_id = ext.user_id").
		Where("usg.skill_group_id = ? AND ext.enable = ? AND ext.del_flag = ?", skillGroupID, true, false).
		Order("ext.id ASC").
		Find(&agents).Error
	if err != nil {
		r.logger().Error("联查技能组坐席分机失败", "skillGroupId", skillGroupID, "error", err.Error())
		return nil, err
	}
	var onlineUserIDs []int
	for _, agent := range agents {
		if r.Statuses == nil {
			onlineUserIDs = append(onlineUserIDs, agent.UserID)
			continue
		}
		status, ok, err := r.Statuses.GetExtensionStatus(ctx, agent.ExtensionNumber)
		if err != nil {
			r.logger().Warn("读取分机 Redis 状态失败", "extension", agent.ExtensionNumber, "error", err.Error())
			continue
		}
		if ok && status != esl.ExtensionStatusOffline {
			onlineUserIDs = append(onlineUserIDs, agent.UserID)
		}
	}
	return onlineUserIDs, nil
}

// GetActiveCallCount 返回指定批量任务当前起呼/呼叫中的号码数量。
func (r *BatchRepository) GetActiveCallCount(ctx context.Context, taskID int) (int, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&MerchantBatchCallTaskListModel{}).
		Where("task_id = ? AND call_status = ? AND del_flag = ?", taskID, BatchTelCalling, false).
		Count(&count).Error
	if err != nil {
		r.logger().Error("查询批量任务活动呼叫数失败", "taskId", taskID, "error", err.Error())
		return 0, err
	}
	return int(count), nil
}

// GetAgentSkillGroups 返回指定坐席所绑定的所有启用技能组 ID 列表。
func (r *BatchRepository) GetAgentSkillGroups(ctx context.Context, userID int) ([]int, error) {
	var skillGroupIDs []int
	err := r.DB.WithContext(ctx).
		Table("cc_res_user_skill_group").
		Where("user_id = ?", userID).
		Pluck("skill_group_id", &skillGroupIDs).Error
	if err != nil {
		r.logger().Error("查询坐席关联技能组失败", "userId", userID, "error", err.Error())
		return nil, err
	}
	return skillGroupIDs, nil
}

func (r *BatchRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
