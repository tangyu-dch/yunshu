# 云枢 (Yunshu) 呼叫中心项目 -- 全面优化审查报告

> 审查范围: 全部 Go 源码 (329 个文件), 覆盖 domain / infra / transport / app / cmd 各层
> 审查日期: 2026-06-05

---

## 一、P0 -- 必须立即修复的严重问题

### 1.1 无优雅关闭机制 (app/server.go)

所有微服务入口 (`cc-console`, `cc-call`, `cc-worker`, `cc-edge`) 的 `ListenAndServe` 直接调用 `server.ListenAndServe()`，**没有监听 SIGTERM/SIGINT 信号，没有实现 graceful shutdown**。收到终止信号时，正在处理的 HTTP 请求会被强制中断，WebSocket 连接断开，outbox 投递中断。`cc-all` 虽然监听了信号，但只做了 `time.Sleep(1s)` 就退出，没有调用 `server.Shutdown(ctx)`。

**建议**: 在 `Server.ListenAndServe` 中增加信号监听，调用 `http.Server.Shutdown(ctx)` 等待在途请求完成。`cc-all` 模式下需要协调多个 Server 的关闭顺序。

### 1.2 panic 替代 error 返回 (app/call_runtime.go)

`openRuntimeDB` 和工作流引擎初始化均使用 `panic` 作为错误处理策略。在 `cc-all` 单进程模式下，任何一个服务初始化失败都会导致整个进程崩溃，其他 3 个服务也跟着挂掉。

```go
// call_runtime.go 第 231-243 行
if cfg.MySQL.DSN == "" {
    panic("MySQL DSN is empty")
}
if err != nil {
    panic(fmt.Sprintf("Failed to connect to MySQL: %v", err))
}
```

**建议**: 将所有 `panic` 改为返回 `error`，让调用者决定处理策略。`cc-all` 模式下某个服务初始化失败应仅记录错误并跳过，而非终止整个进程。

### 1.3 安全漏洞集合

以下安全问题需要优先修复:

**a. 硬编码 AES 加密密钥** (transport/console/operate/dialpad_compat_routes.go 第 31-34 行)

```go
const (
    SIPCredentialKey = "vL4oU4jJ8qS3oC4v"
    PhoneNumberKey   = "2has1d8jef49v0ru"
)
```

AES 密钥直接写在源码中。应从环境变量或密钥管理服务中读取。

**b. AES-ECB 模式使用** (dialpad_compat_routes.go 第 958-992 行)

ECB 模式不提供语义安全（相同明文块总是产生相同密文块），应改用 AES-GCM 或 AES-CBC+HMAC。

**c. 默认明文密码硬编码** (domain/auth/session.go 第 68-73 行)

```go
{Username: "admin", Password: "admin123", ...}
{Username: "operator", Password: "operator123", ...}
```

即使是开发兜底，也极易被意外部署到生产环境。

**d. Token 明文记录到日志** (domain/auth/session.go 第 155 行)

完整的 session token 以 Info 级别被写入日志系统，存在 token 泄露风险。

**e. Token 可通过 URL 参数传递** (transport/console/operate/auth_routes.go 第 92-104 行)

URL 会被记录在 nginx 日志、浏览器历史、Referer 头中。

**f. 下载路由存在路径穿越风险** (dialpad_compat_routes.go 第 700-711 行)

`release.DownloadURL` 来自数据库，如果数据库被注入恶意路径（如 `../../etc/passwd`），可能导致任意文件读取。

**g. 版本上传接口缺少鉴权** (dialpad_compat_routes.go 第 759-851 行)

`POST /mer/version/upload` 路由没有认证中间件，任何人可上传恶意二进制文件。

**h. seed_areas 硬编码数据库密码** (cmd/seed_areas/main.go 第 15 行)

```go
dsn := "root:db123456@tcp(127.0.0.1:3306)/yunshu?..."
```

### 1.4 FreeSWITCH Connect 竞态条件 (infra/fsesl/connection.go 第 173-236 行)

从 RUnlock 到后续 Lock 之间有无锁窗口期。两个并发 Connect 调用可能同时建立连接，后者覆盖前者导致旧连接泄漏（没有被关闭）。

**建议**: 使用 `sync.Map` 或 `singleflight.Group` 确保同一地址只建立一个连接。

