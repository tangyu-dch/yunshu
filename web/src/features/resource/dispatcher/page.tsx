import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Tag, Switch, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { deleteDispatchers, fetchDispatchers, reloadDispatchers, saveDispatcher } from '@/api/operate'

type DispatcherFormValues = {
  id?: number
  setId: number
  destination: string
  flags: number
  priority: number
  attrs?: string
  description: string
  enable: boolean
}

export function DispatcherPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})
  const [form] = Form.useForm<DispatcherFormValues>()
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['operate', 'dispatcher', pageNumber, pageSize], queryFn: () => fetchDispatchers(pageNumber, pageSize) })

  const queryFields = useMemo(() => [
    { key: 'destination', label: '目的地址', type: 'text' as const, placeholder: '请输入 SIP 目的地址模糊搜索' },
  ], [])

  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.destination) {
      records = records.filter((r: any) => String(r.destination).toLowerCase().includes(queryParams.destination.toLowerCase().trim()))
    }
    return records
  }, [data, queryParams])

  const reloadMutation = useMutation({
    mutationFn: reloadDispatchers,
    onSuccess: async () => {
      message.success('Dispatcher 已重载')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'dispatcher'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '重载失败'),
  })
  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteDispatchers(ids),
    onSuccess: async () => {
      message.success('Dispatcher 已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'dispatcher'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })
  const saveMutation = useMutation({
    mutationFn: async (values: DispatcherFormValues) =>
      saveDispatcher({
        id: editingId ?? undefined,
        setId: values.setId,
        destination: values.destination,
        flags: values.flags,
        priority: values.priority,
        attrs: values.attrs,
        description: values.description,
        enable: values.enable,
      }),
    onSuccess: async () => {
      message.success(editingId ? 'Dispatcher 已更新' : 'Dispatcher 已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'dispatcher'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ flags: 0, priority: 10, enable: true, setId: 1, destination: '', description: '' })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id,
        setId: record?.setId ?? 1,
        destination: record?.destination ?? '',
        flags: record?.flags ?? 0,
        priority: record?.priority ?? 10,
        attrs: record?.attrs ?? '',
        description: record?.description ?? '',
        enable: Boolean(record?.enable),
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <QueryBar
        fields={queryFields}
        onSearch={setQueryParams}
        loading={isLoading}
      />

      <div className="flex justify-end items-center mb-2">
        <Space>
          <PermissionGate permission="operate:dispatcher:reload">
            <Button onClick={() => reloadMutation.mutate()}>重载路由</Button>
          </PermissionGate>
          <PermissionGate permission="operate:dispatcher:write">
            <Button type="primary" onClick={openCreate}>
              新增分发项
            </Button>
          </PermissionGate>
        </Space>
      </div>
      <TableWrap
        title="分发列表"
        rowKey="id"
        loading={isLoading}
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
          { title: '目的地址', dataIndex: 'destination' },
          { title: '描述', dataIndex: 'description' },
          { title: 'Set ID', dataIndex: 'setId' },
          { title: '优先级', dataIndex: 'priority' },
          { title: 'Flags', dataIndex: 'flags' },
          { title: '附加参数', dataIndex: 'attrs', render: (val?: string) => val || '-' },
          { title: '状态', dataIndex: 'enable', render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '停用'}</Tag> },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <PermissionGate permission="operate:dispatcher:write">
                  <Button size="small" onClick={() => openEdit(record.id)}>
                    编辑
                  </Button>
                </PermissionGate>
                <PermissionGate permission="operate:dispatcher:write">
                  <Popconfirm title="确认删除这个分发项？" onConfirm={() => deleteMutation.mutate([record.id])}>
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
        title={editingId ? '编辑分发项' : '新增分发项'}
        width={640}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)} initialValues={{ setId: 1, priority: 10, enable: true, flags: 0 }}>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-1">
            <Form.Item name="setId" label="Set ID" rules={[{ required: true, message: '请输入 Set ID' }]}>
              <InputNumber className="w-full" min={1} placeholder="例如: 1" />
            </Form.Item>
            <Form.Item name="destination" label="目的地址" rules={[{ required: true, message: '请输入目的地址' }]}>
              <Input placeholder="例如: sip:192.168.1.10:5060" />
            </Form.Item>
            <Form.Item name="flags" label="Flags" rules={[{ required: true, message: '请输入 Flags' }]}>
              <InputNumber className="w-full" min={0} placeholder="例如: 0" />
            </Form.Item>
            <Form.Item name="priority" label="优先级" rules={[{ required: true, message: '请输入优先级' }]}>
              <InputNumber className="w-full" min={0} placeholder="例如: 10" />
            </Form.Item>
            <Form.Item name="attrs" label="附加参数">
              <Input placeholder="路由附加参数" />
            </Form.Item>
            <Form.Item name="enable" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="description" label="描述" rules={[{ required: true, message: '请输入描述' }]} className="col-span-1 md:col-span-2">
              <Input placeholder="描述分发项的用途" />
            </Form.Item>
          </div>
        </Form>
      </Modal>
    </Space>
  )
}
