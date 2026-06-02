# ☁️ 云枢 (Yunshu) - 高性能分布式智能客服与呼叫中心系统

[English Version](README.md) | **中文说明**

[![Go Version](https://img.shields.io/github/go-mod/go-version/tangyu-dch/yunshu)](https://golang.org)
[![Build Status](https://img.shields.io/badge/go--test-passed-brightgreen.svg)](https://golang.org)
[![Frontend Safety](https://img.shields.io/badge/typescript-typesafe-blue.svg)](https://www.typescriptlang.org)
[![Design](https://img.shields.io/badge/design-premium--neon-blueviolet.svg)](https://github.com/tangyu-dch/yunshu)

**“云枢”** 是专为新一代企业级高并发、强交互通信场景设计的**分布式智能客服与呼叫中心系统**。系统完全基于 Go 语言原生高性能并发架构重构，深度融合底层 CTI 话务并发调度引擎、FreeSWITCH ESL 信令控制核心、多厂商物理大模型流式语音 IVR 可视化编排设计工坊，在单机或云原生集群环境下提供极具弹性、高可靠、秒级结算的智能客服中枢。

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
    - **🧠 AI 厂商与模型**：集中创建和管理不同大模型厂商的 API 凭证（如 DeepSeek API、OpenAI 接口、腾讯混元大模型等），妥善存入 `cc_biz_ai_model_config` 物理表中。
*   **零代码快捷反填**：在 AI 画布的“开始”节点中，商户只需下拉快捷选择配置好的 AI 模型，系统会自动利用 Form 受控模式反填大模型服务商、Endpoint、密钥、Temperature 以及全局 System Prompt 并增量保存入连线图 Metadata，避免金钥在流图中四处散落的维护隐患。

#### 🎛️ 可视化 AI 流程与大模型配置画廊
以下是智能可视化 AI 画布以及全局大模型厂商配置面板的预览：

| 🤖 智能可视化 AI 流程画布 | 🧠 全局大模型厂商配置中心 |
| :---: | :---: |
| ![智能可视化 AI 流程画布](docs/images/visual_flow_designer.png) | ![全局大模型厂商配置中心](docs/images/ai_model_config.png) |
| *支持霓虹科技电荷流动的拖拽式可视化 IVR 编排工作坊* | *完全解耦的模型参数配置与凭证密钥管理器* |

| ⚙️ 开始节点 - 模型快捷反填展示 |
| :---: |
| ![开始节点 - 模型快捷反填展示](docs/images/quick_config_fill.png) |
| *Start 节点声明式 Schema 动态表单与快捷反填卡片联动* |

### 🎙️ mod_audio_stream 旁路实时推流与 Go 原生 PCM VAD 语音网关
*   **实时音频推流**：兼容 FreeSWITCH `mod_audio_stream` 实时流媒体协议。在话务流转到 ASR 节点时，自动通过 ESL 下发 `uuid_audio_stream` 指令，在 `16k` 高清和 `mono` 单声道下，将信道中的原始音频（RTP PCM）通过 WebSocket 旁路近乎零延迟投递给大模型服务。
*   **原生 ASR WebSocket 网关**：内置高性能 WebSocket 服务器（默认监听 `9002` 端口），物理接收 FreeSWITCH 投递过来的 PCM 原始二进制音频帧，物理进行音量能量 RMS 计算。
*   **静音检测 VAD 算法**：网关运行高敏 VAD（Voice Activity Detection）算法，灵敏判断用户说话开始（VAD 开启，支持打断）与结束（持续静音 1.0 秒自动断句），无需依赖第三方臃肿组件。

### 🚫 物理严格调用，杜绝仿真 mock 降级
*   **严格 Fail-Closed 设计**：云枢废弃了任何以仿真模拟（MOCK）或演示性退让为代目的平滑降级机制。当商户未在 Start 节点中配置物理 `llmApiKey`，或者物理 ASR、TTS、LLM 请求接口失败时，系统将坚决拒绝仿真兜底退让，严格按 Fail-Closed 原则立刻向话务状态机返回物理凭证未配置或物理调用失败的严格错误，确保生产计费、话务与 AI 路由的安全和严谨性。
*   **高并发计费与 CDN 录音转储**：计费默认费率完全来自配置，支持 rated 估算审计，最终结算必须拆为余额精确扣减与 settlement ledger 写入独立节点。缺少录音路径自动标记为可修复，录音上传确认后标记 `uploaded`，失败进入 outbox 自动重试。
*   **Webhook 可靠交付**：配置 `DOWNSTREAM_CDR_URL` 后，话单下游推送必须有任务状态和确认语义，支持 HMAC-SHA256 签名校验，失败时写入 last_error 并以指数退避重试，确保话单 100% 不丢。

---

## 🚀 4. 部署建议与物理部署流程

为了确保电信级的话务稳定、超低延迟的交互响应以及顺畅的媒体文件流转，在部署云枢系统时，必须严格参考以下物理部署建议与架构拓扑。

### 📡 4.1 哪些组件需要与 FreeSWITCH 部署在同一台服务器上？

在云枢架构中，**`cc-call` 微服务、`cc-worker` 微服务在生产环境中强烈建议与 FreeSWITCH 媒体网关部署在同一台物理服务器/虚拟机上，或者通过极低延迟的局域网（VPC）加共享高性能网络文件存储（如 NFS、GlusterFS）进行网络拓扑关联**。

具体决策 rationale 如下：

#### 1. TTS 语音合成本地缓存共享 (必须同机或挂载共享卷)
当 `cc-call` 话务运行时接收到大模型生成的文本应答时，会通过已配置的 TTS 提供商（如阿里、腾讯、OpenAI、火山豆包）发起物理合成，并将合成的 MP3/WAV 音频文件写入本地磁盘的 TTS 缓存目录（例如 `/var/lib/yunshu/tts_cache`）。
FreeSWITCH 随后通过 ESL 的 `playback /var/lib/yunshu/tts_cache/xxxx.mp3` 命令来播放此音频。
*   **部署要求**：**FreeSWITCH 所在的服务器必须能够直接通过绝对物理路径读取到这些合成文件**。因此，要么将 `cc-call` 与 FreeSWITCH 部署在同一服务器上，共享相同的本地磁盘；要么通过 NFS 等网络文件系统将共享卷挂载到双方服务器的**相同绝对路径**下。

#### 2. 通话录音文件转储 (`cc-worker` 建议同机部署)
通话接通并开启录音后，FreeSWITCH 的物理媒体层（如 `mod_sndfile` 或 `mod_shout`）会将双向音频直接录制写入 FreeSWITCH 主机的本地磁盘（例如 `/var/log/freeswitch/recordings`）。
呼叫挂断后，`cc-worker` 作为异步任务中心，需要读取这些本地音频文件并上传至商户配置的 OSS / COS 等云端 CDN 存储中。
*   **部署要求**：**`cc-worker` 必须具备对 FreeSWITCH 录音输出目录的直接读写权限**。在多机分布式部署时，必须使用高性能 NFS/NAS 共享存储进行挂载关联，或者直接在 FreeSWITCH 机器上作为 Daemon 进程部署 `cc-worker`。

#### 3. CTI 信令与 ESL 连接时延 (强时延控制)
`cc-call` 作为实时话务控制大脑，需要长连接 FreeSWITCH 的 ESL（Event Socket Library）接口进行高频的信令双向交互。在电信级呼叫场景下，网络延迟如果大于 5ms 极易引发竞态冲突、桥接失败或者按键（DTMF）响应迟缓。
*   **部署要求**：`cc-call` 与 FreeSWITCH 之间的网络延迟必须控制在 **<1ms**。强烈建议将两者部署在同一台物理机中，或通过 VPC 内网千兆/万兆光纤直连。

#### 4. 旁路实时推流延时 (`mod_audio_stream` 极速传输)
FreeSWITCH 在识别到 VAD 或 ASR 开启后，需要将实时的通话音频（16k mono 16bit PCM 裸流）通过 WebSocket 协议推送给云枢的 ASR 语音网关（默认监听 `9002` 端口）。高频实时推流对网络带宽、抖动和丢包极其敏感。
*   **部署要求**：ASR 语音网关（集成在 `cc-call` 或独立部署）必须与 FreeSWITCH 处于同机或同局域网 VPC，防止因公网传输网络丢包导致大模型断句失效。

---

### 🛠️ 4.2 部署流程指引 (Deployment Workflow)

#### 第一步：基础环境与物理中间件准备
1. 准备一台或多台高性能 Linux 服务器（推荐 Centos 7+ / Ubuntu 20.04+）。
2. 安装并拉起中间件服务：
   - **MySQL (>= 5.7)**：配置并导入云枢建表 Schema。
   - **Redis (>= 6.0)**：分机在线状态与原子高并发并发控制的单一事实源。
3. 如果采用分布式部署，需搭建 **NFS / NAS 共享文件系统**，并将共享目录分别挂载至 FreeSWITCH 机器与 `cc-call`, `cc-worker` 机器的相同绝对路径上（例如统一映射到 `/var/yunshu/shared`）。

#### 第二步：FreeSWITCH 媒体网关配置
1. 安装 FreeSWITCH 核心包，启用 `mod_event_socket`（配置 `event_socket.conf.xml`，允许云枢 `cc-call` 所在内网 IP 接入，并配置强密码）。
2. 安装并启用 `mod_audio_stream` 模块，用于旁路实时 WS 推流。
3. 创建录音存放目录，确保 FreeSWITCH 进程对该目录具有读写（r/w）权限。

#### 第三步：前端工程编译与分发
1. 在前端工程目录 `web/` 下，执行依赖安装与静态编译：
   ```bash
   cd web
   npm install
   npm run build
   ```
2. 将生成的 `dist/` 静态资源目录分发至 Web 服务器（如 Nginx），并配置 HTML5 历史路由代理。

#### 第四步：后端微服务编译与配置
1. 在云枢 Go 后端根目录下，编译物理微服务可执行文件：
   ```bash
   go build -o bin/cc-edge ./cmd/cc-edge
   go build -o bin/cc-console ./cmd/cc-console
   go build -o bin/cc-call ./cmd/cc-call
   go build -o bin/cc-worker ./cmd/cc-worker
   ```
2. 拷贝 `configs/default.yaml` 模板为 `configs/production.yaml`。
3. 编辑 `configs/production.yaml`，填入生产物理凭证：
   - 填写 MySQL、Redis 连接的物理 DSN 与地址。
   - 填写 FreeSWITCH ESL 接入端口、强密码以及 `mod_audio_stream` 实时 WebSocket 地址。
   - 确保 `cc-call` 配置的 `tts_cache` 目录与 FreeSWITCH 的挂载点完全一致。
   - 确保 `cc-worker` 配置的 `recordings` 目录指向 FreeSWITCH 的实际录音落盘目录。

#### 第五步：拉起与守护进程配置
1. 在各目标服务器上拉起对应微服务：
   - **管理机**：启动 `cc-console`。
   - **外呼/通信机 (与 FreeSWITCH 同机或内网)**：启动 `cc-call`。
   - **异步处理机 (与 FreeSWITCH 同机或挂载共享卷)**：启动 `cc-worker`。
   - **前置代理机**：启动 `cc-edge` 进行商户接入控制。
2. 强烈推荐使用 `Systemd` 或 `Supervisor` 编写守护进程脚本，确保微服务在异常退出时自动拉起，实现电信级无缝守候。

---

## 📂 5. 项目物理目录结构

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

## 🛠️ 6. 本地开发极速运行

### 1. 物理环境依赖
*   **Go**：`>= 1.21`
*   **NodeJS**：`>= 18`
*   **MySQL**：`>= 5.7`
*   **Redis**：`>= 6.0`

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

---

## 🗺️ 7. 开发路线图 (Development Roadmap)

云枢正积极地将历史遗留的后端话务模块全面迁移并重构为该高并发、高性能的 Go 架构。以下清单详细列示了我们当前活跃的开发里程碑、已完工特性以及规划中特性：

### 第一阶段：商用级 AI 厂商解耦与严谨安全 [100% 已完工]
*   [x] **多厂商统一适配器**：完美接入 DeepSeek、OpenAI、腾讯混元、阿里通义千问、火山引擎豆包物理大模型。
*   [x] **严格去仿真化（Fail-Closed 范式）**：全面移除仿真与 mock 退化兜底，保障生产账务与话务的严谨可靠。
*   [x] **运行时就绪自省**：设计运行时功能检测接口（如 `IsAsrImplemented`），并在前端 UI 中自动置灰禁用未实现厂商。
*   [x] **声明式 Schema 动态小卡片表单**：根据所选的 ASR/TTS/LLM 厂商，动态渲染其专属的 AppKey 凭证与音色下拉。

### 第二阶段：高并发选号策略与分布式事件租约 [进行中]
*   [/] **高并发逐个试选选号系统**：基于 Redis 规则链的并发控制，在批量呼起时对候选号码进行逐个试选直至分配成功。
*   [ ] **FreeSWITCH ESL 节点租约机制**：多实例 `cc-call` 部署时通过 ClaimDue 机制竞争领取 FS 节点事件监听租约，防范重复消费。
*   [ ] **振铃音与早期媒体编排**：支持在呼叫桥接前对 CHANNEL_PROGRESS_MEDIA 进行实时流程编排与回播音控制。

### 第三阶段：账务高安全与独立结算工作流解耦 [规划中]
*   [x] **CDR 信道可靠 Outbox**：基于 MySQL 本地出件箱持久化暂存 CDR 话单挂断基础事实。
*   [ ] **独立计费与费率审计**：将话费结算与 CDR 持久化解耦，由独立 MQ 并发计费微服务进行费率模板抵扣计算。
*   [ ] **原子余额防超扣**：利用 Redis Lua 脚本执行商户扣款与防透支锁定，并发布最终计费单据结转账本。

### 第四阶段：高可靠异步 Worker 结转与下游推送 [进行中]
*   [x] **ClaimDue 分布式 Worker 核心**：多机 Worker 利用 ClaimDue 机制竞争扫描 Outbox，防止重复投递。
*   [x] **可靠下游 Webhook CDR 推送**：支持商户接收挂断/接通事件，内置指数避退重试与 HMAC-SHA256 签名。
*   [/] **异步录音文件 CDN 转储**：后台 Worker 读取 FreeSWITCH 本地物理录音，自动同步至 OSS 并标记话单为已转储。

### 第五阶段：动态权限完整落库与控制台安全 [进行中]
*   [x] **GORM 权限模型与数据库种子**：创建控制台路由、操作码模型实体并完成静态权限的种子注入。
*   [/] **动态运行时权限拦截中间件**：支持在 `cc-console` 中从数据库实时加载并验证商户操作员的菜单与 API 操作权限。

---

## ⚖️ 8. 免责声明与技术承诺 (Apology & Disclaimer)

### 👤 个人开发者身份声明 (Individual Developer Status)
云枢（Yunshu）项目是一个**由独立个人开发者自研、自主维护的开源重构项目，不属于任何公司、企业主体、电信运营商或商业机构**。本系统所有的架构重构、功能开发、文档指南及社区 Bug 修复，均为开发者基于个人技术热情的无偿开源奉献。项目不具备任何企业机构背景，因此无法提供公司级别的商业合同签署、发票开具或长线企业外派服务，请在评估和使用时明确知晓该个人开发属性。

### 🙇‍♂️ 开发者服务承诺与 SLA
由于本项目正处于从旧系统向 Go 原生高性能架构的重写与快速迁移阶段，目前部分电信级高级信令控制、复杂 ACD 坐席动态分配算法及第三方增值话务网关的对接功能仍处于积极完善与迭代补强中。对于现阶段系统未完备性给商户测试及本地集成带来的不便，我们深表歉意！

**🚀 问题解决时效保障（SLA & Support）**：
为了免除您的后顾之忧，项目团队正式承诺提供高标准的技术支持服务保障：
- **快速响应**：对于商户及开源社区反馈的系统 Bug、呼叫故障或功能缺失，我们将在接收到 Issue 反馈后的 **2 小时内** 物理响应，启动首轮技术排查。
- **极速修复**：对于常规程序缺陷（Bug）与环境配置问题，我们承诺在 **24 小时内** 完成物理修复、全量测试回归并向主分支交付热更新补丁；对于涉及复杂 FreeSWITCH 信令编排、高并发抢线竞态或运营商网关物理对接等深水区难题，我们将在 **48 小时内** 物理提供系统规避方案或定制化补丁，全力保障商户的话务及计费业务连续性。

### ⚠️ 免责声明
1. **合规与法律责任**：云枢（Yunshu）作为一套高并发、电信级分布式智能客服与呼叫中心话务系统，其技术设计旨在服务企业合规的高保真客户接待及 IVR 智能应答。**本系统仅供学术研究、系统演示和开发参考之用**。任何单位或个人在使用本系统进行物理起呼、外呼业务时，必须严格遵守所在国家和地区的通信法规、反电信网络诈骗法及用户个人隐私保护政策。**对于因违法外呼、恶意骚扰、信息泄露或滥用本系统所导致的一切法律、行政及民事纠纷，云枢项目开发团队不承担任何直接或间接的法律责任。**
2. **大模型（LLM）生成式声明**：云枢集成了 completions 大模型流式 IVR 编排和旁路 WebSocket 语音推送能力。大模型（如 DeepSeek、OpenAI、腾讯混元等）生成的话术文本和对话流具有概率随机性，**系统无法完全保证其回答的 100% 准确性、妥当性与安全性**。商户及使用方应在流程设计中加入完备的 ACD 转人工（transfer）、安全拦截及 fail-closed 挂断极速闭环，防范生成式 AI 的“幻觉”引发话务舆情风险。
3. **无担保承诺**：本开源软件基于 **GPL-3.0** 开源协议提供“按原样”的无担保服务，不作任何明示或暗示的保证（包括但不限于对适销性、特定用途的适用性或非侵权性的担保）。使用方需自行承担本系统运行可能带来的通信信道开销及硬件资源负载风险。

---

## 📄 9. 开源协议 (License)

本项目采用 **[GNU General Public License v3.0 (GPL-3.0)](LICENSE)** 开源许可证发布。详情请参阅根目录下的 [LICENSE](LICENSE) 文件。
