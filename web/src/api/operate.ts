import { http } from './http'
import type { AiFlowItem, BatchTaskItem, CallRecordItem, DispatcherItem, GatewayItem, NodeItem, PoolItem, PoolPhoneItem, SkillGroupItem, StatItem } from '../types'

type PageResult<T> = {
  pageNumber: number
  pageSize: number
  total: number
  records: T[]
}

type GatewayResp = {
  id: number
  name: string
  description?: string
  channelId?: number
  enable?: boolean
  rateId?: number
  concurrency?: number
  priority?: number
  codecPrefs?: string
}

type MerchantResp = {
  id: number
  name?: string
  account?: string
  enable?: boolean
  expiredTime?: string
  rateId?: number
  whitelistDomains?: string
  sipDomain?: string
  appKey?: string
  appSecret?: string
  maxAgents?: number
}


type PoolResp = {
  id: number
  name?: string
  remark?: string
  type?: number
  gatewayId?: number
  enable?: boolean
  selectionStrategy?: string
}

type PoolPhoneResp = {
  id: number
  phone?: string
  poolId?: number
  province?: string
  city?: string
  concurrency?: number
  callLimit?: number
  enable?: boolean
}

type SkillGroupResp = {
  id: number
  name?: string
  merchantId?: number
  description?: string
  enable?: boolean
}

type DispatcherResp = {
  id: number
  setId?: number
  destination?: string
  description?: string
  priority?: number
  flags?: number
  enable?: boolean
}

type FsNodeResp = {
  id: number
  fsAddr?: string
  address?: string
  localAddress?: string
  eslPort?: number
  sipPort?: number
  cmdPort?: number
  weight?: number
  cc?: number
  enable?: boolean
  status?: string
  owner?: string
  activeCalls?: number
  updatedTime?: string
}

type BatchTaskResp = {
  id: number
  merchantId?: number
  userId?: number
  name?: string
  calledCount?: number
  totalCount?: number
  connectedCount?: number
  enable?: boolean
  pausedReason?: string
  state?: number
  updatedAt?: string
}

type CallRecordResp = {
  callId: string
  merchantId?: number
  userId?: number
  batchTaskId?: number
  caller?: string
  callee?: string
  durationSec?: number
  fsAddr?: string
  finalState?: string
  hangupCause?: string
  recordFilePath?: string
  completedAt?: string

  // 高级报表扩展字段
  billsec?: number
  ringsec?: number
  billingSec?: number
  gatewayName?: string
  extension?: string
  profile?: string
}

type AiFlowResp = {
  id: number
  name?: string
  published?: boolean
  prechecked?: boolean
  description?: string
  prompt?: string
  updatedAt?: string
}

export async function fetchGatewayPage(pageNumber = 1, pageSize = 50) {
  const { data } = await http.get<PageResult<GatewayResp>>('/operate/gateway', {
    params: { pageNumber, pageSize },
  })
  return {
    ...data,
    records: data.records.map<GatewayItem>((item) => ({
      id: item.id,
      name: item.name,
      code: item.description || `GW-${item.id}`,
      region: item.channelId ? `线路 ${item.channelId}` : '未分组',
      enable: Boolean(item.enable),
      syncRequired: false,
      activePools: item.rateId ?? 0,
      concurrency: item.concurrency,
      priority: item.priority,
      codecPrefs: item.codecPrefs,
    })),
  }
}

export async function saveGateway(payload: any) {
  const path = payload.id ? '/operate/gateway/update' : '/operate/gateway/add'
  const method = payload.id ? 'post' : 'put'
  const { data } = await http[method](path, payload)
  return data
}

export async function deleteGateways(ids: number[]) {
  const { data } = await http.post('/operate/gateway/delete', ids.map(id => ({ id })))
  return data
}

export async function syncGateway(id: number) {
  const { data } = await http.post(`/operate/gateway/sync/${id}`)
  return data
}

export async function fetchMerchants(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<MerchantResp>>('/operate/merchant', {
    params: { pageNumber, pageSize },
  })
  return {
    ...data,
    records: data.records.map((item) => ({
      id: item.id,
      name: item.name || `商户 ${item.id}`,
      account: item.account || '-',
      enable: Boolean(item.enable),
      expiredTime: item.expiredTime || '-',
      rateId: item.rateId ?? 0,
      whitelistDomains: item.whitelistDomains || '',
      sipDomain: item.sipDomain || '',
      appKey: item.appKey || '',
      appSecret: item.appSecret || '',
      maxAgents: item.maxAgents ?? 0,
    })),
  }
}

