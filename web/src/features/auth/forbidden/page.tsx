import { Button, Result } from 'antd'
import { useNavigate } from 'react-router-dom'

export function ForbiddenPage() {
  const navigate = useNavigate()
  return (
    <div className="flex min-h-full items-center justify-center">
      <Result
        status="403"
        title="403"
        subTitle="当前账号没有访问该页面的权限。"
        extra={<Button type="primary" onClick={() => navigate('/dashboard')}>返回首页</Button>}
      />
    </div>
  )
}
