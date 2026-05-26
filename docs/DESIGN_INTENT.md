# MCPManagerMCP（3M）— 设计意图（宪法）

> **维护规则**：人工维护；Agent 不得用「实现更方便」覆盖下文取舍。新决策以日期段落**追加**，不删除历史。  
> **宿主约束**：`AgentTest` 负责 Plan/Exec 编排、外挂 SKILL、用户交付；本文约束 **3M 进程内部** 与 **Host↔3M 防火层契约**。  
> **方法论**：对齐 `AgentTest/Agent编码防止架构坍塌的处理方法论/`（意图锚点 + 架构/验收 + 黄金法则 + DRIFT）。

---

## 0. 产品定位（2026-05-26 修订）

### 0.1 外挂 MCP：自动生成 + 安装 + 配置

**动态托管 MCP 的全生命周期**（下载、生成、改配置、启停子进程）在 **3M 独立进程** 内完成；**不在**主架构 `AgentTest` 中实现。

主架构仅：

1. 通过 **防火隔离层** 连接 **一个** `MCPManagerMCP`（3M）；  
2. 下发「要增加哪些 MCP 能力」「要用哪个托管 MCP 完成哪一步」；  
3. 需要最新托管清单时，**显式调用** 3M 的查询工具（见 §5.1），由 Host 用**通用刷新路径**更新能力地图快照。

```text
外挂 MCP 能力进化  ≠  Host 内改 app.yaml / capabilities.Start 子 Server
                    =  Host → 防火层 → 3M（安装/配置/执行）
```

### 0.2 3M 是什么

**能力体系管理 MCP**：管理「可被 Host 使用的 MCP 能力」的注册表、安装流水线、子 MCP 运行时，以及 **单步** 代理调用。内含 **管理 Agent**（3M 私有），根据 Host 的 `add_managed_mcp` 需求完成下载或生成，并写入 **3M 自有配置**，动态加载。

### 0.3 对外三工具（v1 冻结语义）

| 工具 | 职责 |
|------|------|
| **`list_managed_mcps`** | 查询当前托管了哪些 MCP（L1：id、摘要、状态）；**唯一**供 Host 刷新托管能力地图的 3M 数据源 |
| **`add_managed_mcp`** | Host 下发要增加的能力需求 → 3M 管理 Agent 安装/生成/配置/加载 |
| **`execute_step`** | Host 指定 `mcp_id` + 单步目标 + 完整 `rawdata` → 3M 调子 MCP → **原样**回传结果 |

> 实现层公开名以 `list_managed_mcps` 为准；概念上即「get list」，**不得**再引入与三者并列的第四套「地图专用」工具。

---

## 1. 要解决什么问题

| 痛点 | 3M 意图 |
|------|---------|
| Host 内直接挂载大量 MCP，安装/配置/热加载与编排耦合 | **风险隔离**：下载、生成、改配置、启停子 MCP 仅在 3M 沙箱内 |
| Host 需感知每个子 MCP 的全部 tool schema，上下文膨胀 | Host 只认 **3M 三工具** + **L1 托管清单**（经 `list_managed_mcps` 拉取）；子 MCP schema 仅在 `execute_step` 单步内由 3M 内部 LLM 使用 |
| 能力进化要扩展「可安装 MCP」 | 3M 内 **管理 Agent** 按 Host 需求安装/生成并 **动态加载** |
| Host 与 3M 耦合过紧 | **禁止**在 `add_managed_mcp` 响应中夹带「供 Host 拼地图」的专用字段；Host **禁止**对 add 做单独分支更新地图 |
| 外挂 SKILL 是经验路径，不是可执行面 | **SKILL 留在 Host**（`skill_packs` + 热加载）；3M **不**替代 SKILL 体系 |
| 与 Soul/Memory 混淆 | 3M **不**存人格/执行经验；**不**输出 `exec_simple_match` 等 Memory 键 |

定位：**可安装、可执行、可隔离** 的 MCP 运行时；不是 Plan 编排器，不是用户门户。

---

## 2. 铁律（M1～M14 · 宪法级）

