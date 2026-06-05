package callflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/esl"
	operatedomain "yunshu/internal/domain/operate"
	"yunshu/pkg/telephony"
)

// AIVoiceEngine 承载运行时 AI 话务拓扑寻路与 ESL 信令分发核心。
type AIVoiceEngine struct {
	CommandService *esl.CommandService
	SessionStore   esl.SessionStore
	StatusReader   esl.ExtensionStatusReader
	Logger         *slog.Logger
	lifetimeCtx    context.Context
}

// NewAIVoiceEngine 创建智能语音 IVR 运行时寻路引擎。
func NewAIVoiceEngine(lifetimeCtx context.Context, cmdService *esl.CommandService, store esl.SessionStore, statusReader esl.ExtensionStatusReader, logger *slog.Logger) *AIVoiceEngine {
	if lifetimeCtx == nil {
		lifetimeCtx = context.Background()
	}
	return &AIVoiceEngine{
		CommandService: cmdService,
		SessionStore:   store,
		StatusReader:   statusReader,
		Logger:         logger,
		lifetimeCtx:    lifetimeCtx,
	}
}

// StartAIVoiceFlow 启动通话会话的可视化 AI 流程。
// 当被叫客户应答或呼入接通时，由呼叫状态机消费者（consumer.go）触发启动。
func (e *AIVoiceEngine) StartAIVoiceFlow(ctx context.Context, session *esl.CallSession, flow operatedomain.AIModelFlow) error {
	callID := session.CallID
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	fsAddr := session.FSAddr

	logger := e.logger().With("callId", callID, "uuid", customerUUID, "fsAddr", fsAddr, "flowId", flow.ID)
	logger.Info("云枢呼叫运行时：开始驱动可视化 AI 话术流", "flowName", flow.Name)

	if flow.FlowGraph == nil || len(flow.FlowGraph.Nodes) == 0 {
		logger.Warn("AI 流程图拓扑为空，回退使用传统 Prompt 文本提示词")
		return e.playDefaultPrompt(ctx, callID, customerUUID, fsAddr, flow)
	}

	// 1. 寻找开始节点 (Start Node)
	var startNode *operatedomain.AIFlowNode
	for i := range flow.FlowGraph.Nodes {
		if flow.FlowGraph.Nodes[i].Type == "start" {
			startNode = &flow.FlowGraph.Nodes[i]
			break
		}
	}

	if startNode == nil {
		logger.Error("AI 流程拓扑解析失败：缺失 Start 启动节点")
		return errors.New("missing start node")
	}

	// 2. 检查并启动 mod_audio_stream WebSocket 实时音频旁路推流
	asrEnabled, _ := startNode.Metadata["asrEnabled"].(bool)
	wsURL, _ := startNode.Metadata["wsUrl"].(string)
	if asrEnabled && wsURL != "" {
		mixType, _ := startNode.Metadata["mixType"].(string)
		if mixType == "" {
			mixType = "mono" // 默认单声道过滤回音
		}
		sampleRate, _ := startNode.Metadata["sampleRate"].(string)
		if sampleRate == "" {
			sampleRate = "16k" // 默认高清采样
		}
		metadataText, _ := startNode.Metadata["metadata"].(string)

		logger.Info("云枢呼叫运行时：发现 ASR 推流旁路，开始投递 mod_audio_stream 启动命令", "wsUrl", wsURL, "mixType", mixType, "sampleRate", sampleRate)

		// 发送 uuid_audio_stream 物理指令到 FreeSWITCH 媒体网关
		cmd := telephony.NewCommand(
			fmt.Sprintf("audio_stream:%s:start", callID),
			"audio_stream", // 映射 FreeSWITCH ESL API: uuid_audio_stream <uuid> start
			callID,
			customerUUID,
			fsAddr,
			contracts.LegRoleCustomer,
			contracts.CallFlowInbound,
			map[string]any{
				"action":       "start",
				"url":          wsURL,
				"mixType":      mixType,
				"samplingRate": sampleRate,
				"metadata":     metadataText,
			},
		)

		if err := e.CommandService.Execute(ctx, cmd); err != nil {
			logger.Error("云枢呼叫运行时：下发 mod_audio_stream 媒体推流失败", "error", err.Error())
			// 媒体推流失败非致命阻塞，继续走向后续播报节点
		} else {
			logger.Info("云枢呼叫运行时：mod_audio_stream 媒体旁路推流已成功起呼发送")
		}
	}

	// 3. 寻找开始节点的下一个连接目标（通常是 TTS 播报节点）
	nextEdges := e.findOutgoingEdges(flow.FlowGraph, startNode.ID)
	if len(nextEdges) > 0 {
		firstTargetNode := e.findNodeByID(flow.FlowGraph, nextEdges[0].Target)
		if firstTargetNode != nil {
			return e.executeNode(ctx, session, flow.FlowGraph, firstTargetNode)
		}
	}

	return nil
}

