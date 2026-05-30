// contracts 包定义了呼叫中心系统的对外契约，包括 HTTP API、Redis、MQ、错误码和共享类型。
// 所有跨服务通信都必须遵循本包定义的接口和数据结构，任何修改都是兼容性契约变更。
package contracts

// ServiceName 是服务实例的唯一标识，用于路由分发和服务发现。
// 新服务上线或重构服务边界时，需要同步更新此类型和相关常量。
type ServiceName string

const (
	ServiceEdge    ServiceName = "cc-edge"    // cc-edge 边缘网关服务，处理外部 API 入口
	ServiceConsole ServiceName = "cc-console" // cc-console 控制台服务，提供商户和运营管理界面
	ServiceCall    ServiceName = "cc-call"    // cc-call 呼叫核心服务，集成了 CTI 和 ESL 功能
	ServiceWorker  ServiceName = "cc-worker"  // cc-worker 后台任务处理服务，执行批量导入导出等异步任务

	// 兼容旧服务名常量，便于迁移早期按原  模块查询契约。
	// 新代码应使用上述新的服务名常量，旧名保留用于平滑迁移。
	ServiceGateway ServiceName = ServiceEdge    // 旧名：网关服务
	ServiceOpenAPI ServiceName = ServiceEdge    // 旧名：开放API服务
	ServiceAdmin   ServiceName = ServiceConsole // 旧名：管理后台服务
	ServiceCTI     ServiceName = ServiceCall    // 旧名：CTI服务
	ServiceESL     ServiceName = ServiceCall    // 旧名：ESL服务
)

// RouteContract 定义了 HTTP 路由的契约元数据。
// Service 标识路由所属服务，Module 是业务模块名，Controller 是控制器名，
// PathPrefix 是路由前缀，Methods 是支持的 HTTP 方法列表，Notes 是路由用途说明。
type RouteContract struct {
	Service    ServiceName `json:"service"`
	Module     string      `json:"module"`
	Controller string      `json:"controller"`
	PathPrefix string      `json:"pathPrefix"`
	Methods    []string    `json:"methods"`
	Notes      string      `json:"notes"`
}

