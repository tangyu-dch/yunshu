package security

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
	"yunshu/internal/infra/telephony"
)

// BlacklistModel 映射  `blacklist` 表。
type BlacklistModel struct {
	ID                  int       `gorm:"column:id;primaryKey"`
	Name                string    `gorm:"column:name"`
	VerificationChannel int       `gorm:"column:verification_channel"`
	Remark              string    `gorm:"column:remark"`
	Enable              bool      `gorm:"column:enable"`
	DelFlag             bool      `gorm:"column:del_flag"`
	CreatedTime         time.Time `gorm:"column:created_time"`
	UpdatedTime         time.Time `gorm:"column:updated_time"`
}

func (BlacklistModel) TableName() string {
	return "cc_sec_blacklist"
}

// BlacklistGatewayModel 映射  `blacklist_gateway` 表。
type BlacklistGatewayModel struct {
	BlacklistID int `gorm:"column:blacklist_id;primaryKey"`
	GatewayID   int `gorm:"column:gateway_id;primaryKey"`
}

func (BlacklistGatewayModel) TableName() string {
	return "cc_sec_blacklist_gateway"
}

// BlacklistChannelModel 映射 `blacklist_channel` 数据库表。
type BlacklistChannelModel struct {
	Code        int       `gorm:"column:code;primaryKey"`
	Name        string    `gorm:"column:name;type:varchar(255);not null"`
	Vendor      string    `gorm:"column:vendor;type:varchar(64);not null"`
	Remark      string    `gorm:"column:remark;type:varchar(255)"`
	Enable      bool      `gorm:"column:enable;type:boolean;default:true"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

func (BlacklistChannelModel) TableName() string {
	return "cc_sec_blacklist_channel"
}

// BlacklistRepository 基于 GORM 的黑名单管理仓储。
type BlacklistRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewBlacklistRepository 创建黑名单仓储。
func NewBlacklistRepository(db *gorm.DB, logger *slog.Logger) *BlacklistRepository {
	return &BlacklistRepository{DB: db, Logger: logger}
}

// Page 分页查询黑名单。
func (r *BlacklistRepository) Page(ctx context.Context, req operate.BlacklistPageRequest) (operate.BlacklistPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&BlacklistModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if len(req.VerificationChannels) > 0 {
		query = query.Where("verification_channel IN ?", req.VerificationChannels)
	}
	if len(req.Gateways) > 0 {
		sub := r.DB.WithContext(ctx).Model(&BlacklistGatewayModel{}).Select("distinct blacklist_id").Where("gateway_id IN ?", req.Gateways)
		query = query.Where("id IN (?)", sub)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.BlacklistPageResult{}, err
	}
	var models []BlacklistModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.BlacklistPageResult{}, err
	}
	ids := make([]int, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	bindings, err := r.loadGatewayBindings(ctx, ids)
	if err != nil {
		return operate.BlacklistPageResult{}, err
	}
	names, err := r.loadGatewayNames(ctx, flattenGatewayBindings(bindings))
	if err != nil {
		return operate.BlacklistPageResult{}, err
	}
	channelNames := r.loadChannelNamesMap(ctx)
	records := make([]operate.Blacklist, 0, len(models))
	for _, model := range models {
		records = append(records, blacklistFromModel(model, bindings[model.ID], names, channelNames))
	}
	return operate.BlacklistPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 查询单个黑名单。
func (r *BlacklistRepository) GetByID(ctx context.Context, id int) (operate.Blacklist, error) {
	var model BlacklistModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Blacklist{}, operate.ErrBlacklistNotFound
	}
	if err != nil {
		return operate.Blacklist{}, err
	}
	bindings, err := r.loadGatewayBindings(ctx, []int{id})
	if err != nil {
		return operate.Blacklist{}, err
	}
	names, err := r.loadGatewayNames(ctx, bindings[id])
	if err != nil {
		return operate.Blacklist{}, err
	}
	channelNames := r.loadChannelNamesMap(ctx)
	return blacklistFromModel(model, bindings[id], names, channelNames), nil
}

// ExistsName 校验黑名单名称唯一性。
func (r *BlacklistRepository) ExistsName(ctx context.Context, name string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&BlacklistModel{}).
		Where("name = ? AND del_flag = ?", name, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新黑名单及其网关关系。
func (r *BlacklistRepository) Save(ctx context.Context, blacklist operate.Blacklist) (operate.Blacklist, error) {
	r.logger().Info("开始保存黑名单信息", "id", blacklist.ID, "name", blacklist.Name, "verificationChannel", blacklist.VerificationChannel)
	model := blacklistToModel(blacklist)
	now := time.Now().UTC()
	model.UpdatedTime = now
	if model.ID == 0 {
		model.CreatedTime = now
		model.Enable = true
	}
	if err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		saveTx := tx
		if model.ID != 0 {
			saveTx = saveTx.Omit("created_time")
		}
		if err := saveTx.Save(&model).Error; err != nil {
			r.logger().Error("保存黑名单基本信息失败", "name", blacklist.Name, "error", err.Error())
			return err
		}
		if err := tx.Where("blacklist_id = ?", model.ID).Delete(&BlacklistGatewayModel{}).Error; err != nil {
			r.logger().Error("清除黑名单历史网关关联失败", "blacklistId", model.ID, "error", err.Error())
			return err
		}
		for _, gatewayID := range blacklist.GatewayIDs {
			if gatewayID <= 0 {
				continue
			}
			if err := tx.Create(&BlacklistGatewayModel{BlacklistID: model.ID, GatewayID: gatewayID}).Error; err != nil {
				r.logger().Error("创建黑名单网关关联记录失败", "blacklistId", model.ID, "gatewayId", gatewayID, "error", err.Error())
				return err
			}
		}
		return nil
	}); err != nil {
		r.logger().Error("保存黑名单事务执行异常", "name", blacklist.Name, "error", err.Error())
		return operate.Blacklist{}, err
	}
	names, err := r.loadGatewayNames(ctx, blacklist.GatewayIDs)
	if err != nil {
		return operate.Blacklist{}, err
	}
	channelNames := r.loadChannelNamesMap(ctx)
	r.logger().Info("保存黑名单信息成功", "blacklistId", model.ID, "name", model.Name, "gatewayCount", len(blacklist.GatewayIDs))
	return blacklistFromModel(model, blacklist.GatewayIDs, names, channelNames), nil
}

// Delete 逻辑删除黑名单并清除关系。
func (r *BlacklistRepository) Delete(ctx context.Context, id int) error {
	r.logger().Info("开始逻辑删除黑名单", "blacklistId", id)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&BlacklistModel{}).
			Where("id = ?", id).
			Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
		if result.Error != nil {
			r.logger().Error("逻辑删除黑名单基本记录失败", "blacklistId", id, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			r.logger().Warn("逻辑删除黑名单失败：未找到该记录", "blacklistId", id)
			return operate.ErrBlacklistNotFound
		}
		if err := tx.Where("blacklist_id = ?", id).Delete(&BlacklistGatewayModel{}).Error; err != nil {
			r.logger().Error("逻辑删除黑名单网关关联记录失败", "blacklistId", id, "error", err.Error())
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	r.logger().Info("逻辑删除黑名单成功", "blacklistId", id)
	return nil
}

func (r *BlacklistRepository) loadChannelNamesMap(ctx context.Context) map[int]string {
	result := make(map[int]string)
	var list []BlacklistChannelModel
	if err := r.DB.WithContext(ctx).Find(&list).Error; err == nil {
		for _, item := range list {
			result[item.Code] = item.Name
		}
	}
	return result
}

// ListChannels 查询数据库中的所有三方验证通道
func (r *BlacklistRepository) ListChannels(ctx context.Context) ([]operate.BlacklistChannel, error) {
	var list []BlacklistChannelModel
	if err := r.DB.WithContext(ctx).Order("code ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	result := make([]operate.BlacklistChannel, 0, len(list))
	for _, item := range list {
		result = append(result, operate.BlacklistChannel{
			Code:   item.Code,
			Name:   item.Name,
			Vendor: item.Vendor,
			Remark: item.Remark,
			Enable: item.Enable,
		})
	}
	return result, nil
}

// GetChannelByCode 根据通道代码查询验证通道配置
func (r *BlacklistRepository) GetChannelByCode(ctx context.Context, code int) (operate.BlacklistChannel, error) {
	var model BlacklistChannelModel
	if err := r.DB.WithContext(ctx).Where("code = ?", code).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return operate.BlacklistChannel{}, errors.New("风控验证通道不存在")
		}
		return operate.BlacklistChannel{}, err
	}
	return operate.BlacklistChannel{
		Code:   model.Code,
		Name:   model.Name,
		Vendor: model.Vendor,
		Remark: model.Remark,
		Enable: model.Enable,
	}, nil
}

// SaveChannel 保存或更新通道配置（使用事务避免并发竞态）
func (r *BlacklistRepository) SaveChannel(ctx context.Context, channel operate.BlacklistChannel) error {
	now := time.Now().UTC()
	model := BlacklistChannelModel{
		Code:        channel.Code,
		Name:        channel.Name,
		Vendor:      channel.Vendor,
		Remark:      channel.Remark,
		Enable:      channel.Enable,
		UpdatedTime: now,
	}

	// 使用事务 + upsert 避免并发竞态
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing BlacklistChannelModel
		err := tx.Where("code = ?", channel.Code).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			model.CreatedTime = now
			return tx.Create(&model).Error
		}
		if err != nil {
			return err
		}
		model.CreatedTime = existing.CreatedTime
		return tx.Save(&model).Error
	})
}

// DeleteChannel 物理删除通道配置
func (r *BlacklistRepository) DeleteChannel(ctx context.Context, code int) error {
	return r.DB.WithContext(ctx).Where("code = ?", code).Delete(&BlacklistChannelModel{}).Error
}

func (r *BlacklistRepository) loadGatewayBindings(ctx context.Context, blacklistIDs []int) (map[int][]int, error) {
	result := make(map[int][]int, len(blacklistIDs))
	if len(blacklistIDs) == 0 {
		return result, nil
	}
	var refs []BlacklistGatewayModel
	if err := r.DB.WithContext(ctx).Where("blacklist_id IN ?", blacklistIDs).Find(&refs).Error; err != nil {
		return nil, err
	}
	for _, ref := range refs {
		result[ref.BlacklistID] = append(result[ref.BlacklistID], ref.GatewayID)
	}
	for id := range result {
		sort.Ints(result[id])
	}
	return result, nil
}

func (r *BlacklistRepository) loadGatewayNames(ctx context.Context, gatewayIDs []int) (map[int]string, error) {
	result := make(map[int]string, len(gatewayIDs))
	if len(gatewayIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		ID          int    `gorm:"column:id"`
		Name        string `gorm:"column:name"`
		Description string `gorm:"column:description"`
	}
	if err := r.DB.WithContext(ctx).Model(&telephony.GatewayModel{}).
		Select("id,name,description").
		Where("id IN ?", gatewayIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Description != "" {
			result[row.ID] = fmt.Sprintf("%s-%s", row.Name, row.Description)
		} else {
			result[row.ID] = row.Name
		}
	}
	return result, nil
}

// BlacklistDataModel 映射  `blacklist_data` 表。
type BlacklistDataModel struct {
	Phone       string    `gorm:"column:phone;primaryKey"`
	BlackLevel  string    `gorm:"column:black_level;type:varchar(64);default:'LEVEL_1'"`
	Remark      string    `gorm:"column:remark;type:varchar(255)"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

func (BlacklistDataModel) TableName() string {
	return "cc_sec_blacklist_data"
}

// PageNumbers 分页查询具体黑名单号码。
func (r *BlacklistRepository) PageNumbers(ctx context.Context, req operate.BlacklistNumberPageRequest) (operate.BlacklistNumberPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&BlacklistDataModel{})
	if req.Phone != "" {
		query = query.Where("phone LIKE ?", "%"+req.Phone+"%")
	}
	if req.BlackLevel != "" {
		query = query.Where("black_level = ?", req.BlackLevel)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.BlacklistNumberPageResult{}, err
	}
	var models []BlacklistDataModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("created_time DESC, phone ASC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.BlacklistNumberPageResult{}, err
	}
	records := make([]operate.BlacklistNumber, 0, len(models))
	for _, m := range models {
		records = append(records, operate.BlacklistNumber{
			Phone:      m.Phone,
			BlackLevel: m.BlackLevel,
			Remark:     m.Remark,
		})
	}
	return operate.BlacklistNumberPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    records,
	}, nil
}

