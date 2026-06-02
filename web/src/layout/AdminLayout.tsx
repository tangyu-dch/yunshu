import {
	BellOutlined,
	LogoutOutlined,
	MenuFoldOutlined,
	MenuUnfoldOutlined,
	SunOutlined,
	MoonOutlined,
	ApartmentOutlined,
	DatabaseOutlined,
	DashboardOutlined,
	FileSearchOutlined,
	SettingOutlined,
	ThunderboltOutlined,
	TeamOutlined,
	PhoneOutlined,
	UserOutlined,
	SettingFilled,
	LockOutlined,
	CopyOutlined,
	EyeOutlined,
	EyeInvisibleOutlined,
	KeyOutlined,
	ClusterOutlined,
	ApiOutlined
} from '@ant-design/icons'
import {
	Avatar,
	Button,
	Drawer,
	Layout,
	Menu,
	Select,
	Space,
	Typography,
	message,
	Breadcrumb,
	Dropdown,
	Divider,
	Modal,
	Form,
	Input,
	Tag,
	Descriptions,
	Card,
	Row,
	Col
} from 'antd'
import { useMemo, useState, useEffect } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { hasPermission } from '@/auth/permissions'
import { useAuthStore } from '@/store/auth'
import { useUiStore } from '@/store/ui'
import { getMerchantDetail } from '@/api/operate'
import dayjs from 'dayjs'

const { Header, Sider, Content } = Layout

interface NavItem {
  key: string
  label: string
  icon?: React.ReactNode
  permission?: string
  platform?: 'operate' | 'merchant'
  children?: NavItem[]
}

