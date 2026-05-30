import { Button, Form, Input, Modal, Popconfirm, Select, Space, Switch, Tag, Typography, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { TableWrap } from '@/components/TableWrap'
import { deleteAccounts, fetchAccounts, fetchMerchants, saveAccount, toggleAccountEnable, resetAccountPassword } from '@/api/operate'

type AccountFormValues = {
  id?: number
  username: string
  password?: string
  merchantId?: string
  roleId?: string
  accountType: string
  enable: boolean
}

export function AccountPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [resettingId, setResettingId] = useState<number | null>(null)
  const [form] = Form.useForm<AccountFormValues>()
  const [resetForm] = Form.useForm<{ password: string }>()
  const [filterType, setFilterType] = useState<string>('')
  const [filterUser, setFilterUser] = useState<string>('')

  const queryClient = useQueryClient()
  const { data } = useQuery({
    queryKey: ['operate', 'account', pageNumber, pageSize, filterUser, filterType],
    queryFn: () => fetchAccounts(pageNumber, pageSize, filterUser, filterType),
  })

  // Load merchants list to link to accounts
  const { data: merchantsData } = useQuery({
    queryKey: ['operate', 'merchant', 1, 100],
    queryFn: () => fetchMerchants(1, 100),
  })

  const toggleMutation = useMutation({
    mutationFn: async ({ id, enable }: { id: number; enable: boolean }) => toggleAccountEnable(id, enable),
    onSuccess: async () => {
      message.success('账号状态已更新')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'account'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '操作失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteAccounts(ids),
    onSuccess: async () => {
      message.success('账号已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'account'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: AccountFormValues) => saveAccount(values),
    onSuccess: async () => {
      message.success(editingId ? '账号已更新' : '账号已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'account'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  const resetPasswordMutation = useMutation({
    mutationFn: async ({ id, password }: { id: number; password: string }) => resetAccountPassword(id, password),
    onSuccess: async () => {
      message.success('密码重置成功')
      setResettingId(null)
      resetForm.resetFields()
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '重置失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ enable: true, accountType: 'merchant_user' })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item: any) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id: record?.id,
        username: record?.username ?? '',
        merchantId: record?.merchantId ? String(record.merchantId) : undefined,
        roleId: record?.roleId ?? '',
        accountType: record?.accountType ?? 'merchant_user',
        enable: Boolean(record?.enable),
      })
    }, 0)
  }

  const selectedAccountType = Form.useWatch('accountType', form)

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-between items-center mb-2">
        <Typography.Text type="secondary">
          维护平台级管理员、运营人员以及商户的管理员与坐席登录账号。
        </Typography.Text>
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'account'] })}>刷新</Button>
          <Button type="primary" onClick={openCreate}>
            新增账号
          </Button>
        </Space>
      </div>

      <div className="flex gap-4 bg-white p-4 rounded border border-slate-200">
        <Space size="middle">
          <div>
            <span className="mr-2 text-slate-500 text-sm">用户名:</span>
            <Input
              placeholder="请输入用户名检索"
              value={filterUser}
              onChange={(e) => {
                setFilterUser(e.target.value)
                setPageNumber(1)
              }}
              allowClear
              className="w-48"
            />
          </div>
          <div>
            <span className="mr-2 text-slate-500 text-sm">账号类型:</span>
            <Select
              value={filterType}
              onChange={(val) => {
                setFilterType(val)
                setPageNumber(1)
              }}
              className="w-48"
              options={[
                { value: '', label: '全部' },
                { value: 'super_admin', label: '超级管理员' },
                { value: 'operate_user', label: '运营账号' },
                { value: 'merchant_admin', label: '商户管理员' },
                { value: 'merchant_user', label: '商户普通用户' },
              ]}
            />
          </div>
        </Space>
      </div>

      <TableWrap
        title="账号列表"
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
          { title: '用户名', dataIndex: 'username' },
          {
            title: '账号类型',
            dataIndex: 'accountType',
            render: (value: string) => {
              const types: Record<string, { label: string; color: string }> = {
                super_admin: { label: '超级管理员', color: 'red' },
                operate_user: { label: '运营人员', color: 'orange' },
                merchant_admin: { label: '商户管理员', color: 'blue' },
                merchant_user: { label: '商户用户', color: 'green' },
              }
              const info = types[value] || { label: value, color: 'default' }
              return <Tag color={info.color}>{info.label}</Tag>
            },
          },
          {
            title: '归属商户 ID',
            dataIndex: 'merchantId',
            render: (value: string) => value || '-',
          },
          { title: '角色权限 ID', dataIndex: 'roleId' },
          {
            title: '状态',
            dataIndex: 'enable',
            render: (value: boolean, record: any) => (
              <Switch
                checked={value}
                loading={toggleMutation.isPending}
                onChange={(checked) => toggleMutation.mutate({ id: record.id, enable: checked })}
              />
            ),
          },
          {
            title: '操作',
            render: (_, record) => (
              <Space size="small">
                <Button size="small" onClick={() => openEdit(record.id)}>
                  编辑
                </Button>
                <Button size="small" onClick={() => setResettingId(record.id)}>
                  重置密码
                </Button>
                <Popconfirm title="确认删除该账号？" onConfirm={() => deleteMutation.mutate([record.id])}>
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
        title={editingId ? '编辑账号' : '新增账号'}
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
          initialValues={{ enable: true, accountType: 'merchant_user' }}
        >
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input disabled={editingId !== null} />
          </Form.Item>

          {!editingId && (
            <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入初始密码' }]}>
              <Input.Password />
            </Form.Item>
          )}

          <Form.Item name="accountType" label="账号类型" rules={[{ required: true, message: '请选择账号类型' }]}>
            <Select
              options={[
                { value: 'super_admin', label: '超级管理员' },
                { value: 'operate_user', label: '运营账号' },
                { value: 'merchant_admin', label: '商户管理员' },
                { value: 'merchant_user', label: '商户普通用户' },
              ]}
            />
          </Form.Item>

          {(selectedAccountType === 'merchant_admin' || selectedAccountType === 'merchant_user') && (
            <Form.Item name="merchantId" label="关联商户" rules={[{ required: true, message: '请选择关联商户' }]}>
              <Select
                placeholder="请选择绑定的商户"
                options={merchantsData?.records.map((m: any) => ({
                  value: String(m.id),
                  label: `[${m.id}] ${m.name}`,
                }))}
              />
            </Form.Item>
          )}

          <Form.Item name="roleId" label="自定义权限角色代码 (选填)">
            <Input placeholder="如果不填将采用该类型账号默认的角色配置" />
          </Form.Item>

          <Form.Item name="enable" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={resettingId !== null}
        title="重置登录密码"
        onCancel={() => {
          setResettingId(null)
          resetForm.resetFields()
        }}
        onOk={() => resetForm.submit()}
        confirmLoading={resetPasswordMutation.isPending}
        destroyOnClose
      >
        <Form
          form={resetForm}
          layout="vertical"
          onFinish={(values) => {
            if (resettingId !== null) {
              resetPasswordMutation.mutate({ id: resettingId, password: values.password })
            }
          }}
        >
          <Form.Item name="password" label="新密码" rules={[{ required: true, min: 6, message: '请输入新密码，长度不少于 6 位' }]}>
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
