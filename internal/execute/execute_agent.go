package execute

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"AgentTestMCPManageMCP/internal/childmcp"
	"AgentTestMCPManageMCP/internal/llm"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const executeSystemPrompt = `你是 MCPManagerMCP（3M）内部的「单步执行 Agent」。你在一次 execute_step 内有界多轮（无跨 Host 记忆）。
每次只输出一个 JSON 对象：
- call_tool: {"action":"call_tool","tool":"工具名","arguments":{...}}
- finish: {"action":"finish"}

规则：
1. 根据 step_goal 与 rawdata 选择子 MCP 工具并填 arguments。
2. 收到工具结果后，若已成功完成本步，输出 finish；否则可修正再 call_tool。
3. 本步只完成一个逻辑动作；不要要求 Host 补充参数。`

// StepAgent 对子 MCP 的有界 ReAct（LLM 选工具 + 调用）。
type StepAgent struct {
	LLM      llm.Client
	MaxTurns int
}

// Run 多轮直至 finish 或上限；返回子 MCP 原始 payload 文本。
func (a *StepAgent) Run(ctx context.Context, sess *mcp.ClientSession, stepGoal, rawdata string) (toolUsed, payload string, err error) {
	if a.MaxTurns <= 0 {
		a.MaxTurns = 4
	}
	tools, err := childmcp.ListToolNames(ctx, sess)
	if err != nil {
		return "", "", err
	}
	toolsJSON, _ := json.Marshal(tools)

	userBase := fmt.Sprintf("## step_goal\n%s\n\n## rawdata\n%s\n\n## 子 MCP 工具列表（JSON）\n%s\n",
		stepGoal, rawdata, string(toolsJSON))

	history := ""
	var lastTool, lastPayload string

	for turn := 0; turn < a.MaxTurns; turn++ {
		prompt := userBase
		if history != "" {
			prompt += "\n## 工具观测\n" + history
		}
		raw, err := a.LLM.ChatJSON(ctx, executeSystemPrompt, prompt)
		if err != nil {
			return lastTool, lastPayload, err
		}
		var act executeAction
		if err := json.Unmarshal([]byte(raw), &act); err != nil {
			history = "JSON 无效: " + err.Error()
			continue
		}
		switch strings.ToLower(strings.TrimSpace(act.Action)) {
		case "call_tool":
			tool := strings.TrimSpace(act.Tool)
			if tool == "" {
				history = "缺少 tool"
				continue
			}
			args := act.Arguments
			if args == nil {
				args = map[string]any{}
			}
			out, callErr := childmcp.CallOnce(ctx, sess, tool, args)
			lastTool = tool
			lastPayload = out
			if callErr != nil {
				history = fmt.Sprintf("call_tool %s 失败: %v", tool, callErr)
				continue
			}
			history = fmt.Sprintf("call_tool %s 成功，payload:\n%s", tool, out)
		case "finish":
			if lastPayload != "" {
				return lastTool, lastPayload, nil
			}
			history = "finish 前须先成功 call_tool"
		default:
			history = "未知 action: " + act.Action
		}
	}
	if lastPayload != "" {
		return lastTool, lastPayload, nil
	}
	return "", "", fmt.Errorf("execute agent: max turns exceeded")
}

type executeAction struct {
	Action    string         `json:"action"`
	Tool      string         `json:"tool,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}
