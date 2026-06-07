# FreeSWITCH 实时语音构建指南 ✅ 已完成

## 🎉 状态：成功！mod_audio_stream 模块已经安装并可使用

## 当前可用镜像

| 镜像 | 说明 |
|------|------|
| `yunshu/freeswitch:ai-audio` | 已安装 mod_audio_stream 模块，支持实时语音流 |
| `yunshu/freeswitch:latest` | 同 ai-audio，作为默认 |

---

## 如何使用新镜像

### 1. 验证模块是否工作

```bash
# 检查模块是否加载
docker exec -i cc-freeswitch /usr/local/freeswitch/bin/fs_cli -x 'show modules' | grep audio_stream

# 测试命令
docker exec -i cc-freeswitch /usr/local/freeswitch/bin/fs_cli -x 'uuid_audio_stream'
```

### 2. 在通话中开始推流

```
# 在 fs_cli 中
uuid_audio_stream <call_uuid> start ws://<your-server>:<port> mono 16000
```

---

## 构建历史记录 (已完成的工作)

### ✅ 已完成

1. 下载 mod_audio_stream 源码
2. 初始化子模块 (libwsc)
3. 安装所有依赖 (libevent-dev, cmake, 等等)
4. 使用 CMake 成功编译
5. 安装到 /usr/local/freeswitch/lib/freeswitch/mod/mod_audio_stream.so
6. 在运行的 FreeSWITCH 中加载验证
7. 提交为新镜像 `yunshu/freeswitch:ai-audio` 和 `yunshu/freeswitch:latest`

---

## 如果需要重新构建整个镜像 (高级用户)

我们已经有现成的镜像可用，通常不需要重新构建。但如果你需要：

```bash
# 1. 启动一个基础容器
docker run -d --name=fs-build bytedesk/freeswitch:latest

# 2. 进入容器执行构建
docker exec -it fs-build bash
# 然后执行以下步骤：
apt-get update && apt-get install -y git build-essential cmake libssl-dev zlib1g-dev libspeexdsp-dev libevent-dev
cd /tmp
git clone https://github.com/amigniter/mod_audio_stream.git
cd mod_audio_stream
git submodule update --init --recursive
export PKG_CONFIG_PATH=/usr/local/freeswitch/lib/pkgconfig
mkdir -p build && cd build
cmake -DCMAKE_INSTALL_PREFIX=/usr/local/freeswitch -DCMAKE_BUILD_TYPE=Release ..
make -j4
make install

# 3. 提交镜像
docker commit fs-build yunshu/freeswitch:ai-audio
```

---

## 下一步

现在你可以开始：
1. 集成 ASR 服务 (实时语音识别)
2. 使用 RAG 引擎增强 AI 能力
3. 测试完整的 AI 通话流程
