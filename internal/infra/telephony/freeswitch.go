package telephony

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FreeswitchModel 映射  侧 `freeswitch` 表。
//
// 这里沿用  BaseDO/BaseEnable 字段：`id`、`enable`、`del_flag`、
// `created_time`、`updated_time`，避免 Go 重写后生成第二套节点配置表。
type FreeswitchModel struct {
	ID           int       `gorm:"column:id;primaryKey"`
	Address      string    `gorm:"column:address"`
	LocalAddress string    `gorm:"column:local_address"`
	ESLPort      int       `gorm:"column:esl_port"`
	SIPPort      int       `gorm:"column:sip_port"`
	Password     string    `gorm:"column:password"`
	SetID        int       `gorm:"column:setid"`
	Weight       int       `gorm:"column:weight"`
	RWeight      int       `gorm:"column:rweight"`
	CC           int       `gorm:"column:cc"`
	CmdPort      int       `gorm:"column:cmd_port"`
	Canary       bool      `gorm:"column:canary"`
	Enable       bool      `gorm:"column:enable"`
	DelFlag      bool      `gorm:"column:del_flag"`
	CreatedTime  time.Time `gorm:"column:created_time"`
	UpdatedTime  time.Time `gorm:"column:updated_time"`
}

// FreeswitchEventLeaseModel 记录 FS 事件消费租约。
type FreeswitchEventLeaseModel struct {
	FSAddr      string    `gorm:"column:fs_addr;primaryKey;size:128"`
	Owner       string    `gorm:"column:owner;size:128;index:idx_freeswitch_event_lease_owner"`
	LeaseExpiry time.Time `gorm:"column:lease_expiry;index:idx_freeswitch_event_lease_expiry"`
	UpdatedTime time.Time `gorm:"column:updated_time"`
	CreatedTime time.Time `gorm:"column:created_time"`
}

// TableName 返回 FS 事件租约表名。
func (FreeswitchEventLeaseModel) TableName() string {
	return "cc_tel_fs_lease"
}

// TableName 返回  生产库中的 FreeSWITCH 节点表名。
func (FreeswitchModel) TableName() string {
	return "cc_tel_freeswitch"
}

// GormRegistry 从 MySQL 的 `freeswitch` 表读取和维护 FS 节点配置。
//
// 生产环境应优先使用这个实现，配置文件里的节点只作为本地开发兜底。
type GormRegistry struct {
	DB *gorm.DB
}

// NewGormRegistry 创建基于 GORM 的 FreeSWITCH 节点仓储。
// db 参数为已初始化的 GORM 数据库连接。
func NewGormRegistry(db *gorm.DB) *GormRegistry {
	return &GormRegistry{DB: db}
}

// Upsert 保存或更新节点配置。日志和运行时刷新由上层管理服务负责。
func (r *GormRegistry) Upsert(ctx context.Context, node Node) error {
	model := modelFromNode(node)
	if model.UpdatedTime.IsZero() {
		model.UpdatedTime = time.Now().UTC()
	}
	if model.CreatedTime.IsZero() && model.ID == 0 {
		model.CreatedTime = model.UpdatedTime
	}
	return r.DB.WithContext(ctx).Save(&model).Error
}

// Get 按 ESL 地址读取一个未删除节点。
func (r *GormRegistry) Get(ctx context.Context, fsAddr string) (Node, error) {
	address, port, ok := splitFSAddr(fsAddr)
	if !ok {
		return Node{}, ErrNodeNotFound
	}
	var model FreeswitchModel
	err := r.DB.WithContext(ctx).
		Where("address = ? AND esl_port = ? AND del_flag = ?", address, port, false).
		First(&model).Error
	return nodeFromModel(model), translateGormErr(err)
}

// GetByID 按  freeswitch.id 读取节点。
func (r *GormRegistry) GetByID(ctx context.Context, id int) (Node, error) {
	var model FreeswitchModel
	err := r.DB.WithContext(ctx).
		Where("id = ? AND del_flag = ?", id, false).
		First(&model).Error
	return nodeFromModel(model), translateGormErr(err)
}

