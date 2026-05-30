package operate

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"yunshu/internal/contracts"
)

const (
	// AccountTypeSuperAdmin 表示平台超级管理员账号，可查看并维护全部数据。
	AccountTypeSuperAdmin = "super_admin"
	// AccountTypeOperate 表示运营账号，是否能维护商户取决于权限码。
	AccountTypeOperate = "operate_user"
	// AccountTypeMerchantAdmin 表示商户管理员账号，每个商户只能有一个启用中的管理员。
	AccountTypeMerchantAdmin = "merchant_admin"
	// AccountTypeMerchantUser 表示商户实际使用账号，必须绑定到一个商户。
	AccountTypeMerchantUser = "merchant_user"

	// DataScopeGlobal 表示平台级数据范围。
	DataScopeGlobal = "global"
	// DataScopeMerchant 表示商户级数据范围。
	DataScopeMerchant = "merchant"
)

var (
	// ErrInvalidAccount 表示账号参数缺失或类型不合法。
	ErrInvalidAccount = errors.New("invalid account")
	// ErrAccountNotFound 表示账号不存在或已删除。
	ErrAccountNotFound = errors.New("account not found")
	// ErrAccountConflict 表示账号名或商户管理员唯一性冲突。
	ErrAccountConflict = errors.New("account conflict")
	// ErrAccountForbidden 表示操作者无权维护目标账号。
	ErrAccountForbidden = errors.New("account forbidden")
)

// Account 表示 Go 重新设计后的控制台统一账号。
//
// 账号类型明确区分平台、运营和商户侧身份；商户侧账号必须带 merchantId，
// 运营和超级管理员账号不能绑定到单个商户，避免数据范围在登录后漂移。
type Account struct {
	ID          int        `json:"id,omitempty"`
	Username    string     `json:"username"`
	Password    string     `json:"password,omitempty"`
	MerchantID  string     `json:"merchantId,omitempty"`
	UserID      string     `json:"userId,omitempty"`
	RoleID      string     `json:"roleId"`
	AccountType string     `json:"accountType"`
	DataScope   string     `json:"dataScope"`
	Enable      bool       `json:"enable"`
	CreatedBy   string     `json:"createdBy,omitempty"`
	UpdatedBy   string     `json:"updatedBy,omitempty"`
	CreatedTime *time.Time `json:"createdTime,omitempty"`
	UpdatedTime *time.Time `json:"updatedTime,omitempty"`
}

// AccountPageRequest 表示账号分页查询条件。
type AccountPageRequest struct {
	PageNumber  int    `json:"pageNumber"`
	PageSize    int    `json:"pageSize"`
	Username    string `json:"username,omitempty"`
	MerchantID  string `json:"merchantId,omitempty"`
	AccountType string `json:"accountType,omitempty"`
	RoleID      string `json:"roleId,omitempty"`
	Enable      *bool  `json:"enable,omitempty"`
}

// AccountPageResult 是账号分页结果。
type AccountPageResult struct {
	PageNumber int       `json:"pageNumber"`
	PageSize   int       `json:"pageSize"`
	Total      int64     `json:"total"`
	Records    []Account `json:"records"`
}

// AccountMutationResult 描述账号写入后的结果。
type AccountMutationResult struct {
	Account Account `json:"account,omitempty"`
}

// AccountRepository 定义控制台账号仓储能力。
type AccountRepository interface {
	Page(ctx context.Context, req AccountPageRequest) (AccountPageResult, error)
	GetByID(ctx context.Context, id int) (Account, error)
	ExistsUsername(ctx context.Context, username string, excludeID int) (bool, error)
	ActiveMerchantAdminExists(ctx context.Context, merchantID string, excludeID int) (bool, error)
	Save(ctx context.Context, account Account) (Account, error)
	Delete(ctx context.Context, ids []int) error
	SetEnable(ctx context.Context, id int, enable bool, operator string) (Account, error)
	ResetPassword(ctx context.Context, id int, password string, operator string) error
}

// AccountManagementService 承载控制台账号体系维护业务。
type AccountManagementService struct {
	Repository AccountRepository
	Logger     *slog.Logger
}

