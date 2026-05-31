package contracts

import "strings"

// PermissionCode 表示管理端功能权限码。
//
// 权限采用 `domain:resource:action` 风格，并支持以 `*` 结尾的前缀通配。
type PermissionCode string

const (
	// PermissionConsoleAll 表示管理端全量权限。
	PermissionConsoleAll PermissionCode = "console:*"

	PermissionOperateMerchantRead      PermissionCode = "operate:merchant:read"
	PermissionOperateMerchantWrite     PermissionCode = "operate:merchant:write"
	PermissionOperateMerchantDelete    PermissionCode = "operate:merchant:delete"
	PermissionOperateMerchantToggle    PermissionCode = "operate:merchant:toggle"
	PermissionOperateRateRead          PermissionCode = "operate:rate:read"
	PermissionOperateRateWrite         PermissionCode = "operate:rate:write"
	PermissionOperateRateDelete        PermissionCode = "operate:rate:delete"
	PermissionOperateBlacklistRead     PermissionCode = "operate:blacklist:read"
	PermissionOperateBlacklistWrite    PermissionCode = "operate:blacklist:write"
	PermissionOperateBlacklistDelete   PermissionCode = "operate:blacklist:delete"
	PermissionOperateWhitelistRead     PermissionCode = "operate:whitelist:read"
	PermissionOperateWhitelistWrite    PermissionCode = "operate:whitelist:write"
	PermissionOperateWhitelistDelete   PermissionCode = "operate:whitelist:delete"
	PermissionOperateRiskControlRead   PermissionCode = "operate:riskcontrol:read"
	PermissionOperateRiskControlWrite  PermissionCode = "operate:riskcontrol:write"
	PermissionOperateRiskControlDelete PermissionCode = "operate:riskcontrol:delete"
	PermissionOperateBillingRead       PermissionCode = "operate:billing:read"
	PermissionOperateBillingWrite      PermissionCode = "operate:billing:write"
	PermissionOperateAccountRead       PermissionCode = "operate:account:read"
	PermissionOperateAccountWrite      PermissionCode = "operate:account:write"
	PermissionOperateAccountDelete     PermissionCode = "operate:account:delete"
	PermissionOperateAccountToggle     PermissionCode = "operate:account:toggle"
	PermissionOperateAccountPassword   PermissionCode = "operate:account:reset-password"
	PermissionOperateFreeSwitchRead    PermissionCode = "operate:freeswitch:read"
	PermissionOperateFreeSwitchWrite   PermissionCode = "operate:freeswitch:write"
	PermissionOperateFreeSwitchDelete  PermissionCode = "operate:freeswitch:delete"
	PermissionOperateFreeSwitchToggle  PermissionCode = "operate:freeswitch:toggle"
	PermissionOperateGatewayRead       PermissionCode = "operate:gateway:read"
	PermissionOperateGatewayWrite      PermissionCode = "operate:gateway:write"
	PermissionOperateGatewayDelete     PermissionCode = "operate:gateway:delete"
	PermissionOperateGatewaySync       PermissionCode = "operate:gateway:sync"
	PermissionOperateChannelRead       PermissionCode = "operate:channel:read"
	PermissionOperateChannelWrite      PermissionCode = "operate:channel:write"
	PermissionOperateChannelDelete     PermissionCode = "operate:channel:delete"
	PermissionOperateExtensionRead     PermissionCode = "operate:extension:read"
	PermissionOperateExtensionWrite    PermissionCode = "operate:extension:write"
	PermissionOperateExtensionDelete   PermissionCode = "operate:extension:delete"
	PermissionOperateExtensionToggle   PermissionCode = "operate:extension:toggle"
	PermissionOperatePoolRead          PermissionCode = "operate:pool:read"
	PermissionOperatePoolWrite         PermissionCode = "operate:pool:write"
	PermissionOperatePoolDelete        PermissionCode = "operate:pool:delete"
	PermissionOperatePhoneRead         PermissionCode = "operate:phone:read"
	PermissionOperatePhoneWrite        PermissionCode = "operate:phone:write"
	PermissionOperatePhoneDelete       PermissionCode = "operate:phone:delete"
	PermissionMerchantBatchTaskRead    PermissionCode = "merchant:batch-task:read"
	PermissionMerchantBatchTaskWrite   PermissionCode = "merchant:batch-task:write"
	PermissionMerchantBatchTaskDelete  PermissionCode = "merchant:batch-task:delete"
	PermissionMerchantBatchTaskToggle  PermissionCode = "merchant:batch-task:toggle"
	PermissionMerchantDialpadRead      PermissionCode = "merchant:batch-dialpad:read"
	PermissionMerchantDialpadControl   PermissionCode = "merchant:batch-dialpad:control"
	PermissionMerchantCallRecordRead   PermissionCode = "merchant:call-record:read"
	PermissionMerchantAccountRead      PermissionCode = "merchant:account:read"
	PermissionMerchantAccountWrite     PermissionCode = "merchant:account:write"
	PermissionMerchantAccountDelete    PermissionCode = "merchant:account:delete"
	PermissionMerchantAccountToggle    PermissionCode = "merchant:account:toggle"
	PermissionMerchantAccountPassword  PermissionCode = "merchant:account:reset-password"
	PermissionMerchantAIFlowRead       PermissionCode = "merchant:ai-flow:read"
	PermissionMerchantAIFlowWrite      PermissionCode = "merchant:ai-flow:write"
	PermissionMerchantAIFlowDelete     PermissionCode = "merchant:ai-flow:delete"
	PermissionMerchantAIFlowPrecheck   PermissionCode = "merchant:ai-flow:precheck"
	PermissionMerchantAIFlowPublish    PermissionCode = "merchant:ai-flow:publish"
	PermissionMerchantSkillGroupRead   PermissionCode = "merchant:skill-group:read"
	PermissionMerchantSkillGroupWrite  PermissionCode = "merchant:skill-group:write"
	PermissionMerchantSkillGroupDelete PermissionCode = "merchant:skill-group:delete"
	PermissionMerchantPhoneGroupRead   PermissionCode = "merchant:phone-group:read"
	PermissionMerchantPhoneGroupWrite  PermissionCode = "merchant:phone-group:write"
	PermissionMerchantPhoneGroupDelete PermissionCode = "merchant:phone-group:delete"

	PermissionOperateRoleRead      PermissionCode = "operate:role:read"
	PermissionOperateRoleWrite     PermissionCode = "operate:role:write"
	PermissionMerchantBillingWrite PermissionCode = "merchant:billing:write"
)

