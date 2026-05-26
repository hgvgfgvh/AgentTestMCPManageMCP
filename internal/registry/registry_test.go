package registry_test

import (
	"path/filepath"
	"testing"

	"AgentTestMCPManageMCP/internal/registry"
)

func TestStore_PersistRevision(t *testing.T) {
	dir := t.TempDir()
	s1, err := registry.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = s1.Upsert(registry.Record{MCPID: "a", Summary: "s", Status: "ready"})
	r1 := s1.Revision()

	s2, err := registry.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Revision() != r1 {
		t.Fatalf("revision %d vs %d", s2.Revision(), r1)
	}
	list := s2.List()
	if len(list) != 1 || list[0].MCPID != "a" {
		t.Fatalf("list: %+v", list)
	}
	_ = filepath.Join(dir, "registry.json")
}
