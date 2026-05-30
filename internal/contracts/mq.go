// contracts 包定义了呼叫中心系统的对外契约，包括 HTTP API、Redis、MQ、错误码和共享类型。
// 所有跨服务通信都必须遵循本包定义的接口和数据结构，任何修改都是兼容性契约变更。
package contracts

// MQContract 定义了消息队列的契约元数据。
// Name 是队列名称，Owner 是队列所属业务域，Producer/Consumer 标识生产者和消费者服务，
// AckTiming 描述消息确认时机，RetryPolicy 描述重试策略，DeadLetter 是死信队列名称，
// IdempotencyKey 定义消息幂等键格式，Observability 描述该队列的监控指标。
type MQContract struct {
	Name           string `json:"name"`
	Owner          string `json:"owner"`
	Producer       string `json:"producer"`
	Consumer       string `json:"consumer"`
	AckTiming      string `json:"ackTiming"`
	RetryPolicy    string `json:"retryPolicy"`
	DeadLetter     string `json:"deadLetter"`
	IdempotencyKey string `json:"idempotencyKey"`
	Observability  string `json:"observability"`
}

const (
	// QueueInternalCDR 内部话单队列，用于呼叫核心投递话单，由边缘网关消费进行下游推送
	QueueInternalCDR = "internal_cdr_queue"

	// QueueOdsCDR ODS 话单分析队列，用于同步通话记录到 ODS 数据仓库进行大数据分析
	QueueOdsCDR = "ods_cdr_queue"

	// QueueCallCenterCDR 呼叫中心核心话单处理队列，ESL 会话挂断时投递，由 CTI 消费进行持久化和计费流水扣减
	QueueCallCenterCDR = "call_center_cdr_queue"

	// QueueImport 运营/商户端数据导入队列，主要用于 Excel 批量号码或坐席批量导入的异步任务处理
	QueueImport = "import_queue"

	// QueueExport 运营/商户端数据导出队列，主要用于账单、话单等批量数据异步导出为文件
	QueueExport = "export_queue"

	// QueueModel AI 模型处理队列，用于商户端智能话术流程（AI Model Flow）的异步编译与发布校验
	QueueModel = "model_queue"

	// QueueTaskMonitor 外呼任务监控队列，用于后台对批量外呼任务运行指标进行动态监测与故障自愈扫描
	QueueTaskMonitor = "task_monitor_queue"
)

var MQContracts = []MQContract{
	{QueueInternalCDR, "api", "cc-call", "cc-edge", "after downstream CDR push succeeds", "bounded retry with repair task", "internal_cdr_dlq", "callId + recordId", "success/failure/retry metrics"},
	{QueueOdsCDR, "api", "cc-call", "cc-edge", "after ODS push succeeds", "bounded retry with replay", "ods_cdr_dlq", "callId + odsTarget", "latency and failure reason"},
	{QueueCallCenterCDR, "cti", "cc-call", "cc-call", "after DB, billing, recording state are durable", "retry and repair scanner", "cdr_dlq", "callId + recordId", "call trace and upload status"},
	{QueueImport, "worker", "cc-console", "cc-worker", "after Excel job state update", "retry by job id", "import_dlq", "jobId", "rows/success/failure"},
	{QueueExport, "worker", "cc-console", "cc-worker", "after file persisted and job state update", "retry by job id", "export_dlq", "jobId", "file size and duration"},
	{QueueModel, "worker", "cc-console", "cc-worker", "after model process state persisted", "retry by model id and version", "model_dlq", "modelId + version", "model phase metrics"},
	{QueueTaskMonitor, "cti", "cc-call", "cc-worker", "after monitor side effect persisted", "retry with task version", "task_monitor_dlq", "taskId + version", "task lag and repair count"},
}
