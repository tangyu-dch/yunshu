import { Button, Form, Select, Space, Tag, Typography, message, Card, Input, DatePicker, Table, Alert, Row, Col, Switch } from 'antd'
import { SafetyCertificateOutlined, ReloadOutlined, FilterOutlined, SearchOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useEffect } from 'react'
import dayjs from 'dayjs'
import { TableWrap } from '@/components/TableWrap'
import { fetchIPBlockConfig, saveIPBlockConfig, fetchIPBlockLogs, lookupIPAddress } from '@/api/operate'

const { Option } = Select
const { RangePicker } = DatePicker

const countryMap: Record<string, string> = {
  CN: '中国',
  HK: '中国香港',
  TW: '中国台湾',
  MO: '中国澳门',
  US: '美国',
  DE: '德国',
  GB: '英国',
  FR: '法国',
  JP: '日本',
  KR: '韩国',
  RU: '俄罗斯',
  CA: '加拿大',
  AU: '澳大利亚',
  SG: '新加坡',
  NL: '荷兰',
  IN: '印度',
  BR: '巴西',
  VN: '越南',
  MY: '马来西亚',
  TH: '泰国',
  PH: '菲律宾',
  ID: '印尼',
  KH: '柬埔寨',
  MM: '缅甸',
  LA: '老挝',
  BD: '孟加拉国',
  PK: '巴基斯坦',
  AE: '阿联酋',
  SA: '沙特阿拉伯',
  IR: '伊朗',
  IQ: '伊拉克',
  IL: '以色列',
  TR: '土耳其',
  EG: '埃及',
  ZA: '南非',
  IT: '意大利',
  ES: '西班牙',
  PT: '葡萄牙',
  CH: '瑞士',
  SE: '瑞典',
  NO: '挪威',
  FI: '芬兰',
  DK: '丹麦',
  IE: '爱尔兰',
  BE: '比利时',
  AT: '奥地利',
  PL: '波兰',
  UA: '乌克兰',
  RO: '罗马尼亚',
  GR: '希腊',
  NZ: '新西兰',
  MX: '墨西哥',
  CO: '哥伦比亚',
  AR: '阿根廷',
  CL: '智选',
  PE: '秘鲁',
  VE: '委内瑞拉',
  AD: '安道尔',
  AF: '阿富汗',
  AG: '安提瓜和巴布达',
  AI: '安圭拉',
  AL: '阿尔巴尼亚',
  AM: '亚美尼亚',
  AO: '安哥拉',
  AQ: '南极洲',
  AS: '美属萨摩亚',
  AW: '阿鲁巴',
  AX: '奥兰群岛',
  AZ: '阿塞拜疆',
  BA: '波黑',
  BB: '巴巴多斯',
  BF: '布基纳法索',
  BG: '保加利亚',
  BH: '巴林',
  BI: '布隆迪',
  BJ: '贝宁',
  BL: '圣巴泰勒米',
  BM: '百慕大',
  BN: '文莱',
  BO: '玻利维亚',
  BQ: '荷属加勒比',
  BS: '巴哈马',
  BT: '不丹',
  BV: '布韦岛',
  BW: '博茨瓦纳',
  BY: '白俄罗斯',
  BZ: '伯利兹',
  CC: '科科斯群岛',
  CD: '刚果(金)',
  CF: '中非',
  CG: '刚果(布)',
  CI: '科特迪瓦',
  CK: '库克群岛',
  CM: '喀麦隆',
  CR: '哥斯达黎加',
  CU: '古巴',
  CV: '佛得角',
  CW: '库拉索',
  CX: '圣诞岛',
  CY: '塞浦路斯',
  CZ: '捷克',
  DJ: '吉布提',
  DM: '多米尼克',
  DO: '多米尼加',
  DZ: '阿尔及利亚',
  EC: '厄瓜多尔',
  EE: '爱沙尼亚',
  EH: '西撒哈拉',
  ER: '厄立特里亚',
  ET: '埃塞俄比亚',
  FJ: '斐济',
  FK: '马尔维纳斯群岛',
  FM: '密克罗尼西亚',
  FO: '法罗群岛',
  GA: '加蓬',
  GD: '格林纳达',
  GE: '格鲁吉亚',
  GF: '法属圭亚那',
  GG: '根西岛',
  GH: '加纳',
  GI: '直布罗陀',
  GL: '格陵兰',
  GM: '冈比亚',
  GN: '几内亚',
  GP: '瓜德罗普',
  GQ: '赤道几内亚',
  GS: '南乔治亚和南桑威奇群岛',
  GT: '危地马拉',
  GU: '关岛',
  GW: '几内亚比绍',
  GY: '圭亚那',
  HM: '赫德岛和麦克唐纳群岛',
  HN: '洪都拉斯',
  HR: '克罗地亚',
  HT: '海地',
  HU: '匈牙利',
  IM: '马恩岛',
  IO: '英属印度洋领地',
  IS: '冰岛',
  JE: '泽西岛',
  JM: '牙买加',
  JO: '约旦',
  KE: '肯尼亚',
  KG: '吉尔吉斯斯坦',
  KI: '基里巴斯',
  KM: '科摩罗',
  KN: '圣基茨和尼维斯',
  KP: '朝鲜',
  KW: '科威特',
  KY: '开曼群岛',
  KZ: '哈萨克斯坦',
  LB: '黎巴嫩',
  LC: '圣卢西亚',
  LI: '列支敦士登',
  LK: '斯里兰卡',
  LR: '利比里亚',
  LS: '莱索托',
  LT: '立陶宛',
  LU: '卢森堡',
  LV: '拉脱维亚',
  LY: '利比亚',
  MA: '摩洛哥',
  MC: '摩纳哥',
  MD: '摩尔多瓦',
  ME: '黑山',
  MF: '圣马丁',
  MG: '马达加斯加',
  MH: '马绍尔群岛',
  MK: '北马其顿',
  ML: '马里',
  MN: '蒙古',
  MP: '北马里亚纳群岛',
  MQ: '马提尼克',
  MR: '毛里塔尼亚',
  MS: '蒙特塞拉特',
  MT: '马耳他',
  MU: '毛里求斯',
  MV: '马尔代夫',
  MW: '马拉维',
  MZ: '莫桑比克',
  NA: '纳米比亚',
  NC: '新喀里多尼亚',
  NE: '尼日尔',
  NF: '诺福克岛',
  NG: '尼日利亚',
  NI: '尼加拉瓜',
  NP: '尼泊尔',
  NR: '瑙鲁',
  NU: '纽埃',
  OM: '阿曼',
  PA: '巴拿马',
  PF: '法属波利尼西亚',
  PG: '巴布亚新几内亚',
  PM: '圣皮埃尔和密克隆',
  PN: '皮特凯恩群岛',
  PR: '波多黎各',
  PS: '巴勒斯坦',
  PW: '帕劳',
  PY: '巴拉圭',
  QA: '卡塔尔',
  RE: '留尼汪',
  RS: '塞尔维亚',
  RW: '卢旺达',
  SB: '所罗门群岛',
  SC: '塞舌尔',
  SD: '苏丹',
  SH: '圣赫勒拿',
  SI: '斯洛文尼亚',
  SJ: '斯瓦尔巴和扬马延',
  SK: '斯洛伐克',
  SL: '塞拉利昂',
  SM: '圣马力诺',
  SN: '塞内加尔',
  SO: '索马里',
  SR: '苏里南',
  SS: '南苏丹',
  ST: '圣多美和普林西比',
  SV: '萨尔瓦多',
  SX: '荷属圣马丁',
  SY: '叙利亚',
  SZ: '斯威士兰',
  TC: '特克斯和凯科斯群岛',
  TD: '乍得',
  TF: '法属南部领地',
  TG: '多哥',
  TJ: '塔吉克斯坦',
  TK: '托克劳',
  TL: '东帝汶',
  TM: '土库曼斯坦',
  TN: '突尼斯',
  TO: '汤加',
  TT: '特立尼达和多巴哥',
  TV: '图瓦卢',
  TZ: '坦桑尼亚',
  UG: '乌干达',
  UM: '美属本土外小岛屿',
  UY: '乌拉圭',
  UZ: '乌兹别克斯坦',
  VA: '梵蒂冈',
  VC: '圣文森特和格林纳丁斯',
  VG: '美属维尔京群岛',
  VI: '美属维尔京群岛',
  VU: '瓦努阿图',
  WF: '瓦利斯和富图纳',
  WS: '萨摩亚',
  XK: '科索沃',
  YE: '也门',
  YT: '马约特',
  ZM: '赞比亚',
  ZW: '津巴布韦',
}

