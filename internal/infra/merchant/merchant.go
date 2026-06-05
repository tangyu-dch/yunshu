package merchant

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/domain/esl"
	"yunshu/internal/domain/operate"
)

// MerchantRepository 基于 GORM 的商户管理仓储实现。
type MerchantRepository struct {
	DB       *gorm.DB
	Statuses esl.ExtensionStatusReader
	Logger   *slog.Logger
}

// NewMerchantRepository 创建商户仓储。
func NewMerchantRepository(db *gorm.DB, statuses esl.ExtensionStatusReader, logger *slog.Logger) *MerchantRepository {
	return &MerchantRepository{DB: db, Statuses: statuses, Logger: logger}
}

// Page 分页查询未删除商户。
func (r *MerchantRepository) Page(ctx context.Context, req operate.MerchantPageRequest) (operate.MerchantPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&MerchantModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Account != "" {
		query = query.Where("account LIKE ?", "%"+req.Account+"%")
	}
	if req.Enable != nil {
		query = query.Where("enable = ?", *req.Enable)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.MerchantPageResult{}, err
	}
	var models []MerchantModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.MerchantPageResult{}, err
	}
	rateBindings, err := r.loadMerchantRateBindings(ctx, extractMerchantIDs(models))
	if err != nil {
		return operate.MerchantPageResult{}, err
	}
	records := make([]operate.Merchant, 0, len(models))
	for _, model := range models {
		records = append(records, merchantFromModel(model, rateBindings[model.ID]))
	}
	return operate.MerchantPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 读取单个未删除商户。
func (r *MerchantRepository) GetByID(ctx context.Context, id int) (operate.Merchant, error) {
	var model MerchantModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Merchant{}, operate.ErrMerchantNotFound
	}
	if err != nil {
		return operate.Merchant{}, err
	}
	rateBindings, err := r.loadMerchantRateBindings(ctx, []int{id})
	if err != nil {
		return operate.Merchant{}, err
	}
	return merchantFromModel(model, rateBindings[id]), nil
}

// ExistsNameOrAccount 校验商户名称或账号唯一性。
func (r *MerchantRepository) ExistsNameOrAccount(ctx context.Context, name, account string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&MerchantModel{}).
		Where("del_flag = ?", false).
		Where("(name = ? OR account = ?)", name, account)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// RateExists 校验费率是否存在。
