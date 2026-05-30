import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Switch, Tag, Typography, message, Table } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useEffect } from 'react'
import { PermissionGate } from '../../../components/PermissionGate'
import { TableWrap } from '../../../components/TableWrap'
import { useAuthStore } from '../../../store/auth'
import {
  deletePhoneGroups,
  fetchPhoneGroups,
  savePhoneGroup,
  fetchPoolPhones,
  fetchPhoneGroupPhones,
  savePhoneGroupPhones,
  fetchSkillGroups,
  fetchPhoneGroupSkillGroups,
  savePhoneGroupSkillGroups
} from '../../../api/operate'

type PhoneGroupFormValues = {
  id?: number
  name: string
  remark?: string
  desc?: string
  enable: boolean
  merchantId: number
}

export function PhoneGroupPage() {
  const tenant = useAuthStore((state) => state.tenant)
  const merchantId = Number(tenant?.merchantId || '1001')

  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  
  // Binding modal states
  const [bindPhonesId, setBindPhonesId] = useState<number | null>(null)
  const [bindSkillGroupsId, setBindSkillGroupsId] = useState<number | null>(null)

  const [form] = Form.useForm<PhoneGroupFormValues>()
  const queryClient = useQueryClient()
  
  const { data } = useQuery({
    queryKey: ['merchant', 'phone-group', pageNumber, pageSize, merchantId],
    queryFn: () => fetchPhoneGroups(pageNumber, pageSize, '', merchantId)
  })
  
  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deletePhoneGroups(ids),
    onSuccess: async () => {
      message.success('号码组已删除')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'phone-group'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: PhoneGroupFormValues) =>
      savePhoneGroup({
        id: editingId ?? undefined,
        name: values.name,
        remark: values.remark,
        desc: values.desc,
        merchantId: Number(values.merchantId || merchantId),
        enable: values.enable,
      }),
    onSuccess: async () => {
      message.success(editingId ? '号码组已更新' : '号码组已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'phone-group'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ merchantId, enable: true })
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
        desc: record?.desc ?? '',
        merchantId: record?.merchantId ?? merchantId,
        enable: Boolean(record?.enable),
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-between items-center mb-2">
        <Typography.Text type="secondary">
          管理呼叫路由的号码组，绑定外呼号码与坐席技能组。
        </Typography.Text>
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['merchant', 'phone-group'] })}>刷新</Button>
          <PermissionGate permission="merchant:phone-group:write">
            <Button type="primary" onClick={openCreate}>
              新增号码组
            </Button>
          </PermissionGate>
        </Space>
      </div>
      <TableWrap
        title="号码组列表"
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
          { title: '组 ID', dataIndex: 'id' },
          { title: '组名称', dataIndex: 'name' },
          { title: '备注', dataIndex: 'remark' },
          { title: '描述', dataIndex: 'desc' },
          { title: '状态', dataIndex: 'enable', render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '停用'}</Tag> },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <Button size="small" onClick={() => openEdit(record.id)}>
                  编辑
                </Button>
                <Button size="small" onClick={() => setBindPhonesId(record.id)}>
                  绑定号码
                </Button>
                <Button size="small" onClick={() => setBindSkillGroupsId(record.id)}>
                  绑定技能组
                </Button>
                <PermissionGate permission="merchant:phone-group:delete">
                  <Popconfirm title="确认删除这个号码组？" onConfirm={() => deleteMutation.mutate([record.id])}>
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
        title={editingId ? '编辑号码组' : '新增号码组'}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)} initialValues={{ merchantId, enable: true }}>
          <Form.Item name="name" label="组名称" rules={[{ required: true, message: '请输入号码组名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="merchantId" label="商户 ID" rules={[{ required: true, message: '请输入商户 ID' }]}>
            <InputNumber className="w-full" min={1} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input />
          </Form.Item>
          <Form.Item name="desc" label="描述">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      {/* Phone Binding Modal */}
      <PhoneBindingModal
        phoneGroupId={bindPhonesId}
        merchantId={merchantId}
        open={bindPhonesId !== null}
        onCancel={() => setBindPhonesId(null)}
      />

      {/* Skill Group Binding Modal */}
      <SkillGroupBindingModal
        phoneGroupId={bindSkillGroupsId}
        merchantId={merchantId}
        open={bindSkillGroupsId !== null}
        onCancel={() => setBindSkillGroupsId(null)}
      />
    </Space>
  )
}

// Sub-component: Phone Binding
type PhoneBindingProps = {
  phoneGroupId: number | null
  merchantId: number
  open: boolean
  onCancel: () => void
}

function PhoneBindingModal({ phoneGroupId, merchantId, open, onCancel }: PhoneBindingProps) {
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])

  const { data: phonesData, isLoading: phonesLoading } = useQuery({
    queryKey: ['operate', 'pool-phone', 'list'],
    queryFn: () => fetchPoolPhones(1, 1000),
    enabled: open && !!phoneGroupId,
  })

  const { data: boundData, isLoading: boundLoading } = useQuery({
    queryKey: ['merchant', 'phone-group', 'bound-phones', phoneGroupId],
    queryFn: () => fetchPhoneGroupPhones(phoneGroupId!),
    enabled: open && !!phoneGroupId,
  })

  useEffect(() => {
    if (boundData?.phoneIds) {
      setSelectedRowKeys(boundData.phoneIds)
    } else {
      setSelectedRowKeys([])
    }
  }, [boundData, open])

  const mutation = useMutation({
    mutationFn: async (phoneIds: number[]) => savePhoneGroupPhones(phoneGroupId!, merchantId, phoneIds),
    onSuccess: () => {
      message.success('号码绑定已更新')
      onCancel()
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '绑定失败'),
  })

  return (
    <Modal
      open={open}
      title="绑定号码"
      onCancel={onCancel}
      onOk={() => mutation.mutate(selectedRowKeys as number[])}
      confirmLoading={mutation.isPending}
      width={600}
      destroyOnClose
    >
      <Table
        loading={phonesLoading || boundLoading}
        dataSource={phonesData?.records ?? []}
        rowKey="id"
        size="small"
        pagination={{ pageSize: 10 }}
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
        }}
        columns={[
          { title: '号码 ID', dataIndex: 'id' },
          { title: '电话号码', dataIndex: 'phone' },
          { title: '省份', dataIndex: 'province' },
          { title: '城市', dataIndex: 'city' },
        ]}
      />
    </Modal>
  )
}

