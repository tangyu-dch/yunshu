import {
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Space,
  Switch,
  Tag,
  Typography,
  message,
  Drawer,
  Table,
  Statistic,
  Row,
  Col,
  Progress,
  Card,
  Select,
  Radio,
  Tooltip,
  Dropdown,
  Avatar,
  Badge,
  App
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo, useEffect } from 'react'
import { PermissionGate } from '@/components/PermissionGate'
import { QueryBar } from '@/components/QueryBar'
import { useAuthStore } from '@/store/auth'
import {
  fetchBatchTasks,
  saveBatchTask,
  deleteBatchTasks,
  toggleBatchTaskEnable,
  importBatchTaskTels,
  fetchBatchTaskDetails,
  fetchDepartmentsList,
  fetchSkillGroups
} from '@/api/operate'
import {
  ReloadOutlined,
  DeleteOutlined,
  PlusOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  ImportOutlined,
  ProfileOutlined,
  CheckCircleOutlined,
  InfoCircleOutlined,
  DownOutlined,
  EditOutlined,
  ThunderboltOutlined,
  DashboardFilled,
  SignalFilled,
  CheckCircleFilled,
  ClockCircleFilled,
  PlayCircleTwoTone,
  PauseCircleTwoTone
} from '@ant-design/icons'

type BatchTaskFormValues = {
  id?: number
  name: string
  merchantId: number
  userId: number
  connectedInterval: number
  unconnectedInterval: number
  callTimePeriod?: string
  aiFlag: boolean
  enable: boolean
  skillGroupId?: number
  departmentId?: number
  callMode: number
  callRatio?: number
  queueEnable?: boolean
}

// ----------------------------------------------------
// 7x24 小时呼叫时间段网格选择器辅助函数与组件
// ----------------------------------------------------

function parseOldFormat(timeStr: string): boolean[][] {
  const grid = Array.from({ length: 7 }, () => Array(24).fill(false))
  if (!timeStr) {
    // 默认全选
    return Array.from({ length: 7 }, () => Array(24).fill(true))
  }
  
  // 兼容老格式：例如 "09:00-12:00,14:00-18:00"
  const parts = timeStr.split(',')
  parts.forEach(part => {
    const range = part.trim().split('-')
    if (range.length === 2) {
      const startHour = parseInt(range[0].split(':')[0], 10)
      const endHour = parseInt(range[1].split(':')[0], 10)
      if (!isNaN(startHour) && !isNaN(endHour)) {
        for (let day = 0; day < 7; day++) {
          for (let h = startHour; h < endHour; h++) {
            if (h >= 0 && h < 24) {
              grid[day][h] = true
            }
          }
        }
      }
    }
  })
  return grid
}

function serializeGrid(grid: boolean[][]): string {
  return grid.map(row => row.map(cell => cell ? '1' : '0').join('')).join('')
}

function deserializeGrid(str: string): boolean[][] {
  if (!str || str.length !== 168 || !/^[01]+$/.test(str)) {
    return parseOldFormat(str)
  }
  const grid: boolean[][] = []
  for (let i = 0; i < 7; i++) {
    const rowStr = str.substring(i * 24, (i + 1) * 24)
    grid.push(rowStr.split('').map(char => char === '1'))
  }
  return grid
}

function formatTimePeriodSummary(str: string): string {
  if (!str) return '未限制'
  if (str.length !== 168 || !/^[01]+$/.test(str)) {
    return str // 如果是老格式，直接原样返回
  }
  
  const allSelected = str.split('').every(c => c === '1')
  if (allSelected) return '全周整天可选'
  
  const allDeselected = str.split('').every(c => c === '0')
  if (allDeselected) return '全周禁止呼叫'

  const days = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']
  const summaries: string[] = []
  
  for (let i = 0; i < 7; i++) {
    const rowStr = str.substring(i * 24, (i + 1) * 24)
    const selectedHours: number[] = []
    rowStr.split('').forEach((char, h) => {
      if (char === '1') selectedHours.push(h)
    })
    
    if (selectedHours.length === 24) {
      summaries.push(`${days[i]}:全天`)
    } else if (selectedHours.length > 0) {
      const ranges: string[] = []
      let start = selectedHours[0]
      let prev = selectedHours[0]
      for (let idx = 1; idx < selectedHours.length; idx++) {
        const curr = selectedHours[idx]
        if (curr !== prev + 1) {
          ranges.push(`${start.toString().padStart(2, '0')}:00-${(prev + 1).toString().padStart(2, '0')}:00`)
          start = curr
        }
        prev = curr
      }
      ranges.push(`${start.toString().padStart(2, '0')}:00-${(prev + 1).toString().padStart(2, '0')}:00`)
      summaries.push(`${days[i]}:${ranges.join(',')}`)
    }
  }

  const firstDetail = summaries[0]?.split(':')[1]
  const isAllSame = summaries.length === 7 && summaries.every(s => s.split(':')[1] === firstDetail)
  if (isAllSame && firstDetail) {
    return `每天: ${firstDetail}`
  }

  const workSame = summaries.slice(0, 5).every(s => s.split(':')[1] === summaries[0]?.split(':')[1])
  if (workSame && summaries[0]) {
    const workDetail = summaries[0].split(':')[1]
    const weekendHasCalls = summaries.slice(5).some(s => s.includes(':'))
    if (!weekendHasCalls) {
      return `工作日: ${workDetail}`
    }
  }

  if (summaries.length > 2) {
    return `自定义 (${summaries.length} 天已配置)`
  }
  return summaries.join(' | ') || '无配置'
}

