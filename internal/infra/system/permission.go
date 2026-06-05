package system

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"yunshu/internal/contracts"
	authdomain "yunshu/internal/domain/auth"
)

// ConsoleRoleModel 保存管理端可配置角色。
type ConsoleRoleModel struct {
	Code        string    `gorm:"column:code;primaryKey" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	Enable      bool      `gorm:"column:enable" json:"enable"`
	DelFlag     bool      `gorm:"column:del_flag" json:"delFlag"`
	CreatedTime time.Time `gorm:"column:created_time" json:"createdTime"`
	UpdatedTime time.Time `gorm:"column:updated_time" json:"updatedTime"`
}

func (ConsoleRoleModel) TableName() string {
	return "cc_sys_role"
}

// ConsolePermissionModel 保存管理端可配置的功能权限目录。
//
// 代码中的权限常量仍作为稳定标识，数据库记录用于运营端展示、启停和动态授权。
type ConsolePermissionModel struct {
	Code        string    `gorm:"column:code;primaryKey" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Module      string    `gorm:"column:module" json:"module"`
	Description string    `gorm:"column:description" json:"description"`
	Enable      bool      `gorm:"column:enable" json:"enable"`
	DelFlag     bool      `gorm:"column:del_flag" json:"delFlag"`
	CreatedTime time.Time `gorm:"column:created_time" json:"createdTime"`
	UpdatedTime time.Time `gorm:"column:updated_time" json:"updatedTime"`
}

func (ConsolePermissionModel) TableName() string {
	return "cc_sys_permission"
}

// ConsoleRolePermissionModel 保存角色与权限码的绑定。
type ConsoleRolePermissionModel struct {
	ID             int       `gorm:"column:id;primaryKey" json:"id"`
	RoleID         string    `gorm:"column:role_id;index" json:"roleId"`
	PermissionCode string    `gorm:"column:permission_code;index" json:"permissionCode"`
	Enable         bool      `gorm:"column:enable" json:"enable"`
	DelFlag        bool      `gorm:"column:del_flag" json:"delFlag"`
	CreatedTime    time.Time `gorm:"column:created_time" json:"createdTime"`
	UpdatedTime    time.Time `gorm:"column:updated_time" json:"updatedTime"`
}

func (ConsoleRolePermissionModel) TableName() string {
	return "cc_sys_role_permission"
}

// ConsoleRoutePermissionModel 保存路由到权限码的动态映射。
//
// Prefix/Suffix 语义对齐 contracts.PermissionRule，便于从静态规则平滑迁移到数据库。
type ConsoleRoutePermissionModel struct {
	ID             int       `gorm:"column:id;primaryKey" json:"id"`
	PathPrefix     string    `gorm:"column:path_prefix;index" json:"pathPrefix"`
	PathSuffix     string    `gorm:"column:path_suffix" json:"pathSuffix"`
	Method         string    `gorm:"column:method;index" json:"method"`
	PermissionCode string    `gorm:"column:permission_code" json:"permissionCode"`
	Sort           int       `gorm:"column:sort" json:"sort"`
	Enable         bool      `gorm:"column:enable" json:"enable"`
	DelFlag        bool      `gorm:"column:del_flag" json:"delFlag"`
	CreatedTime    time.Time `gorm:"column:created_time" json:"createdTime"`
	UpdatedTime    time.Time `gorm:"column:updated_time" json:"updatedTime"`
}

func (ConsoleRoutePermissionModel) TableName() string {
	return "cc_sys_route_permission"
}

// PermissionRepository 从数据库读取登录权限和路由权限映射。
type PermissionRepository struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

func NewPermissionRepository(db *gorm.DB, logger *slog.Logger) *PermissionRepository {
	return &PermissionRepository{DB: db, Logger: logger}
}

func (r *PermissionRepository) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

