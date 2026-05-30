// Package main 是 cc-console 服务的入口包。
// cc-console 服务是话务控制台后端，提供坐席管理、实时监控、通话记录查询、
// 技能组配置等运营管理功能。该服务暴露 RESTful API 供前端控制台使用。
package main

import (
	"yunshu/internal/app"
	"yunshu/internal/contracts"
)

// main 是 cc-console 服务的启动入口。
// 初始化日志、配置、依赖注入，然后启动服务实例。
// 退出时执行优雅关闭，确保资源正确释放。
func main() { app.Run(contracts.ServiceConsole) }
