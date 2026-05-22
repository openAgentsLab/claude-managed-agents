# Memory System 设计方案

## 一、官方参考

Anthropic 目前提供两套记忆实现，设计理念差异明显。

### Memory Tool（客户端文件系统模式）

这是 Claude Code 使用的方案。Agent 在服务端推理时，决定调用 memory 后通过 SSE 事件流将执行权交回**客户端**，由客户端完成 `/memories` 目录下的文件操作，再把结果返还给 Agent。

工具集仅有六个文件操作：`view / create / str_replace / insert / delete / rename`，本质是一个带 Agent 控制权的文件编辑器。存储后端完全由开发者决定，官方不介入。

这套方案天然适合**本地单用户场景**——Agent 就跑在用户机器上，文件在哪、怎么存都由用户掌控。但在服务端多用户场景中，无法处理隔离和权限问题。

### Managed Agents Memory Store（服务端文档存储模式）

这是 Anthropic 为 Agent 托管服务设计的方案，也是 ai-forge 的设计基准。几个核心思想：

**Store 作为独立资源**：Memory Store 是 workspace 级的持久化资源，生命周期独立于 Session 和容器。Session 结束后记忆不消失，容器重启后记忆不消失。

**多 Store 并行挂载**：每个 Session 最多挂载 8 个 Store，同一个 Store 可被多个 Session 同时访问。多个用户的 Agent 可以共享同一份知识库。

**Agent 完全主动驱动**：没有自动提取，没有后台向量化。Agent 通过系统提示中的指导，自行决定何时读取、何时写入、写什么内容。

**乐观并发控制**：用 `content_sha256` 防止多 Session 并发写冲突。

工具集：`memory_list / memory_search / memory_read / memory_write / memory_edit / memory_delete`。

---

## 二、ai-forge Memory 设计思路

### 核心原则

**单一接口，按 scope 隔离**

所有 Store 类型——无论内置还是自定义——都实现同一个 `MemoryStore` 接口。隔离边界在 Store 的构造阶段确定（scope key 注入），接口方法本身不携带任何租户或用户 id。换句话说，工具层代码不需要知道它在操作哪个用户的数据，Store 实例天然代表了访问边界。

**Store 是边界，文件是内容**

Store 负责权限和数据范围的隔离，不强制内部数据的组织格式。Agent 通过文件名自行管理内容结构。官方 Managed Agents 没有 description 字段，ai-forge 扩展了它——目的是在系统提示中告知 Agent 这个 Store 用来存什么，引导 Agent 选择正确的写入目标。

**Agent 主动驱动，无被动提取**

记忆系统里没有后台任务，没有向量库，没有隐式提取。Agent 在每次任务开始前调用 `memory_list` 浏览索引，按需调用 `memory_read` 读取内容，在任务结束或获得新知识时调用 `memory_write` 写入。所有这些决策都由 Agent 自己完成，框架只提供工具。

---

### 内置 Store：映射多租户三层结构

ai-forge 有 Platform → Tenant → User 三层架构，内置 Store 正好映射用户日常需要的三个隔离级别：

| Store 名称 | 隔离维度 | 典型用途 | 写权限 |
|-----------|---------|---------|--------|
| `user` | 当前用户 | 个人偏好、工作习惯、历史反馈 | 仅本人 |
| `project` | 当前项目所有成员 | 架构决策、约束、开发约定 | 项目成员 |
| `tenant` | 整个组织 | 合规规范、统一策略 | 仅 tenant admin |

这三个 Store 在每个 Session 启动时**自动挂载**，无需配置。系统提示里会列出它们及各自的 description，Agent 看到提示就知道把个人偏好写到 `user`，把架构决策写到 `project`。

设计判断：大多数记忆需求可以归入这三类之一。用户、项目、组织——这正是多租户系统里信息天然流动的三个层次。

---

### 自定义 Store：覆盖内置无法表达的场景

内置三种 Store 无法覆盖的典型需求：

- **跨项目领域知识库**：Python 最佳实践、公司 API 设计规范——跟具体项目无关，但多个项目的 Agent 都应能访问
- **团队级共享知识**：某个小组内共享，范围比 tenant 小、比单个 project 大
- **只读知识库**：由某个角色维护，其他人的 Agent 只读不写
- **临时任务空间**：某次专项任务期间使用，完成后清理

自定义 Store 在创建时需指定：

- `name`：供 Agent 在工具调用中寻址（如 `store="python_standards"`）
- `description`：告知 Agent 这里存什么，是 Agent 选择 Store 的唯一依据，**创建时强制填写**
- `scope`：数据归属的 key（如 tenant_id、project_id）
- `visibility`：谁可以看到并挂载它（private / shared_project / shared_tenant）
- `write_policy`：谁可以写入（owner_only / members）

自定义 Store 由用户显式创建，在 Session 创建时通过参数指定挂载。系统校验权限后挂载，总数不超过 8 个（含三个内置）。

