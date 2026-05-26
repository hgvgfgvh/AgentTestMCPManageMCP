package install

import (
	"context"
	"testing"

	"AgentTestMCPManageMCP/internal/registry"
)

func TestDecideWithRules_Git(t *testing.T) {
	reg, _ := registry.Open(t.TempDir())
	d, ok := decideWithRules(reg.List(), "git管理能力")
	if !ok {
		t.Fatal("expected match")
	}
	if d.Reuse || d.Plan.NPMPackage != "@mseep/git-mcp-server" {
		t.Fatalf("%+v", d)
	}
}

func TestDecideWithRules_ReuseSameRequirement(t *testing.T) {
	reg, _ := registry.Open(t.TempDir())
	req := "git管理能力"
	id := mcpIDFromRequirement(req)
	_ = reg.Upsert(registry.Record{MCPID: id, Summary: "Git", Status: "ready", Requirement: req})
	d, ok := decideWithRules(reg.List(), req)
	if !ok || !d.Reuse || d.MCPID != id {
		t.Fatalf("%+v", d)
	}
}

func TestDecideAdd_MissingRequirement(t *testing.T) {
	reg, _ := registry.Open(t.TempDir())
	_, err := DecideAdd(context.Background(), nil, reg, t.TempDir(), "  ", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
