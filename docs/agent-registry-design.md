# Agent 资源设计方案

> 将 Agent 升级为平台的一等公民资源：租户创建和管理 Agent，用户创建 Session 时选择使用哪个 Agent，Agent 决定该 Session 的完整能力边界。

---

## 一、背景与动机

### 现有问题

当前架构没有 Agent 资源的概念。所有配置（MCP、Skill、模型）挂在租户上，每个 Session 都通过 Resolver 把租户的全量配置加载进来。这带来两个问题：

1. **配置粒度太粗**：租户下所有用户、所有 Session 使用同一套配置，无法为不同场景提供不同能力组合（如"代码助手"和"文档助手"使用不同工具集）
2. **用户 Skill 开关是个补丁**：`user_skill` 的 enable/disable 本质上是在用全量加载后再做减法，语义混乱，管理成本高

### 引入 Agent 资源后

```
租户创建 Agent（定义：用哪个模型、哪些 MCP、哪些 Skill、系统提示是什么）
  └── 用户创建 Session → 选择一个 Agent → Session 完整继承 Agent 配置
```

Agent 是租户对"一类任务"的封装。一个租户可以有多个 Agent，用户按需选择。

---

## 二、Agent 资源定义

Agent 是**版本化的配置快照单元**，包含：

| 字段 | 说明 |
|------|------|
| `name` | 显示名称，如"代码助手"、"文档审查" |
| `description` | 该 Agent 的用途描述，显示在选择列表中 |
| `model` | 模型选择（空则继承租户/全局默认）|
| `system_prompt` | 该 Agent 专用的系统提示 |
| `tool_config` | 启用哪些内置工具（bash/read/write/edit/glob/grep 等）及其参数 |
| `mcp_server_ids` | 引用租户 MCP 列表中的哪几个 |
| `skill_ids` | 引用租户 Skill 列表中的哪几个 |
| `sub_agent_types` | 允许派发哪些类型的 Sub-Agent（explore/verify/general）|
| `version` | 每次修改自动递增，Session 可 pin 到某个版本 |

Agent 引用 MCP 和 Skill，不拷贝内容。MCP/Skill 的实际内容仍由租户统一管理，Agent 只决定"选哪些"。

---

## 三、数据表结构

### agents 表

```sql
CREATE TABLE agents (
    id               TEXT PRIMARY KEY,       -- ulid
    tenant_id        TEXT NOT NULL,
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    version          INT  NOT NULL DEFAULT 1,
    model            TEXT,                   -- 空则继承租户/全局默认
    system_prompt    TEXT NOT NULL DEFAULT '',
    tool_config      JSONB NOT NULL DEFAULT '{}',  -- {"bash":true,"read":true,...}
    sub_agent_types  TEXT[] NOT NULL DEFAULT '{}', -- ["explore","verify","general"]
    is_default       BOOLEAN NOT NULL DEFAULT FALSE, -- 租户默认 Agent（用户未指定时使用）
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at      TIMESTAMPTZ
);
CREATE INDEX ON agents (tenant_id) WHERE archived_at IS NULL;
CREATE UNIQUE INDEX ON agents (tenant_id) WHERE is_default = TRUE AND archived_at IS NULL;
```

### agent_mcps 表（Agent 引用哪些 MCP）

```sql
CREATE TABLE agent_mcps (
    agent_id      TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_id TEXT NOT NULL,   -- 引用 mcp_servers 表的 id
    PRIMARY KEY (agent_id, mcp_server_id)
);
```

### agent_skills 表（Agent 引用哪些 Skill）

```sql
CREATE TABLE agent_skills (
    agent_id  TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    skill_id  TEXT NOT NULL,   -- 引用 user_skills 表的 id（租户 Skill）
    PRIMARY KEY (agent_id, skill_id)
);
```

---

## 四、与现有 MCP / Skill 的关系

```
租户层（source of truth）           Agent 层（引用 + 配置）
─────────────────────────           ─────────────────────────────
mcp_servers 表                      agent_mcps 表
  id, name, url, type,        ←ID   agent_id → mcp_server_id
  command, args, headers, ...

user_skills 表（租户 Skill 内容）    agent_skills 表
  id, name, content, ...      ←ID   agent_id → skill_id
```

- 租户管理 MCP/Skill 的内容和连接配置，与 Agent 无关
- Agent 只决定"这个场景用哪些 MCP/Skill"
- 同一个 MCP/Skill 可以被多个 Agent 引用
- 租户修改 MCP 连接配置（URL、认证等），所有引用它的 Agent 立即生效，不需要更新 Agent

---

