import { Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Switch, Tag, Typography, Tooltip, TreeSelect, message } from 'antd'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { deleteChannels, fetchChannels, saveChannel, fetchAreaCodes } from '@/api/operate'

type ChannelFormValues = {
  id?: number
  name: string
  configList?: { day: number; count: number; type: string }[]
  config?: any
  blindArea?: string | string[]
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
    queryKey: ['operate', 'channel', pageNumber, pageSize, queryParams],
    queryFn: () => fetchChannels(pageNumber, pageSize, {
      name: queryParams.name || undefined,
      enable: queryParams.enable,
    }),
  })

  // 0. 行政区划数据加载与树状转换
  const { data: areaCodes } = useQuery({
    queryKey: ['operate', 'area-codes'],
    queryFn: fetchAreaCodes,
    staleTime: 60000,
  })

  const areaTreeData = useMemo(() => {
    if (!areaCodes || areaCodes.length === 0) return []
    const map = new Map<string, any>()
    const rootNodes: any[] = []
    areaCodes.forEach((item: any) => {
      const node = {
        value: item.code,
        title: `${item.name} (${item.code})`,
        key: item.code,
        children: [] as any[],
        level: item.level,
        parentCode: item.parentCode,
      }
      map.set(item.code, node)
    })
    areaCodes.forEach((item: any) => {
      const node = map.get(item.code)
      if (item.parentCode) {
        const parentNode = map.get(item.parentCode)
        if (parentNode) {
          parentNode.children.push(node)
        } else {
          rootNodes.push(node)
        }
      } else {
        rootNodes.push(node)
      }
    })
    const cleanNodes = (nodes: any[]) => {
      nodes.forEach((n) => {
        if (n.children.length === 0) {
          delete n.children
        } else {
          cleanNodes(n.children)
        }
      })
    }
    cleanNodes(rootNodes)
    return rootNodes
  }, [areaCodes])

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
    mutationFn: async (values: ChannelFormValues) => saveChannel({
      ...values,
      blindArea: typeof values.blindArea === 'string' ? values.blindArea : Array.isArray(values.blindArea) ? values.blindArea.join(',') : undefined
    }),
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
      form.setFieldsValue({ enable: true, configList: [], blindArea: [] })
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
      let initialBlindAreas: string[] = []
      if (record?.blindArea) {
        const cleanArea = typeof record.blindArea === 'string' ? record.blindArea.replace(/^"|"$/g, '') : ''
        initialBlindAreas = cleanArea.split(',').map((s: string) => s.trim()).filter(Boolean)
      }
      form.setFieldsValue({
        id: record?.id,
        name: record?.name ?? '',
        configList,
        blindArea: initialBlindAreas,
        enable: Boolean(record?.enable),
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-end items-center mb-2">
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'channel'] })}>刷新</Button>
          <PermissionGate permission="operate:channel:write">
            <Button type="primary" onClick={openCreate}>
              新增渠道
            </Button>
          </PermissionGate>
        </Space>
      </div>

      <QueryBar
        fields={queryFields}
        onSearch={(params) => { setPageNumber(1); setQueryParams(params) }}
        loading={isPending}
      />

      <TableWrap
        title="渠道列表"
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
          {
            title: '盲区设定',
            dataIndex: 'blindArea',
            ellipsis: true,
            render: (val: string) => {
              if (!val) return '-'
              const cleanVal = typeof val === 'string' ? val.replace(/^"|"$/g, '') : ''
              if (!cleanVal) return '-'
              const codes = cleanVal.split(',').map((s: string) => s.trim()).filter(Boolean)
              const names = codes.map((c: string) => {
                const found = areaCodes?.find((a: any) => a.code === c)
                return found ? found.name : c
              }).join(', ')
              return (
                <Tooltip title={names || cleanVal}>
                  <span>{names || cleanVal}</span>
                </Tooltip>
              )
            }
          },
          {
            title: '状态',
            dataIndex: 'enable',
            render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '禁用'}</Tag>,
          },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <PermissionGate permission="operate:channel:write">
                  <Button size="small" onClick={() => openEdit(record.id)}>
                    编辑
                  </Button>
                </PermissionGate>
                <PermissionGate permission="operate:channel:delete">
                  <Popconfirm title="确认删除该渠道？" onConfirm={() => deleteMutation.mutate([record.id])}>
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
        title={editingId ? '编辑渠道' : '新增渠道'}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnHidden
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
            const blindAreaStr = Array.isArray(values.blindArea)
              ? values.blindArea.join(',')
              : (values.blindArea || '')
            saveMutation.mutate({
              id: editingId ?? undefined,
              name: values.name,
              config: configObj,
              blindArea: blindAreaStr,
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
          <Form.Item
            name="blindArea"
            label="盲区地区 (可选)"
            extra="当被叫号码匹配到选中的省份或城市时，该渠道将被风控拦截（若选择省级节点将自动拦截该省份下所有城市）。"
          >
            <TreeSelect
              treeData={areaTreeData}
              placeholder="请选择受限的省份或地级市"
              allowClear
              multiple
              treeCheckable
              showSearch
              treeNodeFilterProp="title"
              style={{ width: '100%' }}
              dropdownStyle={{ maxHeight: 400, overflow: 'auto' }}
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
