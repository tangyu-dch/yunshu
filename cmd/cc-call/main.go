// Package main 是 cc-call 服务的入口包。
// cc-call 服务负责 CTI/ESL 编排，处理业务调度、号码选择、任务状态、WebSocket 投影、
// CDR 持久化、计费、回调以及与 Kamailio 的集成。该服务不直接管理 FreeSWITCH 连接，
// 而是通过 ESL 事件适配层与 FreeSWITCH 节点进行交互。
package main

import (
	"yunshu/internal/app"
	"yunshu/internal/contracts"
)

// main 是 cc-call 服务的启动入口。
// 初始化日志、配置、依赖注入，然后启动服务实例。
// 退出时执行优雅关闭，确保资源正确释放。
func main() { app.Run(contracts.ServiceCall) }
