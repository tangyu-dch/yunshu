import { Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Switch, Tag, Typography, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar, QueryField } from '@/components/QueryBar'
import { deletePools, fetchPools, savePool, fetchMerchants } from '@/api/operate'

type PoolFormValues = {
  id?: number
  merchantId?: number
  name: string
  remark?: string
  type: number
  gatewayId?: number
  enable: boolean
}

export function PoolPage() {
  const isMerchant = window.location.pathname.startsWith('/merchant')
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<PoolFormValues>()
  const queryClient = useQueryClient()
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  const { data, isPending } = useQuery({
    queryKey: [isMerchant ? 'merchant' : 'operate', 'pool', pageNumber, pageSize, queryParams],
    queryFn: () => fetchPools(pageNumber, pageSize, {
      name: queryParams.name || undefined,
      gatewayId: queryParams.gatewayId ? Number(queryParams.gatewayId) : undefined,
      enable: queryParams.enable,
    }, isMerchant)
  })

  // Fetch merchants list to assign pool
  const { data: merchantsData } = useQuery({
    queryKey: ['operate', 'merchant', 1, 100],
    queryFn: () => fetchMerchants(1, 100),
    enabled: !isMerchant,
  })

  const queryFields = useMemo(() => {
    const fields: QueryField[] = [
      { key: 'name', label: '号码池名称', type: 'text', placeholder: '请输入名称搜索' },
    ]
    if (!isMerchant) {
      fields.push(
        {
          key: 'merchantId',
          label: '所属商户',
          type: 'select' as const,
          options: merchantsData?.records.map((m: any) => ({
            value: String(m.id),
            label: m.name,
          })) ?? [],
        }
      )
    }
    fields.push({
      key: 'enable',
      label: '启用状态',
      type: 'select' as const,
      options: [
        { value: true, label: '启用' },
        { value: false, label: '停用' },
      ],
    })
    return fields
  }, [isMerchant, merchantsData])

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deletePools(ids),
    onSuccess: async () => {
      message.success('号码池已删除')
      await queryClient.invalidateQueries({ queryKey: [isMerchant ? 'merchant' : 'operate', 'pool'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })
  const saveMutation = useMutation({
    mutationFn: async (values: PoolFormValues) => {
      const record = editingId ? data?.records.find((item) => item.id === editingId) : null
      return savePool({
        id: editingId ?? undefined,
        merchantId: values.merchantId ? Number(values.merchantId) : undefined,
        name: values.name,
        remark: values.remark,
        type: values.type,
        gatewayId: record?.gatewayId ?? 0,
        enable: values.enable,
      })
    },
    onSuccess: async () => {
      message.success(editingId ? '号码池已更新' : '号码池已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: [isMerchant ? 'merchant' : 'operate', 'pool'] })
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
        merchantId: record?.merchantId ? record.merchantId : undefined,
        type: record?.typeId ?? 1,
        enable: Boolean(record?.enable),
      })
    }, 0)
  }



  const columns = useMemo(() => {
    const baseCols: any[] = [
      { title: '号码池 ID', dataIndex: 'id' },
      { title: '名称', dataIndex: 'name' },
    ]
    if (!isMerchant) {
      baseCols.push({
        title: '所属商户',
        dataIndex: 'merchantId',
        render: (merchantId: number) => {
          if (!merchantId) return <Tag>未分配</Tag>
          const m = merchantsData?.records.find((x: any) => x.id === merchantId)
          return <Tag color="blue">{m ? m.name : `商户 ID: ${merchantId}`}</Tag>
        },
      })
    }
    baseCols.push({ title: '备注', dataIndex: 'remark' })
    baseCols.push(
      {
        title: '类型',
        dataIndex: 'typeId',
        render: (typeId: number) => {
          if (typeId === 1) return <Tag color="orange">呼入</Tag>
          if (typeId === 2) return <Tag color="blue">呼出</Tag>
          return <Tag>{typeId}</Tag>
        }
      },
      { title: '状态', dataIndex: 'enable', render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '停用'}</Tag> }
    )
    if (!isMerchant) {
      baseCols.push({
        title: '操作',
        render: (_: any, record: any) => (
          <Space size="small">
            <PermissionGate permission="operate:pool:write">
              <Button size="small" onClick={() => openEdit(record.id)}>
                编辑
              </Button>
            </PermissionGate>
            <PermissionGate permission="operate:pool:delete">
              <Popconfirm title="确认删除这个号码池？" onConfirm={() => deleteMutation.mutate([record.id])}>
                <Button size="small" danger>
                  删除
                </Button>
              </Popconfirm>
            </PermissionGate>
          </Space>
        ),
      })
    }
    return baseCols
  }, [isMerchant, merchantsData, data])

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-end mb-2">
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: [isMerchant ? 'merchant' : 'operate', 'pool'] })}>刷新</Button>
          {!isMerchant && (
            <PermissionGate permission="operate:pool:write">
              <Button type="primary" onClick={openCreate}>
                新增号码池
              </Button>
            </PermissionGate>
          )}
        </Space>
      </div>
      <QueryBar
        fields={queryFields}
        onSearch={(params) => { setPageNumber(1); setQueryParams(params) }}
        loading={isPending}
      />

      <TableWrap
        title="号码池列表"
        rowKey="id"
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
        columns={columns}
      />

      {!isMerchant && (
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
          destroyOnHidden
        >
          <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)} initialValues={{ type: 1, enable: true }}>
            <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
              <Input />
            </Form.Item>
            
            <Form.Item name="merchantId" label="分配商户">
              <Select
                placeholder="选择号码池归属的商户"
                allowClear
                options={merchantsData?.records.map((m: any) => ({
                  value: m.id,
                  label: m.name,
                }))}
              />
            </Form.Item>

            <Form.Item name="remark" label="备注">
              <Input />
            </Form.Item>
            


            <Form.Item name="type" label="类型" rules={[{ required: true, message: '请选择类型' }]}>
              <Select
                options={[
                  { value: 1, label: '呼入' },
                  { value: 2, label: '呼出' },
                ]}
              />
            </Form.Item>
            <Form.Item name="enable" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Form>
        </Modal>
      )}
    </Space>
  )
}
