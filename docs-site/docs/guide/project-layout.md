---
title: 项目结构
order: 3
---

# 项目结构

```text
cmd/
  cc-all              # 开发/单机一体化启动入口
  cc-call             # 呼叫控制服务
  cc-console          # 控制台服务
  cc-edge             # 边缘网关
  cc-worker           # 异步任务服务
configs/              # 配置文件
docker/               # Docker/FreeSWITCH/MySQL 初始化资源
docs/                 # 原始 Markdown 文档
docs-site/            # dumi 文档站
internal/
  app/                # 服务组装和运行时
  contracts/          # DTO、事件、错误码、契约
  domain/
    callflow/         # 呼叫流程编排、AI 语音引擎
    cti/              # 选号、批量调度、CTI 工作流
    esl/              # ESL 命令、会话、状态机
    operate/          # 运营/商户/AI 配置领域
  infra/
    business/         # CDR、计费、录音、报表、outbox 存储
    fsesl/            # FreeSWITCH ESL 连接、命令构建、事件适配
    resource/         # 分机、DID、坐席分配、会话嗅探
    selection/        # Redis 选号、运行时标记、队列
    telephony/        # FreeSWITCH/Kamailio 节点仓储
  transport/
    http/             # HTTP API 路由
pkg/                  # 通用工具、工作流、状态机
scripts/sipp/         # SIPp 端到端验证脚本
web/                  # React 前端
```

## 依赖方向

推荐依赖方向：

```text
cmd → internal/app → transport → domain → contracts
                 ↘ infra ↗
```

原则：

- `domain` 不直接依赖 Gin/GORM/Redis 等具体实现。
- `infra` 负责连接外部系统。
- `contracts` 保存跨层共享 DTO、事件和错误码。
- 多步骤业务应优先抽象为工作流或事件消费者。
