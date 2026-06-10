---
title: SIPp 验证
order: 1
---

# SIPp 验证

云枢声讯 提供 SIPp 端到端脚本：

```bash
bash scripts/sipp/run_e2e_tests.sh inbound
bash scripts/sipp/run_e2e_tests.sh dialpad
bash scripts/sipp/run_e2e_tests.sh api
```

## UAS 模式

```bash
SIPP_UAS_MODE=host   bash scripts/sipp/run_e2e_tests.sh inbound
SIPP_UAS_MODE=docker bash scripts/sipp/run_e2e_tests.sh dialpad
```

| 模式 | 说明 |
| --- | --- |
| host | 使用宿主机 SIPp |
| docker | 在 Docker 网络中运行 SIPp UAS |
| auto | 优先 Docker，失败回退 host |

## 当前建议

- 呼入可在本地验证通过。
- 云枢声讯/客户 UAS 如果出现 UDP 路由问题，建议使用 Linux 服务器或 Docker UAS。

## 常见输出

```text
PASS: 呼入 - 客户侧完整信令
API 响应: HTTP 200
```
