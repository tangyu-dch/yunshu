package callflow

// callflow 包负责 CTI/ESL 工作流编排的消费者入口。
// 它订阅内部事件总线，将事件路由到对应的 CTI 或 ESL 工作流引擎，
// 实现业务状态机的推进。消费者不包含业务逻辑，只负责事件分发和流程驱动。

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/cti"
	"yunshu/internal/domain/esl"
	"yunshu/internal/domain/operate"
	"yunshu/internal/infra/events"
	"yunshu/pkg/telephony"
	"yunshu/pkg/workflow"
)

// RegisterConsumers 注册 cc-call 内部事件消费者。
// 当前消费者负责订阅事件总线，并在事件发生时提取上下文，向 CTI 或 ESL 引擎发送状态推进信号。
// 涵盖 API 外呼、批量外呼、拨号盘直呼以及客户呼入四大经典通信流程。
func RegisterConsumers(bus events.Bus, ctiRunner *workflow.Runner, eslRunner *workflow.Runner, sessionService *esl.SessionService, originate *esl.OriginateService, runtimeSelector *cti.RuntimeSelector, candidateSource cti.CandidateSource, statusReader esl.ExtensionStatusReader, batchRepo cti.BatchTaskRepository, queue cti.CallQueue, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}

	// 1. 订阅 API 外呼请求事件：API 外呼入口校验及选号。
	bus.Subscribe(contracts.EventAPICallRequested, func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		logger.Info("消费 API 外呼请求事件并推进 CTI 流程", "eventId", event.EventID, "callId", event.AggregateID)
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowAPIOutbound, event.AggregateID, workflow.Event{Name: "validate", Payload: event.Payload}); err != nil {
			return err
		}
		_, err := ctiRunner.Apply(ctx, cti.WorkflowAPIOutbound, event.AggregateID, workflow.Event{Name: "select_number", Payload: event.Payload})
		return err
	})

	// 2. 订阅批量外呼号码分配请求事件：控制并发槽位及分配网关。
	bus.Subscribe(contracts.EventBatchCallRequested, func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		logger.Info("消费批量外呼号码请求事件并推进 CTI 流程", "eventId", event.EventID, "callId", event.AggregateID, "batchTaskId", event.Payload["batchTaskId"], "batchCallTelId", event.Payload["batchCallTelId"])
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, event.AggregateID, workflow.Event{Name: "acquire_slot", Payload: event.Payload}); err != nil {
			return err
		}
		if _, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, event.AggregateID, workflow.Event{Name: "select_number", Payload: event.Payload}); err != nil {
			return err
		}
		_, err := ctiRunner.Apply(ctx, cti.WorkflowBatchOutbound, event.AggregateID, workflow.Event{Name: "dispatch_originate", Payload: event.Payload})
		return err
	})

	// 3. 订阅 ESL 命令已发送事件：通知 ESL 工作流引擎命令已发出，推进至呼叫执行阶段（originating）。
	bus.Subscribe(contracts.EventESLCommandSent, func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		logger.Info("消费 ESL 起呼命令事件并推进 ESL 流程", "eventId", event.EventID, "callId", event.AggregateID)
		workflowID := eslWorkflowFromPayload(event.Payload)
		if _, err := eslRunner.Apply(ctx, workflowID, event.AggregateID, workflow.Event{Name: "validate_command", Payload: event.Payload}); err != nil {
			return err
		}
		_, err := eslRunner.Apply(ctx, workflowID, event.AggregateID, workflow.Event{Name: "execute_originate", Payload: event.Payload})
		return err
	})

	// 4. 订阅 FreeSWITCH 事件：将物理 FreeSWITCH 事件流映射并投递至 ESL 工作流中，并按通话阶段（如振铃、接通、挂断）执行相应的交互控制。
	bus.Subscribe(contracts.EventFSApplied, func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		eventName, _ := event.Payload["eventName"].(string)
		legRole := stringValue(event.Payload, "legRole")

		// 1.1 自动释放运行时选号并发槽位，防止并发额度泄漏
		if eventName == string(esl.EventChannelHangupComplete) && sessionService != nil && runtimeSelector != nil && runtimeSelector.Allocator != nil {
			callID := event.AggregateID
			if session, serr := sessionService.Store.Get(ctx, callID); serr == nil && session.Metadata != nil {
				claimKey, _ := session.Metadata["selectionClaimKey"].(string)
				selectedCaller, _ := session.Metadata["selectedCaller"].(string)
				selectedGatewayID, _ := session.Metadata["selectedGatewayId"].(string)
				merchantIDVal := session.Metadata["merchantId"]
				var merchantID string
				switch m := merchantIDVal.(type) {
				case int:
					merchantID = strconv.Itoa(m)
				case float64:
					merchantID = strconv.Itoa(int(m))
				case string:
					merchantID = m
				}

				if claimKey != "" || (selectedCaller != "" && selectedGatewayID != "") {
					alloc := cti.RuntimeAllocation{
						CallID:     callID,
						MerchantID: merchantID,
						Caller:     selectedCaller,
						GatewayID:  selectedGatewayID,
						ClaimKey:   claimKey,
					}
					logger.Info("物理挂机事件触发，原子释放运行时选号并发槽位", "callId", callID, "merchantId", merchantID, "caller", selectedCaller, "gatewayId", selectedGatewayID)
					if rerr := runtimeSelector.Allocator.Release(ctx, alloc); rerr != nil {
						logger.Error("原子释放运行时并发槽位失败", "callId", callID, "error", rerr.Error())
					} else {
						// 释放成功后，原子清除 Metadata 中的相关键，避免后续事件重复释放导致计数负溢出
						delete(session.Metadata, "selectionClaimKey")
						delete(session.Metadata, "selectedCaller")
						delete(session.Metadata, "selectedGatewayId")
						_ = sessionService.Store.Save(ctx, session)
					}
				}
			}
		}

		// 1. 实时更新分机状态并处理排队分配
		if legRole == string(contracts.LegRoleAgent) {
			var ext string
			var userID int
			var agentMerchantID int
			if sessionService != nil {
				if session, serr := sessionService.Store.Get(ctx, event.AggregateID); serr == nil {
					ext, _ = session.Metadata["extension"].(string)
					if ext == "" {
						ext, _ = session.Metadata["agentId"].(string)
					}
					userID = intFromMap(session.Metadata, "userId")
					// 提取坐席所属商户 ID，用于 ACW 冷却后从正确的多租户队列中拉取等待呼叫
					agentMerchantID = intFromMap(session.Metadata, "merchantId")
				}
			}
			if ext == "" {
				ext, _ = event.Payload["extension"].(string)
			}
			if ext == "" {
				ext, _ = event.Payload["agentId"].(string)
			}
			if userID <= 0 {
				userID = intFromMap(event.Payload, "userId")
			}
			if agentMerchantID <= 0 {
				agentMerchantID = intFromMap(event.Payload, "merchantId")
			}

			if ext != "" {
				if writer, ok := statusReader.(esl.ExtensionStatusWriter); ok {
					switch eventName {
					case string(esl.EventChannelProgress), string(esl.EventChannelProgressMedia):
						// 坐席振铃：实时标记为振铃状态
						if werr := writer.SetExtensionStatus(ctx, ext, esl.ExtensionStatusRinging); werr != nil {
							logger.Error("实时更新分机状态到 Redis 失败（振铃）", "extension", ext, "error", werr.Error())
						} else {
							logger.Info("已实时更新分机状态到 Redis（振铃）", "extension", ext)
						}
					case string(esl.EventChannelAnswer):
						// 坐席接通：实时标记为通话中
						if werr := writer.SetExtensionStatus(ctx, ext, esl.ExtensionStatusTalking); werr != nil {
							logger.Error("实时更新分机状态到 Redis 失败（通话中）", "extension", ext, "error", werr.Error())
						} else {
							logger.Info("已实时更新分机状态到 Redis（通话中）", "extension", ext)
						}
					case string(esl.EventChannelHangupComplete):
						// 坐席挂断：进入话后整理（ACW）冷却期，5 秒后才置为空闲并拉取排队。
						// 冷却期内坐席状态保持忙碌，避免在整理客户资料时被立即二次派单。
						if batchRepo != nil && queue != nil && sessionService != nil && originate != nil && userID > 0 {
							logger.Info("坐席挂断，进入话后整理冷却（ACW 5s）", "userId", userID, "extension", ext)
							// 捕获闭包变量，避免协程持有外层循环指针
							capturedExt := ext
							capturedUserID := userID
							capturedWriter := writer
							capturedAgentMerchantID := agentMerchantID
							go func() {
								// ACW 冷却：5 秒后再置为空闲并检查排队
								time.Sleep(5 * time.Second)
								bgCtx := context.Background()

								// 将分机状态原子置为空闲
								if werr := capturedWriter.SetExtensionStatus(bgCtx, capturedExt, esl.ExtensionStatusIdle); werr != nil {
									logger.Error("ACW 冷却后置分机为空闲失败", "extension", capturedExt, "error", werr.Error())
									return
								}
								logger.Info("ACW 冷却结束，分机已置为空闲，开始检查排队队列", "userId", capturedUserID, "extension", capturedExt, "merchantId", capturedAgentMerchantID)

								// 检查坐席所属技能组的排队队列，拉取等待中的客户呼叫
								skillGroups, sgErr := batchRepo.GetAgentSkillGroups(bgCtx, capturedUserID)
								if sgErr != nil || len(skillGroups) == 0 {
									return
								}
								logger.Info("ACW 后检查关联技能组排队队列", "userId", capturedUserID, "extension", capturedExt, "skillGroups", skillGroups)
								for _, sgID := range skillGroups {
									// 使用坐席所属商户 ID 从多租户队列中弹出等待呼叫
									waitingCallID, qErr := queue.Pop(bgCtx, capturedAgentMerchantID, sgID)
									if qErr != nil || waitingCallID == "" {
										continue
									}
									logger.Info("ACW 后从排队队列中成功拉取等待呼叫", "skillGroupId", sgID, "waitingCallId", waitingCallID)
									custSession, sErr := sessionService.Store.Get(bgCtx, waitingCallID)
									if sErr != nil {
										logger.Error("ACW 后读取排队呼叫会话失败", "callId", waitingCallID, "error", sErr.Error())
										continue
									}
									customerUUID, _ := custSession.Metadata["customerUuid"].(string)
									if customerUUID == "" {
										logger.Warn("ACW 后排队呼叫缺少 customerUuid，跳过", "callId", waitingCallID)
										continue
									}

									// 停止客户腿的排队等待音
									if boolFromMap(custSession.Metadata, "queueWaitPlaying") {
										breakCmd := telephony.NewCommand(
											"break:"+waitingCallID+":stop_queue_wait",
											"break",
											waitingCallID,
											customerUUID,
											custSession.FSAddr,
											contracts.LegRoleCustomer,
											contracts.CallFlowBatchPredictive,
											map[string]any{},
										)
										if berr := originate.CommandService.Execute(bgCtx, breakCmd); berr != nil {
											logger.Error("ACW 后停止排队客户等待音失败", "callId", waitingCallID, "error", berr.Error())
										} else {
											custSession.Metadata["queueWaitPlaying"] = false
											logger.Info("ACW 后已向客户腿发送停止播放等待音指令", "callId", waitingCallID)
										}
									}

									// 为排队客户起呼该空闲坐席并准备桥接
									agentUUID := esl.NewDeterministicUUID("agent", waitingCallID)
									custSession.Metadata["extension"] = capturedExt
									custSession.Metadata["agentUuid"] = agentUUID
									custSession.Metadata["userId"] = capturedUserID
									custSession.Metadata["agentOriginateSent"] = true
									custSession.Metadata["inQueue"] = false
									if serr := sessionService.Store.Save(bgCtx, custSession); serr != nil {
										logger.Error("ACW 后保存排队呼叫会话失败", "callId", waitingCallID, "error", serr.Error())
										continue
									}
									req := esl.BatchAgentOriginateRequest{
										Version:      stringFromMap(custSession.Metadata, "routeVersion"),
										CallID:       waitingCallID,
										Extension:    capturedExt,
										AgentUUID:    agentUUID,
										CustomerUUID: customerUUID,
										FSAddr:       custSession.FSAddr,
										UserID:       capturedUserID,
										MerchantID:   intFromMap(custSession.Metadata, "merchantId"),
									}
									if oerr := originate.StartBatchAgentOutbound(bgCtx, req); oerr != nil {
										logger.Error("ACW 后为排队客户起呼坐席腿失败", "callId", waitingCallID, "extension", capturedExt, "error", oerr.Error())
									} else {
										logger.Info("ACW 后成功为排队客户起呼坐席腿", "callId", waitingCallID, "extension", capturedExt)
										break // 坐席已分配并被起呼，不能再为该坐席分配其它排队呼叫
									}
								}
							}()
						} else {
							// 无排队依赖时，直接同步置为空闲
							if werr := writer.SetExtensionStatus(ctx, ext, esl.ExtensionStatusIdle); werr != nil {
								logger.Error("实时更新分机状态到 Redis 失败（挂断置空闲）", "extension", ext, "error", werr.Error())
							} else {
								logger.Info("已实时更新分机状态到 Redis（挂断置空闲）", "extension", ext)
							}
						}
					} // end switch eventName
				} // end if writer
			} // end if ext != ""
		} // end if legRole == agent

		// 3. 客户腿挂机时，如果客户正在排队中，原子清理 Redis 队列以避免空指针起呼
		if legRole == string(contracts.LegRoleCustomer) &&
			eventName == string(esl.EventChannelHangupComplete) &&
			queue != nil && sessionService != nil {
			callID := event.AggregateID
			session, serr := sessionService.Store.Get(ctx, callID)
			if serr == nil && boolFromMap(session.Metadata, "inQueue") {
				merchantIDQ := intFromMap(session.Metadata, "merchantId")
				skillGroupIDQ := intFromMap(session.Metadata, "skillGroupId")
				removed, rErr := queue.Remove(ctx, merchantIDQ, skillGroupIDQ, callID)
				if rErr != nil {
					logger.Error("客户中途挂机时从排队队列清理失败", "callId", callID, "merchantId", merchantIDQ, "skillGroupId", skillGroupIDQ, "error", rErr.Error())
				} else {
					session.Metadata["inQueue"] = false
					_ = sessionService.Store.Save(ctx, session)
					logger.Info("客户中途挂机，已原子清理排队队列", "callId", callID, "merchantId", merchantIDQ, "skillGroupId", skillGroupIDQ, "removed", removed)
				}
			}
		}

		logger.Info("消费 FS 事件并推进 ESL 流程", "eventId", event.EventID, "callId", event.AggregateID, "fsEvent", eventName)
		workflowID := eslWorkflowFromPayload(event.Payload)
		instance, err := eslRunner.Apply(ctx, workflowID, event.AggregateID, workflow.Event{Name: workflow.EventName(eventName), Payload: event.Payload})
		if err != nil {
			if errors.Is(err, workflow.ErrTransitionMissing) {
				logger.Info("FS 事件未命中当前流程状态，已忽略", "eventId", event.EventID, "callId", event.AggregateID, "fsEvent", eventName, "workflowId", workflowID)
				return nil
			}
			return err
		}
		if sessionService == nil || originate == nil {
			return nil
		}

		// 4.1 处理 API 外呼下的 FS 物理状态变迁
		if workflowID == esl.WorkflowESLAPIOutbound {
			switch eventName {
			case string(esl.EventChannelProgress), string(esl.EventChannelProgressMedia):
				// 主叫（坐席分机）振铃或收到媒体：发起对被叫（客户）的物理选号分配与呼叫起呼
				if legRole == string(contracts.LegRoleAgent) {
					return handleAPIOutboundAgentProgress(ctx, event, sessionService, originate, runtimeSelector, candidateSource, logger)
				}
				// 被叫（客户）振铃：在需要补振铃的情况下向坐席播放补振铃音
				if legRole == string(contracts.LegRoleCustomer) {
					if eventName == string(esl.EventChannelProgress) {
						return handleAPIOutboundCustomerProgress(ctx, event, sessionService, originate, logger)
					}
					return handleAPIOutboundCustomerReady(ctx, event, sessionService, originate, logger)
				}
			case string(esl.EventChannelAnswer):
				// 主叫（坐席）应答：标记主叫就绪并尝试两腿桥接
				if legRole == string(contracts.LegRoleAgent) {
					if err := handleAPIOutboundAgentAnswer(ctx, event, sessionService, originate, logger); err != nil {
						return err
					}
				}
				// 被叫（客户）应答：标记被叫就绪并尝试两腿桥接，同时切断补振铃音
				if legRole == string(contracts.LegRoleCustomer) {
					return handleAPIOutboundCustomerReady(ctx, event, sessionService, originate, logger)
				}
			}

			// 4.2 处理 批量外呼 下的 FS 物理状态变迁
		} else if workflowID == esl.WorkflowESLBatchOutbound || workflowID == esl.WorkflowESLBatchPredictive || workflowID == esl.WorkflowESLBatchSynergy {
			switch eventName {
			case string(esl.EventChannelProgress), string(esl.EventChannelProgressMedia):
				// 客户侧振铃（如果是协同外呼模式，振铃即起呼坐席）：
				if legRole == string(contracts.LegRoleCustomer) && workflowID == esl.WorkflowESLBatchSynergy {
					return handleBatchOutboundCustomerProgress(ctx, event, sessionService, originate, logger)
				}
				// 坐席侧振铃：在需要补振铃的情况下向客户腿播放补振铃音
				if legRole == string(contracts.LegRoleAgent) {
					if eventName == string(esl.EventChannelProgress) {
						return handleBatchOutboundAgentProgress(ctx, event, sessionService, originate, logger)
					}
					return handleBatchOutboundAgentReady(ctx, event, sessionService, originate, logger)
				}
			case string(esl.EventChannelAnswer):
				// 客户应答：
				if legRole == string(contracts.LegRoleCustomer) {
					if workflowID == esl.WorkflowESLBatchPredictive || workflowID == esl.WorkflowESLBatchOutbound {
						return handleBatchOutboundCustomerAnswer(ctx, event, sessionService, originate, batchRepo, queue, logger)
					}
				}
				// 坐席应答：切断补振铃并桥接双腿
				if legRole == string(contracts.LegRoleAgent) {
					return handleBatchOutboundAgentReady(ctx, event, sessionService, originate, logger)
				}
			}

			// 4.3 处理 拨号盘直呼 下的 FS 物理状态变迁
		} else if workflowID == esl.WorkflowESLDialpadDirect {
			switch eventName {
			case string(esl.EventChannelAnswer):
				// 坐席分机摘机/应答：触发对客户电话的物理选号与呼出起呼
				if legRole == string(contracts.LegRoleAgent) {
					return handleDialpadAgentAnswer(ctx, event, sessionService, originate, runtimeSelector, candidateSource, logger)
				}
				// 客户侧应答：停止补振铃并桥接双腿
				if legRole == string(contracts.LegRoleCustomer) {
					return handleDialpadCustomerReady(ctx, event, sessionService, originate, logger)
				}
			case string(esl.EventChannelProgress), string(esl.EventChannelProgressMedia):
				// 客户侧振铃：播放补振铃音
				if legRole == string(contracts.LegRoleCustomer) {
					if eventName == string(esl.EventChannelProgress) {
						return handleDialpadCustomerProgress(ctx, event, sessionService, originate, logger)
					}
					return handleDialpadCustomerReady(ctx, event, sessionService, originate, logger)
				}
			}

			// 4.4 处理 客户呼入 下的 FS 物理状态变迁
		} else if workflowID == esl.WorkflowESLInbound {
			switch eventName {
			case string(esl.EventChannelAnswer):
				// 客户呼入应答：触发自动分配并起呼分配到的坐席分机
				if legRole == string(contracts.LegRoleCustomer) {
					return handleInboundCustomerAnswer(ctx, event, sessionService, originate, statusReader, logger)
				}
				// 坐席摘机应答：桥接呼入电话与坐席
				if legRole == string(contracts.LegRoleAgent) {
					return handleInboundAgentReady(ctx, event, sessionService, originate, logger)
				}
			}
		}
		_ = instance
		return nil
	})

	// 4.5 订阅智能 IVR ASR/STT 识别文本事件，驱动可视化寻路引擎
	bus.Subscribe("asr_speech_detected", func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		callID := event.AggregateID
		text := stringFromMap(event.Payload, "text")
		session, err := sessionService.Store.Get(ctx, callID)
		if err != nil {
			return nil
		}
		if boolFromMap(session.Metadata, "aiEnabled") {
			var flow operate.AIModelFlow
			if flowJSON, ok := session.Metadata["aiFlowData"].(string); ok && flowJSON != "" {
				_ = json.Unmarshal([]byte(flowJSON), &flow)
			}
			if flow.FlowGraph != nil {
				engine := NewAIVoiceEngine(originate.CommandService, sessionService.Store, statusReader, logger)
				_ = engine.ProcessASRText(ctx, &session, flow, text)
			}
		}
		return nil
	})

	// 4.6 订阅智能 IVR DTMF 按键事件，驱动按键条件分支
	bus.Subscribe("dtmf_detected", func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		callID := event.AggregateID
		digit := stringFromMap(event.Payload, "digit")
		session, err := sessionService.Store.Get(ctx, callID)
		if err != nil {
			return nil
		}
		if boolFromMap(session.Metadata, "aiEnabled") {
			var flow operate.AIModelFlow
			if flowJSON, ok := session.Metadata["aiFlowData"].(string); ok && flowJSON != "" {
				_ = json.Unmarshal([]byte(flowJSON), &flow)
			}
			if flow.FlowGraph != nil {
				engine := NewAIVoiceEngine(originate.CommandService, sessionService.Store, statusReader, logger)
				_ = engine.ProcessDTMFKey(ctx, &session, flow, digit)
			}
		}
		return nil
	})

	// 5. 订阅并处理 Outbox 完成事件：使计费、录音、投影等后置处理流与 CTI 状态机彻底同步。
	outboxEvents := []string{
		"cdr_persisted",
		"billing_completed",
		"recording_completed",
		"push_completed",
		"callback_completed",
	}
	for _, eventType := range outboxEvents {
		eventType := eventType // 捕获迭代变量
		bus.Subscribe(eventType, func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
			logger.Info("消费 outbox 完成事件并推进 CTI 流程", "eventId", event.EventID, "callId", event.AggregateID, "eventType", event.EventType)
			workflowID := ctiWorkflowFromPayload(event.Payload)
			_, err := ctiRunner.Apply(ctx, workflowID, event.AggregateID, workflow.Event{Name: workflow.EventName(eventType), Payload: event.Payload})
			if err != nil {
				if errors.Is(err, workflow.ErrTransitionMissing) {
					logger.Info("CTI 流程未命中当前状态，已忽略", "eventId", event.EventID, "callId", event.AggregateID, "eventType", event.EventType, "workflowId", workflowID)
					return nil
				}
				return err
			}
			return nil
		})
	}
}