export async function fetchPools(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<PoolResp>>('/operate/pool', {
    params: { pageNumber, pageSize },
  })
  return {
    ...data,
    records: data.records.map<PoolItem>((item) => ({
      id: item.id,
      name: item.name || `号码池 ${item.id}`,
      remark: item.remark || '-',
      gateway: item.gatewayId ? `网关 ${item.gatewayId}` : '未绑定',
      gatewayId: item.gatewayId ?? 0,
      type: typeLabel(item.type),
      typeId: item.type ?? 1,
      enable: Boolean(item.enable),
      selectionStrategy: item.selectionStrategy || 'CONCURRENCY',
    })),
  }
}

export async function fetchPoolPhones(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<PoolPhoneResp>>('/operate/pool-phone', {
    params: { pageNumber, pageSize },
  })
  return {
    ...data,
    records: data.records.map<PoolPhoneItem>((item) => ({
      id: item.id,
      phone: item.phone || '-',
      pool: item.poolId ? `号码池 ${item.poolId}` : '未分配',
      poolId: item.poolId ?? 0,
      province: item.province || '-',
      city: item.city || '-',
      concurrency: item.concurrency ?? 0,
      callLimit: item.callLimit ?? 0,
      enable: Boolean(item.enable),
    })),
  }
}

export async function fetchSkillGroups(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<SkillGroupResp>>('/merchant/skill-group', {
    params: { pageNumber, pageSize },
  })
  return {
    ...data,
    records: data.records.map<SkillGroupItem>((item) => ({
      id: item.id,
      name: item.name || `技能组 ${item.id}`,
      merchant: item.merchantId ? `商户 ${item.merchantId}` : '未分配',
      merchantId: item.merchantId ?? 0,
      description: item.description || '-',
      enable: Boolean(item.enable),
    })),
  }
}

export async function reloadDispatchers() {
  const { data } = await http.post('/operate/kamailio/dispatcher/reload')
  return data
}

export async function toggleMerchantEnable(id: number, enable: boolean) {
  const path = enable ? `/operate/merchant/enable/${id}` : `/operate/merchant/disable/${id}`
  const { data } = await http.post(path)
  return data
}

export async function deleteMerchants(ids: number[]) {
  const { data } = await http.post('/operate/merchant/delete', ids.map((id) => ({ id })))
  return data
}

export async function deletePools(ids: number[]) {
  const { data } = await http.post('/operate/pool/delete', ids.map((id) => ({ id })))
  return data
}

export async function deletePoolPhones(ids: number[]) {
  const { data } = await http.post('/operate/pool-phone/delete', ids.map((id) => ({ id })))
  return data
}

export async function togglePoolPhoneEnable(id: number, enable: boolean) {
  const path = enable ? `/operate/pool-phone/enable/${id}` : `/operate/pool-phone/disable/${id}`
  const { data } = await http.post(path)
  return data
}

export async function deleteSkillGroups(ids: number[]) {
  const { data } = await http.post('/merchant/skill-group/delete', ids.map((id) => ({ id })))
  return data
}

export async function saveMerchant(payload: { id?: number; name: string; account: string; expiredTime?: string; enable: boolean; rateId?: number; whitelistDomains?: string; sipDomain?: string; appKey?: string; appSecret?: string; maxAgents?: number }) {
  const path = payload.id ? '/operate/merchant/update' : '/operate/merchant/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function getMerchantDetail(id: number, internal = true) {
  const url = internal ? `/operate/merchant/detail/${id}` : `/merchant/detail/${id}`
  const { data } = await http.get<MerchantResp>(url)
  return data
}

export async function saveDispatcher(payload: { id?: number; setId: number; destination: string; flags: number; priority: number; attrs?: string; description: string; enable: boolean }) {
  const path = payload.id ? '/operate/kamailio/dispatcher/update' : '/operate/kamailio/dispatcher/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deleteDispatchers(ids: number[]) {
  const { data } = await http.post('/operate/kamailio/dispatcher/delete', ids.map((id) => ({ id })))
  return data
}

