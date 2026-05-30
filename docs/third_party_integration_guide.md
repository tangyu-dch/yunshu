# 云枢呼叫中心 第三方系统接入与技术对接指南

本指南面向需要对接云枢呼叫中心系统（Go 重写版）的第三方厂商系统（如 CRM、ERP 或业务系统），介绍如何进行安全鉴权、调用主动呼叫与控制接口，以及如何配置和接收实时的通话话单与录音回调。

---

## 1. 接入准备与安全机制

为了保障系统安全，第三方系统与云枢呼叫中心的全部数据通信均需通过严格的鉴权机制进行验证。

### 1.1 获取商户对接凭证
您需要登录云枢管理后台，在**商户管理/对接配置**页面申请如下密钥对：
- **X-App-Key (应用键)**：公开的商户标识，用于标识发起 API 请求的商户身份。
- **X-App-Secret (应用密钥)**：高敏感密钥，用于与 `X-App-Key` 配合完成接口调用的鉴权。请妥善保管，切勿泄漏。
- **Webhook 推送 Secret (签名密钥)**：用于双向防伪签名。云枢在向您推送话单或事件时，会基于该 Secret 生成签名。

### 1.2 主动 API 调用鉴权
第三方系统在调用云枢提供的任何主动 HTTP API 时，**必须**在请求的 HTTP 头部（Headers）中携带以下鉴权字段：
```http
X-App-Key: your_merchant_app_key
X-App-Secret: your_merchant_app_secret
Content-Type: application/json
```
> [!WARNING]
> 未携带密钥对或密钥对无效的请求，系统将直接返回 `401 Unauthorized` 错误。

### 1.3 被动 Webhook 接收验签 (HMAC-SHA256)
当云枢呼叫中心向您配置的 `DOWNSTREAM_CDR_URL` 推送话单数据时，为了防范请求被伪造，云枢会使用您的 **Webhook 推送 Secret** 对整个 JSON 请求体（Request Body）计算 HMAC-SHA256 签名，并将签名放入 HTTP Header 中：
- 头部字段：`X-Signature-SHA256`
- 散列编码：十六进制（Hex-encoded）

**验签计算逻辑示例（Go 语言）：**
```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func verifySignature(rawBody []byte, secret string, signatureHeader string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(signatureHeader))
}
```

---

## 2. 主动 API 接口说明

核心呼叫服务（`cc-call`）对第三方系统暴露以下主动控制及查询接口。

### 2.1 触发 API 单次外呼
向指定的坐席用户发起双向呼叫。呼叫流程通常为：系统先发起外呼呼叫坐席分机，坐席分机应答后，系统发起外呼呼叫客户号码，待客户应答后将两路通话桥接。
- **请求地址**：`POST /cti/callTask/call`
- **请求头部**：携带 `X-App-Key` 和 `X-App-Secret`
- **请求参数 (JSON Body)**：
  | 字段名 | 类型 | 必须 | 说明 |
  | :--- | :--- | :--- | :--- |
  | `userId` | int | 是 | 坐席的用户 ID（必须归属于该商户） |
  | `callee` | string | 是 | 客户的被叫电话号码 |
  | `extra` | string | 否 | 第三方穿透参数（JSON 字符串或普通字符串），话单推送时原样返回 |

- **请求示例 (Curl)**：
  ```bash
  curl -X POST http://call-center-domain/cti/callTask/call \
    -H "X-App-Key: your_app_key" \
    -H "X-App-Secret: your_app_secret" \
    -H "Content-Type: application/json" \
    -d '{
      "userId": 10025,
      "callee": "13800138000",
      "extra": "{\"crm_customer_id\": \"9527\"}"
    }'
  ```

- **响应示例 (Success)**：
  ```json
  {
    "code": 200,
    "message": "成功",
    "data": null
  }
  ```

---

### 2.2 回查坐席分机实时状态
回查分机在 FreeSWITCH 节点以及系统 Redis 中的实时通话状态（如：离线、空闲、振铃、通话中等）。
- **请求地址**：`GET /cti/extension/active-state/{extension}`
- **请求头部**：携带 `X-App-Key` 和 `X-App-Secret`
- **路径参数**：
  - `{extension}`: 分机号（如 `1001`）

