# “云枢”呼叫中心系统视觉重构、警告消除与业务权限收拢总结 (Walkthrough)

本期对“云枢”呼叫中心系统的整体视觉效果、核心组件交互、API 权限收拢及系统通知逻辑进行了深度重构与优化，消除了历史遗留的代码警告，显著提升了系统的商业质感与安全性。

---

## 🛠️ 主要修改内容

### 1. 呼叫时间段网格选择器样式与交互重构
- **[MODIFY] [page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/business/batch-call-task/page.tsx)**：
  - 重构了“呼叫时间段”的网格选区组件，改为了水平自适应的极简布局。
  - 将时间轴刻度修改为标准的 24 小时制头部（`0:00 - 23:00`）。
  - 编写了连续区间合并及中心定位算法，使用深色胶囊气泡（如 `00:00-24:00`）动态在框选行中央悬浮显示。
  - 支持多行及跨格滑动框选，增加了暂存区 `initialGrid` 逻辑，支持取消时撤销本次未保存的框选状态。
  - 将整周全天、工作日、清空等快捷功能与图例说明一体化收纳。

### 18. 选号配置地区编码化校验、行政区划数据库播种与多级级联选择器升级
- **[MODIFY] [proxy_config.go](file:///Users/tangyu/Projects/yunshu/internal/domain/operate/proxy_config.go)**：
  - 在统一网络参数 `ProxyConfig` 领域模型中加入 `NearbyCities` 属性，并在 `GetConfig` 和 `SaveConfig` 中对系统配置项进行持久化代理，实现了数据层的集中管理。
- **[MODIFY] [risk-control/page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/security/risk-control/page.tsx)**：
  - **可视化邻近城市配置（Tab）**：新增了一个【邻近城市匹配配置】标签页，在表格中优雅展示源城市及按试选优先级以彩色胶囊 Tag 列出的相邻城市映射；提供了“新增/编辑城市映射”的可视化 Modal 弹窗，支持通过带搜索的选择器下拉绑定省市行政区划；闭环实现了前端列表到数据库 JSON 字符串的自动序列化保存机制。
  - **升级多级级联选择器（Cascader）**：将相邻城市表单新增/编辑弹窗中的扁平式 `<Select>` 选择下拉框重构升级为层级清晰的 `<Cascader>` 组件，自动根据数据库返回的行政区划信息构建“省份 -> 地级市/区县”的二级联动树，用户既能逐级点选，也能直接模糊搜索，大大优化了运营配置体验；解决了编辑时回填路径及提交时提取叶子节点编码的数据绑定问题。
  - **拓扑映射卡片美化**：重构了第一页的【选号路由链路拓扑映射】面板。将原本单调的文本箭头拼凑改为了高颜值的**节点式卡片流水线设计**。使用 `ArrowRightOutlined` 图标作为高科技连线指示，并针对“呼叫技能组”、“号码池”、“呼叫网关”、“物理线路”四个物理层级渲染了独特的背景微光、彩色边框、定制图标与状态 Tag，整体视觉品质感跃升，达成 WOW 体验。
- **[NEW] [seed_areas/main.go](file:///Users/tangyu/Projects/yunshu/cmd/seed_areas/main.go)**：
  - 新增一键行政区划数据播种指令。支持在开发及部署环境中一键向数据库表 `cc_sys_area` 快速播种 381 条全国省市行政区划的区域代码与省市名称映射关系，彻底解决因表数据为空导致的选择器空白（暂无数据）问题。

### 2. 界面视觉深度美化与暗黑模式自适应
- **[MODIFY] [login/page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/auth/login/page.tsx)**：
  - 重新设计了登录页，引入了圆角立体玻璃态表单及发光渐变投影，以及多重模糊发光气泡（radial blobs）背景。
  - 设计了高颜值的渐变拓扑图 3D SVG Logo 标志。
- **[MODIFY] [dashboard/page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/dashboard/page.tsx)**：
  - 重新设计了总览面板统计卡片，顶部增加了质感微光渐变彩色横条。
  - 升级了 `ChartWrap` 卡片标题头，左侧增加了垂直渐变彩色指示粗条。
  - 美化了 ECharts 折线、柱状、饼图样式（使用更暗黑和谐调色方案，隐藏折线标记圆点、线宽设为 3，深色半透明毛玻璃悬浮框），大屏视感极佳。
- **[MODIFY] [AdminLayout.tsx](file:///Users/tangyu/Projects/yunshu/web/src/layout/AdminLayout.tsx)**：
  - 支持侧边栏 Sider 及 Menu 的主题（浅色模式白底，深色模式纯黑微蓝 `#090b11`）随系统暗黑/浅色状态无缝平滑切换。
- **[MODIFY] [index.css](file:///Users/tangyu/Projects/yunshu/web/src/styles/index.css)**：
  - 定义了全站全局 `6px` 极细胶囊自定义滚动条，悬浮或滚动时自适应浅色与深色背景色调。

### 3. Ant Design v5 Card 组件警告消除
- 编写自动化替换脚本，将全站所有 12 个 `.tsx` 页面子模块中已被 AntD v5 废弃的 `<Card bordered={false}>` 全部替换修正为最新的包装 API：`<Card variant="borderless">`，彻底清空了控制台的抛错警告。

### 4. 限制商户端套餐自主选择与绑定功能
- **[MODIFY] [page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/merchant/billing/page.tsx)**：
  - 彻底移除了商户端套餐账务详情卡片右上角的“配置套餐”按钮及其绑定的弹窗组件。
  - 移除了商户自主获取可用费率和绑定费率 of React Query 及 Mutation 请求。
  - 使商户端在此模块仅保留对计费单价及结算周期等详细费率参数的**只读查看能力**。
- **[MODIFY] [operate.ts](file:///Users/tangyu/Projects/yunshu/web/src/api/operate.ts)**：
  - 清理了已废弃 of `bindMerchantRate` 及 `fetchActiveRates` 方法。
- **[MODIFY] [permission_routes.go](file:///Users/tangyu/Projects/yunshu/internal/transport/http/console/operate/permission_routes.go)**：
  - 在后端路由注册表中物理删除了用于商户自主选择绑定的 `/merchant/billing/rate/bind` 接口。
- **[MODIFY] [permissions.go](file:///Users/tangyu/Projects/yunshu/internal/contracts/permissions.go)**：
  - 清理了 `PermissionMerchantBillingWrite` 权限常量及对应的静态路由映射关系。
  - 将 `/operate/rate/list-active` 的准入权限安全地变更为运营端专属的 `PermissionOperateRateRead`。
- **[MODIFY] [permission.go](file:///Users/tangyu/Projects/yunshu/internal/infra/system/permission.go)**：
  - 从系统启动初始权限数据库种子中，彻底物理清除了 `merchant:billing:write` 权限码，保证新部署环境纯净安全。

### 5. 试用公告关闭缓存随登录态自动重置
- **[MODIFY] [auth.ts](file:///Users/tangyu/Projects/yunshu/web/src/store/auth.ts)**：
  - 在 `login` 和 `logout` 的核心 Action 中加入 `sessionStorage.removeItem('dismiss_trial_alert')` 清理机制。
  - 确保系统在用户每一次重新登录或登出后，都能自动重置试用宽限期提醒公告的关闭状态，再次正常展现给用户。

### 6. 运营端“商户配置”页面新增快捷充值操作
- **[MODIFY] [page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/merchant/merchant/page.tsx)**：
  - 运营管理平台除了原有的【账务与充值】专属账务模块外，在【商户配置】列表的管理面板操作列中，新增了**“充值”**快捷操作项。
  - 按钮绑定了 `operate:billing:write` 权限控制。
  - 点击后会在当前商户列表页直接拉起**“商户资金充值”弹窗**，支持直接输入金额和充值备注（如凭证号）并一键落库，使运营操作流程更加紧凑连贯。

### 7. 登录页支持暗黑与亮色切换及移除冗余的主题 Select 下拉
- **[MODIFY] [login/page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/auth/login/page.tsx)**：
  - 引入了 `useUiStore` 进行主题状态绑定。
  - 移除了原有的写死 `antdTheme.darkAlgorithm` 以及写死的深色背景，代之以根据 `isDark` 状态动态调色（亮色模式下使用精致的亮灰底色 `bg-slate-50`，浅色玻璃态 Card 边框 `border-slate-200/80`，以及优雅的高对比度深色文字）。
  - 在登录页右上角放置了绝对定位的主题切换图标，点击即可在 Sun (亮色) 和 Moon (暗黑) 模式间流畅切换。
- **[MODIFY] [AdminLayout.tsx](file:///Users/tangyu/Projects/yunshu/web/src/layout/AdminLayout.tsx)**：
  - 移除了顶部导航栏右侧冗余的“浅色主题/深色主题”文本下拉 Select 框，仅保留清爽高颜值的 Sun/Moon 图标按钮，界面更显现代精细。

### 8. 单商户模式（Single-Tenant Mode）商户管理安全保护与充值卡顿修复
- **[MODIFY] [server.go](file:///Users/tangyu/Projects/yunshu/internal/app/server.go)**：
  - 新增了在服务器启动时**自动同步 YAML 配置中的租户模式至数据库 `cc_sys_config` 表中**的逻辑。当 `configs/default.yaml` 设定为 `mode: single` 时，服务启动会自动将数据库配置的 `tenant.mode` 更新为 `single`，从而消除了 YAML 配置文件与数据库字段值冲突（漂移）引起的前端逻辑误判问题。
- **[MODIFY] [operate.ts](file:///Users/tangyu/Projects/yunshu/web/src/api/operate.ts)**：
  - 修复了充值及账务保存请求发送给 Go 后端时返回 `400 Bad Request` 的参数类型问题。将 `merchantId` 从 `string` 正确转换为后端所需的 `number`，并将 `paymentMode` 对应转换为 `paymentModeCode` 字段，彻底解决了资金充值及额度设置卡死的问题。
- **[MODIFY] [page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/merchant/merchant/page.tsx)**：
  - 接入了系统授权隔离模式查询，获取当前的 `tenantMode` 变量。
  - 当以单商户模式（`tenantMode === 'single'`）运行时，**安全隐藏了“新增商户”和“物理删除”两个按钮**，避免管理员在单商户环境下误创建多商户账号或破坏系统默认唯一的商户资质。
- **[MODIFY] [billing-admin/page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/merchant/billing-admin/page.tsx)** & **[merchant/page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/merchant/merchant/page.tsx)**：
  - 将所有 Modal 弹窗在打开时调用 `form.setFieldsValue` 的动作包裹在 `setTimeout(..., 0)` 异步任务中，确保 Form 组件在 DOM 挂载和绑定完成后再进行初始赋值，**彻底消除了控制台中“Instance created by useForm is not connected to any Form element”的抛错警告**。

### 9. 静态 Axios 拦截器与 Query Cache 报错通知的主题上下文绑定
- **[NEW] [antd.ts](file:///Users/tangyu/Projects/yunshu/web/src/utils/antd.ts)**：
  - 编写并创建了 `AntdStaticHelper` 组件，作为 Ant Design v5 的上下文捕获桩。挂载于 `<App>` 内，将动态、感知主题 of `message`、`notification`、`modal` 实例绑定到外部静态变量上输出。
- **[MODIFY] [main.tsx](file:///Users/tangyu/Projects/yunshu/web/src/main.tsx)** & **[http.ts](file:///Users/tangyu/Projects/yunshu/web/src/api/http.ts)**：
  - 在全局渲染入口 `main.tsx` 中挂载 `<AntdStaticHelper />` 组件，并更新了全局 Axios 拦截器 `http.ts` 及 QueryClient 的全局错误提示代码，统一将 `antd` 原生静态导入替换为导入 `@/utils/antd` 的实例。
  - **消除了控制台中“Static function can consume context like dynamic theme. Please use 'App' component instead”的报错警告**，实现了 API 网络错误与全局加载失败弹窗的动态暗黑模式和设计资产完全融合。

### 10. 无访问权限路由屏蔽与全局 404 页面兜底
- **[NEW] [page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/auth/not-found/page.tsx)**：
  - 编写并新增了统一的 `NotFoundPage` 兜底页面组件。展示 404 设计图示，文案为“您访问的页面不存在，或当前账号无权访问”，并提供一键返回系统首页的按钮。
- **[MODIFY] [index.tsx](file:///Users/tangyu/Projects/yunshu/web/src/router/index.tsx)**：
  - 调整了 `RequirePermission` 路由权限守卫。当用户访问其无权访问的敏感页面时，不再展示 403 权限受阻的报错页面，而是**直接返回并渲染 `NotFoundPage` (404 状态)**。这种“无权限即不可知”的安全设计，能够隐匿未授权的路径，防范目录或接口扫描。
### 11. 商户端技能组与号码组表单优化，以及号码池网关显示消除
- **[MODIFY] [pool/page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/resource/pool/page.tsx)**：
  - 彻底删除了号码池页面在运营端/管理端显示的“关联网关”列表列，同时从查询条件、创建和编辑 Modal 表单中移除了“关联呼叫网关”字段，并在保存时完美保持了已有网关数据的安全性，使号码池模块更纯粹专注于号码集合。
- **[MODIFY] [skill-group/page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/business/skill-group/page.tsx)**：
  - 移除了技能组列表中对商户平台冗余的“商户”列。
  - 将创建/编辑 Modal 弹窗中的“商户 ID”输入框隐藏，默认通过底层字段直接提交已登录的商户 ID，避免 SaaS 标识泄露。
  - 在绑定号码的表格中增加了“归属号码池”列，支持清晰展示每个可选号码所归属的号码池，极大地便利了坐席管理员的分配与绑定决策。
- **[MODIFY] [phone-group/page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/business/phone-group/page.tsx)**：
  - 将创建/编辑 Modal 弹窗中的“商户 ID”输入框隐藏。
  - 修复了“绑定号码”弹窗在请求可选号码列表时的 `fetchPoolPhones` API 传参，显式指定 `isMerchant = true` 访问商户私有端点，彻底解决由于以运营身份发起请求造成的 `403 Forbidden` 问题。
- 同样在绑定号码表格中增加了“归属号码池”展示列。

### 12. 本地开发与宿主机运行下媒体代理（RTPEngine）及 FreeSWITCH 节点状态心跳在线检测适配
- **[MODIFY] [rtpengine.go](file:///Users/tangyu/Projects/yunshu/internal/domain/operate/rtpengine.go)**：
  - 在内部核心领域服务中引入了检测是否处于容器环境的 `isRunningInDocker()` 逻辑。
  - 在健康检测 ping 检测中，当检测到目标地址包含容器间专属域名 `rtpengine` 或 `cc-rtpengine` 且后端服务运行在宿主机环境（非容器）下时，自动将解析目标回退到 `127.0.0.1` 环回接口。这使得后台能够通过宿主机的端口转发顺利发出 NG JSON 探测协议，彻底解决了本地开发面板上 RTPEngine 总是提示“故障离线”的显示问题。
- **[MODIFY] [freeswitch.go](file:///Users/tangyu/Projects/yunshu/internal/domain/operate/freeswitch.go)**：
  - 同样在 FreeSWITCH 的 ESL 物理心跳探测中增加了对于 `freeswitch` 与 `cc-freeswitch` 容器域名的检测 fallback 逻辑，保证开发环境下的节点状态判定始终稳定且准确。

---

## 🧪 自动化测试与打包验证报告

### 1. 后端 Go 代码 100% 测试通过
在根目录下执行：
```bash
go test ./... && go vet ./...
```
**结果**：全部核心 CTI/ESL/Operate 测试用例通过，静态语法分析无任何报错，更新契约指令运行平稳。


### 2. 前端 100% 生产环境打包验证成功
在 `web` 目录下执行：
```bash
npm run build
```
**结果**：TypeScript 编译无任何类型推导障碍，Vite 生产环境代码成功打包输出，警告彻底归零，完美运行。

---

## 📡 通话信令追踪 (SIP Trace) 与全局控制开关总结

本期完成了**呼叫信令链路追踪及可视化时序交互图 (sngrep/Homer 功能)** 及其 **全局 ON/OFF 开关控制**，在满足运维排障按需排障分析的同时，具备了“旁路化高效率”与“高并发数据库零负担”的核心亮点。

### 1. 全局配置开关与 Redis 联动 (Fail-Closed Switch)
- **数据库持久化**：在配置参数表 `cc_sys_config` 中追加了 `"siptrace.enable"` 配置项，默认初始种子配置值为 `"0"` (关闭)。
- **微秒级 Redis 校验**：在后端 `ProxyConfigManagementService` 中实现了保存配置时的同步推送逻辑：当运营人员修改系统参数时，服务会将追踪开关状态实时写入 Redis 的 `siptrace:enable` 字符串键。此外，在系统服务启动时，会自动调用 `SyncToRedis` 重新从数据库向 Redis 对齐同步。
- **配置与运行态展示**：在配置管理界面 [page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/system/proxy-config/page.tsx) 的实时健康度卡片中，直观显示了 `SIP 信令追踪：已开启 / 已关闭`，并在参数修改 Modal 表单中增加了 `Switch` 控制开关组件，支持一键动态控制。

### 2. Kamailio 高效旁路抓包 (`kamailio.cfg`)
- 在 [kamailio.cfg](file:///Users/tangyu/Projects/yunshu/configs/kamailio/kamailio.cfg) 的 `route[SIP_TRACE]` 中，使用 `redis_cmd` 执行 `GET siptrace:enable` 命令。
- 当开关未启用或 Redis 获取失败时，**立即返回（return）**，不做任何报文提取和存储，保证常规呼叫没有额外开销。
- 开启时，使用 `###` 强定界符拼接时间戳、方法/状态、源/目的节点和原始报文（`$TV(s).$TV(u)###$rm###$rs###$si:$sp###$ri:$rp###$mb`），以 `RPUSH` 写入 Redis 列表 `sip_trace:$ci` 并通过 `EXPIRE` 赋予 2 小时 TTL，保证海量包仅保存在内存高速缓存，绝不上报 MySQL。

### 3. 后端编排解析 API
- 在 [call_record.go](file:///Users/tangyu/Projects/yunshu/internal/domain/operate/call_record.go) 中，通过 Coder 级别的 parser 方法将 Redis 分割符列表还原，并结合主/被叫腿 Call-ID 回退解析逻辑。
- 提取通信链上的唯一节点集 `nodes` (作为时序图的垂直生命线) 及按微秒排布的信令时序包列表 `trace` 传送至前端。

### 4. 前端 SVG 高仿真交互时序图 (Homer/sngrep UI)
- **行内快捷入口**：在通话记录 (CDR) 列表的操作列中，新增了 **“信令时序”** 快捷按钮。
- **双向自适应 SVG 绘图**：
  - 点击后拉出 1180px 宽的豪华 Drawer，异步载入信令数据。
  - 左侧采用高对比度明暗适配的主题，横向列出节点 IP（如 `坐席手机` | `Kamailio` | `FreeSWITCH`），纵向为交互时序。
  - 采用 React + SVG 自研微秒级画线算法：在多节点生命线之间自动画出横向箭头。如果为 Status 响应（如 200 OK, 180 Ringing）显示对应响应色 Tag，如果为 Method 请求（INVITE, BYE, CANCEL, ACK）显示为控制型 Tag，且对 ACK 使用虚线绘制。
  - 支持自环线弯曲绘制：如果为自节点内部信息（From == To），则渲染贝塞尔三阶曲线（path C）回弹指向自身。
- **高阶报文分析抽屉**：
  - 双栏布局，点击左侧信令行时，右侧会以微动画划出当前消息详情面板。
  - 顶部分离出 **SIP 头部 (Headers)** 与 **媒体协商参数 (SDP)** 分页展示，并从头部提取 `CSeq`、`User-Agent`、`From/To` 做描述表格快速速读。
  - 提供一键复制原始报文的功能，运维排障体验极佳。

### 5. 通话记录默认查询范围与用户 ID 条件检索
- **默认“今天”时间范围**：在 [page.tsx](file:///Users/tangyu/Projects/yunshu/web/src/features/business/call-record/page.tsx) 中，通话记录列表默认初始化为当天（`startOf('day')` 至 `endOf('day')`）。同时，在 `QueryBar` 引入 `initialValues` 支持，重置时恢复今天范围，防止高并发下扫表产生大表慢查询。
- **用户 ID 筛选条件**：在 `queryFields` 过滤字典及 `handleSearch` 提交逻辑中全新加入了 **“用户 ID”** (`userId`) 检索项，结合后端已实现的 `UserID` 查询过滤，完成前后端闭环，支持对指定用户/坐席的历史通话明细进行精准定位。

### 6. 内核级地理围栏防火墙文档完备与代码推送
- **[MODIFY] [README_zh.md](file:///Users/tangyu/Projects/yunshu/README_zh.md)** & **[README.md](file:///Users/tangyu/Projects/yunshu/README.md)**：
  - 在核心产品特色中，新增了 `🛡️ 基于 iptables & ipset 的宿主机级地理围栏防火墙 (cc-firewall-guard)` 的文档说明。详细阐述了黑白名单双模切换、“仅放行国内 IP”强力白名单模式、高平滑热更新与安全保障（Fail-Safe RFC 1918 网段保护）、模拟开发自省（Dry-Run Mode）及前端 IP 归属可视化诊断。
  - 新增了高质量的 **Mermaid 拓扑流与审计流程图**（包含中文与英文双版本），以图形化方式直观展现防火墙配置下发、动态 IP 拦截以及 `journalctl` 实时拦截日志抓取上报控制台的完整数据闭环。
  - 在“生产物理部署步骤”第四步与第五步中补全了 `cc-firewall-guard` 编译指令以及标准的 Linux Systemd 守护进程配置文件配置样例。
  - 更新了“物理项目目录结构”树以及“开发路线图”第一阶段，标记国别 IP 防火墙守护进程为 100% 已完工状态。
- **编译与单元测试校验**：
  - 排除临时 `scratch/` 目录干扰后，运行 `go vet` 及 `go test ./...` 对整站 CTI、ESL、Console 模块进行了 100% 验证，结果全部通过。
- **代码提交与推送**：
  - 将所有修改过的 README 文件提交并推送至远程 `dev` 分支，工作区状态保持 100% 干净。
- **截图更新与 Mermaid 语法修正**：
  - 编写了 Puppeteer 自动化浏览器脚本 `scratch/capture_all.js`，启动本地 Google Chrome 浏览器并依次模拟超级管理员和商户端账号登录，全自动截取了 14 个核心业务及系统页面的最新截图（覆盖更新至 `docs/images/`），图文信息完全符合目前最新版的页面布局，并且为新版内核防火墙专门录入新增了 `operate_ip_block.png` 审计页面截图。
  - 修复了 README 中 Mermaid 语法对于 subgraph 带有特殊字符与括号（如 `(cc-console)`、`(cc-firewall-guard)`）的解析问题，将英文及中文版本的所有 subgraph 声明使用双引号 `""` 闭合处理，完全解决了渲染错误（Unable to render rich display）。
  - 将所有新老图片及修正后的 README 文件一并提交并推送到远程 `dev` 分支。