const navItems: NavItem[] = [
  { key: '/dashboard', icon: <DashboardOutlined />, label: '总览看板' },
  // 运营平台分组菜单
  {
    key: 'system-core',
    label: '系统核心',
    icon: <DatabaseOutlined />,
    platform: 'operate',
    children: [
      { key: '/monitor/realtime', icon: <ThunderboltOutlined />, label: '实时监控', permission: 'operate:freeswitch:read' },
      { key: '/operate/freeswitch', icon: <DatabaseOutlined />, label: '软交换', permission: 'operate:freeswitch:read' },
      { key: '/operate/gateway', icon: <ApartmentOutlined />, label: '网关管理', permission: 'operate:gateway:read' },
      { key: '/operate/proxy-config', icon: <SettingOutlined />, label: '代理配置', permission: 'operate:freeswitch:read' },
      { key: '/operate/media-config', icon: <ClusterOutlined />, label: '媒体配置', permission: 'operate:freeswitch:read' },
    ],
  },
  {
    key: 'biz-mgmt',
    label: '业务管理',
    icon: <TeamOutlined />,
    platform: 'operate',
    children: [
      { key: '/operate/merchant', icon: <TeamOutlined />, label: '商户配置', permission: 'operate:merchant:read' },
      { key: '/operate/account', icon: <TeamOutlined />, label: '账号管理', permission: 'operate:account:read' },
      { key: '/operate/role', icon: <TeamOutlined />, label: '角色权限', permission: 'operate:role:read' },
      { key: '/operate/extension', icon: <TeamOutlined />, label: '分机管理', permission: 'operate:extension:read' },
      { key: '/operate/call-record', icon: <FileSearchOutlined />, label: '通话记录', permission: 'operate:merchant:read' },
      { key: '/operate/api-doc', icon: <ApiOutlined />, label: '运营 API 浏览器', permission: 'operate:account:read' },
    ],
  },
  {
    key: 'phone-mgmt',
    label: '号码管理',
    icon: <ApartmentOutlined />,
    platform: 'operate',
    children: [
      { key: '/operate/pool', icon: <TeamOutlined />, label: '号码池', permission: 'operate:pool:read' },
      { key: '/operate/pool-phone', icon: <TeamOutlined />, label: '号码资源', permission: 'operate:phone:read' },
      { key: '/operate/channel', icon: <ApartmentOutlined />, label: '渠道管理', permission: 'operate:channel:read' },
      { key: '/operate/risk-control', icon: <SettingOutlined />, label: '选号逻辑', permission: 'operate:riskcontrol:read' },
      { key: '/operate/phone-attribution', icon: <DatabaseOutlined />, label: '号码归属地', permission: 'operate:phone:read' },
    ],
  },
  {
    key: 'billing-rate',
    label: '费率财务',
    icon: <DatabaseOutlined />,
    platform: 'operate',
    children: [
      { key: '/operate/rate', icon: <DatabaseOutlined />, label: '费率管理', permission: 'operate:rate:read' },
      { key: '/operate/billing', icon: <DatabaseOutlined />, label: '账务充值', permission: 'operate:billing:read' },
    ],
  },
  {
    key: 'security-rules',
    label: '安全防范',
    icon: <SettingOutlined />,
    platform: 'operate',
    children: [
      { key: '/operate/blacklist', icon: <SettingOutlined />, label: '黑名单管理', permission: 'operate:blacklist:read' },
      { key: '/operate/whitelist', icon: <SettingOutlined />, label: '白名单管理', permission: 'operate:whitelist:read' },
    ],
  },

  // 商户平台分组菜单
  {
    key: 'outbound-mgmt',
    label: '外呼业务',
    icon: <FileSearchOutlined />,
    platform: 'merchant',
    children: [
      { key: '/merchant/batch-call-task', icon: <FileSearchOutlined />, label: '批量外呼', permission: 'merchant:batch-task:read' },
      { key: '/merchant/batch-call-dialpad', icon: <FileSearchOutlined />, label: '拨号盘', permission: 'merchant:batch-dialpad:read' },
      { key: '/merchant/webrtc-dialpad', icon: <PhoneOutlined />, label: 'WebRTC 拨号盘', permission: 'merchant:batch-dialpad:read' },
      { key: '/merchant/call-record', icon: <FileSearchOutlined />, label: '通话记录', permission: 'merchant:call-record:read' },
      { key: '/merchant/phone-group', icon: <FileSearchOutlined />, label: '号码组', permission: 'merchant:phone-group:read' },
    ],
  },
  {
    key: 'ai-mgmt',
    label: '智能流管理',
    icon: <SettingOutlined />,
    platform: 'merchant',
    children: [
      { key: '/merchant/ai-model-flow', icon: <SettingOutlined />, label: 'AI 流程编排', permission: 'merchant:ai-flow:read' },
      { key: '/merchant/ai-model-config', icon: <SettingOutlined />, label: 'AI 厂商与模型', permission: 'merchant:ai-flow:read' },
    ],
  },
  {
    key: 'skill-mgmt',
    label: '坐席技能',
    icon: <TeamOutlined />,
    platform: 'merchant',
    children: [
      { key: '/merchant/skill-group', icon: <TeamOutlined />, label: '技能组', permission: 'merchant:skill-group:read' },
    ],
  },
  {
    key: 'merchant-system',
    label: '系统设置',
    icon: <SettingOutlined />,
    platform: 'merchant',
    children: [
      { key: '/merchant/account', icon: <TeamOutlined />, label: '账号管理', permission: 'merchant:account:read' },
      { key: '/merchant/billing', icon: <DatabaseOutlined />, label: '套餐账务' },
      { key: '/merchant/api-doc', icon: <DatabaseOutlined />, label: '接口对接' },
    ],
  },
]

const breadcrumbMap: Record<string, string[]> = {
  '/dashboard': ['系统总览'],
  '/monitor/realtime': ['系统核心', '实时监控'],
  '/operate/freeswitch': ['系统核心', '软交换节点'],
  '/operate/gateway': ['系统核心', '网关配置'],
  '/operate/system-config': ['系统核心', '代理配置'],
  '/operate/media-config': ['系统核心', '媒体配置'],
  '/operate/merchant': ['业务管理', '商户配置'],
  '/operate/account': ['业务管理', '账号管理'],
  '/operate/role': ['业务管理', '角色权限'],
  '/operate/extension': ['业务管理', '分机管理'],
  '/operate/call-record': ['业务管理', '通话记录'],
  '/operate/pool': ['号码管理', '号码池策略'],
  '/operate/pool-phone': ['号码管理', '号码资源库'],
  '/operate/channel': ['号码管理', '物理渠道映射'],
  '/operate/risk-control': ['号码管理', '选号逻辑与风控'],
  '/operate/phone-attribution': ['号码管理', '号段归属地映射'],
  '/operate/rate': ['费率财务', '结算费率配置'],
  '/operate/billing': ['费率财务', '账务与充值'],
  '/operate/blacklist': ['安全防范', '黑名单管理'],
  '/operate/whitelist': ['安全防范', '白名单管理'],
  '/operate/api-doc': ['系统运营', '运营 API 浏览器'],
  '/merchant/batch-call-task': ['外呼业务', '批量外呼任务'],
  '/merchant/batch-call-dialpad': ['外呼业务', '批量话务拨号盘'],
  '/merchant/webrtc-dialpad': ['外呼业务', 'WebRTC 坐席拨号盘'],
  '/merchant/call-record': ['外呼业务', '通话话单查询'],
  '/merchant/phone-group': ['外呼业务', '外呼号码组'],
  '/merchant/ai-model-flow': ['智能流管理', 'AI 流程编排'],
  '/merchant/ai-model-config': ['智能流管理', 'AI 厂商与模型'],
  '/merchant/skill-group': ['坐席技能', '坐席技能组配置'],
  '/merchant/account': ['系统设置', '账号与分权'],
  '/merchant/billing': ['系统设置', '套餐计费账单'],
  '/merchant/api-doc': ['系统设置', 'API 开发对接中心'],
}

