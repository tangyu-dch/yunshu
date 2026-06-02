# ☁️ Yunshu (云枢) - High-Performance Distributed Intelligent Customer Service & Call Center System

**中文说明** | [中文版](README_zh.md)

[![Go Version](https://img.shields.io/github/go-mod/go-version/tangyu-dch/yunshu)](https://golang.org)
[![Build Status](https://img.shields.io/badge/go--test-passed-brightgreen.svg)](https://golang.org)
[![Frontend Safety](https://img.shields.io/badge/typescript-typesafe-blue.svg)](https://www.typescriptlang.org)
[![Design](https://img.shields.io/badge/design-premium--neon-blueviolet.svg)](https://github.com/tangyu-dch/yunshu)

**“云枢” (Yunshu)** is a state-of-the-art **distributed intelligent customer service and call center system** designed for next-generation enterprise-grade high-concurrency, highly-interactive communication scenarios. Completely rewritten on top of Go's high-performance native concurrent runtime, it deeply integrates a bottom-layer CTI concurrent telephony routing engine, a FreeSWITCH ESL signaling control core, and a multi-provider commercial-grade LLM streaming voice IVR visual orchestration workflow. It delivers an elastic, highly reliable, and second-level billing communication core suitable for single-node deployments or cloud-native containerized clusters.

---

## 📞 Official Desktop Client: Yunshu-Phone

**Yunshu** has an exclusive, official companion desktop softphone client: **[Yunshu-Phone](https://github.com/tangyu-dch/yunshu-phone.git)**. 
Built with **Go + Wails v2 + React 18**, it provides a natively compiled, high-performance CTI workspace for telemarketing agents. **Please note that Yunshu-Phone is the ONLY supported desktop client for this backend.**

---

## 🏗️ 1. Architecture Overview & Component Boundaries

Yunshu adopts a highly cohesive, loosely coupled Domain-Driven Design (DDD) microservices layout. Hotspots can be scaled horizontally and independently in cloud-native production environments, while an All-in-One process launcher (`cc-all`) is provided to spin up all services instantly in development and local staging environments:

```text
                                  ┌────────────────┐
                                  │ Outbound/Calls │
                                  └───────┬────────┘
                                          │
                                          ▼
                                  ┌────────────────┐
                                  │    cc-edge     │ (Edge gateway: Auth / Rate Limiting / Proxy)
                                  └───────┬────────┘
                                          │
                  ┌───────────────────────┼───────────────────────┐
                  ▼                       ▼                       ▼
          ┌───────────────┐       ┌───────────────┐       ┌───────────────┐
          │  cc-console   │       │    cc-call    │       │   cc-worker   │
          │ (Admin / APIs)│       │ (Telephony/ESL│       │(Billing/Recs/ │
          └───────────────┘       │  ACD Control) │       │ Downstream)   │
                                  └───────┬───────┘       └───────────────┘
                                          │
                                          ▼
                           ┌─────────────────────────────┐
                           │   FreeSWITCH Media Gateway  │
                           └─────────────────────────────┘
```

### 🛰️ Component Responsibilities
*   **`cc-edge` (Edge Gateway)**: The unified authentication, security inspection, and rate-limiting gateway. It validates merchant `X-App-Key` and `X-App-Secret` tokens, enforces precise token-bucket rate limits, and reverse-proxies requests to prevent unauthenticated access.
*   **`cc-console` (Administration Console)**: Self-service portal for merchants and system administrators. Features financial overview charts, extension registration tracking, real-time call monitoring, Call Detail Record (CDR) auditing, and a drag-and-drop Visual AI Flow orchestration studio.
*   **`cc-call` (Telephony Runtime)**: The real-time communications brain. It maintains high-performance TCP streams to the FreeSWITCH Event Socket Library (ESL) interface, consumes raw telephony leg state machines, handles dual-leg (agent & customer) concurrent bridging, and hosts the active **AIVoiceEngine** for smart IVR navigation.
*   **`cc-worker` (Async Processor Center)**: Implements a reliable transactional Outbox pattern with lease acquisition (`ClaimDue`) to handle heavy asynchronous workloads with eventual consistency. Responsibilities include CDR persistence, call recording CDN uploads, precise billing settlement, and secure downstream call webhook notifications (with HMAC-SHA256 signatures).

---

## 🔍 2. Technology Stack & Design Philosophy

Yunshu bridges the gap between extreme execution speed and premium design aesthetics:

### 🛠️ Back-end Architecture
*   **Native Go Core**: Employs Go's lightweight Goroutines and low GC overhead to manage thousands of active media channels per instance with sub-20ms control latency.
*   **GORM + MySQL Relational Persistence**: Utilizes GORM as the database abstraction layer, encapsulating strict transaction boundaries for billing ledgers and outbox queue entries.
*   **Redis Concurrent Locks & Sync**: Relies on Redis atomic transactions and key expires to implement high-speed extension selection, skill-group queuing, and cross-instance synchronizations (via `extension:status` hashes).

### 🎨 Front-end Aesthetics
*   **React + Vite Platform**: Built using Vite for instant HMR, combined with React for declarative component views.
*   **React Flow Neon Engine**: Tailored React Flow layout utilizing dark mode HSL palettes, glassmorphism card surfaces (`backdrop-filter: blur`), and SVG bezier paths. During active call traversal, **animated glowing green charge particles pulse along SVG wires** to visualize real-time flow progression.

---

## 🌟 3. Product Features & Core Capabilities

### 🧠 Decoupled AI Configuration & Model Center
*   **Architectural Separation**: The dashboard provides two cleanly decoupled spaces:
    - **🤖 AI Flow Designer**: Visual workspace to manage and publish smart voice IVR graph topographies.
    - **🧠 AI Providers & Models**: Consolidated credential manager (`cc_biz_ai_model_config`) for cloud provider keys (DeepSeek API, OpenAI API, Tencent Hunyuan, Alibaba Qwen, Volcengine Doubao).
*   **One-Click Auto-Fill**: Inside the Visual Designer's `Start` node, selecting a configured model automatically loads and locks the endpoint, credentials, temperature, and system prompt into the canvas metadata, removing the security risk of hardcoding API keys in visual diagrams.

#### 🎛️ Visual AI Flow & Configuration Gallery
Here is a preview of the Visual AI Flow Designer and the Global AI Provider Configuration panels:

| 🤖 Visual AI Flow Canvas | 🧠 Global AI Provider Credentials |
| :---: | :---: |
| ![Visual AI Flow Canvas](docs/images/visual_flow_designer.png) | ![Global AI Provider Credentials](docs/images/ai_model_config.png) |
| *Visual drag-and-drop orchestration with glowing neon routing paths* | *Decoupled global model configuration and credential manager* |

| ⚙️ Start Node - Model Auto-Fill |
| :---: |
| ![Start Node - Model Auto-Fill](docs/images/quick_config_fill.png) |
| *Declarative dynamic schema inspector supporting quick credential autofill* |

#### 🏢 System Operations Portal Gallery (Operator - `/operate`)
Here is a preview of the core management panels of the System Operations Portal:

| 📊 System Dashboard | 🔌 Softswitch Instance Pool & Leases |
| :---: | :---: |
| ![System Dashboard](docs/images/operate_dashboard.png) | ![Softswitch Instance Pool & Leases](docs/images/operate_freeswitch.png) |
| *Real-time high-concurrency traffic metrics and system throughput overview* | *FreeSWITCH instance heartbeat monitor and active dynamic leasing registry* |

| 🎛️ SIP Gateway Management | 🏢 Merchant Billing & Subscription |
| :---: | :---: |
| ![SIP Gateway Management](docs/images/operate_gateway.png) | ![Merchant Billing & Subscription](docs/images/operate_merchant.png) |
| *Telephony carrier trunk registration and concurrent channel limit management* | *Merchant onboarding, financial ledgers, and billing balance lifecycle* |

| ☎️ SIP Extension Center | ⚙️ Number Selection & Risk Control |
| :---: | :---: |
| ![SIP Extension Center](docs/images/operate_extension.png) | ![Number Selection & Risk Control](docs/images/operate_risk_control.png) |
| *Multi-tenant extension credentials, SIP passwords, and active registration state* | *High-concurrency atomic pool selection, rate limiting, and blacklist guard* |

#### 💼 Merchant Control Center Gallery (Merchant - `/merchant`)
Here is a preview of the Merchant Portal for bulk dialing, real-time calling, and audio CDR audit:

| 🚀 Automated Outbound Dialing | 📞 WebRTC SIP Webphone Dialpad |
| :---: | :---: |
| ![Automated Outbound Dialing](docs/images/merchant_batch_task.png) | ![WebRTC SIP Webphone Dialpad](docs/images/merchant_webrtc_dialpad.png) |
| *Secure bulk customer contact list import, sanitization, and automated scheduling* | *Built-in SIP stack in a sleek HTML5 web telephone workstation* |

| 🎙️ CDR Logs & Voice Recording Audit | 👥 Agent Skill Group Queue |
| :---: | :---: |
| ![CDR Logs & Voice Recording Audit](docs/images/merchant_call_record.png) | ![Agent Skill Group Queue](docs/images/merchant_skill_group.png) |
| *Real-time call data record streams with embedded audio waveform player* | *Merchant call center agent queuing, queue binding, and call distribution strategy* |

### 🎙️ mod_audio_stream real-time RTP voice gateway & Native Go VAD
*   **RTP Voice Stream Bypass**: Fully compliant with FreeSWITCH `mod_audio_stream`. When an ASR state node triggers, `cc-call` commands FreeSWITCH via ESL to stream raw channel audio (16k high-definition, mono) via a low-latency WebSocket connection.
*   **High-Performance WS Audio Gateway**: A built-in WebSocket listener (port `9002`) receives raw binary PCM packets, executing real-time Root Mean Square (RMS) energy metrics.
*   **Native Silence Detection (VAD)**: A custom Voice Activity Detection (VAD) algorithm calculates user speech initiation (allowing instant agent interrupt) and completion (1.0s silence threshold) without bloated external binary dependencies.

### 🚫 Strict Physical Telephony, Zero Mock/Simulation Fallback
*   **Absolute Fail-Closed Model**: To protect production integrity and merchant financial systems, Yunshu has fully removed any form of mock simulation or mock fallback logic. If credentials are missing, API keys are blank, or external cloud requests (ASR, TTS, or LLM) fail, the engine strictly rejects fallback. Instead, it returns a rigorous physical error to the state machine, triggering a safe, immediate Fail-Closed hangup or human-agent transfer, eliminating system "hallucinations" or mock leakages in production.
*   **ClaimDue Distributed Lease Workers**: Multi-instance worker orchestration uses database-backed locks to coordinate Outbox task claims, preventing double-billing or downstream notification racing while ensuring automatic recovery of failed tasks.
*   **Reliable Webhook Deliveries**: Pushes downstream CDRs via custom URLs with index retry limits, exponential backoff, and HMAC-SHA256 signature verification.

---

## 🚀 4. Production Deployment Roadmap & Configuration Guide

To guarantee telecom-grade high availability, sub-millisecond signaling latency, and seamless media file synchronization in production, you must adhere to the following deployment architecture recommendations and `production.yaml` configuration guidelines.

### 📡 4.1 Production Topology & Clustering

In a distributed production environment, Yunshu components must be grouped into distinct network boundaries and clustered for high availability:

```text
       DMZ / Public Zone [Firewall Filtered]        VPC Private Zone [Ultra-Low Latency, High-Speed Link]
    ┌──────────────────────┐        ┌────────────────────────────────────────────────────────┐
    │       cc-edge        ├───────►│    cc-console Cluster (2+ Nodes, Load Balanced)        │
    │ (Auth/Limit/Proxy)   │       │   ┌──────────────────────────────────────────────┐     │
    └──────────────────────┘        │   │                                              │     │
                                    │   ▼                                              ▼     │
                                    │ ┌───────────────┐                          ┌─────────┐ │
                                    │ │ MySQL Cluster │                          │  Redis  │ │
                                    │ │ (Primary/Sec) │                          │ Sentinel│ │
                                    │ └───────────────┘                          └────┬────┘ │
                                    │                                                 ▲      │
                                    │   ┌─────────────────────────────────────────────┘      │
                                    │   ▼                                                    │
                                    │ ┌───────────────┐   ESL Long Conn (<1ms) ┌────────────┐ │
                                    │ │    cc-call    ├─────────────────────►│ FreeSWITCH │ │
                                    │ │ (2+ Node Clu) │                      │ GW Cluster │ │
                                    │ └───────┬───────┘                      └─────┬──────┘ │
                                    │         │                                    │         │
                                    │         └──────────┐              ┌──────────┘         │
                                    │                    │Mount Shared  │                    │
                                    │                    ▼              ▼                    │
                                    │               ┌────────────────────────┐               │
                                    │               │   NFS / NAS Storage    │               │
                                    │               │ (TTS Cache & Recs)     │               │
                                    │               └────────────────────────┘               │
                                    │                            ▲                           │
                                    │                            │Mount Shared               │
                                    │                            │                           │
                                    │                      ┌─────┴──────┐                    │
                                    │                      │ cc-worker  │                    │
                                    │                      │ (2+ Nodes) │                    │
                                    │                      └────────────┘                    │
                                    └────────────────────────────────────────────────────────┘
```

#### 1. Microsecond Latency Constraint (VPC Network Planning)
- **Constraint**: Network latency between the signaling core (`cc-call`) and the FreeSWITCH media gateways must be **< 1ms**.
- **Planning**: Co-locate `cc-call` and FreeSWITCH inside the **exact same private VPC subnet**. For extremely high-concurrency environments, we strongly recommend deploying `cc-call` directly on the FreeSWITCH host as a system daemon, communicating over the local loopback interface (`127.0.0.1`) to eliminate any signaling race conditions.

#### 2. Stateless Scaling & Event Lease High Availability
- **`cc-call` Stateless Signaling Cluster**: Horizontally deploy 2+ instances of `cc-call`. To prevent multiple instances from racing or consuming the same FreeSWITCH ESL events redundantly, `cc-call` leverages a Redis-backed node registrar to dynamically claim a single active listener lease per FS node. If an instance fails, the lease expires and a standby instance takes over immediately.
- **`cc-worker` Distributed Task Processor**: Horizontally deploy 2+ instances of `cc-worker`. Outbox queue tasks (billing settlement, recording compression, downstream Webhook pushing) are claimed and processed by competing worker instances via the atomic `ClaimDue` lease mechanic, guaranteeing zero duplicate tasks and automatic failover.
- **Redis Sentinel & Persistent Storage**: Redis acts as the single source of truth for hot pathways (extension status, atomic balance checks, and double concurrency limits). It **MUST be deployed as a Sentinel or Redis Cluster** in production, with AOF persistence enabled to achieve sub-second state recovery.

#### 3. Low-Latency Shared Filesystem (TTS & Recordings)
- **TTS Cache**: `cc-call` synthesizes LLM voice answers and writes MP3 files locally. FreeSWITCH must playback these files via absolute paths instantly.
- **Recordings**: FreeSWITCH records dual-stream calls locally, and `cc-worker` must read them to transcode and upload to Alibaba Cloud OSS or Tencent Cloud COS.
- **Planning**: Enforce a low-latency **NFS / NAS shared volume** mapped to the **exact same absolute folder path** (e.g. `/var/lib/yunshu/shared`) across all FreeSWITCH, `cc-call`, and `cc-worker` hosts, with complete `r/w` permissions.

---

### ⚙️ 4.2 Production Config Guidelines (production.yaml)

For production environments, copy `configs/default.yaml` to `configs/production.yaml` and configure the following parameters to ensure performance and isolation:

```yaml
# =====================================================================
# 1. Relational Database & Connection Pool Optimization (MySQL)
# =====================================================================
database:
  # REQUIRED: Point to your production MySQL primary VIP with strong credentials
  dsn: "yunshu_prod:SecurePass123!@tcp(mysql-vip.prod.lan:3306)/yunshu?charset=utf8mb4&parseTime=True&loc=Local"
  # OPTIMIZATION: Tune connection pool sizes to handle high call volumes
  max_open_conns: 100        # Max active DB connections to avoid transactional waiting queues
  max_idle_conns: 20         # Max idle connections to minimize TCP socket recycle cost
  conn_max_lifetime: 3600    # Connection lifetime limit in seconds

# =====================================================================
# 2. Redis Cache & Concurrency Locking (Sentinel/Cluster)
# =====================================================================
redis:
  # REQUIRED: Point to your production Redis Sentinel VIP or cluster entry point
  addr: "redis-sentinel.prod.lan:6379"
  # REQUIRED: Strong production Redis password
  password: "RedisStrongPassword456$"
  db: 0
  pool_size: 150             # OPTIMIZATION: Scale up connection pool to support high-frequency Lua concurrent allocations

# =====================================================================
# 3. Edge Gateway Security & Access Control (cc-edge)
# =====================================================================
edge:
  port: 8080                 # Exposed proxy API port
  rate_limit:
    enable: true
    capacity: 200            # Enforced maximum bucket capacity (burst limit for outbound API per merchant)
    rate: 50                 # Token replenishment rate per second

# =====================================================================
# 4. FreeSWITCH ESL & Audio Stream Configurations
# =====================================================================
freeswitch:
  # REQUIRED: Enforce strong credentials for ESL (Do NOT use default ClueCon)
  esl_addr: "fs-node1.prod.lan:8021"
  esl_password: "FsSuperSecurePassword789!"
  # REQUIRED: Point mod_audio_stream websocket push target to the cc-call production IP
  audio_stream_ws: "ws://cc-call-internal-vip.prod.lan:9002/audio"

# =====================================================================
# 5. Shared Storage Path Mappings (TTS & Recording NFS Mounts)
# =====================================================================
storage:
  # REQUIRED: Shared NFS/NAS mount path, identical across all servers
  shared_root: "/var/lib/yunshu/shared"
  # REQUIRED: Shared path for TTS synthesis files
  tts_cache_dir: "/var/lib/yunshu/shared/tts_cache"
  # REQUIRED: Shared path matching FreeSWITCH recording destination
  recordings_dir: "/var/lib/yunshu/shared/recordings"

# =====================================================================
# 6. Async Worker & Downstream Webhook Pushing (cc-worker)
# =====================================================================
worker:
  billing:
    enable: true
    # WARNING: Fallback rate. Production billing enforces exact templates; unconfigured accounts trigger system warnings
    default_rate_per_min: 0.15 
  recording:
    enable: true
    oss_bucket: "yunshu-recordings-prod"
    oss_endpoint: "oss-cn-shenzhen.aliyuncs.com"
  downstream:
    # REQUIRED: Endpoint for pushing customer CDR data securely
    webhook_url: "https://api.merchant-platform.com/callbacks/cdr"
    # OPTIMIZATION: SHA256 signing secret for pushing validation
    signature_secret: "MerchantSecretKeySignatureXYZ"

# =====================================================================
# 7. Outbound Telecom-Grade Concurrency Orchestration (CTI Engine)
# =====================================================================
cti:
  concurrency:
    # WARNING: Redis selection locks lease duration (Recommend 30 mins to avoid locking leaks)
    claim_ttl_ms: 1800000 
    # REQUIRED: Enforce simultaneous double concurrency limitations (both Gateway and Phone levels)
    enable_double_limit: true
```

---

### 🛠️ 4.3 Step-by-Step Production Deployment Steps

#### Step 1: Initialize Network & Storage Volumes
1. Provision high-performance Linux hosts (Ubuntu 20.04+ / CentOS 7+).
2. Set up high-availability MySQL & Redis clusters.
3. Configure a low-latency **NFS/NAS shared file share**. Mount it under `/var/lib/yunshu/shared` on the FreeSWITCH, `cc-call`, and `cc-worker` hosts, granting full read/write permission (`chmod -R 777`).

#### Step 2: Configure FreeSWITCH Gateway
1. Edit `autoload_configs/event_socket.conf.xml` to restrict IP access and set a robust password.
2. Enable `mod_audio_stream` for WebSocket raw RTP PCM pushes.
3. Enforce the Shared Recording volume as the default destination for recording files (`/var/lib/yunshu/shared/recordings`).

#### Step 3: Build & Deploy Frontend (Nginx)
1. Build the static workspace inside `web/`:
   ```bash
   cd web
   npm install
   npm run build
   ```
2. Copy the resulting `dist/` directory to Nginx, routing fallback paths to `index.html`.

#### Step 4: Compile & Run Go Microservices
1. Compile Go binaries from the repository root:
   ```bash
   go build -o bin/cc-edge ./cmd/cc-edge
   go build -o bin/cc-console ./cmd/cc-console
   go build -o bin/cc-call ./cmd/cc-call
   go build -o bin/cc-worker ./cmd/cc-worker
   ```
2. Copy `configs/default.yaml` to `configs/production.yaml` and configure it according to **4.2 Production Config Guidelines**.

#### Step 5: Enforce Systemd System Service Availability
1. Manage microservices using **Systemd** to achieve automatic recovery. Create `/etc/systemd/system/cc-call.service`:
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
2. Reload system configurations with `systemctl daemon-reload` and start all services, ensuring 24/7 self-healing and service reliability.

---

## 📂 5. Physical Project Directory Structure

```text
├── cmd/                        # Service process entrypoints
│   ├── cc-call/                # Telephony CTI ESL runtime controller
│   ├── cc-console/             # Web administration and developer APIs
│   ├── cc-worker/              # Async distributed billing & CDN uploader
│   ├── cc-edge/                # Edge authentication & reverse-proxy gateway
│   ├── cc-all/                 # All-in-One local launcher
│   └── update-agents/          # Telephony schema code-generator
├── internal/
│   ├── app/                    # System dependency assembler & Gin launcher
│   ├── domain/                 # Core pure business logic (Zero ORM/DB/Redis imports)
│   │   ├── callflow/           # AIVoiceEngine IVR topology & CDR flows
│   │   ├── cti/                # ACD skill groups, Redis queues, and routing chains
│   │   ├── esl/                # FreeSWITCH signaling abstractions & session state machine
│   │   └── operate/            # AI configurations, billing ledger schemas, and contracts
│   ├── transport/              # Handler adapters (Gin HTTP & Redis Stream consumers)
│   ├── contracts/              # Shared event definitions, error codes, and key patterns
│   └── infra/                  # GORM models, repository patterns, and outbox buffers
├── pkg/                        # Standard utility wheels (Idempotent locks, state machines)
├── web/                        # React + Vite visual flow editor workshop
└── docs/                       # Architectural designs, migration logs, and API guides
```

---

## 🛠️ 6. Quick Start (Local Development)

### 1. Prerequisites
*   **Go**: `Version >= 1.21`
*   **NodeJS**: `Version >= 18`
*   **MySQL**: `Version >= 5.7`
*   **Redis**: `Version >= 6.0`

### 2. Run the Front-End Workspace
```bash
cd web
npm install
npm run dev
```

### 3. Launch Go Microservices (All-in-One Dev Mode)
To simplify local debugging, use the `cc-all` launcher to spin up all four backend services in a single console terminal window:
```bash
# Copy and update local databases and connections
cp configs/default.yaml configs/local.yaml

# Run All services concurrently
go run ./cmd/cc-all -config configs/local.yaml
```

---

## 🗺️ 7. Development Roadmap

Yunshu is aggressively migrating and refining its legacy backend modules into this highly optimized Go architecture. The following checklist details our active development milestones, completed features, and upcoming features:

### Phase 1: Commercial-Grade AI Decoupling & Security [100% Completed]
*   ✅ **Multi-Provider Unified Adapters**: Real-world integration of DeepSeek, OpenAI, Tencent Hunyuan, Alibaba Qwen, and Volcengine Doubao.
*   ✅ **Strict去仿真化 (Fail-Closed Paradigm)**: Removed all mock/simulation fallbacks to ensure production-grade safety and error-handling.
*   ✅ **Runtime Capability Introspection**: Dynamic API endpoints to self-detect backend registration status and disable unsupported options in UI.
*   ✅ **Dynamic Schema Cards**: Automatically load specific input fields and voices based on chosen ASR/TTS/LLM providers.

### Phase 2: High-Concurrency Telephony Rules & Event Leases [In Progress]
*   ⏳ **Dynamic Phone Number Search & Weighted Selection**: Redis-backed rules chain to select candidate gateway numbers with atomic rate throttle.
*   📅 **FreeSWITCH ESL Node Leases**: Claim-due lease management in `cc-call` to prevent double Event Stream subscriptions on multi-instance deployments.
*   📅 **Early Media & Ringback Hooking**: Full tracking and execution of progress media files before call bridging.

### Phase 3: Distributed Billing Ledgers & Workflow Decoupling [Planned]
*   ✅ **CDR Outbox Table Setup**: Native reliable outbox queue to stage basic hangup facts.
*   📅 **Asynchronous Bill Calculation**: Separate MQ billing consumer to handle rate templates and anti-overcharge locking.
*   📅 **Atomic Balance Deductions**: Redis Lua scripts to execute safe balance deductions with immediate overdrawn notifications.

### Phase 4: Reliable Asynchronous Workers & Downstream Push [In Progress]
*   ✅ **ClaimDue Worker Core**: Distributed lock managers to claim and handle outbox queue entries.
*   ✅ **Reliable Downstream CDR Push**: Downstream Webhooks supporting exponential backoff, state tracking, and HMAC-SHA256 signature verification.
*   ⏳ **Async Call Recording CDN Uploads**: Background uploader for recordings, mapping upload receipts to final database indexes.

### Phase 5: Live Database Permissions & Console Security [In Progress]
*   ✅ **GORM Models & DB Seeds**: Schema definitions for routing permissions and seed SQL script.
*   ⏳ **Dynamic Authorization Middleware**: Live middleware verifying operator requests against active MySQL database maps.

---

## ⚖️ 8. Service SLA & Disclaimer

### 👤 Individual Developer Status
Yunshu is an **independently developed and self-maintained open-source project. It does not belong to any corporate entity, telecom carrier, or commercial agency**. All architecture designs, code updates, and Bug fixes are carried out voluntarily by the developer out of technical passion. Because of this personal project status, please note that we cannot provide corporate-level service contracts, business invoices, or long-term on-site consulting services.

### 🙇‍♂️ Support SLA Commitments
As the project is currently in an active rewrite and migration phase to a high-performance Go runtime, some advanced signaling scenarios, complex dynamic ACD skills, and custom third-party gateways are still undergoing active testing and integration. We sincerely apologize for any temporary inconveniences caused during integration testing!

**🚀 Fast-Response Support SLA**:
To guarantee a worry-free experience for adopters and community users, we offer high-grade support SLA commitments:
- **Fast Diagnostic Response**: Bug reports, SIP disconnects, or general feature inquiries submitted via GitHub Issues will receive an engineering response within **2 hours** of submission.
- **Rapid Bug Patching**: Standard bugs and configuration issues will be resolved, verified, and hotpatched into the main branch within **24 hours**. Complex FreeSWITCH signaling conflicts, high-concurrency race issues, or unique telecom carrier configurations will receive a detailed workaround or custom patch within **48 hours** to keep your business running smoothly.

### ⚠️ Legal Disclaimer
1. **Compliance & Telephony Regulations**: Yunshu is built as a high-performance distributed customer service framework. Users deploying this platform for active outbound calling must strictly comply with all national and local telephony regulations, anti-fraud directives, and user privacy laws. **The developers of Yunshu assume zero liability for direct or indirect legal consequences arising from non-compliant calling campaigns, nuisance calling, database leakage, or overall platform abuse.**
2. **Generative AI & LLM Warning**: The platform integrates third-party LLM completions. Outputs generated by external language models (e.g., DeepSeek, OpenAI, Tencent Hunyuan) are probabilistic. **Yunshu does not guarantee 100% accuracy, safety, or compliance of AI-generated responses.** Deployers must configure proper ACD transfer nodes, manual supervisor interventions, and strict fail-closed hangup rules to mitigate potential AI "hallucinations."
3. **No Warranties**: This open-source software is licensed under the **GPL-3.0** license and is provided "AS IS," without warranties or conditions of any kind. Users assume all network and computing resource overhead risks associated with running this platform.

---

## 📄 9. License

This project is licensed under the **[GNU General Public License v3.0 (GPL-3.0)](LICENSE)**. Full details are available in the [LICENSE](LICENSE) file in the root directory.
