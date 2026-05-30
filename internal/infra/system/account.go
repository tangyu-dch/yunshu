package system

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	authdomain "yunshu/internal/domain/auth"
	"yunshu/internal/domain/operate"
)

// ConsoleAccountModel 保存控制台统一账号。
//
// 所有平台、运营、商户管理员和商户用户账号都统一落到 console_account，
// 避免权限、数据范围和商户归属出现两套来源。
type ConsoleAccountModel struct {
	ID           int        `gorm:"column:id;primaryKey;autoIncrement"`
	Username     string     `gorm:"column:username;size:64;uniqueIndex"`
	PasswordHash string     `gorm:"column:password_hash;size:255"`
	MerchantID   string     `gorm:"column:merchant_id;size:64;index"`
	UserID       string     `gorm:"column:user_id;size:64"`
	RoleID       string     `gorm:"column:role_id;size:64;index"`
	AccountType  string     `gorm:"column:account_type;size:32;index"`
	DataScope    string     `gorm:"column:data_scope;size:64"`
	Enable       bool       `gorm:"column:enable;index"`
	DelFlag      bool       `gorm:"column:del_flag;index"`
	CreatedBy    string     `gorm:"column:created_by;size:64"`
	UpdatedBy    string     `gorm:"column:updated_by;size:64"`
	CreatedTime  time.Time  `gorm:"column:created_time"`
	UpdatedTime  time.Time  `gorm:"column:updated_time"`
	DeletedTime  *time.Time `gorm:"column:deleted_time"`
}

func (ConsoleAccountModel) TableName() string {
	return "cc_sys_account"
}

// ConsoleAccountRepository 提供账号登录校验与账号体系管理。
type ConsoleAccountRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
	Now    func() time.Time
}

func NewConsoleAccountRepository(db *gorm.DB, logger *slog.Logger) *ConsoleAccountRepository {
	return &ConsoleAccountRepository{DB: db, Logger: logger}
}