// ProcessASRText 处理 ASR 实时断句语音文字上报，沿着可视化拓扑路径动态寻路匹配。
func (e *AIVoiceEngine) ProcessASRText(ctx context.Context, session *esl.CallSession, flow operatedomain.AIModelFlow, text string) error {
	callID := session.CallID
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	fsAddr := session.FSAddr
	currentNodeID, _ := session.Metadata["aiCurrentNode"].(string)
	if currentNodeID == "" {
		currentNodeID = "node-intent"
	}

	logger := e.logger().With("callId", callID, "uuid", customerUUID, "currentNodeId", currentNodeID, "text", text)
	logger.Info("云枢呼叫运行时：接收到 ASR/STT 实时识别断句结果，开始匹配流程节点分支")

	graph := flow.FlowGraph
	if graph == nil {
		return errors.New("empty flow graph")
	}

	// 寻找当前的意图节点 (Intent Router Node)
	currentNode := e.findNodeByID(graph, currentNodeID)
	if currentNode == nil || currentNode.Type != "intent" {
		logger.Warn("当前活跃节点不是意图路由节点，无法进行 ASR 文本匹配", "type", func() string {
			if currentNode != nil {
				return currentNode.Type
			}
			return "nil"
		}())
		return nil
	}

	// 读取意图出度边连线
	outgoingEdges := e.findOutgoingEdges(graph, currentNode.ID)
	var matchedEdge *operatedomain.AIFlowEdge

	// 1. 语义关键词包含匹配 (Keyword Flow)
	for i := range outgoingEdges {
		edge := &outgoingEdges[i]
		if edge.SourceHandle != "" && strings.Contains(text, edge.SourceHandle) {
			matchedEdge = edge
			break
		}
	}

	// 2. 默认模糊降级逻辑（包含话费/客服/人等核心话务意图）
	if matchedEdge == nil {
		if strings.Contains(text, "话费") || strings.Contains(text, "余额") || strings.Contains(text, "钱") {
			for i := range outgoingEdges {
				if outgoingEdges[i].SourceHandle == "我要查话费" || outgoingEdges[i].SourceHandle == "查话费" {
					matchedEdge = &outgoingEdges[i]
					break
				}
			}
		} else if strings.Contains(text, "人") || strings.Contains(text, "坐席") || strings.Contains(text, "客服") {
			for i := range outgoingEdges {
				if outgoingEdges[i].SourceHandle == "我要人工服务" || outgoingEdges[i].SourceHandle == "转人工" {
					matchedEdge = &outgoingEdges[i]
					break
				}
			}
		}
	}

	if matchedEdge != nil {
		targetNode := e.findNodeByID(graph, matchedEdge.Target)
		if targetNode != nil {
			logger.Info("云枢呼叫运行时：意图匹配成功！发射路径电荷传导流动", "intent", matchedEdge.SourceHandle, "nextLabel", targetNode.Label)
			return e.executeNode(ctx, session, graph, targetNode)
		}
	}

	// 3. 语义连线未匹配，尝试穿透到配置的 AI 大模型进行自由对话
	logger.Info("云枢呼叫运行时：意图未命中流程图分支，尝试请求绑定的大语言模型")

	var startNode *operatedomain.AIFlowNode
	for i := range graph.Nodes {
		if graph.Nodes[i].Type == "start" {
			startNode = &graph.Nodes[i]
			break
		}
	}

	var aiResponse string
	if startNode != nil && startNode.Metadata != nil && startNode.Metadata["llmProvider"] != nil {
		provider, _ := startNode.Metadata["llmProvider"].(string)
		if provider != "" && provider != "mock" {
			apiKey, _ := startNode.Metadata["llmApiKey"].(string)
			model, _ := startNode.Metadata["llmModel"].(string)
			endpoint, _ := startNode.Metadata["llmEndpoint"].(string)
			systemPrompt, _ := startNode.Metadata["llmSystemPrompt"].(string)
			tempVal, _ := startNode.Metadata["llmTemperature"].(float64)
			if tempVal == 0 {
				tempVal = 0.7
			}

			logger.Info("云枢呼叫运行时：检测到商户大模型绑定，通过通用引擎层发起请求", "provider", provider, "model", model, "endpoint", endpoint)
			llmEng := GetLLMEngine(provider)

			// 组装完整的 metadata 配置透传
			configMap := make(map[string]any)
			for k, v := range startNode.Metadata {
				configMap[k] = v
			}
			configMap["llmApiKey"] = apiKey
			configMap["llmModel"] = model
			configMap["llmEndpoint"] = endpoint
			configMap["llmTemperature"] = tempVal

			respText, err := llmEng.GenerateReply(ctx, systemPrompt, text, configMap)
			if err != nil {
				logger.Error("云枢呼叫运行时：调用云端大模型物理引擎失败，拒绝仿真退避", "error", err)
				return fmt.Errorf("调用云端大模型物理引擎失败: %w", err)
			}
			if respText == "" {
				logger.Error("云枢呼叫运行时：调用云端大模型物理引擎返回空内容，拒绝仿真退避")
				return errors.New("调用云端大模型物理引擎返回空内容")
			}
			aiResponse = respText
			logger.Info("云枢呼叫运行时：成功接收到云端大模型应答文本", "reply", aiResponse)
		}
	}

	// 如果大模型返回为空或未配置，则直接严格报错，不再走向任何仿真与 mock 兜底
	if aiResponse == "" {
		logger.Error("云枢呼叫运行时：大语言模型未配置或生成为空，拒绝仿真退避")
		return errors.New("大语言模型物理引擎未配置或应答内容为空")
	}

	return e.playbackTTS(ctx, callID, customerUUID, fsAddr, aiResponse)
}