func (r *MerchantRepository) RateExists(ctx context.Context, rateID int) (bool, error) {
	if rateID <= 0 {
		return true, nil
	}
	var count int64
	if err := r.DB.WithContext(ctx).Model(&CallRateModel{}).
		Where("id = ? AND del_flag = ?", rateID, false).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新商户。
func (r *MerchantRepository) Save(ctx context.Context, merchant operate.Merchant) (operate.Merchant, error) {
	model := merchantToModel(merchant)
	now := time.Now().UTC()
	model.UpdatedTime = now
	if model.ID == 0 {
		model.CreatedTime = now
	} else {
		var existingCreated time.Time
		if err := r.DB.WithContext(ctx).Model(&MerchantModel{}).Where("id = ?", model.ID).Pluck("created_time", &existingCreated).Error; err == nil && !existingCreated.IsZero() {
			model.CreatedTime = existingCreated
		} else {
			model.CreatedTime = now
		}
	}
	r.logger().Info("开始保存商户信息", "merchantId", merchant.ID, "merchantName", merchant.Name, "maxAgents", merchant.MaxAgents)
	if err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&model).Error; err != nil {
			r.logger().Error("商户基本信息保存失败", "merchantName", merchant.Name, "error", err.Error())
			return err
		}
		if err := tx.Where("merchant_id = ?", model.ID).Delete(&CallRateMerchantModel{}).Error; err != nil {
			r.logger().Error("清除商户历史费率映射失败", "merchantId", model.ID, "error", err.Error())
			return err
		}
		if merchant.RateID > 0 {
			if err := tx.Create(&CallRateMerchantModel{RateID: merchant.RateID, MerchantID: model.ID}).Error; err != nil {
				r.logger().Error("创建商户新费率映射失败", "merchantId", model.ID, "rateId", merchant.RateID, "error", err.Error())
				return err
			}
		}
		// Initialize billing overview if it doesn't exist
		var count int64
		if err := tx.Model(&MerchantBillingOverviewModel{}).Where("merchant_id = ?", model.ID).Count(&count).Error; err == nil && count == 0 {
			overview := MerchantBillingOverviewModel{
				MerchantID:         model.ID,
				PaymentMode:        1, // Prepaid
				CurrentBalance:     0,
				CreditLimit:        0,
				DailyTotalAmount:   0,
				MonthlyTotalAmount: 0,
				CreatedTime:        now,
				UpdatedTime:        now,
			}
			if err := tx.Create(&overview).Error; err != nil {
				r.logger().Error("初始化商户账务总览记录失败", "merchantId", model.ID, "error", err.Error())
				return err
			}
			r.logger().Info("成功初始化商户账务总览记录", "merchantId", model.ID)
		}

		// 动态管理坐席分机数量
		var existing []ExtensionModel
		if err := tx.Where("merchant_id = ? AND del_flag = ?", model.ID, false).Order("extension_number ASC").Find(&existing).Error; err != nil {
			return err
		}
		existingCount := len(existing)
		if existingCount < merchant.MaxAgents {
			// 需要扩容坐席分机
			diff := merchant.MaxAgents - existingCount
			r.logger().Info("商户最大坐席数扩容，开始自动生成分机", "merchantId", model.ID, "currentCount", existingCount, "targetCount", merchant.MaxAgents, "diff", diff)
			var maxNum int64 = 99999 // 默认从 100000 开始，所以初始前置为 99999
			var extNumStr string
			// 兼容不同数据库，取出全系统最大的分机号
			err := tx.Model(&ExtensionModel{}).Order("LENGTH(extension_number) DESC, extension_number DESC").Limit(1).Pluck("extension_number", &extNumStr).Error
			if err == nil && extNumStr != "" {
				if parsed, err := strconv.ParseInt(extNumStr, 10, 64); err == nil && parsed >= 100000 {
					maxNum = parsed
				}
			}
			sipDomain := model.SipDomain
			if sipDomain == "" {
				sipDomain = "127.0.0.1"
			}
			for i := 0; i < diff; i++ {
				nextNum := maxNum + 1 + int64(i)
				extNum := strconv.FormatInt(nextNum, 10)
				randomPassword := operate.GenerateRandomPassword(8)
				newExt := ExtensionModel{
					ExtensionNumber: extNum,
					Password:        randomPassword,
					MerchantID:      model.ID,
					UserID:          0, // 初始未分配用户
					Enable:          true,
					BindType:        1, // 默认为手动绑定类型
					DelFlag:         false,
					CreatedTime:     now,
					UpdatedTime:     now,
					SipDomain:       sipDomain,
					HA1:             calculateHA1(extNum, sipDomain, randomPassword),
					HA1b:            calculateHA1b(extNum, sipDomain, randomPassword),
				}
				if err := tx.Create(&newExt).Error; err != nil {
					r.logger().Error("自动生成分机失败", "merchantId", model.ID, "extensionNumber", newExt.ExtensionNumber, "error", err.Error())
					return err
				}
			}
			r.logger().Info("商户坐席分机扩容成功", "merchantId", model.ID, "addedCount", diff)
		} else if existingCount > merchant.MaxAgents {
			// 需要缩减多余坐席分机
			diff := existingCount - merchant.MaxAgents
			r.logger().Info("商户最大坐席数缩缩，开始自动缩减分机", "merchantId", model.ID, "currentCount", existingCount, "targetCount", merchant.MaxAgents, "diff", diff)
			// 排序规则（保留优先级）：
			// 1. 绑定优先：已绑定用户(UserID > 0)的优先保留
			// 2. 在线优先：当前注册在线的优先保留
			// 3. 小号优先：号码小的优先保留，大号在尾部被优先截断删除
			sort.Slice(existing, func(i, j int) bool {
				iBound := existing[i].UserID > 0
				jBound := existing[j].UserID > 0
				if iBound != jBound {
					return iBound
				}
				iOnline := false
				jOnline := false
				if r.Statuses != nil {
					if status, ok, err := r.Statuses.GetExtensionStatus(ctx, existing[i].ExtensionNumber); err == nil && ok && status != esl.ExtensionStatusOffline {
						iOnline = true
					}
					if status, ok, err := r.Statuses.GetExtensionStatus(ctx, existing[j].ExtensionNumber); err == nil && ok && status != esl.ExtensionStatusOffline {
						jOnline = true
					}
				}
				if iOnline != jOnline {
					return iOnline
				}
				return existing[i].ExtensionNumber < existing[j].ExtensionNumber
			})
			// 截取保留优先级较低的尾部 diff 个分机逻辑删除
			toDelete := existing[existingCount-diff:]
			var deleteIDs []int
			var deleteNums []string
			for _, ext := range toDelete {
				deleteIDs = append(deleteIDs, ext.ID)
				deleteNums = append(deleteNums, ext.ExtensionNumber)
			}
			if len(deleteIDs) > 0 {
				r.logger().Info("缩减删除冗余分机", "merchantId", model.ID, "extensionNumbers", deleteNums)
				if err := tx.Model(&ExtensionModel{}).Where("id IN ?", deleteIDs).Updates(map[string]any{"del_flag": true, "updated_time": now}).Error; err != nil {
					r.logger().Error("自动缩减分机更新失败", "merchantId", model.ID, "error", err.Error())
					return err
				}
			}
			r.logger().Info("商户坐席分机缩减成功", "merchantId", model.ID, "deletedCount", len(deleteIDs))
		}

		// 级联同步更新现存的所有分机的域名和鉴权哈希
		var existingExts []ExtensionModel
		if err := tx.Where("merchant_id = ? AND del_flag = ?", model.ID, false).Find(&existingExts).Error; err == nil {
			sipDomain := model.SipDomain
			if sipDomain == "" {
				sipDomain = "127.0.0.1"
			}
			for i := range existingExts {
				ext := &existingExts[i]
				targetHA1 := calculateHA1(ext.ExtensionNumber, sipDomain, ext.Password)
				targetHA1b := calculateHA1b(ext.ExtensionNumber, sipDomain, ext.Password)
				if ext.SipDomain != sipDomain || ext.HA1 != targetHA1 || ext.HA1b != targetHA1b {
					ext.SipDomain = sipDomain
					ext.HA1 = targetHA1
					ext.HA1b = targetHA1b
					ext.UpdatedTime = now
					if err := tx.Save(ext).Error; err != nil {
						r.logger().Error("级联更新分机域名及哈希失败", "merchantId", model.ID, "extensionNumber", ext.ExtensionNumber, "error", err.Error())
						return err
					}
				}
			}
		}

		return nil
	}); err != nil {
		r.logger().Error("保存商户事务执行异常", "merchantName", merchant.Name, "error", err.Error())
		return operate.Merchant{}, err
	}
	r.logger().Info("保存商户信息成功", "merchantId", model.ID, "merchantName", model.Name)
	return merchantFromModel(model, merchant.RateID), nil
}

