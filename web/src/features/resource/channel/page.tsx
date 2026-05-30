import { Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Switch, Tag, Typography, message } from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { deleteChannels, fetchChannels, saveChannel } from '@/api/operate'

type ChannelFormValues = {
  id?: number
  name: string
  configList?: { day: number; count: number; type: string }[]
  config?: any
  blindArea?: string
  enable: boolean
}

export function ChannelPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<ChannelFormValues>()
  const queryClient = useQueryClient()
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  const { data, isPending } = useQuery({
    queryKey: ['operate', 'channel', pageNumber, pageSize],
    queryFn: () => fetchChannels(pageNumber, pageSize),
  })

  // 优雅的客户端精细化组合条件过滤 (Progressive Enhancement)
  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.name) {
      records = records.filter((r: any) => String(r.name).toLowerCase().includes(queryParams.name.toLowerCase().trim()))
    }
    if (queryParams.enable !== undefined) {
      records = records.filter((r: any) => Boolean(r.enable) === Boolean(queryParams.enable))
    }
    return records
  }, [data?.records, queryParams])

  const queryFields = useMemo(() => [
    { key: 'name', label: '渠道名称', type: 'text' as const, placeholder: '请输入渠道名称搜索' },
    {
      key: 'enable',
      label: '启用状态',
      type: 'select' as const,
      options: [
        { value: true, label: '启用' },
        { value: false, label: '禁用' },
      ],
    },
  ], [])

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteChannels(ids),
    onSuccess: async () => {
      message.success('渠道已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'channel'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: ChannelFormValues) => saveChannel(values),
    onSuccess: async () => {
      message.success(editingId ? '渠道已更新' : '渠道已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'channel'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ enable: true, configList: [] })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item: any) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      let configList: { day: number; count: number; type: string }[] = []
      if (record?.config) {
        try {
          const parsed = typeof record.config === 'string' ? JSON.parse(record.config) : record.config
          if (Array.isArray(parsed)) {
            configList = parsed.map((item: any) => ({
              day: item.day,
              count: item.count,
              type: String(item.type ?? 1),
            }))
          }
        } catch (e) {
          console.error('Failed to parse channel config:', e)
        }
      }
      form.setFieldsValue({
        id: record?.id,
        name: record?.name ?? '',
        configList,
        blindArea: record?.blindArea ?? '',
        enable: Boolean(record?.enable),
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-between items-center mb-2">
        <Typography.Text type="secondary">
          维护外呼路由对应的物理渠道及盲区禁拨规则配置。
        </Typography.Text>
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'channel'] })}>刷新</Button>
          <Button type="primary" onClick={openCreate}>
            新增渠道
          </Button>
        </Space>
      </div>

      <QueryBar
        fields={queryFields}
        onSearch={setQueryParams}
        loading={isPending}
      />

      <TableWrap
        title="渠道列表"
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
          { title: '编号', dataIndex: 'id' },
          { title: '渠道名称', dataIndex: 'name' },
          {
            title: '频次限制配置',
            dataIndex: 'config',
            ellipsis: true,
            render: (config: any) => {
              if (!config) return '-'
              try {
                const parsed = typeof config === 'string' ? JSON.parse(config) : config
                if (Array.isArray(parsed)) {
                  return (
                    <Space size={[0, 4]} wrap>
                      {parsed.map((item: any, index: number) => {
                        const typeText = Number(item.type) === 2 ? '接通限制' : '拨打限制'
                        return (
                          <Tag key={index} color="blue">
                            {item.day}天{item.count}次({typeText})
                          </Tag>
                        )
                      })}
                    </Space>
                  )
                }
              } catch (e) {
                // ignore
              }
              return String(config)
            }
          },
          { title: '盲区设定', dataIndex: 'blindArea', ellipsis: true },
          {
            title: '状态',
            dataIndex: 'enable',
            render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '禁用'}</Tag>,
          },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <Button size="small" onClick={() => openEdit(record.id)}>
                  编辑
                </Button>
                <Popconfirm title="确认删除该渠道？" onConfirm={() => deleteMutation.mutate([record.id])}>
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
        title={editingId ? '编辑渠道' : '新增渠道'}
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
            const configObj = (values.configList || []).map((item: any) => ({
              day: Number(item.day),
              count: Number(item.count),
              type: Number(item.type) || 1,
            }))
            saveMutation.mutate({
              id: editingId ?? undefined,
              name: values.name,
              config: configObj,
              blindArea: values.blindArea,
              enable: values.enable,
            })
          }}
          initialValues={{ enable: true }}
        >
          <Form.Item name="name" label="渠道名称" rules={[{ required: true, message: '请输入渠道名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="频次限制规则 (可选)">
            <Form.List name="configList">
              {(fields, { add, remove }) => (
                <>
                  {fields.map(({ key, name, ...restField }) => (
                    <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                      <Form.Item
                        {...restField}
                        name={[name, 'day']}
                        rules={[{ required: true, message: '天数必填' }]}
                        style={{ margin: 0 }}
                      >
                        <InputNumber min={1} addonAfter="天" placeholder="天数" style={{ width: 110 }} />
                      </Form.Item>
                      <Form.Item
                        {...restField}
                        name={[name, 'count']}
                        rules={[{ required: true, message: '次数必填' }]}
                        style={{ margin: 0 }}
                      >
                        <InputNumber min={1} addonAfter="次" placeholder="上限次数" style={{ width: 130 }} />
                      </Form.Item>
                      <Form.Item
                        {...restField}
                        name={[name, 'type']}
                        rules={[{ required: true, message: '限制类型必填' }]}
                        style={{ margin: 0 }}
                      >
                        <Select placeholder="限制类型" style={{ width: 170 }}>
                          <Select.Option value="1">统计拨打次数 (DIAL)</Select.Option>
                          <Select.Option value="2">统计接通次数 (CONNECTED)</Select.Option>
                        </Select>
                      </Form.Item>
                      <Button type="text" danger icon={<DeleteOutlined />} onClick={() => remove(name)} />
                    </Space>
                  ))}
                  <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                    添加频次限制规则
                  </Button>
                </>
              )}
            </Form.List>
          </Form.Item>
          <Form.Item name="blindArea" label="盲区地区 (可选, 英文逗号分隔)">
            <Input.TextArea rows={2} placeholder="北京,上海,广东" />
          </Form.Item>
          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