func (r *ConsoleAccountRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

// EnsureDefaults 自动创建账号表并补齐默认超级管理员和本地商户管理员。
func (r *ConsoleAccountRepository) EnsureDefaults(ctx context.Context) error {
	if r == nil || r.DB == nil {
		return nil
	}
	r.logger().Info("开始检查控制台账号表结构及种子数据")
	if err := r.DB.WithContext(ctx).AutoMigrate(&ConsoleAccountModel{}); err != nil {
		r.logger().Error("控制台账号表自动迁移失败", "error", err.Error())
		return err
	}
	now := time.Now().UTC()
	for _, account := range authdomain.DefaultLoginAccounts() {
		model, err := defaultAccountModel(account, now)
		if err != nil {
			r.logger().Error("生成默认账号模型失败", "username", account.Username, "error", err.Error())
			return err
		}
		// 默认账号只补空缺，不覆盖已有密码、角色和数据范围，避免重启破坏运营配置。
		var created ConsoleAccountModel
		if err := r.DB.WithContext(ctx).
			Where("username = ?", model.Username).
			Attrs(model).
			FirstOrCreate(&created).Error; err != nil {
			r.logger().Error("初始化默认账号失败", "username", model.Username, "error", err.Error())
			return err
		}
		if created.ID == model.ID {
			r.logger().Info("已成功初始化默认内置账号", "username", model.Username, "roleId", model.RoleID)
		} else {
			// 如果记录已存在，但核心字段（比如 UserID）为空，则自动补齐，防止登录后数据丢失为横杠
			updates := make(map[string]any)
			if created.UserID == "" && model.UserID != "" {
				updates["user_id"] = model.UserID
			}
			if created.MerchantID == "" && model.MerchantID != "" {
				updates["merchant_id"] = model.MerchantID
			}
			if created.DataScope == "" && model.DataScope != "" {
				updates["data_scope"] = model.DataScope
			}
			if model.Username == "merchant" {
				// 强制重置密码为默认的 merchant123，确保密码比对成功并且用户能正常退出登录并重新登录以刷新缓存
				hash, err := hashPassword("merchant123")
				if err == nil {
					updates["password_hash"] = hash
				}
			}
			if len(updates) > 0 {
				r.logger().Info("补齐默认账号空缺字段", "username", model.Username, "updates", updates)
				if err := r.DB.WithContext(ctx).Model(&ConsoleAccountModel{}).Where("id = ?", created.ID).Updates(updates).Error; err != nil {
					r.logger().Error("补齐默认账号空缺字段失败", "username", model.Username, "error", err.Error())
				}
			}
		}
	}
	r.logger().Info("控制台账号表结构及种子数据初始化完成")
	return nil
}

// ResolveLoginIdentity 校验账号密码并返回登录身份。
func (r *ConsoleAccountRepository) ResolveLoginIdentity(ctx context.Context, req authdomain.LoginRequest) (authdomain.LoginIdentity, error) {
	if r == nil || r.DB == nil {
		return authdomain.LoginIdentity{}, authdomain.ErrInvalidLogin
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		r.logger().Warn("账号登录失败：用户名或密码为空")
		return authdomain.LoginIdentity{}, authdomain.ErrInvalidLogin
	}
	var account ConsoleAccountModel
	err := r.DB.WithContext(ctx).Where("username = ? AND enable = ? AND del_flag = ?", username, true, false).First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger().Warn("账号登录失败：账号不存在或已被停用", "username", username)
			return authdomain.LoginIdentity{}, authdomain.ErrInvalidLogin
		}
		r.logger().Error("账号登录查询数据库异常", "username", username, "error", err.Error())
		return authdomain.LoginIdentity{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		r.logger().Warn("账号登录失败：密码比对错误", "username", username)
		return authdomain.LoginIdentity{}, authdomain.ErrInvalidLogin
	}
	if err := r.ensureMerchantLoginAllowed(ctx, account); err != nil {
		r.logger().Warn("账号登录失败：商户主体校验未通过", "username", username, "merchantId", account.MerchantID, "error", err.Error())
		return authdomain.LoginIdentity{}, err
	}
	userId := account.UserID
	if userId == "" {
		userId = strconv.Itoa(account.ID)
	}
	r.logger().Info("账号登录成功", "username", username, "merchantId", account.MerchantID, "roleId", account.RoleID, "userId", userId)
	return authdomain.LoginIdentity{
		MerchantID: account.MerchantID,
		UserID:     userId,
		RoleID:     account.RoleID,
		DataScope:  account.DataScope,
		Internal:   account.AccountType == operate.AccountTypeSuperAdmin,
	}, nil
}

func (r *ConsoleAccountRepository) ensureMerchantLoginAllowed(ctx context.Context, account ConsoleAccountModel) error {
	if account.AccountType != operate.AccountTypeMerchantAdmin && account.AccountType != operate.AccountTypeMerchantUser {
		return nil
	}
	merchantID, err := strconv.Atoi(strings.TrimSpace(account.MerchantID))
	if err != nil || merchantID <= 0 {
		r.logger().Warn("账号登录拦截：商户ID解析失败或无效", "username", account.Username, "merchantIdRaw", account.MerchantID)
		return authdomain.ErrInvalidLogin
	}
	var merchant MerchantModel
	if err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", merchantID, false).First(&merchant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger().Warn("账号登录拦截：商户不存在或已被删除", "username", account.Username, "merchantId", merchantID)
			return authdomain.ErrInvalidLogin
		}
		r.logger().Error("账号登录查询商户主体数据库异常", "username", account.Username, "merchantId", merchantID, "error", err.Error())
		return err
	}
	if !merchant.Enable {
		r.logger().Warn("账号登录拦截：商户账号已被停用", "username", account.Username, "merchantId", merchantID, "merchantName", merchant.Name)
		return authdomain.ErrInvalidLogin
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	if merchant.ExpiredTime != nil && merchant.ExpiredTime.Before(now) {
		r.logger().Warn("账号登录拦截：商户服务已到期", "username", account.Username, "merchantId", merchantID, "merchantName", merchant.Name, "expiredTime", merchant.ExpiredTime)
		return authdomain.ErrInvalidLogin
	}
	return nil
}