| # | 铁律 | 含义 |
|---|------|------|
| M1 | **Host 不装子 MCP** | `AgentTest` **不得**在进程内直接 `capabilities.Start` 托管由 3M 管理的子 MCP |
| M2 | **三工具对外** | 对 Host 的稳定面为 **`list_managed_mcps`**、**`add_managed_mcp`**、**`execute_step`**；语义不可拆散或合并为「隐式副作用」 |
| M3 | **L1 仅经 list** | 托管 MCP 的 L1 描述（id、摘要、状态）**只**通过 `list_managed_mcps`（及可选 `catalog_revision`）对外提供；**不**在 L1 枚举子工具 func schema |
| M4 | **add 不携带地图** | `add_managed_mcp` 响应 **不得**包含 `l1_summary`、`managed_catalog` 等供 Host 直接 patch 地图的字段 |
| M5 | **Host 单路径刷新** | Host 更新托管能力快照 **仅**通过调用 `list_managed_mcps`（或 MCP `tools/list_changed` → 再 list）；**禁止**针对 add 成功的专用分支逻辑 |
| M6 | **单步代理** | `execute_step` = 一个逻辑步骤、一次对外回包；Host 传 `mcp_id` + `step_goal` + **完整 `rawdata`**；Host 不得用多步 ReAct 驱动 3M 内部编排 |
| M7 | **原样回传** | `execute_step` 的 `payload` 为子 MCP 原始结果，**不改写**结论（可外包元数据） |
| M8 | **内部全 schema** | 单次 `execute_step` 内，3M 可将该 `mcp_id` 下**全部**子工具 schema 交给**内部 LLM**（单步无 Host 级长上下文压力） |
| M9 | **SKILL 归 Host** | 外挂 SKILL 仍在 Host；3M **不**扫描 `skill_packs` |
| M10 | **防火层** | Host↔3M：鉴权、限流、请求大小上限、审计 |
| M11 | **失败隔离** | 子 MCP 崩溃、安装失败、超时 **不得**拖垮 Host 主进程 |
| M12 | **3M 真源一致** | 3M 内 Registry 与 `list_managed_mcps` 返回 **原子一致**（M10 在 3M 侧） |
| M13 | **Host 编排不变** | 3M **不**替代 PlanAgent、**不**写 TodoList、**不**调用 `report_step_result` |
| M14 | **渐进披露** | Host 渐进披露仅解锁 **3M 三工具** + 内置 + SKILL；子 MCP 工具名 **不**进入 Host 解锁表 |

---

## 3. 边界：什么在 3M、什么在 Host

### 3.1 归属 3M（本仓库）

| 能力 | 说明 |
|------|------|
| 子 MCP **自动生成** / 下载 / 安装 | 白名单源、版本 pinning、目录布局、依赖安装 |
| 子 MCP **配置** | 仅 3M 自有 `config/`（或 Registry）；**不**改 Host `config/app.yaml` 的 `capabilities.mcp.servers` |
| 子 MCP **动态加载** | 启停 stdio/HTTP、健康检查、重连 |
| **管理 Agent** | 解析 `add_managed_mcp`，完成安装流水线 |
| **`list_managed_mcps`** | 返回当前托管列表 + 可选 `catalog_revision` |
| **`execute_step`** | 内部 LLM 选子工具 → 一次调用 → raw `payload` |
| **动态 L1 文案** | 由 3M 根据 Registry **生成**，供 list 工具返回；**不**推送给 Host |

### 3.2 归属 Host（`AgentTest`）

| 能力 | 说明 |
|------|------|
| Plan → Exec / Exec-Simple | 多步编排、`verify.Gate`、`report_step_result` |
| 内置技能、外挂 SKILL | 不变 |
| Soul / Memory、门户 | 不变 |
| 连接 3M | MCP 客户端 + 防火层 |
| **托管 MCP 快照** | `RefreshManagedMCPFrom3M()`：只调 `list_managed_mcps`，写入内存；`BuildAgentCatalogMarkdown` / Plan 概览 **只读快照** |
| **何时刷新**（策略，可配置） | 如：每用户消息前、能力类工具内、`list_agent_capabilities` 内；**不是** add 回调特例 |

### 3.3 明确不做

- 不把 SKILL 迁入 3M；不让 3M 直接回复最终用户。  
- 不让 3M 修改 Host 仓库文件（除约定只读挂载）。  
- 不让 Host 在 `add_managed_mcp` 响应里解析 catalog 更新地图（M4/M5）。  
- 不在 v1 让 `execute_step` 跨 Host 多步保持 3M 内会话记忆。

---

## 4. 架构 — Host 与 3M

```text
┌─────────────────────────────────────────────────────────────┐
│ AgentTest（Host）                                            │
│  Plan / Exec · 内置技能 · 外挂 SKILL                          │
│  托管 MCP 段 ← managedMCPSnapshot（仅 list_managed_mcps 填充）│
│  调用：list_managed_mcps | add_managed_mcp | execute_step    │
└───────────────────────────┬─────────────────────────────────┘
                            │ MCP + 防火层
                            ▼
┌─────────────────────────────────────────────────────────────┐
│ MCPManagerMCP（3M）                                           │
│  list_managed_mcps · add_managed_mcp · execute_step            │
│  Registry · 管理 Agent · 子 MCP 进程池 · 3M 配置               │
│  execute_step 内：子 MCP 全量 tool schema → 内部 LLM           │
└───────────────────────────┬─────────────────────────────────┘
                            ▼
                    [子 MCP …]  （Host 不可见 tool 名）
```

