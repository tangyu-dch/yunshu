---
title: 生产部署
order: 2
---

# 生产部署

本文档描述 云枢声讯 在生产环境中的推荐部署方式。生产部署需要同时考虑 SIP 信令、RTP 媒体、ESL 控制面、数据库、Redis、异步任务、录音存储、安全鉴权与可观测性。

## 1. 推荐拓扑

```text
                                  ┌──────────────────────┐
                                  │   云枢声讯 / SIP  │
                                  │   WebRTC / SIP Trunk  │
                                  └──────────┬───────────┘
                                             │ SIP 5060/5066
                                             ▼
                         ┌────────────────────────────────────┐
                         │              DMZ 区                 │
                         │                                    │
                         │  ┌────────────┐   ┌────────────┐  │
                         │  │ Kamailio   │──▶│ RTPEngine  │  │
                         │  └─────┬──────┘   └────────────┘  │
                         └────────┼───────────────────────────┘
                                  │ SIP dispatcher
                                  ▼
                         ┌────────────────────────────────────┐
                         │          内网媒体/业务区            │
                         │                                    │
                         │  ┌────────────┐  ESL  ┌─────────┐ │
                         │  │FreeSWITCH  │◀─────▶│ cc-call │ │
                         │  └────────────┘       └────┬────┘ │
                         │                            │      │
                         │       ┌────────────┐       │      │
                         │       │ cc-worker  │◀──────┘      │
                         │       └─────┬──────┘              │
                         └─────────────┼─────────────────────┘
                                       │
                      ┌────────────────┴────────────────┐
                      ▼                                 ▼
                ┌──────────┐                      ┌──────────┐
                │  MySQL   │                      │  Redis   │
                └──────────┘                      └──────────┘
```

## 2. 生产服务清单

| 服务 | 建议数量 | 说明 |
| --- | --- | --- |
| Kamailio | 2+ | SIP 边界、注册、dispatcher，高可用可通过 LVS/SLB/VIP |
| RTPEngine | 2+ | RTP 媒体代理，需公网/内网双地址 |
| FreeSWITCH | 2+ | 媒体节点，可按 set_id 分组 |
| cc-call | 2+ | CTI/ESL 事件与命令，FS 事件通过租约单 owner |
| cc-console | 2+ | 控制台和商户后台 |
| cc-edge | 2+ | 对外 OpenAPI 网关 |
| cc-worker | 2+ | outbox、CDR、计费、录音、回调 |
| MySQL | 主从/集群 | 强一致业务数据 |
| Redis | 哨兵/集群 | 热状态、分机状态、选号并发、队列 |
| 对象存储/NAS | 1+ | 录音、TTS、静态文件 |

## 3. 网络规划

### 3.1 端口规划

| 组件 | 端口 | 协议 | 说明 |
| --- | --- | --- | --- |
| Kamailio | 5060 | UDP/TCP | SIP 信令 |
| Kamailio | 5066 | TCP/WSS | WebSocket SIP/WebRTC |
| RTPEngine | 2223/22222 | UDP | 控制端口 |
| RTPEngine | 30000-40000 | UDP | RTP 端口范围 |
| FreeSWITCH SIP | 5080 | UDP/TCP | external profile，Kamailio dispatcher 目标 |
| FreeSWITCH ESL | 8021 | TCP | cc-call 控制面 |
| cc-console | 8080 | HTTP | 控制台 |
| cc-edge | 8081 | HTTP | 外部 API 网关 |
| cc-call | 8082 | HTTP | CTI/ESL/Kamailio webhook |
| cc-worker | 8083 | HTTP | worker health/contract |
| ASR WS | 9002 | WebSocket | mod_audio_stream 推流 |

### 3.2 安全边界

建议：

- Kamailio 暴露公网 SIP/WebSocket 端口。
- FreeSWITCH 不直接暴露公网，只允许 Kamailio/cc-call 内网访问。
- FreeSWITCH ESL `8021` 只允许 cc-call 所在内网访问。
- MySQL/Redis 只允许业务服务访问。
- 录音存储只允许 FreeSWITCH/cc-worker 访问。

### 3.3 延迟要求

