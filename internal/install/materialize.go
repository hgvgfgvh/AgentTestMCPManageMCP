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
func Materialize(ctx context.Context, plan Plan, mcpDir, managerExe, childEchoBin string) (registry.LaunchSpec, error) {
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
		if err := NPMInstall(ctx, mcpDir, plan.NPMPackage, plan.NPMArgs); err != nil {
			return registry.LaunchSpec{}, err
		}
		return ResolveLaunchFromDir(mcpDir, hints)

	case SourceHTTP:
		url := strings.TrimSpace(plan.URL)
		if url == "" {
			url = ExtractURL(plan.Requirement)
		}
		if url == "" {
			return registry.LaunchSpec{}, fmt.Errorf("http source without url")
		}
		bundle := filepath.Join(mcpDir, bundleNameFromURL(url))
		if err := DownloadHTTP(ctx, url, bundle, downloadAllowlistFromEnv()); err != nil {
			return registry.LaunchSpec{}, err
		}
		extractDir := filepath.Join(mcpDir, "extracted")
		if err := ExtractArchive(bundle, extractDir); err != nil {
			// 非压缩包：保留 bundle，尝试作为 launch.json 同目录资源
			_ = WriteLaunchManifest(filepath.Join(mcpDir, "launch.json"), LaunchManifest{
				Command: ResolveChildBinary(managerExe, "child-echo-mcp"),
				Summary: plan.Summary,
			})
			return ResolveLaunchFromDir(mcpDir, hints)
		}
		_ = FlattenSingleRootDir(extractDir)
		// 将 extracted 内容合并到 mcpDir 根（便于 launch 解析）
		if err := mergeDir(extractDir, mcpDir); err != nil {
			return registry.LaunchSpec{}, err
		}
		return ResolveLaunchFromDir(mcpDir, hints)

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
