import { Button, Form, Input, InputNumber, Modal, Radio, Space, Tabs, Tag, Typography, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { TableWrap } from '@/components/TableWrap'
import { fetchBillingOverview, fetchMerchants, fetchRechargeRecords, rechargeMerchant, saveBillingOverview } from '@/api/operate'

type BillingFormValues = {
  id?: number
  merchantId: string
  paymentMode: number
  creditLimit: number
}

type RechargeFormValues = {
  merchantId: string
  amount: number
  remark?: string
}

export function BillingPage() {
  const [activeTab, setActiveTab] = useState('overview')
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [rechargeOpen, setRechargeOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [selectedMerchantId, setSelectedMerchantId] = useState<string | null>(null)

  const [editForm] = Form.useForm<BillingFormValues>()
  const [rechargeForm] = Form.useForm<RechargeFormValues>()
  const queryClient = useQueryClient()

  // Load merchants list to resolve merchant names
  const { data: merchantsData } = useQuery({
    queryKey: ['operate', 'merchant', 1, 100],
    queryFn: () => fetchMerchants(1, 100),
  })

  // Fetch Billing Overview
  const { data: overviewData } = useQuery({
    queryKey: ['operate', 'billing', 'overview', pageNumber, pageSize],
    queryFn: () => fetchBillingOverview(pageNumber, pageSize),
    enabled: activeTab === 'overview',
  })

  // Fetch Recharge Logs
  const { data: recordsData } = useQuery({
    queryKey: ['operate', 'billing', 'recharge', pageNumber, pageSize],
    queryFn: () => fetchRechargeRecords(pageNumber, pageSize),
    enabled: activeTab === 'records',
  })

  const saveOverviewMutation = useMutation({
    mutationFn: async (values: BillingFormValues) => saveBillingOverview(values),
    onSuccess: async () => {
      message.success('账务设置已更新')
      setEditOpen(false)
      editForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'billing'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '设置失败'),
  })

  const rechargeMutation = useMutation({
    mutationFn: async (values: RechargeFormValues) => rechargeMerchant(values),
    onSuccess: async () => {
      message.success('充值成功')
      setRechargeOpen(false)
      rechargeForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'billing'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '充值失败'),
  })

  function getMerchantName(mId: string | number) {
    const found = merchantsData?.records.find((m: any) => String(m.id) === String(mId))
    return found ? found.name : `商户 ${mId}`
  }

  function openEdit(record: any) {
    setSelectedMerchantId(String(record.merchantId))
    setEditOpen(true)
    setTimeout(() => {
      editForm.setFieldsValue({
        id: record.id,
        merchantId: String(record.merchantId),
        paymentMode: record.paymentMode ?? 1,
        creditLimit: record.creditLimit ?? 0,
      })
    }, 0)
  }

  function openRecharge(record: any) {
    setSelectedMerchantId(String(record.merchantId))
    setRechargeOpen(true)
    setTimeout(() => {
      rechargeForm.setFieldsValue({
        merchantId: String(record.merchantId),
        amount: 100,
        remark: '',
      })
    }, 0)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <Tabs
        activeKey={activeTab}
        onChange={(key) => {
          setActiveTab(key)
          setPageNumber(1)
        }}
        items={[
          {
            key: 'overview',
            label: '商户账务概览',
            children: (
              <TableWrap
                title="账务列表"
                rowKey="id"
                dataSource={overviewData?.records ?? []}
                pagination={{
                  current: pageNumber,
                  pageSize,
                  total: overviewData?.total ?? 0,
                  onChange: (current, size) => {
                    setPageNumber(current)
                    setPageSize(size ?? pageSize)
                  },
                  showSizeChanger: true,
                }}
                columns={[
                  {
                    title: '商户名称',
                    dataIndex: 'merchantId',
                    render: (val: any) => getMerchantName(val),
                  },
                  {
                    title: '付费模式',
                    dataIndex: 'paymentMode',
                    render: (val: number) => {
                      return val === 1 ? <Tag color="blue">预付费</Tag> : <Tag color="purple">后付费</Tag>
                    },
                  },
                  {
                    title: '当前余额',
                    dataIndex: 'currentBalance',
                    render: (val: number) => <span className="font-semibold text-slate-800">￥{(val ?? 0).toFixed(2)}</span>,
                  },
                  {
                    title: '信用额度',
                    dataIndex: 'creditLimit',
                    render: (val: number) => `￥${(val ?? 0).toFixed(2)}`,
                  },
                  {
                    title: '操作',
                    render: (_, record) => (
                      <Space size="small">
                        <Button size="small" type="primary" ghost onClick={() => openRecharge(record)}>
                          充值
                        </Button>
                        <Button size="small" onClick={() => openEdit(record)}>
                          设置额度
                        </Button>
                      </Space>
                    ),
                  },
                ]}
              />
            ),
          },
          {
            key: 'records',
            label: '充值历史日志',
            children: (
              <TableWrap
                title="充值流水"
                rowKey="id"
                dataSource={recordsData?.records ?? []}
                pagination={{
                  current: pageNumber,
                  pageSize,
                  total: recordsData?.total ?? 0,
                  onChange: (current, size) => {
                    setPageNumber(current)
                    setPageSize(size ?? pageSize)
                  },
                  showSizeChanger: true,
                }}
                columns={[
                  { title: '编号', dataIndex: 'id' },
                  {
                    title: '商户名称',
                    dataIndex: 'merchantId',
                    render: (val: any) => getMerchantName(val),
                  },
                  {
                    title: '充值金额',
                    dataIndex: 'amount',
                    render: (val: number) => <span className="font-bold text-green-600">+￥{(val ?? 0).toFixed(2)}</span>,
                  },
                  { title: '备注', dataIndex: 'remark' },
                  { title: '操作人', dataIndex: 'operator' },
                  {
                    title: '充值时间',
                    dataIndex: 'createdTime',
                    render: (val: string) => val ? new Date(val).toLocaleString() : '-',
                  },
                ]}
              />
            ),
          },
        ]}
      />

      <Modal
        open={editOpen}
        title="设置商户账务模式"
        onCancel={() => {
          setEditOpen(false)
          editForm.resetFields()
        }}
        onOk={() => editForm.submit()}
        confirmLoading={saveOverviewMutation.isPending}
        destroyOnHidden
      >
        <Form
          form={editForm}
          layout="vertical"
          onFinish={(values) => {
            saveOverviewMutation.mutate(values)
          }}
        >
          <Form.Item name="id" hidden>
            <Input />
          </Form.Item>
          <Form.Item name="merchantId" hidden>
            <Input />
          </Form.Item>

          <Typography.Paragraph>
            正在设置商户：<strong>{selectedMerchantId ? getMerchantName(selectedMerchantId) : ''}</strong>
          </Typography.Paragraph>

          <Form.Item name="paymentMode" label="付费模式" rules={[{ required: true }]}>
            <Radio.Group>
              <Radio value={1}>预付费</Radio>
              <Radio value={2}>后付费</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item name="creditLimit" label="信用透支额度 (￥)" rules={[{ required: true, message: '请输入额度' }]}>
            <InputNumber className="w-full" min={0} precision={2} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={rechargeOpen}
        title="商户资金充值"
        onCancel={() => {
          setRechargeOpen(false)
          rechargeForm.resetFields()
        }}
        onOk={() => rechargeForm.submit()}
        confirmLoading={rechargeMutation.isPending}
        destroyOnHidden
      >
        <Form
          form={rechargeForm}
          layout="vertical"
          onFinish={(values) => {
            rechargeMutation.mutate(values)
          }}
        >
          <Form.Item name="merchantId" hidden>
            <Input />
          </Form.Item>

          <Typography.Paragraph>
            充值对象商户：<strong>{selectedMerchantId ? getMerchantName(selectedMerchantId) : ''}</strong>
          </Typography.Paragraph>

          <Form.Item name="amount" label="充值金额 (￥)" rules={[{ required: true, message: '请输入充值金额' }]}>
            <InputNumber className="w-full" min={0.01} precision={2} />
          </Form.Item>

          <Form.Item name="remark" label="充值说明/备注">
            <Input placeholder="请输入付款凭证号或备注" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
