package contracts

import (
	"context"
	"strings"
)

type TenantContext struct {
	MerchantID  string   `json:"merchantId,omitempty"`
	UserID      string   `json:"userId,omitempty"`
	RoleID      string   `json:"roleId,omitempty"`
	DataScope   string   `json:"dataScope,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Internal    bool     `json:"internal"`
}

type tenantContextKey struct{}

// WithTenant 把租户和权限上下文写入 context。
// HTTP、MQ、定时任务和补偿命令都应显式携带租户上下文，避免领域层从传输细节反推权限。
func WithTenant(ctx context.Context, tenant TenantContext) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenant)
}

// TenantFromContext 从 context 中读取租户上下文。
func TenantFromContext(ctx context.Context) (TenantContext, bool) {
	tenant, ok := ctx.Value(tenantContextKey{}).(TenantContext)
	return tenant, ok
}

// HasPermission 判断当前租户上下文是否拥有指定功能权限。
// Internal 角色默认拥有全部权限，权限字符串支持以 `*` 结尾的前缀通配。
func (t TenantContext) HasPermission(required string) bool {
	if t.RoleID == "super_admin" {
		return true
	}
	required = normalizePermission(required)
	if required == "" {
		return false
	}
	for _, granted := range t.Permissions {
		granted = normalizePermission(granted)
		if granted == "" {
			continue
		}
		if granted == "*" || granted == "console:*" {
			return true
		}
		if granted == required {
			return true
		}
		if len(granted) > 0 && granted[len(granted)-1] == '*' {
			if len(granted) == 1 {
				return true
			}
			prefix := granted[:len(granted)-1]
			if len(required) >= len(prefix) && required[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

func normalizePermission(raw string) string {
	return strings.TrimSpace(raw)
}
