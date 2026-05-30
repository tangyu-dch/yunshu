import { Card, Col, Progress, Row, Space, Tag, Timeline, Typography, Avatar } from 'antd'
import {
  PhoneOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  DashboardOutlined,
  DeploymentUnitOutlined,
  ThunderboltOutlined,
  ApartmentOutlined,
  DatabaseOutlined,
  AuditOutlined,
  CarryOutOutlined
} from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, Link } from 'react-router-dom'
import dayjs from 'dayjs'
import { ChartWrap } from '@/components/ChartWrap'
import { fetchAiFlows, fetchBatchTasks, fetchCallRecords, fetchFsNodes, fetchGatewayPage } from '@/api/operate'
import { useAuthStore } from '@/store/auth'

interface NavItem {
  key: string
  label: string
  icon?: React.ReactNode
  permission?: string
  platform?: 'operate' | 'merchant'
  children?: NavItem[]
}

export function DashboardPage() {
  const navigate = useNavigate()
  
  // 1. Differentiate platforms using Zustand Auth store
  const tenant = useAuthStore((state) => state.tenant)
  const isOperate = tenant?.internal ?? false
  const merchantId = tenant?.merchantId ? Number(tenant.merchantId) : undefined

  // 2. Fetch queries scoped dynamically based on platform context
  const gatewaysQuery = useQuery({
    queryKey: ['dashboard', 'gateways'],
    queryFn: () => fetchGatewayPage(1, 50),
    enabled: isOperate
  })

  const nodesQuery = useQuery({
    queryKey: ['dashboard', 'nodes'],
    queryFn: fetchFsNodes,
    enabled: isOperate
  })

  const batchTasksQuery = useQuery({
    queryKey: ['dashboard', 'batchTasks', isOperate ? 'all' : merchantId],
    queryFn: () => fetchBatchTasks(1, 50),
    enabled: true
  })

  // Scoped Call Records: If Operate, load all records; if Merchant, pass merchantId filter for strict isolation
  const callRecordsQuery = useQuery({
    queryKey: ['dashboard', 'callRecords', isOperate ? 'all' : merchantId],
    queryFn: () => fetchCallRecords(1, 100, isOperate ? {} : { merchantId }),
    enabled: isOperate || !!merchantId
  })

  const aiFlowsQuery = useQuery({
    queryKey: ['dashboard', 'aiFlows'],
    queryFn: () => fetchAiFlows(1, 50),
    enabled: true
  })

  // 3. Compile statistics from GORM-backed Call Records
  const records = callRecordsQuery.data?.records ?? []
  const totalCalls = records.length

  let answered = 0
  let busy = 0
  let failed = 0
  let longCalls = 0 // Duration > 30 seconds

  // Duration buckets
  let durUnder10 = 0
  let dur10to30 = 0
  let dur30to60 = 0
  let durOver60 = 0

  records.forEach((r) => {
    const s = String(r.state).toUpperCase()
    const billsec = Number(r.billsec) || 0
    const isAnswered = s.includes('ANSWER') || s.includes('SUCCESS') || s === 'SUCCESS' || s === 'TALKING' || billsec > 0

    if (isAnswered) {
      answered++
      if (billsec > 30) {
        longCalls++
      }
      
      // Categorize durations
      if (billsec < 10) {
        durUnder10++
      } else if (billsec <= 30) {
        dur10to30++
      } else if (billsec <= 60) {
        dur30to60++
      } else {
        durOver60++
      }
    } else if (s.includes('BUSY')) {
      busy++
    } else {
      failed++
    }
  })

  const answeredRate = totalCalls > 0 ? Math.round((answered / totalCalls) * 100) : 0
  const longCallsPercent = answered > 0 ? Math.round((longCalls / answered) * 100) : 0

  // System-level FreeSWITCH concurrent channels (Only for Operator)
  const activeCalls = nodesQuery.data?.reduce((sum, item) => sum + (item.activeCalls || 0), 0) ?? 0
  const maxChannels = nodesQuery.data?.reduce((sum, item) => sum + (item.maxChannels || 0), 0) ?? 0
  const activeNodes = nodesQuery.data?.filter(n => n.status === 'active').length ?? 0

  // Merchant-level active tasks progress (Only for Merchant)
  const merchantTasks = batchTasksQuery.data?.records ?? []
  const activeTasksCount = merchantTasks.filter(t => t.status === 'running').length
  const totalTasksCount = merchantTasks.length

  // 4. Dynamic 6-Hour Line Chart Data
  const hours: string[] = []
  const callAttempts: number[] = []
  const callConnected: number[] = []
  for (let i = 5; i >= 0; i--) {
    const timeStr = dayjs().subtract(i, 'hour').format('HH:00')
    hours.push(timeStr)
    
    const attempts = records.filter((r) => {
      if (!r.finishedAt) return false
      return dayjs(r.finishedAt).format('HH:00') === timeStr
    }).length
    callAttempts.push(attempts)
    
    const connected = records.filter((r) => {
      if (!r.finishedAt) return false
      if (dayjs(r.finishedAt).format('HH:00') !== timeStr) return false
      const s = String(r.state).toUpperCase()
      const billsec = Number(r.billsec) || 0
      return s.includes('ANSWER') || s.includes('SUCCESS') || s === 'SUCCESS' || s === 'TALKING' || billsec > 0
    }).length
    callConnected.push(connected)
  }

  const lineOption = {
    tooltip: { trigger: 'axis' },
    legend: { data: ['外呼尝试', '接通成功'], bottom: 0 },
    grid: { left: '3%', right: '4%', top: '10%', bottom: '15%', containLabel: true },
    xAxis: { type: 'category', data: hours },
    yAxis: { type: 'value', minInterval: 1 },
    series: [
      {
        name: '外呼尝试',
        type: 'line',
        smooth: true,
        data: callAttempts,
        itemStyle: { color: '#6366f1' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(99, 102, 241, 0.2)' },
              { offset: 1, color: 'rgba(99, 102, 241, 0)' }
            ]
          }
        }
      },
      {
        name: '接通成功',
        type: 'line',
        smooth: true,
        data: callConnected,
        itemStyle: { color: '#10b981' },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(16, 185, 129, 0.2)' },
              { offset: 1, color: 'rgba(16, 185, 129, 0)' }
            ]
          }
        }
      }
    ]
  }

  // 5. Bar Chart: Duration Distribution Options
  const durationBarOption = {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: '3%', right: '4%', top: '10%', bottom: '10%', containLabel: true },
    xAxis: { type: 'category', data: ['< 10s', '10s-30s', '30s-60s', '> 60s'] },
    yAxis: { type: 'value', minInterval: 1 },
    series: [
      {
        type: 'bar',
        barWidth: '45%',
        data: [
          { value: durUnder10, itemStyle: { color: '#f87171' } },
          { value: dur10to30, itemStyle: { color: '#fb923c' } },
          { value: dur30to60, itemStyle: { color: '#60a5fa' } },
          { value: durOver60, itemStyle: { color: '#34d399' } }
        ]
      }
    ]
  }

  // 6. Donut Chart: Outcomes Options
  const pieOption = {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, show: true, itemWidth: 8, itemHeight: 8 },
    series: [
      {
        type: 'pie',
        radius: ['50%', '75%'],
        avoidLabelOverlap: false,
        itemStyle: { borderRadius: 4, borderColor: '#fff', borderWidth: 1 },
        label: { show: false },
        emphasis: { label: { show: true, fontSize: '12', fontWeight: 'bold' } },
        data: [
          { value: answered, name: '已接通', itemStyle: { color: '#10b981' } },
          { value: busy, name: '忙线中', itemStyle: { color: '#fb923c' } },
          { value: failed, name: '失败退费', itemStyle: { color: '#ef4444' } }
        ]
      }
    ]
  }

  // Direct trace path resolver
  const callRecordTraceRoute = isOperate ? '/operate/call-record' : '/merchant/call-record'

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 1. Curated Stats Cards with Hover and Navigation Traceability */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card
            className="shadow-soft hover:shadow-md cursor-pointer transition-all duration-300 border-none group"
            styles={{ body: { padding: '20px' } }}
            onClick={() => navigate(callRecordTraceRoute)}
          >
            <div className="flex justify-between items-start">
              <div>
                <Typography.Text type="secondary" className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                  {isOperate ? '全网今日外呼总量' : '本商户今日外呼'}
                </Typography.Text>
                <Typography.Title level={2} className="!mt-2 !mb-0 !font-bold text-slate-800 group-hover:text-indigo-600 transition-colors">
                  {totalCalls} <span className="text-sm font-normal text-slate-400">次</span>
                </Typography.Title>
              </div>
              <Avatar style={{ backgroundColor: 'rgba(99, 102, 241, 0.1)', color: '#6366f1' }} size="large" icon={<PhoneOutlined />} />
            </div>
            <div className="mt-4 flex items-center justify-between text-xs text-slate-400 border-t border-slate-100 pt-3">
              <span className="text-indigo-500 hover:underline">点击查看呼叫明细 ➜</span>
              <Tag color="blue" className="m-0 border-none text-[10px]">当日追踪</Tag>
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <Card
            className="shadow-soft hover:shadow-md cursor-pointer transition-all duration-300 border-none group"
            styles={{ body: { padding: '20px' } }}
            onClick={() => navigate(callRecordTraceRoute)}
          >
            <div className="flex justify-between items-start">
              <div>
                <Typography.Text type="secondary" className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                  {isOperate ? '全网平均接通占比' : '本商户今日接通率'}
                </Typography.Text>
                <Typography.Title level={2} className={`!mt-2 !mb-0 !font-bold ${answeredRate >= 50 ? 'text-emerald-600' : 'text-amber-500'}`}>{answeredRate}%</Typography.Title>
              </div>
              <Avatar style={{ backgroundColor: 'rgba(16, 185, 129, 0.1)', color: '#10b981' }} size="large" icon={<CheckCircleOutlined />} />
            </div>
            <div className="mt-3">
              <Progress percent={answeredRate} showInfo={false} strokeColor={{ '0%': '#34d399', '100%': '#10b981' }} />
            </div>
            <div className="mt-2 flex items-center justify-between text-xs text-slate-400 border-t border-slate-100 pt-2">
              <span>已接通: {answered} 次</span>
              <span className="text-indigo-500 hover:underline">点击溯源</span>
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <Card
            className="shadow-soft hover:shadow-md cursor-pointer transition-all duration-300 border-none group"
            styles={{ body: { padding: '20px' } }}
            onClick={() => navigate(callRecordTraceRoute)}
          >
            <div className="flex justify-between items-start">
              <div>
                <Typography.Text type="secondary" className="text-xs font-semibold uppercase tracking-wider text-slate-400">高价值通话 (&gt;30秒)</Typography.Text>
                <Typography.Title level={2} className="!mt-2 !mb-0 !font-bold text-indigo-600">{longCalls} <span className="text-sm font-normal text-slate-400">个</span></Typography.Title>
              </div>
              <Avatar style={{ backgroundColor: 'rgba(99, 102, 241, 0.1)', color: '#6366f1' }} size="large" icon={<ClockCircleOutlined />} />
            </div>
            <div className="mt-3">
              <Progress percent={longCallsPercent} showInfo={false} strokeColor={{ '0%': '#818cf8', '100%': '#6366f1' }} />
            </div>
            <div className="mt-2 flex items-center justify-between text-xs text-slate-400 border-t border-slate-100 pt-2">
              <span>占接通比: {longCallsPercent}%</span>
              <span className="text-indigo-500 hover:underline">点击溯源</span>
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          {isOperate ? (
            /* Operator Stats: Overall server concurrent channels */
            <Card
              className="shadow-soft hover:shadow-md cursor-pointer transition-all duration-300 border-none group"
              styles={{ body: { padding: '20px' } }}
              onClick={() => navigate('/operate/freeswitch')}
            >
              <div className="flex justify-between items-start">
                <div>
                  <Typography.Text type="secondary" className="text-xs font-semibold uppercase tracking-wider text-slate-400">系统全网并发通道</Typography.Text>
                  <Typography.Title level={2} className="!mt-2 !mb-0 !font-bold text-slate-800">{activeCalls} / {maxChannels}</Typography.Title>
                </div>
                <Avatar style={{ backgroundColor: 'rgba(245, 158, 11, 0.1)', color: '#f59e0b' }} size="large" icon={<DashboardOutlined />} />
              </div>
              <div className="mt-3">
                <Progress percent={maxChannels > 0 ? Math.round((activeCalls / maxChannels) * 100) : 0} showInfo={false} strokeColor={{ '0%': '#fbbf24', '100%': '#f59e0b' }} />
              </div>
              <div className="mt-2 flex items-center justify-between text-xs text-slate-400 border-t border-slate-100 pt-2">
                <span>负载率: {maxChannels > 0 ? Math.round((activeCalls / maxChannels) * 100) : 0}%</span>
                <span className="text-indigo-500 hover:underline">活动节点: {activeNodes}</span>
              </div>
            </Card>
          ) : (
            /* Merchant Stats: Scoped outbound campaign tasks */
            <Card
              className="shadow-soft hover:shadow-md cursor-pointer transition-all duration-300 border-none group"
              styles={{ body: { padding: '20px' } }}
              onClick={() => navigate('/merchant/batch-call-task')}
            >
              <div className="flex justify-between items-start">
                <div>
                  <Typography.Text type="secondary" className="text-xs font-semibold uppercase tracking-wider text-slate-400">本商户活跃外呼任务</Typography.Text>
                  <Typography.Title level={2} className="!mt-2 !mb-0 !font-bold text-slate-800">{activeTasksCount} / {totalTasksCount}</Typography.Title>
                </div>
                <Avatar style={{ backgroundColor: 'rgba(245, 158, 11, 0.1)', color: '#f59e0b' }} size="large" icon={<CarryOutOutlined />} />
              </div>
              <div className="mt-3">
                <Progress percent={totalTasksCount > 0 ? Math.round((activeTasksCount / totalTasksCount) * 100) : 0} showInfo={false} strokeColor={{ '0%': '#fbbf24', '100%': '#f59e0b' }} />
              </div>
              <div className="mt-2 flex items-center justify-between text-xs text-slate-400 border-t border-slate-100 pt-2">
                <span>并发任务占比: {totalTasksCount > 0 ? Math.round((activeTasksCount / totalTasksCount) * 100) : 0}%</span>
                <span className="text-indigo-500 hover:underline">点击查看任务 ➜</span>
              </div>
            </Card>
          )}
        </Col>
      </Row>

      {/* 2. Scoped Visual NOC Charts Grid */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12} xl={14}>
          <ChartWrap title={isOperate ? '全网系统今日呼叫趋势' : '本商户今日呼叫趋势 (6小时)'} option={lineOption} />
        </Col>
        <Col xs={24} md={12} lg={6} xl={5}>
          <ChartWrap title="接通通话时长分布" option={durationBarOption} />
        </Col>
        <Col xs={24} md={12} lg={6} xl={5}>
          <ChartWrap title="外呼结果状态分布" option={pieOption} />
        </Col>
      </Row>

      {/* 3. NOC Flow timeline and Scoped Resource Progress Meters */}
      <Row gutter={[16, 16]}>
        {/* Timeline Dynamic Log */}
        <Col xs={24} lg={14}>
          <Card title={isOperate ? '全网实时外呼流水监控 (逻辑溯源)' : '本商户最新外呼水流监控'} className="shadow-soft" extra={<span className="text-xs text-slate-400">自动滚动</span>}>
            {records.length > 0 ? (
              <Timeline
                items={records.slice(0, 4).map((record) => {
                  const billsec = Number(record.billsec) || 0
                  const s = String(record.state).toUpperCase()
                  const isAnswered = s.includes('ANSWER') || s === 'SUCCESS' || s === 'TALKING' || billsec > 0
                  return {
                    color: isAnswered ? 'green' : s.includes('BUSY') ? 'orange' : 'red',
                    children: (
                      <div className="flex flex-col gap-1" style={{ fontSize: '12px' }}>
                        <div className="flex justify-between items-center">
                          <span className="font-semibold text-slate-700">主叫: {record.caller || '系统'} ➜ 被叫: {record.callee}</span>
                          <span className="text-slate-400">{record.finishedAt ? dayjs(record.finishedAt).format('HH:mm:ss') : '刚刚'}</span>
                        </div>
                        <div className="text-xs text-slate-400 flex items-center gap-2">
                          {isOperate && (
                            <>
                              <span className="text-indigo-600 font-medium">{record.merchant}</span>
                              <span>|</span>
                            </>
                          )}
                          <span>物理节点: <strong className="text-slate-500">{record.fsAddr}</strong></span>
                          <span>|</span>
                          <span>状态: 
                            <Tag color={isAnswered ? 'success' : s.includes('BUSY') ? 'warning' : 'error'} className="ml-1 border-none text-[10px] py-0 px-1 leading-normal">
                              {record.state}
                            </Tag>
                          </span>
                          <span>|</span>
                          <span>通话时长: <strong className={billsec > 30 ? 'text-indigo-600 font-bold' : 'text-slate-500'}>{billsec}秒</strong></span>
                        </div>
                      </div>
                    )
                  }
                }) ?? []}
              />
            ) : (
              <Timeline items={[{ children: '今日暂无外呼流水动态。' }]} />
            )}
          </Card>
        </Col>

        {/* Dynamic Resource boards: Differentiated strictly by platform */}
        <Col xs={24} lg={10}>
          {isOperate ? (
            /* Operator Resource Dashboard */
            <Card title="系统全网物理架构负载" className="shadow-soft">
              <Space direction="vertical" className="w-full" size="middle">
                <div className="cursor-pointer hover:bg-slate-50 p-2 rounded transition-colors" onClick={() => navigate('/operate/gateway')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-medium text-slate-600"><ApartmentOutlined className="mr-2" />全网物理线路网关</span>
                    <Tag color="blue">{gatewaysQuery.data?.records.length ?? 0} 个</Tag>
                  </div>
                  <Progress percent={Math.min(100, (gatewaysQuery.data?.records.length ?? 0) * 10)} strokeColor="#3b82f6" />
                </div>
                <div className="cursor-pointer hover:bg-slate-50 p-2 rounded transition-colors" onClick={() => navigate('/merchant/ai-model-flow')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-medium text-slate-600"><ThunderboltOutlined className="mr-2" />全网已部署 AI 流程</span>
                    <Tag color="gold">{aiFlowsQuery.data?.records.length ?? 0} 个</Tag>
                  </div>
                  <Progress percent={Math.min(100, (aiFlowsQuery.data?.records.length ?? 0) * 10)} strokeColor="#f59e0b" />
                </div>
                <div className="cursor-pointer hover:bg-slate-50 p-2 rounded transition-colors" onClick={() => navigate('/operate/freeswitch')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-medium text-slate-600"><DatabaseOutlined className="mr-2" />全网活动软交换节点</span>
                    <Tag color="green">{activeNodes} / {nodesQuery.data?.length ?? 0}</Tag>
                  </div>
                  <Progress percent={nodesQuery.data?.length ? Math.round((activeNodes / nodesQuery.data.length) * 100) : 0} strokeColor="#10b981" />
                </div>
              </Space>
            </Card>
          ) : (
            /* Merchant Resource Dashboard */
            <Card title="本商户应用业务规模监控" className="shadow-soft">
              <Space direction="vertical" className="w-full" size="middle">
                <div className="cursor-pointer hover:bg-slate-50 p-2 rounded transition-colors" onClick={() => navigate('/merchant/batch-call-task')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-medium text-slate-600"><CarryOutOutlined className="mr-2" />本商户外呼任务总规模</span>
                    <Tag color="blue">{totalTasksCount} 个</Tag>
                  </div>
                  <Progress percent={Math.min(100, totalTasksCount * 10)} strokeColor="#3b82f6" />
                </div>
                <div className="cursor-pointer hover:bg-slate-50 p-2 rounded transition-colors" onClick={() => navigate('/merchant/ai-model-flow')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-medium text-slate-600"><ThunderboltOutlined className="mr-2" />本商户绑定 AI 模型流</span>
                    <Tag color="gold">{aiFlowsQuery.data?.records.length ?? 0} 个</Tag>
                  </div>
                  <Progress percent={Math.min(100, (aiFlowsQuery.data?.records.length ?? 0) * 10)} strokeColor="#f59e0b" />
                </div>
                <div className="cursor-pointer hover:bg-slate-50 p-2 rounded transition-colors" onClick={() => navigate('/merchant/skill-group')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm font-medium text-slate-600"><AuditOutlined className="mr-2" />已绑定话务坐席技能组</span>
                    <Tag color="green">活动中</Tag>
                  </div>
                  <Progress percent={100} strokeColor="#10b981" />
                </div>
              </Space>
            </Card>
          )}
        </Col>
      </Row>
    </Space>
  )
}
