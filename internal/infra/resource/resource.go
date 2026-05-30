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

// ChannelRepository 基于 GORM 的渠道管理仓储。
type ChannelRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

func NewChannelRepository(db *gorm.DB, logger *slog.Logger) *ChannelRepository {
	return &ChannelRepository{DB: db, Logger: logger}
}

func (r *ChannelRepository) Page(ctx context.Context, req operate.ChannelPageRequest) (operate.ChannelPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&ChannelModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Enable != nil {
		query = query.Where("enable = ?", *req.Enable)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.ChannelPageResult{}, err
	}
	var models []ChannelModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.ChannelPageResult{}, err
	}
	records := make([]operate.Channel, 0, len(models))
	for _, model := range models {
		records = append(records, channelFromModel(model))
	}
	return operate.ChannelPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *ChannelRepository) GetByID(ctx context.Context, id int) (operate.Channel, error) {
	var model ChannelModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Channel{}, operate.ErrChannelNotFound
	}
	return channelFromModel(model), err
}

func (r *ChannelRepository) ExistsName(ctx context.Context, name string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&ChannelModel{}).Where("name = ? AND del_flag = ?", name, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新呼叫渠道配置。
func (r *ChannelRepository) Save(ctx context.Context, channel operate.Channel) (operate.Channel, error) {
	r.logger().Info("开始保存呼叫渠道配置", "id", channel.ID, "name", channel.Name)
	model := channelToModel(channel)
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
		r.logger().Error("保存呼叫渠道配置失败", "id", channel.ID, "name", channel.Name, "error", err.Error())
		return operate.Channel{}, err
	}
	r.logger().Info("保存呼叫渠道配置成功", "id", model.ID, "name", model.Name)
	return channelFromModel(model), nil
}

// Delete 逻辑删除呼叫渠道配置。
func (r *ChannelRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除呼叫渠道配置", "ids", ids)
	result := r.DB.WithContext(ctx).Model(&ChannelModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("逻辑删除呼叫渠道配置失败", "ids", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("逻辑删除呼叫渠道配置未匹配到有效记录", "ids", ids)
		return operate.ErrChannelNotFound
	}
	r.logger().Info("逻辑删除呼叫渠道配置成功", "ids", ids, "rowsAffected", result.RowsAffected)
	return nil
}

// MemoryChannelRepository 供本地开发和测试使用。
type MemoryChannelRepository struct {
	mu       sync.Mutex
	nextID   int
	channels map[int]operate.Channel
}

func NewMemoryChannelRepository() *MemoryChannelRepository {
	return &MemoryChannelRepository{nextID: 1, channels: map[int]operate.Channel{}}
}

func (r *MemoryChannelRepository) Page(_ context.Context, req operate.ChannelPageRequest) (operate.ChannelPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Channel, 0, len(r.channels))
	for _, channel := range r.channels {
		if req.Name != "" && !strings.Contains(channel.Name, req.Name) {
			continue
		}
		if req.Enable != nil && channel.Enable != *req.Enable {
			continue
		}
		records = append(records, channel)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Channel{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.ChannelPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryChannelRepository) GetByID(_ context.Context, id int) (operate.Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	channel, ok := r.channels[id]
	if !ok {
		return operate.Channel{}, operate.ErrChannelNotFound
	}
	return channel, nil
}

func (r *MemoryChannelRepository) ExistsName(_ context.Context, name string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, channel := range r.channels {
		if id == excludeID {
			continue
		}
		if channel.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryChannelRepository) Save(_ context.Context, channel operate.Channel) (operate.Channel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if channel.ID == 0 {
		channel.ID = r.nextID
		r.nextID++
	}
	r.channels[channel.ID] = channel
	return channel, nil
}

func (r *MemoryChannelRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if _, ok := r.channels[id]; !ok {
			return operate.ErrChannelNotFound
		}
		delete(r.channels, id)
	}
	return nil
}

// PoolRepository 基于 GORM 的号码池管理仓储。
type PoolRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

func NewPoolRepository(db *gorm.DB, logger *slog.Logger) *PoolRepository {
	return &PoolRepository{DB: db, Logger: logger}
}

func (r *PoolRepository) Page(ctx context.Context, req operate.PoolPageRequest) (operate.PoolPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&PoolModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.GatewayID > 0 {
		query = query.Where("gateway_id = ?", req.GatewayID)
	}
	if req.Enable != nil {
		query = query.Where("enable = ?", *req.Enable)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.PoolPageResult{}, err
	}
	var models []PoolModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.PoolPageResult{}, err
	}
	records := make([]operate.Pool, 0, len(models))
	for _, model := range models {
		records = append(records, poolFromModel(model))
	}
	return operate.PoolPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *PoolRepository) GetByID(ctx context.Context, id int) (operate.Pool, error) {
	var model PoolModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Pool{}, operate.ErrPoolNotFound
	}
	return poolFromModel(model), err
}

