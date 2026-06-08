# 云枢 AI 实时语音集成文档

## 🎉 恭喜！FreeSWITCH mod_audio_stream 模块安装成功！

## 快速验证

### 1. 验证模块是否加载

```bash
# 进入 FS CLI
docker exec -it cc-freeswitch /usr/local/freeswitch/bin/fs_cli
```

然后在 fs_cli 中输入：
```
show modules
```

你应该能看到这一行：
```
api,uuid_audio_stream,mod_audio_stream,/usr/local/freeswitch/lib/freeswitch/mod/mod_audio_stream.so
```

### 2. 测试 uuid_audio_stream 命令

```bash
docker exec -it cc-freeswitch /usr/local/freeswitch/bin/fs_cli -x 'uuid_audio_stream'
```

输出应该显示命令帮助信息！

---

## 使用 uuid_audio_stream

### 基本用法

```
uuid_audio_stream <uuid> start ws://<your-server-url>:<port> mono 16000
```

### 示例

#### 在通话中开始推流到 WebSocket ASR 服务

```
uuid_audio_stream <call-uuid> start ws://host.docker.internal:9002 mono 16000
```

#### 停止推流

```
uuid_audio_stream <call-uuid> stop
```

---

## 与云枢 AI 引擎集成

### 1. 配置 AI 引擎

修改 `configs/default.yaml`，启用 AI 功能：

```yaml
ai:
  enabled: true
  embedder:
    provider: "openai"
    apiKey: "<your-api-key>"
    model: "text-embedding-3-small"
  vectorDB:
    type: "qdrant"  # 或 "memory"
    address: "http://localhost:6333"
    collection: "yunshu_knowledge"
  rag:
    topK: 5
    scoreThreshold: 0.7
    maxTokens: 4000
```

### 2. 启动 Qdrant 向量数据库

```bash
docker run -d -p 6333:6333 -v qdrant_data:/qdrant/storage --name=cc-qdrant qdrant/qdrant:v1.7.4
```

或使用我们提供的 `docker-compose-dev.yml`

---

## 可用的 Docker 镜像

| 镜像 | 说明 |
|------|------|
| `yunshu/freeswitch:latest` | 含 mod_audio_stream 模块的最新版 |
| `yunshu/freeswitch:ai-audio` | 同 latest，AI 音频版 |
| `bytedesk/freeswitch:latest` | 原版 (无 mod_audio_stream) |

---

## 如何重新构建镜像 (需要时)

### 如果想从源码重新构建

1. 首先启动一个基础容器：
```bash
docker run -d --name=fs-build bytedesk/freeswitch:latest
```

2. 进入并构建（参考之前的步骤）

3. 提交新镜像：
```bash
docker commit fs-build yunshu/freeswitch:ai-audio
```

---

## AI 引擎组件列表

### 已实现的功能

| 组件 | 状态 | 路径 |
|------|------|------|
| RAG 引擎 | ✅ 完成 | `internal/domain/rag/` |
| OpenAI 嵌入 | ✅ 完成 | `internal/infra/embedding/` |
| Qdrant 向量存储 | ✅ 完成 | `internal/infra/embedding/qdrant_store.go` |
| 内存向量存储 | ✅ 完成 | `internal/domain/rag/memory_store.go` |
| FreeSWITCH 实时音频流 | ✅ 完成 | `mod_audio_stream` 已安装 |

### 下一步开发任务

| 任务 | 优先级 |
|------|------|
| 对话历史管理 | 中 |
| 知识库管理界面 | 中 |
| ASR 服务集成 | 高 |
| TTS 服务完善 | 中 |

---

## 常见问题

### Q: mod_audio_stream 重启后不加载？

A: 确保你的 `modules.conf.xml` 包含：
```xml
<load module="mod_audio_stream"/>
```

我们的镜像已经配置好自动加载了。

### Q: 音频流怎么处理？

A: WebSocket 服务端应该先接收一个 JSON 头帧，包含元数据，之后是原始二进制音频流（16kHz 16bit PCM）。

---

## 参考文档

- [Audio Stream Docker Guide](docs/go-rewrite/AUDIO_STREAM_DOCKER_GUIDE.md)
- [RAG 引擎 API 设计](internal/domain/rag/rag_engine.go)
