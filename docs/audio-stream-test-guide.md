# FreeSWITCH 实时音频流测试指南

## 概述

本文档描述如何使用 FreeSWITCH 的 `mod_audio_stream` 模块进行实时音频流测试。

## 前置条件

1. ✅ FreeSWITCH 已安装并运行
2. ✅ `mod_audio_stream` 模块已编译并加载
3. ✅ WebSocket 服务器已准备好接收音频流
4. ✅ 云枢 CallCenter 系统已配置

## 快速验证

### 1. 运行验证脚本

```bash
chmod +x scripts/verify-audio-stream.sh
./scripts/verify-audio-stream.sh
```

### 2. 手动验证步骤

```bash
# 2.1 检查模块是否加载
docker exec cc-freeswitch fs_cli -x "show modules" | grep -i audio_stream

# 2.2 检查命令是否可用
docker exec cc-freeswitch fs_cli -x "uuid_audio_stream"

# 2.3 如果模块未加载，手动加载
docker exec cc-freeswitch fs_cli -x "load mod_audio_stream"
```

## 创建测试呼叫

### 方法 1: 呼入测试

```bash
# 使用 SIP 客户端注册到 FreeSWITCH 并发起呼叫
# 分机: 1000
# 域: 127.0.0.1
```

### 方法 2: 呼出测试

```bash
# 发起外呼到分机 1000
docker exec cc-freeswitch fs_cli -x "originate user/1000 &park()"
```

### 方法 3: 使用 Go 代码

```go
// 使用云枢 AIVoiceEngine 启动 AI 流程
err := aiEngine.StartAIVoiceFlow(ctx, session, flow)
```

## 测试音频流

### 1. 获取呼叫 UUID

```bash
# 查看当前活跃呼叫
docker exec cc-freeswitch fs_cli -x "show calls"
```

### 2. 启动音频流

```bash
# 基本用法（单声道，8k 采样率）
uuid_audio_stream <uuid> start wss://your-server/audio mono 8000

# 高清音频（混合声道，16k 采样率）
uuid_audio_stream <uuid> start wss://your-server/audio mixed 16000

# 带元数据
uuid_audio_stream <uuid> start wss://your-server/audio mono 16k "call_id=123&merchant_id=456"
```

### 3. 停止音频流

```bash
uuid_audio_stream <uuid> stop
```

### 4. 暂停/恢复音频流

```bash
# 暂停
uuid_audio_stream <uuid> pause

# 恢复
uuid_audio_stream <uuid> resume
```

## 使用示例

### 示例 1: 基本音频流测试

```bash
# 1. 创建一个测试呼叫
CALL_UUID=$(docker exec cc-freeswitch fs_cli -x "originate user/1000 &park()" | grep -o '[a-f0-9-]\{36\}')

# 2. 等待呼叫建立
sleep 3

# 3. 启动音频流到本地测试服务器
uuid_audio_stream $CALL_UUID start wss://localhost:8080/audio mono 16k

# 4. 观察日志
docker logs cc-freeswitch --tail 50 | grep audio_stream

# 5. 停止音频流
uuid_audio_stream $CALL_UUID stop

# 6. 挂断呼叫
uuid_kill $CALL_UUID
```

### 示例 2: 带认证的 WebSocket

```bash
# 基本认证
uuid_audio_stream <uuid> start wss://user:pass@your-server/audio mono 16k

# Token 认证（通过 metadata）
uuid_audio_stream <uuid> start wss://your-server/audio mono 16k "token=abc123&user_id=456"
```

### 示例 3: 使用 Go 代码

