import {
  Steps,
  Button,
  Form,
  Input,
  InputNumber,
  Select,
  Card,
  Space,
  Typography,
  message,
  Alert,
  Badge,
  Spin,
  Row,
  Col
} from 'antd'
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  PlayCircleOutlined,
  DatabaseOutlined,
  GlobalOutlined,
  SettingOutlined,
  CloudUploadOutlined,
  ArrowRightOutlined,
  ReloadOutlined,
  ThunderboltOutlined
} from '@ant-design/icons'
import { useState, useEffect, useRef } from 'react'
import {
  fetchInstallStatus,
  saveInstallSetup,
  triggerInstallDeploy,
  fetchInstallDeployStatus,
  startInstallServices
} from '@/api/operate'

const { Title, Text, Paragraph } = Typography

type SetupParams = {
  mysqlHost: string
  mysqlPort: number
  mysqlUser: string
  mysqlPassword: string
  mysqlDatabase: string
  mysqlUseDocker: boolean
  redisHost: string
  redisPort: number
  redisUseDocker: boolean
  externalIp: string
  sipPort: number
  wsPort: number
  rtpStartPort: number
  rtpEndPort: number
  tenantMode: 'single' | 'multi'
  defaultMerchantId: number
}

export function InstallPage() {
  const [currentStep, setCurrentStep] = useState(0)
  const [envStatus, setEnvStatus] = useState<any>(null)
  const [deployLogs, setDeployLogs] = useState<string[]>([])
  const [deployStatus, setDeployStatus] = useState<'idle' | 'deploying' | 'success' | 'failed'>('idle')
  const [deployProgress, setDeployProgress] = useState(0)
  const [isPrechecking, setIsPrechecking] = useState(true)
  const [isSettingUp, setIsSettingUp] = useState(false)
  const [isStartingServices, setIsStartingServices] = useState(false)

  const logEndRef = useRef<HTMLDivElement>(null)
  const [form] = Form.useForm<SetupParams>()

  // 1. 挂载时执行环境预检
  const performPrecheck = async () => {
    setIsPrechecking(true)
    try {
      const res = await fetchInstallStatus()
      setEnvStatus(res)
      if (res?.installed) {
        message.warning('系统已经完成初始化部署！正在重定向主页...')
        setTimeout(() => {
          window.location.href = '/'
        }, 1500)
      }
    } catch (e) {
      message.error('环境状态预检测失败，请检查后端服务是否正常启动')
    } finally {
      setIsPrechecking(false)
    }
  }

  useEffect(() => {
    performPrecheck()
  }, [])

  // 2. 轮询 Docker Compose 后台拉起状态与日志
  useEffect(() => {
    let timer: any = null
    if (deployStatus === 'deploying') {
      timer = setInterval(async () => {
        try {
          const res = await fetchInstallDeployStatus()
          if (res) {
            setDeployLogs(res.logs || [])
            setDeployStatus(res.status)
            setDeployProgress(res.progress)
            if (res.status === 'success') {
              clearInterval(timer)
              message.success('云呼系统 Docker 容器一键部署拉起成功！')
              setCurrentStep(3) // 自动进入下一步
            } else if (res.status === 'failed') {
              clearInterval(timer)
              message.error('Docker Compose 容器部署异常失败，请检查滚动日志。')
            }
          }
        } catch (e) {
          console.error(e)
        }
      }, 1500)
    }
    return () => {
      if (timer) clearInterval(timer)
    }
  }, [deployStatus])

  // 滚动日志到最底部
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [deployLogs])

  // 处理第1步提交：写 configs/default.yaml 与 docker-compose.yml
  const handleSetupSubmit = async (values: SetupParams) => {
    setIsSettingUp(true)
    try {
      await saveInstallSetup(values)
      message.success('基础配置文件已动态生成并写入成功！')
      setCurrentStep(2) // 进入第2步：Docker部署
    } catch (e: any) {
      message.error(e?.message || '写入配置文件异常，请检查写入权限。')
    } finally {
      setIsSettingUp(false)
    }
  }

  // 处理一键启动部署 Docker
  const handleTriggerDeploy = async () => {
    setDeployStatus('deploying')
    setDeployLogs(['>>> 正在初始化 Docker Compose 一键部署命令...'])
    try {
      await triggerInstallDeploy()
    } catch (e: any) {
      setDeployStatus('failed')
      setDeployLogs((prev) => [...prev, `[ERR] 部署命令触发失败: ${e?.message || e}`])
    }
  }

  // 处理第3步提交：执行数据库建表与种子填充
  const handleStartServices = async () => {
    setIsStartingServices(true)
    try {
      const values = form.getFieldsValue()
      await startInstallServices(values)
      message.success('核心数据库表迁移与种子数据填充成功！系统初始化全部完成！')
      setCurrentStep(4) // 进入第4步：安装成功
    } catch (e: any) {
      message.error(e?.message || '数据库自动迁移与配置填充失败，请检查数据库配置与网络连通性。')
    } finally {
      setIsStartingServices(false)
    }
  }

  if (isPrechecking) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen bg-slate-900 text-white">
        <Spin size="large" tip={<span className="text-slate-300">系统正在获取宿主机环境与网络端口占用状态，请稍后...</span>} />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-slate-950 py-12 px-4 sm:px-6 lg:px-8 text-slate-100 flex flex-col justify-between">
      {/* 顶部标题区 */}
      <div className="max-w-6xl mx-auto w-full text-center mb-8">
        <Title level={2} className="!text-white !mb-2 flex items-center justify-center gap-3 font-bold tracking-tight">
          <span className="bg-blue-600 px-3 py-1 rounded text-white text-2xl font-black">云呼</span>
          云呼叫中心系统一键部署引导
        </Title>
        <Text className="text-slate-400 text-sm">
          轻量级交互式安装向导将协助您检测 Docker 运行时环境、录入网络参数并一键拉起话务网关依赖。
        </Text>
      </div>

      {/* 步骤条 */}
      <div className="max-w-6xl mx-auto w-full mb-8 bg-slate-900/50 backdrop-blur border border-slate-800 p-6 rounded-xl shadow-lg">
        <Steps
          current={currentStep}
          className="install-steps"
          items={[
            { title: '环境预检', icon: <PlayCircleOutlined /> },
            { title: '参数配置', icon: <SettingOutlined /> },
            { title: '容器拉起', icon: <CloudUploadOutlined /> },
            { title: '数据库迁移', icon: <DatabaseOutlined /> },
            { title: '安装成功', icon: <CheckCircleOutlined /> }
          ]}
        />
      </div>

      {/* 主体卡片 */}
      <div className="max-w-5xl mx-auto w-full flex-grow">
        {/* 步骤 0: 环境预检 */}
        {currentStep === 0 && (
          <Card
            bordered={false}
            className="bg-slate-900/70 border border-slate-800 shadow-2xl rounded-2xl p-4 sm:p-6"
          >
            <Space direction="vertical" size="large" className="w-full">
              <div className="border-b border-slate-800 pb-4">
                <Title level={4} className="!text-white !mb-1 font-bold">宿主机 Docker 运行时环境预检测</Title>
                <Text className="text-slate-400 text-xs">检测当前操作系统是否支持 Docker 以及软交换/Kamailio 等所需的端口占用情况。</Text>
              </div>

              {/* Docker 检测 */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6 bg-slate-950 p-6 rounded-xl border border-slate-850">
                <div>
                  <div className="text-slate-400 text-xs mb-1">Docker 引擎</div>
                  {envStatus?.dockerInstalled ? (
                    <div className="flex items-center gap-2 text-emerald-400 font-semibold">
                      <CheckCircleOutlined /> 已安装 ({envStatus?.dockerVersion?.split(',')[0]})
                    </div>
                  ) : (
                    <div className="flex items-center gap-2 text-rose-500 font-semibold">
                      <CloseCircleOutlined /> 未安装 (请在宿主机安装 Docker)
                    </div>
                  )}
                </div>
                <div>
                  <div className="text-slate-400 text-xs mb-1">Docker Compose 插件</div>
                  {envStatus?.composeInstalled ? (
                    <div className="flex items-center gap-2 text-emerald-400 font-semibold">
                      <CheckCircleOutlined /> 已安装 ({envStatus?.composeVersion})
                    </div>
                  ) : (
                    <div className="flex items-center gap-2 text-rose-500 font-semibold">
                      <CloseCircleOutlined /> 未检测到 Docker Compose
                    </div>
                  )}
                </div>
              </div>

              {/* 核心端口扫描列表 */}
              <div>
                <div className="text-white font-medium mb-3">系统依赖及信令端口占用扫描:</div>
                <Row gutter={[16, 16]}>
                  {envStatus?.ports?.map((p: any) => (
                    <Col xs={24} sm={12} md={6} key={p.port}>
                      <div className="bg-slate-950 border border-slate-850 p-3 rounded-lg flex items-center justify-between">
                        <div>
                          <div className="text-white text-xs font-mono font-bold">{p.port}</div>
                          <div className="text-slate-500 text-[10px]">{p.name}</div>
                        </div>
                        {p.occupied ? (
                          <Badge count="端口占用" style={{ backgroundColor: '#f5222d' }} />
                        ) : (
                          <Badge count="空闲" style={{ backgroundColor: '#52c41a' }} />
                        )}
                      </div>
                    </Col>
                  ))}
                </Row>
              </div>

              <div className="flex justify-end pt-4 border-t border-slate-800">
                <Space>
                  <Button icon={<ReloadOutlined />} onClick={performPrecheck} type="default" className="bg-slate-800 border-slate-700 text-white hover:!bg-slate-750">
                    重新预检
                  </Button>
                  <Button
                    type="primary"
                    disabled={!envStatus?.dockerInstalled}
                    icon={<ArrowRightOutlined />}
                    onClick={() => {
                      setCurrentStep(1)
                      setTimeout(() => {
                        form.setFieldsValue({
                          mysqlHost: 'mysql',
                          mysqlPort: 3306,
                          mysqlUser: 'root',
                          mysqlPassword: 'Password123!',
                          mysqlDatabase: 'yunshu',
                          mysqlUseDocker: true,
                          redisHost: 'redis',
                          redisPort: 6379,
                          redisUseDocker: true,
                          externalIp: '127.0.0.1',
                          sipPort: 5060,
                          wsPort: 5066,
                          rtpStartPort: 30000,
                          rtpEndPort: 30100,
                          tenantMode: 'single',
                          defaultMerchantId: 1001
                        })
                      }, 0)
                    }}
                  >
                    接受并继续
                  </Button>
                </Space>
              </div>
            </Space>
          </Card>
        )}

        {/* 步骤 1: 参数配置 */}
        {currentStep === 1 && (
          <Card
            bordered={false}
            className="bg-slate-900/70 border border-slate-800 shadow-2xl rounded-2xl p-4 sm:p-6"
          >
            <Form
              form={form}
              layout="vertical"
              onFinish={handleSetupSubmit}
              requiredMark={false}
            >
              <Space direction="vertical" size="large" className="w-full">
                <div className="border-b border-slate-800 pb-2">
                  <Title level={4} className="!text-white !mb-1 font-bold">配置核心组件及网络参数</Title>
                  <Text className="text-slate-400 text-xs">填写将注入生成至 default.yaml 与 docker-compose.yml 配置文件中的配置参数。</Text>
                </div>

                {/* 数据库与 Redis 卡片 */}
                <Card title={<span className="text-white text-xs font-semibold"><DatabaseOutlined className="mr-1 text-blue-500" /> 1. MySQL & Redis 数据源</span>} className="bg-slate-950/80 border border-slate-850 text-slate-300" size="small">
                  <Row gutter={16}>
                    <Col span={8}>
                      <Form.Item name="mysqlHost" label="MySQL 主机" rules={[{ required: true, message: '必填' }]}>
                        <Input />
                      </Form.Item>
                    </Col>
                    <Col span={4}>
                      <Form.Item name="mysqlPort" label="MySQL 端口" rules={[{ required: true, message: '必填' }]}>
                        <InputNumber className="w-full" />
                      </Form.Item>
                    </Col>
                    <Col span={6}>
                      <Form.Item name="mysqlUser" label="MySQL 账号" rules={[{ required: true, message: '必填' }]}>
                        <Input />
                      </Form.Item>
                    </Col>
                    <Col span={6}>
                      <Form.Item name="mysqlPassword" label="MySQL 密码" rules={[{ required: true, message: '必填' }]}>
                        <Input.Password />
                      </Form.Item>
                    </Col>
                  </Row>
                  <Row gutter={16}>
                    <Col span={8}>
                      <Form.Item name="mysqlDatabase" label="库名" rules={[{ required: true, message: '必填' }]}>
                        <Input disabled />
                      </Form.Item>
                    </Col>
                    <Col span={4}>
                      <Form.Item name="mysqlUseDocker" label="由Docker托管">
                        <Select options={[{ label: '内置托管', value: true }, { label: '外部独立', value: false }]} />
                      </Form.Item>
                    </Col>
                    <Col span={6}>
                      <Form.Item name="redisHost" label="Redis 主机" rules={[{ required: true, message: '必填' }]}>
                        <Input />
                      </Form.Item>
                    </Col>
                    <Col span={6}>
                      <Form.Item name="redisPort" label="Redis 端口" rules={[{ required: true, message: '必填' }]}>
                        <InputNumber className="w-full" />
                      </Form.Item>
                    </Col>
                  </Row>
                </Card>

                {/* 网络核心配置卡片 */}
                <Card title={<span className="text-white text-xs font-semibold"><GlobalOutlined className="mr-1 text-indigo-500" /> 2. Kamailio & RTPEngine 网络端口</span>} className="bg-slate-950/80 border border-slate-850 text-slate-300" size="small">
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="externalIp" label="公网映射外部 IP (NAT穿透及媒体宣告关键)" rules={[{ required: true, message: '必填' }]}>
                        <Input placeholder="例如: 203.0.113.5 (或宿主机IP)" />
                      </Form.Item>
                    </Col>
                    <Col span={6}>
                      <Form.Item name="sipPort" label="Kamailio SIP 端口" rules={[{ required: true, message: '必填' }]}>
                        <InputNumber className="w-full" min={1} max={65535} />
                      </Form.Item>
                    </Col>
                    <Col span={6}>
                      <Form.Item name="wsPort" label="WebRTC WebSocket 端口" rules={[{ required: true, message: '必填' }]}>
                        <InputNumber className="w-full" min={1} max={65535} />
                      </Form.Item>
                    </Col>
                  </Row>
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="rtpStartPort" label="RTP 媒体端口范围起始" rules={[{ required: true, message: '必填' }]}>
                        <InputNumber className="w-full" min={1024} max={65535} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="rtpEndPort" label="RTP 媒体端口范围结束" rules={[{ required: true, message: '必填' }]}>
                        <InputNumber className="w-full" min={1024} max={65535} />
                      </Form.Item>
                    </Col>
                  </Row>
                </Card>

                {/* 系统租约模式 */}
                <Card title={<span className="text-white text-xs font-semibold"><SettingOutlined className="mr-1 text-purple-500" /> 3. 云呼商户多租户模式设置</span>} className="bg-slate-950/80 border border-slate-850 text-slate-300" size="small">
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="tenantMode" label="商户隔离模式">
                        <Select options={[{ label: '单商户模式 (极简单机运营版)', value: 'single' }, { label: '多商户模式 (SAAS平台运营版)', value: 'multi' }]} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="defaultMerchantId" label="系统主默认商户 ID (初始化种子使用)">
                        <InputNumber className="w-full" min={1000} />
                      </Form.Item>
                    </Col>
                  </Row>
                </Card>

                <div className="flex justify-between items-center pt-4 border-t border-slate-800">
                  <Button type="default" className="bg-slate-850 border-slate-750 text-white" onClick={() => setCurrentStep(0)}>
                    上一步
                  </Button>
                  <Button type="primary" htmlType="submit" loading={isSettingUp}>
                    保存配置并进入部署
                  </Button>
                </div>
              </Space>
            </Form>
          </Card>
        )}

        {/* 步骤 2: 容器拉起部署 */}
        {currentStep === 2 && (
          <Card
            bordered={false}
            className="bg-slate-900/70 border border-slate-800 shadow-2xl rounded-2xl p-4 sm:p-6"
          >
            <Space direction="vertical" size="large" className="w-full">
              <div className="border-b border-slate-800 pb-2">
                <Title level={4} className="!text-white !mb-1 font-bold">后台拉起 Docker Compose 话务代理容器集群</Title>
                <Text className="text-slate-400 text-xs">通过调用 Docker 宿主机引擎，自动拉起 cc-mysql, cc-redis, cc-rtpengine, cc-freeswitch(软交换) 与 cc-kamailio 容器集群。</Text>
              </div>

              {/* 部署按钮 */}
              {deployStatus === 'idle' && (
                <div className="flex flex-col items-center justify-center p-12 bg-slate-950 rounded-xl border border-slate-850 border-dashed">
                  <PlayCircleOutlined className="text-5xl text-blue-500 mb-4 animate-pulse" />
                  <Paragraph className="text-slate-300 text-center text-sm mb-6">
                    配置文件生成成功！点击下方按钮将开始在后台执行一键部署。这预计需要 2 - 5 分钟 (视网络下载镜像速度而定)。
                  </Paragraph>
                  <Button type="primary" size="large" onClick={handleTriggerDeploy}>
                    一键拉起 Docker 容器部署
                  </Button>
                </div>
              )}

              {/* 部署终端输出 */}
              {deployStatus !== 'idle' && (
                <Space direction="vertical" size="middle" className="w-full">
                  <div className="flex justify-between items-center">
                    <span className="text-xs flex items-center gap-2">
                      状态：
                      {deployStatus === 'deploying' && <Badge status="processing" text="正在拉起镜像及运行容器..." className="!text-blue-400 font-semibold" />}
                      {deployStatus === 'success' && <Badge status="success" text="一键部署容器完成" className="!text-emerald-400 font-semibold" />}
                      {deployStatus === 'failed' && <Badge status="error" text="部署异常中止" className="!text-rose-500 font-semibold" />}
                    </span>
                    <span className="font-mono text-xs text-slate-400">{deployProgress}% 完成</span>
                  </div>

                  {/* 终端风格控制台 */}
                  <div className="bg-black text-slate-300 font-mono text-xs p-5 rounded-xl border border-slate-850 shadow-inner h-96 overflow-y-auto flex flex-col gap-1.5 scrollbar-thin scrollbar-thumb-slate-800">
                    {deployLogs.map((log, index) => (
                      <div key={index} className="leading-relaxed break-all">
                        {log.startsWith('>>>') ? (
                          <span className="text-blue-400 font-bold">{log}</span>
                        ) : log.includes('Error') || log.includes('failed') || log.startsWith('[ERR]') ? (
                          <span className="text-rose-400">{log}</span>
                        ) : (
                          <span>{log}</span>
                        )}
                      </div>
                    ))}
                    <div ref={logEndRef} />
                  </div>

                  {deployStatus === 'failed' && (
                    <div className="flex justify-end gap-3 mt-2">
                      <Button onClick={() => setCurrentStep(1)} className="bg-slate-800 border-slate-700 text-white">
                        修改配置
                      </Button>
                      <Button type="primary" onClick={handleTriggerDeploy}>
                        重新尝试部署
                      </Button>
                    </div>
                  )}
                  {deployStatus === 'success' && (
                    <div className="flex justify-end mt-2">
                      <Button type="primary" onClick={() => setCurrentStep(3)} icon={<ArrowRightOutlined />}>
                        进入数据库迁移
                      </Button>
                    </div>
                  )}
                </Space>
              )}
            </Space>
          </Card>
        )}

        {/* 步骤 3: 数据库迁移与种子数据填充 */}
        {currentStep === 3 && (
          <Card
            bordered={false}
            className="bg-slate-900/70 border border-slate-800 shadow-2xl rounded-2xl p-4 sm:p-6"
          >
            <Space direction="vertical" size="large" className="w-full">
              <div className="border-b border-slate-800 pb-2">
                <Title level={4} className="!text-white !mb-1 font-bold">系统核心数据库结构迁移与种子填充</Title>
                <Text className="text-slate-400 text-xs">连接上一步中拉起的话务端数据库，执行 GORM 数据库自动建表迁移，并播种默认的角色、超管账号及系统参数配置。</Text>
              </div>

              <div className="flex flex-col items-center justify-center p-12 bg-slate-950 rounded-xl border border-slate-850">
                <DatabaseOutlined className="text-5xl text-indigo-500 mb-4 animate-bounce" />
                <Paragraph className="text-slate-300 text-center text-sm mb-6 max-w-lg">
                  Docker 容器部署已成功拉起。现在，我们将开始初始化系统数据库并迁移数据模型架构。点击“执行数据库迁移”将对 MySQL 执行快速表结构映射。
                </Paragraph>
                <Button
                  type="primary"
                  size="large"
                  icon={<ThunderboltOutlined />}
                  loading={isStartingServices}
                  onClick={handleStartServices}
                >
                  执行数据库迁移与种子数据填充
                </Button>
              </div>

              <div className="flex justify-between pt-4 border-t border-slate-800">
                <Button onClick={() => setCurrentStep(2)} className="bg-slate-850 border-slate-750 text-white">
                  返回部署日志
                </Button>
              </div>
            </Space>
          </Card>
        )}

        {/* 步骤 4: 安装成功 */}
        {currentStep === 4 && (
          <Card
            bordered={false}
            className="bg-slate-900/70 border border-slate-800 shadow-2xl rounded-2xl p-4 sm:p-6"
          >
            <div className="flex flex-col items-center text-center py-8">
              <CheckCircleOutlined className="text-6xl text-emerald-500 mb-4" />
              <Title level={3} className="!text-white !mb-2 font-black">恭喜！云呼 callcenter 系统一键安装部署全部完成！</Title>
              <Paragraph className="text-slate-300 max-w-xl text-sm mb-8">
                呼叫中心基础数据库、数据缓存及信令代理容器已就绪并顺利建立桥接通信。系统现在已自动进入 fail-closed 保护状态，拒绝重复初始化。
              </Paragraph>

              {/* 默认账户提示卡片 */}
              <div className="bg-slate-950 p-6 rounded-xl border border-slate-850 max-w-md w-full mb-8 text-left">
                <div className="text-xs text-slate-500 mb-4 font-bold border-b border-slate-800 pb-2 flex items-center justify-between">
                  <span>默认管理员及运营账号凭证</span>
                  <Badge count="安全保障" style={{ backgroundColor: '#52c41a' }} />
                </div>
                <div className="flex justify-between items-center text-xs mb-2">
                  <span className="text-slate-400 font-medium">平台系统管理员 (Operate Admin)</span>
                  <span className="font-mono text-white">admin / 123456</span>
                </div>
                <div className="flex justify-between items-center text-xs">
                  <span className="text-slate-400 font-medium">默认运营端商户 (Merchant Admin)</span>
                  <span className="font-mono text-white">merchant / 123456</span>
                </div>
              </div>

              <Space>
                <Button
                  type="primary"
                  size="large"
                  onClick={() => {
                    window.location.href = '/login/operate'
                  }}
                >
                  去运营管理端登录
                </Button>
                <Button
                  size="large"
                  onClick={() => {
                    window.location.href = '/login'
                  }}
                  className="bg-slate-800 border-slate-700 text-white"
                >
                  去商户端登录
                </Button>
              </Space>
            </div>
          </Card>
        )}
      </div>

      {/* 底部版权 */}
      <div className="text-center text-slate-500 text-xs mt-8">
        &copy; 2026 Yunshu CallCenter Rewrite Workspace. All rights reserved. Go-Native Core.
      </div>
    </div>
  )
}
