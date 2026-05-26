package install

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"AgentTestMCPManageMCP/internal/registry"
	"AgentTestMCPManageMCP/internal/supervisor"
	"AgentTestMCPManageMCP/internal/workspace"
)

// Pipeline 安装/生成流水线（有界步骤，非 Host 可见 Agent）。
type Pipeline struct {
	WS           *workspace.Root
	Reg          *registry.Store
	Pool         *supervisor.Pool
	ManagerExe   string
	ChildEchoBin string // 可选；测试或部署时显式指定 child-echo-mcp 路径
	InstallAgent *InstallAgent
	MaxSteps     int
}

// Result 安装结果。
type Result struct {
	Record registry.Record
	Err    error
}

// Run 执行 add_managed_mcp 内部流水线。
func (p *Pipeline) Run(ctx context.Context, requirement, constraints, correlationID string) Result {
	_ = constraints
	_ = correlationID
	if p.MaxSteps <= 0 {
		p.MaxSteps = 8
	}
	plan := p.resolvePlan(ctx, requirement, constraints)
	if plan.Requirement == "" {
		return Result{Err: fmt.Errorf("missing_requirement")}
	}

	src := plan.Source
	if src == "" {
		src = "generated"
	}
	rec := registry.Record{
		MCPID:       plan.MCPID,
		Summary:     plan.Summary,
		Status:      "pending",
		Source:      src,
		Template:    string(plan.Template),
		Requirement: plan.Requirement,
	}
	_ = p.Reg.Upsert(rec)

	logPath, _ := p.WS.InstallLogPath(plan.MCPID)
	appendLog(logPath, "step 1: plan template=%s mcp_id=%s\n", plan.Template, plan.MCPID)

	mcpDir, err := p.WS.MCPDir(plan.MCPID)
	if err != nil {
		return p.fail(plan.MCPID, err, logPath)
	}

	launch, err := Materialize(ctx, plan, mcpDir, p.ManagerExe, p.ChildEchoBin)
	if err != nil {
		return p.fail(plan.MCPID, err, logPath)
	}
	appendLog(logPath, "step 2: launch command=%s args=%v\n", launch.Command, launch.Args)

	manifest := map[string]any{
		"mcp_id":    plan.MCPID,
		"template":  plan.Template,
		"launch":    launch,
		"installed": time.Now().UTC().Format(time.RFC3339),
	}
	manifestPath, _ := p.WS.ManifestPath(plan.MCPID)
	if b, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		_ = os.WriteFile(manifestPath, b, 0o644)
	}

	rec.Launch = launch
	rec.Status = "pending"
	_ = p.Reg.Upsert(rec)
	appendLog(logPath, "step 3: registry pending\n")

	if _, err := p.Pool.EnsureRunning(ctx, rec); err != nil {
		return p.fail(plan.MCPID, err, logPath)
	}
	appendLog(logPath, "step 4: child mcp running\n")

	rec.Status = "ready"
	rec.LastError = ""
	if err := p.Reg.Upsert(rec); err != nil {
		return Result{Err: err}
	}
	appendLog(logPath, "step 5: ready\n")

	got, _ := p.Reg.Get(plan.MCPID)
	return Result{Record: got}
}

func (p *Pipeline) resolvePlan(ctx context.Context, requirement, constraints string) Plan {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return Plan{}
	}
	var plan Plan
	if p.InstallAgent != nil {
		if pl, ok := p.InstallAgent.Plan(ctx, p.WS.DataDir, requirement, constraints); ok {
			plan = EnrichPlan(pl)
		} else {
			appendLogOnData(p.WS.DataDir, "install-agent: llm plan failed, fallback rules\n")
			plan = PlanRequirement(requirement)
		}
	} else {
		plan = PlanRequirement(requirement)
	}
	return EnrichPlan(plan)
}

func appendLogOnData(dataDir, msg string) {
	_ = os.MkdirAll(dataDir, 0o755)
	appendLog(filepath.Join(dataDir, "install-agent.log"), "%s", msg)
}

func downloadAllowlist() []string {
	raw := strings.TrimSpace(os.Getenv("MCP_MANAGER_DOWNLOAD_HOSTS"))
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func (p *Pipeline) fail(mcpID string, err error, logPath string) Result {
	appendLog(logPath, "failed: %v\n", err)
	rec, ok := p.Reg.Get(mcpID)
	if ok {
		rec.Status = "failed"
		rec.LastError = err.Error()
		_ = p.Reg.Upsert(rec)
		return Result{Record: rec, Err: err}
	}
	return Result{Err: err}
}

func appendLog(path, format string, args ...any) {
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, format, args...)
}

// ManagerExecutable 返回当前 3M 进程可执行路径。
func ManagerExecutable() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(exe)
}