func (r *PoolRepository) ExistsName(ctx context.Context, name string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&PoolModel{}).Where("name = ? AND del_flag = ?", name, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新号码池配置。
func (r *PoolRepository) Save(ctx context.Context, pool operate.Pool) (operate.Pool, error) {
	r.logger().Info("开始保存号码池配置", "id", pool.ID, "name", pool.Name, "gatewayId", pool.GatewayID)
	model := poolToModel(pool)
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
		r.logger().Error("保存号码池配置失败", "id", pool.ID, "name", pool.Name, "error", err.Error())
		return operate.Pool{}, err
	}
	r.logger().Info("保存号码池配置成功", "id", model.ID, "name", model.Name)
	return poolFromModel(model), nil
}

// Delete 逻辑删除号码池配置，并清空关联号码的 pool_id。
func (r *PoolRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除号码池", "ids", ids)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PoolPhoneModel{}).Where("pool_id IN ?", ids).Update("pool_id", 0).Error; err != nil {
			r.logger().Error("清除号码池关联号码失败", "ids", ids, "error", err.Error())
			return err
		}
		result := tx.Model(&PoolModel{}).Where("id IN ?", ids).
			Updates(map[string]any{"del_flag": true, "gateway_id": 0, "updated_time": time.Now().UTC()})
		if result.Error != nil {
			r.logger().Error("逻辑删除号码池基本信息失败", "ids", ids, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			return operate.ErrPoolNotFound
		}
		return nil
	})
	if err != nil {
		r.logger().Warn("逻辑删除号码池未成功", "ids", ids, "error", err.Error())
		return err
	}
	r.logger().Info("逻辑删除号码池成功", "ids", ids)
	return nil
}

