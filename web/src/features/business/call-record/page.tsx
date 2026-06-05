import { Button, Space, Tag, Typography, Card, Row, Col, Drawer, Spin, Empty, Divider, Descriptions, Alert, Tabs, Tooltip, message } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { fetchCallRecords, fetchCallSipTrace } from '@/api/operate'
import {
  SearchOutlined,
  ReloadOutlined,
  DatabaseOutlined,
  ClockCircleOutlined,
  CustomerServiceOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  PieChartOutlined,
  CopyOutlined
} from '@ant-design/icons'
import dayjs from 'dayjs'
import type { SipTraceItem } from '@/types'

// 渠道类型映射中文
const profileMap: Record<string, { label: string; color: string }> = {
  api_outbound: { label: 'API 外呼', color: 'blue' },
  batch_outbound: { label: '批量外呼', color: 'purple' },
  api_direct: { label: '拨号盘直呼', color: 'cyan' },
  inbound: { label: '客户呼入', color: 'green' },
}

// 通话挂断原因与状态中文映射字典
const hangupCauseMap: Record<string, { label: string; color: string }> = {
  // 正常通话结束与接通状态
  NORMAL_CLEARING: { label: '正常挂断', color: 'success' },
  NORMAL_UNSPECIFIED: { label: '正常挂断', color: 'success' },
  complete: { label: '已接通', color: 'success' },
  completed: { label: '已接通', color: 'success' },
  bridged: { label: '已桥接', color: 'success' },

  // 被叫及线路异常状态
  USER_BUSY: { label: '被叫忙线', color: 'warning' },
  NO_USER_RESPONSE: { label: '被叫无响应', color: 'default' },
  NO_ANSWER: { label: '无应答', color: 'default' },
  ALLOTTED_TIMEOUT: { label: '呼叫超时', color: 'default' },
  CALL_REJECTED: { label: '被叫拒接', color: 'error' },
  LOCHUP_REJECTED: { label: '线路拒接', color: 'error' },
  ORIGINATOR_CANCEL: { label: '主叫取消', color: 'processing' },
  NO_ROUTE_DESTINATION: { label: '空号/无路由', color: 'error' },
  SUBSCRIBER_ABSENT: { label: '关机/不在服务区', color: 'warning' },
  RECOVERY_ON_TIMER_EXPIRE: { label: '超时释放', color: 'default' },

  // 风控与选号限制状态
  BLACKLIST_BLOCKED: { label: '黑名单拦截', color: 'error' },
  ATTRIB_BLOCKED: { label: '盲区拦截', color: 'error' },
  RISK_BLOCKED: { label: '风控拦截', color: 'error' },
  CONCURRENCY_EXHAUSTED: { label: '并发超限', color: 'warning' },
  NO_AVAILABLE_CALLER: { label: '无可用主叫', color: 'error' },
  NO_AVAILABLE_NUMBER: { label: '选号失败', color: 'error' },

  // 物理网关及服务注册故障
  SERVICE_UNAVAILABLE: { label: '服务不可用', color: 'error' },
  DESTINATION_OUT_OF_ORDER: { label: '终端故障', color: 'error' },
  NETWORK_OUT_OF_ORDER: { label: '网络故障', color: 'error' },
  SYSTEM_SHUTDOWN: { label: '系统关闭', color: 'error' },
  USER_NOT_REGISTERED: { label: '用户未注册', color: 'warning' },

  // 商户账号及分机异常阻断
  merchant_suspended: { label: '商户欠费停机', color: 'error' },
  extension_busy: { label: '分机正忙', color: 'warning' },
  extension_offline: { label: '分机未注册', color: 'warning' },
  license_concurrency_limit: { label: '系统并发超限', color: 'error' },
  unknown: { label: '未知状态', color: 'default' },
}

