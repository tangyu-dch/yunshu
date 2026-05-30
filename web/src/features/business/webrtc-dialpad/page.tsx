import { Button, Card, Col, Form, Input, Row, Space, Tag, Typography, message } from 'antd'
import { useEffect, useRef, useState } from 'react'
import JsSIP from 'jssip'
import { useAuthStore } from '../../../store/auth'
import { getMerchantDetail } from '../../../api/operate'

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

  // WebRTC / JsSIP instances
  const [ua, setUa] = useState<JsSIP.UA | null>(null)
  const [session, setSession] = useState<any | null>(null)
  const [registered, setRegistered] = useState(false)
  const [regState, setRegState] = useState<'disconnected' | 'connecting' | 'registered' | 'failed'>('disconnected')
  const [callState, setCallState] = useState<'idle' | 'dialing' | 'ringing' | 'connected' | 'ended'>('idle')
  const [logs, setLogs] = useState<string[]>([])

  const audioRef = useRef<HTMLAudioElement | null>(null)
  const timerRef = useRef<any>(null)

  const addLog = (msg: string) => {
    const time = new Date().toLocaleTimeString()
    setLogs((prev) => [`[${time}] ${msg}`, ...prev.slice(0, 99)])
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
              addLog(`已自动加载商户 [${merchant.name || mid}] 的 SIP 域: ${merchant.sipDomain}`)
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

  // Clean up UA on unmount
  useEffect(() => {
    return () => {
      if (ua) {
        addLog('正在清理 WebRTC 客户端连接...')
        ua.stop()
      }
    }
  }, [ua])

  const handleRegister = () => {
    if (ua) {
      addLog('正在断开现有连接...')
      ua.stop()
      setUa(null)
      setRegistered(false)
      setRegState('disconnected')
    }

    addLog(`正在初始化 WebRTC 客户端: sip:${ext}@${domain}`)
    try {
      const socket = new JsSIP.WebSocketInterface(wsUrl)
      const config = {
        sockets: [socket],
        uri: `sip:${ext}@${domain}`,
        password: password,
        register: true,
        session_timers: false,
      }

      const newUa = new JsSIP.UA(config)

      newUa.on('connecting', () => {
        setRegState('connecting')
        addLog('SIP 正在连接 WebSockets 服务器...')
      })

      newUa.on('connected', () => {
        addLog('WebSockets 连接已建立')
      })

      newUa.on('disconnected', () => {
        setRegState('disconnected')
        setRegistered(false)
        addLog('SIP 连接已断开')
      })

      newUa.on('registered', () => {
        setRegState('registered')
        setRegistered(true)
        addLog('SIP 注册成功! 随时可以呼入或呼出')
        message.success(`分机 ${ext} 注册成功`)
      })

      newUa.on('unregistered', () => {
        setRegState('disconnected')
        setRegistered(false)
        addLog('SIP 分机已注销')
      })

      newUa.on('registrationFailed', (e: any) => {
        setRegState('failed')
        setRegistered(false)
        addLog(`SIP 注册失败: ${e.cause || '未知原因'}`)
        message.error(`注册失败: ${e.cause || '网络错误'}`)
      })

      newUa.on('newRTCSession', (e: any) => {
        const rtcSession = e.session
        setSession(rtcSession)

        if (rtcSession.direction === 'incoming') {
          addLog(`收到来电: 来自 ${rtcSession.remote_identity.uri.user}`)
          setCallState('ringing')

          // 自动应答或者提示
          message.info(`收到分机来电: ${rtcSession.remote_identity.uri.user}`)
        } else {
          addLog(`正在呼出: 拨号给 ${rtcSession.remote_identity.uri.user}`)
          setCallState('dialing')
        }

        rtcSession.on('peerconnection', (data: any) => {
          addLog('WebRTC PeerConnection 建立，开始协商媒体流...')
          data.peerconnection.addEventListener('track', (event: any) => {
            addLog('检测到远端媒体音轨，绑定到音频播放器')
            if (audioRef.current && event.streams[0]) {
              audioRef.current.srcObject = event.streams[0]
              audioRef.current.play().catch((err) => {
                addLog(`自动播放音频流失败: ${err.message}`)
              })
            }
          })
        })

        rtcSession.on('connecting', () => {
          addLog('WebRTC 信令呼叫中...')
        })

        rtcSession.on('progress', () => {
          setCallState('ringing')
          addLog('呼叫进行中，对方振铃中...')
        })

        rtcSession.on('accepted', () => {
          setCallState('connected')
          addLog('通话已接通! 媒体协商完成')
          message.success('通话已接通')
        })

        rtcSession.on('failed', (data: any) => {
          setCallState('ended')
          addLog(`呼叫失败/拒绝: ${data.cause}`)
          message.error(`呼叫未成功: ${data.cause}`)
          setTimeout(() => setCallState('idle'), 2000)
        })

        rtcSession.on('ended', () => {
          setCallState('ended')
          addLog('通话结束')
          message.info('通话结束')
          setTimeout(() => setCallState('idle'), 2000)
        })
      })

      newUa.start()
      setUa(newUa)
    } catch (e: any) {
      addLog(`连接配置异常: ${e.message}`)
      message.error(`配置异常: ${e.message}`)
    }
  }

  const handleDisconnect = () => {
    if (ua) {
      ua.stop()
      setUa(null)
      setRegistered(false)
      setRegState('disconnected')
      addLog('已手动断开客户端连接')
      message.info('连接已关闭')
    }
  }

  const handleCall = () => {
    if (!ua || !registered) {
      message.warning('请先注册 SIP 分机后再拨号')
      return
    }
    if (!callee.trim()) {
      message.warning('请输入目标呼叫号码')
      return
    }

    addLog(`开始呼叫号码: ${callee}`)
    const options = {
      mediaConstraints: { audio: true, video: false },
      rtcOfferConstraints: { offerToReceiveAudio: true, offerToReceiveVideo: false },
    }
    ua.call(`sip:${callee}@${domain}`, options)
  }

  const handleHangup = () => {
    if (session) {
      addLog('正在挂断当前通话...')
      session.terminate()
    } else {
      setCallState('idle')
    }
  }

  const handleAnswer = () => {
    if (session && callState === 'ringing') {
      addLog('接听来电...')
      const options = {
        mediaConstraints: { audio: true, video: false },
      }
      session.answer(options)
    }
  }

  const appendDigit = (digit: string) => {
    setCallee((prev) => prev + digit)
    if (session && callState === 'connected') {
      addLog(`发送 DTMF 按键: ${digit}`)
      session.sendDTMF(digit)
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
    addLog(`已载入预设分机 ${selectedExt} 配置`)
  }

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* Hidden Audio element to play remote WebRTC call streams */}
      <audio ref={audioRef} autoPlay id="remoteAudio" style={{ display: 'none' }} />

      <div className="mb-2">
        <Typography.Text type="secondary">
          利用 SIP-over-WebSocket 直接在浏览器中注册分机并进行呼叫测试。请确保浏览器已授予麦克风权限。
        </Typography.Text>
      </div>

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
                  <Typography.Text type="secondary" className="block mb-2">快速载入本地测试账户：</Typography.Text>
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
                  {callState === 'ringing' && '对方振铃中...'}
                  {callState === 'connected' && '通话中'}
                  {callState === 'ended' && '已挂断'}
                </div>
                <div style={{ fontSize: '28px', fontWeight: 'bold', margin: '8px 0', minHeight: '38px', color: '#333' }}>
                  {callee || '请输入号码'}
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

                {callState === 'ringing' && session?.direction === 'incoming' ? (
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
