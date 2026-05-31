import JsSIP from 'jssip'

export interface YunshuCallConfig {
  /**
   * 云枢 WebRTC SIP 网关的 WebSocket 地址 (如 ws://127.0.0.1:5066)
   */
  wsUrl: string
  /**
   * 注册的分机号 (如 1001)
   */
  ext: string
  /**
   * 分机的注册密码 (默认 123456)
   */
  password?: string
  /**
   * 云枢 SIP 域名/服务器 (如 sip.yunshu.local)
   */
  domain: string
  /**
   * 用于自动绑定远端通话音频流的 HTML Audio 元素 ID
   */
  audioElementId?: string
  /**
   * 用于自动绑定远端通话音频流的 HTML Audio 元素实例
   */
  audioElement?: HTMLAudioElement
}

/**
 * 注册失败事件载荷
 */
export interface YunshuRegistrationFailedPayload {
  ext: string
  cause: string
  code: number
  message: string
}

/**
 * 通话状态基础信息
 */
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

/**
 * 呼叫失败事件载荷
 */
export interface YunshuCallFailedPayload {
  sessionId: string
  cause: string
  code: number
  message: string
}

/**
 * 通话结束事件载荷
 */
export interface YunshuCallEndedPayload {
  sessionId: string
  remoteUser: string
  direction: 'incoming' | 'outgoing'
  duration: number
  cause: string
}

/**
 * 振铃状态事件载荷
 */
export interface YunshuCallRingingPayload {
  sessionId: string
  direction: 'incoming' | 'outgoing'
  remoteUser: string
}

/**
 * 极客设备信息
 */
export interface YunshuDeviceInfo {
  deviceId: string
  groupId: string
  kind: 'audioinput' | 'audiooutput'
  label: string
}

/**
 * 深度系统与环境诊断报告
 */
export interface YunshuDiagnosticReport {
  /**
   * 浏览器整体 WebRTC 兼容性
   */
  supported: boolean
  
  // 浏览器特征检查
  browserDetails: {
    userAgent: string
    hasMediaDevices: boolean
    hasGetUserMedia: boolean
    hasRTCPeerConnection: boolean
    hasWebSocket: boolean
    hasAudioContext: boolean
    hasSetSinkId: boolean // 浏览器是否支持切换扬声器输出设备 (setSinkId)
  }

  // 硬件与授权诊断
  devices: {
    /**
     * 是否检测到至少一个麦克风 (输入设备)
     */
    hasMicrophone: boolean
    /**
     * 是否检测到至少一个扬声器/耳机 (输出设备)
     */
    hasSpeaker: boolean
    /**
     * 是否已获得麦克风的授权许可
     */
    microphoneAuthorized: boolean
    /**
     * 可用的麦克风列表
     */
    microphones: YunshuDeviceInfo[]
    /**
     * 可用的扬声器列表
     */
    speakers: YunshuDeviceInfo[]
    /**
     * 麦克风音频输入实时分贝强度测试 (0 - 100)
     * 通过实时捕获一小段音频流计算出的音量大小，验证硬件是否切实可用
     */
    micActiveLevel: number
  }

  // 服务与连通性诊断
  network: {
    /**
     * WebSocket 协议是否可用
     */
    webSocketSupported: boolean
    /**
     * SIP 端口连通性或域名解析状态 (可以通过尝试新建 WebSocket 检查)
     */
    wsReachable: boolean | null
  }

  /**
   * 总体诊断等级:
   * - 'excellent': 完美，有设备、有授权、网络测试通过
   * - 'warning': 存在次要隐患，如没有扬声器设备(部分移动端或虚拟机无输出)，或不支持 setSinkId
   * - 'error': 关键致命错误，没有麦克风、没有授权、或浏览器不支持 WebRTC
   */
  status: 'excellent' | 'warning' | 'error'

  /**
   * 精确的中文排障引导说明与警告
   */
  suggestions: string[]
}

export interface YunshuCallQualityReport {
  packetsLost: number       // 累计丢包数
  packetsReceived: number   // 累计接收包数
  lostRatio: number         // 瞬时丢包率 (百分比 0-100)
  jitter: number            // 抖动延迟 (毫秒)
  rtt: number               // 往返网络延迟 (毫秒)
  audioLevelInput: number   // 麦克风输入分贝级别 (0-100)
  audioLevelOutput: number  // 扬声器输出分贝级别 (0-100)
  quality: 'excellent' | 'good' | 'fair' | 'poor' // 通话质量评估
}