// EnsureDefaults 自动创建权限和角色相关表，并补齐默认种子数据。
func (r *PermissionRepository) EnsureDefaults(ctx context.Context) error {
	if r == nil || r.DB == nil {
		return nil
	}
	r.logger().Info("开始检查权限和角色表结构及种子数据")
	if err := r.DB.WithContext(ctx).AutoMigrate(&ConsoleRoleModel{}, &ConsolePermissionModel{}, &ConsoleRolePermissionModel{}, &ConsoleRoutePermissionModel{}); err != nil {
		r.logger().Error("权限和角色表自动迁移失败", "error", err.Error())
		return err
	}
	now := time.Now().UTC()
	defaultPermissions := defaultConsolePermissions()
	for _, permission := range defaultPermissions {
		permission.CreatedTime = now
		permission.UpdatedTime = now
		if err := r.DB.WithContext(ctx).Where("code = ?", permission.Code).Assign(permission).FirstOrCreate(&ConsolePermissionModel{}).Error; err != nil {
			r.logger().Error("初始化默认权限失败", "code", permission.Code, "error", err.Error())
			return err
		}
	}
	defaultRoles := defaultConsoleRoles()
	for _, role := range defaultRoles {
		role.CreatedTime = now
		role.UpdatedTime = now
		if err := r.DB.WithContext(ctx).Where("code = ?", role.Code).Assign(role).FirstOrCreate(&ConsoleRoleModel{}).Error; err != nil {
			r.logger().Error("初始化默认角色失败", "code", role.Code, "error", err.Error())
			return err
		}
	}
	defaultRolePermissions := defaultConsoleRolePermissions()
	for roleCode, permissions := range defaultRolePermissions {
		for _, permissionCode := range permissions {
			record := ConsoleRolePermissionModel{
				RoleID:         roleCode,
				PermissionCode: permissionCode,
				Enable:         true,
				DelFlag:        false,
				CreatedTime:    now,
				UpdatedTime:    now,
			}
			if err := r.DB.WithContext(ctx).
				Where("role_id = ? AND permission_code = ?", roleCode, permissionCode).
				Assign(record).
				FirstOrCreate(&ConsoleRolePermissionModel{}).Error; err != nil {
				r.logger().Error("初始化角色权限映射失败", "roleCode", roleCode, "permissionCode", permissionCode, "error", err.Error())
				return err
			}
		}
	}
	for _, rule := range contracts.PermissionRules {
		record := ConsoleRoutePermissionModel{
			PathPrefix:     rule.Prefix,
			PathSuffix:     rule.Suffix,
			Method:         rule.Method,
			PermissionCode: string(rule.Permission),
			Sort:           routePermissionSort(rule.Prefix, rule.Suffix),
			Enable:         true,
			DelFlag:        false,
			CreatedTime:    now,
			UpdatedTime:    now,
		}
		if err := r.DB.WithContext(ctx).
			Where("path_prefix = ? AND path_suffix = ? AND method = ?", rule.Prefix, rule.Suffix, strings.ToUpper(rule.Method)).
			Assign(record).
			FirstOrCreate(&ConsoleRoutePermissionModel{}).Error; err != nil {
			r.logger().Error("初始化路由权限映射失败", "prefix", rule.Prefix, "suffix", rule.Suffix, "method", rule.Method, "error", err.Error())
			return err
		}
	}
	r.logger().Info("权限和角色表结构及种子数据初始化完成")
	return nil
}

