# AgentTestMCPManageMCP（3M）

外置 **MCP 能力管理子系统**：在独立进程中完成外挂 MCP 的 **生成、下载、安装、配置、动态加载** 与 **单步代理执行**。  
`AgentTest` **不**在主机内实现上述能力，仅通过 **防火层** 连接本 MCP。

## 文档

- [docs/DESIGN_INTENT.md](./docs/DESIGN_INTENT.md) — 宪法（M1～M14）
- [docs/README.md](./docs/README.md) — 文档地图

## 对外三工具（v1）

| 工具 | 用途 |
|------|------|
| **`list_managed_mcps`** | 查询当前托管了哪些 MCP（Host 刷新能力地图的**唯一** 3M 数据源） |
| **`add_managed_mcp`** | 下发要增加的能力 → 3M 管理 Agent 安装/生成/配置/加载（响应**不含**地图） |
| **`execute_step`** | `mcp_id` + 单步目标 + 完整 `rawdata` → 子 MCP 结果 **原样** `payload` |

`execute_step` 内部可将该子 MCP 的**全部 func schema** 交给 3M 内 LLM，Host 无需感知子工具列表。

## 与 Host 分工

| AgentTest | 3M（本仓库） |
|-----------|----------------|
| Plan / Exec / SKILL / Soul / Memory | Registry、管理 Agent、子 MCP 进程池 |
| `RefreshManagedMCPFrom3M()` ← 只调 list | 自动生成 + 安装 + 3M 自有配置 |
| **禁止** add 响应专用更新地图逻辑 | **禁止** add 返回 catalog 字段 |

## 构建与运行

```bash
cd AgentTestMCPManageMCP
go test ./...
go build -o mcp-manager-mcp.exe ./cmd/mcp-manager-mcp
go build -o child-echo-mcp.exe ./cmd/child-echo-mcp   # 与 manager 放同一目录
```

- **stdio**：`mcp-manager-mcp.exe -data ./data`（默认 `-engine manager`）
- **桩模式**：`-engine stub`
- **HTTP**：`mcp-manager-mcp.exe -http 127.0.0.1:8093`
- **调试 WebUI**（默认随进程启动）：浏览器打开 **http://127.0.0.1:18094/** ，可手动测试三工具；关闭：`-debug-ui 0` 或 `MCP_MANAGER_DEBUG_UI=0`

环境变量：

| 变量 | 说明 |
|------|------|
| `MCP_MANAGER_DATA_DIR` | 数据根（registry、mcps/） |
| `MCP_MANAGER_ENGINE` | `manager` \| `stub` |
| `MCP_MANAGER_CHILD_ECHO_BIN` | 可选，显式指定 child-echo-mcp 路径 |
| `MCP_MANAGER_LLM_API_BASE` | 配置后启用 **内部 Agent**（安装/执行有界多轮） |
| `MCP_MANAGER_LLM_API_KEY` | LLM API Key |
| `MCP_MANAGER_LLM_MODEL` | 默认 `deepseek-chat` |
| `MCP_MANAGER_AGENT` | `1` 开启（默认有 API 即开）；`0` 仅规则路径 |
| `MCP_MANAGER_INSTALL_MAX_TURNS` | 安装 Agent 上限（默认 6） |
| `MCP_MANAGER_EXECUTE_MAX_TURNS` | 执行 Agent 上限（默认 4） |
| `MCP_MANAGER_DEBUG_UI` | `0` 关闭调试 WebUI；默认开启 |
| `MCP_MANAGER_DEBUG_UI_ADDR` | 调试 WebUI 监听地址（默认 `127.0.0.1:18094`） |

## Phase-3：完整安装（manager-v3-full）

`add_managed_mcp` 流水线按需求自动选择来源：

| 来源 | 触发示例 | 行为 |
|------|----------|------|
| **npm** | `npx -y @modelcontextprotocol/server-filesystem` 或 JSON `{"source":"npm","package":"@scope/pkg"}` | `npm install` → 写 `launch.json`（默认 `npx -y <pkg>`）→ 启子进程 |
| **http** | 需求中含 `https://.../*.zip` | 下载 → 解压 zip/tar.gz → 解析 `launch.json` / `mcp.json` / `package.json` |
| **echo** | 「回显」/ 无其它线索 | 启动同目录 `child-echo-mcp`（演示） |

安装成功后 Registry 更新；Host 需再调 **`list_managed_mcps`** 刷新地图（add 响应仍不含 catalog）。

可选：`MCP_MANAGER_DOWNLOAD_HOSTS=example.com,github.com` 限制 HTTP 下载域名。

## Phase-2：内部 Agent（manager-v2-agent）

| 组件 | 说明 |
|------|------|
| `internal/install/install_agent` | add 时有界多轮：plan / 沙箱 write / done → 再走启进程 |
| `internal/execute/execute_agent` | execute 时有界 ReAct：call_tool → finish |
| `internal/sandbox` | 工作区文件与命令白名单 |
| `internal/llm` | OpenAI 兼容 Chat |

未配置 LLM 时自动回退 Phase-1 规则路径（测试无需 API）。

## Phase-1 已实现（manager 引擎）

| 模块 | 职责 |
|------|------|
| `internal/registry` | `registry.json` + `catalog_revision` |
| `internal/workspace` | `data/mcps/{id}/` 沙箱 |
| `internal/install` | add 流水线：规划 → manifest → 启子进程 |
| `internal/supervisor` | 子 MCP stdio 进程池 |
| `internal/execute` | 单步：list tools → 选工具 → call 一次 |
| `cmd/child-echo-mcp` | 示例子 MCP（`echo` 工具） |

## 状态

- **2026-05-26**：Phase-3 完整安装（npm/HTTP/launch 解析）；Phase-0～2 基座不变；**不修改 AgentTest 主项目**。
- Host：**不修改主项目代码**；仅在 `AgentTest/config/app.example.yaml` 提供可选接入示例。
