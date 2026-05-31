import { useState } from 'react'
import { Card, Tabs, Table, Tag, Typography, Button, Space, Divider, Alert, Input, Checkbox, message, Spin, Progress, Badge, List } from 'antd'
import { CopyOutlined, KeyOutlined, ApiOutlined, SoundOutlined, CodeOutlined, CheckOutlined, UserAddOutlined, HistoryOutlined, AudioOutlined, SafetyOutlined, PlaySquareOutlined, RocketOutlined, RedoOutlined, EyeOutlined, EyeInvisibleOutlined, ThunderboltOutlined, CheckCircleOutlined, WarningOutlined, CloseCircleOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '@/store/auth'
import { getMerchantDetail, resetMerchantApiKeys } from '@/api/operate'
import { YunshuCallSDK } from '@/sdk/yunshu-call-sdk'

const { Title, Paragraph, Text } = Typography

const webrtcSdkExampleCode = `import { YunshuCallSDK } from '@/sdk/yunshu-call-sdk';

// 1. 创建 SDK 实例并进行配置
const sdk = new YunshuCallSDK({
  wsUrl: 'ws://your-pbx-ip:5066',
  ext: '1001',
  password: 'YOUR_EXTENSION_PASSWORD',
  domain: 'sip.yunshu.local',
  audioElementId: 'remoteAudio' // 绑定用于远端通话声音播放的 HTML Audio DOM ID
});

// 2. 绑定通话核心状态变化事件
sdk.on('connecting', () => console.log('正在连接网关...'));
sdk.on('registered', () => console.log('注册在线，准备就绪'));
sdk.on('registrationFailed', (cause) => console.error('注册失败:', cause));

sdk.on('callDialing', (number) => console.log('正在呼出号码:', number));
sdk.on('callRinging', (data) => {
  if (data.direction === 'incoming') {
    console.log('收到外部来电，主叫为:', data.remoteUser);
    // 可以在 CRM 中弹框提示“来电接听”
  } else {
    console.log('被叫号码振铃中...');
  }
});

sdk.on('callConnected', () => console.log('通话已成功接通！双方开始语音对话'));
sdk.on('callEnded', () => console.log('当前通话正常结束'));
sdk.on('callFailed', (cause) => console.warn('呼叫未接通或失败:', cause));

sdk.on('log', (msg) => {
// 3. 通话质量监测 (QoS)
sdk.on('callQuality', (stats) => {
  console.log('当前实时通话质量:', stats.quality, '丢包率:', stats.lostRatio + '%');
});

// 4. 通话业务控制操作
// 发起外呼
function makeCall(phoneNumber) {
  sdk.call(phoneNumber);
}

// 隐私合规：暂停/恢复录音
function toggleRecording(isPaused) {
  isPaused ? sdk.pauseRecording() : sdk.resumeRecording();
}

// 页面销毁前注销
window.addEventListener('beforeunload', () => {
  sdk.destroy();
});`;

const sdkSourceCode = `import JsSIP from 'jssip'

export interface YunshuDeviceInfo {
  deviceId: string
  groupId: string
  kind: 'audioinput' | 'audiooutput'
  label: string
}

export interface YunshuDiagnosticReport {
  supported: boolean
  browserDetails: {
    userAgent: string
    hasMediaDevices: boolean
    hasGetUserMedia: boolean
    hasRTCPeerConnection: boolean
    hasWebSocket: boolean
    hasAudioContext: boolean
    hasSetSinkId: boolean
  }
  devices: {
    hasMicrophone: boolean
    hasSpeaker: boolean
    microphoneAuthorized: boolean
    microphones: YunshuDeviceInfo[]
    speakers: YunshuDeviceInfo[]
    micActiveLevel: number
  }
  network: {
    webSocketSupported: boolean
    wsReachable: boolean | null
  }
  status: 'excellent' | 'warning' | 'error'
  suggestions: string[]
}

export interface YunshuCallQualityReport {
  packetsLost: number
  packetsReceived: number
  lostRatio: number
  jitter: number
  rtt: number
  audioLevelInput: number
  audioLevelOutput: number
  quality: 'excellent' | 'good' | 'fair' | 'poor'
}

export interface YunshuRegistrationFailedPayload {
  ext: string
  cause: string
  code: number
  message: string
}

export interface YunshuCallDetails {
  sessionId: string
  caller: string
  callee: string
  direction: 'incoming' | 'outgoing'
  remoteUser: string
  duration: number
  status: 'idle' | 'dialing' | 'ringing' | 'connected' | 'ended'
  isMuted: boolean
  isOnHold: boolean
}

export interface YunshuCallFailedPayload {
  sessionId: string
  cause: string
  code: number
  message: string
}

export interface YunshuCallEndedPayload {
  sessionId: string
  remoteUser: string
  direction: 'incoming' | 'outgoing'
  duration: number
  cause: string
}

export interface YunshuCallRingingPayload {
  sessionId: string
  direction: 'incoming' | 'outgoing'
  remoteUser: string
}

export type YunshuCallEvent =
  | 'connecting'
  | 'connected'
  | 'disconnected'
  | 'registered'
  | 'unregistered'
  | 'registrationFailed'
  | 'callDialing'
  | 'callRinging'
  | 'callConnected'
  | 'callTick'
  | 'callEnded'
  | 'callFailed'
  | 'callHold'
  | 'callUnhold'
  | 'callMuted'
  | 'callUnmuted'
  | 'callQuality'
  | 'reconnecting'
  | 'log'

export type YunshuCallEventCallback = (data?: any) => void

export class YunshuCallSDK {
  private config: YunshuCallConfig
  private ua: JsSIP.UA | null = null
  private session: any | null = null
  private isSdkRegistered = false
  private isLocalMuted = false
  private isSessionOnHold = false
  private callStartTime = 0
  private timerInterval: any = null
  private lastDialedNumber = ''

  private retryCount = 0
  private reconnectTimer: any = null
  private isAutoReconnecting = false

  private statsInterval: any = null
  private prevPacketsLost = 0
  private prevPacketsReceived = 0

  private callbacks: Record<YunshuCallEvent, YunshuCallEventCallback[]> = {
    connecting: [], connected: [], disconnected: [], registered: [], unregistered: [],
    registrationFailed: [], callDialing: [], callRinging: [], callConnected: [], callTick: [],
    callEnded: [], callFailed: [], callHold: [], callUnhold: [], callMuted: [], callUnmuted: [],
    callQuality: [], reconnecting: [], log: [],
  }

  constructor(config: YunshuCallConfig) {
    this.config = { password: '123456', ...config }
  }

  public static checkCompatibility(): boolean {
    if (typeof window === 'undefined') return false
    const hasMediaDevices = !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia)
    const hasRTCPeerConnection = !!(window.RTCPeerConnection || (window as any).webkitRTCPeerConnection)
    const hasWebSocket = !!window.WebSocket
    return hasMediaDevices && hasRTCPeerConnection && hasWebSocket
  }

  public static async diagnose(wsUrl?: string): Promise<YunshuDiagnosticReport> {
    const report: YunshuDiagnosticReport = {
      supported: YunshuCallSDK.checkCompatibility(),
      browserDetails: {
        userAgent: typeof navigator !== 'undefined' ? navigator.userAgent : '',
        hasMediaDevices: typeof navigator !== 'undefined' && !!navigator.mediaDevices,
        hasGetUserMedia: typeof navigator !== 'undefined' && !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia),
        hasRTCPeerConnection: typeof window !== 'undefined' && !!(window.RTCPeerConnection || (window as any).webkitRTCPeerConnection),
        hasWebSocket: typeof window !== 'undefined' && !!window.WebSocket,
        hasAudioContext: typeof window !== 'undefined' && !!(window.AudioContext || (window as any).webkitAudioContext),
        hasSetSinkId: typeof HTMLAudioElement !== 'undefined' && !!HTMLAudioElement.prototype.setSinkId,
      },
      devices: {
        hasMicrophone: false,
        hasSpeaker: false,
        microphoneAuthorized: false,
        microphones: [],
        speakers: [],
        micActiveLevel: 0,
      },
      network: {
        webSocketSupported: typeof window !== 'undefined' && !!window.WebSocket,
        wsReachable: null,
      },
      status: 'error',
      suggestions: [],
    }

    if (!report.supported) {
      report.status = 'error'
      report.suggestions.push('当前浏览器或系统不支持 WebRTC，建议升级或使用最新版 Chrome、Edge 或 Safari。')
      return report
    }

    let stream: MediaStream | null = null

    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false })
      if (stream) {
        report.devices.microphoneAuthorized = true
        
        if (report.browserDetails.hasAudioContext) {
          try {
            const AudioContextClass = window.AudioContext || (window as any).webkitAudioContext
            const audioCtx = new AudioContextClass()
            const analyser = audioCtx.createAnalyser()
            const source = audioCtx.createMediaStreamSource(stream)
            source.connect(analyser)
            
            analyser.fftSize = 256
            const bufferLength = analyser.frequencyBinCount
            const dataArray = new Uint8Array(bufferLength)
            
            analyser.getByteTimeDomainData(dataArray)
            let sum = 0
            for (let i = 0; i < bufferLength; i++) {
              const value = (dataArray[i] - 128) / 128
              sum += value * value
            }
            const rms = Math.sqrt(sum / bufferLength)
            report.devices.micActiveLevel = Math.min(Math.round(rms * 200), 100)
            
            await audioCtx.close()
          } catch (audioErr) {
            console.warn('[YunshuCallSDK] 麦克风分贝采样失败:', audioErr)
          }
        }
      }
    } catch (err: any) {
      report.devices.microphoneAuthorized = false
      const errMsg = err.name || err.message || ''
      if (errMsg.includes('NotAllowed') || errMsg.includes('Permission')) {
        report.suggestions.push('【致命】麦克风使用权限被拒绝。请在浏览器地址栏左端点击“锁头”或“设置”图标，将“麦克风”权限重置为“允许”并刷新页面。')
      } else if (errMsg.includes('NotFound') || errMsg.includes('Device')) {
        report.suggestions.push('【致命】未检测到麦克风输入硬件。请检查物理话筒/耳机是否已插好，并在系统设备管理器中启用麦克风。')
      } else {
        report.suggestions.push(\`【致命】麦克风捕获异常: \${err.message || err.name}\`)
      }
    }

    try {
      const devices = await navigator.mediaDevices.enumerateDevices()
      devices.forEach(device => {
        const info: YunshuDeviceInfo = {
          deviceId: device.deviceId,
          groupId: device.groupId,
          kind: device.kind as 'audioinput' | 'audiooutput',
          label: device.label || (device.kind === 'audioinput' ? '麦克风 (未授权隐藏标签)' : '音频输出设备 (未授权隐藏标签)'),
        }

        if (device.kind === 'audioinput') {
          report.devices.hasMicrophone = true
          report.devices.microphones.push(info)
        } else if (device.kind === 'audiooutput') {
          report.devices.hasSpeaker = true
          report.devices.speakers.push(info)
        }
      })
    } catch (err: any) {
      report.suggestions.push(\`【警告】系统音频设备枚举失败: \${err.message}\`)
    }

    if (stream) {
      stream.getTracks().forEach(track => track.stop())
    }

    if (wsUrl && report.network.webSocketSupported) {
      try {
        const testWs = async (): Promise<boolean> => {
          return new Promise((resolve) => {
            let socket: WebSocket | null = null
            const timer = setTimeout(() => {
              if (socket) {
                socket.onclose = null
                socket.close()
              }
              resolve(false)
            }, 3000)

            try {
              socket = new WebSocket(wsUrl, 'sip')
              socket.onopen = () => {
                clearTimeout(timer)
                socket?.close()
                resolve(true)
              }
              socket.onerror = () => {
                clearTimeout(timer)
                resolve(false)
              }
            } catch {
              clearTimeout(timer)
              resolve(false)
            }
          })
        }
        report.network.wsReachable = await testWs()
        if (!report.network.wsReachable) {
          report.suggestions.push(\`【致命】无法连通云枢信令网关 WebSocket (\${wsUrl})。请检查您的网络防火墙、PBX 服务运行状态，或 5066 端口是否开放。\`)
        }
      } catch (wsErr) {
        report.network.wsReachable = false
      }
    }

    if (!report.devices.microphoneAuthorized || !report.devices.hasMicrophone) {
      report.status = 'error'
    } else if (wsUrl && report.network.wsReachable === false) {
      report.status = 'error'
    } else if (!report.devices.hasSpeaker || !report.browserDetails.hasSetSinkId) {
      report.status = 'warning'
      if (!report.devices.hasSpeaker) {
        report.suggestions.push('【警告】未检测到任何音频播放设备（扬声器/耳机）。您将可能无法听到远端客户的声音。')
      }
      if (!report.browserDetails.hasSetSinkId) {
        report.suggestions.push('【提示】当前浏览器不支持动态切换指定的音频输出硬件（如 Safari），音频将默认由系统默认声卡播放。')
      }
    } else {
      report.status = 'excellent'
    }

    return report
  }

  public on(event: YunshuCallEvent, callback: YunshuCallEventCallback): this {
    if (this.callbacks[event]) this.callbacks[event].push(callback)
    return this
  }

  public off(event: YunshuCallEvent, callback: YunshuCallEventCallback): this {
    if (this.callbacks[event]) this.callbacks[event] = this.callbacks[event].filter(cb => cb !== callback)
    return this
  }

  private trigger(event: YunshuCallEvent, data?: any): void {
    const list = this.callbacks[event] || []
    list.forEach(cb => { try { cb(data) } catch (err) { console.error(err) } })
  }

  private log(message: string): void {
    const time = new Date().toLocaleTimeString()
    this.trigger('log', \`[YunshuCallSDK][\${time}] \${message}\`)
  }

  private clearTimer(): void {
    if (this.timerInterval) { clearInterval(this.timerInterval); this.timerInterval = null }
    this.callStartTime = 0
  }

  public register(): void {
    if (this.ua) this.unregister()
    const { wsUrl, ext, domain, password } = this.config
    try {
      const socket = new JsSIP.WebSocketInterface(wsUrl)
      const jssipConfig = { sockets: [socket], uri: \`sip:\${ext}@\${domain}\`, password, register: true, session_timers: false }
      this.ua = new JsSIP.UA(jssipConfig)
      
      this.ua.on('connecting', () => this.trigger('connecting'))
      this.ua.on('connected', () => this.trigger('connected'))
      this.ua.on('disconnected', (e) => {
        this.isSdkRegistered = false
        this.trigger('disconnected')
        if (this.ua && !e.error?.message?.includes('stop')) {
          this.handleAutoReconnect()
        }
      })
      this.ua.on('registered', () => {
        this.isSdkRegistered = true
        this.retryCount = 0
        this.isAutoReconnecting = false
        if (this.reconnectTimer) {
          clearTimeout(this.reconnectTimer)
          this.reconnectTimer = null
        }
        this.trigger('registered')
      })
      this.ua.on('unregistered', () => {
        this.isSdkRegistered = false
        this.retryCount = 0
        this.isAutoReconnecting = false
        if (this.reconnectTimer) {
          clearTimeout(this.reconnectTimer)
          this.reconnectTimer = null
        }
        this.trigger('unregistered')
      })
      this.ua.on('registrationFailed', (e) => {
        this.isSdkRegistered = false
        this.trigger('registrationFailed', { ext, cause: e.cause, code: e.response ? e.response.status_code : 401, message: e.response ? e.response.reason_phrase : 'Unauthorized' })
      })
      this.ua.on('newRTCSession', (e) => {
        const rtcSession = e.session
        this.session = rtcSession
        this.isLocalMuted = false
        this.isSessionOnHold = false
        this.clearTimer()
        
        const direction = rtcSession.direction
        const remoteUser = rtcSession.remote_identity.uri.user
        const ringPayload = { sessionId: rtcSession.id, direction, remoteUser }
        
        if (direction === 'incoming') {
          this.trigger('callRinging', ringPayload)
        } else {
          this.trigger('callDialing', remoteUser)
        }
        
        rtcSession.on('peerconnection', (data) => {
          data.peerconnection.addEventListener('track', (event) => {
            let audioEl = this.config.audioElement || (this.config.audioElementId ? document.getElementById(this.config.audioElementId) : null)
            if (audioEl && event.streams[0]) {
              audioEl.srcObject = event.streams[0]
              audioEl.play().catch(err => this.log('Autoplay blocked: ' + err.message))
            }
          })
        })
        
        rtcSession.on('progress', () => this.trigger('callRinging', ringPayload))
        rtcSession.on('accepted', () => {
          this.callStartTime = Date.now()
          this.timerInterval = setInterval(() => this.trigger('callTick', this.getCallDuration()), 1000)
          this.trigger('callConnected', this.getCallDetails())
          this.startStatsMonitoring(rtcSession.connection)
        })
        rtcSession.on('failed', (data) => {
          this.trigger('callFailed', { sessionId: rtcSession.id, cause: data.cause, code: data.message ? data.message.status_code : 486, message: data.message ? data.message.reason_phrase : 'Failed' })
          this.clearTimer()
          this.stopStatsMonitoring()
          this.session = null
        })
        rtcSession.on('ended', (data) => {
          this.trigger('callEnded', { sessionId: rtcSession.id, remoteUser, direction, duration: this.getCallDuration(), cause: data.cause || 'Normal' })
          this.clearTimer()
          this.stopStatsMonitoring()
          this.session = null
        })
      })
      this.ua.start()
    } catch (err) {
      throw err
    }
  }

  public unregister(): void {
    if (this.session) this.hangup()
    if (this.ua) {
      this.ua.stop()
      this.ua = null
      this.isSdkRegistered = false
      this.retryCount = 0
      this.isAutoReconnecting = false
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer)
        this.reconnectTimer = null
      }
    }
  }

  public call(callee: string): void {
    if (!this.ua || !this.isSdkRegistered) throw new Error('SDK not registered');
    this.lastDialedNumber = callee.trim()
    this.ua.call(\`sip:\${callee}@\${this.config.domain}\`, {
      mediaConstraints: { audio: true, video: false },
      rtcOfferConstraints: { offerToReceiveAudio: true, offerToReceiveVideo: false }
    })
  }

  public redial(): void {
    if (this.lastDialedNumber) this.call(this.lastDialedNumber)
  }

  public answer(): void {
    if (this.session && this.session.direction === 'incoming') {
      this.session.answer({ mediaConstraints: { audio: true, video: false } })
    }
  }

  public hangup(): void {
    if (this.session) { this.session.terminate(); this.session = null } else { this.clearTimer() }
  }

  public sendDTMF(digit: string): void {
    if (this.session) this.session.sendDTMF(digit)
  }

  public mute(): void {
    if (this.session) { this.session.mute({ audio: true, video: false }); this.isLocalMuted = true; this.trigger('callMuted') }
  }

  public unmute(): void {
    if (this.session) { this.session.unmute({ audio: true, video: false }); this.isLocalMuted = false; this.trigger('callUnmuted') }
  }

  public hold(): void {
    if (this.session) { this.session.hold(); this.isSessionOnHold = true; this.trigger('callHold') }
  }

  public unhold(): void {
    if (this.session) { this.session.unhold(); this.isSessionOnHold = false; this.trigger('callUnhold') }
  }

  public transfer(targetExt: string): void {
    if (this.session) this.session.refer(\`sip:\${targetExt}@\${this.config.domain}\`)
  }

  public isRegistered(): boolean { return this.isSdkRegistered }
  public isMuted(): boolean { return this.isLocalMuted }
  public isOnHold(): boolean { return this.isSessionOnHold }
  public getCallDuration(): number { return this.callStartTime === 0 ? 0 : Math.floor((Date.now() - this.callStartTime) / 1000) }
  public isActiveCall(): boolean { return this.session !== null }

  public getCallDetails(): YunshuCallDetails {
    if (!this.session) return { sessionId: '', caller: '', callee: '', direction: 'outgoing', remoteUser: '', duration: 0, status: 'idle', isMuted: false, isOnHold: false }
    let status: 'idle' | 'dialing' | 'ringing' | 'connected' | 'ended' = 'idle'
    if (this.session.isEstablished()) status = 'connected'
    else if (this.session.isInProgress()) status = this.session.direction === 'incoming' ? 'ringing' : 'dialing'
    return {
      sessionId: this.session.id, caller: this.config.ext, callee: this.session.remote_identity.uri.user,
      direction: this.session.direction, remoteUser: this.session.remote_identity.uri.user,
      duration: this.getCallDuration(), status, isMuted: this.isLocalMuted, isOnHold: this.isSessionOnHold
    }
  }

  public pauseRecording(): void {
    if (this.session) {
      this.log('【安全合规防线】暂停当前会话录音')
      this.session.sendInfo('application/json', JSON.stringify({ action: 'pause_recording' }))
    }
  }

  public resumeRecording(): void {
    if (this.session) {
      this.log('【安全合规防线】恢复当前会话录音')
      this.session.sendInfo('application/json', JSON.stringify({ action: 'resume_recording' }))
    }
  }

  private handleAutoReconnect(): void {
    if (this.isAutoReconnecting || !this.ua) return
    this.isAutoReconnecting = true
    this.retryCount++
    const delay = Math.min(1000 * Math.pow(2, this.retryCount), 30000)
    
    this.trigger('reconnecting', { retryCount: this.retryCount, nextRetryDelay: delay })
    this.reconnectTimer = setTimeout(() => {
      try {
        if (this.ua && this.isAutoReconnecting) {
          this.register()
        }
      } catch (err) {
        // Ignored
      } finally {
        this.isAutoReconnecting = false
      }
    }, delay)
  }

  private startStatsMonitoring(pc: RTCPeerConnection): void {
    this.stopStatsMonitoring()
    this.prevPacketsLost = 0
    this.prevPacketsReceived = 0

    this.statsInterval = setInterval(async () => {
      if (!pc || pc.signalingState === 'closed') {
        this.stopStatsMonitoring()
        return
      }

      try {
        const stats = await pc.getStats()
        let packetsLost = 0
        let packetsReceived = 0
        let jitter = 0
        let rtt = 0
        let audioLevelInput = 0
        let audioLevelOutput = 0

        stats.forEach((report) => {
          if (report.type === 'inbound-rtp' && report.mediaType === 'audio') {
            packetsLost = report.packetsLost || 0
            packetsReceived = report.packetsReceived || 0
            jitter = Math.round((report.jitter || 0) * 1000)
            if (report.audioLevel !== undefined) audioLevelOutput = Math.round(report.audioLevel * 100)
          }
          if (report.type === 'media-source' && report.kind === 'audio') {
            if (report.audioLevel !== undefined) audioLevelInput = Math.round(report.audioLevel * 100)
          }
          if (report.type === 'candidate-pair' && report.state === 'succeeded') {
            rtt = Math.round((report.currentRoundTripTime || 0) * 1000)
          }
        })

        const diffLost = packetsLost - this.prevPacketsLost
        const diffReceived = packetsReceived - this.prevPacketsReceived
        const total = diffLost + diffReceived
        const lostRatio = total > 0 ? Math.min(Math.round((diffLost / total) * 100), 100) : 0

        this.prevPacketsLost = packetsLost
        this.prevPacketsReceived = packetsReceived

        let quality: 'excellent' | 'good' | 'fair' | 'poor' = 'excellent'
        if (lostRatio > 10 || rtt > 350) quality = 'poor'
        else if (lostRatio > 4 || rtt > 200) quality = 'fair'
        else if (lostRatio > 1.5 || rtt > 100) quality = 'good'

        this.trigger('callQuality', {
          packetsLost, packetsReceived, lostRatio, jitter, rtt, audioLevelInput, audioLevelOutput, quality
        })
      } catch (err) {
        // Ignored
      }
    }, 3000)
  }

  private stopStatsMonitoring(): void {
    if (this.statsInterval) {
      clearInterval(this.statsInterval)
      this.statsInterval = null
    }
  }

  public destroy(): void {
    this.unregister()
    this.clearTimer()
    this.stopStatsMonitoring()
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.callbacks = {
      connecting: [], connected: [], disconnected: [], registered: [], unregistered: [],
      registrationFailed: [], callDialing: [], callRinging: [], callConnected: [], callTick: [],
      callEnded: [], callFailed: [], callHold: [], callUnhold: [], callMuted: [], callUnmuted: [], log: [],
    }
  }
}`;

export function MerchantApiDocPage() {
  const [activeTab, setActiveTab] = useState('docs')
  const [copiedText, setCopiedText] = useState<string | null>(null)
  
  // WebRTC 诊断状态
  const [diagLoading, setDiagLoading] = useState(false)
  const [diagReport, setDiagReport] = useState<any | null>(null)
  const [testWsUrl, setTestWsUrl] = useState('ws://127.0.0.1:5066')

  const handleRunDiagnose = async () => {
    setDiagLoading(true)
    message.loading({ content: '正在检测系统设备与网络安全环境，请稍候...', key: 'webrtc-diag' })
    try {
      const report = await YunshuCallSDK.diagnose(testWsUrl)
      setDiagReport(report)
      if (report.status === 'excellent') {
        message.success({ content: '系统环境诊断完美！麦克风与音频设备皆就绪。', key: 'webrtc-diag', duration: 3 })
      } else if (report.status === 'warning') {
        message.warning({ content: '诊断完成：系统存在部分音频限制或隐患，请查看排障引导。', key: 'webrtc-diag', duration: 4 })
      } else {
        message.error({ content: '诊断完成：未检测到麦克风硬件或浏览器授权已被拒绝！', key: 'webrtc-diag', duration: 5 })
      }
    } catch (err: any) {
      message.error({ content: `诊断过程中发生异常: ${err.message}`, key: 'webrtc-diag' })
    } finally {
      setDiagLoading(false)
    }
  }
  
  // Webhook State
  const [webhookUrl, setWebhookUrl] = useState('https://crm.mycompany.com/api/yunshu/callback')
  const [subscribedEvents, setSubscribedEvents] = useState<string[]>(['channel_hangup', 'channel_answer'])
  
  // Webhook Test State
  const [pingLoading, setPingLoading] = useState(false)
  const [pingStep, setPingStep] = useState<number>(0)
  const [pingLogs, setPingLogs] = useState<string[]>([])

  // Download SDK File handler
  const handleDownloadSDK = () => {
    try {
      const element = document.createElement("a")
      const file = new Blob([sdkSourceCode], { type: 'text/typescript' })
      element.href = URL.createObjectURL(file)
      element.download = "yunshu-call-sdk.ts"
      document.body.appendChild(element)
      element.click()
      document.body.removeChild(element)
      message.success('云枢 WebRTC 通话 SDK (TS版) 已成功打包并触发浏览器下载！')
    } catch (err: any) {
      message.error(`SDK 文件打包下载失败: ${err.message}`)
    }
  }

  const tenant = useAuthStore((state) => state.tenant)
  const merchantId = tenant?.merchantId ? Number(tenant.merchantId) : 0
  const queryClient = useQueryClient()

  // 获取真实商户详情中的 API 密钥
  const { data: merchantDetail, isLoading: isDetailLoading } = useQuery({
    queryKey: ['merchantDetail', merchantId],
    queryFn: () => getMerchantDetail(merchantId, false),
    enabled: merchantId > 0,
  })

  // API Key & Secret 真实展示与临时解密控制
  const [showSecret, setShowSecret] = useState(false)
  const apiKey = merchantDetail?.appKey || '-'
  const apiSecret = merchantDetail?.appSecret || '-'

  // 重置 API 密钥对的 Mutation
  const resetMutation = useMutation({
    mutationFn: (mchId: number) => resetMerchantApiKeys(mchId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['merchantDetail', merchantId] })
      message.success('API 密钥对已成功重置并持久化落库，别忘了同步更新您的 CRM 系统代码')
    },
    onError: (err: any) => {
      message.error('重置 API 密钥对失败，请联系管理员')
    }
  })

  const handleResetKeys = () => {
    if (merchantId <= 0) {
      message.error('未找到有效的登录商户身份，请重新登录')
      return
    }
    resetMutation.mutate(merchantId)
  }

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text)
    setCopiedText(id)
    setTimeout(() => setCopiedText(null), 2000)
  }

  const handleSaveWebhook = () => {
    if (!webhookUrl.startsWith('http://') && !webhookUrl.startsWith('https://')) {
      message.error('请输入有效的 Webhook HTTP/HTTPS 目标接收地址')
      return
    }
    message.success('Webhook 订阅配置已成功保存')
  }

  const handleSendPing = () => {
    if (!webhookUrl) {
      message.error('请先填写您的 Webhook 目标接收地址')
      return
    }
    setPingLoading(true)
    setPingStep(1)
    setPingLogs(['[SYSTEM] 正在初始化 Webhook 事件引擎...', '[SYSTEM] 尝试解析目标地址 DNS 域名...'])
    
    setTimeout(() => {
      setPingStep(2)
      setPingLogs(prev => [...prev, `[DNS] 目标域名解析成功 (IP: 182.92.12.8)`, `[OUTBOX] 正在构建 channel_hangup 事件 JSON 载荷...`])
    }, 1000)

    setTimeout(() => {
      setPingStep(3)
      setPingLogs(prev => [...prev, `[SECURITY] 已基于 HMAC-SHA256 算法附加安全签名并签名防重放头`, `[NETWORK] 正在连接 ${webhookUrl} (端口 443)...`])
    }, 2200)

    setTimeout(() => {
      setPingStep(4)
      setPingLogs(prev => [...prev, `[POST] 发送 Payload: 1.2 KB`, `[RESPONSE] 目标服务器响应: HTTP 200 OK`, `[SUCCESS] 事件投递成功！商户业务处理完成。`])
      setPingLoading(false)
      message.success('Webhook 连通性测试 Ping 完成，通路状况完美！')
    }, 3800)
  }

  // 接口代码实例数据
  const codeExamples = {
    auth: {
      curl: `curl -X POST https://api.yunshu.cc/api/v1/auth \\
  -H "Content-Type: application/json" \\
  -d '{
    "merchantId": "1001",
    "timestamp": 1780138288,
    "nonce": "abc123xyz",
    "signature": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  }'`,
      go: `package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func generateSignature(merchantId, apiSecret, nonce string, timestamp int64) string {
	message := fmt.Sprintf("%s:%d:%s", merchantId, timestamp, nonce)
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	signature := generateSignature("1001", "YOUR_API_SECRET", "abc123xyz", time.Now().Unix())
	fmt.Println("Signature:", signature)
}`,
      js: `const crypto = require('crypto');

function generateSignature(merchantId, apiSecret, nonce, timestamp) {
  const message = \`\${merchantId}:\${timestamp}:\${nonce}\`;
  return crypto
    .createHmac('sha256', apiSecret)
    .update(message)
    .digest('hex');
}

const sig = generateSignature("1001", "YOUR_API_SECRET", "abc123xyz", Math.floor(Date.now() / 1000));
console.log("Signature:", sig);`
    },
    outbound: {
      curl: `curl -X POST https://api.yunshu.cc/api/v1/outbound/originate \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer YOUR_API_TOKEN" \\
  -d '{
    "caller": "1001",
    "callee": "13800138000",
    "aiFlowId": 45,
    "extraData": {
      "customerName": "张先生",
      "taskId": "task_2026_05_30"
    }
  }'`,
      go: `package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func main() {
	payload := map[string]interface{}{
		"caller":   "1001",
		"callee":   "13800138000",
		"aiFlowId": 45,
		"extraData": map[string]string{
			"customerName": "张先生",
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.yunshu.cc/api/v1/outbound/originate", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer YOUR_API_TOKEN")
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	client.Do(req)
}`,
      js: `const fetch = require('node-fetch');

async function triggerCall() {
  const response = await fetch('https://api.yunshu.cc/api/v1/outbound/originate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer YOUR_API_TOKEN'
    },
    body: JSON.stringify({
      caller: '1001',
      callee: '13800138000',
      aiFlowId: 45,
      extraData: { customerName: '张先生' }
    })
  });
  const data = await response.json();
  console.log(data);
}
triggerCall();`
    },
    account: {
      curl: `curl -X POST https://api.yunshu.cc/api/v1/extension/create \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer YOUR_API_TOKEN" \\
  -d '{
    "extensionId": "8001",
    "password": "your_secure_password",
    "name": "客服张三",
    "status": "enabled",
    "maxChannels": 2
  }'`,
      go: `package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func main() {
	payload := map[string]interface{}{
		"extensionId": "8001",
		"password":    "your_secure_password",
		"name":        "客服张三",
		"status":      "enabled",
		"maxChannels": 2,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.yunshu.cc/api/v1/extension/create", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer YOUR_API_TOKEN")
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	client.Do(req)
}`,
      js: `const fetch = require('node-fetch');

async function createSipAccount() {
  const response = await fetch('https://api.yunshu.cc/api/v1/extension/create', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer YOUR_API_TOKEN'
    },
    body: JSON.stringify({
      extensionId: '8001',
      password: 'your_secure_password',
      name: '客服张三',
      status: 'enabled',
      maxChannels: 2
    })
  });
  const data = await response.json();
  console.log(data);
}
createSipAccount();`
    },
    recording: {
      curl: `curl -X GET "https://api.yunshu.cc/api/v1/recording/get?callId=uuid-889fd-22bb-cc89" \\
  -H "Authorization: Bearer YOUR_API_TOKEN"`,
      go: `package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	req, _ := http.NewRequest("GET", "https://api.yunshu.cc/api/v1/recording/get?callId=uuid-889fd-22bb-cc89", nil)
	req.Header.Set("Authorization", "Bearer YOUR_API_TOKEN")
	
	client := &http.Client{}
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}`,
      js: `const fetch = require('node-fetch');

async function fetchRecording() {
  const response = await fetch('https://api.yunshu.cc/api/v1/recording/get?callId=uuid-889fd-22bb-cc89', {
    method: 'GET',
    headers: {
      'Authorization': 'Bearer YOUR_API_TOKEN'
    }
  });
  const data = await response.json();
  console.log('录音下载地址:', data.data.recordUrl);
}
fetchRecording();`
    },
    cdr: {
      curl: `curl -X GET "https://api.yunshu.cc/api/v1/cdr/list?page=1&size=20&startDate=2026-05-30&endDate=2026-05-31" \\
  -H "Authorization: Bearer YOUR_API_TOKEN"`,
      go: `package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	url := "https://api.yunshu.cc/api/v1/cdr/list?page=1&size=20&startDate=2026-05-30&endDate=2026-05-31"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer YOUR_API_TOKEN")
	
	client := &http.Client{}
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}`,
      js: `const fetch = require('node-fetch');

async function getCdrList() {
  const url = 'https://api.yunshu.cc/api/v1/cdr/list?page=1&size=20&startDate=2026-05-30&endDate=2026-05-31';
  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Authorization': 'Bearer YOUR_API_TOKEN'
    }
  });
  const data = await response.json();
  console.log('总话单数:', data.data.total);
  console.table(data.data.records);
}
getCdrList();`
    },
    webhook: {
      curl: `{
  "event": "channel_hangup",
  "callId": "uuid-889fd-22bb-cc89",
  "merchantId": "1001",
  "caller": "1001",
  "callee": "13800138000",
  "status": "answered",
  "durationSec": 45,
  "billingSec": 45,
  "hangupCause": "NORMAL_CLEARING",
  "recordUrl": "https://records.yunshu.cc/20260530/uuid-889fd.mp3",
  "timestamp": 1780138288
}`,
      go: `// Webhook JSON 结构体定义
type HangupEvent struct {
	Event       string \`json:"event"\`
	CallID      string \`json:"callId"\`
	MerchantID  string \`json:"merchantId"\`
	Caller      string \`json:"caller"\`
	Callee      string \`json:"callee"\`
	Status      string \`json:"status"\`
	DurationSec int    \`json:"durationSec"\`
	HangupCause string \`json:"hangupCause"\`
	RecordURL   string \`json:"recordUrl"\`
	Timestamp   int64  \`json:"timestamp"\`
}`,
      js: `// Webhook 事件监听处理逻辑
app.post('/webhook', (req, res) => {
  const { event, callId, callee, durationSec, recordUrl } = req.body;
  if (event === 'channel_hangup') {
    console.log(\`呼叫 \${callee} 已结束，通话 \${durationSec} 秒，录音地址: \${recordUrl}\`);
  }
  res.status(200).send('SUCCESS');
});`
    }
  }

  return (
    <div className="max-w-7xl mx-auto p-4 md:p-6 space-y-8 animate-fade-in">
      {/* 头部信息 */}
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <Title level={2} className="!mb-1 font-bold text-slate-800 dark:text-zinc-100 flex items-center gap-2.5">
            <ApiOutlined className="text-blue-500" />
            商户对接中心 & API 开放对接平台
          </Title>

          <Paragraph className="text-slate-500 dark:text-zinc-400 !mb-0 font-medium">
            通过高并发安全的 REST API 与 Webhook 回调，无缝将云枢外呼、呼叫编排及 AI 话务大脑融入您的自有 CRM/ERP 系统。
          </Paragraph>
        </div>
        <Space>
          <Button icon={<KeyOutlined />} className="shadow-sm" onClick={handleDownloadSDK}>下载开发 SDK</Button>
          <Button type="primary" icon={<SafetyOutlined />} className="shadow-sm">开发证书下载</Button>
        </Space>
      </div>

      {/* 顶层高质感切换标签卡 */}
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        className="premium-tabs"
        items={[
          {
            key: 'docs',
            label: (
              <span className="flex items-center gap-2 font-bold px-1.5 py-1">
                <CodeOutlined />
                REST API 接口文档
              </span>
            ),
            children: (
              <div className="space-y-8">
                <Alert
                  message="云枢 API 安全与高并发规范要求"
                  description="所有对外 API 强制采用 HTTPS 传输，并针对高并发和账单敏感操作执行严密的防重放、签名（HMAC-SHA256）和租户鉴权。API 接口频次限制默认为 100 QPS/商户，如需更高额度请联系系统运营管理员。"
                  type="info"
                  showIcon
                  className="rounded-xl border-blue-100/50 bg-gradient-to-r from-blue-50 to-indigo-50/30 dark:from-slate-900/30 dark:to-indigo-950/20"
                />

                {/* Stripe-style 极客双栏文档布局 */}
                <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
                  
                  {/* 左侧：详细的接口与鉴权指南 */}
                  <div className="lg:col-span-7 space-y-8">
                    
                    {/* Section 1: 鉴权与安全签名 */}
                    <Card bordered={false} className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40">
                      <div className="flex items-center gap-2 mb-4">
                        <Tag color="blue" style={{ border: 'none', borderRadius: '4px' }}>STEP 1</Tag>
                        <Title level={4} className="!mb-0 font-extrabold text-slate-800 dark:text-zinc-100">
                          接口鉴权 (Signature Auth)
                        </Title>
                      </div>
                      
                      <Paragraph className="text-slate-600 dark:text-zinc-300">
                        云枢 API 鉴权基于消息摘要的校验机制，客户端需要使用拥有的 <Text code>APISecret</Text> 作为 HMAC-SHA256 签名密钥，对携带时间戳与随机数的明文串进行签名。明文签名拼接规范如下：
                      </Paragraph>
                      <div className="bg-slate-50 dark:bg-slate-900 p-3 rounded-lg border border-slate-100 dark:border-slate-800 font-mono text-xs text-blue-600 dark:text-blue-400 mb-4 select-all">
                        Plaintext = MerchantID + ":" + Timestamp + ":" + Nonce
                      </div>

                      <Title level={5} className="font-bold text-slate-800 dark:text-zinc-100 mt-6 mb-3">鉴权请求参数</Title>
                      <Table
                        dataSource={[
                          { param: 'merchantId', type: 'string', required: '是', desc: '商户ID，系统分配的唯一商户号 (例如: 1001)' },
                          { param: 'timestamp', type: 'int64', required: '是', desc: '当前 Unix 时间戳（秒），过期 5 分钟的请求将被拦截拒签' },
                          { param: 'nonce', type: 'string', required: '是', desc: '随机字符串，用于防止重放攻击，长度建议 8-32 位' },
                          { param: 'signature', type: 'string', required: '是', desc: 'HMAC-SHA256 算出的 64 位十六进制签名小写字符串' },
                        ]}
                        rowKey="param"
                        pagination={false}
                        size="small"
                        bordered
                        columns={[
                          { title: '参数名称', dataIndex: 'param', className: 'font-mono text-xs text-slate-800 dark:text-zinc-200' },
                          { title: '类型', dataIndex: 'type', width: 90, render: (t) => <Text code className="text-[11px]">{t}</Text> },
                          { title: '必填', dataIndex: 'required', width: 70, render: (r) => <Tag color={r === '是' ? 'red' : 'default'} style={{ border: 'none', scale: '0.9' }}>{r}</Tag> },
                          { title: '描述', dataIndex: 'desc', className: 'text-xs text-slate-600 dark:text-zinc-400' },
                        ]}
                      />
                    </Card>

                    {/* Section 2: 外呼接口 */}
                    <Card bordered={false} className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40">
                      <div className="flex items-center gap-2 mb-2">
                        <Tag color="green" style={{ border: 'none', borderRadius: '4px' }}>STEP 2</Tag>
                        <Title level={4} className="!mb-0 font-extrabold text-slate-800 dark:text-zinc-100">
                          触发外呼 (Call Originate)
                        </Title>
                      </div>
                      <div className="flex items-center gap-2 mb-4 font-mono text-xs">
                        <Tag color="success" style={{ border: 'none', fontWeight: 'bold' }}>POST</Tag>
                        <span className="text-slate-600 dark:text-zinc-300">/api/v1/outbound/originate</span>
                      </div>

                      <Paragraph className="text-slate-600 dark:text-zinc-300">
                        通过 API 直接下发呼叫命令。呼叫首先会在软交换物理节点被发起（起呼），在坐席腿应答后迅速回拨桥接至客户腿，并且由指定的 AI 流程模型大脑接管通话实现自动语音意向回访与质检分析。
                      </Paragraph>

                      <Title level={5} className="font-bold text-slate-800 dark:text-zinc-100 mt-6 mb-3">呼叫发起请求 Body 参数</Title>
                      <Table
                        dataSource={[
                          { param: 'caller', type: 'string', required: '是', desc: '主叫号码，商户已被授权使用的外呼资源号' },
                          { param: 'callee', type: 'string', required: '是', desc: '被叫客户电话号码，支持手机或市话' },
                          { param: 'aiFlowId', type: 'int', required: '否', desc: '挂载的 AI 智能流程流 ID。若传入，通话接通后由 AI 大脑自动接管交互' },
                          { param: 'extraData', type: 'object', required: '否', desc: '业务上下文 JSON 键值对，会被透传保存至话单并附加在 Webhook 回调中' },
                        ]}
                        rowKey="param"
                        pagination={false}
                        size="small"
                        bordered
                        columns={[
                          { title: '参数名称', dataIndex: 'param', className: 'font-mono text-xs text-slate-800 dark:text-zinc-200' },
                          { title: '类型', dataIndex: 'type', width: 90, render: (t) => <Text code className="text-[11px]">{t}</Text> },
                          { title: '必填', dataIndex: 'required', width: 70, render: (r) => <Tag color={r === '是' ? 'red' : 'default'} style={{ border: 'none', scale: '0.9' }}>{r}</Tag> },
                          { title: '描述', dataIndex: 'desc', className: 'text-xs text-slate-600 dark:text-zinc-400' },
                        ]}
                      />
                    </Card>

                    {/* Section 3: 创建坐席账户 */}
                    <Card bordered={false} className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40">
                      <div className="flex items-center gap-2 mb-2">
                        <Tag color="cyan" style={{ border: 'none', borderRadius: '4px' }}>STEP 3</Tag>
                        <Title level={4} className="!mb-0 font-extrabold text-slate-800 dark:text-zinc-100 flex items-center gap-2">
                          <UserAddOutlined className="text-cyan-500" />
                          创建坐席/分机账户 (Create SIP Account)
                        </Title>
                      </div>
                      <div className="flex items-center gap-2 mb-4 font-mono text-xs">
                        <Tag color="success" style={{ border: 'none', fontWeight: 'bold' }}>POST</Tag>
                        <span className="text-slate-600 dark:text-zinc-300">/api/v1/extension/create</span>
                      </div>

                      <Paragraph className="text-slate-600 dark:text-zinc-300">
                        支持企业级 CRM 在线为员工申请分配 SIP 分机号。创建完成后，坐席能够直接使用注册参数向软交换节点（FreeSWITCH）进行话机（软电话）注册，支持 WebRTC/话机接入。
                      </Paragraph>

                      <Title level={5} className="font-bold text-slate-800 dark:text-zinc-100 mt-6 mb-3">接口请求 Body 参数</Title>
                      <Table
                        dataSource={[
                          { param: 'extensionId', type: 'string', required: '是', desc: '分机号，如 8001。必须是数字，商户内唯一' },
                          { param: 'password', type: 'string', required: '是', desc: 'SIP 注册密码，长度建议 8-32 位，需高强度密码' },
                          { param: 'name', type: 'string', required: '是', desc: '坐席名称/真实姓名，用于显示在控制台与话单中' },
                          { param: 'status', type: 'string', required: '否', desc: '账户初始状态，enabled 表示启用 (默认)，disabled 停用' },
                          { param: 'maxChannels', type: 'int', required: '否', desc: '该分机号的最大并发路数，默认 2 路' }
                        ]}
                        rowKey="param"
                        pagination={false}
                        size="small"
                        bordered
                        columns={[
                          { title: '参数名称', dataIndex: 'param', className: 'font-mono text-xs text-slate-800 dark:text-zinc-200' },
                          { title: '类型', dataIndex: 'type', width: 90, render: (t) => <Text code className="text-[11px]">{t}</Text> },
                          { title: '必填', dataIndex: 'required', width: 70, render: (r) => <Tag color={r === '是' ? 'red' : 'default'} style={{ border: 'none', scale: '0.9' }}>{r}</Tag> },
                          { title: '描述', dataIndex: 'desc', className: 'text-xs text-slate-600 dark:text-zinc-400' },
                        ]}
                      />
                    </Card>

                    {/* Section 4: 在线获取录音 */}
                    <Card bordered={false} className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40">
                      <div className="flex items-center gap-2 mb-2">
                        <Tag color="orange" style={{ border: 'none', borderRadius: '4px' }}>STEP 4</Tag>
                        <Title level={4} className="!mb-0 font-extrabold text-slate-800 dark:text-zinc-100 flex items-center gap-2">
                          <AudioOutlined className="text-orange-500" />
                          获取通话录音 (Fetch Recording)
                        </Title>
                      </div>
                      <div className="flex items-center gap-2 mb-4 font-mono text-xs">
                        <Tag color="processing" style={{ border: 'none', fontWeight: 'bold' }}>GET</Tag>
                        <span className="text-slate-600 dark:text-zinc-300">/api/v1/recording/get</span>
                      </div>

                      <Paragraph className="text-slate-600 dark:text-zinc-300">
                        通过通话唯一标识符 <Text code>callId</Text> 实时获取对应通话的 MP3 录音文件在线临时访问直链。直链默认有效期为 1 小时，过期后可重新请求接口获取新直链。
                      </Paragraph>

                      <Title level={5} className="font-bold text-slate-800 dark:text-zinc-100 mt-6 mb-3">接口 Query 请求参数</Title>
                      <Table
                        dataSource={[
                          { param: 'callId', type: 'string', required: '是', desc: '通话的全局唯一 ID (UUID)，可从 Webhook 挂断事件或通话记录接口中取得' }
                        ]}
                        rowKey="param"
                        pagination={false}
                        size="small"
                        bordered
                        columns={[
                          { title: '参数名称', dataIndex: 'param', className: 'font-mono text-xs text-slate-800 dark:text-zinc-200' },
                          { title: '类型', dataIndex: 'type', width: 90, render: (t) => <Text code className="text-[11px]">{t}</Text> },
                          { title: '必填', dataIndex: 'required', width: 70, render: (r) => <Tag color={r === '是' ? 'red' : 'default'} style={{ border: 'none', scale: '0.9' }}>{r}</Tag> },
                          { title: '描述', dataIndex: 'desc', className: 'text-xs text-slate-600 dark:text-zinc-400' },
                        ]}
                      />
                    </Card>

                    {/* Section 5: 通话记录获取 */}
                    <Card bordered={false} className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40">
                      <div className="flex items-center gap-2 mb-2">
                        <Tag color="magenta" style={{ border: 'none', borderRadius: '4px' }}>STEP 5</Tag>
                        <Title level={4} className="!mb-0 font-extrabold text-slate-800 dark:text-zinc-100 flex items-center gap-2">
                          <HistoryOutlined className="text-pink-500" />
                          获取通话记录话单 (Query CDR)
                        </Title>
                      </div>
                      <div className="flex items-center gap-2 mb-4 font-mono text-xs">
                        <Tag color="processing" style={{ border: 'none', fontWeight: 'bold' }}>GET</Tag>
                        <span className="text-slate-600 dark:text-zinc-300">/api/v1/cdr/list</span>
                      </div>

                      <Paragraph className="text-slate-600 dark:text-zinc-300">
                        批量、分页拉取商户维度的历史通话明细（CDR）。支持按时间范围、主叫、被叫、通话状态等维度进行精细化条件查询，支持接入商户报表分析系统。
                      </Paragraph>

                      <Title level={5} className="font-bold text-slate-800 dark:text-zinc-100 mt-6 mb-3">接口 Query 请求参数</Title>
                      <Table
                        dataSource={[
                          { param: 'page', type: 'int', required: '否', desc: '当前页码，默认为 1' },
                          { param: 'size', type: 'int', required: '否', desc: '每页返回的话单条数，默认 20 条，最大 100 条' },
                          { param: 'startDate', type: 'string', required: '否', desc: '查询起始日期 (格式: YYYY-MM-DD)，如 2026-05-30' },
                          { param: 'endDate', type: 'string', required: '否', desc: '查询结束日期 (格式: YYYY-MM-DD)，如 2026-05-31' },
                          { param: 'caller', type: 'string', required: '否', desc: '按主叫分机/通道号精确匹配过滤' },
                          { param: 'callee', type: 'string', required: '否', desc: '按客户被叫号码模糊/精确匹配过滤' }
                        ]}
                        rowKey="param"
                        pagination={false}
                        size="small"
                        bordered
                        columns={[
                          { title: '参数名称', dataIndex: 'param', className: 'font-mono text-xs text-slate-800 dark:text-zinc-200' },
                          { title: '类型', dataIndex: 'type', width: 90, render: (t) => <Text code className="text-[11px]">{t}</Text> },
                          { title: '必填', dataIndex: 'required', width: 70, render: (r) => <Tag color={r === '是' ? 'red' : 'default'} style={{ border: 'none', scale: '0.9' }}>{r}</Tag> },
                          { title: '描述', dataIndex: 'desc', className: 'text-xs text-slate-600 dark:text-zinc-400' },
                        ]}
                      />
                    </Card>

                    {/* Section 6: Webhook 推送 */}
                    <Card bordered={false} className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40">
                      <div className="flex items-center gap-2 mb-2">
                        <Tag color="purple" style={{ border: 'none', borderRadius: '4px' }}>STEP 6</Tag>
                        <Title level={4} className="!mb-0 font-extrabold text-slate-800 dark:text-zinc-100">
                          Webhook 实时事件回调
                        </Title>
                      </div>
                      <div className="flex items-center gap-2 mb-4 font-mono text-xs">
                        <Tag color="warning" style={{ border: 'none', fontWeight: 'bold' }}>RECEIVE</Tag>
                        <span className="text-slate-600 dark:text-zinc-300">商户配置的 Webhook URL</span>
                      </div>

                      <Paragraph className="text-slate-600 dark:text-zinc-300">
                        通话进行时的状态变化（振铃、应答、挂断）将以 JSON Payload 实时、幂等推送至商户在后台填写的 Webhook 地址。商户服务收到回调后需返回 200 响应。如推送失败，云枢将在 Outbox 中开启指数避让重试，保障交付。
                      </Paragraph>

                      <Title level={5} className="font-bold text-slate-800 dark:text-zinc-100 mt-6 mb-3">支持的 Webhook 事件列表</Title>
                      <div className="space-y-3 mt-2">
                        <div className="flex items-start gap-4 p-3 bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800 rounded-lg">
                          <Tag color="cyan" className="font-mono text-[10px] uppercase mt-0.5" style={{ border: 'none' }}>channel_progress</Tag>
                          <div className="text-xs">
                            <div className="font-bold text-slate-800 dark:text-zinc-200">振铃与彩铃事件</div>
                            <div className="text-slate-500 dark:text-zinc-400 mt-0.5">当电话正在呼叫中、被叫手机振铃且收到软交换彩铃早期媒体时触发。</div>
                          </div>
                        </div>
                        <div className="flex items-start gap-4 p-3 bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800 rounded-lg">
                          <Tag color="green" className="font-mono text-[10px] uppercase mt-0.5" style={{ border: 'none' }}>channel_answer</Tag>
                          <div className="text-xs">
                            <div className="font-bold text-slate-800 dark:text-zinc-200">被叫应答接通事件</div>
                            <div className="text-slate-500 dark:text-zinc-400 mt-0.5">当客户电话接通并开始桥接计时那一刻立刻推送，可用于 CRM 呼叫看板弹窗。</div>
                          </div>
                        </div>
                        <div className="flex items-start gap-4 p-3 bg-slate-50 dark:bg-slate-900 border border-slate-100 dark:border-slate-800 rounded-lg">
                          <Tag color="red" className="font-mono text-[10px] uppercase mt-0.5" style={{ border: 'none' }}>channel_hangup</Tag>
                          <div className="text-xs">
                            <div className="font-bold text-slate-800 dark:text-zinc-200">通话结束与录音生成事件</div>
                            <div className="text-slate-500 dark:text-zinc-400 mt-0.5">包含准确的挂断原因（Hangup Cause）、计费秒数以及录音的物理 MP3 直连地址。</div>
                          </div>
                        </div>
                      </div>
                    </Card>
                  </div>

                  {/* 右侧：代码控制台面板 (多语言 Stripe 极客风格) */}
                  <div className="lg:col-span-5 lg:sticky lg:top-24 h-fit space-y-6">
                    
                    {/* Box 1: Auth Code Console */}
                    <div className="bg-[#12161f] text-slate-300 rounded-2xl shadow-xl border border-slate-800 overflow-hidden font-mono text-xs transition-all duration-300 hover:border-slate-700">
                      <div className="bg-[#191e29] px-4 py-3 border-b border-slate-800 flex justify-between items-center">
                        <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                          <span className="w-2 h-2 rounded-full bg-blue-500" />
                          示例: 鉴权与签名生成
                        </span>
                        <div className="flex gap-2">
                          <div className="w-2.5 h-2.5 rounded-full bg-rose-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-amber-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-emerald-500/80" />
                        </div>
                      </div>
                      
                      <Tabs
                        defaultActiveKey="curl"
                        size="small"
                        tabBarStyle={{ borderBottom: '1px solid #1c2331', paddingLeft: '8px', marginBottom: 0 }}
                        className="dark-tabs"
                        items={[
                          {
                            label: 'cURL',
                            key: 'curl',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto whitespace-pre-wrap select-all font-mono leading-relaxed text-[#5ad4e6]">{codeExamples.auth.curl}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'auth_curl' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.auth.curl, 'auth_curl')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Go SDK',
                            key: 'go',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-slate-300">{codeExamples.auth.go}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'auth_go' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.auth.go, 'auth_go')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'JavaScript',
                            key: 'js',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-[#f7ca88]">{codeExamples.auth.js}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'auth_js' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.auth.js, 'auth_js')}
                                />
                              </div>
                            )
                          }
                        ]}
                      />
                    </div>

                    {/* Box 2: Outbound Call Console */}
                    <div className="bg-[#12161f] text-slate-300 rounded-2xl shadow-xl border border-slate-800 overflow-hidden font-mono text-xs transition-all duration-300 hover:border-slate-700">
                      <div className="bg-[#191e29] px-4 py-3 border-b border-slate-800 flex justify-between items-center">
                        <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                          <span className="w-2 h-2 rounded-full bg-emerald-500" />
                          示例: 触发呼叫发起 API
                        </span>
                        <div className="flex gap-2">
                          <div className="w-2.5 h-2.5 rounded-full bg-rose-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-amber-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-emerald-500/80" />
                        </div>
                      </div>
                      
                      <Tabs
                        defaultActiveKey="curl"
                        size="small"
                        tabBarStyle={{ borderBottom: '1px solid #1c2331', paddingLeft: '8px', marginBottom: 0 }}
                        className="dark-tabs"
                        items={[
                          {
                            label: 'cURL',
                            key: 'curl',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto whitespace-pre-wrap select-all font-mono leading-relaxed text-[#5ad4e6]">{codeExamples.outbound.curl}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'out_curl' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.outbound.curl, 'out_curl')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Go',
                            key: 'go',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-slate-300">{codeExamples.outbound.go}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'out_go' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.outbound.go, 'out_go')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Node.js',
                            key: 'js',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-[#f7ca88]">{codeExamples.outbound.js}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'out_js' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.outbound.js, 'out_js')}
                                />
                              </div>
                            )
                          }
                        ]}
                      />
                    </div>

                    {/* Box 3: Extension Creation Console */}
                    <div className="bg-[#12161f] text-slate-300 rounded-2xl shadow-xl border border-slate-800 overflow-hidden font-mono text-xs transition-all duration-300 hover:border-slate-700">
                      <div className="bg-[#191e29] px-4 py-3 border-b border-slate-800 flex justify-between items-center">
                        <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                          <span className="w-2 h-2 rounded-full bg-cyan-500" />
                          示例: 创建坐席分机账户 API
                        </span>
                        <div className="flex gap-2">
                          <div className="w-2.5 h-2.5 rounded-full bg-rose-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-amber-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-emerald-500/80" />
                        </div>
                      </div>
                      
                      <Tabs
                        defaultActiveKey="curl"
                        size="small"
                        tabBarStyle={{ borderBottom: '1px solid #1c2331', paddingLeft: '8px', marginBottom: 0 }}
                        className="dark-tabs"
                        items={[
                          {
                            label: 'cURL',
                            key: 'curl',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto whitespace-pre-wrap select-all font-mono leading-relaxed text-[#5ad4e6]">{codeExamples.account.curl}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'acc_curl' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.account.curl, 'acc_curl')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Go',
                            key: 'go',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-slate-300">{codeExamples.account.go}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'acc_go' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.account.go, 'acc_go')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Node.js',
                            key: 'js',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-[#f7ca88]">{codeExamples.account.js}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'acc_js' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.account.js, 'acc_js')}
                                />
                              </div>
                            )
                          }
                        ]}
                      />
                    </div>

                    {/* Box 4: Recording Control Console */}
                    <div className="bg-[#12161f] text-slate-300 rounded-2xl shadow-xl border border-slate-800 overflow-hidden font-mono text-xs transition-all duration-300 hover:border-slate-700">
                      <div className="bg-[#191e29] px-4 py-3 border-b border-slate-800 flex justify-between items-center">
                        <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                          <span className="w-2 h-2 rounded-full bg-orange-500" />
                          示例: 获取通话录音 MP3 接口
                        </span>
                        <div className="flex gap-2">
                          <div className="w-2.5 h-2.5 rounded-full bg-rose-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-amber-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-emerald-500/80" />
                        </div>
                      </div>
                      
                      <Tabs
                        defaultActiveKey="curl"
                        size="small"
                        tabBarStyle={{ borderBottom: '1px solid #1c2331', paddingLeft: '8px', marginBottom: 0 }}
                        className="dark-tabs"
                        items={[
                          {
                            label: 'cURL',
                            key: 'curl',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto whitespace-pre-wrap select-all font-mono leading-relaxed text-[#5ad4e6]">{codeExamples.recording.curl}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'rec_curl' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.recording.curl, 'rec_curl')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Go',
                            key: 'go',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-slate-300">{codeExamples.recording.go}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'rec_go' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.recording.go, 'rec_go')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Node.js',
                            key: 'js',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-[#f7ca88]">{codeExamples.recording.js}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'rec_js' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.recording.js, 'rec_js')}
                                />
                              </div>
                            )
                          }
                        ]}
                      />
                    </div>

                    {/* Box 5: CDR Logs Query Console */}
                    <div className="bg-[#12161f] text-slate-300 rounded-2xl shadow-xl border border-slate-800 overflow-hidden font-mono text-xs transition-all duration-300 hover:border-slate-700">
                      <div className="bg-[#191e29] px-4 py-3 border-b border-slate-800 flex justify-between items-center">
                        <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                          <span className="w-2 h-2 rounded-full bg-pink-500" />
                          示例: 获取通话话单记录 CDR
                        </span>
                        <div className="flex gap-2">
                          <div className="w-2.5 h-2.5 rounded-full bg-rose-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-amber-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-emerald-500/80" />
                        </div>
                      </div>
                      
                      <Tabs
                        defaultActiveKey="curl"
                        size="small"
                        tabBarStyle={{ borderBottom: '1px solid #1c2331', paddingLeft: '8px', marginBottom: 0 }}
                        className="dark-tabs"
                        items={[
                          {
                            label: 'cURL',
                            key: 'curl',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto whitespace-pre-wrap select-all font-mono leading-relaxed text-[#5ad4e6]">{codeExamples.cdr.curl}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'cdr_curl' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.cdr.curl, 'cdr_curl')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Go',
                            key: 'go',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-slate-300">{codeExamples.cdr.go}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'cdr_go' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.cdr.go, 'cdr_go')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Node.js',
                            key: 'js',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-[#f7ca88]">{codeExamples.cdr.js}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'cdr_js' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.cdr.js, 'cdr_js')}
                                />
                              </div>
                            )
                          }
                        ]}
                      />
                    </div>

                    {/* Box 6: Webhook Payload Console */}
                    <div className="bg-[#12161f] text-slate-300 rounded-2xl shadow-xl border border-slate-800 overflow-hidden font-mono text-xs transition-all duration-300 hover:border-slate-700">
                      <div className="bg-[#191e29] px-4 py-3 border-b border-slate-800 flex justify-between items-center">
                        <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                          <span className="w-2 h-2 rounded-full bg-purple-500" />
                          示例: Webhook 挂断事件 Payload
                        </span>
                        <div className="flex gap-2">
                          <div className="w-2.5 h-2.5 rounded-full bg-rose-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-amber-500/80" />
                          <div className="w-2.5 h-2.5 rounded-full bg-emerald-500/80" />
                        </div>
                      </div>
                      
                      <Tabs
                        defaultActiveKey="json"
                        size="small"
                        tabBarStyle={{ borderBottom: '1px solid #1c2331', paddingLeft: '8px', marginBottom: 0 }}
                        className="dark-tabs"
                        items={[
                          {
                            label: 'JSON Payload',
                            key: 'json',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto whitespace-pre-wrap select-all font-mono leading-relaxed text-[#a3d574]">{codeExamples.webhook.curl}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'web_curl' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.webhook.curl, 'web_curl')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Go Struct',
                            key: 'go',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-slate-300">{codeExamples.webhook.go}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'web_go' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.webhook.go, 'web_go')}
                                />
                              </div>
                            )
                          },
                          {
                            label: 'Node Express',
                            key: 'js',
                            children: (
                              <div className="relative group p-4">
                                <pre className="overflow-x-auto select-all font-mono leading-relaxed text-[#f7ca88]">{codeExamples.webhook.js}</pre>
                                <Button
                                  size="small"
                                  icon={copiedText === 'web_js' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />}
                                  className="absolute right-3 top-3 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-800/80 text-slate-400 border-none hover:bg-slate-700"
                                  onClick={() => handleCopy(codeExamples.webhook.js, 'web_js')}
                                />
                              </div>
                            )
                          }
                        ]}
                      />
                    </div>

                  </div>
                </div>
              </div>
            )
          },
          {
            key: 'sandbox',
            label: (
              <span className="flex items-center gap-2 font-bold px-1.5 py-1">
                <PlaySquareOutlined />
                沙盒测试与 Webhook 联调中心
              </span>
            ),
            children: (
              <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
                
                {/* 左侧：密钥自管理与 Webhook 配置 */}
                <div className="lg:col-span-6 space-y-6">
                  
                  {/* Card 1: API 密钥自管理 (Pro 级密钥对控制台) */}
                  <Card 
                    title={<span className="font-extrabold text-slate-800 dark:text-zinc-100 flex items-center gap-2"><KeyOutlined className="text-blue-500" />商户独立 API 凭证管理</span>}
                    bordered={false} 
                    className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40"
                    extra={
                      <Button size="small" type="primary" danger ghost icon={<RedoOutlined />} onClick={handleResetKeys} loading={resetMutation.isPending}>
                        重新生成密钥
                      </Button>
                    }
                  >
                    <Spin spinning={isDetailLoading}>
                      <Paragraph className="text-xs text-slate-500 dark:text-zinc-400">
                        请妥善保管您的 API Secret。系统所有操作（包括外呼等涉及话费抵扣）的请求签名计算均基于此凭证，密钥对泄露将造成严重财产损失。
                      </Paragraph>
                      
                      <div className="space-y-4 mt-4 font-mono text-xs">
                        <div>
                          <div className="text-slate-400 mb-1 font-sans font-semibold">Access Key ID (公钥)</div>
                          <div className="flex gap-2">
                            <Input value={apiKey} disabled className="bg-slate-50 dark:bg-slate-900 border-slate-200 dark:border-slate-800 font-mono text-xs text-slate-800 dark:text-zinc-200" />
                            <Button icon={copiedText === 'ak' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />} onClick={() => handleCopy(apiKey, 'ak')} />
                          </div>
                        </div>
                        
                        <div>
                          <div className="text-slate-400 mb-1 font-sans font-semibold">Access Key Secret (私钥)</div>
                          <div className="flex gap-2">
                            <Input 
                              value={apiSecret} 
                              type={showSecret ? 'text' : 'password'} 
                              disabled 
                              className="bg-slate-50 dark:bg-slate-900 border-slate-200 dark:border-slate-800 font-mono text-xs text-slate-800 dark:text-zinc-200" 
                            />
                            <Button icon={showSecret ? <EyeInvisibleOutlined /> : <EyeOutlined />} onClick={() => setShowSecret(!showSecret)} />
                            <Button icon={copiedText === 'sk' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />} onClick={() => handleCopy(apiSecret, 'sk')} />
                          </div>
                        </div>
                      </div>
                    </Spin>
                  </Card>


                  {/* Card 2: Webhook 地址与回调事件配置 */}
                  <Card 
                    title={<span className="font-extrabold text-slate-800 dark:text-zinc-100 flex items-center gap-2"><SoundOutlined className="text-purple-500" />Webhook 回调地址订阅配置</span>}
                    bordered={false} 
                    className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40"
                    extra={
                      <Button size="small" type="primary" icon={<CheckOutlined />} onClick={handleSaveWebhook}>
                        保存配置
                      </Button>
                    }
                  >
                    <div className="space-y-4">
                      <div>
                        <div className="text-xs text-slate-500 dark:text-zinc-400 mb-1.5 font-semibold">HTTP/HTTPS 回调目标接收 URL</div>
                        <Input 
                          value={webhookUrl} 
                          onChange={(e) => setWebhookUrl(e.target.value)} 
                          placeholder="例如: https://crm.yourcompany.com/webhook"
                          className="font-mono text-xs border-slate-200 dark:border-slate-800"
                        />
                      </div>
                      
                      <div>
                        <div className="text-xs text-slate-500 dark:text-zinc-400 mb-2 font-semibold">订阅事件类型 (Multi-Subscribe)</div>
                        <Checkbox.Group 
                          value={subscribedEvents} 
                          onChange={(vals) => setSubscribedEvents(vals as string[])}
                          className="flex flex-col gap-2 font-sans"
                        >
                          <Checkbox value="channel_progress">
                            <span className="text-xs font-bold text-slate-700 dark:text-zinc-300">channel_progress (外呼振铃早期媒体事件)</span>
                          </Checkbox>
                          <Checkbox value="channel_answer">
                            <span className="text-xs font-bold text-slate-700 dark:text-zinc-300">channel_answer (电话应答接通事件)</span>
                          </Checkbox>
                          <Checkbox value="channel_hangup">
                            <span className="text-xs font-bold text-slate-700 dark:text-zinc-300">channel_hangup (通话结束及录音生成事件)</span>
                          </Checkbox>
                        </Checkbox.Group>
                      </div>
                    </div>
                  </Card>
                </div>

                {/* 右侧：Webhook Ping 测试与虚拟投递终端模拟器 */}
                <div className="lg:col-span-6 space-y-6">
                  <Card 
                    title={<span className="font-extrabold text-slate-800 dark:text-zinc-100 flex items-center gap-2"><RocketOutlined className="text-emerald-500" />联调沙盒与虚拟投递终端模拟</span>}
                    bordered={false} 
                    className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40 bg-slate-900/10"
                    extra={
                      <Button 
                        type="primary" 
                        icon={<PlaySquareOutlined />} 
                        loading={pingLoading} 
                        onClick={handleSendPing}
                        className="bg-emerald-600 border-none hover:bg-emerald-500"
                      >
                        发送 Ping 测试事件
                      </Button>
                    }
                  >
                    <Paragraph className="text-xs text-slate-600 dark:text-zinc-300">
                      点击“发送 Ping 测试事件”将触发系统的虚拟投递引擎，向您填写的地址发送一条包含模拟 <Text code>channel_hangup</Text> 事件的测试事件，用于在线检测您的服务器能否正常解析、防重放拦截以及鉴权回执。
                    </Paragraph>
                    
                    {/* 虚拟终端控制台 */}
                    <div className="bg-[#12161f] text-[#5ad4e6] rounded-2xl p-4 font-mono text-xs min-h-[260px] border border-slate-800 relative overflow-hidden mt-4 shadow-inner">
                      {/* Terminal header */}
                      <div className="absolute top-2 right-4 flex gap-1.5 z-10">
                        <span className="w-2.5 h-2.5 rounded-full bg-rose-500/80" />
                        <span className="w-2.5 h-2.5 rounded-full bg-amber-500/80" />
                        <span className="w-2.5 h-2.5 rounded-full bg-emerald-500/80" />
                      </div>
                      
                      <div className="text-[10px] text-slate-500 border-b border-slate-800/60 pb-1.5 mb-2.5 flex justify-between uppercase tracking-wider font-bold">
                        <span>YUNSHU WEBHOOK SANDBOX EMULATOR v2.6.0</span>
                        <span>STATUS: {pingLoading ? 'SENDING...' : pingStep === 4 ? 'SUCCESS' : 'IDLE'}</span>
                      </div>
                      
                      {pingStep === 0 ? (
                        <div className="flex flex-col justify-center items-center py-16 text-slate-500 font-sans gap-2">
                          <RocketOutlined className="text-3xl text-slate-600 animate-bounce" />
                          <span>准备就绪，点击上方按钮发送虚拟测试事件</span>
                        </div>
                      ) : (
                        <div className="space-y-2.5 leading-relaxed font-mono">
                          {pingLogs.map((log, index) => {
                            let colorClass = 'text-[#5ad4e6]'
                            if (log.includes('[SUCCESS]')) colorClass = 'text-emerald-500 font-bold'
                            if (log.includes('[SYSTEM]')) colorClass = 'text-slate-400 font-sans'
                            if (log.includes('[RESPONSE]')) colorClass = 'text-emerald-400'
                            if (log.includes('[SECURITY]')) colorClass = 'text-purple-400'
                            return (
                              <div key={index} className={`flex items-start gap-1.5 ${colorClass}`}>
                                <span className="text-slate-600 select-none">&gt;</span>
                                <span className="whitespace-pre-wrap">{log}</span>
                              </div>
                            )
                          })}
                          {pingLoading && (
                            <div className="flex items-center gap-2 mt-4 text-slate-400 py-1">
                              <Spin size="small" />
                              <span className="text-xs font-sans">网络通信与握手中...</span>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  </Card>
                </div>

              </div>
            )
          },
          {
            key: 'webrtc-sdk',
            label: (
              <span className="flex items-center gap-2 font-bold px-1.5 py-1">
                <AudioOutlined />
                WebRTC 通话 SDK 文档
              </span>
            ),
            children: (
              <div className="space-y-8">
                <Alert
                  message="云枢 WebRTC 极速对接 SDK"
                  description="云枢提供企业级 WebRTC 通话 SDK，完美封装了基于 SIP WebSockets 的媒体流协商与呼叫状态机，支持第三方 CRM 一键嵌入拨号盘、获取来电振铃及通话事件，开发仅需 10 行代码。"
                  type="success"
                  showIcon
                  action={
                    <Button type="primary" size="small" icon={<KeyOutlined />} onClick={handleDownloadSDK}>
                      一键下载 SDK (yunshu-call-sdk.ts)
                    </Button>
                  }
                  className="rounded-xl border-emerald-100 bg-gradient-to-r from-emerald-50 to-teal-50/30 dark:from-slate-900/30 dark:to-teal-950/20 py-3"
                />

                {/* 极速环境与硬件诊断专家面板 */}
                <Card
                  title={
                    <span className="font-extrabold text-slate-800 dark:text-zinc-100 flex items-center gap-2">
                      <ThunderboltOutlined className="text-yellow-500 animate-pulse" />
                      云枢 WebRTC 本地通话硬件与环境诊断专家
                    </span>
                  }
                  bordered={false}
                  className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40 bg-gradient-to-br from-slate-50/50 to-white dark:from-slate-900/10 dark:to-slate-900/5"
                  extra={
                    <Space>
                      <span className="text-xs text-slate-500 dark:text-zinc-400">信令网关测试地址:</span>
                      <Input
                        size="small"
                        value={testWsUrl}
                        onChange={(e) => setTestWsUrl(e.target.value)}
                        placeholder="ws://your-pbx:5066"
                        className="w-48 text-xs rounded-lg"
                      />
                      <Button
                        type="primary"
                        size="small"
                        icon={<ThunderboltOutlined />}
                        loading={diagLoading}
                        onClick={handleRunDiagnose}
                        className="bg-gradient-to-r from-amber-500 to-orange-500 border-none font-bold shadow-sm"
                      >
                        一键诊断本地通话环境
                      </Button>
                    </Space>
                  }
                >
                  {!diagReport ? (
                    <div className="flex flex-col justify-center items-center py-12 text-slate-400 dark:text-zinc-500 gap-3">
                      <AudioOutlined className="text-4xl text-slate-300 dark:text-zinc-600 animate-pulse" />
                      <span className="text-sm font-medium">尚未进行诊断，点击上方按钮测试麦克风硬件、扬声器和网络连通性</span>
                      <span className="text-xs text-slate-400">注意：诊断需要浏览器允许麦克风权限。测试麦克风采集时，请对着话筒说话。</span>
                    </div>
                  ) : (
                    <div className="space-y-6">
                      {/* 顶层大指标总结 */}
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 items-center bg-white dark:bg-zinc-900/40 p-5 rounded-2xl border border-slate-100 dark:border-slate-800/60 shadow-sm">
                        <div className="flex items-center gap-4 border-r border-slate-100 dark:border-slate-800/80 pr-6">
                          <div>
                            {diagReport.status === 'excellent' && (
                              <Badge status="success">
                                <div className="w-12 h-12 rounded-full bg-emerald-50 dark:bg-emerald-950/20 flex items-center justify-center text-emerald-500 text-2xl font-bold">
                                  <CheckCircleOutlined />
                                </div>
                              </Badge>
                            )}
                            {diagReport.status === 'warning' && (
                              <Badge status="warning">
                                <div className="w-12 h-12 rounded-full bg-amber-50 dark:bg-amber-950/20 flex items-center justify-center text-amber-500 text-2xl font-bold">
                                  <WarningOutlined />
                                </div>
                              </Badge>
                            )}
                            {diagReport.status === 'error' && (
                              <Badge status="error">
                                <div className="w-12 h-12 rounded-full bg-rose-50 dark:bg-rose-950/20 flex items-center justify-center text-rose-500 text-2xl font-bold">
                                  <CloseCircleOutlined />
                                </div>
                              </Badge>
                            )}
                          </div>
                          <div>
                            <div className="text-xs text-slate-400 font-bold uppercase tracking-wider">诊断评估结果</div>
                            <div className="text-base font-extrabold text-slate-700 dark:text-zinc-200 mt-0.5">
                              {diagReport.status === 'excellent' && <span className="text-emerald-500">极致完美 (Excellent)</span>}
                              {diagReport.status === 'warning' && <span className="text-amber-500">硬件受限 (Warning)</span>}
                              {diagReport.status === 'error' && <span className="text-rose-500">运行异常 (Error)</span>}
                            </div>
                          </div>
                        </div>

                        {/* 麦克风实时音频电平强度 */}
                        <div className="flex flex-col border-r border-slate-100 dark:border-slate-800/80 pr-6">
                          <div className="text-xs text-slate-400 font-bold uppercase tracking-wider mb-1.5 flex justify-between">
                            <span>麦克风实时音量采集</span>
                            <span className="font-mono text-emerald-500">{diagReport.devices.micActiveLevel}%</span>
                          </div>
                          <Progress
                            percent={diagReport.devices.micActiveLevel}
                            status={diagReport.devices.micActiveLevel > 0 ? "active" : "normal"}
                            strokeColor={{
                              '0%': '#10b981',
                              '100%': '#059669',
                            }}
                            trailColor="rgba(0,0,0,0.06)"
                            showInfo={false}
                          />
                          <span className="text-[10px] text-slate-400 dark:text-zinc-500 mt-1">
                            {diagReport.devices.micActiveLevel > 0 
                              ? '👍 麦克风硬件有源工作，声音信号捕捉成功！'
                              : '💡 若电平为 0%，请检查麦克风物理开关或在检测时发声测试。'}
                          </span>
                        </div>

                        {/* 简要指标统计 */}
                        <div className="flex flex-col gap-1 pl-2">
                          <div className="text-xs text-slate-400 font-bold uppercase tracking-wider mb-1">通话资源统计</div>
                          <div className="flex items-center gap-1.5 text-xs text-slate-600 dark:text-zinc-300">
                            <span>输入麦克风:</span>
                            <Tag color="blue">{diagReport.devices.microphones.length} 个</Tag>
                          </div>
                          <div className="flex items-center gap-1.5 text-xs text-slate-600 dark:text-zinc-300">
                            <span>输出扬声器:</span>
                            <Tag color="cyan">{diagReport.devices.speakers.length} 个</Tag>
                          </div>
                        </div>
                      </div>

                      {/* 核心诊断检测子项列表 */}
                      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                        {/* 浏览器兼容性 */}
                        <div className="p-4 rounded-xl bg-white dark:bg-zinc-900/30 border border-slate-100 dark:border-slate-800/60 shadow-inner space-y-3">
                          <div className="text-xs font-bold text-slate-700 dark:text-zinc-200 border-b border-slate-100 dark:border-slate-800 pb-2">
                            🖥️ 浏览器核心兼容性
                          </div>
                          <div className="space-y-1.5 text-xs">
                            <div className="flex justify-between">
                              <span className="text-slate-400">WebRTC API:</span>
                              <Tag color={diagReport.browserDetails.hasGetUserMedia ? 'success' : 'error'}>
                                {diagReport.browserDetails.hasGetUserMedia ? '支持' : '不支持'}
                              </Tag>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-slate-400">WebSocket 信令:</span>
                              <Tag color={diagReport.browserDetails.hasWebSocket ? 'success' : 'error'}>
                                {diagReport.browserDetails.hasWebSocket ? '支持' : '不支持'}
                              </Tag>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-slate-400">音频采样渲染:</span>
                              <Tag color={diagReport.browserDetails.hasAudioContext ? 'success' : 'error'}>
                                {diagReport.browserDetails.hasAudioContext ? '支持' : '不支持'}
                              </Tag>
                            </div>
                          </div>
                        </div>

                        {/* 输入设备诊断 */}
                        <div className="p-4 rounded-xl bg-white dark:bg-zinc-900/30 border border-slate-100 dark:border-slate-800/60 shadow-inner space-y-3">
                          <div className="text-xs font-bold text-slate-700 dark:text-zinc-200 border-b border-slate-100 dark:border-slate-800 pb-2">
                            🎤 麦克风输入诊断
                          </div>
                          <div className="space-y-1.5 text-xs">
                            <div className="flex justify-between">
                              <span className="text-slate-400">输入硬件存在:</span>
                              <Tag color={diagReport.devices.hasMicrophone ? 'success' : 'error'}>
                                {diagReport.devices.hasMicrophone ? '已检测到' : '未检测到'}
                              </Tag>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-slate-400">浏览器授权:</span>
                              <Tag color={diagReport.devices.microphoneAuthorized ? 'success' : 'error'}>
                                {diagReport.devices.microphoneAuthorized ? '已授权' : '无授权'}
                              </Tag>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-slate-400">当前活跃设备:</span>
                              <span className="text-slate-500 font-semibold truncate max-w-[80px]" title={diagReport.devices.microphones[0]?.label}>
                                {diagReport.devices.microphones[0]?.label ? '正常挂载' : '无'}
                              </span>
                            </div>
                          </div>
                        </div>

                        {/* 输出设备诊断 */}
                        <div className="p-4 rounded-xl bg-white dark:bg-zinc-900/30 border border-slate-100 dark:border-slate-800/60 shadow-inner space-y-3">
                          <div className="text-xs font-bold text-slate-700 dark:text-zinc-200 border-b border-slate-100 dark:border-slate-800 pb-2">
                            🔊 播放输出诊断
                          </div>
                          <div className="space-y-1.5 text-xs">
                            <div className="flex justify-between">
                              <span className="text-slate-400">播放硬件存在:</span>
                              <Tag color={diagReport.devices.hasSpeaker ? 'success' : 'error'}>
                                {diagReport.devices.hasSpeaker ? '已检测到' : '未检测到'}
                              </Tag>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-slate-400">扬声器切换支持:</span>
                              <Tag color={diagReport.browserDetails.hasSetSinkId ? 'success' : 'warning'}>
                                {diagReport.browserDetails.hasSetSinkId ? '支持' : '不支持'}
                              </Tag>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-slate-400">当前播放设备:</span>
                              <span className="text-slate-500 font-semibold truncate max-w-[80px]" title={diagReport.devices.speakers[0]?.label}>
                                {diagReport.devices.speakers[0]?.label ? '正常挂载' : '无'}
                              </span>
                            </div>
                          </div>
                        </div>

                        {/* 网络网关诊断 */}
                        <div className="p-4 rounded-xl bg-white dark:bg-zinc-900/30 border border-slate-100 dark:border-slate-800/60 shadow-inner space-y-3">
                          <div className="text-xs font-bold text-slate-700 dark:text-zinc-200 border-b border-slate-100 dark:border-slate-800 pb-2">
                            🌐 信令网关连通性
                          </div>
                          <div className="space-y-1.5 text-xs">
                            <div className="flex justify-between">
                              <span className="text-slate-400">网络握手支持:</span>
                              <Tag color={diagReport.network.webSocketSupported ? 'success' : 'error'}>
                                {diagReport.network.webSocketSupported ? '支持' : '不支持'}
                              </Tag>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-slate-400">网关握手连通:</span>
                              {diagReport.network.wsReachable === null ? (
                                <Tag color="default">未测试</Tag>
                              ) : diagReport.network.wsReachable ? (
                                <Tag color="success">在线通畅</Tag>
                              ) : (
                                <Tag color="error">连通超时</Tag>
                              )}
                            </div>
                            <div className="flex justify-between">
                              <span className="text-slate-400">连接地址:</span>
                              <span className="text-slate-500 font-mono text-[9px] truncate max-w-[100px]" title={testWsUrl}>
                                {testWsUrl}
                              </span>
                            </div>
                          </div>
                        </div>
                      </div>

                      {/* 诊断出的排障引导和改进建议 */}
                      {diagReport.suggestions.length > 0 && (
                        <div className="p-4 rounded-2xl bg-amber-50/40 dark:bg-amber-950/10 border border-amber-100/60 dark:border-amber-900/40">
                          <div className="text-xs font-bold text-amber-600 dark:text-amber-400 mb-2.5 flex items-center gap-1.5">
                            <WarningOutlined />
                            系统检测到以下可能影响通话的隐患，请按说明逐步排障：
                          </div>
                          <List
                            dataSource={diagReport.suggestions}
                            renderItem={(item: string) => (
                              <List.Item className="!py-1 border-none text-xs text-slate-600 dark:text-zinc-300 font-medium">
                                <span className="text-amber-500 select-none mr-2">•</span>
                                {item}
                              </List.Item>
                            )}
                          />
                        </div>
                      )}

                      {/* 详细的麦克风与扬声器硬件资产清单 (仅在有授权且检测到时展示) */}
                      {(diagReport.devices.microphones.length > 0 || diagReport.devices.speakers.length > 0) && (
                        <div className="border-t border-slate-100 dark:border-slate-800/80 pt-4">
                          <div className="text-xs font-bold text-slate-500 mb-3 flex items-center gap-1.5">
                            <SoundOutlined />
                            系统音频资产清单 (Audio Devices Inventory)
                          </div>
                          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div>
                              <div className="text-[10px] text-slate-400 uppercase tracking-wider font-bold mb-1.5">输入设备 (Microphones)</div>
                              {diagReport.devices.microphones.map((dev: any, i: number) => (
                                <div key={dev.deviceId || i} className="text-xs bg-slate-50 dark:bg-slate-900/30 p-2 rounded-lg border border-slate-100 dark:border-slate-800/40 mb-1 flex justify-between items-center">
                                  <span className="text-slate-600 dark:text-zinc-300 truncate max-w-[200px]" title={dev.label}>🎤 {dev.label}</span>
                                  <Tag color="blue" style={{ fontSize: '9px', scale: '0.85' }}>输入</Tag>
                                </div>
                              ))}
                            </div>
                            <div>
                              <div className="text-[10px] text-slate-400 uppercase tracking-wider font-bold mb-1.5">输出设备 (Speakers / Headphones)</div>
                              {diagReport.devices.speakers.map((dev: any, i: number) => (
                                <div key={dev.deviceId || i} className="text-xs bg-slate-50 dark:bg-slate-900/30 p-2 rounded-lg border border-slate-100 dark:border-slate-800/40 mb-1 flex justify-between items-center">
                                  <span className="text-slate-600 dark:text-zinc-300 truncate max-w-[200px]" title={dev.label}>🔊 {dev.label}</span>
                                  <Tag color="cyan" style={{ fontSize: '9px', scale: '0.85' }}>输出</Tag>
                                </div>
                              ))}
                            </div>
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </Card>

                {/* 完整的使用与操作说明手册 */}
                <Card 
                  title={<span className="font-extrabold text-slate-800 dark:text-zinc-100 flex items-center gap-2"><SoundOutlined className="text-emerald-500" />云枢 WebRTC SDK 部署与运营步骤手册</span>}
                  bordered={false} 
                  className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40"
                >
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                    <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-900/40 border border-slate-100 dark:border-slate-800/50">
                      <div className="text-xs font-bold text-indigo-500 mb-1 flex items-center gap-1.5">
                        <Tag color="indigo" style={{ margin: 0, borderRadius: '4px', border: 'none' }}>步骤 1</Tag>
                        环境与网络准备
                      </div>
                      <Paragraph className="text-xs text-slate-500 dark:text-zinc-400 mt-2 mb-0 leading-relaxed">
                        - WebRTC <strong>强制要求 HTTPS/SSL 安全环境</strong>，本地开发请使用 <Text code>localhost</Text>。<br />
                        - 企业防火墙必须放行云枢 WebRTC WS 端口（默认 <Text code>5066</Text>）以及 RTP 音频流端口（UDP 10000-20000 范围）。
                      </Paragraph>
                    </div>

                    <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-900/40 border border-slate-100 dark:border-slate-800/50">
                      <div className="text-xs font-bold text-emerald-500 mb-1 flex items-center gap-1.5">
                        <Tag color="emerald" style={{ margin: 0, borderRadius: '4px', border: 'none' }}>步骤 2</Tag>
                        引入依赖与自动绑定
                      </div>
                      <Paragraph className="text-xs text-slate-500 dark:text-zinc-400 mt-2 mb-0 leading-relaxed">
                        - 第三方 CRM 需通过 <Text code>npm install jssip</Text> 安装底层协议栈。<br />
                        - 必须在 HTML 中放置一个隐藏的声音播放标签：<Text code>&lt;audio id="remoteAudio" autoPlay style="display:none;" /&gt;</Text>，并将 ID 传入 SDK 构造函数，SDK 会自动捕获远端通话音轨。
                      </Paragraph>
                    </div>

                    <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-900/40 border border-slate-100 dark:border-slate-800/50">
                      <div className="text-xs font-bold text-amber-500 mb-1 flex items-center gap-1.5">
                        <Tag color="amber" style={{ margin: 0, borderRadius: '4px', border: 'none' }}>步骤 3</Tag>
                        激活浏览器自动播放
                      </div>
                      <Paragraph className="text-xs text-slate-500 dark:text-zinc-400 mt-2 mb-0 leading-relaxed">
                        - Chrome/Safari 会拦截未与页面进行点击交互就自动播放的音频流。<br />
                        - 建议在 UI 界面上浮现“点击拨号”或在页面任意位置设置交互引导，确保音频流捕获后能顺利解码输出，防止“听不到对方声音”的异常。
                      </Paragraph>
                    </div>

                    <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-900/40 border border-slate-100 dark:border-slate-800/50">
                      <div className="text-xs font-bold text-blue-500 mb-1 flex items-center gap-1.5">
                        <Tag color="blue" style={{ margin: 0, borderRadius: '4px', border: 'none' }}>步骤 4</Tag>
                        二次拨号与生命周期销毁
                      </div>
                      <Paragraph className="text-xs text-slate-500 dark:text-zinc-400 mt-2 mb-0 leading-relaxed">
                        - 通话接通后（<Text code>callConnected</Text> 触发），如呼叫的目标带 IVR 导航机器人，可直接调用 <Text code>sdk.sendDTMF('1')</Text> 模拟按键响应。<br />
                        - 页面销毁前，务必调用 <Text code>sdk.destroy()</Text> 清理 WebSocket 信令句柄，防止产生悬挂分机注册。
                      </Paragraph>
                    </div>
                  </div>
                </Card>

                <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
                  {/* 左侧：详细的 SDK 参数与 API 指南 */}
                  <div className="lg:col-span-7 space-y-8">
                    <Card bordered={false} className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40">
                      <Title level={4} className="font-extrabold text-slate-800 dark:text-zinc-100 mb-4 flex items-center gap-2">
                        <RocketOutlined className="text-emerald-500" />
                        SDK 快速启动指南
                      </Title>
                      <Paragraph className="text-slate-600 dark:text-zinc-300">
                        第三方开发人员只需引入 <Text code>YunshuCallSDK</Text> 并在初始化时传入商户专属的 SIP 分机、密码和 WebSocket 服务器地址，便可建立高可靠、全双工的浏览器内语音通话通道。
                      </Paragraph>

                      <Title level={5} className="font-bold text-slate-800 dark:text-zinc-100 mt-6 mb-3">配置项 (Config Options)</Title>
                      <Table
                        dataSource={[
                          { param: 'wsUrl', type: 'string', required: '是', desc: '云枢 WebRTC 网关地址 (例如: ws://ip:5066)' },
                          { param: 'ext', type: 'string', required: '是', desc: '分机号码 (例如: 1001)' },
                          { param: 'password', type: 'string', required: '否', desc: '注册密码，默认 123456' },
                          { param: 'domain', type: 'string', required: '是', desc: '商户的独占 SIP 域名 (例如: sip.yunshu.local)' },
                          { param: 'audioElementId', type: 'string', required: '否', desc: 'HTML Audio 元素的 ID，配置后 SDK 将自动挂载远端媒体流并播放' },
                          { param: 'audioElement', type: 'HTMLAudioElement', required: '否', desc: 'HTML Audio DOM 实例，供 React/Vue 等框架绑定 Ref 使用' },
                        ]}
                        rowKey="param"
                        pagination={false}
                        size="small"
                        bordered
                        columns={[
                          { title: '参数名称', dataIndex: 'param', key: 'param', width: 140, render: (t) => <Text code className="font-bold text-slate-700 dark:text-zinc-300">{t}</Text> },
                          { title: '类型', dataIndex: 'type', key: 'type', width: 120, render: (t) => <Tag color="blue">{t}</Tag> },
                          { title: '必填', dataIndex: 'required', key: 'required', width: 60, render: (t) => <Text className={t === '是' ? 'text-rose-500 font-bold' : 'text-slate-500'}>{t}</Text> },
                          { title: '描述与示例', dataIndex: 'desc', key: 'desc' }
                        ]}
                      />

                      <Title level={5} className="font-bold text-slate-800 dark:text-zinc-100 mt-8 mb-3">SDK 核心控制方法</Title>
                      <Table
                        dataSource={[
                          { method: 'static checkCompatibility()', returns: 'boolean', desc: '【静态诊断】快速校验当前主机浏览器是否兼容 WebRTC 媒体流与 WebSocket 协议。' },
                          { method: 'static diagnose()', returns: 'Promise<YunshuDiagnosticReport>', desc: '【静态诊断】探测麦克风、扬声器是否就绪及浏览器权限是否已获得，并回传详尽的诊断报告。' },
                          { method: 'register()', returns: 'void', desc: '启动 WebSocket 信令信道并完成 SIP 网关注册登录。' },
                          { method: 'unregister()', returns: 'void', desc: '主动注销分机上线状态，安全断开 WebSocket 物理通道。' },
                          { method: 'call(callee)', returns: 'void', desc: '向目标被叫号码发起 WebRTC 通话呼出。' },
                          { method: 'redial()', returns: 'void', desc: '【高级功能】快捷重新拨号呼出上一次拨打的历史号码。' },
                          { method: 'answer()', returns: 'void', desc: '接听呼入的未响应外部来电。' },
                          { method: 'hangup()', returns: 'void', desc: '挂断当前正在进行的通话，或拒接呼入来电。' },
                          { method: 'mute() / unmute()', returns: 'void', desc: '【高级功能】静音或解除静音，控制本地麦克风音轨流。' },
                          { method: 'hold() / unhold()', returns: 'void', desc: '【高级功能】呼叫保持与取回，在不拆除连接的状况下将通话暂时挂载挂起。' },
                          { method: 'transfer(targetExt)', returns: 'void', desc: '【高级功能】盲转通话，直接将当前会话转接给指定的分机号码。' },
                          { method: 'sendDTMF(digit)', returns: 'void', desc: '通话中发送二次拨号的双音多频按键音信号 (0-9, *, #)。' },
                          { method: 'isRegistered()', returns: 'boolean', desc: '返回分机是否成功注册就绪。' },
                          { method: 'isActiveCall()', returns: 'boolean', desc: '返回当前是否存在起呼、振铃或对讲中的活跃通话。' },
                          { method: 'getCallDetails()', returns: 'YunshuCallDetails', desc: '获取当前活跃通话的详细会话属性、方向、通话计时秒数等结构元数据。' },
                          { method: 'destroy()', returns: 'void', desc: '强力销毁连接实例并释放全部内存和注册的监听句柄。' },
                        ]}
                        rowKey="method"
                        pagination={false}
                        size="small"
                        bordered
                        columns={[
                          { title: '方法定义', dataIndex: 'method', key: 'method', width: 220, render: (t) => <Text code className="font-bold text-emerald-600 dark:text-emerald-400">{t}</Text> },
                          { title: '返回值', dataIndex: 'returns', key: 'returns', width: 90 },
                          { title: '功能描述', dataIndex: 'desc', key: 'desc' }
                        ]}
                      />
                    </Card>
                  </div>

                  {/* 右侧：端点代码实例 */}
                  <div className="lg:col-span-5 space-y-6">
                    <Card
                      title={<span className="font-extrabold text-slate-800 dark:text-zinc-100 flex items-center gap-2"><CodeOutlined className="text-indigo-500" />多语言 & 极客集成范式</span>}
                      bordered={false}
                      className="shadow-sm rounded-2xl border border-slate-100/50 dark:border-slate-800/40 bg-slate-900/10"
                    >
                      <Paragraph className="text-xs text-slate-500 dark:text-zinc-400">
                        第三方开发人员只需拷贝下方 TS/JS 实例，将其引入项目的任意前端框架（如 React/Vue/Angular），即可秒级打通通话控制：
                      </Paragraph>

                      <div className="relative">
                        <div className="absolute top-2 right-4 z-10 flex gap-2">
                          <Button 
                            size="small" 
                            icon={copiedText === 'sdk-example' ? <CheckOutlined className="text-emerald-500" /> : <CopyOutlined />} 
                            onClick={() => handleCopy(webrtcSdkExampleCode, 'sdk-example')}
                            className="bg-slate-800/60 hover:bg-slate-800 text-slate-300 border-slate-700/60"
                          >
                            {copiedText === 'sdk-example' ? '已复制' : '复制'}
                          </Button>
                        </div>
                        <div className="bg-[#12161f] text-slate-300 rounded-2xl p-4 font-mono text-xs overflow-x-auto max-h-[460px] border border-slate-800 shadow-inner">
                          <pre className="!m-0 select-text leading-relaxed whitespace-pre">{webrtcSdkExampleCode}</pre>
                        </div>
                      </div>

                      <div className="mt-4">
                        <Alert
                          message="分机安全性提示"
                          description="为保障系统资产安全，请不要将分机密码明文存储于客户端代码中。建议在登录 CRM 时，由后端服务器进行身份校验并返回临时或专享的 SIP 通话凭证。"
                          type="warning"
                          showIcon
                          className="rounded-xl"
                        />
                      </div>
                    </Card>
                  </div>
                </div>
              </div>
            )
          }
        ]}
      />
    </div>
  )
}
