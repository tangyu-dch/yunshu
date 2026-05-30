package security

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"gorm.io/gorm"

	"yunshu/internal/domain/operate"
)

// RiskControlModel 映射  `risk_control` 表。
type RiskControlModel struct {
	ID                  int    `gorm:"column:id;primaryKey;autoIncrement"`
	Name                string `gorm:"column:name"`
	Remark              string `gorm:"column:remark"`
	BlackLevelFlag      bool   `gorm:"column:black_level_flag"`
	BlackLevel          string `gorm:"column:black_level"`
	BlindAreaFlag       bool   `gorm:"column:blind_area_flag"`
	BlindArea           string `gorm:"column:blind_area"`
	CalleeFrequencyFlag bool   `gorm:"column:callee_frequency_flag"`
	CalleeFrequency     string `gorm:"column:callee_frequency"`
	DelFlag             bool   `gorm:"column:del_flag"`
}

func (RiskControlModel) TableName() string {
	return "cc_sec_risk_control"
}

// RiskControlMerchantModel 映射 `risk_control_merchant` 关联表。
type RiskControlMerchantModel struct {
	RiskID     int  `gorm:"column:risk_id;primaryKey"`
	MerchantID int  `gorm:"column:merchant_id;primaryKey"`
	Enable     bool `gorm:"column:enable"`
}

func (RiskControlMerchantModel) TableName() string {
	return "cc_sec_risk_control_merchant"
}

// RiskControlRepository 基于 GORM 实现运营端风控管理仓储。
type RiskControlRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

func NewRiskControlRepository(db *gorm.DB, logger *slog.Logger) *RiskControlRepository {
	return &RiskControlRepository{DB: db, Logger: logger}
}

func (r *RiskControlRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func (r *RiskControlRepository) Page(ctx context.Context, req operate.RiskControlPageRequest) (operate.RiskControlPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&RiskControlModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.RiskControlPageResult{}, err
	}
	var models []RiskControlModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.RiskControlPageResult{}, err
	}
	records := make([]operate.RiskControl, 0, len(models))
	for _, model := range models {
		records = append(records, riskControlFromModel(model))
	}
	return operate.RiskControlPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *RiskControlRepository) GetByID(ctx context.Context, id int) (operate.RiskControl, error) {
	var model RiskControlModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.RiskControl{}, operate.ErrRiskControlNotFound
	}
	return riskControlFromModel(model), err
}

func (r *RiskControlRepository) ExistsName(ctx context.Context, name string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&RiskControlModel{}).Where("name = ? AND del_flag = ?", name, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *RiskControlRepository) Save(ctx context.Context, rc operate.RiskControl) (operate.RiskControl, error) {
	r.logger().Info("开始保存风控配置", "id", rc.ID, "name", rc.Name)
	model := riskControlToModel(rc)
	if err := r.DB.WithContext(ctx).Save(&model).Error; err != nil {
		r.logger().Error("保存风控配置失败", "id", rc.ID, "name", rc.Name, "error", err.Error())
		return operate.RiskControl{}, err
	}
	r.logger().Info("保存风控配置成功", "id", model.ID, "name", model.Name)
	return riskControlFromModel(model), nil
}

func (r *RiskControlRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除风控配置", "ids", ids)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除关联商户配置
		if err := tx.Where("risk_id IN ?", ids).Delete(&RiskControlMerchantModel{}).Error; err != nil {
			r.logger().Error("清除风控配置关联商户失败", "ids", ids, "error", err.Error())
			return err
		}
		result := tx.Model(&RiskControlModel{}).Where("id IN ?", ids).Updates(map[string]any{"del_flag": true})
		if result.Error != nil {
			r.logger().Error("逻辑删除风控基本信息失败", "ids", ids, "error", result.Error.Error())
			return result.Error
		}
		if result.RowsAffected == 0 {
			return operate.ErrRiskControlNotFound
		}
		return nil
	})
	if err != nil {
		r.logger().Warn("逻辑删除风控配置未成功", "ids", ids, "error", err.Error())
		return err
	}
	r.logger().Info("逻辑删除风控配置成功", "ids", ids)
	return nil
}

func (r *RiskControlRepository) GetMerchants(ctx context.Context, riskID int) ([]operate.RiskControlMerchant, error) {
	var models []RiskControlMerchantModel
	err := r.DB.WithContext(ctx).Where("risk_id = ?", riskID).Find(&models).Error
	if err != nil {
		return nil, err
	}
	bindings := make([]operate.RiskControlMerchant, 0, len(models))
	for _, m := range models {
		bindings = append(bindings, operate.RiskControlMerchant{
			RiskID:     m.RiskID,
			MerchantID: m.MerchantID,
			Enable:     m.Enable,
		})
	}
	return bindings, nil
}