export async function savePool(payload: { id?: number; name: string; remark?: string; type: number; gatewayId?: number; enable: boolean; selectionStrategy?: string }) {
  const path = payload.id ? '/operate/pool/update' : '/operate/pool/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function savePoolPhone(payload: { id?: number; poolId: number; phone: string; province?: string; city?: string; concurrency: number; callLimit: number; enable: boolean }) {
  const path = payload.id ? '/operate/pool-phone/update' : '/operate/pool-phone/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function saveSkillGroup(payload: { id?: number; name: string; merchantId: number; description?: string; enable: boolean }) {
  const path = payload.id ? '/merchant/skill-group/update' : '/merchant/skill-group/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function fetchDispatchers(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<DispatcherResp>>('/operate/kamailio/dispatcher', {
    params: { pageNumber, pageSize },
  })
  return {
    ...data,
    records: data.records.map<DispatcherItem>((item) => ({
      id: item.id,
      setId: item.setId ?? 0,
      destination: item.destination || `目的地 ${item.id}`,
      description: item.description || '-',
      priority: item.priority ?? 0,
      flags: item.flags ?? 0,
      enable: Boolean(item.enable),
    })),
  }
}

export async function fetchFsNodes() {
  const { data } = await http.get<FsNodeResp[]>('/operate/freeswitch/list')
  return data.map<NodeItem>((item) => ({
    id: item.id,
    name: item.fsAddr || item.address || `节点 ${item.id}`,
    fsAddr: item.fsAddr || item.address || '',
    address: item.address || '',
    localAddress: item.localAddress || '',
    eslPort: item.eslPort ?? 8021,
    sipPort: item.sipPort,
    cmdPort: item.cmdPort,
    weight: item.weight,
    enable: Boolean(item.enable),
    status: normalizeFsStatus(item.status),
    owner: item.owner || '-',
    activeCalls: item.activeCalls ?? 0,
    maxChannels: item.cc ?? 0,
    updatedAt: item.updatedTime || '',
  }))
}

export async function saveFsNode(payload: {
  id?: number
  address: string
  localAddress?: string
  eslPort: number
  sipPort?: number
  cmdPort?: number
  password?: string
  setId?: number
  weight?: number
  cc?: number
  enable: boolean
}) {
  const path = payload.id ? `/operate/freeswitch/${payload.id}` : '/operate/freeswitch'
  const { data } = await http.request({
    method: payload.id ? 'PUT' : 'POST',
    url: path,
    data: {
      ...payload,
      fsAddr: `${payload.address}:${payload.eslPort}`,
    },
  })
  return data
}

export async function deleteFsNode(id: number) {
  const { data } = await http.delete(`/operate/freeswitch/${id}`)
  return data
}

export async function toggleFsNodeEnable(id: number, enable: boolean) {
  const { data } = await http.post(`/operate/freeswitch/${id}/${enable ? 'enable' : 'disable'}`)
  return data
}


export async function fetchBatchTasks(pageNumber = 1, pageSize = 50) {
  const { data } = await http.get<PageResult<BatchTaskResp>>('/merchant/batch-call-task', {
    params: { pageNumber, pageSize },
  })
  return {
    ...data,
    records: data.records.map<BatchTaskItem>((item) => {
      const total = item.totalCount ?? 0
      const completed = item.calledCount ?? 0
      return {
        id: item.id,
        name: item.name || `任务 ${item.id}`,
        merchant: item.merchantId ? `商户 ${item.merchantId}` : '未分配',
        progress: total > 0 ? Math.min(100, Math.round((completed / total) * 100)) : 0,
        status: item.enable ? 'running' : item.state === 2 ? 'completed' : 'paused',
        total,
        completed,
        connected: item.connectedCount ?? Math.max(0, completed - Math.max(0, Math.floor(completed / 3))),
      }
    }),
  }
}

export async function fetchCallRecords(
  pageNumber = 1,
  pageSize = 50,
  filters: {
    callId?: string
    minDuration?: number
    gatewayId?: string
    profile?: string
    extension?: string
    startTime?: string
    endTime?: string
    merchantId?: number
  } = {}
) {
  const { data } = await http.get<PageResult<CallRecordResp>>('/merchant/call-record', {
    params: { pageNumber, pageSize, ...filters },
  })
  return {
    ...data,
    records: data.records.map<CallRecordItem>((item) => ({
      id: Number(item.callId?.replace(/\D/g, '') || item.callId || item.batchTaskId || 0),
      callId: item.callId,
      merchant: item.merchantId ? `商户 ${item.merchantId}` : '未分配',
      callee: item.callee || '-',
      caller: item.caller || '-',
      fsAddr: item.fsAddr || '',
      state: item.finalState || item.hangupCause || 'unknown',
      duration: item.durationSec !== undefined ? `${item.durationSec} 秒` : '-',
      finishedAt: item.completedAt || '',
      // 高级通话统计与分析报表字段映射
      billsec: item.billsec ?? 0,
      ringsec: item.ringsec ?? 0,
      billingSec: item.billingSec ?? 0,
      gatewayName: item.gatewayName || item.fsAddr || '默认网关',
      extension: item.extension || '',
      userId: item.userId ?? 0,
      profile: item.profile || '',
      recordFilePath: item.recordFilePath,
    })),
  }
}

