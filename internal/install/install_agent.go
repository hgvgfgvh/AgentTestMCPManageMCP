package install

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"AgentTestMCPManageMCP/internal/llm"
	"AgentTestMCPManageMCP/internal/sandbox"
)

const installSystemPrompt = `你是 MCPManagerMCP（3M）内部的「安装 Agent」。你在独立沙箱中工作，不向 Host 回复。
安装策略：统一走「在线 npm 安装 + 3M 自带 Node runtime」；不要输出 URL 安装方案。

每次只输出一个 JSON 对象（不要 markdown），action 取值：
- plan: {"action":"plan","source":"npm|echo","template":"npm-mcp|child-echo","package":"@scope/pkg（npm 时）","summary":"一句话摘要","args":["可选启动参数"]}
- write_file: {"action":"write_file","path":"相对路径","content":"文件内容"}
- write_launch: {"action":"write_launch","command":"node.exe","args":["相对 appDir 的入口 js","可选参数..."],"dir":"app"}
- list_dir: {"action":"list_dir","path":"."}
- run: {"action":"run","command":"npm|node|echo","args":["..."]}
- done: {"action":"done"}

规则：
1. source=npm 时必须给出 package（npm 包名，如 @modelcontextprotocol/server-filesystem）。
2. source=echo 仅用于明确要求回显/演示时；其它模糊需求不得回退 echo 冒充成功。
3. 3M 会自动下载 Node runtime 并在工作区内执行 npm install；不要要求用户预装 Node。
4. write_launch 只写相对路径，dir 固定为 app。
5. run 最多 3 次，仅用于 npm install/验证；不允许 curl/wget。
6. 完成规划与必要文件后输出 done。`

// InstallAgent 有界多轮安装 Agent（LLM + 沙箱工具）。
type InstallAgent struct {
	LLM      llm.Client
	MaxTurns int
}

// Plan 根据 requirement 产出 Plan；失败时 ok=false，由调用方走规则兜底。
func (a *InstallAgent) Plan(ctx context.Context, dataDir, requirement, constraints string) (Plan, bool) {
	if a.MaxTurns <= 0 {
		a.MaxTurns = 8
	}
	base := EnrichPlan(PlanRequirement(requirement))
	sb, err := sandbox.ForMCP(dataDir, base.MCPID)
	if err != nil {
		return Plan{}, false
	}

	user := fmt.Sprintf("需求: %s\n约束: %s\n预分配 mcp_id: %s\n规则初判: source=%s package=%s url=%s\n工作区已创建。输出 plan 后按需 write_file/write_launch/run，最后 done。",
		requirement, strings.TrimSpace(constraints), base.MCPID, base.SourceKind, base.NPMPackage, base.URL)

	var lastPlan *Plan
	history := ""

	for turn := 0; turn < a.MaxTurns; turn++ {
		prompt := user
		if history != "" {
			prompt += "\n\n## 上轮工具结果\n" + history
		}
		raw, err := a.LLM.ChatJSON(ctx, installSystemPrompt, prompt)
		if err != nil {
			return Plan{}, false
		}
		var act installAction
		if err := json.Unmarshal([]byte(raw), &act); err != nil {
			history = "JSON 解析失败: " + err.Error() + "\n原始: " + raw
			continue
		}
		switch strings.ToLower(strings.TrimSpace(act.Action)) {
		case "plan":
			p := planFromAction(act, base, requirement)
			lastPlan = &p
			history = fmt.Sprintf("已记录 plan: source=%s package=%s", p.SourceKind, p.NPMPackage)
		case "write_launch":
			manifest := LaunchManifest{
				Command: strings.TrimSpace(act.Command),
				Args:    act.Args,
				Dir:     strings.TrimSpace(act.Dir),
				Summary: act.Summary,
			}
			if manifest.Command == "" {
				history = "write_launch 缺少 command"
				continue
			}
			if manifest.Dir == "" {
				manifest.Dir = "."
			}
			if err := sb.Write("launch.json", mustLaunchJSON(manifest)); err != nil {
				history = "write_launch 失败: " + err.Error()
			} else {
				history = "已写入 launch.json"
			}
		case "write_file":
			if err := sb.Write(act.Path, act.Content); err != nil {
				history = "write_file 失败: " + err.Error()
			} else {
				history = "已写入 " + act.Path
			}
		case "list_dir":
			names, err := sb.List(act.Path)
			if err != nil {
				history = "list_dir 失败: " + err.Error()
			} else {
				b, _ := json.Marshal(names)
				history = "目录: " + string(b)
			}
		case "run":
			out, err := sb.Run(ctx, act.Command, act.Args)
			if err != nil {
				history = "run 失败: " + err.Error()
			} else {
				history = "run 输出: " + truncate(out, 2000)
			}
		case "done":
			if lastPlan != nil {
				return EnrichPlan(*lastPlan), true
			}
			history = "done 前须先 plan"
		default:
			history = "未知 action: " + act.Action
		}
	}
	if lastPlan != nil {
		return EnrichPlan(*lastPlan), true
	}
	return Plan{}, false
}

func planFromAction(act installAction, base Plan, requirement string) Plan {
	p := base
	p.Requirement = requirement
	if s := strings.TrimSpace(act.Summary); s != "" {
		p.Summary = s
	}
	switch strings.ToLower(strings.TrimSpace(act.Source)) {
	case "npm":
		p.SourceKind = SourceNPM
		p.Template = TemplateNPM
		p.Source = "downloaded"
		if pkg := strings.TrimSpace(act.Package); pkg != "" {
			p.NPMPackage = pkg
		}
		if len(act.Args) > 0 {
			p.NPMArgs = act.Args
		}
	case "http", "url":
		p.SourceKind = SourceHTTP
		p.Template = TemplateHTTP
		p.Source = "downloaded"
		if u := strings.TrimSpace(act.URL); u != "" {
			p.URL = u
		}
	case "echo", "":
		p.SourceKind = SourceEcho
		p.Template = TemplateEcho
		p.Source = "generated"
	}
	if tpl := Template(strings.TrimSpace(act.Template)); tpl != "" {
		p.Template = tpl
	}
	return EnrichPlan(p)
}

func mustLaunchJSON(m LaunchManifest) string {
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

type installAction struct {
	Action   string   `json:"action"`
	Source   string   `json:"source,omitempty"`
	Template string   `json:"template,omitempty"`
	Package  string   `json:"package,omitempty"`
	URL      string   `json:"url,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Path     string   `json:"path,omitempty"`
	Content  string   `json:"content,omitempty"`
	Command  string   `json:"command,omitempty"`
	Args     []string `json:"args,omitempty"`
	Dir      string   `json:"dir,omitempty"`
}
