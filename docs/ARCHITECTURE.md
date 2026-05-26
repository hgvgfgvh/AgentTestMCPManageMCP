# 3M 架构（实现映射）

## 进程内模块

```text
cmd/mcp-manager-mcp
  └─ MCP Server（三工具 + catalog 资源）
internal/engine/ManagerEngine
  ├─ registry.Store          ← registry.json 真源 + catalog_revision
  ├─ install.Pipeline        ← add 流水线 + InstallAgent（可选 LLM）
  ├─ execute.Runner          ← execute_step + StepAgent（可选 LLM）
  ├─ supervisor.Pool         ← 子 MCP stdio 进程
  ├─ guard / audit
  └─ catalog.Resource        ← mcp-manager://catalog（辅助 revision）
```

## add_managed_mcp 内循环

1. InstallAgent（LLM，有界）或规则 `PlanRequirement`
2. 工作区 `data/mcps/{mcp_id}/`：manifest、launch.json、install.log
3. 可选 HTTP 下载（`MCP_MANAGER_DOWNLOAD_HOSTS` 白名单）
4. 启动子 MCP（`CommandTransport`）
5. Registry `status=ready`，revision++，catalog 资源更新

## execute_step 内循环

1. Registry 校验 `ready`
2. `supervisor` 连接 + Ping
3. StepAgent：LLM + 全量 tool schema → `call_tool` / `finish`（有界）
4. 失败时规则 `PickTool` 兜底
5. 原样返回 `payload`

## Host 边界

主项目 **不** 为 3M 增加专用 Go 集成；Host 仅通过 MCP 协议调用三工具。  
托管清单刷新：由调用方在需要时执行 `list_managed_mcps`（见 `DESIGN_INTENT.md` M5，由 Host 侧自行实现，不在本仓库）。
