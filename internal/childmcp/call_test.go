package childmcp_test

import (
	"testing"

	"AgentTestMCPManageMCP/internal/childmcp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPickTool_explicit(t *testing.T) {
	tools := []*mcp.Tool{{Name: "echo"}, {Name: "other"}}
	name, args, err := childmcp.PickTool(tools, "goal", `{"tool":"other","arguments":{"input":"x"}}`)
	if err != nil || name != "other" {
		t.Fatalf("got %s %v %v", name, args, err)
	}
}

func TestPickTool_single(t *testing.T) {
	tools := []*mcp.Tool{{Name: "echo"}}
	name, _, err := childmcp.PickTool(tools, "anything", `plain`)
	if err != nil || name != "echo" {
		t.Fatalf("got %s err=%v", name, err)
	}
}
