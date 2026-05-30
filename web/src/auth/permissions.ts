import type { TenantContext } from '@/store/auth'
import { generateIntegritySignature, useAuthStore } from '@/store/auth'

export type PagePermission =
  | 'operate:freeswitch:read'
  | 'operate:gateway:read'
  | 'operate:dispatcher:read'
  | 'operate:account:read'
  | 'operate:channel:read'
  | 'operate:extension:read'
  | 'operate:pool:read'
  | 'operate:phone:read'
  | 'operate:rate:read'
  | 'operate:blacklist:read'
  | 'operate:whitelist:read'
  | 'operate:billing:read'
  | 'merchant:batch-task:read'
  | 'merchant:batch-dialpad:read'
  | 'merchant:call-record:read'
  | 'merchant:ai-flow:read'
  | 'merchant:phone-group:read'
  | 'merchant:skill-group:read'

export function hasPermission(tenant: TenantContext | null, required?: string | null) {
  if (!required) {
    return true
  }
  if (!tenant) {
    return false
  }

  // 严格的安全完整性校验：防止 localStorage 篡改
  try {
    const authState = useAuthStore.getState()
    if (authState.token && authState.tenant) {
      const expected = generateIntegritySignature(authState.tenant, authState.token)
      if (authState.signature !== expected) {
        console.warn('【安全拦截】检测到 localStorage 权限数据被非法篡改！拒绝权限申请。')
        return false
      }
    }
  } catch (e) {
    // 捕获可能由于状态初始化带来的异常，静默拦截
    return false
  }

  // 数据驱动校验：取消超级管理员角色的硬编码放行逻辑，完全依靠后端在 token 中签发的 console:* 或 * 通配权限码
  const requiredPermission = normalizePermission(required)
  if (!requiredPermission) {
    return true
  }
  return tenant.permissions.some((permission) => matchPermission(normalizePermission(permission), requiredPermission))
}

export function matchPermission(granted: string, required: string) {
  if (!granted || !required) {
    return false
  }
  if (granted === '*' || granted === 'console:*') {
    return true
  }
  if (granted === required) {
    return true
  }
  if (granted.endsWith('*')) {
    const prefix = granted.slice(0, -1)
    return required.startsWith(prefix)
  }
  return false
}

export function normalizePermission(raw: string) {
  return raw.trim()
}
