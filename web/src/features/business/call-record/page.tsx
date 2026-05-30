import { Button, Space, Tag, Typography, Card, Row, Col } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import { fetchCallRecords } from '@/api/operate'
import {
  SearchOutlined,
  ReloadOutlined,
  DatabaseOutlined,
  ClockCircleOutlined,
  CustomerServiceOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  PieChartOutlined
} from '@ant-design/icons'
import dayjs from 'dayjs'

// 渠道类型映射中文
const profileMap: Record<string, { label: string; color: string }> = {
  api_outbound: { label: 'API 外呼', color: 'blue' },
  batch_outbound: { label: '批量外呼', color: 'purple' },
  api_direct: { label: '拨号盘直呼', color: 'cyan' },
  inbound: { label: '客户呼入', color: 'green' },
}

// 通话状态高级语义映射
const stateMap: Record<string, { label: string; color: string }> = {
  NORMAL_CLEARING: { label: '正常挂断', color: 'success' },
  completed: { label: '已接通', color: 'success' },
  bridged: { label: '已桥接', color: 'success' },
  USER_BUSY: { label: '用户忙', color: 'warning' },
  NO_USER_RESPONSE: { label: '无应答', color: 'default' },
  CALL_REJECTED: { label: '用户拒接', color: 'error' },
  ORIGINATOR_CANCEL: { label: '主叫取消', color: 'processing' },
  NO_ROUTE_DESTINATION: { label: '空号/无路由', color: 'error' },
  SUBSCRIBER_ABSENT: { label: '关机/不在服务区', color: 'warning' },
}

// 录音试听播放组件
function CallAudioPlayer({ filePath, callId }: { filePath?: string; callId: string }) {
  const [playing, setPlaying] = useState(false)
  const [audio, setAudio] = useState<HTMLAudioElement | null>(null)

  if (!filePath) {
    return <Typography.Text type="secondary" className="text-xs">无录音</Typography.Text>
  }

  const togglePlay = () => {
    if (playing) {
      audio?.pause()
      setPlaying(false)
    } else {
      const url = filePath.startsWith('http') ? filePath : `http://localhost:8080/records${filePath}`
      let newAudio = audio
      if (!newAudio) {
        newAudio = new Audio(url)
        newAudio.addEventListener('ended', () => setPlaying(false))
        newAudio.addEventListener('error', () => {
          setPlaying(false)
          console.warn(`录音物理文件尚不可用 (ID: ${callId})，路径: ${filePath}`)
        })
        setAudio(newAudio)
      }
      newAudio.play().catch((err) => {
        console.warn('播放失败，可能是游览器策略或录音文件暂未就绪:', err)
      })
      setPlaying(true)
    }
  }

  return (
    <Button
      type={playing ? 'primary' : 'default'}
      size="small"
      icon={playing ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
      onClick={togglePlay}
      className="flex items-center gap-1 text-xs"
    >
      {playing ? '暂停' : '播放'}
    </Button>
  )
}

export function CallRecordPage() {
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  
  // 查询过滤状态
  const [filters, setFilters] = useState<{
    callId?: string
    minDuration?: number
    gatewayId?: string
    profile?: string
    extension?: string
    startTime?: string
    endTime?: string
  }>({})

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
      const isSuccess = r.state === 'NORMAL_CLEARING' || r.state === 'completed' || r.state === 'bridged'
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
    
    if (values.timeRange && values.timeRange.length === 2) {
      nextFilters.startTime = values.timeRange[0].startOf('day').toISOString()
      nextFilters.endTime = values.timeRange[1].endOf('day').toISOString()
    }

    setFilters(nextFilters)
    setPageNumber(1) // 重置到第一页
  }

  // 重置表单
  const handleReset = () => {
    setFilters({})
    setPageNumber(1)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 头部提示与操作区 */}
      <div className="flex justify-between items-center mb-2">
        <Typography.Text type="secondary">
          支持针对网关、分机、计费及振铃等多维 CDR 指标的精细化全局检索与数据分析。
        </Typography.Text>
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
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
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
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
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
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
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
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
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
            title: '主/被叫号码',
            key: 'numbers',
            render: (_: any, record: any) => (
              <Space direction="vertical" size={2} className="text-xs">
                <div>
                  <span className="text-slate-400">主叫: </span>
                  <span className="font-mono">{record.caller}</span>
                </div>
                <div>
                  <span className="text-slate-400">被叫: </span>
                  <span className="font-mono font-semibold text-slate-700 dark:text-slate-300">{record.callee}</span>
                </div>
              </Space>
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
            dataIndex: 'state',
            render: (value: string) => {
              const stateInfo = stateMap[value] || { label: value, color: 'default' }
              return (
                <Tag color={stateInfo.color} className="font-medium text-xs">
                  {stateInfo.label}
                </Tag>
              )
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
        ]}
      />
    </Space>
  )
}