// ProcessDTMFKey 处理 FreeSWITCH 上报的物理数字按键 (DTMF)，驱动按键收集与判断分支。
func (e *AIVoiceEngine) ProcessDTMFKey(ctx context.Context, session *esl.CallSession, flow operatedomain.AIModelFlow, digit string) error {
	callID := session.CallID
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	currentNodeID, _ := session.Metadata["aiCurrentNode"].(string)
	if currentNodeID == "" {
		currentNodeID = "node-intent"
	}

	logger := e.logger().With("callId", callID, "uuid", customerUUID, "currentNodeId", currentNodeID, "digit", digit)
	logger.Info("云枢呼叫运行时：捕获到电话数字按键输入 (DTMF)")

	graph := flow.FlowGraph
	if graph == nil {
		return errors.New("empty flow graph")
	}

	currentNode := e.findNodeByID(graph, currentNodeID)
	if currentNode == nil {
		return nil
	}

	outgoingEdges := e.findOutgoingEdges(graph, currentNode.ID)
	var matchedEdge *operatedomain.AIFlowEdge

	// 查找 SourceHandle 与按键对应的 Edge 分支 (如 dtmf 1 连线分支)
	for i := range outgoingEdges {
		edge := &outgoingEdges[i]
		if edge.SourceHandle == digit {
			matchedEdge = edge
			break
		}
	}

	if matchedEdge != nil {
		targetNode := e.findNodeByID(graph, matchedEdge.Target)
		if targetNode != nil {
			logger.Info("云枢呼叫运行时：DTMF 按键条件路由匹配成功！", "digit", digit, "nextLabel", targetNode.Label)
			return e.executeNode(ctx, session, graph, targetNode)
		}
	}

	logger.Warn("云枢呼叫运行时：捕获的按键未命中流图分支，不做任何跳转", "digit", digit)
	return nil
}