// SaveNumber 保存（新增或更新）具体黑名单号码（使用事务避免并发竞态）
func (r *BlacklistRepository) SaveNumber(ctx context.Context, num operate.BlacklistNumber) (operate.BlacklistNumber, error) {
	now := time.Now().UTC()
	model := BlacklistDataModel{
		Phone:       num.Phone,
		BlackLevel:  num.BlackLevel,
		Remark:      num.Remark,
		UpdatedTime: now,
	}

	// 使用事务 + upsert 避免并发竞态
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing BlacklistDataModel
		err := tx.Where("phone = ?", num.Phone).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			model.CreatedTime = now
			return tx.Create(&model).Error
		}
		if err != nil {
			return err
		}
		model.CreatedTime = existing.CreatedTime
		return tx.Omit("created_time").Save(&model).Error
	})
	if err != nil {
		return operate.BlacklistNumber{}, err
	}
	return operate.BlacklistNumber{
		Phone:      model.Phone,
		BlackLevel: model.BlackLevel,
		Remark:     model.Remark,
	}, nil
}

// DeleteNumbers 批量删除具体黑名单号码。
func (r *BlacklistRepository) DeleteNumbers(ctx context.Context, phones []string) error {
	if len(phones) == 0 {
		return nil
	}
	return r.DB.WithContext(ctx).Where("phone IN ?", phones).Delete(&BlacklistDataModel{}).Error
}