const commonCountries = ['CN', 'US', 'DE', 'GB', 'FR', 'JP', 'KR', 'RU', 'CA', 'AU', 'SG', 'HK', 'TW', 'MO', 'NL', 'IN', 'BR', 'VN', 'MY', 'TH']

const getFlagEmoji = (countryCode: string): string => {
  const code = countryCode.toUpperCase()
  if (code === 'UNKNOWN' || code === 'PRIVATE' || code === 'FOREIGN') return '🌐'
  try {
    const codePoints = code
      .split('')
      .map((char) => 127397 + char.charCodeAt(0))
    return String.fromCodePoint(...codePoints)
  } catch (e) {
    return '🌐'
  }
}

function isPrivateIP(ip: string): boolean {
  if (ip === 'localhost' || ip === '127.0.0.1') return true
  const parts = ip.split('.').map(Number)
  if (parts.length !== 4 || parts.some(isNaN)) return false
  if (parts[0] === 10) return true
  if (parts[0] === 172 && parts[1] >= 16 && parts[1] <= 31) return true
  if (parts[0] === 192 && parts[1] === 168) return true
  if (parts[0] === 127) return true
  return false
}

const renderCountry = (code: string) => {
  if (!code) return '-'
  const upperCode = code.toUpperCase()
  if (upperCode === 'PRIVATE') return '🔒 内网私有网段'
  if (upperCode === 'UNKNOWN') return '🌐 未知地区'
  const flag = getFlagEmoji(upperCode)
  const name = countryMap[upperCode] || `境外地区`
  return `${flag} ${name} (${upperCode})`
}

