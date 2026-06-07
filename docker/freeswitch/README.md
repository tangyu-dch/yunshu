# 云枢 FreeSWITCH 构建指南

## 📋 概述

本项目使用包含 `mod_audio_stream` 模块的 FreeSWITCH 镜像，支持 AI 实时音频流处理。

---

## 🚀 快速开始

### 1. 构建 FreeSWITCH 镜像

```bash
# 进入 docker 目录
cd docker/freeswitch

# 构建镜像
docker build -t yunshu/freeswitch:latest .
```

### 2. 使用 docker-compose-dev.yml 启动完整环境

```bash
# 在项目根目录
docker-compose -f docker-compose-dev.yml up -d

# 查看日志
docker-compose -f docker-compose-dev.yml logs -f freeswitch
```

### 3. 验证 mod_audio_stream 是否加载成功

```bash
# 进入 FreeSWITCH 控制台
docker exec -it cc-freeswitch-dev fs_cli

# 在 fs_cli 中检查模块是否加载
freeswitch@dev> show modules
# 应该能看到 mod_audio_stream

# 或者直接测试 uuid_audio_stream 命令是否存在
freeswitch@dev> uuid_audio_stream
# 如果返回命令说明，表示模块加载成功
```

---

## 📝 使用 uuid_audio_stream

启动音频流推送到 ASR 服务（WebSocket）：

```
# 在 fs_cli 中
# 假设你有一个正在通话的 uuid（比如从 show channels 查看）
freeswitch@dev> uuid_audio_stream <call_uuid> start ws://host.docker.internal:9002 mono 16k

# 停止推流
freeswitch@dev> uuid_audio_stream <call_uuid> stop
```

---

## 🔧 故障排除

### mod_audio_stream 模块加载失败

```bash
# 查看 FreeSWITCH 启动日志
docker logs cc-freeswitch-dev 2>&1 | grep -i audio
```

### 构建失败

```bash
# 如果构建 mod_audio_stream 失败，可能需要尝试不同的分支
# 查看最新的 mod_audio_stream 版本
# https://github.com/amigniter/mod_audio_stream
```

---

## 📦 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| ESL_PASSWORD | ClueCon | ESL 连接密码 |

---

## 📊 端口说明

| 端口 | 协议 | 说明 |
|------|------|------|
| 5060 | UDP/TCP | SIP 信令 |
| 5080 | UDP/TCP | 备用 SIP |
| 8021 | TCP | ESL 连接 |
| 10000-20000 | UDP | RTP 媒体流 |