// MemoryBlacklistRepository 供本地开发和测试使用。
type MemoryBlacklistRepository struct {
	mu               sync.Mutex
	nextID           int
	blacklists       map[int]operate.Blacklist
	blacklistNumbers map[string]operate.BlacklistNumber
	channels         map[int]operate.BlacklistChannel
}

// NewMemoryBlacklistRepository 创建内存黑名单仓储。
func NewMemoryBlacklistRepository() *MemoryBlacklistRepository {
	return &MemoryBlacklistRepository{
		nextID:     1,
		blacklists: map[int]operate.Blacklist{},
		blacklistNumbers: map[string]operate.BlacklistNumber{
			"13888888888": {Phone: "13888888888", BlackLevel: "LEVEL_1", Remark: "测试黑名单手机1"},
			"13999999999": {Phone: "13999999999", BlackLevel: "LEVEL_2", Remark: "测试黑名单手机2"},
		},
		channels: map[int]operate.BlacklistChannel{
			1: {Code: 1, Name: "东信易通黑名单", Vendor: "DONG_XIN", Remark: "系统默认东信易通强风控验证通道", Enable: true},
			2: {Code: 2, Name: "羽乐黑名单", Vendor: "YU_LE", Remark: "系统默认羽乐科技防骚扰拦截通道", Enable: true},
		},
	}
}