| 链路 | 建议 |
| --- | --- |
| cc-call ↔ FreeSWITCH ESL | < 1ms，最好同 VPC/同机房 |
| Kamailio ↔ FreeSWITCH SIP | < 5ms |
| FreeSWITCH ↔ RTPEngine | < 2ms |
| cc-worker ↔ MySQL | < 5ms |

## 4. 配置文件准备

复制默认配置：

```bash
cp configs/default.yaml configs/production.yaml
```

重点修改：

```yaml
service:
  name: cc-call
  instanceId: "${HOSTNAME}-${POD_NAME}"

mysql:
  dsn: "yunshu:password@tcp(mysql-vip:3306)/yunshu?charset=utf8mb4&parseTime=True&loc=Local"

redis:
  addrs:
    - redis-vip:6379

freeswitch:
  kamailioAddr: "kamailio-inner-vip:5060"
  eventLeaseTTL: 30s

worker:
  downstream:
    url: "https://merchant.example.com/yunshu/cdr"
    secret: "shared-secret"
  recording:
    oss:
      endpoint: "https://oss.example.com"
      accessKey: "xxx"
      secretKey: "xxx"
      bucket: "yunshu-recordings"
```

## 5. MySQL 初始化

### 5.1 创建数据库

```sql
CREATE DATABASE yunshu DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
```

### 5.2 初始化基础表

生产推荐使用应用 AutoMigrate + 初始化脚本组合：

```bash
go run ./cmd/cc-all -config configs/production.yaml
```

首次启动时会自动迁移：

- 分机/号码池/技能组/网关表
- CDR 表
- outbox 表
- 计费流水表
- 录音任务表
- 报表投影表
- 下游推送表

### 5.3 Kamailio location 表注意事项

Kamailio `usrloc` 写回时可能写入 `NULL methods`，建议表结构允许：

```sql
ALTER TABLE cc_res_location MODIFY methods INT DEFAULT NULL;
```

## 6. Redis 初始化

Redis 用于：

- 分机状态：`extension:status`
- 选号并发：`cti:select:*`
- 呼入/预测队列：`cti:merchant:{merchantId}:queue:skill_group:{skillGroupId}`
- WebSocket 投影：`batch:{taskId}:*`
- 余额缓存

生产建议：

```text
Redis Sentinel / Cluster
AOF enabled
maxmemory-policy noeviction
```

## 7. Kamailio 部署

### 7.1 关键模块

必须启用：

- `auth_db`
- `usrloc`
- `registrar`
- `dispatcher`
- `rtpengine`
- `http_client`

### 7.2 关键表

| 表 | 用途 |
| --- | --- |
| `cc_res_extension` | 分机鉴权 |
| `cc_res_location` | 注册位置 |
| `cc_res_freeswitch` | dispatcher 目标 |
| `cc_res_rtpengine` | RTPEngine 节点 |

### 7.3 Dispatcher 目标

Kamailio dispatcher 应指向 FreeSWITCH external profile，例如：

```text
sip:10.0.10.20:5080
```

不要指向 FreeSWITCH internal profile，否则可能被 FS 鉴权挑战或走错 dialplan。

### 7.4 内部呼叫路由

FreeSWITCH 呼叫坐席腿时需携带：

```text
X-Internal-Call: true
```

并使用：

```text
1001@sip.merchant.yunshu.com;fs_path=sip:kamailio-inner-vip:5060
```

## 8. FreeSWITCH 部署

### 8.1 ESL

修改 `event_socket.conf.xml`：

```xml
<param name="listen-ip" value="0.0.0.0"/>
<param name="listen-port" value="8021"/>
<param name="password" value="strong-password"/>
```

只允许 cc-call 内网访问。

### 8.2 SIP profile

建议：

- external profile 用于 Kamailio dispatcher 进来的 DID/云枢声讯呼叫。
- internal profile 可用于内网测试，但不要作为 Kamailio dispatcher 主目标。

### 8.3 public dialplan

云枢声讯 需要捕获呼入/云枢声讯呼叫：

```xml
<action application="answer" />
<action application="park" />
```

然后由 cc-call 根据 `CHANNEL_CREATE` 接管。

