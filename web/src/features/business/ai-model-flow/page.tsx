import { Button, Popconfirm, Space, Tag, Typography, message, Tooltip } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import {
  PlusOutlined,
  DeleteOutlined,
  SettingOutlined,
  ReloadOutlined,
  PhoneOutlined,
  CustomerServiceOutlined
} from '@ant-design/icons'
import {
  fetchAiFlows,
  deleteAiFlows,
  publishAiFlow
} from '@/api/operate'

const { Text } = Typography

export function AiModelFlowPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  const navigate = useNavigate()
  const queryClient = useQueryClient()

  // 读取 AI 智能流列表
  const { data, isLoading } = useQuery({
    queryKey: ['merchant', 'ai-flow', pageNumber, pageSize],
    queryFn: () => fetchAiFlows(pageNumber, pageSize),
  })

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
    let records = data?.records ?? []
    if (queryParams.name) {
      records = records.filter((r: any) => String(r.name).toLowerCase().includes(queryParams.name.toLowerCase().trim()))
    }
    if (queryParams.status) {
      records = records.filter((r: any) => r.status === queryParams.status)
    }
    return records
  }, [data, queryParams])

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

  return (
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
        loading={isLoading}
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
            <PermissionGate permission="merchant:ai-flow:write">
              <Popconfirm
                title={`确定要删除选中的 ${selectedIds.length} 个模型流吗？`}
                onConfirm={() => deleteMutation.mutate(selectedIds)}
                okText="确定"
                cancelText="取消"
              >
                <Button danger icon={<DeleteOutlined />}>批量删除</Button>
              </Popconfirm>
            </PermissionGate>
          )}
          <PermissionGate permission="merchant:ai-flow:write">
            <Button 
              type="primary" 
              icon={<PlusOutlined />} 
              onClick={() => navigate('/merchant/ai-model-flow/designer/new')}
              style={{ background: 'linear-gradient(135deg, #0284c7 0%, #0369a1 100%)', border: 'none' }}
              className="shadow-[0_4px_14px_rgba(3,105,161,0.4)]"
            >
              新建智能语音流
            </Button>
          </PermissionGate>
        </Space>
      </div>

      <TableWrap
        title="云枢大模型话术列表"
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
                  <PermissionGate permission="merchant:ai-flow:publish">
                    <Button
                      type="link"
                      disabled={isPublished}
                      onClick={() => publishMutation.mutate(record.id)}
                      className={isPublished ? 'text-slate-400 dark:text-slate-600 cursor-not-allowed p-0' : 'text-emerald-600 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300 font-medium p-0'}
                    >
                      发布上线
                    </Button>
                  </PermissionGate>
                  <PermissionGate permission="merchant:ai-flow:write">
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
                  </PermissionGate>
                </Space>
              )
            },
          },
        ]}
      />
    </Space>
  )
}