// 根据通话详情、渠道以及 SIP 挂断方向计算最终的挂断方
function getHangupParty(record: any): { label: string; color: string } {
  const isAnswered = record.billsec > 0
  if (!isAnswered) {
    if (record.hangupCause === 'ORIGINATOR_CANCEL') {
      return { label: '主叫取消', color: 'blue' }
    }
    return { label: '系统/未接通', color: 'default' }
  }

  const disp = record.sipHangupDisposition
  if (!disp) {
    return { label: '系统/未知', color: 'default' }
  }

  // 是否为客户第一（主被叫逻辑反转）的渠道：批量外呼系列、呼入
  const isCustomerFirst = ['batch_outbound', 'batch_predictive', 'batch_synergy', 'inbound'].includes(record.profile)

  if (disp === 'recv_bye' || disp === 'recv_cancel') {
    return isCustomerFirst 
      ? { label: '客户挂断', color: 'orange' } 
      : { label: '坐席挂断', color: 'purple' }
  } else if (disp === 'send_bye' || disp === 'send_cancel') {
    return isCustomerFirst 
      ? { label: '坐席挂断', color: 'purple' } 
      : { label: '客户挂断', color: 'orange' }
  }

  return { label: `其它(${disp})`, color: 'default' }
}

// 录音试听播放组件
function CallAudioPlayer({ filePath, callId }: { filePath?: string; callId: string }) {
  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [audio, setAudio] = useState<HTMLAudioElement | null>(null)

  if (!filePath) {
    return <Typography.Text type="secondary" className="text-xs">无录音</Typography.Text>
  }

  const formatTime = (secs: number) => {
    if (isNaN(secs)) return '00:00'
    const m = Math.floor(secs / 60)
    const s = Math.floor(secs % 60)
    return `${m < 10 ? '0' : ''}${m}:${s < 10 ? '0' : ''}${s}`
  }

  const togglePlay = () => {
    if (playing) {
      audio?.pause()
      setPlaying(false)
    } else {
      const url = filePath.startsWith('http') ? filePath : `http://localhost:8080/records${filePath}`
      let activeAudio = audio
      if (!activeAudio) {
        const el = new Audio(url)
        el.addEventListener('timeupdate', () => {
          setCurrentTime(el.currentTime)
        })
        el.addEventListener('loadedmetadata', () => {
          setDuration(el.duration)
        })
        el.addEventListener('ended', () => {
          setPlaying(false)
          setCurrentTime(0)
        })
        el.addEventListener('error', () => {
          setPlaying(false)
          console.warn(`录音物理文件尚不可用 (ID: ${callId})，路径: ${filePath}`)
        })
        activeAudio = el
        setAudio(el)
      }
      activeAudio.play().catch((err) => {
        console.warn('播放失败，可能是浏览器策略或录音文件暂未就绪:', err)
      })
      setPlaying(true)
    }
  }

  return (
    <Space size={8} align="center" className="min-w-[130px]">
      <Button
        type={playing ? 'primary' : 'default'}
        size="small"
        shape="circle"
        icon={playing ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
        onClick={togglePlay}
        className="flex items-center justify-center"
      />
      {playing ? (
        <div className="flex flex-col gap-0.5 min-w-[90px]">
          <div className="h-1 w-full bg-slate-200 dark:bg-slate-700 rounded-full overflow-hidden">
            <div 
              className="h-full bg-blue-500 transition-all duration-100" 
              style={{ width: `${duration > 0 ? (currentTime / duration) * 100 : 0}%` }}
            />
          </div>
          <Typography.Text type="secondary" className="text-[10px] font-mono leading-none">
            {formatTime(currentTime)} / {duration > 0 ? formatTime(duration) : '--:--'}
          </Typography.Text>
        </div>
      ) : (
        <Typography.Text type="secondary" className="text-xs">播放录音</Typography.Text>
      )}
    </Space>
  )
}

export function CallRecordPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [selectedCallId, setSelectedCallId] = useState<string | null>(null)
  const [sipDrawerVisible, setSipDrawerVisible] = useState(false)
  
  // 查询过滤状态
  const [filters, setFilters] = useState<{
    callId?: string
    minDuration?: number
    gatewayId?: string
    profile?: string
    extension?: string
    phone?: string
    startTime?: string
    endTime?: string
    userId?: number
  }>(() => ({
    startTime: dayjs().startOf('day').toISOString(),
    endTime: dayjs().endOf('day').toISOString()
  }))

  const queryFields = useMemo(() => [
    { key: 'timeRange', label: '时间范围', type: 'date-range' as const },
    { key: 'minDuration', label: '时长(秒)', type: 'number' as const, placeholder: '≥秒数' },
    { key: 'gatewayId', label: '呼叫网关', type: 'text' as const, placeholder: '输入网关名' },
    {
      key: 'profile',
      label: '呼叫渠道',
      type: 'select' as const,
      options: [
        { value: 'api_outbound', label: 'API 外呼' },
        { value: 'batch_outbound', label: '批量外呼' },
        { value: 'api_direct', label: '拨号盘直呼' },
        { value: 'inbound', label: '客户呼入' },
      ],
    },
    { key: 'extension', label: '分机号码', type: 'text' as const, placeholder: '6位分机号' },
    { key: 'phone', label: '客户号码', type: 'text' as const, placeholder: '输入客户手机号' },
    { key: 'userId', label: '用户 ID', type: 'number' as const, placeholder: '输入用户 ID' },
    { key: 'callId', label: '通话 ID', type: 'text' as const, placeholder: '输入 Call ID' },
  ], [])

  // 用 react-query 自动监听 page、size 和 filters 进行高效条件检索
  const { data, refetch, isPending } = useQuery({
    queryKey: ['merchant', 'call-record', pageNumber, pageSize, filters],
    queryFn: () => fetchCallRecords(pageNumber, pageSize, filters),
  })

  // 多维高级统计指标计算
  const stats = useMemo(() => {
    const records = data?.records ?? []
    let totalBillsec = 0
    let totalRingsec = 0
    let validRingsecCount = 0
    let successCount = 0

    records.forEach((r) => {
      totalBillsec += r.billsec ?? 0
      if (r.ringsec !== undefined && r.ringsec !== null) {
        totalRingsec += r.ringsec
        validRingsecCount++
      }
      const isSuccess = r.billsec > 0
      if (isSuccess) successCount++
    })

    const avgRingsec = validRingsecCount > 0 ? totalRingsec / validRingsecCount : 0
    const successRate = records.length > 0 ? (successCount / records.length) * 100 : 0

    return {
      totalBillsec,
      avgRingsec,
      successRate,
      successCount
    }
  }, [data?.records])

  // 处理查询提交
  const handleSearch = (values: any) => {
    const nextFilters: any = {}
    if (values.callId) nextFilters.callId = values.callId.trim()
    if (values.minDuration !== undefined && values.minDuration !== null) nextFilters.minDuration = values.minDuration
    if (values.gatewayId) nextFilters.gatewayId = values.gatewayId.trim()
    if (values.profile) nextFilters.profile = values.profile
    if (values.extension) nextFilters.extension = values.extension.trim()
    if (values.phone) nextFilters.phone = values.phone.trim()
    if (values.userId !== undefined && values.userId !== null) nextFilters.userId = values.userId
    
    if (values.timeRange && values.timeRange.length === 2) {
      nextFilters.startTime = values.timeRange[0].startOf('day').toISOString()
      nextFilters.endTime = values.timeRange[1].endOf('day').toISOString()
    }

    setFilters(nextFilters)
    setPageNumber(1) // 重置到第一页
  }

  // 重置表单
  const handleReset = () => {
    setFilters({
      startTime: dayjs().startOf('day').toISOString(),
      endTime: dayjs().endOf('day').toISOString()
    })
    setPageNumber(1)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 头部提示与操作区 */}
      <div className="flex justify-end mb-2">
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isPending}>
            刷新数据
          </Button>
          <Button type="primary">导出 Excel 报表</Button>
        </Space>
      </div>

      {/* 奢华高档数据指标看板（支持暗黑模式自适应） */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} md={6}>
          <Card variant="borderless" className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Space direction="vertical" size={4}>
              <Typography.Text type="secondary" className="flex items-center gap-1">
                <DatabaseOutlined className="text-blue-500" /> 总通话次数
              </Typography.Text>
              <Typography.Title level={2} className="!m-0 text-slate-800 dark:text-slate-100">
                {data?.total ?? 0} <span className="text-sm font-normal text-slate-500">次</span>
              </Typography.Title>
            </Space>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card variant="borderless" className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Space direction="vertical" size={4}>
              <Typography.Text type="secondary" className="flex items-center gap-1">
                <ClockCircleOutlined className="text-emerald-500" /> 接通通话总时长
              </Typography.Text>
              <Typography.Title level={2} className="!m-0 text-slate-800 dark:text-slate-100">
                {stats.totalBillsec} <span className="text-sm font-normal text-slate-500">秒</span>
              </Typography.Title>
            </Space>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card variant="borderless" className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Space direction="vertical" size={4}>
              <Typography.Text type="secondary" className="flex items-center gap-1">
                <CustomerServiceOutlined className="text-amber-500" /> 平均振铃时长
              </Typography.Text>
              <Typography.Title level={2} className="!m-0 text-slate-800 dark:text-slate-100">
                {stats.avgRingsec.toFixed(1)} <span className="text-sm font-normal text-slate-500">秒</span>
              </Typography.Title>
            </Space>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card variant="borderless" className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Space direction="vertical" size={4}>
              <Typography.Text type="secondary" className="flex items-center gap-1">
                <PieChartOutlined className="text-purple-500" /> 平均呼叫接通率
              </Typography.Text>
              <Typography.Title level={2} className="!m-0 text-slate-800 dark:text-slate-100">
                {stats.successRate.toFixed(1)} <span className="text-sm font-normal text-slate-500">%</span>
              </Typography.Title>
            </Space>
          </Card>
        </Col>
      </Row>

      <QueryBar
        fields={queryFields}
        onSearch={handleSearch}
        onReset={handleReset}
        loading={isPending}
        initialValues={{
          timeRange: [dayjs().startOf('day'), dayjs().endOf('day')]
        }}
      />

      {/* 多维数据报表主体列表 */}
      <TableWrap
        title="全局呼叫明细数据 (CDR)"
        rowKey="id"
        dataSource={data?.records ?? []}
        loading={isPending}
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
          {
            title: '通话 ID',
            dataIndex: 'callId',
            render: (value: string) => (
              <Typography.Text copyable className="font-mono text-xs text-slate-500">
                {value.length > 20 ? `${value.slice(0, 8)}...${value.slice(-8)}` : value}
              </Typography.Text>
            ),
          },
          {
            title: '所属商户',
            dataIndex: 'merchant',
            render: (value: string) => <Tag color="blue">{value}</Tag>,
          },
          {
            title: '坐席分机',
            dataIndex: 'extension',
            render: (value: string, record: any) => (
              <Space direction="vertical" size={0}>
                <Typography.Text strong className="font-mono">{value || '-'}</Typography.Text>
                <Typography.Text type="secondary" className="text-[10px]">
                  坐席ID: {record.userId || '-'}
                </Typography.Text>
              </Space>
            ),
          },
          {
            title: '渠道',
            dataIndex: 'profile',
            render: (value: string) => {
              const info = profileMap[value] || { label: value, color: 'default' }
              return <Tag color={info.color}>{info.label}</Tag>
            },
          },
          {
            title: '主叫号码',
            dataIndex: 'caller',
            render: (value: string) => (
              <span className="font-mono text-xs text-slate-600 dark:text-zinc-300">
                {value || '-'}
              </span>
            ),
          },
          {
            title: '被叫号码',
            dataIndex: 'callee',
            render: (value: string) => (
              <span className="font-mono text-xs font-semibold text-slate-800 dark:text-zinc-100">
                {value || '-'}
              </span>
            ),
          },
          {
            title: '所经网关',
            dataIndex: 'gatewayName',
            render: (value: string) => (
              <Tag color="cyan" className="max-w-[120px] truncate" title={value}>
                {value}
              </Tag>
            ),
          },
          {
            title: '时段多维分析',
            key: 'timeStats',
            render: (_: any, record: any) => (
              <Space direction="vertical" size={2} className="text-xs">
                <div>
                  <Typography.Text type="secondary">总时长: </Typography.Text>
                  <Typography.Text strong>{record.duration}</Typography.Text>
                </div>
                <div>
                  <Typography.Text type="secondary">实际通话: </Typography.Text>
                  <Typography.Text type="success" strong>
                    {record.billsec} 秒
                  </Typography.Text>
                </div>
                <div>
                  <Typography.Text type="secondary">振铃时长: </Typography.Text>
                  <Typography.Text type="warning" strong>
                    {record.ringsec} 秒
                  </Typography.Text>
                </div>
                <div>
                  <Typography.Text type="secondary">计费时长: </Typography.Text>
                  <Typography.Text type="danger" strong>
                    {record.billingSec} 秒
                  </Typography.Text>
                </div>
              </Space>
            ),
          },
          {
            title: '呼叫状态',
            key: 'callState',
            render: (_: any, record: any) => {
              const isAnswered = record.billsec > 0
              if (isAnswered) {
                const causeInfo = hangupCauseMap[record.hangupCause] || { label: record.hangupCause || '正常结束', color: 'success' }
                return (
                  <Space direction="vertical" size={2}>
                    <Tag color="success" className="font-medium text-xs">
                      已接通
                    </Tag>
                    {record.hangupCause && record.hangupCause !== 'NORMAL_CLEARING' && (
                      <span className="text-[10px] text-slate-400 dark:text-zinc-500">
                        原因: {causeInfo.label}
                      </span>
                    )}
                  </Space>
                )
              } else {
                const causeInfo = hangupCauseMap[record.hangupCause] || { label: record.hangupCause || '未接通', color: 'error' }
                const color = record.hangupCause === 'ORIGINATOR_CANCEL' ? 'processing' : 'error'
                return (
                  <Tag color={color} className="font-medium text-xs">
                    {causeInfo.label || '未接通'}
                  </Tag>
                )
              }
            },
          },
          {
            title: '挂断方',
            key: 'hangupParty',
            render: (_: any, record: any) => {
              const party = getHangupParty(record)
              return <Tag color={party.color}>{party.label}</Tag>
            },
          },
          {
            title: '录音试听',
            dataIndex: 'recordFilePath',
            render: (value: string, record: any) => (
              <CallAudioPlayer filePath={value} callId={record.callId} />
            ),
          },
          {
            title: '通话完成时间',
            dataIndex: 'finishedAt',
            render: (value: string) => (
              <Typography.Text className="text-slate-500 text-xs">
                {value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'}
              </Typography.Text>
            ),
          },
          {
            title: '操作',
            key: 'action',
            render: (_: any, record: any) => (
              <Button
                type="link"
                size="small"
                icon={<DatabaseOutlined />}
                onClick={() => {
                  setSelectedCallId(record.callId)
                  setSipDrawerVisible(true)
                }}
              >
                信令时序
              </Button>
            ),
          },
        ]}
      />

      <SipTraceDrawer
        key={selectedCallId || 'empty'}
        callId={selectedCallId}
        visible={sipDrawerVisible}
        onClose={() => {
          setSipDrawerVisible(false)
          setSelectedCallId(null)
        }}
      />
    </Space>
  )
}