func (r *PoolRepository) ListByGateway(ctx context.Context, gatewayID int) ([]operate.Pool, error) {
	query := r.DB.WithContext(ctx).Model(&PoolModel{}).Where("del_flag = ?", false)
	if gatewayID > 0 {
		query = query.Where("(gateway_id = 0 OR gateway_id = ?)", gatewayID)
	}
	var models []PoolModel
	if err := query.Order("id DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	records := make([]operate.Pool, 0, len(models))
	for _, model := range models {
		records = append(records, poolFromModel(model))
	}
	return records, nil
}

func (r *PoolRepository) ListAll(ctx context.Context) ([]operate.Pool, error) {
	var models []PoolModel
	if err := r.DB.WithContext(ctx).Where("del_flag = ?", false).Order("id DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	records := make([]operate.Pool, 0, len(models))
	for _, model := range models {
		records = append(records, poolFromModel(model))
	}
	return records, nil
}

type MemoryPoolRepository struct {
	mu     sync.Mutex
	nextID int
	pools  map[int]operate.Pool
}

func NewMemoryPoolRepository() *MemoryPoolRepository {
	return &MemoryPoolRepository{nextID: 1, pools: map[int]operate.Pool{}}
}

func (r *MemoryPoolRepository) Page(_ context.Context, req operate.PoolPageRequest) (operate.PoolPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Pool, 0, len(r.pools))
	for _, pool := range r.pools {
		if req.Name != "" && !strings.Contains(pool.Name, req.Name) {
			continue
		}
		if req.GatewayID > 0 && pool.GatewayID != req.GatewayID {
			continue
		}
		if req.Enable != nil && pool.Enable != *req.Enable {
			continue
		}
		records = append(records, pool)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Pool{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.PoolPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryPoolRepository) GetByID(_ context.Context, id int) (operate.Pool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pool, ok := r.pools[id]
	if !ok {
		return operate.Pool{}, operate.ErrPoolNotFound
	}
	return pool, nil
}

func (r *MemoryPoolRepository) ExistsName(_ context.Context, name string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, pool := range r.pools {
		if id == excludeID {
			continue
		}
		if pool.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryPoolRepository) Save(_ context.Context, pool operate.Pool) (operate.Pool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if pool.ID == 0 {
		pool.ID = r.nextID
		r.nextID++
	}
	r.pools[pool.ID] = pool
	return pool, nil
}

func (r *MemoryPoolRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if _, ok := r.pools[id]; !ok {
			return operate.ErrPoolNotFound
		}
		delete(r.pools, id)
	}
	return nil
}

func (r *MemoryPoolRepository) ListByGateway(_ context.Context, gatewayID int) ([]operate.Pool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Pool, 0, len(r.pools))
	for _, pool := range r.pools {
		if pool.GatewayID == 0 || pool.GatewayID == gatewayID {
			records = append(records, pool)
		}
	}
	return records, nil
}

func (r *MemoryPoolRepository) ListAll(_ context.Context) ([]operate.Pool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Pool, 0, len(r.pools))
	for _, pool := range r.pools {
		records = append(records, pool)
	}
	return records, nil
}

// PoolPhoneRepository 基于 GORM 的号码管理仓储。
type PoolPhoneRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

func NewPoolPhoneRepository(db *gorm.DB, logger *slog.Logger) *PoolPhoneRepository {
	return &PoolPhoneRepository{DB: db, Logger: logger}
}

func (r *PoolPhoneRepository) Page(ctx context.Context, req operate.PoolPhonePageRequest) (operate.PoolPhonePageResult, error) {
	query := r.DB.WithContext(ctx).Model(&PoolPhoneModel{}).Where("del_flag = ?", false)
	if req.PoolID > 0 {
		query = query.Where("pool_id = ?", req.PoolID)
	}
	if req.Phone != "" {
		query = query.Where("phone LIKE ?", "%"+req.Phone+"%")
	}
	if req.Enable != nil {
		query = query.Where("enable = ?", *req.Enable)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.PoolPhonePageResult{}, err
	}
	var models []PoolPhoneModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.PoolPhonePageResult{}, err
	}
	records := make([]operate.PoolPhone, 0, len(models))
	for _, model := range models {
		records = append(records, phoneFromModel(model))
	}
	return operate.PoolPhonePageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *PoolPhoneRepository) GetByID(ctx context.Context, id int) (operate.PoolPhone, error) {
	var model PoolPhoneModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.PoolPhone{}, operate.ErrPoolPhoneNotFound
	}
	return phoneFromModel(model), err
}

func (r *PoolPhoneRepository) ExistsPhone(ctx context.Context, phone string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&PoolPhoneModel{}).Where("phone = ? AND del_flag = ?", phone, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新号码池电话号码。
func (r *PoolPhoneRepository) Save(ctx context.Context, phone operate.PoolPhone) (operate.PoolPhone, error) {
	redacted := phone.Phone
	if len(redacted) > 7 {
		redacted = redacted[:3] + "****" + redacted[len(redacted)-4:]
	}
	r.logger().Info("开始保存号码池电话号码", "id", phone.ID, "phone", redacted, "poolId", phone.PoolID)
	model := phoneToModel(phone)
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
		r.logger().Error("保存号码池电话号码失败", "id", phone.ID, "phone", redacted, "error", err.Error())
		return operate.PoolPhone{}, err
	}
	r.logger().Info("保存号码池电话号码成功", "id", model.ID, "poolId", model.PoolID)
	return phoneFromModel(model), nil
}

// Delete 逻辑删除电话号码，并清除技能组关联。
func (r *PoolPhoneRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除号码池电话号码", "ids", ids)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PoolPhoneSkillGroupModel{}).Where("pool_phone_id IN ?", ids).Delete(&PoolPhoneSkillGroupModel{}).Error; err != nil {
			r.logger().Error("清除电话号码技能组关系失败", "ids", ids, "error", err.Error())
			return err
		}
		result := tx.Model(&PoolPhoneModel{}).Where("id IN ?", ids).
			Updates(map[string]any{"del_flag": true, "pool_id": 0, "updated_time": time.Now().UTC()})
		if result.Error != nil {
			r.logger().Error("逻辑删除电话号码基本信息失败", "ids", ids, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			return operate.ErrPoolPhoneNotFound
		}
		return nil
	})
	if err != nil {
		r.logger().Warn("逻辑删除号码池电话号码未成功", "ids", ids, "error", err.Error())
		return err
	}
	r.logger().Info("逻辑删除号码池电话号码成功", "ids", ids)
	return nil
}