// Delete 逻辑删除商户，并级联逻辑删除该商户底下的控制台账号。
func (r *MerchantRepository) Delete(ctx context.Context, ids []int) error {
	r.logger().Info("开始逻辑删除商户及级联删除控制台账号", "merchantIds", ids)
	if len(ids) == 0 {
		return nil
	}
	var merchantIDStrs []string
	for _, id := range ids {
		merchantIDStrs = append(merchantIDStrs, strconv.Itoa(id))
	}
	now := time.Now().UTC()
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 逻辑删除商户自身
		result := tx.Model(&MerchantModel{}).
			Where("id IN ?", ids).
			Updates(map[string]any{"del_flag": true, "updated_time": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return operate.ErrMerchantNotFound
		}

		// 2. 级联逻辑删除商户底下的控制台账号
		if err := tx.Table("cc_sys_account").
			Where("merchant_id IN ? AND del_flag = ?", merchantIDStrs, false).
			Updates(map[string]any{
				"del_flag":     true,
				"updated_time": now,
				"deleted_time": &now,
			}).Error; err != nil {
			r.logger().Error("级联逻辑删除商户控制台账号失败", "merchantIds", ids, "error", err.Error())
			return err
		}

		return nil
	})
	if err != nil {
		r.logger().Error("逻辑删除商户事务执行异常", "merchantIds", ids, "error", err.Error())
		return err
	}
	r.logger().Info("逻辑删除商户及级联删除控制台账号成功", "merchantIds", ids)
	return nil
}

// MemoryMerchantRepository 供本地开发和测试使用。
type MemoryMerchantRepository struct {
	mu        sync.Mutex
	nextID    int
	merchants map[int]operate.Merchant
}

