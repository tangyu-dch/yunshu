// Package main 是 update-agents 工具的入口包。
// update-agents 工具用于自动更新 AGENTS.md 文件中的合约摘要。
// 它从 internal/contracts 包中提取 HTTP 路由、Redis 合约和 MQ 合约的定义，
// 并生成格式化的摘要章节。当 HTTP、Redis 或 MQ 合约发生变更后，应运行此工具。
package main

import (
	"log"

	"yunshu/internal/infra/agents"
)

// main 是 update-agents 工具的启动入口。
// 从 internal/contracts 包中提取最新的合约信息，并更新 AGENTS.md 文件。
// 执行失败时会输出错误信息并以非零状态码退出。
func main() {
	if err := agents.UpdateFile("AGENTS.md"); err != nil {
		log.Fatal(err)
	}
}
