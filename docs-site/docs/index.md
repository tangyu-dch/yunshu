---
title: 云枢声讯
hero:
  title: 云枢声讯
  description: 高性能分布式智能客服与呼叫中心系统
  actions:
    - text: 快速开始
      link: /guide/quick-start
    - text: 呼叫流程
      link: /telephony/yunshu-phone
features:
  - title: Go 原生高并发话务引擎
    emoji: ⚡
    description: 基于 Go、Redis、MySQL、可靠 Outbox 与事件工作流实现高并发 CTI/ESL 调度。
  - title: FreeSWITCH + Kamailio 通信底座
    emoji: ☎️
    description: Kamailio 负责注册、鉴权、调度，FreeSWITCH 负责媒体与 ESL 控制，云枢声讯负责业务编排。
  - title: 云枢声讯专属客户端
    emoji: 💻
    description: 官方配套桌面云枢声讯，支持坐席注册、云枢声讯直呼、客户呼入与 CTI 状态同步。
  - title: 呼入/呼出/批量/AI 全流程
    emoji: 🤖
    description: 支持 API 外呼、云枢声讯直呼、客户呼入、批量外呼、预测外呼、AI 语音流程。
  - title: CDR/计费/录音可靠收口
    emoji: 📊
    description: 只要呼叫经过云枢声讯并进入会话生命周期，挂断后都会写入 CDR outbox。
  - title: 可观测、可排障、可验证
    emoji: 🔍
    description: 提供日志、SIPp 端到端脚本、状态机测试、工作流文档与部署排障手册。
---