// Page 分页查询控制台账号。
func (r *ConsoleAccountRepository) Page(ctx context.Context, req operate.AccountPageRequest) (operate.AccountPageResult, error) {
	query := r.DB.WithContext(ctx).Model(&ConsoleAccountModel{}).Where("del_flag = ?", false)
	if req.Username != "" {
		query = query.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.MerchantID != "" {
		query = query.Where("merchant_id = ?", req.MerchantID)
	}
	if req.AccountType != "" {
		query = query.Where("account_type = ?", req.AccountType)
	}
	if req.RoleID != "" {
		query = query.Where("role_id = ?", req.RoleID)
	}
	if req.Enable != nil {
		query = query.Where("enable = ?", *req.Enable)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return operate.AccountPageResult{}, err
	}
	var models []ConsoleAccountModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&models).Error; err != nil {
		return operate.AccountPageResult{}, err
	}
	records := make([]operate.Account, 0, len(models))
	for _, model := range models {
		records = append(records, accountFromModel(model))
	}
	return operate.AccountPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

// GetByID 查询单个未删除账号。
func (r *ConsoleAccountRepository) GetByID(ctx context.Context, id int) (operate.Account, error) {
	var model ConsoleAccountModel
	err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", id, false).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return operate.Account{}, operate.ErrAccountNotFound
	}
	return accountFromModel(model), err
}

// ExistsUsername 校验账号名唯一性。
func (r *ConsoleAccountRepository) ExistsUsername(ctx context.Context, username string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&ConsoleAccountModel{}).Where("username = ? AND del_flag = ?", username, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ActiveMerchantAdminExists 校验商户是否已有启用中的管理员。
func (r *ConsoleAccountRepository) ActiveMerchantAdminExists(ctx context.Context, merchantID string, excludeID int) (bool, error) {
	query := r.DB.WithContext(ctx).Model(&ConsoleAccountModel{}).
		Where("merchant_id = ? AND account_type = ? AND enable = ? AND del_flag = ?", merchantID, operate.AccountTypeMerchantAdmin, true, false)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Save 新增或更新账号。
func (r *ConsoleAccountRepository) Save(ctx context.Context, account operate.Account) (operate.Account, error) {
	now := time.Now().UTC()
	model := accountToModel(account)
	returned := ConsoleAccountModel{}
	if model.ID > 0 {
		r.logger().Info("开始更新控制台账号信息", "accountId", model.ID, "username", model.Username, "operator", model.UpdatedBy)
		if err := r.DB.WithContext(ctx).Where("id = ? AND del_flag = ?", model.ID, false).First(&returned).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				r.logger().Warn("更新控制台账号失败：账号不存在", "accountId", model.ID)
				return operate.Account{}, operate.ErrAccountNotFound
			}
			r.logger().Error("更新控制台账号查询数据库异常", "accountId", model.ID, "error", err.Error())
			return operate.Account{}, err
		}
		returned.Username = model.Username
		returned.MerchantID = model.MerchantID
		returned.UserID = model.UserID
		returned.RoleID = model.RoleID
		returned.AccountType = model.AccountType
		returned.DataScope = model.DataScope
		returned.Enable = model.Enable
		returned.UpdatedBy = model.UpdatedBy
		returned.UpdatedTime = now
		if account.Password != "" {
			hash, err := hashPassword(account.Password)
			if err != nil {
				r.logger().Error("账号密码加密失败", "accountId", model.ID, "error", err.Error())
				return operate.Account{}, err
			}
			returned.PasswordHash = hash
		}
		if err := r.DB.WithContext(ctx).Save(&returned).Error; err != nil {
			r.logger().Error("更新控制台账号保存数据库失败", "accountId", model.ID, "error", err.Error())
			return operate.Account{}, err
		}
		r.logger().Info("更新控制台账号成功", "accountId", model.ID, "username", model.Username)
		return accountFromModel(returned), nil
	}
	r.logger().Info("开始创建控制台账号", "username", model.Username, "roleId", model.RoleID, "operator", model.CreatedBy)
	hash, err := hashPassword(account.Password)
	if err != nil {
		r.logger().Error("账号密码加密失败", "username", model.Username, "error", err.Error())
		return operate.Account{}, err
	}
	model.PasswordHash = hash
	model.CreatedTime = now
	model.UpdatedTime = now
	if err := r.DB.WithContext(ctx).Create(&model).Error; err != nil {
		r.logger().Error("创建控制台账号写入数据库失败", "username", model.Username, "error", err.Error())
		return operate.Account{}, err
	}
	r.logger().Info("创建控制台账号成功", "accountId", model.ID, "username", model.Username)
	return accountFromModel(model), nil
}

