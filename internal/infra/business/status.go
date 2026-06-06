// Package business 提供 outbox workflow 节点的通用状态常量定义。
//
// 统一管理所有业务模块的状态值，避免散落在多个文件中。
// 状态分为以下几类：
//   - Outbox 状态：pending, processing, published, failed
//   - 通用状态：pending, success, failed, skipped
//   - 计费状态：rated
//   - 结算状态：settled, no_op
//   - 录音状态：uploaded
//   - 下游推送状态：delivered
package business

// Outbox 状态常量，对应 message_outbox 表的 status 字段。
const (
	OutboxPending    = "pending"    // 待投递
	OutboxProcessing  = "processing" // 处理中（已领取租约）
	OutboxPublished   = "published"  // 已投递成功
	OutboxFailed      = "failed"     // 投递失败（待重试）
)

// 通用业务状态常量。
const (
	StatusPending = "pending" // 待处理
	StatusSuccess = "success"  // 成功
	StatusFailed  = "failed"   // 失败
	StatusSkipped = "skipped"  // 跳过/忽略
)

// Billing 状态常量，对应计费流水表的状态。
const (
	StatusRated = "rated" // 已计费
)

// Settlement 状态常量，对应结算任务表的状态。
const (
	StatusSettled = "settled" // 已结算
	StatusNoOp    = "no_op"   // 无操作（商户余额表不存在等）
)

// Recording 状态常量，对应录音任务表的状态。
const (
	StatusUploaded = "uploaded" // 已上传
)

// Downstream 状态常量，对应下游推送任务表的状态。
const (
	StatusDelivered = "delivered" // 已投递
)
