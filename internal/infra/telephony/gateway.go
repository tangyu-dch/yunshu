package telephony

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

// GatewayModel 映射  侧 `gateway` 表。
//
// 该表是运营端网关配置真相，CTI 选号、ESL 起呼网关和号码池关系都依赖它。
// 这里只做低 QPS 管理面读写；呼叫热路径不能直接同步查询该表。
type GatewayModel struct {
	ID                    int       `gorm:"column:id;primaryKey"`
	Name                  string    `gorm:"column:name"`
	Description           string    `gorm:"column:description"`
	ChannelID             int       `gorm:"column:channel_id"`
	Concurrency           int       `gorm:"column:concurrency"`
	Model                 int       `gorm:"column:model"`
	Username              string    `gorm:"column:username"`
	Password              string    `gorm:"column:password"`
	Realm                 string    `gorm:"column:realm"`
	Port                  string    `gorm:"column:port"`
	Priority              int       `gorm:"column:priority"`
	Remark                string    `gorm:"column:remark"`
	BroadcastTime         int       `gorm:"column:broadcast_time"`
	BroadcastTimeFlag     bool      `gorm:"column:broadcast_time_flag"`
	CallerPrefix          string    `gorm:"column:caller_prefix"`
	CallerPrefixFlag      bool      `gorm:"column:caller_prefix_flag"`
	CodecPrefs            string    `gorm:"column:codec_prefs"`
	CalleePrefix          string    `gorm:"column:callee_prefix"`
	CalleePrefixFlag      bool      `gorm:"column:callee_prefix_flag"`
	CallerRewriteRule     string    `gorm:"column:caller_rewrite_rule"`
	CalleeRewriteRule     string    `gorm:"column:callee_rewrite_rule"`
	SupplementRing        bool      `gorm:"column:supplement_ring"`
	SupplementRingFile    string    `gorm:"column:supplement_ring_file"`
	CalleeNumberLimit     bool      `gorm:"column:callee_number_limit"`
	CalleeNumberLimitType string    `gorm:"column:callee_number_limit_type"`
	RateID                int       `gorm:"column:rate_id"`
	Enable                bool      `gorm:"column:enable"`
	DelFlag               bool      `gorm:"column:del_flag"`
	CreatedTime           time.Time `gorm:"column:created_time"`
	UpdatedTime           time.Time `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的网关表名。
func (GatewayModel) TableName() string {
	return "cc_tel_gateway"
}

// ChannelModel 映射  侧 `channel` 表。
//
// 该表承载渠道频次和盲区等运营配置，供号码资源管理和选号策略消费。
type ChannelModel struct {
	ID          int       `gorm:"column:id;primaryKey"`
	Name        string    `gorm:"column:name"`
	Config      string    `gorm:"column:config"`
	BlindArea   string    `gorm:"column:blind_area"`
	Remark      string    `gorm:"column:remark"`
	Enable      bool      `gorm:"column:enable"`
	DelFlag     bool      `gorm:"column:del_flag"`
	CreatedTime time.Time `gorm:"column:created_time"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
}

func (ChannelModel) TableName() string {
	return "cc_tel_channel"
}

// PoolModel 映射  `pool` 表中网关绑定所需字段。
type PoolModel struct {
	ID                int       `gorm:"column:id;primaryKey"`
	MerchantID        int       `gorm:"column:merchant_id"`
	Name              string    `gorm:"column:name"`
	Remark            string    `gorm:"column:remark"`
	Type              int       `gorm:"column:type"`
	GatewayID         int       `gorm:"column:gateway_id"`
	Enable            bool      `gorm:"column:enable"`
	SelectionStrategy string    `gorm:"column:selection_strategy;type:varchar(64);default:'CONCURRENCY'"`
	DelFlag           bool      `gorm:"column:del_flag"`
	CreatedTime       time.Time `gorm:"column:created_time"`
	UpdatedTime       time.Time `gorm:"column:updated_time"`
}

// TableName 返回  生产库中的号码池表名。
func (PoolModel) TableName() string {
	return "cc_tel_pool"
}

// GatewayRepository 基于 GORM 实现运营端网关管理仓储。
//
// 该仓储负责 gateway 表的 CRUD 和号码池绑定关系管理，
// 由 cc-console 管理端和 cc-call 网关同步入口共同使用。
type GatewayRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