---

### Store 的实现后端

`MemoryStore` 是接口，框架内置两种实现：

- **SQLite**：本地开发和单机部署。FTS5 全文搜索和持久化同库，零外部依赖。
- **PostgreSQL**：多节点生产部署。tsvector 全文搜索，天然支持多实例共享同一 Store。

两种实现对上层完全透明。切换后端只需在部署配置里改一行，工具和提示逻辑完全不动。

开发者也可以自行实现 `MemoryStore` 接口，接入任何自定义存储，注册为新的后端驱动即可。

---

### 系统提示的角色

Session 启动时，框架动态生成 Memory 章节注入系统提示：

```
# Memory

You have persistent memory across sessions. Before starting a task, call
`memory_list` to recall relevant context.

Available memory stores:
- **user** — Your personal memory — preferences, habits, feedback about your working style
- **project** — Shared project memory — architecture decisions, constraints, conventions
- **tenant** — Organization-wide standards — policies and rules (read-only)
- **python_standards** — Python backend team's coding standards (read-only)

When writing, choose the store that matches the scope of the information:
personal knowledge → user, project-wide decisions → project,
org standards → tenant (if writable), domain knowledge → named store.
```

内置三个 Store 的当前索引（文件名 + 首行摘要）也会额外注入，让 Agent 无需主动调用 `memory_list` 就能感知已有记忆：

```
## user store index
- [preferences.md] — Prefers TypeScript strict mode, no inline comments
- [feedback.md] — Always explain tradeoffs before implementing

## project store index
- [auth_design.md] — JWT + Redis, 15-minute expiry, refresh token rotation
- [api_conventions.md] — REST, snake_case fields, errors use RFC 7807
```

description 字段的质量直接影响 Agent 的记忆行为。内置 Store 的 description 由框架预设，自定义 Store 强制填写。

---

### 多 Session 并发写入

同一个 Store 可同时被多个 Session 挂载。并发写基于 `content_sha256` 乐观锁：

```
Session A                        Session B
   │                                │
   ├─ Read("auth.md")               │
   │  ← sha256: "abc123"            │
   │                                ├─ Read("auth.md")
   │                                │  ← sha256: "abc123"
   │                                │
   ├─ Write("auth.md", sha256="abc123")
   │  ← OK, new sha256: "def456"    │
   │                                │
   │                   Write("auth.md", sha256="abc123")
   │                                ← ErrConflict, current sha256: "def456"
   │                                │
   │                                ├─ Read("auth.md") → 获取最新内容
   │                                ├─ 合并内容
   │                                └─ Write("auth.md", sha256="def456")
   │                                   ← OK
```

Agent 收到冲突后重新读取、合并、再写入，不需要任何框架层面的锁。

---

### 典型工作流

**任务开始时**：

```
Agent 收到用户请求
  │
  ├─ memory_list
  │  ← 看到 user/project/tenant store 的文件索引
  │
  ├─ memory_read("preferences.md", store="user")  ← 读取个人偏好
  ├─ memory_read("api_conventions.md", store="project")  ← 读取项目约定
  │
  └─ 开始执行任务，带着这些上下文
```

**任务结束后**：

```
Agent 完成任务
  │
  ├─ 发现用户提到了新的偏好（"以后代码里不要写 TODO"）
  │
  └─ memory_write("preferences.md",
                   content="...\n- No TODO comments in code",
                   store="user",
                   sha256="current_sha256")
```

**多用户协作时**：

```
用户 A 的 Agent                    用户 B 的 Agent
  │                                    │
  ├─ 完成架构评审                       │
  ├─ memory_write("arch_decision.md",  │
  │    store="project")                │
  │                                    │
  │                    ├─ memory_list  │
  │                    │  ← 看到 arch_decision.md 已更新
  │                    └─ memory_read("arch_decision.md", store="project")
  │                       ← 读取用户 A 写入的决策
```

---

## 三、与官方 Managed Agents Memory Store 对照

| 特性 | 官方 | ai-forge |
|------|------|---------|
| 工具集（6个） | ✅ | ✅ 名称与语义完全对齐 |
| write 为 upsert-replace | ✅ | ✅ |
| `content_sha256` 并发控制 | ✅ | ✅ |
| 版本化（version 递增） | ✅ | ✅ |
| Redact 合规清除 | ✅ | ✅ |
| 多 Store 挂载（≤8） | ✅ | ✅ 3 内置 + 最多 5 自定义 |
| Store description 字段 | ❌ 官方无 | ✅ 扩展，引导 Agent 选择 Store |
| 内置 Store（user/project/tenant） | ❌ 官方无内置分类 | ✅ 扩展，对应多租户层级 |
| 可自定义 Store | ❌ | ✅ 扩展，覆盖三内置无法表达的场景 |
| Agent 完全主动驱动 | ✅ | ✅ |
