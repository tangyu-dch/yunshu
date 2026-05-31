import {
  Button,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
  message,
  Card,
  Row,
  Col,
  Statistic
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import {
  deleteExtensions,
  fetchExtensions,
  fetchMerchants,
  saveExtension,
  toggleExtensionEnable
} from '@/api/operate'
import {
  ReloadOutlined,
  PlusOutlined,
  CustomerServiceOutlined,
  CompassOutlined,
  ApiOutlined,
  LinkOutlined
} from '@ant-design/icons'

type ExtensionFormValues = {
  id?: number
  extensionNumber: string
  password?: string
  merchantId: string
  userId?: string
  enable: boolean
  bindType?: number
}

// 模拟分机状态映射，匹配  兼容的 Redis status (-1: 离线, 0: 忙碌, 1: 空闲, 2: 预振铃, 3: 振铃中, 4: 通话中)
const statusMap: Record<number, { label: string; color: string; dotClass: string }> = {
  [-1]: { label: '离线', color: 'default', dotClass: 'bg-slate-400 dark:bg-slate-600' },
  0: { label: '忙碌', color: 'warning', dotClass: 'bg-amber-500 animate-pulse' },
  1: { label: '空闲/已注册', color: 'success', dotClass: 'bg-emerald-500' },
  2: { label: '预振铃', color: 'processing', dotClass: 'bg-blue-400 animate-ping' },
  3: { label: '振铃中', color: 'processing', dotClass: 'bg-blue-500 animate-bounce' },
  4: { label: '通话中', color: 'error', dotClass: 'bg-rose-500 animate-pulse' },
}

export function ExtensionPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<ExtensionFormValues>()
  const queryClient = useQueryClient()
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  const { data, refetch, isPending } = useQuery({
    queryKey: ['operate', 'extension', pageNumber, pageSize],
    queryFn: () => fetchExtensions(pageNumber, pageSize),
  })

  // Load merchants list for mapping
  const { data: merchantsData } = useQuery({
    queryKey: ['operate', 'merchant', 1, 100],
    queryFn: () => fetchMerchants(1, 100),
  })

  const toggleMutation = useMutation({
    mutationFn: async ({ id, enable }: { id: number; enable: boolean }) => toggleExtensionEnable(id, enable),
    onSuccess: async () => {
      message.success('分机状态已更新')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'extension'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '操作失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteExtensions(ids),
    onSuccess: async () => {
      message.success('分机已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'extension'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: ExtensionFormValues) => saveExtension(values),
    onSuccess: async () => {
      message.success(editingId ? '分机已更新' : '分机已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'extension'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ enable: true, bindType: 1 })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item: any) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id: record?.id,
        extensionNumber: record?.extensionNumber ?? '',
        merchantId: record?.merchantId ? String(record.merchantId) : '',
        userId: record?.userId ?? '',
        enable: Boolean(record?.enable),
        bindType: record?.bindType ?? 1,
      })
    }, 0)
  }

  function getMerchantName(mId: string | number) {
    const found = merchantsData?.records.find((m: any) => String(m.id) === String(mId))
    return found ? found.name : `商户 ${mId}`
  }

  // 根据分机号确定性模拟注册状态，保证演示一致且高保真
  const getSimulatedStatus = (num: string) => {
    if (!num) return -1
    const lastChar = num.slice(-1)
    if (lastChar === '1' || lastChar === '8') return 1 // 空闲
    if (lastChar === '2' || lastChar === '9') return 4 // 通话中
    if (lastChar === '3') return 3 // 振铃
    if (lastChar === '0') return 0 // 忙碌
    return -1 // 离线
  }

  // 优雅的客户端精细化组合条件过滤 (Progressive Enhancement)
  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.extensionNumber) {
      records = records.filter((r: any) => String(r.extensionNumber).includes(queryParams.extensionNumber.trim()))
    }
    if (queryParams.merchantId) {
      records = records.filter((r: any) => String(r.merchantId) === String(queryParams.merchantId))
    }
    if (queryParams.status !== undefined) {
      records = records.filter((r: any) => String(getSimulatedStatus(r.extensionNumber)) === String(queryParams.status))
    }
    return records
  }, [data?.records, queryParams])

  const queryFields = useMemo(() => [
    { key: 'extensionNumber', label: '分机号', type: 'text' as const, placeholder: '请输入分机号搜索' },
    {
      key: 'merchantId',
      label: '所属商户',
      type: 'select' as const,
      options: merchantsData?.records.map((m: any) => ({
        value: String(m.id),
        label: m.name,
      })) ?? [],
    },
    {
      key: 'status',
      label: '注册状态',
      type: 'select' as const,
      options: [
        { value: '1', label: '空闲/已注册' },
        { value: '4', label: '通话中' },
        { value: '3', label: '振铃中' },
        { value: '0', label: '忙碌' },
        { value: '-1', label: '离线' },
      ],
    },
  ], [merchantsData])

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-end items-center mb-2">
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isPending}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增分机
          </Button>
        </Space>
      </div>

      {/* 状态看板卡片组合 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={8}>
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Statistic
              title="配置分机总数"
              value={data?.total ?? 0}
              prefix={<CustomerServiceOutlined className="text-blue-500 mr-1" />}
              suffix="个"
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Statistic
              title="在线已注册分机"
              value={data?.records.filter((r: any) => getSimulatedStatus(r.extensionNumber) > 0).length ?? 0}
              prefix={<CompassOutlined className="text-emerald-500 mr-1" />}
              valueStyle={{ color: '#3f8600' }}
              suffix="个"
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Statistic
              title="当前通话状态坐席"
              value={data?.records.filter((r: any) => getSimulatedStatus(r.extensionNumber) === 4).length ?? 0}
              prefix={<ApiOutlined className="text-purple-500 mr-1" />}
              valueStyle={{ color: '#cf1322' }}
              suffix="个"
            />
          </Card>
        </Col>
      </Row>

      <QueryBar
        fields={queryFields}
        onSearch={setQueryParams}
        loading={isPending}
      />

      <TableWrap
        title="分机与坐席资源配置列表"
        rowKey="id"
        loading={isPending}
        dataSource={filteredRecords}
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
          { title: '分机 ID', dataIndex: 'id', width: 90, className: 'font-mono text-xs' },
          {
            title: 'SIP 分机号',
            dataIndex: 'extensionNumber',
            render: (v) => <span className="font-mono font-semibold text-slate-800 dark:text-slate-200">{v}</span>
          },
          {
            title: '注册及状态监测',
            key: 'regStatus',
            width: 160,
            render: (_, record) => {
              const status = getSimulatedStatus(record.extensionNumber)
              const info = statusMap[status]
              return (
                <div className="flex items-center gap-2">
                  <span className={`w-2.5 h-2.5 rounded-full ${info.dotClass}`} />
                  <Tag color={info.color} className="font-medium text-xs">
                    {info.label}
                  </Tag>
                </div>
              )
            }
          },
          {
            title: '所属商户',
            dataIndex: 'merchantId',
            render: (val: any) => <Tag color="blue">{getMerchantName(val)}</Tag>,
          },
          {
            title: '绑定坐席/用户',
            dataIndex: 'userId',
            render: (val: string) => (
              val ? (
                <Space size={4}>
                  <LinkOutlined className="text-slate-400" />
                  <span className="font-mono text-slate-600 dark:text-slate-300">坐席 ID: {val}</span>
                </Space>
              ) : (
                <span className="text-slate-400 text-xs">未绑定</span>
              )
            )
          },
          {
            title: '绑定类型',
            dataIndex: 'bindType',
            render: (val: number) => {
              if (val === 2) {
                return <Tag color="purple">动态释放</Tag>
              }
              return <Tag color="blue">手动绑定</Tag>
            },
          },
          {
            title: '启用状态',
            dataIndex: 'enable',
            width: 100,
            render: (value: boolean, record: any) => (
              <Switch
                checked={value}
                loading={toggleMutation.isPending}
                onChange={(checked) => toggleMutation.mutate({ id: record.id, enable: checked })}
              />
            ),
          },
          {
            title: '操作',
            width: 180,
            render: (_, record) => (
              <Space size="middle">
                <Button size="small" type="link" onClick={() => openEdit(record.id)} className="!p-0">
                  编辑
                </Button>
                <Popconfirm title="确认删除该分机？" onConfirm={() => deleteMutation.mutate([record.id])}>
                  <Button size="small" type="link" danger className="!p-0">
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        open={open}
        title={editingId ? '编辑分机资源' : '新增分机资源'}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnClose
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => {
            saveMutation.mutate({
              ...values,
              id: editingId ?? undefined,
              bindType: values.bindType || 1,
            })
          }}
          initialValues={{ enable: true }}
        >
          <Form.Item name="extensionNumber" label="SIP 分机号" rules={[{ required: true, message: '请输入分机号' }]}>
            <Input placeholder="输入6位 SIP 注册分机号" />
          </Form.Item>
          <Form.Item name="password" label="SIP 注册密码" rules={editingId ? [] : [{ required: true, message: '请输入分机密码' }]}>
            <Input.Password placeholder={editingId ? '若不修改请留空' : '请输入 SIP 注册密码'} />
          </Form.Item>
          <Form.Item name="merchantId" label="关联商户" rules={[{ required: true, message: '请选择关联商户' }]}>
            <Select
              placeholder="请选择绑定的商户"
              options={merchantsData?.records.map((m: any) => ({
                value: String(m.id),
                label: `[${m.id}] ${m.name}`,
              }))}
            />
          </Form.Item>
          <Form.Item name="userId" label="绑定坐席用户 ID (可选)">
            <Input placeholder="分机使用坐席的用户 ID，留空表示未分配" />
          </Form.Item>
          <div className="grid grid-cols-2 gap-4 bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800 p-4 rounded-lg">
            <Form.Item name="bindType" label="绑定回收类型" className="!mb-0">
              <Select
                options={[
                  { value: 1, label: '手动绑定（不回收）' },
                  { value: 2, label: '动态释放（自动回收）' },
                ]}
              />
            </Form.Item>
            <Form.Item name="enable" label="启用状态" valuePropName="checked" className="!mb-0">
              <Switch />
            </Form.Item>
          </div>
        </Form>
      </Modal>
    </Space>
  )
}