**Host 执行子能力**：只 `execute_step`，**不** `filesystem__read_file` 等子公开名。

---

## 5. 工具契约（Host 联调冻结 · v1）

### 5.1 `list_managed_mcps`（get list）

**职责**：返回 3M 当前托管 MCP 的 **L1 清单**；Host **唯一**据此刷新能力地图中的「托管 MCP」段。

**入参（JSON）**：

| 字段 | 说明 |
|------|------|
| `correlation_id` | 可选，审计 |

**出参（JSON 字符串）**：

| 字段 | 说明 |
|------|------|
| `catalog_revision` | 单调递增 revision（可选但推荐；Host 用于「是否需要全量 list」） |
| `managed` | 数组，每项见下表 |
| `message` | 可选 |

`managed[]` 每项：

| 字段 | 说明 |
|------|------|
| `mcp_id` | 托管标识，`execute_step` 使用 |
| `summary` | 一句话能力摘要（3M 动态生成，可随 Registry 变） |
| `status` | `ready` \| `pending` \| `failed` \| `degraded` |
| `source` | 可选，`downloaded` \| `generated` \| `manual` |

**不含**：子 MCP 各工具的 parameters schema（属 3M 内部 `execute_step`）。

**Host 集成（M5）**：

```text
RefreshManagedMCPFrom3M()
  → tools/call list_managed_mcps
  → 原子替换 managedMCPSnapshot
  → BuildAgentCatalogMarkdown / BuildPlanCapabilityOverview 读快照
```

**禁止**：从 `add_managed_mcp` 响应写入快照。

**IO 策略（Host 配置，非 3M 职责）**：

| 策略 | 说明 |
|------|------|
| `on_user_turn` | 每用户消息前 refresh（推荐默认） |
| `on_demand` | 仅 `list_agent_capabilities` 等工具内 refresh |
| `revision_gate` | 先轻量比对 revision（若 3M 提供独立工具或 list 内 revision），未变则跳过 |

**业内对齐（可选）**：3M 在 Registry 变更时发 MCP `notifications/tools/list_changed`；Host MCP 客户端收到后 **仍只** 调 `list_managed_mcps` 刷新快照，**不**解析 add 响应。

---

### 5.2 `add_managed_mcp`

**职责**：Host 下发「要增加哪些 MCP 能力」；3M **管理 Agent** 负责下载或**生成** MCP、写入 **3M 配置**、动态加载。

**入参（JSON）**：

| 字段 | 说明 |
|------|------|
| `requirement` | **必需**。自然语言：**同时写清要什么 MCP 能力 + 启动必备的基本配置**（如目录路径、`.db` 文件、默认仓库路径等）。示例：「sqlite MCP，数据库文件 `C:\data\app.db`」；「文件系统 MCP，允许 `C:\WorkSpace`」 |
| `constraints` | 可选。补充配置或约束（如 token、只读、版本）；与 requirement 一并交给内部安装 Agent，写入 launch args / 后续 env（若支持） |
| `correlation_id` | 可选 |

**同步返回（JSON 字符串）** — **仅安装结果，不含地图（M4）**：

| 字段 | 说明 |
|------|------|
| `accepted` | 是否接受任务 |
| `mcp_id` | 成功或预留的托管 id（pending 时也可返回） |
| `status` | `ready` \| `failed` \| `pending` |
| `message` | 人类可读说明 |
| `error` | 失败时 |

**不得出现**：`l1_summary`、`managed_catalog`、`catalog_revision`（revision 仅由 **list** 提供）。

**3M 内部（Host 不可见）**：管理 Agent → 下载/生成 → 写 3M 配置 → 启子进程 → 更新 Registry →（可选）`tools/list_changed` → **不**回调 Host 地图。

**Host 在 add 之后若要编排新 MCP**：应先 `list_managed_mcps`（或依赖已配置的 `on_user_turn` refresh），再 `execute_step`。

---

### 5.3 `execute_step`

**职责**：Host 指定 **哪个托管 MCP**、**本步要完成什么**、**完整 rawdata**；3M 内部完成子工具选择与一次调用；Host **无需**知道子 MCP 有哪些 func。

**入参（JSON）**：