// SetEnable 切换电话号码的启用/禁用状态。
func (r *PoolPhoneRepository) SetEnable(ctx context.Context, id int, enable bool) (operate.PoolPhone, error) {
	r.logger().Info("开始修改号码池电话号码启用状态", "id", id, "enable", enable)
	var model PoolPhoneModel
	if err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger().Warn("修改号码池电话号码启用状态未匹配到有效记录", "id", id)
			return operate.PoolPhone{}, operate.ErrPoolPhoneNotFound
		}
		r.logger().Error("修改号码池电话号码启用状态查询失败", "id", id, "error", err.Error())
		return operate.PoolPhone{}, err
	}
	model.Enable = enable
	model.UpdatedTime = time.Now().UTC()
	if err := r.DB.WithContext(ctx).Save(&model).Error; err != nil {
		r.logger().Error("修改号码池电话号码启用状态保存失败", "id", id, "enable", enable, "error", err.Error())
		return operate.PoolPhone{}, err
	}
	r.logger().Info("修改号码池电话号码启用状态成功", "id", id, "enable", enable)
	return phoneFromModel(model), nil
}

// SetPool 批量将号码分配到指定的号码池。
func (r *PoolPhoneRepository) SetPool(ctx context.Context, ids []int, poolID int) error {
	r.logger().Info("开始批量分配号码到指定的号码池", "ids", ids, "poolId", poolID)
	result := r.DB.WithContext(ctx).Model(&PoolPhoneModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"pool_id": poolID, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("批量分配号码到指定的号码池失败", "ids", ids, "poolId", poolID, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("批量分配号码到指定的号码池未匹配到有效记录", "ids", ids)
		return operate.ErrPoolPhoneNotFound
	}
	r.logger().Info("批量分配号码到指定的号码池成功", "ids", ids, "poolId", poolID, "rowsAffected", result.RowsAffected)
	return nil
}

type MemoryPoolPhoneRepository struct {
	mu     sync.Mutex
	nextID int
	phones map[int]operate.PoolPhone
}

func NewMemoryPoolPhoneRepository() *MemoryPoolPhoneRepository {
	return &MemoryPoolPhoneRepository{nextID: 1, phones: map[int]operate.PoolPhone{}}
}

func (r *MemoryPoolPhoneRepository) Page(_ context.Context, req operate.PoolPhonePageRequest) (operate.PoolPhonePageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.PoolPhone, 0, len(r.phones))
	for _, phone := range r.phones {
		if req.PoolID > 0 && phone.PoolID != req.PoolID {
			continue
		}
		if req.Phone != "" && !strings.Contains(phone.Phone, req.Phone) {
			continue
		}
		if req.Enable != nil && phone.Enable != *req.Enable {
			continue
		}
		records = append(records, phone)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.PoolPhone{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.PoolPhonePageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryPoolPhoneRepository) GetByID(_ context.Context, id int) (operate.PoolPhone, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	phone, ok := r.phones[id]
	if !ok {
		return operate.PoolPhone{}, operate.ErrPoolPhoneNotFound
	}
	return phone, nil
}

func (r *MemoryPoolPhoneRepository) ExistsPhone(_ context.Context, phone string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, item := range r.phones {
		if id == excludeID {
			continue
		}
		if item.Phone == phone {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryPoolPhoneRepository) Save(_ context.Context, phone operate.PoolPhone) (operate.PoolPhone, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if phone.ID == 0 {
		phone.ID = r.nextID
		r.nextID++
	}
	r.phones[phone.ID] = phone
	return phone, nil
}

func (r *MemoryPoolPhoneRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if _, ok := r.phones[id]; !ok {
			return operate.ErrPoolPhoneNotFound
		}
		delete(r.phones, id)
	}
	return nil
}

func (r *MemoryPoolPhoneRepository) SetEnable(_ context.Context, id int, enable bool) (operate.PoolPhone, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	phone, ok := r.phones[id]
	if !ok {
		return operate.PoolPhone{}, operate.ErrPoolPhoneNotFound
	}
	phone.Enable = enable
	r.phones[id] = phone
	return phone, nil
}

func (r *MemoryPoolPhoneRepository) SetPool(_ context.Context, ids []int, poolID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		phone, ok := r.phones[id]
		if !ok {
			return operate.ErrPoolPhoneNotFound
		}
		phone.PoolID = poolID
		r.phones[id] = phone
	}
	return nil
}

// SkillGroupRepository 基于 GORM 的技能组管理仓储。
type SkillGroupRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

func NewSkillGroupRepository(db *gorm.DB, logger *slog.Logger) *SkillGroupRepository {
	return &SkillGroupRepository{DB: db, Logger: logger}
}

func (r *SkillGroupRepository) Page(ctx context.Context, req operate.SkillGroupPageRequest) (operate.SkillGroupPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&SkillGroupModel{}).Where("del_flag = ?", false)
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
		return operate.SkillGroupPageResult{}, err
	}
	var models []SkillGroupModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.SkillGroupPageResult{}, err
	}
	records := make([]operate.SkillGroup, 0, len(models))
	for _, model := range models {
		records = append(records, skillGroupFromModel(model))
	}
	return operate.SkillGroupPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *SkillGroupRepository) GetByID(ctx context.Context, id int) (operate.SkillGroup, error) {
	var model SkillGroupModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.SkillGroup{}, operate.ErrSkillGroupNotFound
	}
	return skillGroupFromModel(model), err
}

