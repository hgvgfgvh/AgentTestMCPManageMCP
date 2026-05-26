package supervisor

import (
	"context"

	"AgentTestMCPManageMCP/internal/registry"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Ping 检查子 MCP 会话是否存活。
func Ping(ctx context.Context, sess *mcp.ClientSession) error {
	return sess.Ping(ctx, nil)
}

// MarkDegraded 将 Registry 条目标为 degraded（不停止进程）。
func MarkDegraded(reg *registry.Store, mcpID string, err error) {
	if reg == nil || mcpID == "" {
		return
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_ = reg.SetStatus(mcpID, "degraded", msg)
}
