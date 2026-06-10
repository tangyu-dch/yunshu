---
title: SIP 注册
order: 2
---

# SIP 注册

SIP 注册是云枢声讯接入系统的第一步，通过 Kamailio 完成鉴权和注册信息存储。

---

## 1. 注册流程

```mermaid
sequenceDiagram
    participant P as 云枢声讯
    participant K as Kamailio
    participant DB as MySQL
    participant C as cc-call

    P->>K: REGISTER sip:yunshu.io
    Note over P,K: From: <sip:1001@yunshu.io>
    Note over P,K: To: <sip:1001@yunshu.io>

    K->>K: 查找 auth_db 配置
    K->>DB: SELECT ha1b FROM cc_res_extension<br/>WHERE extension_number = '1001'
    DB-->>K: ha1b = "91b71b1b..."

    K-->>P: 401 Unauthorized
    Note over P,K: WWW-Authenticate: Digest<br/>realm="yunshu.io",<br/>nonce="abc123..."

    P->>K: REGISTER sip:yunshu.io (带 auth)
    Note over P,K: Authorization: Digest<br/>username="1001",<br/>realm="yunshu.io",<br/>nonce="abc123...",<br/>uri="sip:yunshu.io",<br/>response="xyz789..."

    K->>K: 计算 HA1
    K->>K: 验证 response
    K->>DB: INSERT/UPDATE cc_res_location

    K->>C: POST /cti/kamailio/auth/register
    Note over K,C: {<br/>  "extension": "1001",<br/>  "domain": "yunshu.io",<br/>  "contact": "sip:1001@192.168.1.100:5060",<br/>  "registered": true<br/>}

    C->>C: Redis SET extension:status:1001 = 0 (IDLE)
    C->>C: Redis SET extension:contact:1001 = "sip:1001@192.168.1.100:5060"

    K-->>P: 200 OK
```

---

## 2. Kamailio 配置要点

### auth_db 模块配置

```cfg
loadmodule "auth_db.so"

modparam("auth_db", "db_url", "mysql://user:pass@localhost/yunshu")
modparam("auth_db", "user_column", "extension_number")
modparam("auth_db", "domain_column", "domain")
modparam("auth_db", "password_column", "ha1b")
modparam("auth_db", "calculate_ha1", 0)
modparam("auth_db", "use_domain", 1)
```

### dispatcher 模块配置

```cfg
loadmodule "dispatcher.so"

modparam("dispatcher", "db_url", "mysql://user:pass@localhost/yunshu")
modparam("dispatcher", "table_name", "cc_res_freeswitch")
modparam("dispatcher", "setid_col", "group_id")
modparam("dispatcher", "destination_col", "sip_uri")
modparam("dispatcher", "flags_col", "flags")
modparam("dispatcher", "priority_col", "priority")
modparam("dispatcher", "attrs_col", "attrs")
```

### WebSocket 支持（端口 5066）

```cfg
listen=ws:0.0.0.0:5066

loadmodule "websocket.so"
loadmodule "nathelper.so"
```

---

## 3. 数据库表结构

### cc_res_extension（分机表）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | BIGINT | 主键 |
| merchant_id | BIGINT | 商户 ID |
| user_id | BIGINT | 用户 ID |
| extension_number | VARCHAR | 分机号 |
| password | VARCHAR | SIP 密码（明文，用于生成 ha1b） |
| ha1b | VARCHAR | HA1-B 哈希值 |
| domain | VARCHAR | SIP 域 |
| enable | TINYINT | 是否启用 |
| del_flag | TINYINT | 删除标记 |

### HA1-B 计算方式

```
HA1B = MD5(username ":" realm ":" password)
```

例如：
- username: `1001`
- realm: `yunshu.io`
- password: `sip123456`

```
HA1B = MD5("1001:yunshu.io:sip123456")
```

### cc_res_location（位置表）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | BIGINT | 主键 |
| username | VARCHAR | 用户名（分机号） |
| domain | VARCHAR | 域 |
| contact | VARCHAR | SIP Contact URI |
| received | VARCHAR | 接收地址 |
| path | VARCHAR | PATH 头部 |
| expires | INT | 过期时间（秒） |
| q | DOUBLE | q 值 |
| call_id | VARCHAR | Call-ID |
| cseq | INT | CSeq |
| last_modified | DATETIME | 最后修改时间 |
| flags | INT | 标志位 |
| user_agent | VARCHAR | User-Agent |
| socket | VARCHAR | Socket 信息 |
| methods | VARCHAR | 支持的方法 |