### 1.5 WebSocket Origin 校验全部禁用 (infra/websocket/hub.go, asr_server.go)

```go
CheckOrigin: func(*http.Request) bool { return true }
```

两个 WebSocket 服务都禁用了 Origin 校验，存在 CSRF/WebSocket 劫持风险。恶意网页可通过 JavaScript 建立 WebSocket 连接并接收敏感的 CTI 和 ASR 数据。

### 1.6 全局缺少速率限制

整个 transport 层没有任何速率限制机制: 登录接口无暴力破解防护，API 外呼接口无频率限制，Kamailio 回调无限制，分页查询无限制。项目已有 `ShardedLimiter` 实现但未在 transport 层使用。

### 1.7 大量路由缺少认证中间件

运营端 (`/operate/*`) 和商户端 (`/merchant/*`) 的绝大多数 CRUD 路由没有挂载任何认证中间件。认证应作为全局中间件在路由组层面统一挂载。

---

## 二、P1 -- 生产上线前应修复的高优先级问题

### 2.1 架构违规: domain 层存在基础设施依赖

**a. AI 引擎直接进行 HTTP 外部调用** (domain/callflow/ali.go, doubao.go, deepseek.go, openai.go, tencent.go)

所有 AI 引擎实现都直接在 domain 层创建 `http.Client` 并发起外部 HTTP 请求。domain 层应只定义接口，将 HTTP 实现放到 `internal/infra/` 下。

**b. domain 层直接依赖 Redis 客户端** (domain/auth/session.go)

`RedisSessionStore` 直接嵌入了 `goredis.Client` 具体类型。Redis 实现应放在 `internal/infra/redis/` 包下。

**c. TTS 缓存直接进行文件系统 I/O** (domain/callflow/ai_engine_providers.go 第 99-126 行)

文件系统的创建和写入操作属于基础设施关注点。

### 2.2 goroutine 泄漏风险

多处使用 `context.Background()` 启动的 goroutine 脱离了父上下文的生命周期管理:

- `domain/callflow/ai_engine.go` 第 343-351 行: AI 流程延时跳转
- `domain/callflow/consumer.go` 第 167 行: ACW 冷却
- `domain/callflow/consumer.go` 第 1048 行: 排队超时
- `infra/fsesl/connection.go` 第 205-229 行: 事件监听回调闭包捕获了 Connect 的 ctx
- `infra/websocket/asr_server.go` 第 120-232 行: ASR 连接没有 ReadDeadline

**建议**: 使用可取消的 context 替代 `context.Background()`，为所有长时间 goroutine 增加取消机制。

### 2.3 Redis 配置严重缺失 (infra/redis/client.go)

```go
return goredis.NewClient(&goredis.Options{
    Addr:         addr,
    DB:           cfg.DB,
    ReadTimeout:  readTimeout,
    WriteTimeout: writeTimeout,
})
```

缺少: `PoolSize`, `MinIdleConns`, `DialTimeout`, `MaxRetries`, `ConnMaxIdleTime`, `ConnMaxLifetime`。对于呼叫中心高并发场景，默认配置可能成为瓶颈。

### 2.4 Redis Stream 无死信处理和容量限制

- 处理失败的消息留在 pending list 中，没有 XAUTOCLAIM 或最大重试次数限制
- XAdd 没有设置 `MaxLen` 参数，Redis Stream 会无限增长，高呼叫量下可能导致 Redis OOM

### 2.5 SaveChannel/SaveNumber 缺少事务保护 (infra/security/blacklist.go)

"先查询后写入"(check-then-act) 模式没有事务保护，并发场景下两个请求可能同时判定为 isCreate=true，导致主键冲突。

**建议**: 使用 GORM 的 `clause.OnConflict` 替代 check-then-act。

### 2.6 /healthz 不检查依赖连通性

健康检查只返回 `"status": "UP"`，不检查 MySQL、Redis、FreeSWITCH 的可用性。对 K8s 探活来说是误导性的。

**建议**: 增加 `/readyz` 端点，检查关键依赖的 ping 状态。

### 2.7 无 Dockerfile

`docker/` 目录下只有 `mysql/init.sql`，没有为 Go 服务提供 Dockerfile，无法容器化部署。

### 2.8 无 Prometheus metrics 端点

