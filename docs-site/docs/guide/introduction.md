---
title: 项目介绍
order: 1
---

# 项目介绍

云枢声讯是一套高性能分布式智能客服与呼叫中心系统，目标是把传统呼叫中心中高度耦合的模块拆解为清晰、可测试、可扩展的 Go 原生服务。

## 核心目标

1. **稳定的话务控制**：基于 FreeSWITCH ESL 进行 originate、bridge、hangup、playback、录音、音频流等控制。
2. **可扩展的 CTI 编排**：将 API 外呼、批量外呼、云枢声讯直呼、客户呼入等流程统一纳入事件工作流。
3. **可靠的业务收口**：通过 Reliable Outbox 实现 CDR、计费、录音、报表、Webhook 的最终一致。
4. **多租户隔离**：通过商户、技能组、号码池、分机、网关、Redis key 维度实现资源隔离。
5. **面向 AI 的语音能力**：支持 ASR/TTS/LLM/RAG 与可视化 IVR 编排。

## 系统服务

| 服务 | 默认端口 | 职责 |
| --- | --- | --- |
| cc-console | 8080 | 运营/商户管理后台、云枢声讯 配套接口 |
| cc-edge | 8081 | 边缘网关、鉴权、限流 |
| cc-call | 8082 | CTI、ESL、呼叫流程、WebSocket、Kamailio webhook |
| cc-worker | 8083 | outbox、CDR、计费、录音、回调、投影 |

开发环境可使用 `cc-all` 单进程同时拉起四个服务。

## 话务底座

| 组件 | 职责 |
| --- | --- |
| Kamailio | SIP 注册、鉴权、location、dispatcher、WebSocket SIP |
| RTPEngine | RTP 媒体代理、NAT 穿越 |
| FreeSWITCH | 媒体服务器、B2BUA、ESL 控制面 |
| Redis | 分机状态、号码并发、队列、WebSocket 投影 |
| MySQL | 商户、分机、号码池、CDR、outbox、计费、配置 |

## 官方云枢声讯

云枢声讯配套云枢声讯为 [云枢声讯](https://github.com/tangyu-dch/云枢声讯.git)。

它通过控制台 API 获取 SIP 凭证，通过 Kamailio 注册，并使用 SIP INVITE 触发云枢声讯直呼或接听呼入。
