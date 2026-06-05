package business

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// Page 返回批量任务管理分页结果。
func (r *BatchRepository) Page(ctx context.Context, req operate.BatchTaskPageRequest) (operate.BatchTaskPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&MerchantBatchCallTaskModel{}).Where("cc_biz_task.del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("cc_biz_task.name LIKE ?", "%"+req.Name+"%")
	}
	if req.MerchantID > 0 {
		query = query.Where("cc_biz_task.merchant_id = ?", req.MerchantID)
	}
	if req.UserID > 0 {
		query = query.Where("cc_biz_task.user_id = ?", req.UserID)
	}
	if req.Enable != nil {
		query = query.Where("cc_biz_task.enable = ?", *req.Enable)
	}
	if req.SkillGroupID > 0 {
		query = query.Where("cc_biz_task.skill_group_id = ?", req.SkillGroupID)
	}
	if req.DepartmentID > 0 {
		query = query.Where("cc_biz_task.department_id = ?", req.DepartmentID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.BatchTaskPageResult{}, err
	}
	var rows []struct {
		MerchantBatchCallTaskModel
		ConnectedCount int `gorm:"column:connected_count"`
	}
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Select("cc_biz_task.*, (SELECT COUNT(*) FROM cc_biz_task_list WHERE task_id = cc_biz_task.id AND connect_status = ? AND del_flag = ?) AS connected_count", true, false).
		Order("cc_biz_task.id DESC").Offset(offset).Limit(req.PageSize).Find(&rows).Error; err != nil {
		return operate.BatchTaskPageResult{}, err
	}
	records := make([]operate.BatchTask, 0, len(rows))
	for _, row := range rows {
		task := batchTaskFromModel(row.MerchantBatchCallTaskModel)
		task.ConnectedCount = row.ConnectedCount
		records = append(records, task)
	}
	return operate.BatchTaskPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 读取单个批量任务。
func (r *BatchRepository) GetByID(ctx context.Context, id int) (operate.BatchTask, error) {
	var row struct {
		MerchantBatchCallTaskModel
		ConnectedCount int `gorm:"column:connected_count"`
	}
	err := r.DB.WithContext(ctx).Model(&MerchantBatchCallTaskModel{}).
		Select("cc_biz_task.*, (SELECT COUNT(*) FROM cc_biz_task_list WHERE task_id = cc_biz_task.id AND connect_status = ? AND del_flag = ?) AS connected_count", true, false).
		Where("cc_biz_task.id = ? AND cc_biz_task.del_flag = ?", id, false).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.BatchTask{}, operate.ErrBatchTaskNotFound
	}
	if err != nil {
		return operate.BatchTask{}, err
	}
	task := batchTaskFromModel(row.MerchantBatchCallTaskModel)
	task.ConnectedCount = row.ConnectedCount
	return task, nil
}

// Save 新增或更新批量任务。
func (r *BatchRepository) Save(ctx context.Context, task operate.BatchTask) (operate.BatchTask, error) {
	r.logger().Info("开始保存批量外呼任务配置", "id", task.ID, "merchantId", task.MerchantID, "userId", task.UserID, "name", task.Name)
	model := batchTaskToModel(task)
	now := time.Now().UTC()
	model.UpdatedTime = now
	if model.ID == 0 {
		model.CreatedTime = now
	}
	tx := r.DB.WithContext(ctx)
	if model.ID != 0 {
		tx = tx.Omit("created_time")
	}
	if err := tx.Save(&model).Error; err != nil {
		r.logger().Error("保存批量外呼任务配置失败", "id", task.ID, "merchantId", task.MerchantID, "name", task.Name, "error", err.Error())
		return operate.BatchTask{}, err
	}
	r.logger().Info("保存批量外呼任务配置成功", "id", model.ID, "merchantId", model.MerchantID, "name", model.Name)
	return batchTaskFromModel(model), nil
}

