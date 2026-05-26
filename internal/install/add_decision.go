package install

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"AgentTestMCPManageMCP/internal/registry"
)

// AddDecision add 流水线决策：复用已有或安装新 MCP。
type AddDecision struct {
	Reuse   bool
	MCPID   string
	Plan    Plan
	Message string
}

const addDecisionSystemPrompt = `你是 MCPManagerMCP（3M）内部的「安装决策 Agent」。Host 只提供自然语言需求；你根据「已有托管能力」决定复用或安装。

每次只输出一个 JSON（不要 markdown），action 取值：
- reuse: {"action":"reuse","mcp_id":"已有托管 id","message":"为何复用"}
- install: {"action":"install","package":"npm包名","summary":"一句话摘要","args":["可选启动参数"]}
- fail: {"action":"fail","message":"无法映射到可安装 npm 包时说明原因"}

规则：
1. 若需求与已有 managed 能力语义重叠，优先 reuse，不要重复安装。
2. 不重叠时 install 必须给出真实 npm 包名，并从 requirement/constraints 提取启动必备配置写入 args（如 filesystem 目录、sqlite 的 .db 路径）。
3. 若 requirement 未提供该 MCP 类型必备的基本配置，输出 fail 并说明缺什么（不要装成 ready 但不可用）。
4. 统一在线 npm + 3M 自带 Node；不要要求 Host 传 JSON 或包名。
5. 禁止用 echo 演示冒充 git/filesystem/sqlite 等真实能力。`

// DecideAdd 结合 Registry 与 LLM（可选）决策。
func DecideAdd(ctx context.Context, agent *InstallAgent, reg *registry.Store, dataDir, requirement, constraints string) (AddDecision, error) {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return AddDecision{}, fmt.Errorf("missing_requirement")
	}
	managed := reg.List()
	if agent != nil {
		if d, ok := agent.decideWithLLM(ctx, managed, requirement, constraints); ok {
			return normalizeDecision(d, requirement)
		}
	}
	if d, ok := decideWithRules(managed, requirement); ok {
		return normalizeDecision(d, requirement)
	}
	return AddDecision{}, fmt.Errorf("cannot resolve capability for requirement %q (configure MCP_MANAGER_LLM_API_BASE or use clearer wording)", requirement)
}

func (a *InstallAgent) decideWithLLM(ctx context.Context, managed []registry.Record, requirement, constraints string) (AddDecision, bool) {
	managedJSON, _ := json.Marshal(managedSummary(managed))
	user := fmt.Sprintf("## 外部需求\n%s\n\n## 约束\n%s\n\n## 当前已有托管能力（Registry）\n%s\n",
		requirement, strings.TrimSpace(constraints), string(managedJSON))
	raw, err := a.LLM.ChatJSON(ctx, addDecisionSystemPrompt, user)
	if err != nil {
		return AddDecision{}, false
	}
	var act addDecisionAction
	if err := json.Unmarshal([]byte(raw), &act); err != nil {
		return AddDecision{}, false
	}
	switch strings.ToLower(strings.TrimSpace(act.Action)) {
	case "reuse":
		if strings.TrimSpace(act.MCPID) == "" {
			return AddDecision{}, false
		}
		return AddDecision{Reuse: true, MCPID: act.MCPID, Message: strings.TrimSpace(act.Message)}, true
	case "install":
		if strings.TrimSpace(act.Package) == "" {
			return AddDecision{}, false
		}
		p := Plan{
			MCPID:       mcpIDFromRequirement(requirement),
			Requirement: requirement,
			SourceKind:  SourceNPM,
			Template:    TemplateNPM,
			Source:      "downloaded",
			NPMPackage:  strings.TrimSpace(act.Package),
			NPMArgs:     act.Args,
			Summary:     strings.TrimSpace(act.Summary),
		}
		if p.Summary == "" {
			p.Summary = "MCP：" + p.NPMPackage
		}
		return AddDecision{Plan: p}, true
	case "fail":
		msg := strings.TrimSpace(act.Message)
		if msg == "" {
			msg = "install decision failed"
		}
		return AddDecision{}, false
	default:
		return AddDecision{}, false
	}
}