// NewGatewayRepository 创建  兼容网关仓储。
// logger 用于在关键写操作路径打印结构化日志；传 nil 时回退到 slog.Default()。
func NewGatewayRepository(db *gorm.DB, logger *slog.Logger) *GatewayRepository {
	return &GatewayRepository{DB: db, Logger: logger}
}

// logger 返回注入的 Logger 实例，未注入时回退到全局默认 logger。
func (r *GatewayRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

// Page 分页读取未删除网关，支持名称、启用状态和渠道过滤。
func (r *GatewayRepository) Page(ctx context.Context, req operate.GatewayPageRequest) (operate.GatewayPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&GatewayModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("(name LIKE ? OR description LIKE ?)", "%"+req.Name+"%", "%"+req.Name+"%")
	}
	if req.Enable != nil {
		query = query.Where("enable = ?", *req.Enable)
	}
	if req.ChannelID > 0 {
		query = query.Where("channel_id = ?", req.ChannelID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.GatewayPageResult{}, err
	}
	var models []GatewayModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("priority ASC, id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.GatewayPageResult{}, err
	}
	records := make([]operate.Gateway, 0, len(models))
	for _, model := range models {
		records = append(records, gatewayFromModel(model))
	}
	return operate.GatewayPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 读取单个未删除网关。
func (r *GatewayRepository) GetByID(ctx context.Context, id int) (operate.Gateway, error) {
	var model GatewayModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Gateway{}, operate.ErrGatewayNotFound
	}
	return gatewayFromModel(model), err
}

// GetGatewayNameByID 按 ID 读取网关名称，供 ESL 网关配置同步入口使用。
func (r *GatewayRepository) GetGatewayNameByID(ctx context.Context, id int) (string, error) {
	gateway, err := r.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return gateway.Name, nil
}

// ExistsNameOrDescription 按  逻辑校验 name/description 唯一。
func (r *GatewayRepository) ExistsNameOrDescription(ctx context.Context, name, description string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&GatewayModel{}).
		Where("del_flag = ?", false).
		Where("(name = ? OR description = ?)", name, description)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新网关配置。
// ID == 0 时为新增，否则为更新；GORM Save 会根据主键自动判断 INSERT/UPDATE。
func (r *GatewayRepository) Save(ctx context.Context, gateway operate.Gateway) (operate.Gateway, error) {
	logger := r.logger()
	isCreate := gateway.ID == 0
	model := gatewayToModel(gateway)
	now := time.Now().UTC()
	model.UpdatedTime = now
	if isCreate {
		model.CreatedTime = now
		logger.Info("新增网关配置", "name", gateway.Name, "channelId", gateway.ChannelID, "concurrency", gateway.Concurrency)
	} else {
		logger.Info("更新网关配置", "gatewayId", gateway.ID, "name", gateway.Name, "channelId", gateway.ChannelID)
	}
	tx := r.DB.WithContext(ctx)
	if model.ID != 0 {
		tx = tx.Omit("created_time")
	}
	if err := tx.Save(&model).Error; err != nil {
		logger.Error("保存网关配置失败", "gatewayId", gateway.ID, "name", gateway.Name, "error", err.Error())
		return operate.Gateway{}, err
	}
	if isCreate {
		logger.Info("网关配置新增成功", "gatewayId", model.ID, "name", model.Name)
	}
	return gatewayFromModel(model), nil
}

// Delete 按  语义逻辑删除网关（设置 del_flag=true）。
// 逻辑删除后不会物理清除记录，但后续查询会自动过滤已删除网关。
func (r *GatewayRepository) Delete(ctx context.Context, ids []int) error {
	logger := r.logger()
	logger.Info("逻辑删除网关", "gatewayIds", ids)
	result := r.DB.WithContext(ctx).Model(&GatewayModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		logger.Error("逻辑删除网关失败", "gatewayIds", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		logger.Warn("逻辑删除网关未匹配到记录", "gatewayIds", ids)
		return operate.ErrGatewayNotFound
	}
	logger.Info("逻辑删除网关成功", "gatewayIds", ids, "rowsAffected", result.RowsAffected)
	return nil
}

// BindPools 先解绑旧号码池，再绑定新的号码池到网关。
// 在事务内执行：先清除该网关的所有已有绑定，再将指定号码池绑定到该网关。
// poolIDs 为空时等价于全部解绑。
func (r *GatewayRepository) BindPools(ctx context.Context, gatewayID int, poolIDs []int) error {
	logger := r.logger()
	logger.Info("绑定号码池到网关", "gatewayId", gatewayID, "poolIds", poolIDs)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PoolModel{}).Where("gateway_id = ?", gatewayID).Update("gateway_id", 0).Error; err != nil {
			return err
		}
		if len(poolIDs) == 0 {
			return nil
		}
		return tx.Model(&PoolModel{}).Where("id IN ?", poolIDs).Update("gateway_id", gatewayID).Error
	})
	if err != nil {
		logger.Error("绑定号码池到网关失败", "gatewayId", gatewayID, "poolIds", poolIDs, "error", err.Error())
		return err
	}
	logger.Info("绑定号码池到网关成功", "gatewayId", gatewayID, "poolCount", len(poolIDs))
	return nil
}

