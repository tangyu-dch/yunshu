import {
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Space,
  Typography,
  message,
  Tabs,
  Card,
  Tag,
  Row,
  Col,
  Alert,
  Tooltip,
  Select,
  Divider,
  Switch
} from 'antd'
import {
  SafetyCertificateOutlined,
  SettingOutlined,
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  PhoneOutlined,
  DatabaseOutlined,
  WarningOutlined,
  InfoCircleOutlined,
  SearchOutlined,
  ReloadOutlined,
  ApiOutlined
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo, useEffect } from 'react'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { useUiStore } from '@/store/ui'
import {
  deleteBlacklist,
  fetchBlacklist,
  saveBlacklist,
  fetchBlacklistNumbers,
  saveBlacklistNumber,
  deleteBlacklistNumbers,
  fetchBlacklistChannels,
  saveBlacklistChannel,
  deleteBlacklistChannel
} from '@/api/operate'

const { TabPane } = Tabs
const { Option } = Select

type BlacklistFormValues = {
  id?: number
  name: string
  verificationChannel: number
  remark?: string
}

type BlacklistNumberFormValues = {
  phone: string
  blackLevel: string
  remark: string
}

export type VerificationChannel = {
  code: number
  name: string
  vendor: string
  remark?: string
  enable: boolean
  apiUrl?: string
  appId?: string
  appSecret?: string
  reqTemplate?: string
  respExtractPath?: string
  respMatchValue?: string
  timeoutMs?: number
}

const DEFAULT_CHANNELS: VerificationChannel[] = [
  { code: 1, name: '东信易通黑名单', vendor: 'DONG_XIN', remark: '系统默认东信易通强风控验证通道', enable: true },
  { code: 2, name: '羽乐黑名单', vendor: 'YU_LE', remark: '系统默认羽乐科技防骚扰拦截通道', enable: true },
]

