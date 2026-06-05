import {
  Button,
  Form,
  Input,
  InputNumber,
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
  Statistic,
  Tooltip
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
  toggleExtensionEnable,
  recalculateExtensionHA
} from '@/api/operate'
import {
  ReloadOutlined,
  PlusOutlined,
  CustomerServiceOutlined,
  CompassOutlined,
  ApiOutlined,
  LinkOutlined,
  EditOutlined,
  DeleteOutlined,
  PhoneOutlined,
  SettingOutlined,
  SyncOutlined,
  KeyOutlined,
  EyeOutlined,
  EyeInvisibleOutlined
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

// 注册状态基于后端 offlineAt 字段：null 表示在线，有值表示离线

function generateRandomPassword(length = 8): string {
  const chars = 'ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789'
  let result = ''
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return result
}

export function ExtensionPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<ExtensionFormValues>()
  const queryClient = useQueryClient()
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})
  const [visiblePasswords, setVisiblePasswords] = useState<Set<number>>(new Set())

  const { data, refetch, isPending } = useQuery({
    queryKey: ['operate', 'extension', pageNumber, pageSize],
    queryFn: () => fetchExtensions(pageNumber, pageSize),
  })

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

  const recalcMutation = useMutation({
    mutationFn: async () => recalculateExtensionHA(),
    onSuccess: (res: any) => {
      const count = res?.updated ?? 0
      message.success(`已为 ${count} 条分机重新生成密码并计算 HA1/HA1b`)
      queryClient.invalidateQueries({ queryKey: ['operate', 'extension'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '重算失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ enable: true, bindType: 2, userId: '0', password: generateRandomPassword(8) })
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
        userId: record?.userId != null ? String(record.userId) : '0',
        enable: Boolean(record?.enable),
        bindType: record?.bindType ?? 2,
      })
    }, 0)
  }

  function getMerchantName(mId: string | number) {
    const found = merchantsData?.records.find((m: any) => String(m.id) === String(mId))
    return found ? found.name : `商户 ${mId}`
  }

  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.extensionNumber) {
      records = records.filter((r: any) => String(r.extensionNumber).includes(queryParams.extensionNumber.trim()))
    }
    if (queryParams.merchantId) {
      records = records.filter((r: any) => String(r.merchantId) === String(queryParams.merchantId))
    }
    if (queryParams.status === 'online') {
      records = records.filter((r: any) => !r.offlineAt)
    } else if (queryParams.status === 'offline') {
      records = records.filter((r: any) => !!r.offlineAt)
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
        { value: 'online', label: '在线' },
        { value: 'offline', label: '离线' },
      ],
    },
  ], [merchantsData])

  const totalExts = data?.total ?? 0
  const enabledExts = data?.records.filter((r: any) => r.enable).length ?? 0
  const boundExts = data?.records.filter((r: any) => r.userId && Number(r.userId) > 0).length ?? 0
  const onlineExts = data?.records.filter((r: any) => !r.offlineAt).length ?? 0

  return (
    <Space direction="vertical" size="middle" className="w-full">
      {/* 顶部操作栏 */}
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-500 to-indigo-600 flex items-center justify-center shadow-sm">
            <PhoneOutlined className="text-white text-lg" />
          </div>
          <div>
            <Typography.Title level={4} className="!mb-0 !text-slate-800 dark:!text-slate-200">SIP 分机管理</Typography.Title>
            <Typography.Text type="secondary" className="text-xs">管理分机资源、坐席绑定与 SIP 注册配置</Typography.Text>
          </div>
        </div>
        <Space>
          <Tooltip title="刷新列表">
            <Button icon={<ReloadOutlined spin={isPending} />} onClick={() => refetch()} loading={isPending} />
          </Tooltip>
          <Tooltip title="为所有分机重新生成随机密码并重算 SipDomain / HA1 / HA1b">
            <Button
              icon={<SyncOutlined spin={recalcMutation.isPending} />}
              loading={recalcMutation.isPending}
              onClick={() => recalcMutation.mutate()}
            >
              重算 HA
            </Button>
          </Tooltip>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate} className="shadow-sm">
            新增分机
          </Button>
        </Space>
      </div>

      {/* 状态看板卡片组合 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} md={6}>
          <Card variant="borderless" className="rounded-xl shadow-sm overflow-hidden" styles={{ body: { padding: '20px 24px' } }}>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-slate-500 dark:text-slate-400 text-sm mb-1">配置分机总数</div>
                <div className="text-3xl font-bold text-slate-800 dark:text-slate-100">{totalExts}</div>
              </div>
              <div className="w-12 h-12 rounded-xl bg-blue-50 dark:bg-blue-900/30 flex items-center justify-center">
                <CustomerServiceOutlined className="text-blue-500 text-xl" />
              </div>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card variant="borderless" className="rounded-xl shadow-sm overflow-hidden" styles={{ body: { padding: '20px 24px' } }}>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-slate-500 dark:text-slate-400 text-sm mb-1">已启用分机</div>
                <div className="text-3xl font-bold text-emerald-600 dark:text-emerald-400">{enabledExts}</div>
              </div>
              <div className="w-12 h-12 rounded-xl bg-emerald-50 dark:bg-emerald-900/30 flex items-center justify-center">
                <CompassOutlined className="text-emerald-500 text-xl" />
              </div>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card variant="borderless" className="rounded-xl shadow-sm overflow-hidden" styles={{ body: { padding: '20px 24px' } }}>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-slate-500 dark:text-slate-400 text-sm mb-1">已绑定坐席</div>
                <div className="text-3xl font-bold text-rose-600 dark:text-rose-400">{boundExts}</div>
              </div>
              <div className="w-12 h-12 rounded-xl bg-rose-50 dark:bg-rose-900/30 flex items-center justify-center">
                <ApiOutlined className="text-rose-500 text-xl" />
              </div>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card variant="borderless" className="rounded-xl shadow-sm overflow-hidden" styles={{ body: { padding: '20px 24px' } }}>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-slate-500 dark:text-slate-400 text-sm mb-1">当前在线</div>
                <div className="text-3xl font-bold text-green-600 dark:text-green-400">{onlineExts}</div>
              </div>
              <div className="w-12 h-12 rounded-xl bg-green-50 dark:bg-green-900/30 flex items-center justify-center">
                <LinkOutlined className="text-green-500 text-xl" />
              </div>
            </div>
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
          {
            title: 'ID',
            dataIndex: 'id',
            width: 70,
            className: 'font-mono text-xs text-slate-400',
          },
          {
            title: 'SIP 分机号',
            dataIndex: 'extensionNumber',
            render: (v: string) => (
              <div className="flex items-center gap-2">
                <span className="font-mono font-semibold text-slate-800 dark:text-slate-200 bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded text-sm">{v}</span>
              </div>
            )
          },
          {
            title: '注册密码',
            dataIndex: 'password',
            width: 150,
            render: (val: string, record: any) => {
              const visible = visiblePasswords.has(record.id)
              return (
                <Space size={4}>
                  <span className="font-mono text-xs text-slate-600 dark:text-slate-300">
                    {visible ? val : '••••••••'}
                  </span>
                  <Button
                    size="small"
                    type="text"
                    icon={visible ? <EyeInvisibleOutlined /> : <EyeOutlined />}
                    onClick={() => {
                      const next = new Set(visiblePasswords)
                      visible ? next.delete(record.id) : next.add(record.id)
                      setVisiblePasswords(next)
                    }}
                    className="text-slate-400 hover:text-slate-600 !p-0"
                  />
                </Space>
              )
            },
          },
          {
            title: '所属商户',
            dataIndex: 'merchantId',
            width: 150,
            render: (val: any) => (
              <Tag color="blue" className="!m-0">{getMerchantName(val)}</Tag>
            ),
          },
          {
            title: '绑定坐席',
            dataIndex: 'userId',
            width: 130,
            render: (val: string | number) => (
              val && Number(val) > 0 ? (
                <Space size={4}>
                  <LinkOutlined className="text-slate-400" />
                  <span className="font-mono text-slate-600 dark:text-slate-300 text-xs">ID: {val}</span>
                </Space>
              ) : (
                <span className="text-slate-400 text-xs italic">未绑定</span>
              )
            )
          },
          {
            title: '注册状态',
            dataIndex: 'offlineAt',
            width: 110,
            align: 'center' as const,
            render: (val: string | null) => {
              if (!val) {
                return <Tag color="success" className="!m-0 text-xs">在线</Tag>
              }
              const offlineTime = new Date(val)
              const now = new Date()
              const diffSec = Math.floor((now.getTime() - offlineTime.getTime()) / 1000)
              let duration = ''
              if (diffSec < 60) {
                duration = `${diffSec}秒`
              } else if (diffSec < 3600) {
                duration = `${Math.floor(diffSec / 60)}分钟`
              } else if (diffSec < 86400) {
                duration = `${Math.floor(diffSec / 3600)}小时`
              } else {
                duration = `${Math.floor(diffSec / 86400)}天`
              }
              return (
                <Tooltip title={`离线于 ${offlineTime.toLocaleString()}`}>
                  <Tag color="default" className="!m-0 text-xs">离线 {duration}</Tag>
                </Tooltip>
              )
            },
          },
          {
            title: 'SIP 域名',
            dataIndex: 'sipDomain',
            width: 160,
            ellipsis: true,
            render: (val: string) => val ? (
              <span className="font-mono text-xs text-slate-500 dark:text-slate-400">{val}</span>
            ) : (
              <span className="text-slate-400 text-xs italic">未配置</span>
            ),
          },
          {
            title: 'HA1',
            dataIndex: 'ha1',
            width: 200,
            ellipsis: true,
            render: (val: string) => val ? (
              <Tooltip title={val}>
                <span className="font-mono text-xs text-slate-500 dark:text-slate-400">{val.slice(0, 12)}...</span>
              </Tooltip>
            ) : (
              <span className="text-slate-400 text-xs italic">-</span>
            ),
          },
          {
            title: 'HA1b',
            dataIndex: 'ha1b',
            width: 200,
            ellipsis: true,
            render: (val: string) => val ? (
              <Tooltip title={val}>
                <span className="font-mono text-xs text-slate-500 dark:text-slate-400">{val.slice(0, 12)}...</span>
              </Tooltip>
            ) : (
              <span className="text-slate-400 text-xs italic">-</span>
            ),
          },
          {
            title: '回收类型',
            dataIndex: 'bindType',
            width: 120,
            render: (val: number) => {
              if (val === 2) {
                return <Tag color="purple" className="!m-0 text-xs">动态释放</Tag>
              }
              return <Tag color="blue" className="!m-0 text-xs">手动绑定</Tag>
            },
          },
          {
            title: '启用',
            dataIndex: 'enable',
            width: 80,
            align: 'center' as const,
            render: (value: boolean, record: any) => (
              <Switch
                checked={value}
                size="small"
                loading={toggleMutation.isPending}
                onChange={(checked) => toggleMutation.mutate({ id: record.id, enable: checked })}
              />
            ),
          },
          {
            title: '操作',
            width: 140,
            align: 'center' as const,
            render: (_, record) => (
              <Space size={4}>
                <Tooltip title="编辑分机配置">
                  <Button size="small" type="text" icon={<EditOutlined />} onClick={() => openEdit(record.id)} className="text-blue-500 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20" />
                </Tooltip>
                <Popconfirm title="确认删除该分机？" description="删除后该分机配置将不可恢复" onConfirm={() => deleteMutation.mutate([record.id])}>
                  <Tooltip title="删除分机">
                    <Button size="small" type="text" icon={<DeleteOutlined />} danger className="hover:bg-red-50 dark:hover:bg-red-900/20" />
                  </Tooltip>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        open={open}
        title={
          <div className="flex items-center gap-2">
            <SettingOutlined className="text-blue-500" />
            <span>{editingId ? '编辑分机资源' : '新增分机资源'}</span>
          </div>
        }
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnHidden
        width={560}
        okText={editingId ? '保存修改' : '创建分机'}
      >
        <Form
          form={form}
          layout="vertical"
          className="mt-4"
          onFinish={(values) => {
            saveMutation.mutate({
              ...values,
              id: editingId ?? undefined,
              merchantId: String(Number(values.merchantId) || 0),
              userId: String(Number(values.userId) || 0),
              bindType: values.bindType || 2,
            })
          }}
          initialValues={{ enable: true, bindType: 2 }}
        >
          <div className="bg-slate-50 dark:bg-slate-800/50 rounded-lg p-4 mb-4 border border-slate-100 dark:border-slate-700">
            <Typography.Text className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3 block">SIP 注册信息</Typography.Text>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name="extensionNumber" label="SIP 分机号" rules={[{ required: true, message: '请输入分机号' }]} className="!mb-3">
                  <Input placeholder="6位 SIP 分机号" className="font-mono" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="password" label="SIP 注册密码" rules={editingId ? [] : [{ required: true, message: '请输入密码' }]} className="!mb-3">
                  <Input.Password placeholder={editingId ? '若不修改请留空' : '请输入 SIP 注册密码'} />
                </Form.Item>
              </Col>
            </Row>
          </div>

          <div className="bg-slate-50 dark:bg-slate-800/50 rounded-lg p-4 mb-4 border border-slate-100 dark:border-slate-700">
            <Typography.Text className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3 block">业务关联</Typography.Text>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name="merchantId" label="关联商户" rules={[{ required: true, message: '请选择商户' }]} className="!mb-3">
                  <Select
                    placeholder="请选择绑定的商户"
                    showSearch
                    optionFilterProp="label"
                    options={merchantsData?.records.map((m: any) => ({
                      value: String(m.id),
                      label: `[${m.id}] ${m.name}`,
                    }))}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="userId" label="坐席用户 ID" className="!mb-3">
                  <Input placeholder="坐席用户 ID，0 或留空表示未分配" />
                </Form.Item>
              </Col>
            </Row>
          </div>

          <div className="bg-slate-50 dark:bg-slate-800/50 rounded-lg p-4 border border-slate-100 dark:border-slate-700">
            <Typography.Text className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3 block">策略配置</Typography.Text>
            <Row gutter={16}>
              <Col span={14}>
                <Form.Item name="bindType" label="绑定回收类型" className="!mb-3">
                  <Select
                    options={[
                      { value: 1, label: '手动绑定（不回收）' },
                      { value: 2, label: '动态释放（自动回收）' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col span={10}>
                <Form.Item name="enable" label="启用状态" valuePropName="checked" className="!mb-3">
                  <Switch checkedChildren="启用" unCheckedChildren="停用" />
                </Form.Item>
              </Col>
            </Row>
          </div>
        </Form>
      </Modal>
    </Space>
  )
}