// List 返回所有未删除节点，包含未启用节点，供管理端页面展示。
func (r *GormRegistry) List(ctx context.Context) ([]Node, error) {
	return r.list(ctx, false)
}

// ListEnabled 返回启用节点，供 cc-call 启动和刷新连接池使用。
func (r *GormRegistry) ListEnabled(ctx context.Context) ([]Node, error) {
	return r.list(ctx, true)
}

// Delete 对齐  逻辑删除语义，把 del_flag 标记为 true。
func (r *GormRegistry) Delete(ctx context.Context, id int) error {
	var model FreeswitchModel
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err != nil {
		return translateGormErr(err)
	}
	model.DelFlag = true
	model.UpdatedTime = time.Now().UTC()
	return r.DB.WithContext(ctx).Save(&model).Error
}

// ClaimEvents 使用数据库表领取 FS 事件消费租约。
func (r *GormRegistry) ClaimEvents(ctx context.Context, fsAddr, owner string, ttl time.Duration) (Node, error) {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	now := time.Now().UTC()
	leaseExpiry := now.Add(ttl).UTC()
	address, port, ok := splitFSAddr(fsAddr)
	if !ok {
		return Node{}, ErrNodeNotFound
	}
	var claimed Node
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var nodeModel FreeswitchModel
		if err := tx.Where("address = ? AND esl_port = ? AND del_flag = ?", address, port, false).First(&nodeModel).Error; err != nil {
			return err
		}
		var leaseModel FreeswitchEventLeaseModel
		leaseErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("fs_addr = ?", fsAddr).First(&leaseModel).Error
		if leaseErr != nil && !errors.Is(leaseErr, gorm.ErrRecordNotFound) {
			return leaseErr
		}
		if leaseErr == nil && leaseModel.Owner != "" && leaseModel.Owner != owner && leaseModel.LeaseExpiry.After(now) {
			return ErrLeaseHeld
		}
		model := FreeswitchEventLeaseModel{
			FSAddr:      fsAddr,
			Owner:       owner,
			LeaseExpiry: leaseExpiry,
			UpdatedTime: now,
			CreatedTime: now,
		}
		if leaseErr == nil {
			model.CreatedTime = leaseModel.CreatedTime
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fs_addr"}},
			DoUpdates: clause.AssignmentColumns([]string{"owner", "lease_expiry", "updated_time"}),
		}).Create(&model).Error; err != nil {
			return err
		}
		claimed = nodeFromModel(nodeModel)
		claimed.EventOwner = owner
		claimed.LeaseExpires = leaseExpiry
		return nil
	})
	if err != nil {
		return Node{}, translateGormErr(err)
	}
	return claimed, nil
}

// ReleaseEvents 释放 FS 事件消费租约。
func (r *GormRegistry) ReleaseEvents(ctx context.Context, fsAddr, owner string) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var leaseModel FreeswitchEventLeaseModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("fs_addr = ?", fsAddr).First(&leaseModel).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if owner != "" && leaseModel.Owner != owner {
			return nil
		}
		return tx.Where("fs_addr = ?", fsAddr).Delete(&FreeswitchEventLeaseModel{}).Error
	})
}

