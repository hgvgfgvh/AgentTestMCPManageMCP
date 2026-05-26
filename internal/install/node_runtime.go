package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// NodeRuntimeSpec is the resolved Node runtime entrypoints.
// All paths are absolute.
type NodeRuntimeSpec struct {
	RootDir string
	NodeExe string
	NPMCmd  string
	NPXCmd  string
}

// EnsureNodeRuntime makes sure 3M has a usable Node runtime under dataDir/runtime/.
//
// Defaults:
// - version: v22.13.1 (override with MCP_MANAGER_NODE_VERSION, accepts "22.13.1" or "v22.13.1")
// - platform: current runtime (currently supports windows only)
//
// Disable auto download by setting MCP_MANAGER_NODE_DISABLE_DOWNLOAD=1/true.
func EnsureNodeRuntime(ctx context.Context, dataDir string) (NodeRuntimeSpec, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if runtime.GOOS != "windows" {
		return NodeRuntimeSpec{}, fmt.Errorf("node runtime: only windows is supported currently (GOOS=%s)", runtime.GOOS)
	}
	ver := strings.TrimSpace(os.Getenv("MCP_MANAGER_NODE_VERSION"))
	if ver == "" {
		ver = "v22.13.1"
	}
	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}

	rtDir := filepath.Join(dataDir, "runtime", "node-"+ver)
	nodeExe := filepath.Join(rtDir, "node.exe")
	npmCmd := filepath.Join(rtDir, "npm.cmd")
	npxCmd := filepath.Join(rtDir, "npx.cmd")
	if fileExistsAbs(nodeExe) && fileExistsAbs(npmCmd) && fileExistsAbs(npxCmd) {
		return NodeRuntimeSpec{RootDir: rtDir, NodeExe: nodeExe, NPMCmd: npmCmd, NPXCmd: npxCmd}, nil
	}

	if downloadDisabled() {
		return NodeRuntimeSpec{}, fmt.Errorf("node runtime missing and auto download disabled; expected %s", rtDir)
	}
	if err := os.MkdirAll(rtDir, 0o755); err != nil {
		return NodeRuntimeSpec{}, err
	}

	zipURL := fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-win-x64.zip", ver, ver)
	zipPath := filepath.Join(rtDir, "node.zip")
	if err := DownloadHTTP(ctx, zipURL, zipPath, []string{"nodejs.org"}); err != nil {
		return NodeRuntimeSpec{}, err
	}
	extractDir := filepath.Join(rtDir, "_extract")
	_ = os.RemoveAll(extractDir)
	if err := ExtractArchive(zipPath, extractDir); err != nil {
		return NodeRuntimeSpec{}, err
	}
	_ = FlattenSingleRootDir(extractDir)
	if err := moveDirContents(extractDir, rtDir); err != nil {
		return NodeRuntimeSpec{}, err
	}
	_ = os.RemoveAll(extractDir)
	_ = os.Remove(zipPath)

	if !fileExistsAbs(nodeExe) || !fileExistsAbs(npmCmd) || !fileExistsAbs(npxCmd) {
		return NodeRuntimeSpec{}, fmt.Errorf("node runtime downloaded but missing entrypoints under %s", rtDir)
	}
	return NodeRuntimeSpec{RootDir: rtDir, NodeExe: nodeExe, NPMCmd: npmCmd, NPXCmd: npxCmd}, nil
}

func downloadDisabled() bool {
	v := strings.TrimSpace(os.Getenv("MCP_MANAGER_NODE_DISABLE_DOWNLOAD"))
	return v == "1" || strings.EqualFold(v, "true")
}

func fileExistsAbs(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// moveDirContents moves all children of srcDir into dstDir (non-destructive; overwrites are skipped).
func moveDirContents(srcDir, dstDir string) error {
	ents, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		old := filepath.Join(srcDir, e.Name())
		newPath := filepath.Join(dstDir, e.Name())
		if _, err := os.Stat(newPath); err == nil {
			continue
		}
		if err := os.Rename(old, newPath); err != nil {
			return err
		}
	}
	return nil
}
