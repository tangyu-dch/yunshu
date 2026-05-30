import { Card, Table } from 'antd'
import type { ColumnsType, TableProps } from 'antd/es/table'
import type { ReactNode } from 'react'

type Props<T> = {
  title: string
  extra?: ReactNode
  columns: ColumnsType<T>
  dataSource: T[]
  rowKey: keyof T | ((record: T) => string)
  loading?: boolean
  scroll?: TableProps<T>['scroll']
} & Pick<TableProps<T>, 'pagination' | 'rowSelection' | 'locale'>

export function TableWrap<T extends object>({
  title,
  extra,
  columns,
  dataSource,
  rowKey,
  loading,
  pagination,
  scroll,
  rowSelection,
  locale,
}: Props<T>) {
  return (
    <Card title={title} extra={extra} className="shadow-soft">
      <Table<T>
        columns={columns}
        dataSource={dataSource}
        rowKey={rowKey as never}
        loading={loading}
        pagination={pagination ?? { pageSize: 8 }}
        scroll={scroll ?? { x: 'max-content' }}
        rowSelection={rowSelection}
        locale={locale}
      />
    </Card>
  )
}