// NewMemoryMerchantRepository 创建商户内存仓储。
func NewMemoryMerchantRepository() *MemoryMerchantRepository {
	return &MemoryMerchantRepository{nextID: 1, merchants: map[int]operate.Merchant{}}
}

func (r *MemoryMerchantRepository) Page(_ context.Context, req operate.MerchantPageRequest) (operate.MerchantPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Merchant, 0, len(r.merchants))
	for _, merchant := range r.merchants {
		if req.Name != "" && !strings.Contains(merchant.Name, req.Name) {
			continue
		}
		if req.Account != "" && !strings.Contains(merchant.Account, req.Account) {
			continue
		}
		if req.Enable != nil && merchant.Enable != *req.Enable {
			continue
		}
		records = append(records, merchant)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Merchant{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.MerchantPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryMerchantRepository) GetByID(_ context.Context, id int) (operate.Merchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	merchant, ok := r.merchants[id]
	if !ok {
		return operate.Merchant{}, operate.ErrMerchantNotFound
	}
	return merchant, nil
}

func (r *MemoryMerchantRepository) ExistsNameOrAccount(_ context.Context, name, account string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, merchant := range r.merchants {
		if id == excludeID {
			continue
		}
		if merchant.Name == name || merchant.Account == account {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryMerchantRepository) RateExists(_ context.Context, rateID int) (bool, error) {
	return rateID >= 0, nil
}

func (r *MemoryMerchantRepository) Save(_ context.Context, merchant operate.Merchant) (operate.Merchant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if merchant.ID == 0 {
		merchant.ID = r.nextID
		r.nextID++
	}
	r.merchants[merchant.ID] = merchant
	return merchant, nil
}

func (r *MemoryMerchantRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for _, id := range ids {
		if _, ok := r.merchants[id]; ok {
			delete(r.merchants, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrMerchantNotFound
	}
	return nil
}

func merchantToModel(merchant operate.Merchant) MerchantModel {
	return MerchantModel{
		ID:               merchant.ID,
		Name:             merchant.Name,
		Account:          merchant.Account,
		ExpiredTime:      merchant.ExpiredTime,
		WhitelistDomains: merchant.WhitelistDomains,
		SipDomain:        merchant.SipDomain,
		Enable:           merchant.Enable,
		AppKey:           merchant.AppKey,
		AppSecret:        merchant.AppSecret,
		MaxAgents:        merchant.MaxAgents,
		DelFlag:          false,
	}
}

func merchantFromModel(model MerchantModel, rateID int) operate.Merchant {
	return operate.Merchant{
		ID:               model.ID,
		Name:             model.Name,
		Account:          model.Account,
		ExpiredTime:      model.ExpiredTime,
		RateID:           rateID,
		WhitelistDomains: model.WhitelistDomains,
		SipDomain:        model.SipDomain,
		Enable:           model.Enable,
		AppKey:           model.AppKey,
		AppSecret:        model.AppSecret,
		MaxAgents:        model.MaxAgents,
	}
}

func (r *MerchantRepository) loadMerchantRateBindings(ctx context.Context, merchantIDs []int) (map[int]int, error) {
	result := make(map[int]int, len(merchantIDs))
	if len(merchantIDs) == 0 {
		return result, nil
	}
	var refs []CallRateMerchantModel
	if err := r.DB.WithContext(ctx).Where("merchant_id IN ?", merchantIDs).Find(&refs).Error; err != nil {
		return nil, err
	}
	for _, ref := range refs {
		result[ref.MerchantID] = ref.RateID
	}
	return result, nil
}

func extractMerchantIDs(models []MerchantModel) []int {
	ids := make([]int, 0, len(models))
	for _, model := range models {
		if model.ID > 0 {
			ids = append(ids, model.ID)
		}
	}
	return ids
}

func (r *MerchantRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func calculateMD5(val string) string {
	h := md5.New()
	h.Write([]byte(val))
	return hex.EncodeToString(h.Sum(nil))
}

func calculateHA1(username, realm, password string) string {
	return calculateMD5(username + ":" + realm + ":" + password)
}

func calculateHA1b(username, realm, password string) string {
	return calculateMD5(username + "@" + realm + ":" + realm + ":" + password)
}