// executeNode 运行时具体执行流图拓扑中某个节点的动作逻辑。
func (e *AIVoiceEngine) executeNode(ctx context.Context, session *esl.CallSession, graph *operatedomain.AIFlowGraph, node *operatedomain.AIFlowNode) error {
	callID := session.CallID
	customerUUID, _ := session.Metadata["customerUuid"].(string)
	fsAddr := session.FSAddr

	logger := e.logger().With("callId", callID, "uuid", customerUUID, "nodeId", node.ID, "nodeType", node.Type, "label", node.Label)
	logger.Info("云枢呼叫运行时：开始物理执行拓扑节点动作")

	// 1. 同步保存当前节点状态到会话
	if session.Metadata == nil {
		session.Metadata = map[string]any{}
	}
	session.Metadata["aiCurrentNode"] = node.ID
	if e.SessionStore != nil {
		if err := e.SessionStore.Save(ctx, *session); err != nil {
			logger.Error("云枢呼叫运行时：同步保存 AI 当前活跃节点状态失败", "error", err.Error())
		} else {
			logger.Info("云枢呼叫运行时：已同步当前活跃节点到会话状态中", "nodeId", node.ID)
		}
	}

	switch node.Type {
	case "reply":
		// TTS 播报节点
		text, _ := node.Metadata["text"].(string)
		if text == "" {
			text = "您好！正在处理中。"
		}
		logger.Info("云枢呼叫运行时：触发播报动作", "ttsText", text)
		if err := e.playbackTTS(ctx, callID, customerUUID, fsAddr, text); err != nil {
			return err
		}

		// 播放完成后，自动寻找出度无条件边流动到下一个节点
		nextEdges := e.findOutgoingEdges(graph, node.ID)
		if len(nextEdges) > 0 {
			// 中文 TTS 的播放速度大约为每秒 4 个字。智能估算播放时长，以免背景 goroutine 长时间或短时间不一致。
			runeCount := len([]rune(text))
			playSeconds := runeCount / 4
			if playSeconds < 2 {
				playSeconds = 2
			} else if playSeconds > 15 {
				playSeconds = 15 // 设定上限
			}

			logger.Info("云枢呼叫运行时：TTS 播报启动，开启智能定时器，完毕后自动流转", "estimatedSeconds", playSeconds)
			go func() {
				select {
				case <-time.After(time.Duration(playSeconds) * time.Second):
				case <-e.lifetimeCtx.Done():
					return
				}
				nextTargetNode := e.findNodeByID(graph, nextEdges[0].Target)
				if nextTargetNode != nil {
					// 异步流动执行下一个节点时，我们需要一个干净的上下文，并重新保存状态
					_ = e.executeNode(e.lifetimeCtx, session, graph, nextTargetNode)
				}
			}()
		}
		return nil

	case "transfer":
		// 转人工与智能 ACD 状态排队分流
		targetType, _ := node.Metadata["targetType"].(string)
		targetID, _ := node.Metadata["targetId"].(string)
		enableQueue, _ := node.Metadata["enableQueue"].(bool)
		maxQueueTime, _ := node.Metadata["maxQueueTime"].(float64) // JSON Unmarshal 数值为 float64
		if maxQueueTime == 0 {
			maxQueueTime = 30
		}

		logger.Info("云枢呼叫运行时：触发转人工流程", "targetType", targetType, "targetId", targetID, "enableQueue", enableQueue)

		// 真正进行 ACD 状态检测
		hasAgent := false
		if e.StatusReader != nil && targetID != "" {
			status, ok, err := e.StatusReader.GetExtensionStatus(ctx, targetID)
			if err == nil && ok && status == esl.ExtensionStatusIdle {
				hasAgent = true
				logger.Info("云枢呼叫运行时：ACD 状态检测成功！分机在线且空闲", "targetId", targetID)
			} else {
				logger.Warn("云枢呼叫运行时：ACD 状态检测提示分机不可用或正忙", "targetId", targetID, "status", status, "ok", ok, "error", err)
			}
		} else {
			// 本地开发或测试模式兜底，默认认为目标为 1001 的分机均在线空闲
			if targetID == "1001" || targetID == "" {
				hasAgent = true
			}
			logger.Warn("云枢呼叫运行时：未配置 ExtensionStatusReader，回退为本地模拟 ACD 分流", "hasAgent", hasAgent)
		}

		// 发射 has_agent/no_agent 路由决策
		outgoingEdges := e.findOutgoingEdges(graph, node.ID)
		var chosenEdge *operatedomain.AIFlowEdge

		handlePattern := "no_agent"
		if hasAgent {
			handlePattern = "has_agent"
		}

		for i := range outgoingEdges {
			edge := &outgoingEdges[i]
			if edge.SourceHandle == handlePattern {
				chosenEdge = edge
				break
			}
		}

		// 如果没找到匹配的线，走第一条边兜底
		if chosenEdge == nil && len(outgoingEdges) > 0 {
			chosenEdge = &outgoingEdges[0]
		}

		if chosenEdge != nil {
			targetNode := e.findNodeByID(graph, chosenEdge.Target)
			if targetNode != nil {
				logger.Info("云枢呼叫运行时：ACD 路由分流成功，流动到下一个分支", "handlePattern", handlePattern, "nextLabel", targetNode.Label)
				return e.executeNode(ctx, session, graph, targetNode)
			}
		}
		return nil

	case "end":
		// 结束挂断节点
		logger.Info("云枢呼叫运行时：触发挂断命令，下发 Normal Clearing 挂断信道")
		cmd := telephony.NewCommand(
			fmt.Sprintf("hangup:%s:flow_end", callID),
			"hangup",
			callID,
			customerUUID,
			fsAddr,
			contracts.LegRoleCustomer,
			contracts.CallFlowInbound,
			map[string]any{"cause": "NORMAL_CLEARING"},
		)
		return e.CommandService.Execute(ctx, cmd)

	case "intent":
		// 意图节点仅作为匹配锚点，等待 ProcessASRText 事件触发，自身不触发执行
		logger.Info("云枢呼叫运行时：进入 ASR 意图等待卡点，开启 VAD 能量检测监听")
		return nil

	default:
		logger.Warn("云枢呼叫运行时：未知的拓扑节点类型，直接跳过", "type", node.Type)
	}

	return nil
}