// UnbindPools 解绑指定网关下全部号码池。
// 将 pool 表中 gateway_id 等于指定网关的记录全部清零。
func (r *GatewayRepository) UnbindPools(ctx context.Context, gatewayID int) error {
	logger := r.logger()
	logger.Info("解绑网关全部号码池", "gatewayId", gatewayID)
	if err := r.DB.WithContext(ctx).Model(&PoolModel{}).Where("gateway_id = ?", gatewayID).Update("gateway_id", 0).Error; err != nil {
		logger.Error("解绑网关号码池失败", "gatewayId", gatewayID, "error", err.Error())
		return err
	}
	logger.Info("解绑网关全部号码池成功", "gatewayId", gatewayID)
	return nil
}

// MemoryGatewayRepository 是本地开发和测试用的内存网关仓储。
// 所有数据存储在内存 map 中，进程退出后丢失，不需要日志。
type MemoryGatewayRepository struct {
	mu       sync.Mutex
	nextID   int
	gateways map[int]operate.Gateway
	pools    map[int]int // poolID -> gatewayID 映射
}

// NewMemoryGatewayRepository 创建内存网关仓储，仅用于本地开发和单元测试。
func NewMemoryGatewayRepository() *MemoryGatewayRepository {
	return &MemoryGatewayRepository{nextID: 1, gateways: map[int]operate.Gateway{}, pools: map[int]int{}}
}

