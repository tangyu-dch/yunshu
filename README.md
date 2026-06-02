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

## 🚀 4. Deployment Recommendations & Installation Guide

To achieve telecom-grade stability, sub-millisecond signaling speeds, and smooth media file handling, strict adherence to the following deployment recommendations is highly recommended.

### 📡 4.1 Which Components Must Be Co-Located on the Same Server as FreeSWITCH?

In production, **the `cc-call` and `cc-worker` microservices MUST either be deployed on the exact same physical/virtual host as FreeSWITCH, or have access to a shared high-performance Network File System (e.g., NFS, NAS, GlusterFS) mapped to the same absolute directory paths**.

The engineering rationale is as follows:

#### 1. Shared TTS Voice Synthesis Cache (Required)
When the AI voice engine translates text to speech using configured cloud APIs (Alibaba, Tencent, OpenAI, Volcengine), the microservice writes the synthesized `.mp3`/`.wav` file directly to a local cache directory (e.g., `/var/lib/yunshu/tts_cache`).
FreeSWITCH plays this audio by invoking ESL's `playback /var/lib/yunshu/tts_cache/xxxx.mp3`.
*   **Deployment Constraint**: **FreeSWITCH must have immediate, direct filesystem access to these files.** Therefore, `cc-call` and FreeSWITCH must either share the local disk of a single host, or mount an NFS network volume to the **exact same absolute folder path** on both servers.

#### 2. Call Recording Uploads (Required for `cc-worker`)
When a call is answered, FreeSWITCH records the conversation and saves the resulting audio locally (e.g., `/var/log/freeswitch/recordings/xxxx.wav`).
Upon hangup, `cc-worker` reads this audio file, compresses/transcribes it, and uploads it to the merchant's cloud CDN bucket (OSS/COS).
*   **Deployment Constraint**: **`cc-worker` requires direct read/write permission to FreeSWITCH's recording output directory.** It must either run directly on the FreeSWITCH host as a system daemon, or share filesystem access via a low-latency NFS mount.

#### 3. Sub-Millisecond ESL Control Latency (Highly Recommended)
`cc-call` communicates with FreeSWITCH via active TCP sockets on the ESL port (`8021`). Telephony SIP signaling is extremely latency-sensitive. A round-trip command delay exceeding 5ms can lead to race conditions, SIP bridging failures, and sluggish key (DTMF) menu responses.
*   **Deployment Constraint**: Keep `cc-call` and FreeSWITCH in the **same physical server or inside a dedicated high-speed VPC** to guarantee latency under **1ms**.

#### 4. Real-Time RTP WebSocket Push (`mod_audio_stream`)
FreeSWITCH pushes high-frequency PCM audio payloads over WebSocket streams to the ASR Gateway (port `9002`).
*   **Deployment Constraint**: The WebSocket receiver must be co-located or linked in a ultra-low-jitter local network with FreeSWITCH to avoid packet dropouts and broken AI sentence segmentations.

---

### 🛠️ 4.2 Step-by-Step Deployment Workflow

#### Step 1: Prepare Database & Middlewares
1. Provision a high-performance Linux server (Ubuntu 20.04+ / CentOS 7+).
2. Install and launch:
   - **MySQL (>= 5.7)**: Establish database credentials and import the Yunshu schema.
   - **Redis (>= 6.0)**: Ensure it is running as the single source of truth for extension status and locks.
3. If running distributed hosts, configure an **NFS server** and mount the shared directory (e.g., mapped to `/var/yunshu/shared`) to the same local mount points on both FreeSWITCH and the Go microservice hosts.

#### Step 2: Configure FreeSWITCH
1. Install FreeSWITCH and configure `event_socket.conf.xml` (allow internal network IPs and enforce a strong ESL password).
2. Enable `mod_audio_stream` in `modules.conf.xml` and restart FreeSWITCH to allow raw audio pushes.
3. Ensure the FreeSWITCH system user has proper read and write permissions to the call recordings folder.

#### Step 3: Compile and Deploy Frontend
1. Navigate to the `web/` workspace:
   ```bash
   cd web
   npm install
   npm run build
   ```
2. Copy the compiled static files (`dist/`) to your web server (e.g., Nginx) and set up HTML5 history routing fallback.

#### Step 4: Compile and Configure Go Microservices
1. Build the microservices binaries from the Go root directory:
   ```bash
   go build -o bin/cc-edge ./cmd/cc-edge
   go build -o bin/cc-console ./cmd/cc-console
   go build -o bin/cc-call ./cmd/cc-call
   go build -o bin/cc-worker ./cmd/cc-worker
   ```
2. Copy `configs/default.yaml` to `configs/production.yaml`.
3. Edit `configs/production.yaml` with your production details:
   - Enter your MySQL DSN and Redis host addresses.
   - Provide the FreeSWITCH ESL credentials (host, port, and password).
   - Verify that the `tts_cache` directory configurations perfectly align between the Go services and the FreeSWITCH host mount.
   - Map the `cc-worker` recording paths to FreeSWITCH's active folder.

#### Step 5: Process Daemonization
1. Launch the respective services on their target machines:
   - **Console Host**: Start `cc-console`.
   - **Telephony Host (Co-located with FreeSWITCH)**: Start `cc-call`.
   - **Worker Host (Co-located with FreeSWITCH or via shared storage)**: Start `cc-worker`.
   - **Edge Gateway Host**: Start `cc-edge`.
2. Configure system process managers (like `Systemd` or `Supervisor`) to monitor and automatically restart the Yunshu services in case of failure, ensuring 24/7 telecom-grade SLA availability.

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