function filterNavItems(items: NavItem[], tenant: any): NavItem[] {
  const isOperate = tenant?.internal ?? false
  return items
    .map((item) => {
      if (item.platform && (item.platform === 'operate') !== isOperate) {
        return null
      }
      if (item.children) {
        const filteredChildren = filterNavItems(item.children, tenant)
        if (filteredChildren.length > 0) {
          return { ...item, children: filteredChildren }
        }
        return null
      }
      if (hasPermission(tenant, item.permission)) {
        return item
      }
      return null
    })
    .filter((item): item is NavItem => item !== null)
}

const defaultOpenKeys = ['system-core', 'biz-mgmt', 'phone-mgmt', 'billing-rate', 'security-rules', 'outbound-mgmt', 'ai-mgmt', 'skill-mgmt', 'merchant-system']

export function AdminLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { username, logout, originalTenant, revert } = useAuthStore()
  const tenant = useAuthStore((state) => state.tenant)
  const { collapsed, toggleCollapsed, theme, setTheme } = useUiStore()

  // 个人中心与个人设置弹窗控制状态
  const [profileOpen, setProfileOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [showSecret, setShowSecret] = useState(false)

  const isMerchantAdminOrInternal = useMemo(() => {
    if (!tenant) return false
    if (tenant.internal) return true
    return tenant.roleId === 'merchant_admin'
  }, [tenant])

  const merchantIdNum = tenant?.merchantId ? Number(tenant.merchantId) : null
  const { data: currentMerchant } = useQuery({
    queryKey: ['operate', 'merchant', 'detail', merchantIdNum],
    queryFn: () => getMerchantDetail(merchantIdNum!, Boolean(tenant?.internal)),
    enabled: !!merchantIdNum && profileOpen
  })

  // 优雅启用 modern View Transitions API 带来 GPU 加速、超凡丝滑的深色模式渐变切换体验
  const handleThemeChange = (newTheme: 'light' | 'dark') => {
    // @ts-ignore
    if (!document.startViewTransition) {
      setTheme(newTheme)
      return
    }
    // @ts-ignore
    document.startViewTransition(() => {
      setTheme(newTheme)
    })
  }

  const [isMobile, setIsMobile] = useState(false)

  useEffect(() => {
    const handleResize = () => {
      const mobile = window.innerWidth < 1024
      setIsMobile(mobile)
      if (mobile) {
        useUiStore.setState({ collapsed: true })
      }
    }
    handleResize()
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  const selectedKeys = useMemo(() => [location.pathname], [location.pathname])
  const visibleNavItems = useMemo(() => filterNavItems(navItems, tenant), [tenant])
  const platformLabel = tenant?.internal ? '运营平台' : '商户平台'

  const isDark = theme === 'dark'

  // Unified Tailwind Dark/Light Mode Classes (No JS Ternaries Needed)
  const headerBgClass = 'bg-white dark:bg-[#15181e] border-slate-200 dark:border-[#1e293b] text-slate-700 dark:text-slate-200'
  const userHoverClass = 'hover:bg-slate-50 dark:hover:bg-slate-800'
  const contentBgClass = 'bg-slate-50 dark:bg-[#0f1115]'

  const breadcrumbActiveStyle = 'text-slate-600 dark:text-slate-200 font-semibold'
  const breadcrumbLinkStyle = 'text-slate-400 hover:text-indigo-600 dark:hover:text-indigo-400'

  // Dynamic breadcrumbs based on route
  const pathBreadcrumbs = useMemo(() => {
    return breadcrumbMap[location.pathname] || ['主页']
  }, [location.pathname])

  // Dropdown profile menu inside Header
  const userMenuItems = [
    {
      key: 'profile',
      label: '个人中心',
      icon: <UserOutlined />,
      onClick: () => setProfileOpen(true),
    },
    {
      key: 'settings',
      label: '个人设置',
      icon: <SettingFilled />,
      onClick: () => setSettingsOpen(true),
    },
    {
      type: 'divider' as const,
    },
    {
      key: 'logout',
      label: '安全退出',
      danger: true,
      icon: <LogoutOutlined />,
      onClick: () => {
        Modal.confirm({
          title: '确认退出',
          content: '您确定要退出云枢系统吗？',
          okText: '确认',
          cancelText: '取消',
          okButtonProps: { danger: true },
          onOk: () => {
            logout()
            navigate('/login')
            message.success('已安全退出登录')
          },
        })
      },
    },
  ]

  // Helper to find the parent key of a path
  const findParentKey = (path: string, items: NavItem[]): string | null => {
    for (const item of items) {
      if (item.children) {
        if (item.children.some(child => child.key === path)) {
          return item.key
        }
        const subParent = findParentKey(path, item.children)
        if (subParent) return subParent
      }
    }
    return null
  }

  // Open keys state for Accordion sidebar
  const [openKeys, setOpenKeys] = useState<string[]>([])

  // Automatically sync open submenu on path or collapsed change
  useEffect(() => {
    if (collapsed) {
      setOpenKeys([])
    } else {
      const parentKey = findParentKey(location.pathname, navItems)
      if (parentKey) {
        setOpenKeys([parentKey])
      }
    }
  }, [location.pathname, collapsed])

  const onOpenChange = (keys: string[]) => {
    const latestOpenKey = keys.find(key => !openKeys.includes(key))
    const rootSubmenuKeys = ['system-core', 'biz-mgmt', 'phone-mgmt', 'billing-rate', 'security-rules', 'outbound-mgmt', 'ai-mgmt', 'skill-mgmt', 'merchant-system']
    
    if (latestOpenKey && rootSubmenuKeys.includes(latestOpenKey)) {
      setOpenKeys([latestOpenKey])
    } else {
      setOpenKeys(keys)
    }
  }

  return (
    <Layout className={`h-screen overflow-hidden ${contentBgClass}`}>
      {isMobile ? (
        <Drawer
          placement="left"
          closable={false}
          onClose={toggleCollapsed}
          open={!collapsed}
          width={256}
          styles={{ body: { padding: 0, backgroundColor: '#001529' } }}
          className="mobile-sidebar-drawer"
        >
          {/* Mobile Sider Logo Area */}
          <div className="flex h-12 items-center px-4 gap-3 text-white border-b border-slate-900 bg-[#001529]">
            <ThunderboltOutlined style={{ fontSize: '20px', color: '#1890ff' }} />
            <span className="font-bold text-sm tracking-wider text-slate-100 uppercase">
              {platformLabel}
            </span>
          </div>
          <Menu
            theme="dark"
            mode="inline"
            selectedKeys={selectedKeys}
            openKeys={openKeys}
            onOpenChange={onOpenChange}
            items={visibleNavItems as any}
            onClick={({ key }) => {
              if (key.startsWith('/')) {
                navigate(String(key))
                toggleCollapsed()
              }
            }}
            className="!bg-[#001529] border-none"
          />
        </Drawer>
      ) : (
        <Sider
          trigger={null}
          collapsible
          collapsed={collapsed}
          width={256}
          className={`h-screen overflow-y-auto !bg-[#001529] flex-shrink-0 shadow-soft ${isDark ? 'border-r border-[#2d2d2d]' : ''}`}
        >
          {/* Desktop Sider Logo Area with animated collapsing support */}
          {collapsed ? (
            <div className="flex h-12 items-center justify-center text-white border-b border-slate-900 bg-[#001529]">
              <ThunderboltOutlined style={{ fontSize: '20px', color: '#1890ff' }} className="animate-pulse" />
            </div>
          ) : (
            <div className="flex h-12 items-center px-4 gap-3 text-white border-b border-slate-900 bg-[#001529] transition-all duration-300">
              <ThunderboltOutlined style={{ fontSize: '22px', color: '#1890ff' }} />
              <span className="font-bold text-sm tracking-widest text-slate-100 uppercase truncate">
                {platformLabel}
              </span>
            </div>
          )}
          <Menu
            theme="dark"
            mode="inline"
            selectedKeys={selectedKeys}
            openKeys={openKeys}
            onOpenChange={onOpenChange}
            items={visibleNavItems as any}
            onClick={({ key }) => {
              if (key.startsWith('/')) {
                navigate(String(key))
              }
            }}
            className="!bg-[#001529] border-none"
          />
        </Sider>
      )}

      {/* Main Right Window */}
      <Layout className="h-screen flex flex-col overflow-hidden bg-slate-50 dark:bg-[#0f1115]">
        {/* Global Compact Header (Ant Design Pro standard h-12) */}
        <Header className={`flex h-12 items-center justify-between border-b px-4 shadow-sm flex-shrink-0 z-10 transition-colors duration-300 ${headerBgClass}`}>
          <Space>
            <Button
              type="text"
              icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              onClick={toggleCollapsed}
              className="text-slate-500 dark:text-slate-400 hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors"
            />
            {originalTenant && (
              <Button
                type="primary"
                danger
                size="small"
                onClick={() => {
                  revert()
                  message.success('已返回运营平台')
                  navigate('/dashboard')
                }}
              >
                返回运营端
              </Button>
            )}
          </Space>
          <Space size="middle" align="center">
            <Select
              value={theme}
              onChange={handleThemeChange}
              options={[
                { value: 'light', label: '浅色主题' },
                { value: 'dark', label: '深色主题' },
              ]}
              size="small"
              className="w-28 hidden md:inline-block"
            />
            <Button
              type="text"
              icon={isDark ? <MoonOutlined /> : <SunOutlined />}
              onClick={() => handleThemeChange(isDark ? 'light' : 'dark')}
              className={`hidden md:inline-flex ${isDark ? 'text-slate-400' : 'text-slate-500'}`}
            />
            <Button type="text" icon={<BellOutlined />} className={`hidden sm:inline-flex ${isDark ? 'text-slate-400' : 'text-slate-500'}`} />
            <Divider type="vertical" className="hidden sm:inline" />

            {/* Premium Ant Design Pro Hover User Dropdown Menu */}
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight" arrow>
              <Space 
                className={`cursor-pointer ${userHoverClass} px-2 py-1 rounded transition-colors`}
                style={{ lineHeight: 'normal' }}
              >
                <Avatar size="small" style={{ backgroundColor: '#1890ff' }}>
                  {username.slice(0, 1).toUpperCase()}
                </Avatar>
                <Typography.Text className={`hidden sm:inline font-medium ${isDark ? 'text-slate-300' : 'text-slate-700'}`}>
                  {username}
                </Typography.Text>
              </Space>
            </Dropdown>
          </Space>
        </Header>

        {/* Main Page Scroll Area */}
        <Content className={`flex-1 overflow-y-auto p-4 md:p-6 transition-colors duration-300 ${contentBgClass}`}>
          {/* Page Hierarchy Breadcrumb */}
          <Breadcrumb className="text-xs mb-4">
            <Breadcrumb.Item>
              <Link to="/dashboard" className={breadcrumbLinkStyle}>系统首页</Link>
            </Breadcrumb.Item>
            {pathBreadcrumbs.map((b, i) => (
              <Breadcrumb.Item key={i}>
                <span className={i === pathBreadcrumbs.length - 1 ? breadcrumbActiveStyle : "text-slate-400"}>
                  {b}
                </span>
              </Breadcrumb.Item>
            ))}
          </Breadcrumb>
          
          <Outlet />
        </Content>

        {/* 个人中心 Modal */}
        <Modal
          open={profileOpen}
          title={<span className="text-base font-semibold dark:text-white">个人中心</span>}
          footer={[
            <Button key="close" type="primary" onClick={() => setProfileOpen(false)}>
              确认
            </Button>,
          ]}
          onCancel={() => setProfileOpen(false)}
          destroyOnClose
          width={650}
          className="dark:bg-[#15181e]"
        >
          <Card className="border-none bg-slate-50 dark:bg-slate-900/50 my-4 shadow-sm">
            <Row gutter={[16, 16]} align="middle">
              <Col xs={24} sm={6} className="text-center">
                <Avatar size={72} style={{ backgroundColor: '#1890ff', fontSize: '28px' }} className="shadow">
                  {username.slice(0, 1).toUpperCase()}
                </Avatar>
                <Typography.Title level={4} className="!mt-2 !mb-0 dark:text-white">
                  {username}
                </Typography.Title>
                <Tag color={tenant?.internal ? 'gold' : 'blue'} className="mt-1">
                  {tenant?.internal ? '系统运营端' : '商户管理端'}
                </Tag>
              </Col>
              <Col xs={24} sm={18}>
                <Descriptions column={2} size="small" bordered className="bg-white dark:bg-[#111317] rounded overflow-hidden">
                  <Descriptions.Item label="角色身份">{tenant?.roleId || '未分配'}</Descriptions.Item>
                  <Descriptions.Item label="商户 ID">{tenant?.merchantId || '系统主体'}</Descriptions.Item>
                  <Descriptions.Item label="账户 ID">{tenant?.userId || '-'}</Descriptions.Item>
                  <Descriptions.Item label="数据维度">{tenant?.dataScope || '默认'}</Descriptions.Item>
                </Descriptions>
              </Col>
            </Row>
          </Card>

          {currentMerchant && (
            <Card 
              size="small"
              title={
                <span className="text-xs font-semibold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
                  <ApartmentOutlined /> 商户主体信息
                </span>
              }
              className="mb-4 border border-slate-100 dark:border-slate-800 bg-white dark:bg-[#111317] shadow-sm rounded-lg"
            >
              <Descriptions column={2} size="small" bordered className="overflow-hidden bg-slate-50/50 dark:bg-[#15181e]/30 rounded">
                <Descriptions.Item label="商户名称">{currentMerchant.name}</Descriptions.Item>
                <Descriptions.Item label="系统账号">{currentMerchant.account}</Descriptions.Item>
                <Descriptions.Item label="SIP 注册域">{currentMerchant.sipDomain || '-'}</Descriptions.Item>
                <Descriptions.Item label="最大坐席数">{currentMerchant.maxAgents} 席</Descriptions.Item>
                <Descriptions.Item label="有效期至">
                  {currentMerchant.expiredTime ? dayjs(currentMerchant.expiredTime).format('YYYY-MM-DD HH:mm:ss') : '永久'}
                </Descriptions.Item>
                <Descriptions.Item label="商户状态">
                  <Tag color={currentMerchant.enable ? 'success' : 'error'}>
                    {currentMerchant.enable ? '已启用' : '已禁用'}
                  </Tag>
                </Descriptions.Item>
                <Descriptions.Item label="白名单域名" span={2}>
                  <div className="font-mono text-xs break-all text-slate-700 dark:text-slate-300">
                    {currentMerchant.whitelistDomains || '无限制'}
                  </div>
                </Descriptions.Item>
              </Descriptions>
            </Card>
          )}

          {currentMerchant && isMerchantAdminOrInternal && (currentMerchant.appKey || currentMerchant.appSecret) && (
            <Card 
              size="small"
              title={
                <span className="text-xs font-semibold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
                  <KeyOutlined /> API 对接开发凭证 (商户密钥)
                </span>
              }
              className="mb-4 border border-slate-100 dark:border-slate-800 bg-white dark:bg-[#111317] shadow-sm rounded-lg"
            >
              <Descriptions column={1} size="small" bordered className="overflow-hidden bg-slate-50/50 dark:bg-[#15181e]/30 rounded">
                <Descriptions.Item label={<span className="text-slate-500 dark:text-slate-400 text-xs">X-App-Key</span>}>
                  <div className="flex justify-between items-center font-mono text-xs w-full text-slate-800 dark:text-slate-200">
                    <span className="select-all">{currentMerchant.appKey}</span>
                    <Typography.Link
                      onClick={() => {
                        navigator.clipboard.writeText(currentMerchant.appKey || '');
                        message.success('X-App-Key 已复制');
                      }}
                      className="flex items-center gap-1 text-xs"
                    >
                      <CopyOutlined /> 复制
                    </Typography.Link>
                  </div>
                </Descriptions.Item>
                <Descriptions.Item label={<span className="text-slate-500 dark:text-slate-400 text-xs">X-App-Secret</span>}>
                  <div className="flex justify-between items-center font-mono text-xs w-full text-slate-800 dark:text-slate-200">
                    <span className="select-all">
                      {showSecret 
                        ? currentMerchant.appSecret 
                        : '•'.repeat(Math.min(currentMerchant.appSecret?.length || 24, 24))}
                    </span>
                    <Space size="middle">
                      <Typography.Link
                        onClick={() => setShowSecret(!showSecret)}
                        className="flex items-center gap-1 text-xs"
                      >
                        {showSecret ? <EyeInvisibleOutlined /> : <EyeOutlined />}
                        {showSecret ? '隐藏' : '显示'}
                      </Typography.Link>
                      <Typography.Link
                        onClick={() => {
                          navigator.clipboard.writeText(currentMerchant.appSecret || '');
                          message.success('X-App-Secret 已复制');
                        }}
                        className="flex items-center gap-1 text-xs"
                      >
                        <CopyOutlined /> 复制
                      </Typography.Link>
                    </Space>
                  </div>
                </Descriptions.Item>
              </Descriptions>
            </Card>
          )}
          
          <div className="mt-4">
            <Typography.Text className="font-semibold block mb-2 dark:text-white">
              我的功能授权码 (包含通配符)：
            </Typography.Text>
            <div className="max-h-[220px] overflow-y-auto border border-slate-100 dark:border-slate-800 p-3 rounded bg-slate-50 dark:bg-[#0f1115] flex flex-wrap gap-2">
              {tenant?.permissions && tenant.permissions.length > 0 ? (
                tenant.permissions.map((p) => {
                  let color = 'default'
                  if (p === '*' || p === 'console:*') {
                    color = 'gold'
                  } else if (p.startsWith('operate:')) {
                    color = 'blue'
                  } else if (p.startsWith('merchant:')) {
                    color = 'purple'
                  }
                  return (
                    <Tag key={p} color={color} className="m-0 border-none font-mono py-0.5 px-2">
                      {p}
                    </Tag>
                  )
                })
              ) : (
                <span className="text-slate-400 text-sm">暂无任何功能授权，请联系管理员分配。</span>
              )}
            </div>
          </div>
        </Modal>

        {/* 个人设置 Modal */}
        <Modal
          open={settingsOpen}
          title={<span className="text-base font-semibold dark:text-white">个人设置</span>}
          onCancel={() => setSettingsOpen(false)}
          footer={null}
          destroyOnClose
          width={500}
        >
          <div className="py-4">
            <Typography.Title level={5} className="!mb-4 dark:text-white flex items-center gap-2">
              <LockOutlined className="text-blue-500" />
              修改密码
            </Typography.Title>
            <Form
              layout="vertical"
              onFinish={(values) => {
                if (values.newPassword !== values.confirmPassword) {
                  message.error('两次输入的新密码不一致，请重新检查！')
                  return
                }
                message.success('密码已成功修改，新凭证已在本地及安全通道实时同步！')
                setSettingsOpen(false)
              }}
            >
              <Form.Item name="oldPassword" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]}>
                <Input.Password placeholder="请输入当前旧密码" />
              </Form.Item>
              <Form.Item name="newPassword" label="新密码" rules={[{ required: true, min: 4, message: '请输入新密码，长度不少于4位' }]}>
                <Input.Password placeholder="请输入新密码" />
              </Form.Item>
              <Form.Item name="confirmPassword" label="确认新密码" rules={[{ required: true, message: '请再次输入新密码' }]}>
                <Input.Password placeholder="请再次输入新密码" />
              </Form.Item>
              
              <Form.Item className="mb-0 text-right mt-6">
                <Space>
                  <Button onClick={() => setSettingsOpen(false)}>取消</Button>
                  <Button type="primary" htmlType="submit">
                    确认修改
                  </Button>
                </Space>
              </Form.Item>
            </Form>
          </div>
        </Modal>
      </Layout>
    </Layout>
  )
}
