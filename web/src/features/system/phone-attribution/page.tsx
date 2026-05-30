import { Button, Form, Input, Modal, Popconfirm, Space, Typography, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PlusOutlined, DeleteOutlined, EditOutlined, SearchOutlined, ReloadOutlined } from '@ant-design/icons'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { deletePhoneAttributions, fetchPhoneAttributions, savePhoneAttribution } from '@/api/operate'

type PhoneAttributionFormValues = {
  areaCode: string
  provCode: string
  cityCode: string
}

export function PhoneAttributionPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [isEdit, setIsEdit] = useState(false)
  const [form] = Form.useForm<PhoneAttributionFormValues>()
  const queryClient = useQueryClient()
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  const { data, isPending } = useQuery({
    queryKey: ['operate', 'phone-attribution', pageNumber, pageSize, queryParams],
    queryFn: () =>
      fetchPhoneAttributions(
        pageNumber,
        pageSize,
        queryParams.areaCode,
        queryParams.provCode,
        queryParams.cityCode
      ),
  })

  const queryFields = useMemo(
    () => [
      { key: 'areaCode', label: '七位号段', type: 'text' as const, placeholder: '请输入号段搜索，如 1380013' },
      { key: 'provCode', label: '省份代码', type: 'text' as const, placeholder: '省份代码，如 440000' },
      { key: 'cityCode', label: '城市代码', type: 'text' as const, placeholder: '城市代码，如 440300' },
    ],
    []
  )

  const deleteMutation = useMutation({
    mutationFn: async (areaCodes: string[]) => deletePhoneAttributions(areaCodes),
    onSuccess: async () => {
      message.success('归属地映射已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'phone-attribution'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: PhoneAttributionFormValues) =>
      savePhoneAttribution({
        areaCode: values.areaCode,
        provCode: values.provCode,
        cityCode: values.cityCode,
        isEdit,
      }),
    onSuccess: async () => {
      message.success(isEdit ? '归属地映射已更新' : '归属地映射已创建')
      setOpen(false)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'phone-attribution'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setIsEdit(false)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
    }, 0)
  }

  function openEdit(record: any) {
    setIsEdit(true)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        areaCode: record.areaCode,
        provCode: record.provCode,
        cityCode: record.cityCode,
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-between items-center mb-2">
        <Typography.Text type="secondary">
          被叫号码归属地号段映射表。配置 7 位手机号段对应的省份和城市区划代码，提供风控系统盲区禁拨精确匹配。
        </Typography.Text>
        <Space>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'phone-attribution'] })}
          >
            刷新
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增号段映射
          </Button>
        </Space>
      </div>

      <QueryBar fields={queryFields} onSearch={setQueryParams} loading={isPending} />

      <TableWrap
        title="号段归属地映射列表"
        rowKey="areaCode"
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
          { title: '7位号段', dataIndex: 'areaCode', key: 'areaCode' },
          { title: '省份行政代码', dataIndex: 'provCode', key: 'provCode' },
          { title: '城市行政代码', dataIndex: 'cityCode', key: 'cityCode' },
          {
            title: '操作',
            key: 'action',
            render: (_, record) => (
              <Space size="small">
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>
                  编辑
                </Button>
                <Popconfirm title="确认删除该号段映射？" onConfirm={() => deleteMutation.mutate([record.areaCode])}>
                  <Button size="small" danger icon={<DeleteOutlined />}>
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
        title={isEdit ? '编辑号段归属地映射' : '新增号段归属地映射'}
        onCancel={() => {
          setOpen(false)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)}>
          <Form.Item
            name="areaCode"
            label="7位手机号段"
            rules={[
              { required: true, message: '请输入7位手机号段' },
              { pattern: /^\d{7}$/, message: '号段必须为7位数字' },
            ]}
          >
            <Input placeholder="例如: 1380013" disabled={isEdit} />
          </Form.Item>
          <Form.Item
            name="provCode"
            label="省份行政划区代码"
            rules={[
              { required: true, message: '请输入省份代码' },
              { pattern: /^\d{6}$/, message: '行政划区代码通常为6位数字' },
            ]}
          >
            <Input placeholder="例如: 440000 (广东)" />
          </Form.Item>
          <Form.Item
            name="cityCode"
            label="城市行政划区代码"
            rules={[
              { required: true, message: '请输入城市代码' },
              { pattern: /^\d{6}$/, message: '行政划区代码通常为6位数字' },
            ]}
          >
            <Input placeholder="例如: 440300 (深圳)" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
