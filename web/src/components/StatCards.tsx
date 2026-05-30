import { Card, Col, Row, Statistic, Tag } from 'antd'
import type { StatItem } from '../types'

export function StatCards({ items }: { items: StatItem[] }) {
  return (
    <Row gutter={[16, 16]}>
      {items.map((item) => (
        <Col key={item.label} xs={24} sm={12} xl={6}>
          <Card className="shadow-soft">
            <Statistic title={item.label} value={item.value} />
            <div className="mt-3">
              <Tag color={item.tone === 'success' ? 'green' : item.tone === 'warning' ? 'gold' : item.tone === 'error' ? 'red' : 'blue'}>
                {item.trend ?? '稳定'}
              </Tag>
            </div>
          </Card>
        </Col>
      ))}
    </Row>
  )
}
