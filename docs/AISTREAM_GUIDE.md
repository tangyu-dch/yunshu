# 云枢 AI 实时流处理系统

## 概述

云枢 AI 实时流处理系统是一个完整的语音交互解决方案，集成了实时音频流接收、自动语音识别(ASR)、大语言模型(LLM)对话处理和语音合成(TTS)功能。

### 核心特性

- ✅ **多 LLM 提供商支持**: OpenAI、DeepSeek、通义千问、火山引擎等
- ✅ **动态配置切换**: 支持在运行时切换不同的 AI 提供商
- ✅ **完整的音频流处理**: WebSocket 实时音频接收 + ASR + LLM + TTS
- ✅ **对话历史管理**: 自动管理对话上下文
- ✅ **流程编排集成**: 与云枢呼叫流程深度集成

## 架构

```
┌───────────────────┐
│  FreeSWITCH       │
│  (mod_audio_stream)│
└─────────┬─────────┘
          │ WebSocket (音频流)
          ↓
┌─────────────────────────────────────────┐
│       Audio Stream Hub                │
│  ┌────────────────────────────────┐ │
│  │ Audio Stream Pipeline         │ │
│  │  - ASR Engine Manager         │ │
│  │  - LLM Engine Manager         │ │
│  │  - TTS Engine Manager         │ │
│  │  - Conversation History       │ │
│  └────────────────────────────────┘ │
└─────────────────────────────────────────┘
         ↑              ↓
    ASR 结果      TTS 输出
```

## 快速开始

### 1. 初始化 AI 引擎

```go
import (
    "yunshu/internal/domain/callflow"
    "yunshu/internal/contracts"
)

// 初始化所有 AI 引擎
callflow.InitializeAIEngines()

// 使用默认配置创建管道
config := callflow.DefaultAIStreamConfig()
pipeline := callflow.NewAIStreamPipeline(config, logger)
```

### 2. 使用自定义配置

```json
{
  "llmProviders": {
    "openai-gpt4": {
      "id": "openai-gpt4",
      "name": "OpenAI GPT-4",
      "provider": "openai",
      "enabled": true,
      "apiKey": "sk-xxx",
      "endpoint": "https://api.openai.com/v1/chat/completions",
      "model": "gpt-4",
      "temperature": 0.7,
      "maxTokens": 2000
    },
    "deepseek": {
      "id": "deepseek",
      "name": "DeepSeek AI",
      "provider": "deepseek",
      "enabled": true,
      "apiKey": "sk-xxx",
      "endpoint": "https://api.deepseek.com/v1/chat/completions",
      "model": "deepseek-chat",
      "temperature": 0.7
    }
  },
  "defaultLLMId": "openai-gpt4"
}
```

```go
// 从文件加载配置
import (
    "encoding/json"
    "os"
)

var config contracts.AIStreamPipelineConfig
data, _ := os.ReadFile("config/aistream_config.json")
json.Unmarshal(data, &config)

pipeline := callflow.NewAIStreamPipeline(config, logger)
```

### 3. 启动 WebSocket 音频流服务器

```go
import (
    "yunshu/internal/infra/websocket"
)

serverConfig := websocket.AudioStreamServerConfig{
    Addr:     ":8081",
    Path:     "/ws/audio",
    Pipeline: pipeline,
    Logger:   logger,
}

server, err := websocket.StartAudioStreamServer(serverConfig)
if err != nil {
    panic(err)
}

// 启动云枢 AIVoiceEngine 与现有呼叫流程集成
aiEngine := callflow.NewAIVoiceEngine(ctx, cmdService, sessionStore, statusReader, logger)
```

## WebSocket 通信协议

### 连接

```
ws://localhost:8081/ws/audio?callId=12345&customerUUID=67890&fsAddr=freeswitch:8021
```

### 消息类型

#### 1. 音频数据 (`audio`)

```json
{
  "type": "audio",
  "session": "session-id",
  "data": {
    "data": "base64-encoded-pcm-data",
    "format": "pcm",
    "sampleRate": 16000
  }
}
```

#### 2. ASR 文本 (`asr_text`)

```json
{
  "type": "asr_text",
  "session": "session-id",
  "data": {
    "text": "你好，我想查话费"
  }
}
```

#### 3. 切换 LLM (`switch_llm`)

```json
{
  "type": "switch_llm",
  "session": "session-id",
  "data": {
    "llmId": "deepseek"
  }
}
```

### 服务器端事件

#### 1. 会话启动 (`session_start`)