func (r *RiskControlRepository) SaveMerchants(ctx context.Context, riskID int, bindings []operate.RiskControlMerchant) error {
	r.logger().Info("开始保存风控绑定商户关系", "riskId", riskID, "count", len(bindings))
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("risk_id = ?", riskID).Delete(&RiskControlMerchantModel{}).Error; err != nil {
			return err
		}
		if len(bindings) > 0 {
			models := make([]RiskControlMerchantModel, len(bindings))
			for i, b := range bindings {
				models[i] = RiskControlMerchantModel{
					RiskID:     b.RiskID,
					MerchantID: b.MerchantID,
					Enable:     b.Enable,
				}
			}
			if err := tx.Create(&models).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		r.logger().Error("保存风控绑定商户关系失败", "riskId", riskID, "error", err.Error())
		return err
	}
	r.logger().Info("保存风控绑定商户关系成功", "riskId", riskID)
	return nil
}

func riskControlToModel(rc operate.RiskControl) RiskControlModel {
	return RiskControlModel{
		ID:                  rc.ID,
		Name:                rc.Name,
		Remark:              rc.Remark,
		BlackLevelFlag:      rc.BlackLevelFlag,
		BlackLevel:          rc.BlackLevel,
		BlindAreaFlag:       rc.BlindAreaFlag,
		BlindArea:           rc.BlindArea,
		CalleeFrequencyFlag: rc.CalleeFrequencyFlag,
		CalleeFrequency:     rc.CalleeFrequency,
		DelFlag:             false,
	}
}

func riskControlFromModel(model RiskControlModel) operate.RiskControl {
	return operate.RiskControl{
		ID:                  model.ID,
		Name:                model.Name,
		Remark:              model.Remark,
		BlackLevelFlag:      model.BlackLevelFlag,
		BlackLevel:          model.BlackLevel,
		BlindAreaFlag:       model.BlindAreaFlag,
		BlindArea:           model.BlindArea,
		CalleeFrequencyFlag: model.CalleeFrequencyFlag,
		CalleeFrequency:     model.CalleeFrequency,
	}
}

// MemoryRiskControlRepository 供本地开发和测试使用。
type MemoryRiskControlRepository struct {
	mu           sync.Mutex
	nextID       int
	riskControls map[int]operate.RiskControl
	merchants    map[int][]operate.RiskControlMerchant
}

func NewMemoryRiskControlRepository() *MemoryRiskControlRepository {
	return &MemoryRiskControlRepository{
		nextID:       1,
		riskControls: map[int]operate.RiskControl{},
		merchants:    map[int][]operate.RiskControlMerchant{},
	}
}

func (r *MemoryRiskControlRepository) Page(_ context.Context, req operate.RiskControlPageRequest) (operate.RiskControlPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.RiskControl, 0, len(r.riskControls))
	for _, rc := range r.riskControls {
		if req.Name != "" && rc.Name != req.Name {
			continue
		}
		records = append(records, rc)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.RiskControl{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.RiskControlPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryRiskControlRepository) GetByID(_ context.Context, id int) (operate.RiskControl, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rc, ok := r.riskControls[id]
	if !ok {
		return operate.RiskControl{}, operate.ErrRiskControlNotFound
	}
	return rc, nil
}

func (r *MemoryRiskControlRepository) ExistsName(_ context.Context, name string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, rc := range r.riskControls {
		if id == excludeID {
			continue
		}
		if rc.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryRiskControlRepository) Save(_ context.Context, rc operate.RiskControl) (operate.RiskControl, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rc.ID == 0 {
		rc.ID = r.nextID
		r.nextID++
	}
	r.riskControls[rc.ID] = rc
	return rc, nil
}

func (r *MemoryRiskControlRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if _, ok := r.riskControls[id]; !ok {
			return operate.ErrRiskControlNotFound
		}
		delete(r.riskControls, id)
		delete(r.merchants, id)
	}
	return nil
}

func (r *MemoryRiskControlRepository) GetMerchants(_ context.Context, riskID int) ([]operate.RiskControlMerchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]operate.RiskControlMerchant(nil), r.merchants[riskID]...), nil
}

func (r *MemoryRiskControlRepository) SaveMerchants(_ context.Context, riskID int, bindings []operate.RiskControlMerchant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.merchants[riskID] = append([]operate.RiskControlMerchant(nil), bindings...)
	return nil
}