export async function fetchAiFlows(pageNumber = 1, pageSize = 50) {
  const { data } = await http.post<PageResult<AiFlowResp>>('/merchant/ai-model-flow/page', {
    pageNumber,
    pageSize,
  })
  return {
    ...data,
    records: data.records.map<AiFlowItem>((item) => ({
      id: item.id,
      name: item.name || `流程 ${item.id}`,
      merchant: '当前商户',
      version: item.updatedAt ? item.updatedAt.slice(0, 10) : 'v1',
      status: item.published ? 'published' : item.prechecked ? 'draft' : 'disabled',
      updatedAt: item.updatedAt || '',
      prompt: item.prompt || '',
      description: item.description || '',
    })),
  }
}

export function buildDashboardStats({
  gateways,
  nodes,
  batchTasks,
  callRecords,
}: {
  gateways: PageResult<GatewayItem> | null
  nodes: NodeItem[] | null
  batchTasks: PageResult<BatchTaskItem> | null
  callRecords: PageResult<CallRecordItem> | null
}): StatItem[] {
  const records = callRecords?.records ?? []
  const callTotal = records.length
  let answered = 0
  let busy = 0
  let failed = 0
  records.forEach((r) => {
    const s = String(r.state).toUpperCase()
    if (s.includes('ANSWER') || s.includes('SUCCESS') || s === 'SUCCESS' || s === 'TALKING') {
      answered++
    } else if (s.includes('BUSY')) {
      busy++
    } else {
      failed++
    }
  })

  const answeredRate = callTotal > 0 ? Math.round((answered / callTotal) * 100) : 0
  const failedRate = callTotal > 0 ? Math.round((failed / callTotal) * 100) : 0
  const activeChannels = nodes?.reduce((sum, item) => sum + item.activeCalls, 0) ?? 0
  const maxChannels = nodes?.reduce((sum, item) => sum + item.maxChannels, 0) ?? 0

  return [
    { label: '今日外呼', value: `${callTotal} 次`, trend: `接通数: ${answered} 次`, tone: 'success' },
    { label: '接通率', value: `${answeredRate}%`, trend: `忙线数: ${busy} 次`, tone: 'processing' },
    { label: '失败率', value: `${failedRate}%`, trend: `失败数: ${failed} 次`, tone: 'warning' },
    { label: '活跃通道', value: `${activeChannels} / ${maxChannels}`, trend: nodes?.length ? `节点数: ${nodes.length} 个` : '暂无节点数据', tone: 'default' },
  ]
}

function normalizeFsStatus(status?: string): NodeItem['status'] {
  if (status === 'active') {
    return 'active'
  }
  if (status === 'draining') {
    return 'draining'
  }
  return 'unavailable'
}

function typeLabel(type?: number) {
  switch (type) {
    case 1:
      return '普通'
    case 2:
      return '预测'
    case 3:
      return '外呼'
    default:
      return type ? `类型 ${type}` : '-'
  }
}

// ----------------------------------------------------
// 以下为补全的运营/商户管理端 API 接口定义
// ----------------------------------------------------

export async function fetchAccounts(pageNumber = 1, pageSize = 20, username = '', accountType = '') {
  const { data } = await http.get<PageResult<any>>('/operate/account', {
    params: { pageNumber, pageSize, username, accountType },
  })
  return data
}

export async function saveAccount(payload: { id?: number; username: string; password?: string; merchantId?: string; roleId?: string; accountType: string; enable: boolean }) {
  const path = payload.id ? '/operate/account/update' : '/operate/account/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deleteAccounts(ids: number[]) {
  const { data } = await http.post('/operate/account/delete', ids.map((id) => ({ id })))
  return data
}

export async function toggleAccountEnable(id: number, enable: boolean) {
  const path = enable ? `/operate/account/enable/${id}` : `/operate/account/disable/${id}`
  const { data } = await http.post(path)
  return data
}