// RegisterBatchConsumers 注册批量外呼终结后的流程消费者。
// 该消费者只处理流程推进和调度服务调用，消息推送、回调、计费等后续节点应继续订阅
// EventBatchCallTelCompleted 或 CDR/outbox 事件，避免把多个业务副作用塞进一个消费者。
func RegisterBatchConsumers(bus events.Bus, ctiRunner *workflow.Runner, scheduler *cti.BatchSchedulerService, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	if scheduler == nil {
		logger.Warn("批量外呼终结消费者未注册，调度器为空")
		return
	}

	// 监听挂断事件以驱动批量调度器外呼下一号码或判定任务 drained
	bus.Subscribe(contracts.EventFSApplied, func(ctx context.Context, event contracts.EventEnvelope[map[string]any]) error {
		wf := eslWorkflowFromPayload(event.Payload)
		if wf != esl.WorkflowESLBatchOutbound && wf != esl.WorkflowESLBatchPredictive && wf != esl.WorkflowESLBatchSynergy {
			return nil
		}
		eventName, _ := event.Payload["eventName"].(string)
		if eventName != string(esl.EventChannelHangupComplete) {
			return nil
		}
		logger.Info("消费批量外呼终结 FS 事件并推进 CTI 流程", "eventId", event.EventID, "callId", event.AggregateID, "batchTaskId", event.Payload["batchTaskId"], "batchCallTelId", event.Payload["batchCallTelId"])
		ctiWF := ctiWorkflowFromPayload(event.Payload)
		if _, err := ctiRunner.Apply(ctx, ctiWF, event.AggregateID, workflow.Event{Name: "terminal_event", Payload: event.Payload}); err != nil {
			return err
		}
		return scheduler.HandleTerminal(ctx, event.Payload)
	})

	bus.Subscribe(contracts.EventBatchCallTelCompleted, func(_ context.Context, event contracts.EventEnvelope[map[string]any]) error {
		logger.Info("消费批量外呼号码完成事件，等待投影/推送/计费节点处理", "eventId", event.EventID, "aggregateId", event.AggregateID, "callId", event.Payload["callId"], "batchTaskId", event.Payload["batchTaskId"], "batchCallTelId", event.Payload["batchCallTelId"])
		return nil
	})

	bus.Subscribe(contracts.EventBatchCallTaskCompleted, func(_ context.Context, event contracts.EventEnvelope[map[string]any]) error {
		logger.Info("消费批量外呼任务完成事件，等待任务统计/消息推送/回调投影节点处理", "eventId", event.EventID, "aggregateId", event.AggregateID, "batchTaskId", event.Payload["batchTaskId"])
		return nil
	})
}

