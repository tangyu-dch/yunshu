import { Button, DatePicker, Form, Input, InputNumber, Row, Col, Select, Space, Card } from 'antd'
import { SearchOutlined, ReloadOutlined } from '@ant-design/icons'

export interface QueryField {
  key: string
  label: string
  type: 'text' | 'select' | 'date-range' | 'number'
  placeholder?: string
  options?: { label: string; value: any }[]
}

interface QueryBarProps {
  fields: QueryField[]
  onSearch: (values: Record<string, any>) => void
  onReset?: () => void
  loading?: boolean
  initialValues?: Record<string, any>
}

export function QueryBar({ fields, onSearch, onReset, loading, initialValues }: QueryBarProps) {
  const [form] = Form.useForm()

  const handleFinish = (values: Record<string, any>) => {
    // Clean up empty strings or undefined fields
    const cleanedValues: Record<string, any> = {}
    Object.keys(values).forEach((key) => {
      const val = values[key]
      if (val !== undefined && val !== null && val !== '') {
        cleanedValues[key] = val
      }
    })
    onSearch(cleanedValues)
  }

  const handleReset = () => {
    form.resetFields()
    if (onReset) {
      onReset()
    } else {
      onSearch({})
    }
  }

  return (
    <Card className="shadow-soft mb-6 border-slate-100 dark:border-slate-800 dark:bg-slate-900/60 backdrop-blur-md">
      <Form
        form={form}
        layout="horizontal"
        onFinish={handleFinish}
        className="w-full"
        initialValues={initialValues}
      >
        <Row gutter={[16, 16]} align="middle">
          {fields.map((field) => (
            <Col xs={24} sm={12} md={8} lg={6} key={field.key}>
              <Form.Item
                name={field.key}
                label={<span className="font-medium text-slate-600 dark:text-slate-400">{field.label}</span>}
                className="mb-0 w-full flex-row items-center"
                labelCol={{ span: 8 }}
                wrapperCol={{ span: 16 }}
              >
                {field.type === 'text' && (
                  <Input
                    placeholder={field.placeholder ?? `请输入${field.label}`}
                    allowClear
                    className="w-full dark:bg-slate-800 dark:border-slate-700"
                  />
                )}
                {field.type === 'select' && (
                  <Select
                    placeholder={field.placeholder ?? `请选择${field.label}`}
                    allowClear
                    options={field.options}
                    className="w-full"
                    classNames={{ popup: "dark:bg-slate-800" } as any}
                  />
                )}
                {field.type === 'number' && (
                  <InputNumber
                    placeholder={field.placeholder ?? `请输入`}
                    className="w-full dark:bg-slate-800 dark:border-slate-700"
                  />
                )}
                {field.type === 'date-range' && (
                  <DatePicker.RangePicker
                    className="w-full dark:bg-slate-800 dark:border-slate-700"
                    placeholder={['开始', '结束']}
                  />
                )}
              </Form.Item>
            </Col>
          ))}
          
          <Col xs={24} sm={24} md={fields.length % 3 === 0 ? 24 : 8} lg={fields.length % 4 === 0 ? 24 : (fields.length % 4 === 3 ? 6 : 8)} className="ml-auto text-right">
            <Space size="middle">
              <Button
                type="primary"
                htmlType="submit"
                icon={<SearchOutlined />}
                loading={loading}
                className="bg-blue-600 hover:bg-blue-700 dark:bg-blue-700 dark:hover:bg-blue-800"
              >
                查询
              </Button>
              <Button
                icon={<ReloadOutlined />}
                onClick={handleReset}
                className="dark:bg-slate-800 dark:border-slate-700 dark:text-slate-300"
              >
                重置
              </Button>
            </Space>
          </Col>
        </Row>
      </Form>
    </Card>
  )
}