// ResolveLoginPermissions 按角色读取权限码。
func (r *PermissionRepository) ResolveLoginPermissions(ctx context.Context, req authdomain.LoginRequest) ([]string, bool, error) {
	roleID := strings.TrimSpace(req.RoleID)
	if roleID == "" || r == nil || r.DB == nil {
		return nil, false, nil
	}
	if roleID == "super_admin" {
		r.logger().Info("超级管理员登录，直接赋予所有控制台管理权限", "username", req.Username)
		return []string{string(contracts.PermissionConsoleAll)}, true, nil
	}
	var role ConsoleRoleModel
	if err := r.DB.WithContext(ctx).Where("code = ? AND enable = ? AND del_flag = ?", roleID, true, false).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger().Warn("角色权限解析失败：角色不存在或未启用", "roleID", roleID, "username", req.Username)
			return nil, false, nil
		}
		r.logger().Error("角色权限解析数据库查询异常", "roleID", roleID, "error", err.Error())
		return nil, false, err
	}
	var rows []struct {
		PermissionCode string `gorm:"column:permission_code"`
	}
	err := r.DB.WithContext(ctx).
		Table((ConsoleRolePermissionModel{}).TableName()+" AS rp").
		Select("rp.permission_code").
		Joins("INNER JOIN "+(ConsolePermissionModel{}).TableName()+" AS p ON p.code = rp.permission_code AND p.enable = ? AND p.del_flag = ?", true, false).
		Where("rp.role_id = ? AND rp.enable = ? AND rp.del_flag = ?", roleID, true, false).
		Order("rp.permission_code ASC").
		Scan(&rows).Error
	if err != nil {
		r.logger().Error("角色关联权限表联合查询失败", "roleID", roleID, "error", err.Error())
		return nil, false, err
	}
	if len(rows) == 0 {
		r.logger().Warn("角色权限解析提示：角色下未绑定任何启用中的权限码", "roleID", roleID)
		return nil, false, nil
	}
	permissions := make([]string, 0, len(rows))
	for _, row := range rows {
		code := strings.TrimSpace(row.PermissionCode)
		if code != "" {
			permissions = append(permissions, code)
		}
	}
	r.logger().Info("角色权限解析成功", "username", req.Username, "roleID", roleID, "permissionCount", len(permissions))
	return permissions, true, nil
}

// RequiredPermissionForRequest 按数据库路由配置读取访问所需权限。
func (r *PermissionRepository) RequiredPermissionForRequest(ctx context.Context, path, method string) (contracts.PermissionCode, bool, error) {
	path = strings.TrimSpace(path)
	method = strings.ToUpper(strings.TrimSpace(method))
	if path == "" || method == "" || r == nil || r.DB == nil {
		return "", false, nil
	}
	var rules []ConsoleRoutePermissionModel
	if err := r.DB.WithContext(ctx).
		Where("enable = ? AND del_flag = ? AND (method = ? OR method = ? OR method = '')", true, false, method, "*").
		Order("sort ASC, LENGTH(path_prefix) DESC, id ASC").
		Find(&rules).Error; err != nil {
		r.logger().Error("匹配路由权限数据库查询异常", "path", path, "method", method, "error", err.Error())
		return "", false, err
	}
	for _, rule := range rules {
		if routePermissionMatches(path, rule.PathPrefix, rule.PathSuffix) {
			r.logger().Debug("路由匹配权限规则成功", "path", path, "method", method, "permission", rule.PermissionCode)
			return contracts.PermissionCode(rule.PermissionCode), true, nil
		}
	}
	return "", false, nil
}

func routePermissionMatches(path, prefix, suffix string) bool {
	prefix = strings.TrimSpace(prefix)
	suffix = strings.TrimSpace(suffix)
	if prefix == "" {
		return false
	}
	if strings.HasSuffix(prefix, "/") || suffix != "" {
		if !strings.HasPrefix(path, prefix) {
			return false
		}
	} else if path != prefix {
		return false
	}
	if suffix != "" && !strings.HasSuffix(path, suffix) {
		return false
	}
	return true
}

