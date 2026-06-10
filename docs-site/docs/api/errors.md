---
title: 错误码
order: 5
---

# 错误码

云枢声讯 HTTP API 返回统一结构：

```json
{
  "code": 0,
  "message": "成功",
  "data": {}
}
```

## 常见错误

| HTTP | message | 原因 |
| --- | --- | --- |
| 400 | 请求参数错误 | JSON 格式错误、缺少 callId、userId/callee 无效 |
| 401 | 鉴权失败 | AppKey/AppSecret 错误 |
| 403 | 越权操作 | userId 不属于当前商户 |
| 404 | 通话会话不存在 | callId 未找到 |
| 409 | FS 事件状态迁移失败 | 重复/乱序/非法状态事件 |
| 500 | 内部错误 | ESL、DB、Redis 或外部依赖异常 |

## API 外呼常见 400

- URL 未携带 `callId`
- `userId` 不存在
- `callee` 为空
- 用户未绑定有效分机
