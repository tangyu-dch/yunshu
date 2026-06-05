import { Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Switch, Tag, Typography, Tabs, Card, Table, Row, Col, Alert, Empty, Tooltip, message, Steps, TreeSelect, Cascader } from 'antd'
import { ApartmentOutlined, SafetyCertificateOutlined, SettingOutlined, PlusOutlined, DeleteOutlined, EditOutlined, BranchesOutlined, CheckCircleOutlined, InfoCircleOutlined, InteractionOutlined, ArrowRightOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { useAuthStore } from '@/store/auth'
import {
  fetchPools,
  savePool,
  fetchGatewayPage,
  fetchChannels,
  fetchSkillGroups,
  fetchMerchants,
  fetchRiskControls,
  saveRiskControl,
  deleteRiskControls,
  fetchRiskControlMerchants,
  saveRiskControlMerchants,
  fetchAreaCodes,
  fetchProxyConfig,
  saveProxyConfig
} from '@/api/operate'

const { TabPane } = Tabs
const { Option } = Select

type NearbyCityConfig = {
  cityCode: string
  nearbyCodes: string[]
}

type PoolStrategyFormValues = {
  id: number
  selectionStrategy: string
  gatewayId?: number
  name: string
  remark?: string
  type: number
  enable: boolean
}

type FrequencyRule = {
  day: number
  count: number
  type: 'DIAL' | 'CONNECTED' | string
}

type RiskControlFormValues = {
  id?: number
  name: string
  remark?: string
  blackLevelFlag: boolean
  blackLevel?: string
  blindAreaFlag: boolean
  blindArea?: string | string[]
  calleeFrequencyFlag: boolean
  frequencyRules: FrequencyRule[]
}

function BoundMerchantsCell({ riskId, merchants }: { riskId: number; merchants: any[] }) {
  const { data: bindings, isLoading } = useQuery({
    queryKey: ['operate', 'risk-control', 'merchants', riskId],
    queryFn: () => fetchRiskControlMerchants(riskId),
    staleTime: 30000,
  })

  if (isLoading) {
    return <span style={{ color: '#94a3b8', fontSize: '13px' }}>加载中...</span>
  }

  const activeBoundMerchantIds = bindings
    ?.filter((b: any) => b.enable)
    ?.map((b: any) => b.merchantId) ?? []

  if (activeBoundMerchantIds.length === 0) {
    return <span style={{ color: '#94a3b8', fontSize: '13px' }}>暂无关联商户</span>
  }

  const merchantNames = activeBoundMerchantIds.map((mid: number) => {
    const found = merchants?.find((m: any) => m.id === mid)
    return found ? `${found.name} (${mid})` : `商户 ${mid}`
  })

  return (
    <Space size={[0, 4]} wrap>
      {merchantNames.map((name: string, i: number) => (
        <Tag color="geekblue" key={i} style={{ borderRadius: '4px' }}>
          {name}
        </Tag>
      ))}
    </Space>
  )
}

export function RiskControlPage() {
  const [activeTab, setActiveTab] = useState('1')
  const queryClient = useQueryClient()
  const tenant = useAuthStore((state) => state.tenant)
  const isSingleTenant = !tenant || tenant.merchantId === '1001' && tenant.permissions.includes('*')

  // 0. 行政区划数据加载与树状转换
  const { data: areaCodes } = useQuery({
    queryKey: ['operate', 'area-codes'],
    queryFn: fetchAreaCodes,
    staleTime: 60000,
  })

  const areaTreeData = useMemo(() => {
    if (!areaCodes || areaCodes.length === 0) return []
    const map = new Map<string, any>()
    const rootNodes: any[] = []
    areaCodes.forEach((item: any) => {
      const node = {
        value: item.code,
        title: `${item.name} (${item.code})`,
        key: item.code,
        children: [] as any[],
        level: item.level,
        parentCode: item.parentCode,
      }
      map.set(item.code, node)
    })
    areaCodes.forEach((item: any) => {
      const node = map.get(item.code)
      if (item.parentCode) {
        const parentNode = map.get(item.parentCode)
        if (parentNode) {
          parentNode.children.push(node)
        } else {
          rootNodes.push(node)
        }
      } else {
        rootNodes.push(node)
      }
    })
    const cleanNodes = (nodes: any[]) => {
      nodes.forEach((n) => {
        if (n.children.length === 0) {
          delete n.children
        } else {
          cleanNodes(n.children)
        }
      })
    }
    cleanNodes(rootNodes)
    return rootNodes
  }, [areaCodes])

  // States for Pool Strategy Edit
  const [poolModalOpen, setPoolModalOpen] = useState(false)
  const [editingPool, setEditingPool] = useState<any | null>(null)
  const [poolForm] = Form.useForm<PoolStrategyFormValues>()

  // States for Risk Control Profile Edit
  const [riskModalOpen, setRiskModalOpen] = useState(false)
  const [editingRisk, setEditingRisk] = useState<any | null>(null)
  const [riskForm] = Form.useForm<RiskControlFormValues>()
  const [freqRules, setFreqRules] = useState<FrequencyRule[]>([])

  // States for Merchant Binding
  const [bindModalOpen, setBindModalOpen] = useState(false)
  const [selectedRiskId, setSelectedRiskId] = useState<number | null>(null)
  const [selectedMerchantIds, setSelectedMerchantIds] = useState<number[]>([])

  // States for Nearby Cities Configuration
  const [nearbySearchText, setNearbySearchText] = useState('')
  const [nearbyCityList, setNearbyCityList] = useState<NearbyCityConfig[]>([])
  const [nearbyModalOpen, setNearbyModalOpen] = useState(false)
  const [editingNearbyIndex, setEditingNearbyIndex] = useState<number | null>(null)
  const [nearbyForm] = Form.useForm<{ cityCode: any; nearbyCodes: any[] }>()

  // 1. Fetch Queries
  const { data: proxyConfigData, isLoading: proxyConfigLoading, refetch: refetchProxyConfig } = useQuery({
    queryKey: ['operate', 'proxy-config'],
    queryFn: () => fetchProxyConfig(),
    enabled: activeTab === 'nearby',
  })

  useEffect(() => {
    if (proxyConfigData?.nearbyCities) {
      try {
        const parsed = JSON.parse(proxyConfigData.nearbyCities)
        const list: NearbyCityConfig[] = []
        for (const [key, val] of Object.entries(parsed)) {
          if (Array.isArray(val)) {
            list.push({ cityCode: key, nearbyCodes: val })
          }
        }
        setNearbyCityList(list)
      } catch (e) {
        setNearbyCityList([])
      }
    } else {
      setNearbyCityList([])
    }
  }, [proxyConfigData])

  const citiesList = useMemo(() => {
    if (!areaCodes) return []
    return areaCodes.filter((item: any) => item.level === 2)
  }, [areaCodes])

  const cascaderOptions = useMemo(() => {
    if (!areaCodes) return []
    const citiesByParent = new Map<string, any[]>()
    areaCodes.forEach((item: any) => {
      if (item.level === 2) {
        const parent = item.parentCode
        if (!citiesByParent.has(parent)) {
          citiesByParent.set(parent, [])
        }
        citiesByParent.get(parent)!.push(item)
      }
    })

    const options: any[] = []
    areaCodes.forEach((item: any) => {
      if (item.level === 1) {
        const provinceCode = item.code
        const children = citiesByParent.get(provinceCode) || []
        options.push({
          value: provinceCode,
          label: item.name,
          children: children.map((city: any) => ({
            value: city.code,
            label: `${city.name} (${city.code})`,
          })),
        })
      }
    })
    return options
  }, [areaCodes])

  const areaCodeToNameMap = useMemo(() => {
    const m = new Map<string, string>()
    areaCodes?.forEach((item: any) => {
      m.set(item.code, item.name)
    })
    return m
  }, [areaCodes])

  const filteredNearbyCityList = useMemo(() => {
    return nearbyCityList.filter((item) => {
      const cityName = areaCodeToNameMap.get(item.cityCode) || ''
      const matchCity = cityName.includes(nearbySearchText) || item.cityCode.includes(nearbySearchText)
      const matchNearby = item.nearbyCodes.some((code) => {
        const name = areaCodeToNameMap.get(code) || ''
        return name.includes(nearbySearchText) || code.includes(nearbySearchText)
      })
      return matchCity || matchNearby
    })
  }, [nearbyCityList, nearbySearchText, areaCodeToNameMap])

  const handleNearbyFormSubmit = (values: { cityCode: string | string[]; nearbyCodes: (string | string[])[] }) => {
    const cityCode = Array.isArray(values.cityCode)
      ? values.cityCode[values.cityCode.length - 1]
      : values.cityCode
    const nearbyCodes = Array.isArray(values.nearbyCodes)
      ? values.nearbyCodes.map((path) => (Array.isArray(path) ? path[path.length - 1] : path))
      : []

    const newList = [...nearbyCityList]
    if (editingNearbyIndex !== null) {
      const exists = newList.some((item, idx) => item.cityCode === cityCode && idx !== editingNearbyIndex)
      if (exists) {
        message.error('该城市已配置过邻近城市规则，请勿重复添加！')
        return
      }
      newList[editingNearbyIndex] = { cityCode, nearbyCodes }
    } else {
      const exists = newList.some((item) => item.cityCode === cityCode)
      if (exists) {
        message.error('该城市已配置过邻近城市规则，请勿重复添加！')
        return
      }
      newList.push({ cityCode, nearbyCodes })
    }
    setNearbyCityList(newList)
    setNearbyModalOpen(false)
    setEditingNearbyIndex(null)
    nearbyForm.resetFields()
  }

  const handleNearbyDelete = (index: number) => {
    const newList = nearbyCityList.filter((_, idx) => idx !== index)
    setNearbyCityList(newList)
    message.success('规则已从当前列表移除，请点击下方“保存并生效相邻城市配置”按钮提交至服务器')
  }

  const saveNearbyCitiesMutation = useMutation({
    mutationFn: async (list: NearbyCityConfig[]) => {
      const map: Record<string, string[]> = {}
      list.forEach((item) => {
        map[item.cityCode] = item.nearbyCodes
      })
      const jsonStr = JSON.stringify(map)
      const latestConfig = await fetchProxyConfig()
      const payload = {
        ...latestConfig,
        nearbyCities: jsonStr,
      }
      await saveProxyConfig(payload)
    },
    onSuccess: () => {
      message.success('相邻城市匹配优先级配置已成功保存并立即生效！')
      queryClient.invalidateQueries({ queryKey: ['operate', 'proxy-config'] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '保存相邻城市配置失败')
    }
  })

  // 1. Fetch Queries
  const { data: poolsData, isLoading: poolsLoading } = useQuery({
    queryKey: ['operate', 'pool', 1, 100],
    queryFn: () => fetchPools(1, 100),
  })

  const { data: gatewaysData } = useQuery({
    queryKey: ['operate', 'gateway', 1, 100],
    queryFn: () => fetchGatewayPage(1, 100),
  })

  const { data: channelsData } = useQuery({
    queryKey: ['operate', 'channel', 1, 100],
    queryFn: () => fetchChannels(1, 100),
  })

  const { data: skillGroupsData } = useQuery({
    queryKey: ['merchant', 'skill-group', 1, 100],
    queryFn: () => fetchSkillGroups(1, 100),
    enabled: activeTab === '1',
  })

  const { data: riskData, isLoading: riskLoading } = useQuery({
    queryKey: ['operate', 'risk-control', 1, 100],
    queryFn: () => fetchRiskControls(1, 100),
    enabled: activeTab === '2' || activeTab === '3',
  })

  const { data: merchantsData } = useQuery({
    queryKey: ['operate', 'merchant', 1, 100],
    queryFn: () => fetchMerchants(1, 100),
    enabled: activeTab === '3' && !isSingleTenant,
  })

  // 2. Mutations
  const savePoolMutation = useMutation({
    mutationFn: async (values: PoolStrategyFormValues) => savePool(values),
    onSuccess: async () => {
      message.success('选号分发策略已更新')
      setPoolModalOpen(false)
      setEditingPool(null)
      await queryClient.invalidateQueries({ queryKey: ['operate', 'pool'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '更新策略失败'),
  })

  const saveRiskMutation = useMutation({
    mutationFn: async (values: any) => saveRiskControl(values),
    onSuccess: async () => {
      message.success(editingRisk ? '风控配置已更新' : '风控配置已创建')
      setRiskModalOpen(false)
      setEditingRisk(null)
      riskForm.resetFields()
      setFreqRules([])
      await queryClient.invalidateQueries({ queryKey: ['operate', 'risk-control'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存风控失败'),
  })

  const deleteRiskMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteRiskControls(ids),
    onSuccess: async () => {
      message.success('风控配置已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'risk-control'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveBindingsMutation = useMutation({
    mutationFn: async ({ riskId, bindings }: { riskId: number; bindings: any[] }) =>
      saveRiskControlMerchants(riskId, bindings),
    onSuccess: async () => {
      message.success('商户风控绑定关系已更新')
      setBindModalOpen(false)
      setSelectedRiskId(null)
      await queryClient.invalidateQueries({ queryKey: ['operate', 'risk-control'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '更新商户绑定失败'),
  })

  // 3. Helper Mappings
  const gatewayMap = useMemo(() => {
    const m = new Map<number, any>()
    gatewaysData?.records.forEach((g: any) => m.set(g.id, g))
    return m
  }, [gatewaysData])

  const channelMap = useMemo(() => {
    const m = new Map<number, any>()
    channelsData?.records.forEach((c: any) => m.set(c.id, c))
    return m
  }, [channelsData])

  // 4. Modal Open Hooks
  function openEditPool(record: any) {
    setEditingPool(record)
    setPoolModalOpen(true)
    setTimeout(() => {
      poolForm.setFieldsValue({
        id: record.id,
        name: record.name,
        remark: record.remark,
        type: record.typeId,
        gatewayId: record.gatewayId || undefined,
        enable: record.enable,
        selectionStrategy: record.selectionStrategy || 'CONCURRENCY',
      })
    }, 0)
  }

  // Frequency Rule List Helpers
  function addFreqRule() {
    setFreqRules([...freqRules, { day: 1, count: 5, type: 'DIAL' }])
  }

  function removeFreqRule(idx: number) {
    setFreqRules(freqRules.filter((_, i) => i !== idx))
  }

  function updateFreqRule(idx: number, field: keyof FrequencyRule, val: any) {
    const updated = [...freqRules]
    updated[idx] = { ...updated[idx], [field]: val }
    setFreqRules(updated)
  }

  function openCreateRisk() {
    setEditingRisk(null)
    setFreqRules([])
    setRiskModalOpen(true)
    setTimeout(() => {
      riskForm.resetFields()
      riskForm.setFieldsValue({
        blackLevelFlag: true,
        blackLevel: 'LEVEL_1',
        blindAreaFlag: false,
        blindArea: [],
        calleeFrequencyFlag: false,
      })
    }, 0)
  }

  function openEditRisk(record: any) {
    setEditingRisk(record)
    let parsedFreqs: FrequencyRule[] = []
    if (record.calleeFrequency) {
      try {
        parsedFreqs = JSON.parse(record.calleeFrequency)
      } catch (e) {
        parsedFreqs = []
      }
    }
    setFreqRules(parsedFreqs)
    let initialBlindAreas: string[] = []
    if (record.blindArea) {
      initialBlindAreas = record.blindArea.split(',').map((s: string) => s.trim()).filter(Boolean)
    }
    setRiskModalOpen(true)
    setTimeout(() => {
      riskForm.setFieldsValue({
        id: record.id,
        name: record.name,
        remark: record.remark,
        blackLevelFlag: record.blackLevelFlag,
        blackLevel: record.blackLevel || 'LEVEL_1',
        blindAreaFlag: record.blindAreaFlag,
        blindArea: initialBlindAreas,
        calleeFrequencyFlag: record.calleeFrequencyFlag,
      })
    }, 0)
  }

  async function openBindMerchant(riskId: number) {
    setSelectedRiskId(riskId)
    try {
      const bindings = await fetchRiskControlMerchants(riskId)
      const boundIds = bindings.filter((b: any) => b.enable).map((b: any) => b.merchantId)
      setSelectedMerchantIds(boundIds)
      setBindModalOpen(true)
    } catch (e) {
      message.error('加载商户绑定关系失败')
    }
  }

  // 5. Submit handlers
  function submitPoolStrategy(values: PoolStrategyFormValues) {
    savePoolMutation.mutate(values)
  }

  function submitRiskControl(values: any) {
    let blindAreaStr = ''
    if (values.blindAreaFlag && values.blindArea) {
      blindAreaStr = Array.isArray(values.blindArea) ? values.blindArea.join(',') : String(values.blindArea)
    }
    const payload = {
      id: editingRisk?.id || undefined,
      name: values.name,
      remark: values.remark || '',
      blackLevelFlag: values.blackLevelFlag,
      blackLevel: values.blackLevelFlag ? values.blackLevel : '',
      blindAreaFlag: values.blindAreaFlag,
      blindArea: blindAreaStr,
      calleeFrequencyFlag: values.calleeFrequencyFlag,
      calleeFrequency: values.calleeFrequencyFlag ? JSON.stringify(freqRules) : '',
    }
    saveRiskMutation.mutate(payload)
  }

  function submitBindings() {
    if (selectedRiskId === null) return
    const bindings = selectedMerchantIds.map((mid) => ({
      riskId: selectedRiskId,
      merchantId: mid,
      enable: true,
    }))
    saveBindingsMutation.mutate({ riskId: selectedRiskId, bindings })
  }

  return (
    <Space direction="vertical" size="middle" className="w-full">
      <Card
        bordered={true}
        className="w-full shadow-soft"
        style={{
          background: 'linear-gradient(135deg, var(--bg-container) 0%, var(--bg-app) 100%)',
          borderRadius: '12px',
          border: '1px solid var(--border-color)',
        }}
      >
        <div className="flex items-center gap-4">
          <div
            style={{
              background: 'rgba(59, 130, 246, 0.08)',
              padding: '12px',
              borderRadius: '8px',
              border: '1px solid rgba(59, 130, 246, 0.16)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <SafetyCertificateOutlined style={{ fontSize: '28px', color: '#3b82f6' }} />
          </div>
          <div>
            <Typography.Title level={4} style={{ color: 'var(--text-primary)', margin: 0, fontWeight: 600 }}>
              选号逻辑与风控管理
            </Typography.Title>
          </div>
        </div>
      </Card>

      <Tabs activeKey={activeTab} onChange={setActiveTab} type="card" className="w-full">
        <TabPane
          tab={
            <span>
              <BranchesOutlined /> 号码池分发策略 & 拓扑
            </span>
          }
          key="1"
        >
          <Row gutter={[24, 24]}>
            <Col xs={24} lg={12}>
              <TableWrap
                title="号码池分发策略配置"
                rowKey="id"
                dataSource={poolsData?.records ?? []}
                loading={poolsLoading}
                pagination={{ pageSize: 8 }}
                locale={{
                  emptyText: (
                    <Empty
                      description={
                        <span style={{ color: 'var(--text-secondary)' }}>
                          暂无分发策略数据，请先前往{' '}
                          <Link to="/operate/pool" style={{ color: '#1677ff', fontWeight: 500 }}>
                            [号码池配置]
                          </Link>{' '}
                          进行创建。
                        </span>
                      }
                    />
                  )
                }}
                columns={[
                  { title: 'ID', dataIndex: 'id', width: 60 },
                  { title: '号码池名称', dataIndex: 'name' },
                  {
                    title: '分配调度策略',
                    dataIndex: 'selectionStrategy',
                    render: (val: string) => {
                      switch (val) {
                        case 'RANDOM':
                          return <Tag color="orange">RANDOM - 伪随机均匀哈希</Tag>
                        case 'PRIORITY':
                          return <Tag color="blue">PRIORITY - 网关优先级升序</Tag>
                        default:
                          return <Tag color="green">CONCURRENCY - 物理并发优先</Tag>
                      }
                    },
                  },
                  {
                    title: '操作',
                    width: 100,
                    render: (_, record) => (
                      <PermissionGate permission="operate:pool:write">
                        <Button size="small" type="primary" ghost icon={<InteractionOutlined />} onClick={() => openEditPool(record)}>
                          策略变更
                        </Button>
                      </PermissionGate>
                    ),
                  },
                ]}
              />
            </Col>
            <Col xs={24} lg={12}>
              <Card 
                title="选号路由链路拓扑映射" 
                className="shadow-soft min-h-[400px]"
                style={{ 
                  background: 'var(--bg-container)',
                  border: '1px solid var(--border-color)',
                }}
              >
                <div style={{ padding: '8px' }}>
                  {poolsData?.records && poolsData.records.length > 0 ? (
                    <div className="flex flex-col gap-6" style={{ marginTop: '16px' }}>
                      {poolsData.records.filter((p: any) => p.enable).map((pool: any) => {
                        const gw = pool.gatewayId ? gatewayMap.get(pool.gatewayId) : null
                        const ch = gw?.channelId ? channelMap.get(gw.channelId) : null
                        const matchedSgs = skillGroupsData?.records.slice(0, 2).map((s: any) => s.name).join(', ') || '通用呼叫'

                        return (
                          <div
                            key={pool.id}
                            className="p-4 border rounded-xl"
                            style={{ 
                              borderLeft: '4px solid #10b981', 
                              transition: 'all 0.3s',
                              background: 'var(--bg-app)',
                              borderColor: 'var(--border-color)'
                            }}
                          >
                            <div className="flex items-center gap-3 flex-wrap">
                              <div className="flex-1 min-w-[120px] bg-slate-50 dark:bg-zinc-800/60 p-3 rounded-lg border border-slate-100 dark:border-zinc-700/50">
                                <span className="text-[10px] text-slate-400 dark:text-zinc-500 block font-mono">呼叫技能组</span>
                                <span className="font-semibold text-xs text-slate-800 dark:text-zinc-200">{matchedSgs}</span>
                              </div>
                              <ArrowRightOutlined className="text-slate-300 dark:text-zinc-600 flex-shrink-0" />
                              <div className="flex-1 min-w-[120px] bg-emerald-50/30 dark:bg-emerald-950/10 p-3 rounded-lg border border-emerald-100/30 dark:border-emerald-900/20">
                                <span className="text-[10px] text-emerald-500 block font-mono">号码池 (策略)</span>
                                <span className="font-bold text-xs text-emerald-600 dark:text-emerald-400">{pool.name}</span>
                                <div className="mt-1">
                                  <Tag color="processing" className="text-[9px] px-1 border-none m-0 leading-normal">
                                    {pool.selectionStrategy || 'CONCURRENCY'}
                                  </Tag>
                                </div>
                              </div>
                              <ArrowRightOutlined className="text-slate-300 dark:text-zinc-600 flex-shrink-0" />
                              <div className="flex-1 min-w-[120px] bg-blue-50/30 dark:bg-blue-950/10 p-3 rounded-lg border border-blue-100/30 dark:border-blue-900/20">
                                <span className="text-[10px] text-blue-500 block font-mono">呼叫网关</span>
                                <span className="font-semibold text-xs text-blue-700 dark:text-blue-400">{gw ? gw.name : '未关联'}</span>
                                {gw && (
                                  <div className="mt-1">
                                    <Tag color="warning" className="text-[9px] px-1 border-none m-0 leading-normal">
                                      Priority: {gw.priority || 1}
                                    </Tag>
                                  </div>
                                )}
                              </div>
                              <ArrowRightOutlined className="text-slate-300 dark:text-zinc-600 flex-shrink-0" />
                              <div className="flex-1 min-w-[120px] bg-purple-50/30 dark:bg-purple-950/10 p-3 rounded-lg border border-purple-100/30 dark:border-purple-900/20">
                                <span className="text-[10px] text-purple-500 block font-mono">物理渠道</span>
                                <span className="font-semibold text-xs text-purple-700 dark:text-purple-400">{ch ? ch.name : '未分配'}</span>
                              </div>
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  ) : (
                    <div style={{ padding: '12px 4px' }}>
                      <Steps
                        direction="vertical"
                        size="small"
                        current={0}
                        items={[
                          {
                            title: <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>步骤 1：配置呼叫网关与物理线路</span>,
                            description: (
                              <div style={{ color: 'var(--text-secondary)', fontSize: '13px', margin: '4px 0 12px 0' }}>
                                选号决策前系统必须存在可用的 SIP 落地网关或物理线路渠道。
                                <div style={{ marginTop: '6px' }}>
                                  <Link to="/operate/gateway">
                                    <Button type="link" size="small" style={{ padding: 0 }}>立即前往 [网关管理] ➜</Button>
                                  </Link>
                                </div>
                              </div>
                            ),
                          },
                          {
                            title: <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>步骤 2：创建号码池并关联配置</span>,
                            description: (
                              <div style={{ color: 'var(--text-secondary)', fontSize: '13px', margin: '4px 0 12px 0' }}>
                                创建号码池并将它关联至对应的呼叫网关，设定呼出时的选号调度策略。
                                <div style={{ marginTop: '6px' }}>
                                  <Link to="/operate/pool">
                                    <Button type="link" size="small" style={{ padding: 0 }}>立即前往 [号码池管理] ➜</Button>
                                  </Link>
                                </div>
                              </div>
                            ),
                          },
                          {
                            title: <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>步骤 3：导入和分配号码资源</span>,
                            description: (
                              <div style={{ color: 'var(--text-secondary)', fontSize: '13px', margin: '4px 0 12px 0' }}>
                                在号码资源库中将具体的电话号码分配入库，归属到指定的号码池。
                                <div style={{ marginTop: '6px' }}>
                                  <Link to="/operate/pool-phone">
                                    <Button type="link" size="small" style={{ padding: 0 }}>立即前往 [号码资源库] ➜</Button>
                                  </Link>
                                </div>
                              </div>
                            ),
                          },
                          {
                            title: <span style={{ fontWeight: 600, color: 'var(--text-secondary)' }}>步骤 4：在商户/技能组中应用风控</span>,
                            description: (
                              <div style={{ color: 'var(--text-secondary)', fontSize: '13px', margin: '4px 0 0 0' }}>
                                在本页面配置风控规则并将其绑定应用给指定的商户，使外呼业务安全合规。
                              </div>
                            ),
                          },
                        ]}
                      />
                    </div>
                  )}
                </div>
              </Card>
            </Col>
          </Row>
        </TabPane>

        <TabPane
          tab={
            <span>
              <ApartmentOutlined /> 邻近城市匹配配置
            </span>
          }
          key="nearby"
        >
          <div className="flex justify-between items-center mb-4 flex-wrap gap-2">
            <Space>
              <Input
                placeholder="搜索配置城市/邻近城市..."
                value={nearbySearchText}
                onChange={(e) => setNearbySearchText(e.target.value)}
                allowClear
                style={{ width: 250 }}
              />
            </Space>
            <Space>
              <Button onClick={() => refetchProxyConfig()} loading={proxyConfigLoading}>
                刷新
              </Button>
              <PermissionGate permission="operate:pool:write">
                <Button type="primary" icon={<PlusOutlined />} onClick={() => {
                  setEditingNearbyIndex(null)
                  nearbyForm.resetFields()
                  setNearbyModalOpen(true)
                }}>
                  新增城市映射
                </Button>
              </PermissionGate>
            </Space>
          </div>

          <TableWrap
            title="邻近城市选号匹配映射列表"
            rowKey="cityCode"
            dataSource={filteredNearbyCityList}
            loading={proxyConfigLoading}
            columns={[
              {
                title: '主体城市',
                dataIndex: 'cityCode',
                width: 200,
                render: (code: string) => {
                  const name = areaCodeToNameMap.get(code) || code
                  return (
                    <div className="flex flex-col">
                      <span className="font-bold text-slate-800 dark:text-zinc-100">{name}</span>
                      <span className="text-xs text-slate-400 font-mono">{code}</span>
                    </div>
                  )
                }
              },
              {
                title: '按优先级顺序依次试选邻近城市',
                dataIndex: 'nearbyCodes',
                render: (codes: string[]) => (
                  <Space size={[4, 8]} wrap>
                    {codes.map((code, index) => {
                      const name = areaCodeToNameMap.get(code) || code
                      return (
                        <Tag key={code} color="cyan" style={{ border: 'none', padding: '2px 8px', borderRadius: '4px' }}>
                          <span className="text-slate-400 dark:text-zinc-500 font-mono mr-1">#{index + 1}</span>
                          <span className="font-semibold text-slate-700 dark:text-zinc-200">{name}</span>
                          <span className="text-[10px] text-slate-400 font-mono ml-1">({code})</span>
                        </Tag>
                      )
                    })}
                  </Space>
                )
              },
              {
                title: '操作',
                width: 150,
                render: (_, record, index) => (
                  <Space size="small">
                    <PermissionGate permission="operate:pool:write">
                      <Button size="small" icon={<EditOutlined />} onClick={() => {
                        setEditingNearbyIndex(index)
                        nearbyForm.setFieldsValue({
                          cityCode: [record.cityCode.substring(0, 2) + '0000', record.cityCode],
                          nearbyCodes: record.nearbyCodes.map((code: string) => [code.substring(0, 2) + '0000', code]),
                        })
                        setNearbyModalOpen(true)
                      }}>
                        编辑
                      </Button>
                    </PermissionGate>
                    <PermissionGate permission="operate:pool:write">
                      <Popconfirm title="确认删除这组邻近城市配置？" onConfirm={() => handleNearbyDelete(index)}>
                        <Button size="small" danger icon={<DeleteOutlined />}>
                          删除
                        </Button>
                      </Popconfirm>
                    </PermissionGate>
                  </Space>
                )
              }
            ]}
          />

          <div className="mt-6 p-4 rounded-xl bg-slate-50 dark:bg-zinc-900/40 border border-slate-200/50 dark:border-zinc-800 flex justify-between items-center">
            <div className="flex gap-2.5 items-start">
              <InfoCircleOutlined className="text-slate-400 mt-0.5" />
              <div className="text-xs text-slate-500 dark:text-zinc-400 leading-relaxed max-w-2xl">
                <div className="font-semibold text-slate-700 dark:text-zinc-300 mb-0.5">本地号码优先 & 邻近城市匹配机制说明</div>
                当系统呼叫某被叫时，会根据被叫号码归属地匹配该城市下的主叫号码（Rank=1）。如果该号码池中无此城市可用号码，将依照以上配置的优先级顺序依次试选邻近城市（Rank=2, 3...）。全部邻近城市无法接通时，自动 fallback 到同省其他城市（Rank=100）及全国其他可用主叫（Rank=9999），确保外呼业务永不中断。
              </div>
            </div>
            <PermissionGate permission="operate:pool:write">
              <Button
                type="primary"
                size="large"
                loading={saveNearbyCitiesMutation.isPending}
                onClick={() => saveNearbyCitiesMutation.mutate(nearbyCityList)}
              >
                保存并生效相邻城市配置
              </Button>
            </PermissionGate>
          </div>
        </TabPane>

        <TabPane
          tab={
            <span>
              <SafetyCertificateOutlined /> 风控规则设置
            </span>
          }
          key="2"
        >
          <div className="mb-4 flex justify-end">
            <PermissionGate permission="operate:riskcontrol:write">
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreateRisk}>
                新增风控策略
              </Button>
            </PermissionGate>
          </div>
          <TableWrap
            title="风控策略列表"
            rowKey="id"
            dataSource={riskData?.records ?? []}
            loading={riskLoading}
            columns={[
              { title: 'ID', dataIndex: 'id', width: 60 },
              { title: '策略名称', dataIndex: 'name' },
              { title: '备注', dataIndex: 'remark' },
              {
                title: '黑名单风控',
                dataIndex: 'blackLevelFlag',
                render: (val: boolean, record: any) =>
                  val ? (
                    <Tag color="red">启用 (黑名单等级 {record.blackLevel || 'LEVEL_1'})</Tag>
                  ) : (
                    <Tag color="default">停用</Tag>
                  ),
              },
              {
                title: '外呼盲区',
                dataIndex: 'blindAreaFlag',
                render: (val: boolean, record: any) => {
                  if (!val || !record.blindArea) return <Tag color="default">停用</Tag>
                  const codes = record.blindArea.split(',').map((s: string) => s.trim()).filter(Boolean)
                  const names = codes.map((c: string) => {
                    const found = areaCodes?.find((a: any) => a.code === c)
                    return found ? found.name : c
                  }).join(', ')
                  return (
                    <Tooltip title={names || record.blindArea}>
                      <Tag color="volcano">已配置 ({codes.length} 个地区)</Tag>
                    </Tooltip>
                  )
                }
              },
              {
                title: '被叫频次限制',
                dataIndex: 'calleeFrequencyFlag',
                render: (val: boolean, record: any) => {
                  if (!val || !record.calleeFrequency) return <Tag color="default">停用</Tag>
                  try {
                    const parsed: FrequencyRule[] = JSON.parse(record.calleeFrequency)
                    return (
                      <Space size="small" direction="vertical">
                        {parsed.map((r, i) => (
                          <Tag color="orange" key={i}>
                            {r.day}天限{r.count}次({r.type === 'CONNECTED' ? '接通' : '呼叫'})
                          </Tag>
                        ))}
                      </Space>
                    )
                  } catch (e) {
                    return <Tag color="warning">配置异常</Tag>
                  }
                },
              },
              {
                title: '操作',
                render: (_, record) => (
                  <Space size="small">
                    <PermissionGate permission="operate:riskcontrol:write">
                      <Button size="small" icon={<EditOutlined />} onClick={() => openEditRisk(record)}>
                        编辑
                      </Button>
                    </PermissionGate>
                    <PermissionGate permission="operate:riskcontrol:delete">
                      <Popconfirm title="确认删除此风控策略配置？" onConfirm={() => deleteRiskMutation.mutate([record.id])}>
                        <Button size="small" danger icon={<DeleteOutlined />}>
                          删除
                        </Button>
                      </Popconfirm>
                    </PermissionGate>
                  </Space>
                ),
              },
            ]}
          />
        </TabPane>

        <TabPane
          tab={
            <span>
              <SettingOutlined /> 商户应用范围
            </span>
          }
          key="3"
        >
          {isSingleTenant ? (
            <Card className="shadow-soft" variant="borderless" style={{ borderRadius: '8px' }}>
              <Alert
                message="单商户模式运行中"
                description="当前系统处于单商户运行模式，全部风控规则与号码分发路由默认全局应用给默认商户 1001 (本地默认商户)，无需手动绑定商户应用范围。"
                type="success"
                showIcon
              />
            </Card>
          ) : (
            <Card className="shadow-soft" variant="borderless" style={{ borderRadius: '8px', padding: '16px' }}>
              <div className="flex justify-end mb-6">
                <PermissionGate permission="operate:riskcontrol:write">
                  <Button type="primary" size="large" icon={<PlusOutlined />} onClick={() => { setActiveTab('2'); openCreateRisk(); }}>
                    新增风控策略
                  </Button>
                </PermissionGate>
              </div>
              <TableWrap
                title="风控策略商户配置匹配"
                rowKey="id"
                dataSource={riskData?.records ?? []}
                loading={riskLoading}
                columns={[
                  { title: '风控策略 ID', dataIndex: 'id', width: 120 },
                  { title: '风控策略名称', dataIndex: 'name' },
                  {
                    title: '已应用商户范围',
                    width: 280,
                    render: (_, record) => (
                      <BoundMerchantsCell
                        riskId={record.id}
                        merchants={merchantsData?.records ?? []}
                      />
                    ),
                  },
                  { title: '备注', dataIndex: 'remark' },
                  {
                    title: '关联商户作用域',
                    render: (_, record) => (
                      <Button size="small" type="primary" ghost icon={<CheckCircleOutlined />} onClick={() => openBindMerchant(record.id)}>
                        关联商户应用范围
                      </Button>
                    ),
                  },
                ]}
              />
            </Card>
          )}
        </TabPane>
      </Tabs>

      {/* 1. Modal: Edit Pool Strategy */}
      <Modal
        open={poolModalOpen}
        title="号码池分配策略变更"
        onCancel={() => {
          setPoolModalOpen(false)
          setEditingPool(null)
          poolForm.resetFields()
        }}
        onOk={() => poolForm.submit()}
        confirmLoading={savePoolMutation.isPending}
        destroyOnHidden
      >
        <Form form={poolForm} layout="vertical" onFinish={submitPoolStrategy}>
          <Form.Item name="id" hidden><InputNumber /></Form.Item>
          <Form.Item name="name" hidden><Input /></Form.Item>
          <Form.Item name="remark" hidden><Input /></Form.Item>
          <Form.Item name="type" hidden><InputNumber /></Form.Item>
          <Form.Item name="gatewayId" hidden><InputNumber /></Form.Item>
          <Form.Item name="enable" hidden valuePropName="checked"><Switch /></Form.Item>

          <Form.Item name="selectionStrategy" label="号码池分发策略" rules={[{ required: true, message: '请选择分发策略' }]}>
            <Select style={{ width: '100%' }}>
              <Option value="CONCURRENCY">CONCURRENCY - 最大可用并发优先</Option>
              <Option value="PRIORITY">PRIORITY - 网关优先级升序路由</Option>
              <Option value="RANDOM">RANDOM - 伪随机哈希均匀轮询</Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      {/* 2. Modal: Add/Edit Risk Control */}
      <Modal
        open={riskModalOpen}
        title={editingRisk ? '编辑风控策略' : '创建风控策略'}
        width={650}
        onCancel={() => {
          setRiskModalOpen(false)
          setEditingRisk(null)
          riskForm.resetFields()
          setFreqRules([])
        }}
        onOk={() => riskForm.submit()}
        confirmLoading={saveRiskMutation.isPending}
        destroyOnHidden
      >
        <Form form={riskForm} layout="vertical" onFinish={submitRiskControl}>
          <Form.Item name="name" label="风控策略名称" rules={[{ required: true, message: '请输入策略名称' }]}>
            <Input placeholder="例如: 基础高频外呼风控策略" />
          </Form.Item>
          <Form.Item name="remark" label="描述备注">
            <Input placeholder="备注说明该风控策略的使用场景" />
          </Form.Item>

          {/* Blacklist Config */}
          <Card size="small" title="系统级黑名单检测" className="mb-4">
            <Form.Item name="blackLevelFlag" label="启用黑名单过滤" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item noStyle shouldUpdate={(prev, curr) => prev.blackLevelFlag !== curr.blackLevelFlag}>
              {({ getFieldValue }) =>
                getFieldValue('blackLevelFlag') && (
                  <Form.Item name="blackLevel" label="拦截黑名单等级限制" rules={[{ required: true }]}>
                    <Select>
                      <Option value="LEVEL_1">LEVEL_1 - 只拦截一级严重黑名单 (高危号码)</Option>
                      <Option value="LEVEL_2">LEVEL_2 - 拦截一、二级黑名单 (高危与投诉号码)</Option>
                      <Option value="LEVEL_3">LEVEL_3 - 拦截一、二、三级黑名单 (全部风险号码)</Option>
                    </Select>
                  </Form.Item>
                )
              }
            </Form.Item>
          </Card>

          {/* Blind Area Config */}
          <Card size="small" title="外呼盲区检测 (限制呼叫目的地区域)" className="mb-4">
            <Form.Item name="blindAreaFlag" label="启用盲区过滤" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item noStyle shouldUpdate={(prev, curr) => prev.blindAreaFlag !== curr.blindAreaFlag}>
              {({ getFieldValue }) =>
                getFieldValue('blindAreaFlag') && (
                  <Form.Item
                    name="blindArea"
                    label="限制呼叫目的行政区域 (支持多选及省市折叠)"
                    rules={[{ required: true, message: '请选择外呼拦截的省市区域' }]}
                    extra="匹配到选中的被叫省份或城市时，该号码候选将被风控拦截（若选择省级节点将自动拦截该省份下所有城市）。"
                  >
                    <TreeSelect
                      treeData={areaTreeData}
                      placeholder="请勾选呼叫受限的省份或地级市"
                      allowClear
                      multiple
                      treeCheckable
                      showSearch
                      treeNodeFilterProp="title"
                      style={{ width: '100%' }}
                      dropdownStyle={{ maxHeight: 400, overflow: 'auto' }}
                    />
                  </Form.Item>
                )
              }
            </Form.Item>
          </Card>

          {/* Frequency Config */}
          <Card size="small" title="被叫呼叫频次检测 (防投诉高频拦截)" className="mb-4">
            <Form.Item name="calleeFrequencyFlag" label="启用被叫频次限制" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item noStyle shouldUpdate={(prev, curr) => prev.calleeFrequencyFlag !== curr.calleeFrequencyFlag}>
              {({ getFieldValue }) =>
                getFieldValue('calleeFrequencyFlag') && (
                  <div>
                    <div className="flex justify-end mb-3">
                      <Button size="small" type="dashed" icon={<PlusOutlined />} onClick={addFreqRule}>
                        添加拦截条目
                      </Button>
                    </div>
                    {freqRules.map((rule, idx) => (
                      <Row key={idx} gutter={8} className="mb-2" align="middle">
                        <Col span={7}>
                          <Space size="small">
                            <span>统计天数:</span>
                            <InputNumber min={1} max={90} value={rule.day} onChange={(v) => updateFreqRule(idx, 'day', v)} />
                          </Space>
                        </Col>
                        <Col span={7}>
                          <Space size="small">
                            <span>限制次数:</span>
                            <InputNumber min={1} max={100} value={rule.count} onChange={(v) => updateFreqRule(idx, 'count', v)} />
                          </Space>
                        </Col>
                        <Col span={8}>
                          <Space size="small">
                            <span>限制类型:</span>
                            <Select value={rule.type} onChange={(v) => updateFreqRule(idx, 'type', v)} style={{ width: 100 }}>
                              <Option value="DIAL">呼叫次数</Option>
                              <Option value="CONNECTED">接通次数</Option>
                            </Select>
                          </Space>
                        </Col>
                        <Col span={2}>
                          <Button danger size="small" icon={<DeleteOutlined />} onClick={() => removeFreqRule(idx)} />
                        </Col>
                      </Row>
                    ))}
                    {freqRules.length === 0 && (
                      <div className="text-center p-4 border border-dashed rounded text-gray-400">
                        暂未配置任何频次规则，请添加。
                      </div>
                    )}
                  </div>
                )
              }
            </Form.Item>
          </Card>
        </Form>
      </Modal>

      {/* 3. Modal: Bind Merchants */}
      <Modal
        open={bindModalOpen}
        title="绑定商户风控应用范围"
        onCancel={() => {
          setBindModalOpen(false)
          setSelectedRiskId(null)
          setSelectedMerchantIds([])
        }}
        onOk={submitBindings}
        confirmLoading={saveBindingsMutation.isPending}
        destroyOnHidden
      >

        <div className="mb-3">
          <Typography.Text strong>选择要应用此策略的商户列表 (支持多选):</Typography.Text>
        </div>
        <Select
          mode="multiple"
          placeholder="请选择绑定的商户"
          style={{ width: '100%' }}
          value={selectedMerchantIds}
          onChange={setSelectedMerchantIds}
          allowClear
        >
          {merchantsData?.records.map((m: any) => (
            <Option key={m.id} value={m.id}>
              {m.name} (商户账号: {m.account})
            </Option>
          ))}
        </Select>
      </Modal>

      {/* 4. Modal: Add/Edit Nearby Cities Mapping */}
      <Modal
        open={nearbyModalOpen}
        title={editingNearbyIndex !== null ? '编辑邻近城市映射' : '新增邻近城市映射'}
        onCancel={() => {
          setNearbyModalOpen(false)
          setEditingNearbyIndex(null)
          nearbyForm.resetFields()
        }}
        onOk={() => nearbyForm.submit()}
        confirmLoading={saveNearbyCitiesMutation.isPending}
        destroyOnHidden
      >
        <Form
          form={nearbyForm}
          layout="vertical"
          onFinish={handleNearbyFormSubmit}
          className="pt-4"
        >
          <Form.Item
            name="cityCode"
            label="核心匹配主体城市"
            rules={[{ required: true, message: '请选择主体城市' }]}
            extra="被叫归属的本地城市，作为优先级选号的第一判定点。"
          >
            <Cascader
              options={cascaderOptions}
              placeholder="请选择主体地级市"
              showSearch
            />
          </Form.Item>

          <Form.Item
            name="nearbyCodes"
            label="优先级降级选择邻近城市 (可多选，选号时按添加顺序排序)"
            rules={[{ required: true, message: '请选择至少一个邻近城市' }]}
            extra="当本地主体城市号码并发不足或不可用时，系统将依此处的设置顺序尝试呼叫邻近城市的备选号码。"
          >
            <Cascader
              multiple
              options={cascaderOptions}
              placeholder="请选择邻近备选地级市"
              showSearch
              maxTagCount="responsive"
            />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
