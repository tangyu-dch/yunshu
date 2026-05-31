import { Button, Card, Col, Form, Input, Row, Space, Tag, Typography, message } from 'antd'
import { useEffect, useRef, useState } from 'react'
import { useAuthStore } from '@/store/auth'
import { getMerchantDetail } from '@/api/operate'
import { YunshuCallSDK } from '@/sdk/yunshu-call-sdk'

export function WebRtcDialpadPage() {
  const username = useAuthStore((state) => state.username)
  const tenant = useAuthStore((state) => state.tenant)
  const defaultExt = username?.match(/^\d+$/) ? username : '1001'

  // Form states
  const [wsUrl, setWsUrl] = useState(() => {
    const host = typeof window !== 'undefined' ? window.location.hostname : '127.0.0.1'
    return `ws://${host}:5066`
  })
  const [ext, setExt] = useState(defaultExt)
  const [password, setPassword] = useState('123456')
  const [domain, setDomain] = useState('sip.yunshu.local')

  // Dial states
  const [callee, setCallee] = useState('')
  const [callDuration, setCallDuration] = useState(0)

  // WebRTC SDK instance & States
  const sdkRef = useRef<YunshuCallSDK | null>(null)
  const [registered, setRegistered] = useState(false)
  const [regState, setRegState] = useState<'disconnected' | 'connecting' | 'registered' | 'failed'>('disconnected')
  const [callState, setCallState] = useState<'idle' | 'dialing' | 'ringing' | 'connected' | 'ended'>('idle')
  const [incomingCaller, setIncomingCaller] = useState<string | null>(null)
  const [logs, setLogs] = useState<string[]>([])

  const audioRef = useRef<HTMLAudioElement | null>(null)
  const timerRef = useRef<any>(null)

  const addLog = (msg: string) => {
    setLogs((prev) => [msg, ...prev.slice(0, 99)])
  }

  // Load merchant-specific SIP domain dynamically
  useEffect(() => {
    if (tenant?.merchantId) {
      const mid = Number(tenant.merchantId)
      if (mid > 0) {
        getMerchantDetail(mid)
          .then((merchant) => {
            if (merchant && merchant.sipDomain) {
              setDomain(merchant.sipDomain)
              addLog(`[YunshuCallSDK][${new Date().toLocaleTimeString()}] 已自动加载商户 [${merchant.name || mid}] 的 SIP 域: ${merchant.sipDomain}`)
            }
          })
          .catch((err) => {
            console.error('Failed to fetch merchant detail:', err)
          })
      }
    }
  }, [tenant?.merchantId])

  // Handle timer for call duration
  useEffect(() => {
    if (callState === 'connected') {
      timerRef.current = setInterval(() => {
        setCallDuration((prev) => prev + 1)
      }, 1000)
    } else {
      if (timerRef.current) {
        clearInterval(timerRef.current)
        timerRef.current = null
      }
      if (callState === 'idle') {
        setCallDuration(0)
      }
    }
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [callState])

  // Clean up SDK on unmount
  useEffect(() => {
    return () => {
      if (sdkRef.current) {
        addLog('正在清理 WebRTC 客户端连接...')
        sdkRef.current.destroy()
      }
    }
  }, [])

  const handleRegister = () => {
    if (sdkRef.current) {
      addLog('正在断开现有连接...')
      sdkRef.current.destroy()
      sdkRef.current = null
      setRegistered(false)
      setRegState('disconnected')
    }

    addLog(`正在初始化 WebRTC SDK: sip:${ext}@${domain}`)
    try {
      const sdk = new YunshuCallSDK({
        wsUrl,
        ext,
        password,
        domain,
        audioElement: audioRef.current || undefined,
      })

      sdk.on('connecting', () => {
        setRegState('connecting')
      })

      sdk.on('connected', () => {
        // Will be outputted via log event
      })

      sdk.on('disconnected', () => {
        setRegState('disconnected')
        setRegistered(false)
      })

      sdk.on('registered', () => {
        setRegState('registered')
        setRegistered(true)
        message.success(`分机 ${ext} 注册成功`)
      })

      sdk.on('unregistered', () => {
        setRegState('disconnected')
        setRegistered(false)
      })

      sdk.on('registrationFailed', (cause: any) => {
        setRegState('failed')
        setRegistered(false)
        message.error(`注册失败: ${cause || '网络验证未通过'}`)
      })

      sdk.on('callDialing', (remoteUser) => {
        setCallState('dialing')
        setIncomingCaller(null)
      })

      sdk.on('callRinging', (data: any) => {
        setCallState('ringing')
        if (data && data.direction === 'incoming') {
          setIncomingCaller(data.remoteUser)
          message.info(`收到分机来电: ${data.remoteUser}`)
        }
      })

      sdk.on('callConnected', () => {
        setCallState('connected')
        message.success('通话已接通')
      })

      sdk.on('callFailed', (cause: any) => {
        setCallState('ended')
        message.error(`呼叫未成功: ${cause || '被叫拒接/无人应答'}`)
        setTimeout(() => {
          setCallState('idle')
          setIncomingCaller(null)
        }, 2000)
      })

      sdk.on('callEnded', () => {
        setCallState('ended')
        message.info('通话结束')
        setTimeout(() => {
          setCallState('idle')
          setIncomingCaller(null)
        }, 2000)
      })

      sdk.on('log', (msg: string) => {
        addLog(msg)
      })

      sdk.register()
      sdkRef.current = sdk
    } catch (e: any) {
      addLog(`[Error] 连接配置异常: ${e.message}`)
      message.error(`配置异常: ${e.message}`)
    }
  }

  const handleDisconnect = () => {
    if (sdkRef.current) {
      sdkRef.current.destroy()
      sdkRef.current = null
      setRegistered(false)
      setRegState('disconnected')
      addLog(`[YunshuCallSDK][${new Date().toLocaleTimeString()}] 已手动断开客户端连接`)
      message.info('连接已关闭')
    }
  }

  const handleCall = () => {
    if (!sdkRef.current || !registered) {
      message.warning('请先注册 SIP 分机后再拨号')
      return
    }
    if (!callee.trim()) {
      message.warning('请输入目标呼叫号码')
      return
    }
    sdkRef.current.call(callee)
  }

  const handleHangup = () => {
    if (sdkRef.current) {
      sdkRef.current.hangup()
    } else {
      setCallState('idle')
    }
  }

  const handleAnswer = () => {
    if (sdkRef.current && callState === 'ringing') {
      sdkRef.current.answer()
    }
  }

  const appendDigit = (digit: string) => {
    setCallee((prev) => prev + digit)
    if (sdkRef.current && callState === 'connected') {
      sdkRef.current.sendDTMF(digit)
    }
  }

  const formatDuration = (sec: number) => {
    const mins = Math.floor(sec / 60)
    const secs = sec % 60
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  const fillPreset = (selectedExt: string) => {
    setExt(selectedExt)
    setPassword('123456')
    setDomain('sip.yunshu.local')
    addLog(`[YunshuCallSDK][${new Date().toLocaleTimeString()}] 已载入预设分机 ${selectedExt} 配置`)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* Hidden Audio element to play remote WebRTC call streams */}
      <audio ref={audioRef} autoPlay id="remoteAudio" style={{ display: 'none' }} />

      <Row gutter={[24, 24]}>
        {/* SIP Registration Settings */}
        <Col xs={24} lg={10}>
          <Space direction="vertical" size="middle" className="w-full">
            <Card title="SIP 分机注册配置" className="shadow-soft">
              <Form layout="vertical">
                <Form.Item label="WebSocket 服务器地址" required>
                  <Input value={wsUrl} onChange={(e) => setWsUrl(e.target.value)} placeholder="ws://127.0.0.1:5066" />
                </Form.Item>
                <Row gutter={12}>
                  <Col span={12}>
                    <Form.Item label="SIP 分机号" required>
                      <Input value={ext} onChange={(e) => setExt(e.target.value)} placeholder="例如 1001" />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="注册密码" required>
                      <Input.Password value={password} onChange={(e) => setPassword(e.target.value)} placeholder="默认 123456" />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item label="SIP 域名/服务器" required>
                  <Input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="sip.yunshu.local" />
                </Form.Item>

                <div className="mb-4">
                  <span className="block mb-2 text-xs font-semibold text-slate-500">快速载入本地测试账户：</span>
                  <Space>
                    <Button size="small" onClick={() => fillPreset('1001')}>分机 1001</Button>
                    <Button size="small" onClick={() => fillPreset('1002')}>分机 1002</Button>
                  </Space>
                </div>

                <Space className="w-full justify-between">
                  <div>
                    <Typography.Text className="mr-2">当前状态:</Typography.Text>
                    {regState === 'disconnected' && <Tag color="default">未连接</Tag>}
                    {regState === 'connecting' && <Tag color="blue">正在连接...</Tag>}
                    {regState === 'registered' && <Tag color="green">已注册 (在线)</Tag>}
                    {regState === 'failed' && <Tag color="red">注册失败</Tag>}
                  </div>
                  <Space>
                    {!registered ? (
                      <Button type="primary" onClick={handleRegister} loading={regState === 'connecting'}>
                        注册分机
                      </Button>
                    ) : (
                      <Button danger onClick={handleDisconnect}>
                        断开连接
                      </Button>
                    )}
                  </Space>
                </Space>
              </Form>
            </Card>

            <Card title="信令及媒体日志" className="shadow-soft">
              <div
                style={{
                  height: '240px',
                  overflowY: 'auto',
                  background: '#1e1e1e',
                  color: '#00ff00',
                  fontFamily: 'monospace',
                  padding: '12px',
                  borderRadius: '6px',
                  fontSize: '12px',
                  lineHeight: '1.6',
                }}
              >
                {logs.length === 0 ? (
                  <div style={{ color: '#888' }}>暂无信令或媒体日志。点击右侧拨号或上方注册查看数据流...</div>
                ) : (
                  logs.map((log, idx) => <div key={idx}>{log}</div>)
                )}
              </div>
            </Card>
          </Space>
        </Col>

        {/* Dialpad Dialer */}
        <Col xs={24} lg={14}>
          <Card title="软电话拨号面板" className="shadow-soft">
            <div className="flex flex-col items-center justify-center py-4">
              {/* Screen Display */}
              <div
                style={{
                  width: '100%',
                  maxWidth: '360px',
                  background: '#f5f5f5',
                  borderRadius: '12px',
                  padding: '16px',
                  textAlign: 'center',
                  marginBottom: '24px',
                  boxShadow: 'inset 0 2px 4px rgba(0,0,0,0.06)',
                }}
              >
                <div style={{ fontSize: '12px', color: '#888', textTransform: 'uppercase', letterSpacing: '1px' }}>
                  {callState === 'idle' && '待机空闲'}
                  {callState === 'dialing' && '正在起呼...'}
                  {callState === 'ringing' && (incomingCaller ? `来电: ${incomingCaller}` : '对方振铃中...')}
                  {callState === 'connected' && '通话中'}
                  {callState === 'ended' && '已挂断'}
                </div>
                <div style={{ fontSize: '28px', fontWeight: 'bold', margin: '8px 0', minHeight: '38px', color: '#333' }}>
                  {callee || (incomingCaller ? incomingCaller : '请输入号码')}
                </div>
                <div style={{ fontSize: '14px', color: '#1890ff', fontWeight: '500' }}>
                  {callState === 'connected' ? formatDuration(callDuration) : '00:00'}
                </div>
              </div>

              {/* Dial Buttons grid */}
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(3, 1fr)',
                  gap: '16px',
                  width: '100%',
                  maxWidth: '300px',
                  marginBottom: '24px',
                }}
              >
                {['1', '2', '3', '4', '5', '6', '7', '8', '9', '*', '0', '#'].map((digit) => (
                  <Button
                    key={digit}
                    shape="circle"
                    style={{
                      width: '60px',
                      height: '60px',
                      fontSize: '20px',
                      fontWeight: '500',
                      margin: '0 auto',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                    onClick={() => appendDigit(digit)}
                  >
                    {digit}
                  </Button>
                ))}
              </div>

              {/* Call Controls */}
              <Space size="large">
                <Button
                  shape="circle"
                  style={{ width: '48px', height: '48px' }}
                  onClick={() => setCallee('')}
                  disabled={callState !== 'idle'}
                >
                  清除
                </Button>

                {callState === 'ringing' && incomingCaller ? (
                  <Button
                    type="primary"
                    shape="circle"
                    style={{ width: '64px', height: '64px', backgroundColor: '#52c41a', borderColor: '#52c41a' }}
                    onClick={handleAnswer}
                  >
                    接听
                  </Button>
                ) : (
                  <Button
                    type="primary"
                    shape="circle"
                    style={{ width: '64px', height: '64px', backgroundColor: '#52c41a', borderColor: '#52c41a' }}
                    disabled={callState !== 'idle' || !registered}
                    onClick={handleCall}
                  >
                    呼出
                  </Button>
                )}

                <Button
                  type="primary"
                  danger
                  shape="circle"
                  style={{ width: '64px', height: '64px' }}
                  disabled={callState === 'idle'}
                  onClick={handleHangup}
                >
                  挂断
                </Button>
              </Space>
            </div>
          </Card>
        </Col>
      </Row>
    </Space>
  )
}
