# Managed Agents 问答整理

## Agent 创建

**Q：创建 Agent 都包含哪些字段？**

创建 Agent（`POST /v1/agents`）包含以下字段：

**核心字段**

| 字段 | 说明 |
|------|------|
| `name` | Agent 名称（必填） |
| `model` | 使用的 Claude 模型，如 `claude-sonnet-4-6`（必填） |
| `system` | 系统提示词 |
| `description` | Agent 功能描述 |

**能力配置**

| 字段 | 说明 |
|------|------|
| `tools` | 工具列表：内置工具集 `agent_toolset_20260401`（Bash、文件读写、Glob、Grep、Web Fetch、Web Search，默认全启用）或自定义工具 `type: custom` |
| `mcp_servers` | MCP 服务器连接数组（URL + 认证配置） |
| `skills` | Skills 数组，可用 Anthropic 预置技能（Excel、PowerPoint、Word、PDF）或自定义技能 |
| `multiagent` | 多 Agent 协调声明，列出可委托的子 Agent |

**元数据**

| 字段 | 说明 |
|------|------|
| `metadata` | 任意键值对，更新时按键合并（非全量覆盖） |

版本行为：每次有效更新生成新版本号。数组字段（`tools`、`mcp_servers`、`skills`）更新时是全量替换。

---

## 自定义工具

**Q：自定义工具 `type: custom` 是不是还需要配置工具声明、参数 schema？**

是的，需要完整声明：

```json
{
  "type": "custom",
  "name": "get_weather",
  "description": "Get current weather for a location",
  "input_schema": {
    "type": "object",
    "properties": {
      "location": { "type": "string", "description": "City name" }
    },
    "required": ["location"]
  }
}
```

**必填字段：**

| 字段 | 说明 |
|------|------|
| `type` | 固定值 `"custom"` |
| `name` | 工具名称，Claude 调用时引用 |
| `description` | 工具描述，**Claude 靠这个决定何时调用** |
| `input_schema` | 标准 JSON Schema（`type: object` + `properties` + `required`） |

官方强调 `description` 是影响工具调用准确性最重要的因素，建议每个工具写 3-4 句以上，说明：做什么、何时用（以及何时不用）、每个参数的含义和影响、注意事项。

---

## metadata 字段

**Q：metadata 是干什么的？**

`metadata` 是**纯开发者自用的键值对**，Anthropic/Claude 不解读也不使用它，完全由应用层自由存放追踪信息。

典型用途：
- 关联自己系统的 ID：`{"tenant_id": "xxx", "env": "prod"}`
- CI/CD 溯源：`{"git_sha": "abc123", "deployed_by": "github-actions"}`
- 业务分类标签：`{"team": "billing", "region": "us-west"}`

**Q：metadata 在哪使用、怎么使用？**

使用方式：写入时存，读取时带回，纯粹供应用层使用。

```json
// 创建时写入
{ "metadata": { "tenant_id": "org_abc123", "env": "prod" } }

// GET /v1/agents/{id} 响应里原样带回
{ "metadata": { "tenant_id": "org_abc123", "env": "prod" } }
```

**更新行为（按 key 合并，与其他字段不同）：**

```
当前: {"a": "1", "b": "2"}
传入: {"b": "99", "c": "3"}
结果: {"a": "1", "b": "99", "c": "3"}  ← 省略的 key 保留
```

要删除某个 key，把它的值设为空字符串 `""`。

---

## 多租户与访问控制

**Q：官方 Managed Agents 有团队或者租户的概念吗？创建的 Agent、Tools 等，谁能使用？**

官方 Managed Agents 没有团队或租户的概念，访问控制粒度只到 **Workspace** 级别。

```
Organization（组织）
└── Workspace（工作区，最多 100 个）
    ├── API Key（绑定到工作区）
    ├── Agents         ← 所有持有该 API Key 的代码都能访问
    ├── Sessions
    ├── Environments
    └── Files
```

- 同一 Workspace 下的 API Key 对该工作区内所有 Agent 有完全访问权，没有行级权限控制
- 不同 Workspace 之间完全隔离，互不可见
- 没有用户级、租户级、角色级的权限模型

**Q：Organization 可以创建多个吗？**

不能。Organization 是注册 Anthropic 账号时自动创建的唯一顶层实体，一个账号对应一个 Organization，无法在同一账号下创建多个。想要多个 Organization 只能注册多个 Anthropic 账号。

Workspace 才是可创建的隔离单元（最多 100 个），是官方建议的多环境/多团队隔离手段。

**Q：关于 Memory Store，Organization 也不能操作吗？**

操作权限取决于持有哪种 Key，不是账号角色：

```
Admin API Key（sk-ant-admin...）
└── 管理组织结构：Workspace、成员、API Key 等
    ✗ 不能操作 Memory Store / Agent / Session 等 Managed Agents 资源

普通 API Key（sk-ant-api...，绑定到某个 Workspace）
└── 操作该 Workspace 下的所有资源
    ✓ Memory Store、Agent、Session、Environment、Files...
```

Organization Admin 持有 Admin Key，只能管组织结构，不能操作 Memory Store。Managed Agents 的所有资源都在 Workspace 这一层管理。

---

## 资源归属：租户 vs 用户

**Q：如果自己实现了租户-用户两种级别，Tools、Agent、MCP、Skill 等应该挂在租户还是用户级别？**