function getLabelClass(item: SipTraceItem) {
  const isRequest = !item.status
  const val = item.method || item.status
  if (isRequest) {
    if (val === 'INVITE') return 'bg-blue-50 text-blue-600 dark:bg-blue-950/40 dark:text-blue-400 border border-blue-200 dark:border-blue-800'
    if (val === 'BYE') return 'bg-rose-50 text-rose-600 dark:bg-rose-950/40 dark:text-rose-400 border border-rose-200 dark:border-rose-800'
    if (val === 'CANCEL') return 'bg-amber-50 text-amber-600 dark:bg-amber-950/40 dark:text-amber-400 border border-amber-200 dark:border-amber-800'
    return 'bg-indigo-50 text-indigo-600 dark:bg-indigo-950/40 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-800'
  } else {
    const statusNum = parseInt(val)
    if (statusNum >= 200 && statusNum < 300) {
      return 'bg-emerald-50 text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800'
    }
    if (statusNum >= 300 && statusNum < 400) {
      return 'bg-cyan-50 text-cyan-600 dark:bg-cyan-950/40 dark:text-cyan-400 border border-cyan-200 dark:border-cyan-800'
    }
    if (statusNum >= 400) {
      return 'bg-rose-50 text-rose-600 dark:bg-rose-950/40 dark:text-rose-400 border border-rose-200 dark:border-rose-800'
    }
    return 'bg-zinc-50 text-zinc-600 dark:bg-zinc-900/60 dark:text-zinc-400 border border-zinc-200 dark:border-zinc-800'
  }
}

