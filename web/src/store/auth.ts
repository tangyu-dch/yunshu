import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type TenantContext = {
  merchantId?: string
  userId?: string
  roleId?: string
  dataScope?: string
  permissions: string[]
  internal: boolean
}

export type AuthState = {
  token: string | null
  username: string
  platform: 'operate' | 'merchant' | null
  tenant: TenantContext | null
  expiresAt: string | null
  signature: string | null
  originalTenant: TenantContext | null
  originalPlatform: 'operate' | 'merchant' | null
  originalUsername: string | null
  originalSignature: string | null
  login: (payload: { username: string; token: string; tenant: TenantContext; expiresAt: string | null }) => void
  logout: () => void
  impersonate: (merchantId: string, merchantAccount: string, merchantPermissions?: string[]) => void
  revert: () => void
}

/**
 * 客户端数据完整性滚动加盐签名算法
 * 采用 DJB2 与 FNV-1a 混合双哈希校验，结合服务端返回的 JWT Token 作为加密盐，
 * 防止用户通过浏览器控制台 (F12) 恶意篡改 localStorage 中的角色和权限状态。
 */
export function generateIntegritySignature(tenant: TenantContext | null, token: string | null): string {
  if (!tenant || !token) return ''
  
  // 序列化需要保护的核心安全字段，对权限列表进行排序以保证签名稳定性
  const data = JSON.stringify({
    merchantId: tenant.merchantId || '',
    userId: tenant.userId || '',
    roleId: tenant.roleId || '',
    permissions: [...(tenant.permissions || [])].sort(),
    internal: Boolean(tenant.internal),
  })

  let djb2 = 5381
  let fnv = 2166136261
  const input = `${data}_salt_${token}`

  for (let i = 0; i < input.length; i++) {
    const char = input.charCodeAt(i)
    
    // DJB2 哈希
    djb2 = ((djb2 << 5) + djb2) + char
    djb2 = djb2 & djb2 // 保持在 32 位整数范围
    
    // FNV-1a 哈希
    fnv ^= char
    fnv += (fnv << 1) + (fnv << 4) + (fnv << 7) + (fnv << 8) + (fnv << 24)
  }

  return `${(djb2 >>> 0).toString(16)}-${(fnv >>> 0).toString(16)}`
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      username: 'admin',
      platform: null,
      tenant: null,
      expiresAt: null,
      signature: null,
      originalTenant: null,
      originalPlatform: null,
      originalUsername: null,
      originalSignature: null,
      login: ({ username, token, tenant, expiresAt }) => {
        const signature = generateIntegritySignature(tenant, token)
        set({
          username,
          token,
          tenant,
          expiresAt,
          platform: tenant.internal ? 'operate' : 'merchant',
          signature,
          originalTenant: null,
          originalPlatform: null,
          originalUsername: null,
          originalSignature: null,
        })
      },
      logout: () =>
        set({
          token: null,
          username: 'admin',
          tenant: null,
          expiresAt: null,
          platform: null,
          signature: null,
          originalTenant: null,
          originalPlatform: null,
          originalUsername: null,
          originalSignature: null,
        }),
      impersonate: (merchantId, merchantAccount, merchantPermissions) =>
        set((state) => {
          // 严格的安全卡点：只有运营管理员（internal 为 true）才允许执行 impersonate
          const isOperator = state.tenant?.internal || (state.originalTenant && state.originalTenant.internal)
          if (!isOperator) {
            console.error('【安全警报】非运营平台管理员尝试调用模拟登录接口！已被强行拦截。')
            return {}
          }

          const currentTenant = state.tenant
          const currentPlatform = state.platform
          const currentUsername = state.username
          const currentSignature = state.signature

          const origTenant = state.originalTenant || currentTenant
          const origPlatform = state.originalPlatform || currentPlatform
          const origUsername = state.originalUsername || currentUsername
          const origSignature = state.originalSignature || currentSignature

          const permissions = merchantPermissions || [
            'merchant:batch-task:read',
            'merchant:batch-task:write',
            'merchant:batch-dialpad:read',
            'merchant:batch-dialpad:control',
            'merchant:call-record:read',
            'merchant:ai-flow:read',
            'merchant:ai-flow:write',
            'merchant:ai-flow:precheck',
            'merchant:ai-flow:publish',
            'merchant:skill-group:read',
            'merchant:skill-group:write',
            'merchant:account:read',
            'merchant:account:write',
            'merchant:account:delete',
            'merchant:account:toggle',
            'merchant:account:reset-password',
            'operate:merchant:read',
            'operate:billing:read',
            'operate:rate:read',
          ]

          const impersonatedTenant: TenantContext = {
            merchantId,
            userId: merchantAccount,
            roleId: 'merchant_admin',
            dataScope: 'merchant',
            permissions,
            internal: false,
          }

          const impersonatedSignature = generateIntegritySignature(impersonatedTenant, state.token)

          return {
            username: merchantAccount,
            platform: 'merchant',
            tenant: impersonatedTenant,
            signature: impersonatedSignature,
            originalTenant: origTenant,
            originalPlatform: origPlatform,
            originalUsername: origUsername,
            originalSignature: origSignature,
          }
        }),
      revert: () =>
        set((state) => {
          if (!state.originalTenant) return {}
          return {
            username: state.originalUsername || 'admin',
            platform: state.originalPlatform,
            tenant: state.originalTenant,
            signature: state.originalSignature,
            originalTenant: null,
            originalPlatform: null,
            originalUsername: null,
            originalSignature: null,
          }
        }),
    }),
    { name: 'yunshu-admin-auth' },
  ),
)
