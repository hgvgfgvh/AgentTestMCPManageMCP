package install

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// NPMInstall 在 mcpDir 安装 npm 包并写入 package.json / launch.json。
func NPMInstall(ctx context.Context, mcpDir, pkg string, extraArgs []string) error {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return fmt.Errorf("empty npm package")
	}
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		return err
	}

	pkgJSON := map[string]any{
		"name":    sanitizePkgName(pkg),
		"private": true,
		"dependencies": map[string]string{
			pkg: "latest",
		},
	}
	b, _ := json.MarshalIndent(pkgJSON, "", "  ")
	if err := os.WriteFile(filepath.Join(mcpDir, "package.json"), b, 0o644); err != nil {
		return err
	}

	npm, err := exec.LookPath("npm")
	if err != nil && runtime.GOOS == "windows" {
		npm, err = exec.LookPath("npm.cmd")
	}
	if err != nil {
		return fmt.Errorf("npm not found in PATH: %w", err)
	}
	if out, err := runCmd(ctx, mcpDir, npm, "install", "--omit=dev", "--no-audit", "--no-fund"); err != nil {
		return fmt.Errorf("npm install: %w\n%s", err, out)
	}

	launch := defaultNPXLaunch(mcpDir, pkg, extraArgs)
	_ = WriteLaunchManifest(filepath.Join(mcpDir, "launch.json"), LaunchManifest{
		Command: launch.Command,
		Args:    launch.Args,
		Dir:     ".",
		Summary: "npm: " + pkg,
	})
	return nil
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

func npmCommand() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

func npxCommand() string {
	if runtime.GOOS == "windows" {
		return "npx.cmd"
	}
	return "npx"
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
