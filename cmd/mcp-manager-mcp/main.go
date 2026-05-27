// mcp-manager-mcp：MCPManagerMCP（3M）— 外挂 MCP 安装/配置/单步代理。
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"AgentTestMCPManageMCP/internal/catalog"
	"AgentTestMCPManageMCP/internal/debugui"
	"AgentTestMCPManageMCP/internal/engine"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	httpAddr    = flag.String("http", "", "Streamable HTTP MCP")
	dataDir     = flag.String("data", "", "MCP_MANAGER_DATA_DIR（默认 ./data）")
	engineKind  = flag.String("engine", "", "manager（默认）| stub")
	debugUI     = flag.String("debug-ui", "", "调试 WebUI：1 开启（默认）| 0 关闭")
	debugUIAddr = flag.String("debug-ui-addr", "", "调试 WebUI 地址（默认 127.0.0.1:18094）")
)

func main() {
	flag.Parse()
	log.SetOutput(os.Stderr)

	dir := resolveDataDir()
	kind := resolveEngineKind()

	var eng engine.Engine
	var closer func()
	var catalogRefresh func()

	switch strings.ToLower(kind) {
	case "stub":
		eng = engine.NewStubEngine()
		closer = func() {}
		log.Printf("[mcp-manager-mcp] engine=stub data_dir=%s", dir)
	default:
		mgr, err := engine.NewManagerEngine(dir)
		if err != nil {
			log.Fatalf("manager engine: %v", err)
		}
		eng = mgr
		closer = mgr.Close
		log.Printf("[mcp-manager-mcp] engine=manager data_dir=%s agent=%v", dir, os.Getenv("MCP_MANAGER_LLM_API_BASE") != "")
	}
	defer closer()

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	if debugUIEnabled() {
		debugui.Start(ctx, debugui.Config{
			Addr:   resolveDebugUIAddr(),
			Engine: eng,
		})
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "agent-test-mcp-manager",
		Title:   "AgentTest MCPManagerMCP (3M)",
		Version: "0.5.0",
	}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{
			Tools:     &mcp.ToolCapabilities{ListChanged: true},
			Resources: &mcp.ResourceCapabilities{ListChanged: true},
		},
	})
	registerTools(server, eng)

	if mgr, ok := eng.(*engine.ManagerEngine); ok {
		catRes := catalog.New(mgr)
		catRes.Register(server)
		catalogRefresh = catRes.Refresh
		mgr.HookCatalogRefresh(catalogRefresh)
	}

	if addr := trim(*httpAddr); addr != "" {
		mux := http.NewServeMux()
		mux.Handle("/", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, nil))
		log.Printf("[mcp-manager-mcp] streamable HTTP on %s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("http: %v", err)
		}
		return
	}

	t := &mcp.LoggingTransport{Transport: &mcp.StdioTransport{}, Writer: os.Stderr}
	log.Printf("[mcp-manager-mcp] stdio transport")
	if err := server.Run(context.Background(), t); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func resolveDataDir() string {
	if d := trim(*dataDir); d != "" {
		return d
	}
	if d := trim(os.Getenv("MCP_MANAGER_DATA_DIR")); d != "" {
		return d
	}
	return "data"
}

func resolveEngineKind() string {
	if k := trim(*engineKind); k != "" {
		return k
	}
	if k := trim(os.Getenv("MCP_MANAGER_ENGINE")); k != "" {
		return k
	}
	return "manager"
}

func debugUIEnabled() bool {
	if v := trim(*debugUI); v != "" {
		return v != "0" && !strings.EqualFold(v, "false")
	}
	v := trim(os.Getenv("MCP_MANAGER_DEBUG_UI"))
	if v == "0" || strings.EqualFold(v, "false") {
		return false
	}
	return true
}

