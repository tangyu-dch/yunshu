import type { ReactElement } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'
import { hasPermission } from '@/auth/permissions'
import { AdminLayout } from '@/layout/AdminLayout'
import { useAuthStore, generateIntegritySignature } from '@/store/auth'
import { AiModelFlowPage } from '@/features/business/ai-model-flow/page'
import { AiModelFlowDesigner } from '@/features/business/ai-model-flow/designer'
import { DepartmentPage } from '@/features/system/department/page'
import { BatchTaskPage } from '@/features/business/batch-call-task/page'
import { CallRecordPage } from '@/features/business/call-record/page'
import { DashboardPage } from '@/features/dashboard/page'
import { ForbiddenPage } from '@/features/auth/forbidden/page'
import { NotFoundPage } from '@/features/auth/not-found/page'
import { DispatcherPage } from '@/features/resource/dispatcher/page'
import { FreeSwitchPage } from '@/features/telephony/freeswitch/page'
import { GatewayPage } from '@/features/telephony/gateway/page'
import { MerchantPage } from '@/features/merchant/merchant/page'
import { PoolPage } from '@/features/resource/pool/page'
import { PoolPhonePage } from '@/features/resource/pool-phone/page'
import { LoginPage } from '@/features/auth/login/page'
import { RealtimeMonitorPage } from '@/features/monitor/realtime/page'
import { SkillGroupPage } from '@/features/business/skill-group/page'
import { PhoneGroupPage } from '@/features/business/phone-group/page'
import { MerchantAccountPage } from '@/features/merchant/account/page'
import { MerchantBillingPage } from '@/features/merchant/billing/page'
import { MerchantApiDocPage } from '@/features/merchant/api-doc/page'
import { AccountPage } from '@/features/system/account/page'
import { RolePermissionPage } from '@/features/system/role/page'
import { OperatorApiDocPage } from '@/features/system/api-doc/page'
import { RatePage } from '@/features/merchant/rate/page'
import { BlacklistPage } from '@/features/security/blacklist/page'
import { WhitelistPage } from '@/features/security/whitelist/page'
import { BillingPage } from '@/features/merchant/billing-admin/page'
import { ChannelPage } from '@/features/resource/channel/page'
import { ExtensionPage } from '@/features/resource/extension/page'
import { RiskControlPage } from '@/features/security/risk-control/page'
import { PhoneAttributionPage } from '@/features/system/phone-attribution/page'
import { ProxyConfigPage } from '@/features/system/proxy-config/page'
import { MediaConfigPage } from '@/features/telephony/media-config/page'
import { InstallPage } from '@/features/install/page'
import { DialpadVersionPage } from '@/features/system/dialpad/page'
import { LicensePage } from '@/features/system/license/page'
import { IPBlockPage } from '@/features/security/ip-block/page'



function Guard({ children }: { children: ReactElement }) {
  const state = useAuthStore()
  if (!state.token) {
    const isOperate = window.location.pathname.startsWith('/operate')
    return <Navigate to={isOperate ? "/login/operate" : "/login"} replace />
  }

  // 严格的路由跳转签名审计
  if (state.token && state.tenant) {
    const expected = generateIntegritySignature(state.tenant, state.token)
    if (state.signature !== expected) {
      console.error('【安全拦截】检测到 localStorage 认证凭证与完整性校验签名不匹配！正在强行销毁会话。')
      const isOperate = state.tenant?.internal
      const redirectUrl = isOperate ? '/login/operate?error=security_tampering' : '/login?error=security_tampering'
      setTimeout(() => {
        state.logout()
        window.location.assign(redirectUrl)
      }, 0)
      return <Navigate to={redirectUrl} replace />
    }
  }

  return children
}

function ProtectedShell() {
  return (
    <Guard>
      <AdminLayout />
    </Guard>
  )
}

function RequirePermission({
  permission,
  children,
}: {
  permission?: string | null
  children: ReactElement
}) {
  const state = useAuthStore()

  // 严格的路由访问签名审计
  if (state.token && state.tenant) {
    const expected = generateIntegritySignature(state.tenant, state.token)
    if (state.signature !== expected) {
      console.error('【安全拦截】检测到已篡改的认证凭证！正在强行销毁会话。')
      const isOperate = state.tenant?.internal
      const redirectUrl = isOperate ? '/login/operate?error=security_tampering' : '/login?error=security_tampering'
      setTimeout(() => {
        state.logout()
        window.location.assign(redirectUrl)
      }, 0)
      return <Navigate to={redirectUrl} replace />
    }
  }

  if (!hasPermission(state.tenant, permission)) {
    return <NotFoundPage />
  }
  return children
}