// eslWorkflowFromPayload 根据事件 Payload 自动判断对应的 ESL 工作流标识符。
func eslWorkflowFromPayload(payload map[string]any) string {
	profile, _ := payload["profile"].(string)
	if profile == string(contracts.CallFlowBatchOutbound) {
		return esl.WorkflowESLBatchOutbound
	}
	if profile == string(contracts.CallFlowBatchPredictive) {
		return esl.WorkflowESLBatchPredictive
	}
	if profile == string(contracts.CallFlowBatchSynergy) {
		return esl.WorkflowESLBatchSynergy
	}
	if profile == string(contracts.CallFlowAPIDirect) {
		return esl.WorkflowESLDialpadDirect
	}
	if profile == string(contracts.CallFlowInbound) {
		return esl.WorkflowESLInbound
	}
	if callModeVal, ok := payload["callMode"]; ok {
		var mode int
		switch v := callModeVal.(type) {
		case int:
			mode = v
		case float64:
			mode = int(v)
		}
		if mode == 1 {
			return esl.WorkflowESLBatchPredictive
		} else if mode == 2 {
			return esl.WorkflowESLBatchSynergy
		}
	}
	if _, ok := payload["batchTaskId"]; ok {
		return esl.WorkflowESLBatchOutbound
	}
	return esl.WorkflowESLAPIOutbound
}

