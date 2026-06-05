import { Card, Col, Descriptions, Row, Space, Tag } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import {
  WalletOutlined,
  SafetyCertificateOutlined,
  PayCircleOutlined,
  FileTextOutlined,
} from '@ant-design/icons'
import { TableWrap } from '@/components/TableWrap'
import { fetchBillingOverview, fetchMerchants, fetchRates, fetchRechargeRecords } from '@/api/operate'
import { useAuthStore } from '@/store/auth'

export function MerchantBillingPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const queryClient = useQueryClient()

  const tenant = useAuthStore((state) => state.tenant)
  const currentMerchantId = tenant?.merchantId


  // Load merchants list to resolve merchant names and rate packages
  const { data: merchantsData, isLoading: isMerchantLoading } = useQuery({
    queryKey: ['operate', 'merchant', 1, 100],
    queryFn: () => fetchMerchants(1, 100),
  })

  const currentMerchant = merchantsData?.records.find(
    (m: any) => String(m.id) === String(currentMerchantId)
  )

  // Fetch Billing Overview for this merchant
  const { data: billingOverviewData, isLoading: isOverviewLoading } = useQuery({
    queryKey: ['operate', 'billing', 'overview', currentMerchant?.name],
    queryFn: () => fetchBillingOverview(1, 100, currentMerchant?.name || ''),
    enabled: !!currentMerchant?.name,
  })

  const currentBillingOverview = billingOverviewData?.records.find(
    (b: any) => String(b.merchantId) === String(currentMerchantId)
  )

  // Fetch Rates to find rate details
  const { data: ratesData, isLoading: isRatesLoading } = useQuery({
    queryKey: ['operate', 'rate', 1, 100],
    queryFn: () => fetchRates(1, 100),
  })

  const currentRate = ratesData?.records.find(
    (r: any) => r.id === currentMerchant?.rateId
  )

  // Fetch Recharge Logs
  const { data: recordsData, isLoading: isRecordsLoading } = useQuery({
    queryKey: ['operate', 'billing', 'recharge', pageNumber, pageSize, currentMerchant?.name],
    queryFn: () => fetchRechargeRecords(pageNumber, pageSize, currentMerchant?.name || ''),
    enabled: !!currentMerchant?.name,
  })

  // Safely filter logs just in case of name overlaps
  const filteredRechargeLogs =
    recordsData?.records.filter(
      (r: any) => String(r.merchantId) === String(currentMerchantId)
    ) ?? []

  const isLoading = isMerchantLoading || isOverviewLoading || isRatesLoading

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* Cards for Overview */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} md={6}>
          <Card
            variant="borderless"
            loading={isLoading}
            className="shadow-sm border border-slate-100 hover:shadow-md transition-shadow duration-200"
          >
            <div className="flex items-center justify-between mb-4">
              <span className="text-slate-500 text-sm font-medium">当前账户余额</span>
              <div className="p-2 bg-green-50 text-green-600 rounded-full">
                <WalletOutlined className="text-lg" />
              </div>
            </div>
            <div>
              <span className="text-2xl font-bold text-slate-800">
                ￥{(currentBillingOverview?.currentBalance ?? 0).toFixed(2)}
              </span>
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} md={6}>
          <Card
            variant="borderless"
            loading={isLoading}
            className="shadow-sm border border-slate-100 hover:shadow-md transition-shadow duration-200"
          >
            <div className="flex items-center justify-between mb-4">
              <span className="text-slate-500 text-sm font-medium">可用透支额度</span>
              <div className="p-2 bg-blue-50 text-blue-600 rounded-full">
                <SafetyCertificateOutlined className="text-lg" />
              </div>
            </div>
            <div>
              <span className="text-2xl font-bold text-slate-800">
                ￥{(currentBillingOverview?.creditLimit ?? 0).toFixed(2)}
              </span>
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} md={6}>
          <Card
            variant="borderless"
            loading={isLoading}
            className="shadow-sm border border-slate-100 hover:shadow-md transition-shadow duration-200"
          >
            <div className="flex items-center justify-between mb-4">
              <span className="text-slate-500 text-sm font-medium">当前付费模式</span>
              <div className="p-2 bg-purple-50 text-purple-600 rounded-full">
                <PayCircleOutlined className="text-lg" />
              </div>
            </div>
            <div>
              {currentBillingOverview?.paymentModeCode === 2 ? (
                <Tag color="purple" className="text-base px-3 py-1 font-semibold border-none">
                  后付费模式
                </Tag>
              ) : (
                <Tag color="blue" className="text-base px-3 py-1 font-semibold border-none">
                  预充值模式
                </Tag>
              )}
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} md={6}>
          <Card
            variant="borderless"
            loading={isLoading}
            className="shadow-sm border border-slate-100 hover:shadow-md transition-shadow duration-200"
          >
            <div className="flex items-center justify-between mb-4">
              <span className="text-slate-500 text-sm font-medium">关联计费套餐</span>
              <div className="p-2 bg-amber-50 text-amber-600 rounded-full">
                <FileTextOutlined className="text-lg" />
              </div>
            </div>
            <div>
              <span className="text-base font-bold text-slate-800 block truncate">
                {currentRate?.rateName || '未关联计费套餐'}
              </span>
            </div>
          </Card>
        </Col>
      </Row>

      {/* Package Rates Descriptions */}
      <Card
        title="套餐费率详情"
        variant="borderless"
        loading={isLoading}
        className="shadow-sm border border-slate-100"
      >
        {currentRate ? (
          <Descriptions bordered column={{ xs: 1, sm: 2, md: 3 }}>
            <Descriptions.Item label="套餐名称">{currentRate.rateName}</Descriptions.Item>
            <Descriptions.Item label="费率标准 (元/分钟)">
              <span className="font-semibold text-amber-600">
                ￥{(currentRate.billingPrice ?? 0).toFixed(4)} / 分钟
              </span>
            </Descriptions.Item>
            <Descriptions.Item label="计费周期">{currentRate.billingCycle} 秒</Descriptions.Item>
            <Descriptions.Item label="套餐备注" span={3}>
              {currentRate.remark || '无备注'}
            </Descriptions.Item>
          </Descriptions>
        ) : (
          <div className="text-center py-6 text-slate-400">暂未配置计费套餐，请联系管理员配置。</div>
        )}
      </Card>

      {/* Recharge Logs */}
      <TableWrap
        title="资金充值流水"
        rowKey="id"
        dataSource={filteredRechargeLogs}
        loading={isRecordsLoading}
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
          { title: '记录编号', dataIndex: 'id', width: 100 },
          {
            title: '充值金额',
            dataIndex: 'amount',
            width: 150,
            render: (val: number) => (
              <span className="font-bold text-green-600">+￥{(val ?? 0).toFixed(2)}</span>
            ),
          },
          { title: '说明/备注', dataIndex: 'remark' },
          {
            title: '充值时间',
            dataIndex: 'createdTime',
            width: 200,
            render: (val: string) => (val ? new Date(val).toLocaleString() : '-'),
          },
        ]}
      />
    </Space>
  )
}
