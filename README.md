# ☁️ 云枢 (Yunshu) - 高性能分布式智能客服与呼叫中心系统

[![Go Version](https://img.shields.io/github/go-mod/go-version/tangyu-dch/yunshu)](https://golang.org)
[![Build Status](https://img.shields.io/badge/go--test-passed-brightgreen.svg)](https://golang.org)
[![Frontend Safety](https://img.shields.io/badge/typescript-typesafe-blue.svg)](https://www.typescriptlang.org)
[![Design](https://img.shields.io/badge/design-premium--neon-blueviolet.svg)](https://github.com/tangyu-dch/yunshu)

**“云枢”** 是面向企业级高并发、强交互场景设计的**分布式智能客服与呼叫中心系统**。系统基于 Go 语言高并发原生性能重构，提供边缘网关、通信运行时控制、异步账务结算、以及大模型流式 IVR 可视化编排设计工坊，是支撑企业语音智能化转型的新一代通信数字中枢。

---

## 🏗️ 核心组件与物理边界

云枢系统在微服务设计上遵循高内聚、低耦合的领域驱动设计（DDD）规范，支持单独分布式扩展部署，也可在开发与单机测试时通过合并进程（`cc-all`）一键拉起全套组件：

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

*   **`cc-edge` (边缘通信网关)**：对外统一暴露的鉴权与流量拦截卡口。校验第三方商户 `X-App-Key` 和 `X-App-Secret` 凭证，实施流量控制与逆向路由，防止越权操作与高并发攻击。
*   **`cc-console` (管理后台)**：商户自助控制台与运营管理端。收口商户财务总览、分机管理、实时状态回查、通话记录（CDR）查询以及可视化 AI 流程工作坊的 CRUD 与发布。
*   **`cc-call` (通信运行时)**：系统的实时控制大脑。并发连接 FreeSWITCH 集群，实时消费 ESL 信道事件并驱动复杂的双腿（坐席、客户）起呼与桥接，内置高度动态的 **AIVoiceEngine（智能语音 IVR 寻路引擎）**。
*   **`cc-worker` (异步任务中心)**：基于 Reliable Outbox（可靠本地出件箱）模式，提供最终一致性保障的离线重试处理：包含秒级话单持久化结算、录音文件转储 CDN、商户余额精确抵扣以及话单下游三方 Webhook 可靠推送（支持 HMAC-SHA256 签名）。

---

## 🌟 产品特色与核心能力

### 🧠 1. 全局配置化 AI 厂商与模型中心
*   **配置与编排完全解耦**：控制台提供两个独立、平行的管理页面，确保业务分离与职责清晰：
    - **🤖 AI 流程编排**：展示当前商户设计的所有智能语音 IVR 卡片拓扑流，快速跳转进可视化设计工坊。
    - **🧠 AI 厂商与模型**：集中创建和管理不同厂商的 API 配置（如 DeepSeek API、OpenAI 接口或云枢私有大模型）。保存包括 Endpoint、ApiKey、Temperature 以及全局 System Prompt。
*   **编排零代码快捷绑定**：在 AI 可视化话术设计器中，商户只需在“开始节点”一键下拉选择配置好的模型，即可自动填充所有参数属性。实现“一次配置，全局生效”，大幅降低话术图内金钥散落的维护风险。

### 🎨 2. 可视化暗色霓虹 IVR 画布 (React + SVG)
*   **零代码动态寻路**：流程图支持 `start` (开始)、`reply` (TTS播报)、`intent` (ASR意图分支)、`dtmf` (按键)、`transfer` (转人工)、`end` (挂断) 等卡片的拖拽连线。系统基于拓扑有向边动态解析流转，彻底取代了传统呼叫中心死板硬编码的 XML/Dialplan。
*   **发光贝塞尔曲线与电荷流动**：精美的科技暗色风格网格，连线以优雅的 SVG 贝塞尔曲线呈现。在执行高亮传导时，**带有绿色发光电荷粒子沿着线路方向流动的微动画**，让话务流转流动轨迹一目了然。
*   **一键智能拓扑排版 (Auto Layout)**：内置基于广度优先搜索 (BFS) 的经典分层对齐算法，一键理线，自动调整卡片逻辑间距并垂直居中分布，避免交叉与折线缠绕。

### 🎙️ 3. mod_audio_stream 旁路实时推流与 Go 原生 PCM VAD 语音网关
*   **实时双向音频推流**：完全兼容 FreeSWITCH `mod_audio_stream` 实时流媒体协议。进入 ASR 节点后向媒体通道发送 ESL 音频推流物理指令，支持在 `16k` 高清和 `mono` 单声道下，将通话流旁路近乎零延迟投递给大模型。
*   **原生 ASR WebSocket 网关**：内置高性能 WebSocket 服务器（默认监听 `9002` 端口），物理接收 FreeSWITCH 投递过来的 PCM 原始二进制音频帧，物理进行音量能量 RMS 计算。
*   **能量检测 VAD 算法**：灵敏判断用户说话开始（VAD 开启，支持打断）与结束（持续静音断句），自研 VAD 能量检测无需依赖第三方臃肿组件。

### 🔌 4. 自驱动仿真测试沙盒 (Self-Driving Sandbox)
*   **全自动仿真测试**：检测到用户说话结束后，网关能依据该通话在 Session 缓存中的 AI 可视化流图节点，**自动解析其后置的所有出度 Handle 条件，自动生成符合分支意图的 transcribed ASR 文本**，自动向系统派发 `asr_speech_detected` 领域事件，实现话术流图的完美“自驾游式模拟跑通”。
*   **高保真数字拨号盘 (DTMF Simulation)**：物理点击利用 Web AudioContext 合成真实电话双音多频按键音效，测试 DTMF 按键路由。
*   **TTS 发音回音壁**：仿真播报时，自动调用浏览器自带的语音合成引擎（SpeechSynthesis）大声朗读 TTS 文本，并展现极具视觉震撼的绿色高动态声波图跳动。

### 🎛️ 5. 电信级高并发占位与可靠 Outbox 计费
*   **Redis 原子高并发选号**：呼叫起呼在高并发下必须逐个尝试经过规则链的候选号码，利用 Redis 原子锁进行并发计数、网关健康度及黑名单过滤，避免物理信道抢占冲突。
*   **可靠计费与下游推送**：计费与推送拆成独立分布式 workflow 节点，优先写入计费流水与 Outbox 本地事件表。通过 ClaimDue 租约领取机制由多实例 Worker 竞争处理，保障 billing 抵扣、录音 CDN 上传和 downstream CDR 推送在宕机/异常状态下绝对不丢、失败可重试。

---

## 📂 项目物理目录结构

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

## 🛠️ 本地开发极速运行

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

---

*系统对外及内部日志、注释、说明文案中必须统一且标准使用中文名称 **“云枢”**。*
