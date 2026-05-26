package sandbox_test

import (
	"testing"

	"AgentTestMCPManageMCP/internal/sandbox"
)

func TestSandbox_pathEscape(t *testing.T) {
	sb, err := sandbox.ForMCP(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if err := sb.Write("../escape.txt", "x"); err == nil {
		t.Fatal("expected escape error")
	}
}
