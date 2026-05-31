import { useState } from 'react'
import { Card, Table, Tag, Typography, Button, Space, Divider, Alert, Input, message, Spin, Badge, List } from 'antd'
import { CopyOutlined, ThunderboltOutlined, CodeOutlined, RocketOutlined, BuildOutlined, SettingOutlined, AuditOutlined, CloudServerOutlined, SecurityScanOutlined, PlaySquareOutlined, ApiOutlined, KeyOutlined } from '@ant-design/icons'

const { Title, Paragraph, Text } = Typography

interface ApiParam {
  param: string
  type: string
  required: '是' | '否'
  desc: string
}

interface ApiDefinition {
  id: string
  method: 'GET' | 'POST' | 'PUT' | 'DELETE'
  path: string
  title: string
  desc: string
  params: ApiParam[]
  defaultPayload: string
  mockResponse: string
}

export function OperatorApiDocPage() {
  const [activeModule, setActiveModule] = useState<'merchant' | 'telephony' | 'record' | 'security'>('merchant')
  const [selectedApiId, setSelectedApiId] = useState<string>('mch-create')
  const [copiedText, setCopiedText] = useState<string | null>(null)
  
  // 仿真控制台相关状态
  const [consolePayload, setConsolePayload] = useState<string>('')
  const [sendLoading, setSendLoading] = useState(false)
  const [consoleStep, setConsoleStep] = useState(0)
  const [consoleLogs, setConsoleLogs] = useState<string[]>([])
  const [actualResponse, setActualResponse] = useState<string | null>(null)

  // 1. 运营平台系统级 API 定义集
  const apiModules: Record<'merchant' | 'telephony' | 'record' | 'security', ApiDefinition[]> = {
    merchant: [
      {
        id: 'mch-create',
        method: 'POST',
        path: '/api/v1/operate/merchant/create',
        title: '创建新入驻商户',
        desc: '云枢系统级开户接口。自动为新商户分配唯一的 Merchant ID，初始化计费总览账户，并为后续选号、话务大脑流程配置其专用沙盒。',
        params: [
          { param: 'name', type: 'string', required: '是', desc: '商户企业官方中文全称' },
          { param: 'contactPhone', type: 'string', required: '是', desc: '商户主管理员联系电话' },
          { param: 'initialBalance', type: 'number', required: '否', desc: '商户初始赠送余额 (元)，默认为 0' },
          { param: 'rateTemplateId', type: 'number', required: '是', desc: '关联的系统费率扣费模板 ID' },
        ],
        defaultPayload: `{
  "name": "极客高并发科技公司",
  "contactPhone": "13888888888",
  "initialBalance": 500,
  "rateTemplateId": 12
}`,
        mockResponse: `{
  "code": 200,
  "message": "SUCCESS",
  "data": {
    "merchantId": 1008,
    "name": "极客高并发科技公司",
    "appKey": "ys_key_09fd8b89cca89dd",
    "appSecret": "ys_sec_99fd11aa99cca8bde338d82121e",
    "status": "enabled",
    "balance": 500,
    "createdAt": "2026-05-31T20:11:05+08:00"
  }
}`
      },
      {
        id: 'mch-recharge',
        method: 'POST',
        path: '/api/v1/operate/merchant/recharge',
        title: '商户账户财务充值与扣款',
        desc: '对特定商户的资金流水进行实时干预结算。支持充值或系统强制扣款审计，每次调整均会产生唯一的金融级审计流水号。',
        params: [
          { param: 'merchantId', type: 'number', required: '是', desc: '目标商户唯一 ID' },
          { param: 'amount', type: 'number', required: '是', desc: '变动金额 (元)，正数为充值，负数为扣款' },
          { param: 'remark', type: 'string', required: '是', desc: '本次账务变动审计备注原因' },
        ],
        defaultPayload: `{
  "merchantId": 1008,
  "amount": 2000,
  "remark": "商户端下线充值结转"
}`,
        mockResponse: `{
  "code": 200,
  "message": "SUCCESS",
  "data": {
    "transactionId": "tx_20260531_00989ff",
    "merchantId": 1008,
    "previousBalance": 500,
    "currentBalance": 2500,
    "remark": "商户端下线充值结转",
    "operator": "admin_system"
  }
}`
      }
    ],
    telephony: [
      {
        id: 'tel-node-register',
        method: 'POST',
        path: '/api/v1/operate/telephony/node/register',
        title: '注册物理 FreeSWITCH 节点',
        desc: '向云枢集群中物理热挂载一个新的 FreeSWITCH 交换服务器节点，用于处理话务负载分发。挂载后，系统心跳守护将自动接管该节点的租约监控。',
        params: [
          { param: 'name', type: 'string', required: '是', desc: '节点唯一物理标识名 (如 fs-node-03)' },
          { param: 'ipAddr', type: 'string', required: '是', desc: '物理节点内网通讯 IP' },
          { param: 'eslPort', type: 'number', required: '是', desc: '物理节点 ESL 监听端口，默认 8021' },
          { param: 'eslPassword', type: 'string', required: '是', desc: 'ESL 通讯授权密码' },
        ],
        defaultPayload: `{
  "name": "fs-node-03",
  "ipAddr": "192.168.1.125",
  "eslPort": 8021,
  "eslPassword": "ClueCon_Password"
}`,
        mockResponse: `{
  "code": 200,
  "message": "SUCCESS",
  "data": {
    "nodeId": 5,
    "name": "fs-node-03",
    "ipAddr": "192.168.1.125",
    "status": "online",
    "activeChannels": 0,
    "leaseExpiresAt": "2026-05-31T20:16:05+08:00"
  }
}`
      },
      {
        id: 'tel-gateway-toggle',
        method: 'PUT',
        path: '/api/v1/operate/telephony/gateway/toggle',
        title: '物理网关线路紧急阻断控制',
        desc: '系统级紧急熔断控制。一键封禁或启用特定运营商物理中继网关，已建立的物理通道将根据断线策略自动重分配至备选线路。',
        params: [
          { param: 'gatewayId', type: 'number', required: '是', desc: '目标网关 ID' },
          { param: 'action', type: 'string', required: '是', desc: '操作指令: enable (启用) / disable (停用熔断)' },
          { param: 'reason', type: 'string', required: '是', desc: '熔断原因，将写入系统告警日志' },
        ],
        defaultPayload: `{
  "gatewayId": 4,
  "action": "disable",
  "reason": "运营商中继线路 504 严重碎音，启动线路阻断"
}`,
        mockResponse: `{
  "code": 200,
  "message": "SUCCESS",
  "data": {
    "gatewayId": 4,
    "status": "disabled",
    "activeCallsTerminated": 12,
    "operator": "admin_system",
    "timestamp": 1780138288
  }
}`
      }
    ],
    record: [
      {
        id: 'record-cdr-query',
        method: 'GET',
        path: '/api/v1/operate/billing/cdr/query',
        title: '跨商户系统级话单检索',
        desc: '面向系统运维人员。允许跨商户对整套云枢物理机中产生的所有通话话单 (CDR)、计费明细、通话录音状态进行大规模的多重条件拉取与追溯。',
        params: [
          { param: 'merchantId', type: 'number', required: '否', desc: '筛选特定的商户 ID，不传代表拉取全系统' },
          { param: 'status', type: 'string', required: '否', desc: '通话状态: answered (接通) / no_answer (未接) / busy (忙线)' },
          { param: 'minDuration', type: 'number', required: '否', desc: '最小通话秒数，用于筛选高价值对话' },
        ],
        defaultPayload: `// GET 请求无 Body，Query 参数拼接于 URL：
// ?merchantId=1008&status=answered&minDuration=30`,
        mockResponse: `{
  "code": 200,
  "message": "SUCCESS",
  "data": {
    "total": 1,
    "records": [
      {
        "id": 145009,
        "callId": "uuid-889fd-22bb-cc89",
        "merchantId": 1008,
        "caller": "1001",
        "callee": "13800138000",
        "direction": "outgoing",
        "durationSec": 45,
        "billingSec": 45,
        "hangupCause": "NORMAL_CLEARING",
        "amount": 0.15,
        "recordUrl": "https://records.yunshu.cc/20260530/uuid-889fd.mp3",
        "createdAt": "2026-05-30T14:05:11+08:00"
      }
    ]
  }
}`
      }
    ],
    security: [
      {
        id: 'sec-blacklist-import',
        method: 'POST',
        path: '/api/v1/operate/security/blacklist/import',
        title: '系统级公共黑名单大批量导入',
        desc: '防范高危风险呼叫。允许批量向云枢黑名单数据库导入运营商骚扰号码或高风险欺诈端点，在 ESL 呼叫 GUARD 入口对该批号码直接无感封杀。',
        params: [
          { param: 'phones', type: 'array', required: '是', desc: '需要加入黑名单的电话号码数组' },
          { param: 'type', type: 'string', required: '是', desc: '黑名单类别: risk (风险欺诈) / complain (高投诉高危)' },
          { param: 'remark', type: 'string', required: '否', desc: '黑名单备注' },
        ],
        defaultPayload: `{
  "phones": ["13800000001", "13800000002"],
  "type": "risk",
  "remark": "大批量导入高危欺诈虚假号码"
}`,
        mockResponse: `{
  "code": 200,
  "message": "SUCCESS",
  "data": {
    "importedCount": 2,
    "failedCount": 0,
    "duplicatedCount": 0
  }
}`
      }
    ]
  }

  // 得到当前模块的所有 API
  const currentApis = apiModules[activeModule]
  // 得到选中的 API
  const selectedApi = currentApis.find(api => api.id === selectedApiId) || currentApis[0] || apiModules.merchant[0]

  // 当选择不同模块时，默认选中该模块下的第一个 API，并初始化控制台载荷
  const handleModuleChange = (mod: 'merchant' | 'telephony' | 'record' | 'security') => {
    setActiveModule(mod)
    const apis = apiModules[mod]
    if (apis.length > 0) {
      setSelectedApiId(apis[0].id)
      setConsolePayload(apis[0].defaultPayload)
    }
    setActualResponse(null)
    setConsoleStep(0)
    setConsoleLogs([])
  }

  const handleApiSelect = (api: ApiDefinition) => {
    setSelectedApiId(api.id)
    setConsolePayload(api.defaultPayload)
    setActualResponse(null)
    setConsoleStep(0)
    setConsoleLogs([])
  }

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text)
    setCopiedText(id)
    setTimeout(() => setCopiedText(null), 2000)
  }

  // 极具质感的仿真调试控制台发送逻辑
  const handleSendConsoleRequest = () => {
    setSendLoading(true)
    setConsoleStep(1)
    setConsoleLogs([
      `[CLIENT] 初始化系统运营超级管理员认证凭证 (ROLE: SUPER_ADMIN)...`,
      `[CLIENT] 构建请求目标: ${selectedApi.method} https://api.yunshu.cc${selectedApi.path}`
    ])

    setTimeout(() => {
      setConsoleStep(2)
      setConsoleLogs(prev => [
        ...prev,
        `[AUTH] 物理令牌校验: OK (Admin JWT Token Verified)`,
        `[SECURITY] 本地防篡改完整性签名通过`,
        `[SERVER] 正在转发请求至云枢微服务集群 CTI 调度层...`
      ])
    }, 800)

    setTimeout(() => {
      setConsoleStep(3)
      setConsoleLogs(prev => [
        ...prev,
        `[ROUTING] 物理集群定位成功，已路由至微服务模块.`,
        `[DATABASE] 事务写入/读取完成，记录已持久化落库.`,
        `[SERVER] 正在封包构建 REST 响应 JSON...`
      ])
    }, 1800)

    setTimeout(() => {
      setConsoleStep(4)
      setConsoleLogs(prev => [
        ...prev,
        `[HTTP] 收到响应: HTTP 200 OK (RTT: 145ms)`,
        `[SUCCESS] 仿真操作交互顺利闭环！`
      ])
      setActualResponse(selectedApi.mockResponse)
      setSendLoading(false)
      message.success('云枢极客仿真交互终端数据返回成功！')
    }, 2800)
  }

  return (
    <div className="max-w-7xl mx-auto p-4 md:p-6 space-y-6 animate-fade-in font-sans">
      {/* 头部导航信息 */}
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <Title level={2} className="!mb-1 font-bold text-slate-800 dark:text-zinc-100 flex items-center gap-2.5">
            <CloudServerOutlined className="text-indigo-500" />
            云枢系统级运营管理 API 互动浏览器中心
          </Title>
          <Paragraph className="text-slate-500 dark:text-zinc-400 !mb-0 font-medium text-xs md:text-sm">
            专为云枢超级管理员与系统运营人员提供的高级 API 对接文档与极客仿真沙盒交互控制台。支持物理网关熔断、商户资金注入、公共安全阻断等顶级系统接口联调。
          </Paragraph>
        </div>
        <Tag color="purple" className="px-3 py-1 font-bold rounded-lg border-purple-200">
          运营管理级 JWT 授权
        </Tag>
      </div>

      <Alert
        message="云枢系统运营 API 使用安全与规范提示"
        description="本中心内所有 API 均涉及系统核心资产与底层物理交换机，要求请求必须携带运营管理员高权限 JWT Token 头 (Authorization: Bearer <AdminToken>)。任何商户平台的 API 证书 (AppKey/Secret) 均无法访问此类高特权接口，以防止数据泄露与非法熔断。"
        type="warning"
        showIcon
        className="rounded-2xl shadow-sm border-amber-100 bg-amber-50/20"
      />

      {/* Stripe/Twilio 现代化极客三栏架构面板 */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-stretch">
        
        {/* 第一栏：系统导航树 (3/12 宽度) */}
        <div className="lg:col-span-3 space-y-4">
          <Card 
            title={<span className="font-extrabold text-xs text-slate-400 uppercase tracking-wider">系统级运营接口模块</span>}
            bordered={false} 
            className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40"
          >
            <div className="flex flex-col gap-1">
              <Button 
                type={activeModule === 'merchant' ? 'primary' : 'text'} 
                icon={<SettingOutlined />}
                onClick={() => handleModuleChange('merchant')}
                className={`justify-start text-left font-bold ${activeModule === 'merchant' ? 'shadow-sm bg-indigo-600' : 'text-slate-600 dark:text-zinc-300'}`}
              >
                🏢 商户全生命周期管理
              </Button>
              <Button 
                type={activeModule === 'telephony' ? 'primary' : 'text'} 
                icon={<BuildOutlined />}
                onClick={() => handleModuleChange('telephony')}
                className={`justify-start text-left font-bold ${activeModule === 'telephony' ? 'shadow-sm bg-indigo-600' : 'text-slate-600 dark:text-zinc-300'}`}
              >
                📞 通信物理层与网关监管
              </Button>
              <Button 
                type={activeModule === 'record' ? 'primary' : 'text'} 
                icon={<AuditOutlined />}
                onClick={() => handleModuleChange('record')}
                className={`justify-start text-left font-bold ${activeModule === 'record' ? 'shadow-sm bg-indigo-600' : 'text-slate-600 dark:text-zinc-300'}`}
              >
                📊 话务质检与录音审计
              </Button>
              <Button 
                type={activeModule === 'security' ? 'primary' : 'text'} 
                icon={<SecurityScanOutlined />}
                onClick={() => handleModuleChange('security')}
                className={`justify-start text-left font-bold ${activeModule === 'security' ? 'shadow-sm bg-indigo-600' : 'text-slate-600 dark:text-zinc-300'}`}
              >
                ⚙️ 全局网络与系统安全
              </Button>
            </div>

            <Divider className="my-3.5" />

            <div className="text-[10px] text-slate-400 uppercase tracking-wider font-extrabold mb-2 pl-1.5">接口终点清单 (API Endpoints)</div>
            <div className="space-y-1">
              {currentApis.map(api => (
                <div 
                  key={api.id}
                  onClick={() => handleApiSelect(api)}
                  className={`p-2 rounded-xl border transition-all cursor-pointer flex flex-col gap-1 ${
                    selectedApiId === api.id 
                      ? 'border-indigo-200 bg-indigo-50/30 dark:bg-indigo-950/20' 
                      : 'border-slate-100 hover:border-slate-200 dark:border-slate-800/40 bg-slate-50/40 dark:bg-slate-900/30'
                  }`}
                >
                  <div className="flex items-center gap-1.5">
                    <Tag 
                      color={
                        api.method === 'POST' ? 'blue' : 
                        api.method === 'GET' ? 'green' : 
                        api.method === 'PUT' ? 'orange' : 'rose'
                      } 
                      className="text-[9px] font-extrabold px-1 py-0 border-none rounded-md scale-90"
                    >
                      {api.method}
                    </Tag>
                    <span className="text-xs font-bold text-slate-700 dark:text-zinc-200 truncate max-w-[140px]">{api.title}</span>
                  </div>
                  <span className="text-[10px] font-mono text-slate-400 dark:text-zinc-500 truncate pl-1">{api.path}</span>
                </div>
              ))}
            </div>
          </Card>
        </div>

        {/* 第二栏：接口详情展示与参数表 (4/12 宽度) */}
        <div className="lg:col-span-4 space-y-4">
          <Card 
            title={
              <div className="flex flex-col gap-1 py-0.5">
                <div className="flex items-center gap-2">
                  <Tag 
                    color={
                      selectedApi.method === 'POST' ? 'blue' : 
                      selectedApi.method === 'GET' ? 'green' : 
                      selectedApi.method === 'PUT' ? 'orange' : 'rose'
                    } 
                    className="font-extrabold px-1.5 py-0 rounded-lg border-none"
                  >
                    {selectedApi.method}
                  </Tag>
                  <Title level={5} className="!mb-0 font-extrabold text-slate-800 dark:text-zinc-100">{selectedApi.title}</Title>
                </div>
                <span className="text-[10px] font-mono text-slate-400 dark:text-zinc-500 mt-1">{selectedApi.path}</span>
              </div>
            }
            bordered={false} 
            className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40"
          >
            <Paragraph className="text-xs text-slate-500 dark:text-zinc-400 leading-relaxed">
              {selectedApi.desc}
            </Paragraph>

            <Divider className="my-4" />

            <Title level={5} className="font-extrabold text-xs text-slate-400 uppercase tracking-wider mb-3">请求入参定义 (Parameters)</Title>
            <Table
              dataSource={selectedApi.params}
              rowKey="param"
              pagination={false}
              size="small"
              bordered
              className="text-xs"
              columns={[
                { 
                  title: '参数', 
                  dataIndex: 'param', 
                  key: 'param', 
                  width: 90, 
                  render: (t) => <Text code className="font-bold text-slate-700 dark:text-zinc-300 text-[10px]">{t}</Text> 
                },
                { 
                  title: '类型', 
                  dataIndex: 'type', 
                  key: 'type', 
                  width: 70, 
                  render: (t) => <Tag color="blue" className="text-[9px] scale-90 border-none font-bold">{t}</Tag> 
                },
                { 
                  title: '必填', 
                  dataIndex: 'required', 
                  key: 'required', 
                  width: 50, 
                  render: (t) => <span className={`text-[10px] ${t === '是' ? 'text-rose-500 font-bold' : 'text-slate-400'}`}>{t}</span> 
                },
                { 
                  title: '描述', 
                  dataIndex: 'desc', 
                  key: 'desc',
                  render: (t) => <span className="text-[10px] leading-normal">{t}</span>
                }
              ]}
            />
          </Card>
        </div>

        {/* 第三栏：极客仿真互动沙盒终端 (5/12 宽度 - Wow 交互) */}
        <div className="lg:col-span-5 space-y-4 flex flex-col justify-stretch">
          <Card 
            title={
              <span className="font-extrabold text-xs text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                <CodeOutlined className="text-indigo-500" />
                Live API 仿真联调沙盒与交互式控制台
              </span>
            }
            bordered={false} 
            className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40 bg-[#121620] dark:bg-black/20 text-slate-300 flex-1 flex flex-col"
            bodyStyle={{ padding: '16px', display: 'flex', flexDirection: 'column', flex: 1 }}
            extra={
              <Button
                type="primary"
                size="small"
                icon={<ThunderboltOutlined />}
                loading={sendLoading}
                onClick={handleSendConsoleRequest}
                className="bg-gradient-to-r from-indigo-500 to-purple-600 border-none font-bold text-xs rounded-lg"
              >
                发送仿真测试请求
              </Button>
            }
          >
            <div className="text-[10px] text-slate-500 border-b border-slate-800 pb-2 mb-3 flex justify-between uppercase tracking-wider font-extrabold">
              <span>YUNSHU OPERATOR LIVE EMULATOR v1.0.0</span>
              <span className="text-indigo-400">JWT SIGNED</span>
            </div>

            {/* 参数输入框 */}
            <div className="mb-4">
              <div className="text-[10px] text-slate-400 font-bold mb-1.5 uppercase">Request Payload (JSON Body)</div>
              <Input.TextArea
                value={consolePayload}
                onChange={(e) => setConsolePayload(e.target.value)}
                autoSize={{ minRows: 4, maxRows: 6 }}
                className="bg-[#0b0e14] border-slate-800 text-emerald-400 hover:border-slate-700 focus:border-slate-700 text-xs font-mono rounded-xl p-3 shadow-inner resize-none"
              />
            </div>

            {/* 仿真日志输出终端 */}
            <div className="flex-1 min-h-[160px] bg-[#090b10] border border-slate-800/60 rounded-xl p-4 font-mono text-[11px] overflow-y-auto space-y-2 text-slate-300 shadow-inner">
              {consoleStep === 0 ? (
                <div className="flex flex-col justify-center items-center py-12 text-slate-500 font-sans gap-2">
                  <RocketOutlined className="text-2xl text-slate-600 animate-bounce" />
                  <span>终端就绪，点击上方“发送仿真测试请求”进行 API 仿真联调</span>
                </div>
              ) : (
                <div className="space-y-1.5">
                  {consoleLogs.map((log, index) => {
                    let colorClass = 'text-[#5ad4e6]'
                    if (log.includes('[SUCCESS]')) colorClass = 'text-emerald-500 font-bold'
                    if (log.includes('[CLIENT]')) colorClass = 'text-slate-400 font-sans'
                    if (log.includes('[ROUTING]')) colorClass = 'text-emerald-400'
                    if (log.includes('[HTTP]')) colorClass = 'text-yellow-400 font-bold'
                    return (
                      <div key={index} className={`flex items-start gap-1.5 ${colorClass}`}>
                        <span className="text-slate-700 select-none">&gt;</span>
                        <span className="whitespace-pre-wrap">{log}</span>
                      </div>
                    )
                  })}
                </div>
              )}

              {/* 实时响应结果 */}
              {actualResponse && (
                <div className="mt-4 pt-3 border-t border-slate-800/80">
                  <div className="text-[10px] text-slate-500 uppercase tracking-wider font-extrabold mb-2 flex justify-between">
                    <span>Response JSON Payload</span>
                    <Button 
                      size="small" 
                      icon={<CopyOutlined style={{ fontSize: '10px' }} />} 
                      onClick={() => handleCopy(actualResponse, 'res-copy')}
                      className="bg-slate-800/60 hover:bg-slate-800 text-slate-300 border-none scale-90"
                    >
                      {copiedText === 'res-copy' ? '已复制' : '复制'}
                    </Button>
                  </div>
                  <pre className="text-emerald-400 leading-relaxed text-[10px] overflow-x-auto whitespace-pre">{actualResponse}</pre>
                </div>
              )}
            </div>
          </Card>
        </div>

      </div>
    </div>
  )
}
