import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Switch, Tag, Typography, message, Card, Row, Col } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { fetchRtpenginesPage, saveRtpengine, deleteRtpengines, reloadRtpengines, Rtpengine } from '@/api/operate'
import {
  ClusterOutlined,
  PlusOutlined,
  ReloadOutlined,
  DeleteOutlined,
  SlidersOutlined
} from '@ant-design/icons'

export function MediaConfigPage() {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<Rtpengine>()
  
  // 分页与筛选条件
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  const { data, isLoading } = useQuery({
    queryKey: ['operate', 'rtpengines', page, pageSize],
    queryFn: () => fetchRtpenginesPage({ pageNumber: page, pageSize: pageSize }),
    refetchInterval: 4000, // 每 4 秒定时自动轮询，保证掉线在 5 秒内实时反馈
  })

  const queryFields = useMemo(() => [
    { key: 'rtpengineSock', label: '套接字地址', type: 'text' as const, placeholder: '请输入套接字模糊搜索，如 127.0.0.1' },
    { key: 'description', label: '描述说明', type: 'text' as const, placeholder: '请输入描述模糊搜索' },
  ], [])

  // 优雅的客户端精细化组合条件过滤 (Progressive Enhancement)
  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.rtpengineSock) {
      records = records.filter((r: any) => String(r.rtpengineSock).toLowerCase().includes(queryParams.rtpengineSock.toLowerCase().trim()))
    }
    if (queryParams.description) {
      records = records.filter((r: any) => String(r.description || '').toLowerCase().includes(queryParams.description.toLowerCase().trim()))
    }
    return records
  }, [data?.records, queryParams])

  // 保存节点
  const saveMutation = useMutation({
    mutationFn: async (values: Rtpengine) => saveRtpengine(values),
    onSuccess: (res: any) => {
      message.success(editingId ? '媒体代理节点已更新' : '媒体代理节点已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['operate', 'rtpengines'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  // 删除节点
  const deleteMutation = useMutation({
    mutationFn: async (record: Rtpengine) => deleteRtpengines([{ id: record.id! }]),
    onSuccess: () => {
      message.success('媒体代理节点已成功移除')
      queryClient.invalidateQueries({ queryKey: ['operate', 'rtpengines'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  // 热重载
  const reloadMutation = useMutation({
    mutationFn: async () => reloadRtpengines(),
    onSuccess: () => {
      message.success('Kamailio 媒体代理配置热重载成功，对当前在线呼叫无中断。')
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '热刷新失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ setId: 1, weight: 1, disabled: false })
    }, 0)
  }

  function openEdit(record: Rtpengine) {
    setEditingId(record.id!)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id: record.id,
        setId: record.setId ?? 1,
        rtpengineSock: record.rtpengineSock ?? '',
        weight: record.weight ?? 1,
        disabled: Boolean(record.disabled),
        description: record.description ?? '',
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 全局毛玻璃热刷新加载层 */}
      {reloadMutation.isPending && (
        <div className="fixed inset-0 bg-slate-900/60 dark:bg-black/80 backdrop-blur-md z-[9999] flex flex-col items-center justify-center space-y-6">
          <div className="relative flex items-center justify-center">
            <span className="animate-ping absolute inline-flex h-12 w-12 rounded-full bg-indigo-400 opacity-75"></span>
            <ReloadOutlined className="text-white text-3xl animate-spin" />
          </div>
          <div className="flex flex-col items-center space-y-2 text-center max-w-md px-4">
            <div className="text-white dark:text-zinc-100 font-bold text-base tracking-wider font-mono">
              ⚡️ MEDIA SYSTEM RELOADING
            </div>
            <div className="text-indigo-200 dark:text-zinc-400 text-xs font-mono leading-relaxed">
              正在向信令核心网关（Kamailio）同步动态下发媒体代理连接池。配置将在毫秒级无损更新，请稍候...
            </div>
          </div>
        </div>
      )}

      <div className="flex justify-between items-center mb-2">
        <div>
          <Typography.Title level={4} className="!mb-1 font-bold text-slate-800 dark:text-slate-200">
            媒体配置管理
          </Typography.Title>
        </div>
        <Space>
          <Button
            type="dashed"
            icon={<ReloadOutlined />}
            loading={reloadMutation.isPending}
            onClick={() => reloadMutation.mutate()}
          >
            无损热重载
          </Button>
          <PermissionGate permission="operate:freeswitch:write">
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增媒体节点
            </Button>
          </PermissionGate>
        </Space>
      </div>

      {/* 实时健康度卡片看板 */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-2 animate-fade-in">
        <Card bordered={false} className="shadow-sm rounded-xl bg-gradient-to-br from-slate-50 to-slate-100/50 dark:from-zinc-900/60 dark:to-zinc-950/40 border border-slate-100 dark:border-zinc-850 p-4 transition-all duration-300">
          <div className="flex justify-between items-start">
            <div>
              <div className="text-[10px] font-bold text-slate-400 dark:text-zinc-500 uppercase font-mono tracking-wider">MEDIA SERVICE STATS</div>
              <div className="text-sm font-bold text-slate-800 dark:text-zinc-100 mt-1 flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse inline-block" />
                媒体代理集群
              </div>
              <div className="text-[11px] text-slate-500 dark:text-zinc-400 mt-2 font-mono space-y-0.5">
                <div>已启用且在线: <span className="font-semibold text-emerald-600 dark:text-emerald-400">{data?.records.filter((r) => !r.disabled && r.status === 'online').length ?? 0}</span></div>
                <div>已启用但故障: <span className="font-semibold text-rose-600 dark:text-rose-400">{data?.records.filter((r) => !r.disabled && r.status === 'offline').length ?? 0}</span></div>
                <div>已被禁用节点: <span className="font-semibold text-slate-500 dark:text-zinc-400">{data?.records.filter((r) => r.disabled).length ?? 0}</span></div>
              </div>
            </div>
            {(() => {
              const totalActive = data?.records.filter((r) => !r.disabled).length ?? 0
              const activeOnline = data?.records.filter((r) => !r.disabled && r.status === 'online').length ?? 0
              const activeOffline = data?.records.filter((r) => !r.disabled && r.status === 'offline').length ?? 0

              if (totalActive === 0) {
                return <Tag color="warning" style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }} className="font-mono">NO ACTIVE NODES</Tag>
              }
              if (activeOnline > 0 && activeOffline === 0) {
                return <Tag color="success" style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }} className="font-mono">HEALTHY</Tag>
              }
              if (activeOnline > 0 && activeOffline > 0) {
                return <Tag color="warning" style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }} className="font-mono">DEGRADED</Tag>
              }
              return <Tag color="error" style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }} className="font-mono animate-pulse">ALL OFFLINE</Tag>
            })()}
          </div>
        </Card>

        <Card bordered={false} className="shadow-sm rounded-xl bg-gradient-to-br from-slate-50 to-slate-100/50 dark:from-zinc-900/60 dark:to-zinc-950/40 border border-slate-100 dark:border-zinc-850 p-4 transition-all duration-300">
          <div className="flex justify-between items-start">
            <div className="w-full">
              <div className="text-[10px] font-bold text-slate-400 dark:text-zinc-500 uppercase font-mono tracking-wider">ACTIVE CAPABILITIES</div>
              {data && data.records.length > 0 ? (
                <div className="space-y-1 mt-1.5">
                  {data.records.slice(0, 2).map((node: Rtpengine) => {
                    const dotColor = node.disabled ? 'bg-slate-300' : (node.status === 'online' ? 'bg-emerald-500 animate-pulse' : 'bg-rose-500 animate-ping')
                    return (
                      <div key={node.id} className="flex items-center justify-between text-xs w-full">
                        <span className="font-bold text-slate-800 dark:text-zinc-100 flex items-center gap-1.5 truncate max-w-[150px]">
                          <span className={`w-1.5 h-1.5 rounded-full ${dotColor} inline-block`} />
                          {node.description || `媒体节点 ${node.id}`}
                        </span>
                        <span className="text-[10px] font-mono text-slate-400 dark:text-zinc-500">{node.rtpengineSock}</span>
                      </div>
                    )
                  })}
                </div>
              ) : (
                <div className="text-sm font-bold text-slate-800 dark:text-zinc-100 mt-1 flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-rose-500 animate-ping inline-block" />
                  RTPEngine 网关
                  <div className="text-[10px] text-rose-500 font-medium font-mono mt-1">无活跃媒体节点</div>
                </div>
              )}
            </div>
            {data && data.records.length > 0 ? (
              <Tag color="purple" style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }} className="font-mono">
                {data.total} NODES
              </Tag>
            ) : (
              <Tag color="red" style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }} className="font-mono">OFFLINE</Tag>
            )}
          </div>
        </Card>
      </div>

      <QueryBar
        fields={queryFields}
        onSearch={setQueryParams}
        loading={isLoading}
      />

      {/* 媒体节点 Table 列表 */}
      <TableWrap
        title="媒体代理节点列表"
        rowKey="id"
        loading={isLoading}
        dataSource={filteredRecords}
        pagination={{
          current: page,
          pageSize: pageSize,
          total: data?.total ?? 0,
          onChange: (p) => setPage(p),
        }}
        columns={[
          {
            title: 'Set ID (路由组)',
            dataIndex: 'setId',
            render: (val) => <Tag color="blue" style={{ border: 'none' }} className="font-mono font-semibold">Group {val}</Tag>
          },
          {
            title: '套接字连接串 (Socket)',
            dataIndex: 'rtpengineSock',
            render: (val) => <span className="font-mono text-xs font-semibold text-slate-700 dark:text-zinc-300">{val}</span>
          },
          {
            title: '描述',
            dataIndex: 'description',
            render: (val) => val || '-'
          },
          {
            title: '权重',
            dataIndex: 'weight',
            render: (val) => <span className="font-mono">{val}</span>
          },
          {
            title: '状态',
            render: (_, record) => {
              if (record.disabled) {
                return <Tag color="default" style={{ border: 'none' }}>已禁用</Tag>
              }
              if (record.status === 'online') {
                return (
                  <Tag color="success" style={{ border: 'none' }} className="flex items-center w-fit gap-1 font-semibold text-emerald-600">
                    <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse inline-block" />
                    在线启用
                  </Tag>
                )
              }
              if (record.status === 'offline') {
                return (
                  <Tag color="error" style={{ border: 'none' }} className="flex items-center w-fit gap-1 font-semibold text-rose-600 animate-pulse">
                    <span className="w-1.5 h-1.5 rounded-full bg-rose-500 animate-ping inline-block" />
                    故障离线
                  </Tag>
                )
              }
              return (
                <Tag color="processing" style={{ border: 'none' }} className="flex items-center w-fit gap-1 font-semibold text-blue-600">
                  <span className="w-1.5 h-1.5 rounded-full bg-blue-500 animate-pulse inline-block" />
                  检测中...
                </Tag>
              )
            }
          },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <Button size="small" onClick={() => openEdit(record)}>
                  编辑
                </Button>
                <Button
                  size="small"
                  onClick={() => saveMutation.mutate({ ...record, disabled: !record.disabled })}
                >
                  {record.disabled ? '启用' : '禁用'}
                </Button>
                <PermissionGate permission="operate:freeswitch:delete">
                  <Popconfirm title="确认从媒体集群中移除这个代理节点？" onConfirm={() => deleteMutation.mutate(record)}>
                    <Button size="small" danger>
                      移除
                    </Button>
                  </Popconfirm>
                </PermissionGate>
              </Space>
            ),
          },
        ]}
      />

      {/* 弹框 Modal */}
      <Modal
        open={open}
        title={editingId ? '编辑媒体代理节点' : '新增媒体代理节点'}
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
        >
          <Form.Item name="id" hidden>
            <Input />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="setId"
                label="Set ID (Kamailio 路由组)"
                rules={[{ required: true, message: '请输入信令路由分组 ID' }]}
              >
                <InputNumber min={1} className="w-full" placeholder="例如: 1" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="weight"
                label="调度权重"
                rules={[{ required: true, message: '请输入权重' }]}
              >
                <InputNumber min={1} className="w-full" placeholder="默认: 1" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            name="rtpengineSock"
            label="媒体套接字连接串 (RTPEngine Socket)"
            rules={[
              { required: true, message: '请输入套接字地址，例如 udp:127.0.0.1:2223' },
              {
                validator: (_, value) => {
                  if (!value || value.startsWith('udp:') || value.startsWith('tcp:')) {
                    return Promise.resolve()
                  }
                  return Promise.reject(new Error('套接字必须以 udp: 或 tcp: 开头，如 udp:127.0.0.1:2223'))
                }
              }
            ]}
          >
            <Input placeholder="例如: udp:127.0.0.1:2223 (代表 RTPEngine 控制接收地址)" />
          </Form.Item>

          <Form.Item
            name="description"
            label="节点描述 / 说明"
          >
            <Input placeholder="输入该节点说明，例如: 宿主机媒体转发主节点" />
          </Form.Item>

          <Form.Item
            name="disabled"
            label="禁用节点"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