export async function resetAccountPassword(id: number, password: string) {
  const { data } = await http.post(`/operate/account/reset-password/${id}`, { password })
  return data
}

export async function fetchRates(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<any>>('/operate/rate', {
    params: { pageNumber, pageSize },
  })
  return data
}

export async function saveRate(payload: { id?: number; rateName: string; billingPrice: number; billingCycle: number; remark?: string }) {
  const path = payload.id ? '/operate/rate/update' : '/operate/rate/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deleteRates(ids: number[]) {
  const { data } = await http.post('/operate/rate/delete', ids.map((id) => ({ id })))
  return data
}

export async function fetchBlacklist(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<any>>('/operate/blacklist', {
    params: { pageNumber, pageSize },
  })
  return data
}

export async function saveBlacklist(payload: { id?: number; name: string; verificationChannel: number; remark?: string }) {
  const path = payload.id ? '/operate/blacklist/update' : '/operate/blacklist/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deleteBlacklist(id: number) {
  const { data } = await http.post(`/operate/blacklist/delete/${id}`)
  return data
}

export async function fetchBlacklistNumbers(payload: { pageNumber: number; pageSize: number; phone?: string; blackLevel?: string }) {
  const { data } = await http.post<PageResult<any>>('/operate/blacklist/numbers/page', payload)
  return data
}

export async function saveBlacklistNumber(payload: { phone: string; blackLevel: string; remark?: string }) {
  const { data } = await http.post('/operate/blacklist/numbers/save', payload)
  return data
}

export async function deleteBlacklistNumbers(phones: string[]) {
  const { data } = await http.post('/operate/blacklist/numbers/delete', { phones })
  return data
}

export async function fetchBlacklistChannels() {
  const { data } = await http.get<any[]>('/operate/blacklist/channels')
  return data
}

export async function saveBlacklistChannel(payload: { code: number; name: string; vendor: string; remark?: string; enable: boolean }) {
  const { data } = await http.post('/operate/blacklist/channels/save', payload)
  return data
}

export async function deleteBlacklistChannel(code: number) {
  const { data } = await http.delete(`/operate/blacklist/channels/${code}`)
  return data
}

export async function fetchWhitelist(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<any>>('/operate/whitelist', {
    params: { pageNumber, pageSize },
  })
  return data
}

export async function saveWhitelist(payload: { id?: number; phone: string; numberType: number }) {
  const path = payload.id ? '/operate/whitelist/update' : '/operate/whitelist/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deleteWhitelist(ids: number[]) {
  const { data } = await http.post('/operate/whitelist/delete', ids.map((id) => ({ id })))
  return data
}

export async function fetchBillingOverview(pageNumber = 1, pageSize = 20, merchant = '') {
  const { data } = await http.post<PageResult<any>>('/operate/billing/overview/page', { pageNumber, pageSize, merchant })
  return data
}

export async function saveBillingOverview(payload: { id?: number; merchantId: string; paymentMode: number; creditLimit: number }) {
  const { data } = await http.post('/operate/billing/overview/save', payload)
  return data
}

export async function rechargeMerchant(payload: { merchantId: string; amount: number; remark?: string }) {
  const { data } = await http.post('/operate/billing/recharge', payload)
  return data
}

export async function fetchRechargeRecords(pageNumber = 1, pageSize = 20, merchant = '') {
  const { data } = await http.post<PageResult<any>>('/operate/billing/recharge-records', { pageNumber, pageSize, merchant })
  return data
}

export async function fetchChannels(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<any>>('/operate/channel', {
    params: { pageNumber, pageSize },
  })
  return data
}

export async function saveChannel(payload: { id?: number; name: string; config?: any; blindArea?: string; enable: boolean }) {
  const path = payload.id ? '/operate/channel/update' : '/operate/channel/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deleteChannels(ids: number[]) {
  const { data } = await http.post('/operate/channel/delete', ids.map((id) => ({ id })))
  return data
}

export async function fetchPhoneAttributions(pageNumber = 1, pageSize = 20, areaCode = '', provCode = '', cityCode = '') {
  const { data } = await http.post<PageResult<any>>('/operate/phone-attribution/page', {
    pageNumber,
    pageSize,
    areaCode,
    provCode,
    cityCode,
  })
  return data
}