export type YunshuCallEvent =
  | 'connecting'         // WebSockets 正在连接网关
  | 'connected'          // WebSockets 信令握手成功
  | 'disconnected'       // 物理连接断开
  | 'registered'         // 账号在线注册就绪
  | 'unregistered'       // 账号注销成功
  | 'registrationFailed' // 注册失败 (回传 YunshuRegistrationFailedPayload)
  | 'callDialing'        // 开始起呼呼出 (回传拨号号码)
  | 'callRinging'        // 振铃阶段 (回传 YunshuCallRingingPayload)
  | 'callConnected'      // 通话接通成功 (回传当前通话详情)
  | 'callTick'           // 每一秒周期性通话时长心跳 (回传当前秒数)
  | 'callEnded'          // 通话挂断结束 (回传 YunshuCallEndedPayload)
  | 'callFailed'         // 呼叫失败或被对方拒接 (回传 YunshuCallFailedPayload)
  | 'callHold'           // 通话已进入呼叫保持
  | 'callUnhold'         // 通话取消保持，恢复对话
  | 'callMuted'          // 本地麦克风静音
  | 'callUnmuted'        // 本地麦克风解除静音
  | 'callQuality'        // [新功能] 实时音轨及丢包网络质量监测 (回传 YunshuCallQualityReport)
  | 'reconnecting'       // [新功能] 网络异常触发指数退避自动重连
  | 'log'                // 底层信令日志信息

export type YunshuCallEventCallback = (data?: any) => void

/**
 * 云枢 WebRTC 高级通话 SDK (YunshuCallSDK)
 * 
 * 为企业级 CRM 深度集成量身定制，提供高级电话功能（静音、保持、转接）、
 * 全闭环回调载荷以及精准通话详情查询。
 */
export class YunshuCallSDK {
  private config: YunshuCallConfig
  private ua: JsSIP.UA | null = null
  private session: any | null = null
  
  // 运行态标识
  private isSdkRegistered = false
  private isLocalMuted = false
  private isSessionOnHold = false
  private callStartTime = 0
  private timerInterval: any = null
  private lastDialedNumber = ''

  // 自动化断线指数退避重连机制
  private retryCount = 0
  private reconnectTimer: any = null
  private isAutoReconnecting = false

  // WebRTC 实时质量周期分析器
  private statsInterval: any = null
  private prevPacketsLost = 0
  private prevPacketsReceived = 0

  private callbacks: Record<YunshuCallEvent, YunshuCallEventCallback[]> = {
    connecting: [],
    connected: [],
    disconnected: [],
    registered: [],
    unregistered: [],
    registrationFailed: [],
    callDialing: [],
    callRinging: [],
    callConnected: [],
    callTick: [],
    callEnded: [],
    callFailed: [],
    callHold: [],
    callUnhold: [],
    callMuted: [],
    callUnmuted: [],
    callQuality: [],
    reconnecting: [],
    log: [],
  }

  constructor(config: YunshuCallConfig) {
    this.config = {
      password: '123456',
      ...config,
    }
  }