// ctiWorkflowFromPayload 根据事件 Payload 自动判断对应的 CTI 工作流标识符。
func ctiWorkflowFromPayload(payload map[string]any) string {
	profile, _ := payload["profile"].(string)
	if profile == string(contracts.CallFlowBatchOutbound) {
		return cti.WorkflowBatchOutbound
	}
	if profile == string(contracts.CallFlowBatchPredictive) {
		return cti.WorkflowBatchPredictive
	}
	if profile == string(contracts.CallFlowBatchSynergy) {
		return cti.WorkflowBatchSynergy
	}
	if profile == string(contracts.CallFlowAPIDirect) {
		return cti.WorkflowDialpadDirect
	}
	if profile == string(contracts.CallFlowInbound) {
		return cti.WorkflowInbound
	}
	if callModeVal, ok := payload["callMode"]; ok {
		var mode int
		switch v := callModeVal.(type) {
		case int:
			mode = v
		case float64:
			mode = int(v)
		}
		if mode == 1 {
			return cti.WorkflowBatchPredictive
		} else if mode == 2 {
			return cti.WorkflowBatchSynergy
		}
	}
	if _, ok := payload["batchTaskId"]; ok {
		return cti.WorkflowBatchOutbound
	}
	return cti.WorkflowAPIOutbound
}

// handleAPIOutboundAgentProgress 处理 API 外呼主叫（坐席腿）振铃事件。
// 此处自动调用 RuntimeSelector 规则链获取可用呼出号码，并立即向 FreeSWITCH 发起客户腿（Leg B）的外呼 originate 命令。
// 如果在此阶段选号失败（如无可用号码、并发超限、网关失效），为了避免坐席话机无限盲目等待，
// 必须立即下发“hangup”挂断信令，主动切断主叫坐席腿，确保系统资源得到安全释放，保证通信收口一致性。
func handleAPIOutboundAgentProgress(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, runtimeSelector *cti.RuntimeSelector, candidateSource cti.CandidateSource, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if boolFromMap(session.Metadata, "customerOriginateSent") {
		return nil
	}
	userID := intFromMap(session.Metadata, "userId")
	merchantID := intFromMap(session.Metadata, "merchantId")
	callee, _ := session.Metadata["callee"].(string)
	if callee == "" || userID <= 0 {
		return esl.ErrInvalidCommand
	}

	agentUUID, _ := session.Metadata["agentUuid"].(string)
	if agentUUID == "" {
		agentUUID, _ = event.Payload["uuid"].(string)
	}

	// 挂断清除函数：当发生风控阻断或物理选号不可用时，优雅挂断坐席通道
	hangupAgent := func(reason string) {
		if agentUUID != "" {
			hangupCmd := telephony.NewCommand(
				"hangup:"+callID+":cleanup_api",
				"hangup",
				callID,
				agentUUID,
				session.FSAddr,
				contracts.LegRoleAgent,
				session.Profile,
				map[string]any{"cause": "NORMAL_TEMPORARY_FAILURE", "reason": reason},
			)
			if herr := originate.CommandService.Execute(ctx, hangupCmd); herr == nil {
				logger.Warn("API 外呼因异常已挂断正在交互中的坐席", "callId", callID, "agentUuid", agentUUID, "reason", reason)
			} else {
				logger.Error("API 外呼挂断坐席失败", "callId", callID, "agentUuid", agentUUID, "error", herr.Error())
			}
		}
	}

	// 加载可选主叫列表
	candidates, err := loadAPICandidates(ctx, candidateSource, userID)
	if err != nil {
		hangupAgent("load_candidates_failed: " + err.Error())
		return err
	}
	if len(candidates) == 0 {
		hangupAgent("no_available_number_candidates")
		return cti.ErrNoAvailableNumber
	}
	if runtimeSelector == nil {
		logger.Warn("API 外呼事件消费者缺少运行时选号分配器，拒绝继续起客户腿", "callId", callID, "merchantId", merchantID)
		hangupAgent("allocator_not_configured")
		return cti.ErrRuntimeAllocatorNotConfigured
	}

	// 选号并占用通道并发槽位
	selectionReq := cti.SelectionRequest{
		CallID:     callID,
		MerchantID: strconv.Itoa(merchantID),
		UserID:     userID,
		Callee:     callee,
		Candidates: candidates,
	}
	var selection cti.SelectionResult
	var allocation *cti.RuntimeAllocation
	selection, allocation, err = runtimeSelector.SelectAndClaim(ctx, selectionReq)
	if err != nil {
		hangupAgent("select_and_claim_failed: " + err.Error())
		return err
	}
	if !selection.Success || selection.Caller == nil {
		hangupAgent("no_available_selected_number")
		return cti.ErrNoAvailableNumber
	}

	if allocation != nil {
		session.Metadata["selectionClaimKey"] = allocation.ClaimKey
		session.Metadata["selectedCaller"] = allocation.Caller
		session.Metadata["selectedGatewayId"] = allocation.GatewayID
	}

	// 发起客户腿（Leg B）的 originate 外呼
	selectionResp := selectionFromCandidate(*selection.Caller)
	if err := originate.StartAPICustomerOutbound(ctx, esl.APICustomerOriginateRequest{Version: stringFromMap(session.Metadata, "routeVersion"), CallID: callID, Selection: selectionResp}); err != nil {
		if allocation != nil {
			logger.Warn("API 外呼发起客户腿失败，立即原子释放运行时选号并发槽位", "callId", callID, "caller", allocation.Caller, "gatewayId", allocation.GatewayID)
			_ = runtimeSelector.Allocator.Release(ctx, *allocation)
		}
		hangupAgent("originate_customer_failed: " + err.Error())
		return err
	}

	// 保存外呼路由选择与状态
	session.Metadata["customerOriginateSent"] = true
	session.Metadata["selectedCaller"] = selectionResp.Phone
	session.Metadata["selectedGatewayId"] = selectionResp.GatewayID
	session.Metadata["selectedGatewayName"] = selectionResp.GatewayName
	session.Metadata["selectedGatewayRegion"] = selectionResp.GatewayRegion
	session.Metadata["selectedModel"] = selectionResp.Model
	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("API 外呼已完成选号并发起客户腿", "callId", callID, "caller", selectionResp.Phone, "gatewayId", selectionResp.GatewayID, "gatewayName", selectionResp.GatewayName, "gatewayRegion", selectionResp.GatewayRegion)
	return nil
}