export async function savePhoneAttribution(payload: { areaCode: string; provCode: string; cityCode: string; isEdit?: boolean }) {
  const path = payload.isEdit ? '/operate/phone-attribution/update' : '/operate/phone-attribution/add'
  const { data } = await http.request({
    method: payload.isEdit ? 'POST' : 'PUT',
    url: path,
    data: {
      areaCode: payload.areaCode,
      provCode: payload.provCode,
      cityCode: payload.cityCode,
    },
  })
  return data
}

export async function deletePhoneAttributions(areaCodes: string[]) {
  const { data } = await http.post('/operate/phone-attribution/delete', areaCodes)
  return data
}

export async function lookupPhoneAttribution(phone: string): Promise<{ areaCode: string; provCode: string; cityCode: string }> {
  const { data } = await http.get('/operate/phone-attribution/lookup', {
    params: { phone },
  })
  return data
}

export async function fetchExtensions(pageNumber = 1, pageSize = 20) {
  const { data } = await http.get<PageResult<any>>('/operate/extension', {
    params: { pageNumber, pageSize },
  })
  return data
}

export async function saveExtension(payload: { id?: number; extensionNumber: string; password?: string; merchantId: string; userId?: string; enable: boolean; bindType?: number }) {
  const path = payload.id ? '/operate/extension/update' : '/operate/extension/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deleteExtensions(ids: number[]) {
  const { data } = await http.post('/operate/extension/delete', ids.map((id) => ({ id })))
  return data
}

export async function toggleExtensionEnable(id: number, enable: boolean) {
  const path = enable ? `/operate/extension/enable/${id}` : `/operate/extension/disable/${id}`
  const { data } = await http.post(path)
  return data
}

// ----------------------------------------------------
// 号码组及资源绑定 API 接口
// ----------------------------------------------------

export async function fetchPhoneGroups(pageNumber = 1, pageSize = 20, name = '', merchantId?: number) {
  const { data } = await http.get<PageResult<any>>('/merchant/phone-group', {
    params: { pageNumber, pageSize, name, merchantId },
  })
  return data
}

export async function savePhoneGroup(payload: { id?: number; name: string; remark?: string; desc?: string; merchantId: number; enable: boolean }) {
  const path = payload.id ? '/merchant/phone-group/update' : '/merchant/phone-group/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deletePhoneGroups(ids: number[]) {
  const { data } = await http.post('/merchant/phone-group/delete', ids.map((id) => ({ id })))
  return data
}

export async function fetchPhoneGroupPhones(id: number) {
  const { data } = await http.get<{ phoneIds: number[] }>(`/merchant/phone-group/phones/${id}`)
  return data
}

export async function savePhoneGroupPhones(id: number, merchantId: number, phoneIds: number[]) {
  const { data } = await http.post(`/merchant/phone-group/phones/${id}`, { merchantId, phoneIds })
  return data
}

export async function fetchPhoneGroupSkillGroups(id: number) {
  const { data } = await http.get<{ skillGroupIds: number[] }>(`/merchant/phone-group/skill-groups/${id}`)
  return data
}

export async function savePhoneGroupSkillGroups(id: number, merchantId: number, skillGroupIds: number[]) {
  const { data } = await http.post(`/merchant/phone-group/skill-groups/${id}`, { merchantId, skillGroupIds })
  return data
}

export async function fetchMerchantAccounts(pageNumber = 1, pageSize = 20, username = '') {
  const { data } = await http.get<PageResult<any>>('/merchant/account', {
    params: { pageNumber, pageSize, username },
  })
  return data
}

export async function saveMerchantAccount(payload: { id?: number; username: string; password?: string; merchantId?: string; roleId?: string; accountType: string; enable: boolean }) {
  const path = payload.id ? '/merchant/account/update' : '/merchant/account/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deleteMerchantAccounts(ids: number[]) {
  const { data } = await http.post('/merchant/account/delete', ids.map((id) => ({ id })))
  return data
}

export async function toggleMerchantAccountEnable(id: number, enable: boolean) {
  const path = enable ? `/merchant/account/enable/${id}` : `/merchant/account/disable/${id}`
  const { data } = await http.post(path)
  return data
}

export async function resetMerchantAccountPassword(id: number, password: string) {
  const { data } = await http.post(`/merchant/account/reset-password/${id}`, { password })
  return data
}


export async function fetchSkillGroupUsers(id: number) {
  const { data } = await http.get<{ userIds: number[] }>(`/merchant/skill-group/users/${id}`)
  return data
}