func (r *SkillGroupRepository) ExistsName(ctx context.Context, name string, merchantID int, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&SkillGroupModel{}).Where("name = ? AND merchant_id = ? AND del_flag = ?", name, merchantID, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新技能组资料。
func (r *SkillGroupRepository) Save(ctx context.Context, skillGroup operate.SkillGroup) (operate.SkillGroup, error) {
	r.logger().Info("开始保存技能组资料", "id", skillGroup.ID, "name", skillGroup.Name, "merchantId", skillGroup.MerchantID)
	model := skillGroupToModel(skillGroup)
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
		r.logger().Error("保存技能组资料失败", "id", skillGroup.ID, "name", skillGroup.Name, "error", err.Error())
		return operate.SkillGroup{}, err
	}
	r.logger().Info("保存技能组资料成功", "id", model.ID, "name", model.Name)
	return skillGroupFromModel(model), nil
}

// Delete 逻辑删除技能组配置，并清空关联的号码技能组和用户技能组关系。
func (r *SkillGroupRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除技能组", "ids", ids)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("skill_group_id IN ?", ids).Delete(&PoolPhoneSkillGroupModel{}).Error; err != nil {
			r.logger().Error("清除技能组号码绑定关系失败", "ids", ids, "error", err.Error())
			return err
		}
		if err := tx.Where("skill_group_id IN ?", ids).Delete(&UserSkillGroupModel{}).Error; err != nil {
			r.logger().Error("清除技能组用户绑定关系失败", "ids", ids, "error", err.Error())
			return err
		}
		result := tx.Model(&SkillGroupModel{}).Where("id IN ?", ids).
			Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
		if result.Error != nil {
			r.logger().Error("逻辑删除技能组基本信息失败", "ids", ids, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			return operate.ErrSkillGroupNotFound
		}
		return nil
	})
	if err != nil {
		r.logger().Warn("逻辑删除技能组失败", "ids", ids, "error", err.Error())
		return err
	}
	r.logger().Info("逻辑删除技能组成功", "ids", ids)
	return nil
}

// ReplaceUsers 重新分配绑定至技能组的坐席成员。
func (r *SkillGroupRepository) ReplaceUsers(ctx context.Context, skillGroupID int, userIDs []int) error {
	r.logger().Info("开始重新分配绑定技能组坐席成员", "skillGroupId", skillGroupID, "userIds", userIDs)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("skill_group_id = ?", skillGroupID).Delete(&UserSkillGroupModel{}).Error; err != nil {
			r.logger().Error("删除技能组历史用户绑定失败", "skillGroupId", skillGroupID, "error", err.Error())
			return err
		}
		if len(userIDs) == 0 {
			return nil
		}
		now := time.Now().UTC()
		refs := make([]UserSkillGroupModel, 0, len(userIDs))
		for _, userID := range userIDs {
			if userID > 0 {
				refs = append(refs, UserSkillGroupModel{UserID: userID, SkillGroupID: skillGroupID, CreatedTime: now, UpdatedTime: now})
			}
		}
		if len(refs) == 0 {
			return nil
		}
		return tx.Create(&refs).Error
	})
	if err != nil {
		r.logger().Warn("重新分配绑定技能组坐席成员失败", "skillGroupId", skillGroupID, "error", err.Error())
		return err
	}
	r.logger().Info("重新分配绑定技能组坐席成员成功", "skillGroupId", skillGroupID, "count", len(userIDs))
	return nil
}