---

## 4. Webhook 回调

### 注册/注销回调

Kamailio 在注册成功或注销时会调用云枢声讯的 Webhook 接口。

**接口地址：** `POST /cti/kamailio/auth/register`

**请求内容：**
```json
{
  "extension": "1001",
  "domain": "yunshu.io",
  "contact": "sip:1001@192.168.1.100:5060",
  "received": "192.168.1.100:5060",
  "registered": true,
  "expires": 3600,
  "userAgent": "YunshuPhone/1.0.0",
  "callId": "abc-123-xyz@192.168.1.100"
}
```

**响应：**
```json
{
  "code": 0,
  "message": "success"
}
```

### cc-call 处理逻辑

```mermaid
graph TD
    A[收到 Webhook] --> B{registered?}
    B -->|true| C[更新分机状态为 IDLE]
    B -->|false| D[更新分机状态为 OFFLINE]
    C --> E[保存 contact 信息]
    D --> E
    E --> F[发布分机状态变更事件]
    F --> G[返回 200 OK]
```

---

## 5. Redis 状态存储

### 分机状态

**Key:** `extension:status:{extensionNumber}`

**Value:**
| 值 | 状态 | 说明 |
| --- | --- | --- |
| -1 | OFFLINE | 离线 |
| 0 | IDLE | 空闲 |
| 1 | BUSY | 忙碌 |
| 2 | RINGING | 振铃中 |
| 3 | ANSWERED | 已接听 |
| 4 | TALKING | 通话中 |
| 5 | ACW | 话后处理 |

**过期时间:** 永久（主动更新）

### 分机联系信息

**Key:** `extension:contact:{extensionNumber}`

**Value:** SIP Contact URI，例如：`sip:1001@192.168.1.100:5060`

**过期时间:** 3600 秒（随注册刷新）

### 分机注册信息

**Key:** `extension:registration:{extensionNumber}`

**Value:** JSON 格式
```json
{
  "extension": "1001",
  "domain": "yunshu.io",
  "contact": "sip:1001@192.168.1.100:5060",
  "received": "192.168.1.100:5060",
  "userAgent": "YunshuPhone/1.0.0",
  "registeredAt": 1718000000,
  "expiresAt": 1718003600
}
```

---

## 6. 注册保活

### 刷新注册

云枢声讯需要定期发送 REGISTER 请求刷新注册，通常在过期时间的一半时刷新。

```mermaid
sequenceDiagram
    participant P as 云枢声讯
    participant K as Kamailio
    participant C as cc-call

    P->>K: REGISTER (Expires: 3600)
    K->>C: Webhook
    K-->>P: 200 OK (Expires: 3600)

    Note over P: 等待 1800 秒

    P->>K: REGISTER (Expires: 3600)
    K->>C: Webhook
    K-->>P: 200 OK (Expires: 3600)

    Note over P: 循环往复
```

### 离线检测

如果 Kamailio 超过 `Expires` 时间未收到刷新注册，会自动清理位置记录。

云枢声讯也会通过以下方式检测离线：
1. WebSocket 连接断开
2. 定期 ping/pong
3. SIP OPTIONS 探测

---

## 7. 鉴权字段

| 字段 | 表 |
| --- | --- |
| extension_number | `cc_res_extension` |
| sip_domain | `cc_res_extension` |
| ha1 / ha1b | `cc_res_extension` |

---

## 8. SIPp 鉴权注册测试

```bash
sipp -sf scripts/sipp/register_auth_uac.xml \
  -s 1001 \
  -i <local-ip> \
  -p 6060 \
  kamailio:5060
```

---

## 9. 常见问题

### Q: 注册失败，返回 401 Unauthorized

**A:** 检查以下几点：
1. 分机号是否存在
2. ha1b 是否正确计算
3. realm 是否匹配
4. 密码是否正确

### Q: 注册成功但无法接听来电

**A:** 检查以下几点：
1. Contact 地址是否可路由
2. NAT 穿越是否配置（RTPEngine）
3. 防火墙是否允许 SIP 和 RTP 端口

### Q: 坐席状态显示离线

**A:** 检查以下几点：
1. Webhook 是否配置正确
2. cc-call 是否正常运行
3. Redis 连接是否正常

---

## 10. 相关代码索引

| 功能 | 文件位置 |
| --- | --- |
| SIP 注册 Webhook 处理 | `internal/transport/http/cti/kamailio_routes.go` |
| 分机状态管理 | `internal/domain/extension/status_service.go` |
