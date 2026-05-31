# 云枢 Docker 本地部署与 mod_audio_stream 语音旁路推流排障白皮书

在本地通过 Docker 部署 “云枢” 呼叫中心系统（包含 FreeSWITCH、Kamailio 边缘代理）并使用开源项目 `mod_audio_stream` 进行实时 ASR 语音旁路推流时，由于容器网络隔离、WSS 自签名证书校验和浏览器的 Mixed Content 安全策略，常常会遇到 WebSocket 握手失败或音频流丢包的经典网络问题。

本白皮书为您提供电信级的 Docker 网络连通与推流排障一键配通方案。

---

## 🌐 1. Docker 网络拓扑与端口规划

在本地 Docker 容器环境中，FreeSWITCH 需要将实时音频流（通过 WebSocket 协议）旁路发送给位于宿主机或另一个容器中的 ASR/STT 语音网关。

### 推荐的容器网络拓扑 (Docker Host Network)

由于实时语音（RTP）有极高的并发和极低的延迟要求，且 RTP 涉及数千个动态端口映射，**强烈建议在生产或本地真实测试时，将 FreeSWITCH 与 Kamailio 容器以 `host` 网络模式运行**：

```yaml
version: '3.8'
services:
  freeswitch:
    image: yunshu-freeswitch:v1.0
    network_mode: "host"  # 直接共享宿主机网络栈，避免 DNAT 造成的 RTP 丢包与延迟
    volumes:
      - ./conf:/usr/local/freeswitch/conf
    restart: always

  asr-gateway:
    image: yunshu-asr-gateway:latest
    ports:
      - "9002:9002"  # ASR WebSocket 监听端口
    restart: always
```

> [!IMPORTANT]
> 如果 FreeSWITCH 必须以 Bridge 桥接网络模式运行，请确保宿主机网络与容器内子网可以通过网关互通，并且在 FreeSWITCH 中配置 `ext-rtp-ip` 与 `ext-sip-ip` 为宿主机的真实物理 IP 地址，否则音频流将因 NAT 穿透失败而静音。

---

## 🔒 2. WSS 自签名证书与 Mixed Content 安全拦截

当商户平台在 HTTPS 模式（如访问 `https://127.0.0.1:3000`）下运行时，浏览器安全策略有两项非常严格的物理拦截防线：

### 1) WSS (WebSocket Secure) 与自签名证书拦截
- **痛点**：如果 ASR 网关或信令服务器配置的是自签名的 SSL 证书，由于浏览器不信任该证书，`new WebSocket("wss://...")` 物理建链时会在控制台静默抛出 `ERR_CERT_AUTHORITY_INVALID`，并不向应用层投递任何错误，直接断开！
- **完美解决方案**：
  1. 在浏览器中**新建一个标签页**，手动访问 ASR 的 WebSocket 端口 HTTPS 地址（如访问 `https://127.0.0.1:9002`）。
  2. 此时浏览器会拦截并显示“您的连接不是私密连接”。
  3. 点击“高级” -> “继续前往 127.0.0.1（不安全）”。
  4. 一旦您在浏览器中手动豁免并“信任此证书”，浏览器便会彻底放开拦截，允许流图编辑器或 SDK 进行无感知的 `wss://` WebSocket 握手！

### 2) Mixed Content (混合内容拦截)
- **痛点**：若在 HTTPS 页面中，尝试连接未加密的 `ws://`（非安全端口），浏览器会直接静默拦截该连接。
- **解决方案**：本地开发和测试期间，请确保使用 `http://127.0.0.1` 降级协议访问商户控制台，此时浏览器将放宽限制，允许自由连接本地的 `ws://` 非加密 WebSocket 端口。

---

## 📞 3. mod_audio_stream API 参数调试清单

编译安装 [amigniter/mod_audio_stream](https://github.com/amigniter/mod_audio_stream) 后，在 FreeSWITCH 命令行（`fs_cli`）中可使用以下命令进行深度排障：

### 1) 查看模块加载状态
```bash
fs_cli> show modules
# 确认是否存在 mod_audio_stream
```
如果未加载，手动加载：
```bash
fs_cli> load mod_audio_stream
```

### 2) 手动模拟推流测试
当电话接通且处于活动信道时，获取信道 UUID：
```bash
fs_cli> show channels
# 复制目标 UUID，例如: f81d4fae-7dec-11d0-a765-00a0c91e6bf6
```
手动向本地 ASR 测试网关发起音频旁路推送：
```bash
fs_cli> uuid_audio_stream f81d4fae-7dec-11d0-a765-00a0c91e6bf6 start ws://127.0.0.1:9002 mono 16k {"merchantId":"test_mch"}
```
- 观察 ASR 测试网关的控制台，确认是否收到了**首帧元数据 JSON 字符串**，以及后续源源不断的 **16000Hz 单声道 PCM 二进制音频包**！

### 3) 停止推流
```bash
fs_cli> uuid_audio_stream f81d4fae-7dec-11d0-a765-00a0c91e6bf6 stop
```

---

## 🧪 4. ASR 语音推流极速验证服务

为了让您能够秒级验证 `mod_audio_stream` 音频推流是否成功到达、连接是否稳定、首帧 Metadata 属性是否解析正确，我们在 `scratch/asr_receiver.py` 中为您准备了一个极简的 ASR 语音接收测试服务。您可以直接使用 Python 一键拉起进行抓包验证！
