package install

import "strings"

type catalogEntry struct {
	Keywords []string
	Package  string
	Summary  string
}

// 内置能力 → npm 包映射（无 LLM 时的规则兜底；LLM 可用但不必依赖）。
var defaultCatalog = []catalogEntry{
	{Keywords: []string{"git", "版本", "commit", "branch", "仓库"}, Package: "@mseep/git-mcp-server", Summary: "Git 仓库操作 MCP"},
	{Keywords: []string{"filesystem", "文件系统", "读写文件", "目录"}, Package: "@modelcontextprotocol/server-filesystem", Summary: "文件系统 MCP"},
	{Keywords: []string{"github"}, Package: "@modelcontextprotocol/server-github", Summary: "GitHub MCP"},
	{Keywords: []string{"postgres", "数据库", "sql"}, Package: "@modelcontextprotocol/server-postgres", Summary: "Postgres MCP"},
	{Keywords: []string{"echo", "回显"}, Package: "", Summary: "Echo 演示"}, // 无 npm 包，规则层特殊处理
}

func matchCatalog(requirement string) (catalogEntry, bool) {
	lower := strings.ToLower(requirement)
	for _, e := range defaultCatalog {
		for _, kw := range e.Keywords {
			if kw != "" && strings.Contains(lower, strings.ToLower(kw)) {
				return e, true
			}
		}
	}
	return catalogEntry{}, false
}
