package install

import (
	"encoding/json"
	"regexp"
	"strings"
)

// SourceKind 安装来源类型。
type SourceKind string

const (
	SourceEcho SourceKind = "echo"
	SourceNPM  SourceKind = "npm"
	SourceHTTP SourceKind = "http"
)

var (
	reNPMPackage = regexp.MustCompile(`@?[a-zA-Z0-9][\w.\-]*/[@a-zA-Z0-9][\w.\-]*|@[a-zA-Z0-9][\w.\-]*`)
	reNpxLine    = regexp.MustCompile(`(?i)npx\s+(?:-y\s+)?(@?[\w.\-]+/[@\w.\-]+|@[\w.\-]+|[\w.\-]+)`)
)

// structuredRequirement 支持 Host 传入 JSON 需求。
type structuredRequirement struct {
	Source      string   `json:"source"`
	Package     string   `json:"package"`
	URL         string   `json:"url"`
	Args        []string `json:"args"`
	Summary     string   `json:"summary"`
	Template    string   `json:"template"`
	Requirement string   `json:"requirement"`
}

// EnrichPlan 从自然语言或 JSON 需求补全 SourceKind、npm 包名、URL 等。
func EnrichPlan(plan Plan) Plan {
	req := strings.TrimSpace(plan.Requirement)
	if req == "" {
		return plan
	}

	var st structuredRequirement
	if strings.HasPrefix(req, "{") {
		if err := json.Unmarshal([]byte(req), &st); err == nil {
			if strings.TrimSpace(st.Requirement) != "" {
				plan.Requirement = strings.TrimSpace(st.Requirement)
				req = plan.Requirement
			}
			if strings.TrimSpace(st.Summary) != "" {
				plan.Summary = strings.TrimSpace(st.Summary)
			}
			if tpl := Template(strings.TrimSpace(st.Template)); tpl != "" {
				plan.Template = tpl
			}
			switch strings.ToLower(strings.TrimSpace(st.Source)) {
			case "npm":
				plan.SourceKind = SourceNPM
				plan.NPMPackage = strings.TrimSpace(st.Package)
				plan.NPMArgs = st.Args
				plan.Source = "downloaded"
				return finalizePlan(plan, req)
			case "http", "url":
				// 在线 npm 安装模式下，不支持直接 URL 安装；保留 URL 仅用于错误提示/未来扩展。
				plan.SourceKind = SourceNPM
				plan.URL = strings.TrimSpace(st.URL)
				plan.Source = "downloaded"
				return finalizePlan(plan, req)
			case "echo", "generated":
				plan.SourceKind = SourceEcho
				plan.Template = TemplateEcho
				plan.Source = "generated"
				return finalizePlan(plan, req)
			}
			if p := strings.TrimSpace(st.Package); p != "" {
				plan.SourceKind = SourceNPM
				plan.NPMPackage = p
				plan.NPMArgs = st.Args
				plan.Source = "downloaded"
				return finalizePlan(plan, req)
			}
			if u := strings.TrimSpace(st.URL); u != "" {
				plan.SourceKind = SourceNPM
				plan.URL = u
				plan.Source = "downloaded"
				return finalizePlan(plan, req)
			}
		}
	}

	if pkg := extractNPMPackage(req); pkg != "" {
		plan.SourceKind = SourceNPM
		plan.NPMPackage = pkg
		plan.NPMArgs = extractNPMArgs(req)
		plan.Source = "downloaded"
		plan.Template = TemplateNPM
		return finalizePlan(plan, req)
	}

	// Preserve URL (if any) for hinting / future catalog mapping, but do not treat it as an install source.
	if u := ExtractURL(req); u != "" {
		plan.URL = u
	}

	lower := strings.ToLower(req)
	if strings.Contains(lower, "echo") || strings.Contains(req, "回显") {
		plan.SourceKind = SourceEcho
		plan.Template = TemplateEcho
		plan.Source = "generated"
		return finalizePlan(plan, req)
	}

	// 默认可生成 echo 占位；明确 npm/github 关键词时尝试 npm
	if strings.Contains(lower, "npm") || strings.Contains(lower, "npx") ||
		strings.Contains(lower, "package") || strings.Contains(lower, "安装包") {
		if pkg := extractNPMPackage(req); pkg != "" {
			plan.SourceKind = SourceNPM
			plan.NPMPackage = pkg
			plan.Template = TemplateNPM
			plan.Source = "downloaded"
			return finalizePlan(plan, req)
		}
	}

	// 默认不再回退到 echo（避免“看起来成功但其实是演示”）。
	// 若未解析出 npm 包名，则保持 SourceNPM 但 NPMPackage 为空，由安装流水线报错提示用户/LLM补全包名。
	plan.SourceKind = SourceNPM
	if plan.Template == "" {
		plan.Template = TemplateNPM
	}
	if plan.Source == "" {
		plan.Source = "downloaded"
	}
	return finalizePlan(plan, req)
}

func finalizePlan(plan Plan, req string) Plan {
	if plan.MCPID == "" {
		plan.MCPID = mcpIDFromRequirement(req)
	}
	if plan.Summary == "" {
		plan.Summary = summarize(req)
	}
	if plan.SourceKind == SourceNPM && plan.Template == "" {
		plan.Template = TemplateNPM
	}
	if plan.SourceKind == SourceEcho && plan.Template == "" {
		plan.Template = TemplateEcho
	}
	return plan
}

func extractNPMPackage(req string) string {
	if m := reNpxLine.FindStringSubmatch(req); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// npm install @scope/pkg
	lower := strings.ToLower(req)
	for _, prefix := range []string{"npm install ", "npm i ", "package "} {
		if idx := strings.Index(lower, prefix); idx >= 0 {
			rest := strings.TrimSpace(req[idx+len(prefix):])
			if pkg := firstNPMPackageToken(rest); pkg != "" {
				return pkg
			}
		}
	}
	if m := reNPMPackage.FindString(req); m != "" && (strings.Contains(m, "/") || strings.HasPrefix(m, "@")) {
		if strings.Contains(m, "://") || strings.Contains(m, ".com/") || strings.Contains(m, ".zip") {
			return ""
		}
		return strings.Trim(m, `"'`)
	}
	return ""
}

func firstNPMPackageToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], `"'`)
}

func extractNPMArgs(req string) []string {
	// 路径参数：引号内绝对/相对路径
	var args []string
	for _, q := range []string{`"`, `'`} {
		start := 0
		for {
			i := strings.Index(req[start:], q)
			if i < 0 {
				break
			}
			i += start
			j := strings.Index(req[i+1:], q)
			if j < 0 {
				break
			}
			inner := req[i+1 : i+1+j]
			if looksLikePathArg(inner) {
				args = append(args, inner)
			}
			start = i + 1 + j + 1
		}
	}
	return args
}

func looksLikePathArg(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	return strings.Contains(s, "/") || strings.Contains(s, `\`) ||
		strings.HasPrefix(s, ".") || len(s) > 2 && s[1] == ':'
}
