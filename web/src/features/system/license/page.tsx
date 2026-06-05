import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Card, Col, Row, Button, Upload, Alert, Tag, Space, Typography, Spin, App, message, Modal, Descriptions, Statistic, Badge } from 'antd'
import { 
  CopyOutlined, 
  DownloadOutlined, 
  UploadOutlined, 
  SafetyCertificateOutlined, 
  CheckCircleOutlined,
  WarningOutlined,
  CloseCircleOutlined,
  InfoCircleOutlined,
  CalendarOutlined,
  DashboardOutlined,
  UsergroupAddOutlined,
  FileProtectOutlined
} from '@ant-design/icons'
import { 
  fetchLicenseStatus, 
  uploadLicenseFile 
} from '@/api/operate'

const { Title, Text, Paragraph } = Typography

export function LicensePage() {
  const queryClient = useQueryClient()
  const [uploading, setUploading] = useState(false)

  // 获取授权状态与系统规格
  const { data: status, isLoading: statusLoading } = useQuery({
    queryKey: ['system', 'license', 'status'],
    queryFn: fetchLicenseStatus,
    refetchInterval: 10000, // 每10秒轮询一次以保持防时钟回滚高水位更新与状态刷新
  })

  // 一键复制部署 ID
  const handleCopyDeploymentId = () => {
    if (status?.deploymentId) {
      navigator.clipboard.writeText(status.deploymentId)
      message.success('部署 ID 已成功复制到剪贴板！')
    }
  }

  // 校验并激活授权证书 (Upload)
  const handleUpload = async (file: File) => {
    setUploading(true)
    try {
      await uploadLicenseFile(file)
      message.success('证书文件验证通过，授权已成功激活！')
      
      // 重新查询最新授权信息
      const freshStatus = await queryClient.fetchQuery({
        queryKey: ['system', 'license', 'status'],
        queryFn: fetchLicenseStatus
      })

      // 根据证书类型弹出不同样式的通知 (平移 vs 续期)
      if (freshStatus.licenseType === 'migration') {
        Modal.success({
          title: '授权平移成功！',
          content: (
            <div className="py-2">
              <p>检测到系统已顺利由原服务器平移至本服务器。</p>
              <div className="my-2 p-2 bg-slate-50 dark:bg-slate-900 rounded font-mono text-xs text-slate-600 dark:text-slate-400">
                <div>原部署ID: <strong>{freshStatus.previousDeploymentId || '未知'}</strong></div>
                <div>本部署ID: <strong>{freshStatus.deploymentId}</strong></div>
              </div>
              <p>有效期保持不变，截止至: <strong className="text-indigo-600">{freshStatus.notAfter}</strong></p>
            </div>
          ),
          okText: '确认',
        })
      } else if (freshStatus.licenseType === 'renewal') {
        Modal.success({
          title: '授权续期成功！',
          content: (
            <div className="py-2">
              <p>系统授权已成功延长。</p>
              <p>到期时间延至: <strong className="text-green-600">{freshStatus.notAfter}</strong></p>
              <p>当前剩余天数: <strong className="text-green-600">{freshStatus.remainingDays} 天</strong></p>
            </div>
          ),
          okText: '确认',
        })
      } else {
        Modal.success({
          title: '授权激活成功！',
          content: `当前系统已成功绑定商业授权，到期时间至 ${freshStatus.notAfter}。`,
          okText: '确认',
        })
      }
    } catch (err: any) {
      // 错误已被拦截器显示，仅做捕获
    } finally {
      setUploading(false)
    }
    return false // 阻止默认的 action 提交
  }

  if (statusLoading) {
    return (
      <div className="flex h-[400px] items-center justify-center">
        <Spin size="large" tip="正在加载系统授权与安全参数..." />
      </div>
    )
  }

  // 状态解析与 UI 表现
  const getStatusConfig = () => {
    switch (status?.status) {
      case 'normal':
        return {
          color: 'success' as const,
          badgeStatus: 'success' as const,
          text: '已激活 (ACTIVE)',
          icon: <CheckCircleOutlined className="text-green-500 text-3xl" />,
          alertType: 'success' as const,
          desc: '系统授权运行正常，所有功能模块可用。',
        }
      case 'grace_period':
        return {
          color: 'warning' as const,
          badgeStatus: 'warning' as const,
          text: '宽限期内 (GRACE PERIOD)',
          icon: <WarningOutlined className="text-yellow-500 text-3xl" />,
          alertType: 'warning' as const,
          desc: status.statusMsg || '授权证书已过期，当前处于15天宽限期内，额定最大并发数受限调低至80%。请尽快续期。',
        }
      case 'expired':
        return {
          color: 'error' as const,
          badgeStatus: 'error' as const,
          text: '已过期 (EXPIRED)',
          icon: <CloseCircleOutlined className="text-red-500 text-3xl" />,
          alertType: 'error' as const,
          desc: status.statusMsg || '授权证书已过期且宽限期结束，呼叫控制通道已锁死。请上传新证书恢复。',
        }
      case 'time_rollback_locked':
        return {
          color: 'error' as const,
          badgeStatus: 'error' as const,
          text: '安全锁死 (LOCKED)',
          icon: <CloseCircleOutlined className="text-red-600 text-3xl animate-pulse" />,
          alertType: 'error' as const,
          desc: status.statusMsg || '检测到服务器时钟发生恶意回拨，系统安全保护锁死。请调整回正常时钟或联系客服处理。',
        }
      default:
        return {
          color: 'default' as const,
          badgeStatus: 'default' as const,
          text: '未授权 (UNLICENSED)',
          icon: <InfoCircleOutlined className="text-slate-400 text-3xl" />,
          alertType: 'info' as const,
          desc: '系统当前处于未授权状态。请下载设备指纹并导入有效授权证书进行激活。',
        }
    }
  }

  const statusConfig = getStatusConfig()

  return (
    <App>
      <div className="space-y-6 max-w-6xl mx-auto px-4 py-6 bg-slate-50/50 dark:bg-transparent rounded-xl">
        {/* 页头及警示栏 */}
        <div className="flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <div>
              <Title level={4} className="!mb-1 flex items-center gap-2 dark:text-white">
                <SafetyCertificateOutlined className="text-indigo-500 text-xl" />
                系统授权管理
              </Title>
              <Paragraph className="text-slate-400 dark:text-slate-400 text-xs !mb-0">
                查看并管理当前独立物理环境中的云枢授权书，监控并发资源上限。
              </Paragraph>
            </div>
            <Tag color={statusConfig.color === 'default' ? 'default' : statusConfig.color} className="text-xs px-2.5 py-0.5 font-medium m-0">
              <Badge status={statusConfig.badgeStatus} className="mr-1.5" />
              {statusConfig.text}
            </Tag>
          </div>

          <Alert
            message={<span className="font-semibold text-sm">{statusConfig.text} - 运行状态通知</span>}
            description={<span className="text-xs text-slate-500 dark:text-slate-400">{statusConfig.desc}</span>}
            type={statusConfig.alertType}
            showIcon
            className="shadow-sm border-0 bg-white dark:bg-[#15181e] rounded-lg"
          />
        </div>

        {/* 关键数据卡片栏 (Antd Statistic) */}
        {status?.status !== 'unlicensed' && status?.status !== 'time_rollback_locked' && (
          <Row gutter={[16, 16]}>
            <Col xs={24} sm={8}>
              <Card className="shadow-sm border-0 dark:bg-[#15181e] rounded-lg">
                <Statistic
                  title={<span className="text-slate-400 dark:text-slate-400 text-xs">剩余有效期</span>}
                  value={status?.remainingDays ?? 0}
                  suffix={<span className="text-sm">天</span>}
                  valueStyle={{ 
                    color: status?.remainingDays !== undefined && status.remainingDays < 30 ? '#ef4444' : '#10b981', 
                    fontWeight: 700,
                    fontSize: '24px'
                  }}
                  prefix={<CalendarOutlined className="mr-2 text-slate-300" />}
                />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card className="shadow-sm border-0 dark:bg-[#15181e] rounded-lg">
                <Statistic
                  title={<span className="text-slate-400 dark:text-slate-400 text-xs">系统并发授权上限</span>}
                  value={status?.maxConcurrentCalls ?? 0}
                  suffix={<span className="text-sm">线</span>}
                  valueStyle={{ color: '#6366f1', fontWeight: 700, fontSize: '24px' }}
                  prefix={<DashboardOutlined className="mr-2 text-slate-300" />}
                />
              </Card>
            </Col>
            <Col xs={24} sm={8}>
              <Card className="shadow-sm border-0 dark:bg-[#15181e] rounded-lg">
                <Statistic
                  title={<span className="text-slate-400 dark:text-slate-400 text-xs">最大坐席分机数</span>}
                  value={status?.maxExtensions ?? 0}
                  suffix={<span className="text-sm">个</span>}
                  valueStyle={{ color: '#0ea5e9', fontWeight: 700, fontSize: '24px' }}
                  prefix={<UsergroupAddOutlined className="mr-2 text-slate-300" />}
                />
              </Card>
            </Col>
          </Row>
        )}

        {/* 栅格内容 */}
        <Row gutter={[16, 16]}>
          {/* 左侧：授权详细信息与规格限制 */}
          <Col xs={24} md={14}>
            <Card 
              title={
                <Space>
                  <FileProtectOutlined className="text-indigo-500" />
                  <span className="font-semibold text-sm dark:text-white">授权规格详情</span>
                </Space>
              }
              extra={
                status?.status !== 'unlicensed' && status?.status !== 'time_rollback_locked' && status?.licenseType !== 'trial' && (
                  <Button 
                    type="link" 
                    icon={<DownloadOutlined />} 
                    className="p-0 text-xs text-indigo-500 hover:text-indigo-600"
                    onClick={() => window.open('/api/operate/license/download')}
                  >
                    备份当前证书 (.lic)
                  </Button>
                )
              }
              className="shadow-sm border-0 dark:bg-[#15181e] rounded-lg h-full"
            >
              {status?.status === 'unlicensed' || status?.status === 'time_rollback_locked' ? (
                <div className="flex flex-col items-center justify-center py-20 text-slate-400 text-center h-full">
                  <InfoCircleOutlined className="text-5xl mb-4 text-slate-300 dark:text-slate-700" />
                  <Text type="secondary" className="max-w-xs block text-xs">
                    系统当前尚未绑定任何商业授权证书。请下载右侧设备指纹向云枢团队获取授权后导入激活。
                  </Text>
                </div>
              ) : (
                <Descriptions column={2} size="middle" className="mt-2" labelStyle={{ color: '#94a3b8' }}>
                  <Descriptions.Item label="授权编号" span={2}>
                    <Text className="font-mono font-medium dark:text-slate-200">{status?.licenseId || '-'}</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="客户主体" span={2}>
                    <Text className="font-medium dark:text-slate-200">{status?.customerName || '-'}</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="授权类型" span={2}>
                    {status?.licenseType === 'migration' ? (
                      <Tag color="purple">平移证书 (Migrated)</Tag>
                    ) : status?.licenseType === 'renewal' ? (
                      <Tag color="success">续期证书 (Renewed)</Tag>
                    ) : status?.licenseType === 'trial' ? (
                      <Tag color="orange">试用授权 (Trial)</Tag>
                    ) : (
                      <Tag color="blue">标准授权 (Standard)</Tag>
                    )}
                  </Descriptions.Item>
                  <Descriptions.Item label="生效时间">
                    <Text className="text-xs dark:text-slate-300">{status?.notBefore || '-'}</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="到期时间">
                    <Text className="text-xs dark:text-slate-300">{status?.notAfter || '-'}</Text>
                  </Descriptions.Item>
                  {status?.features && status.features.length > 0 && (
                    <Descriptions.Item label="解锁功能" span={2}>
                      <div className="flex flex-wrap gap-1 mt-1">
                        {status.features.map(f => (
                          <Tag color="indigo" key={f} className="m-0 text-[10px] py-0.5 px-1.5 border-none font-mono">
                            {f}
                          </Tag>
                        ))}
                      </div>
                    </Descriptions.Item>
                  )}
                </Descriptions>
              )}
            </Card>
          </Col>

          {/* 右侧：设备识别码与激活证书 */}
          <Col xs={24} md={10} className="flex flex-col gap-4">
            {/* 部署唯一特征 */}
            <Card 
              title={<span className="font-semibold text-sm dark:text-white">环境识别特征</span>}
              className="shadow-sm border-0 dark:bg-[#15181e] rounded-lg"
            >
              <div className="space-y-4">
                <div className="p-3 bg-slate-50 dark:bg-slate-900 rounded-lg border border-slate-100 dark:border-slate-800">
                  <Text className="text-[10px] text-slate-400 dark:text-slate-400 block mb-1">环境唯一部署 ID (Deployment ID)</Text>
                  <div className="flex justify-between items-center gap-3">
                    <Text className="font-mono text-xs font-bold tracking-wider dark:text-indigo-400 break-all select-all">
                      {status?.deploymentId || 'DEPLOY-N/A'}
                    </Text>
                    <Space size="small" className="flex-shrink-0">
                      <Button icon={<CopyOutlined />} size="small" onClick={handleCopyDeploymentId} className="text-xs">
                        复制
                      </Button>
                      <Button 
                        type="primary" 
                        icon={<DownloadOutlined />} 
                        size="small"
                        className="text-xs bg-indigo-500 hover:bg-indigo-600 border-0"
                        onClick={() => window.open('/api/operate/license/fingerprint/download')}
                      >
                        下载指纹
                      </Button>
                    </Space>
                  </div>
                </div>
                <p className="text-[11px] text-slate-400 leading-relaxed m-0">
                  说明：系统根据宿主机物理硬件（如主网卡MAC等）生成确定性部署 ID。在授权激活前，请将该 ID 复制或下载为 JSON 格式的指纹文件并发送至技术支持换取证书。
                </p>
              </div>
            </Card>

            {/* 导入授权证书 */}
            <Card 
              title={<span className="font-semibold text-sm dark:text-white">导入系统授权证书</span>}
              className="shadow-sm border-0 dark:bg-[#15181e] rounded-lg"
            >
              <Upload.Dragger
                name="file"
                multiple={false}
                beforeUpload={handleUpload}
                showUploadList={false}
                disabled={uploading}
                className="bg-slate-50/50 dark:bg-slate-900/10 border-dashed border-slate-200 dark:border-slate-800 rounded-lg py-4"
              >
                <p className="ant-upload-drag-icon">
                  <UploadOutlined className="text-2xl text-indigo-400" />
                </p>
                <p className="ant-upload-text text-xs dark:text-slate-300 font-medium">
                  点击浏览或将证书文件拖拽至此处
                </p>
                <p className="ant-upload-hint text-[10px] text-slate-400 dark:text-slate-400 mt-1">
                  仅支持上传导出的 .lic 授权文件，导入后即时验签激活生效。
                </p>
              </Upload.Dragger>
            </Card>
          </Col>
        </Row>
      </div>
    </App>
  )
}
