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

  // Operator gateway utilization
  const gatewayRecords = gatewaysQuery.data?.records ?? []
  const enabledGateways = gatewayRecords.filter(r => r.enable).length
  const totalGateways = gatewayRecords.length

  // AI flow publish ratio
  const aiFlowRecords = aiFlowsQuery.data?.records ?? []
  const publishedFlows = aiFlowRecords.filter(r => r.status === 'published').length
  const totalFlows = aiFlowRecords.length

  // Merchant-level active tasks progress (Only for Merchant)
  const merchantTasks = batchTasksQuery.data?.records ?? []
  const activeTasksCount = merchantTasks.filter(t => t.status === 'running').length
  const totalTasksCount = merchantTasks.length

  // 4. Dynamic 6-Hour Line Chart Data
  const hours: string[] = []
  const callAttempts: number[] = []
  const callConnected: number[] = []
  for (let i = 5; i >= 0; i--) {
    const bucketEnd = dayjs().subtract(i, 'hour').startOf('hour').add(1, 'hour')
    const bucketStart = dayjs().subtract(i, 'hour').startOf('hour')
    const timeStr = bucketStart.format('HH:00')
    hours.push(timeStr)
    
    const attempts = records.filter((r) => {
      if (!r.finishedAt) return false
      const t = dayjs(r.finishedAt)
      return t.isAfter(bucketStart) && t.isBefore(bucketEnd)
    }).length
    callAttempts.push(attempts)
    
    const connected = records.filter((r) => {
      if (!r.finishedAt) return false
      const t = dayjs(r.finishedAt)
      if (!t.isAfter(bucketStart) || !t.isBefore(bucketEnd)) return false
      const s = String(r.state).toUpperCase()
      const billsec = Number(r.billsec) || 0
      return s.includes('ANSWER') || s.includes('SUCCESS') || s === 'SUCCESS' || s === 'TALKING' || billsec > 0
    }).length
    callConnected.push(connected)
  }

  const lineOption = {
    backgroundColor: 'transparent',
    tooltip: { 
      trigger: 'axis',
      backgroundColor: 'rgba(15, 23, 42, 0.95)',
      borderColor: 'rgba(51, 65, 85, 0.7)',
      borderWidth: 1,
      textStyle: { color: '#f8fafc', fontSize: 11 },
      padding: [10, 14],
      borderRadius: 8,
      shadowColor: 'rgba(0, 0, 0, 0.25)',
      shadowBlur: 8
    },
    legend: { 
      data: ['外呼尝试', '接通成功'], 
      bottom: 0,
      textStyle: { color: '#64748b', fontSize: 11 },
      icon: 'circle'
    },
    grid: { left: '3%', right: '4%', top: '10%', bottom: '15%', containLabel: true },
    xAxis: { 
      type: 'category', 
      data: hours,
      axisLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.15)' } },
      axisLabel: { color: '#64748b', fontSize: 10 }
    },
    yAxis: { 
      type: 'value', 
      minInterval: 1,
      splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)', type: 'dashed' } },
      axisLabel: { color: '#64748b', fontSize: 10 }
    },
    series: [
      {
        name: '外呼尝试',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: callAttempts,
        itemStyle: { color: '#6366f1' },
        lineStyle: { width: 3 },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(99, 102, 241, 0.25)' },
              { offset: 1, color: 'rgba(99, 102, 241, 0)' }
            ]
          }
        }
      },
      {
        name: '接通成功',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: callConnected,
        itemStyle: { color: '#10b981' },
        lineStyle: { width: 3 },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(16, 185, 129, 0.25)' },
              { offset: 1, color: 'rgba(16, 185, 129, 0)' }
            ]
          }
        }
      }
    ]
  }

  const durationBarOption = {
    backgroundColor: 'transparent',
    tooltip: { 
      trigger: 'axis', 
      axisPointer: { type: 'shadow' },
      backgroundColor: 'rgba(15, 23, 42, 0.95)',
      borderColor: 'rgba(51, 65, 85, 0.7)',
      borderWidth: 1,
      textStyle: { color: '#f8fafc', fontSize: 11 },
      borderRadius: 8
    },
    grid: { left: '3%', right: '4%', top: '10%', bottom: '10%', containLabel: true },
    xAxis: { 
      type: 'category', 
      data: ['< 10s', '10s-35s', '35s-60s', '> 60s'],
      axisLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.15)' } },
      axisLabel: { color: '#64748b', fontSize: 10 }
    },
    yAxis: { 
      type: 'value', 
      minInterval: 1,
      splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)', type: 'dashed' } },
      axisLabel: { color: '#64748b', fontSize: 10 }
    },
    series: [
      {
        type: 'bar',
        barWidth: '45%',
        data: [
          { value: durUnder10, itemStyle: { color: 'rgba(239, 68, 68, 0.8)', borderRadius: [4, 4, 0, 0] } },
          { value: dur10to30, itemStyle: { color: 'rgba(249, 115, 22, 0.8)', borderRadius: [4, 4, 0, 0] } },
          { value: dur30to60, itemStyle: { color: 'rgba(59, 130, 246, 0.8)', borderRadius: [4, 4, 0, 0] } },
          { value: durOver60, itemStyle: { color: 'rgba(16, 185, 129, 0.8)', borderRadius: [4, 4, 0, 0] } }
        ]
      }
    ]
  }

  const pieOption = {
    backgroundColor: 'transparent',
    tooltip: { 
      trigger: 'item',
      backgroundColor: 'rgba(15, 23, 42, 0.95)',
      borderColor: 'rgba(51, 65, 85, 0.7)',
      borderWidth: 1,
      textStyle: { color: '#f8fafc', fontSize: 11 },
      borderRadius: 8
    },
    legend: { 
      bottom: 0, 
      show: true, 
      itemWidth: 8, 
      itemHeight: 8,
      textStyle: { color: '#64748b', fontSize: 10 },
      icon: 'circle'
    },
    series: [
      {
        type: 'pie',
        radius: ['52%', '75%'],
        center: ['50%', '42%'],
        avoidLabelOverlap: false,
        itemStyle: { borderRadius: 6, borderColor: 'rgba(255,255,255,0.05)', borderWidth: 1.5 },
        label: { show: false },
        emphasis: { label: { show: false } },
        data: [
          { value: answered, name: '已接通', itemStyle: { color: '#10b981' } },
          { value: busy, name: '忙线中', itemStyle: { color: '#fb923c' } },
          { value: failed, name: '失败挂断', itemStyle: { color: '#ef4444' } }
        ]
      }
    ]
  }

  const callRecordTraceRoute = isOperate ? '/operate/call-record' : '/merchant/call-record'

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 1. Curated Stats Cards with Hover and Top Border Accent Lines */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card
            className="hover:-translate-y-1 cursor-pointer transition-all duration-300 bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/70 rounded-2xl overflow-hidden group shadow-soft hover:shadow-xl hover:shadow-indigo-500/5 dark:hover:shadow-blue-950/10 relative"
            styles={{ body: { padding: '24px 20px 20px' } }}
            onClick={() => navigate(callRecordTraceRoute)}
          >
            <div className="absolute top-0 left-0 right-0 h-[3px] bg-gradient-to-r from-blue-500 to-indigo-500"></div>
            <div className="flex justify-between items-start">
              <div>
                <Typography.Text className="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">
                  {isOperate ? '全网今日外呼总量' : '本商户今日外呼'}
                </Typography.Text>
                <Typography.Title level={2} className="!mt-2.5 !mb-0 !font-extrabold text-slate-800 dark:text-white group-hover:text-indigo-600 transition-colors">
                  {totalCalls} <span className="text-xs font-normal text-slate-400 dark:text-slate-500">次</span>
                </Typography.Title>
              </div>
              <div className="flex items-center justify-center w-11 h-11 rounded-xl bg-indigo-50 dark:bg-indigo-950/40 text-indigo-600 dark:text-indigo-400 shadow-sm border border-indigo-100/50 dark:border-indigo-900/30">
                <PhoneOutlined className="text-lg" />
              </div>
            </div>
            <div className="mt-5 flex items-center justify-between text-xs text-slate-400 dark:text-slate-500 border-t border-slate-100 dark:border-slate-800/80 pt-3">
              <span className="text-indigo-500 hover:text-indigo-600 font-medium transition-colors">查看呼叫明细 ➜</span>
              <Tag color="blue" className="m-0 border-none rounded-full px-2.5 text-[10px] bg-blue-500/10 text-blue-500">当日追踪</Tag>
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <Card
            className="hover:-translate-y-1 cursor-pointer transition-all duration-300 bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/70 rounded-2xl overflow-hidden group shadow-soft hover:shadow-xl hover:shadow-emerald-500/5 dark:hover:shadow-emerald-950/10 relative"
            styles={{ body: { padding: '24px 20px 20px' } }}
            onClick={() => navigate(callRecordTraceRoute)}
          >
            <div className="absolute top-0 left-0 right-0 h-[3px] bg-gradient-to-r from-emerald-400 to-teal-500"></div>
            <div className="flex justify-between items-start">
              <div>
                <Typography.Text className="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">
                  {isOperate ? '全网平均接通占比' : '本商户今日接通率'}
                </Typography.Text>
                <Typography.Title level={2} className={`!mt-2.5 !mb-0 !font-extrabold ${answeredRate >= 50 ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-500'}`}>{answeredRate}%</Typography.Title>
              </div>
              <div className="flex items-center justify-center w-11 h-11 rounded-xl bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 shadow-sm border border-emerald-100/50 dark:border-emerald-900/30">
                <CheckCircleOutlined className="text-lg" />
              </div>
            </div>
            <div className="mt-4">
              <Progress percent={answeredRate} showInfo={false} strokeColor={{ '0%': '#34d399', '100%': '#10b981' }} size="small" />
            </div>
            <div className="mt-2.5 flex items-center justify-between text-xs text-slate-400 dark:text-slate-500 border-t border-slate-100 dark:border-slate-800/80 pt-2">
              <span>已接通: {answered} 次</span>
              <span className="text-emerald-500 hover:text-emerald-600 font-medium">点击溯源</span>
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          <Card
            className="hover:-translate-y-1 cursor-pointer transition-all duration-300 bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/70 rounded-2xl overflow-hidden group shadow-soft hover:shadow-xl hover:shadow-purple-500/5 dark:hover:shadow-purple-950/10 relative"
            styles={{ body: { padding: '24px 20px 20px' } }}
            onClick={() => navigate(callRecordTraceRoute)}
          >
            <div className="absolute top-0 left-0 right-0 h-[3px] bg-gradient-to-r from-indigo-500 to-purple-500"></div>
            <div className="flex justify-between items-start">
              <div>
                <Typography.Text className="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">高价值通话 (&gt;30秒)</Typography.Text>
                <Typography.Title level={2} className="!mt-2.5 !mb-0 !font-extrabold text-indigo-600 dark:text-indigo-400">{longCalls} <span className="text-xs font-normal text-slate-400 dark:text-slate-500">个</span></Typography.Title>
              </div>
              <div className="flex items-center justify-center w-11 h-11 rounded-xl bg-purple-50 dark:bg-purple-950/40 text-purple-600 dark:text-purple-400 shadow-sm border border-purple-100/50 dark:border-purple-900/30">
                <ClockCircleOutlined className="text-lg" />
              </div>
            </div>
            <div className="mt-4">
              <Progress percent={longCallsPercent} showInfo={false} strokeColor={{ '0%': '#818cf8', '100%': '#6366f1' }} size="small" />
            </div>
            <div className="mt-2.5 flex items-center justify-between text-xs text-slate-400 dark:text-slate-500 border-t border-slate-100 dark:border-slate-800/80 pt-2">
              <span>占比: {longCallsPercent}%</span>
              <span className="text-indigo-500 hover:text-indigo-600 font-medium">点击溯源</span>
            </div>
          </Card>
        </Col>

        <Col xs={24} sm={12} lg={6}>
          {isOperate ? (
            <Card
              className="hover:-translate-y-1 cursor-pointer transition-all duration-300 bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/70 rounded-2xl overflow-hidden group shadow-soft hover:shadow-xl hover:shadow-amber-500/5 dark:hover:shadow-amber-950/10 relative"
              styles={{ body: { padding: '24px 20px 20px' } }}
              onClick={() => navigate('/operate/freeswitch')}
            >
              <div className="absolute top-0 left-0 right-0 h-[3px] bg-gradient-to-r from-amber-400 to-orange-500"></div>
              <div className="flex justify-between items-start">
                <div>
                  <Typography.Text className="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">系统全网并发通道</Typography.Text>
                  <Typography.Title level={2} className="!mt-2.5 !mb-0 !font-extrabold text-slate-850 dark:text-white">{activeCalls} / {maxChannels}</Typography.Title>
                </div>
                <div className="flex items-center justify-center w-11 h-11 rounded-xl bg-amber-50 dark:bg-amber-950/40 text-amber-600 dark:text-amber-400 shadow-sm border border-amber-100/50 dark:border-amber-900/30">
                  <DashboardOutlined className="text-lg" />
                </div>
              </div>
              <div className="mt-4">
                <Progress percent={maxChannels > 0 ? Math.round((activeCalls / maxChannels) * 100) : 0} showInfo={false} strokeColor={{ '0%': '#fbbf24', '100%': '#f59e0b' }} size="small" />
              </div>
              <div className="mt-2.5 flex items-center justify-between text-xs text-slate-400 dark:text-slate-500 border-t border-slate-100 dark:border-slate-800/80 pt-2">
                <span>并发占比: {maxChannels > 0 ? Math.round((activeCalls / maxChannels) * 100) : 0}%</span>
                <span className="text-amber-500 hover:text-amber-600 font-medium">活动节点: {activeNodes}</span>
              </div>
            </Card>
          ) : (
            <Card
              className="hover:-translate-y-1 cursor-pointer transition-all duration-300 bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/70 rounded-2xl overflow-hidden group shadow-soft hover:shadow-xl hover:shadow-amber-500/5 dark:hover:shadow-amber-950/10 relative"
              styles={{ body: { padding: '24px 20px 20px' } }}
              onClick={() => navigate('/merchant/batch-call-task')}
            >
              <div className="absolute top-0 left-0 right-0 h-[3px] bg-gradient-to-r from-amber-400 to-orange-500"></div>
              <div className="flex justify-between items-start">
                <div>
                  <Typography.Text className="text-[11px] font-bold uppercase tracking-wider text-slate-400 dark:text-slate-500">本商户活跃外呼任务</Typography.Text>
                  <Typography.Title level={2} className="!mt-2.5 !mb-0 !font-extrabold text-slate-850 dark:text-white">{activeTasksCount} / {totalTasksCount}</Typography.Title>
                </div>
                <div className="flex items-center justify-center w-11 h-11 rounded-xl bg-amber-50 dark:bg-amber-950/40 text-amber-600 dark:text-amber-400 shadow-sm border border-amber-100/50 dark:border-amber-900/30">
                  <CarryOutOutlined className="text-lg" />
                </div>
              </div>
              <div className="mt-4">
                <Progress percent={totalTasksCount > 0 ? Math.round((activeTasksCount / totalTasksCount) * 100) : 0} showInfo={false} strokeColor={{ '0%': '#fbbf24', '100%': '#f59e0b' }} size="small" />
              </div>
              <div className="mt-2.5 flex items-center justify-between text-xs text-slate-400 dark:text-slate-500 border-t border-slate-100 dark:border-slate-800/80 pt-2">
                <span>活跃比: {totalTasksCount > 0 ? Math.round((activeTasksCount / totalTasksCount) * 100) : 0}%</span>
                <span className="text-amber-500 hover:text-amber-600 font-medium">查看任务 ➜</span>
              </div>
            </Card>
          )}
        </Col>
      </Row>

      {/* 2. NOC Charts Grid */}
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
        <Col xs={24} lg={14}>
          <Card 
            title={
              <div className="flex items-center gap-2 py-1">
                <span className="w-[3px] h-3.5 rounded-full bg-gradient-to-b from-blue-500 to-indigo-500"></span>
                <span className="text-[13px] font-bold text-slate-800 dark:text-slate-100">
                  {isOperate ? '全网实时外呼流水监控 (逻辑溯源)' : '本商户最新外呼水流监控'}
                </span>
              </div>
            }
            className="bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/70 rounded-2xl shadow-soft"
            styles={{ body: { padding: '20px 20px 8px' } }}
            extra={<span className="text-[10px] text-slate-400 font-medium bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-full">自动滚动</span>}
          >
            {records.length > 0 ? (
              <Timeline
                className="mt-2 text-slate-600 dark:text-slate-350"
                items={records.slice(0, 4).map((record) => {
                  const billsec = Number(record.billsec) || 0
                  const s = String(record.state).toUpperCase()
                  const isAnswered = s.includes('ANSWER') || s === 'SUCCESS' || s === 'TALKING' || billsec > 0
                  return {
                    color: isAnswered ? 'green' : s.includes('BUSY') ? 'orange' : 'red',
                    children: (
                      <div className="flex flex-col gap-1.5 pb-2.5 text-xs">
                        <div className="flex justify-between items-center">
                          <span className="font-semibold text-slate-750 dark:text-slate-200">
                            主叫: <span className="font-mono text-slate-900 dark:text-white bg-slate-50 dark:bg-slate-800 px-1 py-0.5 rounded">{record.caller || '系统'}</span> ➜ 被叫: <span className="font-mono text-slate-900 dark:text-white bg-slate-50 dark:bg-slate-800 px-1 py-0.5 rounded">{record.callee}</span>
                          </span>
                          <span className="text-[10px] text-slate-400 dark:text-slate-500 font-medium">{record.finishedAt ? dayjs(record.finishedAt).format('HH:mm:ss') : '刚刚'}</span>
                        </div>
                        <div className="text-[11px] text-slate-400 dark:text-slate-500 flex flex-wrap items-center gap-x-3 gap-y-1">
                          {isOperate && (
                            <>
                              <span className="text-indigo-500 dark:text-indigo-400 font-bold bg-indigo-500/10 px-1.5 py-0.5 rounded-md">{record.merchant}</span>
                              <span className="text-slate-200 dark:text-slate-800">|</span>
                            </>
                          )}
                          <span>物理节点: <strong className="text-slate-600 dark:text-slate-300 font-mono font-normal">{record.fsAddr}</strong></span>
                          <span className="text-slate-200 dark:text-slate-800">|</span>
                          <span className="flex items-center gap-1">状态: 
                            <Tag color={isAnswered ? 'success' : s.includes('BUSY') ? 'warning' : 'error'} className="ml-1 border-none text-[9px] font-bold py-0 px-1.5 leading-normal rounded-full">
                              {record.state}
                            </Tag>
                          </span>
                          <span className="text-slate-200 dark:text-slate-800">|</span>
                          <span>通话时长: <strong className={billsec > 30 ? 'text-indigo-500 dark:text-indigo-400 font-bold bg-indigo-500/5 px-1 py-0.5 rounded' : 'text-slate-500 dark:text-slate-400'}>{billsec}秒</strong></span>
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

        <Col xs={24} lg={10}>
          {isOperate ? (
            <Card 
              title={
                <div className="flex items-center gap-2 py-1">
                  <span className="w-[3px] h-3.5 rounded-full bg-gradient-to-b from-blue-500 to-indigo-500"></span>
                  <span className="text-[13px] font-bold text-slate-800 dark:text-slate-100">系统全网物理架构负载</span>
                </div>
              }
              className="bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/70 rounded-2xl shadow-soft"
              styles={{ body: { padding: '16px 20px 20px' } }}
            >
              <Space direction="vertical" className="w-full" size="middle">
                <div className="cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/40 p-3 rounded-xl transition-all border border-transparent hover:border-slate-100 dark:hover:border-slate-850/50" onClick={() => navigate('/operate/gateway')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-[12px] font-bold text-slate-650 dark:text-slate-300 flex items-center gap-1.5"><ApartmentOutlined className="text-slate-450" />全网物理线路网关</span>
                    <Tag color="blue" className="border-none bg-blue-500/10 text-blue-500 rounded-full px-2">{gatewaysQuery.data?.records.length ?? 0} 个</Tag>
                  </div>
                  <Progress percent={totalGateways > 0 ? Math.round((enabledGateways / totalGateways) * 100) : 0} strokeColor="#3b82f6" size={['100%', 5]} showInfo={false} />
                </div>
                <div className="cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/40 p-3 rounded-xl transition-all border border-transparent hover:border-slate-100 dark:hover:border-slate-850/50" onClick={() => navigate('/merchant/ai-model-flow')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-[12px] font-bold text-slate-650 dark:text-slate-300 flex items-center gap-1.5"><ThunderboltOutlined className="text-slate-450" />全网已部署 AI 流程</span>
                    <Tag color="gold" className="border-none bg-amber-500/10 text-amber-500 rounded-full px-2">{aiFlowsQuery.data?.records.length ?? 0} 个</Tag>
                  </div>
                  <Progress percent={totalFlows > 0 ? Math.round((publishedFlows / totalFlows) * 100) : 0} strokeColor="#f59e0b" size={['100%', 5]} showInfo={false} />
                </div>
                <div className="cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/40 p-3 rounded-xl transition-all border border-transparent hover:border-slate-100 dark:hover:border-slate-850/50" onClick={() => navigate('/operate/freeswitch')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-[12px] font-bold text-slate-650 dark:text-slate-300 flex items-center gap-1.5"><DatabaseOutlined className="text-slate-450" />全网活动软交换节点</span>
                    <Tag color="green" className="border-none bg-emerald-500/10 text-emerald-500 rounded-full px-2">{activeNodes} / {nodesQuery.data?.length ?? 0}</Tag>
                  </div>
                  <Progress percent={nodesQuery.data?.length ? Math.round((activeNodes / nodesQuery.data.length) * 100) : 0} strokeColor="#10b981" size={['100%', 5]} showInfo={false} />
                </div>
              </Space>
            </Card>
          ) : (
            <Card 
              title={
                <div className="flex items-center gap-2 py-1">
                  <span className="w-[3px] h-3.5 rounded-full bg-gradient-to-b from-blue-500 to-indigo-500"></span>
                  <span className="text-[13px] font-bold text-slate-800 dark:text-slate-100">本商户应用业务规模监控</span>
                </div>
              }
              className="bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/70 rounded-2xl shadow-soft"
              styles={{ body: { padding: '16px 20px 20px' } }}
            >
              <Space direction="vertical" className="w-full" size="middle">
                <div className="cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/40 p-3 rounded-xl transition-all border border-transparent hover:border-slate-100 dark:hover:border-slate-850/50" onClick={() => navigate('/merchant/batch-call-task')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-[12px] font-bold text-slate-650 dark:text-slate-300 flex items-center gap-1.5"><CarryOutOutlined className="text-slate-450" />本商户外呼任务总规模</span>
                    <Tag color="blue" className="border-none bg-blue-500/10 text-blue-500 rounded-full px-2">{totalTasksCount} 个</Tag>
                  </div>
                  <Progress percent={totalTasksCount > 0 ? Math.round((activeTasksCount / totalTasksCount) * 100) : 0} strokeColor="#3b82f6" size={['100%', 5]} showInfo={false} />
                </div>
                <div className="cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/40 p-3 rounded-xl transition-all border border-transparent hover:border-slate-100 dark:hover:border-slate-850/50" onClick={() => navigate('/merchant/ai-model-flow')}>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-[12px] font-bold text-slate-650 dark:text-slate-300 flex items-center gap-1.5"><ThunderboltOutlined className="text-slate-450" />本商户绑定 AI 模型流</span>
                    <Tag color="gold" className="border-none bg-amber-500/10 text-amber-500 rounded-full px-2">{aiFlowsQuery.data?.records.length ?? 0} 个</Tag>
                  </div>
                  <Progress percent={totalFlows > 0 ? Math.round((publishedFlows / totalFlows) * 100) : 0} strokeColor="#f59e0b" size={['100%', 5]} showInfo={false} />
                </div>
                <div className="cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/40 p-3 rounded-xl transition-all border border-transparent hover:border-slate-100 dark:hover:border-slate-850/50 flex items-center justify-between" onClick={() => navigate('/merchant/skill-group')}>
                  <span className="text-[12px] font-bold text-slate-650 dark:text-slate-300 flex items-center gap-1.5"><AuditOutlined className="text-slate-450" />话务坐席技能组</span>
                  <Tag color="blue" className="border-none bg-blue-500/10 text-blue-500 rounded-full px-2.5 py-0.5">点击查看 ➜</Tag>
                </div>
              </Space>
            </Card>
          )}
        </Col>
      </Row>
    </Space>
  )
}
