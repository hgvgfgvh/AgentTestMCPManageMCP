// 一次性导出 tools/list 原始 JSON（与 MCP 客户端所见一致）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"AgentTestMCPManageMCP/internal/engine"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	eng := engine.NewStubEngine()
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "agent-test-mcp-manager",
		Title:   "AgentTest MCPManagerMCP (3M)",
		Version: "0.1.0-stub",
	}, nil)
	registerLikeMCP(server, eng)

	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		fmt.Fprintf(os.Stderr, "server connect: %v\n", err)
		os.Exit(1)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "dump-client", Version: "0.0.1"}, nil)
	sess, err := client.Connect(ctx, t2, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "client connect: %v\n", err)
		os.Exit(1)
	}
	defer sess.Close()

	res, err := sess.ListTools(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list tools: %v\n", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}

func registerLikeMCP(server *mcp.Server, eng engine.Engine) {
	type listArgs struct {
		CorrelationID string `json:"correlation_id,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "list_managed_mcps",
		Description: `返回当前托管 MCP 的 L1 清单（mcp_id、summary、status、catalog_revision）。
Host 刷新「托管能力」地图的唯一 3M 数据源；不含子工具 schema。Phase-0 为内存桩 Registry。`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: eng.ListManaged(ctx, engine.ListInput{})}}}, nil, nil
	})

	type addArgs struct {
		Requirement   string `json:"requirement" jsonschema:"required,要增加的 MCP 能力描述"`
		Constraints   string `json:"constraints,omitempty"`
		CorrelationID string `json:"correlation_id,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "add_managed_mcp",
		Description: `Host 下发要增加的 MCP 能力需求。Phase-0 桩：仅写入内存 Registry，不下载/生成/启子进程。
同步返回 JSON：accepted、mcp_id、status、message、error；不得含 l1_summary / managed_catalog / catalog_revision。`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args addArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "{}"}}}, nil, nil
	})

	type execArgs struct {
		MCPID         string `json:"mcp_id" jsonschema:"required,托管 MCP 标识"`
		StepGoal      string `json:"step_goal" jsonschema:"required,本步要完成的目标（自然语言）"`
		Rawdata       string `json:"rawdata" jsonschema:"required,本步完整原始输入"`
		CorrelationID string `json:"correlation_id,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "execute_step",
		Description: `指定托管 MCP 完成单步动作。Phase-0 桩：不回问 Host；payload 为模拟子 MCP 原始 JSON 文本。
入参须含 mcp_id、step_goal、rawdata（均 required）。`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args execArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "{}"}}}, nil, nil
	})
}