- **请求示例 (Curl)**：
  ```bash
  curl -X GET http://call-center-domain/cti/extension/active-state/1001 \
    -H "X-App-Key: your_app_key" \
    -H "X-App-Secret: your_app_secret"
  ```

- **响应数据说明 (JSON)**：
  | 字段名 | 类型 | 说明 |
  | :--- | :--- | :--- |
  | `extension` | string | 请求查询的分机号 |
  | `fsActive` | bool | 通信层是否有活跃通道存在 |
  | `fsAddr` | string | 所在 FreeSWITCH 节点的物理 IP 与地址 |
  | `redisStatus` | string | Redis 中缓存的全局状态码：`-1` 离线，`1` 空闲/在线，`2` 预振铃，`3` 振铃，`4` 通话中 |
  | `hasStuckState` | bool | 系统是否检测到并自动清理了卡死状态标识 |

- **响应示例**：
  ```json
  {
    "code": 200,
    "message": "成功",
    "data": {
      "extension": "1001",
      "fsActive": true,
      "fsAddr": "10.0.0.1:8021",
      "redisStatus": "4",
      "hasStuckState": false,
      "oldStatus": "4",
      "action": "sync"
    }
  }
  ```

---

### 2.3 通话控制：挂断通话
立即终止指定的呼叫或通道。
- **请求地址**：`POST /esl/control/hangup`
- **请求头部**：携带 `X-App-Key` 和 `X-App-Secret`
- **请求参数 (JSON Body)**：
  | 字段名 | 类型 | 必须 | 说明 |
  | :--- | :--- | :--- | :--- |
  | `callId` | string | 是 | 通话的全局唯一标识 CallID |
  | `uuid` | string | 否 | 通道唯一标识 UUID（不传默认挂断该呼叫的主叫及被叫全通道） |
  | `reasonCode` | string | 否 | 挂断原因值，默认 `NORMAL_CLEARING` |

- **请求示例 (Curl)**：
  ```bash
  curl -X POST http://call-center-domain/esl/control/hangup \
    -H "X-App-Key: your_app_key" \
    -H "X-App-Secret: your_app_secret" \
    -H "Content-Type: application/json" \
    -d '{
      "callId": "call_17800412345",
      "reasonCode": "USER_BUSY"
    }'
  ```

- **响应示例**：
  ```json
  {
    "code": 200,
    "message": "成功",
    "data": null
  }
  ```

---

### 2.4 通话控制：通话转接 (Transfer)
将当前正在进行的呼叫转接到其他分机或外线号码。
- **请求地址**：`POST /esl/control/transfer`
- **请求头部**：携带 `X-App-Key` 和 `X-App-Secret`
- **请求参数 (JSON Body)**：
  | 字段名 | 类型 | 必须 | 说明 |
  | :--- | :--- | :--- | :--- |
  | `callId` | string | 是 | 呼叫的 CallID |
  | `uuid` | string | 否 | 目标操作的通道 UUID |
  | `destination` | string | 是 | 转接的目的地号码（可为其他分机号如 `1002`，或外呼号码） |

---

### 2.5 通话控制：静音控制 (Audio)
控制指定通话通道的静音（Mute）或解除静音状态。
- **请求地址**：`POST /esl/control/audio`
- **请求参数 (JSON Body)**：
  | 字段名 | 类型 | 必须 | 说明 |
  | :--- | :--- | :--- | :--- |
  | `callId` | string | 是 | 呼叫的 CallID |
  | `uuid` | string | 否 | 目标操作的通道 UUID |
  | `payload` | object | 是 | 控制参数，例如 `{"mute": true}` 表示静音，`{"mute": false}` 表示解除静音 |

---

### 2.6 通话控制：发送 DTMF 按键 (DTMF)
在通话中向对端通道发送二次按键双音多频信号（例如自动语音菜单的数字按键选择）。
- **请求地址**：`POST /esl/control/dtmf`
- **请求参数 (JSON Body)**：
  | 字段名 | 类型 | 必须 | 说明 |
  | :--- | :--- | :--- | :--- |
  | `callId` | string | 是 | 呼叫的 CallID |
  | `digit` | string | 是 | 要发送的按键字符，如 `"1"` 或 `"*"`, `"#"` |

