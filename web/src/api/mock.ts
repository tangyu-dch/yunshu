import type { AiFlowItem, BatchTaskItem, CallRecordItem, GatewayItem, NodeItem, StatItem } from '@/types'

export const overviewStats: StatItem[] = [
  { label: '今日外呼', value: '12,480', trend: '+12.8%', tone: 'success' },
  { label: '接通率', value: '38.6%', trend: '+2.1%', tone: 'processing' },
  { label: '失败率', value: '2.4%', trend: '-0.6%', tone: 'success' },
  { label: '活跃通道', value: '86 / 120', trend: '运行稳定', tone: 'warning' },
]

export const fsNodes: NodeItem[] = [
  { id: 1, name: '华东主节点', fsAddr: '10.0.10.11:8021', address: '10.0.10.11', eslPort: 8021, enable: true, status: 'active', owner: 'cc-call-1', activeCalls: 38, maxChannels: 120, updatedAt: '2026-05-23 09:40:12' },
  { id: 2, name: '华北备用节点', fsAddr: '10.0.20.21:8021', address: '10.0.20.21', eslPort: 8021, enable: true, status: 'draining', owner: 'cc-call-2', activeCalls: 12, maxChannels: 100, updatedAt: '2026-05-23 09:36:45' },
  { id: 3, name: '华南空闲节点', fsAddr: '10.0.30.31:8021', address: '10.0.30.31', eslPort: 8021, enable: false, status: 'unavailable', owner: '-', activeCalls: 0, maxChannels: 80, updatedAt: '2026-05-23 09:28:03' },
]

export const gateways: GatewayItem[] = [
  { id: 1, name: '电信主线路', code: 'GW-TELECOM-01', region: '华东', enable: true, syncRequired: false, activePools: 4 },
  { id: 2, name: '联通备用线路', code: 'GW-UNICOM-02', region: '华北', enable: true, syncRequired: true, activePools: 2 },
  { id: 3, name: '移动低成本线路', code: 'GW-MOBILE-03', region: '华南', enable: false, syncRequired: true, activePools: 1 },
]

export const batchTasks: BatchTaskItem[] = [
  { id: 1001, name: '5月促销回访', merchant: '星海科技', progress: 68, status: 'running', total: 5000, completed: 3400, connected: 1260 },
  { id: 1002, name: '沉默用户唤醒', merchant: '蓝湾教育', progress: 100, status: 'completed', total: 2800, completed: 2800, connected: 910 },
  { id: 1003, name: '售后满意度', merchant: '云舟物流', progress: 24, status: 'paused', total: 1200, completed: 290, connected: 94 },
]

export const callRecords: CallRecordItem[] = [
  { id: 1, callId: 'call-20260523-001', merchant: '星海科技', callee: '138****0001', caller: '02180010001', fsAddr: '10.0.10.11:8021', state: 'completed', duration: '00:03:18', finishedAt: '2026-05-23 09:42:11', billsec: 198, ringsec: 12, billingSec: 198, gatewayName: '电信主网关', extension: '1001', userId: 7, profile: 'api_outbound' },
  { id: 2, callId: 'call-20260523-002', merchant: '蓝湾教育', callee: '139****0007', caller: '01080020002', fsAddr: '10.0.20.21:8021', state: 'busy', duration: '00:00:16', finishedAt: '2026-05-23 09:40:42', billsec: 0, ringsec: 16, billingSec: 0, gatewayName: '联通备用网关', extension: '1002', userId: 8, profile: 'batch_outbound' },
  { id: 3, callId: 'call-20260523-003', merchant: '云舟物流', callee: '137****0019', caller: '07558003003', fsAddr: '10.0.10.11:8021', state: 'failed', duration: '00:00:04', finishedAt: '2026-05-23 09:39:31', billsec: 0, ringsec: 4, billingSec: 0, gatewayName: '电信主网关', extension: '1003', userId: 9, profile: 'api_direct' },
]

export const aiFlows: AiFlowItem[] = [
  { id: 11, name: '外呼质检初筛', merchant: '星海科技', version: 'v3.2', status: 'published', updatedAt: '2026-05-22 18:24:10' },
  { id: 12, name: '通话摘要生成', merchant: '蓝湾教育', version: 'v2.1', status: 'draft', updatedAt: '2026-05-23 08:58:44' },
  { id: 13, name: '客户意向分层', merchant: '云舟物流', version: 'v1.9', status: 'disabled', updatedAt: '2026-05-21 14:12:02' },
]

export const realtimeCalls = [
  { callId: 'call-20260523-011', merchant: '星海科技', node: '10.0.10.11:8021', leg: 'agent/customer', status: 'bridge', elapsed: '00:01:42' },
  { callId: 'call-20260523-012', merchant: '蓝湾教育', node: '10.0.20.21:8021', leg: 'customer', status: 'ringing', elapsed: '00:00:24' },
  { callId: 'call-20260523-013', merchant: '云舟物流', node: '10.0.10.11:8021', leg: 'agent', status: 'answer', elapsed: '00:00:58' },
]
