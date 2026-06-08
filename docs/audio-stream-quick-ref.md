# FreeSWITCH mod_audio_stream 快速参考

## 命令速查

### 基本命令

```bash
# 检查模块状态
docker exec cc-freeswitch fs_cli -x "show modules" | grep audio_stream

# 查看命令帮助
docker exec cc-freeswitch fs_cli -x "uuid_audio_stream"

# 加载模块
docker exec cc-freeswitch fs_cli -x "load mod_audio_stream"
```

### 音频流控制

```bash
# 启动音频流
uuid_audio_stream <uuid> start wss://server/audio mono 8k
uuid_audio_stream <uuid> start wss://server/audio mixed 16k
uuid_audio_stream <uuid> start wss://server/audio stereo 16k "metadata"

# 停止音频流
uuid_audio_stream <uuid> stop

# 暂停
uuid_audio_stream <uuid> pause

# 恢复
uuid_audio_stream <uuid> resume

# 优雅关闭
uuid_audio_stream <uuid> graceful-shutdown
```

## 参数说明

| 参数 | 值 | 说明 |
|------|-----|------|
| `<uuid>` | UUID | FreeSWITCH 会话 UUID |
| `control` | start/stop/pause/resume/graceful-shutdown | 控制命令 |
| `url` | wss://... 或文件路径 | WebSocket 地址或本地文件 |
| `mix_type` | mono/mixed/stereo | 音频混合模式 |
| `sampling_rate` | 8000/16000 | 采样率 (Hz) |
| `metadata` | string | 可选元数据 |

## 音频混合模式

- **mono**: 单声道（默认），只传输被叫方音频
- **mixed**: 混合，坐席和客户音频混合
- **stereo**: 立体声，左声道坐席，右声道客户

## 采样率

- **8000** (8k): 标准质量，节省带宽
- **16000** (16k): 高清质量，推荐用于 AI 处理

## 常见用法

### 1. AI 实时语音识别

```bash
# 启动单声道 16k 音频流到 ASR 服务
uuid_audio_stream $UUID start wss://asr-service.example.com/recognize mono 16k "call_id=123"

# 处理完成后停止
uuid_audio_stream $UUID stop
```

### 2. 实时监控

```bash
# 混合模式监控双方对话
uuid_audio_stream $UUID start wss://monitor-server/audio mixed 16k
```

### 3. 录音备份

```bash
# 保存到本地文件
uuid_audio_stream $UUID start /tmp/call_${UUID}.raw mono 16k
```

## WebSocket 数据格式

### 输出音频数据

```
┌──────────────────────────────────────┐
│ Header (Optional)                   │
├──────────────────────────────────────┤
│ PCM Audio Data                      │
│ - 16-bit signed linear               │
│ - 8kHz or 16kHz sampling             │
│ - 1 channel (mono)                   │
└──────────────────────────────────────┘
```

### 接收文本（通过 send_text）

```json
{
  "text": "识别结果文本",
  "timestamp": 1234567890
}
```

## 故障排查

```bash
# 查看模块加载状态
docker exec cc-freeswitch fs_cli -x "module_exists mod_audio_stream"

# 查看音频流状态
docker exec cc-freeswitch fs_cli -x "uuid_audio_stream <uuid>"

# 查看详细日志
docker logs cc-freeswitch --tail 100 | grep audio_stream

# 检查 WebSocket 连接
docker exec cc-freeswitch fs_cli -x "show connections"
```

## 云枢集成

云枢已经完整集成了 mod_audio_stream 功能：

- **Go SDK**: [internal/domain/callflow/ai_engine.go](internal/domain/callflow/ai_engine.go)
- **命令构建**: [internal/infra/fsesl/command_builder.go](internal/infra/fsesl/command_builder.go)
- **配置示例**: 在 AI 流程节点中配置 `asrEnabled` 和 `wsUrl`

### Go 代码示例

```go
// 在 AI 流程节点中启用音频流
node := AIFlowNode{
    Type: "start",
    Metadata: map[string]interface{}{
        "asrEnabled":  true,
        "wsUrl":      "wss://your-ai-service/audio",
        "mixType":    "mono",
        "sampleRate": "16k",
        "metadata":   "call_id=xxx",
    },
}

// 云枢会自动执行:
// uuid_audio_stream <uuid> start wss://your-ai-service/audio mono 16k call_id=xxx
```

## 性能指标

- **延迟**: 典型的端到端延迟为 100-300ms
- **带宽**: ~256 kbps (16kHz, mono, 16-bit)
- **并发**: 支持数百个并发音频流（取决于硬件）
- **CPU**: 每个音频流约 5-10% CPU

## 安全建议

1. ✅ 生产环境必须使用 **WSS** (TLS)
2. ✅ 实现认证机制（Token/Signature）
3. ✅ 限制访问源 IP
4. ✅ 监控异常流量
5. ✅ 记录审计日志

## 相关文件

- 模块配置: `/usr/local/freeswitch/conf/autoload_configs/audio_stream.conf.xml`
- 模块文件: `/usr/local/freeswitch/lib/freeswitch/mod/mod_audio_stream.so`
- 自动加载: `/usr/local/freeswitch/conf/autoload_configs/modules.conf.xml`
- 验证脚本: [scripts/verify-audio-stream.sh](scripts/verify-audio-stream.sh)
- 测试指南: [docs/audio-stream-test-guide.md](docs/audio-stream-test-guide.md)