func defaultConsolePermissions() []ConsolePermissionModel {
	return []ConsolePermissionModel{
		{Code: string(contracts.PermissionConsoleAll), Name: "全部权限", Module: "console", Description: "管理端全量权限", Enable: true},
		{Code: string(contracts.PermissionOperateMerchantRead), Name: "商户查看", Module: "operate", Description: "商户列表和详情", Enable: true},
		{Code: string(contracts.PermissionOperateMerchantWrite), Name: "商户编辑", Module: "operate", Description: "商户新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperateMerchantDelete), Name: "商户删除", Module: "operate", Description: "商户删除", Enable: true},
		{Code: string(contracts.PermissionOperateMerchantToggle), Name: "商户启停", Module: "operate", Description: "商户启用和停用", Enable: true},
		{Code: string(contracts.PermissionOperateRateRead), Name: "费率查看", Module: "operate", Description: "费率列表和详情", Enable: true},
		{Code: string(contracts.PermissionOperateRateWrite), Name: "费率编辑", Module: "operate", Description: "费率新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperateRateDelete), Name: "费率删除", Module: "operate", Description: "费率删除", Enable: true},
		{Code: string(contracts.PermissionOperateBlacklistRead), Name: "黑名单查看", Module: "operate", Description: "黑名单列表和详情", Enable: true},
		{Code: string(contracts.PermissionOperateBlacklistWrite), Name: "黑名单编辑", Module: "operate", Description: "黑名单新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperateBlacklistDelete), Name: "黑名单删除", Module: "operate", Description: "黑名单删除", Enable: true},
		{Code: string(contracts.PermissionOperateWhitelistRead), Name: "白名单查看", Module: "operate", Description: "白名单列表和详情", Enable: true},
		{Code: string(contracts.PermissionOperateWhitelistWrite), Name: "白名单编辑", Module: "operate", Description: "白名单新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperateWhitelistDelete), Name: "白名单删除", Module: "operate", Description: "白名单删除", Enable: true},
		{Code: string(contracts.PermissionOperateBillingRead), Name: "账务查看", Module: "operate", Description: "商户账务总览和充值记录查看", Enable: true},
		{Code: string(contracts.PermissionOperateBillingWrite), Name: "账务编辑", Module: "operate", Description: "商户账务配置和余额调整", Enable: true},
		{Code: string(contracts.PermissionOperateAccountRead), Name: "平台账号查看", Module: "operate", Description: "平台和商户账号列表、详情", Enable: true},
		{Code: string(contracts.PermissionOperateAccountWrite), Name: "平台账号编辑", Module: "operate", Description: "运营账号、商户管理员和商户用户新增更新", Enable: true},
		{Code: string(contracts.PermissionOperateAccountDelete), Name: "平台账号删除", Module: "operate", Description: "运营账号、商户管理员和商户用户删除", Enable: true},
		{Code: string(contracts.PermissionOperateAccountToggle), Name: "平台账号启停", Module: "operate", Description: "运营账号、商户管理员和商户用户启用停用", Enable: true},
		{Code: string(contracts.PermissionOperateAccountPassword), Name: "平台账号重置密码", Module: "operate", Description: "运营账号、商户管理员和商户用户重置密码", Enable: true},
		{Code: string(contracts.PermissionOperateFreeSwitchRead), Name: "FS查看", Module: "operate", Description: "FreeSWITCH 节点查看", Enable: true},
		{Code: string(contracts.PermissionOperateFreeSwitchWrite), Name: "FS编辑", Module: "operate", Description: "FreeSWITCH 节点新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperateFreeSwitchDelete), Name: "FS删除", Module: "operate", Description: "FreeSWITCH 节点删除", Enable: true},
		{Code: string(contracts.PermissionOperateFreeSwitchToggle), Name: "FS启停", Module: "operate", Description: "FreeSWITCH 节点启用和停用", Enable: true},
		{Code: string(contracts.PermissionOperateGatewayRead), Name: "网关查看", Module: "operate", Description: "网关列表和详情", Enable: true},
		{Code: string(contracts.PermissionOperateGatewayWrite), Name: "网关编辑", Module: "operate", Description: "网关新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperateGatewayDelete), Name: "网关删除", Module: "operate", Description: "网关删除", Enable: true},
		{Code: string(contracts.PermissionOperateGatewaySync), Name: "网关同步", Module: "operate", Description: "网关运行时同步", Enable: true},
		{Code: string(contracts.PermissionOperateChannelRead), Name: "渠道查看", Module: "operate", Description: "渠道列表和详情", Enable: true},
		{Code: string(contracts.PermissionOperateChannelWrite), Name: "渠道编辑", Module: "operate", Description: "渠道新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperateChannelDelete), Name: "渠道删除", Module: "operate", Description: "渠道删除", Enable: true},
		{Code: string(contracts.PermissionOperateExtensionRead), Name: "分机查看", Module: "operate", Description: "分机列表和详情", Enable: true},
		{Code: string(contracts.PermissionOperateExtensionWrite), Name: "分机编辑", Module: "operate", Description: "分机新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperateExtensionDelete), Name: "分机删除", Module: "operate", Description: "分机删除", Enable: true},
		{Code: string(contracts.PermissionOperateExtensionToggle), Name: "分机启停", Module: "operate", Description: "分机启用和停用", Enable: true},
		{Code: string(contracts.PermissionOperatePoolRead), Name: "号码池查看", Module: "operate", Description: "号码池列表和详情", Enable: true},
		{Code: string(contracts.PermissionOperatePoolWrite), Name: "号码池编辑", Module: "operate", Description: "号码池新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperatePoolDelete), Name: "号码池删除", Module: "operate", Description: "号码池删除", Enable: true},
		{Code: string(contracts.PermissionOperatePhoneRead), Name: "号码查看", Module: "operate", Description: "号码列表和详情", Enable: true},
		{Code: string(contracts.PermissionOperatePhoneWrite), Name: "号码编辑", Module: "operate", Description: "号码新增和更新", Enable: true},
		{Code: string(contracts.PermissionOperatePhoneDelete), Name: "号码删除", Module: "operate", Description: "号码删除", Enable: true},
		{Code: string(contracts.PermissionMerchantBatchTaskRead), Name: "批量任务查看", Module: "merchant", Description: "批量任务查看", Enable: true},
		{Code: string(contracts.PermissionMerchantBatchTaskWrite), Name: "批量任务编辑", Module: "merchant", Description: "批量任务新增和更新", Enable: true},
		{Code: string(contracts.PermissionMerchantBatchTaskDelete), Name: "批量任务删除", Module: "merchant", Description: "批量任务删除", Enable: true},
		{Code: string(contracts.PermissionMerchantBatchTaskToggle), Name: "批量任务启停", Module: "merchant", Description: "批量任务启用和停用", Enable: true},
		{Code: string(contracts.PermissionMerchantDialpadRead), Name: "拨号盘查看", Module: "merchant", Description: "拨号盘查看", Enable: true},
		{Code: string(contracts.PermissionMerchantDialpadControl), Name: "拨号盘控制", Module: "merchant", Description: "拨号盘启动、暂停、恢复", Enable: true},
		{Code: string(contracts.PermissionMerchantCallRecordRead), Name: "呼叫记录查看", Module: "merchant", Description: "呼叫记录查看", Enable: true},
		{Code: string(contracts.PermissionMerchantAccountRead), Name: "商户账号查看", Module: "merchant", Description: "本商户用户账号列表、详情", Enable: true},
		{Code: string(contracts.PermissionMerchantAccountWrite), Name: "商户账号编辑", Module: "merchant", Description: "本商户用户账号新增更新", Enable: true},
		{Code: string(contracts.PermissionMerchantAccountDelete), Name: "商户账号删除", Module: "merchant", Description: "本商户用户账号删除", Enable: true},
		{Code: string(contracts.PermissionMerchantAccountToggle), Name: "商户账号启停", Module: "merchant", Description: "本商户用户账号启用停用", Enable: true},
		{Code: string(contracts.PermissionMerchantAccountPassword), Name: "商户账号重置密码", Module: "merchant", Description: "本商户用户账号重置密码", Enable: true},
		{Code: string(contracts.PermissionMerchantAIFlowRead), Name: "AI流程查看", Module: "merchant", Description: "AI 流程查看", Enable: true},
		{Code: string(contracts.PermissionMerchantAIFlowWrite), Name: "AI流程编辑", Module: "merchant", Description: "AI 流程新增和更新", Enable: true},
		{Code: string(contracts.PermissionMerchantAIFlowDelete), Name: "AI流程删除", Module: "merchant", Description: "AI 流程删除", Enable: true},
		{Code: string(contracts.PermissionMerchantAIFlowPrecheck), Name: "AI流程预检", Module: "merchant", Description: "AI 流程预检", Enable: true},
		{Code: string(contracts.PermissionMerchantAIFlowPublish), Name: "AI流程发布", Module: "merchant", Description: "AI 流程发布", Enable: true},
		{Code: string(contracts.PermissionMerchantSkillGroupRead), Name: "技能组查看", Module: "merchant", Description: "技能组列表和详情", Enable: true},
		{Code: string(contracts.PermissionMerchantSkillGroupWrite), Name: "技能组编辑", Module: "merchant", Description: "技能组新增、更新和关联操作", Enable: true},
		{Code: string(contracts.PermissionMerchantSkillGroupDelete), Name: "技能组删除", Module: "merchant", Description: "技能组删除", Enable: true},
		{Code: string(contracts.PermissionMerchantPhoneGroupRead), Name: "号码组查看", Module: "merchant", Description: "号码组列表和详情", Enable: true},
		{Code: string(contracts.PermissionMerchantPhoneGroupWrite), Name: "号码组编辑", Module: "merchant", Description: "号码组新增、更新和关联操作", Enable: true},
		{Code: string(contracts.PermissionMerchantPhoneGroupDelete), Name: "号码组删除", Module: "merchant", Description: "号码组删除", Enable: true},
		{Code: string(contracts.PermissionMerchantDepartmentRead), Name: "部门查看", Module: "merchant", Description: "部门列表和详情", Enable: true},
		{Code: string(contracts.PermissionMerchantDepartmentWrite), Name: "部门编辑", Module: "merchant", Description: "部门新增和更新", Enable: true},
		{Code: string(contracts.PermissionMerchantDepartmentDelete), Name: "部门删除", Module: "merchant", Description: "部门删除", Enable: true},

		{Code: string(contracts.PermissionOperateRoleRead), Name: "角色权限查看", Module: "operate", Description: "管理端角色列表和权限配置查看", Enable: true},
		{Code: string(contracts.PermissionOperateRoleWrite), Name: "角色权限编辑", Module: "operate", Description: "管理端角色和权限配置修改", Enable: true},
	}
}

func defaultConsoleRoles() []ConsoleRoleModel {
	return []ConsoleRoleModel{
		{Code: "super_admin", Name: "超级管理员", Description: "拥有全部权限", Enable: true},
		{Code: "operate_lead", Name: "运营主管", Description: "维护商户、商户账号和运营配置", Enable: true},
		{Code: "operate_staff", Name: "运营专员", Description: "按授权创建商户并维护商户账号", Enable: true},
		{Code: "merchant_admin", Name: "商户管理员", Description: "商户唯一管理员，可维护本商户用户", Enable: true},
		{Code: "merchant_user", Name: "商户用户", Description: "商户实际使用账号，默认只读业务数据", Enable: true},
	}
}

func defaultConsoleRolePermissions() map[string][]string {
	all := make([]string, 0, len(defaultConsolePermissions()))
	for _, permission := range defaultConsolePermissions() {
		if permission.Code == string(contracts.PermissionConsoleAll) {
			continue
		}
		all = append(all, permission.Code)
	}
	return map[string][]string{
		"operate_lead":   defaultOperateLeadPermissions(all),
		"operate_staff":  defaultOperateStaffPermissions(),
		"merchant_admin": defaultMerchantAdminPermissions(),
		"merchant_user":  defaultMerchantUserPermissions(),
	}
}

func defaultOperateLeadPermissions(all []string) []string {
	out := make([]string, 0, len(all))
	for _, permission := range all {
		if strings.HasPrefix(permission, "merchant:account:") {
			out = append(out, permission)
			continue
		}
		if strings.HasPrefix(permission, "merchant:") {
			continue
		}
		out = append(out, permission)
	}
	return out
}

func defaultOperateStaffPermissions() []string {
	return []string{
		string(contracts.PermissionOperateMerchantRead),
		string(contracts.PermissionOperateMerchantWrite),
		string(contracts.PermissionOperateAccountRead),
		string(contracts.PermissionOperateAccountWrite),
		string(contracts.PermissionOperateAccountToggle),
		string(contracts.PermissionOperateAccountPassword),
	}
}

func defaultMerchantAdminPermissions() []string {
	return []string{
		string(contracts.PermissionMerchantAccountRead),
		string(contracts.PermissionMerchantAccountWrite),
		string(contracts.PermissionMerchantAccountDelete),
		string(contracts.PermissionMerchantAccountToggle),
		string(contracts.PermissionMerchantAccountPassword),
		string(contracts.PermissionMerchantBatchTaskRead),
		string(contracts.PermissionMerchantBatchTaskWrite),
		string(contracts.PermissionMerchantBatchTaskDelete),
		string(contracts.PermissionMerchantBatchTaskToggle),
		string(contracts.PermissionMerchantDialpadRead),
		string(contracts.PermissionMerchantDialpadControl),
		string(contracts.PermissionMerchantCallRecordRead),
		string(contracts.PermissionMerchantAIFlowRead),
		string(contracts.PermissionMerchantAIFlowWrite),
		string(contracts.PermissionMerchantAIFlowDelete),
		string(contracts.PermissionMerchantAIFlowPrecheck),
		string(contracts.PermissionMerchantAIFlowPublish),
		string(contracts.PermissionMerchantSkillGroupRead),
		string(contracts.PermissionMerchantSkillGroupWrite),
		string(contracts.PermissionMerchantSkillGroupDelete),
		string(contracts.PermissionMerchantPhoneGroupRead),
		string(contracts.PermissionMerchantPhoneGroupWrite),
		string(contracts.PermissionMerchantPhoneGroupDelete),
		string(contracts.PermissionMerchantDepartmentRead),
		string(contracts.PermissionMerchantDepartmentWrite),
		string(contracts.PermissionMerchantDepartmentDelete),
	}
}

func defaultMerchantUserPermissions() []string {
	return []string{
		string(contracts.PermissionMerchantBatchTaskRead),
		string(contracts.PermissionMerchantDialpadRead),
		string(contracts.PermissionMerchantCallRecordRead),
		string(contracts.PermissionMerchantAIFlowRead),
		string(contracts.PermissionMerchantSkillGroupRead),
		string(contracts.PermissionMerchantPhoneGroupRead),
		string(contracts.PermissionMerchantDepartmentRead),
	}
}

func routePermissionSort(prefix, suffix string) int {
	score := len(prefix) * 10
	if suffix != "" {
		score++
	}
	return score
}

// RolePageRequest 表示角色分页查询条件。
type RolePageRequest struct {
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Name       string `json:"name,omitempty"`
}

// RolePageResult 表示角色分页结果。
type RolePageResult struct {
	PageNumber int                `json:"pageNumber"`
	PageSize   int                `json:"pageSize"`
	Total      int64              `json:"total"`
	Records    []ConsoleRoleModel `json:"records"`
}

// PageRoles 分页查询角色。
func (r *PermissionRepository) PageRoles(ctx context.Context, req RolePageRequest) (RolePageResult, error) {
	if req.PageNumber <= 0 {
		req.PageNumber = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	query := r.DB.WithContext(ctx).Model(&ConsoleRoleModel{}).Where("del_flag = ?", false)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return RolePageResult{}, err
	}
	var records []ConsoleRoleModel
	offset := (req.PageNumber - 1) * req.PageSize
	if err := query.Order("code ASC").Offset(offset).Limit(req.PageSize).Find(&records).Error; err != nil {
		return RolePageResult{}, err
	}
	return RolePageResult{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Total:      total,
		Records:    records,
	}, nil
}

// SaveRole 保存角色。
func (r *PermissionRepository) SaveRole(ctx context.Context, role ConsoleRoleModel) (ConsoleRoleModel, error) {
	if role.Code == "" || role.Name == "" {
		return ConsoleRoleModel{}, errors.New("角色编码和角色名称不能为空")
	}
	r.logger().Info("开始保存角色配置", "roleCode", role.Code, "roleName", role.Name)
	var existing ConsoleRoleModel
	err := r.DB.WithContext(ctx).Where("code = ? AND del_flag = ?", role.Code, false).First(&existing).Error
	now := time.Now().UTC()
	role.UpdatedTime = now
	if err == nil {
		// Update
		role.CreatedTime = existing.CreatedTime
		if err := r.DB.WithContext(ctx).Model(&ConsoleRoleModel{}).Where("code = ?", role.Code).Updates(role).Error; err != nil {
			r.logger().Error("更新角色配置失败", "roleCode", role.Code, "error", err.Error())
			return ConsoleRoleModel{}, err
		}
		r.logger().Info("更新角色配置成功", "roleCode", role.Code)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create
		role.CreatedTime = now
		role.DelFlag = false
		if err := r.DB.WithContext(ctx).Create(&role).Error; err != nil {
			r.logger().Error("创建新角色失败", "roleCode", role.Code, "error", err.Error())
			return ConsoleRoleModel{}, err
		}
		r.logger().Info("创建新角色成功", "roleCode", role.Code)
	} else {
		r.logger().Error("保存角色查询数据库异常", "roleCode", role.Code, "error", err.Error())
		return ConsoleRoleModel{}, err
	}
	return role, nil
}

// DeleteRoles 逻辑删除角色。
func (r *PermissionRepository) DeleteRoles(ctx context.Context, codes []string) error {
	if len(codes) == 0 {
		return nil
	}
	r.logger().Info("开始逻辑删除角色", "roleCodes", codes)
	for _, code := range codes {
		if code == "super_admin" || code == "merchant_admin" || code == "merchant_user" {
			r.logger().Warn("逻辑删除角色拦截：默认核心角色不允许删除", "roleCode", code)
			return errors.New("默认核心角色不允许删除")
		}
	}
	err := r.DB.WithContext(ctx).Model(&ConsoleRoleModel{}).
		Where("code IN ?", codes).
		Updates(map[string]any{"del_flag": true, "updated_time": time.Now().UTC()}).Error
	if err != nil {
		r.logger().Error("逻辑删除角色失败", "roleCodes", codes, "error", err.Error())
		return err
	}
	r.logger().Info("逻辑删除角色成功", "roleCodes", codes)
	return nil
}

// ListPermissions 获取所有启用权限。
func (r *PermissionRepository) ListPermissions(ctx context.Context) ([]ConsolePermissionModel, error) {
	var list []ConsolePermissionModel
	err := r.DB.WithContext(ctx).Where("enable = ? AND del_flag = ?", true, false).Order("module ASC, code ASC").Find(&list).Error
	return list, err
}

// GetRolePermissions 获取角色的权限编码列表。
func (r *PermissionRepository) GetRolePermissions(ctx context.Context, roleCode string) ([]string, error) {
	var rpList []ConsoleRolePermissionModel
	err := r.DB.WithContext(ctx).Where("role_id = ? AND enable = ? AND del_flag = ?", roleCode, true, false).Find(&rpList).Error
	if err != nil {
		return nil, err
	}
	codes := make([]string, len(rpList))
	for i, rp := range rpList {
		codes[i] = rp.PermissionCode
	}
	return codes, nil
}

// SaveRolePermissions 保存角色的权限编码映射。
func (r *PermissionRepository) SaveRolePermissions(ctx context.Context, roleCode string, permissionCodes []string) error {
	if roleCode == "" {
		return errors.New("角色编码不能为空")
	}
	if roleCode == "super_admin" {
		r.logger().Warn("保存角色权限拦截：超级管理员不允许手动分配权限")
		return errors.New("超级管理员角色不需要手动分配权限")
	}
	r.logger().Info("开始保存角色权限关联映射", "roleCode", roleCode, "permissionCount", len(permissionCodes))
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleCode).Delete(&ConsoleRolePermissionModel{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, pCode := range permissionCodes {
			rp := ConsoleRolePermissionModel{
				RoleID:         roleCode,
				PermissionCode: pCode,
				Enable:         true,
				DelFlag:        false,
				CreatedTime:    now,
				UpdatedTime:    now,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		r.logger().Error("保存角色权限关联映射失败", "roleCode", roleCode, "error", err.Error())
		return err
	}
	r.logger().Info("保存角色权限关联映射成功", "roleCode", roleCode, "permissionCount", len(permissionCodes))
	return nil
}
