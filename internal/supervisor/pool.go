// Package supervisor 子 MCP 进程池（stdio CommandTransport）。
package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"AgentTestMCPManageMCP/internal/registry"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Pool 按 mcp_id 持有子 MCP 客户端会话。
type Pool struct {
	mu       sync.Mutex
	sessions map[string]*mcp.ClientSession
	cmds     map[string]*exec.Cmd
}

// NewPool 创建空进程池。
func NewPool() *Pool {
	return &Pool{
		sessions: make(map[string]*mcp.ClientSession),
		cmds:     make(map[string]*exec.Cmd),
	}
}

// EnsureRunning 若未连接则按 LaunchSpec 启动子 MCP 并 Connect。
func (p *Pool) EnsureRunning(ctx context.Context, rec registry.Record) (*mcp.ClientSession, error) {
	if rec.Launch.Command == "" {
		return nil, fmt.Errorf("mcp %q: empty launch command", rec.MCPID)
	}
	p.mu.Lock()
	if s, ok := p.sessions[rec.MCPID]; ok && s != nil {
		p.mu.Unlock()
		return s, nil
	}
	p.mu.Unlock()

	cmd := exec.CommandContext(ctx, rec.Launch.Command, rec.Launch.Args...)
	if rec.Launch.Dir != "" {
		cmd.Dir = rec.Launch.Dir
	}
	cmd.Env = os.Environ()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "mcp-manager-child-client",
		Version: "0.1.0",
	}, nil)
	sess, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect child %q: %w", rec.MCPID, err)
	}
	if err := sess.Ping(ctx, nil); err != nil {
		_ = sess.Close()
		return nil, fmt.Errorf("ping child %q: %w", rec.MCPID, err)
	}

	p.mu.Lock()
	if old, ok := p.sessions[rec.MCPID]; ok && old != nil {
		p.mu.Unlock()
		_ = sess.Close()
		return old, nil
	}
	p.sessions[rec.MCPID] = sess
	p.cmds[rec.MCPID] = cmd
	p.mu.Unlock()
	return sess, nil
}

// Stop 关闭单个子 MCP。
func (p *Pool) Stop(mcpID string) {
	p.mu.Lock()
	sess := p.sessions[mcpID]
	delete(p.sessions, mcpID)
	delete(p.cmds, mcpID)
	p.mu.Unlock()
	if sess != nil {
		_ = sess.Close()
	}
}

// CloseAll 关闭全部子 MCP。
func (p *Pool) CloseAll() {
	p.mu.Lock()
	ids := make([]string, 0, len(p.sessions))
	for id := range p.sessions {
		ids = append(ids, id)
	}
	p.mu.Unlock()
	for _, id := range ids {
		p.Stop(id)
	}
}