| 字段 | 说明 |
|------|------|
| `mcp_id` | **必需** |
| `step_goal` | **必需**。本步意图（供 3M 内部 LLM） |
| `rawdata` | **必需**。实现该步所需的**全部**原始输入；3M **不得**因缺参向 Host 追问（无跨步记忆） |
| `correlation_id` | 可选 |

**出参（JSON 字符串）**：

| 字段 | 说明 |
|------|------|
| `ok` | 是否成功 |
| `mcp_id` | 回显 |
| `tool_used` | 审计：内部实际子工具名 |
| `payload` | **子 MCP 原始结果**（不改写） |
| `error` | 失败时 |
| `duration_ms` | 可选 |

**内部行为（M6/M8）**：

1. 按 `mcp_id` 加载子 MCP，取得 **全部** tool 的详细 schema。  
2. 将 `step_goal` + `rawdata` + 全量 schema 交给 **3M 内部 LLM**（单步会话，无 Host 级历史）。  
3. 内部选定 **一个** 子工具并调用（或判定无法完成而失败）。  
4. 将子 MCP 返回写入 `payload`；**不**为 Host 保留 3M 内多轮状态。

**设计理由**：Host 侧无长期 tool schema 压力；3M 单步内可「一次性把该 MCP 所有 func 细节给 LLM」，提高选对工具的概率。

---

## 6. 防火层（Host 实现要点）

| 项 | 要求 |
|----|------|
| 鉴权 | 连接 3M 的密钥或本地 ACL |
| 限流 | 对三工具分别或统一限流 |
| 大小 | `rawdata` / `payload` 上限 |
| 审计 | `correlation_id`、耗时、错误；默认不记完整 payload |
| 路径 | 子 MCP 默认仅 3M 工作区 |

---

## 7. 与 Host 能力地图的三层关系

| 层 | 来源 | Host 刷新方式 |
|----|------|----------------|
| **托管 MCP** | 3M `list_managed_mcps` | **仅** `RefreshManagedMCPFrom3M()` |
| 内置技能 | `abilities.yml` | Host 注册 |
| 外挂 SKILL | `skill_packs` | `skillpacks.Reload` + 监视 |

AGENTS / Plan 概览中 **分节展示**；托管段脚注可写：「以最近一次 `list_managed_mcps` 为准」。

---

## 8. 分阶段交付（建议）

| 阶段 | 范围 | 验收要点 |
|------|------|----------|
| **P0** | Registry + 1 子 MCP + `execute_step` + `list_managed_mcps` | payload 原样；list 与 Registry 一致 |
| **P1** | `add_managed_mcp` + 管理 Agent + 自动生成/安装/配置 | add 响应无 catalog 字段；Host 靠 list 看到新 `mcp_id` |
| **P2** | 防火层、失败隔离、`list_changed` 可选 | 子 MCP 崩溃不影响 Host |
| **P3** | Host 下线直连子 MCP，仅连 3M | `plan→exec` 保底仍可用 |

---

## 9. 决策记录

### 2026-05-26 — 3M 子系统从 Host 拆出

MCP 安装/生成/配置/运行时迁入 `AgentTestMCPManageMCP`；Host 仅连 3M。外挂 SKILL 不迁移。

### 2026-05-26 — 托管清单：显式 list，禁止 add 推送地图（M4/M5）

**决策**：

- 采用独立工具 **`list_managed_mcps`** 作为 Host 刷新托管能力的**唯一** 3M 数据源。  
- **拒绝**「`add_managed_mcp` 返回 `catalog_revision` / `l1_summary`，Host 对 add 做专用分支更新地图」方案。

**原因**：

- 高内聚：安装流水线与查询 API 分离；3M 不承载 Host prompt 拼装知识。  
- 低耦合：Host 仅「调用 list → 替换快照」，与 Soul/Memory 的 store/retrieve 模式一致。  
- 避免 3M 与主架构在响应形状上绑定，减少 Host 内「不伦不类」特例代码。

**影响**：

- Host 需实现 **一条** 通用 `RefreshManagedMCPFrom3M()`（接受多一次 list IO，可用 revision 节流）。  
- `add_managed_mcp` 响应形状长期稳定，仅表示安装结果。  
- 业内 `tools/list_changed` 可作为 3M 可选优化，Host 仍统一走 list 工具。

---

## 10. 文档与宿主方法论对齐

| 宿主方法论文件 | 3M 对应物 |
|----------------|-----------|
| `DESIGN_INTENT.md` | 本文 |
| `ARCHITECTURE.md` | 待实现后 `docs/ARCHITECTURE.md` |
| `ACCEPTANCE_RULES.md` | 待补充 |

编码 Agent 修改 **本仓库** 前须读本文；修改 **Host** 时：实现 §5.1 Host 集成，**禁止**违反 M4/M5。
