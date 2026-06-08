import { Button, Space, Typography, message, Modal, Form, Input, Select, Table, Popconfirm, Tabs, InputNumber, Slider, Tag, Tooltip } from 'antd'
import { PlusOutlined, DeleteOutlined, EditOutlined, SearchOutlined, ReloadOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { TableWrap } from '@/components/TableWrap'

import {
  fetchKnowledgeBases,
  saveKnowledgeBase,
  deleteKnowledgeBase,
  fetchKnowledgeBaseDocuments,
  saveKnowledgeBaseDocument,
  deleteKnowledgeBaseDocument,
  searchKnowledgeBase,
  fetchASRConfigs,
  saveASRConfig,
  deleteASRConfig,
  fetchTTSConfigs,
  saveTTSConfig,
  deleteTTSConfig,
  KnowledgeBase,
  KnowledgeBaseDocument,
  ASRConfig,
  TTSConfig
} from '@/api/operate'

const { Title, Text, Paragraph } = Typography
const { TextArea } = Input
const { Option } = Select

export function KnowledgeBasePage() {
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState('kb')
  const [selectedKB, setSelectedKB] = useState<KnowledgeBase | null>(null)
  const [kbModalVisible, setKbModalVisible] = useState(false)
  const [docModalVisible, setDocModalVisible] = useState(false)
  const [asrModalVisible, setAsrModalVisible] = useState(false)
  const [ttsModalVisible, setTtsModalVisible] = useState(false)
  const [editingKB, setEditingKB] = useState<KnowledgeBase | null>(null)
  const [editingDoc, setEditingDoc] = useState<KnowledgeBaseDocument | null>(null)
  const [editingASR, setEditingASR] = useState<ASRConfig | null>(null)
  const [editingTTS, setEditingTTS] = useState<TTSConfig | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<any[]>([])
  const [searching, setSearching] = useState(false)
  const [kbForm] = Form.useForm()
  const [docForm] = Form.useForm()
  const [asrForm] = Form.useForm()
  const [ttsForm] = Form.useForm()

  // 获取知识库列表
  const { data: kbList, isLoading: kbLoading } = useQuery({
    queryKey: ['knowledge-bases'],
    queryFn: fetchKnowledgeBases
  })

  // 获取文档列表
  const { data: docList, isLoading: docLoading } = useQuery({
    queryKey: ['kb-documents', selectedKB?.id],
    queryFn: () => selectedKB ? fetchKnowledgeBaseDocuments(selectedKB.id) : Promise.resolve([]),
    enabled: !!selectedKB
  })

  // 获取 ASR 配置列表
  const { data: asrList, isLoading: asrLoading } = useQuery({
    queryKey: ['asr-configs'],
    queryFn: fetchASRConfigs
  })

  // 获取 TTS 配置列表
  const { data: ttsList, isLoading: ttsLoading } = useQuery({
    queryKey: ['tts-configs'],
    queryFn: fetchTTSConfigs
  })

  // 保存知识库
  const saveKBMutation = useMutation({
    mutationFn: saveKnowledgeBase,
    onSuccess: () => {
      message.success('知识库保存成功')
      setKbModalVisible(false)
      kbForm.resetFields()
      setEditingKB(null)
      queryClient.invalidateQueries({ queryKey: ['knowledge-bases'] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '保存失败')
    }
  })

  // 删除知识库
  const deleteKBMutation = useMutation({
    mutationFn: deleteKnowledgeBase,
    onSuccess: () => {
      message.success('知识库删除成功')
      setSelectedKB(null)
      queryClient.invalidateQueries({ queryKey: ['knowledge-bases'] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '删除失败')
    }
  })

  // 保存文档
  const saveDocMutation = useMutation({
    mutationFn: saveKnowledgeBaseDocument,
    onSuccess: () => {
      message.success('文档保存成功')
      setDocModalVisible(false)
      docForm.resetFields()
      setEditingDoc(null)
      queryClient.invalidateQueries({ queryKey: ['kb-documents', selectedKB?.id] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '保存失败')
    }
  })

  // 删除文档
  const deleteDocMutation = useMutation({
    mutationFn: ({ kbId, docId }: { kbId: string, docId: string }) => deleteKnowledgeBaseDocument(kbId, docId),
    onSuccess: () => {
      message.success('文档删除成功')
      queryClient.invalidateQueries({ queryKey: ['kb-documents', selectedKB?.id] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '删除失败')
    }
  })

  // 保存 ASR 配置
  const saveASRMutation = useMutation({
    mutationFn: saveASRConfig,
    onSuccess: () => {
      message.success('ASR 配置保存成功')
      setAsrModalVisible(false)
      asrForm.resetFields()
      setEditingASR(null)
      queryClient.invalidateQueries({ queryKey: ['asr-configs'] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '保存失败')
    }
  })

  // 删除 ASR 配置
  const deleteASRMutation = useMutation({
    mutationFn: deleteASRConfig,
    onSuccess: () => {
      message.success('ASR 配置删除成功')
      queryClient.invalidateQueries({ queryKey: ['asr-configs'] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '删除失败')
    }
  })

  // 保存 TTS 配置
  const saveTTSMutation = useMutation({
    mutationFn: saveTTSConfig,
    onSuccess: () => {
      message.success('TTS 配置保存成功')
      setTtsModalVisible(false)
      ttsForm.resetFields()
      setEditingTTS(null)
      queryClient.invalidateQueries({ queryKey: ['tts-configs'] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '保存失败')
    }
  })

  // 删除 TTS 配置
  const deleteTTSMutation = useMutation({
    mutationFn: deleteTTSConfig,
    onSuccess: () => {
      message.success('TTS 配置删除成功')
      queryClient.invalidateQueries({ queryKey: ['tts-configs'] })
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '删除失败')
    }
  })

  // 搜索知识库
  const handleSearch = async () => {
    if (!searchQuery.trim() || !selectedKB) {
      message.warning('请选择知识库并输入搜索内容')
      return
    }

    setSearching(true)
    try {
      const results = await searchKnowledgeBase(selectedKB.id!, searchQuery)
      setSearchResults(results)
    } catch (error) {
      message.error('搜索失败')
    } finally {
      setSearching(false)
    }
  }

  // 打开新建知识库模态框
  const handleNewKB = () => {
    setEditingKB(null)
    kbForm.resetFields()
    setKbModalVisible(true)
  }

  // 打开编辑知识库模态框
  const handleEditKB = (kb: KnowledgeBase) => {
    setEditingKB(kb)
    kbForm.setFieldsValue(kb)
    setKbModalVisible(true)
  }

  // 打开新建文档模态框
  const handleNewDoc = () => {
    if (!selectedKB) {
      message.warning('请先选择知识库')
      return
    }
    setEditingDoc(null)
    docForm.resetFields()
    setDocModalVisible(true)
  }

  // 打开编辑文档模态框
  const handleEditDoc = (doc: KnowledgeBaseDocument) => {
    setEditingDoc(doc)
    docForm.setFieldsValue(doc)
    setDocModalVisible(true)
  }

  // 打开新建 ASR 配置模态框
  const handleNewASR = () => {
    setEditingASR(null)
    asrForm.resetFields()
    setAsrModalVisible(true)
  }

  // 打开编辑 ASR 配置模态框
  const handleEditASR = (asr: ASRConfig) => {
    setEditingASR(asr)
    asrForm.setFieldsValue(asr)
    setAsrModalVisible(true)
  }

  // 打开新建 TTS 配置模态框
  const handleNewTTS = () => {
    setEditingTTS(null)
    ttsForm.resetFields()
    setTtsModalVisible(true)
  }

  // 打开编辑 TTS 配置模态框
  const handleEditTTS = (tts: TTSConfig) => {
    setEditingTTS(tts)
    ttsForm.setFieldsValue(tts)
    setTtsModalVisible(true)
  }

  // 知识库表格列
  const kbColumns = [
    {
      title: '知识库名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (text: string) => text ? new Date(text).toLocaleString() : '-',
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      render: (text: string) => text ? new Date(text).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: any, record: KnowledgeBase) => (
        <Space size="small">
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEditKB(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个知识库吗？"
            description="删除后将无法恢复"
            onConfirm={() => deleteKBMutation.mutate(record.id!)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              danger
              icon={<DeleteOutlined />}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // 文档表格列
  const docColumns = [
    {
      title: '文档标题',
      dataIndex: 'title',
      key: 'title',
    },
    {
      title: '内容',
      dataIndex: 'content',
      key: 'content',
      ellipsis: true,
      render: (text: string) => (
        <Tooltip title={text}>
          <span>{text?.substring(0, 100)}{text?.length > 100 ? '...' : ''}</span>
        </Tooltip>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (text: string) => text ? new Date(text).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: any, record: KnowledgeBaseDocument) => (
        <Space size="small">
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEditDoc(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个文档吗？"
            description="删除后将无法恢复"
            onConfirm={() => deleteDocMutation.mutate({ kbId: selectedKB!.id!, docId: record.id! })}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              danger
              icon={<DeleteOutlined />}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // ASR 配置表格列
  const asrColumns = [
    {
      title: '配置名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '服务提供商',
      dataIndex: 'provider',
      key: 'provider',
      render: (provider: string) => {
        const providers: Record<string, string> = {
          'volc': '火山引擎',
          'ali': '阿里云',
          'tencent': '腾讯云',
          'openai': 'OpenAI',
          'mock': 'Mock 测试'
        }
        return providers[provider] || provider
      }
    },
    {
      title: '语言',
      dataIndex: 'language',
      key: 'language',
      render: (lang: string) => lang || '中文(zh-CN)',
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'green' : 'red'}>
          {enabled ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: any, record: ASRConfig) => (
        <Space size="small">
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEditASR(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个配置吗？"
            description="删除后将无法恢复"
            onConfirm={() => deleteASRMutation.mutate(record.id!)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              danger
              icon={<DeleteOutlined />}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // TTS 配置表格列
  const ttsColumns = [
    {
      title: '配置名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '服务提供商',
      dataIndex: 'provider',
      key: 'provider',
      render: (provider: string) => {
        const providers: Record<string, string> = {
          'volc': '火山引擎',
          'ali': '阿里云',
          'tencent': '腾讯云',
          'openai': 'OpenAI',
          'mock': 'Mock 测试'
        }
        return providers[provider] || provider
      }
    },
    {
      title: '音色',
      dataIndex: 'voiceType',
      key: 'voiceType',
    },
    {
      title: '语速',
      dataIndex: 'speed',
      key: 'speed',
      render: (speed: number) => speed || 1.0,
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'green' : 'red'}>
          {enabled ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: any, record: TTSConfig) => (
        <Space size="small">
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEditTTS(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个配置吗？"
            description="删除后将无法恢复"
            onConfirm={() => deleteTTSMutation.mutate(record.id!)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              danger
              icon={<DeleteOutlined />}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  // 搜索结果表格列
  const searchResultColumns = [
    {
      title: '相关度',
      dataIndex: 'score',
      key: 'score',
      render: (score: number) => (
        <Tag color={score > 0.9 ? 'green' : score > 0.8 ? 'blue' : 'orange'}>
          {(score * 100).toFixed(1)}%
        </Tag>
      ),
    },
    {
      title: '内容',
      dataIndex: 'text',
      key: 'text',
      ellipsis: true,
    },
  ]

  const tabItems = [
    {
      key: 'kb',
      label: '向量知识库',
      children: (
        <Space direction="vertical" className="w-full" size="large">
          {/* 知识库列表 */}
          <div className="bg-white dark:bg-slate-800 p-6 rounded-2xl">
            <div className="flex justify-between items-center mb-4">
              <Title level={4} style={{ margin: 0 }}>知识库管理</Title>
              <Space>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={() => queryClient.invalidateQueries({ queryKey: ['knowledge-bases'] })}
                >
                  刷新
                </Button>
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={handleNewKB}
                >
                  新建知识库
                </Button>
              </Space>
            </div>
            <TableWrap
              title="知识库列表"
              rowKey="id"
              loading={kbLoading}
              dataSource={kbList || []}
              columns={kbColumns}
              pagination={false}
              rowSelection={{
                type: 'radio',
                selectedRowKeys: selectedKB ? [selectedKB.id] : [],
                onChange: (keys) => {
                  const selected = kbList?.find(kb => kb.id === keys[0])
                  setSelectedKB(selected || null)
                },
              }}
            />
          </div>

          {/* 文档管理 */}
          {selectedKB && (
            <div className="bg-white dark:bg-slate-800 p-6 rounded-2xl">
              <div className="flex justify-between items-center mb-4">
                <Title level={4} style={{ margin: 0 }}>文档管理 - {selectedKB.name}</Title>
                <Space>
                  <Button
                    icon={<ReloadOutlined />}
                    onClick={() => queryClient.invalidateQueries({ queryKey: ['kb-documents', selectedKB.id] })}
                  >
                    刷新
                  </Button>
                  <Button
                    type="primary"
                    icon={<PlusOutlined />}
                    onClick={handleNewDoc}
                  >
                    添加文档
                  </Button>
                </Space>
              </div>

              {/* 搜索区域 */}
              <div className="mb-4 p-4 bg-slate-50 dark:bg-slate-700 rounded-xl">
                <div className="flex gap-2">
                  <Input
                    placeholder="输入搜索内容..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    onPressEnter={handleSearch}
                    prefix={<SearchOutlined />}
                    style={{ flex: 1 }}
                  />
                  <Button
                    type="primary"
                    icon={<SearchOutlined />}
                    onClick={handleSearch}
                    loading={searching}
                  >
                    搜索
                  </Button>
                </div>

                {/* 搜索结果 */}
                {searchResults.length > 0 && (
                  <div className="mt-4">
                    <Title level={5}>搜索结果</Title>
                    <Table
                      dataSource={searchResults}
                      columns={searchResultColumns}
                      rowKey="id"
                      pagination={false}
                      size="small"
                    />
                  </div>
                )}
              </div>

              <TableWrap
                title="文档列表"
                rowKey="id"
                loading={docLoading}
                dataSource={docList || []}
                columns={docColumns}
                pagination={false}
              />
            </div>
          )}
        </Space>
      ),
    },
    {
      key: 'asr',
      label: 'ASR 配置',
      children: (
        <div className="bg-white dark:bg-slate-800 p-6 rounded-2xl">
          <div className="flex justify-between items-center mb-4">
            <Title level={4} style={{ margin: 0 }}>语音识别(ASR)配置</Title>
            <Space>
              <Button
                icon={<ReloadOutlined />}
                onClick={() => queryClient.invalidateQueries({ queryKey: ['asr-configs'] })}
              >
                刷新
              </Button>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={handleNewASR}
              >
                新增配置
              </Button>
            </Space>
          </div>
          <TableWrap
            title="ASR 配置列表"
            rowKey="id"
            loading={asrLoading}
            dataSource={asrList || []}
            columns={asrColumns}
            pagination={false}
          />
        </div>
      ),
    },
    {
      key: 'tts',
      label: 'TTS 配置',
      children: (
        <div className="bg-white dark:bg-slate-800 p-6 rounded-2xl">
          <div className="flex justify-between items-center mb-4">
            <Title level={4} style={{ margin: 0 }}>语音合成(TTS)配置</Title>
            <Space>
              <Button
                icon={<ReloadOutlined />}
                onClick={() => queryClient.invalidateQueries({ queryKey: ['tts-configs'] })}
              >
                刷新
              </Button>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={handleNewTTS}
              >
                新增配置
              </Button>
            </Space>
          </div>
          <TableWrap
            title="TTS 配置列表"
            rowKey="id"
            loading={ttsLoading}
            dataSource={ttsList || []}
            columns={ttsColumns}
            pagination={false}
          />
        </div>
      ),
    },
  ]

  return (
    <Space direction="vertical" className="w-full" size="large">
      <div className="bg-white dark:bg-slate-800 p-6 rounded-2xl">
        <Title level={3}>AI 能力配置</Title>
        <Paragraph type="secondary">
          配置向量知识库、语音识别(ASR)和语音合成(TTS)服务，用于减少大模型 token 消耗，提升 AI 通话体验。
        </Paragraph>
      </div>

      <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} />

      {/* 知识库编辑模态框 */}
      <Modal
        title={editingKB ? '编辑知识库' : '新建知识库'}
        open={kbModalVisible}
        onCancel={() => {
          setKbModalVisible(false)
          kbForm.resetFields()
          setEditingKB(null)
        }}
        footer={null}
      >
        <Form
          form={kbForm}
          layout="vertical"
          onFinish={(values) => saveKBMutation.mutate({ ...editingKB, ...values })}
        >
          <Form.Item
            name="name"
            label="知识库名称"
            rules={[{ required: true, message: '请输入知识库名称' }]}
          >
            <Input placeholder="请输入知识库名称" />
          </Form.Item>
          <Form.Item
            name="description"
            label="描述"
          >
            <TextArea rows={4} placeholder="请输入知识库描述" />
          </Form.Item>
          <Form.Item className="mb-0 text-right">
            <Space>
              <Button onClick={() => setKbModalVisible(false)}>取消</Button>
              <Button
                type="primary"
                htmlType="submit"
                loading={saveKBMutation.isPending}
              >
                保存
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 文档编辑模态框 */}
      <Modal
        title={editingDoc ? '编辑文档' : '添加文档'}
        open={docModalVisible}
        onCancel={() => {
          setDocModalVisible(false)
          docForm.resetFields()
          setEditingDoc(null)
        }}
        footer={null}
        width={700}
      >
        <Form
          form={docForm}
          layout="vertical"
          onFinish={(values) => saveDocMutation.mutate({ ...editingDoc, kbId: selectedKB!.id!, ...values })}
        >
          <Form.Item
            name="title"
            label="文档标题"
            rules={[{ required: true, message: '请输入文档标题' }]}
          >
            <Input placeholder="请输入文档标题" />
          </Form.Item>
          <Form.Item
            name="content"
            label="文档内容"
            rules={[{ required: true, message: '请输入文档内容' }]}
          >
            <TextArea rows={10} placeholder="请输入文档内容，支持长文本" />
          </Form.Item>
          <Form.Item className="mb-0 text-right">
            <Space>
              <Button onClick={() => setDocModalVisible(false)}>取消</Button>
              <Button
                type="primary"
                htmlType="submit"
                loading={saveDocMutation.isPending}
              >
                保存
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* ASR 配置编辑模态框 */}
      <Modal
        title={editingASR ? '编辑 ASR 配置' : '新增 ASR 配置'}
        open={asrModalVisible}
        onCancel={() => {
          setAsrModalVisible(false)
          asrForm.resetFields()
          setEditingASR(null)
        }}
        footer={null}
      >
        <Form
          form={asrForm}
          layout="vertical"
          onFinish={(values) => saveASRMutation.mutate({ ...editingASR, ...values })}
          initialValues={{
            language: 'zh-CN',
            enabled: true
          }}
        >
          <Form.Item
            name="name"
            label="配置名称"
            rules={[{ required: true, message: '请输入配置名称' }]}
          >
            <Input placeholder="例如：火山引擎 ASR" />
          </Form.Item>
          <Form.Item
            name="provider"
            label="服务提供商"
            rules={[{ required: true, message: '请选择服务提供商' }]}
          >
            <Select placeholder="请选择服务提供商">
              <Option value="volc">火山引擎</Option>
              <Option value="ali">阿里云</Option>
              <Option value="tencent">腾讯云</Option>
              <Option value="openai">OpenAI</Option>
              <Option value="mock">Mock 测试</Option>
            </Select>
          </Form.Item>
          <Form.Item
            name="apiKey"
            label="API Key"
            rules={[{ required: true, message: '请输入 API Key' }]}
          >
            <Input.Password placeholder="请输入 API Key" />
          </Form.Item>
          <Form.Item
            name="endpoint"
            label="API Endpoint (可选)"
          >
            <Input placeholder="留空使用默认端点" />
          </Form.Item>
          <Form.Item
            name="language"
            label="语言"
          >
            <Select placeholder="请选择语言">
              <Option value="zh-CN">中文 (zh-CN)</Option>
              <Option value="en-US">英文 (en-US)</Option>
              <Option value="ja-JP">日文 (ja-JP)</Option>
            </Select>
          </Form.Item>
          <Form.Item
            name="enabled"
            label="启用状态"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item className="mb-0 text-right">
            <Space>
              <Button onClick={() => setAsrModalVisible(false)}>取消</Button>
              <Button
                type="primary"
                htmlType="submit"
                loading={saveASRMutation.isPending}
              >
                保存
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* TTS 配置编辑模态框 */}
      <Modal
        title={editingTTS ? '编辑 TTS 配置' : '新增 TTS 配置'}
        open={ttsModalVisible}
        onCancel={() => {
          setTtsModalVisible(false)
          ttsForm.resetFields()
          setEditingTTS(null)
        }}
        footer={null}
      >
        <Form
          form={ttsForm}
          layout="vertical"
          onFinish={(values) => saveTTSMutation.mutate({ ...editingTTS, ...values })}
          initialValues={{
            speed: 1.0,
            enabled: true
          }}
        >
          <Form.Item
            name="name"
            label="配置名称"
            rules={[{ required: true, message: '请输入配置名称' }]}
          >
            <Input placeholder="例如：火山引擎 TTS" />
          </Form.Item>
          <Form.Item
            name="provider"
            label="服务提供商"
            rules={[{ required: true, message: '请选择服务提供商' }]}
          >
            <Select placeholder="请选择服务提供商">
              <Option value="volc">火山引擎</Option>
              <Option value="ali">阿里云</Option>
              <Option value="tencent">腾讯云</Option>
              <Option value="openai">OpenAI</Option>
              <Option value="mock">Mock 测试</Option>
            </Select>
          </Form.Item>
          <Form.Item
            name="apiKey"
            label="API Key"
            rules={[{ required: true, message: '请输入 API Key' }]}
          >
            <Input.Password placeholder="请输入 API Key" />
          </Form.Item>
          <Form.Item
            name="endpoint"
            label="API Endpoint (可选)"
          >
            <Input placeholder="留空使用默认端点" />
          </Form.Item>
          <Form.Item
            name="voiceType"
            label="音色"
          >
            <Input placeholder="请输入音色标识" />
          </Form.Item>
          <Form.Item
            name="speed"
            label="语速"
          >
            <InputNumber min={0.5} max={2.0} step={0.1} style={{ width: '100%' }} placeholder="1.0 为正常语速" />
          </Form.Item>
          <Form.Item
            name="enabled"
            label="启用状态"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item className="mb-0 text-right">
            <Space>
              <Button onClick={() => setTtsModalVisible(false)}>取消</Button>
              <Button
                type="primary"
                htmlType="submit"
                loading={saveTTSMutation.isPending}
              >
                保存
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
