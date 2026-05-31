# ☁️ 云枢 (Yunshu) - 高性能分布式智能客服与呼叫中心系统

[![Go Version](https://img.shields.io/github/go-mod/go-version/tangyu-dch/yunshu)](https://golang.org)
[![Build Status](https://img.shields.io/badge/go--test-passed-brightgreen.svg)](https://golang.org)
[![Frontend Safety](https://img.shields.io/badge/typescript-typesafe-blue.svg)](https://www.typescriptlang.org)
[![Design](https://img.shields.io/badge/design-premium--neon-blueviolet.svg)](https://github.com/tangyu-dch/yunshu)

**“云枢”** 是专为新一代企业级高并发、强交互通信场景设计的**分布式智能客服与呼叫中心系统**。系统完全基于 Go 语言原生高性能并发架构重构，深度融合底层 CTI 话务并发调度引擎、FreeSWITCH ESL 信令控制核心、大模型智能流式语音 IVR 可视化编排设计工坊，在单机或云原生集群环境下提供极具弹性、高可靠、秒级结算的智能客服中枢。

---

## 🏗️ 1. 全景组件架构与物理边界

云枢系统在微服务设计上遵循高内聚、低耦合的领域驱动设计（DDD）规范，支持在云原生集群中针对热点模块单独分布式水平扩展，也可在开发与单机测试时通过合并进程（`cc-all`）一键拉起全套组件：

```text
                                  ┌────────────────┐
                                  │  外呼与话务请求  │
                                  └───────┬────────┘
                                          │
                                          ▼
                                  ┌────────────────┐
                                  │    cc-edge     │ (边缘网关鉴权/限流/逆向代理)
                                  └───────┬────────┘
                                          │
                  ┌───────────────────────┼───────────────────────┐
                  ▼                       ▼                       ▼
          ┌───────────────┐       ┌───────────────┐       ┌───────────────┐
          │  cc-console   │       │    cc-call    │       │   cc-worker   │
          │ (管理后台及API)│       │ (话务与ESL控制)│       │(计费/录音/推送)│
          └───────────────┘       └───────┬───────┘       └───────────────┘
                                          │
                                          ▼
                           ┌─────────────────────────────┐
                           │   FreeSWITCH 媒体网关集群    │
                           └─────────────────────────────┘
```

### 🛰️ 物理边界职责界定
*   **`cc-edge` (边缘通信网关)**：对外统一暴露的鉴权与物理拦截卡口。校验第三方商户 `X-App-Key` 和 `X-App-Secret` 凭证，实施高灵敏度令牌桶限流与逆向代理路由，防止非法越权与高并发穿透。
*   **`cc-console` (管理后台)**：商户自助控制台与运营管理端。收口商户财务总览、分机管理、实时话务监控、通话记录（CDR）查询以及可视化 AI 流程工作坊的 CRUD、模型管理与发布。
*   **`cc-call` (通信运行时)**：话务实时控制大脑。并发连接 FreeSWITCH 网关集群，实时消费底层 ESL 信道信令并驱动双腿（坐席腿、客户腿）的并发起呼与桥接，内置高度动态的 **AIVoiceEngine（智能语音 IVR 寻路引擎）**。
*   **`cc-worker` (异步任务中心)**：基于 Reliable Outbox（可靠本地出件箱）与租约竞争（ClaimDue）模式，提供最终一致性保障的离线重试处理：包含话单持久化结算、录音转储 CDN、商户余额精确抵扣以及话单下游三方 Webhook 可靠推送（支持 HMAC-SHA256 签名）。

---

## 🔍 2. 技术栈选型与系统设计哲学

云枢在选型与设计上追求“极致性能”与“高级视觉体验”的有机结合：

### 🛠️ 后端架构选型
*   **高性能 Go 核心**：利用 Go 原生并发 Goroutine 调度模型与低 GC 开销，在单机并发控制上万路呼叫信道，满足电信级超低延时（<20ms）控制要求。
*   **GORM + MySQL 关系型持久化**：采用 GORM 作为 ORM 映射层，封装底层的财务交易隔离事务与 Reliable Outbox 出件箱，保障商户资金与计费明细的绝对一致。
*   **Redis 原子高并发锁与会话同步**：利用 Redis 的单线程原子性实现分机高并发抢线选号、ACD 技能组资源分配，并提供 `extension:status` 多实例分机在线状态同步。

### 🎨 前端科技美学
*   **React + Vite 高性能构建**：基于 Vite 极速构建热更新，采用 React 声明式 UI 进行高频状态机的视图投射。
*   **React Flow + Canvas 霓虹特效**：深度定制 React Flow。网格背景采用精心调和的 HSL 暗色科技色调，连接线以 SVG 贝塞尔曲线呈现，且在**高亮传导时发射绿色发光电荷粒子沿着线路循环流动**，展现完美的话务轨迹。
*   **毛玻璃拟态 UI (Glassmorphism)**：运用现代 CSS `backdrop-filter: blur` 与精心搭配的渐变投影，构建极具科技感与立体感的控制卡片，wow 用户的每一次交互。

---

## 🌟 3. 产品核心特色与硬核能力

### 🧠 全局配置化 AI 厂商与模型中心
*   **配置与编排完全解耦**：控制台提供两个平行独立的功能页面，确保业务逻辑分离：
    - **🤖 AI 流程编排**：管理已设计好的智能 IVR 拓扑流，快速跳转进可视化画布进行节点物理编排。
    - **🧠 AI 厂商与模型**：集中创建和管理不同大模型厂商的 API 凭证（如 DeepSeek API、OpenAI 接口或云枢自研大模型），妥善存入 `cc_biz_ai_model_config` 物理表中。
*   **零代码快捷反填**：在 AI 画布的“开始”节点中，商户只需下拉快捷选择配置好的 AI 模型，系统会自动利用 Form 受控模式反填大模型服务商、Endpoint、密钥、Temperature 以及全局 System Prompt 并增量保存入连线图 Metadata，避免金钥在流图中四处散落的维护隐患。

### 🎙️ mod_audio_stream 旁路实时推流与 Go 原生 PCM VAD 语音网关
*   **实时音频推流**：兼容 FreeSWITCH `mod_audio_stream` 实时流媒体协议。在话务流转到 ASR 节点时，自动通过 ESL 下发 `uuid_audio_stream` 指令，在 `16k` 高清和 `mono` 单声道下，将信道中的原始音频（RTP PCM）通过 WebSocket 旁路近乎零延迟投递给大模型服务。
*   **原生 ASR WebSocket 网关**：内置高性能 WebSocket 服务器（默认监听 `9002` 端口），物理接收 FreeSWITCH 投递过来的 PCM 原始二进制音频帧，物理进行音量能量 RMS 计算。
*   **静音检测 VAD 算法**：网关运行高敏 VAD（Voice Activity Detection）算法，灵敏判断用户说话开始（VAD 开启，支持打断）与结束（持续静音 1.0 秒自动断句），无需依赖第三方臃肿组件。

### 🔌 自驱动仿真测试沙盒 (Self-Driving Sandbox)
*   **全自动自驾驶寻路**：检测到用户说话结束后，网关能依据该通话在 Session 缓存中的 AI 可视化流图节点，**自动解析其后置的所有出度 Handle 条件，自动生成符合分支意图的 transcribed ASR 文本**，自动向系统派发 `asr_speech_detected` 领域事件，实现话术流图的完美“自驾游式模拟跑通”。
*   **真人声学仿真拨号盘 (DTMF Simulation)**：物理点击利用 Web AudioContext 合成真实电话双音多频按键音效，测试 DTMF 物理按键分支路由。
*   **TTS 发音回音壁**：仿真播报时，自动调用浏览器自带的语音合成引擎（SpeechSynthesis）大声朗读 TTS 文本，并展现极具视觉震撼的绿色高动态声波图跳动。

### 🎛️ 电信级异步租约计费与 Webhook 话单推送
*   **ClaimDue 多实例竞争租约**：为了防止多实例 Worker 裸扫 Outbox 话单表导致重复计费或消息乱序，cc-worker 在投递前必须先通过 ClaimDue 机制在 MySQL 中竞争领取租约（锁定 `locked_by` 与 `locked_until`），从而在崩溃发生时允许其它 Worker 自动重领并进行数据修复。
*   **高并发计费与 CDN 录音转储**：计费默认费率完全来自配置，支持 rated 估算审计，最终结算必须拆为余额精确扣减与 settlement ledger 写入独立节点。缺少录音路径自动标记为可修复，录音上传确认后标记 `uploaded`，失败进入 outbox 自动重试。
-   **Webhook 可靠交付**：配置 `DOWNSTREAM_CDR_URL` 后，话单下游推送必须有任务状态和确认语义，支持 HMAC-SHA256 签名校验，失败时写入 last_error 并以指数退避重试，确保话单 100% 不丢。

---

## 📂 4. 项目物理目录结构

```text
├── cmd/                        # 物理服务独立进程启动入口
│   ├── cc-call/                # 实时 CTI ESL 通信服务入口
│   ├── cc-console/             # 运营管理及商户后台服务入口
│   ├── cc-worker/              # 异步分布式计费与流处理器入口
│   ├── cc-edge/                # 边缘通信鉴权网关入口
│   ├── cc-all/                 # All-in-One 一键合并启动入口
│   └── update-agents/          # 系统契约自动生成工具
├── internal/
│   ├── app/                    # 微服务进程依赖组装中心与共享 Gin 引擎
│   ├── domain/                 # 核心纯净业务领域层 (不依赖任何外部 GORM/Redis)
│   │   ├── callflow/           # AIVoiceEngine IVR 话务寻路及 CDR 流程编排
│   │   ├── cti/                # 智能高并发调度、Redis 原子选号、规则过滤链
│   │   ├── esl/                # FreeSWITCH ESL 信令构建、通话 Lifecycle 状态机
│   │   └── operate/            # AI 流程拓扑、商户计费、大模型厂商配置领域实体
│   ├── transport/              # 传输适配层 (Gin HTTP Handlers、Redis Stream 事件消费)
│   ├── contracts/              # 契约层 (定义共享事件、统一业务错误码、Redis KEY 规范)
│   └── infra/                  # 基础设施层 (GORM 模型及仓储、Redis/PubSub 适配器、Outbox 物理队列)
├── pkg/                        # 系统级共享独立轮子 (ID 幂等锁、流程引擎组件、状态机)
├── web/                        # 前端 React + Vite 可视化编辑器工作坊
└── docs/                       # 系统架构设计、双写迁移决策及三方接入指南
```

---

## 🛠️ 5. 本地开发极速运行

### 1. 物理环境依赖
*   **Go**：`>= 1.21`
*   **NodeJS**：`>= 18`
*   **MySQL**：`>= 5.7`
*   **Redis**：`>= 6.0` (负责分机注册状态同步及原子高并发选号占位)

### 2. 启动前端工作区
```bash
cd web
npm install
npm run dev
```

### 3. 一键启动后端全套微服务 (All-in-One 模式)
云枢提供了一个专门的 `cc-all` 合并进程入口，支持在单个控制台下一键拉起 `cc-edge`、`cc-console` 、`cc-call`、`cc-worker` 四个微服务进程，免去繁琐的多终端维护：
```bash
# 复制默认配置并填写您的 MySQL / Redis 物理地址
cp configs/default.yaml configs/local.yaml

# 一键并发拉起四个微服务，共享 local 配置文件
go run ./cmd/cc-all -config configs/local.yaml
```
