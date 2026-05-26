// Package workspace 3M 沙箱工作区路径。
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// Root 3M 数据根目录。
type Root struct {
	DataDir string
}

// Open 确保根目录存在。
func Open(dataDir string) (*Root, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	return &Root{DataDir: dataDir}, nil
}

// MCPDir 单个托管 MCP 的工作目录。
func (r *Root) MCPDir(mcpID string) (string, error) {
	if mcpID == "" {
		return "", fmt.Errorf("empty mcp_id")
	}
	p := filepath.Join(r.DataDir, "mcps", mcpID)
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return p, nil
}

// ManifestPath 安装清单路径。
func (r *Root) ManifestPath(mcpID string) (string, error) {
	dir, err := r.MCPDir(mcpID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "manifest.json"), nil
}

// InstallLogPath 安装流水线日志。
func (r *Root) InstallLogPath(mcpID string) (string, error) {
	dir, err := r.MCPDir(mcpID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "install.log"), nil
}
