package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"yunshu/internal/domain/callflow"
	"yunshu/internal/infra/business"
	"yunshu/internal/infra/callback"
	"yunshu/internal/infra/config"
	"yunshu/internal/infra/projection"
	redisinfra "yunshu/internal/infra/redis"
)

// WorkerRuntime 聚合 cc-worker 后台流程节点。
//
// 当前先承接 outbox 投递，后续导入导出、录音修复、回调补偿等 worker 节点都应
// 以同样方式注册为明确的流程投递器或事件消费者。
type WorkerRuntime struct {
	Outbox     business.OutboxStore
	CDR        business.CdrStore
	Billing    business.BillingLedgerStore
	Settlement business.SettlementStore
	Recording  business.RecordingStore
	Reporting  business.ReportingStore
	Downstream business.DownstreamStore
	Dispatcher *callflow.OutboxDispatcher
	Logger     *slog.Logger
}

// NewWorkerRuntimeWithConfig 创建 worker 运行时。
func NewWorkerRuntimeWithConfig(cfg config.Config, logger *slog.Logger) (*WorkerRuntime, error) {
	if logger == nil {
		logger = slog.Default()
	}
	gormDB, err := openRuntimeDB(cfg, logger)
	if err != nil {
		return nil, err
	}
	outboxStore := buildOutboxStore(gormDB, logger)
	cdrStore := buildCDRStore(gormDB, logger)
	billingStore := buildBillingStore(gormDB, logger)
	settlementStore := buildSettlementStore(gormDB, logger)
	recordingStore := buildRecordingStore(gormDB, logger)
	reportingStore := buildReportingStore(gormDB, logger)
	downstreamStore := buildDownstreamStore(gormDB, logger)
	// 使用重构后独立出来的高凝聚力 projection 包的 Redis 批量外呼投影器
	var batchProjector *projection.RedisBatchProjector
	if len(cfg.Redis.Addrs) > 0 {
		batchProjector = projection.NewRedisBatchProjector(redisinfra.NewClient(cfg.Redis), logger)
		logger.Info("cc-worker 批量外呼 Redis 投影已启用", "redisAddr", cfg.Redis.Addrs[0])
	} else {
		logger.Warn("cc-worker 未配置 Redis 地址，批量外呼投影仅记录日志", "impact", "WebSocket/控制台无法读取 Redis 投影视图")
	}
	callbackClient := callback.NewHTTPClient(cfg.Worker.Callback.URL, cfg.Worker.Callback.Secret, cfg.Worker.Callback.Timeout, logger)
	if cfg.Worker.Callback.URL == "" {
		logger.Warn("cc-worker 客户回调地址未配置，回调 outbox 将按本地跳过处理", "impact", "生产环境应配置 CALLBACK_URL")
	} else {
		logger.Info("cc-worker 客户回调投递已启用", "callbackURL", cfg.Worker.Callback.URL, "timeout", cfg.Worker.Callback.Timeout)
	}
	downstreamClient := business.NewDownstreamHTTPClient(cfg.Worker.Downstream.URL, cfg.Worker.Downstream.Secret, cfg.Worker.Downstream.Timeout, logger)
	if !downstreamClient.Enabled() {
		logger.Warn("cc-worker CDR 下游 HTTP 地址未配置，下游推送任务将仅持久化为 pending", "impact", "生产环境应配置 DOWNSTREAM_CDR_URL")
	} else {
		logger.Info("cc-worker CDR 下游 HTTP 投递已启用", "downstreamURL", cfg.Worker.Downstream.URL, "timeout", cfg.Worker.Downstream.Timeout)
	}
	recordingClient := business.NewRecordingHTTPClient(cfg.Worker.Recording.URL, cfg.Worker.Recording.Secret, cfg.Worker.Recording.Timeout, logger)
	if !recordingClient.Enabled() {
		logger.Warn("cc-worker 录音上传地址未配置，录音任务将仅持久化为 pending/skipped", "impact", "生产环境应配置 RECORDING_UPLOAD_URL")
	} else {
		logger.Info("cc-worker 录音上传 HTTP 投递已启用", "recordingURL", cfg.Worker.Recording.URL, "timeout", cfg.Worker.Recording.Timeout)
	}
	if cfg.Worker.Billing.DefaultRatePerMin <= 0 {
		logger.Warn("cc-worker 计费默认费率未配置或为零，当前只会产出零金额审计记录", "impact", "生产环境应配置 WORKER_BILLING_DEFAULT_RATE_PER_MIN")
	} else {
		logger.Info("cc-worker 计费默认费率已启用", "defaultRatePerMin", cfg.Worker.Billing.DefaultRatePerMin)
	}
	dispatcher := &callflow.OutboxDispatcher{
		Store:      outboxStore,
		Handlers:   defaultOutboxHandlers(outboxStore, batchProjector, callbackClient, downstreamClient, recordingClient, cdrStore, billingStore, settlementStore, recordingStore, reportingStore, downstreamStore, cfg.Worker.Billing.DefaultRatePerMin, logger),
		WorkerID:   cfg.Worker.Outbox.WorkerID,
		RetryDelay: cfg.Worker.Outbox.RetryDelay,
		Lease:      cfg.Worker.Outbox.Lease,
		BatchSize:  cfg.Worker.Outbox.BatchSize,
		Logger:     logger,
	}
	logger.Info("cc-worker outbox 投递器已初始化", "workerId", dispatcher.WorkerID, "batchSize", dispatcher.BatchSize, "retryDelay", dispatcher.RetryDelay, "lease", dispatcher.Lease)
	return &WorkerRuntime{Outbox: outboxStore, CDR: cdrStore, Billing: billingStore, Settlement: settlementStore, Recording: recordingStore, Reporting: reportingStore, Downstream: downstreamStore, Dispatcher: dispatcher, Logger: logger}, nil
}

