import { Button, Popconfirm, Space, Tag, Typography, message, Tooltip, Tabs, Modal, Form, Input, Select, Slider, Card, InputNumber } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo, useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import {
  PlusOutlined,
  DeleteOutlined,
  SettingOutlined,
  ReloadOutlined,
  PhoneOutlined,
  CustomerServiceOutlined,
  RobotOutlined,
  SlidersOutlined
} from '@ant-design/icons'
import {
  fetchAiFlows,
  deleteAiFlows,
  publishAiFlow,
  fetchAiModelConfigs,
  saveAiModelConfig,
  deleteAiModelConfigs,
  saveAiFlow,
  fetchAiProviders,
  AiProviderItem
} from '@/api/operate'

const { Text } = Typography

export function AiModelFlowPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  // Tab-Route 智能联动：根据当前 URL 路径自动确定高亮 Tab
  const defaultTab = location.pathname.includes('ai-model-config') ? '2' : '1'
  const [activeTab, setActiveTab] = useState(defaultTab)

  // 侦听 URL 物理路径变更，实现 Tab 自动切换联动
  useEffect(() => {
    const targetTab = location.pathname.includes('ai-model-config') ? '2' : '1'
    if (activeTab !== targetTab) {
      setActiveTab(targetTab)
    }
  }, [location.pathname])

  // 处理用户手动点击 Tab 切换时的动作，同步更新 URL 路径
  const handleTabChange = (key: string) => {
    setActiveTab(key)
    if (key === '1') {
      navigate('/merchant/ai-model-flow')
    } else {
      navigate('/merchant/ai-model-config')
    }
  }

  // Tab 1: AI流程状态与列表
  const [pageNumber, setPageNumber] = useState(1)
  const [isFlowEditModalOpen, setIsFlowEditModalOpen] = useState(false)
  const [editingFlow, setEditingFlow] = useState<any>(null)
  const [flowForm] = Form.useForm()
  const [pageSize, setPageSize] = useState(20)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  // Tab 2: AI模型厂商配置状态
  const [isConfigModalOpen, setIsConfigModalOpen] = useState(false)
  const [selectedConfigIds, setSelectedConfigIds] = useState<number[]>([])
  const [configForm] = Form.useForm()
  const [editingConfig, setEditingConfig] = useState<any>(null)

  // 读取 AI 智能流列表
  const { data: flowsData, isLoading: isFlowsLoading } = useQuery({
    queryKey: ['merchant', 'ai-flow', pageNumber, pageSize],
    queryFn: () => fetchAiFlows(pageNumber, pageSize),
  })

  // 读取 AI 模型配置列表
  const { data: configsData, isLoading: isConfigsLoading } = useQuery({
    queryKey: ['merchant', 'ai-model-configs'],
    queryFn: () => fetchAiModelConfigs(),
    enabled: activeTab === '2',
  })

  // 读取已支持的大模型服务商列表（后端配置驱动，高可扩展性）
  const { data: providersList } = useQuery({
    queryKey: ['merchant', 'ai-providers'],
    queryFn: () => fetchAiProviders(),
  })

  const DEFAULT_PROVIDERS: AiProviderItem[] = useMemo(() => [
    { value: 'deepseek', label: 'DeepSeek API', emoji: '🐳', color: 'cyan', implemented: true, supportAsr: false, supportTts: false, supportLlm: true },
    { value: 'openai', label: 'OpenAI 兼容接口', emoji: '🌐', color: 'purple', implemented: true, supportAsr: true, supportTts: true, supportLlm: true },
    { value: 'ali', label: '阿里通义千问 Qwen', emoji: '☁️', color: 'geekblue', implemented: true, supportAsr: true, supportTts: true, supportLlm: true },
    { value: 'tencent', label: '腾讯混元 Hunyuan', emoji: '🐧', color: 'blue', implemented: true, supportAsr: true, supportTts: true, supportLlm: true },
    { value: 'volc', label: '火山引擎“豆包”大模型', emoji: '🌋', color: 'orange', implemented: true, supportAsr: true, supportTts: true, supportLlm: true },
    { value: 'zhipu', label: '智谱AI GLM大模型', emoji: '🤖', color: 'green', implemented: true, supportAsr: false, supportTts: false, supportLlm: true }
  ], [])

  const providers = providersList || DEFAULT_PROVIDERS

  const queryFields = useMemo(() => [
    { key: 'name', label: '模型流名称', type: 'text' as const, placeholder: '请输入名称模糊搜索' },
    {
      key: 'status',
      label: '状态',
      type: 'select' as const,
      options: [
        { value: 'published', label: '已发布' },
        { value: 'draft', label: '草稿 (预检)' }
      ],
    },
  ], [])

  const filteredRecords = useMemo(() => {
    let records = flowsData?.records ?? []
    if (queryParams.name) {
      records = records.filter((r: any) => String(r.name).toLowerCase().includes(queryParams.name.toLowerCase().trim()))
    }
    if (queryParams.status) {
      records = records.filter((r: any) => r.status === queryParams.status)
    }
    return records
  }, [flowsData, queryParams])

  // 计算看板实时统计数据
  const stats = useMemo(() => {
    const total = filteredRecords.length
    const published = filteredRecords.filter((r: any) => r.status === 'published').length
    const drafts = total - published
    return { total, published, drafts }
  }, [filteredRecords])

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteAiFlows(ids),
    onSuccess: async () => {
      message.success('云枢 AI 模型流已删除成功')
      setSelectedIds([])
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const publishMutation = useMutation({
    mutationFn: async (id: number) => publishAiFlow(id),
    onSuccess: async () => {
      message.success('云枢大模型智能话术流已成功发布上线')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '发布失败'),
  })

  const saveFlowMutation = useMutation({
    mutationFn: async (payload: any) => saveAiFlow(payload),
    onSuccess: async () => {
      message.success('云枢大模型话术基本信息修改成功')
      setIsFlowEditModalOpen(false)
      flowForm.resetFields()
      setEditingFlow(null)
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '修改失败'),
  })

  // Tab 2: CRUD Mutations
  const saveConfigMutation = useMutation({
    mutationFn: async (payload: any) => saveAiModelConfig(payload),
    onSuccess: async () => {
      message.success('大模型厂商配置保存成功')
      setIsConfigModalOpen(false)
      configForm.resetFields()
      setEditingConfig(null)
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-model-configs'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '配置保存失败'),
  })

  const deleteConfigsMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteAiModelConfigs(ids),
    onSuccess: async () => {
      message.success('大模型配置删除成功')
      setSelectedConfigIds([])
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-model-configs'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const handleEditConfig = (record: any) => {
    setEditingConfig(record)
    configForm.setFieldsValue(record)
    setIsConfigModalOpen(true)
  }

  const handleOpenNewConfigModal = () => {
    setEditingConfig(null)
    configForm.resetFields()
    configForm.setFieldsValue({
      provider: 'deepseek',
      temperature: 0.7,
      systemPrompt: '您是云枢智能客服话务员，请根据用户的咨询礼貌作答。'
    })
    setIsConfigModalOpen(true)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <Tabs
        activeKey={activeTab}
        onChange={handleTabChange}
        className="w-full"
        type="card"
        items={[
          {
            key: '1',
            label: (
              <span className="flex items-center gap-2">
                <RobotOutlined />
                <span>智能语音话术流编排</span>
              </span>
            ),
            children: (
              <Space direction="vertical" size="large" className="w-full">
                {/* 极客感看板统计卡片区 */}
                <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                  {/* Card 1: 总数 */}
                  <div className="bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/80 p-5 rounded-2xl flex items-center justify-between shadow-sm relative overflow-hidden group hover:border-blue-500/40 hover:shadow-md transition-all duration-300">
                    <div className="absolute -right-6 -bottom-6 text-slate-100/50 dark:text-slate-800/10 text-7xl font-bold font-mono pointer-events-none select-none group-hover:scale-110 transition-transform">
                      TOTAL
                    </div>
                    <div className="space-y-1">
                      <span className="text-[12px] text-slate-500 dark:text-slate-400 font-medium tracking-wide block">话术流总部署数</span>
                      <span className="text-3xl font-bold font-mono text-slate-800 dark:text-slate-100">{stats.total}</span>
                    </div>
                    <div className="w-12 h-12 rounded-2xl bg-blue-50 dark:bg-blue-950/40 border border-blue-100/50 dark:border-blue-800/20 flex items-center justify-center text-blue-600 dark:text-blue-400 shadow-sm">
                      <CustomerServiceOutlined style={{ fontSize: '20px' }} />
                    </div>
                  </div>

                  {/* Card 2: 已发布 */}
                  <div className="bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/80 p-5 rounded-2xl flex items-center justify-between shadow-sm relative overflow-hidden group hover:border-emerald-500/40 hover:shadow-md transition-all duration-300">
                    <div className="absolute -right-6 -bottom-6 text-slate-100/50 dark:text-slate-800/10 text-7xl font-bold font-mono pointer-events-none select-none group-hover:scale-110 transition-transform">
                      LIVE
                    </div>
                    <div className="space-y-1">
                      <span className="text-[12px] text-slate-500 dark:text-slate-400 font-medium tracking-wide block">线上生产运行环境</span>
                      <span className="text-3xl font-bold font-mono text-emerald-600 dark:text-emerald-400">{stats.published}</span>
                    </div>
                    <div className="w-12 h-12 rounded-2xl bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-100/50 dark:border-emerald-800/20 flex items-center justify-center text-emerald-600 dark:text-emerald-400 shadow-sm">
                      <PhoneOutlined style={{ fontSize: '20px' }} />
                    </div>
                  </div>

                  {/* Card 3: 草稿 */}
                  <div className="bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/80 p-5 rounded-2xl flex items-center justify-between shadow-sm relative overflow-hidden group hover:border-amber-500/40 hover:shadow-md transition-all duration-300">
                    <div className="absolute -right-6 -bottom-6 text-slate-100/50 dark:text-slate-800/10 text-7xl font-bold font-mono pointer-events-none select-none group-hover:scale-110 transition-transform">
                      DRAFT
                    </div>
                    <div className="space-y-1">
                      <span className="text-[12px] text-slate-500 dark:text-slate-400 font-medium tracking-wide block">草稿与预检状态</span>
                      <span className="text-3xl font-bold font-mono text-amber-600 dark:text-amber-400">{stats.drafts}</span>
                    </div>
                    <div className="w-12 h-12 rounded-2xl bg-amber-50 dark:bg-amber-950/40 border border-amber-100/50 dark:border-amber-800/20 flex items-center justify-center text-amber-600 dark:text-amber-400 shadow-sm">
                      <ReloadOutlined style={{ fontSize: '20px' }} />
                    </div>
                  </div>
                </div>

                <QueryBar
                  fields={queryFields}
                  onSearch={setQueryParams}
                  loading={isFlowsLoading}
                />

                <div className="flex justify-between items-center mb-2">
                  <div className="text-slate-500 dark:text-slate-400 text-xs">
                    您可以在画布中通过<strong>图形可视化编排</strong>的方式组装智能话术流，实现实时语音推流及排队调度。
                  </div>
                  <Space>
                    <Button 
                      icon={<ReloadOutlined />} 
                      onClick={() => queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })}
                      className="bg-white text-slate-600 hover:text-slate-850 hover:border-slate-400 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300 dark:hover:text-slate-100"
                    >
                      刷新列表
                    </Button>
                    {selectedIds.length > 0 && (
                      <Popconfirm
                        title={`确定要删除选中的 ${selectedIds.length} 个模型流吗？`}
                        onConfirm={() => deleteMutation.mutate(selectedIds)}
                        okText="确定"
                        cancelText="取消"
                      >
                        <Button danger icon={<DeleteOutlined />}>批量删除</Button>
                      </Popconfirm>
                    )}
                    <Button 
                      type="primary" 
                      icon={<PlusOutlined />} 
                      onClick={() => navigate('/merchant/ai-model-flow/designer/new')}
                      style={{ background: 'linear-gradient(135deg, #0284c7 0%, #0369a1 100%)', border: 'none' }}
                      className="shadow-[0_4px_14px_rgba(3,105,161,0.4)]"
                    >
                      新建智能语音流
                    </Button>
                  </Space>
                </div>

                <TableWrap
                  title="云枢大模型话术列表"
                  rowKey="id"
                  loading={isFlowsLoading}
                  dataSource={filteredRecords}
                  rowSelection={{
                    selectedRowKeys: selectedIds,
                    onChange: (keys: any[]) => setSelectedIds(keys as number[]),
                  }}
                  pagination={{
                    current: pageNumber,
                    pageSize,
                    total: flowsData?.total ?? 0,
                    onChange: (current, size) => {
                      setPageNumber(current)
                      setPageSize(size ?? pageSize)
                    },
                    showSizeChanger: true,
                  }}
                  columns={[
                    { 
                      title: '模型流 ID', 
                      dataIndex: 'id', 
                      width: 100,
                      render: (id) => <span className="font-mono text-slate-600 dark:text-slate-400 font-bold bg-slate-50 dark:bg-slate-900 border border-slate-200/60 dark:border-slate-800/80 px-2 py-0.5 rounded-lg text-xs"># {String(id).padStart(2, '0')}</span>
                    },
                    { 
                      title: '话术名称', 
                      dataIndex: 'name', 
                      render: (val, record: any) => (
                        <Space>
                          {record.status === 'published' ? (
                            <PhoneOutlined className="text-emerald-500" />
                          ) : (
                            <CustomerServiceOutlined className="text-amber-500" />
                          )}
                          <span className="text-slate-800 dark:text-slate-200 font-semibold text-[13px]">{val}</span>
                        </Space>
                      )
                    },
                    { 
                      title: '商户作用域', 
                      dataIndex: 'merchant', 
                      render: () => <span className="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-semibold border border-blue-100 bg-blue-50/50 text-blue-700 dark:border-blue-900/40 dark:bg-blue-950/20 dark:text-blue-400">当前商户独立托管</span>
                    },
                    { 
                      title: '触发 Prompt 提示词', 
                      dataIndex: 'prompt', 
                      ellipsis: true,
                      render: (val) => (
                        <Tooltip title={val}>
                          <span className="font-mono text-xs text-slate-600 dark:text-slate-400 bg-slate-50 dark:bg-slate-900/40 px-2.5 py-1 rounded-md border border-slate-200 dark:border-slate-800 cursor-help block max-w-[280px] truncate">
                            {val || '未定义提示词'}
                          </span>
                        </Tooltip>
                      )
                    },
                    {
                      title: '状态',
                      dataIndex: 'status',
                      width: 140,
                      render: (value: string) => {
                        if (value === 'published') {
                          return (
                            <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold border border-emerald-200 dark:border-emerald-500/30 text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/40 shadow-sm dark:shadow-[0_0_8px_rgba(16,185,129,0.2)]">
                              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 mr-1.5 animate-ping" />
                              已发布上线
                            </span>
                          )
                        }
                        return (
                          <span className="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold border border-amber-200 dark:border-amber-500/30 text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/40 shadow-sm dark:shadow-[0_0_8px_rgba(245,158,11,0.2)]">
                            <span className="w-1.5 h-1.5 rounded-full bg-amber-400 mr-1.5" />
                            草稿 (预检)
                          </span>
                        )
                      },
                    },
                    { 
                      title: '最新修改时间', 
                      dataIndex: 'updatedAt', 
                      width: 180,
                      render: (val) => <span className="font-mono text-xs text-slate-500 dark:text-slate-400">{val ? val.replace('T', ' ').slice(0, 19) : '-'}</span>
                    },
                    {
                      key: 'actions',
                      width: 260,
                      render: (_, record) => {
                        const isPublished = record.status === 'published'
                        return (
                          <Space size="middle">
                            <Button
                              type="link"
                              disabled={isPublished}
                              onClick={() => publishMutation.mutate(record.id)}
                              className={isPublished ? 'text-slate-400 dark:text-slate-600 cursor-not-allowed p-0' : 'text-emerald-600 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300 font-medium p-0'}
                            >
                              发布上线
                            </Button>
                            <Button 
                              type="link" 
                              onClick={() => {
                                setEditingFlow(record)
                                flowForm.setFieldsValue(record)
                                setIsFlowEditModalOpen(true)
                              }}
                              className="text-sky-600 hover:text-sky-700 dark:text-sky-400 dark:hover:text-sky-300 font-medium p-0"
                            >
                              编辑
                            </Button>
                            <Button 
                              type="link" 
                              icon={<SettingOutlined />} 
                              onClick={() => navigate(`/merchant/ai-model-flow/designer/${record.id}`)}
                              className="text-sky-600 hover:text-sky-700 dark:text-sky-400 dark:hover:text-sky-300 font-medium p-0"
                            >
                              编排画布
                            </Button>
                            <Popconfirm
                              title="确定要删除该模型流吗？"
                              onConfirm={() => deleteMutation.mutate([record.id])}
                              okText="确定"
                              cancelText="取消"
                            >
                              <Button type="link" danger style={{ padding: 0 }}>
                                删除
                              </Button>
                            </Popconfirm>
                          </Space>
                        )
                      },
                    },
                  ]}
                />
              </Space>
            ),
          },
          {
            key: '2',
            label: (
              <span className="flex items-center gap-2">
                <SlidersOutlined />
                <span>大模型厂商与 API 设置</span>
              </span>
            ),
            children: (
              <Space direction="vertical" size="large" className="w-full">
                <div className="bg-slate-50 dark:bg-slate-900/30 border border-slate-100 dark:border-slate-800/80 p-5 rounded-2xl flex justify-between items-center shadow-sm">
                  <div className="space-y-1">
                    <span className="text-slate-800 dark:text-slate-200 font-bold block">🧠 AI 大模型全局配置中心</span>
                    <span className="text-slate-500 dark:text-slate-400 text-xs block">
                      在此集中配置 DeepSeek, OpenAI 或自研私有大模型的 API 金钥和 Endpoint，在 IVR 可视化连线图的开始节点即可快速选择绑定，一处修改、全局生效。
                    </span>
                  </div>
                  <Space>
                    <Button 
                      icon={<ReloadOutlined />} 
                      onClick={() => queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-model-configs'] })}
                      className="bg-white text-slate-600 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300"
                    >
                      刷新
                    </Button>
                    {selectedConfigIds.length > 0 && (
                      <Popconfirm
                        title="确定要删除选中的大模型配置吗？"
                        onConfirm={() => deleteConfigsMutation.mutate(selectedConfigIds)}
                      >
                        <Button danger icon={<DeleteOutlined />}>删除配置</Button>
                      </Popconfirm>
                    )}
                    <Button
                      type="primary"
                      icon={<PlusOutlined />}
                      onClick={handleOpenNewConfigModal}
                      style={{ background: 'linear-gradient(135deg, #0284c7 0%, #0369a1 100%)', border: 'none' }}
                      className="shadow-[0_4px_14px_rgba(3,105,161,0.4)]"
                    >
                      添加 AI 模型配置
                    </Button>
                  </Space>
                </div>

                <TableWrap
                  title="已配置的大模型服务列表"
                  rowKey="id"
                  loading={isConfigsLoading}
                  dataSource={configsData ?? []}
                  rowSelection={{
                    selectedRowKeys: selectedConfigIds,
                    onChange: (keys: any[]) => setSelectedConfigIds(keys as number[]),
                  }}
                  pagination={false}
                  columns={[
                    {
                      title: '配置名称',
                      dataIndex: 'name',
                      render: (val, record: any) => (
                        <div className="space-y-0.5">
                          <span className="text-slate-800 dark:text-slate-200 font-semibold">{val}</span>
                          {record.description && (
                            <span className="text-slate-500 dark:text-slate-400 text-[11px] block">{record.description}</span>
                          )}
                        </div>
                      )
                    },
                    {
                      title: '服务商 (Provider)',
                      dataIndex: 'provider',
                      render: (val) => {
                        const lowerVal = String(val || '').toLowerCase()
                        const found = providers.find((p: any) => p.value === lowerVal || (p.value === 'mock' && lowerVal === 'cloud枢私有大模型'))
                        if (found) {
                          if (!found.supportLlm) {
                            return <Tag color="default">⚙️ {found.label} (未接入大模型驱动)</Tag>
                          }
                          return <Tag color={found.color}>{found.emoji} {found.label}</Tag>
                        }
                        return <Tag color="blue">{val}</Tag>
                      }
                    },
                    {
                      title: '模型名称',
                      dataIndex: 'modelName',
                      render: (val) => <span className="font-mono text-xs bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-800 px-2 py-0.5 rounded text-slate-700 dark:text-slate-300">{val}</span>
                    },
                    {
                      title: '网关地址 (Endpoint)',
                      dataIndex: 'endpoint',
                      render: (val) => <span className="font-mono text-xs text-slate-500 block truncate max-w-[240px]">{val || '官方默认'}</span>
                    },
                    {
                      title: 'API 密钥',
                      dataIndex: 'apiKey',
                      render: (val) => <span className="font-mono text-xs text-slate-400">••••••••••••••••</span>
                    },
                    {
                      title: '动作',
                      key: 'actions',
                      width: 140,
                      render: (_, record: any) => (
                        <Space>
                          <Button
                            type="link"
                            onClick={() => handleEditConfig(record)}
                            className="text-sky-600 hover:text-sky-700 dark:text-sky-400 dark:hover:text-sky-300 font-medium p-0"
                          >
                            编辑配置
                          </Button>
                          <Popconfirm
                            title="确定要删除该大模型配置吗？"
                            onConfirm={() => deleteConfigsMutation.mutate([record.id])}
                          >
                            <Button type="link" danger style={{ padding: 0 }}>
                              删除
                            </Button>
                          </Popconfirm>
                        </Space>
                      )
                    }
                  ]}
                />
              </Space>
            )
          }
        ]}
      />

      {/* AI 模型配置编辑模态弹窗 */}
      <Modal
        title={editingConfig ? "🧠 编辑大模型配置" : "🧠 新增大模型配置"}
        open={isConfigModalOpen}
        onCancel={() => setIsConfigModalOpen(false)}
        footer={null}
        width={600}
        destroyOnHidden
      >
        <Form
          form={configForm}
          layout="vertical"
          onFinish={(values) => saveConfigMutation.mutate({ ...values, id: editingConfig?.id })}
          initialValues={{
            temperature: 0.7,
            provider: 'deepseek'
          }}
          className="mt-4"
        >
          <Form.Item
            name="name"
            label="配置名称"
            rules={[{ required: true, message: '请输入配置名称，便于编排时识别' }]}
          >
            <Input placeholder="例如: DeepSeek-V3官方、OpenAI-gpt-4o" />
          </Form.Item>

          <div className="grid grid-cols-2 gap-4">
            <Form.Item
              name="provider"
              label="大模型服务商"
              rules={[{ required: true }]}
            >
              <Select>
                {providers.map((p: any) => (
                  <Select.Option key={p.value} value={p.value} disabled={!p.supportLlm}>
                    {p.emoji} {p.label} {!p.supportLlm && ' ⚠️ (不支持大模型决断)'}
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>

            <Form.Item
              name="modelName"
              label="大模型名称 (Model Name)"
              rules={[{ required: true, message: '请输入模型名称' }]}
            >
              <Input placeholder="例如: deepseek-chat 或 gpt-4o" />
            </Form.Item>
          </div>

          <Form.Item
            name="endpoint"
            label="API 代理网关地址 (Endpoint)"
          >
            <Input placeholder="空代表使用厂商默认 Endpoint" />
          </Form.Item>

          <Form.Item
            name="apiKey"
            label="API 密钥 (API Key)"
            rules={[{ required: true, message: '请输入 API 密钥建立物理握手' }]}
          >
            <Input.Password placeholder="输入大模型 API Key" />
          </Form.Item>

          <Form.Item
            name="temperature"
            label="生成温度 (Temperature)"
          >
            <Slider min={0.0} max={1.5} step={0.1} />
          </Form.Item>

          <Form.Item
            name="systemPrompt"
            label="全局 System Prompt 角色提示词"
          >
            <Input.TextArea rows={4} placeholder="您是云枢智能话务员，请根据用户的提问礼貌回答..." />
          </Form.Item>

          <Form.Item noStyle shouldUpdate={(prev, curr) => prev.provider !== curr.provider}>
            {({ getFieldValue }) => {
              const provider = getFieldValue('provider') || 'deepseek'
              
              if (provider === 'volc') {
                return (
                  <div className="border-t border-slate-200 dark:border-slate-800/80 my-4 pt-3">
                    <span className="text-slate-800 dark:text-slate-200 font-bold text-xs block mb-3">🌋 火山引擎豆包语音配置 (ASR/TTS 可选)</span>
                    
                    <div className="grid grid-cols-2 gap-4">
                      <Form.Item
                        name="volcAppId"
                        label="火山语音 AppId"
                      >
                        <Input placeholder="Application ID" />
                      </Form.Item>

                      <Form.Item
                        name="volcCluster"
                        label="ASR 集群"
                      >
                        <Input placeholder="ASR 默认 volc_common_asr" />
                      </Form.Item>
                    </div>

                    <Form.Item
                      name="volcToken"
                      label="火山语音 Access Token"
                    >
                      <Input.Password placeholder="火山 OpenSpeech 鉴权 Token" />
                    </Form.Item>

                    <div className="grid grid-cols-2 gap-4">
                      <Form.Item
                        name="volcVoiceType"
                        label="豆包发音人音色"
                      >
                        <Select style={{ width: '100%' }} placeholder="默认豆包女声">
                          <Select.Option value="bv001_streaming">🎤 豆包女声 (极具情感)</Select.Option>
                          <Select.Option value="bv002_streaming">🎙️ 豆包男声 (专业高保真)</Select.Option>
                          <Select.Option value="bv051_streaming">📚 豆包说书 (自然流畅)</Select.Option>
                          <Select.Option value="bv004_streaming">🎮 豆包游戏 (朝气灵动)</Select.Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="volcSpeedRatio"
                        label="TTS 朗读语速"
                      >
                        <InputNumber min={0.5} max={2.0} step={0.1} style={{ width: '100%' }} placeholder="默认 1.0" />
                      </Form.Item>
                    </div>
                  </div>
                )
              }

              if (provider === 'ali') {
                return (
                  <div className="border-t border-slate-200 dark:border-slate-800/80 my-4 pt-3">
                    <span className="text-slate-800 dark:text-sky-400 font-bold text-xs block mb-3">☁️ 阿里云 ASR/TTS 语音设置 (可选)</span>
                    
                    <div className="grid grid-cols-2 gap-4">
                      <Form.Item
                        name="volcAppId"
                        label="阿里语音 AppKey"
                      >
                        <Input placeholder="Ali AppKey" />
                      </Form.Item>

                      <Form.Item
                        name="volcToken"
                        label="阿里语音 Access Token"
                      >
                        <Input.Password placeholder="阿里云 智能语音交互 Token" />
                      </Form.Item>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <Form.Item
                        name="volcVoiceType"
                        label="阿里发音人音色"
                      >
                        <Select style={{ width: '100%' }} placeholder="默认使用小云女声">
                          <Select.Option value="Xiaoyun">🎤 小云 (标准女声)</Select.Option>
                          <Select.Option value="Xiaoyu">🎙️ 小宇 (温柔女声)</Select.Option>
                          <Select.Option value="Xiaoting">📚 小婷 (甜美英文)</Select.Option>
                          <Select.Option value="Siyue">🎙️ 思悦 (温柔男声)</Select.Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="volcSpeedRatio"
                        label="TTS 朗读语速"
                      >
                        <InputNumber min={0.5} max={2.0} step={0.1} style={{ width: '100%' }} placeholder="默认 1.0" />
                      </Form.Item>
                    </div>
                  </div>
                )
              }

              if (provider === 'tencent') {
                return (
                  <div className="border-t border-slate-200 dark:border-slate-800/80 my-4 pt-3">
                    <span className="text-slate-800 dark:text-sky-400 font-bold text-xs block mb-3">🐧 腾讯云 ASR/TTS 语音设置 (可选)</span>
                    
                    <div className="grid grid-cols-2 gap-4">
                      <Form.Item
                        name="volcAppId"
                        label="腾讯云 SecretId"
                      >
                        <Input placeholder="SecretId" />
                      </Form.Item>

                      <Form.Item
                        name="volcToken"
                        label="腾讯云 SecretKey"
                      >
                        <Input.Password placeholder="SecretKey" />
                      </Form.Item>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <Form.Item
                        name="volcVoiceType"
                        label="腾讯发音人音色"
                      >
                        <Select style={{ width: '100%' }} placeholder="默认使用智雅女声">
                          <Select.Option value="101001">🎤 智雅 (标准女声)</Select.Option>
                          <Select.Option value="101002">🎙️ 智宽 (标准男声)</Select.Option>
                          <Select.Option value="101016">📚 智美 (亲切女声)</Select.Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="volcSpeedRatio"
                        label="TTS 朗读语速"
                      >
                        <InputNumber min={0.5} max={2.0} step={0.1} style={{ width: '100%' }} placeholder="默认 1.0" />
                      </Form.Item>
                    </div>
                  </div>
                )
              }

              if (provider === 'openai') {
                return (
                  <div className="border-t border-slate-200 dark:border-slate-800/80 my-4 pt-3">
                    <span className="text-slate-800 dark:text-sky-400 font-bold text-xs block mb-3">🌐 OpenAI 语音设置 (使用上方 API Key 物理鉴权)</span>
                    
                    <div className="grid grid-cols-2 gap-4">
                      <Form.Item
                        name="volcVoiceType"
                        label="OpenAI 明星发音人"
                      >
                        <Select style={{ width: '100%' }} placeholder="默认使用 Alloy 音色">
                          <Select.Option value="alloy">🎤 Alloy (中性高拟真)</Select.Option>
                          <Select.Option value="echo">🎙️ Echo (温柔男声)</Select.Option>
                          <Select.Option value="fable">📚 Fable (动感叙事)</Select.Option>
                          <Select.Option value="onyx">🎙️ Onyx (低沉男声)</Select.Option>
                          <Select.Option value="nova">🎤 Nova (明亮女声)</Select.Option>
                          <Select.Option value="shimmer">🎙️ Shimmer (朝气女声)</Select.Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="volcSpeedRatio"
                        label="TTS 朗读语速"
                      >
                        <InputNumber min={0.5} max={2.0} step={0.1} style={{ width: '100%' }} placeholder="默认 1.0" />
                      </Form.Item>
                    </div>
                  </div>
                )
              }

              if (provider === 'deepseek') {
                return (
                  <div className="border-t border-slate-200 dark:border-slate-800/80 my-4 pt-3 text-slate-500 text-xs bg-slate-50 dark:bg-slate-900/40 p-4 rounded-xl leading-relaxed">
                    💡 <strong>DeepSeek 厂商专属提示</strong>：DeepSeek 官方主要提供卓越的大语言模型推理决断能力。其语音识别 (ASR) 与合成 (TTS) 服务，建议您在连线流图画布中级联绑定火山或阿里语音服务进行混合推流。此处无需额外配置专属语音参数。
                  </div>
                )
              }

              if (provider === 'zhipu') {
                return (
                  <div className="border-t border-slate-200 dark:border-slate-800/80 my-4 pt-3 text-slate-500 text-xs bg-slate-50 dark:bg-slate-900/40 p-4 rounded-xl leading-relaxed">
                    💡 <strong>智谱AI 厂商专属提示</strong>：智谱AI 提供 GLM 系列大语言模型推理能力，默认使用 glm-4 模型，API 地址为 https://open.bigmodel.cn/api/paas/v4/chat/completions。语音识别 (ASR) 与合成 (TTS) 服务，建议您在连线流图画布中级联绑定其他服务商的语音服务进行混合推流。此处无需额外配置专属语音参数。
                  </div>
                )
              }

              return null
            }}
          </Form.Item>

          <Form.Item
            name="description"
            label="配置描述"
          >
            <Input placeholder="可在此输入配置备注" />
          </Form.Item>

          <Form.Item className="mb-0 text-right mt-6">
            <Space>
              <Button onClick={() => setIsConfigModalOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={saveConfigMutation.isPending}>
                保存配置
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* AI 智能流基本属性修改模态框 */}
      <Modal
        title="🤖 编辑智能话术流程基本信息"
        open={isFlowEditModalOpen}
        onCancel={() => setIsFlowEditModalOpen(false)}
        footer={null}
        width={600}
        destroyOnHidden
      >
        <Form
          form={flowForm}
          layout="vertical"
          onFinish={(values) => saveFlowMutation.mutate({ ...editingFlow, ...values })}
          className="mt-4"
        >
          <Form.Item
            name="name"
            label="话术名称"
            rules={[{ required: true, message: '请输入智能话术流名称，用于列表分类' }]}
          >
            <Input placeholder="例如: 智能话费查询、业务导航话术" />
          </Form.Item>
          <Form.Item
            name="prompt"
            label="触发 Prompt 提示词"
            rules={[{ required: true, message: '请输入话术触发的全局提示词' }]}
          >
            <Input.TextArea rows={4} placeholder="例如: 你是一个云枢智能电话应答助手。根据客户说的话，分发到对应节点。" />
          </Form.Item>
          <Form.Item
            name="description"
            label="备注描述"
          >
            <Input placeholder="可在此处输入话术的说明与备注" />
          </Form.Item>
          <Form.Item className="mb-0 text-right mt-6">
            <Space>
              <Button onClick={() => setIsFlowEditModalOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={saveFlowMutation.isPending}>
                保存修改
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
