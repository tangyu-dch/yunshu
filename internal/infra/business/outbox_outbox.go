// Package business 聚合系统所有核心业务模型与仓储实现。
package business

import (
	"yunshu/internal/domain/outbox"
)

type Status = outbox.Status

const (
	Pending    Status = outbox.Pending
	Processing Status = outbox.Processing
	Published  Status = outbox.Published
	Failed     Status = Status(outbox.Failed)
)

type Entry = outbox.Entry

// OutboxStore 是 outbox 存储接口。生产实现应支持分页扫描、重试时间和幂等写入。
type OutboxStore = outbox.Store

// Store 是 OutboxStore 的别名，便于以 outbox.Store 方式引用。
type Store = OutboxStore

// LeaseStore 是支持多 worker 领取租约的 outbox 存储接口。
type LeaseStore = outbox.LeaseStore