// Page 按操作者数据范围返回账号列表。
func (s *AccountManagementService) Page(ctx context.Context, req AccountPageRequest) (AccountPageResult, error) {
	logger := s.logger()
	req = normalizeAccountPage(req)
	tenant, _ := contracts.TenantFromContext(ctx)
	req = scopeAccountPage(req, tenant)
	logger.Info("控制台开始分页查询账号", "pageNumber", req.PageNumber, "pageSize", req.PageSize, "merchantId", req.MerchantID, "accountType", req.AccountType)
	page, err := s.Repository.Page(ctx, req)
	if err != nil {
		logger.Error("控制台分页查询账号失败", "merchantId", req.MerchantID, "accountType", req.AccountType, "error", err.Error())
		return AccountPageResult{}, err
	}
	logger.Info("控制台分页查询账号完成", "total", page.Total, "recordCount", len(page.Records))
	return page, nil
}

// Get 返回账号详情，并按操作者数据范围二次校验。
func (s *AccountManagementService) Get(ctx context.Context, id int) (Account, error) {
	if id <= 0 {
		return Account{}, ErrInvalidAccount
	}
	account, err := s.Repository.GetByID(ctx, id)
	if err != nil {
		return Account{}, err
	}
	tenant, _ := contracts.TenantFromContext(ctx)
	if err := authorizeAccountRead(tenant, account); err != nil {
		s.logger().Warn("控制台查询账号详情被拒绝", "id", id, "operatorUserId", tenant.UserID, "accountType", account.AccountType, "merchantId", account.MerchantID)
		return Account{}, err
	}
	return account, nil
}

// Save 创建或更新账号，并执行账号类型、商户范围和唯一管理员约束。
func (s *AccountManagementService) Save(ctx context.Context, account Account) (AccountMutationResult, error) {
	logger := s.logger()
	tenant, _ := contracts.TenantFromContext(ctx)
	normalized, err := normalizeAccountForSave(account, tenant)
	if err != nil {
		logger.Warn("控制台保存账号参数无效", "id", account.ID, "username", account.Username, "accountType", account.AccountType, "error", err.Error())
		return AccountMutationResult{}, err
	}
	if err := authorizeAccountMutation(tenant, normalized, contracts.PermissionOperateAccountWrite, contracts.PermissionMerchantAccountWrite); err != nil {
		logger.Warn("控制台保存账号被拒绝", "operatorUserId", tenant.UserID, "targetAccountType", normalized.AccountType, "merchantId", normalized.MerchantID)
		return AccountMutationResult{}, err
	}
	exists, err := s.Repository.ExistsUsername(ctx, normalized.Username, normalized.ID)
	if err != nil {
		logger.Error("控制台校验账号名唯一性失败", "username", normalized.Username, "error", err.Error())
		return AccountMutationResult{}, err
	}
	if exists {
		logger.Warn("控制台保存账号冲突，账号名已存在", "username", normalized.Username)
		return AccountMutationResult{}, ErrAccountConflict
	}
	if normalized.AccountType == AccountTypeMerchantAdmin && normalized.Enable {
		existsAdmin, err := s.Repository.ActiveMerchantAdminExists(ctx, normalized.MerchantID, normalized.ID)
		if err != nil {
			logger.Error("控制台校验商户管理员唯一性失败", "merchantId", normalized.MerchantID, "error", err.Error())
			return AccountMutationResult{}, err
		}
		if existsAdmin {
			logger.Warn("控制台保存账号冲突，商户已存在管理员", "merchantId", normalized.MerchantID)
			return AccountMutationResult{}, ErrAccountConflict
		}
	}
	operator := operatorID(tenant)
	normalized.UpdatedBy = operator
	if normalized.ID == 0 {
		normalized.CreatedBy = operator
	}
	logger.Info("控制台开始保存账号", "id", normalized.ID, "username", normalized.Username, "accountType", normalized.AccountType, "merchantId", normalized.MerchantID, "operatorUserId", tenant.UserID)
	saved, err := s.Repository.Save(ctx, normalized)
	if err != nil {
		logger.Error("控制台保存账号失败", "username", normalized.Username, "merchantId", normalized.MerchantID, "error", err.Error())
		return AccountMutationResult{}, err
	}
	logger.Info("控制台保存账号完成", "id", saved.ID, "username", saved.Username, "accountType", saved.AccountType, "merchantId", saved.MerchantID)
	return AccountMutationResult{Account: saved}, nil
}

