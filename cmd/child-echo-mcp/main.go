// child-echo-mcp：最小子 MCP，供 3M 安装流水线联调（单工具 echo）。
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	log.SetOutput(os.Stderr)
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "child-echo",
		Title:   "Child Echo MCP",
		Version: "0.1.0",
	}, nil)

	type echoArgs struct {
		Input string `json:"input" jsonschema:"required,要回显的文本"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "原样回显 input 字段。",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args echoArgs) (*mcp.CallToolResult, any, error) {
		out, _ := json.Marshal(map[string]string{"echo": args.Input})
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
		}, nil, nil
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
