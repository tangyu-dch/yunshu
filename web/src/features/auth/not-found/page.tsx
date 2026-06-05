import { Button, Result } from 'antd'
import { useNavigate } from 'react-router-dom'

export function NotFoundPage() {
  const navigate = useNavigate()
  return (
    <div className="flex min-h-full items-center justify-center">
      <Result
        status="404"
        title="404"
        subTitle="您访问的页面不存在，或当前账号无权访问。"
        extra={<Button type="primary" onClick={() => navigate('/dashboard')}>返回首页</Button>}
      />
    </div>
  )
}