项目没有暴露 `/metrics` 端点，无法采集请求量、延迟、错误率、呼叫成功率等关键运营指标。

### 2.9 重复数据库连接 (app/server.go 第 546 行)

`RegisterDialpadCompatRoutes` 中再次调用 `openRuntimeDB`，导致同一进程创建多个独立的数据库连接池，浪费连接资源。

**建议**: 将 `gorm.DB` 作为共享依赖注入到 Server 结构体中。

### 2.10 cc-all 模式下 AutoMigrate 并发竞争

AutoMigrate 放在 `openRuntimeDB` 中，4 个服务进程同时执行迁移可能产生竞争条件。生产环境应使用独立的迁移工具（如 golang-migrate），或确保只执行一次。

---

## 三、P2 -- 建议尽快优化的中优先级问题

### 3.1 代码质量问题

**a. consumer.go 超过 1665 行** (domain/callflow/consumer.go)

"上帝函数"包含了所有呼叫流程的路由和处理逻辑。建议按流程类型拆分为独立文件。

**b. NewConsoleRuntimeWithConfig 函数 340 行** (app/server.go 第 122-461 行)

承担了 20+ 个仓储的创建和装配。建议将每个领域的装配逻辑抽取为独立的 builder 函数。

**c. 大量重复的工具函数**

`stringFromMap`, `boolFromMap`, `intFromMap`, `firstNonEmpty` 等在 callflow、esl、cti 多个包中重复实现。建议提取到 `pkg/util` 或 `internal/contracts` 中。

**d. originate.go 中方法代码重复** (domain/esl/originate.go)

`StartAPICustomerOutbound` 和 `StartDialpadCustomerOutbound` 共享约 80% 的代码逻辑，应提取公共方法。

**e. 死代码: retryDelay 方法未使用** (domain/callflow/outbox_dispatcher.go 第 167-172 行)

### 3.2 数据库性能问题

**a. 批量任务统计使用 4 次独立 COUNT** (infra/business/task.go 第 289-330 行)

应使用单次查询 + `SUM(CASE WHEN ...)` 聚合。

**b. 通话统计 6 次独立 COUNT** (transport/dialpad_compat_routes.go 第 573-579 行)

同样可用一条条件聚合 SQL 替代。

**c. 分页查询中的关联子查询** (infra/business/task_admin.go 第 46-47 行)

每行结果都执行一次关联子查询，建议改用 JOIN + GROUP BY。

**d. 批量导入号码使用循环单条插入** (infra/resource/phone_group.go 第 185-189 行)

100 个号码产生 100 次 INSERT，应使用 `tx.Create(&refs)` 批量插入。

### 3.3 并发安全

**a. 全局引擎注册表非线程安全** (domain/callflow/ai_engine_providers.go)

`Register*` 函数是导出的公共 API，读写 map 没有加锁保护。

**b. ShardedLimiter 使用全局互斥锁** (infra/limit/sharded_limiter.go)

名字叫"ShardedLimiter"但实际没有分片，高并发场景下全局锁成为热点。

**c. MemoryBus 同步执行无超时保护** (infra/events/bus.go)

所有 handler 同步执行且无超时，某个 handler 阻塞会导致整个发布方阻塞。

### 3.4 安全问题

**a. 缺少 CORS、CSRF 防护**: 整个 transport 层没有 CORS 配置或 CSRF Token 验证。

**b. 错误信息泄露内部实现细节**: 多处将 `err.Error()` 直接拼入响应消息，可能暴露数据库表名、SQL 语句等。

**c. LIKE 查询允许通配符注入**: 用户输入包含 `%` 或 `_` 字符会改变查询语义。

**d. FreeSWITCH ESL 密码和 API 密钥明文存储在数据库中。**

**e. `checkAppCredentials` 静默放行**: 不携带凭证的请求也能自由调用 CTI 外呼接口。

### 3.5 配置和运维

**a. 环境变量覆盖不完整**: Redis 地址、连接池参数、FreeSWITCH 配置、日志配置等关键项无法通过环境变量覆盖。

**b. PID 管理原始**: 使用 `nohup + pidfile` 方式管理后台进程，生产环境应使用 systemd 或容器编排。

**c. cc-edge 是空壳服务但仍连接数据库**: 浪费连接资源。