// handleAPIOutboundAgentAnswer 处理 API 外呼主叫（坐席侧）应答事件。
// 当坐席已接听电话时，如果被叫客户侧已经振铃但尚未应答，为了提升坐席感知，可立即向坐席下发播放补振铃音命令。
func handleAPIOutboundAgentAnswer(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	session.Metadata["agentAnswered"] = true

	// 若被叫已经振铃，当主叫应答时，立即向主叫播放补振铃
	if boolFromMap(session.Metadata, "supplementRing") && !boolFromMap(session.Metadata, "supplementRingPlaying") && session.State == esl.CallProgress {
		supplementRingFile, _ := session.Metadata["supplementRingFile"].(string)
		agentUUID, _ := session.Metadata["agentUuid"].(string)
		if supplementRingFile != "" && agentUUID != "" {
			cmd := telephony.NewCommand(
				"playback:"+callID+":supplement_ring",
				"playback",
				callID,
				agentUUID,
				session.FSAddr,
				contracts.LegRoleAgent,
				session.Profile,
				map[string]any{
					"file": supplementRingFile,
					"both": "aleg",
				},
			)
			if err := originate.CommandService.Execute(ctx, cmd); err == nil {
				session.Metadata["supplementRingPlaying"] = true
				logger.Info("API 外呼主叫(坐席)应答时被叫已振铃，向主叫发送补振铃", "callId", callID, "agentUuid", agentUUID, "file", supplementRingFile)
			} else {
				logger.Error("API 外呼主叫应答时发送补振铃失败", "callId", callID, "error", err.Error())
			}
		}
	}

	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	return maybeBridgeAPIOutbound(ctx, session, originate, logger, "agent_answer")
}

// handleAPIOutboundCustomerReady 处理 API 外呼被叫（客户侧）振铃或就绪事件。
// 一旦客户准备好或应答，立即切断向坐席侧播放的补振铃音，并在两腿同时就绪时触发桥接命令。
func handleAPIOutboundCustomerReady(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	session.Metadata["customerReady"] = true

	// 停止主叫(坐席)侧播放的补振铃音
	if boolFromMap(session.Metadata, "supplementRingPlaying") {
		agentUUID, _ := session.Metadata["agentUuid"].(string)
		if agentUUID != "" {
			breakCmd := telephony.NewCommand(
				"break:"+callID+":stop_supplement",
				"break",
				callID,
				agentUUID,
				session.FSAddr,
				contracts.LegRoleAgent,
				contracts.CallFlowAPIOutbound,
				map[string]any{},
			)
			if err := originate.CommandService.Execute(ctx, breakCmd); err == nil {
				session.Metadata["supplementRingPlaying"] = false
				logger.Info("停止主叫(坐席)腿的补振铃", "callId", callID, "agentUuid", agentUUID)
			} else {
				logger.Error("停止主叫腿的补振铃失败", "callId", callID, "error", err.Error())
			}
		}
	}

	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	return maybeBridgeAPIOutbound(ctx, session, originate, logger, stringValue(event.Payload, "eventName"))
}

// maybeBridgeAPIOutbound 条件桥接。当主叫和被叫均完成应答后，向 FreeSWITCH 下发双腿桥接命令以合并通话媒体。
func maybeBridgeAPIOutbound(ctx context.Context, session esl.CallSession, originate *esl.OriginateService, logger *slog.Logger, reason string) error {
	if boolFromMap(session.Metadata, "apiBridgeSent") {
		return nil
	}
	if !boolFromMap(session.Metadata, "customerOriginateSent") {
		return nil
	}
	if !boolFromMap(session.Metadata, "agentAnswered") || !boolFromMap(session.Metadata, "customerReady") {
		logger.Info("API 外呼两腿尚未同时就绪，暂不桥接", "callId", session.CallID, "reason", reason, "agentAnswered", boolFromMap(session.Metadata, "agentAnswered"), "customerReady", boolFromMap(session.Metadata, "customerReady"))
		return nil
	}
	agentUUID, _ := session.Metadata["agentUuid"].(string)
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	if agentUUID == "" || customerUUID == "" {
		return esl.ErrInvalidCommand
	}
	if err := originate.BridgeAPIOutbound(ctx, esl.APIBridgeRequest{CallID: session.CallID, AgentUUID: agentUUID, CustomerUUID: customerUUID, FSAddr: session.FSAddr}); err != nil {
		return err
	}
	session.Metadata["apiBridgeSent"] = true
	if err := originate.SessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("API 外呼两腿桥接已完成", "callId", session.CallID, "reason", reason, "agentUuid", agentUUID, "customerUuid", customerUUID)
	return nil
}

// loadAPICandidates 获取某用户的可选主叫列表。
func loadAPICandidates(ctx context.Context, source cti.CandidateSource, userID int) ([]cti.NumberCandidate, error) {
	if source == nil {
		return nil, esl.ErrInvalidCommand
	}
	return source.CandidatesForUser(ctx, userID)
}

// selectionFromCandidate 候选号码类型格式化映射。
func selectionFromCandidate(candidate cti.NumberCandidate) contracts.SelectPhoneResp {
	return contracts.SelectPhoneResp{
		Phone:              candidate.Phone,
		GatewayID:          atoi(candidate.GatewayID),
		SkillGroupID:       candidate.SkillGroupID,
		ChannelID:          candidate.ChannelID,
		GatewayName:        candidate.GatewayName,
		GatewayRegion:      candidate.GatewayRegion,
		Model:              candidate.Model,
		CallerPrefix:       candidate.CallerPrefix,
		CalleePrefix:       candidate.CalleePrefix,
		CallerRewriteRule:  candidate.CallerRewriteRule,
		CalleeRewriteRule:  candidate.CalleeRewriteRule,
		SupplementRing:     candidate.SupplementRing,
		SupplementRingFile: candidate.SupplementRingFile,
		Province:           candidate.Province,
		City:               candidate.City,
		PoolID:             candidate.PoolID,
		CodecPrefs:         candidate.CodecPrefs,
		BroadcastTime:      candidate.BroadcastTime,
		BroadcastTimeFlag:  candidate.BroadcastTimeFlag,
	}
}

// stringFromMap 工具方法，从 Payload 中安全截取 String。
func stringFromMap(payload map[string]any, key string) string {
	if value, ok := payload[key]; ok && value != nil {
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

func stringValue(payload map[string]any, key string) string {
	return stringFromMap(payload, key)
}

// boolFromMap 工具方法，安全截取布尔值。
func boolFromMap(payload map[string]any, key string) bool {
	if value, ok := payload[key]; ok && value != nil {
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			return typed == "true" || typed == "1" || typed == "yes"
		}
	}
	return false
}

// intFromMap 工具方法，安全截取整形值。
func intFromMap(payload map[string]any, key string) int {
	if value, ok := payload[key]; ok && value != nil {
		switch typed := value.(type) {
		case int:
			return typed
		case int8:
			return int(typed)
		case int16:
			return int(typed)
		case int32:
			return int(typed)
		case int64:
			return int(typed)
		case float32:
			return int(typed)
		case float64:
			return int(typed)
		}
	}
	return 0
}

func atoi(raw string) int {
	value, _ := strconv.Atoi(raw)
	return value
}

// handleAPIOutboundCustomerProgress 处理 API 外呼被叫客户侧振铃补振铃动作。
func handleAPIOutboundCustomerProgress(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	if boolFromMap(session.Metadata, "supplementRingPlaying") {
		return nil
	}
	if !boolFromMap(session.Metadata, "supplementRing") {
		return nil
	}
	supplementRingFile, _ := session.Metadata["supplementRingFile"].(string)
	if supplementRingFile == "" {
		return nil
	}
	if !boolFromMap(session.Metadata, "agentAnswered") {
		logger.Info("API 外呼被叫(客户)已振铃(180)，但主叫(坐席)未应答，暂不发送补振铃", "callId", callID)
		return nil
	}
	agentUUID, _ := session.Metadata["agentUuid"].(string)
	if agentUUID == "" {
		return esl.ErrInvalidCommand
	}
	cmd := telephony.NewCommand(
		"playback:"+callID+":supplement_ring",
		"playback",
		callID,
		agentUUID,
		session.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowAPIOutbound,
		map[string]any{
			"file": supplementRingFile,
			"both": "aleg",
		},
	)
	if err := originate.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("API 外呼发送补振铃命令失败", "callId", callID, "error", err.Error())
		return err
	}
	session.Metadata["supplementRingPlaying"] = true
	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("API 外呼已向主叫(坐席)腿发送补振铃", "callId", callID, "agentUuid", agentUUID, "file", supplementRingFile)
	return nil
}