// Sub-component: Skill Group Binding
type SkillGroupBindingProps = {
  phoneGroupId: number | null
  merchantId: number
  open: boolean
  onCancel: () => void
}

function SkillGroupBindingModal({ phoneGroupId, merchantId, open, onCancel }: SkillGroupBindingProps) {
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])

  const { data: skillGroupsData, isLoading: skillGroupsLoading } = useQuery({
    queryKey: ['merchant', 'skill-group', 'list'],
    queryFn: () => fetchSkillGroups(1, 1000),
    enabled: open && !!phoneGroupId,
  })

  const { data: boundData, isLoading: boundLoading } = useQuery({
    queryKey: ['merchant', 'phone-group', 'bound-skillgroups', phoneGroupId],
    queryFn: () => fetchPhoneGroupSkillGroups(phoneGroupId!),
    enabled: open && !!phoneGroupId,
  })

  useEffect(() => {
    if (boundData?.skillGroupIds) {
      setSelectedRowKeys(boundData.skillGroupIds)
    } else {
      setSelectedRowKeys([])
    }
  }, [boundData, open])

  const mutation = useMutation({
    mutationFn: async (skillGroupIds: number[]) => savePhoneGroupSkillGroups(phoneGroupId!, merchantId, skillGroupIds),
    onSuccess: () => {
      message.success('技能组绑定已更新')
      onCancel()
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '绑定失败'),
  })

  return (
    <Modal
      open={open}
      title="绑定技能组"
      onCancel={onCancel}
      onOk={() => mutation.mutate(selectedRowKeys as number[])}
      confirmLoading={mutation.isPending}
      width={600}
      destroyOnClose
    >
      <Table
        loading={skillGroupsLoading || boundLoading}
        dataSource={skillGroupsData?.records ?? []}
        rowKey="id"
        size="small"
        pagination={{ pageSize: 10 }}
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
        }}
        columns={[
          { title: '组 ID', dataIndex: 'id' },
          { title: '名称', dataIndex: 'name' },
          { title: '描述', dataIndex: 'description' },
        ]}
      />
    </Modal>
  )
}