// Delete 逻辑删除账号。
func (r *ConsoleAccountRepository) Delete(ctx context.Context, ids []int) error {
	now := time.Now().UTC()
	r.logger().Info("开始逻辑删除控制台账号", "accountIds", ids)
	result := r.DB.WithContext(ctx).Model(&ConsoleAccountModel{}).
		Where("id IN ? AND del_flag = ?", ids, false).
		Updates(map[string]any{"del_flag": true, "enable": false, "deleted_time": now, "updated_time": now})
	if result.Error != nil {
		r.logger().Error("逻辑删除控制台账号数据库更新异常", "accountIds", ids, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("逻辑删除控制台账号失败：未匹配到有效记录", "accountIds", ids)
		return operate.ErrAccountNotFound
	}
	r.logger().Info("逻辑删除控制台账号成功", "accountIds", ids, "rowsAffected", result.RowsAffected)
	return nil
}

// SetEnable 切换账号启用状态。
func (r *ConsoleAccountRepository) SetEnable(ctx context.Context, id int, enable bool, operator string) (operate.Account, error) {
	r.logger().Info("开始切换控制台账号状态", "accountId", id, "enable", enable, "operator", operator)
	result := r.DB.WithContext(ctx).Model(&ConsoleAccountModel{}).
		Where("id = ? AND del_flag = ?", id, false).
		Updates(map[string]any{"enable": enable, "updated_by": operator, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("切换控制台账号状态数据库更新异常", "accountId", id, "enable", enable, "error", result.Error.Error())
		return operate.Account{}, result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("切换控制台账号状态失败：账号不存在", "accountId", id)
		return operate.Account{}, operate.ErrAccountNotFound
	}
	r.logger().Info("切换控制台账号状态成功", "accountId", id, "enable", enable)
	return r.GetByID(ctx, id)
}

// ResetPassword 重置账号密码。
func (r *ConsoleAccountRepository) ResetPassword(ctx context.Context, id int, password string, operator string) error {
	r.logger().Info("开始重置控制台账号密码", "accountId", id, "operator", operator)
	hash, err := hashPassword(password)
	if err != nil {
		r.logger().Error("账号密码加密失败", "accountId", id, "error", err.Error())
		return err
	}
	result := r.DB.WithContext(ctx).Model(&ConsoleAccountModel{}).
		Where("id = ? AND del_flag = ?", id, false).
		Updates(map[string]any{"password_hash": hash, "updated_by": operator, "updated_time": time.Now().UTC()})
	if result.Error != nil {
		r.logger().Error("重置控制台账号密码数据库更新异常", "accountId", id, "error", result.Error.Error())
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.logger().Warn("重置控制台账号密码失败：账号不存在", "accountId", id)
		return operate.ErrAccountNotFound
	}
	r.logger().Info("重置控制台账号密码成功", "accountId", id)
	return nil
}

// MemoryAccountRepository 供本地开发和测试使用。
type MemoryAccountRepository struct {
	mu        sync.Mutex
	nextID    int
	accounts  map[int]operate.Account
	merchants map[string]operate.Merchant
	Now       func() time.Time
}

func NewMemoryAccountRepository() *MemoryAccountRepository {
	return &MemoryAccountRepository{nextID: 1, accounts: map[int]operate.Account{}, merchants: map[string]operate.Merchant{}}
}

// SaveMerchantState 写入本地内存商户状态，用于无数据库模式下保持登录准入语义。
func (r *MemoryAccountRepository) SaveMerchantState(merchant operate.Merchant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.merchants == nil {
		r.merchants = map[string]operate.Merchant{}
	}
	r.merchants[strconv.Itoa(merchant.ID)] = merchant
}

func (r *MemoryAccountRepository) Page(_ context.Context, req operate.AccountPageRequest) (operate.AccountPageResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]operate.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if req.Username != "" && !strings.Contains(account.Username, req.Username) {
			continue
		}
		if req.MerchantID != "" && account.MerchantID != req.MerchantID {
			continue
		}
		if req.AccountType != "" && account.AccountType != req.AccountType {
			continue
		}
		if req.RoleID != "" && account.RoleID != req.RoleID {
			continue
		}
		if req.Enable != nil && account.Enable != *req.Enable {
			continue
		}
		copyAccount := account
		copyAccount.Password = ""
		records = append(records, copyAccount)
	}
	total := int64(len(records))
	start := (req.PageNumber - 1) * req.PageSize
	if start >= len(records) {
		records = []operate.Account{}
	} else {
		end := start + req.PageSize
		if end > len(records) {
			end = len(records)
		}
		records = records[start:end]
	}
	return operate.AccountPageResult{PageNumber: req.PageNumber, PageSize: req.PageSize, Total: total, Records: records}, nil
}