// PermissionRule 定义一个路径和 HTTP 方法对应的功能权限。
type PermissionRule struct {
	Prefix     string
	Suffix     string
	Method     string
	Permission PermissionCode
}

// PermissionRules 是管理端功能级权限的静态路由映射。
// 新增管理路由时必须同步补充此列表，否则 middleware 会按失败关闭返回 403。
var PermissionRules = []PermissionRule{
	{Prefix: "/operate/role/page", Method: "POST", Permission: PermissionOperateRoleRead},
	{Prefix: "/operate/role/detail/", Method: "GET", Permission: PermissionOperateRoleRead},
	{Prefix: "/operate/role/add", Method: "PUT", Permission: PermissionOperateRoleWrite},
	{Prefix: "/operate/role/update", Method: "POST", Permission: PermissionOperateRoleWrite},
	{Prefix: "/operate/role/delete", Method: "POST", Permission: PermissionOperateRoleWrite},
	{Prefix: "/operate/role/enable/", Method: "POST", Permission: PermissionOperateRoleWrite},
	{Prefix: "/operate/role/disable/", Method: "POST", Permission: PermissionOperateRoleWrite},
	{Prefix: "/operate/role/permissions/", Method: "GET", Permission: PermissionOperateRoleRead},
	{Prefix: "/operate/role/permissions/save", Method: "POST", Permission: PermissionOperateRoleWrite},
	{Prefix: "/operate/permission", Method: "GET", Permission: PermissionOperateRoleRead},
	{Prefix: "/merchant/billing/rate/bind", Method: "POST", Permission: PermissionMerchantBillingWrite},
	{Prefix: "/operate/rate/list-active", Method: "GET", Permission: PermissionMerchantBillingWrite},

	{Prefix: "/operate/merchant/page", Method: "POST", Permission: PermissionOperateMerchantRead},
	{Prefix: "/operate/merchant/detail/", Method: "GET", Permission: PermissionOperateMerchantRead},
	{Prefix: "/operate/merchant/enable/", Method: "POST", Permission: PermissionOperateMerchantToggle},
	{Prefix: "/operate/merchant/disable/", Method: "POST", Permission: PermissionOperateMerchantToggle},
	{Prefix: "/operate/merchant/delete", Method: "POST", Permission: PermissionOperateMerchantDelete},
	{Prefix: "/operate/merchant/add", Method: "PUT", Permission: PermissionOperateMerchantWrite},
	{Prefix: "/operate/merchant/update", Method: "POST", Permission: PermissionOperateMerchantWrite},
	{Prefix: "/operate/merchant", Method: "GET", Permission: PermissionOperateMerchantRead},
	{Prefix: "/operate/rate/page", Method: "POST", Permission: PermissionOperateRateRead},
	{Prefix: "/operate/rate/detail/", Method: "GET", Permission: PermissionOperateRateRead},
	{Prefix: "/operate/rate/delete", Method: "POST", Permission: PermissionOperateRateDelete},
	{Prefix: "/operate/rate/add", Method: "PUT", Permission: PermissionOperateRateWrite},
	{Prefix: "/operate/rate/update", Method: "POST", Permission: PermissionOperateRateWrite},
	{Prefix: "/operate/rate", Method: "GET", Permission: PermissionOperateRateRead},
	{Prefix: "/operate/blacklist/page", Method: "POST", Permission: PermissionOperateBlacklistRead},
	{Prefix: "/operate/blacklist/detail/", Method: "GET", Permission: PermissionOperateBlacklistRead},
	{Prefix: "/operate/blacklist/delete/", Method: "POST", Permission: PermissionOperateBlacklistDelete},
	{Prefix: "/operate/blacklist/add", Method: "PUT", Permission: PermissionOperateBlacklistWrite},
	{Prefix: "/operate/blacklist/update", Method: "POST", Permission: PermissionOperateBlacklistWrite},
	{Prefix: "/operate/blacklist/numbers/page", Method: "POST", Permission: PermissionOperateBlacklistRead},
	{Prefix: "/operate/blacklist/numbers/save", Method: "POST", Permission: PermissionOperateBlacklistWrite},
	{Prefix: "/operate/blacklist/numbers/delete", Method: "POST", Permission: PermissionOperateBlacklistDelete},
	{Prefix: "/operate/blacklist/channels/save", Method: "POST", Permission: PermissionOperateBlacklistWrite},
	{Prefix: "/operate/blacklist/channels/", Method: "DELETE", Permission: PermissionOperateBlacklistWrite},
	{Prefix: "/operate/blacklist/channels", Method: "GET", Permission: PermissionOperateBlacklistRead},
	{Prefix: "/operate/blacklist", Method: "GET", Permission: PermissionOperateBlacklistRead},
	{Prefix: "/operate/risk-control/page", Method: "POST", Permission: PermissionOperateRiskControlRead},
	{Prefix: "/operate/risk-control/detail/", Method: "GET", Permission: PermissionOperateRiskControlRead},
	{Prefix: "/operate/risk-control/delete", Method: "POST", Permission: PermissionOperateRiskControlDelete},
	{Prefix: "/operate/risk-control/add", Method: "PUT", Permission: PermissionOperateRiskControlWrite},
	{Prefix: "/operate/risk-control/update", Method: "POST", Permission: PermissionOperateRiskControlWrite},
	{Prefix: "/operate/risk-control/merchants/", Method: "GET", Permission: PermissionOperateRiskControlRead},
	{Prefix: "/operate/risk-control/merchants/", Method: "POST", Permission: PermissionOperateRiskControlWrite},
	{Prefix: "/operate/whitelist/page", Method: "POST", Permission: PermissionOperateWhitelistRead},
	{Prefix: "/operate/whitelist/detail/", Method: "GET", Permission: PermissionOperateWhitelistRead},
	{Prefix: "/operate/whitelist/delete", Method: "POST", Permission: PermissionOperateWhitelistDelete},
	{Prefix: "/operate/whitelist/add", Method: "PUT", Permission: PermissionOperateWhitelistWrite},
	{Prefix: "/operate/whitelist/update", Method: "POST", Permission: PermissionOperateWhitelistWrite},
	{Prefix: "/operate/whitelist", Method: "GET", Permission: PermissionOperateWhitelistRead},
	{Prefix: "/operate/billing/overview/page", Method: "POST", Permission: PermissionOperateBillingRead},
	{Prefix: "/operate/billing/overview/save", Method: "POST", Permission: PermissionOperateBillingWrite},
	{Prefix: "/operate/billing/recharge", Method: "POST", Permission: PermissionOperateBillingWrite},
	{Prefix: "/operate/billing/recharge-records", Method: "POST", Permission: PermissionOperateBillingRead},
	{Prefix: "/operate/account/page", Method: "POST", Permission: PermissionOperateAccountRead},
	{Prefix: "/operate/account/detail/", Method: "GET", Permission: PermissionOperateAccountRead},
	{Prefix: "/operate/account/enable/", Method: "POST", Permission: PermissionOperateAccountToggle},
	{Prefix: "/operate/account/disable/", Method: "POST", Permission: PermissionOperateAccountToggle},
	{Prefix: "/operate/account/reset-password/", Method: "POST", Permission: PermissionOperateAccountPassword},
	{Prefix: "/operate/account/delete", Method: "POST", Permission: PermissionOperateAccountDelete},
	{Prefix: "/operate/account/add", Method: "PUT", Permission: PermissionOperateAccountWrite},
	{Prefix: "/operate/account/update", Method: "POST", Permission: PermissionOperateAccountWrite},
	{Prefix: "/operate/account", Method: "GET", Permission: PermissionOperateAccountRead},
	{Prefix: "/operate/freeswitch/list", Method: "GET", Permission: PermissionOperateFreeSwitchRead},
	{Prefix: "/operate/freeswitch/detail/", Method: "GET", Permission: PermissionOperateFreeSwitchRead},
	{Prefix: "/operate/freeswitch", Method: "POST", Permission: PermissionOperateFreeSwitchWrite},
	{Prefix: "/operate/freeswitch/", Method: "PUT", Permission: PermissionOperateFreeSwitchWrite},
	{Prefix: "/operate/freeswitch/", Suffix: "/enable", Method: "POST", Permission: PermissionOperateFreeSwitchToggle},
	{Prefix: "/operate/freeswitch/", Suffix: "/disable", Method: "POST", Permission: PermissionOperateFreeSwitchToggle},
	{Prefix: "/operate/freeswitch/", Method: "DELETE", Permission: PermissionOperateFreeSwitchDelete},
	{Prefix: "/operate/system-config", Method: "GET", Permission: PermissionOperateFreeSwitchRead},
	{Prefix: "/operate/system-config/save", Method: "POST", Permission: PermissionOperateFreeSwitchWrite},
	{Prefix: "/operate/system-config/apply", Method: "POST", Permission: PermissionOperateFreeSwitchWrite},
	{Prefix: "/operate/system-config/reload-rtp", Method: "POST", Permission: PermissionOperateFreeSwitchWrite},
	{Prefix: "/operate/gateway/page", Method: "POST", Permission: PermissionOperateGatewayRead},
	{Prefix: "/operate/gateway/detail/", Method: "GET", Permission: PermissionOperateGatewayRead},
	{Prefix: "/operate/gateway/encode", Method: "GET", Permission: PermissionOperateGatewayRead},
	{Prefix: "/operate/gateway", Method: "GET", Permission: PermissionOperateGatewayRead},
	{Prefix: "/operate/gateway/sync/", Method: "POST", Permission: PermissionOperateGatewaySync},
	{Prefix: "/operate/gateway/delete", Method: "POST", Permission: PermissionOperateGatewayDelete},
	{Prefix: "/operate/gateway/add", Method: "PUT", Permission: PermissionOperateGatewayWrite},
	{Prefix: "/operate/gateway/update", Method: "POST", Permission: PermissionOperateGatewayWrite},
	{Prefix: "/operate/channel/page", Method: "POST", Permission: PermissionOperateChannelRead},
	{Prefix: "/operate/channel/detail/", Method: "GET", Permission: PermissionOperateChannelRead},
	{Prefix: "/operate/channel/delete", Method: "POST", Permission: PermissionOperateChannelDelete},
	{Prefix: "/operate/channel/add", Method: "PUT", Permission: PermissionOperateChannelWrite},
	{Prefix: "/operate/channel/update", Method: "POST", Permission: PermissionOperateChannelWrite},
	{Prefix: "/operate/channel", Method: "GET", Permission: PermissionOperateChannelRead},
	{Prefix: "/operate/extension/page", Method: "POST", Permission: PermissionOperateExtensionRead},
	{Prefix: "/operate/extension/detail/", Method: "GET", Permission: PermissionOperateExtensionRead},
	{Prefix: "/operate/extension/enable/", Method: "POST", Permission: PermissionOperateExtensionToggle},
	{Prefix: "/operate/extension/disable/", Method: "POST", Permission: PermissionOperateExtensionToggle},
	{Prefix: "/operate/extension/delete", Method: "POST", Permission: PermissionOperateExtensionDelete},
	{Prefix: "/operate/extension/add", Method: "PUT", Permission: PermissionOperateExtensionWrite},
	{Prefix: "/operate/extension/update", Method: "POST", Permission: PermissionOperateExtensionWrite},
	{Prefix: "/operate/extension/dynamic-bind", Method: "POST", Permission: PermissionOperateExtensionWrite},
	{Prefix: "/operate/extension", Method: "GET", Permission: PermissionOperateExtensionRead},
	{Prefix: "/operate/pool/page", Method: "POST", Permission: PermissionOperatePoolRead},
	{Prefix: "/operate/pool/detail/", Method: "GET", Permission: PermissionOperatePoolRead},
	{Prefix: "/operate/pool/list/", Method: "GET", Permission: PermissionOperatePoolRead},
	{Prefix: "/operate/pool/list", Method: "GET", Permission: PermissionOperatePoolRead},
	{Prefix: "/operate/pool/delete", Method: "POST", Permission: PermissionOperatePoolDelete},
	{Prefix: "/operate/pool/add", Method: "PUT", Permission: PermissionOperatePoolWrite},
	{Prefix: "/operate/pool/update", Method: "POST", Permission: PermissionOperatePoolWrite},
	{Prefix: "/operate/pool", Method: "GET", Permission: PermissionOperatePoolRead},
	{Prefix: "/operate/pool-phone/page", Method: "POST", Permission: PermissionOperatePhoneRead},
	{Prefix: "/operate/pool-phone/detail/", Method: "GET", Permission: PermissionOperatePhoneRead},
	{Prefix: "/operate/pool-phone/delete", Method: "POST", Permission: PermissionOperatePhoneDelete},
	{Prefix: "/operate/pool-phone/enable/", Method: "POST", Permission: PermissionOperatePhoneWrite},
	{Prefix: "/operate/pool-phone/disable/", Method: "POST", Permission: PermissionOperatePhoneWrite},
	{Prefix: "/operate/pool-phone/batch-move", Method: "POST", Permission: PermissionOperatePhoneWrite},
	{Prefix: "/operate/pool-phone/add", Method: "PUT", Permission: PermissionOperatePhoneWrite},
	{Prefix: "/operate/pool-phone/update", Method: "POST", Permission: PermissionOperatePhoneWrite},
	{Prefix: "/operate/pool-phone", Method: "GET", Permission: PermissionOperatePhoneRead},
	{Prefix: "/merchant/batch-call-task/page", Method: "POST", Permission: PermissionMerchantBatchTaskRead},
	{Prefix: "/merchant/batch-call-task/detail/", Method: "GET", Permission: PermissionMerchantBatchTaskRead},
	{Prefix: "/merchant/batch-call-task/enable/", Method: "POST", Permission: PermissionMerchantBatchTaskToggle},
	{Prefix: "/merchant/batch-call-task/disable/", Method: "POST", Permission: PermissionMerchantBatchTaskToggle},
	{Prefix: "/merchant/batch-call-task/delete", Method: "POST", Permission: PermissionMerchantBatchTaskDelete},
	{Prefix: "/merchant/batch-call-task/add", Method: "PUT", Permission: PermissionMerchantBatchTaskWrite},
	{Prefix: "/merchant/batch-call-task/update", Method: "POST", Permission: PermissionMerchantBatchTaskWrite},
	{Prefix: "/merchant/batch-call-task", Method: "GET", Permission: PermissionMerchantBatchTaskRead},
	{Prefix: "/merchant/batch-call-dialpad/detail/", Method: "GET", Permission: PermissionMerchantDialpadRead},
	{Prefix: "/merchant/batch-call-dialpad/start/", Method: "POST", Permission: PermissionMerchantDialpadControl},
	{Prefix: "/merchant/batch-call-dialpad/pause/", Method: "POST", Permission: PermissionMerchantDialpadControl},
	{Prefix: "/merchant/batch-call-dialpad/resume/", Method: "POST", Permission: PermissionMerchantDialpadControl},
	{Prefix: "/merchant/batch-call-dialpad/disconnect-pause/", Method: "POST", Permission: PermissionMerchantDialpadControl},
	{Prefix: "/merchant/call-record/page", Method: "POST", Permission: PermissionMerchantCallRecordRead},
	{Prefix: "/merchant/call-record/detail/", Method: "GET", Permission: PermissionMerchantCallRecordRead},
	{Prefix: "/merchant/call-record", Method: "GET", Permission: PermissionMerchantCallRecordRead},
	{Prefix: "/merchant/account/page", Method: "POST", Permission: PermissionMerchantAccountRead},
	{Prefix: "/merchant/account/detail/", Method: "GET", Permission: PermissionMerchantAccountRead},
	{Prefix: "/merchant/account/enable/", Method: "POST", Permission: PermissionMerchantAccountToggle},
	{Prefix: "/merchant/account/disable/", Method: "POST", Permission: PermissionMerchantAccountToggle},
	{Prefix: "/merchant/account/reset-password/", Method: "POST", Permission: PermissionMerchantAccountPassword},
	{Prefix: "/merchant/account/delete", Method: "POST", Permission: PermissionMerchantAccountDelete},
	{Prefix: "/merchant/account/add", Method: "PUT", Permission: PermissionMerchantAccountWrite},
	{Prefix: "/merchant/account/update", Method: "POST", Permission: PermissionMerchantAccountWrite},
	{Prefix: "/merchant/account", Method: "GET", Permission: PermissionMerchantAccountRead},
	{Prefix: "/merchant/detail/", Method: "GET", Permission: PermissionMerchantAccountRead},
	{Prefix: "/merchant/extension/dynamic-bind", Method: "POST", Permission: PermissionMerchantAccountRead},
	{Prefix: "/merchant/ai-model-flow/providers", Method: "GET", Permission: PermissionMerchantAIFlowRead},
	{Prefix: "/merchant/ai-model-flow/page", Method: "POST", Permission: PermissionMerchantAIFlowRead},
	{Prefix: "/merchant/ai-model-flow/detail/", Method: "GET", Permission: PermissionMerchantAIFlowRead},
	{Prefix: "/merchant/ai-model-flow/delete", Method: "POST", Permission: PermissionMerchantAIFlowDelete},
	{Prefix: "/merchant/ai-model-flow/precheck", Method: "POST", Permission: PermissionMerchantAIFlowPrecheck},
	{Prefix: "/merchant/ai-model-flow/publish/", Method: "POST", Permission: PermissionMerchantAIFlowPublish},
	{Prefix: "/merchant/ai-model-flow/add", Method: "PUT", Permission: PermissionMerchantAIFlowWrite},
	{Prefix: "/merchant/ai-model-flow/update", Method: "POST", Permission: PermissionMerchantAIFlowWrite},

	{Prefix: "/merchant/phone-group/page", Method: "POST", Permission: PermissionMerchantPhoneGroupRead},
	{Prefix: "/merchant/phone-group/detail/", Method: "GET", Permission: PermissionMerchantPhoneGroupRead},
	{Prefix: "/merchant/phone-group/phones/", Method: "GET", Permission: PermissionMerchantPhoneGroupRead},
	{Prefix: "/merchant/phone-group/skill-groups/", Method: "GET", Permission: PermissionMerchantPhoneGroupRead},
	{Prefix: "/merchant/phone-group/phones/", Method: "POST", Permission: PermissionMerchantPhoneGroupWrite},
	{Prefix: "/merchant/phone-group/skill-groups/", Method: "POST", Permission: PermissionMerchantPhoneGroupWrite},
	{Prefix: "/merchant/phone-group/delete", Method: "POST", Permission: PermissionMerchantPhoneGroupDelete},
	{Prefix: "/merchant/phone-group/add", Method: "PUT", Permission: PermissionMerchantPhoneGroupWrite},
	{Prefix: "/merchant/phone-group/update", Method: "POST", Permission: PermissionMerchantPhoneGroupWrite},
	{Prefix: "/merchant/phone-group", Method: "GET", Permission: PermissionMerchantPhoneGroupRead},
	{Prefix: "/merchant/skill-group/page", Method: "POST", Permission: PermissionMerchantSkillGroupRead},
	{Prefix: "/merchant/skill-group/detail/", Method: "GET", Permission: PermissionMerchantSkillGroupRead},
	{Prefix: "/merchant/skill-group/users/", Method: "POST", Permission: PermissionMerchantSkillGroupWrite},
	{Prefix: "/merchant/skill-group/phones/", Method: "POST", Permission: PermissionMerchantSkillGroupWrite},
	{Prefix: "/merchant/skill-group/delete", Method: "POST", Permission: PermissionMerchantSkillGroupDelete},
	{Prefix: "/merchant/skill-group/add", Method: "PUT", Permission: PermissionMerchantSkillGroupWrite},
	{Prefix: "/merchant/skill-group/update", Method: "POST", Permission: PermissionMerchantSkillGroupWrite},
	{Prefix: "/merchant/skill-group", Method: "GET", Permission: PermissionMerchantSkillGroupRead},
}

// RequiredPermissionForRequest 根据请求路径和方法返回需要的功能权限。
// 若返回 false，说明该请求属于公开入口或尚未纳入功能权限体系。
func RequiredPermissionForRequest(path, method string) (PermissionCode, bool) {
	path = strings.TrimSpace(path)
	method = strings.ToUpper(strings.TrimSpace(method))
	if path == "" || method == "" {
		return "", false
	}
	for _, rule := range PermissionRules {
		if !strings.EqualFold(rule.Method, method) {
			continue
		}
		matched := false
		if strings.HasSuffix(rule.Prefix, "/") || rule.Suffix != "" {
			matched = strings.HasPrefix(path, rule.Prefix)
		} else {
			matched = path == rule.Prefix
		}
		if !matched {
			continue
		}
		if rule.Suffix != "" && !strings.HasSuffix(path, rule.Suffix) {
			continue
		}
		return rule.Permission, true
	}
	return "", false
}
