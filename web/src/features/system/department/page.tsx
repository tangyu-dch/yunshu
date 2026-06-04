import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Switch, Tag, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { useAuthStore } from '@/store/auth'
import {
  fetchDepartments,
  saveDepartment,
  deleteDepartments,
  Department
} from '@/api/operate'

export function DepartmentPage() {
  const tenant = useAuthStore((state) => state.tenant)
  const currentMerchantId = Number(tenant?.merchantId || 0)
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  const [form] = Form.useForm<Department>()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['merchant', 'department', pageNumber, pageSize, queryParams],
    queryFn: () => fetchDepartments(pageNumber, pageSize, {
      name: queryParams.name,
      merchantId: currentMerchantId > 0 ? currentMerchantId : queryParams.merchantId,
      enable: queryParams.enable === 'all' ? undefined : (queryParams.enable === 'true')
    })
  })

  const queryFields = useMemo(() => [
    { key: 'name', label: '部门名称', type: 'text' as const, placeholder: '请输入部门名称模糊搜索' },
    ...(!tenant?.internal ? [] : [
      { key: 'merchantId', label: '商户 ID', type: 'number' as const, placeholder: '商户 ID' }
    ]),
    {
      key: 'enable',
      label: '状态',
      type: 'select' as const,
      options: [
        { label: '全部', value: 'all' },
        { label: '启用', value: 'true' },
        { label: '禁用', value: 'false' }
      ]
    }
  ], [tenant])

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteDepartments(ids),
    onSuccess: async () => {
      message.success('部门已删除')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'department'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: Department) =>
      saveDepartment({
        id: editingId ?? undefined,
        name: values.name,
        merchantId: values.merchantId,
        description: values.description,
        enable: values.enable,
      }),
    onSuccess: async () => {
      message.success(editingId ? '部门已更新' : '部门已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'department'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ merchantId: currentMerchantId, enable: true })
    }, 0)
  }

  function openEdit(record: Department) {
    setEditingId(record.id ?? null)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id: record.id,
        name: record.name,
        merchantId: record.merchantId,
        description: record.description,
        enable: record.enable,
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

      <div className="flex justify-end mb-2">
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['merchant', 'department'] })}>刷新</Button>
          <PermissionGate permission="merchant:department:write">
            <Button type="primary" onClick={openCreate}>
              新增部门
            </Button>
          </PermissionGate>
        </Space>
      </div>

      <TableWrap
        title="部门列表"
        rowKey="id"
        loading={isLoading}
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
          { title: '部门 ID', dataIndex: 'id', width: 100 },
          { title: '部门名称', dataIndex: 'name' },
          { title: '描述', dataIndex: 'description' },
          { title: '商户 ID', dataIndex: 'merchantId', width: 120 },
          { title: '状态', dataIndex: 'enable', width: 120, render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '禁用'}</Tag> },
          {
            title: '操作',
            width: 180,
            render: (_, record) => (
              <Space size="small">
                <PermissionGate permission="merchant:department:write">
                  <Button size="small" onClick={() => openEdit(record)}>
                    编辑
                  </Button>
                </PermissionGate>
                <PermissionGate permission="merchant:department:delete">
                  <Popconfirm title="确认删除这个部门？" onConfirm={() => deleteMutation.mutate([record.id!])}>
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
        title={editingId ? '编辑部门' : '新增部门'}
        width={500}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)} initialValues={{ enable: true }}>
          <Form.Item name="name" label="部门名称" rules={[{ required: true, message: '请输入部门名称' }]}>
            <Input placeholder="例如: 话务中心" />
          </Form.Item>
          <Form.Item name="merchantId" label="商户 ID" rules={[{ required: true, message: '请输入商户 ID' }]}>
            <InputNumber className="w-full" min={1} disabled={currentMerchantId > 0} placeholder="例如: 1001" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea placeholder="部门主要职责" rows={3} />
          </Form.Item>
          <Form.Item name="enable" label="启用状态" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
