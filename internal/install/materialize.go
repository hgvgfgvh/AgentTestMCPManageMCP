package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"AgentTestMCPManageMCP/internal/registry"
)

// Materialize 按 Plan 将子 MCP 物化到 mcpDir 并返回 LaunchSpec。
func Materialize(ctx context.Context, plan Plan, dataDir, mcpDir, managerExe, childEchoBin string) (registry.LaunchSpec, error) {
	plan = EnrichPlan(plan)
	hints := LaunchHints{
		NPMPackage: plan.NPMPackage,
		NPMArgs:    plan.NPMArgs,
		ManagerExe: managerExe,
		ChildEcho:  childEchoBin,
	}

	switch plan.SourceKind {
	case SourceNPM:
		if strings.TrimSpace(plan.NPMPackage) == "" {
			return registry.LaunchSpec{}, fmt.Errorf("npm source without package name")
		}
		// Online install, using 3M-managed Node runtime under WS.DataDir.
		if _, _, err := NPMInstallOnline(ctx, dataDir, mcpDir, plan.NPMPackage, plan.NPMArgs); err != nil {
			return registry.LaunchSpec{}, err
		}
		return ResolveLaunchFromDir(mcpDir, hints)

	case SourceHTTP:
		return registry.LaunchSpec{}, fmt.Errorf("http install path disabled: only npm online install is supported")

	case SourceEcho, "":
		bin := strings.TrimSpace(childEchoBin)
		if bin == "" {
			bin = ResolveChildBinary(managerExe, "child-echo-mcp")
		}
		_ = WriteLaunchManifest(filepath.Join(mcpDir, "launch.json"), LaunchManifest{
			Command: bin,
			Args:    []string{},
			Dir:     ".",
			Summary: plan.Summary,
		})
		return registry.LaunchSpec{Command: bin, Args: []string{}, Dir: mcpDir}, nil

	default:
		return registry.LaunchSpec{}, fmt.Errorf("unknown source kind %q", plan.SourceKind)
	}
}

func bundleNameFromURL(u string) string {
	lower := strings.ToLower(u)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return "download.zip"
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "download.tar.gz"
	default:
		return "download.bundle"
	}
}

func downloadAllowlistFromEnv() []string {
	return downloadAllowlist()
}

// mergeDir 将 src 下文件移入 dst（不覆盖已有同名）。
func mergeDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		return os.Rename(path, target)
	})
}