type SipTraceDrawerProps = {
  callId: string | null
  visible: boolean
  onClose: () => void
}

function SipTraceDrawer({ callId, visible, onClose }: SipTraceDrawerProps) {
  const [activeItem, setActiveItem] = useState<SipTraceItem | null>(null)
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null)

  const { data, isLoading, error } = useQuery({
    queryKey: ['call-record', 'sip-trace', callId],
    queryFn: () => fetchCallSipTrace(callId!),
    enabled: !!callId && visible,
  })

  // 采用 key 机制在父组件中实现重置，此处无需在渲染阶段执行 state 更新

  const nodes = data?.nodes || []
  const trace = data?.trace || []

  // 格式化展示时间，如 15:25:20.123456
  const formatTime = (ts: string, us: number) => {
    try {
      const d = dayjs(ts)
      const usStr = String(us).padStart(6, '0')
      return `${d.format('HH:mm:ss')}.${usStr}`
    } catch (e) {
      return ts
    }
  }

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
    message.success('SIP 原始报文已成功复制到剪贴板')
  }

  const parsedMsg = useMemo(() => {
    if (!activeItem) return null
    const parts = activeItem.rawMsg.split(/\r?\n\r?\n/)
    const headers = parts[0] || ''
    const body = parts.slice(1).join('\n\n') || ''
    
    // 从 headers 里解析一些关键字段用于结构化展示
    const headerLines = headers.split('\n')
    const keyFields: Record<string, string> = {}
    headerLines.forEach(line => {
      const idx = line.indexOf(':')
      if (idx > 0) {
        const key = line.slice(0, idx).trim().toLowerCase()
        const val = line.slice(idx + 1).trim()
        if (['call-id', 'cseq', 'content-type', 'user-agent', 'from', 'to'].includes(key)) {
          keyFields[key] = val
        }
      }
    })

    return { headers, body, keyFields }
  }, [activeItem])

  return (
    <Drawer
      title={
        <div className="flex justify-between items-center pr-8 w-full">
          <span className="font-bold text-slate-800 dark:text-zinc-200">SIP 信令链路时序追踪 (sngrep)</span>
          {callId && (
            <span className="font-mono text-xs font-normal text-slate-400 dark:text-zinc-500">
              Call-ID: {callId}
            </span>
          )}
        </div>
      }
      placement="right"
      width={1180}
      onClose={onClose}
      open={visible}
      destroyOnClose
    >
      {isLoading ? (
        <div className="flex flex-col items-center justify-center h-full gap-4">
          <Spin size="large" tip="正在加载并编排 SIP 信令时序链路..." />
        </div>
      ) : error ? (
        <div className="p-6">
          <Alert
            message="加载信令链路失败"
            description={
              <div>
                <p>请求错误: {error instanceof Error ? error.message : '未知网络异常'}</p>
                <p className="text-xs text-slate-400 mt-2">
                  提示：如果该通话是近期呼叫，请确认全局 “SIP 信令双时追踪 (SipTrace)” 开关已开启，并且缓存未超过 2 小时限制。
                </p>
              </div>
            }
            type="error"
            showIcon
          />
        </div>
      ) : trace.length === 0 ? (
        <Empty
          description={
            <div className="text-center py-8">
              <p className="text-slate-500 font-semibold mb-1">未捕获到该通话的 SIP 信令报文</p>
              <p className="text-xs text-slate-400 max-w-md mx-auto leading-relaxed">
                可能的原因：<br />
                1. 系统全局 “SIP 信令双时追踪” 开关在呼叫发生时未开启。<br />
                2. 该通话发生时间已超过 2 小时，已被 Redis 自动过期释放。<br />
                3. 此通话为内部心跳 options 检测或未经过 Kamailio 代理节点。
              </p>
            </div>
          }
        />
      ) : (
        <div className="flex h-full gap-4 overflow-hidden select-none animate-fade-in">
          {/* 左侧：时序图绘制区 */}
          <div className="w-[62%] h-full flex flex-col border border-slate-200 dark:border-zinc-800 rounded-xl bg-slate-50/50 dark:bg-zinc-900/20 overflow-hidden">
            {/* Lifeline Headers */}
            <div className="bg-slate-100/80 dark:bg-zinc-850 border-b border-slate-200 dark:border-zinc-800 p-4 shrink-0 overflow-x-auto overflow-y-hidden">
              <div className="flex relative" style={{ minWidth: nodes.length * 200 }}>
                {nodes.map((node, i) => (
                  <div key={node} className="text-center" style={{ width: 200 }}>
                    <div className="bg-white dark:bg-zinc-800 px-3 py-2 rounded-lg border border-slate-200 dark:border-zinc-700 truncate shadow-sm font-mono text-[11px] font-bold text-slate-700 dark:text-zinc-300">
                      {node}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Lifeline Lines and Messages Grid */}
            <div className="grow overflow-auto relative p-4 bg-white dark:bg-zinc-950">
              {/* Backing Vertical Dashed Lines */}
              <div className="absolute top-0 bottom-0 left-4 right-4 pointer-events-none flex" style={{ minWidth: nodes.length * 200 }}>
                {nodes.map((node, i) => (
                  <div key={node} className="h-full border-r border-dashed border-slate-200 dark:border-zinc-850" style={{ width: 200 }} />
                ))}
              </div>

              {/* Trace items rows */}
              <div className="relative flex flex-col space-y-0" style={{ minWidth: nodes.length * 200 }}>
                {trace.map((item, idx) => {
                  const fromIdx = nodes.indexOf(item.fromIp)
                  const toIdx = nodes.indexOf(item.toIp)
                  
                  if (fromIdx < 0 || toIdx < 0) return null

                  const fromX = fromIdx * 200 + 100
                  const toX = toIdx * 200 + 100
                  const isHovered = hoveredIdx === idx
                  const isActive = activeItem?.id === item.id

                  const isSelfLoop = fromIdx === toIdx

                  return (
                    <div
                      key={item.id}
                      className={`relative h-[56px] transition-colors duration-150 cursor-pointer rounded-lg ${
                        isActive 
                          ? 'bg-indigo-500/5 dark:bg-indigo-500/10' 
                          : isHovered 
                            ? 'bg-slate-50 dark:bg-zinc-900/40' 
                            : ''
                      }`}
                      onMouseEnter={() => setHoveredIdx(idx)}
                      onMouseLeave={() => setHoveredIdx(null)}
                      onClick={() => setActiveItem(item)}
                    >
                      {/* Left side timestamp */}
                      <div className="absolute left-1 top-4 pointer-events-none font-mono text-[9px] text-slate-400 dark:text-zinc-500 bg-white/80 dark:bg-zinc-950/80 px-1 rounded z-10">
                        {formatTime(item.timestamp, item.timeUs)}
                      </div>

                      {/* Line arrow SVG */}
                      <svg width={nodes.length * 200} height="56" className="overflow-visible absolute top-0 left-0">
                        {isSelfLoop ? (
                          <>
                            {/* Self loop path */}
                            <path
                              d={`M ${fromX} 14 C ${fromX + 45} 14, ${fromX + 45} 42, ${fromX} 42`}
                              fill="none"
                              stroke={isActive || isHovered ? "#6366f1" : "#94a3b8"}
                              strokeWidth={isActive || isHovered ? 2.2 : 1.5}
                            />
                            <polygon 
                              points={`${fromX},42 ${fromX+7},37 ${fromX+7},47`} 
                              fill={isActive || isHovered ? "#6366f1" : "#94a3b8"}
                            />
                          </>
                        ) : (
                          <>
                            {/* Straight line */}
                            <line
                              x1={fromX}
                              y1={28}
                              x2={toX}
                              y2={28}
                              stroke={isActive || isHovered ? "#6366f1" : "#94a3b8"}
                              strokeWidth={isActive || isHovered ? 2.2 : 1.5}
                              strokeDasharray={item.method === "ACK" ? "4 3" : undefined}
                            />
                            {/* Arrowhead */}
                            {toX > fromX ? (
                              <polygon 
                                points={`${toX},28 ${toX-8},23 ${toX-8},33`} 
                                fill={isActive || isHovered ? "#6366f1" : "#94a3b8"}
                              />
                            ) : (
                              <polygon 
                                points={`${toX},28 ${toX+8},23 ${toX+8},33`} 
                                fill={isActive || isHovered ? "#6366f1" : "#94a3b8"}
                              />
                            )}
                          </>
                        )}
                      </svg>

                      {/* Text Label */}
                      <div 
                        className="absolute flex items-center justify-center pointer-events-none" 
                        style={{ 
                          left: isSelfLoop ? fromX + 15 : Math.min(fromX, toX) + 10, 
                          width: isSelfLoop ? 100 : Math.abs(fromX - toX) - 20, 
                          height: '100%' 
                        }}
                      >
                        <span className={`px-2 py-0.5 rounded shadow-sm text-[10px] font-mono font-bold ${getLabelClass(item)}`}>
                          {item.method || item.status}
                        </span>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          </div>

          {/* 右侧：报文详情查看区 */}
          <div className="w-[38%] h-full flex flex-col border border-slate-200 dark:border-zinc-800 rounded-xl bg-slate-50/30 dark:bg-zinc-900/10 overflow-hidden">
            {activeItem ? (
              <div className="h-full flex flex-col overflow-hidden">
                {/* Active Header summary */}
                <div className="p-4 bg-slate-100/40 dark:bg-zinc-900 border-b border-slate-200 dark:border-zinc-800 shrink-0">
                  <div className="flex justify-between items-center">
                    <div>
                      <div className="text-[10px] text-slate-400 font-mono tracking-wider font-bold">SIP MESSAGE DETAILS</div>
                      <div className="text-sm font-bold font-mono text-slate-800 dark:text-zinc-200 mt-0.5">
                        {activeItem.method || activeItem.status}
                      </div>
                    </div>
                    <Button 
                      size="small" 
                      icon={<CopyOutlined />}
                      onClick={() => handleCopy(activeItem.rawMsg)}
                    >
                      复制
                    </Button>
                  </div>
                </div>

                {/* Tab layout for headers/SDP */}
                <div className="grow overflow-hidden flex flex-col">
                  <Tabs
                    className="h-full flex flex-col"
                    tabBarStyle={{ paddingLeft: 16, marginBottom: 0 }}
                    items={[
                      {
                        key: 'headers',
                        label: 'SIP 头部 (Headers)',
                        children: (
                          <div className="h-full p-4 overflow-auto bg-slate-50 dark:bg-zinc-950">
                            {/* Quick specs */}
                            <Descriptions column={1} size="small" className="mb-4 bg-white dark:bg-zinc-900 p-3 rounded-lg border border-slate-150 dark:border-zinc-850 text-xs font-mono">
                              <Descriptions.Item label="时间">{formatTime(activeItem.timestamp, activeItem.timeUs)}</Descriptions.Item>
                              <Descriptions.Item label="源 IP">{activeItem.fromIp}</Descriptions.Item>
                              <Descriptions.Item label="目标 IP">{activeItem.toIp}</Descriptions.Item>
                              {parsedMsg?.keyFields['cseq'] && (
                                <Descriptions.Item label="CSeq">{parsedMsg.keyFields['cseq']}</Descriptions.Item>
                              )}
                              {parsedMsg?.keyFields['user-agent'] && (
                                <Descriptions.Item label="User-Agent">{parsedMsg.keyFields['user-agent']}</Descriptions.Item>
                              )}
                            </Descriptions>

                            <pre className="font-mono text-xs text-zinc-300 whitespace-pre-wrap select-text p-3 rounded bg-zinc-900 overflow-x-auto border border-zinc-800 leading-relaxed shadow-inner">
                              {parsedMsg?.headers}
                            </pre>
                          </div>
                        )
                      },
                      {
                        key: 'sdp',
                        label: '媒体参数 (SDP)',
                        disabled: !parsedMsg?.body,
                        children: (
                          <div className="h-full p-4 overflow-auto bg-slate-50 dark:bg-zinc-950">
                            {parsedMsg?.body ? (
                              <pre className="font-mono text-xs text-zinc-300 whitespace-pre-wrap select-text p-3 rounded bg-zinc-900 overflow-x-auto border border-zinc-800 leading-relaxed shadow-inner">
                                {parsedMsg.body}
                              </pre>
                            ) : (
                              <Empty description="该消息不带 SDP 媒体协商内容" className="mt-8" />
                            )}
                          </div>
                        )
                      }
                    ]}
                  />
                </div>
              </div>
            ) : (
              <div className="h-full flex flex-col items-center justify-center text-slate-400 p-8 text-center bg-white dark:bg-zinc-950">
                <DatabaseOutlined className="text-3xl mb-3 text-slate-300 dark:text-zinc-700" />
                <p className="text-xs font-semibold text-slate-400 dark:text-zinc-500">
                  点击左侧信令序列中的行，可在此查看原始 SIP Header 和 SDP 媒体属性协商报文
                </p>
              </div>
            )}
          </div>
        </div>
      )}
    </Drawer>
  )
}