// Start 启动 worker 后台循环。
func (r *WorkerRuntime) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	r.Logger.Info("cc-worker 后台投递循环启动", "interval", interval.String())
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			if _, err := r.Dispatcher.DispatchOnce(ctx); err != nil {
				r.Logger.Error("cc-worker outbox 投递轮次失败", "error", err.Error())
			}
			select {
			case <-ctx.Done():
				r.Logger.Info("cc-worker 后台投递循环退出", "reason", ctx.Err().Error())
				return
			case <-ticker.C:
			}
		}
	}()
}

func defaultOutboxHandlers(outboxStore business.OutboxStore, batchProjector *projection.RedisBatchProjector, callbackClient *callback.HTTPClient, downstreamClient *business.DownstreamHTTPClient, recordingClient *business.RecordingHTTPClient, cdrStore business.CdrStore, billingStore business.BillingLedgerStore, settlementStore business.SettlementStore, recordingStore business.RecordingStore, reportingStore business.ReportingStore, downstreamStore business.DownstreamStore, defaultRatePerMin float64, logger *slog.Logger) map[string]callflow.OutboxHandler {
	return map[string]callflow.OutboxHandler{
		"call_center_cdr_queue": func(ctx context.Context, entry business.Entry) error {
			if cdrStore != nil {
				if err := cdrStore.SaveFromOutbox(ctx, entry); err != nil {
					return err
				}
			} else {
				logger.Info("CDR outbox 已消费但未配置数据库落库", "outboxId", entry.ID, "aggregateId", entry.AggregateID, "destination", entry.Destination)
			}
			if err := appendCDRFanout(ctx, outboxStore, entry, logger); err != nil {
				return err
			}
			return nil
		},
		callflow.DestinationCDRBilling: func(ctx context.Context, entry business.Entry) error {
			if billingStore == nil {
				logger.Info("CDR 计费流程节点已领取但未配置计费仓储", "outboxId", entry.ID, "callId", entry.AggregateID, "destination", entry.Destination)
				return nil
			}
			ledger, err := billingStore.SaveFromOutbox(ctx, entry)
			if err != nil {
				return err
			}

			if ledger.Status == business.StatusRated {
				return nil
			}

			rateNote := ""
			rate := defaultRatePerMin
			if rate <= 0 {
				logger.Warn("【云枢计费告警】系统默认计费费率未配置或为零！将只生成审计用的零费率估算，不允许直接当作最终扣费结果。", "callId", ledger.CallID)
				rateNote = "【审计估算】系统默认费率未配置，采用零金额审计结转"
			}

			rating := business.EstimateByMinute(ledger.DurationSec, rate)
			if rateNote != "" {
				rating.Note = rateNote
				rating.RatePerMin = 0
				rating.Amount = 0
			}

			if err := billingStore.MarkRated(ctx, ledger.CallID, rating.Amount, rating.RatePerMin, rating.Note); err != nil {
				return err
			}
			return appendBillingSettlement(ctx, outboxStore, entry, rating.Amount, rating.RatePerMin, rating.Note, logger)
		},
		callflow.DestinationBillingSettlement: func(ctx context.Context, entry business.Entry) error {
			if settlementStore == nil {
				logger.Info("结算流程节点已领取但未配置结算仓储", "outboxId", entry.ID, "callId", entry.AggregateID, "destination", entry.Destination)
				return nil
			}
			job, err := settlementStore.SaveFromOutbox(ctx, entry)
			if err != nil {
				return err
			}
			before, after, err := settlementStore.DebitBalance(ctx, job.MerchantID, job.Amount)
			if err != nil {
				if errors.Is(err, business.ErrBillingOverviewNotFound) {
					logger.Warn("【云枢账务审计】商户账单总览表不存在，跳过余额扣减，显式记录为 no-op 结算事实", "merchantId", job.MerchantID, "jobId", job.ID)
					if markErr := settlementStore.MarkNoOp(ctx, job.ID, "merchant billing overview not found"); markErr != nil {
						return markErr
					}
					return nil
				}
				if markErr := settlementStore.MarkFailed(ctx, job.ID, err.Error()); markErr != nil {
					return markErr
				}
				return err
			}
			return settlementStore.MarkSettled(ctx, job.ID, before, after, time.Now().UTC())
		},
		callflow.DestinationCDRRecording: func(ctx context.Context, entry business.Entry) error {
			if recordingStore == nil {
				logger.Info("CDR 录音流程节点已领取但未配置录音仓储", "outboxId", entry.ID, "callId", entry.AggregateID, "destination", entry.Destination)
				return nil
			}
			job, err := recordingStore.SaveFromOutbox(ctx, entry)
			if err != nil {
				return err
			}
			if job.Status == business.StatusSkipped || job.Status == business.StatusUploaded {
				return nil
			}
			if recordingClient == nil || !recordingClient.Enabled() {
				return nil
			}
			if err := recordingClient.Upload(ctx, entry, job); err != nil {
				if markErr := recordingStore.MarkFailed(ctx, job.ID, err.Error()); markErr != nil {
					return markErr
				}
				return err
			}
			return recordingStore.MarkUploaded(ctx, job.ID, time.Now().UTC())
		},
		callflow.DestinationCDRReportProjection: func(ctx context.Context, entry business.Entry) error {
			if reportingStore != nil {
				return reportingStore.SaveFromOutbox(ctx, entry)
			}
			logger.Info("CDR 报表投影流程节点已领取但未配置报表仓储", "outboxId", entry.ID, "callId", entry.AggregateID, "destination", entry.Destination)
			return nil
		},
		callflow.DestinationCDRDownstreamPush: func(ctx context.Context, entry business.Entry) error {
			if downstreamStore == nil {
				logger.Info("CDR 下游推送流程节点已领取但未配置下游仓储", "outboxId", entry.ID, "callId", entry.AggregateID, "destination", entry.Destination)
				return nil
			}
			job, err := downstreamStore.SaveFromOutbox(ctx, entry)
			if err != nil {
				return err
			}
			if downstreamClient == nil || !downstreamClient.Enabled() {
				return nil
			}
			if err := downstreamClient.Deliver(ctx, entry, job); err != nil {
				if markErr := downstreamStore.MarkFailed(ctx, job.ID, err.Error()); markErr != nil {
					return markErr
				}
				return err
			}
			return downstreamStore.MarkDelivered(ctx, job.ID, time.Now().UTC())
		},
		callflow.DestinationBatchTelProjection: func(ctx context.Context, entry business.Entry) error {
			if batchProjector != nil {
				return batchProjector.ProjectTelCompleted(ctx, entry)
			}
			logger.Info("投递批量外呼号码完成投影", "outboxId", entry.ID, "aggregateId", entry.AggregateID, "destination", entry.Destination)
			return nil
		},
		callflow.DestinationBatchTaskProjection: func(ctx context.Context, entry business.Entry) error {
			if batchProjector != nil {
				return batchProjector.ProjectTaskCompleted(ctx, entry)
			}
			logger.Info("投递批量外呼任务完成投影", "outboxId", entry.ID, "aggregateId", entry.AggregateID, "destination", entry.Destination)
			return nil
		},
		callflow.DestinationBatchCallback: func(ctx context.Context, entry business.Entry) error {
			if callbackClient != nil {
				return callbackClient.Deliver(ctx, entry)
			}
			logger.Info("投递批量外呼客户回调", "outboxId", entry.ID, "aggregateId", entry.AggregateID, "destination", entry.Destination)
			return nil
		},
	}
}