interface TimePeriodSelectorProps {
  value?: string
  onChange?: (value: string) => void
}

interface TimeSegment {
  start: number
  end: number
  text: string
  mid: number
}

const getRowSegments = (row: boolean[]): TimeSegment[] => {
  const segments: TimeSegment[] = []
  let start: number | null = null
  for (let h = 0; h < 24; h++) {
    if (row[h]) {
      if (start === null) {
        start = h
      }
    } else {
      if (start !== null) {
        const end = h - 1
        segments.push({
          start,
          end,
          text: `${String(start).padStart(2, '0')}:00 - ${String(h).padStart(2, '0')}:00`,
          mid: Math.floor((start + end) / 2)
        })
        start = null
      }
    }
  }
  if (start !== null) {
    segments.push({
      start,
      end: 23,
      text: `${String(start).padStart(2, '0')}:00 - 24:00`,
      mid: Math.floor((start + 23) / 2)
    })
  }
  return segments
}

export function TimePeriodSelector({ value, onChange }: TimePeriodSelectorProps) {
  const grid = useMemo(() => deserializeGrid(value ?? ''), [value])

  const handleGridChange = (newGrid: boolean[][]) => {
    if (onChange) {
      onChange(serializeGrid(newGrid))
    }
  }

  const [isMouseDown, setIsMouseDown] = useState(false)
  const [selectMode, setSelectMode] = useState<boolean>(true)
  const [startPos, setStartPos] = useState<{ r: number; c: number } | null>(null)
  const [initialGrid, setInitialGrid] = useState<boolean[][] | null>(null)

  useEffect(() => {
    const handleGlobalMouseUp = () => {
      setIsMouseDown(false)
      setStartPos(null)
      setInitialGrid(null)
    }
    window.addEventListener('mouseup', handleGlobalMouseUp)
    return () => {
      window.removeEventListener('mouseup', handleGlobalMouseUp)
    }
  }, [])

  const handleMouseDown = (r: number, c: number) => {
    setIsMouseDown(true)
    const currentVal = grid[r][c]
    const mode = !currentVal
    setSelectMode(mode)
    setStartPos({ r, c })
    setInitialGrid(grid)

    const newGrid = grid.map((row, ri) =>
      row.map((cell, ci) => (ri === r && ci === c ? mode : cell))
    )
    handleGridChange(newGrid)
  }

  const handleMouseEnter = (r: number, c: number) => {
    if (!isMouseDown || !startPos || !initialGrid) return

    const minR = Math.min(startPos.r, r)
    const maxR = Math.max(startPos.r, r)
    const minC = Math.min(startPos.c, c)
    const maxC = Math.max(startPos.c, c)

    const newGrid = initialGrid.map((row, ri) =>
      row.map((cell, ci) => {
        if (ri >= minR && ri <= maxR && ci >= minC && ci <= maxC) {
          return selectMode
        }
        return cell
      })
    )
    handleGridChange(newGrid)
  }

  const handleQuickAction = (action: 'all' | 'workdays' | 'clear') => {
    let newGrid = Array.from({ length: 7 }, () => Array(24).fill(false))
    if (action === 'all') {
      newGrid = Array.from({ length: 7 }, () => Array(24).fill(true))
    } else if (action === 'workdays') {
      newGrid = Array.from({ length: 7 }, (_, r) => Array(24).fill(r < 5))
    }
    handleGridChange(newGrid)
  }

  const days = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']
  const hours = Array.from({ length: 24 }, (_, i) => i)

  return (
    <div className="col-span-1 md:col-span-2 flex flex-col md:flex-row items-start gap-4 w-full select-none mt-4 mb-2">
      <div className="flex-shrink-0 pt-2 text-[14px] font-medium text-slate-800 dark:text-slate-200 w-[100px] text-right">
        <span className="text-red-500 mr-1">*</span>呼叫时间段：
      </div>

      <div className="flex-1 min-w-0 w-full">
        <div className="overflow-x-auto border border-slate-200 dark:border-slate-800 rounded-lg">
          <table className="border-collapse table-fixed w-full min-w-[760px] select-none">
            <thead>
              <tr className="bg-slate-50 dark:bg-slate-900/60">
                <th className="border border-slate-200 dark:border-slate-800 p-2 text-center font-normal text-xs text-slate-500 w-[80px]">
                  星期/时间
                </th>
                {hours.map(h => (
                  <th key={h} className="border border-slate-200 dark:border-slate-800 p-1 text-center font-normal text-[10px] text-slate-500">
                    {h}:00
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {days.map((day, r) => {
                const segments = getRowSegments(grid[r])
                const segmentMap = new Map<number, TimeSegment>()
                segments.forEach(seg => {
                  if (seg.end - seg.start >= 2) {
                    segmentMap.set(seg.mid, seg)
                  }
                })

                return (
                  <tr key={day}>
                    <td className="border border-slate-200 dark:border-slate-800 p-2 text-center font-normal text-xs bg-slate-50/50 dark:bg-slate-900/20 text-slate-600 dark:text-slate-400">
                      {day}
                    </td>
                    {hours.map(c => {
                      const isSelected = grid[r][c]
                      const seg = segmentMap.get(c)
                      const showRowText = !!seg
                      const rowText = seg ? seg.text : ''

                      return (
                        <td
                          key={c}
                          onMouseDown={() => handleMouseDown(r, c)}
                          onMouseEnter={() => handleMouseEnter(r, c)}
                          className={`border border-slate-200 dark:border-slate-800 cursor-pointer h-9 transition-colors duration-150 relative ${
                            isSelected
                              ? 'bg-[#1677ff] border-white/20 hover:bg-[#4096ff]'
                              : 'bg-white dark:bg-slate-950 hover:bg-slate-100/80 dark:hover:bg-slate-900'
                          }`}
                        >
                          {showRowText && (
                            <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-10 w-[120px] left-1/2 -translate-x-1/2">
                              <span className="text-[10px] font-medium text-white bg-[#0f2547]/95 px-2 py-0.5 rounded shadow-md border border-white/10 scale-95 md:scale-100">
                                {rowText}
                              </span>
                            </div>
                          )}
                        </td>
                      )
                    })}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>

        <div className="mt-3 flex flex-wrap justify-between items-center text-[11px] gap-2 select-none">
          <div className="flex flex-wrap items-center gap-4 text-slate-400 dark:text-slate-500">
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 bg-[#1677ff] rounded-sm inline-block"></span>
              <span>允许呼叫</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 bg-white dark:bg-slate-950 border border-slate-300 dark:border-slate-700 rounded-sm inline-block"></span>
              <span>禁止呼叫</span>
            </div>
            <span className="hidden sm:inline text-slate-400 dark:text-slate-500">
              💡 提示：按住鼠标左键拖拽可批量框选/取消时段
            </span>
          </div>
          <Space size="middle" className="text-slate-400 dark:text-slate-500">
            <Button type="link" size="small" onClick={() => handleQuickAction('all')} className="text-[11px] p-0 h-auto font-normal">整周全天</Button>
            <span className="text-slate-300 dark:text-slate-800">|</span>
            <Button type="link" size="small" onClick={() => handleQuickAction('workdays')} className="text-[11px] p-0 h-auto font-normal">工作日</Button>
            <span className="text-slate-300 dark:text-slate-800">|</span>
            <Button type="link" size="small" onClick={() => handleQuickAction('clear')} danger className="text-[11px] p-0 h-auto font-normal">清空</Button>
          </Space>
        </div>
      </div>
    </div>
  )
}

// ----------------------------------------------------
// 主页面组件 (BatchTaskPage)
// ----------------------------------------------------

export function BatchTaskPage() {
  const { modal, message: appMessage } = App.useApp()
  const tenant = useAuthStore((state) => state.tenant)
  const merchantId = Number(tenant?.merchantId || '1001')
  const userId = Number(tenant?.userId || '1')

  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  // Import Numbers Modal State
  const [importTaskId, setImportTaskId] = useState<number | null>(null)
  const [importForm] = Form.useForm()

  // Details Drawer State
  const [detailTaskId, setDetailTaskId] = useState<number | null>(null)

  const [form] = Form.useForm<BatchTaskFormValues>()
  const queryClient = useQueryClient()

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['merchant', 'batch-task', pageNumber, pageSize, merchantId],
    queryFn: () => fetchBatchTasks(pageNumber, pageSize),
  })

  // Fetch department list for dropdown
  const { data: deptsData } = useQuery({
    queryKey: ['merchant', 'department', 'list', merchantId],
    queryFn: () => fetchDepartmentsList(merchantId),
  })

  // Fetch skill groups list for dropdown
  const { data: skillGroupsData } = useQuery({
    queryKey: ['merchant', 'skill-group', 'list', merchantId],
    queryFn: () => fetchSkillGroups(1, 1000),
  })

  const queryFields = useMemo(() => [
    { key: 'name', label: '任务名称', type: 'text' as const, placeholder: '请输入外呼任务名称模糊搜索' },
    {
      key: 'status',
      label: '任务状态',
      type: 'select' as const,
      options: [
        { value: 'running', label: '运行中' },
        { value: 'paused', label: '已暂停' },
        { value: 'completed', label: '已完成' },
      ],
    },
  ], [])

  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.name) {
      records = records.filter((r: any) => String(r.name).toLowerCase().includes(queryParams.name.toLowerCase().trim()))
    }
    if (queryParams.status) {
      records = records.filter((r: any) => r.status === queryParams.status)
    }
    return records
  }, [data, queryParams])

  // Fetch detailed dial logs of selected task
  const { data: detailsData, isLoading: detailsLoading } = useQuery({
    queryKey: ['merchant', 'batch-task-details', detailTaskId],
    queryFn: () => fetchBatchTaskDetails(detailTaskId!),
    enabled: !!detailTaskId,
  })

  const saveMutation = useMutation({
    mutationFn: async (values: BatchTaskFormValues) =>
      saveBatchTask({
        id: editingId ?? undefined,
        name: values.name,
        merchantId,
        userId,
        connectedInterval: values.connectedInterval ?? 600,
        unconnectedInterval: values.unconnectedInterval ?? 1200,
        callTimePeriod: values.callTimePeriod || '111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111',
        aiFlag: Boolean(values.aiFlag),
        enable: Boolean(values.enable),
        skillGroupId: values.skillGroupId || undefined,
        departmentId: values.departmentId || undefined,
        callMode: values.callMode ?? 1,
        callRatio: values.callMode === 1 ? (values.callRatio ?? 1.5) : undefined,
        queueEnable: values.callMode === 1 ? Boolean(values.queueEnable) : undefined,
      }),
    onSuccess: async () => {
      appMessage.success(editingId ? '外呼任务已更新' : '外呼任务已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-task'] })
    },
    onError: (error) => appMessage.error(error instanceof Error ? error.message : '保存失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteBatchTasks(ids),
    onSuccess: async () => {
      appMessage.success('外呼任务已删除')
      setSelectedIds([])
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-task'] })
    },
    onError: (error) => appMessage.error(error instanceof Error ? error.message : '删除失败'),
  })

  const toggleEnableMutation = useMutation({
    mutationFn: async ({ id, enable }: { id: number; enable: boolean }) =>
      toggleBatchTaskEnable(id, enable, enable ? '' : '手动暂停'),
    onSuccess: async () => {
      appMessage.success('任务状态已更新')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-task'] })
    },
    onError: (error) => appMessage.error(error instanceof Error ? error.message : '状态更新失败'),
  })

  const importTelsMutation = useMutation({
    mutationFn: async (values: { tels: string }) => {
      const telList = values.tels
        .split(/[\n,]/)
        .map((t) => t.trim())
        .filter((t) => t.length > 0)
      if (telList.length === 0) throw new Error('请输入有效的号码列表')
      return importBatchTaskTels(importTaskId!, merchantId, userId, telList)
    },
    onSuccess: async (res) => {
      appMessage.success(`成功导入 ${res.imported ?? 0} 个号码`)
      setImportTaskId(null)
      importForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'batch-task'] })
    },
    onError: (error) => appMessage.error(error instanceof Error ? error.message : '导入号码失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({
        connectedInterval: 600,
        unconnectedInterval: 1200,
        callTimePeriod: '111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111',
        aiFlag: false,
        enable: true,
        callMode: 1,
        callRatio: 1.5,
        queueEnable: true,
      })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item) => item.id === id)
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        name: record?.name ?? '',
        connectedInterval: record?.connectedInterval ?? 600,
        unconnectedInterval: record?.unconnectedInterval ?? 1200,
        callTimePeriod: record?.callTimePeriod ?? '111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111',
        aiFlag: record?.aiFlag ?? false,
        enable: record?.status === 'running',
        skillGroupId: record?.skillGroupId,
        departmentId: record?.departmentId,
        callMode: record?.callMode ?? 1,
        callRatio: record?.callRatio ?? 1.5,
        queueEnable: record?.queueEnable !== false,
      })
    }, 0)
  }

  // Pre-calculate statistics for the top cards grid
  const stats = useMemo(() => {
    const records = data?.records ?? []
    const totalTasks = data?.total ?? 0
    const runningTasks = records.filter(r => r.status === 'running').length
    const totalCompleted = records.reduce((sum, r) => sum + (r.completed ?? 0), 0)
    const totalConnected = records.reduce((sum, r) => sum + (r.connected ?? 0), 0)
    const avgConnectRate = totalCompleted > 0 ? Math.round((totalConnected / totalCompleted) * 100) : 0

    return {
      totalTasks,
      runningTasks,
      totalCompleted,
      avgConnectRate
    }
  }, [data])

  // Pre-calculate statistics for the current details Drawer
  const getDrawerStats = () => {
    if (!detailsData || detailsData.length === 0) {
      return { total: 0, called: 0, connected: 0, rate: 0, gt30s: 0, gt30sRate: 0, active: 0, calling: 0 }
    }
    const total = detailsData.length
    let called = 0
    let connected = 0
    let gt30s = 0
    let calling = 0
    let active = 0

    detailsData.forEach((item: any) => {
      if (item.callStatus === 2) {
        calling++
      }
      if (item.callStatus === 3) {
        called++
        if (item.connectStatus === true) {
          connected++
          if (item.durationSec && item.durationSec > 30) {
            gt30s++
          }
        }
      } else {
        active++
      }
    })

    const rate = called > 0 ? Math.round((connected / called) * 100) : 0
    const gt30sRate = called > 0 ? Math.round((gt30s / called) * 100) : 0

    return { total, called, connected, rate, gt30s, gt30sRate, active, calling }
  }

  const drawerStats = getDrawerStats()

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 顶部高表现力数据指标看板 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} md={6}>
          <Card
            variant="borderless"
            className="rounded-xl shadow-soft bg-white dark:bg-[#15181e] border border-slate-100 dark:border-slate-800/60 hover:shadow-md transition-all duration-300"
            styles={{ body: { padding: '20px' } }}
          >
            <div className="flex justify-between items-center">
              <div>
                <Typography.Text type="secondary" className="text-xs font-semibold uppercase tracking-wider block mb-1">外呼任务总数</Typography.Text>
                <div className="text-2xl font-bold dark:text-white">
                  {stats.totalTasks} <span className="text-xs font-normal text-slate-400">个</span>
                </div>
              </div>
              <div className="p-3 bg-blue-50 dark:bg-blue-950/40 rounded-xl">
                <DashboardFilled className="text-2xl text-blue-500" />
              </div>
            </div>
            <div className="mt-3 text-xs text-slate-400 flex items-center border-t border-slate-50 dark:border-slate-800/40 pt-2">
              <span className="font-semibold text-blue-500 mr-1">云枢调度</span> 系统级批处理呼叫队列
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card
            variant="borderless"
            className="rounded-xl shadow-soft bg-white dark:bg-[#15181e] border border-slate-100 dark:border-slate-800/60 hover:shadow-md transition-all duration-300"
            styles={{ body: { padding: '20px' } }}
          >
            <div className="flex justify-between items-center">
              <div>
                <Typography.Text type="secondary" className="text-xs font-semibold uppercase tracking-wider block mb-1">执行中任务</Typography.Text>
                <div className="text-2xl font-bold dark:text-white">
                  {stats.runningTasks} <span className="text-xs font-normal text-slate-400">个</span>
                </div>
              </div>
              <div className="p-3 bg-emerald-50 dark:bg-emerald-950/40 rounded-xl">
                <PlayCircleTwoTone twoToneColor="#10b981" className="text-2xl" />
              </div>
            </div>
            <div className="mt-3 text-xs text-slate-400 flex items-center border-t border-slate-50 dark:border-slate-800/40 pt-2">
              <span className="font-semibold text-emerald-500 mr-1">{stats.runningTasks > 0 ? '正在起呼' : '挂起就绪'}</span> 实时并发活动实例
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card
            variant="borderless"
            className="rounded-xl shadow-soft bg-white dark:bg-[#15181e] border border-slate-100 dark:border-slate-800/60 hover:shadow-md transition-all duration-300"
            styles={{ body: { padding: '20px' } }}
          >
            <div className="flex justify-between items-center">
              <div>
                <Typography.Text type="secondary" className="text-xs font-semibold uppercase tracking-wider block mb-1">累计试呼叫次数</Typography.Text>
                <div className="text-2xl font-bold dark:text-white">
                  {stats.totalCompleted.toLocaleString()} <span className="text-xs font-normal text-slate-400">次</span>
                </div>
              </div>
              <div className="p-3 bg-amber-50 dark:bg-amber-950/40 rounded-xl">
                <SignalFilled className="text-2xl text-amber-500" />
              </div>
            </div>
            <div className="mt-3 text-xs text-slate-400 flex items-center border-t border-slate-50 dark:border-slate-800/40 pt-2">
              <span className="font-semibold text-amber-500 mr-1">话务负荷</span> 通道已试拨总量
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card
            variant="borderless"
            className="rounded-xl shadow-soft bg-white dark:bg-[#15181e] border border-slate-100 dark:border-slate-800/60 hover:shadow-md transition-all duration-300"
            styles={{ body: { padding: '20px' } }}
          >
            <div className="flex justify-between items-center">
              <div>
                <Typography.Text type="secondary" className="text-xs font-semibold uppercase tracking-wider block mb-1">全局接通率</Typography.Text>
                <div className="text-2xl font-bold dark:text-white">
                  {stats.avgConnectRate} <span className="text-xs font-normal text-slate-400">%</span>
                </div>
              </div>
              <div className="p-3 bg-violet-50 dark:bg-violet-950/40 rounded-xl">
                <CheckCircleFilled className="text-2xl text-violet-500" />
              </div>
            </div>
            <div className="mt-3 text-xs text-slate-400 flex items-center border-t border-slate-50 dark:border-slate-800/40 pt-2">
              <span className="font-semibold text-violet-500 mr-1">话务效率</span> 客户应答占比平均值
            </div>
          </Card>
        </Col>
      </Row>

      <QueryBar
        fields={queryFields}
        onSearch={setQueryParams}
        loading={isLoading}
      />

      <Card
        variant="borderless"
        className="rounded-xl shadow-soft bg-white dark:bg-[#15181e] border border-slate-100 dark:border-slate-800/60"
        title={
          <div className="flex items-center gap-2">
            <span className="w-1 h-4 bg-blue-600 dark:bg-blue-500 rounded-full inline-block"></span>
            <span className="font-bold text-base text-slate-800 dark:text-slate-100">外呼活动任务列表</span>
          </div>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>刷新</Button>
            {selectedIds.length > 0 && (
              <PermissionGate permission="merchant:batch-task:write">
                <Popconfirm
                  title={`确定要删除选中的 ${selectedIds.length} 个任务吗？`}
                  onConfirm={() => deleteMutation.mutate(selectedIds)}
                  okText="确定"
                  cancelText="取消"
                >
                  <Button danger icon={<DeleteOutlined />}>批量删除</Button>
                </Popconfirm>
              </PermissionGate>
            )}
            <PermissionGate permission="merchant:batch-task:write">
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate} className="bg-blue-600 hover:bg-blue-700">
                新建任务
              </Button>
            </PermissionGate>
          </Space>
        }
      >
        <Table
          rowKey="id"
          loading={isLoading}
          dataSource={filteredRecords}
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
          scroll={{ x: 'max-content' }}
          columns={[
            {
              title: '任务 ID',
              dataIndex: 'id',
              width: 90,
              render: (v) => <span className="font-mono text-slate-400 dark:text-slate-500 text-xs">#{v}</span>
            },
            {
              title: '任务名称',
              dataIndex: 'name',
              render: (v, record) => (
                <Space size="middle">
                  <Avatar
                    size="small"
                    icon={<ThunderboltOutlined />}
                    className={record.status === 'running' ? 'bg-emerald-500 dark:bg-emerald-600 animate-pulse' : 'bg-slate-400 dark:bg-slate-700'}
                  />
                  <div>
                    <div className="font-semibold text-slate-700 dark:text-slate-200">{v}</div>
                    <div className="text-[11px] text-slate-450 dark:text-slate-500 mt-1 flex items-center gap-1">
                      <ClockCircleFilled className="text-[10px] text-slate-300 dark:text-slate-650" />
                      <span className="truncate max-w-[240px]">
                        时段: {formatTimePeriodSummary(record.callTimePeriod ?? '')}
                      </span>
                    </div>
                  </div>
                </Space>
              )
            },
            {
              title: '所属商户',
              dataIndex: 'merchant',
              render: (v) => <Tag color="blue" bordered={false} className="rounded-full px-2.5">{v}</Tag>
            },
            {
              title: '号码池规模',
              key: 'size',
              render: (_, record) => (
                <div>
                  <div className="text-xs text-slate-700 dark:text-slate-300 font-medium">
                    总号码: <span className="font-bold">{record.total}</span>
                  </div>
                  <div className="text-[11px] text-slate-400 dark:text-slate-500 mt-0.5">
                    已呼叫: <span className="font-mono font-medium">{record.completed}</span>
                  </div>
                </div>
              )
            },
            {
              title: '接通情况',
              key: 'connects',
              render: (_, record) => {
                const rate = record.completed > 0 ? Math.round((record.connected / record.completed) * 100) : 0
                return (
                  <div>
                    <div className="text-xs text-slate-700 dark:text-slate-300 font-medium">
                      接通数: <Typography.Text type="success" strong>{record.connected}</Typography.Text>
                    </div>
                    <div className="mt-1">
                      <Badge
                        status={rate > 40 ? 'success' : rate > 10 ? 'warning' : 'default'}
                        text={<span className="text-[11px] font-semibold text-slate-500 dark:text-slate-400">{rate}% 接通率</span>}
                      />
                    </div>
                  </div>
                )
              }
            },
            {
              title: '进度比例',
              key: 'progress',
              width: 150,
              render: (_, record) => {
                const pct = record.total > 0 ? Math.round((record.completed / record.total) * 100) : 0
                return (
                  <div className="w-full pr-4">
                    <Progress
                      percent={pct}
                      size="small"
                      strokeColor={record.status === 'running' ? {
                        '0%': '#10b981',
                        '100%': '#3b82f6',
                      } : '#94a3b8'}
                      status={record.status === 'running' && pct < 100 ? 'active' : 'normal'}
                    />
                  </div>
                )
              }
            },
            {
              title: '任务状态',
              dataIndex: 'status',
              width: 100,
              render: (value: string) => {
                if (value === 'running') {
                  return <Tag color="success" bordered={false} className="px-2.5 py-0.5 rounded-full font-medium">运行中</Tag>
                } else if (value === 'paused') {
                  return <Tag color="warning" bordered={false} className="px-2.5 py-0.5 rounded-full font-medium">已暂停</Tag>
                } else if (value === 'completed') {
                  return <Tag color="processing" bordered={false} className="px-2.5 py-0.5 rounded-full font-medium">已完成</Tag>
                }
                return <Tag color="default" bordered={false} className="px-2.5 py-0.5 rounded-full font-medium">已挂起</Tag>
              },
            },
            {
              title: '操作面板',
              key: 'actions',
              width: 200,
              render: (_, record) => {
                const isRunning = record.status === 'running'
                const dropdownItems = [
                  {
                    key: 'import',
                    label: '导入号码',
                    icon: <ImportOutlined className="text-slate-500" />,
                  },
                  {
                    key: 'edit',
                    label: '修改参数',
                    icon: <EditOutlined className="text-slate-500" />,
                  },
                  {
                    type: 'divider' as const,
                  },
                  {
                    key: 'delete',
                    label: '删除任务',
                    icon: <DeleteOutlined />,
                    danger: true,
                  }
                ]

                return (
                  <Space size="small">
                    <Button
                      type="text"
                      size="small"
                      icon={isRunning ? <PauseCircleOutlined className="text-amber-500" /> : <PlayCircleOutlined className="text-emerald-500" />}
                      onClick={() => toggleEnableMutation.mutate({ id: record.id, enable: !isRunning })}
                      className="font-medium text-xs !px-1.5"
                    >
                      {isRunning ? '暂停' : '启动'}
                    </Button>
                    <Button
                      type="primary"
                      size="small"
                      ghost
                      icon={<ProfileOutlined />}
                      onClick={() => setDetailTaskId(record.id)}
                      className="text-xs !px-2"
                    >
                      明细
                    </Button>
                    <Dropdown
                      menu={{
                        items: dropdownItems,
                        onClick: ({ key }) => {
                          if (key === 'import') setImportTaskId(record.id)
                          else if (key === 'edit') openEdit(record.id)
                          else if (key === 'delete') {
                            modal.confirm({
                              title: '删除外呼任务',
                              content: '您确定要删除该外呼活动任务吗？删除后将清理关联号码队列且不可恢复！',
                              okText: '确定',
                              cancelText: '取消',
                              okButtonProps: { danger: true },
                              onOk: () => deleteMutation.mutate([record.id])
                            })
                          }
                        }
                      }}
                      placement="bottomRight"
                      arrow
                    >
                      <Button size="small" type="text" icon={<DownOutlined />} className="text-slate-400 hover:text-slate-600" />
                    </Dropdown>
                  </Space>
                )
              },
            },
          ]}
        />
      </Card>

      <Modal
        title={editingId ? '编辑外呼任务' : '新建外呼任务'}
        open={open}
        width={960}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)} className="mt-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-1">
            <Form.Item
              name="name"
              label="任务名称"
              rules={[{ required: true, message: '请输入外呼任务名称' }]}
              className="col-span-1 md:col-span-2"
            >
              <Input placeholder="例如: 优质客户回访任务" />
            </Form.Item>

            <Form.Item
              name="departmentId"
              label="所属部门"
            >
              <Select
                placeholder="请选择所属部门"
                allowClear
                options={deptsData?.map(d => ({ label: d.name, value: d.id }))}
              />
            </Form.Item>

            <Form.Item
              name="skillGroupId"
              label="分配技能组"
            >
              <Select
                placeholder="请选择分配技能组"
                allowClear
                options={skillGroupsData?.records?.map(s => ({ label: s.name, value: s.id }))}
              />
            </Form.Item>

            <Form.Item
              name="callMode"
              label="呼叫模式"
              rules={[{ required: true, message: '请选择呼叫模式' }]}
              className="col-span-1 md:col-span-2"
            >
              <Radio.Group className="w-full flex gap-4">
                <Radio.Button value={1} className="flex-1 text-center py-1.5 h-auto">预测模式 (接听后分配坐席)</Radio.Button>
                <Radio.Button value={2} className="flex-1 text-center py-1.5 h-auto">协同模式 (振铃即分配坐席)</Radio.Button>
              </Radio.Group>
            </Form.Item>

            <Form.Item noStyle shouldUpdate={(prevValues, currentValues) => prevValues.callMode !== currentValues.callMode}>
              {({ getFieldValue }) => {
                const callMode = getFieldValue('callMode')
                if (callMode === 1) {
                  return (
                    <div className="col-span-1 md:col-span-2 grid grid-cols-1 md:grid-cols-2 gap-x-6 bg-slate-50 dark:bg-slate-900/50 p-4 rounded-xl mb-4 border border-slate-100 dark:border-slate-800">
                      <Form.Item
                        name="callRatio"
                        label="呼叫比例 (1:N)"
                        rules={[{ required: true, message: '请选择呼叫比例' }]}
                      >
                        <Select
                          options={[
                            { value: 1.0, label: '1:1.0 (保守)' },
                            { value: 1.2, label: '1:1.2' },
                            { value: 1.5, label: '1:1.5' },
                            { value: 2.0, label: '1:2.0' },
                            { value: 2.5, label: '1:2.5' },
                            { value: 3.0, label: '1:3.0 (进取)' },
                          ]}
                          dropdownRender={(menu) => (
                            <>
                              {menu}
                              <div className="p-2 border-t border-slate-100 dark:border-slate-800">
                                <InputNumber
                                  min={0.5}
                                  max={10}
                                  step={0.1}
                                  className="w-full"
                                  placeholder="自定义比例"
                                  value={form.getFieldValue('callRatio')}
                                  onChange={(val) => form.setFieldsValue({ callRatio: val || 1.5 })}
                                />
                              </div>
                            </>
                          )}
                        />
                      </Form.Item>
                      <Form.Item
                        name="queueEnable"
                        label="无空闲坐席时排队"
                        valuePropName="checked"
                      >
                        <Switch checkedChildren="排队播放等待音" unCheckedChildren="立即挂断" className="bg-slate-300 dark:bg-slate-700" />
                      </Form.Item>
                    </div>
                  )
                }
                return null
              }}
            </Form.Item>

            <Form.Item
              name="connectedInterval"
              label="接通重试间隔 (秒)"
              rules={[{ required: true, message: '请输入接通重试间隔' }]}
            >
              <InputNumber min={60} className="w-full" placeholder="默认 600 秒" />
            </Form.Item>

            <Form.Item
              name="unconnectedInterval"
              label="未接通重试间隔 (秒)"
              rules={[{ required: true, message: '请输入未接通重试间隔' }]}
            >
              <InputNumber min={60} className="w-full" placeholder="默认 1200 秒" />
            </Form.Item>

            <Form.Item
              name="callTimePeriod"
              className="col-span-1 md:col-span-2"
              noStyle
            >
              <TimePeriodSelector />
            </Form.Item>

            <Form.Item name="aiFlag" label="启用 AI 智能交互" valuePropName="checked" className="flex items-center">
              <Switch checkedChildren="开" unCheckedChildren="关" className="bg-slate-300 dark:bg-slate-700" />
            </Form.Item>

            <Form.Item name="enable" label="启用状态" valuePropName="checked" className="flex items-center">
              <Switch checkedChildren="启动" unCheckedChildren="挂起" className="bg-slate-300 dark:bg-slate-700" />
            </Form.Item>
          </div>
        </Form>
      </Modal>

      {/* Import Numbers Modal */}
      <Modal
        title="导入外呼号码"
        open={!!importTaskId}
        onCancel={() => {
          setImportTaskId(null)
          importForm.resetFields()
        }}
        onOk={() => importForm.submit()}
        confirmLoading={importTelsMutation.isPending}
        destroyOnHidden
      >
        <Form form={importForm} layout="vertical" onFinish={(values) => importTelsMutation.mutate(values)} className="mt-4">
          <Form.Item
            name="tels"
            label="号码清单"
            rules={[{ required: true, message: '请输入要导入的号码列表' }]}
            help="输入电话号码，每行一个或用逗号隔开。例: 13800000001,13800000002"
          >
            <Input.TextArea rows={8} placeholder="13800000001&#10;13900000002&#10;15911112222" className="font-mono text-sm bg-slate-50 dark:bg-slate-900 border-slate-200 dark:border-slate-800" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Dial Details Drawer */}
      <Drawer
        title={
          <div className="flex items-center gap-2">
            <span className="w-1 h-4 bg-blue-600 dark:bg-blue-500 rounded-full inline-block"></span>
            <span className="font-bold text-base text-slate-800 dark:text-slate-100">任务拨号及呼叫接通明细</span>
          </div>
        }
        placement="right"
        width={850}
        onClose={() => setDetailTaskId(null)}
        open={!!detailTaskId}
        destroyOnHidden
      >
        {detailsLoading ? (
          <div className="py-20 text-center text-slate-400">正在读取拨打明细...</div>
        ) : (
          <Space direction="vertical" size="large" className="w-full">
            <Card className="bg-slate-50/50 dark:bg-slate-900/40 border border-slate-100 dark:border-slate-800/80 shadow-soft rounded-xl p-2">
              <Row gutter={16}>
                <Col span={6}>
                  <Statistic title="分配总数" value={drawerStats.total} suffix="个" valueStyle={{ fontWeight: 'bold' }} />
                </Col>
                <Col span={6}>
                  <Statistic title="已拨打" value={drawerStats.called} suffix="次" valueStyle={{ fontWeight: 'bold' }} />
                </Col>
                <Col span={6}>
                  <Statistic
                    title="接通率"
                    value={drawerStats.rate}
                    suffix="%"
                    valueStyle={{ color: drawerStats.rate > 40 ? '#10b981' : '#f59e0b', fontWeight: 'bold' }}
                  />
                </Col>
                <Col span={6}>
                  <Statistic
                    title="优质通话 (>30s)"
                    value={drawerStats.gt30s}
                    suffix={`次 (${drawerStats.gt30sRate}%)`}
                    valueStyle={{ color: '#3b82f6', fontWeight: 'bold' }}
                  />
                </Col>
              </Row>
              <div className="mt-4 flex gap-4 text-xs border-t border-slate-100 dark:border-slate-800/60 pt-3">
                <Tag icon={<InfoCircleOutlined />} color="default" className="rounded-full px-2.5">待拨打: {drawerStats.active} 个</Tag>
                <Tag icon={<InfoCircleOutlined />} color="processing" className="rounded-full px-2.5">呼叫中: {drawerStats.calling} 个</Tag>
                <Tag icon={<CheckCircleOutlined />} color="success" className="rounded-full px-2.5">已接通: {drawerStats.connected} 个</Tag>
              </div>
            </Card>

            <Table
              dataSource={detailsData ?? []}
              rowKey="id"
              pagination={{ pageSize: 15 }}
              columns={[
                { title: '号码', dataIndex: 'tel', className: 'font-mono text-xs text-slate-700 dark:text-slate-300' },
                {
                  title: '拨打状态',
                  dataIndex: 'callStatus',
                  render: (value: number) => {
                    if (value === 2) return <Tag color="blue" bordered={false} className="rounded-full px-2.5">呼叫中</Tag>
                    if (value === 3) return <Tag color="green" bordered={false} className="rounded-full px-2.5">已拨打</Tag>
                    return <Tag color="default" bordered={false} className="rounded-full px-2.5">待拨打</Tag>
                  }
                },
                {
                  title: '是否接通',
                  dataIndex: 'connectStatus',
                  render: (value: boolean | null) => {
                    if (value === true) return <Tag color="success" bordered={false} className="rounded-full px-2.5">已接通</Tag>
                    if (value === false) return <Tag color="error" bordered={false} className="rounded-full px-2.5">未接通</Tag>
                    return <Tag color="default" bordered={false} className="rounded-full px-2.5">-</Tag>
                  }
                },
                {
                  title: '通话时长',
                  dataIndex: 'durationSec',
                  render: (value: number) => {
                    if (!value) return '-'
                    return (
                      <span className={value > 30 ? 'font-bold text-blue-600 dark:text-blue-400' : 'text-slate-600 dark:text-slate-400'}>
                        {value} 秒 {value > 30 && <BadgeText>优质</BadgeText>}
                      </span>
                    )
                  }
                },
                {
                  title: '呼叫时间',
                  dataIndex: 'callTime',
                  render: (value: string) => value ? value.replace('T', ' ').slice(0, 19) : '-'
                }
              ]}
            />
          </Space>
        )}
      </Drawer>
    </Space>
  )
}

function BadgeText({ children }: { children: React.ReactNode }) {
  return (
    <span className="ml-1 inline-flex items-center rounded-full bg-blue-100 dark:bg-blue-950 px-2 py-0.5 text-[10px] font-semibold text-blue-800 dark:text-blue-200">
      {children}
    </span>
  )
}
