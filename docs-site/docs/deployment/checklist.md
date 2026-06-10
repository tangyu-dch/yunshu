---
title: 上线检查清单
order: 8
---

# 上线检查清单

## 基础服务

- [ ] MySQL 可用并完成迁移
- [ ] Redis 可用并开启持久化
- [ ] FreeSWITCH ESL 可连接
- [ ] Kamailio 可接收 REGISTER/INVITE
- [ ] RTPEngine 可用
- [ ] cc-call 健康检查通过
- [ ] cc-worker outbox 循环启动

## SIP

- [ ] 分机可 REGISTER 成功
- [ ] `cc_res_location` 有有效 contact
- [ ] Kamailio dispatcher 指向 FreeSWITCH external profile
- [ ] FreeSWITCH public dialplan 已 reload
- [ ] 坐席腿能通过 `X-Internal-Call` 路由到 location

## 话务

- [ ] 呼入 SIPp 通过
- [ ] API 外呼 HTTP 200
- [ ] 云枢声讯直呼能进入 `api_direct`
- [ ] 无坐席呼入进入队列
- [ ] ACW 后能拉取队列
- [ ] 挂断后产生 CDR

## 数据

- [ ] `call_cdr_record` 有记录
- [ ] `cc_biz_ledger` 有计费流水
- [ ] outbox 无长期失败积压

## 安全

- [ ] 修改默认密码
- [ ] 配置 AppKey/AppSecret
- [ ] 配置防火墙/白名单
- [ ] 配置 HTTPS 和反向代理
