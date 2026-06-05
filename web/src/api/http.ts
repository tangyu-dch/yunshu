import axios, { AxiosError } from 'axios'
import { message } from '@/utils/antd'
import { useAuthStore, generateIntegritySignature } from '@/store/auth'

type BackendResult<T = unknown> = {
  code: number
  message: string
  data?: T
}

function isBackendResult(value: unknown): value is BackendResult {
  if (!value || typeof value !== 'object') {
    return false
  }
  const maybe = value as Partial<BackendResult>
  return typeof maybe.code === 'number' && typeof maybe.message === 'string'
}

export const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '/api',
  timeout: 15000,
})

http.interceptors.request.use((config) => {
  const auth = useAuthStore.getState()

  // 发送任何 API 请求前校验客户端权限缓存的数据完整性，防 F12 恶意篡改
  if (auth.token && auth.tenant) {
    const expected = generateIntegritySignature(auth.tenant, auth.token)
    if (auth.signature !== expected) {
      console.error('【安全防御】检测到非法篡改的客户端权限数据，拒绝发送 API 请求！')
      auth.logout()
      if (typeof window !== 'undefined') {
        window.location.assign('/login?error=security_tampering')
      }
      return Promise.reject(new Error('Security check failed'))
    }
  }

  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  if (auth.tenant?.merchantId) {
    config.headers['X-Merchant-Id'] = auth.tenant.merchantId
  }
  if (auth.tenant?.userId) {
    config.headers['X-User-Id'] = auth.tenant.userId
  }
  return config
})

http.interceptors.response.use(
  (response) => {
    if (isBackendResult(response.data)) {
      if (response.data.code !== 0) {
        const errorMsg = response.data.message || '请求失败'
        message.error({ content: errorMsg, key: errorMsg })
        throw new Error(errorMsg)
      }
      response.data = response.data.data
    }
    return response
  },
  (error: AxiosError<BackendResult>) => {
    if (error.response?.status === 401) {
      const shouldRedirect = typeof window !== 'undefined' && !window.location.pathname.startsWith('/login')
      useAuthStore.getState().logout()
      if (shouldRedirect) {
        window.location.assign('/login')
      }
    }
    const errorMsg = error.response?.data?.message || error.message || '网络请求失败'
    message.error({ content: errorMsg, key: errorMsg })
    return Promise.reject(new Error(errorMsg))
  },
)
