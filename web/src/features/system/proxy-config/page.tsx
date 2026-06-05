import {
  Button,
  Form,
  Input,
  InputNumber,
  Space,
  Typography,
  message,
  Card,
  Row,
  Col,
  Alert,
  Spin,
  Tag,
  Modal,
  Switch
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import {
  fetchProxyConfig,
  saveProxyConfig,
  applyProxyConfig
} from '@/api/operate'
import {
  SettingOutlined,
  SaveOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
  GlobalOutlined
} from '@ant-design/icons'
import { TableWrap } from '@/components/TableWrap'

type ProxyConfigValues = {
  kamailioUdpIp: string
  kamailioTcpIp: string
  kamailioSipPort: number
  kamailioWsPort: number
  kamailioExternalIp: string
  kamailioStatus?: 'online' | 'offline'
  sipTraceEnable?: boolean
}

export function ProxyConfigPage() {
  const queryClient = useQueryClient()
  const [form] = Form.useForm<ProxyConfigValues>()
  const [isApplying, setIsApplying] = useState(false)
  const [operationStatus, setOperationStatus] = useState<{ type: 'success' | 'error' | 'info'; message: string; detail?: string } | null>(null)
  const [openEditModal, setOpenEditModal] = useState(false)

  // 获取当前配置
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['operate', 'proxy-config'],
    queryFn: () => fetchProxyConfig(),
    refetchInterval: 4000, // 每 4 秒定时自动轮询，保证掉线在 5 秒内实时反馈
  })

  // 保存配置变动至数据库
  const saveMutation = useMutation({
    mutationFn: async (values: ProxyConfigValues) => saveProxyConfig(values),
    onSuccess: () => {
      message.success('信令代理核心配置已成功保存至数据库')
      setOperationStatus({
        type: 'success',
        message: '配置保存成功',
        detail: '最新的 Kamailio 信令网络参数已成功持久化至数据库。配置在服务应用重启前仅保存在数据库中，必须点击“应用并重启容器”来刷新在线运行态。'
      })
      queryClient.invalidateQueries({ queryKey: ['operate', 'proxy-config'] })
      setOpenEditModal(false)
    },
    onError: (error) => {
      const errMsg = error instanceof Error ? error.message : '保存配置失败'
      message.error(errMsg)
      setOperationStatus({
        type: 'error',
        message: '配置保存失败',
        detail: errMsg
      })
    }
  })

  // 应用变动并重启 Docker 容器
  const applyMutation = useMutation({
    mutationFn: async () => applyProxyConfig(),
    onMutate: () => setIsApplying(true),
    onSuccess: (res: any) => {
      message.success('信令代理容器拉起应用触发成功')
      setOperationStatus({
        type: 'success',
        message: '信令代理服务器重启成功',
        detail: typeof res === 'string' ? res : JSON.stringify(res) || '配置应用成功，正在重新拉起信令代理服务器，请稍后刷新监控'
      })
      setIsApplying(false)
      setOpenEditModal(false)
    },
    onError: (error) => {
      const errMsg = error instanceof Error ? error.message : '应用并重启失败'
      message.error(errMsg)
      setOperationStatus({
        type: 'error',
        message: '应用并重启失败',
        detail: errMsg
      })
      setIsApplying(false)
    }
  })

  const handleEditOpen = () => {
    setOpenEditModal(true)
    if (data) {
      form.setFieldsValue(data)
    }
  }

  // 构造单节点信令核心Table数据列表，符合“信令节点在列表中各自展示”要求
  const tableData = data ? [
    {
      key: 'kamailio-core',
      name: 'Kamailio 信令核心',
      udpIp: data.kamailioUdpIp ?? '0.0.0.0',
      tcpIp: data.kamailioTcpIp ?? '0.0.0.0',
      sipPort: data.kamailioSipPort ?? 5060,
      wsPort: data.kamailioWsPort ?? 5066,
      externalIp: data.kamailioExternalIp ?? '127.0.0.1',
      status: data.kamailioStatus ?? 'offline'
    }
  ] : []

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-96">
        <Spin size="large" tip="正在读取系统信令代理网络配置..." />
      </div>
    )
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 全局高科技信令容器重组遮罩 */}
      {(applyMutation.isPending || isApplying) && (
        <div className="fixed inset-0 bg-slate-900/60 dark:bg-black/80 backdrop-blur-md z-[9999] flex flex-col items-center justify-center space-y-6">
          <div className="relative flex items-center justify-center">
            <Spin size="large" />
            <div className="absolute w-24 h-24 rounded-full border-4 border-indigo-500/20 border-t-indigo-500 animate-spin" style={{ animationDuration: '1.5s' }} />
          </div>
          <div className="flex flex-col items-center space-y-2 text-center max-w-md px-4">
            <div className="text-white dark:text-zinc-100 font-bold text-base tracking-wider font-mono">
              🚀 CORE PROXIES RECONSTRUCTING
            </div>
            <div className="text-indigo-200 dark:text-zinc-400 text-xs font-mono leading-relaxed">
              正在安全重启信令核心网关（Kamailio）容器。此操作将进行网络端口重分配与服务刷新，持续约 3-5 秒，请勿刷新或关闭页面...
            </div>
          </div>
        </div>
      )}

      <div className="flex justify-between items-center mb-2">
        <div>
          <Typography.Title level={4} className="!mb-1 font-bold text-slate-800 dark:text-slate-200">
            代理配置管理
          </Typography.Title>
        </div>
        <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>
          刷新状态
        </Button>
      </div>

      {operationStatus && (
        <Alert
          message={operationStatus.type === 'success' ? '系统执行反馈 - 操作成功' : '系统执行反馈 - 操作失败'}
          description={
            <div className="flex flex-col gap-2">
              <div className="font-semibold text-slate-800 dark:text-zinc-100">{operationStatus.message}</div>
              {operationStatus.detail && (
                <div className="p-3.5 rounded-lg bg-black/5 dark:bg-black/20 font-mono text-xs text-slate-700 dark:text-zinc-300 border border-slate-200/50 dark:border-zinc-880 whitespace-pre-wrap leading-relaxed">
                  {operationStatus.detail}
                </div>
              )}
            </div>
          }
          type={operationStatus.type}
          showIcon
          closable
          onClose={() => setOperationStatus(null)}
          className="rounded-lg shadow-sm border border-slate-200 dark:border-zinc-850 animate-fade-in"
        />
      )}

      {/* 实时健康度卡片看板 */}
      <div className="grid grid-cols-1 gap-4 mb-2 animate-fade-in">
        <Card variant="borderless" className="shadow-sm rounded-xl bg-gradient-to-br from-slate-50 to-slate-100/50 dark:from-zinc-900/60 dark:to-zinc-950/40 border border-slate-100 dark:border-zinc-850 p-4 transition-all duration-300">
          <div className="flex justify-between items-start">
            <div>
              <div className="text-[10px] font-bold text-slate-400 dark:text-zinc-500 uppercase font-mono tracking-wider">SIP SIGNAL PROXY SERVICE</div>
              <div className="text-sm font-bold text-slate-800 dark:text-zinc-100 mt-1 flex items-center gap-1.5">
                <span className={`w-2 h-2 rounded-full ${data?.kamailioStatus === 'online' ? 'bg-emerald-500 animate-pulse' : 'bg-rose-500 animate-ping'} inline-block`} />
                Kamailio 信令代理核心
              </div>
              <div className="text-[11px] text-slate-500 dark:text-zinc-400 mt-2 font-mono space-y-0.5">
                <div>SIP 监听端口: <span className="font-semibold text-slate-700 dark:text-zinc-200">{data?.kamailioSipPort ?? 5060}</span></div>
                <div>WS 监听端口: <span className="font-semibold text-slate-700 dark:text-zinc-200">{data?.kamailioWsPort ?? 5066}</span></div>
                <div>SIP 信令追踪: <span className="font-semibold text-slate-700 dark:text-zinc-200">{data?.sipTraceEnable ? '已开启 (ON)' : '已关闭 (OFF)'}</span></div>
              </div>
            </div>
            {data?.kamailioStatus === 'online' ? (
              <Tag color="success" style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }} className="font-mono">HEALTHY</Tag>
            ) : (
              <Tag color="error" style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }} className="font-mono animate-pulse">OFFLINE</Tag>
            )}
          </div>
        </Card>
      </div>

      {/* 信令代理节点 Table 列表 */}
      <TableWrap
        title="信令代理节点列表"
        rowKey="key"
        dataSource={tableData}
        loading={isLoading}
        columns={[
          {
            title: '名称',
            dataIndex: 'name',
            render: (val) => <span className="font-bold text-slate-800 dark:text-zinc-100">{val}</span>
          },
          {
            title: '监听 IP (UDP/TCP)',
            render: (_, r) => <span className="font-mono text-xs">UDP: {r.udpIp} / TCP: {r.tcpIp}</span>
          },
          {
            title: '对外映射公网 IP',
            dataIndex: 'externalIp',
            render: (val) => <span className="font-mono text-xs font-semibold text-blue-600 dark:text-blue-400">{val}</span>
          },
          {
            title: '监听端口 (SIP/WS)',
            render: (_, r) => <span className="font-mono text-xs">SIP: {r.sipPort} / WS: {r.wsPort}</span>
          },
          {
            title: '状态',
            dataIndex: 'status',
            render: (val) => {
              if (val === 'online') {
                return (
                  <Tag color="success" style={{ border: 'none' }} className="flex items-center w-fit gap-1 font-semibold text-emerald-600">
                    <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse inline-block" />
                    在线启用
                  </Tag>
                )
              }
              return (
                <Tag color="error" style={{ border: 'none' }} className="flex items-center w-fit gap-1 font-semibold text-rose-600 animate-pulse">
                  <span className="w-1.5 h-1.5 rounded-full bg-rose-500 animate-ping inline-block" />
                  故障离线
                </Tag>
              )
            }
          },
          {
            title: '操作',
            render: () => (
              <Button size="small" onClick={handleEditOpen}>
                编辑核心网络参数
              </Button>
            )
          }
        ]}
      />

      {/* 核心网络配置 Modal 表单 */}
      <Modal
        open={openEditModal}
        title="配置 Kamailio 信令代理核心参数"
        onCancel={() => setOpenEditModal(false)}
        width={650}
        destroyOnHidden
        footer={[
          <Button key="cancel" onClick={() => setOpenEditModal(false)}>
            取消
          </Button>,
          <Button
            key="saveOnly"
            icon={<SaveOutlined />}
            loading={saveMutation.isPending}
            onClick={async () => {
              try {
                const values = await form.validateFields()
                saveMutation.mutate(values)
              } catch (e) {
                message.error('请先修正表单验证项错误')
              }
            }}
          >
            仅保存至数据库
          </Button>,
          <Button
            key="saveAndApply"
            type="primary"
            danger
            icon={<ThunderboltOutlined />}
            loading={saveMutation.isPending || applyMutation.isPending || isApplying}
            onClick={async () => {
              try {
                const values = await form.validateFields()
                // 先保存
                await saveMutation.mutateAsync(values)
                // 后重启
                applyMutation.mutate()
              } catch (e) {
                message.error('请先修正表单验证项错误')
              }
            }}
          >
            保存并应用重启 (Docker)
          </Button>
        ]}
      >
        <Form
          form={form}
          layout="vertical"
          disabled={isApplying}
          className="pt-4"
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="kamailioUdpIp"
                label="UDP 监听 IP"
                rules={[{ required: true, message: '请输入 UDP 监听 IP' }]}
              >
                <Input placeholder="默认 0.0.0.0" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="kamailioTcpIp"
                label="TCP 监听 IP"
                rules={[{ required: true, message: '请输入 TCP 监听 IP' }]}
              >
                <Input placeholder="默认 0.0.0.0" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="kamailioSipPort"
                label="SIP 端口 (UDP/TCP)"
                rules={[{ required: true, message: '请输入 SIP 端口' }]}
              >
                <InputNumber min={1} max={65535} className="w-full" placeholder="默认 5060" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="kamailioWsPort"
                label="WebRTC WebSocket 端口"
                rules={[{ required: true, message: '请输入 WebRTC WS 端口' }]}
              >
                <InputNumber min={1} max={65535} className="w-full" placeholder="默认 5066" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item
            name="kamailioExternalIp"
            label="Kamailio 外部映射公网 IP (外网SIP注册与NAT关键)"
            rules={[{ required: true, message: '请输入外部公网 IP' }]}
          >
            <Input placeholder="例如: 203.0.113.5 (如无公网映射可填 127.0.0.1 或内网IP)" />
          </Form.Item>
          <Form.Item
            name="sipTraceEnable"
            label="开启 SIP 信令双时追踪 (SipTrace)"
            valuePropName="checked"
            extra="开启后，呼叫的 SIP 握手信令将实时捕获并记录至 Redis 缓存（仅保留 2 小时），方便在通话记录中渲染交互时序图与 SDP 调试详情。"
          >
            <Switch checkedChildren="开启" unCheckedChildren="关闭" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
