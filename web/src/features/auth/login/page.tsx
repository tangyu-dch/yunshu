import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { Alert, App, Button, Card, ConfigProvider, Form, Input, Radio, Select, Space, Typography, theme as antdTheme, message } from 'antd'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { z } from 'zod'
import { login as loginRequest } from '../../../api/auth'
import { useAuthStore } from '../../../store/auth'

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
        algorithm: antdTheme.darkAlgorithm,
        token: {
          colorPrimary: '#2563eb',
          borderRadius: 8,
          fontFamily: "Inter, 'PingFang SC', 'Microsoft YaHei', sans-serif",
        },
      }}
    >
      <App>
        <div className="flex min-h-screen items-center justify-center bg-slate-950 px-4">
          <Card className="w-full max-w-md shadow-soft border-slate-900 bg-slate-950/80 backdrop-blur-md">
            <Space direction="vertical" size="large" className="w-full">
              <div>
                <Typography.Title level={2} className="!mb-1 !text-white font-semibold">
                  云枢管理端 - {actualPlatform === 'operate' ? '系统运营平台' : '商户管理端'}
                </Typography.Title>
                <Typography.Text type="secondary" className="text-slate-400">
                  {actualPlatform === 'operate' 
                    ? '软交换与系统全局配置终端' 
                    : '批量呼叫与 AI 话务控制中心'}
                </Typography.Text>
              </div>
              {loginError && (
                <Alert
                  message={loginError}
                  type="error"
                  showIcon
                  closable
                  onClose={() => setLoginError(null)}
                  className="border-red-900 bg-red-950/40 text-red-200"
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
                  // 强制覆盖 platform 字段，保障安全边界
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
                {/* 剥离单选 Radio 切换，由实际配置的路由静态绑定 */}
                <Form.Item name="platform" noStyle>
                  <input type="hidden" />
                </Form.Item>

                <Form.Item name="username" label={<span className="text-slate-300">账号</span>} rules={[{ required: true, message: '请输入账号' }]}>
                  <Input prefix={<UserOutlined className="text-slate-500" />} className="bg-slate-900 border-slate-800 text-white placeholder-slate-600 focus:border-blue-500" placeholder="账号名称" />
                </Form.Item>
                <Form.Item name="password" label={<span className="text-slate-300">密码</span>} rules={[{ required: true, message: '请输入密码' }]}>
                  <Input.Password prefix={<LockOutlined className="text-slate-500" />} className="bg-slate-900 border-slate-800 text-white placeholder-slate-600 focus:border-blue-500" placeholder="••••••••" />
                </Form.Item>
                
                {useMockAuth ? (
                  <Form.Item name="permissionProfile" label={<span className="text-slate-300">权限模板</span>}>
                    <Select
                      className="bg-slate-900 border-slate-800 text-white"
                      options={
                        actualPlatform === 'operate' 
                          ? [
                              { value: 'console', label: '超级管理员' },
                              { value: 'operate', label: '运营管理员' },
                            ]
                          : [
                              { value: 'merchant', label: '商户管理员' },
                            ]
                      }
                    />
                  </Form.Item>
                ) : null}
                
                <Button loading={loading} type="primary" htmlType="submit" block size="large" className="bg-blue-600 hover:bg-blue-700 border-none font-medium mt-2">
                  登录
                </Button>

                {/* 运营平台可以调整到商户登录，但是商户平台入口是完全隐藏与隔离的 */}
                {actualPlatform === 'operate' ? (
                  <div className="text-center mt-6 border-t border-slate-900 pt-4">
                    <Button 
                      type="link" 
                      onClick={() => navigate('/login')} 
                      className="text-slate-400 hover:text-white transition-colors"
                    >
                      切换至商户登录
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
