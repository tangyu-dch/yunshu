---
title: RTPEngine
order: 5
---

# RTPEngine 部署

RTPEngine 是 云枢声讯/Kamailio 体系中的 RTP 媒体代理，用于解决 NAT 穿越、媒体地址重写、RTP 转发和 WebRTC/传统 SIP 媒体兼容问题。

## 1. 为什么需要 RTPEngine

SIP 信令里携带 SDP，SDP 中声明了媒体 IP 和端口。如果终端在公网、内网、NAT 或 WebRTC 环境中混合存在，媒体可能无法互通。

RTPEngine 负责：

- 替换 SDP 中的媒体地址。
- 转发 RTP 包。
- 处理 NAT 场景。
- 支持 WebRTC 与传统 RTP 互通。
- 减轻 FreeSWITCH 媒体转发压力。

## 2. 推荐拓扑

```text
SIP 终端 / 运营商
       │ SIP
       ▼
   Kamailio
       │ rtpengine_manage()
       ▼
   RTPEngine
       │ RTP
       ▼
 FreeSWITCH / SIP 终端
```

## 3. 部署方式

### 3.1 Docker 部署

本地开发环境：

```bash
docker compose up -d rtpengine
```

查看容器：

```bash
docker ps | grep rtpengine
```

### 3.2 Linux 原生部署

Ubuntu/Debian 示例：

```bash
apt-get update
apt-get install -y rtpengine
```

启动：

```bash
systemctl enable rtpengine
systemctl start rtpengine
```

## 4. 端口规划

| 端口 | 协议 | 说明 |
| --- | --- | --- |
| 2223/22222 | UDP | Kamailio NG 控制协议 |
| 30000-40000 | UDP | RTP 媒体端口范围 |

生产环境应在防火墙放行 RTP 范围。

## 5. 单网卡配置

如果 Kamailio、FreeSWITCH、终端都在同一内网，可使用单接口：

```ini
interface = 10.0.10.30
listen-ng = 10.0.10.30:2223
port-min = 30000
port-max = 40000
```

## 6. 公私网双网卡配置

云服务器常见场景：

- 内网 IP：`10.0.10.30`
- 公网 IP：`1.2.3.4`

配置：

```ini
interface = internal/10.0.10.30;external/1.2.3.4
listen-ng = 10.0.10.30:2223
port-min = 30000
port-max = 40000
```

Kamailio 中：

```cfg
modparam("rtpengine", "rtpengine_sock", "udp:10.0.10.30:2223")
```

## 7. Docker Compose 示例

```yaml
rtpengine:
  image: jambonz/rtpengine:latest
  restart: always
  environment:
    - RTP_START_PORT=30000
    - RTP_END_PORT=30100
    - LOGLEVEL=6
  command:
    - rtpengine
    - --listen-ng=2223
  ports:
    - "2223:2223/udp"
    - "30000-30100:30000-30100/udp"
```

生产环境建议扩大端口范围：

```yaml
ports:
  - "30000-40000:30000-40000/udp"
```

## 8. Kamailio 集成

加载模块：

```cfg
loadmodule "rtpengine.so"
```

配置 socket：

```cfg
modparam("rtpengine", "rtpengine_sock", "udp:rtpengine:2223")
```

处理 SDP：

```cfg
route[MANAGE_MEDIA] {
    if (is_request() && has_body("application/sdp")) {
        rtpengine_manage("trust-address replace-origin replace-session-connection");
    } else if (is_reply() && has_body("application/sdp")) {
        rtpengine_manage("trust-address replace-origin replace-session-connection");
    }
}
```

## 9. 数据库管理

云枢声讯 运营端可通过表管理 RTPEngine：

```text
cc_res_rtpengine
```

常见字段：

| 字段 | 说明 |
| --- | --- |
| set_id | RTP engine 分组 |
| rtpengine_sock | 控制 socket |
| enable | 是否启用 |

示例：

```sql
INSERT INTO cc_res_rtpengine(set_id, rtpengine_sock, enable, del_flag)
VALUES (1, 'udp:rtpengine:2223', 1, 0);
```

## 10. 配置修改提示

### 10.1 出现单通

检查：

- SDP 中的 IP 是否是内网地址。
- RTPEngine 是否收到 offer/answer。
- 防火墙是否放行 RTP 端口。
- Kamailio 是否调用 `rtpengine_manage()`。

### 10.2 RTP 无流量

检查端口：

```bash
ss -lunp | grep 30000
```

或 Docker：

```bash
docker logs cc-rtpengine
```

### 10.3 WebRTC 场景

WebRTC 需要额外关注：

- ICE
- DTLS
- SRTP
- WSS
- 浏览器证书和 HTTPS

云枢声讯 桌面 SIP 模式可先按传统 SIP/RTP 验证，WebRTC 再单独调优。

## 11. 验证命令

查看 RTPEngine 日志：

```bash
docker logs -f cc-rtpengine
```

查看 Kamailio 是否调用 RTPEngine：

```bash
docker logs -f cc-kamailio | grep rtpengine
```

发起 SIPp 呼叫后，应能看到 RTPEngine offer/answer 日志。

## 12. 生产建议

- RTPEngine 尽量与 Kamailio 同机房部署。
- RTP 端口范围应按并发量放大。
- 多节点部署时使用 set_id 分组。
- 公网部署必须正确配置 external IP。
- 云安全组、防火墙、iptables 必须放通 UDP RTP 端口。
