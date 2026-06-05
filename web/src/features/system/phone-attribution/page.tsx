import { Button, Form, Input, Modal, Popconfirm, Space, Typography, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PlusOutlined, DeleteOutlined, EditOutlined, SearchOutlined, ReloadOutlined } from '@ant-design/icons'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { deletePhoneAttributions, fetchPhoneAttributions, savePhoneAttribution } from '@/api/operate'

type PhoneAttributionFormValues = {
  areaCode: string
  province: string
  city: string
  provCode: string
  cityCode: string
  serviceProvider: string
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
        queryParams.cityCode,
        queryParams.province,
        queryParams.city,
        queryParams.serviceProvider
      ),
  })

  const queryFields = useMemo(
    () => [
      { key: 'areaCode', label: '七位号段', type: 'text' as const, placeholder: '如 1380013' },
      { key: 'province', label: '省份', type: 'text' as const, placeholder: '如 广东' },
      { key: 'city', label: '城市', type: 'text' as const, placeholder: '如 深圳' },
      { key: 'serviceProvider', label: '运营商', type: 'text' as const, placeholder: '如 中国联通' },
      { key: 'provCode', label: '省份代码', type: 'text' as const, placeholder: '如 440000' },
      { key: 'cityCode', label: '城市代码', type: 'text' as const, placeholder: '如 440300' },
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
        province: values.province,
        city: values.city,
        provCode: values.provCode,
        cityCode: values.cityCode,
        serviceProvider: values.serviceProvider,
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
        province: record.province,
        city: record.city,
        provCode: record.provCode,
        cityCode: record.cityCode,
        serviceProvider: record.serviceProvider,
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-end mb-2">
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

      <QueryBar fields={queryFields} onSearch={(params) => { setPageNumber(1); setQueryParams(params) }} loading={isPending} />

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
          { title: '省份', dataIndex: 'province', key: 'province' },
          { title: '城市', dataIndex: 'city', key: 'city' },
          { title: '省份代码', dataIndex: 'provCode', key: 'provCode' },
          { title: '城市代码', dataIndex: 'cityCode', key: 'cityCode' },
          { title: '运营商', dataIndex: 'serviceProvider', key: 'serviceProvider' },
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
        destroyOnHidden
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
            name="province"
            label="省份名称"
            rules={[{ required: true, message: '请输入省份名称' }]}
          >
            <Input placeholder="例如: 广东" />
          </Form.Item>
          <Form.Item
            name="city"
            label="城市名称"
            rules={[{ required: true, message: '请输入城市名称' }]}
          >
            <Input placeholder="例如: 深圳" />
          </Form.Item>
          <Form.Item
            name="provCode"
            label="省份行政划区代码"
            rules={[
              { required: true, message: '请输入省份代码' },
              { pattern: /^\d{6}$/, message: '行政划区代码通常为6位数字' },
            ]}
          >
            <Input placeholder="例如: 440000" />
          </Form.Item>
          <Form.Item
            name="cityCode"
            label="城市行政划区代码"
            rules={[
              { required: true, message: '请输入城市代码' },
              { pattern: /^\d{6}$/, message: '行政划区代码通常为6位数字' },
            ]}
          >
            <Input placeholder="例如: 440300" />
          </Form.Item>
          <Form.Item
            name="serviceProvider"
            label="运营商"
            rules={[{ required: true, message: '请输入运营商名称' }]}
          >
            <Input placeholder="例如: 中国联通" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
