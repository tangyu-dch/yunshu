package resource

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/esl"
	"yunshu/internal/domain/operate"
)

func (r *ExtensionRepository) Page(ctx context.Context, req operate.ExtensionPageRequest) (operate.ExtensionPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&ExtensionModel{}).Where("del_flag = ?", false)
	if req.ExtensionNumber != "" {
		query = query.Where("extension_number LIKE ?", "%"+req.ExtensionNumber+"%")
	}
	if req.MerchantID > 0 {
		query = query.Where("merchant_id = ?", req.MerchantID)
	}
	if req.UserID > 0 {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.Enable != nil {
		query = query.Where("enable = ?", *req.Enable)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.ExtensionPageResult{}, err
	}
	var models []ExtensionModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.ExtensionPageResult{}, err
	}
	records := make([]operate.Extension, 0, len(models))
	for _, model := range models {
		records = append(records, extensionFromModel(model))
	}
	return operate.ExtensionPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *ExtensionRepository) GetByID(ctx context.Context, id int) (operate.Extension, error) {
	var model ExtensionModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Extension{}, operate.ErrExtensionNotFound
	}
	return extensionFromModel(model), err
}

func (r *ExtensionRepository) ExistsNumber(ctx context.Context, extensionNumber string, merchantID int, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&ExtensionModel{}).Where("extension_number = ? AND merchant_id = ? AND del_flag = ?", extensionNumber, merchantID, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新分机配置资料。
func (r *ExtensionRepository) Save(ctx context.Context, extension operate.Extension) (operate.Extension, error) {
	r.logger().Info("开始保存分机配置资料", "id", extension.ID, "extensionNumber", extension.ExtensionNumber, "merchantId", extension.MerchantID, "userId", extension.UserID)
	model := extensionToModel(extension)
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
		r.logger().Error("保存分机配置资料失败", "id", extension.ID, "extensionNumber", extension.ExtensionNumber, "error", err.Error())
		return operate.Extension{}, err
	}
	r.logger().Info("保存分机配置资料成功", "id", model.ID, "extensionNumber", model.ExtensionNumber)
	return extensionFromModel(model), nil
}

// Delete 逻辑删除指定的分机配置。
func (r *ExtensionRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除分机配置", "ids", ids)
	result := r.DB.WithContext(ctx).Model(&ExtensionModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("逻辑删除分机配置失败", "ids", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("逻辑删除分机配置未匹配到有效记录", "ids", ids)
		return operate.ErrExtensionNotFound
	}
	r.logger().Info("逻辑删除分机配置成功", "ids", ids, "rowsAffected", result.RowsAffected)
	return nil
}

// SetEnable 切换指定分机的启用/禁用状态。
func (r *ExtensionRepository) SetEnable(ctx context.Context, id int, enable bool) (operate.Extension, error) {
	r.logger().Info("开始修改分机启用状态", "id", id, "enable", enable)
	result := r.DB.WithContext(ctx).Model(&ExtensionModel{}).
		Where("id = ? AND del_flag = ?", id, false).
		Updates(map[string]any{"enable": enable, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("修改分机启用状态失败", "id", id, "enable", enable, "error", result.Error.Error())
		return operate.Extension{}, result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("修改分机启用状态未匹配到有效记录", "id", id)
		return operate.Extension{}, operate.ErrExtensionNotFound
	}
	r.logger().Info("修改分机启用状态成功", "id", id, "enable", enable)
	return r.GetByID(ctx, id)
}

// DynamicBind 动态绑定坐席到指定分机。
//
// 1. 如果分机被手动绑定到其他用户，则拒绝抢占覆盖；
// 2. 解绑该用户此前动态绑定的所有其他分机；
// 3. 将当前分机绑定至该坐席，并将绑定类型记为 2 (动态绑定)。
func (r *ExtensionRepository) DynamicBind(ctx context.Context, extensionNumber string, userID int, merchantID int) error {
	r.logger().Info("开始为坐席执行分机动态绑定", "extensionNumber", extensionNumber, "userId", userID, "merchantId", merchantID)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var model ExtensionModel
		err := tx.Where("extension_number = ? AND merchant_id = ? AND del_flag = ?", extensionNumber, merchantID, false).First(&model).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return operate.ErrExtensionNotFound
		}
		if err != nil {
			return err
		}

		// 校验：如果已被手动绑定到其他用户，不允许抢占/覆盖
		if model.UserID > 0 && model.UserID != userID && model.BindType == 1 {
			return errors.New("该分机已被手动绑定至其他用户，无法进行动态绑定")
		}

		now := time.Now().UTC()

		// 先解绑该用户之前动态绑定的其他分机
		err = tx.Model(&ExtensionModel{}).
			Where("user_id = ? AND bind_type = ? AND del_flag = ?", userID, 2, false).
			Updates(map[string]any{
				"user_id":      0,
				"bind_type":    1, // 恢复为默认手动类型（且未绑定）
				"updated_time": now,
			}).Error
		if err != nil {
			return err
		}

		// 绑定新分机为动态类型
		err = tx.Model(&ExtensionModel{}).
			Where("id = ?", model.ID).
			Updates(map[string]any{
				"user_id":      userID,
				"bind_type":    2, // 动态绑定
				"updated_time": now,
			}).Error
		return err
	})
	if err != nil {
		r.logger().Warn("坐席分机动态绑定失败", "extensionNumber", extensionNumber, "userId", userID, "merchantId", merchantID, "error", err.Error())
		return err
	}
	r.logger().Info("坐席分机动态绑定成功", "extensionNumber", extensionNumber, "userId", userID, "merchantId", merchantID)
	return nil
}

