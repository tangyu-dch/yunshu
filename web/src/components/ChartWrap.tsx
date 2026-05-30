import { Card } from 'antd'
import ReactECharts from 'echarts-for-react'

export function ChartWrap({
  title,
  option,
}: {
  title: string
  option: Record<string, unknown>
}) {
  return (
    <Card title={title} className="shadow-soft">
      <ReactECharts option={option} style={{ height: 320 }} />
    </Card>
  )
}
