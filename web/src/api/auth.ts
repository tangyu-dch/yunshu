import type { AuthLoginResp } from '@/types/auth'
import { http } from '@/api/http'

type LoginInput = {
  username: string
  password: string
  platform: 'operate' | 'merchant'
  permissionProfile?: 'console' | 'operate' | 'merchant'
}

const mockProfiles: Record<NonNullable<LoginInput['permissionProfile']>, string[]> = {
  console: ['console:*'],
  operate: [
    'operate:freeswitch:read', 'operate:freeswitch:write', 'operate:freeswitch:toggle', 
    'operate:gateway:read', 'operate:gateway:write', 'operate:gateway:sync', 
    'operate:dispatcher:read', 'operate:dispatcher:write', 'operate:dispatcher:reload',
    'operate:channel:read', 'operate:channel:write', 'operate:channel:delete',
    'operate:extension:read', 'operate:extension:write', 'operate:extension:delete',
    'operate:pool:read', 'operate:pool:write', 'operate:pool:delete',
    'operate:phone:read', 'operate:phone:write', 'operate:phone:delete',
    'operate:rate:read', 'operate:rate:write', 'operate:rate:delete',
    'operate:blacklist:read', 'operate:blacklist:write', 'operate:blacklist:delete',
    'operate:whitelist:read', 'operate:whitelist:write', 'operate:whitelist:delete',
    'operate:billing:read', 'operate:billing:write',
    'operate:account:read', 'operate:account:write', 'operate:account:delete',
  ],
  merchant: [
    'merchant:batch-task:read', 'merchant:batch-task:write', 'merchant:batch-task:delete',
    'merchant:batch-dialpad:read', 'merchant:batch-dialpad:control', 
    'merchant:call-record:read', 
    'merchant:ai-flow:read', 'merchant:ai-flow:write', 'merchant:ai-flow:precheck', 'merchant:ai-flow:publish', 'merchant:ai-flow:delete',
    'merchant:phone-group:read', 'merchant:phone-group:write', 'merchant:phone-group:delete',
    'merchant:skill-group:read', 'merchant:skill-group:write', 'merchant:skill-group:delete',
    'merchant:account:read', 'merchant:account:write', 'merchant:account:delete',
  ],
}

export async function login(input: LoginInput): Promise<AuthLoginResp> {
  const useMockAuth = import.meta.env.VITE_MOCK_AUTH === 'true'
  if (!useMockAuth) {
    try {
      const prefix = input.platform === 'merchant' ? '/merchant/auth/login' : '/operate/auth/login'
      const { data } = await http.post<AuthLoginResp>(prefix, {
        username: input.username,
        password: input.password,
        internal: input.platform === 'operate',
      })
      return data
    } catch (error) {
      throw error
    }
  }
  const profile = input.permissionProfile ?? 'console'
  return {
    token: `mock-token-${input.username}`,
    expiresAt: new Date(Date.now() + 12 * 60 * 60 * 1000).toISOString(),
    tenant: {
      merchantId: '1001',
      userId: input.username,
      roleId: profile,
      dataScope: 'all',
      permissions: mockProfiles[profile],
      internal: profile === 'console',
    },
  }
}