export async function saveSkillGroupUsers(id: number, userIds: number[]) {
  const { data } = await http.post(`/merchant/skill-group/users/${id}`, { userIds })
  return data
}

export async function fetchSkillGroupPhones(id: number) {
  const { data } = await http.get<{ phoneIds: number[] }>(`/merchant/skill-group/phones/${id}`)
  return data
}

export async function saveSkillGroupPhones(id: number, phoneIds: number[]) {
  const { data } = await http.post(`/merchant/skill-group/phones/${id}`, { phoneIds })
  return data
}

export async function fetchRoles(pageNumber = 1, pageSize = 20, name = '') {
  const { data } = await http.post<PageResult<any>>('/operate/role/page', { pageNumber, pageSize, name })
  return data
}

export async function saveRole(payload: { code: string; name: string; description?: string; enable: boolean }, isEdit: boolean) {
  const path = isEdit ? '/operate/role/update' : '/operate/role/add'
  const method = isEdit ? 'POST' : 'PUT'
  const { data } = await http.request({
    method,
    url: path,
    data: payload,
  })
  return data
}

export async function deleteRoles(roles: { code: string }[]) {
  const { data } = await http.post('/operate/role/delete', roles)
  return data
}

export async function toggleRoleEnable(code: string, enable: boolean) {
  const path = enable ? `/operate/role/enable/${code}` : `/operate/role/disable/${code}`
  const { data } = await http.post(path)
  return data
}

export async function fetchPermissions() {
  const { data } = await http.get<any[]>('/operate/permission')
  return data
}

export async function fetchRolePermissions(code: string) {
  const { data } = await http.get<string[]>(`/operate/role/permissions/${code}`)
  return data
}

export async function saveRolePermissions(roleCode: string, permissionCodes: string[]) {
  const { data } = await http.post('/operate/role/permissions/save', { roleCode, permissionCodes })
  return data
}

export async function bindMerchantRate(rateId: number) {
  const { data } = await http.post('/merchant/billing/rate/bind', { rateId })
  return data
}

export async function fetchActiveRates() {
  const { data } = await http.get<any[]>('/operate/rate/list-active')
  return data
}

export async function saveBatchTask(payload: any) {
  const path = payload.id ? '/merchant/batch-call-task/update' : '/merchant/batch-call-task/add'
  const method = payload.id ? 'post' : 'put'
  const { data } = await http[method](path, payload)
  return data
}

export async function deleteBatchTasks(ids: number[]) {
  const { data } = await http.post('/merchant/batch-call-task/delete', ids.map(id => ({ id })))
  return data
}

export async function toggleBatchTaskEnable(id: number, enable: boolean, reason?: string) {
  const path = enable ? `/merchant/batch-call-task/enable/${id}` : `/merchant/batch-call-task/disable/${id}`
  const url = reason ? `${path}?reason=${encodeURIComponent(reason)}` : path
  const { data } = await http.post(url)
  return data
}

export async function startBatchDialpad(id: number) {
  const { data } = await http.post(`/merchant/batch-call-dialpad/start/${id}`)
  return data
}

export async function pauseBatchDialpad(id: number, reason?: string) {
  const path = `/merchant/batch-call-dialpad/pause/${id}`
  const url = reason ? `${path}?reason=${encodeURIComponent(reason)}` : path
  const { data } = await http.post(url)
  return data
}

export async function resumeBatchDialpad(id: number) {
  const { data } = await http.post(`/merchant/batch-call-dialpad/resume/${id}`)
  return data
}

export async function disconnectPauseBatchDialpad(id: number, reason?: string) {
  const path = `/merchant/batch-call-dialpad/disconnect-pause/${id}`
  const url = reason ? `${path}?reason=${encodeURIComponent(reason)}` : path
  const { data } = await http.post(url)
  return data
}

export async function saveAiFlow(payload: any) {
  const path = payload.id ? '/merchant/ai-model-flow/update' : '/merchant/ai-model-flow/add'
  const method = payload.id ? 'post' : 'put'
  const { data } = await http[method](path, payload)
  return data
}

export async function deleteAiFlows(ids: number[]) {
  const { data } = await http.post('/merchant/ai-model-flow/delete', ids.map(id => ({ id })))
  return data
}

export async function precheckAiFlow(payload: any) {
  const { data } = await http.post('/merchant/ai-model-flow/precheck', payload)
  return data
}

export async function publishAiFlow(id: number) {
  const { data } = await http.post(`/merchant/ai-model-flow/publish/${id}`)
  return data
}

