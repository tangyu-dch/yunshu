package resource

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

// DepartmentModel 映射生产库中的 `cc_res_department` 表。
type DepartmentModel struct {
	ID          int       `gorm:"column:id;primaryKey"`
	MerchantID  int       `gorm:"column:merchant_id"`
	Name        string    `gorm:"column:name"`
	Description string    `gorm:"column:description"`
	Enable      bool      `gorm:"column:enable"`
	DelFlag     bool      `gorm:"column:del_flag"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

// TableName 返回生产库中的部门表名。
func (DepartmentModel) TableName() string {
	return "cc_res_department"
}

// DepartmentRepository 基于 GORM 实现的部门管理仓储。
type DepartmentRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewDepartmentRepository 创建部门数据库仓储。
func NewDepartmentRepository(db *gorm.DB, logger *slog.Logger) *DepartmentRepository {
	return &DepartmentRepository{DB: db, Logger: logger}
}

// Page 实现部门分页查询。
func (r *DepartmentRepository) Page(ctx context.Context, req operate.DepartmentPageRequest) (operate.DepartmentPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&DepartmentModel{}).Where("del_flag = ?", false)
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
		return operate.DepartmentPageResult{}, err
	}
	var models []DepartmentModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.DepartmentPageResult{}, err
	}
	records := make([]operate.Department, 0, len(models))
	for _, model := range models {
		records = append(records, departmentFromModel(model))
	}
	return operate.DepartmentPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 根据 ID 查询单个部门。
func (r *DepartmentRepository) GetByID(ctx context.Context, id int) (operate.Department, error) {
	var model DepartmentModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Department{}, operate.ErrDepartmentNotFound
	}
	return departmentFromModel(model), err
}

// Save 保存或更新部门。
func (r *DepartmentRepository) Save(ctx context.Context, dept operate.Department) (operate.Department, error) {
	r.logger().Info("开始保存部门记录", "id", dept.ID, "name", dept.Name, "merchantId", dept.MerchantID)
	model := departmentToModel(dept)
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
		r.logger().Error("保存部门记录失败", "id", dept.ID, "name", dept.Name, "error", err.Error())
		return operate.Department{}, err
	}
	r.logger().Info("保存部门记录成功", "id", model.ID, "name", model.Name)
	return departmentFromModel(model), nil
}

// Delete 逻辑删除部门。
func (r *DepartmentRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除部门记录", "ids", ids)
	result := r.DB.WithContext(ctx).Model(&DepartmentModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("逻辑删除部门记录失败", "ids", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("逻辑删除部门记录未匹配到有效记录", "ids", ids)
		return operate.ErrDepartmentNotFound
	}
	r.logger().Info("逻辑删除部门记录成功", "ids", ids, "rowsAffected", result.RowsAffected)
	return nil
}

// ListAll 查询商户下所有未删除的部门。
func (r *DepartmentRepository) ListAll(ctx context.Context, merchantID int) ([]operate.Department, error) {
	var models []DepartmentModel
	query := r.DB.WithContext(ctx).Where("merchant_id = ? AND del_flag = ?", merchantID, false)
	if err := query.Order("id DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	records := make([]operate.Department, 0, len(models))
	for _, m := range models {
		records = append(records, departmentFromModel(m))
	}
	return records, nil
}

// HasBindings 检查部门是否被活动外呼任务或账号引用。
func (r *DepartmentRepository) HasBindings(ctx context.Context, ids []int) (bool, error) {
	if len(ids) == 0 {
		return false, nil
	}
	var count int64
	// 检查 cc_biz_task 是否绑定了未完成的任务
	if err := r.DB.WithContext(ctx).Table("cc_biz_task").
		Where("department_id IN ? AND state != ? AND del_flag = ?", ids, 3, false).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	// 检查 cc_sys_account (即 console_account) 是否绑定了部门
	if r.DB.Migrator().HasColumn("cc_sys_account", "department_id") {
		if err := r.DB.WithContext(ctx).Table("cc_sys_account").
			Where("department_id IN ? AND del_flag = ?", ids, false).
			Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (r *DepartmentRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func departmentFromModel(m DepartmentModel) operate.Department {
	return operate.Department{
		ID:          m.ID,
		MerchantID:  m.MerchantID,
		Name:        m.Name,
		Description: m.Description,
		Enable:      m.Enable,
	}
}

func departmentToModel(d operate.Department) DepartmentModel {
	return DepartmentModel{
		ID:          d.ID,
		MerchantID:  d.MerchantID,
		Name:        d.Name,
		Description: d.Description,
		Enable:      d.Enable,
		DelFlag:     false,
	}
}

// MemoryDepartmentRepository 内存版本的部门仓储，供单元测试使用。
type MemoryDepartmentRepository struct {
	mu           sync.Mutex
	nextID       int
	depts        map[int]operate.Department
	MockBindings bool
}

func NewMemoryDepartmentRepository() *MemoryDepartmentRepository {
	return &MemoryDepartmentRepository{
		nextID:       1,
		depts:        make(map[int]operate.Department),
		MockBindings: false,
	}
}

func (r *MemoryDepartmentRepository) Page(_ context.Context, req operate.DepartmentPageRequest) (operate.DepartmentPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var records []operate.Department
	for _, dept := range r.depts {
		if req.MerchantID > 0 && dept.MerchantID != req.MerchantID {
			continue
		}
		if req.Name != "" && !strings.Contains(dept.Name, req.Name) {
			continue
		}
		if req.Enable != nil && dept.Enable != *req.Enable {
			continue
		}
		records = append(records, dept)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Department{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.DepartmentPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryDepartmentRepository) GetByID(_ context.Context, id int) (operate.Department, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	dept, ok := r.depts[id]
	if !ok {
		return operate.Department{}, operate.ErrDepartmentNotFound
	}
	return dept, nil
}

func (r *MemoryDepartmentRepository) Save(_ context.Context, dept operate.Department) (operate.Department, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if dept.ID == 0 {
		dept.ID = r.nextID
		r.nextID++
	}
	r.depts[dept.ID] = dept
	return dept, nil
}

func (r *MemoryDepartmentRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if _, ok := r.depts[id]; !ok {
			return operate.ErrDepartmentNotFound
		}
		delete(r.depts, id)
	}
	return nil
}

func (r *MemoryDepartmentRepository) ListAll(_ context.Context, merchantID int) ([]operate.Department, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var records []operate.Department
	for _, dept := range r.depts {
		if dept.MerchantID == merchantID {
			records = append(records, dept)
		}
	}
	return records, nil
}

func (r *MemoryDepartmentRepository) HasBindings(_ context.Context, ids []int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.MockBindings, nil
}
