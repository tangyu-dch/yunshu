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
  message,
  Drawer,
  Table,
  Statistic,
  Row,
  Col,
  Progress,
  Card
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { useAuthStore } from '@/store/auth'
import {
  fetchBatchTasks,
  saveBatchTask,
  deleteBatchTasks,
  toggleBatchTaskEnable,
  importBatchTaskTels,
  fetchBatchTaskDetails
} from '@/api/operate'
import {
  ReloadOutlined,
  DeleteOutlined,
  PlusOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  ImportOutlined,
  ProfileOutlined,
  CheckCircleOutlined,
  InfoCircleOutlined
} from '@ant-design/icons'

type BatchTaskFormValues = {
  id?: number
  name: string
  merchantId: number
  userId: number
  connectedInterval: number
  unconnectedInterval: number
  callTimePeriod?: string
  aiFlag: boolean
  enable: boolean
}

export function BatchTaskPage() {
  const tenant = useAuthStore((state) => state.tenant)
  const merchantId = Number(tenant?.merchantId || '1001')
  const userId = Number(tenant?.userId || '1')

  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  // Import Numbers Modal State
  const [importTaskId, setImportTaskId] = useState<number | null>(null)
  const [importForm] = Form.useForm()

  // Details Drawer State
  const [detailTaskId, setDetailTaskId] = useState<number | null>(null)

  const [form] = Form.useForm<BatchTaskFormValues>()
  const queryClient = useQueryClient()

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['merchant', 'batch-task', pageNumber, pageSize, merchantId],
    queryFn: () => fetchBatchTasks(pageNumber, pageSize),
  })

  const queryFields = useMemo(() => [
    { key: 'name', label: '任务名称', type: 'text' as const, placeholder: '请输入外呼任务名称模糊搜索' },
    {
      key: 'status',
      label: '任务状态',
      type: 'select' as const,
      options: [
        { value: 'running', label: '运行中' },
        { value: 'paused', label: '暂停中' },
        { value: 'completed', label: '已完成' },
      ],
    },
  ], [])

  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.name) {
      records = records.filter((r: any) => String(r.name).toLowerCase().includes(queryParams.name.toLowerCase().trim()))
    }
    if (queryParams.status) {
      records = records.filter((r: any) => r.status === queryParams.status)
    }
    return records
  }, [data, queryParams])

  // Fetch detailed dial logs of selected task
  const { data: detailsData, isLoading: detailsLoading } = useQuery({
    queryKey: ['merchant', 'batch-task-details', detailTaskId],
    queryFn: () => fetchBatchTaskDetails(detailTaskId!),
    enabled: !!detailTaskId,
  })

  const saveMutation = useMutation({
    mutationFn: async (values: BatchTaskFormValues) =>
      saveBatchTask({
        id: editingId ?? undefined,
        name: values.name,
        merchantId,
        userId,
        connectedInterval: values.connectedInterval ?? 600,
        unconnectedInterval: values.unconnectedInterval ?? 1200,
        callTimePeriod: values.callTimePeriod || '09:00-12:00,14:00-18:00',
        aiFlag: Boolean(values.aiFlag),
        enable: Boolean(values.enable),
      }),
    onSuccess: async () => {
      message.success(editingId ? '外呼任务已更新' : '外呼任务已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-task'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteBatchTasks(ids),
    onSuccess: async () => {
      message.success('外呼任务已删除')
      setSelectedIds([])
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-task'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const toggleEnableMutation = useMutation({
    mutationFn: async ({ id, enable }: { id: number; enable: boolean }) =>
      toggleBatchTaskEnable(id, enable, enable ? '' : '手动暂停'),
    onSuccess: async () => {
      message.success('任务状态已更新')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-task'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '状态更新失败'),
  })

  const importTelsMutation = useMutation({
    mutationFn: async (values: { tels: string }) => {
      const telList = values.tels
        .split(/[\n,]/)
        .map((t) => t.trim())
        .filter((t) => t.length > 0)
      if (telList.length === 0) throw new Error('请输入有效的号码列表')
      return importBatchTaskTels(importTaskId!, merchantId, userId, telList)
    },
    onSuccess: async (res) => {
      message.success(`成功导入 ${res.imported ?? 0} 个号码`)
      setImportTaskId(null)
      importForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-task'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '导入号码失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({
        connectedInterval: 600,
        unconnectedInterval: 1200,
        callTimePeriod: '09:00-12:00,14:00-18:00',
        aiFlag: false,
        enable: true,
      })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        name: record?.name ?? '',
        connectedInterval: record?.connectedInterval ?? 600,
        unconnectedInterval: record?.unconnectedInterval ?? 1200,
        callTimePeriod: record?.callTimePeriod ?? '09:00-12:00,14:00-18:00',
        aiFlag: record?.aiFlag ?? false,
        enable: record?.status === 'running',
      })
    }, 0)
  }

  // Pre-calculate statistics for the current details Drawer
  const getDrawerStats = () => {
    if (!detailsData || detailsData.length === 0) {
      return { total: 0, called: 0, connected: 0, rate: 0, gt30s: 0, gt30sRate: 0, active: 0, calling: 0 }
    }
    const total = detailsData.length
    let called = 0
    let connected = 0
    let gt30s = 0
    let calling = 0
    let active = 0

    detailsData.forEach((item: any) => {
      if (item.callStatus === 2) {
        calling++
      }
      if (item.callStatus === 3) {
        called++
        if (item.connectStatus === true) {
          connected++
          if (item.durationSec && item.durationSec > 30) {
            gt30s++
          }
        }
      } else {
        active++
      }
    })

    const rate = called > 0 ? Math.round((connected / called) * 100) : 0
    const gt30sRate = called > 0 ? Math.round((gt30s / called) * 100) : 0

    return { total, called, connected, rate, gt30s, gt30sRate, active, calling }
  }

  const drawerStats = getDrawerStats()

  return (
    <Space direction="vertical" size="large" className="w-full">
      <QueryBar
        fields={queryFields}
        onSearch={setQueryParams}
        loading={isLoading}
      />

      <div className="flex justify-end mb-2">
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>刷新</Button>
          {selectedIds.length > 0 && (
            <PermissionGate permission="merchant:batch-task:write">
              <Popconfirm
                title={`确定要删除选中的 ${selectedIds.length} 个任务吗？`}
                onConfirm={() => deleteMutation.mutate(selectedIds)}
                okText="确定"
                cancelText="取消"
              >
                <Button danger icon={<DeleteOutlined />}>批量删除</Button>
              </Popconfirm>
            </PermissionGate>
          )}
          <PermissionGate permission="merchant:batch-task:write">
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建任务
            </Button>
          </PermissionGate>
        </Space>
      </div>

      <TableWrap
        title="外呼活动任务列表"
        rowKey="id"
        loading={isLoading}
        dataSource={filteredRecords}
        rowSelection={{
          selectedRowKeys: selectedIds,
          onChange: (keys: any[]) => setSelectedIds(keys as number[]),
        }}
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
          { title: '任务 ID', dataIndex: 'id', width: 90, className: 'font-mono text-xs' },
          { title: '任务名称', dataIndex: 'name', render: (v) => <span className="font-semibold text-slate-700 dark:text-slate-200">{v}</span> },
          { title: '所属商户', dataIndex: 'merchant', render: (v) => <Tag color="blue">{v}</Tag> },
          {
            title: '号码池规模',
            key: 'size',
            render: (_, record) => (
              <Space direction="vertical" size={2} className="text-xs">
                <div>总号码: <span className="font-semibold">{record.total}</span></div>
                <div>已呼叫: <span className="font-mono text-slate-500">{record.completed}</span></div>
              </Space>
            )
          },
          {
            title: '接通情况',
            key: 'connects',
            render: (_, record) => {
              const rate = record.completed > 0 ? Math.round((record.connected / record.completed) * 100) : 0
              return (
                <Space direction="vertical" size={2} className="text-xs">
                  <div>接通数: <Typography.Text type="success" strong>{record.connected}</Typography.Text></div>
                  <div>接通率: <Tag color={rate > 40 ? 'green' : 'orange'}>{rate}%</Tag></div>
                </Space>
              )
            }
          },
          {
            title: '进度比例',
            key: 'progress',
            width: 160,
            render: (_, record) => {
              const pct = record.total > 0 ? Math.round((record.completed / record.total) * 100) : 0
              return (
                <div className="w-full pr-4">
                  <Progress percent={pct} size="small" status={record.status === 'running' ? 'active' : 'normal'} />
                </div>
              )
            }
          },
          {
            title: '任务状态',
            dataIndex: 'status',
            width: 110,
            render: (value: string) => {
              let color = 'default'
              let label = '未开始'
              if (value === 'running') {
                color = 'success'
                label = '运行中'
              } else if (value === 'paused') {
                color = 'warning'
                label = '已暂停'
              } else if (value === 'completed') {
                color = 'processing'
                label = '已完成'
              }
              return <Tag color={color} className="font-medium">{label}</Tag>
            },
          },
          {
            title: '操作面板',
            key: 'actions',
            width: 320,
            render: (_, record) => {
              const isRunning = record.status === 'running'
              return (
                <Space size="middle" className="text-xs">
                  <PermissionGate permission="merchant:batch-task:write">
                    <Button
                      type="link"
                      icon={isRunning ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
                      onClick={() => toggleEnableMutation.mutate({ id: record.id, enable: !isRunning })}
                      className="!p-0 flex items-center"
                    >
                      {isRunning ? '暂停' : '启动'}
                    </Button>
                    <Button
                      type="link"
                      icon={<ImportOutlined />}
                      onClick={() => setImportTaskId(record.id)}
                      className="!p-0 flex items-center"
                    >
                      导入号码
                    </Button>
                    <Button
                      type="link"
                      icon={<ProfileOutlined />}
                      onClick={() => setDetailTaskId(record.id)}
                      className="!p-0 flex items-center"
                    >
                      拨打明细
                    </Button>
                    <Button type="link" onClick={() => openEdit(record.id)} className="!p-0">
                      编辑
                    </Button>
                    <Popconfirm
                      title="确定要删除该外呼任务吗？"
                      onConfirm={() => deleteMutation.mutate([record.id])}
                      okText="确定"
                      cancelText="取消"
                    >
                      <Button type="link" danger className="!p-0">
                        删除
                      </Button>
                    </Popconfirm>
                  </PermissionGate>
                </Space>
              )
            },
          },
        ]}
      />

      <Modal
        title={editingId ? '编辑外呼任务' : '新建外呼任务'}
        open={open}
        width={640}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)}>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-1">
            <Form.Item
              name="name"
              label="任务名称"
              rules={[{ required: true, message: '请输入外呼任务名称' }]}
              className="col-span-1 md:col-span-2"
            >
              <Input placeholder="例如: 优质客户回访任务" />
            </Form.Item>

            <Form.Item
              name="connectedInterval"
              label="接通重试间隔 (秒)"
              rules={[{ required: true, message: '请输入接通重试间隔' }]}
            >
              <InputNumber min={60} className="w-full" placeholder="默认 600 秒" />
            </Form.Item>

            <Form.Item
              name="unconnectedInterval"
              label="未接通重试间隔 (秒)"
              rules={[{ required: true, message: '请输入未接通重试间隔' }]}
            >
              <InputNumber min={60} className="w-full" placeholder="默认 1200 秒" />
            </Form.Item>

            <Form.Item
              name="callTimePeriod"
              label="允许呼叫时段"
              help="格式如: 09:00-12:00,14:00-18:00"
              className="col-span-1 md:col-span-2"
            >
              <Input placeholder="09:00-12:00,14:00-18:00" />
            </Form.Item>

            <Form.Item name="aiFlag" label="启用 AI 智能交互" valuePropName="checked" className="flex items-center">
              <Switch checkedChildren="开" unCheckedChildren="关" />
            </Form.Item>

            <Form.Item name="enable" label="启用状态" valuePropName="checked" className="flex items-center">
              <Switch checkedChildren="启动" unCheckedChildren="挂起" />
            </Form.Item>
          </div>
        </Form>
      </Modal>

      {/* Import Numbers Modal */}
      <Modal
        title="导入外呼号码"
        open={!!importTaskId}
        onCancel={() => {
          setImportTaskId(null)
          importForm.resetFields()
        }}
        onOk={() => importForm.submit()}
        confirmLoading={importTelsMutation.isPending}
        destroyOnClose
      >
        <Form form={importForm} layout="vertical" onFinish={(values) => importTelsMutation.mutate(values)}>
          <Form.Item
            name="tels"
            label="号码清单"
            rules={[{ required: true, message: '请输入要导入的号码列表' }]}
            help="输入电话号码，每行一个或用逗号隔开。例: 13800000001,13800000002"
          >
            <Input.TextArea rows={8} placeholder="13800000001&#10;13900000002&#10;15911112222" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Dial Details Drawer */}
      <Drawer
        title="任务拨号及呼叫接通明细"
        placement="right"
        width={850}
        onClose={() => setDetailTaskId(null)}
        open={!!detailTaskId}
        destroyOnClose
      >
        {detailsLoading ? (
          <div className="py-20 text-center text-slate-500">正在读取拨打明细...</div>
        ) : (
          <Space direction="vertical" size="large" className="w-full">
            <Card className="bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800 shadow-sm rounded-lg">
              <Row gutter={16}>
                <Col span={6}>
                  <Statistic title="分配总数" value={drawerStats.total} suffix="个" />
                </Col>
                <Col span={6}>
                  <Statistic title="已拨打" value={drawerStats.called} suffix="次" />
                </Col>
                <Col span={6}>
                  <Statistic
                    title="接通率"
                    value={drawerStats.rate}
                    suffix="%"
                    valueStyle={{ color: drawerStats.rate > 40 ? '#3f8600' : '#cf1322' }}
                  />
                </Col>
                <Col span={6}>
                  <Statistic
                    title="优质通话 (>30s)"
                    value={drawerStats.gt30s}
                    suffix={`次 (${drawerStats.gt30sRate}%)`}
                    valueStyle={{ color: '#096dd9' }}
                  />
                </Col>
              </Row>
              <div className="mt-4 flex gap-4 text-xs">
                <Tag icon={<InfoCircleOutlined />} color="default">待拨打: {drawerStats.active} 个</Tag>
                <Tag icon={<InfoCircleOutlined />} color="processing">呼叫中: {drawerStats.calling} 个</Tag>
                <Tag icon={<CheckCircleOutlined />} color="success">接通完成: {drawerStats.connected} 个</Tag>
              </div>
            </Card>

            <Table
              dataSource={detailsData ?? []}
              rowKey="id"
              pagination={{ pageSize: 15 }}
              columns={[
                { title: '号码', dataIndex: 'tel', className: 'font-mono text-xs' },
                {
                  title: '拨打状态',
                  dataIndex: 'callStatus',
                  render: (value: number) => {
                    if (value === 2) return <Tag color="blue">呼叫中</Tag>
                    if (value === 3) return <Tag color="green">已拨打</Tag>
                    return <Tag color="default">待拨打</Tag>
                  }
                },
                {
                  title: '是否接通',
                  dataIndex: 'connectStatus',
                  render: (value: boolean | null) => {
                    if (value === true) return <Tag color="success">已接通</Tag>
                    if (value === false) return <Tag color="error">未接通</Tag>
                    return <Tag color="default">-</Tag>
                  }
                },
                {
                  title: '通话时长',
                  dataIndex: 'durationSec',
                  render: (value: number) => {
                    if (!value) return '-'
                    return (
                      <span className={value > 30 ? 'font-bold text-blue-600 dark:text-blue-400' : ''}>
                        {value} 秒 {value > 30 && <BadgeText>优质</BadgeText>}
                      </span>
                    )
                  }
                },
                {
                  title: '呼叫时间',
                  dataIndex: 'callTime',
                  render: (value: string) => value ? value.replace('T', ' ').slice(0, 19) : '-'
                }
              ]}
            />
          </Space>
        )}
      </Drawer>
    </Space>
  )
}

function BadgeText({ children }: { children: React.ReactNode }) {
  return (
    <span className="ml-1 inline-flex items-center rounded-full bg-blue-100 dark:bg-blue-900 px-2 py-0.5 text-xs font-medium text-blue-800 dark:text-blue-100">
      {children}
    </span>
  )
}
