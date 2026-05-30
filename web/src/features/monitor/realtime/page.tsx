import { Badge, Card, Col, Row, Space, Tag, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { TableWrap } from '../../../components/TableWrap'
import { fetchCallRecords, fetchFsNodes } from '../../../api/operate'

export function RealtimeMonitorPage() {
  const { data: nodes } = useQuery({ queryKey: ['monitor', 'nodes'], queryFn: fetchFsNodes })
  const { data: calls } = useQuery({ queryKey: ['monitor', 'calls'], queryFn: () => fetchCallRecords(1, 50) })

  return (
    <Space direction="vertical" size="large" className="w-full">
      <div className="mb-2">
        <Typography.Text type="secondary">看当前通话、FS 节点、事件租约和推送状态。</Typography.Text>
      </div>
      <Row gutter={[16, 16]}>
        {nodes?.map((node) => (
          <Col key={node.id} xs={24} md={8}>
            <Card className="shadow-soft">
              <div className="flex items-start justify-between">
                <div>
                  <Typography.Text strong>{node.name}</Typography.Text>
                  <div className="text-slate-500">{node.fsAddr}</div>
                </div>
                <Tag color={node.status === 'active' ? 'green' : node.status === 'draining' ? 'gold' : 'red'}>{node.status}</Tag>
              </div>
              <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                <div>
                  <div className="text-slate-500">事件租约</div>
                  <div>{node.owner}</div>
                </div>
                <div>
                  <div className="text-slate-500">通道占用</div>
                  <div>
                    {node.activeCalls}/{node.maxChannels}
                  </div>
                </div>
              </div>
            </Card>
          </Col>
        ))}
      </Row>
      <TableWrap
        title="实时通话"
        rowKey="callId"
        dataSource={calls?.records ?? []}
        columns={[
          { title: 'Call ID', dataIndex: 'callId' },
          { title: '商户', dataIndex: 'merchant' },
          { title: '节点', dataIndex: 'fsAddr' },
          { title: '状态', dataIndex: 'state', render: (value: string) => <Badge status={value === 'completed' ? 'success' : 'processing'} text={value} /> },
          { title: '完成时间', dataIndex: 'finishedAt' },
        ]}
      />
    </Space>
  )
}