func appendCDRFanout(ctx context.Context, store business.OutboxStore, entry business.Entry, logger *slog.Logger) error {
	if store == nil {
		logger.Warn("CDR 后续流程节点未写入，outbox store 为空", "outboxId", entry.ID, "callId", entry.AggregateID)
		return nil
	}
	for _, fanout := range callflow.BuildCDRFanoutEntries(entry, time.Now().UTC()) {
		if err := store.Append(ctx, fanout); err != nil {
			if errors.Is(err, business.ErrDuplicateEntry) {
				logger.Info("CDR 后续流程节点已存在，按幂等跳过", "outboxId", fanout.ID, "callId", fanout.AggregateID, "destination", fanout.Destination)
				continue
			}
			logger.Error("CDR 后续流程节点写入失败", "outboxId", fanout.ID, "callId", fanout.AggregateID, "destination", fanout.Destination, "error", err.Error())
			return err
		}
		logger.Info("CDR 后续流程节点已写入", "outboxId", fanout.ID, "callId", fanout.AggregateID, "destination", fanout.Destination)
	}
	return nil
}

func appendBillingSettlement(ctx context.Context, store business.OutboxStore, billingEntry business.Entry, amount, ratePerMin float64, note string, logger *slog.Logger) error {
	if store == nil {
		logger.Warn("结算后续流程节点未写入，outbox store 为空", "outboxId", billingEntry.ID, "callId", billingEntry.AggregateID)
		return nil
	}
	payload := make(map[string]any, len(billingEntry.Payload)+4)
	for key, value := range billingEntry.Payload {
		payload[key] = value
	}
	payload["amount"] = amount
	payload["ratePerMin"] = ratePerMin
	payload["ratingNote"] = note
	settlementEntry := callflow.BuildBillingSettlementEntry(business.Entry{
		ID:             billingEntry.ID,
		AggregateType:  billingEntry.AggregateType,
		AggregateID:    billingEntry.AggregateID,
		Destination:    billingEntry.Destination,
		IdempotencyKey: billingEntry.IdempotencyKey,
		Payload:        payload,
	}, time.Now().UTC())
	if err := store.Append(ctx, settlementEntry); err != nil {
		if errors.Is(err, business.ErrDuplicateEntry) {
			logger.Info("结算后续流程节点已存在，按幂等跳过", "outboxId", settlementEntry.ID, "callId", settlementEntry.AggregateID, "destination", settlementEntry.Destination)
			return nil
		}
		logger.Error("结算后续流程节点写入失败", "outboxId", settlementEntry.ID, "callId", settlementEntry.AggregateID, "destination", settlementEntry.Destination, "error", err.Error())
		return err
	}
	logger.Info("结算后续流程节点已写入", "outboxId", settlementEntry.ID, "callId", settlementEntry.AggregateID, "destination", settlementEntry.Destination)
	return nil
}

