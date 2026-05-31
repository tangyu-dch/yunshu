import { Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Switch, Tag, Typography, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { deletePools, fetchGatewayPage, fetchPools, savePool } from '@/api/operate'

type PoolFormValues = {
  id?: number
  name: string
  remark?: string
  type: number
  gatewayId?: number
  enable: boolean
}

export function PoolPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<PoolFormValues>()
  const queryClient = useQueryClient()
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  const { data, isPending } = useQuery({
    queryKey: ['operate', 'pool', pageNumber, pageSize],
    queryFn: () => fetchPools(pageNumber, pageSize)
  })

  // Fetch gateways list to select gatewayId
  const { data: gatewaysData } = useQuery({
    queryKey: ['operate', 'gateway', 1, 100],
    queryFn: () => fetchGatewayPage(1, 100),
  })

  // 优雅的客户端精细化组合条件过滤 (Progressive Enhancement)
  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.name) {
      records = records.filter((r: any) => String(r.name).toLowerCase().includes(queryParams.name.toLowerCase().trim()))
    }
    if (queryParams.gatewayId) {
      records = records.filter((r: any) => String(r.gatewayId) === String(queryParams.gatewayId))
    }
    if (queryParams.typeId !== undefined) {
      records = records.filter((r: any) => Number(r.typeId) === Number(queryParams.typeId))
    }
    if (queryParams.enable !== undefined) {
      records = records.filter((r: any) => Boolean(r.enable) === Boolean(queryParams.enable))
    }
    return records
  }, [data?.records, queryParams])

  const queryFields = useMemo(() => [
    { key: 'name', label: '号码池名称', type: 'text' as const, placeholder: '请输入名称搜索' },
    {
      key: 'gatewayId',
      label: '呼叫网关',
      type: 'select' as const,
      options: gatewaysData?.records.map((g: any) => ({
        value: String(g.id),
        label: g.name,
      })) ?? [],
    },
    {
      key: 'typeId',
      label: '号码池类型',
      type: 'select' as const,
      options: [
        { value: 1, label: '普通' },
        { value: 2, label: '预测' },
        { value: 3, label: '外呼' },
      ],
    },
    {
      key: 'enable',
      label: '启用状态',
      type: 'select' as const,
      options: [
        { value: true, label: '启用' },
        { value: false, label: '停用' },
      ],
    },
  ], [gatewaysData])

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deletePools(ids),
    onSuccess: async () => {
      message.success('号码池已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'pool'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })
  const saveMutation = useMutation({
    mutationFn: async (values: PoolFormValues) =>
      savePool({
        id: editingId ?? undefined,
        name: values.name,
        remark: values.remark,
        type: values.type,
        gatewayId: values.gatewayId ? Number(values.gatewayId) : 0,
        enable: values.enable,
      }),
    onSuccess: async () => {
      message.success(editingId ? '号码池已更新' : '号码池已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'pool'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ type: 1, enable: true })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id,
        name: record?.name ?? '',
        remark: record?.remark ?? '',
        type: record?.typeId ?? 1,
        gatewayId: record?.gatewayId ? record.gatewayId : undefined,
        enable: Boolean(record?.enable),
      })
    }, 0)
  }

  function getGatewayName(gId: number) {
    if (!gId) return '未绑定网关'
    const found = gatewaysData?.records.find((g: any) => g.id === gId)
    return found ? found.name : `网关 ID: ${gId}`
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-end mb-2">
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'pool'] })}>刷新</Button>
          <PermissionGate permission="operate:pool:write">
            <Button type="primary" onClick={openCreate}>
              新增号码池
            </Button>
          </PermissionGate>
        </Space>
      </div>
      <QueryBar
        fields={queryFields}
        onSearch={setQueryParams}
        loading={isPending}
      />

      <TableWrap
        title="号码池列表"
        rowKey="id"
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
          { title: '号码池 ID', dataIndex: 'id' },
          { title: '名称', dataIndex: 'name' },
          { title: '备注', dataIndex: 'remark' },
          {
            title: '关联网关',
            dataIndex: 'gatewayId',
            render: (gatewayId: number) => <Tag color={gatewayId ? 'cyan' : 'default'}>{getGatewayName(gatewayId)}</Tag>,
          },
          { title: '类型', dataIndex: 'type' },
          { title: '状态', dataIndex: 'enable', render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '停用'}</Tag> },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <Button size="small" onClick={() => openEdit(record.id)}>
                  编辑
                </Button>
                <PermissionGate permission="operate:pool:delete">
                  <Popconfirm title="确认删除这个号码池？" onConfirm={() => deleteMutation.mutate([record.id])}>
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
        title={editingId ? '编辑号码池' : '新增号码池'}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)} initialValues={{ type: 1, enable: true }}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input />
          </Form.Item>
          
          <Form.Item name="gatewayId" label="关联呼叫网关">
            <Select
              placeholder="选择号码池绑定的呼叫网关"
              allowClear
              options={gatewaysData?.records.map((g: any) => ({
                value: g.id,
                label: g.name,
              }))}
            />
          </Form.Item>

          <Form.Item name="type" label="类型代码 (1-普通, 2-预测, 3-外呼)" rules={[{ required: true, message: '请输入类型' }]}>
            <Select
              options={[
                { value: 1, label: '普通 (1)' },
                { value: 2, label: '预测 (2)' },
                { value: 3, label: '外呼 (3)' },
              ]}
            />
          </Form.Item>
          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