func resolveDebugUIAddr() string {
	if a := trim(*debugUIAddr); a != "" {
		return a
	}
	if a := trim(os.Getenv("MCP_MANAGER_DEBUG_UI_ADDR")); a != "" {
		return a
	}
	return "127.0.0.1:18094"
}

func registerTools(server *mcp.Server, eng engine.Engine) {
	type listArgs struct {
		CorrelationID string `json:"correlation_id,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "list_managed_mcps",
		Description: `返回当前托管 MCP 的 L1 清单（mcp_id、summary、status、catalog_revision）。
Host 刷新「托管能力」地图的唯一 3M 数据源；不含子工具 schema。`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listArgs) (*mcp.CallToolResult, any, error) {
		out := eng.ListManaged(ctx, engine.ListInput{CorrelationID: args.CorrelationID})
		return textResult(out), nil, nil
	})

	type addArgs struct {
		Requirement   string `json:"requirement" jsonschema:"required,要什么 MCP 能力，并写明该能力启动必备的基本配置（如目录路径、数据库文件、token 等），自然语言即可"`
		Constraints   string `json:"constraints,omitempty" jsonschema:"可选，补充配置或约束（如只读、版本、网络）；会一并交给内部安装 Agent"`
		CorrelationID string `json:"correlation_id,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "add_managed_mcp",
		Description: `Host 下发要增加的 MCP 能力及启动必备的基本配置（自然语言，写在 requirement；补充写 constraints）。

requirement 应同时包含：
· 能力类型（如 git / 文件系统 / sqlite / github）
· 该 MCP 启动所需的基本配置（常见为 launch args：目录路径、.db 文件路径、仓库路径等；敏感项如 token 可写 constraints）

示例 requirement：
· 「git 管理能力，默认仓库目录 C:\repo」
· 「sqlite MCP，数据库文件 C:\data\app.db」
· 「文件系统 MCP，允许访问 C:\DATA\GODATA\AgentTest\WorkSpace」

内部：读取 Registry → LLM 判重叠/选型 → 将基本配置写入 launch args → 在线 npm 安装（3M 自带 Node）→ 热加载。
同步返回 accepted、mcp_id、status、message、error；不得含 catalog 字段。`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args addArgs) (*mcp.CallToolResult, any, error) {
		out := eng.AddManaged(ctx, engine.AddInput{
			Requirement: args.Requirement, Constraints: args.Constraints, CorrelationID: args.CorrelationID,
		})
		return textResult(out), nil, nil
	})

	type execArgs struct {
		MCPID         string `json:"mcp_id" jsonschema:"required,托管 MCP 标识"`
		StepGoal      string `json:"step_goal" jsonschema:"required,本步要完成的目标（自然语言）"`
		Rawdata       string `json:"rawdata" jsonschema:"required,本步参考数据（自然语言或任意文本/JSON）；勿用于指定子 MCP 工具名"`
		CorrelationID string `json:"correlation_id,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "execute_step",
		Description: `指定托管 MCP 完成单步动作：连接子 MCP →（内部 Agent 或规则）按工具列表选型 → 调用 → 原样返回 payload。

mcp_id：托管 MCP 标识（由 list_managed_mcps 或 add_managed_mcp 得到）。
step_goal：本步目标（自然语言）；内部 Agent 据此在子 MCP 工具列表中选工具。
rawdata：仅供完成本步的参考数据（路径、业务参数、片段文本等），无固定格式，不必传 {"tool":...}；子工具名以 3M 拉取的列表为准，rawdata 中的 tool 字段会被忽略。`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args execArgs) (*mcp.CallToolResult, any, error) {
		out := eng.ExecuteStep(ctx, engine.ExecuteInput{
			MCPID: args.MCPID, StepGoal: args.StepGoal, Rawdata: args.Rawdata, CorrelationID: args.CorrelationID,
		})
		return textResult(out), nil, nil
	})
}

func textResult(jsonText string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: jsonText}}}
}

func trim(s string) string { return strings.TrimSpace(s) }
