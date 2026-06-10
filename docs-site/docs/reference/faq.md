---
title: FAQ
order: 2
---

# FAQ

## 为什么云枢声讯 SIPp 在本地失败？

如果出现：

```text
Unable to send UDP message: No route to host
```

通常是 macOS Docker Desktop/OrbStack 的 UDP 回包路径问题。建议在 Linux 服务器或 Docker 网络中运行 SIPp UAS。

## 为什么坐席腿 UNALLOCATED_NUMBER？

通常是 Kamailio `usrloc use_domain=1` 时 R-URI 域不匹配。坐席腿应使用：

```text
1001@sip.merchant.yunshu.com;fs_path=sip:<kamailio-ip>:5060
```

## 为什么 API 外呼返回 400？

检查：

- 是否请求 `cc-call:8082`
- 是否携带 `callId`
- userId 是否存在并绑定分机

## 所有呼叫都会有通话记录吗？

只要呼叫进入云枢声讯会话并收到 `CHANNEL_HANGUP_COMPLETE`，都会写入 CDR outbox，后续由 worker 落库。
