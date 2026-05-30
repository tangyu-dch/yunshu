// contracts 包定义了呼叫中心系统的对外契约，包括 HTTP API、Redis、MQ、错误码和共享类型。
// 所有跨服务通信都必须遵循本包定义的接口和数据结构，任何修改都是兼容性契约变更。
package contracts

// RedisContract 定义了 Redis 键的契约元数据。
// KeyPattern 是键的命名模式（支持 * 通配符），Owner 是键所属业务域，
// Writers/Readers 标识可写入和读取的服务，TTL 描述键的生存时间策略，
// ValueSchema 描述值的结构和语义，DeleteBehavior 描述删除行为和时机，
// IdempotencyRole 定义幂等键的用途（去重、补偿等）。
type RedisContract struct {
	KeyPattern      string `json:"keyPattern"`
	Owner           string `json:"owner"`
	Writers         string `json:"writers"`
	Readers         string `json:"readers"`
	TTL             string `json:"ttl"`
	ValueSchema     string `json:"valueSchema"`
	DeleteBehavior  string `json:"deleteBehavior"`
	IdempotencyRole string `json:"idempotencyRole"`
}

const (
	// KeyCallCenterEvent 呼叫中心终端话务事件键，用于终端事件状态追踪
	KeyCallCenterEvent = "call_center_event"

	// KeyCallCenterCDRQueue 核心话单缓存队列，ESL 投递后由 CTI 消费进行持久化和计费
	KeyCallCenterCDRQueue = "call_center_cdr_queue"

	// KeyCtiWebsocketPushEvent WebSocket 消息推送触发通道，通知控制台节点刷新数据投影
	KeyCtiWebsocketPushEvent = "cti_websocket_push_event"

	// KeyCallCenterBatchStatus 批量任务状态缓存队列，用于跟踪外呼任务更新
	KeyCallCenterBatchStatus = "call_center_batch_status_queue"

	// KeyBatchPrefix 批量外呼任务运行数据前缀，格式为 batch:{taskId}
	KeyBatchPrefix = "batch:*"

	// KeyCtiPhoneResourceUser 用户可用外呼号码资源缓存前缀，格式为 cti:phone_resource:user:{userId}
	KeyCtiPhoneResourceUser = "cti:phone_resource:user:*"

	// KeyConsoleAuthSession 控制台用户登录会话缓存前缀，格式为 console:auth:session:{token}
	KeyConsoleAuthSession = "console:auth:session:*"

	// KeyConsoleAuthSessionPrefix 控制台用户登录会话缓存前缀，格式为 console:auth:session:
	KeyConsoleAuthSessionPrefix = "console:auth:session:"

	// KeyBatchTelPrefix 批量任务号码拨打详情投影前缀，格式为 batch:{taskId}:tel:{telId}
	KeyBatchTelPrefix = "batch:*:tel:*"

	// KeyBatchSummaryPrefix 批量任务总览汇总投影前缀，格式为 batch:{taskId}:summary
	KeyBatchSummaryPrefix = "batch:*:summary"

	// KeyCallStatePrefix 通话信令状态与事件审计日志前缀，格式为 cc:{callId}
	KeyCallStatePrefix = "cc:*"

	// KeyConcurrencyPrefix 资源分配及呼叫并发计数器前缀，限制线路或网关的路数
	KeyConcurrencyPrefix = "concurrency:*"

	// KeyCallLimitPrefix 呼叫频次频控限制计数器前缀，限制特定号码的外呼速率
	KeyCallLimitPrefix = "call:limit:*"

	// KeyKamailioAuthPrefix SIP 注册认证鉴权缓存前缀，格式为 kamailio:auth:{subscriberId}
	KeyKamailioAuthPrefix = "kamailio:auth:*"

	// KeyBlacklistGatewaySync 黑名单与网关同步触发通道，通知 CTI/ESL 节点更新配置
	KeyBlacklistGatewaySync = "blacklist_gateway_sync"

	// KeyCallResourceAllocation 选号资源锁前缀，用于并发起呼时的资源排他性占用
	KeyCallResourceAllocation = "CALL_RESOURCE_ALLOCATION_KEY_PREFIX:*"

	// KeyCallCdrSentPrefix 话单已推送去重标记前缀，用于防止第三方回调的重复推送
	KeyCallCdrSentPrefix = "CALL_CDR_SENT_KEY_PREFIX:*"

	// KeyExtensionStatus 坐席分机实时注册与通话状态 Hash 键，由 ESL/注册服务写入，用于外呼准入校验
	KeyExtensionStatus = "extension:status"
)