var RouteContracts = []RouteContract{
	{Service: ServiceEdge, Module: "api", Controller: "CallApiController", PathPrefix: "/api/call", Methods: []string{"GET", "POST"}, Notes: "external call entry, signature required"},
	{Service: ServiceEdge, Module: "api", Controller: "TaskController", PathPrefix: "/api/task", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "external batch task API"},
	{Service: ServiceEdge, Module: "api", Controller: "RecordApiController", PathPrefix: "/api/record", Methods: []string{"GET", "POST"}, Notes: "record URL and CDR push"},
	{Service: ServiceEdge, Module: "api", Controller: "OrgApiController", PathPrefix: "/api/org", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "external organization API"},
	{Service: ServiceEdge, Module: "api", Controller: "UserApiController", PathPrefix: "/api/user", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "external user API"},
	{Service: ServiceConsole, Module: "merchant", Controller: "MerchantBackendAuthController", PathPrefix: "/merchant/auth", Methods: []string{"GET", "POST"}, Notes: "merchant login, logout, token"},
	{Service: ServiceConsole, Module: "merchant", Controller: "MerchantManageController", PathPrefix: "/merchant/detail", Methods: []string{"GET"}, Notes: "merchant details query"},
	{Service: ServiceConsole, Module: "merchant", Controller: "MerchantAccountController", PathPrefix: "/merchant/account", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "merchant scoped account management"},
	{Service: ServiceConsole, Module: "merchant", Controller: "BatchCallTaskController", PathPrefix: "/merchant/batch-call-task", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "batch task configuration and operation"},
	{Service: ServiceConsole, Module: "merchant", Controller: "BatchCallDialpadController", PathPrefix: "/merchant/batch-call-dialpad", Methods: []string{"GET", "POST"}, Notes: "dialpad task start, pause, resume, disconnect pause"},
	{Service: ServiceConsole, Module: "merchant", Controller: "CallRecordMerchantController", PathPrefix: "/merchant/call-record", Methods: []string{"GET", "POST"}, Notes: "merchant record query and recording URL"},
	{Service: ServiceConsole, Module: "merchant", Controller: "AiModelFlowController", PathPrefix: "/merchant/ai-model-flow", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "AI flow draft, publish, precheck"},
	{Service: ServiceConsole, Module: "merchant", Controller: "PhoneGroupController", PathPrefix: "/merchant/phone-group", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "phone group configuration and resource binding"},
	{Service: ServiceConsole, Module: "merchant", Controller: "SkillGroupManageController", PathPrefix: "/merchant/skill-group", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "skill group configuration and binding"},
	{Service: ServiceConsole, Module: "operate", Controller: "OperateBackendAuthController", PathPrefix: "/operate/auth", Methods: []string{"GET", "POST"}, Notes: "operate login and logout"},
	{Service: ServiceConsole, Module: "operate", Controller: "OperateAccountController", PathPrefix: "/operate/account", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "platform, operate, merchant admin, and merchant user account management"},
	{Service: ServiceConsole, Module: "operate", Controller: "MerchantManageController", PathPrefix: "/operate/merchant", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "merchant lifecycle"},
	{Service: ServiceConsole, Module: "operate", Controller: "RateManageController", PathPrefix: "/operate/rate", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "call rate management and merchant binding source"},
	{Service: ServiceConsole, Module: "operate", Controller: "BlacklistController", PathPrefix: "/operate/blacklist", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "blacklist and gateway ignore mapping"},
	{Service: ServiceConsole, Module: "operate", Controller: "WhiteListDataController", PathPrefix: "/operate/whitelist", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "whitelist data and merchant binding"},
	{Service: ServiceConsole, Module: "operate", Controller: "MerchantBillingController", PathPrefix: "/operate/billing", Methods: []string{"POST"}, Notes: "merchant billing overview and recharge management"},
	{Service: ServiceConsole, Module: "operate", Controller: "FreeSwitchLoadManageController", PathPrefix: "/operate/freeswitch", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "FreeSWITCH node management and cache refresh"},
	{Service: ServiceConsole, Module: "operate", Controller: "ChannelController", PathPrefix: "/operate/channel", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "channel configuration and rule payload"},
	{Service: ServiceConsole, Module: "operate", Controller: "PoolManageController", PathPrefix: "/operate/pool", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "number pool configuration"},
	{Service: ServiceConsole, Module: "operate", Controller: "PhoneManageController", PathPrefix: "/operate/pool-phone", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "caller number configuration"},
	{Service: ServiceConsole, Module: "operate", Controller: "ExtensionManageController", PathPrefix: "/operate/extension", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "agent extension configuration"},
	{Service: ServiceConsole, Module: "operate", Controller: "GatewayController", PathPrefix: "/operate/gateway", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "gateway management and ESL sync"},
	{Service: ServiceCall, Module: "cti", Controller: "CallTaskController", PathPrefix: "/cti/callTask", Methods: []string{"POST"}, Notes: "API outbound and task state"},
	{Service: ServiceCall, Module: "cti", Controller: "CallTaskBatchController", PathPrefix: "/cti/batch-call-task", Methods: []string{"GET", "POST", "PUT"}, Notes: "batch scheduling state machine"},
	{Service: ServiceCall, Module: "cti", Controller: "SelectNumberRuleController", PathPrefix: "/cti/select/number/rule", Methods: []string{"GET", "POST"}, Notes: "number selection rule chain"},
	{Service: ServiceCall, Module: "cti", Controller: "RemoteWebSocketController", PathPrefix: "/cti/ws", Methods: []string{"GET", "POST"}, Notes: "cluster websocket push"},
	{Service: ServiceCall, Module: "esl", Controller: "ApiCallController", PathPrefix: "/esl/call", Methods: []string{"POST"}, Notes: "API outbound start"},
	{Service: ServiceCall, Module: "esl", Controller: "BatchCallController", PathPrefix: "/esl/batch/call", Methods: []string{"POST"}, Notes: "batch outbound start"},
	{Service: ServiceCall, Module: "esl", Controller: "CallControlController", PathPrefix: "/esl/control", Methods: []string{"POST"}, Notes: "play, stop, monitor, mute, transfer, hangup, bridge, stream, dtmf"},
	{Service: ServiceCall, Module: "esl", Controller: "FreeswitchNodeController", PathPrefix: "/esl/freeswitch", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "node registry, health, event ownership"},
	{Service: ServiceCall, Module: "esl", Controller: "GatewayConfigController", PathPrefix: "/esl/gateway", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Notes: "gateway sync"},
}

// RoutesFor 返回指定服务的所有路由契约。
// 用于服务发现、路由文档生成和接口测试等场景。
// 如果该服务没有定义路由，返回空切片而非 nil。
func RoutesFor(service ServiceName) []RouteContract {
	out := make([]RouteContract, 0)
	for _, route := range RouteContracts {
		if route.Service == service {
			out = append(out, route)
		}
	}
	return out
}