分界线：**租户级定义"能做什么"，用户级定义"谁在用、用什么数据"。**

**租户级别（Agent 定义层）**

| 资源 | 原因 |
|------|------|
| Agent（含 system prompt、model） | 定义行为规范，同租户下所有用户共享 |
| Tools 配置 | 哪些工具可用是租户的业务决策 |
| MCP server URL | 连哪个服务是租户级配置；凭证在 Session 层注入 |
| Skills | 领域能力包，租户级定制 |
| Environment | 容器模板是租户运维配置 |

**用户级别（Session 运行层）**

| 资源 | 原因 |
|------|------|
| Session | 每次用户交互产生一个 Session |
| Vault（MCP 凭证） | 每个用户有自己的 OAuth token，Session 创建时注入 |
| Files | 用户自己的文件，挂载到 Session 容器 |
| Memory Store | 跨 Session 的用户记忆，按用户隔离 |

官方 Vault 文档明确表述：实现 **Agent 配置（workspace 级）** 与 **用户认证（session 级）** 的解耦。

**三层架构对应关系：**

```
Platform  →  Agent 模板（可选，复用基础配置）
Tenant    →  Agent 实例（定制 system prompt / tools / MCP / skills）+ Environment
User      →  Session + Vault（用户 OAuth 凭证）+ Files + Memory Store
```

system prompt 写租户的行为规范，用户身份/权限通过第一条 `user.message` 或 Session 创建时的初始消息传入，不要为每个用户创建不同的 Agent。

---

## 容器与环境

**Q：官方提供容器镜像吗？**

没有官方容器镜像，也不支持自带镜像。Anthropic 提供固定的托管环境，底层是 Ubuntu 22.04 LTS x86_64，无法替换基础镜像，只能在 Environment 配置里预装额外的包。

**固定规格：**

| 项目 | 值 |
|------|----|
| OS | Ubuntu 22.04 LTS (x86_64) |
| 内存 | 最高 8 GB |
| 磁盘 | 最高 10 GB |
| 网络 | 默认关闭，需在 Environment 中显式开启 |

预装：Python 3.12、Node.js 20、Go 1.22、Rust、Java 21、Ruby、PHP、C/C++、SQLite、git、curl、jq、ripgrep 等。

**只能通过 Environment 定制：**

```json
{
  "packages": {
    "pip": ["pandas", "numpy"],
    "npm": ["typescript"],
    "apt": ["ffmpeg"]
  },
  "network": { "policy": "limited", "allowed_domains": ["api.example.com"] }
}
```

**Q：cloud-containers 页面管理的是什么？**

这个页面只是参考手册，列出容器里预装了什么，不是管理页面。容器本身通过 Environment 来管理。cloud-containers 是只读的规格说明书，Environment 才是实际操作的配置层。

---

## Memory Store

**Q：Memory 都有哪些？用户可以创建 Memory 吗？**

Memory Store 只能由应用后端（API Key）创建，终端用户无法直接创建。

**概念层次：**

```
Memory Store（memstore_...）    ← 后端创建/管理
└── Memory（具体文件，按 path 寻址）← 后端可写，Agent 在 Session 中也可写
    └── Memory Version（memver_...）← 每次写入自动生成，不可变
```

**操作权限：**

| 操作 | 谁能做 |
|------|--------|
| 创建/归档/删除 Memory Store | 后端（API Key） |
| 预置 Memory 内容 | 后端 |
| Session 中读写 Memory 文件 | Agent（工具调用），需挂载为 `read_write` |
| 查看版本历史、回滚、合规删除 | 后端 |

**关键限制：**

- 每个 Session 最多挂载 **8 个** Memory Store
- 单个 Memory 文件上限 **100KB**（约 25K tokens）
- 版本历史保留 **30 天**
- Archive 是单向操作，不可撤销
- 作用域是 Workspace 级

**多租户场景下的典型用法：**

```
租户 A
├── Memory Store: 租户级规范（read_only）  ← 所有用户共享，后端维护
└── Memory Store: 用户甲的记忆（read_write）← 按用户创建，挂载到该用户的 Session
```

**Q：Memory 挂载在容器路径上，重启后会丢失吗？Memory 是否集中存储？**

不会丢失。Memory Store 和容器是独立的：

```
容器（Session 生命周期，临时）
└── /mnt/memory/        ← 只是挂载点，类似 Docker Volume

Memory Store（Workspace 级，持久化在 Anthropic 服务器）
└── 实际数据在这里      ← Session 结束后依然存在
```

Agent 写 `/mnt/memory/prefs.md` 时，数据实时同步回 Memory Store 并生成版本记录。容器销毁后 Memory Store 完好无损，下次新 Session 挂载同一个 Store，文件还在。

**关于集中存储：**

- **存储是集中的**：Memory Store 持久化在 Anthropic 服务器，Workspace 内任何 Session 都可以挂载同一个 Store
- **但结构是自己设计的**：官方不会自动汇总所有 Session 的信息，需要自己决定存储策略

```
方案A：每个用户一个 Store（隔离）
  user_001 → memstore_aaa
  user_002 → memstore_bbb

方案B：租户共享一个 Store（汇聚）
  tenant_x 所有用户 → memstore_ccc
  ⚠️ 并发写入需要 content_sha256 乐观锁

方案C：混合（用户私有 read_write + 租户共享 read_only）
  每个 Session 同时挂两个 Store
```