// playDefaultPrompt 传统 Prompt 降级文本播放。
func (e *AIVoiceEngine) playDefaultPrompt(ctx context.Context, callID string, customerUUID string, fsAddr string, flow operatedomain.AIModelFlow) error {
	defaultText := "您好！这里是云枢呼叫中心，我们的大模型正在为您服务，请问有什么可以帮您？"
	if flow.Prompt != "" {
		defaultText = flow.Prompt
	}
	return e.playbackTTS(ctx, callID, customerUUID, fsAddr, defaultText)
}

// playbackTTS 下发 TTS 语音播报指令给 FreeSWITCH 媒体通道（支持多厂商物理接入并提供本地 MD5 缓存功能）。
func (e *AIVoiceEngine) playbackTTS(ctx context.Context, callID string, customerUUID string, fsAddr string, text string) error {
	var playFile string
	var volcAppId, volcToken, volcCluster string
	var volcVoiceType, ttsProvider string
	var volcSpeedRatio float64 = 1.0

	// 1. 尝试从通话会话元数据中提取 AI 配置
	session, err := e.SessionStore.Get(ctx, callID)
	if err == nil {
		var flow operatedomain.AIModelFlow
		if flowJSON, ok := session.Metadata["aiFlowData"].(string); ok && flowJSON != "" {
			_ = json.Unmarshal([]byte(flowJSON), &flow)
		}
		if flow.FlowGraph != nil {
			var startNode *operatedomain.AIFlowNode
			for i := range flow.FlowGraph.Nodes {
				if flow.FlowGraph.Nodes[i].Type == "start" {
					startNode = &flow.FlowGraph.Nodes[i]
					break
				}
			}
			if startNode != nil && startNode.Metadata != nil {
				volcAppId, _ = startNode.Metadata["volcAppId"].(string)
				volcToken, _ = startNode.Metadata["volcToken"].(string)
				volcCluster, _ = startNode.Metadata["volcCluster"].(string)
				volcVoiceType, _ = startNode.Metadata["volcVoiceType"].(string)
				ttsProvider, _ = startNode.Metadata["ttsProvider"].(string)
				if val, ok := startNode.Metadata["volcSpeedRatio"].(float64); ok && val > 0 {
					volcSpeedRatio = val
				}

				// 尝试兼容大模型 Key 字段作为 ASR/TTS 凭证
				if volcAppId == "" {
					provider, _ := startNode.Metadata["llmProvider"].(string)
					apiKey, _ := startNode.Metadata["llmApiKey"].(string)
					if (strings.Contains(provider, "豆包") || strings.Contains(provider, "火山")) && apiKey != "" {
						volcToken = apiKey
						volcAppId = "cloudshu_demo"
					}
				}
			}
		}
	}

	// 2. 如果配置了火山/豆包语音合成凭证，物理启动通用 TTS 引擎
	if volcAppId != "" && volcToken != "" {
		configMap := map[string]any{
			"volcAppId":      volcAppId,
			"volcToken":      volcToken,
			"volcCluster":    volcCluster,
			"volcVoiceType":  volcVoiceType,
			"volcSpeedRatio": volcSpeedRatio,
		}
		if ttsProvider == "" {
			ttsProvider = "volc"
		}
		filePath, err := SynthesizeAndCacheTTS(ctx, text, ttsProvider, configMap)
		if err == nil {
			absPath, err := filepath.Abs(filePath)
			if err == nil {
				playFile = absPath
			} else {
				playFile = filePath
			}
		} else {
			e.logger().Error("云枢呼叫运行时：物理 TTS 引擎合成失败，降级使用虚拟模拟播放", "error", err)
		}
	}

	if playFile == "" {
		playFile = fmt.Sprintf("tts://%s", text)
	}

	e.logger().Info("云枢呼叫运行时：物理执行 FreeSWITCH 播发命令", "file", playFile)

	cmd := telephony.NewCommand(
		fmt.Sprintf("playback:%s:tts", callID),
		"playback",
		callID,
		customerUUID,
		fsAddr,
		contracts.LegRoleCustomer,
		contracts.CallFlowInbound,
		map[string]any{
			"file": playFile,
			"both": "aleg",
		},
	)
	return e.CommandService.Execute(ctx, cmd)
}

// findOutgoingEdges 查找拓扑图中有向出度边。
func (e *AIVoiceEngine) findOutgoingEdges(graph *operatedomain.AIFlowGraph, sourceID string) []operatedomain.AIFlowEdge {
	var edges []operatedomain.AIFlowEdge
	for i := range graph.Edges {
		if graph.Edges[i].Source == sourceID {
			edges = append(edges, graph.Edges[i])
		}
	}
	return edges
}

// findNodeByID 寻找特定 ID 的节点实体。
func (e *AIVoiceEngine) findNodeByID(graph *operatedomain.AIFlowGraph, id string) *operatedomain.AIFlowNode {
	for i := range graph.Nodes {
		if graph.Nodes[i].ID == id {
			return &graph.Nodes[i]
		}
	}
	return nil
}

func (e *AIVoiceEngine) logger() *slog.Logger {
	if e != nil && e.Logger != nil {
		return e.Logger
	}
	return slog.Default()
}
