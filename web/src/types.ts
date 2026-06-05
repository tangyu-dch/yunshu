export type StatusTone = 'success' | 'warning' | 'error' | 'default' | 'processing'

export type StatItem = {
  label: string
  value: string
  trend?: string
  tone?: StatusTone
}

export type NodeItem = {
  id: number
  name: string
  fsAddr: string
  address: string
  localAddress?: string
  eslPort: number
  sipPort?: number
  cmdPort?: number
  setId?: number
  weight?: number
  enable: boolean
  status: 'active' | 'draining' | 'unavailable'
  owner: string
  activeCalls: number
  maxChannels: number
  updatedAt: string
}

export type GatewayItem = {
  id: number
  name: string
  code: string
  region: string
  enable: boolean
  syncRequired: boolean
  activePools: number
  concurrency?: number
  priority?: number
  codecPrefs?: string
  channelId?: number
  rateId?: number
  realm?: string
  port?: string
  username?: string
  remark?: string
}

export type BatchTaskItem = {
  id: number
  name: string
  merchant: string
  progress: number
  status: 'running' | 'paused' | 'completed'
  total: number
  completed: number
  connected: number
  connectedInterval?: number
  unconnectedInterval?: number
  callTimePeriod?: string
  aiFlag?: boolean
  skillGroupId?: number
  departmentId?: number
  callMode?: number
  callRatio?: number
  queueEnable?: boolean
}

export type CallRecordItem = {
  id: number
  callId: string
  merchant: string
  callee: string
  caller: string
  fsAddr: string
  state: string
  duration: string
  finishedAt: string
  // 新增高级通话统计字段
  billsec: number
  ringsec: number
  billingSec: number
  gatewayName: string
  extension: string
  userId: number
  profile: string
  recordFilePath?: string
  hangupCause?: string
  sipHangupDisposition?: string
}

export type AiFlowItem = {
  id: number
  name: string
  merchant: string
  version: string
  status: 'draft' | 'published' | 'disabled'
  updatedAt: string
  prompt?: string
  description?: string
  flowGraph?: any
}

export type PoolItem = {
  id: number
  merchantId?: number
  name: string
  remark: string
  gateway: string
  type: string
  gatewayId: number
  typeId: number
  enable: boolean
  selectionStrategy?: string
}

export type PoolPhoneItem = {
  id: number
  phone: string
  pool: string
  poolId: number
  province: string
  city: string
  concurrency: number
  callLimit: number
  enable: boolean
}

export type SkillGroupItem = {
  id: number
  name: string
  merchant: string
  merchantId: number
  description: string
  enable: boolean
}

export type DispatcherItem = {
  id: number
  setId: number
  destination: string
  description: string
  priority: number
  flags: number
  enable: boolean
  attrs?: string
}

export type SipTraceItem = {
  id: number
  timestamp: string
  timeUs: number
  method: string
  status: string
  fromIp: string
  toIp: string
  direction: string
  rawMsg: string
}

export type CallSipTraceResult = {
  callId: string
  nodes: string[]
  trace: SipTraceItem[]
}

export interface IPBlockConfig {
  countries: string
  onlyAllowCn: boolean
}

export interface IPBlockLog {
  id: number
  ip: string
  countryCode: string
  callId: string
  method: string
  blockedAt: string
}

export interface IPBlockLogQuery {
  pageNumber: number
  pageSize: number
  ip?: string
  countryCode?: string
  startTime?: string
  endTime?: string
}

