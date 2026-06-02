# ☁️ 云枢 (Yunshu) - 高性能分布式智能客服与呼叫中心系统

[English Version](README.md) | **中文说明**

[![Go Version](https://img.shields.io/github/go-mod/go-version/tangyu-dch/yunshu)](https://golang.org)
[![Build Status](https://img.shields.io/badge/go--test-passed-brightgreen.svg)](https://golang.org)
[![Frontend Safety](https://img.shields.io/badge/typescript-typesafe-blue.svg)](https://www.typescriptlang.org)
[![Design](https://img.shields.io/badge/design-premium--neon-blueviolet.svg)](https://github.com/tangyu-dch/yunshu)

**“云枢”** 是专为新一代企业级高并发、强交互通信场景设计的**分布式智能客服与呼叫中心系统**。系统完全基于 Go 语言原生高性能并发架构重构，深度融合底层 CTI 话务并发调度引擎、FreeSWITCH ESL 信令控制核心、多厂商物理大模型流式语音 IVR 可视化编排设计工坊，在单机或云原生集群环境下提供极具弹性、高可靠、秒级结算的智能客服中枢。

---

## 📞 官方配套桌面端：Yunshu-Phone (云枢软电话)

**云枢 (Yunshu)** 拥有专门的官方配套桌面软电话客户端：**[Yunshu-Phone](https://github.com/tangyu-dch/yunshu-phone.git)**。
基于 **Go + Wails v2 + React 18** 构建，为电销坐席提供原生编译的高性能工作台。**请注意，Yunshu-Phone 是云枢后端唯一支持和兼容的专属桌面客户端。**

---

## 🏗️ 1. 全景组件架构与物理边界

云枢系统在微服务设计上遵循高内聚、低耦合 of 领域驱动设计（DDD）规范，支持在云原生集群中针对热点模块单独分布式水平扩展，也可在开发与单机测试时通过合并进程（`cc-all`）一键拉起全套组件：

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
以下是智能可视化 AI 画布以及全局大模型厂商配置面板의 预览：

| 🤖 智能可视化 AI 流程画布 | 🧠 全局大模型厂商配置中心 |
| :---: | :---: |
| ![智能可视化 AI 流程画布](docs/images/visual_flow_designer.png) | ![全局大模型厂商配置中心](docs/images/ai_model_config.png) |
| *支持霓虹科技电荷流动的拖拽式可视化 IVR 编排工作坊* | *完全解耦的模型参数配置与凭证密钥管理器* |

| ⚙️ 开始节点 - 模型快捷反填展示 |
| :---: |
| ![开始节点 - 模型快捷反填展示](docs/images/quick_config_fill.png) |
| *Start 节点声明式 Schema 动态表单与快捷反填卡片联动* |

#### 🏢 系统运营平台全景画廊 (运营端 - `/operate`)
以下是云枢电信级系统运营平台的核心管理面板预览：

| 📊 系统总览看板 | 🔌 软交换节点池与状态租约 |
| :---: | :---: |
| ![系统总览看板](docs/images/operate_dashboard.png) | ![软交换节点池与状态租约](docs/images/operate_freeswitch.png) |
| *全局高并发实时话务指标与系统吞吐量总览面板* | *物理 FreeSWITCH 实例心跳监测与动态租约注册表* |

| 🎛️ 主被叫 SIP 网关管理 | 🏢 商户余额与计费授权中心 |
| :---: | :---: |
| ![主被叫 SIP 网关管理](docs/images/operate_gateway.png) | ![商户余额与计费授权中心](docs/images/operate_merchant.png) |
| *电信级中继 SIP 网关注册与物理并发信道限额管理* | *商户主体准入、账户计费流水与余额扣减生命周期管理* |

| ☎️ SIP 分机配置中心 | ⚙️ 动态选号规则与频次风控 |
| :---: | :---: |
| ![SIP 分机配置中心](docs/images/operate_extension.png) | ![动态选号规则与频次风控](docs/images/operate_risk_control.png) |
| *SaaS 分机配置、SIP 密码分发与在线/忙碌注册表实时自省* | *高并发外呼号码源原子级分配、频次盲区过滤与黑白名单机制* |

#### 💼 商户控制台全景画廊 (商户端 - `/merchant`)
以下是商户端进行批量话务导入、在线 WebRTC 呼叫与通话审计的核心工作台预览：

| 🚀 批量外呼自动化任务调度 | 📞 WebRTC 嵌入式软电话拨号盘 |
| :---: | :---: |
| ![批量外呼自动化任务调度](docs/images/merchant_batch_task.png) | ![WebRTC 嵌入式软电话拨号盘](docs/images/merchant_webrtc_dialpad.png) |
| *批量客户号码文件安全导入、名单清洗与并发自动呼叫引擎* | *内嵌 SIP 协议栈的实时音视频网页电话软终端工作台* |

| 🎙️ 通话记录与录音回放审计 | 👥 技能组坐席与话务分配 |
| :---: | :---: |
| ![通话记录与录音回放审计](docs/images/merchant_call_record.png) | ![👥 技能组坐席与话务分配](docs/images/merchant_skill_group.png) |
| *带可视化音频波形播放器的高并发 CDR 审计与话单推送控制台* | *商户坐席队列绑定、话务转接与负载均衡分配策略中心* |

### 🎙️ mod_audio_stream 旁路实时推流与 Go 原生 PCM VAD 语音网关
*   **实时音频推流**：兼容 FreeSWITCH `mod_audio_stream` 实时流媒体协议。在话务流转到 ASR 节点时，自动通过 ESL 下发 `uuid_audio_stream` 指令，在 `16k` 高清和 `mono` 单声道下，将信道中的原始音频（RTP PCM）通过 WebSocket 旁路近乎零延迟投递给大模型服务。
*   **原生 ASR WebSocket 网关**：内置高性能 WebSocket 服务器（默认监听 `9002` 端口），物理接收 FreeSWITCH 投递过来的 PCM 原始二进制音频帧，物理进行音量能量 RMS 计算。
*   **静音检测 VAD 算法**：网关运行 high- VAD（Voice Activity Detection）算法，灵敏判断用户说话开始（VAD 开启，支持打断）与结束（持续静音 1.0 秒自动断句），无需依赖第三方臃肿组件。

### 🚫 物理严格调用，杜绝仿真 mock 降级
*   **严格 Fail-Closed 设计**：云枢废弃了任何以仿真模拟（MOCK）或演示性退让为代目的平滑降级机制。当商户未在 Start 节点中配置物理 `llmApiKey`，或者物理 ASR、TTS、LLM 请求接口失败时，系统将坚决拒绝仿真兜底退让，严格按 Fail-Closed 原则立刻向话务状态机返回物理凭证未配置或物理调用失败的严格错误，确保生产计费、话务与 AI 路由的安全和严谨性。
*   **高并发计费与 CDN 录音转储**：计费默认费率完全来自配置，支持 rated 估算审计，最终结算必须拆为余额精确扣减与 settlement ledger 写入独立节点。缺少录音路径自动标记为可修复，录音上传确认后标记 `uploaded`，失败进入 outbox 自动重试。
*   **Webhook 可靠交付**：配置 `DOWNSTREAM_CDR_URL` 后，话单下游推送必须有任务状态和确认语义，支持 HMAC-SHA256 签名校验，失败时写入 last_error 并以指数退避重试，确保话单 100% 不丢。

---

## 🚀 4. 生产环境部署规划与配置文件指南

为了确保电信级的话务高可用、超低延迟的信令交互以及顺畅的音频文件流转，在生产部署云枢系统时，必须严格参考以下生产部署规划与配置文件（`production.yaml`）优化指南。

### 📡 4.1 生产环境部署规划 (Topology & Clustering)

在分布式生产环境下，云枢各组件的物理边界和网络拓扑应按如下规划进行集群化部署：

```text
       DMZ公网区 [防火墙过滤]              VPC内网安全区 [超低延迟千兆/万兆直连]
    ┌──────────────────────┐        ┌────────────────────────────────────────────────────────┐
    │       cc-edge        ├───────►│    cc-console 集群 (2+ 节点，负载均衡)                   │
    │ (公网鉴权/限流/逆向代理) │       │   ┌──────────────────────────────────────────────┐     │
    └──────────────────────┘        │   │                                              │     │
                                    │   ▼                                              ▼     │
                                    │ ┌───────────────┐                          ┌─────────┐ │
                                    │ │  MySQL 集群   │                          │  Redis  │ │
                                    │ │ (读写分离/主备)│                          │ 哨兵集群 │ │
                                    │ └───────────────┘                          └────┬────┘ │
                                    │                                                 ▲      │
                                    │   ┌─────────────────────────────────────────────┘      │
                                    │   ▼                                                    │
                                    │ ┌───────────────┐   ESL 长连接 (<1ms)   ┌────────────┐ │
                                    │ │    cc-call    ├─────────────────────►│ FreeSWITCH │ │
                                    │ │ (2+ 节点集群)  │                      │ 媒体网关集群│ │
                                    │ └───────┬───────┘                      └─────┬──────┘ │
                                    │         │                                    │         │
                                    │         └──────────┐              ┌──────────┘         │
                                    │                    │挂载共享卷     │                    │
                                    │                    ▼              ▼                    │
                                    │               ┌────────────────────────┐               │
                                    │               │   NFS / NAS 共享存储    │               │
                                    │               │ (TTS 缓存与本地录音共享) │               │
                                    │               └────────────────────────┘               │
                                    │                            ▲                           │
                                    │                            │挂载共享卷                  │
                                    │                            │                           │
                                    │                      ┌─────┴──────┐                    │
                                    │                      │ cc-worker  │                    │
                                    │                      │ (2+ 异步节) │                    │
                                    │                      └────────────┘                    │
                                    └────────────────────────────────────────────────────────┘
```

#### 1. 核心时延控制 (网络 VPC 规划)
- **要求**：`cc-call`（信令控制）与 FreeSWITCH 媒体网关之间的网络延迟必须控制在 **< 1ms**。
- **规划**：必须将 `cc-call` 与 FreeSWITCH 部署在**同一个 VPC 子网**内。若高频呼叫并发量极大，建议将 `cc-call` 进程作为 Daemon 守护进程与 FreeSWITCH **同机部署**，利用本地环回网络进行信令控制，彻底避免由于网络抖动引发的起呼延迟或桥接失败。

#### 2. 水平扩展与事件租约高可用 (Clustering & HA)
- **`cc-call` 无状态集群**：允许水平部署 2+ 个 `cc-call` 实例。为了防范多实例重复消费同一个 FreeSWITCH 节点的 ESL 事件，`cc-call` 在注册 FS 事件监听前必须先通过 Redis 注册表动态领取事件租约，并在存活期间持续续约。当实例断开或停机时自动释放，由备用实例抢占接管。
- **`cc-worker` 分布式任务队列**：允许水平部署 2+ 个 `cc-worker` 实例。多个 worker 基于可靠本地出件箱（Reliable Outbox）与 Redis 抢占锁，通过 `ClaimDue` 机制安全领取到期的待处理任务（计费流水结算、CDN录音转储、Webhook下游推送等），即使单台 worker 宕机也绝无漏单和重复推送。
- **Redis 哨兵与持久化**：由于运行时分机状态（`extension:status`）、防透支原子余额扣减以及网关和号码的双重原子并发限制计数均存放在 Redis 极热路径上，生产环境 Redis 必须配置为**主从+哨兵（Sentinel）集群**，并开启 AOF 持久化，保障秒级热备无损切换。

#### 3. TTS 缓存与通话录音文件共享 (共享网络存储)
- **TTS 共享**：`cc-call` 将大模型应答文本合成的 MP3 语音音频写入本地目录，FreeSWITCH 需要直接通过绝对路径执行播放。
- **录音共享**：FreeSWITCH 物理录制的通话 MP3/WAV 录音保存在本地目录，`cc-worker` 需要读取并转储至阿里云 OSS / 腾讯云 COS。
- **部署要求**：必须在生产环境搭建**低延迟 NFS / NAS 共享文件存储**，将共享卷分别挂载到 FreeSWITCH、`cc-call` 以及 `cc-worker` 所在主机的**完全一致的绝对物理路径**下（如 `/var/lib/yunshu/shared`），确保读写权限（r/w）对三方进程完全开放。

---

### ⚙️ 4.2 配置文件修改指南 (production.yaml)

在生产环境部署时，需将 `configs/default.yaml` 复制为 `configs/production.yaml`，并对以下高频关键参数进行物理修改和生产级优化：

```yaml
# =====================================================================
# 1. 关系型数据库与连接池优化 (MySQL)
# =====================================================================
database:
  # 必须修改：指向生产 MySQL 主库（或读写分离 VIP），配置强密码
  dsn: "yunshu_prod:SecurePass123!@tcp(mysql-vip.prod.lan:3306)/yunshu?charset=utf8mb4&parseTime=True&loc=Local"
  # 生产优化：根据预期并发量（如 5000+ 路）调整连接池参数
  max_open_conns: 100        # 最大打开连接数，保障高频事务无需排队
  max_idle_conns: 20         # 最大空闲连接数，降低连接频繁创建与销毁的开销
  conn_max_lifetime: 3600    # 单个连接的最大生存时间（秒）

# =====================================================================
# 2. 高并发原子锁与状态存储 (Redis)
# =====================================================================
redis:
  # 必须修改：指向生产高可用 Redis 哨兵或集群的 VIP 地址与端口
  addr: "redis-sentinel.prod.lan:6379"
  # 必须修改：生产 Redis 强密码
  password: "RedisStrongPassword456$"
  db: 0
  pool_size: 150             # 生产优化：调高 Redis 连接池以支撑号码/网关双重并发 Lua 原子限制判定

# =====================================================================
# 3. 边缘网关鉴权与商户准入限制 (cc-edge)
# =====================================================================
edge:
  port: 8080                 # 对外暴露的边缘网关 API 端口
  rate_limit:
    enable: true
    capacity: 200            # 令牌桶最大容量（单个商户 API 外呼起呼并发上限限制）
    rate: 50                 # 令牌桶每秒恢复速率

# =====================================================================
# 4. FreeSWITCH ESL 信令与实时语音推流控制
# =====================================================================
freeswitch:
  # 必须修改：物理 FreeSWITCH ESL 监听端口及强密码（严禁使用默认的 ClueCon）
  esl_addr: "fs-node1.prod.lan:8021"
  esl_password: "FsSuperSecurePassword789!"
  # 必须修改：旁路实时 PCM 裸流推送 WebSocket 接口地址，配置为 cc-call 网关生产 IP
  audio_stream_ws: "ws://cc-call-internal-vip.prod.lan:9002/audio"

# =====================================================================
# 5. 物理文件路径映射与网络共享卷挂载点 (TTS & Recording)
# =====================================================================
storage:
  # 必须修改：挂载低延迟网络共享文件存储 (NFS/NAS) 的统一绝对路径
  shared_root: "/var/lib/yunshu/shared"
  # 生产映射：TTS 音频合成缓存绝对路径（必须在共享挂载目录下，FS与cc-call必须完全同路）
  tts_cache_dir: "/var/lib/yunshu/shared/tts_cache"
  # 生产映射：通话录音本地输出路径（指向 FreeSWITCH 录音落盘卷，供 cc-worker 并发扫描转储）
  recordings_dir: "/var/lib/yunshu/shared/recordings"

# =====================================================================
# 6. 异步任务与 Webhook 下游话单可靠推送 (cc-worker)
# =====================================================================
worker:
  billing:
    enable: true
    # 必须警告：默认估算费率。若生产环境未配置费率，系统会打出中文告警，且只能生成 audit rated 估算，不准直接扣费
    default_rate_per_min: 0.15 
  recording:
    enable: true
    oss_bucket: "yunshu-recordings-prod"
    oss_endpoint: "oss-cn-shenzhen.aliyuncs.com"
  downstream:
    # 必须修改：下游接收商户话单 CDR 推送的物理 Webhook 地址
    webhook_url: "https://api.merchant-platform.com/callbacks/cdr"
    # 生产优化：支持 HMAC-SHA256 签名的共享私钥
    signature_secret: "MerchantSecretKeySignatureXYZ"

# =====================================================================
# 7. 电信级运行时并发调度策略 (CTI Engine)
# =====================================================================
cti:
  concurrency:
    # 必须注意：号码和网关双重并发原子限制锁持有租约过期时间（生产环境建议 30 分钟，防漏释放）
    claim_ttl_ms: 1800000 
    # 是否强制开启“网关与号码双重物理并发级联限制”判定，生产环境必须为 true
    enable_double_limit: true
```

---

### 🛠️ 4.3 物理部署流程指引 (Deployment Steps)

#### 第一步：基础环境与物理中间件准备
1. 准备一台或多台高性能 Linux 服务器（推荐 Centos 7+ / Ubuntu 20.04+）。
2. 安装并拉起中间件服务（建议 MySQL 与 Redis 均配置为 high- 可用架构）。
3. 搭建 **NFS / NAS 共享文件系统**，并将共享目录分别挂载至 FreeSWITCH 机器与 `cc-call`、`cc-worker` 机器的**相同绝对路径**上（统一映射至 `/var/lib/yunshu/shared`），执行 `chmod -R 777` 确保读写权限完全放开。

#### 第二步：FreeSWITCH 媒体网关配置
1. 安装 FreeSWITCH 核心包，修改 `autoload_configs/event_socket.conf.xml`，配置强密码并允许 `cc-call` 所在内网 IP 接入。
2. 安装并启用 `mod_audio_stream` 模块，用于旁路实时 PCM WS 音频推流。
3. 修改 FreeSWITCH 默认拨号计划（dialplan）与录音输出配置，将录音物理落盘路径指向共享卷 `/var/lib/yunshu/shared/recordings`。

#### 第三步：前端工程编译与 Nginx 分发
1. 在前端工程目录 `web/` 下，执行依赖安装与静态编译：
   ```bash
   cd web
   npm install
   npm run build
   ```
2. 将生成的 `dist/` 静态资源目录分发至 Web 服务器（如 Nginx），并配置 Nginx 代理前端路由与反向代理后端 `cc-edge` 公网接口。

#### 第四步：后端微服务编译与配置
1. 在云枢 Go 后端根目录下，编译物理微服务可执行文件：
   ```bash
   go build -o bin/cc-edge ./cmd/cc-edge
   go build -o bin/cc-console ./cmd/cc-console
   go build -o bin/cc-call ./cmd/cc-call
   go build -o bin/cc-worker ./cmd/cc-worker
   ```
2. 拷贝 `configs/default.yaml` 模板为 `configs/production.yaml`。
3. 参考 **4.2 配置文件修改指南** 修改 `configs/production.yaml` 并填入生产物理凭证与优化参数。

#### 第五步：守护进程拉起与高可用自愈
1. 强烈推荐使用 `Systemd` 编写各微服务的守护进程配置文件。以 `cc-call` 为例，创建 `/etc/systemd/system/cc-call.service`：
   ```ini
   [Unit]
   Description=Yunshu CallCenter Telephony Engine
   After=network.target

   [Service]
   Type=simple
   User=root
   WorkingDirectory=/var/yunshu
   ExecStart=/var/yunshu/bin/cc-call -config /var/yunshu/configs/production.yaml
   Restart=always
   RestartSec=5
   LimitNOFILE=65535

   [Install]
   WantedBy=multi-user.target
   ```
2. 执行 `systemctl daemon-reload`，启用并一键拉起各微服务守护进程：
   ```bash
   systemctl enable cc-call cc-worker cc-console cc-edge
   systemctl start cc-call cc-worker cc-console cc-edge
   ```

---

### 📡 4.4 电信级软交换集群优化 (Kamailio + RTPEngine + FreeSWITCH)

在承载几千路高并发的电信级分布式呼叫中心架构中，Kamailio 负责 SIP 控制信令与请求路由，RTPEngine 负责高性能内核态 RTP 媒体流转发与 NAT 穿越，而 FreeSWITCH 则退居内网隔离区，专门作为纯粹的 IVR 状态机、语音流 PCM WebSocket 旁路推送（mod_audio_stream）以及媒体转码中枢。

下面是各核心组件的生产配置修改与整合指南：

#### 1. Kamailio 负载均衡与 SIP 信令控制配置 (kamailio.cfg)
- **职责**：作为前置 SIP 代理与防火墙，负责外网 SIP 端口暴露与安全防骚扰过滤，并通过 `dispatcher` 模块轮询负载均衡给后端的 FreeSWITCH 节点池。
- **关键修改**：
  - **信令网口绑定与公网宣告 (NAT 穿越) (`kamailio.cfg`)**：
    在云服务器环境下，指定 Kamailio 绑定在私有内网 IP 上，但向外发送信令时对外宣告公网 IP，确保外网终端正常交互：
    ```kamailio
    # 绑定内网私有 IP 进行监听，同时向外宣告外部公网 IP 用于 NAT 穿越
    listen=udp:<KAMAILIO_PRIVATE_IP>:5060 advertise <KAMAILIO_PUBLIC_IP>:5060
    ```
  - **后端路由与心跳检测 (`dispatcher.list`)**：
    配置后端隐藏的 FreeSWITCH 节点池（使用内网私有 IP 地址），并通过 SIP OPTIONS 包对各节点执行秒级存活心跳探测：
    ```text
    # setid(1) 代表后端 FreeSWITCH 媒体网关池，配置为内网私网地址
    1 sip:<FREESWITCH1_PRIVATE_IP>:5060 0 0 weight=50
    1 sip:<FREESWITCH2_PRIVATE_IP>:5060 0 0 weight=50
    ```
  - **RTP 媒体劫持与 NAT 接管 (`kamailio.cfg`)**：
    在信令流中截获 SDP 媒体参数，调用 RTPEngine 进行 RTP 端口接管代理，打通外网终端与内网网关：
    ```kamailio
    # 加载 rtpengine 模块
    loadmodule "rtpengine.so"
    modparam("rtpengine", "rtpengine_sock", "udp:<RTPENGINE_PRIVATE_IP>:22222") # 指向 RTPEngine 的内网 UDP 监听地址

    # 在 route 逻辑中拦截并接管 SDP 媒体流
    route[MANAGE_MEDIA] {
        if (is_request() && has_body("application/sdp")) {
            rtpengine_manage("trust-address replace-origin replace-session-connection");
        } else if (is_reply() && has_body("application/sdp")) {
            rtpengine_manage("trust-address replace-origin replace-session-connection");
        }
    }
    ```

#### 2. RTPEngine 媒体转发优化配置 (rtpengine.conf)
- **职责**：内核级媒体包转发，打通 NAT 穿越。
- **关键修改 (`/etc/rtpengine/rtpengine.conf`)**：
  - **双网卡 IP 绑定**：绑定内网 IP 与外网 IP，供外网软电话直接连通内网媒体：
    ```ini
    interface = internal/<RTPENGINE_PRIVATE_IP>;external/<RTPENGINE_PUBLIC_IP>
    ```
  - **控制通信配置**：与 Kamailio 中 `rtpengine_sock` 对接的 UDP 协议 NG 端口：
    ```ini
    listen-ng = <RTPENGINE_PRIVATE_IP>:22222
    ```
  - **UDP 媒体端口范围**：配置充足的端口以支撑超高并发：
    ```ini
    port-min = 30000
    port-max = 40000
    ```

#### 3. FreeSWITCH 集群与安全对接配置 (内网安全区)
- **职责**：执行 CTI 话务信令与 AI IVR 录音/转码/大模型 PCM 旁路推送。
- **关键修改**：
  - **盲信任前置代理信令**：
    由于 Kamailio 已经在最前端完成了所有的鉴权与防扫描，内网 FreeSWITCH 应该配置为免去二次握手注册鉴权，直接信任来自 Kamailio 的请求：
    ```xml
    <!-- internal.xml -->
    <param name="accept-blind-reg" value="true"/>
    <param name="accept-blind-auth" value="true"/>
    <param name="apply-inbound-acl" value="kamailio-nodes"/>
    ```
  - **配置网关 ACL 加白控制列表 (`acl.conf.xml`)**：
    严禁外网任何 SIP 扫描直接触达内网，加白仅允许 Kamailio 的内网 IP 进行访问：
    ```xml
    <list name="kamailio-nodes" default="deny">
      <node type="allow" cidr="10.0.10.0/24"/>
    </list>
    ```
  - **媒体旁路 (Bypass Media) 与 IVR 音频捕获调优**：
    对于普通的坐席/分机双腿桥接通话，若无需录音或 IVR 交互，可在 Kamailio 处下发 `bypass_media`，让 RTP 媒体不经过 FreeSWITCH 而是由 RTPEngine 纯内核直接转发，以实现单机数千路的超高性能指标；但对于云枢系统调度的 **IVR 智能大模型对话流程**，由于需要进行 PCM 语音推流（mod_audio_stream）与 VAD 断句，**必须禁用旁路媒体，确保媒体流经过 FreeSWITCH 本地**，保障 AI 语音网关的完美捕获。
  - **SIP Profile 内外网隔离与公网 IP 映射 (NAT 穿越)**：
    当 FreeSWITCH 部署于云服务器的 NAT 环境下（网卡仅有内网 IP）且需要直接与外网进行信令与媒体交互时，必须正确宣告外部公网 IP，否则会导致外网终端因无法将语音包发回服务器而出现“单通”或“完全无声”问题：
    *   **外部通信配置 (`sip_profiles/external.xml`)**：专门用于对接外部的 SIP 实体（如外网坐席终端、外部互联公网网关等）：
        ```xml
        <!-- 强制将对外的 SIP 信令和 SDP 媒体 IP 替换为您的公网 IP -->
        <param name="ext-rtp-ip" value="<YOUR_PUBLIC_IP>"/>
        <param name="ext-sip-ip" value="<YOUR_PUBLIC_IP>"/>
        ```
    *   **内部通信配置 (`sip_profiles/internal.xml`)**：主要接管系统内网可信任设备（如同 VPC 的 Kamailio 节点），并在呼叫出局时进行正确的 NAT 穿越：
        ```xml
        <!-- 1. 绑定内网 IP 监听，确保与 Kamailio 在内网快速通信，避免绕行公网 -->
        <param name="rtp-ip" value="$${local_ip_v4}"/>
        <param name="sip-ip" value="$${local_ip_v4}"/>
        <!-- 2. 出局宣告或响应外部终端时，强制提供公网 IP 进行 NAT 穿越 -->
        <param name="ext-rtp-ip" value="<YOUR_PUBLIC_IP>"/>
        <param name="ext-sip-ip" value="<YOUR_PUBLIC_IP>"/>
        ```

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
*   ✅ **多厂商统一适配器**：完美接入 DeepSeek、OpenAI、腾讯混元、阿里通义千问、火山引擎豆包物理大模型。
*   ✅ **严格去仿真化（Fail-Closed 范式）**：全面移除仿真与 mock 退化兜底，保障生产账务与话务的严谨可靠。
*   ✅ **运行时就绪自省**：设计运行时功能检测接口（如 `IsAsrImplemented`），并在前端 UI 中自动置灰禁用未实现厂商。
*   ✅ **声明式 Schema 动态小卡片表单**：根据所选的 ASR/TTS/LLM 厂商，动态渲染其专属的 AppKey 凭证与音色下拉。

### 第二阶段：高并发选号策略与分布式事件租约 [进行中]
*   ⏳ **高并发逐个试选选号系统**：基于 Redis 规则链的并发控制，在批量呼起时对候选号码进行逐个试选直至分配成功。
*   ⏳ **FreeSWITCH ESL 节点租约机制**：多实例 `cc-call` 部署时通过 ClaimDue 机制竞争领取 FS 节点事件监听租约，防范重复消费。
*   ⏳ **振铃音与早期媒体编排**：支持在呼叫桥接前对 CHANNEL_PROGRESS_MEDIA 进行实时流程编排与回播音控制。

### 第三阶段：账务高安全与独立结算工作流解耦 [规划中]
*   ✅ **CDR 信道可靠 Outbox**：基于 MySQL 本地出件箱持久化暂存 CDR 话单挂断基础事实。
*   ⏳ **独立计费与费率审计**：将话费结算与 CDR 持久化解耦，由独立 MQ 并发计费微服务进行费率模板抵扣计算。
*   ⏳ **原子余额防超扣**：利用 Redis Lua 脚本执行商户扣款与防透支锁定，并发布最终计费单据结转账本。

### 第四阶段：高可靠异步 Worker 结转与下游推送 [进行中]
*   ✅ **ClaimDue 分布式 Worker 核心**：多机 Worker 利用 ClaimDue 机制竞争扫描 Outbox，防止重复投递。
*   ✅ **可靠下游 Webhook CDR 推送**：支持商户接收挂断/接通事件，内置指数避退重试与 HMAC-SHA256 签名。
*   ⏳ **异步录音文件 CDN 转储**：后台 Worker 读取 FreeSWITCH 本地物理录音，自动同步至 OSS 并标记话单为已转储。

### 第五阶段：动态权限完整落库与控制台安全 [进行中]
*   ✅ **GORM 权限模型与数据库种子**：创建控制台路由、操作码模型实体并完成静态权限的种子注入。
*   ⏳ **动态运行时权限拦截中间件**：支持在 `cc-console` 中从数据库实时加载并验证商户操作员的菜单与 API 操作权限。

---

## ⚖️ 8. 免责声明与技术承诺 (Apology & Disclaimer)

### 👤 个人开发者身份声明 (Individual Developer Status)
云枢（Yunshu）项目是一个**由独立个人开发者自研、自主维护的开源重构项目，不属于任何公司、企业主体、电信运营商或商业机构**。本系统所有的架构重构、功能开发、文档指南及社区 Bug 修复，均为开发者基于个人技术热情的无偿开源奉献。项目不具备任何企业机构背景，因此无法提供公司级别的商业合同签署、发票开具或长线企业外派服务，请在评估和使用时明确知晓该个人开发属性。

### 🙇‍♂️ 开发者服务承诺与 SLA
由于本项目正处于从旧系统向 Go 原生高性能架构 of 重写与快速迁移阶段，目前部分电信级高级信令控制、复杂 ACD 坐席动态分配算法及第三方增值话务网关的对接功能仍处于积极完善与迭代补强中。对于现阶段系统未完备性给商户测试及本地集成带来的不便，我们深表歉意！

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
