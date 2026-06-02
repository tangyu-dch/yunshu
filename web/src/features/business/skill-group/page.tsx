import { Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Switch, Tag, Typography, message, Table } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useEffect, useMemo } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { useAuthStore } from '@/store/auth'
import {
  deleteSkillGroups,
  fetchSkillGroups,
  saveSkillGroup,
  fetchMerchantAccounts,
  fetchPoolPhones,
  fetchSkillGroupUsers,
  saveSkillGroupUsers,
  fetchSkillGroupPhones,
  saveSkillGroupPhones
} from '@/api/operate'

type SkillGroupFormValues = {
  id?: number
  name: string
  merchantId: number
  description?: string
  enable: boolean
}

export function SkillGroupPage() {
  const tenant = useAuthStore((state) => state.tenant)
  const currentMerchantId = Number(tenant?.merchantId || 0)
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})
  
  // Binding modal states
  const [bindUsersId, setBindUsersId] = useState<number | null>(null)
  const [bindPhonesId, setBindPhonesId] = useState<number | null>(null)

  const [form] = Form.useForm<SkillGroupFormValues>()
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['merchant', 'skill-group', pageNumber, pageSize], queryFn: () => fetchSkillGroups(pageNumber, pageSize) })
  
  const queryFields = useMemo(() => [
    { key: 'name', label: '技能组名称', type: 'text' as const, placeholder: '请输入技能组名称模糊搜索' },
  ], [])

  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.name) {
      records = records.filter((r: any) => String(r.name).toLowerCase().includes(queryParams.name.toLowerCase().trim()))
    }
    return records
  }, [data, queryParams])
  
  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteSkillGroups(ids),
    onSuccess: async () => {
      message.success('技能组已删除')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'skill-group'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })
  const saveMutation = useMutation({
    mutationFn: async (values: SkillGroupFormValues) =>
      saveSkillGroup({
        id: editingId ?? undefined,
        name: values.name,
        merchantId: values.merchantId,
        description: values.description,
        enable: values.enable,
      }),
    onSuccess: async () => {
      message.success(editingId ? '技能组已更新' : '技能组已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'skill-group'] })
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

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id,
        name: record?.name ?? '',
        merchantId: record?.merchantId ?? 0,
        description: record?.description ?? '',
        enable: Boolean(record?.enable),
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
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['merchant', 'skill-group'] })}>刷新</Button>
          <PermissionGate permission="merchant:skill-group:write">
            <Button type="primary" onClick={openCreate}>
              新增技能组
            </Button>
          </PermissionGate>
        </Space>
      </div>
      <TableWrap
        title="技能组列表"
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
          { title: '名称', dataIndex: 'name' },
          { title: '商户', dataIndex: 'merchant' },
          { title: '描述', dataIndex: 'description' },
          { title: '状态', dataIndex: 'enable', render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '停用'}</Tag> },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <Button size="small" onClick={() => openEdit(record.id)}>
                  编辑
                </Button>
                <Button size="small" onClick={() => setBindUsersId(record.id)}>
                  绑定成员
                </Button>
                <Button size="small" onClick={() => setBindPhonesId(record.id)}>
                  绑定号码
                </Button>
                <PermissionGate permission="merchant:skill-group:delete">
                  <Popconfirm title="确认删除这个技能组？" onConfirm={() => deleteMutation.mutate([record.id])}>
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
        title={editingId ? '编辑技能组' : '新增技能组'}
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
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)} initialValues={{ enable: true }}>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-1">
            <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
              <Input placeholder="例如: 智能客服一组" />
            </Form.Item>
            <Form.Item name="merchantId" label="商户 ID" rules={[{ required: true, message: '请输入商户 ID' }]}>
              <InputNumber className="w-full" min={1} placeholder="例如: 1001" />
            </Form.Item>
            <Form.Item name="description" label="描述" className="col-span-1 md:col-span-2">
              <Input placeholder="该技能组的主要话务方向" />
            </Form.Item>
            <Form.Item name="enable" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          </div>
        </Form>
      </Modal>

      {/* User Binding Modal */}
      <UserBindingModal
        skillGroupId={bindUsersId}
        open={bindUsersId !== null}
        onCancel={() => setBindUsersId(null)}
      />

      {/* Phone Binding Modal */}
      <PhoneBindingModal
        skillGroupId={bindPhonesId}
        open={bindPhonesId !== null}
        onCancel={() => setBindPhonesId(null)}
      />
    </Space>
  )
}

// Sub-component for User Binding
type UserBindingModalProps = {
  skillGroupId: number | null
  open: boolean
  onCancel: () => void
}

function UserBindingModal({ skillGroupId, open, onCancel }: UserBindingModalProps) {
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  
  const { data: accountsData, isLoading: accountsLoading } = useQuery({
    queryKey: ['merchant', 'account', 'list'],
    queryFn: () => fetchMerchantAccounts(1, 1000),
    enabled: open && !!skillGroupId,
  })
  
  const { data: boundData, isLoading: boundLoading } = useQuery({
    queryKey: ['merchant', 'skill-group', 'bound-users', skillGroupId],
    queryFn: () => fetchSkillGroupUsers(skillGroupId!),
    enabled: open && !!skillGroupId,
  })

  useEffect(() => {
    if (boundData?.userIds) {
      setSelectedRowKeys(boundData.userIds)
    } else {
      setSelectedRowKeys([])
    }
  }, [boundData, open])

  const mutation = useMutation({
    mutationFn: async (userIds: number[]) => saveSkillGroupUsers(skillGroupId!, userIds),
    onSuccess: () => {
      message.success('成员绑定已更新')
      onCancel()
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '绑定失败'),
  })

  return (
    <Modal
      open={open}
      title="绑定成员"
      onCancel={onCancel}
      onOk={() => mutation.mutate(selectedRowKeys as number[])}
      confirmLoading={mutation.isPending}
      width={600}
      destroyOnClose
    >
      <Table
        loading={accountsLoading || boundLoading}
        dataSource={accountsData?.records ?? []}
        rowKey="id"
        size="small"
        pagination={{ pageSize: 10 }}
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys),
        }}
        columns={[
          { title: '用户 ID', dataIndex: 'id' },
          { title: '用户名', dataIndex: 'username' },
          { title: '账号类型', dataIndex: 'accountType' },
        ]}
      />
    </Modal>
  )
}

// Sub-component for Phone Binding
type PhoneBindingModalProps = {
  skillGroupId: number | null
  open: boolean
  onCancel: () => void
}

function PhoneBindingModal({ skillGroupId, open, onCancel }: PhoneBindingModalProps) {
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])

  const { data: phonesData, isLoading: phonesLoading } = useQuery({
    queryKey: ['operate', 'pool-phone', 'list'],
    queryFn: () => fetchPoolPhones(1, 1000),
    enabled: open && !!skillGroupId,
  })

  const { data: boundData, isLoading: boundLoading } = useQuery({
    queryKey: ['merchant', 'skill-group', 'bound-phones', skillGroupId],
    queryFn: () => fetchSkillGroupPhones(skillGroupId!),
    enabled: open && !!skillGroupId,
  })

  useEffect(() => {
    if (boundData?.phoneIds) {
      setSelectedRowKeys(boundData.phoneIds)
    } else {
      setSelectedRowKeys([])
    }
  }, [boundData, open])

  const mutation = useMutation({
    mutationFn: async (phoneIds: number[]) => saveSkillGroupPhones(skillGroupId!, phoneIds),
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
