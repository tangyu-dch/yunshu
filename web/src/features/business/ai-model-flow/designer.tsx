import { Button, Form, Input, Modal, Popconfirm, Space, Tag, Typography, message, Select, Switch, InputNumber, Slider, Badge, Tooltip, Breadcrumb, Drawer } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, useMemo, useRef, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  PlusOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  SettingOutlined,
  SendOutlined,
  PhoneOutlined,
  CustomerServiceOutlined,
  NodeIndexOutlined,
  InfoCircleOutlined,
  SaveOutlined,
  ArrowLeftOutlined,
  SlidersOutlined,
  DisconnectOutlined,
  SoundOutlined
} from '@ant-design/icons'
import {
  fetchAiFlows,
  saveAiFlow,
  fetchAiModelConfigs,
  fetchAiProviders,
  AiProviderItem
} from '@/api/operate'

const { Title, Text, Paragraph } = Typography

// ==========================================
// 1. 类型定义与常理配置
// ==========================================

export type CustomReplyRule = {
  id: string
  name: string
  matchMode: 'dtmf' | 'semantic' | 'keyword' | 'fallback'
  intent: string
  replyText: string
  action: 'continue' | 'collect' | 'transfer' | 'hangup'
  actionParam?: string
}

export type AIFlowNode = {
  id: string
  type: 'start' | 'intent' | 'dtmf' | 'reply' | 'condition' | 'transfer' | 'end'
  label: string
  x: number
  y: number
  metadata: {
    asrEnabled?: boolean
    wsUrl?: string
    mixType?: 'mono' | 'mixed' | 'stereo'
    sampleRate?: '16k' | '8k'
    metadata?: string
    text?: string
    ttsVoice?: string
    speed?: number
    volume?: number
    maxDigits?: number
    timeout?: number
    targetType?: 'extension' | 'group'
    targetId?: string
    enableQueue?: boolean
    maxQueueTime?: number
    mohFile?: string
    llmProvider?: string
    llmModel?: string
    llmEndpoint?: string
    llmApiKey?: string
    llmTemperature?: number
    llmSystemPrompt?: string
  }
}

export type AIFlowEdge = {
  id: string
  source: string
  target: string
  sourceHandle?: string
}

export type AIFlowGraph = {
  nodes: AIFlowNode[]
  edges: AIFlowEdge[]
}

// 动态音色元数据 (用于支持前端高度可扩展动态表单渲染)
export const TTS_VOICES_BY_PROVIDER: Record<string, { label: string; value: string }[]> = {
  volc: [
    { label: '🎤 豆包女声 (极具情感)', value: 'bv001_streaming' },
    { label: '🎙️ 豆包男声 (专业高保真)', value: 'bv002_streaming' },
    { label: '📚 豆包说书 (自然流畅)', value: 'bv051_streaming' },
    { label: '🎮 豆包游戏 (朝气灵动)', value: 'bv004_streaming' },
  ],
  ali: [
    { label: '🎤 晓云 (标准女声)', value: 'Xiaoyun' },
    { label: '🎙️ 小宇 (标准男声)', value: 'Xiaoyu' },
    { label: '👧 小婷 (甜美客服女声)', value: 'Xiaoting' },
  ],
  openai: [
    { label: '🎤 Alloy (通用女声)', value: 'alloy' },
    { label: '🎙️ Echo (温柔男声)', value: 'echo' },
    { label: '📚 Fable (故事讲书)', value: 'fable' },
    { label: '🎙️ Onyx (低沉男声)', value: 'onyx' },
    { label: '🎤 Nova (朝气女声)', value: 'nova' },
    { label: '👧 Shimmer (活泼女声)', value: 'shimmer' },
  ],
  tencent: [
    { label: '🎤 智雅 (标准女声)', value: '101001' },
    { label: '🎙️ 智宽 (标准男声)', value: '101002' },
    { label: '👧 智美 (客服女声)', value: '101016' },
  ]
};

const defaultSampleGraph: AIFlowGraph = {
  nodes: [
    {
      id: 'node-start',
      type: 'start',
      label: '🏁 呼入接通 (推流启动)',
      x: 40,
      y: 220,
      metadata: {
        asrEnabled: true,
        wsUrl: 'ws://127.0.0.1:9002/asr',
        mixType: 'mono',
        sampleRate: '16k',
        metadata: '{"merchantId": "1001"}',
        llmProvider: 'Cloud枢私有大模型',
        llmModel: 'yunshu-private-v2',
        llmEndpoint: '',
        llmApiKey: '',
        llmTemperature: 0.7,
        llmSystemPrompt: '您是云枢智能话务员，请根据用户的提问礼貌回答，如果用户想要查话费请引导他说查话费，如果想要转接人工请引导他说转人工。'
      }
    },
    {
      id: 'node-intent',
      type: 'intent',
      label: '🤖 语音意图路由',
      x: 280,
      y: 220,
      metadata: {}
    },
    {
      id: 'node-bill',
      type: 'reply',
      label: '💬 播报: 查话费',
      x: 520,
      y: 80,
      metadata: {
        text: '您好，您的当前账户余额为 58 元，详情请登录云枢商户客户端查看。',
        ttsVoice: '晓雅',
        speed: 1.0,
        volume: 0.8
      }
    },
    {
      id: 'node-transfer',
      type: 'transfer',
      label: '👤 排队转人工',
      x: 520,
      y: 350,
      metadata: {
        targetType: 'group',
        targetId: '8001',
        enableQueue: true,
        maxQueueTime: 30,
        mohFile: 'standard_moh.wav'
      }
    },
    {
      id: 'node-bill-end',
      type: 'end',
      label: '❌ 挂断呼叫',
      x: 780,
      y: 80,
      metadata: {}
    },
    {
      id: 'node-agent-succ',
      type: 'reply',
      label: '💬 播报: 转接成功',
      x: 780,
      y: 230,
      metadata: {
        text: '正在为您接通人工座席，通话将被录音以保证合规质量。',
        ttsVoice: '阿强',
        speed: 1.0,
        volume: 0.9
      }
    },
    {
      id: 'node-agent-fail',
      type: 'reply',
      label: '💬 播报: 客服下班',
      x: 780,
      y: 450,
      metadata: {
        text: '对不起，当前人工客服均已下班。了解宽带迁移请按 2，或者留言。',
        ttsVoice: '晓雅',
        speed: 1.0,
        volume: 0.8
      }
    }
  ],
  edges: [
    { id: 'e-1', source: 'node-start', target: 'node-intent' },
    { id: 'e-2', source: 'node-intent', target: 'node-bill', sourceHandle: '我要查话费' },
    { id: 'e-3', source: 'node-intent', target: 'node-transfer', sourceHandle: '我要人工服务' },
    { id: 'e-4', source: 'node-bill', target: 'node-bill-end' },
    { id: 'e-5', source: 'node-transfer', target: 'node-agent-succ', sourceHandle: 'has_agent' },
    { id: 'e-6', source: 'node-transfer', target: 'node-agent-fail', sourceHandle: 'no_agent' }
  ]
}