func (r *MemoryBlacklistRepository) Page(_ context.Context, req operate.BlacklistPageRequest) (operate.BlacklistPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Blacklist, 0, len(r.blacklists))
	for _, blacklist := range r.blacklists {
		if req.Name != "" && !strings.Contains(blacklist.Name, req.Name) {
			continue
		}
		if len(req.VerificationChannels) > 0 && !containsInt(req.VerificationChannels, blacklist.VerificationChannel) {
			continue
		}
		if len(req.Gateways) > 0 && !containsAnyInt(req.Gateways, blacklist.GatewayIDs) {
			continue
		}
		records = append(records, blacklist)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Blacklist{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.BlacklistPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryBlacklistRepository) GetByID(_ context.Context, id int) (operate.Blacklist, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	blacklist, ok := r.blacklists[id]
	if !ok {
		return operate.Blacklist{}, operate.ErrBlacklistNotFound
	}
	return blacklist, nil
}

func (r *MemoryBlacklistRepository) ExistsName(_ context.Context, name string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, blacklist := range r.blacklists {
		if id == excludeID {
			continue
		}
		if blacklist.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryBlacklistRepository) Save(_ context.Context, blacklist operate.Blacklist) (operate.Blacklist, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if blacklist.ID == 0 {
		blacklist.ID = r.nextID
		r.nextID++
	}
	chName := ""
	if ch, ok := r.channels[blacklist.VerificationChannel]; ok {
		chName = ch.Name
	} else {
		chName = blacklistChannelName(blacklist.VerificationChannel)
	}
	blacklist.VerificationChannelName = chName
	blacklist.Gateways = joinGatewayIDs(blacklist.GatewayIDs)
	r.blacklists[blacklist.ID] = blacklist
	return blacklist, nil
}

func (r *MemoryBlacklistRepository) Delete(_ context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.blacklists[id]; !ok {
		return operate.ErrBlacklistNotFound
	}
	delete(r.blacklists, id)
	return nil
}

// ListChannels 内存版通道查询
func (r *MemoryBlacklistRepository) ListChannels(_ context.Context) ([]operate.BlacklistChannel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]operate.BlacklistChannel, 0, len(r.channels))
	for _, item := range r.channels {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Code < result[j].Code
	})
	return result, nil
}

