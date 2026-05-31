# 云枢 (Yunshu) - 高性能分布式智能呼叫中心系统

[![Go Version](https://img.shields.io/github/go-mod/go-version/tangyu-dch/yunshu)](https://golang.org)
[![Build Status](https://img.shields.io/badge/go--test-passed-brightgreen.svg)](https://golang.org)
[![Frontend Safety](https://img.shields.io/badge/typescript-typesafe-blue.svg)](https://www.typescriptlang.org)

**“云枢”** 是新一代智能呼叫中心系统（Yunshu CallCenter）的 Go 语言原生高性能重写项目。在完美继承原有系统外部通信契约（API Outbound, Batch Outbound）的同时，云枢引入了领域驱动设计（DDD）思想，重构了底层 CTI 话务调度、FreeSWITCH ESL 事件驱动流与大模型智能语音 IVR 编排工作坊，能够满足高并发话务下的弹性伸缩、精准计费、可靠录音投递以及可视化大屏流控交互。

---

## 🚀 核心组件服务

云枢系统采用高凝聚力、低耦合的服务边界设计，既支持多实例微服务集群部署，亦支持单进程多 worker 合并运行（`cc-all`），完美适配本地开发与生产级吞吐：

*   **`cc-edge`**：高性能边缘通信网关，收口并分发外部 OpenAPI 调用，为外部系统提供标准的安全鉴权与高并发话务准入防护。
*   **`cc-console`**（云枢控制台）：提供商户自助管理控制台与后台运营支撑系统，收口商户充值流水、费率模板配置、分机在线监控及 AI 可视化话术流配置。
*   **`cc-call`**（云枢通信运行时）：系统的核心话务灵魂。集成分布式 CTI 话务逻辑与 FreeSWITCH ESL 物理控制机，内置自研的 **AIVoiceEngine（可视化智能 IVR 寻路引擎）**。
*   **`cc-worker`**：异步分布式任务集群。以 Outbox 可靠消息投递为基石，负责计费流结算、录音文件转储补偿、话单（CDR）下游推送以及批量呼叫任务智能调度。

---

## ✨ 核心特色亮点

### 1. 🤖 大模型可视化混编 IVR 画布 (`AI-model-flow`)
基于 React + Antd 深度定制开发的点状定位网格流程设计器，赋予了云枢顶级的科技质感与毛玻璃霓虹视觉特效：
*   **零硬编码动态寻路**：彻底抛弃了传统 IVR 冗长生硬的代码逻辑。流程图中所有的节点（`start` 开始、`reply` 播报、`intent` 意图分支、`dtmf` 按键、`transfer` 转人工、`end` 挂断）均支持零硬编码的可视化连线与参数配置。
*   **智能 VAD 与 ASR 意图流转**：当客户说话经 ASR 实时断句上报后，运行时引擎自动解析流图中的出度有向连线，实时匹配 `SourceHandle` 意图关键字，自动跳转并回写会话活跃卡片 ID。
*   **基于 Redis 感知的 ACD 智能排队分流 (`transfer` 节点)**：真正接入 `AGENTS.md` 规范。当通话流转到转人工节点时，引擎将实时读取 Redis hash `extension:status` 获取坐席真实是在线空闲，还是离线/正忙。若是空闲则流向 `🟢 has_agent`，否则秒级引导至 `🔴 no_agent` 的商户大理线兜底分支（如播放留言或自动排队挂断），坚固防线，Fail-Closed 逻辑绝不崩溃！
*   **TTS 播报字数自适应模拟**：根据播报中文文本的字数，智能估算播放时长，结束自动流转，音控反馈极致逼真。

### 2. 🎛️ 真声仿真沙盒与科技交互体验
*   **抓手平移与视口无损缩放 (Zoom & Pan)**：支持全局按住 `Space` 空格键抓手拖拽大画布，或使用鼠标中键任意平移；支持 `40% ~ 200%` 范围无损视口缩放，满足超大型多分支话术图的顺滑编排。
*   **一键智能拓扑排版算法 (Auto Layout)**：内置基于广度优先搜索（BFS）层级的排版算法，一键理线，卡片自适应居中对齐，告别混乱连线。
*   **真人发声仿真测试沙盒**：抽屉集成了物理按键拨号盘（声音合成发生器仿真双音频 DTMF 音效）、ASR 语音输入框模拟器、坐席是在线忙线一键开关、以及 **SpeechSynthesis 拟真 TTS 朗读发音回音壁**（伴有极客绿色声波图跳动），实现画布的无实体硬件秒级跑通与调试。

### 3. 🎙️ mod_audio_stream 旁路实时推流
*   完美对接开源模块 `amigniter/mod_audio_stream` 实时音频流交互协议。开始节点一旦开启 ASR 推流，即可向 FreeSWITCH 媒体网关下发 `uuid_audio_stream` WebSocket 旁路推流物理命令（支持获取 `16k` 采样与 `mono` 单声道消音），将客户 PCM 语音近乎零延迟投递给大模型服务进行实时解析。

---

## 📂 项目物理目录结构

```text
├── cmd/                        # 各微服务独立极简进程入口
│   ├── cc-call/                # 实时 CTI ESL 通信运行时启动入口
│   ├── cc-console/             # 运营管理及商户后台启动入口
│   └── update-agents/          # 契约自动生成工具
├── internal/
│   ├── app/                    # 服务物理依赖装配中心及共享 Gin 引擎
│   ├── domain/                 # 核心纯净业务领域层 (不依赖 GORM/Redis)
│   │   ├── callflow/           # AIVoiceEngine IVR、流程消费者与 CDR 编排
│   │   ├── cti/                # 智能并发调度、Redis 原子选号、规则过滤链
│   │   ├── esl/                # FreeSWITCH ESL 指令构造、通话 Lifecycle 状态机
│   │   └── operate/            # AI 话术流、商户费率结构
│   ├── transport/              # 传输适配层 (Gin HTTP Handler, MQ/Stream 监听)
│   ├── contracts/              # 系统迁移与多实例共享契约，含事件、错误码、Redis 键声明
│   └── infra/                  # 基础设施层 (GORM 实体, Redis 读写器, outbox 队列)
├── pkg/                        # 全局共享独立轮子 (ID 幂等, 流程引擎, 状态机组件)
├── web/                        # 前端 React + Vite 可视化编辑器工作坊
└── docs/                       # 系统架构、数据表流转决策及设计手册
```

---

## 🛠️ 本地开发运行指南

### 1. 环境依赖准备
*   **Go**：`>= 1.21`
*   **NodeJS**：`>= 18`
*   **MySQL**：`>= 5.7`（云枢彻底剥离了 SQLite 内存库，生产和测试环境均采用 MySQL，配置好 DSN 即可连接）
*   **Redis**：`>= 6.0`（用于存储分机状态 `extension:status` 及分布式原子选号并发占位）

### 2. 启动前端工作坊
```bash
cd web
npm install
npm run dev
```

### 3. 启动后端通信服务
```bash
# 启动实时 CTI Realtime Call 服务
go run ./cmd/cc-call -addr :8085
```

---

## 🔬 测试与代码合规规范

我们坚持最高品质的工程实践。修改代码后，必须在 handoff 移交前确保全量静态分析与单元测试全绿通过：

```bash
# 1. 规范代码格式化
gofmt -w .

# 2. 静态代码合规扫描
go vet ./...

# 3. 运行全项目单元测试
go test ./...

# 4. 前端 TypeScript 编译类型检查
cd web
npx tsc --noEmit
```

*项目中文名称必须严格且统一叫 **“云枢”**（禁止使用错别同音字）。*
