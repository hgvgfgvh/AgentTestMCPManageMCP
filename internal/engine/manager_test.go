package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"AgentTestMCPManageMCP/internal/engine"
	"AgentTestMCPManageMCP/internal/response"
)

func TestManager_AddExecuteEcho(t *testing.T) {
	root := moduleRoot(t)
	tmp := t.TempDir()
	childBin := buildChild(t, root, tmp, "./cmd/child-echo-mcp", "child-echo-mcp")
	t.Setenv("MCP_MANAGER_CHILD_ECHO_BIN", childBin)

	mgr, err := engine.NewManagerEngine(filepath.Join(tmp, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	ctx := context.Background()
	addJSON := mgr.AddManaged(ctx, engine.AddInput{Requirement: "需要 echo 回显"})
	if key, bad := response.AddResponseHasForbiddenFields(addJSON); bad {
		t.Fatalf("forbidden %q", key)
	}
	var add response.AddPayload
	if err := json.Unmarshal([]byte(addJSON), &add); err != nil {
		t.Fatal(err)
	}
	if !add.Accepted {
		t.Fatalf("add: %s", addJSON)
	}

	execJSON := mgr.ExecuteStep(ctx, engine.ExecuteInput{
		MCPID:    add.MCPID,
		StepGoal: "echo",
		Rawdata:  `{"input":"hello-3m"}`,
	})
	var ex response.ExecutePayload
	if err := json.Unmarshal([]byte(execJSON), &ex); err != nil {
		t.Fatal(err)
	}
	if !ex.OK {
		t.Fatalf("execute: %s", execJSON)
	}
	if !stringsContains(ex.Payload, "hello-3m") {
		t.Fatalf("payload: %s", ex.Payload)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func buildChild(t *testing.T, root, outDir, pkg, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	out := filepath.Join(outDir, name)
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, b)
	}
	return out
}

func stringsContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && searchSub(s, sub))
}

func searchSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
