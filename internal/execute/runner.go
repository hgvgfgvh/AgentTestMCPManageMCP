package execute

import (
	"context"
	"strings"
	"time"

	"AgentTestMCPManageMCP/internal/childmcp"
	"AgentTestMCPManageMCP/internal/registry"
	"AgentTestMCPManageMCP/internal/supervisor"
)

// Runner 单步代理：连接子 MCP → 选工具 → 调用一次（有界；Phase-1 无内部 LLM 多轮）。
type Runner struct {
	Reg       *registry.Store
	Pool      *supervisor.Pool
	StepAgent *StepAgent
}

// Outcome 执行结果。
type Outcome struct {
	OK         bool
	ToolUsed   string
	Payload    string
	Error      string
	DurationMS int64
}

// Run 执行 execute_step 内部逻辑。
func (r *Runner) Run(ctx context.Context, mcpID, stepGoal, rawdata string) Outcome {
	start := time.Now()
	mcpID = strings.TrimSpace(mcpID)
	if mcpID == "" {
		return Outcome{Error: "missing_mcp_id", DurationMS: since(start)}
	}
	if strings.TrimSpace(stepGoal) == "" {
		return Outcome{Error: "missing_step_goal", DurationMS: since(start)}
	}
	if strings.TrimSpace(rawdata) == "" {
		return Outcome{Error: "missing_rawdata", DurationMS: since(start)}
	}

	rec, ok := r.Reg.Get(mcpID)
	if !ok {
		return Outcome{Error: "unknown_mcp_id", DurationMS: since(start)}
	}
	switch rec.Status {
	case "ready", "degraded":
		// degraded：允许尝试重新拉起子进程（勿仅凭 Registry 快照拒调）
	default:
		return Outcome{Error: "mcp_not_ready:" + rec.Status, DurationMS: since(start)}
	}

	sess, err := r.Pool.EnsureRunning(ctx, rec)
	if err != nil {
		supervisor.MarkDegraded(r.Reg, mcpID, err)
		return Outcome{Error: err.Error(), DurationMS: since(start)}
	}
	if rec.Status == "degraded" {
		_ = r.Reg.SetStatus(mcpID, "ready", "")
	}

	if r.StepAgent != nil {
		toolUsed, payload, err := r.StepAgent.Run(ctx, sess, stepGoal, rawdata)
		if err != nil {
			if payload == "" {
				if tools, e2 := childmcp.ListToolNames(ctx, sess); e2 == nil {
					if t, a, e3 := childmcp.PickTool(tools, stepGoal, rawdata); e3 == nil {
						if p, e4 := childmcp.CallOnce(ctx, sess, t, a); e4 == nil {
							return Outcome{OK: true, ToolUsed: t, Payload: p, DurationMS: since(start)}
						}
					}
				}
			}
			return Outcome{Error: err.Error(), ToolUsed: toolUsed, DurationMS: since(start)}
		}
		return Outcome{
			OK:         true,
			ToolUsed:   toolUsed,
			Payload:    payload,
			DurationMS: since(start),
		}
	}

	tools, err := childmcp.ListToolNames(ctx, sess)
	if err != nil {
		return Outcome{Error: "list_tools: " + err.Error(), DurationMS: since(start)}
	}
	toolName, args, err := childmcp.PickTool(tools, stepGoal, rawdata)
	if err != nil {
		return Outcome{Error: err.Error(), DurationMS: since(start)}
	}
	payload, err := childmcp.CallOnce(ctx, sess, toolName, args)
	if err != nil {
		return Outcome{Error: err.Error(), ToolUsed: toolName, DurationMS: since(start)}
	}
	return Outcome{
		OK:         true,
		ToolUsed:   toolName,
		Payload:    payload,
		DurationMS: since(start),
	}
}

func since(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
