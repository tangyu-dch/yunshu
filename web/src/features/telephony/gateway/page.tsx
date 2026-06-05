import {
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Space,
  Switch,
  Tag,
  Typography,
  Select,
  message,
  Card,
  Row,
  Col,
  Statistic
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import {
  fetchGatewayPage,
  saveGateway,
  deleteGateways,
  syncGateway,
  fetchChannels,
  fetchRates
} from '@/api/operate'
import {
  ReloadOutlined,
  PlusOutlined,
  SyncOutlined,
  EditOutlined,
  DeleteOutlined,
  HddOutlined,
  SafetyOutlined,
  SlidersOutlined
} from '@ant-design/icons'

type GatewayFormValues = {
  id?: number
  name: string
  description: string
  channelId?: number
  concurrency?: number
  model?: number
  username?: string
  password?: string
  realm?: string
  port?: string
  priority?: number
  remark?: string
  codecPrefs?: string
  rateId?: number
  enable: boolean
}

// 优先级颜色映射
const getPriorityTag = (p?: number) => {
  const val = p ?? 1
  if (val === 1) return <Tag color="gold" className="font-medium">主用 (首选)</Tag>
  if (val === 2) return <Tag color="cyan" className="font-medium">备用一</Tag>
  if (val === 3) return <Tag color="blue" className="font-medium">备用二</Tag>
  return <Tag color="default" className="font-medium">备用 {val}</Tag>
}

export function GatewayPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<GatewayFormValues>()
  const queryClient = useQueryClient()

  // Queries
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['operate', 'gateway', pageNumber, pageSize],
    queryFn: () => fetchGatewayPage(pageNumber, pageSize),
  })
  
  const { data: channelsData } = useQuery({
    queryKey: ['operate', 'channels', 1, 100],
    queryFn: () => fetchChannels(1, 100),
  })

  const { data: ratesData } = useQuery({
    queryKey: ['operate', 'rates', 1, 100],
    queryFn: () => fetchRates(1, 100),
  })

  // Mutations
  const syncMutation = useMutation({
    mutationFn: async (id: number) => syncGateway(id),
    onSuccess: () => {
      message.success('网关已成功同步至软交换运行态')
      queryClient.invalidateQueries({ queryKey: ['operate', 'gateway'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '同步失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => deleteGateways([id]),
    onSuccess: () => {
      message.success('网关已删除')
      queryClient.invalidateQueries({ queryKey: ['operate', 'gateway'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: GatewayFormValues) => saveGateway(values),
    onSuccess: () => {
      message.success(editingId ? '网关已更新' : '网关已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['operate', 'gateway'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({
        enable: true,
        concurrency: 100,
        priority: 1,
        port: '5060',
        codecPrefs: 'PCMU,PCMA',
        model: 1,
      })
    }, 0)
  }

  function openEdit(record: any) {
    setEditingId(record.id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id: record.id,
        name: record.name ?? '',
        description: record.description ?? '',
        concurrency: record.concurrency ?? 100,
        enable: record.enable ?? true,
        priority: record.priority ?? 1,
        codecPrefs: record.codecPrefs ?? 'PCMU,PCMA',
        channelId: record.channelId,
        rateId: record.rateId,
        realm: record.realm ?? '',
        port: record.port ?? '',
        username: record.username ?? '',
        password: '',
        remark: record.remark ?? '',
      })
    }, 0)
  }

  function getChannelName(cId: any) {
    const found = channelsData?.records.find((c: any) => String(c.id) === String(cId))
    return found ? found.name : `渠道/线路 ${cId || '未配置'}`
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-end items-center mb-2">
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>
            刷新
          </Button>
          <PermissionGate permission="operate:gateway:write">
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增网关
            </Button>
          </PermissionGate>
        </Space>
      </div>

      {/* 运营看板数据指标 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={8}>
          <Card variant="borderless" className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Statistic
              title="配置网关线路总数"
              value={data?.total ?? 0}
              prefix={<HddOutlined className="text-blue-500 mr-1" />}
              suffix="条"
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card variant="borderless" className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Statistic
              title="已启用正常中继"
              value={(data?.records ?? []).filter((r) => r.enable).length}
              prefix={<SafetyOutlined className="text-emerald-500 mr-1" />}
              valueStyle={{ color: '#3f8600' }}
              suffix="条"
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card variant="borderless" className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Statistic
              title="总承载并发上限"
              value={(data?.records ?? []).reduce((sum, r) => sum + (r.concurrency ?? 0), 0)}
              prefix={<SlidersOutlined className="text-purple-500 mr-1" />}
              valueStyle={{ color: '#096dd9' }}
              suffix="线"
            />
          </Card>
        </Col>
      </Row>

      <TableWrap
        title="运营商 SIP 中继配置列表"
        rowKey="id"
        loading={isLoading}
        dataSource={data?.records ?? []}
        pagination={{
          current: pageNumber,
          pageSize,
          total: data?.total ?? 0,
          onChange: (current, size) => {
            setPageNumber(current)
            setPageSize(size ?? pageSize)
          },
          showSizeChanger: true,
        }}
        columns={[
          {
            title: '网关名称',
            dataIndex: 'name',
            render: (v) => <span className="font-semibold text-slate-800 dark:text-slate-200">{v}</span>
          },
          { title: '网关描述/编码', dataIndex: 'code', render: (v) => <span className="text-slate-500 text-xs">{v}</span> },
          {
            title: '中继服务器地址',
            key: 'sipServer',
            render: (_, record: any) => (
              record.realm ? (
                <span className="font-mono text-xs text-slate-600 dark:text-slate-300">
                  {record.realm}:{record.port || '5060'}
                </span>
              ) : (
                <span className="text-slate-400 text-xs italic">未配置</span>
              )
            )
          },
          {
            title: '并发限制 (CC)',
            dataIndex: 'concurrency',
            width: 110,
            render: (v) => <Tag color="blue" className="font-mono">{v ?? 100} 线</Tag>
          },
          {
            title: '调度优先级',
            dataIndex: 'priority',
            width: 120,
            render: (v) => getPriorityTag(v)
          },
          {
            title: '编解码偏好',
            dataIndex: 'codecPrefs',
            render: (v) => {
              const codecs = (v || 'PCMU,PCMA').split(',')
              return (
                <Space size={4} wrap>
                  {codecs.map((codec: string) => (
                    <Tag key={codec} color="geekblue" className="text-[10px] uppercase font-mono px-1">
                      {codec.trim()}
                    </Tag>
                  ))}
                </Space>
              )
            }
          },
          {
            title: '归属渠道',
            key: 'channel',
            render: (_, record) => <Tag color="cyan">{getChannelName(record.channelId)}</Tag>
          },
          {
            title: '启用状态',
            dataIndex: 'enable',
            width: 100,
            render: (value: boolean) => (
              <Tag color={value ? 'success' : 'default'} className="font-medium">
                {value ? '已启用' : '已停用'}
              </Tag>
            ),
          },
          {
            title: '操作面板',
            width: 220,
            render: (_, record) => (
              <Space size="middle" className="text-xs">
                <PermissionGate permission="operate:gateway:write">
                  <Button size="small" type="link" icon={<EditOutlined />} onClick={() => openEdit(record)} className="!p-0">
                    编辑
                  </Button>
                </PermissionGate>
                <PermissionGate permission="operate:gateway:sync">
                  <Button size="small" type="link" icon={<SyncOutlined />} onClick={() => syncMutation.mutate(record.id)} className="!p-0">
                    配置同步
                  </Button>
                </PermissionGate>
                <PermissionGate permission="operate:gateway:delete">
                  <Popconfirm title="确认删除该网关线路？" onConfirm={() => deleteMutation.mutate(record.id)}>
                    <Button size="small" type="link" danger icon={<DeleteOutlined />} className="!p-0">
                      删除
                    </Button>
                  </Popconfirm>
                </PermissionGate>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        open={open}
        title={editingId ? '编辑 SIP 网关配置' : '新增 SIP 网关配置'}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnHidden
        width={720}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => saveMutation.mutate(values)}
        >
          <Form.Item name="id" hidden>
            <Input />
          </Form.Item>

          <Typography.Title level={5} className="!mb-4 border-b pb-2 dark:border-slate-800 flex items-center gap-1.5 text-slate-700 dark:text-slate-300">
            <SlidersOutlined /> 网关基础信息
          </Typography.Title>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="name"
                label="网关名称"
                rules={[{ required: true, message: '请输入网关名称' }]}
              >
                <Input placeholder="例如: GW-TELECOM-01 (仅限英文和数字)" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="description"
                label="网关描述"
                rules={[{ required: true, message: '请输入网关描述' }]}
              >
                <Input placeholder="例如: 华东电信SIP中继" />
              </Form.Item>
            </Col>
          </Row>

          <Typography.Title level={5} className="!mb-4 border-b pb-2 dark:border-slate-800 flex items-center gap-1.5 text-slate-700 dark:text-slate-300">
            <HddOutlined /> SIP 服务器及安全鉴权 (SIP Endpoint)
          </Typography.Title>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="realm" label="SIP 域 (Realm / Server IP)">
                <Input placeholder="例如: sip.provider.com 或 192.168.1.50" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="port" label="SIP 端口">
                <Input placeholder="默认: 5060" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="username" label="注册账号 / Username (可选)">
                <Input placeholder="非注册中继/对等IP模式可留空" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="password" label="注册密码 / Password (可选)">
                <Input.Password placeholder="非注册中继/对等IP模式可留空" />
              </Form.Item>
            </Col>
          </Row>

          <Typography.Title level={5} className="!mb-4 border-b pb-2 dark:border-slate-800 flex items-center gap-1.5 text-slate-700 dark:text-slate-300">
            <SlidersOutlined /> 通讯流控与计费属性
          </Typography.Title>
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="concurrency" label="最大并发线数 (CC)" rules={[{ required: true, message: '请配置最大并发' }]}>
                <InputNumber min={1} className="w-full" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="priority" label="调度优先级 (数字越小越优先)" rules={[{ required: true, message: '请配置调度优先级' }]}>
                <InputNumber min={1} className="w-full" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="enable" label="启用状态" valuePropName="checked">
                <Switch checkedChildren="开启" unCheckedChildren="关闭" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="channelId" label="绑定物理渠道" rules={[{ required: true, message: '请选择绑定渠道' }]}>
                <Select
                  placeholder="请选择渠道模型"
                  allowClear
                  options={channelsData?.records.map((c: any) => ({ label: c.name, value: c.id })) ?? []}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="rateId" label="结算费率模板" rules={[{ required: true, message: '请选择关联费率' }]}>
                <Select
                  placeholder="请选择收费费率"
                  allowClear
                  options={ratesData?.records.map((r: any) => ({ label: r.name, value: r.id })) ?? []}
                />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="codecPrefs" label="编解码协议偏好偏序 (Codec Preferences)" help="用英文逗号隔开，首个代表最优先商定">
            <Input placeholder="例如: PCMU,PCMA,G729,G722" />
          </Form.Item>

          <Form.Item name="remark" label="运维备忘录">
            <Input.TextArea placeholder="写入网关维护、运营商联系方式或物理专线备忘信息" rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
