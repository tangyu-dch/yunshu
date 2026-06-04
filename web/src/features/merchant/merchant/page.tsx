import {
  Button,
  DatePicker,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
  message,
  Card,
  Row,
  Col,
  Statistic,
  InputNumber,
  Divider,
  Tooltip,
  Spin
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo } from 'react'
import dayjs from 'dayjs'
import { useNavigate } from 'react-router-dom'
import { PermissionGate } from '@/components/PermissionGate'
import { TableWrap } from '@/components/TableWrap'
import { QueryBar } from '@/components/QueryBar'
import {
  deleteMerchants,
  fetchMerchants,
  fetchRates,
  saveMerchant,
  toggleMerchantEnable,
  fetchAccounts,
  saveAccount,
  resetAccountPassword,
  fetchRiskControls,
  fetchRiskControlMerchants,
  saveRiskControlMerchants,
  savePool,
  fetchPools,
  fetchGatewayPage,
  fetchChannels
} from '@/api/operate'
import { useAuthStore } from '@/store/auth'
import { useUiStore } from '@/store/ui'
import {
  ReloadOutlined,
  PlusOutlined,
  UserSwitchOutlined,
  KeyOutlined,
  CompassOutlined,
  SafetyCertificateOutlined,
  CalendarOutlined,
  ShopOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  ArrowRightOutlined,
  SlidersOutlined,
  FundProjectionScreenOutlined,
  SafetyOutlined,
  PartitionOutlined,
  NodeIndexOutlined,
  GatewayOutlined,
  GlobalOutlined
} from '@ant-design/icons'

type MerchantFormValues = {
  id?: number
  name: string
  account: string
  password?: string
  expiredTime?: any
  enable: boolean
  rateId?: number
  whitelistDomains?: string
  sipDomain?: string
  appKey?: string
  appSecret?: string
  maxAgents?: number
  riskControlIds?: number[]
}

