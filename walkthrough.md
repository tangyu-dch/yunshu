# 云枢多厂商 AI 引擎解耦与高可扩展架构重构总结 (Walkthrough)

本期重构任务完美达成了**“云枢”呼叫中心大模型 (LLM)、语音识别 (ASR)、语音合成 (TTS)** 底层引擎的多厂商完全解耦、多态抽象重构，以及**全站业务对仿真/Mock 的彻底物理净化**。

按照最新的规划，我们已将历史测试性质的 **“自研仿真模拟大模型 (MOCK)”** （value 为 `"mock"`）从全栈中**彻底物理清理与删除**。同时，将 Go 后端运行时引擎（`ai_engine.go` 与 `tencent.go`）中残留的所有仿真退化兜底、MOCK 应答及本地 mock 生成函数完全剔除。物理环境在未配置凭证或接口调用失败时将执行严格的 **Fail-Closed（物理拦截与错误中断）**，使配置中心完全专注于真实的物理商用引擎驱动（火山豆包、阿里通义、腾讯混元、OpenAI 及 DeepSeek）。

同步完成了项目主文档的国际化重构，默认 `README.md` 采用全英文撰写，并与 `README_zh.md`（中文）提供无缝互通切换。文档中新增了极其详尽的**物理部署建议与架构拓扑**，明确阐述了哪些微服务需要与 FreeSWITCH 进行同机协同部署或共享挂载卷，极大提升了项目的工业化与商业严谨性。

---

## 🛠️ 修改与新增文件

### 1. 后端多厂商物理驱动彻底去仿真化 (Zero Mock / Fail-Closed)

- **[MODIFY] [tencent.go](file:///Users/tangyu/Projects/yunshu/internal/domain/callflow/tencent.go)**：
  - **彻底移除 ASR 仿真**：`TencentASREngine.Transcribe` 彻底删除了在无凭证时的腾讯云 ASR 仿真转译退化分支，改为严格返回凭证未配置错误。
  - **彻底移除 TTS 仿真**：`TencentTTSEngine.Synthesize` 彻底清除了 `MOCK_TENCENT_TTS_AUDIO_DATA_MP3` 兜底，改为严格返回物理凭证未配置错误。
  - **彻底移除 LLM 仿真**：`TencentLLMEngine.GenerateReply` 彻底清除了无凭证时模拟腾讯混元大模型回复的业务分支，改为直接严格报错。
  - **清理 unused imports**：移除了因仿真分支被删而多余的 `"strings"` 包导入，保障极致整洁。
- **[MODIFY] [ai_engine.go](file:///Users/tangyu/Projects/yunshu/internal/domain/callflow/ai_engine.go)**：
  - **拦截与报错严谨化**：在 `ProcessASRText` 意图未命中进行 LLM 穿透对话时，如果云端大模型接口请求报错或返回空内容，系统坚决拒绝仿真退位，直接将 error 泡泡上抛给 CTI 通信运行时；如果大模型提供商未配置，直接抛出未配置物理报错，实现 100% Fail-Closed 通信拓扑控制。
  - **完全物理删除本地 Mock 生成**：删除了原有的 `mockLLMGenerate` 方法，消除任何仿真应答的代码痕迹。
- **[MODIFY] [ali.go](file:///Users/tangyu/Projects/yunshu/internal/domain/callflow/ali.go)** / **[deepseek.go](file:///Users/tangyu/Projects/yunshu/internal/domain/callflow/deepseek.go)** / **[openai.go](file:///Users/tangyu/Projects/yunshu/internal/domain/callflow/openai.go)**：
  - 清理了未使用的 `"strings"` 包导入，确保项目无任何编译警告。

### 2. 单元测试物理报错回归断言

- **[MODIFY] [ai_engine_test.go](file:///Users/tangyu/Projects/yunshu/internal/domain/callflow/ai_engine_test.go)**：
  - **重构 Fallback 测试**：将 `TestAIVoiceEngineProcessASRTextFallback` 从断言 “播放本地 Mock 兜底回复” 修改为断言 “物理大语言模型未配置并严格报错”，并确认没有任何播放动作被下发（拒绝仿真兜底），完美契合 Fail-Closed 通信行为。

### 3. 项目主文档国际化与部署架构建议补齐

- **[MODIFY] [README.md](file:///Users/tangyu/Projects/yunshu/README.md)**：
  - **默认英文文档**：将主库根目录下的 README 全面翻译为极其专业规范的英文，排版精致大方。
  - **物理去仿真化**：删除了历史遗留的 `Self-Driving Sandbox` 等仿真内容说明。
  - **高性能物理部署建议**：新增了部署指引（Deployment Recommendations & Guide），清晰列出了哪些组件需要与 FreeSWITCH 部署在同一服务器上（如 TTS 缓存共享、`cc-worker` 录音文件转储、<1ms 延迟的 ESL 信令控制以及 `mod_audio_stream` 旁路 WebSocket 推流），并附带了详细的部署流程步骤（Deployment Workflow）。
- **[NEW] [README_zh.md](file:///Users/tangyu/Projects/yunshu/README_zh.md)**：
  - **中英文互通切换**：新建了高质量的中文 README 业务文档，首行与英文 README 提供互相切换的 Markdown 锚点链接。同样完全净化了仿真字样，并包含了中文版本的 FreeSWITCH 同机部署架构指导与具体部署工作流。

---

## 🧪 自动化回归测试报告

### 1. 前端 React/TypeScript 100% 静态编译成功
我们在 web 根目录下对整个前端画布与新类型声明执行了严格模式下的静态编译：
```bash
npx tsc --noEmit
```
**结果**：
编译 100% 成功，前端 providers 剔除 mock 后无任何类型推导错误！

### 2. 后端 Go 代码 100% 全绿绿灯回归与 Vet 静态分析
我们在根目录下执行了全量测试和格式化检查：
```bash
gofmt -w .
go test ./...
go vet ./...
```
**结果**：
后端所有核心单元测试（基于 mock 接口断言和 GORM 内存防护）**100% 全绿通过**，Go Vet 静态分析完全零报错！

---

## 🎙️ 交付与就绪校验步骤

1. **大模型物理拦截验证**：
   - 部署微服务后，创建一个不填 API Key 的大模型配置，并在智能语音 IVR 画布的 Start 节点绑定该服务商。
   - 发起通话进入 ProcessASRText 对话逻辑，故意说出不包含在流程图 keyword 里的词（促使其发起 LLM 对话）。
   - 验证控制台日志直接输出 `物理凭证未配置，物理引擎拒绝仿真退化` 并严格报错挂断，没有任何仿真文本兜底。
2. **多语言文档体验**：
   - 打开根目录下的 [README.md](file:///Users/tangyu/Projects/yunshu/README.md)（默认呈现全英文说明）。
   - 点击最上方的 `[中文版](README_zh.md)` 锚点，即可无缝跳转至全中文描述的 [README_zh.md](file:///Users/tangyu/Projects/yunshu/README_zh.md)。
   - 在部署建议章节中，可仔细查阅关于共享 TTS 音频缓存、`cc-worker` 录音读写以及信令同机部署的硬核工业指南，结构严谨，内容精美。
