import type { ReactNode } from 'react'
import { useAuthStore } from '@/store/auth'
import { hasPermission } from '@/auth/permissions'

export function PermissionGate({
  permission,
  fallback = null,
  children,
}: {
  permission?: string | null
  fallback?: ReactNode
  children: ReactNode
}) {
  const tenant = useAuthStore((state) => state.tenant)
  if (!hasPermission(tenant, permission)) {
    return <>{fallback}</>
  }
  return <>{children}</>
}

