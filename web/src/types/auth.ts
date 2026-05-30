export type AuthLoginResp = {
  token: string
  expiresAt: string
  tenant: {
    merchantId?: string
    userId?: string
    roleId?: string
    dataScope?: string
    permissions?: string[]
    internal?: boolean
  }
}