var RedisContracts = []RedisContract{
	{KeyCallCenterEvent, "esl", "cc-call", "cc-call", "none", "telephony terminal event JSON", "never delete manually", "event id and call id dedupe"},
	{KeyCallCenterCDRQueue, "esl", "cc-call", "cc-call", "none", "CDR task JSON list item", "consumer pop after durable side effect", "callId + recordId"},
	{KeyCtiWebsocketPushEvent, "cti", "cc-worker/cc-call", "cc-call websocket instances", "none", "websocket refresh event JSON with projectionKey, merchantId and taskId", "pub/sub only, Redis/DB projections remain truth", "event id + projection key"},
	{KeyCallCenterBatchStatus, "cti", "cc-call", "cc-call/cc-worker", "none", "batch status message", "ack after projection update", "taskId + status + version"},
	{KeyBatchPrefix, "cti", "cc-call", "cc-call/cc-console", "task scoped", "batch runtime projection", "delete on settlement or repair", "taskId + listId"},
	{KeyCtiPhoneResourceUser, "cti", "cc-call", "cc-call", "15m", "user candidate phone resource JSON list", "expire and refresh on source change", "userId"},
	{KeyConsoleAuthSession, "operate", "cc-console", "cc-console", "12h", "management auth ticket JSON with token and tenant context", "expire on TTL or delete on logout", "login token"},
	{KeyBatchTelPrefix, "cti", "cc-worker", "cc-console/cc-call", "7d", "batch tel projection hash with merchantId/taskId/telId", "expire after audit window or repair", "merchantId + taskId + telId + outboxId"},
	{KeyBatchSummaryPrefix, "cti", "cc-worker", "cc-console/cc-call", "7d", "batch task summary projection hash with merchantId/taskId", "expire after audit window or repair", "merchantId + taskId + outboxId"},
	{KeyCallStatePrefix, "esl/cti", "cc-call", "cc-call", "event scoped", "call state, audit, event buffers", "expire after final audit window", "callId + eventId"},
	{KeyConcurrencyPrefix, "cti", "cc-call", "cc-call", "runtime", "integer counters", "release on hangup complete or repair", "command id"},
	{KeyCallLimitPrefix, "cti", "cc-call", "cc-call", "rate window", "integer counters", "expire by window", "callee/caller rate window"},
	{KeyKamailioAuthPrefix, "cti", "cc-call/cc-console", "cc-call", "config scoped", "auth cache JSON", "delete on subscriber change", "subscriber id"},
	{KeyBlacklistGatewaySync, "operate", "cc-console", "cc-call", "none", "gateway sync event", "consumer ack after sync state", "blacklist version"},
	{KeyCallResourceAllocation, "cti", "cc-call", "cc-call", "short", "allocation lock", "release after allocation outcome", "command id"},
	{KeyCallCdrSentPrefix, "cti", "cc-call", "cc-call/cc-worker", "audit window", "cdr sent marker", "expire after retry window", "callId + downstream"},
	{KeyExtensionStatus, "esl", "cc-call", "cc-call/cc-console", "none", "-compatible Redis extension status hash (-1 offline, 0 busy, 1 idle, etc.)", "never delete manually", "extensionNumber"},
}

// BuildBatchTelKey 构造用于存储批量外呼单个号码拨打状态及详情的 Redis 投影键 (Hash 类型)
// 格式为: batch:{taskId}:tel:{telId}
func BuildBatchTelKey(taskID, telID string) string {
	return "batch:" + taskID + ":tel:" + telID
}

// BuildBatchSummaryKey 构造用于存储整个批量任务执行概览与汇总投影的 Redis 投影键 (Hash 类型)
// 格式为: batch:{taskId}:summary
func BuildBatchSummaryKey(taskID string) string {
	return "batch:" + taskID + ":summary"
}
