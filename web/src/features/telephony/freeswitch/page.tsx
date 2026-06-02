import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Switch, Tag, Typography, message, Card } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { fetchFsNodes, saveFsNode, deleteFsNode, toggleFsNodeEnable } from '@/api/operate'

type NodeFormValues = {
  id?: number
  address: string
  localAddress?: string
  eslPort: number
  sipPort?: number
  cmdPort?: number
  password?: string
  setId?: number
  weight?: number
  cc?: number
  enable: boolean
}

export function FreeSwitchPage() {
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<NodeFormValues>()
  const queryClient = useQueryClient()
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  const { data, isLoading } = useQuery({
    queryKey: ['operate', 'freeswitch'],
    queryFn: fetchFsNodes,
    refetchInterval: 4000, // 每 4 秒定时自动轮询，保证掉线在 5 秒内实时反馈
  })

  const queryFields = useMemo(() => [
    { key: 'address', label: '节点地址', type: 'text' as const, placeholder: '请输入 IP 或域名模糊搜索' },
    {
      key: 'enable',
      label: '启用状态',
      type: 'select' as const,
      options: [
        { value: true, label: '启用' },
        { value: false, label: '停用' },
      ],
    },
  ], [])

  // 优雅的客户端精细化组合条件过滤 (Progressive Enhancement)
  const filteredRecords = useMemo(() => {
    let records = data ?? []
    if (queryParams.address) {
      records = records.filter((r: any) => String(r.address).toLowerCase().includes(queryParams.address.toLowerCase().trim()))
    }
    if (queryParams.enable !== undefined) {
      records = records.filter((r: any) => Boolean(r.enable) === Boolean(queryParams.enable))
    }
    return records
  }, [data, queryParams])


  const toggleMutation = useMutation({
    mutationFn: async ({ id, enable }: { id: number; enable: boolean }) => toggleFsNodeEnable(id, enable),
    onSuccess: async () => {
      message.success('节点状态已更新，别忘了刷新运行时')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'freeswitch'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '操作失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => deleteFsNode(id),
    onSuccess: async () => {
      message.success('节点已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'freeswitch'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: NodeFormValues) => saveFsNode(values),
    onSuccess: async () => {
      message.success(editingId ? '节点已更新' : '节点已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'freeswitch'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ enable: true, eslPort: 8021, sipPort: 5060, cmdPort: 8080, setId: 1, weight: 100, cc: 1000 })
    }, 0)
  }

  function openEdit(record: any) {
    setEditingId(record.id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id: record.id,
        address: record.address ?? '',
        localAddress: record.localAddress ?? '',
        eslPort: record.eslPort ?? 8021,
        sipPort: record.sipPort ?? 5060,
        cmdPort: record.cmdPort ?? 8080,
        password: '', // Keep blank for security during edit unless changed
        setId: record.setId ?? 1,
        weight: record.weight ?? 100,
        cc: record.maxChannels ?? 1000,
        enable: Boolean(record.enable),
      })
    }, 0)
  }

  // 软交换集群指标计算
  const totalActiveCalls = data?.reduce((sum: number, node: any) => sum + (node.activeCalls ?? 0), 0) ?? 0
  const totalMaxChannels = data?.reduce((sum: number, node: any) => sum + (node.maxChannels ?? 1000), 0) ?? 0
  const activeNodesCount = data?.filter((n: any) => n.enable).length ?? 0
  const onlineNodesCount = data?.filter((n: any) => n.status === 'active').length ?? 0
  const loadPercentage = totalMaxChannels > 0 ? (totalActiveCalls / totalMaxChannels) * 100 : 0

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 软交换核心指标集群看板 */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-2 animate-fade-in">
        {/* 1. 软交换集群规模 & 健康度 */}
        <Card bordered={false} className="shadow-sm rounded-xl bg-gradient-to-br from-blue-50 to-indigo-100/50 dark:from-indigo-950/30 dark:to-slate-900/30 border border-blue-100/50 dark:border-indigo-900/20 p-4 transition-all duration-300">
          <div className="flex justify-between items-start">
            <div>
              <div className="text-[10px] font-bold text-indigo-500 dark:text-indigo-400 uppercase font-mono tracking-wider">软交换核心集群</div>
              <div className="text-xl font-extrabold text-slate-800 dark:text-zinc-100 mt-1.5 flex items-baseline gap-1">
                {onlineNodesCount}
                <span className="text-xs font-normal text-slate-400 dark:text-zinc-500">/ {data?.length ?? 0} 在线</span>
              </div>
              <div className="text-[11px] text-slate-500 dark:text-zinc-400 mt-2 font-mono space-y-0.5">
                <div>已启用节点: <span className="font-semibold text-slate-700 dark:text-zinc-200">{activeNodesCount} 个</span></div>
                <div>整体运行健康度: <span className="font-semibold text-emerald-500">{data?.length ? `${((onlineNodesCount / data.length) * 100).toFixed(0)}%` : '0%'}</span></div>
              </div>
            </div>
            <Tag color={onlineNodesCount > 0 ? "success" : "error"} style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }} className="font-mono">
              {onlineNodesCount > 0 ? "CLUSTER_HEALTHY" : "NO_ACTIVE_NODE"}
            </Tag>
          </div>
        </Card>

        {/* 2. 话务并发负载率 */}
        <Card bordered={false} className="shadow-sm rounded-xl bg-gradient-to-br from-purple-50 to-fuchsia-100/50 dark:from-purple-950/30 dark:to-slate-900/30 border border-purple-100/50 dark:border-purple-900/20 p-4 transition-all duration-300">
          <div className="flex justify-between items-start">
            <div className="w-full">
              <div className="text-[10px] font-bold text-purple-500 dark:text-purple-400 uppercase font-mono tracking-wider">并发与话务负载</div>
              <div className="text-xl font-extrabold text-slate-800 dark:text-zinc-100 mt-1.5 flex items-baseline gap-1">
                {totalActiveCalls}
                <span className="text-xs font-normal text-slate-400 dark:text-zinc-500">/ {totalMaxChannels} CC</span>
              </div>
              <div className="mt-2.5">
                <div className="w-full bg-slate-200 dark:bg-zinc-800 rounded-full h-1.5 overflow-hidden">
                  <div 
                    className="bg-gradient-to-r from-purple-500 to-fuchsia-500 h-1.5 rounded-full transition-all duration-500" 
                    style={{ width: `${Math.min(loadPercentage, 100)}%` }}
                  />
                </div>
                <div className="flex justify-between items-center text-[10px] text-slate-400 dark:text-zinc-500 font-mono mt-1">
                  <span>并发负载率</span>
                  <span className="font-semibold text-purple-600 dark:text-purple-400">{loadPercentage.toFixed(1)}%</span>
                </div>
              </div>
            </div>
          </div>
        </Card>

        {/* 3. 物理节点与 ESL 连接 */}
        <Card bordered={false} className="shadow-sm rounded-xl bg-gradient-to-br from-emerald-50 to-teal-100/50 dark:from-emerald-950/30 dark:to-slate-900/30 border border-emerald-100/50 dark:border-emerald-900/20 p-4 transition-all duration-300">
          <div className="flex justify-between items-start">
            <div className="w-full">
              <div className="text-[10px] font-bold text-emerald-600 dark:text-emerald-400 uppercase font-mono tracking-wider">节点负载分配</div>
              {data && data.length > 0 ? (
                <div className="space-y-1.5 mt-2">
                  {data.slice(0, 2).map((node: any) => {
                    const isOnline = node.status === 'active';
                    return (
                      <div key={node.id} className="flex items-center justify-between text-xs w-full">
                        <span className="font-bold text-slate-800 dark:text-zinc-100 flex items-center gap-1.5 truncate max-w-[140px]">
                          <span className={`w-1.5 h-1.5 rounded-full ${isOnline ? 'bg-emerald-500 animate-pulse' : 'bg-rose-500 animate-ping'} inline-block`} />
                          {node.address}:{node.eslPort}
                        </span>
                        <span className="text-[10px] font-mono text-slate-500 dark:text-zinc-400">
                          {node.activeCalls ?? 0} / {node.maxChannels ?? 1000} CC
                        </span>
                      </div>
                    );
                  })}
                  {data.length > 2 && (
                    <div className="text-[9px] text-slate-400 dark:text-zinc-500 font-mono">
                      + 还有 {data.length - 2} 个节点正在轮询调度中
                    </div>
                  )}
                </div>
              ) : (
                <div className="text-sm font-bold text-slate-800 dark:text-zinc-100 mt-2 flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-rose-500 animate-ping inline-block" />
                  未检测到可用物理节点
                </div>
              )}
            </div>
          </div>
        </Card>
      </div>

      <QueryBar
        fields={queryFields}
        onSearch={setQueryParams}
        loading={isLoading}
      />

      <div className="flex justify-end items-center mb-2">
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'freeswitch'] })}>刷新状态</Button>
          <PermissionGate permission="operate:freeswitch:write">
            <Button type="primary" onClick={openCreate}>新增节点</Button>
          </PermissionGate>
        </Space>
      </div>

      <TableWrap
        title="软交换节点列表"
        rowKey="id"
        loading={isLoading}
        dataSource={filteredRecords}
        columns={[
          { title: '名称', dataIndex: 'name' },
          { title: '内网/外网地址', render: (_, r) => `${r.address}:${r.eslPort} ${r.localAddress ? `(${r.localAddress})` : ''}` },
          { title: '权重 / SetID', render: (_, r) => `Set: ${r.setId ?? 1} / W: ${r.weight ?? 100}` },
          { 
            title: '状态', 
            render: (_, record: any) => {
              if (!record.enable) {
                return <Tag color="default" style={{ border: 'none' }}>已停用</Tag>
              }
              if (record.status === 'active') {
                return (
                  <Tag color="success" style={{ border: 'none' }} className="flex items-center w-fit gap-1 font-semibold text-emerald-600">
                    <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse inline-block" />
                    在线启用
                  </Tag>
                )
              }
              if (record.status === 'draining') {
                return (
                  <Tag color="warning" style={{ border: 'none' }} className="flex items-center w-fit gap-1 font-semibold text-amber-600">
                    <span className="w-1.5 h-1.5 rounded-full bg-amber-500 animate-pulse inline-block" />
                    正在排空
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
          { title: '租约持有者', dataIndex: 'owner' },
          { title: '活跃通话/并发上限', render: (_, record) => `${record.activeCalls}/${record.maxChannels}` },
          { title: '更新时间', dataIndex: 'updatedAt', render: (val) => val ? new Date(val).toLocaleString() : '-' },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <PermissionGate permission="operate:freeswitch:write">
                  <Button size="small" onClick={() => openEdit(record)}>
                    编辑
                  </Button>
                </PermissionGate>
                <PermissionGate permission="operate:freeswitch:write">
                  <Button size="small" onClick={() => toggleMutation.mutate({ id: record.id, enable: !record.enable })}>
                    {record.enable ? '停用' : '启用'}
                  </Button>
                </PermissionGate>
                <PermissionGate permission="operate:freeswitch:delete">
                  <Popconfirm title="确认删除这个软交换节点？" onConfirm={() => deleteMutation.mutate(record.id)}>
                    <Button size="small" danger>
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
        title={editingId ? '编辑软交换节点' : '新增软交换节点'}
        width={640}
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
          onFinish={(values) => saveMutation.mutate(values)}
          initialValues={{ enable: true, eslPort: 8021, sipPort: 5060, cmdPort: 8080, setId: 1, weight: 100, cc: 1000 }}
        >
          <Form.Item name="id" hidden>
            <Input />
          </Form.Item>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-1">
            <Form.Item name="address" label="节点 IP 地址 / 主机名" rules={[{ required: true, message: '请输入节点 IP 地址' }]}>
              <Input placeholder="例如: 192.168.1.100" />
            </Form.Item>

            <Form.Item name="localAddress" label="内网 IP 地址 (可选)">
              <Input placeholder="例如: 10.0.0.10" />
            </Form.Item>

            <Form.Item name="eslPort" label="ESL 端口" rules={[{ required: true, message: '请输入 ESL 端口' }]}>
              <InputNumber className="w-full" min={1} max={65535} />
            </Form.Item>

            <Form.Item name="sipPort" label="SIP 端口 (可选)">
              <InputNumber className="w-full" min={1} max={65535} />
            </Form.Item>

            <Form.Item name="cmdPort" label="命令控制端口 (可选)">
              <InputNumber className="w-full" min={1} max={65535} />
            </Form.Item>

            <Form.Item name="password" label="ESL 密码" rules={editingId ? [] : [{ required: true, message: '请输入 ESL 密码' }]}>
              <Input.Password placeholder={editingId ? '留空表示不修改' : '默认: ClueCon'} />
            </Form.Item>

            <Form.Item name="setId" label="Set ID (负载组)">
              <InputNumber className="w-full" min={1} />
            </Form.Item>

            <Form.Item name="weight" label="权重">
              <InputNumber className="w-full" min={1} />
            </Form.Item>

            <Form.Item name="cc" label="最大并发限制 (CC)">
              <InputNumber className="w-full" min={1} />
            </Form.Item>

            <Form.Item name="enable" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          </div>
        </Form>
      </Modal>
    </Space>
  )
}
