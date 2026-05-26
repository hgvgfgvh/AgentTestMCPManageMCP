// Package sandbox 3M 工作区内的有界文件与命令能力（供内部 Agent 使用）。
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Sandbox 限制在单个 MCP 工作目录下。
type Sandbox struct {
	Root string
}

// ForMCP 返回某托管 MCP 目录的沙箱。
func ForMCP(dataDir, mcpID string) (*Sandbox, error) {
	if mcpID == "" {
		return nil, fmt.Errorf("empty mcp_id")
	}
	root := filepath.Join(dataDir, "mcps", mcpID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Sandbox{Root: root}, nil
}

func (s *Sandbox) resolve(rel string) (string, error) {
	rel = strings.TrimSpace(filepath.FromSlash(rel))
	if rel == "" || rel == "." {
		return s.Root, nil
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed")
	}
	clean := filepath.Clean(rel)
	if strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("path escapes sandbox")
	}
	abs := filepath.Join(s.Root, clean)
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes sandbox")
	}
	return abs, nil
}

// Read 读相对路径文件。
func (s *Sandbox) Read(rel string) ([]byte, error) {
	p, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// Write 写相对路径文件。
func (s *Sandbox) Write(rel, content string) error {
	p, err := s.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

// List 列出相对目录下一层名称。
func (s *Sandbox) List(rel string) ([]string, error) {
	p, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out, nil
}

var allowedCommands = map[string]bool{
	"echo": true,
	"npm":  true,
	"npx":  true,
	"node": true,
	"go":   true,
}

// RunAllowed 兼容旧接口。
func (s *Sandbox) RunAllowed(ctx context.Context, name string, args []string) (string, error) {
	return s.Run(ctx, name, args)
}

// Run 在沙箱根目录执行白名单命令。
func (s *Sandbox) Run(ctx context.Context, name string, args []string) (string, error) {
	name = strings.TrimSpace(name)
	base := filepath.Base(name)
	if runtime.GOOS == "windows" {
		base = strings.TrimSuffix(strings.ToLower(base), ".cmd")
		base = strings.TrimSuffix(base, ".exe")
	}
	if !allowedCommands[base] {
		return "", fmt.Errorf("command %q not in sandbox allowlist", name)
	}
	exe, err := exec.LookPath(name)
	if err != nil {
		// Windows: try npm.cmd
		if runtime.GOOS == "windows" {
			exe, err = exec.LookPath(base + ".cmd")
		}
		if err != nil {
			return "", fmt.Errorf("command not found: %s", name)
		}
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = s.Root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("%w: %s", err, truncateOut(buf.String()))
	}
	return buf.String(), nil
}

func truncateOut(s string) string {
	if len(s) > 4000 {
		return s[:4000] + "…"
	}
	return s
}
