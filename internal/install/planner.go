package install

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
)

var urlInText = regexp.MustCompile(`https?://[^\s<>"']+`)

// Template 安装模板标识。
type Template string

const (
	TemplateEcho Template = "child-echo"
	TemplateNPM  Template = "npm-mcp"
	TemplateHTTP Template = "http-bundle"
)

// Plan 根据 requirement 选择模板与 mcp_id。
type Plan struct {
	MCPID       string
	Template    Template
	Summary     string
	Requirement string
	Source      string
	SourceKind  SourceKind
	NPMPackage  string
	NPMArgs     []string
	URL         string
}

// PlanRequirement 规则规划（Phase-1 无 LLM）。
// ExtractURL 从需求文本中提取首个 http(s) URL。
func ExtractURL(requirement string) string {
	m := urlInText.FindString(requirement)
	return strings.TrimRight(m, ".,;)")
}

func PlanRequirement(requirement string) Plan {
	req := strings.TrimSpace(requirement)
	id := mcpIDFromRequirement(req)
	plan := Plan{
		MCPID:       id,
		Requirement: req,
	}
	plan = EnrichPlan(plan)
	lower := strings.ToLower(req)
	switch plan.SourceKind {
	case SourceNPM:
		if plan.Summary == "" {
			plan.Summary = "NPM MCP：" + plan.NPMPackage
		}
	case SourceHTTP:
		if plan.Summary == "" {
			plan.Summary = "HTTP 安装包 MCP"
		}
	default:
		if strings.Contains(lower, "echo") || strings.Contains(req, "回显") {
			plan.Summary = "Echo 回显 MCP：" + summarize(req)
		} else if plan.Summary == "" {
			plan.Summary = "托管 MCP：" + summarize(req)
		}
	}
	return plan
}

func mcpIDFromRequirement(requirement string) string {
	h := sha256.Sum256([]byte(strings.ToLower(requirement)))
	return "mcp-" + hex.EncodeToString(h[:6])
}

func summarize(requirement string) string {
	r := []rune(requirement)
	if len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return requirement
}

// ResolveChildBinary 解析子 MCP 可执行文件路径（与 3M 同目录优先）。
func ResolveChildBinary(managerExe string, name string) string {
	if managerExe != "" {
		dir := filepath.Dir(managerExe)
		candidates := []string{
			filepath.Join(dir, name),
			filepath.Join(dir, name+".exe"),
		}
		for _, c := range candidates {
			if fileExists(c) {
				return c
			}
		}
	}
	return name
}

func fileExists(p string) bool {
	_, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	// os.Stat without import cycle - use simple check in pipeline
	return statFile(p)
}