**d. cc-firewall-guard 的 consoleBaseURL 条件判断无效**: 两个分支赋值相同，是代码 bug。

---

## 四、P3 -- 可渐进优化的低优先级问题

### 4.1 领域模型设计

- `CallSession.Metadata` 过度使用 `map[string]any`，缺乏类型安全，建议定义强类型结构体
- `LicenseService` 使用 XOR 混淆作为"加密"，不是真正的加密算法
- CTI 工作流 Handlers 全部为空，应在文档中说明设计意图

### 4.2 测试覆盖盲区

- `outbox/outbox.go`: 接口定义文件没有测试
- AI 引擎 (ali.go, doubao.go 等): 无对应测试文件
- 并发场景测试缺失: consumer.go 中大量 goroutine 没有并发测试
- cc-firewall-guard: 整个守护进程 563 行没有测试
- infra/db/mysql.go: 数据库连接工具没有测试
- infra/storage/rustfs.go: 文件存储没有测试

### 4.3 HTTP 响应体未完全读取

多个 AI 引擎实现在错误分支只调用 `resp.Body.Close()` 而没有先排空响应体，导致 HTTP 连接无法复用。建议在关闭前使用 `io.Copy(io.Discard, resp.Body)`。

### 4.4 其他

- `resolveSetID` 使用手动字符串解析 JSON，对格式变化脆弱
- `env()` 辅助函数在多处重复定义
- go.mod 所有依赖标记为 `// indirect`
- ASR Server 端口 9002 硬编码
- FreeSWITCH 重连是线性退避而非指数退避
- Redis Pub/Sub 断线重连固定 1 秒，无指数退避
- `parseID` 函数在两个文件中重复定义
- REST 语义不一致 (PUT 做新增, POST 做删除)

---

## 五、项目亮点 (值得保持的优点)

1. **DDD 分层架构清晰**: domain/infra/transport/app 职责边界明确，依赖方向正确
2. **Outbox 模式设计优秀**: 支持租约、重试和指数退避，CDR 通过 fanout 分发到多个后续处理节点
3. **状态机设计严谨**: 通话生命周期和工作流状态转换覆盖完整
4. **接口隔离良好**: `SessionStore`, `CommandExecutor`, `CandidateSource` 等小接口设计得当
5. **幂等设计完善**: 命令服务、资源分配均考虑了幂等性
6. **统一的日志框架**: 全项目使用 `log/slog` 结构化日志
7. **统一的响应格式**: `contracts.OK/Fail` 标准化返回
8. **契约发现端点**: `/contracts/routes`, `/contracts/redis`, `/contracts/mq` 对微服务治理非常有用
9. **测试覆盖面广**: 核心流程（选号、桥接、补振铃、排队、ACW、outbox 投递）均有测试
10. **Makefile 完善**: 提供了完整的构建、运行、运维生命周期管理

---

## 六、修复优先级路线图

```
第一阶段 (P0, 1-2 周):
  ├── 实现优雅关闭 (graceful shutdown)
  ├── panic 改为 error 返回
  ├── 修复安全漏洞 (硬编码密钥、ECB、Token 泄露、路径穿越、版本上传鉴权)
  ├── 修复 FreeSWITCH Connect 竞态
  ├── 启用 WebSocket Origin 校验
  ├── 增加速率限制
  └── 补全路由认证中间件

第二阶段 (P1, 2-4 周):
  ├── 迁移 domain 层的基础设施依赖到 infra 层
  ├── 修复 goroutine 泄漏 (使用可取消 context)
  ├── 完善 Redis 连接池和 Stream 管理
  ├── 修复事务缺失 (SaveChannel/SaveNumber)
  ├── 增强 /healthz + 增加 /readyz
  ├── 提供 Dockerfile
  ├── 增加 Prometheus metrics
  └── 消除重复数据库连接

第三阶段 (P2, 持续优化):
  ├── 拆分巨型函数和文件
  ├── 提取公共工具函数
  ├── 优化数据库查询 (合并 COUNT、批量插入)
  ├── 修复并发安全问题
  ├── 增加 CORS/CSRF 防护
  └── 完善配置管理

第四阶段 (P3, 长期):
  ├── 补充测试覆盖盲区
  ├── 优化领域模型 (强类型 Metadata)
  ├── 替换伪加密为标准方案
  └── 清理死代码和代码重复
```
