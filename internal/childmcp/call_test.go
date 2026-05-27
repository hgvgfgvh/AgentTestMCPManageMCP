package childmcp_test

import (
	"testing"

	"AgentTestMCPManageMCP/internal/childmcp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPickTool_ignoresToolInRawdata(t *testing.T) {
	tools := []*mcp.Tool{{Name: "echo"}, {Name: "other"}}
	name, _, err := childmcp.PickTool(tools, "goal", `{"tool":"other","arguments":{"input":"x"}}`)
	if err != nil || name != "echo" {
		t.Fatalf("rawdata tool hint must be ignored: got %s err=%v", name, err)
	}
}

func TestPickTool_goalNameMatch(t *testing.T) {
	tools := []*mcp.Tool{{Name: "echo"}, {Name: "other"}}
	name, _, err := childmcp.PickTool(tools, "call other on repo", `{"path":"/tmp"}`)
	if err != nil || name != "other" {
		t.Fatalf("got %s err=%v", name, err)
	}
}

func TestPickTool_single(t *testing.T) {
	tools := []*mcp.Tool{{Name: "echo"}}
	name, _, err := childmcp.PickTool(tools, "anything", `plain`)
	if err != nil || name != "echo" {
		t.Fatalf("got %s err=%v", name, err)
	}
}
