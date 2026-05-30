package resource

// directory 包属于基础设施（infra）层中的仓储层，负责将领域层的 operate 实体持久化到 MySQL/GORM 物理存储。
// 当前文件主要针对商户号码组（PhoneGroupModel）、号码组号码关系映射（PhoneGroupPoolPhoneRefModel）
// 以及号码组技能组映射关系进行数据库的交互封装，并提供高并发多实例安全的 GORM 实现与纯内存开发的 Memory 存根。

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// PhoneGroupModel 映射数据库 `merchant_phone_group` 号码组主物理表。
type PhoneGroupModel struct {
	ID          int       `gorm:"column:id;primaryKey"`     // 号码组唯一自增ID
	Name        string    `gorm:"column:name"`              // 号码组名称，用于呼叫分配和前端筛选识别
	Remark      string    `gorm:"column:remark"`            // 备注
	Desc        string    `gorm:"column:desc"`              // 详尽描述
	MerchantID  int       `gorm:"column:merchant_id;index"` // 所属商户ID，用于数据范围硬隔离隔离
	Enable      bool      `gorm:"column:enable"`            // 状态启用标志，未启用的号码组不参与话务分发
	DelFlag     bool      `gorm:"column:del_flag"`          // 逻辑删除标志，del_flag=true 视为彻底注销
	CreatedTime time.Time `gorm:"column:created_time"`      // 创建UTC时间
	UpdatedTime time.Time `gorm:"column:updated_time"`      // 修改时间
}

// TableName 指定物理数据库表名为 `merchant_phone_group`。
func (PhoneGroupModel) TableName() string {
	return "merchant_phone_group"
}

// PhoneGroupPoolPhoneRefModel 关系模型，关联号码池子号码和号码组。
// 实现 M:N（多对多）映射关系，方便号码按组分组调度。
type PhoneGroupPoolPhoneRefModel struct {
	PoolPhoneID  int `gorm:"column:pool_phone_id;primaryKey"`  // 号码池号码主键ID
	PhoneGroupID int `gorm:"column:phone_group_id;primaryKey"` // 所属号码组主键ID
	MerchantID   int `gorm:"column:merchant_id;index"`         // 所属商户ID
}

// TableName 指定物理表名为 `merchant_phone_group_pool_phone_ref`。
func (PhoneGroupPoolPhoneRefModel) TableName() string {
	return "merchant_phone_group_pool_phone_ref"
}

// PhoneGroupSkillGroupRefModel 关系模型，关联技能组（SkillGroup）和号码组（PhoneGroup）。
// 呼叫调度时，特定技能组的话务只会分配给其绑定的号码组内包含的可用号码。
type PhoneGroupSkillGroupRefModel struct {
	PhoneGroupID int `gorm:"column:phone_group_id;primaryKey"` // 所属号码组ID
	SkillGroupID int `gorm:"column:skill_group_id;primaryKey"` // 绑定的技能组ID
	MerchantID   int `gorm:"column:merchant_id;index"`         // 绑定的商户ID
}

// TableName 指定物理表名为 `merchant_phone_group_skill_group_ref`。
func (PhoneGroupSkillGroupRefModel) TableName() string {
	return "merchant_phone_group_skill_group_ref"
}

// PhoneGroupRepository 为领域层 `operate.PhoneGroupRepository` 提供的标准 GORM 关系数据库物理持久化实现。
type PhoneGroupRepository struct {
	DB     *gorm.DB     // 数据库主从连接句柄
	Logger *slog.Logger // 日志工具
}

// NewPhoneGroupRepository 构造号码组物理仓储实例。
func NewPhoneGroupRepository(db *gorm.DB, logger *slog.Logger) *PhoneGroupRepository {
	return &PhoneGroupRepository{DB: db, Logger: logger}
}

// Page 对号码组数据列表进行分页与条件查询，支持名称模糊搜索与商户隔离。
func (r *PhoneGroupRepository) Page(ctx context.Context, req operate.PhoneGroupPageRequest) (operate.PhoneGroupPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&PhoneGroupModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.MerchantID > 0 {
		query = query.Where("merchant_id = ?", req.MerchantID)
	}
	if req.Enable != nil {
		query = query.Where("enable = ?", *req.Enable)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.PhoneGroupPageResult{}, err
	}
	var models []PhoneGroupModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.PhoneGroupPageResult{}, err
	}
	records := make([]operate.PhoneGroup, 0, len(models))
	for _, model := range models {
		records = append(records, phoneGroupFromModel(model))
	}
	return operate.PhoneGroupPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 通过 ID 查询单个未逻辑删除的号码组。
func (r *PhoneGroupRepository) GetByID(ctx context.Context, id int) (operate.PhoneGroup, error) {
	var model PhoneGroupModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.PhoneGroup{}, operate.ErrPhoneGroupNotFound
	}
	return phoneGroupFromModel(model), err
}