// handleBatchOutboundCustomerAnswer 处理批量外呼客户应答（Leg A 先应答）。
// 批量外呼遵循 Customer-First 规范：一旦客户接听电话，立即发起呼叫绑定分机以合并坐席（Leg B）的 originate 动作。
func handleBatchOutboundCustomerAnswer(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, batchRepo cti.BatchTaskRepository, queue cti.CallQueue, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	session.Metadata["customerReady"] = true
	if boolFromMap(session.Metadata, "agentOriginateSent") {
		return nil
	}

	userID := intFromMap(session.Metadata, "userId")
	merchantID := intFromMap(session.Metadata, "merchantId")
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	if customerUUID == "" {
		customerUUID, _ = event.Payload["uuid"].(string)
		session.Metadata["customerUuid"] = customerUUID
	}
	callMode := intFromMap(session.Metadata, "callMode")

	var extension string
	var agentUUID string

	if callMode == 1 { // 预测模式
		skillGroupID := intFromMap(session.Metadata, "skillGroupId")
		if skillGroupID <= 0 {
			logger.Warn("预测外呼模式缺少 skillGroupId，无法分配坐席", "callId", callID)
			return esl.ErrInvalidCommand
		}

		var idleUserID int
		var idleExtension string
		if batchRepo != nil {
			idleUserID, idleExtension, err = batchRepo.GetIdleAgentFromSkillGroup(ctx, skillGroupID)
			if err != nil {
				logger.Error("预测外呼获取空闲坐席失败", "callId", callID, "skillGroupId", skillGroupID, "error", err.Error())
				return err
			}
		}

		if idleExtension != "" {
			extension = idleExtension
			agentUUID = esl.NewDeterministicUUID("agent", callID)
			session.Metadata["extension"] = extension
			session.Metadata["agentUuid"] = agentUUID
			session.Metadata["userId"] = idleUserID
			logger.Info("预测外呼成功分配到空闲坐席", "callId", callID, "extension", extension, "userId", idleUserID)
		} else {
			queueEnable := boolFromMap(session.Metadata, "queueEnable")
			if queueEnable && queue != nil {
				logger.Info("预测外呼无空闲坐席，客户进入排队队列", "callId", callID, "skillGroupId", skillGroupID, "merchantId", merchantID)
				// 使用多租户前缀推入队列
				if qerr := queue.Push(ctx, merchantID, skillGroupID, callID); qerr != nil {
					logger.Error("推送呼叫到排队队列失败", "callId", callID, "merchantId", merchantID, "error", qerr.Error())
					return qerr
				}

				waitMusic := "local_stream://default"
				if music, ok := session.Metadata["supplementRingFile"].(string); ok && music != "" {
					waitMusic = music
				}
				playCmd := telephony.NewCommand(
					"playback:"+callID+":queue_wait",
					"playback",
					callID,
					customerUUID,
					session.FSAddr,
					contracts.LegRoleCustomer,
					contracts.CallFlowBatchPredictive,
					map[string]any{
						"file": waitMusic,
						"both": "aleg",
					},
				)
				if perr := originate.CommandService.Execute(ctx, playCmd); perr != nil {
					logger.Error("向客户播放排队等待音失败", "callId", callID, "error", perr.Error())
				} else {
					session.Metadata["queueWaitPlaying"] = true
					logger.Info("已向客户腿发送排队等待音播放指令", "callId", callID, "file", waitMusic)
				}

				session.Metadata["inQueue"] = true
				if serr := sessionService.Store.Save(ctx, session); serr != nil {
					return serr
				}

				// 启动排队超时协程：30 秒后如果客户仍在队列中（LRem 返回 >0），则自动挂断并播放道歉提示。
				// 若客户已被坐席接听（Remove 返回 0），则不做任何操作，避免误挂。
				capturedCallID := callID
				capturedCustomerUUID := customerUUID
				capturedFSAddr := session.FSAddr
				capturedMerchantID := merchantID
				capturedSkillGroupID := skillGroupID
				go func() {
					const queueWaitTimeout = 30 * time.Second
					time.Sleep(queueWaitTimeout)
					bgCtx := context.Background()

					// 原子从队列中移除，返回 removed>0 说明客户仍在等待（超时需挂断）
					removed, rErr := queue.Remove(bgCtx, capturedMerchantID, capturedSkillGroupID, capturedCallID)
					if rErr != nil {
						logger.Error("排队超时：从队列清理失败", "callId", capturedCallID, "error", rErr.Error())
						return
					}
					if removed <= 0 {
						// 已被坐席接听或已提前挂断，无需处理
						logger.Info("排队超时检查：呼叫已离队，无需超时挂断", "callId", capturedCallID)
						return
					}
					// 客户仍在队列中等待，超时挂断
					logger.Warn("排队等待超时，系统自动挂断客户腿", "callId", capturedCallID, "merchantId", capturedMerchantID, "skillGroupId", capturedSkillGroupID)
					hangupCmd := telephony.NewCommand(
						"hangup:"+capturedCallID+":queue_timeout",
						"hangup",
						capturedCallID,
						capturedCustomerUUID,
						capturedFSAddr,
						contracts.LegRoleCustomer,
						contracts.CallFlowBatchPredictive,
						map[string]any{"cause": "NO_ANSWER", "reason": "queue_wait_timeout"},
					)
					if herr := originate.CommandService.Execute(bgCtx, hangupCmd); herr != nil {
						logger.Error("排队超时挂断客户腿失败", "callId", capturedCallID, "error", herr.Error())
					} else {
						logger.Info("排队超时已成功挂断客户腿", "callId", capturedCallID)
					}
				}()

				return nil
			} else {
				logger.Warn("预测外呼无空闲坐席且未启用排队，立即挂断客户", "callId", callID)
				hangupCmd := telephony.NewCommand(
					"hangup:"+callID+":no_agent",
					"hangup",
					callID,
					customerUUID,
					session.FSAddr,
					contracts.LegRoleCustomer,
					contracts.CallFlowBatchPredictive,
					map[string]any{"cause": "NO_ANSWER", "reason": "no_idle_agent_and_queue_disabled"},
				)
				if herr := originate.CommandService.Execute(ctx, hangupCmd); herr != nil {
					logger.Error("挂断客户腿失败", "callId", callID, "error", herr.Error())
				}
				return nil
			}
		}
	} else {
		extension, _ = session.Metadata["extension"].(string)
		agentUUID, _ = session.Metadata["agentUuid"].(string)
	}

	if extension == "" || agentUUID == "" || customerUUID == "" {
		return esl.ErrInvalidCommand
	}

	req := esl.BatchAgentOriginateRequest{
		Version:      stringFromMap(session.Metadata, "routeVersion"),
		CallID:       callID,
		Extension:    extension,
		AgentUUID:    agentUUID,
		CustomerUUID: customerUUID,
		FSAddr:       session.FSAddr,
		UserID:       userID,
		MerchantID:   merchantID,
	}

	if err := originate.StartBatchAgentOutbound(ctx, req); err != nil {
		return err
	}

	session.Metadata["agentOriginateSent"] = true
	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("批量外呼已完成被叫(客户)应答并发起主叫(坐席)腿", "callId", callID, "extension", extension)
	return nil
}

// handleBatchOutboundCustomerProgress 处理批量外呼客户振铃（协同模式，振铃即起呼坐席）。
func handleBatchOutboundCustomerProgress(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	if boolFromMap(session.Metadata, "agentOriginateSent") {
		return nil
	}

	userID := intFromMap(session.Metadata, "userId")
	merchantID := intFromMap(session.Metadata, "merchantId")
	extension, _ := session.Metadata["extension"].(string)
	agentUUID, _ := session.Metadata["agentUuid"].(string)
	customerUUID, _ := session.Metadata["customerUuid"].(string)

	if extension == "" || agentUUID == "" || customerUUID == "" {
		return esl.ErrInvalidCommand
	}

	req := esl.BatchAgentOriginateRequest{
		Version:      stringFromMap(session.Metadata, "routeVersion"),
		CallID:       callID,
		Extension:    extension,
		AgentUUID:    agentUUID,
		CustomerUUID: customerUUID,
		FSAddr:       session.FSAddr,
		UserID:       userID,
		MerchantID:   merchantID,
	}

	if err := originate.StartBatchAgentOutbound(ctx, req); err != nil {
		return err
	}

	session.Metadata["agentOriginateSent"] = true
	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("协同批量外呼已收到被叫(客户)振铃并提前发起主叫(坐席)腿", "callId", callID, "extension", extension)
	return nil
}