// SaveChannel 内存版通道保存
func (r *MemoryBlacklistRepository) SaveChannel(_ context.Context, channel operate.BlacklistChannel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[channel.Code] = channel
	return nil
}

// DeleteChannel 内存版通道删除
func (r *MemoryBlacklistRepository) DeleteChannel(_ context.Context, code int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.channels, code)
	return nil
}

func (r *MemoryBlacklistRepository) PageNumbers(_ context.Context, req operate.BlacklistNumberPageRequest) (operate.BlacklistNumberPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	records := make([]operate.BlacklistNumber, 0, len(r.blacklistNumbers))
	for _, num := range r.blacklistNumbers {
		if req.Phone != "" && !strings.Contains(num.Phone, req.Phone) {
			continue
		}
		if req.BlackLevel != "" && num.BlackLevel != req.BlackLevel {
			continue
		}
		records = append(records, num)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Phone < records[j].Phone
	})

	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.BlacklistNumber{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}

	return operate.BlacklistNumberPageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    records,
	}, nil
}

func (r *MemoryBlacklistRepository) SaveNumber(_ context.Context, num operate.BlacklistNumber) (operate.BlacklistNumber, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.blacklistNumbers[num.Phone] = num
	return num, nil
}

func (r *MemoryBlacklistRepository) DeleteNumbers(_ context.Context, phones []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, phone := range phones {
		delete(r.blacklistNumbers, phone)
	}
	return nil
}

func blacklistToModel(blacklist operate.Blacklist) BlacklistModel {
	return BlacklistModel{
		ID:                  blacklist.ID,
		Name:                blacklist.Name,
		VerificationChannel: blacklist.VerificationChannel,
		Remark:              blacklist.Remark,
		Enable:              true,
		DelFlag:             false,
	}
}

func blacklistFromModel(model BlacklistModel, gatewayIDs []int, gatewayNames map[int]string, channelNames map[int]string) operate.Blacklist {
	chName := channelNames[model.VerificationChannel]
	if chName == "" {
		chName = blacklistChannelName(model.VerificationChannel)
	}
	return operate.Blacklist{
		ID:                      model.ID,
		Name:                    model.Name,
		VerificationChannel:     model.VerificationChannel,
		VerificationChannelName: chName,
		GatewayIDs:              gatewayIDs,
		Gateways:                joinGatewayNames(gatewayIDs, gatewayNames),
		Remark:                  model.Remark,
	}
}

func blacklistChannelName(code int) string {
	switch code {
	case operate.BlacklistVerificationChannelDongXin:
		return "东信易通黑名单"
	case operate.BlacklistVerificationChannelYuLe:
		return "羽乐黑名单"
	default:
		return ""
	}
}

func flattenGatewayBindings(bindings map[int][]int) []int {
	set := make(map[int]struct{})
	for _, ids := range bindings {
		for _, id := range ids {
			if id > 0 {
				set[id] = struct{}{}
			}
		}
	}
	result := make([]int, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	sort.Ints(result)
	return result
}

func joinGatewayNames(gatewayIDs []int, names map[int]string) string {
	parts := make([]string, 0, len(gatewayIDs))
	for _, id := range gatewayIDs {
		if text, ok := names[id]; ok && text != "" {
			parts = append(parts, text)
		} else if id > 0 {
			parts = append(parts, strconvItoa(id))
		}
	}
	return strings.Join(parts, ",")
}

func joinGatewayIDs(gatewayIDs []int) string {
	parts := make([]string, 0, len(gatewayIDs))
	for _, id := range gatewayIDs {
		if id > 0 {
			parts = append(parts, strconvItoa(id))
		}
	}
	return strings.Join(parts, ",")
}

func containsInt(items []int, want int) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func containsAnyInt(left []int, right []int) bool {
	for _, item := range left {
		if containsInt(right, item) {
			return true
		}
	}
	return false
}

func strconvItoa(v int) string {
	return fmt.Sprintf("%d", v)
}

func (r *BlacklistRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}