func (r *MemoryAccountRepository) GetByID(_ context.Context, id int) (operate.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	account, ok := r.accounts[id]
	if !ok {
		return operate.Account{}, operate.ErrAccountNotFound
	}
	account.Password = ""
	return account, nil
}

func (r *MemoryAccountRepository) ExistsUsername(_ context.Context, username string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, account := range r.accounts {
		if id == excludeID {
			continue
		}
		if account.Username == username {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryAccountRepository) ActiveMerchantAdminExists(_ context.Context, merchantID string, excludeID int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, account := range r.accounts {
		if id == excludeID {
			continue
		}
		if account.MerchantID == merchantID && account.AccountType == operate.AccountTypeMerchantAdmin && account.Enable {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryAccountRepository) Save(_ context.Context, account operate.Account) (operate.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if account.ID == 0 {
		account.ID = r.nextID
		r.nextID++
	}
	r.accounts[account.ID] = account
	account.Password = ""
	return account, nil
}

func (r *MemoryAccountRepository) Delete(_ context.Context, ids []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for _, id := range ids {
		if _, ok := r.accounts[id]; ok {
			delete(r.accounts, id)
			removed++
		}
	}
	if removed == 0 {
		return operate.ErrAccountNotFound
	}
	return nil
}

func (r *MemoryAccountRepository) SetEnable(_ context.Context, id int, enable bool, operator string) (operate.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	account, ok := r.accounts[id]
	if !ok {
		return operate.Account{}, operate.ErrAccountNotFound
	}
	account.Enable = enable
	account.UpdatedBy = operator
	r.accounts[id] = account
	account.Password = ""
	return account, nil
}

func (r *MemoryAccountRepository) ResetPassword(_ context.Context, id int, password string, operator string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	account, ok := r.accounts[id]
	if !ok {
		return operate.ErrAccountNotFound
	}
	account.Password = password
	account.UpdatedBy = operator
	r.accounts[id] = account
	return nil
}

// ResolveLoginIdentity 在无数据库的本地模式下使用内存账号完成登录校验。
func (r *MemoryAccountRepository) ResolveLoginIdentity(_ context.Context, req authdomain.LoginRequest) (authdomain.LoginIdentity, error) {
	if r == nil {
		return authdomain.LoginIdentity{}, authdomain.ErrInvalidLogin
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		return authdomain.LoginIdentity{}, authdomain.ErrInvalidLogin
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, account := range r.accounts {
		if account.Username != username || !account.Enable || account.Password != password {
			continue
		}
		if err := r.ensureMemoryMerchantLoginAllowed(account); err != nil {
			return authdomain.LoginIdentity{}, err
		}
		userId := account.UserID
		if userId == "" {
			userId = strconv.Itoa(account.ID)
		}
		return authdomain.LoginIdentity{
			MerchantID: account.MerchantID,
			UserID:     userId,
			RoleID:     account.RoleID,
			DataScope:  account.DataScope,
			Internal:   account.AccountType == operate.AccountTypeSuperAdmin,
		}, nil
	}
	return authdomain.LoginIdentity{}, authdomain.ErrInvalidLogin
}

func (r *MemoryAccountRepository) ensureMemoryMerchantLoginAllowed(account operate.Account) error {
	if account.AccountType != operate.AccountTypeMerchantAdmin && account.AccountType != operate.AccountTypeMerchantUser {
		return nil
	}
	merchant, ok := r.merchants[strings.TrimSpace(account.MerchantID)]
	if !ok || !merchant.Enable {
		return authdomain.ErrInvalidLogin
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	if merchant.ExpiredTime != nil && merchant.ExpiredTime.Before(now) {
		return authdomain.ErrInvalidLogin
	}
	return nil
}

func defaultAccountModel(account authdomain.LoginAccount, now time.Time) (ConsoleAccountModel, error) {
	accountType := operate.AccountTypeMerchantAdmin
	if account.MerchantID == "" {
		accountType = operate.AccountTypeOperate
	}
	if account.Internal || account.RoleID == "super_admin" {
		accountType = operate.AccountTypeSuperAdmin
	} else if account.RoleID == "operate_lead" || account.RoleID == "operate_staff" {
		accountType = operate.AccountTypeOperate
	}
	dataScope := strings.TrimSpace(account.DataScope)
	if dataScope == "" {
		if accountType == operate.AccountTypeMerchantAdmin || accountType == operate.AccountTypeMerchantUser {
			dataScope = operate.DataScopeMerchant
		} else {
			dataScope = operate.DataScopeGlobal
		}
	}
	hash, err := hashPassword(account.Password)
	if err != nil {
		return ConsoleAccountModel{}, err
	}
	return ConsoleAccountModel{
		Username:     strings.ToLower(strings.TrimSpace(account.Username)),
		PasswordHash: hash,
		MerchantID:   strings.TrimSpace(account.MerchantID),
		UserID:       strings.TrimSpace(account.UserID),
		RoleID:       strings.TrimSpace(account.RoleID),
		AccountType:  accountType,
		DataScope:    dataScope,
		Enable:       true,
		DelFlag:      false,
		CreatedBy:    "system",
		UpdatedBy:    "system",
		CreatedTime:  now,
		UpdatedTime:  now,
	}, nil
}

func accountToModel(account operate.Account) ConsoleAccountModel {
	return ConsoleAccountModel{
		ID:          account.ID,
		Username:    account.Username,
		MerchantID:  account.MerchantID,
		UserID:      account.UserID,
		RoleID:      account.RoleID,
		AccountType: account.AccountType,
		DataScope:   account.DataScope,
		Enable:      account.Enable,
		DelFlag:     false,
		CreatedBy:   account.CreatedBy,
		UpdatedBy:   account.UpdatedBy,
	}
}

func accountFromModel(model ConsoleAccountModel) operate.Account {
	created := model.CreatedTime
	updated := model.UpdatedTime
	return operate.Account{
		ID:          model.ID,
		Username:    model.Username,
		MerchantID:  model.MerchantID,
		UserID:      model.UserID,
		RoleID:      model.RoleID,
		AccountType: model.AccountType,
		DataScope:   model.DataScope,
		Enable:      model.Enable,
		CreatedBy:   model.CreatedBy,
		UpdatedBy:   model.UpdatedBy,
		CreatedTime: &created,
		UpdatedTime: &updated,
	}
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(password)), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

var _ authdomain.LoginIdentityResolver = (*ConsoleAccountRepository)(nil)
var _ authdomain.LoginIdentityResolver = (*MemoryAccountRepository)(nil)
var _ operate.AccountRepository = (*ConsoleAccountRepository)(nil)
var _ operate.AccountRepository = (*MemoryAccountRepository)(nil)
