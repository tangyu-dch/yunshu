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
    <Card 
      title={
        <div className="flex items-center gap-2 py-1">
          <span className="w-[3px] h-3.5 rounded-full bg-gradient-to-b from-blue-500 to-indigo-500"></span>
          <span className="text-[13px] font-bold text-slate-800 dark:text-slate-100">{title}</span>
        </div>
      }
      className="bg-white dark:bg-slate-900/60 border border-slate-100 dark:border-slate-800/70 rounded-2xl shadow-soft overflow-hidden"
      styles={{ body: { padding: '12px 16px 16px' } }}
    >
      <ReactECharts option={option} style={{ height: 280 }} lazyUpdate={true} />
    </Card>
  )
}
