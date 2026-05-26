package engine

import (
	"context"
	"fmt"
	"os"
	"strings"

	"AgentTestMCPManageMCP/internal/audit"
	"AgentTestMCPManageMCP/internal/execute"
	"AgentTestMCPManageMCP/internal/guard"
	"AgentTestMCPManageMCP/internal/install"
	"AgentTestMCPManageMCP/internal/llm"
	"AgentTestMCPManageMCP/internal/registry"
	"AgentTestMCPManageMCP/internal/response"
	"AgentTestMCPManageMCP/internal/supervisor"
	"AgentTestMCPManageMCP/internal/workspace"
)

// ManagerEngine 3M 核心：Registry、安装流水线、子 MCP 单步代理、内部 Agent、审计与防火层。
type ManagerEngine struct {
	reg    *registry.Store
	ws     *workspace.Root
	pool   *supervisor.Pool
	pipe   *install.Pipeline
	runner *execute.Runner
	guard  guard.Guard
	audit  *audit.Logger
}

// NewManagerEngine 打开数据目录并初始化子系统。
func NewManagerEngine(dataDir string) (*ManagerEngine, error) {
	ws, err := workspace.Open(dataDir)
	if err != nil {
		return nil, err
	}
	reg, err := registry.Open(ws.DataDir)
	if err != nil {
		return nil, err
	}
	aud, _ := audit.Open(ws.DataDir)
	pool := supervisor.NewPool()
	g := guard.LoadFromEnv()
	pipe := &install.Pipeline{
		WS:           ws,
		Reg:          reg,
		Pool:         pool,
		ManagerExe:   install.ManagerExecutable(),
		ChildEchoBin: strings.TrimSpace(os.Getenv("MCP_MANAGER_CHILD_ECHO_BIN")),
	}
	runner := &execute.Runner{Reg: reg, Pool: pool}
	if c, ok := llm.ConfigFromEnv(); ok && llmAgentEnabled() {
		pipe.InstallAgent = &install.InstallAgent{LLM: c, MaxTurns: agentMaxTurns("MCP_MANAGER_INSTALL_MAX_TURNS", 6)}
		runner.StepAgent = &execute.StepAgent{LLM: c, MaxTurns: agentMaxTurns("MCP_MANAGER_EXECUTE_MAX_TURNS", 4)}
	}
	e := &ManagerEngine{
		reg:    reg,
		ws:     ws,
		pool:   pool,
		pipe:   pipe,
		runner: runner,
		guard:  g,
		audit:  aud,
	}
	return e, nil
}

// HookCatalogRefresh Registry revision 变更时回调（由 main 绑定 catalog.Resource.Refresh）。
func (e *ManagerEngine) HookCatalogRefresh(fn func()) {
	e.reg.SetOnChange(func(uint64) {
		if fn != nil {
			fn()
		}
	})
}

// ListManagedJSON 供 catalog 资源读取。
func (e *ManagerEngine) ListManagedJSON(ctx context.Context) string {
	return e.ListManaged(ctx, ListInput{})
}

// Close 停止全部子 MCP。
func (e *ManagerEngine) Close() {
	e.pool.CloseAll()
}

func (e *ManagerEngine) ListManaged(ctx context.Context, in ListInput) string {
	_ = ctx
	_ = in
	recs := e.reg.List()
	managed := make([]response.ManagedEntry, 0, len(recs))
	for _, r := range recs {
		managed = append(managed, response.ManagedEntry{
			MCPID:   r.MCPID,
			Summary: r.Summary,
			Status:  r.Status,
			Source:  r.Source,
		})
	}
	out := response.FormatList(response.ListPayload{
		CatalogRevision: e.reg.Revision(),
		Managed:         managed,
		Message:         "registry snapshot",
		Phase:           response.Phase(),
	})
	if e.audit != nil {
		e.audit.Log("list_managed_mcps", map[string]any{
			"revision": e.reg.Revision(),
			"count":    len(managed),
		})
	}
	return out
}

func (e *ManagerEngine) AddManaged(ctx context.Context, in AddInput) string {
	if err := e.guard.CheckAuth(); err != nil {
		return response.FormatAdd(response.AddPayload{
			Accepted: false, Status: "failed", Message: err.Error(), Error: "auth", Phase: response.Phase(),
		})
	}
	if err := e.guard.CheckAdd(in.Requirement); err != nil {
		return response.FormatAdd(response.AddPayload{
			Accepted: false, Status: "failed", Message: err.Error(), Error: "validation", Phase: response.Phase(),
		})
	}
	res := e.pipe.Run(ctx, in.Requirement, in.Constraints, in.CorrelationID)
	if e.audit != nil {
		e.audit.Log("add_managed_mcp", map[string]any{
			"correlation_id": in.CorrelationID,
			"mcp_id":         res.Record.MCPID,
			"status":         res.Record.Status,
			"error":          errString(res.Err),
		})
	}
	if res.Err != nil && res.Record.MCPID == "" {
		return response.FormatAdd(response.AddPayload{
			Accepted: false, Status: "failed", Message: res.Err.Error(), Error: "install_failed", Phase: response.Phase(),
		})
	}
	rec := res.Record
	status := rec.Status
	if status == "" {
		status = "failed"
	}
	accepted := res.Err == nil && (status == "ready" || res.Reused)
	msg := "installed and child mcp ready"
	if res.Reused {
		msg = res.Message
		if msg == "" {
			msg = "reused existing managed mcp"
		}
	} else if res.Err != nil {
		msg = res.Err.Error()
	}
	return response.FormatAdd(response.AddPayload{
		Accepted: accepted,
		MCPID:    rec.MCPID,
		Status:   status,
		Message:  msg,
		Error:    errString(res.Err),
		Phase:    response.Phase(),
	})
}

func (e *ManagerEngine) ExecuteStep(ctx context.Context, in ExecuteInput) string {
	if err := e.guard.CheckAuth(); err != nil {
		return response.FormatExecute(response.ExecutePayload{
			OK: false, MCPID: in.MCPID, Error: "auth", Phase: response.Phase(),
		})
	}
	if err := e.guard.CheckExecute(in.Rawdata); err != nil {
		return response.FormatExecute(response.ExecutePayload{
			OK: false, MCPID: in.MCPID, Error: err.Error(), Phase: response.Phase(),
		})
	}
	out := e.runner.Run(ctx, in.MCPID, in.StepGoal, in.Rawdata)
	if e.audit != nil {
		e.audit.Log("execute_step", map[string]any{
			"correlation_id": in.CorrelationID,
			"mcp_id":         in.MCPID,
			"ok":             out.OK,
			"tool_used":      out.ToolUsed,
			"error":          out.Error,
			"duration_ms":    out.DurationMS,
		})
	}
	return response.FormatExecute(response.ExecutePayload{
		OK:         out.OK,
		MCPID:      strings.TrimSpace(in.MCPID),
		ToolUsed:   out.ToolUsed,
		Payload:    out.Payload,
		Error:      out.Error,
		DurationMS: out.DurationMS,
		Phase:      response.Phase(),
	})
}

func llmAgentEnabled() bool {
	v := strings.TrimSpace(os.Getenv("MCP_MANAGER_AGENT"))
	if v == "0" || strings.EqualFold(v, "false") {
		return false
	}
	return true
}

func agentMaxTurns(env string, def int) int {
	v := strings.TrimSpace(os.Getenv(env))
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
		return n
	}
	return def
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
