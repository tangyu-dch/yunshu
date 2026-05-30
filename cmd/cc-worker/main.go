// Package main 是 cc-worker 服务的入口包。
// cc-worker 服务是后台任务处理器，负责异步任务执行、定时任务调度、
// 批量外呼、录音转写、报表生成等后台运算密集型工作。该服务通过消息队列
// 接收任务，按照配置的消费策略执行，并汇报执行结果。
package main

import (
	"yunshu/internal/app"
	"yunshu/internal/contracts"
)

// main 是 cc-worker 服务的启动入口。
// 初始化日志、配置、依赖注入，然后启动服务实例。
// 退出时执行优雅关闭，确保资源正确释放。
func main() { app.Run(contracts.ServiceWorker) }
