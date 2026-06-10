---
title: CTI API
order: 1
---

# CTI API

## API 外呼

```http
POST /cti/callTask/call?callId=<call-id>
Content-Type: application/json

{
  "userId": 2094,
  "callee": "13800001111"
}
```

成功：

```json
{"code":0,"message":"成功"}
```

## 批量任务调度

```http
POST /cti/batch-call-task/dispatch
Content-Type: application/json

{"taskId": 1}
```

## WebSocket

```http
GET /cti/ws?merchantId=1001&taskId=1
```