func (r *GormRegistry) list(ctx context.Context, enabledOnly bool) ([]Node, error) {
	var models []FreeswitchModel
	query := r.DB.WithContext(ctx).Where("del_flag = ?", false)
	if enabledOnly {
		query = query.Where("enable = ?", true)
	}
	if err := query.Order("setid ASC, weight DESC, id ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	nodes := make([]Node, 0, len(models))
	for _, model := range models {
		nodes = append(nodes, nodeFromModel(model))
	}
	return nodes, nil
}

func modelFromNode(node Node) FreeswitchModel {
	return FreeswitchModel{
		ID:           node.ID,
		Address:      node.Address,
		LocalAddress: node.LocalAddress,
		ESLPort:      node.ESLPort,
		SIPPort:      node.SIPPort,
		Password:     node.Password,
		SetID:        node.SetID,
		Weight:       node.Weight,
		RWeight:      node.RWeight,
		CC:           node.CC,
		CmdPort:      node.CmdPort,
		Canary:       node.Canary,
		Enable:       node.Enable,
		DelFlag:      false,
		UpdatedTime:  node.UpdatedAt,
	}
}

func nodeFromModel(model FreeswitchModel) Node {
	fsAddr := model.Address
	if model.ESLPort > 0 {
		fsAddr += ":" + strconv.Itoa(model.ESLPort)
	}
	commandURL := ""
	if model.Address != "" && model.CmdPort > 0 {
		commandURL = model.Address + ":" + strconv.Itoa(model.CmdPort)
	}
	status := NodeUnavailable
	if model.Enable && !model.DelFlag {
		status = NodeActive
	}
	return Node{
		ID:           model.ID,
		FSAddr:       fsAddr,
		Name:         fsAddr,
		Address:      model.Address,
		LocalAddress: model.LocalAddress,
		ESLPort:      model.ESLPort,
		SIPPort:      model.SIPPort,
		CmdPort:      model.CmdPort,
		Password:     model.Password,
		SetID:        model.SetID,
		Weight:       model.Weight,
		RWeight:      model.RWeight,
		CC:           model.CC,
		Canary:       model.Canary,
		Enable:       model.Enable,
		Status:       status,
		CommandURL:   commandURL,
		UpdatedAt:    model.UpdatedTime,
	}
}

func splitFSAddr(fsAddr string) (string, int, bool) {
	for i := len(fsAddr) - 1; i >= 0; i-- {
		if fsAddr[i] != ':' {
			continue
		}
		port, err := strconv.Atoi(fsAddr[i+1:])
		if err != nil {
			return "", 0, false
		}
		return fsAddr[:i], port, true
	}
	return "", 0, false
}

func translateGormErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNodeNotFound
	}
	return err
}

// AfterSave 在 FreeSWITCH 节点创建/更新/逻辑删除时，自动级联同步至 kamailio_dispatcher 路由表
func (m *FreeswitchModel) AfterSave(tx *gorm.DB) (err error) {
	if !tx.Migrator().HasTable("kamailio_dispatcher") {
		return nil
	}

	destination := fmt.Sprintf("sip:%s:%d", m.Address, m.SIPPort)
	attrs := fmt.Sprintf("max-concurrency=%d", m.CC)
	description := fmt.Sprintf("FS-Node:%d", m.ID)

	// Determine flags: 0 if enabled & not deleted, 1 (inactive) if disabled/deleted
	flags := 0
	if !m.Enable || m.DelFlag {
		flags = 1
	}

	if m.DelFlag {
		// 同步逻辑删除状态，同时将 flags 标记为 1 (inactive)
		err = tx.Table("kamailio_dispatcher").
			Where("description = ?", description).
			Updates(map[string]any{
				"flags":        flags,
				"enable":       false,
				"del_flag":     true,
				"updated_time": time.Now().UTC(),
			}).Error
		return err
	}

	// 检查是否已存在对应 description (FS-Node:ID) 的记录
	var count int64
	err = tx.Table("kamailio_dispatcher").Where("description = ?", description).Count(&count).Error
	if err != nil {
		return err
	}

	if count == 0 {
		// 新增路由记录
		err = tx.Table("kamailio_dispatcher").Create(map[string]any{
			"set_id":       m.SetID,
			"destination":  destination,
			"flags":        flags,
			"priority":     m.Weight,
			"attrs":        attrs,
			"description":  description,
			"enable":       m.Enable,
			"del_flag":     false,
			"created_time": time.Now().UTC(),
			"updated_time": time.Now().UTC(),
		}).Error
		return err
	}

	// 更新现有路由记录
	err = tx.Table("kamailio_dispatcher").
		Where("description = ?", description).
		Updates(map[string]any{
			"set_id":       m.SetID,
			"destination":  destination,
			"flags":        flags,
			"priority":     m.Weight,
			"attrs":        attrs,
			"enable":       m.Enable,
			"del_flag":     false,
			"updated_time": time.Now().UTC(),
		}).Error
	return err
}