export const router = createBrowserRouter([
  { path: '/', element: <Navigate to="/dashboard" replace /> },
  { path: '/login', element: <LoginPage platformType="merchant" /> },
  { path: '/login/operate', element: <LoginPage platformType="operate" /> },
  { path: '/install', element: <InstallPage /> },
  { path: '/403', element: <ForbiddenPage /> },
  {
    path: '/merchant/ai-model-flow/designer/:id',
    element: (
      <Guard>
        <RequirePermission permission="merchant:ai-flow:write">
          <AiModelFlowDesigner />
        </RequirePermission>
      </Guard>
    )
  },
  {
    path: '/',
    element: <ProtectedShell />,
    children: [
      { path: 'dashboard', element: <DashboardPage /> },
      { path: 'monitor/realtime', element: <RequirePermission permission="operate:freeswitch:read"><RealtimeMonitorPage /></RequirePermission> },
      { path: 'operate/freeswitch', element: <RequirePermission permission="operate:freeswitch:read"><FreeSwitchPage /></RequirePermission> },
      { path: 'operate/gateway', element: <RequirePermission permission="operate:gateway:read"><GatewayPage /></RequirePermission> },
      { path: 'operate/merchant', element: <RequirePermission permission="operate:merchant:read"><MerchantPage /></RequirePermission> },
      { path: 'operate/pool', element: <RequirePermission permission="operate:pool:read"><PoolPage /></RequirePermission> },
      { path: 'operate/pool-phone', element: <RequirePermission permission="operate:phone:read"><PoolPhonePage /></RequirePermission> },
      { path: 'operate/dispatcher', element: <RequirePermission permission="operate:dispatcher:read"><DispatcherPage /></RequirePermission> },

      { path: 'operate/account', element: <RequirePermission permission="operate:account:read"><AccountPage /></RequirePermission> },
      { path: 'operate/role', element: <RequirePermission permission="operate:role:read"><RolePermissionPage /></RequirePermission> },
      { path: 'operate/rate', element: <RequirePermission permission="operate:rate:read"><RatePage /></RequirePermission> },
      { path: 'operate/blacklist', element: <RequirePermission permission="operate:blacklist:read"><BlacklistPage /></RequirePermission> },
      { path: 'operate/whitelist', element: <RequirePermission permission="operate:whitelist:read"><WhitelistPage /></RequirePermission> },
      { path: 'operate/billing', element: <RequirePermission permission="operate:billing:read"><BillingPage /></RequirePermission> },
      { path: 'operate/channel', element: <RequirePermission permission="operate:channel:read"><ChannelPage /></RequirePermission> },
      { path: 'operate/extension', element: <RequirePermission permission="operate:extension:read"><ExtensionPage /></RequirePermission> },
      { path: 'operate/risk-control', element: <RequirePermission permission="operate:riskcontrol:read"><RiskControlPage /></RequirePermission> },
      { path: 'operate/ip-block', element: <RequirePermission permission="operate:riskcontrol:read"><IPBlockPage /></RequirePermission> },
      { path: 'operate/phone-attribution', element: <RequirePermission permission="operate:phone:read"><PhoneAttributionPage /></RequirePermission> },
      { path: 'operate/proxy-config', element: <RequirePermission permission="operate:freeswitch:read"><ProxyConfigPage /></RequirePermission> },
      { path: 'operate/media-config', element: <RequirePermission permission="operate:freeswitch:read"><MediaConfigPage /></RequirePermission> },
      { path: 'operate/dialpad', element: <RequirePermission permission="operate:account:read"><DialpadVersionPage /></RequirePermission> },
      { path: 'operate/license', element: <LicensePage /> },
      { path: 'operate/call-record', element: <RequirePermission permission="operate:merchant:read"><CallRecordPage /></RequirePermission> },

      { path: 'operate/api-doc', element: <RequirePermission permission="operate:account:read"><OperatorApiDocPage /></RequirePermission> },
      { path: 'merchant/batch-call-task', element: <RequirePermission permission="merchant:batch-task:read"><BatchTaskPage /></RequirePermission> },
      { path: 'merchant/department', element: <RequirePermission permission="merchant:department:read"><DepartmentPage /></RequirePermission> },
      { path: 'merchant/call-record', element: <RequirePermission permission="merchant:call-record:read"><CallRecordPage /></RequirePermission> },
      { path: 'merchant/ai-model-flow', element: <RequirePermission permission="merchant:ai-flow:read"><AiModelFlowPage /></RequirePermission> },
      { path: 'merchant/ai-model-config', element: <RequirePermission permission="merchant:ai-flow:read"><AiModelFlowPage /></RequirePermission> },

      { path: 'merchant/pool', element: <RequirePermission permission="merchant:account:read"><PoolPage /></RequirePermission> },
      { path: 'merchant/pool-phone', element: <RequirePermission permission="merchant:account:read"><PoolPhonePage /></RequirePermission> },

      { path: 'merchant/skill-group', element: <RequirePermission permission="merchant:skill-group:read"><SkillGroupPage /></RequirePermission> },
      { path: 'merchant/phone-group', element: <RequirePermission permission="merchant:phone-group:read"><PhoneGroupPage /></RequirePermission> },
      { path: 'merchant/account', element: <RequirePermission permission="merchant:account:read"><MerchantAccountPage /></RequirePermission> },
      { path: 'merchant/billing', element: <MerchantBillingPage /> },
      { path: 'merchant/api-doc', element: <MerchantApiDocPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
  { path: '*', element: <NotFoundPage /> },
])