export function IPBlockPage() {
  const queryClient = useQueryClient()
  const [pageNumber, setPageNumber] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  // 过滤状态
  const [ipFilter, setIpFilter] = useState('')
  const [countryFilter, setCountryFilter] = useState<string | undefined>(undefined)
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null)

  // IP 查询状态
  const [searchIp, setSearchIp] = useState('')
  const [lookupResult, setLookupResult] = useState<{ ip: string; countryCode: string } | null>(null)
  const [isSearching, setIsSearching] = useState(false)

  const handleLookup = async () => {
    if (!searchIp.trim()) {
      message.warning('请输入待查询的 IP 地址')
      return
    }
    setIsSearching(true)
    try {
      const res = await lookupIPAddress(searchIp.trim())
      setLookupResult(res)
    } catch (err: any) {
      message.error(err?.message || 'IP 查询失败')
      setLookupResult(null)
    } finally {
      setIsSearching(false)
    }
  }

  const [form] = Form.useForm<{ countries: string[]; onlyAllowCn: boolean }>()

  // 获取黑名单配置
  const { data: configData, isLoading: isConfigLoading } = useQuery({
    queryKey: ['operate', 'ip-block', 'config'],
    queryFn: fetchIPBlockConfig,
  })

  // 绑定配置表单初始值
  useEffect(() => {
    if (configData) {
      const parts = configData.countries
        ? configData.countries.split(',').map((c: string) => c.trim().toUpperCase()).filter(Boolean)
        : []
      form.setFieldsValue({
        countries: parts,
        onlyAllowCn: !!configData.onlyAllowCn,
      })
    }
  }, [configData, form])

  // 保存黑名单配置 mutation
  const saveConfigMutation = useMutation({
    mutationFn: async (values: { countries: string[]; onlyAllowCn: boolean }) => {
      return saveIPBlockConfig({
        countries: values.countries ? values.countries.join(',') : '',
        onlyAllowCn: !!values.onlyAllowCn,
      })
    },
    onSuccess: async () => {
      message.success('IP 地理拦截规则与放行配置更新成功，内核 ipset/iptables 已热同步！')
      await queryClient.invalidateQueries({ queryKey: ['operate', 'ip-block', 'config'] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '更新配置失败')
    },
  })

  // 查询拦截日志
  const { data: logData, isLoading: isLogsLoading, refetch: refetchLogs } = useQuery({
    queryKey: ['operate', 'ip-block', 'logs', pageNumber, pageSize, ipFilter, countryFilter, dateRange],
    queryFn: () =>
      fetchIPBlockLogs({
        pageNumber,
        pageSize,
        ip: ipFilter || undefined,
        countryCode: countryFilter || undefined,
        startTime: dateRange?.[0] ? dateRange[0].toISOString() : undefined,
        endTime: dateRange?.[1] ? dateRange[1].toISOString() : undefined,
      }),
  })

  const handleResetFilters = () => {
    setIpFilter('')
    setCountryFilter(undefined)
    setDateRange(null)
    setPageNumber(1)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 说明区 */}
      <Alert
        message="内核级 IP 地理防火墙说明"
        description="通过动态配置防火墙拦截外部非国内或特定高风险国家/地区的攻击与扫描流量。本功能基于 iptables 与 ipset 机制。当来自被封禁国家的 IP 向本服务器的 SIP（5060）或 WebSocket（5066）端口发起连接时，内核网络层将直接丢弃（DROP）数据包并生成审计日志。内网 IP 已经默认放行。"
        type="info"
        showIcon
      />

      {/* 规则配置与IP分析双栏区 */}
      <Row gutter={[16, 16]}>
        <Col xs={24} md={14}>
          <Card
            title={
              <Space>
                <SafetyCertificateOutlined className="text-blue-500" />
                <span>拦截国家/地区黑名单与放行规则</span>
              </Space>
            }
            loading={isConfigLoading}
            variant="borderless"
            style={{ height: '100%' }}
          >
            <Form
              form={form}
              layout="vertical"
              onFinish={(values) => {
                saveConfigMutation.mutate(values)
              }}
            >
              <Form.Item
                name="onlyAllowCn"
                valuePropName="checked"
                label="仅放行国内 IP（强力白名单模式）"
                extra="开启后，除了中国大陆 (CN) 以及私有/内网网段外，其他所有国家和地区的外部 IP 均会被直接阻断。无需下载/同步多国网段文件，并大幅提升拦截性能。"
              >
                <Switch checkedChildren="已开启 (仅放行中国)" unCheckedChildren="已关闭 (使用黑名单)" />
              </Form.Item>

              <Form.Item noStyle shouldUpdate={(prevValues, currentValues) => prevValues.onlyAllowCn !== currentValues.onlyAllowCn}>
                {({ getFieldValue }) => {
                  const onlyAllow = getFieldValue('onlyAllowCn')
                  return !onlyAllow ? (
                    <Form.Item
                      name="countries"
                      label="选择要拦截的外部国家/地区"
                      extra="勾选后，该国家所有 IP 段都将被阻止连接云枢呼叫服务。请勿拦截中国 (CN) 造成坐席断连。"
                      rules={[{ required: true, message: '请选择至少一个需要封禁的国家/地区' }]}
                    >
                      <Select
                        mode="multiple"
                        placeholder="请选择国家或地区"
                        style={{ width: '100%' }}
                        allowClear
                        optionFilterProp="children"
                      >
                        {commonCountries.map((code) => {
                          const name = countryMap[code] || code
                          return (
                            <Option key={code} value={code}>
                              <Space>
                                <span>{getFlagEmoji(code)}</span>
                                <span>{name}</span>
                                <span style={{ color: '#94a3b8' }}>({code})</span>
                              </Space>
                            </Option>
                          )
                        })}
                      </Select>
                    </Form.Item>
                  ) : null
                }}
              </Form.Item>

              <Form.Item className="mb-0">
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={saveConfigMutation.isPending}
                >
                  应用拦截与放行规则
                </Button>
              </Form.Item>
            </Form>
          </Card>
        </Col>

        <Col xs={24} md={10}>
          <Card
            title={
              <Space>
                <SearchOutlined className="text-blue-500" />
                <span>IP 地理归属与状态分析</span>
              </Space>
            }
            variant="borderless"
            style={{ height: '100%' }}
          >
            <Space.Compact style={{ width: '100%', marginBottom: '16px' }}>
              <Input
                placeholder="请输入待分析的 IP 地址，如 8.8.8.8"
                value={searchIp}
                onChange={(e) => setSearchIp(e.target.value)}
                onPressEnter={handleLookup}
              />
              <Button type="primary" loading={isSearching} onClick={handleLookup}>
                分析归属
              </Button>
            </Space.Compact>

            {lookupResult ? (
              <div style={{
                background: 'rgba(148, 163, 184, 0.05)',
                padding: '12px 16px',
                borderRadius: '8px',
                border: '1px solid rgba(148, 163, 184, 0.15)'
              }}>
                <Space direction="vertical" size="small" className="w-full">
                  <div>
                    <span style={{ color: '#64748b', marginRight: '8px' }}>查询 IP:</span>
                    <span className="font-mono font-bold">{lookupResult.ip}</span>
                  </div>
                  <div>
                    <span style={{ color: '#64748b', marginRight: '8px' }}>地理位置:</span>
                    <span>{renderCountry(lookupResult.countryCode)}</span>
                  </div>
                  <div style={{ marginTop: '8px' }}>
                    {(() => {
                      const isPrivate = isPrivateIP(lookupResult.ip)
                      if (isPrivate) {
                        return <Tag color="success" style={{ borderRadius: '4px' }}>✅ 内网白名单放行（不会拦截）</Tag>
                      }
                      const onlyAllowCn = !!configData?.onlyAllowCn
                      const ipCountry = lookupResult.countryCode.toUpperCase()
                      if (onlyAllowCn) {
                        if (ipCountry === 'CN') {
                          return <Tag color="success" style={{ borderRadius: '4px' }}>✅ 国内 IP 放行中（系统正常放行）</Tag>
                        } else {
                          return <Tag color="error" style={{ borderRadius: '4px' }}>⚠️ 拦截生效中（『仅放行国内 IP』模式已阻断该境外 IP）</Tag>
                        }
                      } else {
                        const blockedCountries = configData?.countries
                          ? configData.countries.split(',').map((c: string) => c.trim().toUpperCase())
                          : []
                        if (blockedCountries.includes(ipCountry)) {
                          return <Tag color="error" style={{ borderRadius: '4px' }}>⚠️ 拦截生效中（该 IP 的 SIP 连接将被内核 DROP）</Tag>
                        }
                      }
                      return <Tag color="warning" style={{ borderRadius: '4px' }}>ℹ️ 未被封禁（系统正常放行，非内网 IP）</Tag>
                    })()}
                  </div>
                </Space>
              </div>
            ) : (
              <div style={{
                height: '110px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: '1px dashed rgba(148, 163, 184, 0.2)',
                borderRadius: '8px',
                color: '#94a3b8',
                fontSize: '13px'
              }}>
                在上方输入 IP 地址进行安全策略分析
              </div>
            )}
          </Card>
        </Col>
      </Row>

      {/* 审计日志过滤 */}
      <Card title="拦截日志查询与审计" variant="borderless" bodyStyle={{ paddingBottom: 0 }}>
        <Space size="middle" wrap className="mb-4">
          <Input
            placeholder="搜索源 IP 地址"
            value={ipFilter}
            onChange={(e) => setIpFilter(e.target.value)}
            style={{ width: 200 }}
            allowClear
          />
          <Select
            placeholder="筛选国家/地区"
            value={countryFilter}
            onChange={setCountryFilter}
            style={{ width: 200 }}
            allowClear
          >
            {commonCountries.map((code) => {
              const name = countryMap[code] || code
              return (
                <Option key={code} value={code}>
                  {getFlagEmoji(code)} {name} ({code})
                </Option>
              )
            })}
          </Select>
          <RangePicker
            showTime
            value={dateRange}
            onChange={(dates) => setDateRange(dates as any)}
            placeholder={['开始时间', '结束时间']}
          />
          <Button type="primary" icon={<FilterOutlined />} onClick={() => setPageNumber(1)}>
            筛选
          </Button>
          <Button icon={<ReloadOutlined />} onClick={handleResetFilters}>
            重置
          </Button>
        </Space>
      </Card>

      {/* 日志列表表格 */}
      <TableWrap
        title="内核拦截审计日志"
        rowKey="id"
        loading={isLogsLoading}
        dataSource={logData?.records ?? []}
        pagination={{
          current: pageNumber,
          pageSize,
          total: logData?.total ?? 0,
          onChange: (current, size) => {
            setPageNumber(current)
            setPageSize(size ?? pageSize)
          },
          showSizeChanger: true,
        }}
        columns={[
          {
            title: '流水 ID',
            dataIndex: 'id',
            width: 100,
          },
          {
            title: '拦截时间',
            dataIndex: 'blockedAt',
            width: 180,
            render: (val) => dayjs(val).format('YYYY-MM-DD HH:mm:ss'),
          },
          {
            title: '源 IP 地址',
            dataIndex: 'ip',
            width: 180,
            render: (val) => <span className="font-mono">{val}</span>,
          },
          {
            title: '归属国家/地区',
            dataIndex: 'countryCode',
            width: 200,
            render: (val) => renderCountry(val),
          },
          {
            title: 'SIP 请求方法',
            dataIndex: 'method',
            width: 150,
            render: (val) => (val ? <Tag color="geekblue">{val}</Tag> : <span style={{ color: '#94a3b8' }}>-</span>),
          },
          {
            title: '拦截动作',
            key: 'action',
            width: 120,
            render: () => <Tag color="error">DROP / 已阻断</Tag>,
          },
          {
            title: '事件关联 Call-ID',
            dataIndex: 'callId',
            render: (val) => (val ? <span className="font-mono text-xs text-gray-500">{val}</span> : <span style={{ color: '#94a3b8' }}>-</span>),
          },
        ]}
      />
    </Space>
  )
}