---

## 3. Webhook 话单与录音推送规范

通话结束且话单持久化后，系统将自动触发回调推送。系统通过可靠队列投递，支持指数退避重试，若您的系统返回非 `200` 状态码，系统将启动重新推送机制，以确保数据完整送达。

### 3.1 推送请求头 (Headers)
```http
Content-Type: application/json
X-Outbox-Id: outbox_entry_id
X-Idempotency-Key: cdr:downstream:call_id
X-Downstream-Target: downstream
X-Signature-SHA256: 6245f8fa2... (由推送 Secret 基于 HMAC-SHA256 计算出的 Hex 签名值)
```

### 3.2 推送参数说明 (JSON Body)
| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `jobId` | string | 推送任务 ID |
| `callId` | string | 呼叫的全局唯一标识 |
| `merchantId` | int | 归属商户 ID |
| `outboxId` | string | 数据块出件箱唯一 ID |
| `payload` | object | 话单核心负载，包含字段详见下方解释 |

**`payload` 字段详情：**
| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `callerNumber` | string | 呼出的主叫显号（客户手机端显示的来电号码） |
| `calleeNumber` | string | 被叫号码（客户电话或坐席电话） |
| `durationSec` | int | 呼叫的总物理时长（秒数，从起呼到挂断） |
| `billsec` | int | 双方接通的计费时长（秒数，双方在媒体链路接通的净通话时长） |
| `hangupCause` | string | 通话挂断的原因代码。例如 `NORMAL_CLEARING` 正常挂断，`NO_ANSWER` 无人接听，`USER_BUSY` 用户忙 |
| `finalState` | string | 通话状态机的最终状态，如 `complete` 代表正常结束 |
| `recordFilePath`| string | 录音文件的存储相对路径（如 `/record/2026/05/29/call_123.wav`）。如果配置了 `RECORDING_UPLOAD_URL`，此处可返回完整的公网 CDN 下载 URL |
| `extra` | string | API 呼叫发起时由第三方传入的自定义穿透参数（原样返还） |
| `completedAt` | string | 通话挂断的结束时间 (UTC 时间戳，例如 `"2026-05-29T08:44:22Z"`) |

### 3.3 推送 JSON 示例
```json
{
  "jobId": "push_job_88a7c64b",
  "callId": "call_17800412345_99321",
  "merchantId": 88,
  "target": "downstream",
  "outboxId": "cdr:downstream:call_17800412345_99321",
  "idempotencyKey": "cdr:downstream:call_17800412345_99321",
  "payload": {
    "callId": "call_17800412345_99321",
    "uuid": "4c8f382a-e731-9b10-fbe4-58a8871b48aa",
    "callerNumber": "02161234567",
    "calleeNumber": "13800138000",
    "durationSec": 45,
    "billsec": 32,
    "hangupCause": "NORMAL_CLEARING",
    "finalState": "complete",
    "recordFilePath": "/record/2026/05/29/call_17800412345_99321.wav",
    "extra": "{\"crm_customer_id\": \"9527\", \"operator\": \"system\"}",
    "completedAt": "2026-05-29T08:44:22Z",
    "eventType": "cdr_persisted"
  }
}
```

---

## 4. 接口通用返回信封与错误码说明

所有的主动 API 响应均使用标准的统一 JSON 结构体：
```json
{
  "code": 200,
  "message": "成功",
  "data": null
}
```

### 4.1 核心状态码字典
| Code | HTTP 状态码 | 业务状态码说明 |
| :--- | :--- | :--- |
| `200` | 200 OK | 请求成功 |
| `400` | 400 Bad Request | 请求参数格式错误或缺少必须的字段 |
| `401` | 401 Unauthorized | 对接凭证 (`X-App-Key`/`X-App-Secret`) 无效，或商户已被禁用 |
| `403` | 403 Forbidden | 水平越权。目标操作的分机或通话不属于该凭证所关联的商户 |
| `404` | 404 Not Found | 请求的目标资源（如未找到通话会话）不存在 |
| `409` | 409 Conflict | 动作冲突，例如对已挂断的通话重复发起挂断 |
| `500` | 500 Internal Error | 呼叫中心内部服务执行失败（如 ESL 断开或 FS 呼叫超时） |
