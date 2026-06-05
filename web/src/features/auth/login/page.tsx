import { LockOutlined, UserOutlined, SunOutlined, MoonOutlined } from '@ant-design/icons'
import { Alert, App, Button, Card, ConfigProvider, Form, Input, Select, Space, Typography, theme as antdTheme, message } from 'antd'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { z } from 'zod'
import { login as loginRequest } from '@/api/auth'
import { useAuthStore } from '@/store/auth'
import { useUiStore } from '@/store/ui'

const schema = z.object({
  platform: z.enum(['operate', 'merchant']),
  username: z.string().min(2, '请输入账号'),
  password: z.string().min(4, '请输入密码'),
  permissionProfile: z.enum(['console', 'operate', 'merchant']).optional(),
})

interface LoginPageProps {
  platformType?: 'operate' | 'merchant'
}

export function LoginPage({ platformType }: LoginPageProps) {
  const login = useAuthStore((state) => state.login)
  const navigate = useNavigate()
  const useMockAuth = import.meta.env.VITE_MOCK_AUTH === 'true'
  const [form] = Form.useForm()
  const [loginError, setLoginError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const theme = useUiStore((state) => state.theme)
  const setTheme = useUiStore((state) => state.setTheme)
  const isDark = theme === 'dark'

  const actualPlatform = platformType || 'merchant'

  useEffect(() => {
    // 监听安全拦截错误
    const params = new URLSearchParams(window.location.search)
    if (params.get('error') === 'security_tampering') {
      message.error('【安全审计】检测到客户端身份凭证完整性校验异常！会话已强行销毁并重置，请重新登录。')
    }
  }, [])

  // 路由变化时，强制重置表单为对应平台的默认凭证
  useEffect(() => {
    if (actualPlatform === 'operate') {
      form.setFieldsValue({
        platform: 'operate',
        username: 'admin',
        password: 'admin123',
        permissionProfile: 'console',
      })
    } else {
      form.setFieldsValue({
        platform: 'merchant',
        username: 'merchant',
        password: 'merchant123',
        permissionProfile: 'merchant',
      })
    }
  }, [actualPlatform, form])

  return (
    <ConfigProvider
      theme={{
        algorithm: isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: '#2563eb',
          borderRadius: 8,
          fontFamily: "Inter, 'PingFang SC', 'Microsoft YaHei', sans-serif",
        },
      }}
    >
      <App>
        <div className={`relative flex min-h-screen items-center justify-center px-4 overflow-hidden select-none transition-colors duration-300 ${isDark ? 'bg-slate-950' : 'bg-slate-50'}`}>
          {/* Theme switcher */}
          <Button
            type="text"
            icon={isDark ? <SunOutlined /> : <MoonOutlined />}
            onClick={() => setTheme(isDark ? 'light' : 'dark')}
            className={`absolute top-4 right-4 z-20 transition-colors ${isDark ? 'text-slate-400 hover:text-white' : 'text-slate-500 hover:text-slate-800'}`}
            size="large"
          />

          {/* Glowing gradient background blobs for high-end NOC feel */}
          {isDark ? (
            <>
              <div className="absolute top-[-20%] left-[-15%] w-[70%] h-[70%] rounded-full bg-blue-900/10 blur-[130px] pointer-events-none animate-pulse duration-[8000ms]"></div>
              <div className="absolute bottom-[-20%] right-[-15%] w-[70%] h-[70%] rounded-full bg-indigo-900/15 blur-[130px] pointer-events-none animate-pulse duration-[12000ms]"></div>
              <div className="absolute top-[30%] right-[10%] w-[300px] h-[300px] rounded-full bg-purple-900/5 blur-[100px] pointer-events-none"></div>
            </>
          ) : (
            <>
              <div className="absolute top-[-20%] left-[-15%] w-[70%] h-[70%] rounded-full bg-blue-100/30 blur-[130px] pointer-events-none animate-pulse duration-[8000ms]"></div>
              <div className="absolute bottom-[-20%] right-[-15%] w-[70%] h-[70%] rounded-full bg-indigo-100/40 blur-[130px] pointer-events-none animate-pulse duration-[12000ms]"></div>
              <div className="absolute top-[30%] right-[10%] w-[300px] h-[300px] rounded-full bg-purple-100/15 blur-[100px] pointer-events-none"></div>
            </>
          )}

          <Card className={`w-full max-w-md backdrop-blur-xl relative z-10 rounded-2xl overflow-hidden shadow-2xl transition-all duration-300 ${isDark ? 'border-slate-900 bg-slate-950/40 shadow-blue-950/20' : 'border-slate-200/80 bg-white/70 shadow-slate-200/30'}`}>
            {/* Top border highlight glow line */}
            <div className="absolute top-0 left-0 right-0 h-[2px] bg-gradient-to-r from-blue-500 via-indigo-500 to-purple-600"></div>
            
            <Space direction="vertical" size="large" className="w-full pt-2">
              {/* Premium Logo and Title Section */}
              <div className="text-center">
                <div className="flex justify-center mb-4">
                  <div className="relative flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-tr from-blue-600 to-indigo-500 shadow-lg shadow-blue-500/30">
                    <svg className="w-6 h-6 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
                    </svg>
                    <div className="absolute inset-0 rounded-xl bg-blue-500/20 blur-md -z-10 animate-pulse"></div>
                  </div>
                </div>
                <Typography.Title level={3} className={`!mb-1.5 font-bold tracking-tight bg-clip-text text-transparent ${isDark ? 'bg-gradient-to-r from-white via-slate-100 to-slate-350' : 'bg-gradient-to-r from-slate-800 via-slate-700 to-slate-900'}`}>
                  云枢呼叫中心系统
                </Typography.Title>
                <Typography.Text className={`text-xs font-medium ${isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                  {actualPlatform === 'operate' 
                    ? '系统运营端 — 软交换与底层配置中心' 
                    : '商户管理端 — 批量外呼与 AI 话务调度'}
                </Typography.Text>
              </div>

              {loginError && (
                <Alert
                  message={loginError}
                  type="error"
                  showIcon
                  closable
                  onClose={() => setLoginError(null)}
                  className={`text-xs rounded-lg ${isDark ? 'border-red-900 bg-red-950/30 text-red-200' : 'border-red-200 bg-red-50 text-red-700'}`}
                />
              )}

              <Form
                form={form}
                layout="vertical"
                initialValues={{
                  platform: actualPlatform,
                  permissionProfile: actualPlatform === 'operate' ? 'console' : 'merchant',
                  username: actualPlatform === 'operate' ? 'admin' : 'merchant',
                  password: actualPlatform === 'operate' ? 'admin123' : 'merchant123',
                }}
                onFinish={async (values) => {
                  setLoginError(null)
                  setLoading(true)
                  const valuesToSubmit = {
                    ...values,
                    platform: actualPlatform,
                  }
                  
                  const parsed = schema.safeParse(valuesToSubmit)
                  if (!parsed.success) {
                    const errMsg = parsed.error.issues[0]?.message ?? '参数错误'
                    setLoginError(errMsg)
                    message.error(errMsg)
                    setLoading(false)
                    return
                  }
                  try {
                    const result = await loginRequest({
                      platform: parsed.data.platform,
                      username: parsed.data.username,
                      password: parsed.data.password,
                      permissionProfile: parsed.data.permissionProfile,
                    })
                    login({
                      username: parsed.data.username,
                      token: result.token,
                      tenant: {
                        merchantId: result.tenant.merchantId,
                        userId: result.tenant.userId,
                        roleId: result.tenant.roleId,
                        dataScope: result.tenant.dataScope,
                        permissions: result.tenant.permissions ?? [],
                        internal: Boolean(result.tenant.internal),
                      },
                      expiresAt: result.expiresAt,
                    })
                    navigate('/dashboard')
                  } catch (error) {
                    const errMsg = error instanceof Error ? error.message : '登录失败'
                    setLoginError(errMsg)
                    message.error(errMsg)
                  } finally {
                    setLoading(false)
                  }
                }}
              >
                <Form.Item name="platform" noStyle>
                  <input type="hidden" />
                </Form.Item>

                <Form.Item name="username" label={<span className={`text-xs font-semibold ${isDark ? 'text-slate-350' : 'text-slate-600'}`}>账号</span>} rules={[{ required: true, message: '请输入账号' }]}>
                  <Input 
                    prefix={<UserOutlined className="text-slate-500 mr-1" />} 
                    className={`transition-all rounded-lg h-10 ${isDark ? 'bg-slate-900/60 border-slate-800 text-white placeholder-slate-600 focus:border-blue-500/80 focus:bg-slate-900' : 'bg-white/80 border-slate-200 text-slate-800 placeholder-slate-400 focus:border-blue-500'}`} 
                    placeholder="请输入账号名称" 
                  />
                </Form.Item>

                <Form.Item name="password" label={<span className={`text-xs font-semibold ${isDark ? 'text-slate-350' : 'text-slate-600'}`}>密码</span>} rules={[{ required: true, message: '请输入密码' }]}>
                  <Input.Password 
                    prefix={<LockOutlined className="text-slate-500 mr-1" />} 
                    className={`transition-all rounded-lg h-10 ${isDark ? 'bg-slate-900/60 border-slate-800 text-white placeholder-slate-600 focus:border-blue-500/80 focus:bg-slate-900' : 'bg-white/80 border-slate-200 text-slate-800 placeholder-slate-400 focus:border-blue-500'}`} 
                    placeholder="请输入密码" 
                  />
                </Form.Item>
                
                {useMockAuth ? (
                  <Form.Item name="permissionProfile" label={<span className={`text-xs font-semibold ${isDark ? 'text-slate-350' : 'text-slate-600'}`}>权限模板</span>}>
                    <Select
                      className={`rounded-lg h-10 ${isDark ? 'bg-slate-900/60 border-slate-800 text-white' : 'bg-white/80 border-slate-200 text-slate-800'}`}
                      popupClassName={isDark ? 'bg-slate-900 border-slate-800' : 'bg-white border-slate-200'}
                      options={
                        actualPlatform === 'operate' 
                          ? [
                              { value: 'console', label: '超级管理员 (Console)' },
                              { value: 'operate', label: '运营管理员 (Operate)' },
                            ]
                          : [
                              { value: 'merchant', label: '商户管理员 (Merchant)' },
                            ]
                      }
                    />
                  </Form.Item>
                ) : null}
                
                <Button 
                  loading={loading} 
                  type="primary" 
                  htmlType="submit" 
                  block 
                  size="large" 
                  className="bg-gradient-to-r from-blue-600 via-indigo-600 to-indigo-700 hover:from-blue-500 hover:to-indigo-600 border-none font-medium mt-4 h-10 rounded-lg shadow-lg shadow-blue-500/10 text-white"
                >
                  确认登录
                </Button>

                {actualPlatform === 'operate' ? (
                  <div className={`text-center mt-6 border-t pt-4 ${isDark ? 'border-slate-900/80' : 'border-slate-200'}`}>
                    <Button 
                      type="link" 
                      onClick={() => navigate('/login')} 
                      className={`transition-colors text-xs ${isDark ? 'text-slate-400 hover:text-white' : 'text-slate-500 hover:text-slate-800'}`}
                    >
                      切换至商户登录 ➔
                    </Button>
                  </div>
                ) : null}
              </Form>
            </Space>
          </Card>
        </div>
      </App>
    </ConfigProvider>
  )
}
