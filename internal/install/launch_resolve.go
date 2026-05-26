package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"AgentTestMCPManageMCP/internal/registry"
)

// LaunchHints 物化后解析 launch 的提示。
type LaunchHints struct {
	NPMPackage string
	NPMArgs    []string
	ManagerExe string
	ChildEcho  string
}

// ResolveLaunchFromDir 在工作区内解析子 MCP 启动方式。
func ResolveLaunchFromDir(mcpDir string, hints LaunchHints) (registry.LaunchSpec, error) {
	mcpDir = strings.TrimSpace(mcpDir)
	if mcpDir == "" {
		return registry.LaunchSpec{}, fmt.Errorf("empty mcp dir")
	}

	launchJSON := filepath.Join(mcpDir, "launch.json")
	if m, err := ReadLaunchManifest(launchJSON); err == nil && strings.TrimSpace(m.Command) != "" {
		return launchFromManifest(m, mcpDir)
	}

	if spec, err := resolveFromMCPConfig(mcpDir); err == nil {
		return spec, nil
	}
	if spec, err := resolveFromPackageJSON(mcpDir, hints); err == nil {
		return spec, nil
	}
	if strings.TrimSpace(hints.NPMPackage) != "" {
		return defaultNPXLaunch(mcpDir, hints.NPMPackage, hints.NPMArgs), nil
	}

	// 可执行文件 bin/
	if spec, err := findBinaryLaunch(mcpDir); err == nil {
		return spec, nil
	}

	if hints.ChildEcho != "" || hints.ManagerExe != "" {
		bin := strings.TrimSpace(hints.ChildEcho)
		if bin == "" {
			bin = ResolveChildBinary(hints.ManagerExe, "child-echo-mcp")
		}
		return registry.LaunchSpec{Command: bin, Args: []string{}, Dir: mcpDir}, nil
	}
	return registry.LaunchSpec{}, fmt.Errorf("no launch configuration found under %s", mcpDir)
}

func launchFromManifest(m LaunchManifest, mcpDir string) (registry.LaunchSpec, error) {
	cmd := m.Command
	if !filepath.IsAbs(cmd) {
		cmd = filepath.Join(mcpDir, cmd)
	}
	dir := mcpDir
	if strings.TrimSpace(m.Dir) != "" {
		dir = filepath.Join(mcpDir, m.Dir)
	}
	return registry.LaunchSpec{Command: cmd, Args: m.Args, Dir: dir}, nil
}

// mcpLaunchFile 常见 MCP 安装清单（Smithery / 手工）。
type mcpLaunchFile struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

func resolveFromMCPConfig(mcpDir string) (registry.LaunchSpec, error) {
	for _, name := range []string{"mcp.json", ".mcp.json"} {
		b, err := os.ReadFile(filepath.Join(mcpDir, name))
		if err != nil {
			continue
		}
		var root struct {
			MCP     mcpLaunchFile            `json:"mcp"`
			Servers map[string]mcpLaunchFile `json:"mcpServers"`
			Server  mcpLaunchFile            `json:"server"`
		}
		if err := json.Unmarshal(b, &root); err != nil {
			continue
		}
		candidates := []mcpLaunchFile{root.MCP, root.Server}
		for _, s := range root.Servers {
			candidates = append(candidates, s)
		}
		for _, c := range candidates {
			if strings.TrimSpace(c.Command) != "" {
				return registry.LaunchSpec{
					Command: c.Command,
					Args:    c.Args,
					Dir:     mcpDir,
				}, nil
			}
		}
	}
	return registry.LaunchSpec{}, fmt.Errorf("no mcp.json")
}

func resolveFromPackageJSON(mcpDir string, hints LaunchHints) (registry.LaunchSpec, error) {
	b, err := os.ReadFile(filepath.Join(mcpDir, "package.json"))
	if err != nil {
		return registry.LaunchSpec{}, err
	}
	var pkg struct {
		Name    string            `json:"name"`
		Bin     map[string]string `json:"bin"`
		Scripts map[string]string `json:"scripts"`
		MCP     *mcpLaunchFile    `json:"mcp"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return registry.LaunchSpec{}, err
	}
	if pkg.MCP != nil && strings.TrimSpace(pkg.MCP.Command) != "" {
		return registry.LaunchSpec{Command: pkg.MCP.Command, Args: pkg.MCP.Args, Dir: mcpDir}, nil
	}
	if s, ok := pkg.Scripts["mcp"]; ok && strings.TrimSpace(s) != "" {
		return registry.LaunchSpec{Command: "npm", Args: []string{"run", "mcp"}, Dir: mcpDir}, nil
	}
	if len(pkg.Bin) == 1 {
		for name := range pkg.Bin {
			bin := filepath.Join(mcpDir, "node_modules", ".bin", name)
			if runtime.GOOS == "windows" {
				bin += ".cmd"
			}
			if statFile(bin) {
				return registry.LaunchSpec{Command: bin, Args: hints.NPMArgs, Dir: mcpDir}, nil
			}
		}
	}
	return registry.LaunchSpec{}, fmt.Errorf("package.json has no mcp entry")
}

func defaultNPXLaunch(mcpDir, pkg string, extraArgs []string) registry.LaunchSpec {
	args := []string{"-y", pkg}
	args = append(args, extraArgs...)
	return registry.LaunchSpec{
		Command: "npx",
		Args:    args,
		Dir:     mcpDir,
	}
}

func findBinaryLaunch(mcpDir string) (registry.LaunchSpec, error) {
	binDir := filepath.Join(mcpDir, "bin")
	ents, err := os.ReadDir(binDir)
	if err != nil || len(ents) == 0 {
		return registry.LaunchSpec{}, fmt.Errorf("no bin")
	}
	name := ents[0].Name()
	return registry.LaunchSpec{
		Command: filepath.Join(binDir, name),
		Args:    []string{},
		Dir:     mcpDir,
	}, nil
}