// Delete 逻辑删除批量任务。
func (r *BatchRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除批量外呼任务", "ids", ids)
	result := r.DB.WithContext(ctx).Model(&MerchantBatchCallTaskModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("逻辑删除批量外呼任务失败", "ids", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("逻辑删除批量外呼任务未匹配到有效记录", "ids", ids)
		return operate.ErrBatchTaskNotFound
	}
	r.logger().Info("逻辑删除批量外呼任务成功", "ids", ids, "rowsAffected", result.RowsAffected)
	return nil
}

// SetEnable 切换批量任务启用状态，并可同步暂停原因。
func (r *BatchRepository) SetEnable(ctx context.Context, id int, enable bool, pausedReason string) (operate.BatchTask, error) {
	r.logger().Info("开始修改批量外呼任务启用状态", "id", id, "enable", enable, "pausedReason", pausedReason)
	updates := map[string]any{"enable": enable, "updated_time": time.Now().UTC()}
	if enable {
		updates["paused_reason"] = ""
	} else if pausedReason != "" {
		updates["paused_reason"] = pausedReason
	}
	result := r.DB.WithContext(ctx).Model(&MerchantBatchCallTaskModel{}).
		Where("id = ? AND del_flag = ?", id, false).
		Updates(updates)
	if result.Error != nil {
		r.logger().Error("修改批量外呼任务启用状态失败", "id", id, "enable", enable, "error", result.Error.Error())
		return operate.BatchTask{}, result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("修改批量外呼任务启用状态未匹配到有效记录", "id", id)
		return operate.BatchTask{}, operate.ErrBatchTaskNotFound
	}
	r.logger().Info("修改批量外呼任务启用状态成功", "id", id, "enable", enable)
	return r.GetByID(ctx, id)
}

// MemoryBatchTaskRepository 是本地开发和测试使用的批量任务仓储。
type MemoryBatchTaskRepository struct {
	mu      sync.Mutex
	nextID  int
	tasks   map[int]operate.BatchTask
	details map[int][]operate.BatchTaskDetail
}

// NewMemoryBatchTaskRepository 创建内存批量任务仓储。
func NewMemoryBatchTaskRepository() *MemoryBatchTaskRepository {
	return &MemoryBatchTaskRepository{
		nextID:  1,
		tasks:   map[int]operate.BatchTask{},
		details: map[int][]operate.BatchTaskDetail{},
	}
}

func (r *MemoryBatchTaskRepository) Page(_ context.Context, req operate.BatchTaskPageRequest) (operate.BatchTaskPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.BatchTask, 0, len(r.tasks))
	for _, task := range r.tasks {
		if req.Name != "" && !strings.Contains(task.Name, req.Name) {
			continue
		}
		if req.MerchantID > 0 && task.MerchantID != req.MerchantID {
			continue
		}
		if req.UserID > 0 && task.UserID != req.UserID {
			continue
		}
		if req.Enable != nil && task.Enable != *req.Enable {
			continue
		}
		if req.SkillGroupID > 0 && task.SkillGroupID != req.SkillGroupID {
			continue
		}
		if req.DepartmentID > 0 && task.DepartmentID != req.DepartmentID {
			continue
		}
		records = append(records, task)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.BatchTask{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.BatchTaskPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryBatchTaskRepository) GetByID(_ context.Context, id int) (operate.BatchTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[id]
	if !ok {
		return operate.BatchTask{}, operate.ErrBatchTaskNotFound
	}
	return task, nil
}

func (r *MemoryBatchTaskRepository) Save(_ context.Context, task operate.BatchTask) (operate.BatchTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task.ID == 0 {
		task.ID = r.nextID
		r.nextID++
	}
	r.tasks[task.ID] = task
	return task, nil
}

func (r *MemoryBatchTaskRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for _, id := range ids {
		if _, ok := r.tasks[id]; ok {
			delete(r.tasks, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrBatchTaskNotFound
	}
	return nil
}

func (r *MemoryBatchTaskRepository) SetEnable(_ context.Context, id int, enable bool, pausedReason string) (operate.BatchTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[id]
	if !ok {
		return operate.BatchTask{}, operate.ErrBatchTaskNotFound
	}
	task.Enable = enable
	if enable {
		task.PausedReason = ""
	} else {
		task.PausedReason = pausedReason
	}
	r.tasks[id] = task
	return task, nil
}

func (r *MemoryBatchTaskRepository) ImportTels(_ context.Context, taskID int, merchantID int, userID int, tels []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return operate.ErrBatchTaskNotFound
	}
	var details []operate.BatchTaskDetail
	for i, tel := range tels {
		details = append(details, operate.BatchTaskDetail{
			ID:           i + 1,
			TaskID:       taskID,
			MerchantID:   merchantID,
			UserID:       userID,
			CustomerName: fmt.Sprintf("客户 %d", i+1),
			Tel:          tel,
			TelView:      tel,
			CallStatus:   1,
			Extra:        "",
		})
	}
	r.details[taskID] = append(r.details[taskID], details...)
	task.TotalCount = len(r.details[taskID])
	r.tasks[taskID] = task
	return nil
}

func (r *MemoryBatchTaskRepository) GetDetails(_ context.Context, taskID int) ([]operate.BatchTaskDetail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.details[taskID], nil
}

func (r *MemoryBatchTaskRepository) GetIdleAgentFromSkillGroup(ctx context.Context, skillGroupID int) (int, string, error) {
	return 0, "", nil
}

func (r *MemoryBatchTaskRepository) GetOnlineAgents(ctx context.Context, skillGroupID int) ([]int, error) {
	return []int{1}, nil
}

func (r *MemoryBatchTaskRepository) GetActiveCallCount(ctx context.Context, taskID int) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	details := r.details[taskID]
	count := 0
	for _, d := range details {
		if d.CallStatus == 2 { // calling
			count++
		}
	}
	return count, nil
}

func (r *MemoryBatchTaskRepository) GetAgentSkillGroups(ctx context.Context, userID int) ([]int, error) {
	return []int{1}, nil
}

func batchTaskToModel(task operate.BatchTask) MerchantBatchCallTaskModel {
	return MerchantBatchCallTaskModel{
		ID:                  task.ID,
		MerchantID:          task.MerchantID,
		UserID:              task.UserID,
		Name:                task.Name,
		State:               task.State,
		LastStartTime:       task.LastStartTime,
		ConnectedInterval:   task.ConnectedInterval,
		UnconnectedInterval: task.UnconnectedInterval,
		CallTimePeriod:      task.CallTimePeriod,
		TerminationTime:     task.TerminationTime,
		TotalCount:          task.TotalCount,
		CalledCount:         task.CalledCount,
		AIFlag:              task.AIFlag,
		Extra:               task.Extra,
		PausedReason:        task.PausedReason,
		Enable:              task.Enable,
		DelFlag:             false,
		SkillGroupID:        task.SkillGroupID,
		DepartmentID:        task.DepartmentID,
		CallMode:            task.CallMode,
		CallRatio:           task.CallRatio,
		QueueEnable:         task.QueueEnable,
	}
}

func batchTaskFromModel(model MerchantBatchCallTaskModel) operate.BatchTask {
	return operate.BatchTask{
		ID:                  model.ID,
		MerchantID:          model.MerchantID,
		UserID:              model.UserID,
		Name:                model.Name,
		State:               model.State,
		LastStartTime:       model.LastStartTime,
		ConnectedInterval:   model.ConnectedInterval,
		UnconnectedInterval: model.UnconnectedInterval,
		CallTimePeriod:      model.CallTimePeriod,
		TerminationTime:     model.TerminationTime,
		TotalCount:          model.TotalCount,
		CalledCount:         model.CalledCount,
		AIFlag:              model.AIFlag,
		Extra:               model.Extra,
		PausedReason:        model.PausedReason,
		Enable:              model.Enable,
		SkillGroupID:        model.SkillGroupID,
		DepartmentID:        model.DepartmentID,
		CallMode:            model.CallMode,
		CallRatio:           model.CallRatio,
		QueueEnable:         model.QueueEnable,
	}
}