export function AiModelFlowDesigner() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  
  const isNew = id === 'new'
  const flowId = isNew ? null : Number(id)
  
  // 画布拓扑数据
  const [graph, setGraph] = useState<AIFlowGraph>(defaultSampleGraph)
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [draggingNodeId, setDraggingNodeId] = useState<string | null>(null)

  // 1. 撤销 / 重做 (Undo / Redo) 历史记录栈及防闭包实时 Refs
  const [history, setHistory] = useState<AIFlowGraph[]>([defaultSampleGraph])
  const [historyIndex, setHistoryIndex] = useState<number>(0)
  
  const historyRef = useRef<AIFlowGraph[]>([defaultSampleGraph])
  const historyIndexRef = useRef<number>(0)
  const graphRef = useRef<AIFlowGraph>(defaultSampleGraph)
  const draggingNodeIdRef = useRef<string | null>(null)

  // 保持实时同步 Refs，防止快捷键及全局 mouseup 闭包过期旧数据
  useEffect(() => {
    historyRef.current = history
    historyIndexRef.current = historyIndex
    graphRef.current = graph
    draggingNodeIdRef.current = draggingNodeId
  }, [history, historyIndex, graph, draggingNodeId])

  // 物理推送新图快照锁入历史栈，自动截断失效的重做未来帧
  const saveHistory = (newGraph: AIFlowGraph) => {
    const nextHistory = historyRef.current.slice(0, historyIndexRef.current + 1)
    
    // 排重校验：如果拓扑完全无变化，不重复记录
    const lastGraph = nextHistory[nextHistory.length - 1]
    if (lastGraph && JSON.stringify(lastGraph) === JSON.stringify(newGraph)) {
      return
    }

    const updatedHistory = [...nextHistory, newGraph]
    setHistory(updatedHistory)
    setHistoryIndex(nextHistory.length)
  }

  // 物理撤销 Undo (Ctrl + Z / Cmd + Z)
  const undo = () => {
    const idx = historyIndexRef.current
    const hist = historyRef.current
    if (idx > 0) {
      const prevIndex = idx - 1
      setHistoryIndex(prevIndex)
      setGraph(hist[prevIndex])
      setSelectedNodeId(null)
      message.info('↩️ 已撤销操作')
    }
  }

  // 物理重做 Redo (Ctrl + Shift + Z / Cmd + Shift + Z)
  const redo = () => {
    const idx = historyIndexRef.current
    const hist = historyRef.current
    if (idx < hist.length - 1) {
      const nextIndex = idx + 1
      setHistoryIndex(nextIndex)
      setGraph(hist[nextIndex])
      setSelectedNodeId(null)
      message.info('↪️ 已重做操作')
    }
  }
  
  // 实时拖拽连线引擎状态 (Dynamic Connecting string)
  const [connectingStart, setConnectingStart] = useState<{ nodeId: string; handle?: string } | null>(null)
  const [mousePos, setMousePos] = useState<{ x: number; y: number }>({ x: 0, y: 0 })

  // 2. 无限大画布高级拖拉拽、平移、缩放状态
  const [pan, setPan] = useState<{ x: number; y: number }>({ x: 0, y: 0 })
  const [zoom, setZoom] = useState<number>(1)
  const [panMode, setPanMode] = useState<boolean>(false) // Figma 级空格/中键平移状态切换
  const [isDraggingCanvas, setIsDraggingCanvas] = useState<boolean>(false)
  const [isDragOverCanvas, setIsDragOverCanvas] = useState<boolean>(false) // 画布放置外部拖拽高亮状态
  const [hoveredNodeIdDuringConnect, setHoveredNodeIdDuringConnect] = useState<string | null>(null) // 拉线悬浮目标卡片吸附高亮
  const [hoveredEdgeId, setHoveredEdgeId] = useState<string | null>(null) // 悬浮激活连线删除按钮
  const canvasDragStart = useRef<{ x: number; y: number }>({ x: 0, y: 0 })
  
  // 仿真沙盒状态
  const [sandboxOpen, setSandboxOpen] = useState(false)
  const [isSaveModalOpen, setIsSaveModalOpen] = useState(false)
  const [mockAgentOnline, setMockAgentOnline] = useState(true)
  const [mockLogs, setMockLogs] = useState<string[]>([])
  const [asrInput, setAsrInput] = useState('')
  const [activeNodeId, setActiveNodeId] = useState<string | null>(null)
  const [isPlayingAudio, setIsPlayingAudio] = useState(false)
  const [audioText, setAudioText] = useState('')
  
  const canvasRef = useRef<HTMLDivElement>(null)
  const [form] = Form.useForm()
  const [nodeForm] = Form.useForm()

  // 监听全局空格键与 Ctrl+Z/Shift+Z 快捷键，无缝激活抓取平移与撤回重做
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const activeElem = document.activeElement
      const isInputActive = 
        activeElem && 
        (activeElem.tagName === 'INPUT' || 
          activeElem.tagName === 'TEXTAREA' || 
          activeElem.getAttribute('contenteditable') === 'true')

      // 1. 撤销快捷键 Ctrl+Z / Cmd+Z (输入框内除外)
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z' && !e.shiftKey) {
        if (isInputActive) return
        e.preventDefault()
        undo()
        return
      }

      // 2. 重做快捷键 Ctrl+Shift+Z / Cmd+Shift+Z 或 Ctrl+Y (输入框内除外)
      if (
        ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z' && e.shiftKey) ||
        ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'y')
      ) {
        if (isInputActive) return
        e.preventDefault()
        redo()
        return
      }

      // 3. 空格平移抓取模式 (输入框内除外)
      if (e.code === 'Space') {
        if (isInputActive) return
        e.preventDefault()
        setPanMode(true)
      }
    }
    const handleKeyUp = (e: KeyboardEvent) => {
      if (e.code === 'Space') {
        setPanMode(false)
      }
    }
    
    // 全局 mouseup 终极防护锁：如果在任何区域松开了鼠标，强行归零所有拖动状态，并在拖动卡片结束时保存历史快照
    const handleGlobalMouseUp = () => {
      const draggedId = draggingNodeIdRef.current
      if (draggedId) {
        // 卡片移动物理释放，锁定当前位置历史快照
        saveHistory(graphRef.current)
      }
      setDraggingNodeId(null)
      setIsDraggingCanvas(false)
    }
    
    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('keyup', handleKeyUp)
    window.addEventListener('mouseup', handleGlobalMouseUp)
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('keyup', handleKeyUp)
      window.removeEventListener('mouseup', handleGlobalMouseUp)
    }
  }, [])

  // 读取商户 AI 智能流列表
  const { data: flowsData, isLoading: flowsLoading } = useQuery({
    queryKey: ['merchant', 'ai-flow', 1, 100],
    queryFn: () => fetchAiFlows(1, 100),
  })

  // 读取 AI 大模型厂商与 API 预设配置
  const { data: configsData } = useQuery({
    queryKey: ['merchant', 'ai-model-configs'],
    queryFn: () => fetchAiModelConfigs(),
  })

  // 读取已支持的大模型服务商列表（后端配置驱动，高可扩展性）
  const { data: providersList } = useQuery({
    queryKey: ['merchant', 'ai-providers'],
    queryFn: () => fetchAiProviders(),
  })

  const DEFAULT_PROVIDERS: AiProviderItem[] = useMemo(() => [
    { value: 'deepseek', label: 'DeepSeek API', emoji: '🐳', color: 'cyan', implemented: true, supportAsr: false, supportTts: false, supportLlm: true },
    { value: 'openai', label: 'OpenAI 兼容接口', emoji: '🌐', color: 'purple', implemented: true, supportAsr: true, supportTts: true, supportLlm: true },
    { value: 'ali', label: '阿里通义千问 Qwen', emoji: '☁️', color: 'geekblue', implemented: true, supportAsr: true, supportTts: true, supportLlm: true },
    { value: 'tencent', label: '腾讯混元 Hunyuan', emoji: '🐧', color: 'blue', implemented: true, supportAsr: true, supportTts: true, supportLlm: true },
    { value: 'volc', label: '火山引擎“豆包”大模型', emoji: '🌋', color: 'orange', implemented: true, supportAsr: true, supportTts: true, supportLlm: true }
  ], [])

  const providers = providersList || DEFAULT_PROVIDERS

  const currentFlow = useMemo(() => {
    if (isNew || !flowsData) return null
    return flowsData.records.find(r => r.id === flowId)
  }, [flowsData, flowId, isNew])

  // 初始化话术配置与图数据
  useEffect(() => {
    if (currentFlow) {
      form.setFieldsValue({
        name: currentFlow.name,
        prompt: currentFlow.prompt || '你是一个云枢智能电话应答助手。根据客户说的话，分发到对应节点。',
        description: currentFlow.description || ''
      })
      if (currentFlow.flowGraph && currentFlow.flowGraph.nodes && currentFlow.flowGraph.nodes.length > 0) {
        setGraph(currentFlow.flowGraph)
        setHistory([currentFlow.flowGraph])
        setHistoryIndex(0)
      } else {
        setGraph(defaultSampleGraph)
        setHistory([defaultSampleGraph])
        setHistoryIndex(0)
      }
    } else if (isNew) {
      form.setFieldsValue({
        prompt: '你是一个云枢智能电话应答助手。根据客户说的话，分发到对应节点。'
      })
      setGraph(defaultSampleGraph)
      setHistory([defaultSampleGraph])
      setHistoryIndex(0)
    }
  }, [currentFlow, isNew, form])

  // 编译生成向下兼容传统匹配规则
  const compileCustomReplies = (g: AIFlowGraph): CustomReplyRule[] => {
    const rules: CustomReplyRule[] = []
    const intentNode = g.nodes.find(n => n.type === 'intent')
    if (intentNode) {
      const edges = g.edges.filter(e => e.source === intentNode.id)
      edges.forEach(e => {
        if (e.sourceHandle) {
          const targetNode = g.nodes.find(n => n.id === e.target)
          rules.push({
            id: `rule-${e.id}`,
            name: `${e.sourceHandle}规则`,
            matchMode: 'semantic',
            intent: e.sourceHandle,
            replyText: targetNode?.metadata?.text || '处理中',
            action: targetNode?.type === 'transfer' ? 'transfer' : 'continue',
            actionParam: targetNode?.metadata?.targetId || undefined
          })
        }
      })
    }
    const transferNode = g.nodes.find(n => n.type === 'transfer')
    if (transferNode) {
      rules.push({
        id: 'rule-acd-timeout',
        name: '排队无坐席兜底',
        matchMode: 'fallback',
        intent: 'queue_timeout',
        replyText: g.nodes.find(n => n.id === g.edges.find(e => e.source === transferNode.id && e.sourceHandle === 'no_agent')?.target)?.metadata?.text || '当前坐席全忙',
        action: 'hangup'
      })
    }
    return rules
  }

  const saveMutation = useMutation({
    mutationFn: async (values: any) => {
      const payload = {
        id: flowId ?? undefined,
        name: values.name,
        prompt: values.prompt,
        description: values.description,
        customReplies: compileCustomReplies(graph),
        flowGraph: graph
      }
      return saveAiFlow(payload)
    },
    onSuccess: async () => {
      message.success(isNew ? '云枢 AI 模型流创建成功' : '云枢 AI 可视化画布已保存')
      await queryClient.invalidateQueries({ queryKey: ['merchant', 'ai-flow'] })
      navigate('/merchant/ai-model-flow')
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '保存失败'),
  })

  // ==========================================
  // 画布平移、卡片移动与 HTML5 托拽算法
  // ==========================================
  
  const handleCanvasMouseDown = (e: React.MouseEvent) => {
    // 只有在 panMode 激活，或者按下鼠标中键 (button === 1) 时启动画布拖拽平移
    if (panMode || e.button === 1) {
      e.preventDefault()
      setIsDraggingCanvas(true)
      canvasDragStart.current = { x: e.clientX, y: e.clientY }
    }
  }

  const handleNodeMouseDown = (nodeId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    // 如果是在端子物理圆点上按下，不触发卡片拖动，转为触发拉连线
    const target = e.target as HTMLElement
    if (target.classList.contains('port-dot')) return

    setDraggingNodeId(nodeId)
    setSelectedNodeId(nodeId)
    const node = graph.nodes.find(n => n.id === nodeId)
    if (node) {
      nodeForm.setFieldsValue({
        label: node.label,
        ...node.metadata
      })
    }
  }

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!canvasRef.current) return
    const rect = canvasRef.current.getBoundingClientRect()
    const currentX = e.clientX - rect.left
    const currentY = e.clientY - rect.top

    // 核心物理保护：如果鼠标左键物理上并没有被按下（e.buttons 的左键掩码为 1），
    // 强制终止并重置所有的拖动、拉线和平移状态，防止卡片悬空跟着移动的幽灵 bug。
    if ((e.buttons & 1) === 0) {
      if (draggingNodeId) setDraggingNodeId(null)
      if (isDraggingCanvas) setIsDraggingCanvas(false)
      if (connectingStart) {
        setConnectingStart(null)
        setHoveredNodeIdDuringConnect(null)
      }
      return
    }

    // 1. 无限大画布抓取平移
    if (isDraggingCanvas) {
      const dx = e.clientX - canvasDragStart.current.x
      const dy = e.clientY - canvasDragStart.current.y
      setPan(prev => ({ x: prev.x + dx, y: prev.y + dy }))
      canvasDragStart.current = { x: e.clientX, y: e.clientY }
      return
    }

    // 2. 拖拽卡片移动位置 (逻辑坐标转换与网格 10px 对齐)
    if (draggingNodeId) {
      const logicalX = (currentX - pan.x) / zoom - 105
      const logicalY = (currentY - pan.y) / zoom - 36
      const gridX = Math.round(logicalX / 10) * 10
      const gridY = Math.round(logicalY / 10) * 10
      
      const x = Math.max(10, gridX)
      const y = Math.max(10, gridY)
      setGraph(prev => ({
        ...prev,
        nodes: prev.nodes.map(node => node.id === draggingNodeId ? { ...node, x, y } : node)
      }))
    }

    // 3. 拖拽拉连线
    if (connectingStart) {
      const logicalX = (currentX - pan.x) / zoom
      const logicalY = (currentY - pan.y) / zoom
      setMousePos({ x: logicalX, y: logicalY })
    }
  }

  const handleMouseUp = (e: React.MouseEvent) => {
    if (isDraggingCanvas) {
      setIsDraggingCanvas(false)
      return
    }
    setDraggingNodeId(null)
    
    // 如果处于拉线连接释放状态
    if (connectingStart) {
      // 检查释放时的目标卡片是否是有效节点
      const target = e.target as HTMLElement
      const destNodeId = target.getAttribute('data-node-id') || target.closest('[data-node-id]')?.getAttribute('data-node-id')
      
      if (destNodeId && destNodeId !== connectingStart.nodeId) {
        // 创建一条全新的有向连接线
        const newEdge: AIFlowEdge = {
          id: `e-${Date.now()}`,
          source: connectingStart.nodeId,
          target: destNodeId,
          sourceHandle: connectingStart.handle
        }
        
        // 排重，避免重复连线
        const exists = graph.edges.some(edge => 
          edge.source === newEdge.source && 
          edge.target === newEdge.target && 
          edge.sourceHandle === newEdge.sourceHandle
        )
        
        if (!exists) {
          setGraph(prev => {
            const nextGraph = {
              ...prev,
              edges: [...prev.edges, newEdge]
            }
            saveHistory(nextGraph)
            return nextGraph
          })
          message.success('🔗 节点连线建立成功！')
        }
      }
      setConnectingStart(null)
      setHoveredNodeIdDuringConnect(null)
    }
  }

  // ==========================================
  // 右侧物理端子拖拽连线引擎 (Drag-to-Connect)
  // ==========================================
  const handlePortMouseDown = (nodeId: string, handle: string | undefined, e: React.MouseEvent) => {
    e.stopPropagation()
    e.preventDefault()
    if (!canvasRef.current) return
    const rect = canvasRef.current.getBoundingClientRect()
    const currentX = e.clientX - rect.left
    const currentY = e.clientY - rect.top
    
    // 逻辑坐标系下起点与终点初始值
    const logicalX = (currentX - pan.x) / zoom
    const logicalY = (currentY - pan.y) / zoom

    setConnectingStart({ nodeId, handle })
    setMousePos({ x: logicalX, y: logicalY })
  }

  // 释放至卡片实体上，触发连接建立
  const handleNodeMouseUp = (nodeId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    if (connectingStart && connectingStart.nodeId !== nodeId) {
      const newEdge: AIFlowEdge = {
        id: `e-${Date.now()}`,
        source: connectingStart.nodeId,
        target: nodeId,
        sourceHandle: connectingStart.handle
      }
      const exists = graph.edges.some(edge => 
        edge.source === newEdge.source && 
        edge.target === newEdge.target && 
        edge.sourceHandle === newEdge.sourceHandle
      )
      if (!exists) {
        setGraph(prev => {
          const nextGraph = {
            ...prev,
            edges: [...prev.edges, newEdge]
          }
          saveHistory(nextGraph)
          return nextGraph
        })
        message.success('🔗 节点连线建立成功！')
      }
    }
    setConnectingStart(null)
    setHoveredNodeIdDuringConnect(null)
  }

  // 物理从画布中删除指定连线
  const deleteEdge = (edgeId: string) => {
    setGraph(prev => {
      const nextGraph = {
        ...prev,
        edges: prev.edges.filter(e => e.id !== edgeId)
      }
      saveHistory(nextGraph)
      return nextGraph
    })
    message.info('连线已抹除')
  }

  // 双击新增节点
  const handleCanvasDoubleClick = (e: React.MouseEvent) => {
    if (e.target !== canvasRef.current && (e.target as HTMLElement).tagName !== 'svg') return
    if (!canvasRef.current) return
    const rect = canvasRef.current.getBoundingClientRect()
    // 物理坐标 -> 逻辑坐标并网格对齐
    const logicalX = (e.clientX - rect.left - pan.x) / zoom - 105
    const logicalY = (e.clientY - rect.top - pan.y) / zoom - 36
    const x = Math.round(logicalX / 10) * 10
    const y = Math.round(logicalY / 10) * 10
    addPaletteNode('reply', x, y)
  }

  // 左侧节点库面板实例化节点卡片
  const addPaletteNode = (type: AIFlowNode['type'], x = 200, y = 200) => {
    const id = `node-${Date.now()}`
    let label = '💬 播报播放'
    let meta: AIFlowNode['metadata'] = {}

    if (type === 'start') {
      label = '🏁 新呼入接通'
      meta = { 
        asrEnabled: true, 
        wsUrl: 'ws://127.0.0.1:9002/asr', 
        mixType: 'mono', 
        sampleRate: '16k',
        llmProvider: 'Cloud枢私有大模型',
        llmModel: 'yunshu-private-v2',
        llmEndpoint: '',
        llmApiKey: '',
        llmTemperature: 0.7,
        llmSystemPrompt: '您是云枢智能话务员，请根据用户的提问礼貌回答，如果用户想要查话费请引导他说查话费，如果想要转接人工请引导他说转人工。'
      }
    } else if (type === 'intent') {
      label = '🤖 新意图路由'
    } else if (type === 'dtmf') {
      label = '📞 新按键收集'
      meta = { maxDigits: 1, timeout: 5 }
    } else if (type === 'transfer') {
      label = '👤 新 ACD 排队转人工'
      meta = { targetType: 'group', enableQueue: true, maxQueueTime: 30 }
    } else if (type === 'end') {
      label = '❌ 新挂断呼叫'
    } else if (type === 'reply') {
      label = '💬 新播报播放'
      meta = { text: '您好！自定义语音播报内容', ttsVoice: '晓雅', speed: 1.0 }
    }

    const newNode: AIFlowNode = { id, type, label, x, y, metadata: meta }
    setGraph(prev => {
      const nextGraph = { ...prev, nodes: [...prev.nodes, newNode] }
      saveHistory(nextGraph)
      return nextGraph
    })
    setSelectedNodeId(id)
    nodeForm.setFieldsValue({ label, ...meta })
  }

  // 物理删除卡片
  const deleteNode = (nodeId: string) => {
    const node = graph.nodes.find(n => n.id === nodeId)
    if (node?.type === 'start') {
      message.error('🏁 开始节点为流图入口，禁止抹除！')
      return
    }
    setGraph(prev => {
      const nextGraph = {
        nodes: prev.nodes.filter(n => n.id !== nodeId),
        edges: prev.edges.filter(e => e.source !== nodeId && e.target !== nodeId)
      }
      saveHistory(nextGraph)
      return nextGraph
    })
    if (selectedNodeId === nodeId) {
      setSelectedNodeId(null)
    }
    message.info('节点及关联连线已抹除')
  }

  const deleteNodeAndConnection = (nodeId: string) => {
    deleteNode(nodeId)
  }

  // ==========================================
  // HTML5 Drag & Drop 托拽物理创建节点
  // ==========================================
  const handlePaletteDragStart = (e: React.DragEvent, type: AIFlowNode['type']) => {
    e.dataTransfer.setData('application/reactflow-nodetype', type)
    e.dataTransfer.effectAllowed = 'move'
  }

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
  }

  const handleDragEnter = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragOverCanvas(true)
  }

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault()
    // 防止移入子元素时误触 leave
    const rect = canvasRef.current?.getBoundingClientRect()
    if (rect) {
      if (
        e.clientX < rect.left || 
        e.clientX > rect.right || 
        e.clientY < rect.top || 
        e.clientY > rect.bottom
      ) {
        setIsDragOverCanvas(false)
      }
    }
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragOverCanvas(false)
    if (!canvasRef.current) return
    
    const type = e.dataTransfer.getData('application/reactflow-nodetype') as AIFlowNode['type']
    if (!type) return

    const rect = canvasRef.current.getBoundingClientRect()
    // 计算逻辑坐标（放置到鼠标中心）
    const logicalX = (e.clientX - rect.left - pan.x) / zoom - 105
    const logicalY = (e.clientY - rect.top - pan.y) / zoom - 36
    
    const gridX = Math.round(logicalX / 10) * 10
    const gridY = Math.round(logicalY / 10) * 10
    const x = Math.max(10, gridX)
    const y = Math.max(10, gridY)

    addPaletteNode(type, x, y)
    message.success(`📥 成功托拽创建了新【${getNodeTypeNameChinese(type)}】节点`)
  }

  const getNodeTypeNameChinese = (type: AIFlowNode['type']) => {
    switch (type) {
      case 'start': return '开始推流'
      case 'intent': return '语音意图路由'
      case 'dtmf': return '按键收集路由'
      case 'reply': return '语音播报播放'
      case 'transfer': return 'ACD 转人工'
      case 'end': return '呼叫挂断结束'
      default: return '未知'
    }
  }

  // ==========================================
  // ✨ 智能拓扑一键物理自动整理布局算法
  // ==========================================
  const autoLayoutGraph = () => {
    const layers: Record<string, number> = {}
    const visited = new Set<string>()
    
    const startNode = graph.nodes.find(n => n.type === 'start')
    if (!startNode) {
      message.error('未找到开始节点，无法自动布局')
      return
    }

    // BFS 广度优先层级判定
    const queue: [string, number][] = [[startNode.id, 0]]
    
    while (queue.length > 0) {
      const [currentId, depth] = queue.shift()!
      if (layers[currentId] === undefined || depth > layers[currentId]) {
        layers[currentId] = depth
      }
      
      const outEdges = graph.edges.filter(e => e.source === currentId)
      outEdges.forEach(edge => {
        if (!visited.has(edge.target)) {
          queue.push([edge.target, depth + 1])
        }
      })
      visited.add(currentId)
    }

    // 对孤立节点兜底
    graph.nodes.forEach(node => {
      if (layers[node.id] === undefined) {
        layers[node.id] = 1 
      }
    })

    // 按层级对节点进行分组
    const groups: Record<number, string[]> = {}
    Object.entries(layers).forEach(([nodeId, depth]) => {
      if (!groups[depth]) {
        groups[depth] = []
      }
      groups[depth].push(nodeId)
    })

    // 物理对齐排版
    const updatedNodes = graph.nodes.map(node => {
      const depth = layers[node.id] ?? 1
      const columnNodes = groups[depth] ?? []
      const index = columnNodes.indexOf(node.id)
      
      const x = 40 + depth * 240
      const y = 80 + index * 120
      
      return {
        ...node,
        x: Math.round(x / 10) * 10,
        y: Math.round(y / 10) * 10
      }
    })

    setGraph(prev => {
      const nextGraph = { ...prev, nodes: updatedNodes }
      saveHistory(nextGraph)
      return nextGraph
    })
    
    message.success('✨ 智能节点物理排版对齐成功！')
  }

  const debounceHistoryTimeout = useRef<any | null>(null)

  // Inspector 卡片属性物理变更增量分发
  const handleNodeMetadataChange = (changedValues: any) => {
    if (!selectedNodeId) return
    setGraph(prev => {
      const nextGraph = {
        ...prev,
        nodes: prev.nodes.map(node => {
          if (node.id !== selectedNodeId) return node
          const { label, ...metadata } = changedValues
          return {
            ...node,
            label: label !== undefined ? label : node.label,
            metadata: {
              ...node.metadata,
              ...metadata
            }
          }
        })
      }

      // 防抖锁入历史记录栈，打字完后 600ms 锁定快照
      if (debounceHistoryTimeout.current) {
        clearTimeout(debounceHistoryTimeout.current)
      }
      debounceHistoryTimeout.current = setTimeout(() => {
        saveHistory(nextGraph)
      }, 600)

      return nextGraph
    })
  }

  // ==========================================
  // 连线坐标获取 (逻辑坐标系)
  // ==========================================
  const getPortCoordinates = (nodeId: string, handle?: string, isSource?: boolean) => {
    const node = graph.nodes.find(n => n.id === nodeId)
    if (!node) return { x: 0, y: 0 }
    const cardWidth = 210
    const cardHeight = 72
    
    // 如果是源节点（即输出端子），且 handle 未指定（单输出端口情况），它的 x 应该在卡片右侧！
    if (isSource && !handle) {
      return { x: node.x + cardWidth, y: node.y + cardHeight / 2 }
    }
    
    // 如果是目标节点（即输入端子），且 handle 未指定，它的 x 应该在卡片左侧！
    if (!isSource && !handle) {
      return { x: node.x, y: node.y + cardHeight / 2 }
    }
    
    // 如果指定了 handle，则是多输出端子，必在右侧
    let offsetFraction = 0.5
    if (handle === 'has_agent') offsetFraction = 0.3
    if (handle === 'no_agent') offsetFraction = 0.7
    if (handle === '我要人工服务') offsetFraction = 0.7
    if (handle === '我要查话费') offsetFraction = 0.3
    if (handle === '1') offsetFraction = 0.3
    if (handle === '2') offsetFraction = 0.7
    return { x: node.x + cardWidth, y: node.y + cardHeight * offsetFraction }
  }

  // ==========================================
  // 仿真沙盒与真人声音播放算法
  // ==========================================
  const speakText = (text: string) => {
    if (!text) return
    window.speechSynthesis.cancel()
    const utterance = new SpeechSynthesisUtterance(text)
    utterance.lang = 'zh-CN'
    utterance.rate = 1.0
    utterance.onstart = () => {
      setIsPlayingAudio(true)
      setAudioText(text)
    }
    utterance.onend = () => setIsPlayingAudio(false)
    utterance.onerror = () => setIsPlayingAudio(false)
    window.speechSynthesis.speak(utterance)
  }

  const startSimulation = () => {
    setMockLogs([])
    setActiveNodeId(null)
    window.speechSynthesis.cancel()
    const startNode = graph.nodes.find(n => n.type === 'start')
    if (!startNode) {
      message.error('流图中缺失“开始节点”，无法进行仿真！')
      return
    }
    setMockLogs([
      `[系统] 📞 模拟发起呼叫接通，信道 UUID: fs-${Date.now().toString().slice(-6)}`,
      `[系统] ⚙️ mod_audio_stream 接收到 ASR 配置...`,
      `[系统] 🛰️ 连接 WebSocket 旁路推流: ${startNode.metadata?.wsUrl || 'ws://127.0.0.1:9002'} [ mix: ${startNode.metadata?.mixType || 'mono'} / samplingRate: ${startNode.metadata?.sampleRate || '16k'} ]`,
      `[系统] 🧬 发送扩展首帧 Metadata: ${startNode.metadata?.metadata || '{}'}`,
      `[系统] 🟢 WebSocket 信令连接成功，开始实时采集 PCM 旁路语音流。`
    ])
    setActiveNodeId(startNode.id)
    setTimeout(() => {
      const intentNode = graph.nodes.find(n => n.type === 'intent')
      if (intentNode) {
        setActiveNodeId(intentNode.id)
        setMockLogs(prev => [
          ...prev,
          `[云枢 IVR] 🤖 AI 客服已开启 VAD 静音检测断句。请在下方输入说话内容，或者使用拨号盘。`
        ])
        speakText("您好！我是云枢智能应答客服。请问有什么可以帮您？您可以查话费，或者转接人工。")
      }
    }, 1500)
  }

  const handleAsrSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!asrInput.trim()) return
    const query = asrInput.trim()
    setAsrInput('')
    setMockLogs(prev => [
      ...prev,
      `[客户 🗣️] "${query}"`,
      `[ASR / STT 🎙️] VAD 探测静音结束，语音分片识别中... 成功识别文本: "${query}"`
    ])
    if (isPlayingAudio) {
      window.speechSynthesis.cancel()
      setIsPlayingAudio(false)
      setMockLogs(prev => [...prev, `[VAD ⚡] 检测到客户说话，触发 Barge-in 瞬时打断当前 TTS 播放。`])
    }
    setTimeout(() => {
      const intentNode = graph.nodes.find(n => n.type === 'intent')
      if (!intentNode) return
      const outgoingEdges = graph.edges.filter(e => e.source === intentNode.id)
      let matchedEdge = outgoingEdges.find(e => e.sourceHandle && query.includes(e.sourceHandle))
      if (!matchedEdge) {
        if (query.includes('钱') || query.includes('话费') || query.includes('余额')) {
          matchedEdge = outgoingEdges.find(e => e.sourceHandle === '我要查话费')
        } else if (query.includes('客服') || query.includes('人') || query.includes('坐席')) {
          matchedEdge = outgoingEdges.find(e => e.sourceHandle === '我要人工服务')
        }
      }
      if (matchedEdge) {
        setMockLogs(prev => [...prev, `[路由链 🎯] 大模型意图识别成功！命中分支: "${matchedEdge?.sourceHandle}"`])
        const targetNode = graph.nodes.find(n => n.id === matchedEdge?.target)
        if (targetNode) {
          setActiveNodeId(targetNode.id)
          if (targetNode.type === 'reply') {
            setMockLogs(prev => [
              ...prev,
              `[数字人 💬] 触发播报节点: ${targetNode.label}`,
              `[TTS 🔊] 文本转语音合成: "${targetNode.metadata?.text}"`
            ])
            speakText(targetNode.metadata?.text || '')
            setTimeout(() => {
              const nextEdge = graph.edges.find(e => e.source === targetNode.id)
              if (nextEdge) {
                const endNode = graph.nodes.find(n => n.id === nextEdge.target)
                if (endNode) {
                  setActiveNodeId(endNode.id)
                  setMockLogs(prev => [
                    ...prev,
                    `[挂断 ❌] 呼叫已由系统节点正常挂断。`,
                    `[系统] 🛑 mod_audio_stream 物理推流已释放。连接关闭。`
                  ])
                }
              }
            }, 5000)
          } else if (targetNode.type === 'transfer') {
            executeTransfer(targetNode)
          }
        }
      } else {
        setMockLogs(prev => [...prev, `[大模型 🤖] 意图未匹配。触发默认兜底回复。`])
        speakText("对不起，云枢助手没有听懂您的意思。请问您是要查话费还是转接人工客服？")
      }
    }, 800)
  }

  const executeTransfer = (node: AIFlowNode) => {
    setMockLogs(prev => [
      ...prev,
      `[转人工 👤] 触发转接节点: ${node.label} [目标类型: ${node.metadata?.targetType || 'group'} / 号码: ${node.metadata?.targetId || '8001'}]`,
      `[ACD ⚖️] 开始查询在线分机状态 (Mock Redis extension:status)...`
    ])
    setTimeout(() => {
      if (mockAgentOnline) {
        setMockLogs(prev => [
          ...prev,
          `[ACD 🟢] 探测到空闲空载在线分机 ${node.metadata?.targetId || '8001'}。建立桥接。`,
          `[路由链 🎯] 走向 "has_agent" 成功转接分支。`
        ])
        const successEdge = graph.edges.find(e => e.source === node.id && e.sourceHandle === 'has_agent')
        const targetNode = graph.nodes.find(n => n.id === successEdge?.target)
        if (targetNode) {
          setActiveNodeId(targetNode.id)
          setMockLogs(prev => [
            ...prev,
            `[数字人 💬] 触发播报节点: ${targetNode.label}`,
            `[TTS 🔊] 播放成功播报: "${targetNode.metadata?.text}"`
          ])
          speakText(targetNode.metadata?.text || '')
        }
      } else {
        setMockLogs(prev => [
          ...prev,
          `[ACD 🔴] 检测到当前所有在线分机全部离线或在忙状态。`,
          `[排队 ⏳] ACD 将呼叫挂载至排队队列中。`,
          `[排队 ⏳] 排队超时！触发兜底路由。`,
          `[路由链 🎯] 走向 "no_agent" 无客服/超时兜底分支。`
        ])
        const failEdge = graph.edges.find(e => e.source === node.id && e.sourceHandle === 'no_agent')
        const targetNode = graph.nodes.find(n => n.id === failEdge?.target)
        if (targetNode) {
          setActiveNodeId(targetNode.id)
          setMockLogs(prev => [
            ...prev,
            `[数字人 💬] 触发播报节点: ${targetNode.label}`,
            `[TTS 🔊] 播放兜底播报: "${targetNode.metadata?.text}"`
          ])
          speakText(targetNode.metadata?.text || '')
        }
      }
    }, 1200)
  }

  const handleDtmfClick = (digit: string) => {
    try {
      const ctx = new (window.AudioContext || (window as any).webkitAudioContext)()
      const osc1 = ctx.createOscillator()
      const osc2 = ctx.createOscillator()
      const gain = ctx.createGain()
      const dtmfFrequencies: Record<string, [number, number]> = {
        '1': [697, 1209], '2': [697, 1336], '3': [697, 1477],
        '4': [770, 1209], '5': [770, 1336], '6': [770, 1477],
        '7': [852, 1209], '8': [852, 1336], '9': [852, 1477],
        '*': [941, 1209], '0': [941, 1336], '#': [941, 1477]
      }
      const freqs = dtmfFrequencies[digit] || [697, 1209]
      osc1.frequency.value = freqs[0]
      osc2.frequency.value = freqs[1]
      osc1.connect(gain)
      osc2.connect(gain)
      gain.connect(ctx.destination)
      gain.gain.setValueAtTime(0.15, ctx.currentTime)
      osc1.start()
      osc2.start()
      setTimeout(() => {
        osc1.stop()
        osc2.stop()
        ctx.close()
      }, 150)
    } catch (e) {
      console.warn(e)
    }
    setMockLogs(prev => [...prev, `[DTMF 按键 📞] 探测到用户按键输入: "${digit}"`])
    if (activeNodeId === 'node-intent') {
      if (digit === '1') {
        setMockLogs(prev => [...prev, `[按键路由 🎯] 按键 "1" 被捕获，匹配进入查话费路径。`])
        const targetNode = graph.nodes.find(n => n.id === 'node-bill')
        if (targetNode) {
          setActiveNodeId(targetNode.id)
          speakText(targetNode.metadata?.text || '')
        }
      } else if (digit === '2') {
        setMockLogs(prev => [...prev, `[按键路由 🎯] 按键 "2" 被捕获，跳转至宽带迁移/排队人工。`])
        const targetNode = graph.nodes.find(n => n.id === 'node-transfer')
        if (targetNode) {
          executeTransfer(targetNode)
        }
      }
    }
  }

  const edgesSvgPaths = useMemo(() => {
    return graph.edges.map(edge => {
      const start = getPortCoordinates(edge.source, edge.sourceHandle, true)
      const end = getPortCoordinates(edge.target, undefined, false)
      const dx = Math.abs(end.x - start.x) * 0.5
      const path = `M ${start.x} ${start.y} C ${start.x + dx} ${start.y}, ${end.x - dx} ${end.y}, ${end.x} ${end.y}`
      const isHighlighted = edge.source === activeNodeId || edge.target === activeNodeId
      
      // 计算连线中点的物理坐标，用于悬浮渲染删除 `x` 按钮
      const midX = (start.x + end.x) / 2
      const midY = (start.y + end.y) / 2
      
      return { id: edge.id, path, isHighlighted, midX, midY }
    })
  }, [graph, activeNodeId])

  // 处理拉连线过程中的 SVG 跟随曲线绘制 (逻辑坐标系)
  const connectingStringPath = useMemo(() => {
    if (!connectingStart || !canvasRef.current) return null
    const start = getPortCoordinates(connectingStart.nodeId, connectingStart.handle, true)
    const end = mousePos
    const dx = Math.abs(end.x - start.x) * 0.5
    return `M ${start.x} ${start.y} C ${start.x + dx} ${start.y}, ${end.x - dx} ${end.y}, ${end.x} ${end.y}`
  }, [connectingStart, mousePos])

  return (
    <div className="flex flex-col h-screen w-screen bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-100 overflow-hidden font-sans select-none">
      
      {/* 顶部极客控制栏 */}
      <header className="h-[64px] border-b border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 backdrop-blur flex justify-between items-center px-6 relative z-20">
        <Space size="middle">
          <Button
            shape="circle"
            icon={<ArrowLeftOutlined />}
            onClick={() => {
              window.speechSynthesis.cancel()
              navigate('/merchant/ai-model-flow')
            }}
            className="bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-200 flex items-center justify-center"
          />
          <div>
            <div className="flex items-center space-x-2">
              <span className="text-[15px] font-bold text-slate-800 dark:text-slate-200">
                {isNew ? '新建智能语音 IVR 可视化流图' : `流图设计工坊: ${currentFlow?.name || '加载中...'}`}
              </span>
              <Tag color={isNew ? 'gold' : 'green'}>{isNew ? 'New Draft' : 'Production Ready'}</Tag>
            </div>
            <Breadcrumb style={{ fontSize: '11px', color: '#64748b' }}>
              <Breadcrumb.Item>业务管理</Breadcrumb.Item>
              <Breadcrumb.Item>AI 模型流</Breadcrumb.Item>
              <Breadcrumb.Item>设计工坊</Breadcrumb.Item>
            </Breadcrumb>
          </div>
        </Space>
        
        <Space size="middle">
          <Button
            icon={<PlayCircleOutlined />}
            type="default"
            onClick={() => setSandboxOpen(true)}
            className="bg-slate-100 hover:bg-slate-200 dark:bg-slate-900/50 border-slate-200 dark:border-slate-700 text-sky-600 dark:text-sky-400 font-medium"
          >
            仿真测试沙盒
          </Button>
          <Button
            icon={<SaveOutlined />}
            type="primary"
            loading={saveMutation.isPending}
            onClick={() => setIsSaveModalOpen(true)}
            style={{ background: 'linear-gradient(135deg, #0284c7 0%, #0369a1 100%)', border: 'none' }}
          >
            保存并应用流图
          </Button>
        </Space>
      </header>

      {/* 核心三栏：左侧节点选择面板 (Palette) + 中间全屏SVG连线画布 (Canvas) + 右侧属性Inspector */}
      <div className="flex-1 flex overflow-hidden relative">
        
        {/* 1. 左侧功能节点库 Palette Panel (220px) */}
        <div className="w-[220px] bg-white dark:bg-slate-900 border-r border-slate-200 dark:border-slate-800 p-4 flex flex-col z-10 overflow-y-auto">
          <span className="text-slate-800 dark:text-slate-200 font-bold text-[13px] border-b border-slate-150 dark:border-slate-800/80 pb-2 mb-3 block">
            🧩 功能节点库
          </span>
          <p className="text-slate-500 dark:text-slate-400 text-[10px] leading-relaxed mb-4">
            按住左侧功能卡片<strong>拖拽</strong>至右侧画布指定位置放下，或者<strong>双击/直接点击</strong>卡片在画布中实例化。
          </p>
          
          <div className="space-y-3 mt-2">
            
            {/* Start Node */}
            <div
              draggable
              onDragStart={(e) => handlePaletteDragStart(e, 'start')}
              className="p-3 bg-emerald-50/50 dark:bg-emerald-950/30 border border-emerald-100 dark:border-emerald-900/50 hover:border-emerald-500 rounded-xl cursor-grab transition-all flex items-center space-x-3 hover:scale-[1.02] active:scale-[0.98]"
              onClick={() => addPaletteNode('start')}
            >
              <PhoneOutlined style={{ color: '#10b981', fontSize: '16px' }} />
              <div>
                <span className="text-slate-700 dark:text-slate-200 text-xs font-semibold block">开始推流节点</span>
                <div className="text-slate-400 dark:text-slate-500 text-[10px]">旁路复制 ASR</div>
              </div>
            </div>

            {/* Intent Node */}
            <div
              draggable
              onDragStart={(e) => handlePaletteDragStart(e, 'intent')}
              className="p-3 bg-blue-50/50 dark:bg-blue-950/30 border border-blue-100 dark:border-blue-900/50 hover:border-blue-500 rounded-xl cursor-grab transition-all flex items-center space-x-3 hover:scale-[1.02] active:scale-[0.98]"
              onClick={() => addPaletteNode('intent')}
            >
              <CustomerServiceOutlined style={{ color: '#3b82f6', fontSize: '16px' }} />
              <div>
                <span className="text-slate-700 dark:text-slate-200 text-xs font-semibold block">语音意图路由</span>
                <div className="text-slate-400 dark:text-slate-500 text-[10px]">大模型语义 VAD</div>
              </div>
            </div>

            {/* DTMF Node */}
            <div
              draggable
              onDragStart={(e) => handlePaletteDragStart(e, 'dtmf')}
              className="p-3 bg-amber-50/50 dark:bg-amber-950/30 border border-amber-100 dark:border-amber-900/50 hover:border-amber-500 rounded-xl cursor-grab transition-all flex items-center space-x-3 hover:scale-[1.02] active:scale-[0.98]"
              onClick={() => addPaletteNode('dtmf')}
            >
              <SlidersOutlined style={{ color: '#f59e0b', fontSize: '16px' }} />
              <div>
                <span className="text-slate-700 dark:text-slate-200 text-xs font-semibold block">按键收集路由</span>
                <div className="text-slate-400 dark:text-slate-500 text-[10px]">物理数字 DTMF</div>
              </div>
            </div>

            {/* Speech Reply Node */}
            <div
              draggable
              onDragStart={(e) => handlePaletteDragStart(e, 'reply')}
              className="p-3 bg-slate-50 dark:bg-slate-800/40 border border-slate-200 dark:border-slate-700/50 hover:border-slate-500 rounded-xl cursor-grab transition-all flex items-center space-x-3 hover:scale-[1.02] active:scale-[0.98]"
              onClick={() => addPaletteNode('reply')}
            >
              <SoundOutlined style={{ color: '#94a3b8', fontSize: '16px' }} />
              <div>
                <span className="text-slate-700 dark:text-slate-200 text-xs font-semibold block">语音播报播放</span>
                <div className="text-slate-400 dark:text-slate-500 text-[10px]">极速 TTS 缓存</div>
              </div>
            </div>

            {/* ACD Transfer Node */}
            <div
              draggable
              onDragStart={(e) => handlePaletteDragStart(e, 'transfer')}
              className="p-3 bg-amber-50/50 dark:bg-amber-950/30 border border-amber-100 dark:border-amber-900/50 hover:border-amber-500 rounded-xl cursor-grab transition-all flex items-center space-x-3 hover:scale-[1.02] active:scale-[0.98]"
              onClick={() => addPaletteNode('transfer')}
            >
              <NodeIndexOutlined style={{ color: '#f59e0b', fontSize: '16px' }} />
              <div>
                <span className="text-slate-700 dark:text-slate-200 text-xs font-semibold block">ACD 转人工</span>
                <div className="text-slate-400 dark:text-slate-500 text-[10px]">忙时排队状态感知</div>
              </div>
            </div>

            {/* End Node */}
            <div
              draggable
              onDragStart={(e) => handlePaletteDragStart(e, 'end')}
              className="p-3 bg-rose-50/50 dark:bg-rose-950/20 border border-rose-100 dark:border-rose-900/40 hover:border-rose-500 rounded-xl cursor-grab transition-all flex items-center space-x-3 hover:scale-[1.02] active:scale-[0.98]"
              onClick={() => addPaletteNode('end')}
            >
              <InfoCircleOutlined style={{ color: '#ef4444', fontSize: '16px' }} />
              <div>
                <span className="text-slate-700 dark:text-slate-200 text-xs font-semibold block">呼叫挂断结束</span>
                <div className="text-slate-400 dark:text-slate-500 text-[10px]">信道正常释放</div>
              </div>
            </div>

          </div>
        </div>

        {/* 2. 中间大网格连线编辑器画布 Canvas */}
        <div
          ref={canvasRef}
          className={`flex-1 h-full relative overflow-hidden transition-all duration-300 ${
            isDragOverCanvas ? 'ring-2 ring-blue-500 ring-offset-2 ring-offset-slate-950 dark:ring-offset-slate-950 shadow-[inset_0_0_40px_rgba(59,130,246,0.15)] dark:shadow-[inset_0_0_40px_rgba(59,130,246,0.3)]' : ''
          }`}
          style={{
            backgroundSize: '28px 28px',
            backgroundImage: 'radial-gradient(circle, var(--canvas-dots) 1px, transparent 1px)',
            backgroundColor: 'var(--canvas-bg)',
            cursor: isDraggingCanvas ? 'grabbing' : (panMode ? 'grab' : (draggingNodeId ? 'grabbing' : (connectingStart ? 'crosshair' : 'default')))
          }}
          onMouseDown={handleCanvasMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
          onDoubleClick={handleCanvasDoubleClick}
          onDragOver={handleDragOver}
          onDragEnter={handleDragEnter}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
        >
          
          {/* 视口平移与缩放物理变换容器 */}
          <div
            className="absolute inset-0 origin-top-left w-full h-full"
            style={{
              transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
              transition: isDraggingCanvas || draggingNodeId ? 'none' : 'transform 0.08s ease-out'
            }}
          >
            {/* SVG 发光有向拓扑线条与交互连线 */}
            <svg className="absolute inset-0 pointer-events-none w-full h-full" style={{ zIndex: 12 }}>
              <defs>
                <filter id="flowGlow" x="-20%" y="-20%" width="140%" height="140%">
                  <feGaussianBlur stdDeviation="3" result="blur" />
                  <feComposite in="SourceGraphic" in2="blur" operator="over" />
                </filter>
              </defs>
              
              {/* 2.1 渲染已建立的有向贝塞尔曲线 */}
              {edgesSvgPaths.map(edge => {
                const isHovered = hoveredEdgeId === edge.id
                return (
                  <g
                    key={edge.id}
                    className="pointer-events-auto cursor-pointer"
                    onMouseEnter={() => setHoveredEdgeId(edge.id)}
                    onMouseLeave={() => setHoveredEdgeId(null)}
                  >
                    {/* 影子发光轨 */}
                    <path
                      d={edge.path}
                      stroke={edge.isHighlighted ? '#10b981' : isHovered ? '#3b82f6' : 'var(--edge-stroke)'}
                      strokeWidth={edge.isHighlighted ? 6 : isHovered ? 5 : 3}
                      strokeOpacity={edge.isHighlighted ? 0.35 : isHovered ? 0.35 : 0.4}
                      fill="none"
                      className="transition-all duration-150"
                    />
                    {/* 前景实体曲线 */}
                    <path
                      d={edge.path}
                      stroke={edge.isHighlighted ? '#34d399' : isHovered ? '#60a5fa' : 'var(--edge-foreground)'}
                      strokeWidth={edge.isHighlighted ? 3 : 2}
                      fill="none"
                      className="transition-all duration-150"
                    />
                    {/* 宽大的隐形交互触发轨道，用于极大优化鼠标 Hover 体验 */}
                    <path
                      d={edge.path}
                      stroke="transparent"
                      strokeWidth={20}
                      fill="none"
                      className="pointer-events-stroke cursor-pointer"
                    />
                    
                    {/* 连线正中悬浮显示极小红色删除按钮 `x`，支持连线点击删除，仅在 hover 时显现 */}
                    {isHovered && (
                      <g className="transition-opacity duration-200">
                        <circle
                          cx={edge.midX}
                          cy={edge.midY}
                          r="9"
                          fill="var(--bg-container)"
                          stroke="#ef4444"
                          strokeWidth="1.5"
                          className="hover:fill-red-600 transition-colors cursor-pointer pointer-events-auto"
                          onClick={(e) => {
                            e.stopPropagation()
                            deleteEdge(edge.id)
                          }}
                        />
                        <text
                          x={edge.midX}
                          y={edge.midY + 3.5}
                          textAnchor="middle"
                          fill="#ef4444"
                          fontSize="10px"
                          fontWeight="bold"
                          className="hover:fill-slate-100 transition-colors cursor-pointer pointer-events-none"
                        >
                          ×
                        </text>
                      </g>
                    )}

                    {edge.isHighlighted && (
                      <circle r="4.5" fill="#a7f3d0" filter="url(#flowGlow)" className="pointer-events-none">
                        <animateMotion dur="2.2s" repeatCount="indefinite" path={edge.path} />
                      </circle>
                    )}
                  </g>
                )
              })}

              {/* 2.2 实时跟随鼠标光标的拖拽拉线虚线 */}
              {connectingStringPath && (
                <path
                  d={connectingStringPath}
                  stroke="#38bdf8"
                  strokeWidth={2}
                  strokeDasharray="4 4"
                  fill="none"
                />
              )}
            </svg>

            {/* 渲染各具特色的功能节点卡片 */}
            {graph.nodes.map(node => {
              const isActive = node.id === activeNodeId
              const isSelected = node.id === selectedNodeId
              const isHoveredConnect = node.id === hoveredNodeIdDuringConnect
              
              let nodeBorderColor = 'border-slate-200 dark:border-slate-800'
              let nodeIcon = <NodeIndexOutlined />
              let nodeTheme = 'bg-white dark:bg-slate-900/90'
              
              if (isActive) {
                nodeBorderColor = 'border-emerald-500 shadow-[0_0_15px_rgba(16,185,129,0.5)]'
              } else if (isHoveredConnect) {
                nodeBorderColor = 'border-dashed border-sky-400 shadow-[0_0_12px_rgba(56,189,248,0.5)] scale-[1.03]'
              } else if (isSelected) {
                nodeBorderColor = 'border-blue-500 shadow-[0_0_10px_rgba(59,130,246,0.4)]'
              }

              if (node.type === 'start') {
                nodeIcon = <PhoneOutlined style={{ color: '#10b981' }} />
                nodeTheme = 'bg-emerald-50 dark:bg-emerald-950/85 border-emerald-200 dark:border-emerald-500/30 shadow-[inset_0_0_12px_rgba(16,185,129,0.05)] dark:shadow-[inset_0_0_12px_rgba(16,185,129,0.15)]'
              } else if (node.type === 'intent') {
                nodeIcon = <CustomerServiceOutlined style={{ color: '#3b82f6' }} />
                nodeTheme = 'bg-blue-50 dark:bg-blue-950/85 border-blue-200 dark:border-blue-500/30 shadow-[inset_0_0_12px_rgba(59,130,246,0.05)] dark:shadow-[inset_0_0_12px_rgba(59,130,246,0.15)]'
              } else if (node.type === 'transfer') {
                nodeIcon = <NodeIndexOutlined style={{ color: '#f59e0b' }} />
                nodeTheme = 'bg-amber-50 dark:bg-amber-950/85 border-amber-200 dark:border-amber-500/30 shadow-[inset_0_0_12px_rgba(245,158,11,0.05)] dark:shadow-[inset_0_0_12px_rgba(245,158,11,0.15)]'
              } else if (node.type === 'end') {
                nodeIcon = <InfoCircleOutlined style={{ color: '#ef4444' }} />
                nodeTheme = 'bg-rose-50 dark:bg-rose-950/85 border-rose-200 dark:border-rose-500/30 shadow-[inset_0_0_12px_rgba(239,68,68,0.05)] dark:shadow-[inset_0_0_12px_rgba(239,68,68,0.15)]'
              } else if (node.type === 'dtmf') {
                nodeIcon = <SlidersOutlined style={{ color: '#f59e0b' }} />
                nodeTheme = 'bg-amber-50/80 dark:bg-amber-950/80 border-amber-200/60 dark:border-amber-500/25 shadow-[inset_0_0_12px_rgba(245,158,11,0.04)] dark:shadow-[inset_0_0_12px_rgba(245,158,11,0.12)]'
              } else if (node.type === 'reply') {
                nodeIcon = <SoundOutlined style={{ color: '#0284c7' }} />
                nodeTheme = 'bg-white dark:bg-slate-900/85 border-slate-200 dark:border-slate-700/50 shadow-[inset_0_0_12px_rgba(148,163,184,0.04)] dark:shadow-[inset_0_0_12px_rgba(148,163,184,0.12)]'
              }

              // 清理开头的 Emoji 图标，保持标题文字纯净美观，防止与左侧 Badge 的物理 Antd 图标重合
              const cleanLabel = node.label.replace(/^(🏁|🤖|📞|💬|👤|❌)\s*/g, '').trim()

              return (
                <div
                  key={node.id}
                  data-node-id={node.id}
                  className={`absolute flex flex-col justify-between p-3.5 rounded-2xl border w-[210px] h-[72px] transition-all duration-200 backdrop-blur-md z-10 ${nodeBorderColor} ${nodeTheme}`}
                  style={{
                    left: node.x,
                    top: node.y,
                    cursor: draggingNodeId === node.id ? 'grabbing' : (panMode ? 'grab' : (connectingStart ? 'crosshair' : 'grab')),
                    boxShadow: isActive ? '0 0 20px rgba(16,185,129,0.4)' : (isSelected ? '0 0 12px rgba(59,130,246,0.3)' : 'none')
                  }}
                  onMouseDown={(e) => handleNodeMouseDown(node.id, e)}
                  onMouseUp={(e) => handleNodeMouseUp(node.id, e)}
                  onMouseEnter={() => { if (connectingStart) setHoveredNodeIdDuringConnect(node.id) }}
                  onMouseLeave={() => { if (connectingStart) setHoveredNodeIdDuringConnect(null) }}
                >
                  {/* 左侧输入端子 (完美纵向居中 top: 30px，高12px，其物理中心刚好是 36px) */}
                  {node.type !== 'start' && (
                    <div
                      className="absolute w-3 h-3 rounded-full bg-slate-700 border-2 border-slate-900 port-dot hover:scale-125 hover:bg-sky-400 transition-all z-20"
                      style={{ left: '-6px', top: '30px', cursor: 'pointer' }}
                    />
                  )}

                  {/* 极客感左侧圆形 Badge 与右侧文本排版组件 */}
                  <div className="flex items-center space-x-3 w-full pointer-events-none">
                    <div className="w-9 h-9 rounded-full bg-slate-100 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800/80 flex items-center justify-center shadow-inner flex-shrink-0">
                      {nodeIcon}
                    </div>
                    <div className="flex flex-col min-w-0 flex-1">
                      <span className="text-slate-800 dark:text-slate-100 text-[11px] font-bold truncate block">
                        {cleanLabel}
                      </span>
                      <div className="flex justify-between items-center pointer-events-none mt-0.5">
                        <span className="text-[9px] text-slate-500 font-mono tracking-wider">
                          {node.type.toUpperCase()}
                        </span>
                        {isSelected && (
                          <span className="pointer-events-auto flex items-center" onClick={(e) => {
                            e.stopPropagation()
                            deleteNodeAndConnection(node.id)
                          }}>
                            <DeleteOutlined
                              style={{ color: '#ef4444', fontSize: '11px', cursor: 'pointer' }}
                            />
                          </span>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* 右侧输出端子 (按住即可拉线，高度像素级绝对居中或分立 top 15.6px / 44.4px 完美贴合 ports 数值计算) */}
                  {node.type !== 'end' && (
                    <>
                      {node.type === 'transfer' ? (
                        <>
                          <Tooltip title="按住拖拽连线: 有可用坐席">
                            <div
                              className="absolute w-3 h-3 rounded-full bg-emerald-500 border-2 border-slate-900 port-dot hover:scale-125 transition-transform z-20"
                              style={{ right: '-6px', top: '15.6px', cursor: 'crosshair' }}
                              onMouseDown={(e) => handlePortMouseDown(node.id, 'has_agent', e)}
                            />
                          </Tooltip>
                          <Tooltip title="按住拖拽连线: 无在线坐席/超时">
                            <div
                              className="absolute w-3 h-3 rounded-full bg-rose-500 border-2 border-slate-900 port-dot hover:scale-125 transition-transform z-20"
                              style={{ right: '-6px', top: '44.4px', cursor: 'crosshair' }}
                              onMouseDown={(e) => handlePortMouseDown(node.id, 'no_agent', e)}
                            />
                          </Tooltip>
                        </>
                      ) : node.type === 'intent' ? (
                        <>
                          <Tooltip title="按住拖拽连线: 话费意图">
                            <div
                              className="absolute w-3 h-3 rounded-full bg-blue-500 border-2 border-slate-900 port-dot hover:scale-125 transition-transform z-20"
                              style={{ right: '-6px', top: '15.6px', cursor: 'crosshair' }}
                              onMouseDown={(e) => handlePortMouseDown(node.id, '我要查话费', e)}
                            />
                          </Tooltip>
                          <Tooltip title="按住拖拽连线: 人工意图">
                            <div
                              className="absolute w-3 h-3 rounded-full bg-amber-500 border-2 border-slate-900 port-dot hover:scale-125 transition-transform z-20"
                              style={{ right: '-6px', top: '44.4px', cursor: 'crosshair' }}
                              onMouseDown={(e) => handlePortMouseDown(node.id, '我要人工服务', e)}
                            />
                          </Tooltip>
                        </>
                      ) : node.type === 'dtmf' ? (
                        <>
                          <Tooltip title="按住拖拽连线: 按键 1 分支">
                            <div
                              className="absolute w-3 h-3 rounded-full bg-blue-500 border-2 border-slate-900 port-dot hover:scale-125 transition-transform z-20"
                              style={{ right: '-6px', top: '15.6px', cursor: 'crosshair' }}
                              onMouseDown={(e) => handlePortMouseDown(node.id, '1', e)}
                            />
                          </Tooltip>
                          <Tooltip title="按住拖拽连线: 按键 2 分支">
                            <div
                              className="absolute w-3 h-3 rounded-full bg-amber-500 border-2 border-slate-900 port-dot hover:scale-125 transition-transform z-20"
                              style={{ right: '-6px', top: '44.4px', cursor: 'crosshair' }}
                              onMouseDown={(e) => handlePortMouseDown(node.id, '2', e)}
                            />
                          </Tooltip>
                        </>
                      ) : (
                        <div
                          className="absolute w-3 h-3 rounded-full bg-slate-400 border-2 border-slate-900 port-dot hover:scale-125 transition-transform z-20"
                          style={{ right: '-6px', top: '30px', cursor: 'crosshair' }}
                          onMouseDown={(e) => handlePortMouseDown(node.id, undefined, e)}
                        />
                      )}
                    </>
                  )}
                </div>
              )
            })}
          </div>

          {/* 画布高级视口控制工具栏 (Mini Toolbar) */}
          <div className="absolute bottom-6 left-6 z-20 bg-white/95 dark:bg-slate-900/90 backdrop-blur-md border border-slate-200 dark:border-slate-800 px-4 py-2.5 rounded-full flex items-center space-x-3 shadow-lg dark:shadow-2xl">
            
            <Tooltip title="撤销上一步操作 (Ctrl + Z / Cmd + Z)">
              <Button
                type="text"
                disabled={historyIndex <= 0}
                icon={<span>↩️</span>}
                className="text-slate-400 hover:text-slate-200 disabled:text-slate-600 disabled:cursor-not-allowed flex items-center justify-center rounded-full transition-all"
                onClick={undo}
              />
            </Tooltip>
            
            <Tooltip title="重做上一步被撤销的操作 (Ctrl + Shift + Z / Cmd + Shift + Z)">
              <Button
                type="text"
                disabled={historyIndex >= history.length - 1}
                icon={<span>↪️</span>}
                className="text-slate-400 hover:text-slate-200 disabled:text-slate-600 disabled:cursor-not-allowed flex items-center justify-center rounded-full transition-all"
                onClick={redo}
              />
            </Tooltip>
            
            <div className="h-4 w-px bg-slate-200 dark:bg-slate-800" />

            <Tooltip title="按住空格键即可通过鼠标拖拽平移画布">
              <Button
                type="text"
                icon={<span>🖐️</span>}
                className={`!text-[12px] flex items-center justify-center rounded-full transition-all ${
                  panMode ? 'bg-blue-600/40 text-blue-400 font-bold border border-blue-500/50' : 'text-slate-400 hover:text-slate-200'
                }`}
                onClick={() => setPanMode(!panMode)}
              >
                抓取平移 {panMode ? 'ON' : 'OFF'}
              </Button>
            </Tooltip>
            
            <div className="h-4 w-px bg-slate-200 dark:bg-slate-800" />
            
            <Space size={4}>
              <Button
                shape="circle"
                size="small"
                onClick={() => setZoom(z => Math.max(0.4, z - 0.1))}
                className="bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-350 hover:!text-slate-850 dark:hover:!text-slate-100 flex items-center justify-center"
              >-</Button>
              <span className="text-[11px] font-mono text-slate-600 dark:text-slate-450 w-10 text-center font-bold">
                {Math.round(zoom * 100)}%
              </span>
              <Button
                shape="circle"
                size="small"
                onClick={() => setZoom(z => Math.min(2.0, z + 0.1))}
                className="bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-350 hover:!text-slate-850 dark:hover:!text-slate-100 flex items-center justify-center"
              >+</Button>
            </Space>
            
            <div className="h-4 w-px bg-slate-200 dark:bg-slate-800" />
            
            <Tooltip title="还原画布至100%并重置位置">
              <Button
                size="small"
                onClick={() => {
                  setPan({ x: 0, y: 0 })
                  setZoom(1)
                  message.info('🎯 视口已重置为中心 (100%)')
                }}
                className="bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 text-xs border-0 flex items-center justify-center"
              >
                复位
              </Button>
            </Tooltip>

            <Tooltip title="基于拓扑分析，自动整齐布局和理线">
              <Button
                size="small"
                icon={<span>✨</span>}
                onClick={autoLayoutGraph}
                className="bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-800/40 text-emerald-600 dark:text-emerald-400 font-medium text-xs hover:bg-emerald-100 dark:hover:bg-emerald-900/50 flex items-center justify-center"
              >
                智能排版
              </Button>
            </Tooltip>

            <Popconfirm
              title="确定要清空画布中除‘开始’外的所有节点和连线吗？"
              onConfirm={() => {
                const startNode = graph.nodes.find(n => n.type === 'start')
                setGraph({
                  nodes: startNode ? [startNode] : [],
                  edges: []
                })
                setSelectedNodeId(null)
                message.warning('🧹 画布已清空')
              }}
              okText="确定"
              cancelText="取消"
            >
              <Button
                size="small"
                danger
                style={{ fontSize: '11px' }}
              >
                清空
              </Button>
            </Popconfirm>
          </div>

        </div>

        {/* 3. 右侧属性修改 Inspector (360px) */}
        <div className="w-[360px] bg-white dark:bg-slate-900 border-l border-slate-200 dark:border-slate-800 p-5 flex flex-col z-10 overflow-y-auto">
          <span className="text-slate-800 dark:text-slate-100 font-bold border-b border-slate-150 dark:border-slate-800/80 pb-2 mb-4 flex justify-between items-center text-sm">
            <span>⚙️ Node Inspector</span>
            {selectedNodeId && <Tag color="blue">{graph.nodes.find(n => n.id === selectedNodeId)?.type}</Tag>}
          </span>

          {selectedNodeId ? (
            <Form
              form={nodeForm}
              layout="vertical"
              size="small"
              onValuesChange={handleNodeMetadataChange}
              className="flex-1"
            >
              <Form.Item name="label" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">节点显示名称</span>}>
                <Input />
              </Form.Item>

              {graph.nodes.find(n => n.id === selectedNodeId)?.type === 'start' && (
                <>
                  <Form.Item name="asrEnabled" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">开启 ASR 旁路推流</span>} valuePropName="checked">
                    <Switch />
                  </Form.Item>
                  <Form.Item name="asrProvider" label={<span className="text-slate-600 dark:text-sky-400 font-bold text-xs">ASR 识别厂商 (ASR Provider)</span>}>
                    <Select style={{ width: '100%' }} placeholder="默认使用火山语音 ASR">
                      {providers.map((p: any) => (
                        <Select.Option key={p.value} value={p.value} disabled={!p.supportAsr}>
                          {p.emoji} {p.label} {!p.supportAsr && ' ⚠️ (不支持 ASR 识别)'}
                        </Select.Option>
                      ))}
                    </Select>
                  </Form.Item>

                  {/* ⚡ ASR 专属配置动态 Schema 展现 */}
                  <Form.Item noStyle shouldUpdate={(prev, curr) => prev.asrProvider !== curr.asrProvider}>
                    {({ getFieldValue }) => {
                      const provider = getFieldValue('asrProvider') || 'volc';
                      if (provider === 'volc') {
                        return (
                          <div className="bg-slate-50 dark:bg-slate-950 p-3 rounded-lg border border-slate-200/50 dark:border-slate-800/80 mb-3">
                            <span className="text-[10px] font-bold text-slate-400 block mb-2">🌋 火山语音 ASR 专属参数</span>
                            <Form.Item name="volcAppId" label={<span className="text-slate-500 text-[11px]">火山语音 AppId</span>} style={{ marginBottom: '8px' }}>
                              <Input placeholder="输入 Application ID" />
                            </Form.Item>
                            <Form.Item name="volcToken" label={<span className="text-slate-500 text-[11px]">火山语音 Access Token</span>} style={{ marginBottom: '8px' }}>
                              <Input.Password placeholder="输入 OpenSpeech Token" />
                            </Form.Item>
                            <Form.Item name="volcCluster" label={<span className="text-slate-500 text-[11px]">ASR 集群标识 (Cluster)</span>} style={{ marginBottom: '0px' }}>
                              <Input placeholder="默认使用 volc_common_asr" />
                            </Form.Item>
                          </div>
                        );
                      }
                      if (provider === 'ali') {
                        return (
                          <div className="bg-slate-50 dark:bg-slate-950 p-3 rounded-lg border border-slate-200/50 dark:border-slate-800/80 mb-3">
                            <span className="text-[10px] font-bold text-sky-400 block mb-2">☁️ 阿里云一句话识别 ASR 参数</span>
                            <Form.Item name="aliAppKey" label={<span className="text-slate-500 text-[11px]">阿里语音 AppKey</span>} style={{ marginBottom: '8px' }}>
                              <Input placeholder="输入阿里云 NLS AppKey" />
                            </Form.Item>
                            <Form.Item name="aliToken" label={<span className="text-slate-500 text-[11px]">阿里云 Token (NLS Token)</span>} style={{ marginBottom: '0px' }}>
                              <Input.Password placeholder="输入阿里云 NLS 鉴权 Token" />
                            </Form.Item>
                          </div>
                        );
                      }
                      if (provider === 'tencent') {
                        return (
                          <div className="bg-slate-50 dark:bg-slate-950 p-3 rounded-lg border border-slate-200/50 dark:border-slate-800/80 mb-3">
                            <span className="text-[10px] font-bold text-indigo-400 block mb-2">🐧 腾讯云一句话识别 ASR 参数</span>
                            <Form.Item name="tencentSecretId" label={<span className="text-slate-500 text-[11px]">腾讯云 SecretId</span>} style={{ marginBottom: '8px' }}>
                              <Input placeholder="输入腾讯 SecretId" />
                            </Form.Item>
                            <Form.Item name="tencentSecretKey" label={<span className="text-slate-500 text-[11px]">腾讯云 SecretKey</span>} style={{ marginBottom: '0px' }}>
                              <Input.Password placeholder="输入腾讯 SecretKey" />
                            </Form.Item>
                          </div>
                        );
                      }
                      if (provider === 'openai') {
                        return (
                          <div className="bg-slate-50 dark:bg-slate-950 p-3 rounded-lg border border-slate-200/50 dark:border-slate-800/80 mb-3">
                            <span className="text-[10px] font-bold text-emerald-400 block mb-2">🌐 OpenAI Whisper ASR 参数</span>
                            <Form.Item name="llmApiKey" label={<span className="text-slate-500 text-[11px]">OpenAI API Key (与 LLM 共用)</span>} style={{ marginBottom: '0px' }}>
                              <Input.Password placeholder="输入 OpenAI API Key 物理寻路" />
                            </Form.Item>
                          </div>
                        );
                      }
                      return null;
                    }}
                  </Form.Item>
                  <Form.Item name="wsUrl" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">WebSocket 旁路地址</span>}>
                    <Input placeholder="ws://127.0.0.1:9002/asr" />
                  </Form.Item>
                  <Form.Item name="mixType" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">混音模式 (Mix Type)</span>}>
                    <Select style={{ width: '100%' }}>
                      <Select.Option value="mono">mono (单声道-特选 ASR)</Select.Option>
                      <Select.Option value="mixed">mixed (混音双轨)</Select.Option>
                      <Select.Option value="stereo">stereo (立体声分离轨)</Select.Option>
                    </Select>
                  </Form.Item>
                  <Form.Item name="sampleRate" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">采样率 (Sampling Rate)</span>}>
                    <Select style={{ width: '100%' }}>
                      <Select.Option value="16k">16k (16000Hz 特优 ASR)</Select.Option>
                      <Select.Option value="8k">8k (8000Hz 窄带语音)</Select.Option>
                    </Select>
                  </Form.Item>
                  <Form.Item name="metadata" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">首帧元数据 Metadata (JSON)</span>}>
                    <Input.TextArea rows={3} placeholder='{"merchantId": "1001"}' />
                  </Form.Item>

                  {/* ⚡ TTS 专属配置与动态音色 Schema 展现 */}
                  <div className="border-t border-slate-200 dark:border-slate-800/80 my-4 pt-3">
                    <Form.Item name="ttsProvider" label={<span className="text-slate-600 dark:text-sky-400 font-bold text-xs">TTS 合成厂商 (TTS Provider)</span>}>
                      <Select style={{ width: '100%' }} placeholder="默认使用火山语音 TTS">
                        {providers.map((p: any) => (
                          <Select.Option key={p.value} value={p.value} disabled={!p.supportTts}>
                            {p.emoji} {p.label} {!p.supportTts && ' ⚠️ (不支持 TTS 合成)'}
                          </Select.Option>
                        ))}
                      </Select>
                    </Form.Item>

                    <Form.Item noStyle shouldUpdate={(prev, curr) => prev.ttsProvider !== curr.ttsProvider}>
                      {({ getFieldValue }) => {
                        const provider = getFieldValue('ttsProvider') || 'volc';
                        const voices = TTS_VOICES_BY_PROVIDER[provider] || [];

                        if (provider === 'volc') {
                          return (
                            <div className="bg-slate-50 dark:bg-slate-950 p-3 rounded-lg border border-slate-200/50 dark:border-slate-800/80 mb-3">
                              <span className="text-[10px] font-bold text-slate-400 block mb-2">🌋 火山 TTS 参数</span>
                              <Form.Item name="volcAppId" label={<span className="text-slate-500 text-[11px]">火山 AppId (TTS/ASR 共用)</span>} style={{ marginBottom: '8px' }}>
                                <Input placeholder="输入 Application ID" />
                              </Form.Item>
                              <Form.Item name="volcToken" label={<span className="text-slate-500 text-[11px]">火山 Token</span>} style={{ marginBottom: '8px' }}>
                                <Input.Password placeholder="输入 OpenSpeech Token" />
                              </Form.Item>
                              <Form.Item name="volcVoiceType" label={<span className="text-slate-500 text-[11px]">豆包发音人音色</span>} style={{ marginBottom: '8px' }}>
                                <Select style={{ width: '100%' }} placeholder="选择音色">
                                  {voices.map((v: any) => <Select.Option key={v.value} value={v.value}>{v.label}</Select.Option>)}
                                </Select>
                              </Form.Item>
                              <Form.Item name="volcSpeedRatio" label={<span className="text-slate-500 text-[11px]">语速比例</span>} style={{ marginBottom: '0px' }}>
                                <InputNumber min={0.5} max={2.0} step={0.1} style={{ width: '100%' }} />
                              </Form.Item>
                            </div>
                          );
                        }
                        if (provider === 'ali') {
                          return (
                            <div className="bg-slate-50 dark:bg-slate-950 p-3 rounded-lg border border-slate-200/50 dark:border-slate-800/80 mb-3">
                              <span className="text-[10px] font-bold text-sky-400 block mb-2">☁️ 阿里云一句话语音合成 TTS 参数</span>
                              <Form.Item name="aliAppKey" label={<span className="text-slate-500 text-[11px]">阿里语音 AppKey (TTS/ASR 共用)</span>} style={{ marginBottom: '8px' }}>
                                <Input placeholder="输入 AppKey" />
                              </Form.Item>
                              <Form.Item name="aliToken" label={<span className="text-slate-500 text-[11px]">阿里云 Token</span>} style={{ marginBottom: '8px' }}>
                                <Input.Password placeholder="输入 Access Token" />
                              </Form.Item>
                              <Form.Item name="aliVoice" label={<span className="text-slate-500 text-[11px]">阿里特有发音人</span>} style={{ marginBottom: '0px' }}>
                                <Select style={{ width: '100%' }} placeholder="选择阿里发音人">
                                  {voices.map((v: any) => <Select.Option key={v.value} value={v.value}>{v.label}</Select.Option>)}
                                </Select>
                              </Form.Item>
                            </div>
                          );
                        }
                        if (provider === 'tencent') {
                          return (
                            <div className="bg-slate-50 dark:bg-slate-950 p-3 rounded-lg border border-slate-200/50 dark:border-slate-800/80 mb-3">
                              <span className="text-[10px] font-bold text-indigo-400 block mb-2">🐧 腾讯云语音合成 TTS 参数</span>
                              <Form.Item name="tencentSecretId" label={<span className="text-slate-500 text-[11px]">腾讯云 SecretId (TTS/ASR 共用)</span>} style={{ marginBottom: '8px' }}>
                                <Input placeholder="输入 SecretId" />
                              </Form.Item>
                              <Form.Item name="tencentSecretKey" label={<span className="text-slate-500 text-[11px]">腾讯云 SecretKey</span>} style={{ marginBottom: '8px' }}>
                                <Input.Password placeholder="输入 SecretKey" />
                              </Form.Item>
                              <Form.Item name="tencentVoice" label={<span className="text-slate-500 text-[11px]">腾讯特有发音人</span>} style={{ marginBottom: '0px' }}>
                                <Select style={{ width: '100%' }} placeholder="选择腾讯发音人">
                                  {voices.map((v: any) => <Select.Option key={v.value} value={v.value}>{v.label}</Select.Option>)}
                                </Select>
                              </Form.Item>
                            </div>
                          );
                        }
                        if (provider === 'openai') {
                          return (
                            <div className="bg-slate-50 dark:bg-slate-950 p-3 rounded-lg border border-slate-200/50 dark:border-slate-800/80 mb-3">
                              <span className="text-[10px] font-bold text-emerald-400 block mb-2">🌐 OpenAI 高保真 TTS 参数</span>
                              <Form.Item name="llmApiKey" label={<span className="text-slate-500 text-[11px]">OpenAI API Key (与 LLM 共用)</span>} style={{ marginBottom: '8px' }}>
                                <Input.Password placeholder="输入 OpenAI API Key" />
                              </Form.Item>
                              <Form.Item name="openaiVoice" label={<span className="text-slate-500 text-[11px]">OpenAI 特有音色</span>} style={{ marginBottom: '0px' }}>
                                <Select style={{ width: '100%' }} placeholder="选择 OpenAI 音色">
                                  {voices.map((v: any) => <Select.Option key={v.value} value={v.value}>{v.label}</Select.Option>)}
                                </Select>
                              </Form.Item>
                            </div>
                          );
                        }
                        return null;
                      }}
                    </Form.Item>
                  </div>

                  <div className="border-t border-slate-200 dark:border-slate-800/80 my-4 pt-3">
                    <span className="text-slate-800 dark:text-slate-200 font-bold text-xs block mb-3">🧠 AI 大模型全局配置 (LLM)</span>
                    <Form.Item label={<span className="text-sky-600 dark:text-sky-400 font-bold text-xs">⚡ 快捷选择已配置的 AI 模型</span>}>
                      <Select
                        placeholder="点击选择已有的模型配置自动填充"
                        style={{ width: '100%' }}
                        onChange={(configId) => {
                          const conf = configsData?.find((c: any) => c.id === configId)
                          if (conf) {
                            const patch = {
                              llmProvider: (() => {
                                const p = String(conf.provider || '').toLowerCase();
                                if (p === 'deepseek') return 'deepseek';
                                if (p === 'openai') return 'openai';
                                if (p === 'ali') return 'ali';
                                if (p === 'tencent') return 'tencent';
                                if (p === 'volc') return 'volc';
                                if (p === 'mock' || p === 'cloud枢私有大模型') return 'mock';
                                return p;
                              })(),
                              llmModel: conf.modelName,
                              llmEndpoint: conf.endpoint,
                              llmApiKey: conf.apiKey,
                              llmTemperature: conf.temperature,
                              llmSystemPrompt: conf.systemPrompt,
                              volcAppId: conf.volcAppId,
                              volcToken: conf.volcToken,
                              volcCluster: conf.volcCluster,
                              volcVoiceType: conf.volcVoiceType,
                              volcSpeedRatio: conf.volcSpeedRatio,
                              aliAppKey: conf.volcAppId,
                              aliToken: conf.volcToken,
                              aliVoice: conf.volcVoiceType || "Xiaoyun",
                              tencentSecretId: conf.volcAppId,
                              tencentSecretKey: conf.volcToken,
                              tencentVoice: conf.volcVoiceType || "101001",
                              openaiVoice: conf.volcVoiceType || "alloy",
                              asrProvider: conf.volcAppId ? "volc" : "mock",
                              ttsProvider: conf.volcAppId ? "volc" : "mock"
                            }
                            nodeForm.setFieldsValue(patch)
                            handleNodeMetadataChange(patch)
                            message.success(`已成功应用大模型配置「${conf.name}」`)
                          }
                        }}
                      >
                        {configsData?.map((c: any) => (
                          <Select.Option key={c.id} value={c.id}>
                            🧠 {c.name} ({c.provider} / {c.modelName})
                          </Select.Option>
                        ))}
                      </Select>
                    </Form.Item>
                    <Form.Item name="llmProvider" label={<span className="text-slate-600 dark:text-sky-400 font-bold text-xs">大模型服务商 (LLM Provider)</span>}>
                      <Select style={{ width: '100%' }}>
                        {providers.map((p: any) => (
                          <Select.Option key={p.value} value={p.value} disabled={!p.supportLlm}>
                            {p.emoji} {p.label} {!p.supportLlm && ' ⚠️ (不支持大模型决断)'}
                          </Select.Option>
                        ))}
                      </Select>
                    </Form.Item>
                    <Form.Item name="llmModel" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">大模型名称 (Model Name)</span>}>
                      <Input placeholder="例如: deepseek-chat 或 gpt-4o" />
                    </Form.Item>
                    <Form.Item name="llmEndpoint" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">API 代理网关地址 (Endpoint)</span>}>
                      <Input placeholder="空代表使用服务商默认 Endpoint" />
                    </Form.Item>
                    <Form.Item name="llmApiKey" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">API 密钥 (API Key)</span>}>
                      <Input.Password placeholder="输入大模型 API Key 建立物理交互" />
                    </Form.Item>
                    <Form.Item name="llmTemperature" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">生成温度 (Temperature)</span>}>
                      <Slider min={0.0} max={1.5} step={0.1} defaultValue={0.7} />
                    </Form.Item>
                    <Form.Item name="llmSystemPrompt" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">全局 System Prompt 角色提示词</span>}>
                      <Input.TextArea rows={4} placeholder="您是云枢智能话务员，请根据用户的提问礼貌回答..." />
                    </Form.Item>
                  </div>
                </>
              )}

              {graph.nodes.find(n => n.id === selectedNodeId)?.type === 'reply' && (
                <>
                  <Form.Item name="text" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">TTS 朗读文本</span>}>
                    <Input.TextArea rows={5} placeholder="输入数字客服播报的话术..." />
                  </Form.Item>
                  <Form.Item name="ttsVoice" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">智能发音人</span>}>
                    <Select style={{ width: '100%' }}>
                      <Select.Option value="晓雅">晓雅 (🎤 甜美客服女声)</Select.Option>
                      <Select.Option value="阿强">阿强 (🎙️ 专业播音男声)</Select.Option>
                    </Select>
                  </Form.Item>
                  <Form.Item label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">朗读语速</span>}>
                    <Slider min={0.5} max={2.0} step={0.1} defaultValue={1.0} />
                  </Form.Item>
                  <Button
                    type="dashed"
                    block
                    icon={<PlayCircleOutlined />}
                    onClick={() => speakText(nodeForm.getFieldValue('text'))}
                    style={{ color: '#38bdf8', borderColor: '#0284c7' }}
                  >
                    合成并试听
                  </Button>
                </>
              )}

              {graph.nodes.find(n => n.id === selectedNodeId)?.type === 'transfer' && (
                <>
                  <Form.Item name="targetType" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">转接目标类型</span>}>
                    <Select style={{ width: '100%' }}>
                      <Select.Option value="group">技能组 (Skill Group)</Select.Option>
                      <Select.Option value="extension">分机 (Extension)</Select.Option>
                    </Select>
                  </Form.Item>
                  <Form.Item name="targetId" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">目标 ID / 分机号</span>}>
                    <Input placeholder="例如: 8001" />
                  </Form.Item>
                  <Form.Item name="enableQueue" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">开启排队等待</span>} valuePropName="checked">
                    <Switch />
                  </Form.Item>
                  <Form.Item name="maxQueueTime" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">最大排队等待时长 (秒)</span>}>
                    <InputNumber min={5} max={300} style={{ width: '100%' }} />
                  </Form.Item>
                  <Form.Item name="mohFile" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">排队背景音 (MOH)</span>}>
                    <Input placeholder="standard_moh.wav" />
                  </Form.Item>
                </>
              )}

              {graph.nodes.find(n => n.id === selectedNodeId)?.type === 'dtmf' && (
                <>
                  <Form.Item name="maxDigits" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">收集最大按键位数</span>}>
                    <InputNumber min={1} max={32} style={{ width: '100%' }} />
                  </Form.Item>
                  <Form.Item name="timeout" label={<span className="text-slate-600 dark:text-slate-400 font-semibold text-xs">等待按键超时 (秒)</span>}>
                    <InputNumber min={3} max={60} style={{ width: '100%' }} />
                  </Form.Item>
                </>
              )}

              {graph.nodes.find(n => n.id === selectedNodeId)?.type === 'intent' && (
                <div className="bg-slate-50 dark:bg-slate-950 p-3.5 rounded-xl border border-slate-200 dark:border-slate-800">
                  <span className="text-slate-700 dark:text-slate-350 text-[11px] block mb-2 leading-relaxed font-semibold">
                    意图路由根据 ASR 文本进行大模型智能匹配，连线到特定功能节点。
                  </span>
                  <p className="text-slate-500 dark:text-slate-500 text-[10px] leading-relaxed">
                    当前已在流图中配置的分支：<br />
                    - 分支1: <strong>我要查话费</strong><br />
                    - 分支2: <strong>我要人工服务</strong>
                  </p>
                </div>
              )}

              {graph.nodes.find(n => n.id === selectedNodeId)?.type === 'end' && (
                <div className="bg-slate-50 dark:bg-slate-950 p-3.5 rounded-xl border border-slate-200 dark:border-slate-800">
                  <span className="text-slate-700 dark:text-slate-350 text-[11px] block leading-relaxed font-semibold">
                    挂断呼叫节点为话务终点，执行后将物理释放 FreeSWITCH 通话信道。
                  </span>
                </div>
              )}
              
              {graph.nodes.find(n => n.id === selectedNodeId)?.type === 'condition' && (
                <div className="bg-slate-50 dark:bg-slate-950 p-3.5 rounded-xl border border-slate-200 dark:border-slate-800">
                  <span className="text-slate-700 dark:text-slate-350 text-[11px] block leading-relaxed font-semibold">
                    条件判断节点。依据系统配置规则条件执行逻辑分支流转。
                  </span>
                </div>
              )}
            </Form>
          ) : (
            <div className="flex-1 flex flex-col items-center justify-center text-slate-500 py-12">
              <NodeIndexOutlined style={{ fontSize: '32px', color: '#475569', marginBottom: '12px' }} />
              <span className="text-slate-400 dark:text-slate-500 text-xs">双击或选中画布节点查看属性</span>
            </div>
          )}
        </div>

      </div>

      {/* 4. 仿真测试沙盒 (Sandbox Drawer) */}
      <Drawer
        title="云枢 IVR 智能话术仿真沙盒"
        placement="right"
        width={400}
        onClose={() => {
          window.speechSynthesis.cancel()
          setIsPlayingAudio(false)
          setSandboxOpen(false)
        }}
        open={sandboxOpen}
        bodyStyle={{ backgroundColor: '#070a13', color: '#cbd5e1', padding: '16px' }}
      >
        <div className="flex flex-col h-full space-y-4">
          <div className="flex justify-between items-center bg-slate-900 p-3 rounded-lg border border-slate-800">
            <div>
              <span className="text-[12px] text-slate-300 font-semibold block">分机在线状态 (ACD Mock)</span>
              <span className="text-[10px] text-slate-500">模拟 Redis `extension:status` 状态</span>
            </div>
            <Switch
              checkedChildren="在线"
              unCheckedChildren="离线"
              checked={mockAgentOnline}
              onChange={setMockAgentOnline}
            />
          </div>

          <div className="flex-1 bg-black/50 border border-slate-800 rounded-lg p-3 font-mono text-[11px] overflow-y-auto space-y-2">
            {mockLogs.map((log, index) => (
              <div key={index} className={`whitespace-pre-wrap ${
                log.startsWith('[客户') ? 'text-blue-400' :
                log.startsWith('[ASR') ? 'text-sky-400' :
                log.startsWith('[数字人') ? 'text-emerald-400 font-bold' :
                log.startsWith('[系统') ? 'text-slate-500' : 'text-slate-300'
              }`}>
                {log}
              </div>
            ))}
          </div>

          {isPlayingAudio && (
            <div className="bg-emerald-950/20 border border-emerald-500/20 p-3 rounded-lg flex items-center space-x-3">
              <div className="flex space-x-1">
                <span className="w-1 h-4 bg-emerald-500 animate-bounce" />
                <span className="w-1 h-4 bg-emerald-500 animate-bounce delay-75" />
                <span className="w-1 h-4 bg-emerald-500 animate-bounce delay-150" />
              </div>
              <div className="flex-1 min-w-0">
                <span className="text-[10px] text-emerald-400 block font-semibold">TTS 语音播放中...</span>
                <span className="text-[11px] text-slate-300 truncate block">{audioText}</span>
              </div>
            </div>
          )}

          <Form onSubmitCapture={handleAsrSubmit} className="flex space-x-2">
            <Input
              value={asrInput}
              onChange={(e) => setAsrInput(e.target.value)}
              placeholder="输入客户说话的内容..."
              className="flex-1 bg-slate-900 border-slate-800 text-slate-200"
            />
            <Button
              type="primary"
              htmlType="submit"
              disabled={mockLogs.length === 0}
              icon={<SendOutlined />}
            />
          </Form>

          <div className="grid grid-cols-3 gap-2">
            {['1', '2', '3', '4', '5', '6', '7', '8', '9', '*', '0', '#'].map(digit => (
              <Button
                key={digit}
                onClick={() => handleDtmfClick(digit)}
                style={{ backgroundColor: '#1e293b', borderColor: '#334155', color: '#cbd5e1' }}
              >
                {digit}
              </Button>
            ))}
          </div>

          <div className="flex space-x-2">
            <Button
              type="primary"
              danger
              block
              onClick={() => {
                window.speechSynthesis.cancel()
                setIsPlayingAudio(false)
                setMockLogs([])
              }}
            >
              重置挂断
            </Button>
            <Button
              type="primary"
              block
              onClick={startSimulation}
            >
              模拟起呼接通
            </Button>
          </div>
        </div>
      </Drawer>
      <Modal
        title="💾 保存并应用智能语音 IVR 流图"
        open={isSaveModalOpen}
        onCancel={() => setIsSaveModalOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        okText="确认保存并上线"
        cancelText="取消"
        destroyOnHidden
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => {
            saveMutation.mutate(values)
            setIsSaveModalOpen(false)
          }}
          className="mt-4"
        >
          <Form.Item
            name="name"
            label="话术流程名称"
            rules={[{ required: true, message: '请输入话术流程名称，便于列表识别' }]}
          >
            <Input placeholder="例如: 智能业务导航、话费查询客服" />
          </Form.Item>

          <Form.Item
            name="prompt"
            label="触发 Prompt 提示词"
            rules={[{ required: true, message: '请输入全局触发提示词' }]}
          >
            <Input.TextArea rows={4} placeholder="例如: 你是一个智能客服机器人，根据用户说的话导航业务..." />
          </Form.Item>

          <Form.Item
            name="description"
            label="备注描述"
          >
            <Input placeholder="可在此输入话术备注" />
          </Form.Item>
        </Form>
      </Modal>

    </div>
  )
}