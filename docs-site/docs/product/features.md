---
title: 功能矩阵
order: 2
---

# 功能矩阵

| 模块 | 功能 | 状态 |
| --- | --- | --- |
| 坐席云枢声讯 | 云枢声讯 登录、SIP 注册、云枢声讯 | 已支持 |
| 呼入 | DID 识别、技能组分配、无坐席排队 | 已支持 |
| 呼出 | API 外呼、云枢声讯直呼、批量外呼 | 已支持 |
| 队列 | 呼入排队、预测外呼排队、ACW 后拉取 | 已支持 |
| CDR | 挂断后写入 outbox，worker 落库 | 已支持 |
| 计费 | 计费流水、结算节点、余额扣减 | 已支持 |
| 录音 | 录音任务、HTTP 上传、OSS 上传节点 | 已支持 |
| WebSocket | 批量外呼投影推送 | 已支持 |
| AI IVR | ASR/TTS/LLM/RAG 流程 | 迭代中 |
| SIPp 验证 | inbound/api/dialpad/batch | 已提供脚本 |

## 通话记录保证

只要呼叫进入云枢声讯并有最终挂断事件，必须生成 CDR：

```text
CHANNEL_HANGUP_COMPLETE → call_center_cdr_queue
```

该规则已由单元测试覆盖所有核心 profile。