// handleBatchOutboundAgentProgress 批量外呼下坐席分机侧振铃，向客户侧播放补振铃音。
func handleBatchOutboundAgentProgress(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	if boolFromMap(session.Metadata, "supplementRingPlaying") {
		return nil
	}
	if !boolFromMap(session.Metadata, "supplementRing") {
		return nil
	}
	supplementRingFile, _ := session.Metadata["supplementRingFile"].(string)
	if supplementRingFile == "" {
		return nil
	}
	if !boolFromMap(session.Metadata, "customerReady") {
		logger.Info("批量外呼主叫(坐席)腿已振铃(180)，但被叫(客户)未应答，暂不发送补振铃", "callId", callID)
		return nil
	}
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	if customerUUID == "" {
		return esl.ErrInvalidCommand
	}
	cmd := telephony.NewCommand(
		"playback:"+callID+":supplement_ring",
		"playback",
		callID,
		customerUUID,
		session.FSAddr,
		contracts.LegRoleCustomer,
		contracts.CallFlowBatchOutbound,
		map[string]any{
			"file": supplementRingFile,
			"both": "aleg",
		},
	)
	if err := originate.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("批量外呼发送补振铃命令失败", "callId", callID, "error", err.Error())
		return err
	}
	session.Metadata["supplementRingPlaying"] = true
	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("批量外呼已向被叫(客户)腿发送补振铃", "callId", callID, "customerUuid", customerUUID, "file", supplementRingFile)
	return nil
}

// handleBatchOutboundAgentReady 批量外呼下坐席分机侧摘机应答。切断向客户腿播放的补振铃并桥接。
func handleBatchOutboundAgentReady(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	session.Metadata["agentAnswered"] = true

	// 停止被叫(客户)侧播放的补振铃音
	if boolFromMap(session.Metadata, "supplementRingPlaying") {
		customerUUID, _ := session.Metadata["customerUuid"].(string)
		if customerUUID != "" {
			breakCmd := telephony.NewCommand(
				"break:"+callID+":stop_supplement",
				"break",
				callID,
				customerUUID,
				session.FSAddr,
				contracts.LegRoleCustomer,
				contracts.CallFlowBatchOutbound,
				map[string]any{},
			)
			if err := originate.CommandService.Execute(ctx, breakCmd); err == nil {
				session.Metadata["supplementRingPlaying"] = false
				logger.Info("停止被叫(客户)腿的补振铃", "callId", callID, "customerUuid", customerUUID)
			} else {
				logger.Error("停止被叫腿的补振铃失败", "callId", callID, "error", err.Error())
			}
		}
	}

	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	return maybeBridgeBatchOutbound(ctx, session, originate, logger, stringValue(event.Payload, "eventName"))
}

// maybeBridgeBatchOutbound 批量外呼桥接动作下发。
func maybeBridgeBatchOutbound(ctx context.Context, session esl.CallSession, originate *esl.OriginateService, logger *slog.Logger, reason string) error {
	if boolFromMap(session.Metadata, "batchBridgeSent") {
		return nil
	}
	if !boolFromMap(session.Metadata, "agentOriginateSent") {
		return nil
	}
	if !boolFromMap(session.Metadata, "agentAnswered") || !boolFromMap(session.Metadata, "customerReady") {
		logger.Info("批量外呼两腿尚未同时就绪，暂不桥接", "callId", session.CallID, "reason", reason, "agentAnswered", boolFromMap(session.Metadata, "agentAnswered"), "customerReady", boolFromMap(session.Metadata, "customerReady"))
		return nil
	}
	agentUUID, _ := session.Metadata["agentUuid"].(string)
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	if agentUUID == "" || customerUUID == "" {
		return esl.ErrInvalidCommand
	}
	if err := originate.BridgeBatchOutbound(ctx, esl.APIBridgeRequest{CallID: session.CallID, AgentUUID: agentUUID, CustomerUUID: customerUUID, FSAddr: session.FSAddr}); err != nil {
		return err
	}
	session.Metadata["batchBridgeSent"] = true
	if err := originate.SessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("批量外呼两腿桥接已完成", "callId", session.CallID, "reason", reason, "agentUuid", agentUUID, "customerUuid", customerUUID)
	return nil
}

// handleDialpadAgentAnswer 拨号盘直呼（坐席主动点拨号盘）下的坐席分机摘机。
// 此时自动通过选号器分配外呼号码，并对Leg B（客户侧）发起外呼起呼命令。
// 若选号故障同样自动 Hangup 切断摘机坐席的通道，进行收口清退。
func handleDialpadAgentAnswer(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, runtimeSelector *cti.RuntimeSelector, candidateSource cti.CandidateSource, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	session.Metadata["agentAnswered"] = true

	if boolFromMap(session.Metadata, "customerOriginateSent") {
		return nil
	}

	userID := intFromMap(session.Metadata, "userId")
	merchantID := intFromMap(session.Metadata, "merchantId")
	callee, _ := session.Metadata["callee"].(string)
	if callee == "" || userID <= 0 {
		return esl.ErrInvalidCommand
	}

	agentUUID, _ := session.Metadata["agentUuid"].(string)
	if agentUUID == "" {
		agentUUID, _ = event.Payload["uuid"].(string)
	}

	// 坐席清理动作
	hangupAgent := func(reason string) {
		if agentUUID != "" {
			hangupCmd := telephony.NewCommand(
				"hangup:"+callID+":cleanup_dialpad",
				"hangup",
				callID,
				agentUUID,
				session.FSAddr,
				contracts.LegRoleAgent,
				session.Profile,
				map[string]any{"cause": "NORMAL_TEMPORARY_FAILURE", "reason": reason},
			)
			if herr := originate.CommandService.Execute(ctx, hangupCmd); herr == nil {
				logger.Warn("拨号盘直呼因异常已挂断摘机坐席", "callId", callID, "agentUuid", agentUUID, "reason", reason)
			} else {
				logger.Error("拨号盘直呼挂断坐席失败", "callId", callID, "agentUuid", agentUUID, "error", herr.Error())
			}
		}
	}

	candidates, err := loadAPICandidates(ctx, candidateSource, userID)
	if err != nil {
		hangupAgent("load_candidates_failed: " + err.Error())
		return err
	}
	if len(candidates) == 0 {
		hangupAgent("no_available_number_candidates")
		return cti.ErrNoAvailableNumber
	}
	if runtimeSelector == nil {
		logger.Warn("拨号盘直呼消费者缺少运行时选号分配器，拒绝继续起客户腿", "callId", callID, "merchantId", merchantID)
		hangupAgent("allocator_not_configured")
		return cti.ErrRuntimeAllocatorNotConfigured
	}

	selectionReq := cti.SelectionRequest{
		CallID:     callID,
		MerchantID: strconv.Itoa(merchantID),
		UserID:     userID,
		Callee:     callee,
		Candidates: candidates,
	}

	var selection cti.SelectionResult
	var allocation *cti.RuntimeAllocation
	selection, allocation, err = runtimeSelector.SelectAndClaim(ctx, selectionReq)
	if err != nil {
		hangupAgent("select_and_claim_failed: " + err.Error())
		return err
	}
	if !selection.Success || selection.Caller == nil {
		hangupAgent("no_available_selected_number")
		return cti.ErrNoAvailableNumber
	}

	if allocation != nil {
		session.Metadata["selectionClaimKey"] = allocation.ClaimKey
		session.Metadata["selectedCaller"] = allocation.Caller
		session.Metadata["selectedGatewayId"] = allocation.GatewayID
	}

	customerUUID := esl.NewDeterministicUUID("customer", callID)
	session.Metadata["customerUuid"] = customerUUID

	selectionResp := selectionFromCandidate(*selection.Caller)
	if err := originate.StartDialpadCustomerOutbound(ctx, esl.APICustomerOriginateRequest{
		Version:   stringFromMap(session.Metadata, "routeVersion"),
		CallID:    callID,
		Selection: selectionResp,
	}); err != nil {
		if allocation != nil {
			logger.Warn("拨号盘直呼发起客户腿失败，立即原子释放运行时选号并发槽位", "callId", callID, "caller", allocation.Caller, "gatewayId", allocation.GatewayID)
			_ = runtimeSelector.Allocator.Release(ctx, *allocation)
		}
		hangupAgent("originate_customer_failed: " + err.Error())
		return err
	}

	session.Metadata["customerOriginateSent"] = true
	session.Metadata["selectedCaller"] = selectionResp.Phone
	session.Metadata["selectedGatewayId"] = selectionResp.GatewayID
	session.Metadata["selectedGatewayName"] = selectionResp.GatewayName
	session.Metadata["selectedGatewayRegion"] = selectionResp.GatewayRegion
	session.Metadata["selectedModel"] = selectionResp.Model
	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("拨号盘直呼已完成选号并发起客户腿", "callId", callID, "caller", selectionResp.Phone, "gatewayId", selectionResp.GatewayID, "gatewayName", selectionResp.GatewayName, "gatewayRegion", selectionResp.GatewayRegion)
	return nil
}