export function MerchantPage() {
  const theme = useUiStore((state) => state.theme)
  const isDark = theme === 'dark'
  const navigate = useNavigate()
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [open, setOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form] = Form.useForm<MerchantFormValues>()
  const queryClient = useQueryClient()
  const [queryParams, setQueryParams] = useState<Record<string, any>>({})

  // 决策配置中心状态
  const [decisionModalOpen, setDecisionModalOpen] = useState(false)
  const [decisionMerchantId, setDecisionMerchantId] = useState<number | null>(null)
  const [decisionForm] = Form.useForm()
  const [decisionFormValues, setDecisionFormValues] = useState<any>({ riskControlIds: [], poolStrategies: {} })

  // 1. 获取所有风控策略
  const { data: riskControlsData } = useQuery({
    queryKey: ['operate', 'risk-control', 1, 100],
    queryFn: () => fetchRiskControls(1, 100),
  })

  const riskControls = riskControlsData?.records ?? []

  // 2. 并发查询各策略关联的商户，合并映射
  const { data: allBindingsMap, isLoading: isBindingsLoading } = useQuery({
    queryKey: ['operate', 'merchant-risk-bindings', riskControls.map((r: any) => r.id).join(',')],
    queryFn: async () => {
      if (riskControls.length === 0) return {}
      const results = await Promise.all(
        riskControls.map(async (rc: any) => {
          try {
            const bindings = await fetchRiskControlMerchants(rc.id)
            return { riskId: rc.id, bindings }
          } catch (e) {
            return { riskId: rc.id, bindings: [] }
          }
        })
      )
      const map: Record<number, any[]> = {}
      results.forEach(({ riskId, bindings }) => {
        const rc = riskControls.find((r: any) => r.id === riskId)
        if (!rc) return
        bindings.forEach((b: any) => {
          if (b.enable) {
            if (!map[b.merchantId]) {
              map[b.merchantId] = []
            }
            map[b.merchantId].push(rc)
          }
        })
      })
      return map
    },
    enabled: riskControls.length > 0,
    staleTime: 10000,
  })
  
  const { data, refetch, isPending } = useQuery({
    queryKey: ['operate', 'merchant', pageNumber, pageSize],
    queryFn: () => fetchMerchants(pageNumber, pageSize)
  })

  // 获取所有号码池
  const { data: poolsData, isLoading: isPoolsLoading } = useQuery({
    queryKey: ['operate', 'pool', 1, 100],
    queryFn: () => fetchPools(1, 100),
  })
  const pools = poolsData?.records ?? []

  // 获取所有网关列表
  const { data: gatewaysData, isLoading: isGatewaysLoading } = useQuery({
    queryKey: ['operate', 'gateway', 1, 100],
    queryFn: () => fetchGatewayPage(1, 100),
  })
  const gateways = gatewaysData?.records ?? []

  // 获取所有物理线路/渠道列表
  const { data: channelsData, isLoading: isChannelsLoading } = useQuery({
    queryKey: ['operate', 'channel', 1, 100],
    queryFn: () => fetchChannels(1, 100),
  })
  const channels = channelsData?.records ?? []

  const isDecisionDataLoading = isBindingsLoading || isPoolsLoading || isGatewaysLoading || isChannelsLoading

  // 优雅的客户端精细化组合条件过滤 (Progressive Enhancement)
  const filteredRecords = useMemo(() => {
    let records = data?.records ?? []
    if (queryParams.name) {
      records = records.filter((r: any) => String(r.name).toLowerCase().includes(queryParams.name.toLowerCase().trim()))
    }
    if (queryParams.account) {
      records = records.filter((r: any) => String(r.account).toLowerCase().includes(queryParams.account.toLowerCase().trim()))
    }
    if (queryParams.enable !== undefined) {
      records = records.filter((r: any) => Boolean(r.enable) === Boolean(queryParams.enable))
    }
    return records
  }, [data?.records, queryParams])

  const queryFields = useMemo(() => [
    { key: 'name', label: '商户名称', type: 'text' as const, placeholder: '请输入商户名称搜索' },
    { key: 'account', label: '主账号', type: 'text' as const, placeholder: '请输入主账号搜索' },
    {
      key: 'enable',
      label: '合作状态',
      type: 'select' as const,
      options: [
        { value: true, label: '合作中' },
        { value: false, label: '已中止' },
      ],
    },
  ], [])

  // Load fee rates list for dropdown selection
  const { data: ratesData } = useQuery({
    queryKey: ['operate', 'rate', 1, 100],
    queryFn: () => fetchRates(1, 100),
  })

  const toggleMutation = useMutation({
    mutationFn: async ({ id, enable }: { id: number; enable: boolean }) => toggleMerchantEnable(id, enable),
    onSuccess: async () => {
      message.success('商户状态已更新')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'merchant'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '操作失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: async (ids: number[]) => deleteMerchants(ids),
    onSuccess: async () => {
      message.success('商户已删除')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'merchant'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '删除失败'),
  })

  const saveMutation = useMutation({
    mutationFn: async (values: MerchantFormValues) => {
      // 1. 保存商户主体信息
      const savedMerchant = await saveMerchant({
        id: editingId ?? undefined,
        name: values.name,
        account: values.account,
        expiredTime: values.expiredTime ? dayjs(values.expiredTime).toISOString() : undefined,
        enable: values.enable,
        rateId: values.rateId ? Number(values.rateId) : 0,
        whitelistDomains: values.whitelistDomains || '',
        sipDomain: values.sipDomain || '',
        appKey: values.appKey || undefined,
        appSecret: values.appSecret || undefined,
        maxAgents: values.maxAgents ? Number(values.maxAgents) : 0,
      })

      const merchantId = savedMerchant.id || editingId
      if (!merchantId || merchantId <= 0) {
        throw new Error('商户保存后未获得有效 ID，无法继续关联操作')
      }
      const merchantIdStr = String(merchantId)

      // 2. 检查商户登录账号是否存在，如果不存在则自动创建，如果存在则进行状态同步或密码重置
      const accountsResult = await fetchAccounts(1, 10, values.account)
      const existingAccount = accountsResult?.records?.find((acc: any) => acc.username === values.account)

      if (existingAccount) {
        // 同步商户关联信息及启停状态
        await saveAccount({
          id: existingAccount.id,
          username: values.account,
          merchantId: merchantIdStr,
          accountType: 'merchant_admin',
          roleId: 'merchant_admin',
          enable: values.enable,
        })
        // 如果输入了密码，则进行密码重置
        if (values.password) {
          await resetAccountPassword(existingAccount.id, values.password)
        }
      } else {
        // 创建新的商户管理员账号
        await saveAccount({
          username: values.account,
          password: values.password || '123456',
          merchantId: merchantIdStr,
          accountType: 'merchant_admin',
          roleId: 'merchant_admin',
          enable: values.enable,
        })
      }

      // 3. 处理商户的风控策略关联更新
      if (merchantId) {
        const nextRiskIds = values.riskControlIds ?? []
        const updatePromises = riskControls.map(async (rc: any) => {
          const isCurrentlyBound = allBindingsMap?.[merchantId]?.some((r: any) => r.id === rc.id) ?? false
          const shouldBeBound = nextRiskIds.includes(rc.id)

          if (isCurrentlyBound !== shouldBeBound) {
            const currentBindings = await fetchRiskControlMerchants(rc.id)
            let nextBindings = [...currentBindings]

            if (shouldBeBound) {
              const exists = nextBindings.some((b: any) => b.merchantId === merchantId)
              if (exists) {
                nextBindings = nextBindings.map((b: any) =>
                  b.merchantId === merchantId ? { ...b, enable: true } : b
                )
              } else {
                nextBindings.push({ riskId: rc.id, merchantId, enable: true })
              }
            } else {
              nextBindings = nextBindings.map((b: any) =>
                b.merchantId === merchantId ? { ...b, enable: false } : b
              )
            }
            await saveRiskControlMerchants(rc.id, nextBindings)
          }
        })
        await Promise.all(updatePromises)
      }

      return savedMerchant
    },
    onSuccess: async () => {
      message.success(editingId ? '商户已更新' : '商户已创建')
      setOpen(false)
      setEditingId(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['operate', 'merchant'] })
      await queryClient.invalidateQueries({ queryKey: ['operate', 'risk-control'] })
      await queryClient.invalidateQueries({ queryKey: ['operate', 'merchant-risk-bindings'] })
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  function openCreate() {
    setEditingId(null)
    setOpen(true)
    setTimeout(() => {
      form.resetFields()
      form.setFieldsValue({ enable: true, rateId: 0, whitelistDomains: '', sipDomain: '', maxAgents: 0, riskControlIds: [] })
    }, 0)
  }

  function openEdit(id: number) {
    setEditingId(id)
    const record = data?.records.find((item) => item.id === id)
    const boundRiskIds = allBindingsMap?.[id]?.map((rc: any) => rc.id) ?? []
    setOpen(true)
    setTimeout(() => {
      form.setFieldsValue({
        id,
        name: record?.name ?? '',
        account: record?.account ?? '',
        expiredTime: record?.expiredTime && record.expiredTime !== '-' ? dayjs(record.expiredTime) : undefined,
        enable: Boolean(record?.enable),
        rateId: record?.rateId ? record.rateId : undefined,
        whitelistDomains: record?.whitelistDomains ?? '',
        sipDomain: record?.sipDomain ?? '',
        appKey: record?.appKey ?? '',
        appSecret: record?.appSecret ?? '',
        maxAgents: record?.maxAgents ?? 0,
        riskControlIds: boundRiskIds,
      })
    }, 0)
  }

  const [decisionSubmitting, setDecisionSubmitting] = useState(false)

  function openDecisionCenter(id: number) {
    setDecisionMerchantId(id)
    const boundRiskIds = allBindingsMap?.[id]?.map((rc: any) => rc.id) ?? []
    
    // 初始化号码池的选号策略
    const poolStrategies: Record<number, string> = {}
    pools.forEach((p: any) => {
      poolStrategies[p.id] = p.selectionStrategy || 'CONCURRENCY'
    })
    
    setDecisionModalOpen(true)
    setTimeout(() => {
      decisionForm.resetFields()
      decisionForm.setFieldsValue({
        riskControlIds: boundRiskIds,
        poolStrategies: poolStrategies
      })
      setDecisionFormValues({
        riskControlIds: boundRiskIds,
        poolStrategies: poolStrategies
      })
    }, 0)
  }

  const applyQuickTemplate = (templateType: 'defense' | 'balancer' | 'vip') => {
    const boundRiskIds = templateType === 'balancer' ? [] : riskControls.map((r: any) => r.id)
    const poolStrategies: Record<number, string> = {}
    pools.forEach((p: any) => {
      if (templateType === 'defense') {
        poolStrategies[p.id] = 'CONCURRENCY'
      } else if (templateType === 'balancer') {
        poolStrategies[p.id] = 'RANDOM'
      } else {
        poolStrategies[p.id] = 'PRIORITY'
      }
    })
    
    decisionForm.setFieldsValue({
      riskControlIds: boundRiskIds,
      poolStrategies: poolStrategies
    })
    setDecisionFormValues({
      riskControlIds: boundRiskIds,
      poolStrategies: poolStrategies
    })
    message.success(`已应用「${templateType === 'defense' ? '全防线安全策略' : templateType === 'balancer' ? '高并发负载均衡' : '黄金VIP品质选路'}」模板`)
  }

  const submitDecision = async (values: any) => {
    if (!decisionMerchantId) return
    setDecisionSubmitting(true)
    try {
      // 1. 并发更新风控绑定
      const nextRiskIds = values.riskControlIds ?? []
      const updateRiskPromises = riskControls.map(async (rc: any) => {
        const isCurrentlyBound = allBindingsMap?.[decisionMerchantId]?.some((r: any) => r.id === rc.id) ?? false
        const shouldBeBound = nextRiskIds.includes(rc.id)

        if (isCurrentlyBound !== shouldBeBound) {
          const currentBindings = await fetchRiskControlMerchants(rc.id)
          let nextBindings = [...currentBindings]

          if (shouldBeBound) {
            const exists = nextBindings.some((b: any) => b.merchantId === decisionMerchantId)
            if (exists) {
              nextBindings = nextBindings.map((b: any) =>
                b.merchantId === decisionMerchantId ? { ...b, enable: true } : b
              )
            } else {
              nextBindings.push({ riskId: rc.id, merchantId: decisionMerchantId, enable: true })
            }
          } else {
            nextBindings = nextBindings.map((b: any) =>
              b.merchantId === decisionMerchantId ? { ...b, enable: false } : b
            )
          }
          await saveRiskControlMerchants(rc.id, nextBindings)
        }
      })

      // 2. 并发更新号码池策略
      const nextStrategies = values.poolStrategies ?? {}
      const updatePoolPromises = pools.map(async (p: any) => {
        const nextStrategy = nextStrategies[p.id]
        if (nextStrategy && nextStrategy !== p.selectionStrategy) {
          await savePool({
            id: p.id,
            name: p.name,
            remark: p.remark === '-' ? '' : p.remark,
            type: p.typeId,
            gatewayId: p.gatewayId || undefined,
            enable: p.enable,
            selectionStrategy: nextStrategy
          })
        }
      })

      await Promise.all([...updateRiskPromises, ...updatePoolPromises])
      message.success('一站式选号与安全风控配置更新成功')
      setDecisionModalOpen(false)
      await queryClient.invalidateQueries({ queryKey: ['operate', 'merchant'] })
      await queryClient.invalidateQueries({ queryKey: ['operate', 'risk-control'] })
      await queryClient.invalidateQueries({ queryKey: ['operate', 'merchant-risk-bindings'] })
      await queryClient.invalidateQueries({ queryKey: ['operate', 'pool'] })
    } catch (error) {
      message.error(error instanceof Error ? error.message : '保存失败')
    } finally {
      setDecisionSubmitting(false)
    }
  }

  function getRateName(rateId: number) {
    if (!rateId) return '未绑定费率'
    const found = ratesData?.records.find((r: any) => r.id === rateId)
    return found ? found.rateName : `费率 ID: ${rateId}`
  }

  // 格式化到期时间并提供警示标签
  const renderExpiredTime = (timeStr?: string) => {
    if (!timeStr || timeStr === '-') return <Tag>永不过期</Tag>
    const expired = dayjs(timeStr)
    const now = dayjs()
    const diffDays = expired.diff(now, 'day')
    
    if (diffDays < 0) {
      return (
        <Space size={4}>
          <ExclamationCircleOutlined className="text-red-500" />
          <span className="text-red-500 font-medium text-xs">已过期 ({expired.format('YYYY-MM-DD')})</span>
        </Space>
      )
    } else if (diffDays <= 7) {
      return (
        <Space size={4}>
          <ExclamationCircleOutlined className="text-amber-500" />
          <span className="text-amber-500 font-medium text-xs">即将过期 ({diffDays}天内)</span>
        </Space>
      )
    } else {
      return (
        <Space size={4}>
          <CalendarOutlined className="text-slate-400" />
          <span className="text-slate-500 text-xs">{expired.format('YYYY-MM-DD HH:mm')}</span>
        </Space>
      )
    }
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="flex justify-end mb-2">
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isPending}>刷新</Button>
          <PermissionGate permission="operate:merchant:write">
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增商户
            </Button>
          </PermissionGate>
        </Space>
      </div>

      {/* 商户数据简报统计 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={8}>
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Statistic
              title="商户总注册规模"
              value={data?.total ?? 0}
              prefix={<ShopOutlined className="text-blue-500 mr-1" />}
              suffix="家"
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Statistic
              title="正常运行中商户"
              value={data?.records.filter((r) => r.enable).length ?? 0}
              prefix={<CheckCircleOutlined className="text-emerald-500 mr-1" />}
              valueStyle={{ color: '#3f8600' }}
              suffix="家"
            />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card bordered={false} className="shadow-sm rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
            <Statistic
              title="已超期/超限商户"
              value={data?.records.filter((r) => r.expiredTime && r.expiredTime !== '-' && dayjs(r.expiredTime).isBefore(dayjs())).length ?? 0}
              prefix={<ExclamationCircleOutlined className="text-red-500 mr-1" />}
              valueStyle={{ color: '#cf1322' }}
              suffix="家"
            />
          </Card>
        </Col>
      </Row>

      <QueryBar
        fields={queryFields}
        onSearch={setQueryParams}
        loading={isPending}
      />

      <TableWrap
        title="入驻商户资源列表"
        rowKey="id"
        dataSource={filteredRecords}
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
          { title: '商户 ID', dataIndex: 'id', width: 90, className: 'font-mono text-xs' },
          {
            title: '商户名称',
            dataIndex: 'name',
            render: (v) => <span className="font-semibold text-slate-800 dark:text-slate-200">{v}</span>
          },
          { title: '系统主账号', dataIndex: 'account', render: (v) => <span className="font-mono text-xs">{v}</span> },
          {
            title: '计费费率',
            dataIndex: 'rateId',
            render: (rateId: number) => <Tag color={rateId ? 'blue' : 'default'}>{getRateName(rateId)}</Tag>,
          },
          {
            title: '安全风控策略',
            key: 'riskControls',
            width: 180,
            render: (_, record) => {
              if (isBindingsLoading) {
                return <span style={{ color: '#94a3b8', fontSize: '12px' }}>加载中...</span>
              }
              const bounds = allBindingsMap?.[record.id] ?? []
              if (bounds.length === 0) {
                return <span style={{ color: '#94a3b8', fontSize: '12px' }}>未绑定风控</span>
              }
              return (
                <Space size={[0, 4]} wrap>
                  {bounds.map((rc: any) => (
                    <Tag color="red" key={rc.id} style={{ borderRadius: '4px' }}>
                      {rc.name}
                    </Tag>
                  ))}
                </Space>
              )
            }
          },
          {
            title: '坐席上限',
            dataIndex: 'maxAgents',
            width: 100,
            render: (val: number) => <Tag color="geekblue">{val || 0} 席</Tag>,
          },
          {
            title: '状态',
            dataIndex: 'enable',
            width: 90,
            render: (value: boolean, record: any) => (
              <Switch
                checked={value}
                loading={toggleMutation.isPending}
                onChange={(checked) => toggleMutation.mutate({ id: record.id, enable: checked })}
              />
            ),
          },
          { title: 'API 白名单', dataIndex: 'whitelistDomains', ellipsis: true, render: (val: string) => val || <span className="text-slate-400 text-xs">未限制</span> },
          { title: '到期时段', dataIndex: 'expiredTime', render: (v) => renderExpiredTime(v) },
          {
            title: '管理面板操作',
            width: 250,
            render: (_, record) => (
              <Space size="middle" className="text-xs">
                <Button size="small" type="link" onClick={() => openEdit(record.id)} className="!p-0">
                  编辑
                </Button>
                <Button
                  size="small"
                  type="link"
                  icon={<SlidersOutlined />}
                  onClick={() => openDecisionCenter(record.id)}
                  className="!p-0"
                >
                  选号与风控
                </Button>
                <Button
                  size="small"
                  type="link"
                  icon={<UserSwitchOutlined />}
                  onClick={() => {
                    useAuthStore.getState().impersonate(String(record.id), record.account)
                    message.success(`安全模式：已切换至商户 [${record.id}] ${record.name}`)
                    navigate('/dashboard')
                  }}
                  className="!p-0"
                >
                  模拟登录
                </Button>
                <PermissionGate permission="operate:merchant:delete">
                  <Popconfirm title="确认物理删除该商户主体？其下关联分机及拨号盘关系将被完全解绑！" onConfirm={() => deleteMutation.mutate([record.id])}>
                    <Button size="small" type="link" danger className="!p-0">
                      物理删除
                    </Button>
                  </Popconfirm>
                </PermissionGate>
              </Space>
            ),
          },
        ]}
      />
      
      <Modal
        open={open}
        title={editingId ? '编辑商户配置' : '新增商户入驻'}
        onCancel={() => {
          setOpen(false)
          setEditingId(null)
          form.resetFields()
        }}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnHidden
        width={720}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => saveMutation.mutate(values)}
          initialValues={{ enable: true }}
          onValuesChange={(changedValues) => {
            if (!editingId && changedValues.account !== undefined) {
              const account = changedValues.account;
              const currentSip = form.getFieldValue('sipDomain');
              if (!currentSip || (currentSip.startsWith('sip.') && currentSip.endsWith('.yunshu.com'))) {
                form.setFieldsValue({
                  sipDomain: account ? `sip.${account}.yunshu.com` : ''
                });
              }
            }
          }}
        >
          <Typography.Title level={5} className="!mb-4 border-b pb-2 dark:border-slate-800 flex items-center gap-1.5 text-slate-700 dark:text-slate-300">
            <ShopOutlined /> 基础资质与主账号
          </Typography.Title>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="商户名称" rules={[{ required: true, message: '请输入商户名称' }]}>
                <Input placeholder="输入商户公司/组织主体名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="account" label="主账号" rules={[{ required: true, message: '请输入账号' }]}>
                <Input disabled={Boolean(editingId)} placeholder="例如: merchant_admin" />
              </Form.Item>
            </Col>
          </Row>
          
          <Form.Item
            name="password"
            label={editingId ? '重置密码 (若不修改请留空)' : '主账号密码'}
            rules={editingId ? [] : [{ required: true, message: '请输入密码' }]}
          >
            <Input.Password placeholder={editingId ? '留空表示不修改' : '输入该商户管理员账号的登录密码'} />
          </Form.Item>
          
          <Typography.Title level={5} className="!mb-4 border-b pb-2 dark:border-slate-800 flex items-center gap-1.5 text-slate-700 dark:text-slate-300">
            <CompassOutlined /> 计费费率与服务时限
          </Typography.Title>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="rateId" label="关联计费费率">
                <Select
                  placeholder="选择收费及结算费率"
                  allowClear
                  options={ratesData?.records.map((r: any) => ({
                    value: r.id,
                    label: `${r.rateName} (￥${r.billingPrice.toFixed(4)}/分钟)`,
                  }))}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="expiredTime" label="服务到期时间">
                <DatePicker showTime className="w-full" placeholder="请选择限时时间，留空为永久" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="maxAgents" label="最大坐席并发席位数" rules={[{ required: true, message: '请输入最大坐席数' }]}>
                <InputNumber min={0} className="w-full" placeholder="0代表无上限限制" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="enable" label="启用合作状态" valuePropName="checked">
                <Switch checkedChildren="合作中" unCheckedChildren="已中止" />
              </Form.Item>
            </Col>
          </Row>

          <Typography.Title level={5} className="!mb-4 border-b pb-2 dark:border-slate-800 flex items-center gap-1.5 text-slate-700 dark:text-slate-300">
            <SafetyCertificateOutlined /> 网络边界与 SIP 安全
          </Typography.Title>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="whitelistDomains" label="API 限制 IP / 域名白名单 (可选)">
                <Input placeholder="英文逗号隔开，留空代表不限制外网对接" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="sipDomain" label="SIP 专享注册域 (Sip Realm)">
                <Input placeholder="默认: sip.账号.yunshu.com" />
              </Form.Item>
            </Col>
          </Row>
          
          <Row gutter={16}>
            <Col span={24}>
              <Form.Item name="riskControlIds" label="关联呼叫安全风控策略">
                <Select
                  mode="multiple"
                  placeholder="选择该商户生效的外呼选号风控限制策略"
                  allowClear
                  options={riskControls.map((r: any) => ({
                    value: r.id,
                    label: `${r.name} (策略 ID: ${r.id})`,
                  }))}
                />
              </Form.Item>
            </Col>
          </Row>

          {editingId && (
            <div className="p-4 mb-4 rounded-lg bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800">
              <Typography.Text strong className="block mb-3 text-sm text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
                <KeyOutlined /> API 对接开发凭证
              </Typography.Text>
              <Form.Item name="appKey" label="系统分配 AppKey" className="!mb-3 font-mono text-xs">
                <Input 
                  readOnly 
                  className="font-mono bg-slate-50/50 dark:bg-zinc-800 text-slate-800 dark:text-zinc-200" 
                  suffix={
                    <Typography.Link
                      onClick={() => {
                        const val = form.getFieldValue('appKey');
                        if (val) {
                          navigator.clipboard.writeText(val);
                          message.success('AppKey 已复制');
                        }
                      }}
                    >
                      复制
                    </Typography.Link>
                  }
                />
              </Form.Item>
              <Form.Item name="appSecret" label="系统分配 AppSecret" className="!mb-1 font-mono text-xs">
                <Input.Password 
                  readOnly 
                  className="font-mono bg-slate-50/50 dark:bg-zinc-800 text-slate-800 dark:text-zinc-200" 
                />
              </Form.Item>
              <div className="flex justify-end mb-2">
                <Typography.Link
                  className="text-xs"
                  onClick={() => {
                    const val = form.getFieldValue('appSecret');
                    if (val) {
                      navigator.clipboard.writeText(val);
                      message.success('AppSecret 已复制');
                    }
                  }}
                >
                  一键复制 AppSecret
                </Typography.Link>
              </div>
            </div>
          )}
        </Form>
      </Modal>

      <Modal
        open={decisionModalOpen}
        title={
          <div className="flex items-center gap-2 text-slate-800 dark:text-zinc-100 font-semibold text-base border-b pb-2.5 dark:border-zinc-800">
            <FundProjectionScreenOutlined className="text-blue-500" />
            <span>商户呼叫选号与安全风控一站式配置中心</span>
          </div>
        }
        onCancel={() => setDecisionModalOpen(false)}
        onOk={() => decisionForm.submit()}
        confirmLoading={decisionSubmitting}
        destroyOnHidden
        width={950}
        okText="一键应用配置"
        cancelText="取消"
      >
        {isDecisionDataLoading ? (
          <div className="flex flex-col items-center justify-center py-24 w-full space-y-4">
            <Spin size="large" />
            <div className="text-slate-500 dark:text-zinc-400 text-xs font-mono animate-pulse">
              正在安全构建全局呼叫决策及拓扑数据链...
            </div>
          </div>
        ) : (
          <Row gutter={24} style={{ paddingTop: '8px' }}>
          {/* 左侧配置面板 */}
          <Col span={12} className="border-r pr-5 dark:border-zinc-800" style={{ maxHeight: '600px', overflowY: 'auto' }}>
            <Form
              form={decisionForm}
              layout="vertical"
              onFinish={submitDecision}
              onValuesChange={(changedValues, allValues) => {
                setDecisionFormValues(allValues)
              }}
            >
              {/* 智能快捷模板 */}
              <div className="mb-5 p-3.5 rounded-xl bg-slate-50/60 dark:bg-zinc-950/40 border border-slate-200/50 dark:border-zinc-800/80">
                <div className="text-xs font-bold text-slate-500 dark:text-zinc-400 mb-2.5 flex items-center gap-1">
                  <SlidersOutlined /> 一键快捷配置智能模板
                </div>
                <div className="flex gap-2">
                  <Button 
                    size="small" 
                    type="dashed" 
                    danger 
                    className="hover:scale-[1.02] transition-transform duration-200" 
                    onClick={() => applyQuickTemplate('defense')}
                  >
                    🛡️ 全防御安全策略
                  </Button>
                  <Button 
                    size="small" 
                    type="dashed" 
                    className="hover:scale-[1.02] transition-transform duration-200 text-blue-500 border-blue-200 hover:text-blue-600 hover:border-blue-500" 
                    onClick={() => applyQuickTemplate('balancer')}
                  >
                    ⚖️ 负载均衡分流
                  </Button>
                  <Button 
                    size="small" 
                    type="dashed" 
                    className="hover:scale-[1.02] transition-transform duration-200 text-amber-500 border-amber-200 hover:text-amber-600 hover:border-amber-500" 
                    onClick={() => applyQuickTemplate('vip')}
                  >
                    👑 VIP黄金选路
                  </Button>
                </div>
              </div>

              <div className="mb-4">
                <Typography.Title level={5} className="!mb-3 flex items-center gap-1.5 text-slate-800 dark:text-zinc-200">
                  <SafetyOutlined className="text-red-500" />
                  <span>1. 呼叫安全风控拦截绑定</span>
                </Typography.Title>
                <Form.Item name="riskControlIds" noStyle>
                  <Select
                    mode="multiple"
                    placeholder="选择需要生效的外呼风控限制策略"
                    allowClear
                    style={{ width: '100%' }}
                    options={riskControls.map((r: any) => ({
                      value: r.id,
                      label: `${r.name} (策略 ID: ${r.id})`,
                    }))}
                  />
                </Form.Item>
                <div className="text-slate-400 dark:text-zinc-500 text-xs mt-1.5 pl-0.5">
                  绑定后系统将实时进行呼出频次、地市盲区、黑名单防火墙拦截。
                </div>
              </div>

              <Divider className="my-4 dark:border-zinc-800" />

              <div>
                <Typography.Title level={5} className="!mb-3 flex items-center gap-1.5 text-slate-800 dark:text-zinc-200">
                  <SlidersOutlined className="text-blue-500" />
                  <span>2. 全局号码池分配调度策略</span>
                </Typography.Title>

                {pools.length === 0 ? (
                  <div className="p-6 text-center rounded-lg bg-slate-50 dark:bg-zinc-900 border border-dashed border-slate-300 dark:border-zinc-800">
                    <Typography.Text type="secondary" className="text-xs block mb-2">
                      系统尚未配置任何号码池。请先前往号码池管理页面创建。
                    </Typography.Text>
                    <Button size="small" type="primary" onClick={() => navigate('/operate/risk-control')}>
                      去配置号码池
                    </Button>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {pools.map((p: any) => (
                      <div key={p.id} className="p-3.5 rounded-lg bg-slate-50 dark:bg-zinc-900/50 border border-slate-200/60 dark:border-zinc-800">
                        <div className="flex justify-between items-center mb-2">
                          <span className="font-semibold text-slate-800 dark:text-zinc-200 text-sm flex items-center gap-1.5">
                            <PartitionOutlined className="text-slate-500" />
                            {p.name}
                          </span>
                          <Tag color="blue" style={{ border: 'none', borderRadius: '4px', fontSize: '11px' }}>
                            网关 ID: {p.gatewayId || '未绑定'}
                          </Tag>
                        </div>
                        <Form.Item name={['poolStrategies', String(p.id)]} noStyle>
                          <Select placeholder="选择分配调度策略" style={{ width: '100%' }}>
                            <Select.Option value="CONCURRENCY">
                              CONCURRENCY - 最低并发优先 (物理并发最优分配)
                            </Select.Option>
                            <Select.Option value="RANDOM">
                              RANDOM - 随机轮询分配 (流量全局平摊)
                            </Select.Option>
                            <Select.Option value="PRIORITY">
                              PRIORITY - 优先级路由 (低ID/核心优质通道首选)
                            </Select.Option>
                          </Select>
                        </Form.Item>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </Form>
          </Col>

          {/* 右侧实时可视化拓扑决策链 */}
          <Col span={12} className="pl-5">
            <div 
              style={{ 
                maxHeight: '600px', 
                overflowY: 'auto', 
                padding: '20px',
                borderRadius: '16px',
                backgroundImage: 'radial-gradient(rgba(0,0,0,0.03) 1.2px, transparent 0)',
                backgroundSize: '16px 16px',
                position: 'relative'
              }} 
              className="bg-slate-50/70 dark:bg-zinc-950/20 backdrop-blur-md border border-slate-200/60 dark:border-zinc-800/80 shadow-inner rounded-2xl"
            >
              <div className="flex justify-between items-center mb-4">
                <span className="font-semibold text-slate-800 dark:text-zinc-200 text-sm flex items-center gap-1.5">
                  <FundProjectionScreenOutlined className="text-emerald-500" />
                  呼叫选号与风控实时路由决策拓扑
                </span>
                <Tag color="processing" className="animate-pulse">Live</Tag>
              </div>

              <div className="flex flex-col items-center">
                {/* 1. 外呼源头 */}
                <div className="w-full p-4 rounded-xl bg-emerald-50/50 dark:bg-emerald-950/10 border border-emerald-100 dark:border-emerald-900/40 shadow-sm transition-all duration-300">
                  <div className="flex justify-between items-start">
                    <div className="flex items-center gap-2">
                      <div className="p-1.5 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
                        <ShopOutlined />
                      </div>
                      <div>
                        <div className="font-bold text-slate-800 dark:text-zinc-100 text-sm">商户外呼发起</div>
                        <div className="text-slate-500 dark:text-zinc-400 text-xs mt-0.5">
                          商户：{data?.records.find((m) => m.id === decisionMerchantId)?.name ?? '未知'} (ID: {decisionMerchantId})
                        </div>
                      </div>
                    </div>
                    <Tag color="success" style={{ border: 'none', borderRadius: '4px', fontSize: '10px' }} className="font-mono">
                      INITIATED
                    </Tag>
                  </div>
                </div>

                <div className="flex justify-center my-3 text-slate-400">
                  <ArrowRightOutlined rotate={90} className="text-emerald-400 dark:text-emerald-600" style={{ fontSize: '16px' }} />
                </div>

                {/* 2. 第一道防线 (安全拦截层) */}
                {(() => {
                  const selectedRiskIds = decisionFormValues.riskControlIds ?? []
                  if (selectedRiskIds.length === 0) {
                    return (
                      <div className="w-full p-4 rounded-xl bg-amber-50/60 dark:bg-amber-950/10 border border-amber-200/80 dark:border-amber-900/30 shadow-sm transition-all duration-300">
                        <div className="flex justify-between items-start">
                          <div className="flex items-center gap-2">
                            <div className="p-1.5 rounded-lg bg-amber-500/10 text-amber-600 dark:text-amber-400">
                              <ExclamationCircleOutlined />
                            </div>
                            <div>
                              <div className="font-bold text-amber-800 dark:text-amber-200 text-sm flex items-center gap-1">
                                ⚠️ 安全风控：未启用防护
                              </div>
                              <div className="text-slate-500 dark:text-zinc-400 text-xs mt-1.5 leading-relaxed">
                                警告：当前呼叫未受任何安全防火墙保护！容易引发高频投诉叫停、黑名单封波和运营商地市盲区拦截风控，封号概率极高。
                              </div>
                            </div>
                          </div>
                        </div>
                      </div>
                    )
                  }

                  return (
                    <div className="w-full p-4 rounded-xl bg-red-50/50 dark:bg-rose-950/10 border border-red-100 dark:border-rose-900/40 shadow-sm transition-all duration-300">
                      <div className="flex justify-between items-start">
                        <div className="flex items-center gap-2 w-full">
                          <div className="p-1.5 rounded-lg bg-red-500/10 text-red-600 dark:text-red-400 flex-shrink-0">
                            <SafetyOutlined />
                          </div>
                          <div className="w-full">
                            <div className="font-bold text-red-800 dark:text-red-300 text-sm flex justify-between items-center">
                              <span>🛡️ 安全防御拦截层</span>
                              <span className="text-xs font-normal text-red-500">已激活 {selectedRiskIds.length} 个风控盾牌</span>
                            </div>
                            <div className="mt-2.5 space-y-1.5">
                              {selectedRiskIds.map((rid: number) => {
                                const rc = riskControls.find((r: any) => r.id === rid)
                                return (
                                  <div key={rid} className="flex justify-between items-center text-xs bg-white/60 dark:bg-zinc-800/40 px-2.5 py-1.5 rounded border border-red-100/50 dark:border-rose-950">
                                    <span className="font-medium text-red-700 dark:text-rose-400">{rc?.name ?? `风控策略 #${rid}`}</span>
                                    <Tag color="red" style={{ border: 'none', fontSize: '9px', lineHeight: '14px', height: '16px' }}>拦截机制生效</Tag>
                                  </div>
                                )
                              })}
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  )
                })()}

                <div className="flex justify-center my-3 text-slate-400">
                  <ArrowRightOutlined rotate={90} className="text-blue-400 dark:text-blue-600" style={{ fontSize: '16px' }} />
                </div>

                {/* 3. 第二道防线 (选号调度层) */}
                <div className="w-full p-4 rounded-xl bg-blue-50/50 dark:bg-blue-950/10 border border-blue-100 dark:border-blue-900/40 shadow-sm transition-all duration-300">
                  <div className="flex items-start gap-2 w-full">
                    <div className="p-1.5 rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400 flex-shrink-0">
                      <SlidersOutlined />
                    </div>
                    <div className="w-full">
                      <div className="font-bold text-blue-800 dark:text-blue-300 text-sm flex justify-between items-center">
                        <span>🎯 智能选号调度策略</span>
                        <span className="text-xs font-normal text-blue-500">动态计算分配</span>
                      </div>
                      {pools.length === 0 ? (
                        <div className="text-slate-400 dark:text-zinc-500 text-xs mt-2">
                          无可用号码池配置调度
                        </div>
                      ) : (
                        <div className="mt-2.5 space-y-2.5">
                          {pools.map((p: any) => {
                            const strategy = decisionFormValues.poolStrategies?.[p.id] || p.selectionStrategy || 'CONCURRENCY'
                            let strategyName = ''
                            let strategyDesc = ''
                            let strategyColor = 'cyan'
                            if (strategy === 'CONCURRENCY') {
                              strategyName = '并发优先 (CONCURRENCY)'
                              strategyDesc = '实时扫描 Redis 通信活跃锁，自动挑选最低物理并发的线路，呼出成功率最稳健'
                              strategyColor = 'emerald'
                            } else if (strategy === 'RANDOM') {
                              strategyName = '随机分配 (RANDOM)'
                              strategyDesc = '多路并发随机轮询分发，全局物理平摊外呼负载，最适合大规模防物理叫停'
                              strategyColor = 'blue'
                            } else if (strategy === 'PRIORITY') {
                              strategyName = '优先级路由 (PRIORITY)'
                              strategyDesc = '按号码预设的静态优先级降序依次抢占，首选物理最优质的核心大通道'
                              strategyColor = 'purple'
                            }

                            return (
                              <div key={p.id} className="text-xs bg-white dark:bg-zinc-800/30 p-3 rounded-lg border border-blue-100 dark:border-blue-900/50 shadow-sm transition-all duration-200">
                                <div className="flex justify-between items-center mb-1.5 font-bold text-slate-800 dark:text-zinc-200">
                                  <span className="flex items-center gap-1">
                                    <PartitionOutlined className="text-blue-500" />
                                    {p.name}
                                  </span>
                                  <Tag color={strategyColor} style={{ border: 'none', fontSize: '9px', margin: 0 }}>{strategyName}</Tag>
                                </div>
                                <div className="text-slate-500 dark:text-zinc-400 text-[10px] leading-relaxed mt-1 border-b pb-2 dark:border-zinc-800/80">
                                  {strategyDesc}
                                </div>
                                
                                {/* 智能选号仿真插槽指示 */}
                                <div className="mt-2 pt-1 flex justify-between items-center">
                                  <span className="text-[9.5px] text-slate-400 dark:text-zinc-500 flex items-center gap-1 font-mono">
                                    <span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse" />
                                    ATOMIC ROTOR ACTIVE
                                  </span>
                                  <div className="flex gap-1.5">
                                    <span className="px-1.5 py-0.5 rounded text-[8px] bg-slate-100 hover:bg-slate-200/80 text-slate-500 font-mono border border-slate-200/60 dark:bg-zinc-800/80 dark:text-zinc-400 dark:border-zinc-700/50">Slot-A</span>
                                    <span className="px-1.5 py-0.5 rounded text-[8px] bg-blue-100 border border-blue-200 text-blue-700 font-mono font-semibold shadow-xs dark:bg-blue-950/40 dark:border-blue-800 dark:text-blue-400">Slot-B</span>
                                    <span className="px-1.5 py-0.5 rounded text-[8px] bg-slate-100 hover:bg-slate-200/80 text-slate-500 font-mono border border-slate-200/60 dark:bg-zinc-800/80 dark:text-zinc-400 dark:border-zinc-700/50">Slot-C</span>
                                  </div>
                                </div>
                              </div>
                            )
                          })}
                        </div>
                      )}
                    </div>
                  </div>
                </div>

                <div className="flex justify-center my-3 text-slate-400">
                  <ArrowRightOutlined rotate={90} className="text-purple-400 dark:text-purple-600" style={{ fontSize: '16px' }} />
                </div>

                {/* 4. 底层物理网关 & 落地物理线路 */}
                <div className="w-full p-4 rounded-xl bg-purple-50/50 dark:bg-purple-950/10 border border-purple-100 dark:border-purple-900/40 shadow-sm transition-all duration-300">
                  <div className="flex items-start gap-2 w-full">
                    <div className="p-1.5 rounded-lg bg-purple-500/10 text-purple-600 dark:text-purple-400 flex-shrink-0">
                      <GatewayOutlined />
                    </div>
                    <div className="w-full">
                      <div className="font-bold text-purple-800 dark:text-purple-300 text-sm flex justify-between items-center mb-2.5">
                        <span>🔌 落地物理网关及承载渠道</span>
                        <Tag color="purple" style={{ border: 'none', borderRadius: '4px', fontSize: '9px' }}>SIP TRUNK</Tag>
                      </div>
                      {pools.length === 0 ? (
                        <div className="text-slate-400 dark:text-zinc-500 text-xs mt-2">
                          物理网关断开连接
                        </div>
                      ) : (
                        <div className="mt-2.5 space-y-2.5">
                          {pools.map((p: any) => {
                            const gw = gateways.find((g: any) => g.id === p.gatewayId)
                            return (
                              <div key={p.id} className="text-xs bg-gradient-to-br from-slate-100 via-slate-200/80 to-slate-200 border border-slate-300 text-slate-800 dark:bg-gradient-to-br dark:from-zinc-950 dark:to-zinc-900 dark:border-zinc-800 dark:text-zinc-100 p-3 rounded-lg font-mono shadow-sm dark:shadow-inner transition-all duration-300">
                                <div className="flex justify-between items-center border-b border-slate-300/80 dark:border-zinc-800/80 pb-1.5 mb-2">
                                  <span className="text-[10px] text-blue-600 dark:text-blue-450 font-bold flex items-center gap-1.5">
                                    <span className={`w-1.5 h-1.5 rounded-full ${gw ? 'bg-emerald-500 animate-pulse' : 'bg-rose-500 animate-ping'} inline-block`} />
                                    CHASSIS-RACK UNIT #{p.id}
                                  </span>
                                  <span className="text-[9px] text-slate-500 dark:text-zinc-500">GATEWAY: #{p.gatewayId || 'NONE'}</span>
                                </div>
                                <div className="grid grid-cols-2 gap-2 text-[10px]">
                                  <div>
                                    <span className="text-slate-500 dark:text-zinc-500">SYS-GW:</span> <span className={gw ? "text-emerald-600 dark:text-emerald-450 font-semibold" : "text-rose-600 dark:text-rose-450 font-semibold"}>{gw?.name ?? 'OFFLINE'}</span>
                                  </div>
                                  <div>
                                    <span className="text-slate-500 dark:text-zinc-500">LINE-TRUNK:</span> <span className="text-purple-600 dark:text-purple-400 font-semibold">{gw?.region || 'UNASSIGNED'}</span>
                                  </div>
                                </div>
                                {/* 机架指示灯与插槽模拟 */}
                                <div className="mt-2.5 pt-2 border-t border-slate-300/60 dark:border-zinc-850/80 flex justify-between items-center">
                                  <div className="flex items-center gap-1">
                                    <span className="text-[9px] text-slate-600 dark:text-zinc-400 font-semibold">SLOTS:</span>
                                    <div className="flex gap-0.5">
                                      <span className="w-2.5 h-1 bg-emerald-500 dark:bg-emerald-400 rounded-sm opacity-90" />
                                      <span className="w-2.5 h-1 bg-emerald-500 dark:bg-emerald-400 rounded-sm opacity-90" />
                                      <span className="w-2.5 h-1 bg-emerald-500 dark:bg-emerald-400 rounded-sm opacity-50" />
                                      <span className="w-2.5 h-1 bg-slate-300/80 dark:bg-zinc-800 rounded-sm" />
                                      <span className="w-2.5 h-1 bg-slate-300/80 dark:bg-zinc-800 rounded-sm" />
                                    </div>
                                  </div>
                                  <span className="text-[9px] text-slate-600 dark:text-zinc-400 flex items-center gap-1 font-semibold">
                                    SIP LINK: <span className="w-1.5 h-1.5 rounded-full bg-blue-500 inline-block animate-pulse" />
                                  </span>
                                </div>
                              </div>
                            )
                          })}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </Col>
        </Row>
        )}
      </Modal>
    </Space>
  )
}