export async function importBatchTaskTels(id: number, merchantId: number, userId: number, tels: string[]) {
  const { data } = await http.post(`/merchant/batch-call-task/import/${id}`, { merchantId, userId, tels })
  return data
}

export async function fetchBatchTaskDetails(id: number) {
  const { data } = await http.get<any[]>(`/merchant/batch-call-task/details/${id}`)
  return data
}

export async function dynamicBindExtension(payload: { extensionNumber: string; userId: number; merchantId: number }, role: 'operate' | 'merchant' = 'operate') {
  const path = `/${role}/extension/dynamic-bind`
  const { data } = await http.post(path, payload)
  return data
}

// ----------------------------------------------------
// 风控策略 API 接口
// ----------------------------------------------------

export async function fetchRiskControls(pageNumber = 1, pageSize = 20, name = '') {
  const { data } = await http.post<PageResult<any>>('/operate/risk-control/page', {
    pageNumber,
    pageSize,
    name,
  })
  return data
}

export async function saveRiskControl(payload: any) {
  const path = payload.id ? '/operate/risk-control/update' : '/operate/risk-control/add'
  const { data } = await http.request({
    method: payload.id ? 'POST' : 'PUT',
    url: path,
    data: payload,
  })
  return data
}

export async function deleteRiskControls(ids: number[]) {
  const { data } = await http.post('/operate/risk-control/delete', ids.map(id => ({ id })))
  return data
}

export async function fetchRiskControlMerchants(id: number) {
  const { data } = await http.get<any[]>(`/operate/risk-control/merchants/${id}`)
  return data
}

export async function saveRiskControlMerchants(id: number, bindings: any[]) {
  const { data } = await http.post(`/operate/risk-control/merchants/${id}`, bindings)
  return data
}

// ----------------------------------------------------
// 系统安装与初始化部署 API 接口
// ----------------------------------------------------

export async function fetchInstallStatus() {
  const { data } = await http.get('/api/install/status')
  return data
}

export async function saveInstallSetup(payload: any) {
  const { data } = await http.post('/api/install/setup', payload)
  return data
}

export async function triggerInstallDeploy() {
  const { data } = await http.post('/api/install/deploy')
  return data
}

export async function fetchInstallDeployStatus() {
  const { data } = await http.get('/api/install/deploy/status')
  return data
}

export async function startInstallServices(payload: any) {
  const { data } = await http.post('/api/install/services/start', payload)
  return data
}

// ----------------------------------------------------
// 系统代理与网络动态配置 API 接口
// ----------------------------------------------------

export async function fetchProxyConfig() {
  const { data } = await http.get('/operate/proxy-config')
  return data
}

export async function saveProxyConfig(payload: any) {
  const { data } = await http.post('/operate/proxy-config/save', payload)
  return data
}

export async function applyProxyConfig() {
  const { data } = await http.post('/operate/proxy-config/apply')
  return data
}

export async function reloadRtpengineConfig() {
  const { data } = await http.post('/operate/proxy-config/reload-rtp')
  return data
}

export async function fetchAreaCodes() {
  const { data } = await http.get<any[]>('/operate/area-code/list')
  return data
}

// 多 RTPEngine 媒体代理节点 API
// ----------------------------------------------------

export type Rtpengine = {
  id?: number
  setId: number
  rtpengineSock: string
  disabled: boolean
  weight: number
  description: string
}

export type RtpenginePageRequest = {
  pageNumber: number
  pageSize: number
  setId?: number
  rtpengineSock?: string
  disabled?: boolean
}

export type RtpenginePageResult = {
  pageNumber: number
  pageSize: number
  total: number
  records: Rtpengine[]
}

export async function fetchRtpenginesPage(payload: RtpenginePageRequest) {
  const { data } = await http.post<RtpenginePageResult>('/operate/kamailio/rtpengine/page', payload)
  return data
}

export async function saveRtpengine(payload: Rtpengine) {
  const path = payload.id ? '/operate/kamailio/rtpengine/update' : '/operate/kamailio/rtpengine/add'
  const method = payload.id ? 'post' : 'put'
  const { data } = await http[method](path, payload)
  return data
}

export async function deleteRtpengines(payload: { id: number }[]) {
  const { data } = await http.post('/operate/kamailio/rtpengine/delete', payload)
  return data
}

export async function reloadRtpengines() {
  const { data } = await http.post('/operate/kamailio/rtpengine/reload')
  return data
}

