# 3M 验收规则

## 对外工具（M2：仅三工具）

| ID | 规则 |
|----|------|
| A1 | `tools/list` 仅含 `list_managed_mcps`、`add_managed_mcp`、`execute_step` |
| A2 | `list_managed_mcps` 含 `catalog_revision` + `managed[]`，无子 tool schema |
| A3 | `add_managed_mcp` 响应无 `l1_summary` / `managed_catalog` / `catalog_revision` |
| A4 | `execute_step` 成功时 `payload` 为子 MCP 原始文本 |

## Registry

| ID | 规则 |
|----|------|
| R1 | `add` 成功后 `list` 可见新 `mcp_id`，revision 递增 |
| R2 | 子 MCP Ping 失败可将状态标为 `degraded` |

## 内部 Agent

| ID | 规则 |
|----|------|
| G1 | 配置 `MCP_MANAGER_LLM_API_BASE` 后 `phase=manager-v2-agent` |
| G2 | 未配置 LLM 时规则路径仍可 `add`+`execute`（child-echo） |
| G3 | `source=npm` 需求可 `npm install` 并写入 `launch.json`（需本机 npm） |
| G4 | HTTP zip/tar.gz 可下载解压并解析 `launch.json` / `mcp.json` / `package.json` |

## Host

| ID | 规则 |
|----|------|
| H1 | `list_managed_mcps` 调用后 `buildManagedMCPCatalogSection` 非空 |
| H2 | 无 add 响应专用 patch 逻辑 |

## 测试

```bash
cd AgentTestMCPManageMCP && go test ./...
```