```json
{
  "type": "session_start",
  "data": {
    "sessionID": "session-id",
    "callID": "12345",
    "status": "streaming"
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

#### 2. ASR 结果 (`asr`)

```json
{
  "type": "asr",
  "data": {
    "text": "你好，我想查话费",
    "confidence": 0.95,
    "isFinal": true
  }
}
```

#### 3. LLM 响应 (`llm`)

```json
{
  "type": "llm",
  "data": {
    "content": "好的，我帮您查询话费。请问您是哪个手机号？",
    "finishReason": "stop"
  }
}
```

#### 4. TTS 音频 (`tts`)

```json
{
  "type": "tts",
  "data": "base64-encoded-mp3-data"
}
```

## 多 LLM 配置和切换

### 配置多个 LLM 提供商

```go
config := contracts.AIStreamPipelineConfig{
    LLMProviders: map[string]contracts.LLMProviderConfig{
        "openai-gpt4": {
            ID:          "openai-gpt4",
            Name:        "OpenAI GPT-4",
            Provider:    "openai",
            Enabled:     true,
            APIKey:      "sk-xxx",
            Model:       "gpt-4",
            Temperature: 0.7,
        },
        "volc-doubao": {
            ID:          "volc-doubao",
            Name:        "火山引擎豆包",
            Provider:    "volc",
            Enabled:     true,
            APIKey:      "xxx",
            Endpoint:    "https://ark.cn-beijing.volces.com/api/v3/chat/completions",
            Model:       "ep-xxx",
            Temperature: 0.7,
        },
    },
    DefaultLLMID: "openai-gpt4",
}
```

### 运行时切换 LLM

```go
// 通过 WebSocket 消息切换
// 或直接在代码中切换
pipeline.SwitchLLMProvider(sessionID, "volc-doubao")
```

## 流程编排集成

### 在 AI 流程中启用音频流

在 AI 流程配置的 `start` 节点中添加音频流配置：

```go
startNode := &operatedomain.AIFlowNode{
    Type: "start",
    Metadata: map[string]interface{}{
        "asrEnabled":  true,
        "wsUrl":      "ws://localhost:8081/ws/audio",
        "mixType":    "mono",
        "sampleRate": "16k",
        "llmProvider": "openai-gpt4",  // 配置默认 LLM
        "llmApiKey":  "sk-xxx",
        "llmModel":   "gpt-4",
        "systemPrompt": "你是云枢呼叫中心的AI助手...",
    },
}
```

### 完整的 AI 处理流程

1. **呼叫接通** → 触发 `StartAIVoiceFlow`
2. **启动音频流** → FreeSWITCH 发送音频到 WebSocket
3. **ASR 转写** → 音频转文字
4. **LLM 处理** → 对话上下文 + 用户输入 → 生成回复
5. **TTS 合成** → 文字转语音
6. **播放回复** → 通过 FreeSWITCH 播放给用户

## 代码示例

### 完整的流处理示例

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "yunshu/internal/contracts"
    "yunshu/internal/domain/callflow"
    "yunshu/internal/infra/websocket"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    logger := slog.Default()

    // 1. 初始化引擎
    callflow.InitializeAIEngines()

    // 2. 加载配置
    config := callflow.DefaultAIStreamConfig()

    // 3. 创建管道
    pipeline := callflow.NewAIStreamPipeline(config, logger)

    // 4. 启动 WebSocket 服务器
    server, err := websocket.StartAudioStreamServer(websocket.AudioStreamServerConfig{
        Addr:     ":8081",
        Path:     "/ws/audio",
        Pipeline: pipeline,
        Logger:   logger,
    })
    if err != nil {
        logger.Error("启动音频流服务器失败", "error", err)
        return
    }
    defer server.Shutdown(ctx)

    // 5. 监听管道事件
    go func() {
        eventChan := pipeline.GetEventChannel()
        for event := range eventChan {
            logger.Info("收到流处理事件", "type", event.Type)
        }
    }()

    // 等待关闭信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    logger.Info("正在关闭...")
}
```

## 目录结构

```
internal/
├── contracts/
│   └── aistream.go                    # 核心数据结构和接口
├── domain/callflow/
│   ├── ai_engine.go                   # AI 引擎 (已存在)
│   ├── ai_engine_providers.go         # 引擎接口 (已存在)
│   ├── llm_engines.go                 # LLM 引擎实现
│   ├── asr_tts_engines.go             # ASR/TTS 引擎实现
│   └── aistream_pipeline.go           # 流处理管道
└── infra/websocket/
    ├── hub.go                         # WebSocket Hub (已存在)
    └── audio_stream_hub.go            # 音频流 WebSocket Hub

config/
└── aistream_config.example.json        # 配置示例

docs/
└── AUDIO_STREAM_SUMMARY.md            # 本文档
```

## 已支持的 LLM 提供商

| 提供商 | ID | 状态 |
|--------|-----|------|
| Mock (测试用) | mock | ✅ 已支持 |
| OpenAI (GPT-3.5/4) | openai | ✅ 已支持 |
| DeepSeek | deepseek | ✅ 已支持 |
| 通义千问 | qwen | ✅ 已支持 |
| 火山引擎豆包 | volc | ✅ 已支持 |
| 自定义 OpenAI API | custom | ✅ 已支持 |

## 环境变量

```env
# OpenAI
OPENAI_API_KEY=sk-xxx

# 火山引擎
VOLC_API_KEY=xxx

# DeepSeek
DEEPSEEK_API_KEY=sk-xxx

# 音频流服务器
AUDIO_STREAM_ADDR=:8081
AUDIO_STREAM_PATH=/ws/audio
```

## 测试

### 使用 Mock 引擎测试

默认配置使用 Mock 引擎，可以立即测试：

1. 启动服务器
2. 连接 WebSocket
3. 发送音频数据或 ASR 文本
4. 查看 LLM 和 TTS 响应

### 运行验证脚本

```bash
./scripts/verify-audio-stream.sh
```

## 下一步

- [ ] 实现真实的 ASR 服务集成
- [ ] 实现真实的 TTS 服务集成
- [ ] 添加流式 LLM 支持
- [ ] 实现更复杂的对话管理策略
- [ ] 添加性能监控和指标
- [ ] 实现对话历史持久化

## 相关文档

- [音频流测试指南](./audio-stream-test-guide.md)
- [快速参考卡片](./audio-stream-quick-ref.md)
- [配置持久化](./audio-stream-persistence.md)
- [FreeSWITCH mod_audio_stream 集成](../docs/AUDIO_STREAM_SUMMARY.md)
