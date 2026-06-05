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
	ApiOutlined,
	SafetyCertificateOutlined,
	WarningOutlined,
	DownloadOutlined,
	UploadOutlined,
	CloseOutlined
} from '@ant-design/icons'
import {
	Alert,
	Avatar,
	Button,
	Carousel,
	Drawer,
	Layout,
	Menu,
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
	Col,
	Upload,
	App,
	Badge
} from 'antd'
import Marquee from 'react-fast-marquee'
import { useMemo, useState, useEffect } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { hasPermission } from '@/auth/permissions'
import { useAuthStore } from '@/store/auth'
import { useUiStore } from '@/store/ui'
import { getMerchantDetail, fetchLicenseStatus, uploadLicenseFile } from '@/api/operate'
import { http } from '@/api/http'
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
      { key: '/operate/dialpad', icon: <ApiOutlined />, label: '拨号盘版本', permission: 'operate:account:read' },
      { key: '/operate/license', icon: <SafetyCertificateOutlined />, label: '授权管理' },
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
      { key: '/operate/api-doc', icon: <ApiOutlined />, label: '运营 API', permission: 'operate:account:read' },
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
      { key: '/operate/ip-block', icon: <SettingOutlined />, label: 'IP地理拦截', permission: 'operate:riskcontrol:read' },
    ],
  },

  // 商户平台分组菜单
  {
    key: 'outbound-mgmt',
    label: '外呼业务',
    icon: <FileSearchOutlined />,
    platform: 'merchant',
    children: [
      { key: '/merchant/batch-call-task', icon: <FileSearchOutlined />, label: '批量任务', permission: 'merchant:batch-task:read' },
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
    key: 'inbound-mgmt',
    label: '呼入管理',
    icon: <PhoneOutlined />,
    platform: 'merchant',
    children: [
      { key: '/merchant/pool', icon: <PhoneOutlined />, label: '号码池', permission: 'merchant:account:read' },
      { key: '/merchant/pool-phone', icon: <PhoneOutlined />, label: '号码资源', permission: 'merchant:account:read' },
    ],
  },
  {
    key: 'merchant-system',
    label: '系统设置',
    icon: <SettingOutlined />,
    platform: 'merchant',
    children: [
      { key: '/merchant/department', icon: <ApartmentOutlined />, label: '部门管理', permission: 'merchant:department:read' },
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
  '/operate/dialpad': ['系统核心', '拨号盘版本'],
  '/operate/license': ['系统核心', '授权管理'],
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
  '/operate/ip-block': ['安全防范', 'IP地理拦截'],
  '/operate/api-doc': ['系统运营', '运营 API'],
  '/merchant/batch-call-task': ['外呼业务', '批量任务'],
  '/merchant/call-record': ['外呼业务', '通话话单查询'],
  '/merchant/phone-group': ['外呼业务', '外呼号码组'],
  '/merchant/ai-model-flow': ['智能流管理', 'AI 流程编排'],
  '/merchant/ai-model-config': ['智能流管理', 'AI 厂商与模型'],
  '/merchant/skill-group': ['坐席技能', '坐席技能组配置'],
  '/merchant/pool': ['呼入管理', '号码池'],
  '/merchant/pool-phone': ['呼入管理', '号码资源'],
  '/merchant/department': ['系统设置', '部门管理'],
  '/merchant/account': ['系统设置', '账号与分权'],
  '/merchant/billing': ['系统设置', '套餐计费账单'],
  '/merchant/api-doc': ['系统设置', 'API 开发对接中心'],
}

function filterNavItems(items: NavItem[], tenant: any, tenantMode: string): NavItem[] {
  const isOperate = tenant?.internal ?? false
  return items
    .map((item) => {
      if (item.platform && (item.platform === 'operate') !== isOperate) {
        return null
      }
      // 单租户模式下隐藏 SaaS/多租户相关的菜单
      if (isOperate && tenantMode === 'single') {
        if (item.key === '/operate/merchant' || item.key === '/operate/billing') {
          return null
        }
      }
      if (item.children) {
        const filteredChildren = filterNavItems(item.children, tenant, tenantMode)
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
  const { modal, message: appMessage } = App.useApp()
  const navigate = useNavigate()
  const location = useLocation()
  const { username, logout, originalTenant, revert } = useAuthStore()
  const tenant = useAuthStore((state) => state.tenant)
  const { collapsed, toggleCollapsed, theme, setTheme } = useUiStore()

  // 个人中心与个人设置弹窗控制状态
  const [profileOpen, setProfileOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [showSecret, setShowSecret] = useState(false)
  const [uploadingLicense, setUploadingLicense] = useState(false)
  const [licenseOpen, setLicenseOpen] = useState(false)

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

  // 全局获取系统授权状态（在私有化部署中，商户端用户登录也需感知授权是否过期以拦截）
  const { data: licenseStatus, refetch: refetchLicense } = useQuery({
    queryKey: ['system', 'license', 'status'],
    queryFn: fetchLicenseStatus,
    enabled: !!tenant,
    refetchInterval: 15000, // 每15秒轮询一次以保持高水位时钟更新与状态拉取
  })
  const tenantMode = licenseStatus?.tenantMode || 'single'

  const isLocked = useMemo(() => {
    if (!licenseStatus) return false
    const s = licenseStatus.status
    return s === 'expired' || s === 'time_rollback_locked' || s === 'unlicensed'
  }, [licenseStatus])

  const isExpiringSoon = useMemo(() => {
    if (!licenseStatus) return false
    return licenseStatus.remainingDays !== undefined && 
           licenseStatus.remainingDays > 0 && 
           licenseStatus.remainingDays < 15
  }, [licenseStatus])

  const [showTrialAlert, setShowTrialAlert] = useState(() => {
    return sessionStorage.getItem('dismiss_trial_alert') !== 'true'
  })

  const announcements = useMemo(() => {
    if (!licenseStatus) return []
    const list = []
    
    if (licenseStatus.status === 'grace_period') {
      list.push(
        {
          tag: '授权提示',
          text: `系统当前未激活，处于 15 天试用宽限期内（截止日期：${licenseStatus.notAfter || 'N/A'}，试用期后将锁定核心功能）`,
          color: 'orange'
        }
      )
    } else if (licenseStatus.remainingDays !== undefined && licenseStatus.remainingDays > 0 && licenseStatus.remainingDays < 15) {
      list.push(
        {
          tag: '授权预警',
          text: `您的系统商用授权即将到期，剩余有效期仅剩 ${licenseStatus.remainingDays} 天（到期日：${licenseStatus.notAfter || 'N/A'}）。到期后呼叫核心将被安全锁定，请尽快联系管理员或技术支持更新证书！`,
          color: 'red'
        }
      )
    }
    
    return list
  }, [licenseStatus])

  useEffect(() => {
    const handleModeChange = () => {
      refetchLicense()
    }
    window.addEventListener('tenant-mode-changed', handleModeChange)
    return () => window.removeEventListener('tenant-mode-changed', handleModeChange)
  }, [refetchLicense])

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
  const visibleNavItems = useMemo(() => filterNavItems(navItems, tenant, tenantMode), [tenant, tenantMode])
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

  const handleUserMenuClick = ({ key }: { key: string }) => {
    if (key === 'profile') {
      setProfileOpen(true)
    } else if (key === 'settings') {
      setSettingsOpen(true)
    } else if (key === 'license') {
      setLicenseOpen(true)
    } else if (key === 'logout') {
      modal.confirm({
        title: '确认退出',
        content: '您确定要退出云枢系统吗？',
        okText: '确认',
        cancelText: '取消',
        okButtonProps: { danger: true },
        onOk: () => {
          const dest = tenant?.internal ? '/login/operate' : '/login'
          const prefix = tenant?.internal ? '/operate/auth/logout' : '/merchant/auth/logout'
          http.post(prefix, {}).catch(() => {})
          logout()
          window.location.assign(dest)
        },
      })
    }
  }

  // Dropdown profile menu inside Header
  const userMenuItems = useMemo(() => {
    const items: any[] = [
      {
        key: 'profile',
        label: '个人中心',
        icon: <UserOutlined />,
      },
      {
        key: 'settings',
        label: '个人设置',
        icon: <SettingFilled />,
      },
    ]

    // 只有商户管理员（merchant_admin）且非多商户模式下才能看到授权信息
    if (tenant?.roleId === 'merchant_admin' && tenantMode !== 'multi') {
      items.push({
        key: 'license',
        label: '授权信息',
        icon: <SafetyCertificateOutlined />,
      })
    }

    items.push(
      {
        type: 'divider' as const,
      } as any,
      {
        key: 'logout',
        label: '安全退出',
        danger: true,
        icon: <LogoutOutlined />,
      }
    )

    return items
  }, [tenant, tenantMode])

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
          styles={{ body: { padding: 0, backgroundColor: isDark ? '#090b11' : '#ffffff' } }}
          className="mobile-sidebar-drawer"
        >
          {/* Mobile Sider Logo Area */}
          <div className={`flex h-12 items-center px-4 gap-3 border-b transition-all duration-300 ${
            isDark ? 'border-slate-900/60 bg-[#090b11] text-white' : 'border-slate-100 bg-white text-slate-800'
          }`}>
            <ThunderboltOutlined style={{ fontSize: '20px', color: '#1677ff' }} />
            <span className="font-bold text-sm tracking-wider uppercase">
              {platformLabel}
            </span>
          </div>
          <Menu
            theme={isDark ? 'dark' : 'light'}
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
            className="!bg-transparent border-none"
          />
        </Drawer>
      ) : (
        <Sider
          trigger={null}
          collapsible
          collapsed={collapsed}
          width={256}
          className={`h-screen overflow-y-auto flex-shrink-0 shadow-soft transition-all duration-300 ${
            isDark 
              ? '!bg-[#090b11] border-r border-slate-900/60' 
              : '!bg-white border-r border-slate-100'
          }`}
        >
          {/* Desktop Sider Logo Area with animated collapsing support */}
          {collapsed ? (
            <div className={`flex h-12 items-center justify-center border-b transition-all duration-300 ${
              isDark ? 'border-slate-900/60 bg-[#090b11] text-white' : 'border-slate-100 bg-white text-slate-850'
            }`}>
              <ThunderboltOutlined style={{ fontSize: '20px', color: '#1677ff' }} className="animate-pulse" />
            </div>
          ) : (
            <div className={`flex h-12 items-center px-4 gap-3 border-b transition-all duration-300 ${
              isDark ? 'border-slate-900/60 bg-[#090b11] text-white' : 'border-slate-100 bg-white text-slate-850'
            }`}>
              <ThunderboltOutlined style={{ fontSize: '22px', color: '#1677ff' }} />
              <span className={`font-bold text-sm tracking-widest uppercase truncate ${
                isDark ? 'text-slate-100' : 'text-slate-800'
              }`}>
                {platformLabel}
              </span>
            </div>
          )}
          <Menu
            theme={isDark ? 'dark' : 'light'}
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
            className="!bg-transparent border-none"
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
                  appMessage.success('已返回运营平台')
                  navigate('/dashboard')
                }}
              >
                返回运营端
              </Button>
            )}
            {!originalTenant && tenant?.internal && tenantMode === 'single' && (
              <Button
                type="primary"
                size="small"
                onClick={() => {
                  useAuthStore.getState().impersonate('1001', 'merchant')
                  appMessage.success('已切换至商户工作台')
                  navigate('/dashboard')
                }}
              >
                进入商户端
              </Button>
            )}
          </Space>
          <Space size="middle" align="center">
            <Button
              type="text"
              icon={isDark ? <SunOutlined /> : <MoonOutlined />}
              onClick={() => handleThemeChange(isDark ? 'light' : 'dark')}
              className={`hidden md:inline-flex ${isDark ? 'text-slate-400' : 'text-slate-500'}`}
            />
            <Button type="text" icon={<BellOutlined />} className={`hidden sm:inline-flex ${isDark ? 'text-slate-400' : 'text-slate-500'}`} />
            <Divider type="vertical" className="hidden sm:inline" />

            {/* Premium Ant Design Pro Hover User Dropdown Menu */}
            <Dropdown menu={{ items: userMenuItems, onClick: handleUserMenuClick }} placement="bottomRight" arrow>
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

        {showTrialAlert && (licenseStatus?.status === 'grace_period' || isExpiringSoon) && announcements.length > 0 && (
          <Alert
            type="warning"
            banner
            closable
            onClose={() => {
              setShowTrialAlert(false)
              sessionStorage.setItem('dismiss_trial_alert', 'true')
            }}
            className="z-20 border-b border-amber-200 dark:border-amber-950/20"
            message={
              <Marquee pauseOnHover gradient={false} speed={40}>
                {announcements.map((item, idx) => (
                  <span key={idx} className="inline-flex items-center mr-16">
                    <Tag 
                      color={isDark ? 'warning' : item.color} 
                      className="mr-2 text-[10px] font-bold px-1.5 py-0 inline-flex items-center justify-center scale-95 origin-left"
                    >
                      {item.tag}
                    </Tag>
                    <span className={`text-xs md:text-sm font-medium ${isDark ? 'text-slate-300' : 'text-slate-700'}`}>
                      {item.text}
                    </span>
                  </span>
                ))}
              </Marquee>
            }
            action={
              <Button 
                size="small" 
                type="link" 
                onClick={() => navigate('/operate/license')}
                className={`!p-0 h-auto font-semibold ${
                  isDark 
                    ? 'text-[#d48806] hover:text-[#fadb14]' 
                    : 'text-amber-700 hover:text-amber-950'
                }`}
              >
                管理授权
              </Button>
            }
          />
        )}

        {/* Main Page Scroll Area */}
        <Content className={`flex-1 overflow-y-auto p-4 md:p-6 transition-colors duration-300 ${contentBgClass}`}>
          {isLocked && location.pathname !== '/operate/license' ? (
            <div className="flex h-full items-center justify-center py-8">
              <Card 
                className="w-full max-w-2xl shadow-soft border-red-200 dark:border-red-950/60 bg-white dark:bg-[#15181e] p-6 text-center"
              >
                <Space direction="vertical" size="large" className="w-full">
                  <div>
                    <WarningOutlined className="text-red-500 text-6xl animate-pulse" />
                    <Typography.Title level={3} className="!mt-4 !mb-2 dark:text-white">
                      系统授权到期阻断锁定
                    </Typography.Title>
                    <Typography.Paragraph type="secondary" className="text-slate-500 dark:text-slate-400 max-w-md mx-auto text-sm">
                      {licenseStatus?.status === 'time_rollback_locked' 
                        ? '检测到服务器系统时钟被恶意向回调整，安全锁定机制已被触发。' 
                        : '您的呼叫中心系统离线商用授权已经到期（或未激活），呼叫控制核心已被安全锁死，当前无法访问后续业务功能。'}
                    </Typography.Paragraph>
                  </div>

                  <div className="p-4 bg-slate-50 dark:bg-slate-900/60 rounded-lg border border-slate-100 dark:border-slate-800 text-left">
                    <div className="flex justify-between items-center mb-3">
                      <div>
                        <Typography.Text type="secondary" className="text-xs block mb-0.5">物理环境唯一部署 ID (Deployment ID)</Typography.Text>
                        <Typography.Text className="font-mono text-sm font-bold tracking-wider dark:text-indigo-400">
                          {licenseStatus?.deploymentId || 'DEPLOY-N/A'}
                        </Typography.Text>
                      </div>
                      <Space>
                        <Button 
                          icon={<CopyOutlined />} 
                          size="small"
                          onClick={() => {
                            if (licenseStatus?.deploymentId) {
                              navigator.clipboard.writeText(licenseStatus.deploymentId)
                              appMessage.success('部署 ID 已成功复制！')
                            }
                          }}
                        >
                          复制 ID
                        </Button>
                        <Button 
                          type="primary" 
                          icon={<DownloadOutlined />} 
                          size="small"
                          onClick={() => window.open('/api/operate/license/fingerprint/download')}
                        >
                          下载指纹
                        </Button>
                        <Button 
                          danger
                          icon={<LogoutOutlined />} 
                          size="small"
                          onClick={() => {
                            modal.confirm({
                              title: '确认退出',
                              content: '您确定要退出云枢系统吗？',
                              okText: '确认',
                              cancelText: '取消',
                              okButtonProps: { danger: true },
                              onOk: () => {
                                const dest = tenant?.internal ? '/login/operate' : '/login'
                                const prefix = tenant?.internal ? '/operate/auth/logout' : '/merchant/auth/logout'
                                http.post(prefix, {}).catch(() => {})
                                logout()
                                window.location.assign(dest)
                              },
                            })
                          }}
                        >
                          安全退出
                        </Button>
                      </Space>
                    </div>
                    <Typography.Text className="text-[11px] text-slate-400 block border-t border-slate-100 dark:border-slate-800 pt-2">
                      💡 请下载上方的指纹文件并发送给“云枢”客服或技术支持团队以换取续期或迁移授权证书 (.lic)。
                    </Typography.Text>
                  </div>

                  <div className="text-left">
                    <Typography.Text className="font-semibold block mb-2 dark:text-white">
                      导入新授权证书以即时解锁：
                    </Typography.Text>
                    <Upload.Dragger
                      name="file"
                      multiple={false}
                      beforeUpload={async (file) => {
                        setUploadingLicense(true)
                        try {
                          await uploadLicenseFile(file)
                          appMessage.success('授权验证成功，系统已即时解锁！')
                          refetchLicense()
                        } catch (err: any) {
                          // 错误已被拦截
                        } finally {
                          setUploadingLicense(false)
                        }
                        return false
                      }}
                      showUploadList={false}
                      disabled={uploadingLicense}
                    >
                      <p className="ant-upload-drag-icon">
                        <UploadOutlined className="text-3xl text-indigo-400" />
                      </p>
                      <p className="ant-upload-text text-sm dark:text-slate-300 font-medium">
                        点击浏览或将最新的 yunshu.lic 证书拖拽至此处
                      </p>
                    </Upload.Dragger>
                  </div>
                </Space>
              </Card>
            </div>
          ) : (
            <>
              {/* Page Hierarchy Breadcrumb */}
              <Breadcrumb
                className="text-xs mb-4"
                items={[
                  {
                    title: <Link to="/dashboard" className={breadcrumbLinkStyle}>系统首页</Link>
                  },
                  ...pathBreadcrumbs.map((b, i) => ({
                    title: (
                      <span className={i === pathBreadcrumbs.length - 1 ? breadcrumbActiveStyle : "text-slate-400"}>
                        {b}
                      </span>
                    )
                  }))
                ]}
              />
              
              <Outlet />
            </>
          )}
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
          destroyOnHidden
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
                        appMessage.success('X-App-Key 已复制');
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
                          appMessage.success('X-App-Secret 已复制');
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
          destroyOnHidden
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
                  appMessage.error('两次输入的新密码不一致，请重新检查！')
                  return
                }
                appMessage.success('密码已成功修改，新凭证已在本地及安全通道实时同步！')
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

        {/* 授权信息 Modal */}
        <Modal
          open={licenseOpen}
          title={
            <Space>
              <SafetyCertificateOutlined className="text-indigo-500" />
              <span className="text-base font-semibold dark:text-white">云枢系统授权信息</span>
            </Space>
          }
          onCancel={() => setLicenseOpen(false)}
          footer={[
            <Button key="close" type="primary" onClick={() => setLicenseOpen(false)}>
              关闭
            </Button>
          ]}
          destroyOnHidden
          width={560}
        >
          {licenseStatus ? (
            <div className="py-4 space-y-4">
              {/* Alert / Notification based on status */}
              {(() => {
                const s = licenseStatus.status
                let type: 'success' | 'warning' | 'error' | 'info' = 'info'
                let title = '未授权 (UNLICENSED)'
                let desc = '系统当前处于未授权状态，请获取授权证书。'
                
                if (s === 'normal') {
                  type = 'success'
                  title = '已激活 (ACTIVE)'
                  desc = '系统商用授权验证成功，并发资源运行状态正常。'
                } else if (s === 'grace_period') {
                  type = 'warning'
                  title = '宽限期内 (GRACE PERIOD)'
                  desc = licenseStatus.statusMsg || '授权证书已过期，当前处于15天宽限期内，额定最大并发数已受限。请尽快续期。'
                } else if (s === 'expired') {
                  type = 'error'
                  title = '已过期 (EXPIRED)'
                  desc = licenseStatus.statusMsg || '授权证书已过期，宽限期结束，呼叫控制通道已锁死。请上传新证书恢复。'
                } else if (s === 'time_rollback_locked') {
                  type = 'error'
                  title = '安全锁死 (LOCKED)'
                  desc = '检测到服务器系统时钟被向回调整，安全锁定机制已被触发。'
                }
                
                return (
                  <Alert
                    message={<span className="font-semibold text-xs">{title}</span>}
                    description={<span className="text-xs text-slate-500 dark:text-slate-400">{desc}</span>}
                    type={type}
                    showIcon
                    className="border-0 shadow-sm"
                  />
                )
              })()}

              {/* Statistics for Specifications */}
              <Row gutter={16}>
                <Col span={12}>
                  <Card size="small" className="bg-slate-50/50 dark:bg-[#111317] border border-slate-100 dark:border-slate-800 text-center">
                    <Typography.Text type="secondary" className="text-[10px] block mb-1">系统并发授权上限</Typography.Text>
                    <Typography.Text className="text-xl font-bold dark:text-indigo-400">
                      {licenseStatus.maxConcurrentCalls ?? 0} <span className="text-xs font-normal text-slate-400 dark:text-slate-500">线</span>
                    </Typography.Text>
                  </Card>
                </Col>
                <Col span={12}>
                  <Card size="small" className="bg-slate-50/50 dark:bg-[#111317] border border-slate-100 dark:border-slate-800 text-center">
                    <Typography.Text type="secondary" className="text-[10px] block mb-1">最大坐席分机数</Typography.Text>
                    <Typography.Text className="text-xl font-bold dark:text-sky-400">
                      {licenseStatus.maxExtensions ?? 0} <span className="text-xs font-normal text-slate-400 dark:text-slate-500">个</span>
                    </Typography.Text>
                  </Card>
                </Col>
              </Row>

              {/* Descriptions table */}
              <Card 
                size="small"
                title={<span className="text-xs font-semibold text-slate-700 dark:text-slate-350">授权规格详情</span>}
                className="border border-slate-100 dark:border-slate-800 bg-white dark:bg-[#111317] shadow-sm rounded-lg"
              >
                <Descriptions column={2} size="small" bordered className="overflow-hidden bg-slate-50/50 dark:bg-[#15181e]/30 rounded">
                  <Descriptions.Item label="授权编号" span={2}>
                    <span className="font-mono text-xs text-slate-850 dark:text-slate-200">{licenseStatus.licenseId || '-'}</span>
                  </Descriptions.Item>
                  <Descriptions.Item label="客户主体" span={2}>
                    <span className="text-xs font-medium text-slate-850 dark:text-slate-200">{licenseStatus.customerName || '-'}</span>
                  </Descriptions.Item>
                  <Descriptions.Item label="生效时间">
                    <span className="text-[11px] text-slate-650 dark:text-slate-350">{licenseStatus.notBefore || '-'}</span>
                  </Descriptions.Item>
                  <Descriptions.Item label="到期时间">
                    <span className="text-[11px] text-slate-650 dark:text-slate-350">{licenseStatus.notAfter || '-'}</span>
                  </Descriptions.Item>
                  <Descriptions.Item label="授权类型">
                    {licenseStatus.licenseType === 'migration' ? (
                      <Tag color="purple" className="m-0 text-[10px]">平移</Tag>
                    ) : licenseStatus.licenseType === 'renewal' ? (
                      <Tag color="success" className="m-0 text-[10px]">续期</Tag>
                    ) : licenseStatus.licenseType === 'trial' ? (
                      <Tag color="orange" className="m-0 text-[10px]">试用</Tag>
                    ) : (
                      <Tag color="blue" className="m-0 text-[10px]">标准</Tag>
                    )}
                  </Descriptions.Item>
                  <Descriptions.Item label="剩余有效期">
                    <span className="text-xs font-semibold text-green-600 dark:text-green-400">
                      {licenseStatus.remainingDays ?? 0} 天
                    </span>
                  </Descriptions.Item>
                </Descriptions>
              </Card>

              {/* Deployment ID info */}
              <div className="p-3 bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-850 rounded-lg">
                <Typography.Text type="secondary" className="text-[10px] block mb-1">物理环境唯一部署 ID (Deployment ID)</Typography.Text>
                <div className="flex justify-between items-center font-mono text-xs w-full text-slate-800 dark:text-slate-200">
                  <span className="select-all font-bold tracking-wider dark:text-indigo-400">{licenseStatus.deploymentId || 'DEPLOY-N/A'}</span>
                  <Typography.Link
                    onClick={() => {
                      if (licenseStatus.deploymentId) {
                        navigator.clipboard.writeText(licenseStatus.deploymentId)
                        appMessage.success('部署 ID 已成功复制到剪贴板！')
                      }
                    }}
                    className="flex items-center gap-1 text-xs"
                  >
                    <CopyOutlined /> 复制
                  </Typography.Link>
                </div>
              </div>

              {/* Upload Certificate Section */}
              <Divider className="!my-2" />
              <div>
                <Typography.Text className="font-semibold block mb-2 dark:text-white text-xs">
                  更新系统授权证书 (.lic)：
                </Typography.Text>
                <Upload.Dragger
                  name="file"
                  multiple={false}
                  beforeUpload={async (file) => {
                    setUploadingLicense(true)
                    try {
                      await uploadLicenseFile(file)
                      appMessage.success('授权证书文件验证通过，系统授权已成功更新！')
                      refetchLicense()
                    } catch (err: any) {
                      // 错误已被拦截
                    } finally {
                      setUploadingLicense(false)
                    }
                    return false
                  }}
                  showUploadList={false}
                  disabled={uploadingLicense}
                  className="bg-slate-50/50 dark:bg-slate-900/10 border-dashed border-slate-200 dark:border-slate-800 rounded-lg py-3"
                >
                  <p className="ant-upload-drag-icon">
                    <UploadOutlined className="text-xl text-indigo-400" />
                  </p>
                  <p className="ant-upload-text text-xs dark:text-slate-300 font-medium">
                    点击浏览或将最新的 yunshu.lic 证书拖拽至此处以更新授权
                  </p>
                  <p className="ant-upload-hint text-[10px] text-slate-400 dark:text-slate-500 mt-1">
                    系统将即时进行证书签发签名校验，校验通过后热加载生效。
                  </p>
                </Upload.Dragger>
              </div>
            </div>
          ) : (
            <div className="py-8 text-center text-slate-400 text-xs">
              正在加载系统授权状态...
            </div>
          )}
        </Modal>
      </Layout>
    </Layout>
  )
}