## 9. cc-call 部署

### 9.1 启动

```bash
SERVICE_INSTANCE_ID=call-1 ./cc-call -config configs/production.yaml -addr :8082
```

### 9.2 多实例

多实例可以同时部署，但同一个 FS 节点的事件消费只有一个 owner：

```text
cc_res_fs_lease
```

每个实例必须设置不同：

```bash
SERVICE_INSTANCE_ID=call-a
SERVICE_INSTANCE_ID=call-b
```

### 9.3 ESL 连接断开处理

系统会识别：

- broken pipe
- connection reset
- EOF
- use of closed network connection

然后清理连接并重试一次命令。

## 10. cc-worker 部署

### 10.1 启动

```bash
./cc-worker -config configs/production.yaml -addr :8083
```

### 10.2 Outbox 节点

Worker 负责：

```text
call_center_cdr_queue
cti_cdr_billing
cti_cdr_recording
cti_cdr_report_projection
cti_cdr_downstream_push
cti_cdr_recording_oss
```

### 10.3 自动迁移表

启动时会自动迁移：

- `call_cdr_record`
- `cc_biz_ledger`
- `call_billing_settlement_job`
- `cc_biz_recording`
- `call_report_projection`
- `call_downstream_push_job`

## 11. Nginx / 网关

### 11.1 控制台

```nginx
location / {
  proxy_pass http://cc-console:8080;
}
```

### 11.2 API 网关

```nginx
location /api/ {
  proxy_pass http://cc-edge:8081;
}
```

### 11.3 WebSocket

```nginx
location /cti/ws {
  proxy_pass http://cc-call:8082;
  proxy_http_version 1.1;
  proxy_set_header Upgrade $http_upgrade;
  proxy_set_header Connection "upgrade";
}
```

## 12. 生产验证

### 12.1 健康检查

```bash
curl http://cc-call:8082/healthz
curl http://cc-worker:8083/healthz
```

### 12.2 SIPp 验证

```bash
bash scripts/sipp/run_e2e_tests.sh inbound
bash scripts/sipp/run_e2e_tests.sh api
SIPP_UAS_MODE=docker bash scripts/sipp/run_e2e_tests.sh dialpad
```

### 12.3 CDR 检查

```sql
SELECT call_id, profile, hangup_cause, duration_sec, completed_at
FROM call_cdr_record
ORDER BY completed_at DESC
LIMIT 10;
```

## 13. 上线前检查

- [ ] Kamailio 能 REGISTER 分机
- [ ] `cc_res_location` 有有效 contact
- [ ] FreeSWITCH ESL 连接成功
- [ ] `cc_res_fs_lease` owner 正常续约
- [ ] 呼入 SIPp 通过
- [ ] API 外呼返回 200
- [ ] 云枢声讯直呼能进入 `api_direct`
- [ ] CDR outbox 写入成功
- [ ] `call_cdr_record` 有落库记录
- [ ] outbox 无持续失败任务
- [ ] Redis 分机状态正常
- [ ] 录音目录或 OSS 可写

## 14. 回滚方案

建议生产发布保留：

- 上一个二进制版本
- 上一个配置文件
- 数据库备份
- Kamailio/FreeSWITCH 配置备份

回滚顺序：

1. 停止新版本服务。
2. 恢复旧二进制和配置。
3. 重启 cc-call / cc-worker。
4. reload Kamailio / FreeSWITCH。
5. 使用 SIPp 验证基础呼入。

## 15. 常见生产故障

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| 呼入无事件 | FS 事件未进入 cc-call | 检查 ESL 连接和 EventFromESL |
| 坐席腿 404 | SIP 域不匹配 | 检查 R-URI 和 cc_res_location.domain |
| API 外呼 400 | callId/userId/callee 错误 | 检查请求参数 |
| CDR 缺失 | 未收到 HANGUP_COMPLETE | 检查 FS 事件和 outbox |
| Worker 一直重试 | 后置表缺失或下游不可用 | 检查 worker 日志和表迁移 |
| SIPp UAS no route | 本地 Docker UDP 路由问题 | 换 Linux 或容器内 UAS |
