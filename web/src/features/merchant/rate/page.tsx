import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Tag, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { deleteRates, fetchRates, saveRate } from '@/api/operate'

type RateFormValues = {
  id?: number
  rateName: string
  billingPrice: number
  billingCycle: number
  remark?: string
}

export function RatePage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})
  const [form] = Form.useForm<RateFormValues>()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['operate', 'rate', pageNumber, pageSize],
    queryFn: () => fetchRates(pageNumber, pageSize),
  })

  const queryFields = useMemo(() => [
    { key: 'rateName', label: '费率名称', type: 'text' as const, placeholder: '请输入费率名称模糊搜索' },
  ], [])

  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.rateName) {
      records = records.filter((r: any) => String(r.rateName).toLowerCase().includes(queryParams.rateName.toLowerCase().trim()))
    }
    return records
  }, [data, queryParams])

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteRates(ids),
    onSuccess: async () => {
      message.success('费率已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'rate'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败，可能已被商户或网关关联'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: RateFormValues) => saveRate(values),
    onSuccess: async () => {
      message.success(editingId ? '费率已更新' : '费率已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'rate'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ billingCycle: 60 })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item: any) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id: record?.id,
        rateName: record?.rateName ?? '',
        billingPrice: record?.billingPrice ?? 0,
        billingCycle: record?.billingCycle ?? 60,
        remark: record?.remark ?? '',
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
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'rate'] })}>刷新</Button>
          <Button type="primary" onClick={openCreate}>
            新增费率
          </Button>
        </Space>
      </div>

      <TableWrap
        title="费率列表"
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
          { title: '费率 ID', dataIndex: 'id' },
          { title: '费率名称', dataIndex: 'rateName' },
          {
            title: '单价',
            dataIndex: 'billingPrice',
            render: (value: number) => `￥${value.toFixed(4)} / 分钟`,
          },
          {
            title: '计费周期 (秒)',
            dataIndex: 'billingCycle',
            render: (value: number) => `${value}秒`,
          },
          { title: '备注', dataIndex: 'remark' },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <Button size="small" onClick={() => openEdit(record.id)}>
                  编辑
                </Button>
                <Popconfirm title="确认删除该费率？" onConfirm={() => deleteMutation.mutate([record.id])}>
                  <Button size="small" danger>
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        open={open}
        title={editingId ? '编辑费率' : '新增费率'}
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
          onFinish={(values) => {
            saveMutation.mutate({
              ...values,
              id: editingId ?? undefined,
            })
          }}
          initialValues={{ billingCycle: 60 }}
        >
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-1">
            <Form.Item name="rateName" label="费率名称" rules={[{ required: true, message: '请输入费率名称' }]}>
              <Input placeholder="例如: 基础运营费率" />
            </Form.Item>
            <Form.Item name="billingPrice" label="单价 (￥/分钟)" rules={[{ required: true, message: '请输入计费单价' }]}>
              <InputNumber className="w-full" min={0} step={0.0001} precision={4} placeholder="例如: 0.1500" />
            </Form.Item>
            <Form.Item name="billingCycle" label="计费周期 (秒)" rules={[{ required: true, message: '请输入周期秒数' }]}>
              <InputNumber className="w-full" min={1} defaultValue={60} placeholder="默认: 60" />
            </Form.Item>
            <Form.Item name="remark" label="备注" className="col-span-1 md:col-span-2">
              <Input placeholder="备注说明，例如: 某某网关专用" />
            </Form.Item>
          </div>
        </Form>
      </Modal>
    </Space>
  )
}
