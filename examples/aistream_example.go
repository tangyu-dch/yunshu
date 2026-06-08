package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yunshu/internal/contracts"
	"yunshu/internal/domain/callflow"
)

// ============================================================================
// 云枢 AI 流处理系统 - 使用示例
// ============================================================================

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.Default()
	logger.Info("=== 云枢 AI 流处理系统 - 使用示例 ===\n")

	// 1. 初始化所有 AI 引擎
	logger.Info("步骤 1: 初始化 AI 引擎")
	callflow.InitializeAIEngines()
	logger.Info("✓ AI 引擎初始化完成")

	// 2. 创建配置
	logger.Info("\n步骤 2: 创建配置")
	config := createExampleConfig()
	logger.Info("✓ 配置创建完成", "defaultLLM", config.DefaultLLMID)

	// 3. 创建流处理管道
	logger.Info("\n步骤 3: 创建 AI 流处理管道")
	pipeline := callflow.NewAIStreamPipeline(config, logger)
	logger.Info("✓ 流处理管道创建完成")

	// 4. 启动一个会话
	logger.Info("\n步骤 4: 启动会话")
	session, err := pipeline.StartSession(
		"example-call-12345",
		"customer-uuid-67890",
		"freeswitch-1:8021",
		map[string]interface{}{
			"systemPrompt": "你是云枢呼叫中心的AI助手，友好、专业地帮助用户解决问题。",
			"merchantId":   "merchant-abc123",
		},
	)
	if err != nil {
		logger.Error("启动会话失败", "error", err)
		return
	}
	logger.Info("✓ 会话启动成功", "sessionID", session.SessionID, "callID", session.CallID)

	// 5. 模拟处理用户输入
	logger.Info("\n步骤 5: 模拟处理用户输入")
	go func() {
		// 等待一下
		time.Sleep(500 * time.Millisecond)

		// 模拟 ASR 文本输入
		testInputs := []string{
			"你好",
			"我想查一下话费",
			"转人工",
		}

		for _, input := range testInputs {
			logger.Info("模拟用户输入", "text", input)
			err := pipeline.ProcessASRText(ctx, session.SessionID, input, true)
			if err != nil {
				logger.Error("处理输入失败", "error", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// 6. 监听管道事件
	logger.Info("\n步骤 6: 监听管道事件")
	go func() {
		eventChan := pipeline.GetEventChannel()
		for event := range eventChan {
			logger.Info("收到事件", "type", event.Type, "timestamp", event.Timestamp)
			
			// 打印事件详情
			eventJSON, _ := json.MarshalIndent(event.Data, "", "  ")
			logger.Info("事件数据", "data", string(eventJSON))
		}
	}()

	// 7. 演示切换 LLM
	logger.Info("\n步骤 7: 演示动态切换 LLM (如果配置了多个)")
	if len(config.LLMProviders) > 1 {
		// 获取所有可用的 LLM
		for id, prov := range config.LLMProviders {
			if id != config.DefaultLLMID && prov.Enabled {
				logger.Info("尝试切换 LLM", "newLLM", id)
				err := pipeline.SwitchLLMProvider(session.SessionID, id)
				if err != nil {
					logger.Warn("切换失败 (这是正常的，除非已注册该引擎)", "error", err)
				} else {
					logger.Info("✓ 切换成功", "currentLLM", id)
				}
				break
			}
		}
	}

	// 8. 优雅关闭
	logger.Info("\n步骤 8: 等待测试完成 (5 秒后自动关闭)")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		logger.Info("\n收到关闭信号")
	case <-time.After(5 * time.Second):
		logger.Info("\n测试时间到")
	}

	// 9. 清理
	logger.Info("\n步骤 9: 清理")
	pipeline.StopSession(session.SessionID)
	logger.Info("✓ 会话已停止")

	logger.Info("\n=== 示例完成 ===")
}

// createExampleConfig 创建示例配置
func createExampleConfig() contracts.AIStreamPipelineConfig {
	return contracts.AIStreamPipelineConfig{
		LLMProviders: map[string]contracts.LLMProviderConfig{
			"mock": {
				ID:          "mock",
				Name:        "Mock LLM (测试用)",
				Provider:    "mock",
				Enabled:     true,
				Model:       "mock-model",
				Temperature: 0.7,
				MaxTokens:   2000,
			},
			"mock-2": {
				ID:          "mock-2",
				Name:        "另一 Mock LLM",
				Provider:    "mock",
				Enabled:     true,
				Model:       "mock-model-2",
				Temperature: 0.8,
				MaxTokens:   1000,
			},
		},
		DefaultLLMID: "mock",

		ASRProviders: map[string]contracts.ASRProviderConfig{
			"mock": {
				ID:       "mock",
				Name:     "Mock ASR",
				Provider: "mock",
				Enabled:  true,
				Language: "zh-CN",
			},
		},
		DefaultASRID: "mock",

		TTSProviders: map[string]contracts.TTSProviderConfig{
			"mock": {
				ID:        "mock",
				Name:      "Mock TTS",
				Provider:  "mock",
				Enabled:   true,
				VoiceType: "default",
				Speed:     1.0,
			},
		},
		DefaultTTSID: "mock",

		EnableStream: true,
		BufferSize:   4096,
	}
}

// ============================================================================
// 更多示例
// ============================================================================

// Example_UsingMultipleLLMs 演示如何配置和使用多个 LLM
func Example_UsingMultipleLLMs() {
	logger := slog.Default()

	// 创建带有多个 LLM 的配置
	config := contracts.AIStreamPipelineConfig{
		LLMProviders: map[string]contracts.LLMProviderConfig{
			"openai": {
				ID:          "openai",
				Name:        "OpenAI GPT-4",
				Provider:    "openai",
				Enabled:     true,
				APIKey:      "sk-your-api-key-here",
				Endpoint:    "https://api.openai.com/v1/chat/completions",
				Model:       "gpt-4",
				Temperature: 0.7,
				MaxTokens:   2000,
			},
			"deepseek": {
				ID:          "deepseek",
				Name:        "DeepSeek AI",
				Provider:    "deepseek",
				Enabled:     true,
				APIKey:      "sk-your-deepseek-key",
				Endpoint:    "https://api.deepseek.com/v1/chat/completions",
				Model:       "deepseek-chat",
				Temperature: 0.7,
				MaxTokens:   2000,
			},
			"volc": {
				ID:          "volc",
				Name:        "火山引擎豆包",
				Provider:    "volc",
				Enabled:     true,
				APIKey:      "your-volc-key",
				Endpoint:    "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
				Model:       "ep-your-endpoint",
				Temperature: 0.7,
				MaxTokens:   2000,
			},
		},
		DefaultLLMID: "openai",
	}

	pipeline := callflow.NewAIStreamPipeline(config, logger)
	fmt.Println("配置了", len(config.LLMProviders), "个 LLM 提供商")
}

// Example_ConversationHistory 演示对话历史管理
func Example_ConversationHistory() {
	convManager := callflow.GetConversationManager()

	// 获取或创建对话历史
	history := convManager.GetOrCreateHistory("call-12345")

	// 添加消息
	history.AddSystemMessage("你是云枢AI助手")
	history.AddUserMessage("你好")
	history.AddAssistantMessage("您好，有什么可以帮助您的？")
	history.AddUserMessage("查话费")
	history.AddAssistantMessage("好的，帮您查询话费余额")

	// 截断历史
	history.Truncate(10)

	fmt.Println("历史消息数:", len(history.Messages))
}
