import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Tag, Typography, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { TableWrap } from '../../../components/TableWrap'
import { deleteWhitelist, fetchWhitelist, saveWhitelist } from '../../../api/operate'

type WhitelistFormValues = {
  id?: number
  phone: string
  numberType: number
}

export function WhitelistPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<WhitelistFormValues>()
  const queryClient = useQueryClient()

  const { data } = useQuery({
    queryKey: ['operate', 'whitelist', pageNumber, pageSize],
    queryFn: () => fetchWhitelist(pageNumber, pageSize),
  })

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteWhitelist(ids),
    onSuccess: async () => {
      message.success('白名单号码已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'whitelist'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: WhitelistFormValues) => saveWhitelist(values),
    onSuccess: async () => {
      message.success(editingId ? '白名单已更新' : '白名单已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'whitelist'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ numberType: 1 })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item: any) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id: record?.id,
        phone: record?.phone ?? '',
        numberType: record?.numberType ?? 1,
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-between items-center mb-2">
        <Typography.Text type="secondary">
          维护免风控或高优先起呼的特定客户白名单手机号。
        </Typography.Text>
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'whitelist'] })}>刷新</Button>
          <Button type="primary" onClick={openCreate}>
            新增白名单号码
          </Button>
        </Space>
      </div>

      <TableWrap
        title="白名单号码列表"
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
        columns={[
          { title: '编号', dataIndex: 'id' },
          { title: '手机号码', dataIndex: 'phone' },
          {
            title: '号码类型代码',
            dataIndex: 'numberType',
            render: (value: number) => {
              return <Tag color="blue">{value === 1 ? '普通 (1)' : `特殊 (${value})`}</Tag>
            },
          },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <Button size="small" onClick={() => openEdit(record.id)}>
                  编辑
                </Button>
                <Popconfirm title="确认删除该白名单号码？" onConfirm={() => deleteMutation.mutate([record.id])}>
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
        title={editingId ? '编辑白名单号码' : '新增白名单号码'}
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
          initialValues={{ numberType: 1 }}
        >
          <Form.Item name="phone" label="手机号码" rules={[{ required: true, message: '请输入白名单手机号码' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="numberType" label="号码类型代码" rules={[{ required: true, message: '请输入号码类型代码' }]}>
            <InputNumber className="w-full" min={1} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
