import { Button, Card, Space, Table, Tag, Typography, message, Modal, Form, Input, InputNumber, Popconfirm } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import type { ColumnsType } from 'antd/es/table'
import { PermissionGate } from '../../../components/PermissionGate'
import { useAuthStore } from '../../../store/auth'
import {
  fetchBatchTasks,
  saveBatchTask,
  startBatchDialpad,
  pauseBatchDialpad,
  resumeBatchDialpad,
  disconnectPauseBatchDialpad
} from '../../../api/operate'

type DialpadItem = {
  id: number
  name: string
  scope: string
  status: 'enabled' | 'disabled'
  updatedAt: string
  calledCount: number
  totalCount: number
}

export function BatchDialpadPage() {
  const tenant = useAuthStore((state) => state.tenant)
  const merchantId = Number(tenant?.merchantId || '1001')
  const userId = Number(tenant?.userId || '1')

  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()

  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['merchant', 'batch-dialpad'],
    queryFn: () => fetchBatchTasks(1, 50),
  })

  const dialpads: DialpadItem[] = (data?.records ?? []).map((item) => ({
    id: item.id,
    name: item.name,
    scope: `${item.merchant} (任务 ID: ${item.id})`,
    status: item.status === 'running' ? 'enabled' : 'disabled',
    updatedAt: item.completed > 0 ? `${item.completed} / ${item.total} 已拨` : '等待呼叫',
    calledCount: item.completed,
    totalCount: item.total,
  }))

  const startMutation = useMutation({
    mutationFn: async (id: number) => startBatchDialpad(id),
    onSuccess: async () => {
      message.success('拨号盘已启动')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-dialpad'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '启动失败'),
  })

  const pauseMutation = useMutation({
    mutationFn: async (id: number) => pauseBatchDialpad(id, '手动暂停'),
    onSuccess: async () => {
      message.success('拨号盘已暂停')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-dialpad'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '暂停失败'),
  })

  const resumeMutation = useMutation({
    mutationFn: async (id: number) => resumeBatchDialpad(id),
    onSuccess: async () => {
      message.success('拨号盘已恢复呼叫')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-dialpad'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '恢复失败'),
  })

  const disconnectMutation = useMutation({
    mutationFn: async (id: number) => disconnectPauseBatchDialpad(id, '线路强制断开'),
    onSuccess: async () => {
      message.warning('已断开并暂停拨号盘')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-dialpad'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '断开操作失败'),
  })

  const syncRulesMutation = useMutation({
    mutationFn: async () => {
      // 模拟拨号规则下发同步
      return new Promise((resolve) => setTimeout(resolve, 600))
    },
    onSuccess: () => {
      message.success('拨号路由及并发规则同步成功')
    },
  })

  const createDialpadMutation = useMutation({
    mutationFn: async (values: any) =>
      saveBatchTask({
        name: values.name,
        merchantId,
        userId,
        connectedInterval: 600,
        unconnectedInterval: 1200,
        callTimePeriod: '09:00-12:00,14:00-18:00',
        aiFlag: true,
        enable: false, // 默认先挂起
        totalCount: values.totalCount ?? 1000,
      }),
    onSuccess: async () => {
      message.success('新拨号盘已成功创建')
      setOpen(false)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-dialpad'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '创建失败'),
  })

  const columns: ColumnsType<DialpadItem> = [
    { title: '拨号盘 ID', dataIndex: 'id', width: 100 },
    { title: '拨号盘名称', dataIndex: 'name' },
    { title: '作用域/商户分组', dataIndex: 'scope' },
    {
      title: '并发状态',
      dataIndex: 'status',
      width: 120,
      render: (value: string) => (
        <Tag color={value === 'enabled' ? 'green' : 'default'}>
          {value === 'enabled' ? '进行中' : '已暂停'}
        </Tag>
      ),
    },
    { title: '拨号进度', dataIndex: 'updatedAt' },
    {
      title: '操作',
      key: 'actions',
      width: 280,
      render: (_, record) => {
        const isEnabled = record.status === 'enabled'
        return (
          <Space size="middle">
            <PermissionGate permission="merchant:batch-dialpad:control">
              {!isEnabled ? (
                <Button
                  type="link"
                  onClick={() => startMutation.mutate(record.id)}
                  loading={startMutation.isPending}
                >
                  启动
                </Button>
              ) : (
                <Button
                  type="link"
                  onClick={() => pauseMutation.mutate(record.id)}
                  loading={pauseMutation.isPending}
                >
                  暂停
                </Button>
              )}
              <Button
                type="link"
                disabled={isEnabled}
                onClick={() => resumeMutation.mutate(record.id)}
                loading={resumeMutation.isPending}
              >
                恢复
              </Button>
              <Popconfirm
                title="确定要紧急断开并暂停该拨号盘的所有并发呼叫吗？"
                onConfirm={() => disconnectMutation.mutate(record.id)}
                okText="紧急断开"
                cancelText="取消"
              >
                <Button type="link" danger disabled={!isEnabled}>
                  紧急断开
                </Button>
              </Popconfirm>
            </PermissionGate>
          </Space>
        )
      },
    },
  ]

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-between items-center mb-2">
        <Typography.Text type="secondary">
          配置并发呼叫频率，监控并发水位，并控制当前拨号盘的生命周期状态。
        </Typography.Text>
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-dialpad'] })}>刷新</Button>
          <PermissionGate permission="merchant:batch-dialpad:control">
            <Button
              onClick={() => syncRulesMutation.mutate()}
              loading={syncRulesMutation.isPending}
            >
              同步规则
            </Button>
          </PermissionGate>
          <PermissionGate permission="merchant:batch-dialpad:control">
            <Button type="primary" onClick={() => setOpen(true)}>
              新建拨号盘
            </Button>
          </PermissionGate>
        </Space>
      </div>
      <Card className="shadow-soft">
        <Table
          rowKey="id"
          columns={columns}
          dataSource={dialpads}
          loading={isLoading}
          pagination={false}
        />
      </Card>

      <Modal
        title="新建拨号盘"
        open={open}
        onCancel={() => {
          setOpen(false)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={createDialpadMutation.isPending}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={(values) => createDialpadMutation.mutate(values)}>
          <Form.Item
            name="name"
            label="拨号盘名称"
            rules={[{ required: true, message: '请输入拨号盘任务名称' }]}
          >
            <Input placeholder="例如: 智能回访预测拨号盘" />
          </Form.Item>

          <Form.Item
            name="totalCount"
            label="号码池分配总数量"
            rules={[{ required: true, message: '请输入号码池总数量' }]}
          >
            <InputNumber min={1} max={100000} className="w-full" placeholder="默认 1000 条" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