export function BlacklistPage() {
  const theme = useUiStore((state) => state.theme)
  const isDark = theme === 'dark'
  const [activeTab, setActiveTab] = useState('1')
  const queryClient = useQueryClient()

  // --- Dynamic Verification Channels Configurations ---
  const { data: channels = DEFAULT_CHANNELS, refetch: refetchChannels } = useQuery<VerificationChannel[]>({
    queryKey: ['operate', 'blacklist-channels'],
    queryFn: fetchBlacklistChannels,
    initialData: DEFAULT_CHANNELS,
  })

  const saveChannelMutation = useMutation({
    mutationFn: saveBlacklistChannel,
    onSuccess: () => {
      refetchChannels()
      queryClient.invalidateQueries({ queryKey: ['operate', 'blacklist'] })
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '保存通道失败'),
  })

  const deleteChannelMutation = useMutation({
    mutationFn: deleteBlacklistChannel,
    onSuccess: () => {
      refetchChannels()
      queryClient.invalidateQueries({ queryKey: ['operate', 'blacklist'] })
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '删除通道失败'),
  })

  // --- Tab 1: Blacklist Groups/Libraries States ---
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<BlacklistFormValues>()

  // --- Tab 2: Blacklist Numbers States ---
  const [numPageNumber, setNumPageNumber] = useState(1)
  const [numPageSize, setNumPageSize] = useState(20)
  const [phoneFilter, setPhoneFilter] = useState('')
  const [levelFilter, setLevelFilter] = useState<string | undefined>(undefined)
  const [numOpen, setNumOpen] = useState(false)
  const [editingPhone, setEditingPhone] = useState<string | null>(null)
  const [selectedPhones, setSelectedPhones] = useState<string[]>([])
  const [numForm] = Form.useForm<BlacklistNumberFormValues>()

  // --- Tab 3: Dynamic Channels Configuration States ---
  const [channelOpen, setChannelOpen] = useState(false)
  const [editingChannelCode, setEditingChannelCode] = useState<number | null>(null)
  const [channelForm] = Form.useForm<VerificationChannel>()

  const queryFields = useMemo(() => [
    { key: 'phone', label: '手机号码', type: 'text' as const, placeholder: '搜索手机号码' },
    {
      key: 'blackLevel',
      label: '拦截等级',
      type: 'select' as const,
      options: [
        { value: 'LEVEL_1', label: '一级拦截 (高危号码)' },
        { value: 'LEVEL_2', label: '二级拦截 (营销投诉)' },
        { value: 'LEVEL_3', label: '三级拦截 (频次超限)' },
      ],
    },
  ], [])

  const handleNumSearch = (values: Record<string, any>) => {
    setPhoneFilter(values.phone || '')
    setLevelFilter(values.blackLevel)
    setNumPageNumber(1)
  }

  const handleNumReset = () => {
    setPhoneFilter('')
    setLevelFilter(undefined)
    setNumPageNumber(1)
  }

  // --- Queries & Mutations: Tab 1 ---
  const { data, isLoading: isBlacklistLoading } = useQuery({
    queryKey: ['operate', 'blacklist', pageNumber, pageSize],
    queryFn: () => fetchBlacklist(pageNumber, pageSize),
    enabled: activeTab === '1',
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => deleteBlacklist(id),
    onSuccess: async () => {
      message.success('黑名单库已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'blacklist'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: BlacklistFormValues) => saveBlacklist(values),
    onSuccess: async () => {
      message.success(editingId ? '黑名单已更新' : '黑名单已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'blacklist'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  // --- Queries & Mutations: Tab 2 ---
  const { data: numData, isLoading: isNumLoading } = useQuery({
    queryKey: ['operate', 'blacklist-numbers', numPageNumber, numPageSize, phoneFilter, levelFilter],
    queryFn: () =>
      fetchBlacklistNumbers({
        pageNumber: numPageNumber,
        pageSize: numPageSize,
        phone: phoneFilter || undefined,
        blackLevel: levelFilter || undefined,
      }),
    enabled: activeTab === '2',
  })

  const saveNumMutation = useMutation({
    mutationFn: async (values: BlacklistNumberFormValues) => saveBlacklistNumber(values),
    onSuccess: async () => {
      message.success(editingPhone ? '黑名单号码已更新' : '黑名单号码录入成功')
      setNumOpen(false)
      setEditingPhone(null)
      numForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'blacklist-numbers'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '录入失败'),
  })

  const deleteNumsMutation = useMutation({
    mutationFn: async (phones: string[]) => deleteBlacklistNumbers(phones),
    onSuccess: async () => {
      message.success('选中的黑名单号码已移出')
      setSelectedPhones([])
      await queryClient.invalidateQueries({ queryKey: ['operate', 'blacklist-numbers'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '移除失败'),
  })

  // --- Actions: Tab 1 ---
  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      // 默认选择第一个启用的通道
      const firstActive = channels.find(c => c.enable)
      form.setFieldsValue({ verificationChannel: firstActive ? firstActive.code : 1 })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item: any) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id: record?.id,
        name: record?.name ?? '',
        verificationChannel: record?.verificationChannel ?? 1,
        remark: record?.remark ?? '',
      })
    }, 0)
  }

  // --- Actions: Tab 2 ---
  function openCreateNum() {
    setEditingPhone(null)
    setNumOpen(true)
    setTimeout(() => {
      numForm.resetFields()
      numForm.setFieldsValue({ blackLevel: 'LEVEL_1' })
    }, 0)
  }

  function openEditNum(record: any) {
    setEditingPhone(record.phone)
    setNumOpen(true)
    setTimeout(() => {
      numForm.setFieldsValue({
        phone: record.phone,
        blackLevel: record.blackLevel,
        remark: record.remark,
      })
    }, 0)
  }

  // --- Actions: Tab 3 (Verification Channels) ---
  function openCreateChannel() {
    setEditingChannelCode(null)
    setChannelOpen(true)
    setTimeout(() => {
      channelForm.resetFields()
      channelForm.setFieldsValue({ enable: true })
    }, 0)
  }

  function openEditChannel(record: VerificationChannel) {
    setEditingChannelCode(record.code)
    setChannelOpen(true)
    setTimeout(() => {
      channelForm.setFieldsValue({
        code: record.code,
        name: record.name,
        vendor: record.vendor,
        remark: record.remark,
        enable: record.enable,
        apiUrl: record.apiUrl ?? '',
        appId: record.appId ?? '',
        appSecret: record.appSecret ?? '',
        reqTemplate: record.reqTemplate ?? '',
        respExtractPath: record.respExtractPath ?? '',
        respMatchValue: record.respMatchValue ?? '',
        timeoutMs: record.timeoutMs ?? 500,
      })
    }, 0)
  }

  function deleteChannel(code: number) {
    deleteChannelMutation.mutate(code, {
      onSuccess: () => {
        message.success('通道配置已成功删除')
      }
    })
  }

  // --- Custom Level Styles ---
  function renderLevelTag(level: string) {
    const isDarkTheme = isDark

    switch (level) {
      case 'LEVEL_1':
        return (
          <Tag
            style={isDarkTheme ? {
              background: 'linear-gradient(135deg, rgba(239, 68, 68, 0.15) 0%, rgba(239, 68, 68, 0.25) 100%)',
              color: '#fca5a5',
              border: '1px solid rgba(239, 68, 68, 0.4)',
              borderRadius: '4px',
              fontWeight: 500,
            } : {
              background: 'linear-gradient(135deg, #ffebee 0%, #ffcdd2 100%)',
              color: '#c62828',
              border: '1px solid #ef9a9a',
              borderRadius: '4px',
              fontWeight: 500,
            }}
          >
            一级拦截 (高危号码)
          </Tag>
        )
      case 'LEVEL_2':
        return (
          <Tag
            style={isDarkTheme ? {
              background: 'linear-gradient(135deg, rgba(249, 115, 22, 0.15) 0%, rgba(249, 115, 22, 0.25) 100%)',
              color: '#fdba74',
              border: '1px solid rgba(249, 115, 22, 0.4)',
              borderRadius: '4px',
              fontWeight: 500,
            } : {
              background: 'linear-gradient(135deg, #fff3e0 0%, #ffe0b2 100%)',
              color: '#ef6c00',
              border: '1px solid #ffcc80',
              borderRadius: '4px',
              fontWeight: 500,
            }}
          >
            二级拦截 (营销投诉)
          </Tag>
        )
      case 'LEVEL_3':
        return (
          <Tag
            style={isDarkTheme ? {
              background: 'linear-gradient(135deg, rgba(234, 179, 8, 0.15) 0%, rgba(234, 179, 8, 0.25) 100%)',
              color: '#fef08a',
              border: '1px solid rgba(234, 179, 8, 0.4)',
              borderRadius: '4px',
              fontWeight: 500,
            } : {
              background: 'linear-gradient(135deg, #fffde7 0%, #fff9c4 100%)',
              color: '#f57f17',
              border: '1px solid #fff59d',
              borderRadius: '4px',
              fontWeight: 500,
            }}
          >
            三级拦截 (频次超限)
          </Tag>
        )
      default:
        return <Tag color="default">{level}</Tag>
    }
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* Main Tabs Container */}
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        type="card"
        className="w-full"
        style={{ marginTop: '8px' }}
      >
        {/* TAB 1: 黑名单验证库 */}
        <TabPane
          tab={
            <span>
              <DatabaseOutlined /> 黑名单验证库通道配置
            </span>
          }
          key="1"
        >
          <div className="flex justify-end mb-4">
            <Space>
              <Button
                icon={<ReloadOutlined />}
                onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'blacklist'] })}
              >
                刷新列表
              </Button>
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                新增黑名单库
              </Button>
            </Space>
          </div>

          <TableWrap
            title="黑名单库与网关通道映射"
            rowKey="id"
            loading={isBlacklistLoading}
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
              { title: '库 ID', dataIndex: 'id', width: 80 },
              {
                title: '名称',
                dataIndex: 'name',
                render: (name) => <span className="font-semibold text-slate-700 dark:text-slate-200">{name}</span>,
              },
              {
                title: '验证通道代码',
                dataIndex: 'verificationChannel',
                render: (code) => {
                  const ch = channels.find((c) => c.code === code)
                  return (
                    <Space>
                      <Tag color="purple">{code}</Tag>
                      <span className="text-slate-500 dark:text-slate-400 text-xs">
                        ({ch ? ch.name : '未知外部通道'})
                      </span>
                    </Space>
                  )
                },
              },
              { title: '备注', dataIndex: 'remark' },
              {
                title: '操作',
                width: 150,
                render: (_, record) => (
                  <Space size="small">
                    <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record.id)}>
                      编辑
                    </Button>
                    <Popconfirm
                      title="确认删除该黑名单库配置？"
                      onConfirm={() => deleteMutation.mutate(record.id)}
                    >
                      <Button size="small" danger icon={<DeleteOutlined />}>
                        删除
                      </Button>
                    </Popconfirm>
                  </Space>
                ),
              },
            ]}
          />
        </TabPane>

        {/* TAB 2: 黑名单具体号码 */}
        <TabPane
          tab={
            <span>
              <PhoneOutlined /> 风控黑名单号码拦截配置
            </span>
          }
          key="2"
        >
          <QueryBar
            fields={queryFields}
            onSearch={handleNumSearch}
            onReset={handleNumReset}
            loading={isNumLoading}
          />

          {/* Table Actions Bar */}
          <div className="flex justify-between items-center mb-4">
            <div className="flex-1 mr-4">
              {selectedPhones.length > 0 && (
                <Alert
                  type="warning"
                  showIcon
                  className="dark:bg-yellow-950/20 dark:border-yellow-900/50"
                  message={
                    <span>
                      已选中 <strong className="text-indigo-600 dark:text-indigo-400">{selectedPhones.length}</strong> 个黑名单号码{' '}
                      <Popconfirm
                        title={`确定要批量移出选中的 ${selectedPhones.length} 个黑名单号码吗？`}
                        onConfirm={() => deleteNumsMutation.mutate(selectedPhones)}
                      >
                        <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                          立即批量移出
                        </Button>
                      </Popconfirm>
                    </span>
                  }
                />
              )}
            </div>
            <Space>
              <Button icon={<ReloadOutlined />} onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'blacklist-numbers'] })}>
                刷新数据
              </Button>
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreateNum}>
                录入黑名单号码
              </Button>
            </Space>
          </div>

          <TableWrap
            title="黑名单号码列表 (支持批量移出)"
            rowKey="phone"
            loading={isNumLoading}
            dataSource={numData?.records ?? []}
            rowSelection={{
              selectedRowKeys: selectedPhones,
              onChange: (keys) => setSelectedPhones(keys as string[]),
            }}
            pagination={{
              current: numPageNumber,
              pageSize: numPageSize,
              total: numData?.total ?? 0,
              onChange: (current, size) => {
                setNumPageNumber(current)
                setNumPageSize(size ?? numPageSize)
              },
              showSizeChanger: true,
            }}
            columns={[
              {
                title: '黑名单号码',
                dataIndex: 'phone',
                render: (phone) => (
                  <Space>
                    <PhoneOutlined className="text-slate-400" />
                    <span className="font-semibold text-slate-800 dark:text-slate-200">{phone}</span>
                  </Space>
                ),
              },
              {
                title: '黑名单等级',
                dataIndex: 'blackLevel',
                render: (level) => renderLevelTag(level),
              },
              { title: '备注原因', dataIndex: 'remark' },
              {
                title: '操作',
                width: 150,
                render: (_, record) => (
                  <Space size="small">
                    <Button size="small" icon={<EditOutlined />} onClick={() => openEditNum(record)}>
                      修改
                    </Button>
                    <Popconfirm
                      title="确定要将该号码移出黑名单库吗？"
                      onConfirm={() => deleteNumsMutation.mutate([record.phone])}
                    >
                      <Button size="small" danger icon={<DeleteOutlined />}>
                        移出
                      </Button>
                    </Popconfirm>
                  </Space>
                ),
              },
            ]}
          />
        </TabPane>

        {/* TAB 3: 三方通道动态配置 (动态接入外部黑名单) */}
        <TabPane
          tab={
            <span>
              <ApiOutlined /> 三方通道动态配置
            </span>
          }
          key="3"
        >
          <div className="flex justify-end mb-4">
            <Space>
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreateChannel}>
                接入外部新通道
              </Button>
            </Space>
          </div>

          <TableWrap
            title="已配置的三方风控验证通道"
            rowKey="code"
            dataSource={channels}
            columns={[
              { title: '通道代码 (唯一识别码)', dataIndex: 'code', width: 180 },
              {
                title: '通道显示名称',
                dataIndex: 'name',
                render: (name) => <span className="font-semibold text-slate-700 dark:text-slate-200">{name}</span>,
              },
              {
                title: '厂商标识 (Vendor ID)',
                dataIndex: 'vendor',
                render: (vendor) => <Tag color="cyan">{vendor}</Tag>,
              },
              { title: '描述说明', dataIndex: 'remark' },
              {
                title: '状态',
                dataIndex: 'enable',
                width: 120,
                render: (enable, record) => (
                  <Switch
                    checked={enable}
                    onChange={(checked) => {
                      saveChannelMutation.mutate({
                        ...record,
                        enable: checked,
                      }, {
                        onSuccess: () => {
                          message.success(`${record.name} 已被${checked ? '启用' : '停用'}`)
                        }
                      })
                    }}
                  />
                ),
              },
              {
                title: '操作',
                width: 180,
                render: (_, record) => (
                  <Space size="small">
                    <Button size="small" icon={<EditOutlined />} onClick={() => openEditChannel(record)}>
                      编辑
                    </Button>
                    <Popconfirm
                      title="确认彻底移除该外部通道接入配置吗？该通道下所有的验证库可能会失效！"
                      onConfirm={() => deleteChannel(record.code)}
                    >
                      <Button size="small" danger icon={<DeleteOutlined />}>
                        移除
                      </Button>
                    </Popconfirm>
                  </Space>
                ),
              },
            ]}
          />
        </TabPane>
      </Tabs>

      {/* MODAL 1: Add/Edit Blacklist Group */}
      <Modal
        open={open}
        title={editingId ? '编辑黑名单外部验证库' : '新增黑名单外部验证库'}
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
            })
          }}
        >
          <Form.Item name="name" label="库名称" rules={[{ required: true, message: '请输入库名称' }]}>
            <Input placeholder="输入黑名单库名称，如：东信金融强风控库" />
          </Form.Item>
          <Form.Item
            name="verificationChannel"
            label="外部通道验证厂商代码"
            rules={[{ required: true, message: '请选择通道验证厂商' }]}
          >
            <Select placeholder="请选择风控验证通道">
              {channels.filter(c => c.enable).map(c => (
                <Option key={c.code} value={c.code}>
                  {c.code} - {c.name} ({c.vendor})
                </Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input placeholder="输入该验证库的作用和说明" />
          </Form.Item>
        </Form>
      </Modal>

      {/* MODAL 2: Add/Edit Blacklist Number */}
      <Modal
        open={numOpen}
        title={editingPhone ? '修改黑名单拦截号码' : '录入黑名单拦截号码'}
        onCancel={() => {
          setNumOpen(false)
          setEditingPhone(null)
          numForm.resetFields()
        }}
        onOk={() => numForm.submit()}
        confirmLoading={saveNumMutation.isPending}
        destroyOnClose
      >
        <Form form={numForm} layout="vertical" onFinish={(values) => saveNumMutation.mutate(values)}>
          <Form.Item
            name="phone"
            label="拦截手机号码"
            rules={[
              { required: true, message: '请输入合法的手机号码' },
              { pattern: /^[0-9+-\s]{7,20}$/, message: '请输入合法的手机号码格式' },
            ]}
          >
            <Input placeholder="例如: 13888888888" disabled={!!editingPhone} />
          </Form.Item>

          <Form.Item
            name="blackLevel"
            label="风控拦截等级"
            rules={[{ required: true, message: '请选择号码对应的拦截风控等级' }]}
          >
            <Select>
              <Option value="LEVEL_1">LEVEL_1 - 一级拦截 (高危/涉诈风险/恶意投诉号)</Option>
              <Option value="LEVEL_2">LEVEL_2 - 二级拦截 (高频营销投诉/历史骚扰退订)</Option>
              <Option value="LEVEL_3">LEVEL_3 - 三级拦截 (一般嫌疑/频次超限警告)</Option>
            </Select>
          </Form.Item>

          <Form.Item name="remark" label="限制原因说明" rules={[{ required: true, message: '请输入录入说明' }]}>
            <Input.TextArea placeholder="请输入该号码列入黑名单的具体事由，例如：用户投诉风控拦截、主叫申请禁拨等" />
          </Form.Item>
        </Form>
      </Modal>

      {/* MODAL 3: Add/Edit Third-Party Channel */}
      <Modal
        open={channelOpen}
        title={editingChannelCode ? '编辑外部验证通道' : '接入外部新风控验证通道'}
        onCancel={() => {
          setChannelOpen(false)
          setEditingChannelCode(null)
          channelForm.resetFields()
        }}
        onOk={() => channelForm.submit()}
        destroyOnClose
      >
        <Form
          form={channelForm}
          layout="vertical"
          onFinish={(values) => {
            if (editingChannelCode === null) {
              // 唯一性检验
              if (channels.some(c => c.code === values.code)) {
                message.error('该通道代码已存在，请重新输入唯一识别码！')
                return
              }
            }
            const payload = {
              ...values,
              code: editingChannelCode !== null ? editingChannelCode : values.code,
            }
            saveChannelMutation.mutate(payload, {
              onSuccess: () => {
                message.success('验证通道配置已成功保存！')
                setChannelOpen(false)
              }
            })
          }}
        >
          <Form.Item
            name="code"
            label="通道识别码 (系统内部唯一数字 ID)"
            rules={[
              { required: true, message: '请输入通道数字识别码' }
            ]}
          >
            <InputNumber className="w-full" disabled={editingChannelCode !== null} min={1} max={9999} placeholder="例如: 3" />
          </Form.Item>

          <Form.Item
            name="name"
            label="通道显示名称"
            rules={[{ required: true, message: '请输入通道名称' }]}
          >
            <Input placeholder="例如: 智能防投诉风控通道" />
          </Form.Item>

          <Form.Item
            name="vendor"
            label="厂商技术标识 (Vendor ID/Slug)"
            rules={[{ required: true, message: '请输入厂商技术标识' }]}
          >
            <Input placeholder="例如: ZHI_NENG_RC" />
          </Form.Item>

          <Form.Item
            name="apiUrl"
            label="三方风控验证 API 地址"
            rules={[
              { required: true, message: '请输入请求的 API 地址' },
              { type: 'url', message: '请输入合法的 URL 地址' }
            ]}
          >
            <Input placeholder="例如: https://api.smartrc.com/v1/blacklist/check" />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="appId" label="应用账号 (App ID)">
                <Input placeholder="例如: client_1028" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="appSecret" label="应用密钥 (App Secret)">
                <Input.Password placeholder="例如: ******" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            name="reqTemplate"
            label="请求体参数 JSON 模板"
            rules={[{ required: true, message: '请输入请求参数 JSON 模板' }]}
            tooltip="支持使用 {phone}、{app_id}、{app_secret} 占位符进行拨号时的实时动态求值替换"
          >
            <Input.TextArea
              rows={3}
              placeholder={`例如:\n{\n  "phone": "{phone}",\n  "app_id": "{app_id}",\n  "sign": "{app_secret}"\n}`}
            />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="respExtractPath"
                label="响应提取路径 (JSONPath)"
                rules={[{ required: true, message: '请输入提取路径' }]}
                tooltip="使用 '.' 符号提取多层嵌套中的目标属性值，如: data.isHit"
              >
                <Input placeholder="例如: data.hit" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="respMatchValue"
                label="拦截判定匹配值"
                rules={[{ required: true, message: '请输入用于拦截判定匹配的值' }]}
                tooltip="如果从响应中提取出的值等于该值，则呼叫系统将判定命中黑名单并执行强制阻断，如: true 或 1"
              >
                <Input placeholder="例如: true" />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16} align="middle">
            <Col span={12}>
              <Form.Item
                name="timeoutMs"
                label="请求超时时间 (毫秒)"
                rules={[{ required: true, message: '请输入请求超时时间' }]}
              >
                <InputNumber className="w-full" min={50} max={5000} placeholder="例如: 500" />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="enable" label="启用状态" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="remark" label="风控厂商备注说明">
            <Input placeholder="例如: 用于防骚扰实时风控过滤" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