// ReplacePhones 重新分配绑定至技能组的呼叫号码。
func (r *SkillGroupRepository) ReplacePhones(ctx context.Context, skillGroupID int, phoneIDs []int) error {
	r.logger().Info("开始重新分配绑定技能组呼叫号码", "skillGroupId", skillGroupID, "phoneIds", phoneIDs)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("skill_group_id = ?", skillGroupID).Delete(&PoolPhoneSkillGroupModel{}).Error; err != nil {
			r.logger().Error("删除技能组历史号码绑定失败", "skillGroupId", skillGroupID, "error", err.Error())
			return err
		}
		if len(phoneIDs) == 0 {
			return nil
		}
		refs := make([]PoolPhoneSkillGroupModel, 0, len(phoneIDs))
		for _, phoneID := range phoneIDs {
			if phoneID > 0 {
				refs = append(refs, PoolPhoneSkillGroupModel{PoolPhoneID: phoneID, SkillGroupID: skillGroupID})
			}
		}
		if len(refs) == 0 {
			return nil
		}
		return tx.Create(&refs).Error
	})
	if err != nil {
		r.logger().Warn("重新分配绑定技能组呼叫号码失败", "skillGroupId", skillGroupID, "error", err.Error())
		return err
	}
	r.logger().Info("重新分配绑定技能组呼叫号码成功", "skillGroupId", skillGroupID, "count", len(phoneIDs))
	return nil
}

func (r *SkillGroupRepository) UsersBySkillGroup(ctx context.Context, skillGroupID int) ([]int, error) {
	var rows []UserSkillGroupModel
	if err := r.DB.WithContext(ctx).Where("skill_group_id = ?", skillGroupID).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	return ids, nil
}

func (r *SkillGroupRepository) PhonesBySkillGroup(ctx context.Context, skillGroupID int) ([]int, error) {
	var rows []PoolPhoneSkillGroupModel
	if err := r.DB.WithContext(ctx).Where("skill_group_id = ?", skillGroupID).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PoolPhoneID)
	}
	return ids, nil
}

type MemorySkillGroupRepository struct {
	mu          sync.Mutex
	nextID      int
	skillGroups map[int]operate.SkillGroup
	users       map[int]map[int]struct{}
	phones      map[int]map[int]struct{}
}

func NewMemorySkillGroupRepository() *MemorySkillGroupRepository {
	return &MemorySkillGroupRepository{
		nextID:      1,
		skillGroups: map[int]operate.SkillGroup{},
		users:       map[int]map[int]struct{}{},
		phones:      map[int]map[int]struct{}{},
	}
}

