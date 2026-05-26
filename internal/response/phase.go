package response

import (
	"os"
	"strings"
)

// PhaseStub 标识 Phase-0 桩实现。
func PhaseStub() string { return "stub-v0" }

// PhaseManager 标识 Phase-1（规则选 tool）。
func PhaseManager() string { return "manager-v1" }

// PhaseAgent 标识 Phase-2 内部 Agent。
func PhaseAgent() string { return "manager-v2-agent" }

// PhaseFull 标识 Phase-3 完整安装（npm/http/launch 解析）。
func PhaseFull() string { return "manager-v3-full" }

// Phase 当前阶段标签（供 JSON 回包）。
func Phase() string {
	return PhaseFull()
}

func agentPhase() bool {
	v := os.Getenv("MCP_MANAGER_AGENT")
	if v == "0" || strings.EqualFold(strings.TrimSpace(v), "false") {
		return false
	}
	return strings.TrimSpace(os.Getenv("MCP_MANAGER_LLM_API_BASE")) != ""
}
