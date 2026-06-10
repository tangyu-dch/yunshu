---
title: Kamailio
order: 4
---

# Kamailio 部署

Kamailio 是 云枢声讯的 SIP 边界网关，负责分机注册、SIP 鉴权、location 存储、dispatcher 到 FreeSWITCH，以及内部坐席寻址。

## 1. 职责边界

Kamailio 负责：

- 接收 云枢声讯 REGISTER。
- 通过 `auth_db` 校验 `cc_res_extension` 中的 ha1/ha1b。
- 将注册位置写入 `cc_res_location`。
- 将外部 INVITE dispatcher 到 FreeSWITCH。
- 将 FreeSWITCH 发起的内部坐席腿路由到 location。
- 通过 RTPEngine 管理 SDP/RTP。

Kamailio 不负责：

- 坐席分配
- 业务状态机
- CDR/计费
- 呼入队列

## 2. 生产拓扑

```text
云枢声讯 / SIP Trunk
        │
        ▼
Kamailio:5060/5066
        │ dispatcher
        ▼
FreeSWITCH external:5080
```

## 3. 端口

| 端口 | 协议 | 用途 |
| --- | --- | --- |
| 5060 | UDP/TCP | SIP REGISTER/INVITE |
| 5066 | TCP/WSS | WebSocket SIP / WebRTC |
| 2223/22222 | UDP | RTPEngine 控制端口 |

## 4. 必备模块

```cfg
loadmodule "tm.so"
loadmodule "sl.so"
loadmodule "rr.so"
loadmodule "usrloc.so"
loadmodule "registrar.so"
loadmodule "auth.so"
loadmodule "auth_db.so"
loadmodule "dispatcher.so"
loadmodule "rtpengine.so"
loadmodule "http_client.so"
loadmodule "db_mysql.so"
```

## 5. 数据库配置

```cfg
#!define DBURL "mysql://root:db123456@mysql:3306/yunshu"
```

生产环境建议使用独立账号：

```text
mysql://kamailio:<password>@mysql-vip:3306/yunshu
```

## 6. 鉴权配置

云枢声讯使用 `cc_res_extension` 作为分机鉴权表。

```cfg
modparam("auth_db", "db_url", DBURL)
modparam("auth_db", "calculate_ha1", 0)
modparam("auth_db", "password_column", "ha1")
modparam("auth_db", "password_column_2", "ha1b")
modparam("auth_db", "use_domain", 1)
modparam("auth_db", "user_column", "extension_number")
modparam("auth_db", "domain_column", "sip_domain")
```

## 7. 注册位置 usrloc

```cfg
modparam("usrloc", "db_url", DBURL)
modparam("usrloc", "db_mode", 1)
modparam("usrloc", "use_domain", 1)
modparam("registrar", "use_path", 1)
```

数据库表：

```text
cc_res_location
```

注意：`methods` 字段应允许 NULL：

```sql
ALTER TABLE cc_res_location MODIFY methods INT DEFAULT NULL;
```

否则 Kamailio usrloc 写回可能报：

```text
Column 'methods' cannot be null
```

## 8. REGISTER 流程

```text
云枢声讯 REGISTER
  → auth_db(cc_res_extension)
  → save(cc_res_location)
  → HTTP webhook /cti/kamailio/auth
  → Redis extension:status = IDLE
```

测试：

```bash
sipp -sf scripts/sipp/register_auth_uac.xml \
  -s 1001 \
  -i <local-ip> \
  -p 6060 \
  <kamailio-ip>:5060
```

## 9. Dispatcher 到 FreeSWITCH

Kamailio dispatcher 表：

```text
cc_res_freeswitch
```

关键字段：

| 字段 | 说明 |
| --- | --- |
| set_id | FS 分组 |
| destination | SIP 目标 |
| attrs | 权重/并发属性 |
| enable | 是否启用 |

本地 Docker 示例：

```sql
UPDATE cc_res_freeswitch
SET destination = 'sip:192.168.107.6:5080'
WHERE id = 1;
```

生产示例：

```text
sip:10.0.10.20:5080
sip:10.0.10.21:5080
```

:::warning{title=注意}
Dispatcher 应指向 FreeSWITCH external profile。不要指向 internal profile，否则可能被 FreeSWITCH 认证挑战或进入错误 dialplan。
:::

## 10. 内部坐席寻址

当 FreeSWITCH 起呼坐席腿时，Kamailio 需要把 INVITE 路由到 location，而不是重新 dispatcher 回 FreeSWITCH。

云枢声讯会在坐席腿 originate 中写入：

```text
X-Internal-Call: true
```

Kamailio 配置中：

```cfg
if (is_present_hf("X-Internal-Call")) {
    route(LOCATION);
} else {
    route(DISPATCH);
}
```

坐席腿 R-URI 应类似：

```text
sip:1001@sip.merchant.yunshu.com
```

而实际下一跳通过 `fs_path`：

```text
1001@sip.merchant.yunshu.com;fs_path=sip:kamailio-inner-vip:5060
```

## 11. RTPEngine 配置

```cfg
modparam("rtpengine", "db_url", DBURL)
modparam("rtpengine", "table_name", "cc_res_rtpengine")
modparam("rtpengine", "rtpengine_sock", "udp:rtpengine:2223")
```

在 INVITE 和响应中处理 SDP：

```cfg
if (has_body("application/sdp")) {
    rtpengine_manage("trust-address replace-origin replace-session-connection");
}
```

## 12. 配置修改提示

### 12.1 云服务器 NAT

如果 Kamailio 监听内网 IP，但对外提供公网 SIP：

```cfg
listen=udp:10.0.1.10:5060 advertise 1.2.3.4:5060
```

### 12.2 多域租户

必须启用：

```cfg
modparam("auth_db", "use_domain", 1)
modparam("usrloc", "use_domain", 1)
```

分机 `1001` 的注册域必须和 `cc_res_extension.sip_domain` 一致。

### 12.3 dispatcher 更新后不生效

重启 Kamailio 或执行 dispatcher reload：

```bash
kamcmd dispatcher.reload
```

如果容器没有 RPC socket，可直接：

```bash
docker restart cc-kamailio
```

## 13. 验证命令

### 13.1 查看日志

```bash
docker logs -f cc-kamailio
```

### 13.2 查看注册表

```sql
SELECT username, domain, contact, received, expires
FROM cc_res_location
WHERE username='1001';
```

### 13.3 验证 SIP REGISTER

```bash
sipp -sf scripts/sipp/register_auth_uac.xml -s 1001 -p 6060 <kamailio-ip>:5060
```

### 13.4 验证 dispatcher

```bash
bash scripts/sipp/verify_sip.sh
```

## 14. 常见故障

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| REGISTER 401 后失败 | ha1/ha1b 或域不匹配 | 检查 `cc_res_extension` |
| location 表有记录但呼叫找不到坐席 | usrloc 内存未注册或 use_domain 不匹配 | 让 SIPp/Phone 真实 REGISTER |
| 坐席腿 404 | R-URI 域不对 | 使用 `1001@sip.merchant.yunshu.com` |
| 外呼被 FS 407 | dispatcher 指向 FS internal profile | 改为 FS external 5080 |
| RTP 单通 | SDP 地址错误或 RTPEngine 未接管 | 检查 RTPEngine 和 advertise |
