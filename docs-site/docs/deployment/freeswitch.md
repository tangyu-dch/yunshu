---
title: FreeSWITCH
order: 3
---

# FreeSWITCH 部署

FreeSWITCH 是云枢声讯的话务媒体节点，负责承载 SIP 通道、执行 bridge/playback/record/audio-stream 等媒体动作，并通过 ESL 向 `cc-call` 暴露控制面。

## 1. 职责边界

FreeSWITCH 在 云枢声讯 中负责：

- 接收 Kamailio dispatcher 转发的 SIP INVITE。
- 使用 public dialplan 捕获 DID/云枢声讯呼叫。
- 通过 ESL 接收 `originate`、`uuid_bridge`、`uuid_kill`、`uuid_broadcast` 等命令。
- 推送 `CHANNEL_CREATE`、`CHANNEL_ANSWER`、`CHANNEL_BRIDGE`、`CHANNEL_HANGUP_COMPLETE` 等事件。
- 承载录音、播放等待音、AI `mod_audio_stream` 推流。

FreeSWITCH 不负责：

- 商户鉴权
- 坐席分配
- 选号策略
- CDR 计费
- 技能组队列

这些由 云枢声讯 Go 服务负责。

## 2. 推荐部署拓扑

```text
Kamailio:5060
   │ dispatcher
   ▼
FreeSWITCH external profile:5080
   │ ESL 8021
   ▼
cc-call
```

生产建议：

- SIP 入口只允许 Kamailio 访问 FreeSWITCH。
- ESL 只允许 cc-call 内网访问。
- RTP 通过 RTPEngine 转发或由 FreeSWITCH/RTPEngine 内网处理。

## 3. 端口规划

| 端口 | 协议 | 用途 | 是否公网暴露 |
| --- | --- | --- | --- |
| 5080 | UDP/TCP | external SIP profile，Kamailio dispatcher 目标 | 否，建议仅内网 |
| 8021 | TCP | ESL 控制端口 | 否，仅 cc-call |
| 16384-32768 | UDP | RTP 媒体端口 | 视部署方式决定 |
| 9002 | WS | 不是 FS 端口，cc-call ASR WS | 否，FS 内网访问 |

## 4. 安装方式

### 4.1 Docker 部署

本项目开发环境使用 Docker：

```bash
docker compose up -d freeswitch
```

检查：

```bash
docker ps | grep cc-freeswitch
```

### 4.2 原生部署

可使用官方包或源码安装：

```bash
apt-get install -y freeswitch freeswitch-mod-sofia freeswitch-mod-event-socket
```

确保安装模块：

- `mod_sofia`
- `mod_event_socket`
- `mod_dptools`
- `mod_commands`
- `mod_conference`（可选）
- `mod_audio_stream`（AI 语音流需要）

## 5. ESL 配置

文件通常为：

```text
autoload_configs/event_socket.conf.xml
```

示例：

```xml
<configuration name="event_socket.conf" description="Socket Client">
  <settings>
    <param name="nat-map" value="false"/>
    <param name="listen-ip" value="0.0.0.0"/>
    <param name="listen-port" value="8021"/>
    <param name="password" value="CHANGE_ME_STRONG_PASSWORD"/>
  </settings>
</configuration>
```

云枢声讯 配置中对应：

```yaml
freeswitch:
  commandTimeout: 5s
  eventLeaseTTL: 30s
```

数据库节点表对应：

```text
cc_res_freeswitch_node.address
cc_res_freeswitch_node.esl_port
cc_res_freeswitch_node.password
```

## 6. SIP Profile 配置

### 6.1 external profile

Kamailio dispatcher 应指向 FreeSWITCH external profile，例如：

```text
sip:10.0.10.20:5080
```

本地 Docker 示例：

```text
sip:192.168.107.6:5080
```

### 6.2 internal profile

internal profile 可用于内部测试或分机目录，但不要作为 Kamailio dispatcher 的主要目标，否则可能触发 FreeSWITCH 自身鉴权或错误 dialplan。

## 7. Public Dialplan

云枢声讯 依赖 FreeSWITCH 将外部呼叫捕获并 park，再由 cc-call 接管。

推荐 public dialplan：

```xml
<include>
  <extension name="yunshu_inbound_did" continue="false">
    <condition field="destination_number" expression="^(0\d{2,3}\d{7,8}|1[3-9]\d{9})$">
      <action application="set" data="call_direction=inbound" />
      <action application="set" data="outside_call=true" />
      <action application="answer" />
      <action application="park" />
    </condition>
  </extension>
</include>
```

## 8. 配置修改提示

### 8.1 如果呼入 SIPp 超时

检查实际加载的 dialplan：

```bash
docker exec cc-freeswitch sh -lc \
  "grep -R 'yunshu_inbound_did\|answer\|park' -n /usr/local/freeswitch/conf/dialplan/public"
```

如果仍然只有 `park()` 没有 `answer()`，客户 UAC 可能一直等不到 200 OK。

### 8.2 如果坐席腿 UNALLOCATED_NUMBER

这通常不是 FreeSWITCH 端口问题，而是 Kamailio location 域不匹配。检查 FS originate 的目标：

```text
sofia/external/1001@sip.merchant.yunshu.com;fs_path=sip:<kamailio-ip>:5060
```

不要发成：

```text
sofia/external/1001@<kamailio-ip>:5060
```

因为 Kamailio 开启 `usrloc use_domain=1` 后会按域查找。

### 8.3 如果 ESL broken pipe

检查是否有多个旧 `cc-call/cc-all`：

```bash
ps -ef | grep cc-all
```

并确认启动时设置：

```bash
SERVICE_INSTANCE_ID=call-1
```

新版本会在 broken pipe 后清理连接并重试一次。

## 9. Reload 命令

```bash
fs_cli -H 127.0.0.1 -P 8021 -p ClueCon -x 'reloadxml'
```

Docker：

```bash
docker exec cc-freeswitch fs_cli -H 127.0.0.1 -P 8021 -p ClueCon -x 'reloadxml'
```

## 10. 验证命令

### 10.1 ESL 登录

```bash
docker exec cc-freeswitch fs_cli -H 127.0.0.1 -P 8021 -p ClueCon -x status
```

### 10.2 查看通道

```bash
docker exec cc-freeswitch fs_cli -H 127.0.0.1 -P 8021 -p ClueCon -x 'show channels'
```

### 10.3 查看日志

```bash
docker logs -f cc-freeswitch
```

## 11. 与 云枢声讯的关键联动

| FreeSWITCH 事件 | 云枢声讯 行为 |
| --- | --- |
| CHANNEL_CREATE | 创建/识别会话，推进工作流 |
| CHANNEL_PROGRESS | 更新坐席状态、播放回铃音 |
| CHANNEL_PROGRESS_MEDIA | 处理早期媒体 |
| CHANNEL_ANSWER | 标记 ready，尝试 bridge |
| CHANNEL_BRIDGE | 工作流进入 bridged |
| CHANNEL_HANGUP_COMPLETE | 写入 CDR outbox |

## 12. 生产建议

- ESL 密码必须修改。
- FreeSWITCH 不直接暴露公网。
- external profile 仅允许 Kamailio 访问。
- 录音路径应放在共享盘或可被 cc-worker 读取的位置。
- 高并发场景建议多个 FS 节点按 `set_id` 分组。
