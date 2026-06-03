import {
  Button,
  Form,
  Input,
  Select,
  Switch,
  Space,
  Typography,
  message,
  Card,
  Row,
  Col,
  Alert,
  Spin,
  Tag,
  Modal,
  Upload,
  Progress,
  Tooltip
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import {
  fetchDialpadVersions,
  uploadDialpadVersion,
  deleteDialpadVersion,
  DialpadVersion
} from '@/api/operate'
import {
  CloudUploadOutlined,
  DeleteOutlined,
  DownloadOutlined,
  ReloadOutlined,
  InfoCircleOutlined,
  SafetyCertificateOutlined,
  AppleOutlined,
  WindowsOutlined,
  PieChartOutlined
} from '@ant-design/icons'
import { TableWrap } from '@/components/TableWrap'

const { Title, Text, Paragraph } = Typography
const { Option } = Select
const { TextArea } = Input

export function DialpadVersionPage() {
  const queryClient = useQueryClient()
  const [form] = Form.useForm()
  const [uploadFileList, setUploadFileList] = useState<any[]>([])
  const [uploadProgress, setUploadProgress] = useState<number>(0)
  const [isUploading, setIsUploading] = useState<boolean>(false)
  const [openUploadModal, setOpenUploadModal] = useState<boolean>(false)

  // 1. 获取所有拨号盘版本发布列表
  const { data: versions = [], isLoading, refetch } = useQuery({
    queryKey: ['operate', 'dialpad-versions'],
    queryFn: () => fetchDialpadVersions(),
    refetchInterval: 10000, // 每10秒自动刷新列表
  })

  // 2. 删除版本 Mutation
  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteDialpadVersion(id),
    onSuccess: () => {
      message.success('版本发布包已成功下架删除')
      queryClient.invalidateQueries({ queryKey: ['operate', 'dialpad-versions'] })
    },
    onError: (err: any) => {
      message.error(err?.message || '删除版本失败')
    }
  })

  // 3. 上传版本 Mutation
  const uploadMutation = useMutation({
    mutationFn: (formData: FormData) => uploadDialpadVersion(formData),
    onSuccess: () => {
      message.success('新版本已成功上传并部署至云端对象存储 (S3/RustFS)')
      queryClient.invalidateQueries({ queryKey: ['operate', 'dialpad-versions'] })
      setOpenUploadModal(false)
      form.resetFields()
      setUploadFileList([])
      setUploadProgress(0)
    },
    onError: (err: any) => {
      message.error(err?.message || '上传新版本发布包失败')
    },
    onSettled: () => {
      setIsUploading(false)
    }
  })

  // 处理删除确认
  const handleDelete = (record: DialpadVersion) => {
    if (!record.id) return
    Modal.confirm({
      title: '确认要删除并下架该客户端版本包吗？',
      icon: <DeleteOutlined style={{ color: '#ef4444' }} />,
      content: `这将会永久下架云枢拨号盘客户端 v${record.version} (${record.platform}/${record.arch})，该版本将无法被用户客户端下载或检测升级。`,
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => deleteMutation.mutateAsync(record.id!)
    })
  }

  // 格式化文件大小
  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  // 自定义上传逻辑，构建 FormData 直传后端
  const handleUploadSubmit = async () => {
    try {
      const values = await form.validateFields()
      if (uploadFileList.length === 0) {
        message.error('请选择需要上传的客户端二进制文件包 (.exe 或 .app)')
        return
      }

      setIsUploading(true)
      const fileObj = uploadFileList[0]
      const formData = new FormData()
      formData.append('version', values.version)
      formData.append('platform', values.platform)
      formData.append('arch', values.arch)
      formData.append('forceUpdate', values.forceUpdate ? 'true' : 'false')
      formData.append('changelog', values.changelog || '')
      formData.append('file', fileObj)

      // 模拟前端进度条
      let currentProgress = 0
      const interval = setInterval(() => {
        currentProgress += Math.floor(Math.random() * 10) + 5
        if (currentProgress >= 95) {
          clearInterval(interval)
          setUploadProgress(95)
        } else {
          setUploadProgress(currentProgress)
        }
      }, 200)

      await uploadMutation.mutateAsync(formData)
      clearInterval(interval)
      setUploadProgress(100)
    } catch (e) {
      setIsUploading(false)
    }
  }

  // 统计概览
  const totalVersions = versions.length
  const forceUpdateCount = versions.filter(v => v.forceUpdate).length
  const totalStorageSize = versions.reduce((acc, v) => acc + (v.fileSize || 0), 0)
  const latestVersionStr = versions.length > 0 ? versions[0].version : '暂无'

  return (
    <Space direction="vertical" size="large" className="w-full">
      {/* 顶部标题与操作栏 */}
      <div className="flex justify-between items-center mb-2 animate-fade-in">
        <div>
          <Title level={4} className="!mb-1 font-bold text-slate-800 dark:text-slate-200">
            拨号盘版本与文件管理
          </Title>
          <Paragraph className="text-xs text-slate-400 dark:text-zinc-500">
            用于管理和分发云枢软电话 (yunshu-phone) 桌面客户端二进制安装包，支持直传 S3/RustFS 存储系统，并支持配置强制更新策略。
          </Paragraph>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={isLoading}>
            刷新数据
          </Button>
          <Button
            type="primary"
            icon={<CloudUploadOutlined />}
            onClick={() => setOpenUploadModal(true)}
            style={{ background: 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)', border: 'none' }}
          >
            发布新版本包
          </Button>
        </Space>
      </div>

      {/* 统计数据概览看板 */}
      <Row gutter={[16, 16]} className="animate-fade-in">
        <Col span={6}>
          <Card bordered={false} className="shadow-sm rounded-xl bg-gradient-to-br from-slate-50 to-slate-100/50 dark:from-zinc-900/60 dark:to-zinc-950/40 border border-slate-100 dark:border-zinc-850 p-4">
            <div className="text-[10px] font-bold text-slate-400 dark:text-zinc-500 uppercase font-mono tracking-wider">LATEST STABLE VERSION</div>
            <div className="text-2xl font-extrabold text-indigo-600 dark:text-indigo-400 mt-2 font-mono">{latestVersionStr}</div>
            <div className="text-xs text-slate-400 dark:text-zinc-500 mt-2 flex items-center gap-1">
              <SafetyCertificateOutlined className="text-emerald-500" /> 运行状态: 线上正常分发中
            </div>
          </Card>
        </Col>
        <Col span={6}>
          <Card bordered={false} className="shadow-sm rounded-xl bg-gradient-to-br from-slate-50 to-slate-100/50 dark:from-zinc-900/60 dark:to-zinc-950/40 border border-slate-100 dark:border-zinc-850 p-4">
            <div className="text-[10px] font-bold text-slate-400 dark:text-zinc-500 uppercase font-mono tracking-wider">TOTAL UPLOADED PACKAGES</div>
            <div className="text-2xl font-extrabold text-slate-800 dark:text-slate-100 mt-2 font-mono">{totalVersions} <span className="text-xs font-normal text-slate-400">个文件</span></div>
            <div className="text-xs text-slate-400 dark:text-zinc-500 mt-2">包含多架构、多操作系统的独立构建包</div>
          </Card>
        </Col>
        <Col span={6}>
          <Card bordered={false} className="shadow-sm rounded-xl bg-gradient-to-br from-slate-50 to-slate-100/50 dark:from-zinc-900/60 dark:to-zinc-950/40 border border-slate-100 dark:border-zinc-850 p-4">
            <div className="text-[10px] font-bold text-slate-400 dark:text-zinc-500 uppercase font-mono tracking-wider">TOTAL STORAGE USED</div>
            <div className="text-2xl font-extrabold text-emerald-600 dark:text-emerald-400 mt-2 font-mono">{formatBytes(totalStorageSize)}</div>
            <div className="text-xs text-slate-400 dark:text-zinc-500 mt-2 flex items-center gap-1">
              <PieChartOutlined className="text-emerald-500" /> S3/RustFS 桶占用空间大小
            </div>
          </Card>
        </Col>
        <Col span={6}>
          <Card bordered={false} className="shadow-sm rounded-xl bg-gradient-to-br from-slate-50 to-slate-100/50 dark:from-zinc-900/60 dark:to-zinc-950/40 border border-slate-100 dark:border-zinc-850 p-4">
            <div className="text-[10px] font-bold text-slate-400 dark:text-zinc-500 uppercase font-mono tracking-wider">CRITICAL FORCE UPDATES</div>
            <div className="text-2xl font-extrabold text-rose-500 mt-2 font-mono">{forceUpdateCount} <span className="text-xs font-normal text-slate-400">个强制更新</span></div>
            <div className="text-xs text-slate-400 dark:text-zinc-500 mt-2">被拦截并要求必须更新的客户端版本</div>
          </Card>
        </Col>
      </Row>

      {/* S3/RustFS 提示 Alert */}
      <Alert
        message="S3/RustFS 分发机制提示"
        description="系统后台自动直传至配置的 S3/RustFS 存储桶。上传成功后会自动返回外网下载的直达 URL。如果没有配置 S3 访问凭证，系统将退避保存到云枢的本地目录中，前端依然可正常提供流式下载。"
        type="info"
        showIcon
        closable
        className="rounded-lg shadow-sm border border-indigo-100 dark:border-zinc-850 bg-indigo-50/10 dark:bg-zinc-900/10 animate-fade-in"
      />

      {/* 版本发布列表 */}
      <div className="animate-fade-in">
        <TableWrap
          title="已发布客户端版本列表"
          rowKey="id"
          dataSource={versions}
          loading={isLoading}
          columns={[
            {
              title: '版本号',
              dataIndex: 'version',
              key: 'version',
              render: (val) => <span className="font-mono font-bold text-slate-800 dark:text-slate-100">v{val}</span>
            },
            {
              title: '系统平台',
              dataIndex: 'platform',
              key: 'platform',
              render: (val) => {
                const icon = val === 'darwin' ? <AppleOutlined /> : <WindowsOutlined />
                const color = val === 'darwin' ? 'default' : 'processing'
                const text = val === 'darwin' ? 'macOS' : 'Windows'
                return (
                  <Tag icon={icon} color={color} className="font-semibold px-2 py-0.5">
                    {text}
                  </Tag>
                )
              }
            },
            {
              title: 'CPU 架构',
              dataIndex: 'arch',
              key: 'arch',
              render: (val) => <Tag className="font-mono text-xs">{val}</Tag>
            },
            {
              title: '强制更新',
              dataIndex: 'forceUpdate',
              key: 'forceUpdate',
              render: (val) => val ? (
                <Tag color="error" style={{ border: 'none' }} className="font-semibold text-rose-600">
                  <span className="w-1.5 h-1.5 rounded-full bg-rose-500 animate-pulse inline-block mr-1" />
                  强制性更新
                </Tag>
              ) : (
                <Tag color="success" style={{ border: 'none' }} className="font-semibold text-emerald-600">
                  可选性更新
                </Tag>
              )
            },
            {
              title: '文件大小',
              dataIndex: 'fileSize',
              key: 'fileSize',
              render: (val) => <span className="font-mono text-xs text-slate-600 dark:text-zinc-400">{formatBytes(val)}</span>
            },
            {
              title: '更新日志',
              dataIndex: 'changelog',
              key: 'changelog',
              render: (val) => (
                <Tooltip title={val} placement="topLeft">
                  <span className="text-slate-600 dark:text-zinc-400 text-xs truncate max-w-xs block">
                    {val || '暂无说明'}
                  </span>
                </Tooltip>
              )
            },
            {
              title: '发布日期',
              dataIndex: 'createdTime',
              key: 'createdTime',
              render: (val) => {
                if (!val) return '未知'
                // 格式化时间为本地显示
                return <span className="font-mono text-xs text-slate-500 dark:text-zinc-500">{new Date(val).toLocaleString('zh-CN')}</span>
              }
            },
            {
              title: '操作',
              key: 'action',
              render: (_, record) => (
                <Space size="middle">
                  <Button
                    type="link"
                    icon={<DownloadOutlined />}
                    href={record.downloadUrl}
                    target="_blank"
                    size="small"
                  >
                    下载
                  </Button>
                  <Button
                    type="link"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => handleDelete(record)}
                    size="small"
                    disabled={deleteMutation.isPending}
                  >
                    删除
                  </Button>
                </Space>
              )
            }
          ]}
        />
      </div>

      {/* 发布新版本 Modal */}
      <Modal
        open={openUploadModal}
        title={
          <span className="font-bold text-base flex items-center gap-2 text-slate-800 dark:text-slate-100">
            <CloudUploadOutlined className="text-indigo-500" />
            发布新客户端安装包
          </span>
        }
        onCancel={() => {
          if (!isUploading) {
            setOpenUploadModal(false)
            form.resetFields()
            setUploadFileList([])
            setUploadProgress(0)
          }
        }}
        width={580}
        destroyOnClose
        maskClosable={false}
        footer={[
          <Button
            key="cancel"
            onClick={() => {
              setOpenUploadModal(false)
              form.resetFields()
              setUploadFileList([])
            }}
            disabled={isUploading}
          >
            取消
          </Button>,
          <Button
            key="submit"
            type="primary"
            icon={<CloudUploadOutlined />}
            loading={isUploading}
            onClick={handleUploadSubmit}
            style={{ background: 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)', border: 'none' }}
          >
            {isUploading ? '正在上传中...' : '提交发布'}
          </Button>
        ]}
      >
        <Form
          form={form}
          layout="vertical"
          disabled={isUploading}
          className="pt-4"
          initialValues={{
            platform: 'darwin',
            arch: 'arm64',
            forceUpdate: false
          }}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="version"
                label={
                  <Space size={4}>
                    <span>客户端版本号</span>
                    <Tooltip title="输入三段式 SemVer 版本号，例如 1.0.2 或 1.1.0">
                      <InfoCircleOutlined className="text-slate-400" />
                    </Tooltip>
                  </Space>
                }
                rules={[
                  { required: true, message: '请输入版本号' },
                  { pattern: /^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$/, message: '请输入符合标准的语义化版本格式 (如 1.0.0)' }
                ]}
              >
                <Input placeholder="例如: 1.0.2" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="forceUpdate"
                label={
                  <Space size={4}>
                    <span>强制性更新限制</span>
                    <Tooltip title="若开启，客户端将必须更新才能进入系统，无法被用户取消或跳过">
                      <InfoCircleOutlined className="text-slate-400" />
                    </Tooltip>
                  </Space>
                }
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="platform"
                label="目标运行平台"
                rules={[{ required: true }]}
              >
                <Select>
                  <Option value="darwin">macOS (darwin)</Option>
                  <Option value="windows">Windows (windows)</Option>
                  <Option value="linux">Linux (linux)</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="arch"
                label="目标 CPU 架构"
                rules={[{ required: true }]}
              >
                <Select>
                  <Option value="arm64">Apple Silicon / ARM64 (arm64)</Option>
                  <Option value="amd64">Intel / AMD 64位 (amd64)</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item
            name="changelog"
            label="更新日志说明"
            rules={[{ required: true, message: '请输入版本更新内容日志' }]}
          >
            <TextArea
              rows={4}
              placeholder="请输入当前版本的更新细节与错误修复日志..."
            />
          </Form.Item>

          <Form.Item
            label="上传二进制包文件 (.exe / .app / .zip)"
            required
          >
            <Upload
              beforeUpload={(file) => {
                setUploadFileList([file])
                return false // 不自动上传，由我们手动触发
              }}
              fileList={uploadFileList}
              onRemove={() => setUploadFileList([])}
              maxCount={1}
            >
              <Button icon={<CloudUploadOutlined />}>选择物理文件</Button>
            </Upload>
          </Form.Item>

          {isUploading && (
            <div className="mt-4 p-3 bg-slate-50 dark:bg-zinc-900/40 rounded-lg border border-slate-100 dark:border-zinc-800 animate-fade-in">
              <div className="flex justify-between items-center text-xs mb-1.5 font-mono">
                <span className="text-slate-500 dark:text-zinc-400">正在打包并流式上传至云端...</span>
                <span className="font-bold text-indigo-600 dark:text-indigo-400">{uploadProgress}%</span>
              </div>
              <Progress
                percent={uploadProgress}
                status={uploadProgress === 100 ? 'success' : 'active'}
                strokeColor={{ from: '#6366f1', to: '#4f46e5' }}
                showInfo={false}
              />
            </div>
          )}
        </Form>
      </Modal>
    </Space>
  )
}