func decideWithRules(managed []registry.Record, requirement string) (AddDecision, bool) {
	lower := strings.ToLower(requirement)
	if strings.Contains(lower, "echo") || strings.Contains(requirement, "回显") {
		wantID := mcpIDFromRequirement(requirement)
		for _, r := range managed {
			if r.MCPID == wantID && r.Status == "ready" {
				return AddDecision{Reuse: true, MCPID: r.MCPID, Message: "reused echo demo"}, true
			}
		}
		return AddDecision{Plan: Plan{
			MCPID: wantID, Requirement: requirement, SourceKind: SourceEcho,
			Template: TemplateEcho, Source: "generated", Summary: "Echo 回显 MCP",
		}}, true
	}

	// 复用：同 requirement 或同包名且 ready
	wantID := mcpIDFromRequirement(requirement)
	for _, r := range managed {
		if r.MCPID == wantID && r.Status == "ready" {
			return AddDecision{Reuse: true, MCPID: r.MCPID, Message: "same requirement already managed"}, true
		}
	}
	entry, ok := matchCatalog(requirement)
	if !ok {
		return AddDecision{}, false
	}
	if entry.Package == "" {
		return AddDecision{}, false
	}
	for _, r := range managed {
		if r.Status != "ready" {
			continue
		}
		blob := strings.ToLower(r.Summary + " " + r.Requirement)
		for _, kw := range entry.Keywords {
			if strings.Contains(blob, strings.ToLower(kw)) {
				return AddDecision{Reuse: true, MCPID: r.MCPID, Message: "catalog overlap with existing"}, true
			}
		}
	}
	p := Plan{
		MCPID:       wantID,
		Requirement: requirement,
		SourceKind:  SourceNPM,
		Template:    TemplateNPM,
		Source:      "downloaded",
		NPMPackage:  entry.Package,
		Summary:     entry.Summary,
	}
	return AddDecision{Plan: p}, true
}

func normalizeDecision(d AddDecision, requirement string) (AddDecision, error) {
	if d.Reuse {
		if d.Message == "" {
			d.Message = "reused existing managed mcp"
		}
		return d, nil
	}
	if d.Plan.SourceKind == SourceEcho {
		if d.Plan.MCPID == "" {
			d.Plan.MCPID = mcpIDFromRequirement(requirement)
		}
		d.Plan.Requirement = requirement
		return d, nil
	}
	if strings.TrimSpace(d.Plan.NPMPackage) == "" {
		return AddDecision{}, fmt.Errorf("no npm package resolved")
	}
	if d.Plan.MCPID == "" {
		d.Plan.MCPID = mcpIDFromRequirement(requirement)
	}
	d.Plan.Requirement = requirement
	d.Plan.SourceKind = SourceNPM
	d.Plan.Template = TemplateNPM
	d.Plan.Source = "downloaded"
	if d.Plan.Summary == "" {
		d.Plan.Summary = "MCP：" + d.Plan.NPMPackage
	}
	return d, nil
}

func managedSummary(managed []registry.Record) []map[string]string {
	out := make([]map[string]string, 0, len(managed))
	for _, r := range managed {
		out = append(out, map[string]string{
			"mcp_id":      r.MCPID,
			"summary":     r.Summary,
			"status":      r.Status,
			"source":      r.Source,
			"requirement": r.Requirement,
		})
	}
	return out
}

type addDecisionAction struct {
	Action  string   `json:"action"`
	MCPID   string   `json:"mcp_id,omitempty"`
	Package string   `json:"package,omitempty"`
	Summary string   `json:"summary,omitempty"`
	Args    []string `json:"args,omitempty"`
	Message string   `json:"message,omitempty"`
}
