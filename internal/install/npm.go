package install

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NPMInstallOnline installs an npm MCP package into the managed workspace and
// writes a launch.json that uses the 3M-managed Node runtime (no system node/npm required).
//
// Layout:
// - mcpDir/app/   : npm project (package.json + node_modules)
// - mcpDir/launch.json : command=node.exe, args=[entry.js, ...]
func NPMInstallOnline(ctx context.Context, dataDir, mcpDir, pkg string, extraArgs []string) (NodeRuntimeSpec, string, error) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return NodeRuntimeSpec{}, "", fmt.Errorf("empty npm package")
	}
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		return NodeRuntimeSpec{}, "", err
	}
	appDir := filepath.Join(mcpDir, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return NodeRuntimeSpec{}, "", err
	}

	pkgJSON := map[string]any{
		"name":    sanitizePkgName(pkg),
		"private": true,
		"dependencies": map[string]string{
			pkg: "latest",
		},
	}
	b, _ := json.MarshalIndent(pkgJSON, "", "  ")
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), b, 0o644); err != nil {
		return NodeRuntimeSpec{}, "", err
	}

	rt, err := EnsureNodeRuntime(ctx, dataDir)
	if err != nil {
		return NodeRuntimeSpec{}, "", err
	}
	if out, err := runCmd(ctx, appDir, rt.NPMCmd, "install", "--omit=dev", "--no-audit", "--no-fund"); err != nil {
		return NodeRuntimeSpec{}, "", fmt.Errorf("npm install: %w\n%s", err, out)
	}

	entry, err := resolveNodeMCPEntry(appDir, pkg)
	if err != nil {
		return NodeRuntimeSpec{}, "", err
	}
	// launch.json uses node.exe directly for stable stdio behavior.
	_ = WriteLaunchManifest(filepath.Join(mcpDir, "launch.json"), LaunchManifest{
		Command: rt.NodeExe,
		Args:    append([]string{entry}, extraArgs...),
		Dir:     "app",
		Summary: "npm: " + pkg,
	})
	return rt, entry, nil
}

func sanitizePkgName(pkg string) string {
	s := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, pkg)
	if s == "" {
		return "managed-mcp"
	}
	if len(s) > 48 {
		return s[:48]
	}
	return s
}

func runCmd(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// LookPath 解析可执行文件（供测试）。
func LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func resolveNodeMCPEntry(appDir, pkg string) (string, error) {
	// Prefer the package's "main" if present; otherwise try common MCP dist entry.
	pkgDir := filepath.Join(appDir, "node_modules", filepath.FromSlash(pkg))
	// Common layouts
	candidates := []string{
		filepath.Join(pkgDir, "dist", "index.js"),
		filepath.Join(pkgDir, "build", "index.js"),
		filepath.Join(pkgDir, "index.js"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			// launch.json is relative to mcpDir/app
			rel, _ := filepath.Rel(appDir, c)
			return filepath.ToSlash(rel), nil
		}
	}
	// Fall back to reading package.json main
	b, err := os.ReadFile(filepath.Join(pkgDir, "package.json"))
	if err == nil {
		var pj struct {
			Main string `json:"main"`
		}
		if json.Unmarshal(b, &pj) == nil && strings.TrimSpace(pj.Main) != "" {
			mainPath := filepath.Join(pkgDir, filepath.FromSlash(strings.TrimSpace(pj.Main)))
			if _, err := os.Stat(mainPath); err == nil {
				rel, _ := filepath.Rel(appDir, mainPath)
				return filepath.ToSlash(rel), nil
			}
		}
	}
	return "", fmt.Errorf("cannot resolve node entry for %q (looked under %s)", pkg, pkgDir)
}
