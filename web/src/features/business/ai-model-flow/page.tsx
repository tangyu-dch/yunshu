import { Button, Form, Input, Modal, Popconfirm, Space, Tag, Typography, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { PermissionGate } from '../../../components/PermissionGate'
import { TableWrap } from '../../../components/TableWrap'
import {
  fetchAiFlows,
  saveAiFlow,
  deleteAiFlows,
  precheckAiFlow,
  publishAiFlow
} from '../../../api/operate'

type AiFlowFormValues = {
  id?: number
  name: string
  prompt: string
  description?: string
}

export function AiModelFlowPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])

  const [form] = Form.useForm<AiFlowFormValues>()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['merchant', 'ai-flow', pageNumber, pageSize],
    queryFn: () => fetchAiFlows(pageNumber, pageSize),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: AiFlowFormValues) =>
      saveAiFlow({
        id: editingId ?? undefined,
        name: values.name,
        prompt: values.prompt,
        description: values.description,
      }),
    onSuccess: async () => {
      message.success(editingId ? '模型流已更新' : '模型流已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteAiFlows(ids),
    onSuccess: async () => {
      message.success('模型流已删除')
      setSelectedIds([])
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const precheckMutation = useMutation({
    mutationFn: async (flow: any) => precheckAiFlow(flow),
    onSuccess: async () => {
      message.success('模型流预检查通过')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '预检查失败'),
  })

  const publishMutation = useMutation({
    mutationFn: async (id: number) => publishAiFlow(id),
    onSuccess: async () => {
      message.success('模型流已成功发布')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '发布失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        name: record?.name ?? '',
        // 对于只读的模型流列表，可默认把 description 与 prompt 关联
        prompt: record?.prompt || '请进行相关质检，检测以下对话的异常情绪：',
        description: record?.description || '',
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-between items-center mb-2">
        <Typography.Text type="secondary">
          管理用于自动质检、业务流分析及大模型意向分层的提示词模型流。
        </Typography.Text>
        <Space>
          <Button onClick={() => queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })}>刷新</Button>
          {selectedIds.length > 0 && (
            <PermissionGate permission="merchant:ai-flow:write">
              <Popconfirm
                title={`确定要删除选中的 ${selectedIds.length} 个模型流吗？`}
                onConfirm={() => deleteMutation.mutate(selectedIds)}
                okText="确定"
                cancelText="取消"
              >
                <Button danger>批量删除</Button>
              </Popconfirm>
            </PermissionGate>
          )}
          <PermissionGate permission="merchant:ai-flow:write">
            <Button type="primary" onClick={openCreate}>
              新建模型流
            </Button>
          </PermissionGate>
        </Space>
      </div>

      <TableWrap
        title="模型流列表"
        rowKey="id"
        loading={isLoading}
        dataSource={data?.records ?? []}
        rowSelection={{
          selectedRowKeys: selectedIds,
          onChange: (keys: any[]) => setSelectedIds(keys as number[]),
        }}
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
          { title: '模型流 ID', dataIndex: 'id', width: 100 },
          { title: '名称', dataIndex: 'name' },
          { title: '商户', dataIndex: 'merchant' },
          { title: '版本', dataIndex: 'version', width: 120 },
          {
            title: '状态',
            dataIndex: 'status',
            width: 120,
            render: (value: string) => {
              let color = 'default'
              let label = '未预检'
              if (value === 'published') {
                color = 'green'
                label = '已发布'
              } else if (value === 'draft') {
                color = 'gold'
                label = '草稿 (预检)'
              }
              return <Tag color={color}>{label}</Tag>
            },
          },
          { title: '更新时间', dataIndex: 'updatedAt', width: 180 },
          {
            title: '操作',
            key: 'actions',
            width: 280,
            render: (_, record) => {
              const isPublished = record.status === 'published'
              const isDraft = record.status === 'draft'
              return (
                <Space size="middle">
                  <PermissionGate permission="merchant:ai-flow:precheck">
                    <Button
                      type="link"
                      onClick={() =>
                        precheckMutation.mutate({
                          id: record.id,
                          name: record.name,
                          prompt: '请进行相关质检，检测对话异常：',
                        })
                      }
                    >
                      预检
                    </Button>
                  </PermissionGate>
                  <PermissionGate permission="merchant:ai-flow:publish">
                    <Button
                      type="link"
                      disabled={isPublished}
                      onClick={() => publishMutation.mutate(record.id)}
                    >
                      发布
                    </Button>
                  </PermissionGate>
                  <PermissionGate permission="merchant:ai-flow:write">
                    <Button type="link" onClick={() => openEdit(record.id)}>
                      编辑
                    </Button>
                    <Popconfirm
                      title="确定要删除该模型流吗？"
                      onConfirm={() => deleteMutation.mutate([record.id])}
                      okText="确定"
                      cancelText="取消"
                    >
                      <Button type="link" danger>
                        删除
                      </Button>
                    </Popconfirm>
                  </PermissionGate>
                </Space>
              )
            },
          },
        ]}
      />

      <Modal
        title={editingId ? '编辑 AI 模型流' : '创建 AI 模型流'}
        open={open}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)}>
          <Form.Item
            name="name"
            label="模型流名称"
            rules={[{ required: true, message: '请输入模型流名称' }]}
          >
            <Input placeholder="例如: 客户意向智能分析模型" />
          </Form.Item>

          <Form.Item
            name="prompt"
            label="提示词 (Prompt)"
            rules={[{ required: true, message: '请输入模型提示词 Prompt' }]}
          >
            <Input.TextArea
              rows={6}
              placeholder="编写发送给大语言模型的系统提示词 Prompt。例如:
你是一个智能客服分析助手，分析以下对话内容中用户的意向，并以 JSON 格式输出意向等级 (A/B/C/D)..."
            />
          </Form.Item>

          <Form.Item name="description" label="描述 (可选)">
            <Input.TextArea placeholder="该模型流的用途或修改备注" rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
