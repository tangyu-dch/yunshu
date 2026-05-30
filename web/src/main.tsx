import React from 'react'
import ReactDOM from 'react-dom/client'
import { App, ConfigProvider, theme as antdTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { QueryClient, QueryClientProvider, QueryCache, MutationCache } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { router } from '@/router'
import { useUiStore } from '@/store/ui'
import { message } from 'antd'
import '@/styles/index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 0,
      refetchOnWindowFocus: false,
    },
  },
  queryCache: new QueryCache({
    onError: (error) => {
      // Avoid showing auth redirect errors twice
      if (error.message && error.message.includes('请先登录')) return
      message.error(error.message || '数据加载失败，请检查后端服务')
    },
  }),
  mutationCache: new MutationCache({
    onError: (error) => {
      message.error(error.message || '操作执行失败')
    },
  }),
})

function AppProviders() {
  const uiTheme = useUiStore((state) => state.theme)

  React.useEffect(() => {
    if (uiTheme === 'dark') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }, [uiTheme])

  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: uiTheme === 'dark' ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: '#1d4ed8',
          borderRadius: 8,
          fontFamily: "Inter, 'PingFang SC', 'Microsoft YaHei', sans-serif",
        },
      }}
    >
      <App>
        <RouterProvider router={router} />
      </App>
    </ConfigProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <AppProviders />
    </QueryClientProvider>
  </React.StrictMode>,
)
