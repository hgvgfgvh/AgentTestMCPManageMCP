// Package childmcp 对子 MCP 的 tools/list 与 tools/call 封装。
package childmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PickTool 有界选工具：rawdata JSON 指定 tool > 唯一工具 > 名称匹配 step_goal > 名为 echo > 第一个。
func PickTool(tools []*mcp.Tool, stepGoal, rawdata string) (string, map[string]any, error) {
	if len(tools) == 0 {
		return "", nil, fmt.Errorf("child has no tools")
	}
	var hint struct {
		Tool      string         `json:"tool"`
		Arguments map[string]any `json:"arguments"`
	}
	if rawdata != "" {
		_ = json.Unmarshal([]byte(rawdata), &hint)
	}
	if hint.Tool != "" {
		args := hint.Arguments
		if args == nil {
			args = map[string]any{"input": rawdata}
		}
		return hint.Tool, args, nil
	}
	if len(tools) == 1 {
		return tools[0].Name, defaultArgs(tools[0].Name, rawdata), nil
	}
	goal := strings.ToLower(stepGoal)
	for _, t := range tools {
		if strings.Contains(goal, strings.ToLower(t.Name)) {
			return t.Name, defaultArgs(t.Name, rawdata), nil
		}
	}
	for _, t := range tools {
		if t.Name == "echo" {
			return t.Name, map[string]any{"input": rawdata}, nil
		}
	}
	t := tools[0]
	return t.Name, defaultArgs(t.Name, rawdata), nil
}

func defaultArgs(toolName, rawdata string) map[string]any {
	if toolName == "echo" {
		var nested struct {
			Input string `json:"input"`
		}
		if json.Unmarshal([]byte(rawdata), &nested) == nil && nested.Input != "" {
			return map[string]any{"input": nested.Input}
		}
		return map[string]any{"input": rawdata}
	}
	return map[string]any{"input": rawdata}
}

// CallOnce 调用子工具一次，返回原始文本 payload（不改写）。
func CallOnce(ctx context.Context, sess *mcp.ClientSession, toolName string, args map[string]any) (string, error) {
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return "", err
	}
	if res.IsError {
		return "", fmt.Errorf("child tool error: %s", textFromResult(res))
	}
	payload := textFromResult(res)
	if payload == "" {
		b, _ := json.Marshal(res)
		payload = string(b)
	}
	return payload, nil
}

func textFromResult(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// ListToolNames 列出子 MCP 工具名（供 execute 审计）。
func ListToolNames(ctx context.Context, sess *mcp.ClientSession) ([]*mcp.Tool, error) {
	out, err := sess.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	return out.Tools, nil
}