// ExistsName 对同一商户下的号码组名称执行防重冲突校验（支持排除自身 ID）。
func (r *PhoneGroupRepository) ExistsName(ctx context.Context, name string, merchantID int, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&PhoneGroupModel{}).Where("name = ? AND merchant_id = ? AND del_flag = ?", name, merchantID, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或修改号码组主表记录。
// 在更新模式下（ID != 0），利用 `Omit("created_time")` 系统性防止 MySQL 零值写入冲突。
func (r *PhoneGroupRepository) Save(ctx context.Context, group operate.PhoneGroup) (operate.PhoneGroup, error) {
	r.logger().Info("开始保存号码组信息", "id", group.ID, "name", group.Name, "merchantId", group.MerchantID)
	model := phoneGroupToModel(group)
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
		r.logger().Error("保存号码组信息失败", "name", group.Name, "merchantId", group.MerchantID, "error", err.Error())
		return operate.PhoneGroup{}, err
	}
	r.logger().Info("保存号码组信息成功", "id", model.ID, "name", model.Name)
	return phoneGroupFromModel(model), nil
}

// Delete 逻辑注销号码组，并强一致性事务级级联解除绑定的号码与技能组关联记录。
func (r *PhoneGroupRepository) Delete(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	r.logger().Info("开始批量逻辑删除号码组", "ids", ids)
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("phone_group_id IN ?", ids).Delete(&PhoneGroupPoolPhoneRefModel{}).Error; err != nil {
			r.logger().Error("逻辑删除号码组：删除号码关联记录失败", "ids", ids, "error", err.Error())
			return err
		}
		if err := tx.Where("phone_group_id IN ?", ids).Delete(&PhoneGroupSkillGroupRefModel{}).Error; err != nil {
			r.logger().Error("逻辑删除号码组：删除技能组关联记录失败", "ids", ids, "error", err.Error())
			return err
		}
		result := tx.Model(&PhoneGroupModel{}).Where("id IN ?", ids).
			Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
		if result.Error != nil {
			r.logger().Error("逻辑删除号码组数据库更新异常", "ids", ids, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			r.logger().Warn("逻辑删除号码组失败：未匹配到有效记录", "ids", ids)
			return operate.ErrPhoneGroupNotFound
		}
		r.logger().Info("逻辑删除号码组成功", "ids", ids, "rowsAffected", result.RowsAffected)
		return nil
	})
}

// ReplacePhones 在事务中完全替换并重写号码组绑定的号码池号码映射集。
func (r *PhoneGroupRepository) ReplacePhones(ctx context.Context, groupID int, merchantID int, phoneIDs []int) error {
	r.logger().Info("开始更新号码组的绑定号码关联", "groupId", groupID, "merchantId", merchantID, "phoneCount", len(phoneIDs))
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("phone_group_id = ?", groupID).Delete(&PhoneGroupPoolPhoneRefModel{}).Error; err != nil {
			return err
		}
		for _, phoneID := range phoneIDs {
			ref := PhoneGroupPoolPhoneRefModel{PhoneGroupID: groupID, PoolPhoneID: phoneID, MerchantID: merchantID}
			if err := tx.Create(&ref).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		r.logger().Error("更新号码组绑定号码关联失败", "groupId", groupID, "error", err.Error())
		return err
	}
	r.logger().Info("更新号码组绑定号码关联成功", "groupId", groupID, "phoneCount", len(phoneIDs))
	return nil
}

// ReplaceSkillGroups 在事务中完全替换并重写号码组与技能组的绑定映射集。
func (r *PhoneGroupRepository) ReplaceSkillGroups(ctx context.Context, groupID int, merchantID int, skillGroupIDs []int) error {
	r.logger().Info("开始更新号码组的绑定技能组关联", "groupId", groupID, "merchantId", merchantID, "skillGroupCount", len(skillGroupIDs))
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("phone_group_id = ?", groupID).Delete(&PhoneGroupSkillGroupRefModel{}).Error; err != nil {
			return err
		}
		for _, skillGroupID := range skillGroupIDs {
			ref := PhoneGroupSkillGroupRefModel{PhoneGroupID: groupID, SkillGroupID: skillGroupID, MerchantID: merchantID}
			if err := tx.Create(&ref).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		r.logger().Error("更新号码组绑定技能组关联失败", "groupId", groupID, "error", err.Error())
		return err
	}
	r.logger().Info("更新号码组绑定技能组关联成功", "groupId", groupID, "skillGroupCount", len(skillGroupIDs))
	return nil
}

// PhonesByGroup 查询号码组当前绑定的所有号码池主键ID切片。
func (r *PhoneGroupRepository) PhonesByGroup(ctx context.Context, groupID int) ([]int, error) {
	var rows []PhoneGroupPoolPhoneRefModel
	if err := r.DB.WithContext(ctx).Where("phone_group_id = ?", groupID).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PoolPhoneID)
	}
	return ids, nil
}