// Delete 逻辑删除账号。
func (s *AccountManagementService) Delete(ctx context.Context, accounts []Account) (AccountMutationResult, error) {
	logger := s.logger()
	ids := positiveAccountIDs(accounts)
	if len(ids) == 0 {
		return AccountMutationResult{}, ErrInvalidAccount
	}
	if err := s.authorizeExistingAccounts(ctx, ids, contracts.PermissionOperateAccountDelete, contracts.PermissionMerchantAccountDelete); err != nil {
		return AccountMutationResult{}, err
	}
	logger.Info("控制台开始删除账号", "accountCount", len(ids))
	if err := s.Repository.Delete(ctx, ids); err != nil {
		logger.Error("控制台删除账号失败", "accountCount", len(ids), "error", err.Error())
		return AccountMutationResult{}, err
	}
	logger.Info("控制台删除账号完成", "accountCount", len(ids))
	return AccountMutationResult{}, nil
}

// Enable 切换账号启用状态；启用商户管理员时仍需保持单管理员约束。
func (s *AccountManagementService) Enable(ctx context.Context, id int, enable bool) (AccountMutationResult, error) {
	logger := s.logger()
	if id <= 0 {
		return AccountMutationResult{}, ErrInvalidAccount
	}
	tenant, _ := contracts.TenantFromContext(ctx)
	account, err := s.Repository.GetByID(ctx, id)
	if err != nil {
		return AccountMutationResult{}, err
	}
	if err := authorizeAccountMutation(tenant, account, contracts.PermissionOperateAccountToggle, contracts.PermissionMerchantAccountToggle); err != nil {
		logger.Warn("控制台切换账号状态被拒绝", "id", id, "operatorUserId", tenant.UserID, "accountType", account.AccountType, "merchantId", account.MerchantID)
		return AccountMutationResult{}, err
	}
	if enable && account.AccountType == AccountTypeMerchantAdmin {
		existsAdmin, err := s.Repository.ActiveMerchantAdminExists(ctx, account.MerchantID, account.ID)
		if err != nil {
			return AccountMutationResult{}, err
		}
		if existsAdmin {
			return AccountMutationResult{}, ErrAccountConflict
		}
	}
	logger.Info("控制台开始切换账号状态", "id", id, "enable", enable, "operatorUserId", tenant.UserID)
	saved, err := s.Repository.SetEnable(ctx, id, enable, operatorID(tenant))
	if err != nil {
		logger.Error("控制台切换账号状态失败", "id", id, "enable", enable, "error", err.Error())
		return AccountMutationResult{}, err
	}
	logger.Info("控制台切换账号状态完成", "id", saved.ID, "enable", saved.Enable)
	return AccountMutationResult{Account: saved}, nil
}

// ResetPassword 重置账号密码。
func (s *AccountManagementService) ResetPassword(ctx context.Context, id int, password string) (AccountMutationResult, error) {
	logger := s.logger()
	password = strings.TrimSpace(password)
	if id <= 0 || password == "" {
		return AccountMutationResult{}, ErrInvalidAccount
	}
	tenant, _ := contracts.TenantFromContext(ctx)
	account, err := s.Repository.GetByID(ctx, id)
	if err != nil {
		return AccountMutationResult{}, err
	}
	if err := authorizeAccountMutation(tenant, account, contracts.PermissionOperateAccountPassword, contracts.PermissionMerchantAccountPassword); err != nil {
		logger.Warn("控制台重置账号密码被拒绝", "id", id, "operatorUserId", tenant.UserID, "accountType", account.AccountType, "merchantId", account.MerchantID)
		return AccountMutationResult{}, err
	}
	logger.Info("控制台开始重置账号密码", "id", id, "operatorUserId", tenant.UserID)
	if err := s.Repository.ResetPassword(ctx, id, password, operatorID(tenant)); err != nil {
		logger.Error("控制台重置账号密码失败", "id", id, "error", err.Error())
		return AccountMutationResult{}, err
	}
	logger.Info("控制台重置账号密码完成", "id", id)
	return AccountMutationResult{}, nil
}

func (s *AccountManagementService) authorizeExistingAccounts(ctx context.Context, ids []int, operatePermission contracts.PermissionCode, merchantPermission contracts.PermissionCode) error {
	tenant, _ := contracts.TenantFromContext(ctx)
	for _, id := range ids {
		account, err := s.Repository.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if err := authorizeAccountMutation(tenant, account, operatePermission, merchantPermission); err != nil {
			s.logger().Warn("控制台账号维护被拒绝", "id", id, "operatorUserId", tenant.UserID, "accountType", account.AccountType, "merchantId", account.MerchantID)
			return err
		}
	}
	return nil
}