// handleDialpadCustomerProgress 拨号盘直呼下，客户侧振铃补振铃。
func handleDialpadCustomerProgress(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	if boolFromMap(session.Metadata, "supplementRingPlaying") {
		return nil
	}
	if !boolFromMap(session.Metadata, "supplementRing") {
		return nil
	}
	supplementRingFile, _ := session.Metadata["supplementRingFile"].(string)
	if supplementRingFile == "" {
		return nil
	}
	if !boolFromMap(session.Metadata, "agentAnswered") {
		logger.Info("拨号盘直呼被叫(客户)已振铃(180)，但主叫(坐席)未应答，暂不发送补振铃", "callId", callID)
		return nil
	}
	agentUUID, _ := session.Metadata["agentUuid"].(string)
	if agentUUID == "" {
		return esl.ErrInvalidCommand
	}
	cmd := telephony.NewCommand(
		"playback:"+callID+":supplement_ring",
		"playback",
		callID,
		agentUUID,
		session.FSAddr,
		contracts.LegRoleAgent,
		contracts.CallFlowAPIDirect,
		map[string]any{
			"file": supplementRingFile,
			"both": "aleg",
		},
	)
	if err := originate.CommandService.Execute(ctx, cmd); err != nil {
		logger.Error("拨号盘直呼发送补振铃命令失败", "callId", callID, "error", err.Error())
		return err
	}
	session.Metadata["supplementRingPlaying"] = true
	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("拨号盘直呼已向主叫(坐席)腿发送补振铃", "callId", callID, "agentUuid", agentUUID, "file", supplementRingFile)
	return nil
}

// handleDialpadCustomerReady 拨号盘直呼被叫（客户）应答或就绪。
func handleDialpadCustomerReady(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	session.Metadata["customerReady"] = true

	// 停止主叫(坐席)侧播放的补振铃音
	if boolFromMap(session.Metadata, "supplementRingPlaying") {
		agentUUID, _ := session.Metadata["agentUuid"].(string)
		if agentUUID != "" {
			breakCmd := telephony.NewCommand(
				"break:"+callID+":stop_supplement",
				"break",
				callID,
				agentUUID,
				session.FSAddr,
				contracts.LegRoleAgent,
				contracts.CallFlowAPIDirect,
				map[string]any{},
			)
			if err := originate.CommandService.Execute(ctx, breakCmd); err == nil {
				session.Metadata["supplementRingPlaying"] = false
				logger.Info("停止主叫(坐席)腿的补振铃", "callId", callID, "agentUuid", agentUUID)
			} else {
				logger.Error("停止主叫腿的补振铃失败", "callId", callID, "error", err.Error())
			}
		}
	}

	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	return maybeBridgeDialpadDirect(ctx, session, originate, logger, stringValue(event.Payload, "eventName"))
}

// maybeBridgeDialpadDirect 拨号盘直呼双腿桥接命令发送。
func maybeBridgeDialpadDirect(ctx context.Context, session esl.CallSession, originate *esl.OriginateService, logger *slog.Logger, reason string) error {
	if boolFromMap(session.Metadata, "apiBridgeSent") {
		return nil
	}
	if !boolFromMap(session.Metadata, "customerOriginateSent") {
		return nil
	}
	if !boolFromMap(session.Metadata, "agentAnswered") || !boolFromMap(session.Metadata, "customerReady") {
		logger.Info("拨号盘直呼两腿尚未同时就绪，暂不桥接", "callId", session.CallID, "reason", reason, "agentAnswered", boolFromMap(session.Metadata, "agentAnswered"), "customerReady", boolFromMap(session.Metadata, "customerReady"))
		return nil
	}
	agentUUID, _ := session.Metadata["agentUuid"].(string)
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	if agentUUID == "" || customerUUID == "" {
		return esl.ErrInvalidCommand
	}
	if err := originate.BridgeDialpadDirect(ctx, esl.APIBridgeRequest{CallID: session.CallID, AgentUUID: agentUUID, CustomerUUID: customerUUID, FSAddr: session.FSAddr}); err != nil {
		return err
	}
	session.Metadata["apiBridgeSent"] = true
	if err := originate.SessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("拨号盘直呼两腿桥接已完成", "callId", session.CallID, "reason", reason, "agentUuid", agentUUID, "customerUuid", customerUUID)
	return nil
}

// handleInboundCustomerAnswer 处理客户呼入应答动作（Leg A 先就绪）。
// 随后触发向绑定坐席分机的 originate 呼叫。
func handleInboundCustomerAnswer(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, statusReader esl.ExtensionStatusReader, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	session.Metadata["customerReady"] = true

	// 🚀 如果配置了启用机器人智能话术流 (AI Robot First)
	if boolFromMap(session.Metadata, "aiEnabled") {
		var flow operate.AIModelFlow
		if flowJSON, ok := session.Metadata["aiFlowData"].(string); ok && flowJSON != "" {
			_ = json.Unmarshal([]byte(flowJSON), &flow)
		}

		engine := NewAIVoiceEngine(originate.CommandService, sessionService.Store, statusReader, logger)
		err := engine.StartAIVoiceFlow(ctx, &session, flow)
		if err != nil {
			logger.Error("云枢运行时：AI 智能话术流启动失败，降级回退人工坐席", "error", err.Error())
		} else {
			session.Metadata["aiFlowActive"] = true
			session.Metadata["aiCurrentNode"] = "node-intent" // 默认停留在意图卡点监听
			return sessionService.Store.Save(ctx, session)
		}
	}

	if boolFromMap(session.Metadata, "agentOriginateSent") {
		return nil
	}

	userID := intFromMap(session.Metadata, "userId")
	merchantID := intFromMap(session.Metadata, "merchantId")
	extension, _ := session.Metadata["extension"].(string)
	if extension == "" {
		extension = "1001" // 默认测试分机
	}
	agentUUID := esl.NewDeterministicUUID("agent", callID)
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	if customerUUID == "" {
		customerUUID, _ = event.Payload["uuid"].(string)
		session.Metadata["customerUuid"] = customerUUID
	}

	req := esl.BatchAgentOriginateRequest{
		Version:      stringFromMap(session.Metadata, "routeVersion"),
		CallID:       callID,
		Extension:    extension,
		AgentUUID:    agentUUID,
		CustomerUUID: customerUUID,
		FSAddr:       session.FSAddr,
		UserID:       userID,
		MerchantID:   merchantID,
	}

	if err := originate.StartInboundAgentOutbound(ctx, req); err != nil {
		return err
	}

	session.Metadata["agentUuid"] = agentUUID
	session.Metadata["extension"] = extension
	session.Metadata["agentOriginateSent"] = true
	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("客户呼入已完成客户应答并发起坐席分机呼叫", "callId", callID, "extension", extension)
	return nil
}

// handleInboundAgentReady 客户呼入下，坐席应答摘机。
func handleInboundAgentReady(ctx context.Context, event contracts.EventEnvelope[map[string]any], sessionService *esl.SessionService, originate *esl.OriginateService, logger *slog.Logger) error {
	callID := event.AggregateID
	session, err := sessionService.Store.Get(ctx, callID)
	if err != nil {
		return err
	}
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	session.Metadata["agentAnswered"] = true

	if err := sessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	return maybeBridgeInbound(ctx, session, originate, logger, stringValue(event.Payload, "eventName"))
}

// maybeBridgeInbound 客户呼入下双腿媒体桥接发送。
func maybeBridgeInbound(ctx context.Context, session esl.CallSession, originate *esl.OriginateService, logger *slog.Logger, reason string) error {
	if boolFromMap(session.Metadata, "inboundBridgeSent") {
		return nil
	}
	if !boolFromMap(session.Metadata, "agentOriginateSent") {
		return nil
	}
	if !boolFromMap(session.Metadata, "agentAnswered") {
		logger.Info("客户呼入坐席分机尚未就绪，暂不桥接", "callId", session.CallID, "reason", reason)
		return nil
	}
	agentUUID, _ := session.Metadata["agentUuid"].(string)
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	if agentUUID == "" || customerUUID == "" {
		return esl.ErrInvalidCommand
	}
	if err := originate.BridgeInbound(ctx, esl.APIBridgeRequest{CallID: session.CallID, AgentUUID: agentUUID, CustomerUUID: customerUUID, FSAddr: session.FSAddr}); err != nil {
		return err
	}
	session.Metadata["inboundBridgeSent"] = true
	if err := originate.SessionService.Store.Save(ctx, session); err != nil {
		return err
	}
	logger.Info("客户呼入两腿桥接已完成", "callId", session.CallID, "reason", reason, "agentUuid", agentUUID, "customerUuid", customerUUID)
	return nil
}