<style>
.yunshu-section {
  margin: 56px 0;
}
.yunshu-kicker {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 999px;
  color: #1677ff;
  background: rgba(22, 119, 255, 0.08);
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 14px;
}
.yunshu-title {
  font-size: 32px;
  line-height: 1.25;
  font-weight: 800;
  margin: 0 0 12px;
  letter-spacing: -0.02em;
}
.yunshu-desc {
  max-width: 860px;
  color: #64748b;
  font-size: 16px;
  line-height: 1.9;
  margin: 0 0 24px;
}
.yunshu-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
  margin-top: 24px;
}
.yunshu-card {
  position: relative;
  min-height: 150px;
  padding: 22px;
  border-radius: 18px;
  background: linear-gradient(180deg, rgba(255,255,255,0.92), rgba(248,250,252,0.92));
  border: 1px solid rgba(148, 163, 184, 0.22);
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.06);
  overflow: hidden;
}
.yunshu-card::after {
  content: '';
  position: absolute;
  inset: auto -30px -40px auto;
  width: 120px;
  height: 120px;
  border-radius: 999px;
  background: radial-gradient(circle, rgba(59,130,246,0.18), rgba(59,130,246,0));
}
.yunshu-card h3 {
  margin: 0 0 10px;
  font-size: 18px;
  font-weight: 750;
}
.yunshu-card p {
  margin: 0;
  color: #64748b;
  line-height: 1.75;
}
.yunshu-icon {
  display: inline-flex;
  width: 38px;
  height: 38px;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  margin-bottom: 14px;
  color: white;
  background: linear-gradient(135deg, #38bdf8, #6366f1, #a855f7);
  box-shadow: 0 10px 24px rgba(99, 102, 241, 0.28);
  font-size: 18px;
}
.yunshu-split {
  display: grid;
  grid-template-columns: 1.05fr 0.95fr;
  gap: 26px;
  align-items: center;
  margin-top: 22px;
}
.yunshu-panel {
  border-radius: 22px;
  padding: 24px;
  background: #07111f;
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.18);
  border: 1px solid rgba(125, 211, 252, 0.22);
}
.yunshu-panel img {
  width: 100%;
  display: block;
  border-radius: 16px;
}
.yunshu-status {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-top: 22px;
}
.yunshu-status-item {
  padding: 16px;
  border-radius: 16px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
}
.yunshu-status-item b {
  display: block;
  color: #0f172a;
  font-size: 15px;
  margin-bottom: 8px;
}
.yunshu-status-item span {
  color: #16a34a;
  font-size: 13px;
  font-weight: 650;
}
.yunshu-links {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-top: 20px;
}
.yunshu-link {
  display: block;
  padding: 18px;
  border-radius: 16px;
  background: linear-gradient(180deg, #ffffff, #f8fafc);
  border: 1px solid #e2e8f0;
  color: #0f172a;
  text-decoration: none;
  font-weight: 700;
  transition: all .2s ease;
}
.yunshu-link:hover {
  transform: translateY(-3px);
  box-shadow: 0 16px 32px rgba(15, 23, 42, 0.1);
  color: #1677ff;
}
@media (max-width: 960px) {
  .yunshu-grid, .yunshu-status, .yunshu-links, .yunshu-split {
    grid-template-columns: 1fr;
  }
}
</style>

<section class="yunshu-section">
  <div class="yunshu-kicker">定位 / Positioning</div>
  <h2 class="yunshu-title">不只是一个呼叫中心，而是一套完整的话务中枢</h2>
  <p class="yunshu-desc">
    云枢声讯面向企业客服、电销、智能 IVR 与 SaaS 多租户场景，
    将 SIP 注册、信令代理、媒体控制、CTI 编排、通话记录、计费、录音与 AI 语音流程拆分为清晰可维护的工程边界。
    它既可以作为私有化客服系统，也可以作为多租户云呼叫中心底座。
  </p>

  <div class="yunshu-split">
    <div class="yunshu-grid" style="grid-template-columns: 1fr 1fr; margin-top: 0;">
      <div class="yunshu-card">
        <div class="yunshu-icon">☎</div>
        <h3>统一话务入口</h3>
        <p>云枢声讯、API 外呼、客户 DID 呼入、批量任务都进入同一套 CTI/ESL 工作流。</p>
      </div>
      <div class="yunshu-card">
        <div class="yunshu-icon">⚙</div>
        <h3>清晰服务边界</h3>
        <p>Kamailio 管信令，FreeSWITCH 管媒体，cc-call 管编排，cc-worker 管异步收口。</p>
      </div>
      <div class="yunshu-card">
        <div class="yunshu-icon">📊</div>
        <h3>通话记录必达</h3>
        <p>只要呼叫进入云枢声讯会话并收到最终挂断事件，就会写入 CDR outbox。</p>
      </div>
      <div class="yunshu-card">
        <div class="yunshu-icon">🤖</div>
        <h3>面向 AI IVR</h3>
        <p>预留 ASR、TTS、LLM、RAG 与可视化流程编排能力，支持智能语音业务扩展。</p>
      </div>
    </div>
    <div class="yunshu-panel">
      <img src="/images/architecture.svg" alt="云枢声讯架构" />
    </div>
  </div>
</section>

<section class="yunshu-section">
  <div class="yunshu-kicker">流程 / Call Flow</div>
  <h2 class="yunshu-title">从 SIP 事件到 CDR，一条可追踪的状态链路</h2>
  <p class="yunshu-desc">
    FreeSWITCH 的物理事件会被转换为 云枢声讯领域事件，由 SessionService 写入会话状态，
    再通过 EventBus 推动 CTI/ESL 工作流，最后由 cc-worker 完成 CDR、计费、录音、投影和回调。
  </p>
  <div class="yunshu-panel">
    <img src="/images/call-flow.svg" alt="云枢声讯呼叫流程" />
  </div>
</section>

<section class="yunshu-section">
  <div class="yunshu-kicker">能力状态 / Capability</div>
  <h2 class="yunshu-title">当前核心能力状态</h2>
  <div class="yunshu-status">
    <div class="yunshu-status-item"><b>云枢声讯注册</b><span>已支持</span></div>
    <div class="yunshu-status-item"><b>客户呼入</b><span>端到端已通过</span></div>
    <div class="yunshu-status-item"><b>API 外呼</b><span>入口已验证 200</span></div>
    <div class="yunshu-status-item"><b>云枢声讯直呼</b><span>核心链路已触发</span></div>
    <div class="yunshu-status-item"><b>无坐席排队</b><span>已支持</span></div>
    <div class="yunshu-status-item"><b>ACW 拉取队列</b><span>已支持</span></div>
    <div class="yunshu-status-item"><b>CDR 通话记录</b><span>已单测覆盖</span></div>
    <div class="yunshu-status-item"><b>Worker 后置任务</b><span>已拆分</span></div>
  </div>
</section>

<section class="yunshu-section">
  <div class="yunshu-kicker">人群 / Audience</div>
  <h2 class="yunshu-title">不同角色如何使用这份文档</h2>
  <div class="yunshu-grid">
    <div class="yunshu-card">
      <div class="yunshu-icon">🚀</div>
      <h3>运维 / 部署人员</h3>
      <p>从快速开始、生产部署、FreeSWITCH、Kamailio、RTPEngine 和上线检查清单开始。</p>
    </div>
    <div class="yunshu-card">
      <div class="yunshu-icon">🧑‍💻</div>
      <h3>后端 / 集成开发</h3>
      <p>查看 API 外呼、CTI API、Webhook、事件工作流和 CDR 计费链路。</p>
    </div>
    <div class="yunshu-card">
      <div class="yunshu-icon">📞</div>
      <h3>SIP / 通信工程师</h3>
      <p>重点查看 SIP 注册、云枢声讯呼出、客户呼入、SIPp 验证和排障 Runbook。</p>
    </div>
  </div>
</section>

<section class="yunshu-section">
  <div class="yunshu-kicker">入口 / Start Here</div>
  <h2 class="yunshu-title">快速入口</h2>
  <div class="yunshu-links">
    <a class="yunshu-link" href="/guide/quick-start">快速开始 →</a>
    <a class="yunshu-link" href="/deployment/production">生产部署 →</a>
    <a class="yunshu-link" href="/telephony/inbound">客户呼入 →</a>
    <a class="yunshu-link" href="/telephony/dialpad-outbound">云枢声讯呼出 →</a>
    <a class="yunshu-link" href="/telephony/api-outbound">API 外呼 →</a>
    <a class="yunshu-link" href="/operations/sipp">SIPp 验证 →</a>
    <a class="yunshu-link" href="/deployment/kamailio">Kamailio →</a>
    <a class="yunshu-link" href="/deployment/freeswitch">FreeSWITCH →</a>
  </div>
</section>