// SkillGroupsByGroup 查询号码组当前绑定的所有业务技能组主键ID切片。
func (r *PhoneGroupRepository) SkillGroupsByGroup(ctx context.Context, groupID int) ([]int, error) {
	var rows []PhoneGroupSkillGroupRefModel
	if err := r.DB.WithContext(ctx).Where("phone_group_id = ?", groupID).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.SkillGroupID)
	}
	return ids, nil
}

// MemoryPhoneGroupRepository 专为单元测试及无 MySQL 本地轻量运行设计的纯内存安全版本，内部通过互斥锁 mu 实现并发保护。
type MemoryPhoneGroupRepository struct {
	mu        sync.Mutex
	nextID    int
	groups    map[int]operate.PhoneGroup
	phoneRefs map[int][]int
	skillRefs map[int][]int
}

// NewMemoryPhoneGroupRepository 构造内存号码组仓储。
func NewMemoryPhoneGroupRepository() *MemoryPhoneGroupRepository {
	return &MemoryPhoneGroupRepository{nextID: 1, groups: map[int]operate.PhoneGroup{}, phoneRefs: map[int][]int{}, skillRefs: map[int][]int{}}
}

// Page 内存版分页与过滤。
func (r *MemoryPhoneGroupRepository) Page(_ context.Context, req operate.PhoneGroupPageRequest) (operate.PhoneGroupPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.PhoneGroup, 0, len(r.groups))
	for _, group := range r.groups {
		if req.Name != "" && !strings.Contains(group.Name, req.Name) {
			continue
		}
		if req.MerchantID > 0 && group.MerchantID != req.MerchantID {
			continue
		}
		if req.Enable != nil && group.Enable != *req.Enable {
			continue
		}
		records = append(records, group)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.PhoneGroup{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.PhoneGroupPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 内存版详情查询。
func (r *MemoryPhoneGroupRepository) GetByID(_ context.Context, id int) (operate.PhoneGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	group, ok := r.groups[id]
	if !ok {
		return operate.PhoneGroup{}, operate.ErrPhoneGroupNotFound
	}
	return group, nil
}

// ExistsName 内存版重名校验。
func (r *MemoryPhoneGroupRepository) ExistsName(_ context.Context, name string, merchantID int, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, group := range r.groups {
		if id == excludeID {
			continue
		}
		if group.Name == name && group.MerchantID == merchantID {
			return true, nil
		}
	}
	return false, nil
}

// Save 内存版覆盖写。
func (r *MemoryPhoneGroupRepository) Save(_ context.Context, group operate.PhoneGroup) (operate.PhoneGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if group.ID == 0 {
		group.ID = r.nextID
		r.nextID++
	}
	r.groups[group.ID] = group
	return group, nil
}

// Delete 内存版注销及清空映射关系。
func (r *MemoryPhoneGroupRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if _, ok := r.groups[id]; !ok {
			return operate.ErrPhoneGroupNotFound
		}
		delete(r.groups, id)
		delete(r.phoneRefs, id)
		delete(r.skillRefs, id)
	}
	return nil
}

// ReplacePhones 内存版号码替换。
func (r *MemoryPhoneGroupRepository) ReplacePhones(_ context.Context, groupID int, _ int, phoneIDs []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phoneRefs[groupID] = append([]int(nil), phoneIDs...)
	return nil
}

// ReplaceSkillGroups 内存版技能组替换。
func (r *MemoryPhoneGroupRepository) ReplaceSkillGroups(_ context.Context, groupID int, _ int, skillGroupIDs []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skillRefs[groupID] = append([]int(nil), skillGroupIDs...)
	return nil
}

// PhonesByGroup 内存版主键查询。
func (r *MemoryPhoneGroupRepository) PhonesByGroup(_ context.Context, groupID int) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.phoneRefs[groupID]...), nil
}

// SkillGroupsByGroup 内存版技能组查询。
func (r *MemoryPhoneGroupRepository) SkillGroupsByGroup(_ context.Context, groupID int) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.skillRefs[groupID]...), nil
}

// phoneGroupToModel 转换器：将领域操作类型转换为物理模型映射表结构。
func phoneGroupToModel(group operate.PhoneGroup) PhoneGroupModel {
	return PhoneGroupModel{
		ID:         group.ID,
		Name:       group.Name,
		Remark:     group.Remark,
		Desc:       group.Desc,
		MerchantID: group.MerchantID,
		Enable:     group.Enable,
		DelFlag:    false,
	}
}

// phoneGroupFromModel 转换器：将物理数据模型转换为纯净的领域对象。
func phoneGroupFromModel(model PhoneGroupModel) operate.PhoneGroup {
	return operate.PhoneGroup{
		ID:         model.ID,
		Name:       model.Name,
		Remark:     model.Remark,
		Desc:       model.Desc,
		MerchantID: model.MerchantID,
		Enable:     model.Enable,
	}
}

// logger 返回配置的结构化 slog 接口。
func (r *PhoneGroupRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