```go
package main

import (
    "context"
    "fmt"

    "yunshu/internal/domain/callflow"
    "yunshu/internal/domain/esl"
    "yunshu/internal/infra/fsesl"
)

func main() {
    // 1. 创建 AI 语音引擎
    engine := callflow.NewAIVoiceEngine(
        context.Background(),
        commandService,
        sessionStore,
        statusReader,
        logger,
    )

    // 2. 配置 AI 流程（包含 WebSocket URL）
    flow := AIModelFlow{
        ID:   "test-flow",
        Name: "测试音频流",
        FlowGraph: &FlowGraph{
            Nodes: []AIFlowNode{
                {
                    ID:   "start",
                    Type: "start",
                    Metadata: map[string]interface{}{
                        "asrEnabled":  true,
                        "wsUrl":      "wss://your-ai-server/audio",
                        "mixType":    "mono",
                        "sampleRate": "16k",
                        "metadata":   "call_id=123&merchant_id=456",
                    },
                },
            },
        },
    }

    // 3. 启动 AI 流程
    session := &esl.CallSession{
        CallID: "test-call-001",
        UUID:   "test-uuid-001",
        FSAddr: "freeswitch.local:8021",
        Metadata: map[string]interface{}{
            "customerUuid": "customer-uuid-001",
        },
    }

    // 4. 执行流程
    err := engine.StartAIVoiceFlow(context.Background(), session, flow)
    if err != nil {
        fmt.Printf("启动 AI 流程失败: %v\n", err)
        return
    }

    fmt.Println("AI 音频流已启动")
}
```

## 音频格式说明

### PCM 格式

`mod_audio_stream` 传输的是原始 PCM 音频数据：
- **采样率**: 8000 Hz (8k) 或 16000 Hz (16k)
- **位深度**: 16-bit
- **声道**: 单声道 (mono) 或立体声 (stereo)
- **编码**: PCM 线性编码

### 音频混合模式

- **mono**: 只传输被叫方（客户）音频
- **mixed**: 传输坐席和客户的混合音频
- **stereo**: 左声道坐席，右声道客户

## 故障排查

### 问题 1: 模块未加载

**症状**: `uuid_audio_stream` 命令不存在

**解决方案**:
```bash
# 检查模块文件
docker exec cc-freeswitch ls -la /usr/local/freeswitch/lib/freeswitch/mod/mod_audio_stream.so

# 手动加载模块
docker exec cc-freeswitch fs_cli -x "load mod_audio_stream"

# 检查错误日志
docker logs cc-freeswitch --tail 50 | grep -i audio
```

### 问题 2: WebSocket 连接失败

**症状**: FreeSWITCH 日志显示 WebSocket 连接错误

**解决方案**:
1. 确认 WebSocket 服务器正在运行
2. 检查防火墙和网络连接
3. 确认 WebSocket URL 格式正确
4. 查看 FreeSWITCH 日志获取详细错误

### 问题 3: 音频质量差

**症状**: 音频延迟高或音质差

**解决方案**:
1. 降低采样率 (8k vs 16k)
2. 使用 TCP WebSocket (wss://) 而非 UDP
3. 检查网络延迟
4. 调整缓冲区大小

## 性能优化建议

### 1. 网络优化

- 使用专线或内网连接
- 启用 TCP 长连接
- 配置合理的 keep-alive

### 2. 资源优化

- 限制并发音频流数量
- 实施流控机制
- 监控资源使用

### 3. 监控和日志

```bash
# 查看音频流相关日志
docker logs cc-freeswitch --tail 100 | grep -E "(audio_stream|WebSocket)"

# 监控活跃呼叫
docker exec cc-freeswitch fs_cli -x "show calls count"
```

## 安全建议

1. **使用 WSS**: 生产环境必须使用 TLS 加密的 WebSocket
2. **认证机制**: 实现 token 或签名认证
3. **访问控制**: 限制 WebSocket 服务器访问
4. **数据加密**: 敏感音频数据加密传输
5. **审计日志**: 记录所有音频流操作

## 参考资源

- [mod_audio_stream 官方文档](https://freeswitch.org/confluence/display/FREESWITCH/mod_audio_stream)
- [FreeSWITCH ESL API](https://freeswitch.org/confluence/display/FREESWITCH/Event+Socket+Library)
- [云枢 AI 引擎实现](internal/domain/callflow/ai_engine.go)
- [ESL 命令构建器](internal/infra/fsesl/command_builder.go)