type MemoryExtensionManagementRepository struct {
	mu         sync.Mutex
	nextID     int
	extensions map[int]operate.Extension
}

func NewMemoryExtensionManagementRepository() *MemoryExtensionManagementRepository {
	return &MemoryExtensionManagementRepository{nextID: 1, extensions: map[int]operate.Extension{}}
}

func (r *MemoryExtensionManagementRepository) Page(_ context.Context, req operate.ExtensionPageRequest) (operate.ExtensionPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Extension, 0, len(r.extensions))
	for _, extension := range r.extensions {
		if req.ExtensionNumber != "" && !strings.Contains(extension.ExtensionNumber, req.ExtensionNumber) {
			continue
		}
		if req.MerchantID > 0 && extension.MerchantID != req.MerchantID {
			continue
		}
		if req.UserID > 0 && extension.UserID != req.UserID {
			continue
		}
		if req.Enable != nil && extension.Enable != *req.Enable {
			continue
		}
		records = append(records, extension)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Extension{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.ExtensionPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryExtensionManagementRepository) GetByID(_ context.Context, id int) (operate.Extension, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	extension, ok := r.extensions[id]
	if !ok {
		return operate.Extension{}, operate.ErrExtensionNotFound
	}
	return extension, nil
}

func (r *MemoryExtensionManagementRepository) ExistsNumber(_ context.Context, extensionNumber string, merchantID int, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, extension := range r.extensions {
		if id == excludeID {
			continue
		}
		if extension.ExtensionNumber == extensionNumber && extension.MerchantID == merchantID {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryExtensionManagementRepository) Save(_ context.Context, extension operate.Extension) (operate.Extension, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if extension.ID == 0 {
		extension.ID = r.nextID
		r.nextID++
	}
	r.extensions[extension.ID] = extension
	return extension, nil
}

func (r *MemoryExtensionManagementRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if _, ok := r.extensions[id]; !ok {
			return operate.ErrExtensionNotFound
		}
		delete(r.extensions, id)
	}
	return nil
}

func (r *MemoryExtensionManagementRepository) SetEnable(_ context.Context, id int, enable bool) (operate.Extension, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	extension, ok := r.extensions[id]
	if !ok {
		return operate.Extension{}, operate.ErrExtensionNotFound
	}
	extension.Enable = enable
	r.extensions[id] = extension
	return extension, nil
}

func (r *MemoryExtensionManagementRepository) DynamicBind(_ context.Context, extensionNumber string, userID int, merchantID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var targetID int = 0
	for id, ext := range r.extensions {
		if ext.ExtensionNumber == extensionNumber && ext.MerchantID == merchantID {
			targetID = id
			break
		}
	}
	if targetID == 0 {
		return operate.ErrExtensionNotFound
	}

	target := r.extensions[targetID]
	if target.UserID > 0 && target.UserID != userID && target.BindType == 1 {
		return errors.New("该分机已被手动绑定至其他用户，无法进行动态绑定")
	}

	// 解绑该用户旧的动态绑定
	for id, ext := range r.extensions {
		if ext.UserID == userID && ext.BindType == 2 {
			ext.UserID = 0
			ext.BindType = 1
			r.extensions[id] = ext
		}
	}

	target.UserID = userID
	target.BindType = 2
	r.extensions[targetID] = target
	return nil
}

func extensionToModel(extension operate.Extension) ExtensionModel {
	bindType := extension.BindType
	if bindType == 0 {
		bindType = 1
	}
	return ExtensionModel{
		ID:              extension.ID,
		ExtensionNumber: extension.ExtensionNumber,
		Password:        extension.Password,
		MerchantID:      extension.MerchantID,
		UserID:          extension.UserID,
		Enable:          extension.Enable,
		BindType:        bindType,
		DelFlag:         false,
		SipDomain:       extension.SipDomain,
		HA1:             extension.HA1,
		HA1b:            extension.HA1b,
	}
}

func extensionFromModel(model ExtensionModel) operate.Extension {
	bindType := model.BindType
	if bindType == 0 {
		bindType = 1
	}
	return operate.Extension{
		ID:              model.ID,
		ExtensionNumber: model.ExtensionNumber,
		Password:        model.Password,
		MerchantID:      model.MerchantID,
		UserID:          model.UserID,
		Enable:          model.Enable,
		BindType:        bindType,
		SipDomain:       model.SipDomain,
		HA1:             model.HA1,
		HA1b:            model.HA1b,
	}
}

// ReleaseOfflineDynamicBindings 扫描所有动态绑定分机，如果离线则自动解除绑定。
//
// 1. 查询数据库中所有当前处于动态绑定状态（bind_type = 2）且已分配用户（user_id > 0）的未删除分机；
// 2. 通过 ExtensionStatusReader (通常由 ESL/Redis 状态实现) 检查分机在线状态；
// 3. 如果分机不在线或处于 Offline 状态，则将其解绑并恢复为默认手动类型（bind_type = 1）。
func ReleaseOfflineDynamicBindings(ctx context.Context, db *gorm.DB, reader esl.ExtensionStatusReader) error {
	slog.Info("开始自动扫描并清理离线坐席的动态分机绑定")
	var models []ExtensionModel
	err := db.WithContext(ctx).
		Where("user_id > 0 AND bind_type = ? AND del_flag = ?", 2, false).
		Find(&models).Error
	if err != nil {
		slog.Error("自动扫描清理离线坐席动态分机绑定失败，查询数据库出错", "error", err.Error())
		return err
	}
	now := time.Now().UTC()
	unbindCount := 0
	for _, model := range models {
		status, ok, err := reader.GetExtensionStatus(ctx, model.ExtensionNumber)
		if err != nil {
			slog.Warn("获取动态绑定分机状态失败", "extensionNumber", model.ExtensionNumber, "error", err.Error())
			continue
		}
		if !ok || status == esl.ExtensionStatusOffline {
			// 离线状态
			if model.OfflineAt == nil {
				// 第一次检测到离线，记录离线时间戳进入 30 秒防抖宽限期
				err = db.WithContext(ctx).Model(&ExtensionModel{}).
					Where("id = ?", model.ID).
					Updates(map[string]any{
						"offline_at":   now,
						"updated_time": now,
					}).Error
				if err != nil {
					slog.Error("记录分机离线时间戳失败", "extensionNumber", model.ExtensionNumber, "error", err.Error())
				} else {
					slog.Info("分机进入离线状态，已记录首次离线时间，进入 30 秒防抖宽限期", "extensionNumber", model.ExtensionNumber, "userId", model.UserID)
				}
			} else if now.Sub(*model.OfflineAt) >= 30*time.Second {
				// 持续离线满 30 秒，执行正式解绑
				err = db.WithContext(ctx).Model(&ExtensionModel{}).
					Where("id = ?", model.ID).
					Updates(map[string]any{
						"user_id":      0,
						"bind_type":    1, // 恢复为默认手动类型
						"offline_at":   nil,
						"updated_time": now,
					}).Error
				if err != nil {
					slog.Error("自动解绑离线分机更新数据库失败", "id", model.ID, "extensionNumber", model.ExtensionNumber, "userId", model.UserID, "error", err.Error())
				} else {
					unbindCount++
					slog.Info("持续离线满 30 秒，动态分机自动解绑成功", "extensionNumber", model.ExtensionNumber, "userId", model.UserID, "merchantId", model.MerchantID)
				}
			}
		} else {
			// 在线状态，如果之前记录过离线时间，清除离线标记以重置宽限期
			if model.OfflineAt != nil {
				err = db.WithContext(ctx).Model(&ExtensionModel{}).
					Where("id = ?", model.ID).
					Updates(map[string]any{
						"offline_at":   nil,
						"updated_time": now,
					}).Error
				if err != nil {
					slog.Error("清除分机离线标记失败", "extensionNumber", model.ExtensionNumber, "error", err.Error())
				} else {
					slog.Info("分机已恢复在线状态，重置防抖宽限期", "extensionNumber", model.ExtensionNumber, "userId", model.UserID)
				}
			}
		}
	}
	slog.Info("自动扫描并清理离线坐席动态分机绑定完成", "unbindCount", unbindCount)
	return nil
}