func (r *MemorySkillGroupRepository) Page(_ context.Context, req operate.SkillGroupPageRequest) (operate.SkillGroupPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.SkillGroup, 0, len(r.skillGroups))
	for _, skillGroup := range r.skillGroups {
		if req.Name != "" && !strings.Contains(skillGroup.Name, req.Name) {
			continue
		}
		if req.MerchantID > 0 && skillGroup.MerchantID != req.MerchantID {
			continue
		}
		if req.Enable != nil && skillGroup.Enable != *req.Enable {
			continue
		}
		records = append(records, skillGroup)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.SkillGroup{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.SkillGroupPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemorySkillGroupRepository) GetByID(_ context.Context, id int) (operate.SkillGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	skillGroup, ok := r.skillGroups[id]
	if !ok {
		return operate.SkillGroup{}, operate.ErrSkillGroupNotFound
	}
	return skillGroup, nil
}

func (r *MemorySkillGroupRepository) ExistsName(_ context.Context, name string, merchantID int, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, skillGroup := range r.skillGroups {
		if id == excludeID {
			continue
		}
		if skillGroup.Name == name && skillGroup.MerchantID == merchantID {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemorySkillGroupRepository) Save(_ context.Context, skillGroup operate.SkillGroup) (operate.SkillGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if skillGroup.ID == 0 {
		skillGroup.ID = r.nextID
		r.nextID++
	}
	r.skillGroups[skillGroup.ID] = skillGroup
	return skillGroup, nil
}

func (r *MemorySkillGroupRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if _, ok := r.skillGroups[id]; !ok {
			return operate.ErrSkillGroupNotFound
		}
		delete(r.skillGroups, id)
		delete(r.users, id)
		delete(r.phones, id)
	}
	return nil
}

func (r *MemorySkillGroupRepository) ReplaceUsers(_ context.Context, skillGroupID int, userIDs []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.skillGroups[skillGroupID]; !ok {
		return operate.ErrSkillGroupNotFound
	}
	set := map[int]struct{}{}
	for _, id := range userIDs {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	r.users[skillGroupID] = set
	return nil
}

func (r *MemorySkillGroupRepository) ReplacePhones(_ context.Context, skillGroupID int, phoneIDs []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.skillGroups[skillGroupID]; !ok {
		return operate.ErrSkillGroupNotFound
	}
	set := map[int]struct{}{}
	for _, id := range phoneIDs {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	r.phones[skillGroupID] = set
	return nil
}

func (r *MemorySkillGroupRepository) UsersBySkillGroup(_ context.Context, skillGroupID int) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set := r.users[skillGroupID]
	ids := make([]int, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *MemorySkillGroupRepository) PhonesBySkillGroup(_ context.Context, skillGroupID int) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	set := r.phones[skillGroupID]
	ids := make([]int, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids, nil
}

func channelToModel(channel operate.Channel) ChannelModel {
	return ChannelModel{
		ID:        channel.ID,
		Name:      channel.Name,
		Config:    string(channel.Config),
		BlindArea: string(channel.BlindArea),
		Remark:    channel.Remark,
		Enable:    channel.Enable,
		DelFlag:   false,
	}
}

func channelFromModel(model ChannelModel) operate.Channel {
	return operate.Channel{
		ID:        model.ID,
		Name:      model.Name,
		Config:    rawJSON(model.Config),
		BlindArea: rawJSON(model.BlindArea),
		Remark:    model.Remark,
		Enable:    model.Enable,
	}
}

func poolToModel(pool operate.Pool) PoolModel {
	return PoolModel{
		ID:                pool.ID,
		Name:              pool.Name,
		Remark:            pool.Remark,
		Type:              pool.Type,
		GatewayID:         pool.GatewayID,
		Enable:            pool.Enable,
		SelectionStrategy: pool.SelectionStrategy,
		DelFlag:           false,
	}
}

func poolFromModel(model PoolModel) operate.Pool {
	return operate.Pool{
		ID:                model.ID,
		Name:              model.Name,
		Remark:            model.Remark,
		Type:              model.Type,
		GatewayID:         model.GatewayID,
		Enable:            model.Enable,
		SelectionStrategy: model.SelectionStrategy,
	}
}

func phoneToModel(phone operate.PoolPhone) PoolPhoneModel {
	return PoolPhoneModel{
		ID:          phone.ID,
		PoolID:      phone.PoolID,
		Phone:       phone.Phone,
		Province:    phone.Province,
		City:        phone.City,
		Concurrency: phone.Concurrency,
		Remark:      phone.Remark,
		CallLimit:   phone.CallLimit,
		Enable:      phone.Enable,
		DelFlag:     false,
	}
}

func phoneFromModel(model PoolPhoneModel) operate.PoolPhone {
	return operate.PoolPhone{
		ID:          model.ID,
		PoolID:      model.PoolID,
		Phone:       model.Phone,
		Province:    model.Province,
		City:        model.City,
		Concurrency: model.Concurrency,
		Remark:      model.Remark,
		CallLimit:   model.CallLimit,
		Enable:      model.Enable,
	}
}

func skillGroupToModel(skillGroup operate.SkillGroup) SkillGroupModel {
	return SkillGroupModel{
		ID:          skillGroup.ID,
		Name:        skillGroup.Name,
		MerchantID:  skillGroup.MerchantID,
		Description: skillGroup.Description,
		Enable:      skillGroup.Enable,
		DelFlag:     false,
	}
}

func skillGroupFromModel(model SkillGroupModel) operate.SkillGroup {
	return operate.SkillGroup{
		ID:          model.ID,
		Name:        model.Name,
		MerchantID:  model.MerchantID,
		Description: model.Description,
		Enable:      model.Enable,
	}
}

func rawJSON(raw string) []byte {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return []byte(raw)
}

func (r *ChannelRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func (r *PoolRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func (r *PoolPhoneRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func (r *SkillGroupRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
