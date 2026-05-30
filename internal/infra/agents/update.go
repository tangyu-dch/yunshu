// Package agents 提供 AGENTS.md 文档自动更新工具。
//
// 该包负责从 internal/contracts 包提取路由、Redis、MQ 契约统计信息，
// 并将自动生成的摘要写入 AGENTS.md 文件，便于维护者追踪契约变更。
package agents

import (
	"fmt"
	"os"
	"strings"

	"yunshu/internal/contracts"
)

// beginMarker 标记 AGENTS.md 中自动生成区块的起始位置。
// endMarker 标记该区块的结束位置。两者之间的内容会被 RenderSummary 覆盖。
const (
	beginMarker = "<!-- BEGIN AUTO-GENERATED CONTRACT SUMMARY -->"
	endMarker   = "<!-- END AUTO-GENERATED CONTRACT SUMMARY -->"
)

// RenderSummary 从 contracts 包收集当前注册的路由、Redis、MQ 契约数量，
// 生成符合 Markdown 格式的自动生成摘要区块。
// 返回的字符串包含 beginMarker 和 endMarker 包裹的完整内容。
func RenderSummary() string {
	var b strings.Builder
	b.WriteString(beginMarker + "\n")
	b.WriteString("\n")
	b.WriteString("## Auto-Generated Contract Summary\n\n")
	b.WriteString("This section is generated from `internal/contracts`. Run `go run ./cmd/update-agents` after contract changes.\n\n")
	b.WriteString(fmt.Sprintf("- HTTP route contracts: %d\n", len(contracts.RouteContracts)))
	b.WriteString(fmt.Sprintf("- Redis contracts: %d\n", len(contracts.RedisContracts)))
	b.WriteString(fmt.Sprintf("- MQ contracts: %d\n", len(contracts.MQContracts)))
	b.WriteString("\n")
	b.WriteString("Service route counts:\n\n")
	for _, service := range []contracts.ServiceName{
		contracts.ServiceEdge,
		contracts.ServiceConsole,
		contracts.ServiceCall,
		contracts.ServiceWorker,
	} {
		b.WriteString(fmt.Sprintf("- `%s`: %d\n", service, len(contracts.RoutesFor(service))))
	}
	b.WriteString("\n")
	b.WriteString(endMarker + "\n")
	return b.String()
}

func UpdateFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(raw)
	summary := RenderSummary()
	start := strings.Index(text, beginMarker)
	end := strings.Index(text, endMarker)
	if start == -1 || end == -1 || end < start {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n" + summary
	} else {
		end += len(endMarker)
		text = text[:start] + strings.TrimRight(summary, "\n") + text[end:]
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
	}
	return os.WriteFile(path, []byte(text), 0o644)
}
