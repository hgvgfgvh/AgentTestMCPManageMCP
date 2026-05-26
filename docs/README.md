# MCPManagerMCP（3M）— 方法论文档集

本目录存放 **MCP 能力管理子系统（3M）** 的人工设计意图与架构/验收文档。

**方法论来源**：`AgentTest/Agent编码防止架构坍塌的处理方法论/`。

---

## 文档地图

| 文件 | 用途 |
|------|------|
| [DESIGN_INTENT.md](./DESIGN_INTENT.md) | **宪法**：三工具契约、M1～M14、Host 仅经 list 刷新地图 |
| ARCHITECTURE.md | 待实现后补充 |
| ACCEPTANCE_RULES.md | 待补充 |

---

## 核心设计（一页）

| 能力 | 位置 |
|------|------|
| 外挂 MCP **生成 / 下载 / 安装 / 配置 / 动态加载** | **3M**（管理 Agent + Registry） |
| Plan / Exec / SKILL / Soul / Memory | **AgentTest** |
| 查询托管了哪些 MCP | 3M **`list_managed_mcps`**（Host **唯一**地图数据源） |
| 增加 MCP 能力 | 3M **`add_managed_mcp`**（响应**不含**地图字段） |
| 用某 MCP 完成一步 | 3M **`execute_step`**（`mcp_id` + `step_goal` + `rawdata` → 原样 `payload`） |

```text
Host  RefreshManagedMCPFrom3M()  ──only──►  list_managed_mcps
Host  add 成功  ──X──►  禁止专用分支 patch 地图
```

---

## 编码 Agent 最小流程

1. 读 [DESIGN_INTENT.md](./DESIGN_INTENT.md)（§0、§2 M4/M5、§5）  
2. 读宿主阶段一铁律，不破坏 `plan → exec`  
3. Host 集成：只实现通用 list 刷新，不对 add 做特例  

---

## 同步说明

- **2026-05-26**：初版（隔离、双工具 + list 轮询）。  
- **2026-05-26（修订）**：三工具冻结；**list-only** 刷新 Host 地图；add 禁止携带 catalog。
- **2026-05-26**：Phase-1 代码 — Registry / install 流水线 / supervisor / execute。
- **2026-05-26**：Phase-3 完整安装（npm / HTTP 解压 / launch 解析 / InstallAgent）；**3M 自闭环**，不修改 AgentTest 主项目代码。