func (s *AccountManagementService) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func normalizeAccountPage(req AccountPageRequest) AccountPageRequest {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}
	req.Username = strings.TrimSpace(req.Username)
	req.MerchantID = strings.TrimSpace(req.MerchantID)
	req.AccountType = strings.TrimSpace(req.AccountType)
	req.RoleID = strings.TrimSpace(req.RoleID)
	return req
}

func scopeAccountPage(req AccountPageRequest, tenant contracts.TenantContext) AccountPageRequest {
	if tenant.Internal || tenant.HasPermission(string(contracts.PermissionConsoleAll)) || tenant.HasPermission(string(contracts.PermissionOperateAccountRead)) {
		return req
	}
	if tenant.MerchantID != "" {
		req.MerchantID = tenant.MerchantID
	}
	return req
}

func normalizeAccountForSave(account Account, tenant contracts.TenantContext) (Account, error) {
	account.Username = strings.ToLower(strings.TrimSpace(account.Username))
	account.Password = strings.TrimSpace(account.Password)
	account.MerchantID = strings.TrimSpace(account.MerchantID)
	account.UserID = strings.TrimSpace(account.UserID)
	account.RoleID = strings.TrimSpace(account.RoleID)
	account.AccountType = strings.TrimSpace(account.AccountType)
	account.DataScope = strings.TrimSpace(account.DataScope)
	if account.Username == "" || account.AccountType == "" {
		return Account{}, ErrInvalidAccount
	}
	if account.ID == 0 && account.Password == "" {
		return Account{}, ErrInvalidAccount
	}
	switch account.AccountType {
	case AccountTypeSuperAdmin:
		account.MerchantID = ""
		account.DataScope = DataScopeGlobal
		if account.RoleID == "" {
			account.RoleID = "super_admin"
		}
	case AccountTypeOperate:
		account.MerchantID = ""
		account.DataScope = DataScopeGlobal
		if account.RoleID == "" {
			account.RoleID = "operate_staff"
		}
	case AccountTypeMerchantAdmin:
		if account.MerchantID == "" {
			return Account{}, ErrInvalidAccount
		}
		account.DataScope = DataScopeMerchant
		if account.RoleID == "" {
			account.RoleID = "merchant_admin"
		}
	case AccountTypeMerchantUser:
		if account.MerchantID == "" {
			if tenant.MerchantID == "" {
				return Account{}, ErrInvalidAccount
			}
			account.MerchantID = tenant.MerchantID
		}
		account.DataScope = DataScopeMerchant
		if account.RoleID == "" {
			account.RoleID = "merchant_user"
		}
	default:
		return Account{}, ErrInvalidAccount
	}
	return account, nil
}

func authorizeAccountRead(tenant contracts.TenantContext, target Account) error {
	if tenant.Internal || tenant.HasPermission(string(contracts.PermissionConsoleAll)) {
		return nil
	}
	if tenant.HasPermission(string(contracts.PermissionOperateAccountRead)) {
		return nil
	}
	if tenant.MerchantID != "" && tenant.MerchantID == target.MerchantID && tenant.HasPermission(string(contracts.PermissionMerchantAccountRead)) {
		return nil
	}
	return ErrAccountForbidden
}

func authorizeAccountMutation(tenant contracts.TenantContext, target Account, operatePermission contracts.PermissionCode, merchantPermission contracts.PermissionCode) error {
	if tenant.Internal || tenant.HasPermission(string(contracts.PermissionConsoleAll)) {
		return nil
	}
	if target.AccountType == AccountTypeOperate || target.AccountType == AccountTypeSuperAdmin {
		return ErrAccountForbidden
	}
	if tenant.HasPermission(string(operatePermission)) {
		return nil
	}
	if tenant.MerchantID != "" && tenant.MerchantID == target.MerchantID && target.AccountType == AccountTypeMerchantUser && tenant.HasPermission(string(merchantPermission)) {
		return nil
	}
	return ErrAccountForbidden
}

func positiveAccountIDs(accounts []Account) []int {
	ids := make([]int, 0, len(accounts))
	for _, account := range accounts {
		if account.ID > 0 {
			ids = append(ids, account.ID)
		}
	}
	return ids
}

func operatorID(tenant contracts.TenantContext) string {
	if tenant.UserID != "" {
		return tenant.UserID
	}
	if tenant.RoleID != "" {
		return tenant.RoleID
	}
	return "system"
}
