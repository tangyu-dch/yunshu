import { Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Switch, Tag, Typography, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { deletePoolPhones, fetchPools, fetchPoolPhones, savePoolPhone, togglePoolPhoneEnable, lookupPhoneAttribution } from '@/api/operate'

type PoolPhoneFormValues = {
  id?: number
  poolId: number
  phone: string
  province?: string
  city?: string
  concurrency: number
  callLimit: number
  enable: boolean
}

export function PoolPhonePage() {
  const isMerchant = window.location.pathname.startsWith('/merchant')
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<PoolPhoneFormValues>()
  const queryClient = useQueryClient()

  const { data } = useQuery({
    queryKey: [isMerchant ? 'merchant' : 'operate', 'pool-phone', pageNumber, pageSize],
    queryFn: () => fetchPoolPhones(pageNumber, pageSize, isMerchant)
  })

  // Fetch pools list to select poolId
  const { data: poolsData } = useQuery({
    queryKey: [isMerchant ? 'merchant' : 'operate', 'pool', 1, 100],
    queryFn: () => fetchPools(1, 100, undefined, isMerchant),
  })

  const toggleMutation = useMutation({
    mutationFn: async ({ id, enable }: { id: number; enable: boolean }) => togglePoolPhoneEnable(id, enable),
    onSuccess: async () => {
      message.success('号码状态已更新')
      await queryClient.invalidateQueries({ queryKey: [isMerchant ? 'merchant' : 'operate', 'pool-phone'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '操作失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deletePoolPhones(ids),
    onSuccess: async () => {
      message.success('号码已删除')
      await queryClient.invalidateQueries({ queryKey: [isMerchant ? 'merchant' : 'operate', 'pool-phone'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: PoolPhoneFormValues) =>
      savePoolPhone({
        id: editingId ?? undefined,
        poolId: Number(values.poolId),
        phone: values.phone,
        province: values.province,
        city: values.city,
        concurrency: Number(values.concurrency),
        callLimit: Number(values.callLimit),
        enable: values.enable,
      }),
    onSuccess: async () => {
      message.success(editingId ? '号码已更新' : '号码已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: [isMerchant ? 'merchant' : 'operate', 'pool-phone'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ poolId: undefined, concurrency: 1, callLimit: 0, enable: true })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id,
        poolId: record?.poolId ? record.poolId : undefined,
        phone: record?.phone ?? '',
        province: record?.province ?? '',
        city: record?.city ?? '',
        concurrency: record?.concurrency ?? 1,
        callLimit: record?.callLimit ?? 0,
        enable: Boolean(record?.enable),
      })
    }, 0)
  }

  function getPoolName(pId: number) {
    if (!pId) return '未分配号码池'
    const found = poolsData?.records.find((p: any) => p.id === pId)
    return found ? found.name : `号码池 ID: ${pId}`
  }

  const columns = useMemo(() => {
    const baseCols: any[] = [
      { title: '号码 ID', dataIndex: 'id' },
      { title: '号码', dataIndex: 'phone' },
      {
        title: '归属号码池',
        dataIndex: 'poolId',
        render: (poolId: number) => <Tag color={poolId ? 'blue' : 'default'}>{getPoolName(poolId)}</Tag>,
      },
      { title: '省份', dataIndex: 'province' },
      { title: '城市', dataIndex: 'city' },
      { title: '并发限制', dataIndex: 'concurrency' },
      { title: '呼叫上限', dataIndex: 'callLimit' },
      { title: '状态', dataIndex: 'enable', render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '停用'}</Tag> },
    ]
    if (!isMerchant) {
      baseCols.push({
        title: '操作',
        render: (_: any, record: any) => (
          <Space size="small">
            <PermissionGate permission="operate:phone:write">
              <Button size="small" onClick={() => openEdit(record.id)}>
                编辑
              </Button>
            </PermissionGate>
            <PermissionGate permission="operate:phone:write">
              <Button size="small" onClick={() => toggleMutation.mutate({ id: record.id, enable: !record.enable })}>
                {record.enable ? '停用' : '启用'}
              </Button>
            </PermissionGate>
            <PermissionGate permission="operate:phone:delete">
              <Popconfirm title="确认删除这个号码？" onConfirm={() => deleteMutation.mutate([record.id])}>
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
  }, [isMerchant, poolsData, data])

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-end mb-2">
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: [isMerchant ? 'merchant' : 'operate', 'pool-phone'] })}>刷新</Button>
          {!isMerchant && (
            <PermissionGate permission="operate:phone:write">
              <Button type="primary" onClick={openCreate}>
                新增号码
              </Button>
            </PermissionGate>
          )}
        </Space>
      </div>
      <TableWrap
        title="号码列表"
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
          title={editingId ? '编辑号码' : '新增号码'}
          onCancel={() => {
            setOpen(false)
            setEditingId(null)
            form.resetFields()
          }}
          onOk={() => form.submit()}
          confirmLoading={saveMutation.isPending}
          destroyOnHidden
        >
          <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)} initialValues={{ concurrency: 1, callLimit: 0, enable: true }}>
            <Form.Item name="poolId" label="归属号码池" rules={[{ required: true, message: '请选择号码池' }]}>
              <Select
                placeholder="选择号码归属的号码池"
                options={poolsData?.records.map((p: any) => ({
                  value: p.id,
                  label: p.name,
                }))}
              />
            </Form.Item>
            <Form.Item name="phone" label="号码" rules={[{ required: true, message: '请输入号码' }]}>
              <Input onChange={async (e) => {
                const val = e.target.value.trim()
                if (val.length >= 7) {
                  try {
                    const res = await lookupPhoneAttribution(val)
                    if (res && res.province) {
                      form.setFieldsValue({
                        province: res.province,
                        city: res.city,
                      })
                    }
                  } catch (err) {
                    // ignore
                  }
                }
              }} />
            </Form.Item>
            <Form.Item name="province" label="省份">
              <Input />
            </Form.Item>
            <Form.Item name="city" label="城市">
              <Input />
            </Form.Item>
            <Form.Item name="concurrency" label="并发限制" rules={[{ required: true, message: '请输入并发' }]}>
              <InputNumber className="w-full" min={0} />
            </Form.Item>
            <Form.Item name="callLimit" label="呼叫上限 (0代表无限制)" rules={[{ required: true, message: '请输入呼叫上限' }]}>
              <InputNumber className="w-full" min={0} />
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