// Page 内存分页查询，支持名称和启用状态过滤。
func (r *MemoryGatewayRepository) Page(_ context.Context, req operate.GatewayPageRequest) (operate.GatewayPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Gateway, 0, len(r.gateways))
	for _, gateway := range r.gateways {
		if req.Name != "" && !strings.Contains(gateway.Name, req.Name) && !strings.Contains(gateway.Description, req.Name) {
			continue
		}
		if req.Enable != nil && gateway.Enable != *req.Enable {
			continue
		}
		if req.ChannelID > 0 && gateway.ChannelID != req.ChannelID {
			continue
		}
		records = append(records, gateway)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Gateway{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.GatewayPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 按 ID 查找内存中的网关。
func (r *MemoryGatewayRepository) GetByID(_ context.Context, id int) (operate.Gateway, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	gateway, ok := r.gateways[id]
	if !ok {
		return operate.Gateway{}, operate.ErrGatewayNotFound
	}
	return gateway, nil
}

// GetGatewayNameByID 按 ID 读取网关名称，委托给 GetByID。
func (r *MemoryGatewayRepository) GetGatewayNameByID(ctx context.Context, id int) (string, error) {
	gateway, err := r.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return gateway.Name, nil
}

// ExistsNameOrDescription 检查内存中是否已存在同名或同描述的网关。
func (r *MemoryGatewayRepository) ExistsNameOrDescription(_ context.Context, name, description string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, gateway := range r.gateways {
		if id == excludeID {
			continue
		}
		if gateway.Name == name || gateway.Description == description {
			return true, nil
		}
	}
	return false, nil
}

// Save 保存网关到内存，ID 为 0 时自动分配自增 ID。
func (r *MemoryGatewayRepository) Save(_ context.Context, gateway operate.Gateway) (operate.Gateway, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gateway.ID == 0 {
		gateway.ID = r.nextID
		r.nextID++
	}
	r.gateways[gateway.ID] = gateway
	return gateway, nil
}

// Delete 从内存中物理删除网关（与 GORM 版逻辑删除不同）。
func (r *MemoryGatewayRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if _, ok := r.gateways[id]; !ok {
			return operate.ErrGatewayNotFound
		}
		delete(r.gateways, id)
	}
	return nil
}

// BindPools 在内存中绑定号码池到网关，先清除旧绑定再设置新绑定。
func (r *MemoryGatewayRepository) BindPools(_ context.Context, gatewayID int, poolIDs []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for poolID, currentGatewayID := range r.pools {
		if currentGatewayID == gatewayID {
			r.pools[poolID] = 0
		}
	}
	for _, poolID := range poolIDs {
		r.pools[poolID] = gatewayID
	}
	return nil
}

// UnbindPools 在内存中解绑指定网关的全部号码池。
func (r *MemoryGatewayRepository) UnbindPools(_ context.Context, gatewayID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for poolID, currentGatewayID := range r.pools {
		if currentGatewayID == gatewayID {
			r.pools[poolID] = 0
		}
	}
	return nil
}

// gatewayToModel 将领域对象转为 GORM 模型，DelFlag 固定设为 false（新增/更新时始终为未删除状态）。
func gatewayToModel(gateway operate.Gateway) GatewayModel {
	return GatewayModel{
		ID:                    gateway.ID,
		Name:                  gateway.Name,
		Description:           gateway.Description,
		ChannelID:             gateway.ChannelID,
		Concurrency:           gateway.Concurrency,
		Model:                 gateway.Model,
		Username:              gateway.Username,
		Password:              gateway.Password,
		Realm:                 gateway.Realm,
		Port:                  gateway.Port,
		Priority:              gateway.Priority,
		Remark:                gateway.Remark,
		BroadcastTime:         gateway.BroadcastTime,
		BroadcastTimeFlag:     gateway.BroadcastTimeFlag,
		CallerPrefix:          gateway.CallerPrefix,
		CallerPrefixFlag:      gateway.CallerPrefixFlag,
		CodecPrefs:            gateway.CodecPrefs,
		CalleePrefix:          gateway.CalleePrefix,
		CalleePrefixFlag:      gateway.CalleePrefixFlag,
		CallerRewriteRule:     gateway.CallerRewriteRule,
		CalleeRewriteRule:     gateway.CalleeRewriteRule,
		SupplementRing:        gateway.SupplementRing,
		SupplementRingFile:    gateway.SupplementRingFile,
		CalleeNumberLimit:     gateway.CalleeNumberLimit,
		CalleeNumberLimitType: gateway.CalleeNumberLimitType,
		RateID:                gateway.RateID,
		Enable:                gateway.Enable,
		DelFlag:               false,
	}
}

// gatewayFromModel 将 GORM 模型转为领域对象，同时将逗号分隔的 CodecPrefs 拆分为 GatewayCode 切片。
func gatewayFromModel(model GatewayModel) operate.Gateway {
	gatewayCode := []string{}
	if model.CodecPrefs != "" {
		gatewayCode = strings.Split(model.CodecPrefs, ",")
	}
	return operate.Gateway{
		ID:                    model.ID,
		Name:                  model.Name,
		Description:           model.Description,
		ChannelID:             model.ChannelID,
		Concurrency:           model.Concurrency,
		Model:                 model.Model,
		Username:              model.Username,
		Password:              model.Password,
		Realm:                 model.Realm,
		Port:                  model.Port,
		Priority:              model.Priority,
		Remark:                model.Remark,
		BroadcastTime:         model.BroadcastTime,
		BroadcastTimeFlag:     model.BroadcastTimeFlag,
		CallerPrefix:          model.CallerPrefix,
		CallerPrefixFlag:      model.CallerPrefixFlag,
		CalleePrefix:          model.CalleePrefix,
		CalleePrefixFlag:      model.CalleePrefixFlag,
		CallerRewriteRule:     model.CallerRewriteRule,
		CalleeRewriteRule:     model.CalleeRewriteRule,
		SupplementRing:        model.SupplementRing,
		SupplementRingFile:    model.SupplementRingFile,
		CalleeNumberLimit:     model.CalleeNumberLimit,
		CalleeNumberLimitType: model.CalleeNumberLimitType,
		CodecPrefs:            model.CodecPrefs,
		GatewayCode:           gatewayCode,
		RateID:                model.RateID,
		Enable:                model.Enable,
	}
}
