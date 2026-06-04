import { Button, Form, Input, Modal, Popconfirm, Space, Switch, Tag, Typography, Checkbox, message, Row, Col, Card } from 'antd'
import { InfoCircleOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import {
  fetchRoles,
  saveRole,
  deleteRoles,
  toggleRoleEnable,
  fetchPermissions,
  fetchRolePermissions,
  saveRolePermissions,
} from '@/api/operate'

type RoleFormValues = {
  code: string
  name: string
  description?: string
  enable: boolean
}

export function RolePermissionPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [nameQuery, setNameQuery] = useState('')
  const [open, setOpen] = useState(false)
  const [editingCode, setEditingCode] = useState<string | null>(null)
  
  // Mappings state
  const [permOpen, setPermOpen] = useState(false)
  const [selectedRoleCode, setSelectedRoleCode] = useState<string | null>(null)
  const [selectedRoleName, setSelectedRoleName] = useState('')
  const [checkedKeys, setCheckedKeys] = useState<string[]>([])

  const queryFields = useMemo(() => [
    { key: 'name', label: '角色名称', type: 'text' as const, placeholder: '请输入角色名称搜索' },
  ], [])

  const [form] = Form.useForm<RoleFormValues>()
  const queryClient = useQueryClient()

  // Fetch Roles
  const { data: rolesData, isLoading: isRolesLoading } = useQuery({
    queryKey: ['operate', 'role', pageNumber, pageSize, nameQuery],
    queryFn: () => fetchRoles(pageNumber, pageSize, nameQuery),
  })

  // Fetch All Permissions
  const { data: allPermissions = [] } = useQuery({
    queryKey: ['operate', 'permission'],
    queryFn: fetchPermissions,
  })

  // Toggle Enable Mutation
  const toggleMutation = useMutation({
    mutationFn: async ({ code, enable }: { code: string; enable: boolean }) => toggleRoleEnable(code, enable),
    onSuccess: async () => {
      message.success('角色状态已更新')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'role'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '操作失败'),
  })

  // Delete Mutation
  const deleteMutation = useMutation({
    mutationFn: async (roles: { code: string }[]) => deleteRoles(roles),
    onSuccess: async () => {
      message.success('角色已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'role'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败，默认核心角色不可删除'),
  })

  // Save Role Mutation
  const saveMutation = useMutation({
    mutationFn: async (values: RoleFormValues) => saveRole(values, !!editingCode),
    onSuccess: async () => {
      message.success(editingCode ? '角色信息已更新' : '角色已创建')
      setOpen(false)
      setEditingCode(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'role'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  // Save Permissions Mutation
  const savePermsMutation = useMutation({
    mutationFn: async () => {
      if (!selectedRoleCode) return
      return saveRolePermissions(selectedRoleCode, checkedKeys)
    },
    onSuccess: () => {
      message.success('功能权限已成功保存并下发')
      setPermOpen(false)
      setSelectedRoleCode(null)
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存权限失败'),
  })

  function openCreate() {
    setEditingCode(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ enable: true })
    }, 0)
  }

  function openEdit(record: any) {
    setEditingCode(record.code)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        code: record.code,
        name: record.name,
        description: record.description,
        enable: record.enable,
      })
    }, 0)
  }

  async function openAssignPermissions(record: any) {
    setSelectedRoleCode(record.code)
    setSelectedRoleName(record.name)
    try {
      const keys = await fetchRolePermissions(record.code)
      setCheckedKeys(keys)
      setPermOpen(true)
    } catch {
      message.error('加载角色权限失败')
    }
  }

  // Group permissions by module
  const groupedPermissions = allPermissions.reduce((acc: Record<string, any[]>, item: any) => {
    const mod = item.module || 'other'
    if (!acc[mod]) {
      acc[mod] = []
    }
    acc[mod].push(item)
    return acc
  }, {})

  const getModuleLabel = (moduleName: string) => {
    switch (moduleName) {
      case 'console': return '系统控制端'
      case 'operate': return '运营管理模块'
      case 'merchant': return '商户管理模块'
      default: return moduleName
    }
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <QueryBar
        fields={queryFields}
        onSearch={(params) => { setPageNumber(1); setNameQuery(params.name || '') }}
        loading={isRolesLoading}
      />

      <div className="flex justify-end mb-2">
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['operate', 'role'] })}>刷新</Button>
          <Button type="primary" onClick={openCreate}>
            新增角色
          </Button>
        </Space>
      </div>

      <TableWrap
        title="角色列表"
        rowKey="code"
        loading={isRolesLoading}
        dataSource={rolesData?.records ?? []}
        pagination={{
          current: pageNumber,
          pageSize,
          total: rolesData?.total ?? 0,
          onChange: (current, size) => {
            setPageNumber(current)
            setPageSize(size ?? pageSize)
          },
          showSizeChanger: true,
        }}
        columns={[
          { title: '角色编码', dataIndex: 'code', width: 180 },
          { title: '角色名称', dataIndex: 'name', width: 180 },
          { title: '角色描述', dataIndex: 'description' },
          {
            title: '状态',
            dataIndex: 'enable',
            width: 100,
            render: (value: boolean) => <Tag color={value ? 'green' : 'default'}>{value ? '启用' : '停用'}</Tag>,
          },
          {
            title: '操作',
            width: 260,
            render: (_, record) => {
              const isCoreRole = ['super_admin', 'merchant_admin', 'merchant_user'].includes(record.code)
              return (
                <Space size="small">
                  <Button size="small" onClick={() => openEdit(record)}>
                    编辑
                  </Button>
                  <Button size="small" type="primary" ghost onClick={() => openAssignPermissions(record)} disabled={record.code === 'super_admin'}>
                    分配权限
                  </Button>
                  <Button size="small" onClick={() => toggleMutation.mutate({ code: record.code, enable: !record.enable })}>
                    {record.enable ? '停用' : '启用'}
                  </Button>
                  {!isCoreRole ? (
                    <Popconfirm title="确认删除该自定义角色？" onConfirm={() => deleteMutation.mutate([{ code: record.code }])}>
                      <Button size="small" danger>
                        删除
                      </Button>
                    </Popconfirm>
                  ) : (
                    <Button size="small" danger disabled title="系统核心角色不可删除">
                      删除
                    </Button>
                  )}
                </Space>
              )
            },
          },
        ]}
      />

      {/* Save/Edit Role Modal */}
      <Modal
        open={open}
        title={editingCode ? '编辑角色信息' : '新增角色'}
        width={640}
        onCancel={() => {
          setOpen(false)
          setEditingCode(null)
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
            saveMutation.mutate(values)
          }}
          initialValues={{ enable: true }}
        >
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-1">
            <Form.Item
              name="code"
              label="角色编码 (英文标识)"
              rules={[
                { required: true, message: '请输入角色编码' },
                { pattern: /^[a-zA-Z0-9_]+$/, message: '仅支持字母、数字及下划线' }
              ]}
            >
              <Input disabled={Boolean(editingCode)} placeholder="例如: operate_staff" />
            </Form.Item>
            <Form.Item name="name" label="角色名称" rules={[{ required: true, message: '请输入角色名称' }]}>
              <Input placeholder="例如: 运营专员" />
            </Form.Item>
            <Form.Item name="enable" label="启用状态" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="description" label="角色描述" className="col-span-1 md:col-span-2">
              <Input.TextArea placeholder="简短描述该角色拥有的工作权限范围" rows={3} />
            </Form.Item>
          </div>
        </Form>
      </Modal>

      {/* Assign Permissions Modal */}
      <Modal
        open={permOpen}
        title={`分配功能权限 - ${selectedRoleName}`}
        width={750}
        onCancel={() => {
          setPermOpen(false)
          setSelectedRoleCode(null)
        }}
        onOk={() => savePermsMutation.mutate()}
        confirmLoading={savePermsMutation.isPending}
        destroyOnHidden
      >
        <div className="max-h-[500px] overflow-y-auto pr-2 pb-6">
          <Checkbox.Group
            value={checkedKeys}
            onChange={(checkedValues) => setCheckedKeys(checkedValues as string[])}
            className="w-full"
          >
            {Object.keys(groupedPermissions).map((moduleName) => {
              const perms = groupedPermissions[moduleName]
              return (
                <Card
                  key={moduleName}
                  title={<span className="dark:text-slate-100">{getModuleLabel(moduleName)}</span>}
                  size="small"
                  className="w-full mb-4 shadow-sm border border-slate-100 dark:border-slate-800 bg-white dark:bg-[#15181e] hover:border-blue-200 transition-colors"
                  extra={
                    <Button
                      type="link"
                      size="small"
                      onClick={() => {
                        const allCodes = perms.map((p: any) => p.code)
                        const allChecked = allCodes.every((c) => checkedKeys.includes(c))
                        if (allChecked) {
                          setCheckedKeys(checkedKeys.filter((k) => !allCodes.includes(k)))
                        } else {
                          setCheckedKeys(Array.from(new Set([...checkedKeys, ...allCodes])))
                        }
                      }}
                    >
                      全选 / 取消全选
                    </Button>
                  }
                >
                  <Row gutter={[12, 12]}>
                    {perms.map((p: any) => (
                      <Col xs={24} sm={12} key={p.code}>
                        <Checkbox value={p.code} className="align-top dark:text-slate-300">
                          <span className="font-medium text-slate-800 dark:text-slate-200 block">{p.name}</span>
                          <span className="text-xs text-slate-400 dark:text-slate-500 block mt-0.5">{p.description || p.code}</span>
                        </Checkbox>
                      </Col>
                    ))}
                  </Row>
                </Card>
              )
            })}
          </Checkbox.Group>
        </div>
      </Modal>
    </Space>
  )
}
