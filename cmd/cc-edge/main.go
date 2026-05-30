// Package main 是 cc-edge 服务的入口包。
// cc-edge 服务是边界网关，负责外部请求接入、协议转换、安全认证、限流熔断等。
// 该服务部署在网络边缘，处理来自外部渠道（如电话、微信、网页）的呼叫请求。
package main

import (
	"yunshu/internal/app"
	"yunshu/internal/contracts"
)

// main 是 cc-edge 服务的启动入口。
// 初始化日志、配置、依赖注入，然后启动服务实例。
// 退出时执行优雅关闭，确保资源正确释放。
func main() { app.Run(contracts.ServiceEdge) }