  /**
   * [高级诊断] 静态方法：检查浏览器是否具备 WebRTC 通话运行环境
   */
  public static checkCompatibility(): boolean {
    if (typeof window === 'undefined') return false
    const hasMediaDevices = !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia)
    const hasRTCPeerConnection = !!(window.RTCPeerConnection || (window as any).webkitRTCPeerConnection)
    const hasWebSocket = !!window.WebSocket
    return hasMediaDevices && hasRTCPeerConnection && hasWebSocket
  }

  /**
   * [高级诊断] 静态方法：检查并获取系统的麦克风、扬声器以及使用授权状况，进行实时分贝电平测试与信令连通性探测
   * @param wsUrl 可选，传入云枢 WebRTC 网关的 WebSocket 地址，将自动发起短暂的连通性探测
   */
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
      // 1. 尝试获取麦克风媒体流以探测物理硬件和授权状态
      stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false })
      if (stream) {
        report.devices.microphoneAuthorized = true
        
        // 2. 硬件确实可用后，进行麦克风音频采集强度测试 (Wow 体验)
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
            
            // 简单等待并采集一点样本，测算音量 RMS
            analyser.getByteTimeDomainData(dataArray)
            let sum = 0
            for (let i = 0; i < bufferLength; i++) {
              const value = (dataArray[i] - 128) / 128
              sum += value * value
            }
            const rms = Math.sqrt(sum / bufferLength)
            // 归一化为 0-100 的值
            report.devices.micActiveLevel = Math.min(Math.round(rms * 200), 100)
            
            // 关闭测试上下文
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
        report.suggestions.push(`【致命】麦克风捕获异常: ${err.message || err.name}`)
      }
    }

    try {
      // 3. 枚举物理音频输入和输出设备
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
      report.suggestions.push(`【警告】系统音频设备枚举失败: ${err.message}`)
    }

    // 如果拿到了 stream，及时予以关闭释放，防止流占用的浏览器指示灯一直亮着
    if (stream) {
      stream.getTracks().forEach(track => track.stop())
    }

    // 4. WebSocket 网关连通性简易测试
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
            }, 3000) // 3 秒超时

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
          report.suggestions.push(`【致命】无法连通云枢信令网关 WebSocket (${wsUrl})。请检查您的网络防火墙、PBX 服务运行状态，或 5066 端口是否开放。`)
        }
      } catch (wsErr) {
        report.network.wsReachable = false
      }
    }

    // 5. 综合评定诊断整体状态
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

  /**
   * 注册事件监听
   */
  public on(event: YunshuCallEvent, callback: YunshuCallEventCallback): this {
    if (this.callbacks[event]) {
      this.callbacks[event].push(callback)
    }
    return this
  }

  /**
   * 取消事件监听
   */
  public off(event: YunshuCallEvent, callback: YunshuCallEventCallback): this {
    if (this.callbacks[event]) {
      this.callbacks[event] = this.callbacks[event].filter(cb => cb !== callback)
    }
    return this
  }

  /**
   * 触发内部事件通知并记录日志
   */
  private trigger(event: YunshuCallEvent, data?: any): void {
    const list = this.callbacks[event] || []
    list.forEach(cb => {
      try {
        cb(data)
      } catch (err) {
        console.error(`[YunshuCallSDK] 触发回调 ${event} 发生内部异常:`, err)
      }
    })
  }

  /**
   * 打印 SDK 标准审计日志
   */
  private log(message: string): void {
    const time = new Date().toLocaleTimeString()
    const formatted = `[YunshuCallSDK][${time}] ${message}`
    this.trigger('log', formatted)
  }

  /**
   * 清除计时器
   */
  private clearTimer(): void {
    if (this.timerInterval) {
      clearInterval(this.timerInterval)
      this.timerInterval = null
    }
    this.callStartTime = 0
  }

  /**
   * 启动并向云枢 SIP 服务器注册分机
   */
  public register(): void {
    if (this.ua) {
      this.log('检测到已有连接，执行自动销毁重连流程')
      this.unregister()
    }

    const { wsUrl, ext, domain, password } = this.config
    this.log(`正在接入云枢 WebRTC 通话网关: sip:${ext}@${domain} (WS: ${wsUrl})`)

    try {
      const socket = new JsSIP.WebSocketInterface(wsUrl)
      const jssipConfig = {
        sockets: [socket],
        uri: `sip:${ext}@${domain}`,
        password: password,
        register: true,
        session_timers: false,
      }

      this.ua = new JsSIP.UA(jssipConfig)

      // 1. WebSocket 网络信令层监听
      this.ua.on('connecting', () => {
        this.log('网关信令连接握手中...')
        this.trigger('connecting')
      })

      this.ua.on('connected', () => {
        this.log('信令通道 WebSocket 握手成功')
        this.trigger('connected')
      })

      this.ua.on('disconnected', (e: any) => {
        this.log(`网关连接中断: ${e.error ? e.error.message : '主动下线或网络抖动'}`)
        this.isSdkRegistered = false
        this.trigger('disconnected')
        
        // [自我修复机制]：如果是异常中断且并非主动关闭导致，触发指数退避自动重连
        if (this.ua && !e.error?.message?.includes('stop')) {
          this.handleAutoReconnect()
        }
      })

      // 2. SIP 账户鉴权注册状态机
      this.ua.on('registered', () => {
        this.log(`分机 ${ext} 成功注册上线，状态：可呼出/可呼入`)
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
        this.log(`分机 ${ext} 已完成服务注销`)
        this.isSdkRegistered = false
        this.retryCount = 0
        this.isAutoReconnecting = false
        if (this.reconnectTimer) {
          clearTimeout(this.reconnectTimer)
          this.reconnectTimer = null
        }
        this.trigger('unregistered')
      })

      this.ua.on('registrationFailed', (e: any) => {
        const payload: YunshuRegistrationFailedPayload = {
          ext: ext,
          cause: e.cause || 'Unknown',
          code: e.response ? e.response.status_code : 401,
          message: e.response ? e.response.reason_phrase : 'Unauthorized credentials',
        }
        this.log(`分机注册鉴权失败! 状态码: ${payload.code} (${payload.message}), 原因: ${payload.cause}`)
        this.isSdkRegistered = false
        this.trigger('registrationFailed', payload)
      })

      // 3. 核心 WebRTC 呼叫会话处理器
      this.ua.on('newRTCSession', (e: any) => {
        const rtcSession = e.session
        this.session = rtcSession
        
        // 重置内部话务参数
        this.isLocalMuted = false
        this.isSessionOnHold = false
        this.clearTimer()

        const direction = rtcSession.direction
        const remoteUser = rtcSession.remote_identity.uri.user

        const ringingPayload: YunshuCallRingingPayload = {
          sessionId: rtcSession.id,
          direction: direction,
          remoteUser: remoteUser,
        }

        if (direction === 'incoming') {
          this.log(`接收到来电呼入, 主叫方: ${remoteUser}, 会话ID: ${rtcSession.id}`)
          this.trigger('callRinging', ringingPayload)
        } else {
          this.log(`正在发起外呼呼出, 被叫号码: ${remoteUser}, 会话ID: ${rtcSession.id}`)
          this.trigger('callDialing', remoteUser)
        }

        // WebRTC 信道 PeerConnection 初始化
        rtcSession.on('peerconnection', (data: any) => {
          this.log('WebRTC PeerConnection 信道初始化已建立，开始协商 SDP 握手')
          data.peerconnection.addEventListener('track', (event: any) => {
            this.log('信道捕获远端媒体轨道成功，准备播放...')
            let audioEl: HTMLAudioElement | null = null
            if (this.config.audioElement) {
              audioEl = this.config.audioElement
            } else if (this.config.audioElementId) {
              audioEl = document.getElementById(this.config.audioElementId) as HTMLAudioElement
            }

            if (audioEl && event.streams[0]) {
              audioEl.srcObject = event.streams[0]
              audioEl.play()
                .then(() => this.log('远端音轨已顺利在扬声器播放中'))
                .catch((err: any) => this.log(`远端音频自动播放因浏览器策略被挂起拦截: ${err.message}`))
            } else {
              this.log('【警告】未配置音频播放媒介 (audioElement)，无法播放远端声音！')
            }
          })
        })

        rtcSession.on('progress', () => {
          this.log('呼叫链路已送达，对方手机振铃中...')
          this.trigger('callRinging', ringingPayload)
        })

        rtcSession.on('accepted', () => {
          this.callStartTime = Date.now()
          this.log('对方接听接通！开启秒级时长计数器')
          
          // 开始周期性通话心跳事件
          this.timerInterval = setInterval(() => {
            this.trigger('callTick', this.getCallDuration())
          }, 1000)

          this.trigger('callConnected', this.getCallDetails())
          
          // [高级扩展功能]：启动 WebRTC Stats 实时音轨与丢包质量监测
          this.startStatsMonitoring(rtcSession.connection)
        })

        rtcSession.on('failed', (data: any) => {
          const payload: YunshuCallFailedPayload = {
            sessionId: rtcSession.id,
            cause: data.cause || 'Rejected',
            code: data.message ? data.message.status_code : 486,
            message: data.message ? data.message.reason_phrase : 'Rejected / No Answer',
          }
          this.log(`通话未能接通，失败码: ${payload.code} (${payload.message}), 原因: ${payload.cause}`)
          this.trigger('callFailed', payload)
          this.clearTimer()
          this.stopStatsMonitoring()
          this.session = null
        })

        rtcSession.on('ended', (data: any) => {
          const payload: YunshuCallEndedPayload = {
            sessionId: rtcSession.id,
            remoteUser: remoteUser,
            direction: direction,
            duration: this.getCallDuration(),
            cause: data.cause || 'Normal',
          }
          this.log(`通话结束正常挂断，总通话时长: ${payload.duration}秒, 挂断原因: ${payload.cause}`)
          this.trigger('callEnded', payload)
          this.clearTimer()
          this.stopStatsMonitoring()
          this.session = null
        })
      })

      this.ua.start()
    } catch (err: any) {
      this.log(`分机配置参数及实例初始化异常: ${err.message}`)
      throw err
    }
  }

  /**
   * 手动注销下线分机，清空 WS 链接
   */
  public unregister(): void {
    if (this.session) {
      this.hangup()
    }
    if (this.ua) {
      this.log('正在注销分机登录状态并主动断开网关通信链接')
      this.ua.stop()
      this.ua = null
      this.isSdkRegistered = false
    }
  }

  /**
   * 呼出电话
   * @param callee 被叫号码
   */
  public call(callee: string): void {
    if (!this.ua || !this.isSdkRegistered) {
      throw new Error('SDK 尚未注册在线！请先完成 register() 注册。')
    }
    if (!callee || !callee.trim()) {
      throw new Error('被叫拨打号码不能为空')
    }
    this.lastDialedNumber = callee.trim()
    this.log(`外呼号码: ${callee}`)
    const options = {
      mediaConstraints: { audio: true, video: false },
      rtcOfferConstraints: { offerToReceiveAudio: true, offerToReceiveVideo: false },
    }
    this.ua.call(`sip:${callee}@${this.config.domain}`, options)
  }

  /**
   * 重拨上一次呼出的电话
   */
  public redial(): void {
    if (!this.lastDialedNumber) {
      throw new Error('当前会话中无历史拨号记录，重拨失败')
    }
    this.log(`重新呼出上一次拨打号码: ${this.lastDialedNumber}`)
    this.call(this.lastDialedNumber)
  }

  /**
   * 接听外部来电
   */
  public answer(): void {
    if (this.session && this.session.direction === 'incoming') {
      this.log('响应接听外部来电')
      const options = {
        mediaConstraints: { audio: true, video: false },
      }
      this.session.answer(options)
    } else {
      this.log('无有效的未接听呼入会话')
    }
  }

  /**
   * 主动挂断或拒接电话
   */
  public hangup(): void {
    if (this.session) {
      this.log('主动切断呼叫会话/拒绝来电')
      this.session.terminate()
      this.session = null
    } else {
      this.log('无活跃会话需要挂断，重置计时器')
      this.clearTimer()
    }
  }

  /**
   * 发送 DTMF 按键
   * @param digit 按键字符 (0-9, *, #)
   */
  public sendDTMF(digit: string): void {
    if (this.session) {
      this.log(`呼叫中发送 DTMF 双音多频键音: ${digit}`)
      this.session.sendDTMF(digit)
    } else {
      this.log('不在活跃通话中，发送 DTMF 失败')
    }
  }

  /**
   * 开启静音（关闭本地麦克风流）
   */
  public mute(): void {
    if (this.session) {
      this.session.mute({ audio: true, video: false })
      this.isLocalMuted = true
      this.log('已开启静音，远端听不到您的麦克风声音')
      this.trigger('callMuted')
    } else {
      this.log('无活跃通话，静音无效')
    }
  }

  /**
   * 解除静音（恢复本地麦克风流）
   */
  public unmute(): void {
    if (this.session) {
      this.session.unmute({ audio: true, video: false })
      this.isLocalMuted = false
      this.log('已取消静音，远端已恢复麦克风拾音')
      this.trigger('callUnmuted')
    }
  }

  /**
   * 呼叫保持 (Place Call On Hold)
   * 将当前通话流挂起，远端听到背景保持音乐，不关闭连接
   */
  public hold(): void {
    if (this.session) {
      this.session.hold()
      this.isSessionOnHold = true
      this.log('通话已挂起置为保持状态')
      this.trigger('callHold')
    } else {
      this.log('无活跃通话，无法进行呼叫保持')
    }
  }

  /**
   * 呼叫取回 (Retrieve Call From Hold)
   * 恢复保持状态的通话，恢复双方直接对讲
   */
  public unhold(): void {
    if (this.session) {
      this.session.unhold()
      this.isSessionOnHold = false
      this.log('通话保持已恢复，重新进入对讲模式')
      this.trigger('callUnhold')
    }
  }

  /**
   * 盲转 (Blind Transfer)
   * 在通话过程中直接将当前客户转接给另外一个分机号，立即退出本次会话
   * @param targetExt 目标分机号 (例如: 1002)
   */
  public transfer(targetExt: string): void {
    if (!this.session) {
      throw new Error('没有活跃通话会话，无法发起转接。')
    }
    if (!targetExt || !targetExt.trim()) {
      throw new Error('转接目标分机号不能为空')
    }
    this.log(`正在将当前通话转接至分机: ${targetExt}`)
    this.session.refer(`sip:${targetExt}@${this.config.domain}`)
  }

  /**
   * 返回分机是否已正常注册上线
   */
  public isRegistered(): boolean {
    return this.isSdkRegistered
  }

  /**
   * 是否处于静音状态
   */
  public isMuted(): boolean {
    return this.isLocalMuted
  }

  /**
   * 通话是否在保持状态
   */
  public isOnHold(): boolean {
    return this.isSessionOnHold
  }

  /**
   * 获取当前通话进行的秒数
   */
  public getCallDuration(): number {
    if (this.callStartTime === 0) return 0
    return Math.floor((Date.now() - this.callStartTime) / 1000)
  }

  /**
   * 检查当前是否有活跃的起呼、振铃或已接通会话
   */
  public isActiveCall(): boolean {
    return this.session !== null
  }

  /**
   * 返回当前会话的极客详细话务元数据
   */
  public getCallDetails(): YunshuCallDetails {
    if (!this.session) {
      return {
        sessionId: '',
        caller: '',
        callee: '',
        direction: 'outgoing',
        remoteUser: '',
        duration: 0,
        status: 'idle',
        isMuted: false,
        isOnHold: false,
      }
    }

    let status: 'idle' | 'dialing' | 'ringing' | 'connected' | 'ended' = 'idle'
    if (this.session.isEstablished()) {
      status = 'connected'
    } else if (this.session.isInProgress()) {
      status = this.session.direction === 'incoming' ? 'ringing' : 'dialing'
    }

    return {
      sessionId: this.session.id,
      caller: this.config.ext,
      callee: this.session.remote_identity.uri.user,
      direction: this.session.direction,
      remoteUser: this.session.remote_identity.uri.user,
      duration: this.getCallDuration(),
      status: status,
      isMuted: this.isLocalMuted,
      isOnHold: this.isSessionOnHold,
    }
  }

  /**
   * [安全隐私扩展] 暂停当前会话在云端的录音 (防止敏感密码、卡号等隐私数据泄露)
   */
  public pauseRecording(): void {
    if (this.session) {
      this.log('【安全合规防线】敏感操作前，主动向云端信令网关请求临时暂停当前会话录音')
      this.session.sendInfo('application/json', JSON.stringify({ action: 'pause_recording' }))
    } else {
      this.log('无活跃通话，暂停录音无效')
    }
  }

  /**
   * [安全隐私扩展] 恢复当前会话在云端的录音
   */
  public resumeRecording(): void {
    if (this.session) {
      this.log('【安全合规防线】敏感操作结束，向云端信令网关请求恢复当前会话录音')
      this.session.sendInfo('application/json', JSON.stringify({ action: 'resume_recording' }))
    }
  }

  /**
   * [自我修复扩展] 指数退避网络自动重连逻辑
   */
  private handleAutoReconnect(): void {
    if (this.isAutoReconnecting || !this.ua) return
    this.isAutoReconnecting = true
    this.retryCount++
    const delay = Math.min(1000 * Math.pow(2, this.retryCount), 30000)
    
    this.log(`[自我修复] 检测到信道非正常下线，启动指数退避自动重连。第 ${this.retryCount} 次尝试，延迟 ${delay}ms...`)
    this.trigger('reconnecting', { retryCount: this.retryCount, nextRetryDelay: delay })

    this.reconnectTimer = setTimeout(() => {
      try {
        if (this.ua && this.isAutoReconnecting) {
          this.log('[自我修复] 重连触发，正在初始化注册重试...')
          this.register()
        }
      } catch (err: any) {
        this.log(`[自我修复] 自动重连注册异常: ${err.message}`)
      } finally {
        this.isAutoReconnecting = false
      }
    }, delay)
  }

  /**
   * [实时网络诊断扩展] 开启 WebRTC 核心 Stats 周期级丢包、延迟与分贝拾音监控
   */
  private startStatsMonitoring(pc: RTCPeerConnection): void {
    this.stopStatsMonitoring()
    this.prevPacketsLost = 0
    this.prevPacketsReceived = 0

    // 每 3 秒执行一次底层 Stats 数据采集 (MOS评估与防断线警告)
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
          // 1. 监测接收到的音频流丢包和抖动 (Inbound RTP)
          if (report.type === 'inbound-rtp' && report.mediaType === 'audio') {
            packetsLost = report.packetsLost || 0
            packetsReceived = report.packetsReceived || 0
            jitter = Math.round((report.jitter || 0) * 1000) // 转化为 ms
            
            // 2. 监测实时音轨播放分贝 (通过 inbound-rtp 的 audioLevel)
            if (report.audioLevel !== undefined) {
              audioLevelOutput = Math.round(report.audioLevel * 100)
            }
          }
          // 3. 监测采集到的音频流分贝 (Media Source)
          if (report.type === 'media-source' && report.kind === 'audio') {
            if (report.audioLevel !== undefined) {
              audioLevelInput = Math.round(report.audioLevel * 100)
            }
          }
          // 4. 监测物理往返延迟 (Remote Candidate Pair)
          if (report.type === 'candidate-pair' && report.state === 'succeeded') {
            rtt = Math.round((report.currentRoundTripTime || 0) * 1000) // 转化为 ms
          }
        })

        // 5. 计算瞬时丢包率
        const diffLost = packetsLost - this.prevPacketsLost
        const diffReceived = packetsReceived - this.prevPacketsReceived
        const total = diffLost + diffReceived
        const lostRatio = total > 0 ? Math.min(Math.round((diffLost / total) * 100), 100) : 0

        this.prevPacketsLost = packetsLost
        this.prevPacketsReceived = packetsReceived

        // 6. 通话质量度量评估 (电信级 MOS 等级分类)
        let quality: 'excellent' | 'good' | 'fair' | 'poor' = 'excellent'
        if (lostRatio > 10 || rtt > 350) {
          quality = 'poor' // 严重丢包或高延时
        } else if (lostRatio > 4 || rtt > 200) {
          quality = 'fair' // 质量一般，通话可能有碎音
        } else if (lostRatio > 1.5 || rtt > 100) {
          quality = 'good' // 良好
        }

        const qualityReport: YunshuCallQualityReport = {
          packetsLost,
          packetsReceived,
          lostRatio,
          jitter,
          rtt,
          audioLevelInput,
          audioLevelOutput,
          quality,
        }

        // 7. 发送实时话务事件与日志
        this.trigger('callQuality', qualityReport)
        if (quality === 'poor') {
          this.log(`【网络预警】当前通话网络较差！丢包率: ${lostRatio}%, 往返延迟(RTT): ${rtt}ms`)
        }
      } catch (err: any) {
        console.warn('[YunshuCallSDK] WebRTC Stats 收集异常:', err)
      }
    }, 3000)
  }

  /**
   * 停止 WebRTC 状态监控器
   */
  private stopStatsMonitoring(): void {
    if (this.statsInterval) {
      clearInterval(this.statsInterval)
      this.statsInterval = null
    }
  }

  /**
   * 彻底销毁 SDK 实例释放全部句柄与回调
   */
  public destroy(): void {
    this.unregister()
    this.clearTimer()
    this.stopStatsMonitoring()
    
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    
    this.callbacks = {
      connecting: [],
      connected: [],
      disconnected: [],
      registered: [],
      unregistered: [],
      registrationFailed: [],
      callDialing: [],
      callRinging: [],
      callConnected: [],
      callTick: [],
      callEnded: [],
      callFailed: [],
      callHold: [],
      callUnhold: [],
      callMuted: [],
      callUnmuted: [],
      callQuality: [],
      reconnecting: [],
      log: [],
    }
  }
}