## 五、Session 创建与 Agent 的关系

用户创建 Session 时传入 `agent_id`（可选）：

- 传了 `agent_id` → Session 使用指定 Agent
- 没传 → 使用租户的 `is_default = true` 的 Agent
- 租户也没有默认 Agent → 走现有 Resolver 路径兜底（渐进迁移期间保留）

Session 可以 pin 到 Agent 的具体版本（`agent_version`），灰度发布时有用。

---

## 六、Resolver 的变化

现有 Resolver 做三层合并（global → tenant → user），产出 `EffectiveConfig`。

引入 Agent 后，流程变为：

```
1. 确定当前 Session 使用哪个 Agent（从 Session 元数据读取）
2. 加载 Agent 配置（model, system_prompt, tool_config, sub_agent_types）
3. 加载 Agent 引用的 MCP 列表（JOIN agent_mcps → mcp_servers）
4. 加载 Agent 引用的 Skill 列表（JOIN agent_skills → user_skills）
5. 模型：Agent.model → 租户 ModelOverride → 全局默认（三层 fallback）
6. 产出 EffectiveConfig
```

**不再需要**：
- `applyUserLayer()`（用户不再有 model/brain 偏好覆盖）
- `applySkillPrefs()`（用户不再有 Skill enable/disable）
- `store.UserSkills().List(ctx, tenantID, userID)` 的用户偏好查询

---

## 七、可以删除的东西

| 内容 | 说明 |
|------|------|
| `user_skill` 表中的 `enabled` 字段 | 能力开关移到 Agent 层 |
| `store.UserSkills().List(ctx, tenantID, userID)` 用户偏好查询 | Resolver 不再需要 |
| `applySkillPrefs()` | 逻辑消失 |
| `applyUserLayer()` | 用户不再覆盖模型配置 |
| `UserSettings.ModelOverride / BrainOverride` | 由 Agent 选择替代 |

`user_skills` 表本身保留，作为"租户 Skill 内容库"，语义变为租户资产，不再有 per-user 状态。

---

## 八、对现有组件的影响

| 组件 | 变化 |
|------|------|
| `gateway/store/store.go` | 新增 `Agents() AgentRepository` |
| `gateway/store/agent.go` | 新增：CRUD + `ListByTenant` + `GetDefault` + `GetWithMCPsAndSkills` |
| `gateway/store/sqlite/` `postgres/` | 新增 agents、agent_mcps、agent_skills 表迁移和查询实现 |
| `gateway/store/user_skill.go` | 移除 per-user enabled 查询；`List(tenantID, "")` 保留作为全量 Skill 列表 |
| `resolver/resolver.go` | `Resolve()` 接收 `agentID`，从 Agent 加载 MCP/Skill 列表，替换现有三层合并逻辑 |
| `resolver/session_manager.go` | `Ensure()` 传入 agentID；brainCache key 加入 agentID |
| `orchestration/http_session.go` | Session 创建请求增加可选 `agent_id` 字段；Session 记录持久化 agent_id |
| `orchestration/http.go` 新增路由 | `GET/POST/PATCH /agents`、`POST /agents/:id/archive` |
| `gateway/session/store.go` | Session 元数据增加 `agent_id`、`agent_version` 字段 |
| `harness/harness.go` | 不变，harness 本身不感知 Agent 概念 |

---

## 九、实现阶段建议

**Phase 1：Agent CRUD 基础设施**
- `agents`、`agent_mcps`、`agent_skills` 表迁移
- `AgentRepository` 接口 + SQLite/Postgres 实现
- HTTP 路由：创建/更新/列表/归档 Agent
- 管理员可在 Web UI 中创建 Agent，为其选择 MCP 和 Skill

**Phase 2：Session 接入 Agent**
- Session 创建支持 `agent_id` 参数
- Resolver 从 Agent 加载配置，替换全量 tenant 加载
- 删除 `applySkillPrefs()` 和 per-user 查询

**Phase 3：清理旧逻辑**
- 移除 `user_skills.enabled` 字段和相关代码
- 移除 `UserSettings.ModelOverride / BrainOverride`
- 迁移存量数据：为每个租户自动创建一个默认 Agent，内容对应现有 Resolver 的全量输出

**Phase 4：Multi-Agent 接入**
- Agent 的 `sub_agent_types` 字段控制可以派发哪些 Worker 类型
- Worker 执行时从 Session 的 Agent 配置中读取 MCP/Skill（Sub-Agent 的能力边界由 Agent 类型静态决定，不继承 coordinator 的 Agent 配置）
- 详见 `multi-agent-architecture.md`
