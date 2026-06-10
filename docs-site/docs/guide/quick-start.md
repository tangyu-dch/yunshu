---
title: 快速开始
order: 2
---

# 快速开始

本文用于在本地或一台开发服务器上快速拉起 云枢声讯的核心服务，并完成基础呼叫验证。

## 1. 环境要求

| 依赖 | 推荐版本 | 用途 |
| --- | --- | --- |
| Go | 1.21+ | 后端服务编译和运行 |
| Node.js | 18+ | 前端和文档站开发 |
| MySQL | 8.0 | 业务数据、CDR、outbox |
| Redis | 7.x | 分机状态、选号并发、队列 |
| Docker | 24+ | 本地基础设施 |
| SIPp | 可选 | SIP 端到端验证 |

## 2. 启动基础设施

```bash
docker compose up -d mysql redis rtpengine freeswitch kamailio
```

确认容器状态：

```bash
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
```

至少应看到：

```text
cc-mysql
cc-redis
cc-rtpengine
cc-freeswitch
cc-kamailio
```

## 3. 检查配置

默认配置文件：

```text
configs/default.yaml
```

重点确认：

```yaml
mysql:
  dsn: root:db123456@tcp(127.0.0.1:3306)/yunshu?charset=utf8mb4&parseTime=True&loc=Local

redis:
  addrs:
    - 127.0.0.1:63790

freeswitch:
  kamailioAddr: "192.168.107.2:5060"
```

其中 `kamailioAddr` 是 **FreeSWITCH 容器访问 Kamailio 的地址**，不是宿主机 `127.0.0.1`。

## 4. 启动后端

开发环境推荐用 `cc-all`：

```bash
SERVICE_INSTANCE_ID=local-main go run ./cmd/cc-all -config configs/default.yaml
```

`SERVICE_INSTANCE_ID` 用于区分 FreeSWITCH 事件租约 owner，避免本机多实例冲突。

默认端口：

| 服务 | 地址 | 说明 |
| --- | --- | --- |
| cc-console | `:8080` | 控制台 |
| cc-edge | `:8081` | 边缘网关 |
| cc-call | `:8082` | CTI/ESL |
| cc-worker | `:8083` | outbox/异步任务 |

健康检查：

```bash
curl http://127.0.0.1:8082/healthz
```

成功示例：

```json
{"code":0,"message":"成功","data":{"service":"cc-call","status":"UP"}}
```

## 5. 检查 FreeSWITCH ESL

启动日志应出现：

```text
Successfully authenticated <fs>:8021
FreeSWITCH ESL 连接已成功并启用事件
FreeSWITCH 事件租约声明成功
```

如果出现 `broken pipe`，通常是：

- 同机运行了多个旧的 `cc-call/cc-all`
- FreeSWITCH 连接陈旧
- 没有设置唯一 `SERVICE_INSTANCE_ID`

新版本会自动清理死连接并重连一次。

## 6. 启动前端

```bash
cd web
npm install
npm run dev
```

## 7. 验证呼入

```bash
bash scripts/sipp/run_e2e_tests.sh inbound
```

通过示例：

```text
PASS: 呼入 - 客户侧完整信令 (INVITE→200 OK→ACK→BYE)
```

呼入验证会执行：

1. 初始化测试商户、分机、DID、技能组、Redis 分机状态。
2. 使用 SIPp 注册坐席 `1001`。
3. 启动坐席 UAS。
4. 发起客户呼入 DID。
5. 云枢声讯 分配坐席并桥接。
6. 挂断后写入 CDR outbox。

## 8. 验证 API 外呼

```bash
bash scripts/sipp/run_e2e_tests.sh api
```

API 入口应返回：

```text
API 响应: HTTP 200
```

脚本会动态从数据库读取分机 `1001` 当前 user_id，并自动生成 `callId`。

## 9. 验证云枢声讯直呼

```bash
bash scripts/sipp/run_e2e_tests.sh dialpad
```

如果本机出现：

```text
Unable to send UDP message: No route to host
```

说明是宿主机 Docker 网络的 UDP 回包问题。建议改为：

```bash
SIPP_UAS_MODE=docker bash scripts/sipp/run_e2e_tests.sh dialpad
```

如果本地无法构建 SIPp Docker 镜像，请在 Linux 服务器上验证。

## 10. 检查 CDR

只要呼叫进入云枢声讯并收到最终挂断事件，应写入 CDR：

```sql
SELECT call_id, profile, hangup_cause, duration_sec
FROM call_cdr_record
ORDER BY completed_at DESC
LIMIT 10;
```

同时 outbox 应有：

```text
call_center_cdr_queue
cti_cdr_billing
cti_cdr_recording
cti_cdr_report_projection
cti_cdr_downstream_push
```

## 11. 常见问题速查

### API 外呼返回 400

检查：

- 是否请求 `cc-call:8082`
- URL 是否包含 `callId`
- `userId` 是否为有效分机用户
- `callee` 是否为空

### 坐席腿 UNALLOCATED_NUMBER

检查 Kamailio location：

```sql
SELECT username, domain, contact FROM cc_res_location WHERE username='1001';
```

确保 R-URI 是：

```text
1001@sip.merchant.yunshu.com
```

### 呼叫没有通话记录

检查是否收到：

```text
CHANNEL_HANGUP_COMPLETE
```

并确认 outbox：

```sql
SELECT id,destination,aggregate_id FROM message_outbox WHERE id LIKE 'cdr:%';
```