func buildCDRStore(gormDB *gorm.DB, logger *slog.Logger) business.CdrStore {
	if gormDB == nil {
		logger.Warn("CDR 将使用内存存储，本地开发可用，生产环境必须配置 MySQL", "table", "call_cdr_record")
		return business.NewCdrMemoryStore()
	}
	logger.Info("CDR 将使用数据库持久化", "table", "call_cdr_record")
	return business.NewCdrGormStore(gormDB, logger)
}

func buildBillingStore(gormDB *gorm.DB, logger *slog.Logger) business.BillingLedgerStore {
	if gormDB == nil {
		logger.Warn("CDR 计费将使用内存存储，本地开发可用，生产环境必须配置 MySQL", "table", "call_billing_ledger")
		return business.NewBillingLedgerMemoryStore()
	}
	logger.Info("CDR 计费将使用数据库持久化", "table", "call_billing_ledger")
	return business.NewBillingLedgerGormStore(gormDB, logger)
}

func buildRecordingStore(gormDB *gorm.DB, logger *slog.Logger) business.RecordingStore {
	if gormDB == nil {
		logger.Warn("CDR 录音任务将使用内存存储，本地开发可用，生产环境必须配置 MySQL", "table", "call_recording_job")
		return business.NewRecordingMemoryStore()
	}
	logger.Info("CDR 录音任务将使用数据库持久化", "table", "call_recording_job")
	return business.NewRecordingGormStore(gormDB, logger)
}

func buildReportingStore(gormDB *gorm.DB, logger *slog.Logger) business.ReportingStore {
	if gormDB == nil {
		logger.Warn("CDR 报表投影将使用内存存储，本地开发可用，生产环境必须配置 MySQL", "table", "call_report_projection")
		return business.NewReportMemoryStore()
	}
	logger.Info("CDR 报表投影将使用数据库持久化", "table", "call_report_projection")
	return business.NewReportGormStore(gormDB, logger)
}

func buildDownstreamStore(gormDB *gorm.DB, logger *slog.Logger) business.DownstreamStore {
	if gormDB == nil {
		logger.Warn("CDR 下游推送任务将使用内存存储，本地开发可用，生产环境必须配置 MySQL", "table", "call_downstream_push_job")
		return business.NewPushMemoryStore()
	}
	logger.Info("CDR 下游推送任务将使用数据库持久化", "table", "call_downstream_push_job")
	return business.NewPushGormStore(gormDB, logger)
}

func buildSettlementStore(gormDB *gorm.DB, logger *slog.Logger) business.SettlementStore {
	if gormDB == nil {
		logger.Warn("结算任务将使用内存存储，本地开发可用，生产环境必须配置 MySQL", "table", "call_billing_settlement_job")
		return business.NewSettlementMemoryStore()
	}
	logger.Info("结算任务将使用数据库持久化", "table", "call_billing_settlement_job")
	return business.NewSettlementGormStore(gormDB, logger)
}
