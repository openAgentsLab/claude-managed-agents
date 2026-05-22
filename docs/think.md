# Anthropic Managed Agents 学习笔记

---

## Programmatic Tool Calling

**Q: 这是什么意思？**

让 Claude 用 Python 代码一次性批量调用多个工具，而不是一轮一轮地调用。

| 传统方式 | Programmatic 方式 |
|---|---|
| Call tool A → 等结果 → Call tool B → 等结果 | 一段 Python 代码，asyncio.gather 并发调用 A、B、C |
| N 个工具 = N 轮推理 | N 个工具 = 1 轮推理 |
| 所有中间结果进 context window | 中间结果不进 context window |
| 每次工具结果都计 token | 工具结果不计 input/output token |

**Q: 中间结果不进 context window 什么意思？工具调用的结果不都需要交给模型吗？**

关键在于谁来"消费"工具结果：

- 传统方式：每次工具结果都作为 `tool_result` 消息追加到对话历史，模型每轮都要读完整个 context
- Programmatic 方式：Claude 生成 Python 代码 → 沙箱执行 → 中间结果由 Python 代码处理 → 只有最终 return 值回到 context

类比：传统方式像老板每拿到一份资料就来问你一次；Programmatic 方式像助理一次性收集所有资料、整理好摘要，再来找你做最终决策。

**Q: Python 代码通常也是直接把内容给模型吧，他还能怎么预处理这些内容？**

Python 代码可以做真正的预处理：

```python
# 过滤：只返回包含 "auth" 的行，而不是三个完整文件
files = await asyncio.gather(read_file("a.go"), read_file("b.go"))
return [line for f in files for line in f.split('\n') if 'auth' in line]

# 条件分支：根据结果决定是否继续读文件
log = await bash("git log --oneline -50")
if "security" in log:
    return await bash("git show HEAD")
return "no security commits"
```

但如果 Claude 写的代码没有任何处理直接 return，节省的只是多轮往返的 overhead，而不是 context 大小本身。

**Q: 这个 Python 代码也是模型生成的吧？所以可以自定义处理逻辑、预处理内容？**

对。整个流程是：
1. 模型生成 Python 代码（包含预处理逻辑）
2. 沙箱执行这段代码（调用工具、处理结果）
3. 只有最终 return 值回到 context

模型在"写代码"这一步就已经决定了要调用哪些工具、结果怎么过滤裁剪、最后返回什么给自己看。本质上是模型在用代码描述自己的信息需求。

---

## Bash Tool

**Q: 这段说的是什么？**

给 Claude 用的 Bash Tool，让模型能执行 shell 命令。

- **持久 bash session**：同一 session 里环境变量、工作目录状态保持
- **API 是无状态的**：客户端负责维护 session（保持一个长期运行的 bash 子进程）
- **每次 API 调用固定增加 245 input tokens**：Bash tool 定义本身占用，是固定成本
- **不支持交互命令**：vim、less、sudo 要密码这类需要用户输入的命令不行

安全要求：必须在 Docker/VM 隔离环境运行；用命令白名单而非黑名单；设置 ulimit 资源约束；长期任务推荐 git checkpoint 模式。

**Q: 持久 bash session 如何实现？**

在客户端保持一个长期运行的 bash 子进程，不要每次都新开一个：

```python
class BashSession:
    def __init__(self):
        self.process = subprocess.Popen(["bash"], stdin=subprocess.PIPE, stdout=subprocess.PIPE)
    
    def run(self, command: str) -> str:
        marker = "___END___"
        self.process.stdin.write(f"{command}\necho {marker}\n".encode())
        self.process.stdin.flush()
        output = []
        while True:
            line = self.process.stdout.readline().decode()
            if marker in line:
                break
            output.append(line)
        return "".join(output)

session = BashSession()  # 全程保活，跨多次 Claude API 调用复用
```

bash 进程自己维护状态（env、cwd、shell 变量），不需要手动保存和恢复。`restart: true` 就是把进程 kill 掉再重新 Popen 一个。

---

## Tool Search Tool

**Q: 这一点的目的是什么？**

解决"工具太多"的 token 问题。正常情况下所有工具定义都要放进 context，1000 个工具 = 20 万 token，即使只用到 3 个工具也全部占着。

Tool Search Tool 的思路：把工具定义从 context 里拿出去，变成按需查找：
- 初始 context 只有工具目录（名称+简短描述）
- Claude 搜索需要的工具 → 完整定义展开进 context
- 节省 85%+ 的工具定义 token

**Q: 这样的话，理论上执行工具会多一些循环次数吧？**

对，这是明确的 trade-off：每次用一个新工具都多一次搜索的往返。

但工具多到一定量级后，省 token 的收益远超多一轮的代价——token 多意味着成本高、速度慢（prefill 慢）、context 被撑大影响推理质量。本质是**用延迟换空间**。

---

## Agent Setup

**Q: 具体是什么流程？预先配置好 agent？还是创建会话时动态创建？**

两个独立步骤：

1. **预先定义 Agent（一次性/变更时）**：`POST /agents`，配置 name、model、system、tools 等，返回 agent_id + version
2. **创建 Session 时引用 Agent（每次对话）**：`POST /sessions { agent_id }`，返回 session_id

Agent 定义 ≈ 部署一个服务；Session ≈ 打开一个连接。Agent 改了版本递增，旧 session 不受影响。

**Q: 会话请求时，是不是要根据配置初始化相关 agent 代码逻辑，加载工具等？**

对，Session 创建时会有初始化工作，由 Anthropic 的基础设施来做，不是客户端：
- Session 创建时：查找 agent_id 对应配置、初始化空对话历史、MCP server 建立连接
- 每次推理时：system prompt、工具定义装进 context，如配了 Tool Search Tool 则延迟加载

Managed 的核心价值：配置和运行时都托管给 Anthropic，客户端只管发消息。

**Q: 应该还有一些内置工具吧？每次肯定会加载的？**

这类工具（bash、computer use 等）通常需要在 agent 定义的 `tools` 字段里显式声明才会加载，不是自动存在的。但 Managed Agents 作为托管服务可能有不同行为，具体以文档为准。

---

## Sessions 状态机

**Q: 为什么 session 还有状态啊？这有什么用？**

因为 Managed Agent 的执行是异步的、长时运行的：

| 状态 | 含义 |
|------|------|
| `idle` | 等待用户事件 |
| `running` | 正在处理，不能发新消息 |
| `rescheduling` | 基础设施临时问题，自动重试 |
| `terminated` | 永久结束 |

删除运行中的 Session 需要先发 interrupt，让 agent 干净地停下来，避免留下脏状态。

**Q: 为什么这么设计？优势在哪？**

本质是把 Agent 从"API 调用"变成了"后台 Job"：

1. **连接断了任务不丢**：重连后 poll 状态，任务还在 running，等 idle 再取结果
2. **Human-in-the-loop 自然支持**：Agent 需要人确认时回到 idle，人发消息后继续
3. **基础设施故障透明恢复**：rescheduling 状态自动迁移重试，对客户端透明
4. **多客户端观察同一任务**：同一 session_id，移动端、Web 端都可以 poll

---

## Events & Streaming

**Q: 这段说的是什么？用在什么场景？**

客户端和 Agent 之间的实时双向事件流（20+ 种事件类型）：

```
客户端 →  user.message          Agent 开始处理
       ←  agent.thinking        思考中
       ←  agent.tool_use        调工具
       ←  agent.tool_result     工具结果
       ←  agent.message         回复内容
       ←  session.status_idle   完成
```

`agent.custom_tool_use` 场景：Agent 需要调用你自己的数据库/API，你执行后发回 `user.custom_tool_result`。

`stop_reason: requires_action` 场景：Human-in-the-loop 确认门控，Agent 调用危险操作时暂停等人工审核，人工回复 `user.tool_confirmation` 后继续。

**Q: 如何订阅事件流？**

用 SSE（Server-Sent Events）长连接：

```python
with httpx.stream("GET", f"/sessions/{session_id}/events",
    headers={"Authorization": f"Bearer {api_key}"}
) as response:
    for line in response.iter_lines():
        if line.startswith("data:"):
            event = json.loads(line[5:])
            handle_event(event)
```

SSE 特点：单向推送（服务端→客户端），客户端要发数据单独 POST；连接断了可带 `Last-Event-ID` 头重连不漏事件。

**Q: 事件流可以多次回放吗？**

可以。实时 SSE 流只能消费一次，但事件会持久化存储，可多次查询历史：

```python
GET /sessions/{id}/events?page=1  # 拉取历史事件，分页，可多次
```

适用场景：连接中断补拉遗漏事件、离线分析完整执行过程、排查生产问题复现行为。

---

## Skills 按需加载

**Q: Skill 如何实现按需加载？**

类似 Tool Search Tool 的 Progressive Disclosure 模式：
- 初始 context 只有 skill 名称和简短描述
- Claude 判断需要某个 skill 时，完整定义才展开进 context

Skill 是 session 级别，最多 20 个/session，Anthropic 预置了 pptx、xlsx、docx、pdf 处理技能。

---

## Multi-Agent

**Q: 介绍一下 Multi-Agent？**

一个 Orchestrator Agent 协调多个 Sub-Agent：

- **共享文件系统 + 独立对话历史**：Sub-Agent A 写的文件 B 可以读，但各自推理过程互不干扰
- **视图分层**：Session 主流显示汇总视图，Thread 流显示单 Agent 详细视图
- **只支持一层委托**：Orchestrator → Sub-Agent，Sub-Agent 不能再继续委托，防止递归失控
- **`session_thread_id` 路由**：多 Sub-Agent 并发时，工具确认回复路由到正确的 Thread

典型场景：分析代码库并生成文档，Orchestrator 分发给代码分析、文档生成、测试运行三个 Sub-Agent 并发执行。

---

## Define Outcomes

**Q: 这是什么意思？**

目标驱动的自动评估迭代——把"人工 review → 打回修改"的循环自动化：

1. 定义 rubric（验收标准，Markdown 格式）
2. 发送 `user.define_outcome` 事件
3. Agent 做一版输出
4. 独立 Grader（独立 context）对照 rubric 评估，返回 per-criterion 细分反馈
5. `needs_revision` → 反馈自动注入 Agent context → Agent 修改重做
6. 循环直到 `satisfied` 或 `max_iterations_reached`

Grader 用独立 context 的原因：避免受主 Agent 实现过程影响，判断更客观。

**Q: 我每次提需求都需要提供验收标准吗？**

不是必须的，Define Outcomes 是可选功能。适合有明确、可量化标准的场景（代码必须通过测试、文档必须覆盖所有 API）。开放性任务、标准主观难以量化、一次性简单任务直接发消息就够了。

**Q: Grader 评估完，如何交给 Agent 看到反馈修改重做？**

这个循环由 Anthropic 基础设施自动编排，客户端不需要干预：
- Grader 评估结果自动注入 Agent 的 context
- Agent 读到结构化反馈，继续下一轮修改
- 客户端只看到最终 `session.status_idle`（stop_reason: satisfied 或 max_iterations_reached）

---

## Observability

**Q: 这块具体应该怎么实现？**

不同层面分别处理：

- **Console 追踪视图**：直接用 Anthropic 控制台 UI，无需写代码
- **Token 计费数据**：监听 `span.model_request_end` 事件，读取 `model_usage` 写入监控系统
- **错误处理**：监听 `session.error`，判断 `retry_status` 决定是告警还是等自动重试
- **历史事件**：`GET /sessions/{id}/events` 拉取完整分页历史，事后复现执行过程
- **系统提示加日志指令**（最实用）：在 system prompt 里让 Agent 输出 `[ACTION]`、`[UNCERTAIN]` 等结构化推理日志

实际项目：开发阶段靠系统提示日志 + 事件流打印；生产环境用 `span.model_request_end` 接监控、`session.error` 接告警。

---

## Migration（从 Messages API 迁移）

**Q: Agent 循环变了？**

对，最核心的变化：

| | Messages API | Managed Agents |
|---|---|---|
| 循环控制 | 客户端 while 循环 | 服务端托管 |
| 对话历史 | 客户端 messages 数组 | 服务端 Session 事件日志 |
| 工具执行 | 全部客户端 | 内置工具服务端，自定义工具客户端 |
| 结束判断 | `stop_reason == end_turn` | `session.status_idle` 事件 |

从"主动驱动循环"变成"被动响应事件"，复杂度从客户端转移到了服务端。

**Q: 工具还能在客户端执行？服务端不是也有工具吗？**

两类工具要区分：

- **服务端内置工具**（Anthropic 执行）：bash、文件读写、MCP Connector、Skills。客户端完全不感知，只看到 `agent.tool_use` + `agent.tool_result`。
- **自定义工具**（客户端执行）：逻辑在你自己服务器上（查私有数据库、调内部 API）。触发 `agent.custom_tool_use` 事件，客户端执行后发回 `user.custom_tool_result`。

"工具还在客户端执行"特指需要访问私有资源的自定义工具，不是所有工具。

---

## 测试与评估

**Q: 具体应该怎么做？**

**第一步：定义成功标准（SMART）**

差：Agent 能正确回答问题
好：对给定代码审查任务，能识别出所有安全漏洞，误报率 < 10%，耗时 < 30s

**第二步：建评测集**

覆盖正常 case + 边缘 case，可用 Claude 批量生成变体（给几个示例 → 生成 50 个）。

**第三步：选打分方式（优先级）**

1. 代码打分（精确匹配）→ 最可靠
2. LLM 打分（详细 rubric + 先推理再判断）→ 次选
3. 人工打分 → 最后选

**第四步：跑评测，记基线，迭代对比**

有了基线数据，每次改 prompt 才知道是变好了还是变差了。

**对症下药：**

| 问题 | 优先手段 |
|------|---------|
| 输出乱编 | 允许说"不知道"，要求附引用 |
| 响应太慢 | 换 Haiku、开 Prompt Cache |
| 输出格式不稳定 | Structured Outputs |
| 被越狱 | Haiku 预筛 + 输出监控 |

---

## Prompt Caching

**Q: 具体怎么操作？**

给稳定内容打上 `cache_control` 标记：

```python
client.messages.create(
    system=[{
        "type": "text",
        "text": "很长的系统提示...",
        "cache_control": {"type": "ephemeral"}  # 打标记，缓存到这里
    }],
    messages=[{"role": "user", "content": "用户消息（变化内容，不 cache）"}]
)
```

内容排列顺序：稳定内容（系统提示、工具定义、历史对话）在前，变化内容（当前用户消息）在后。变化内容放前面会导致后面的缓存全部失效。

TTL：默认 5 分钟（1.25x 写入价格），可配置 1 小时（2x 写入价格）。

验证命中：检查 `usage.cache_read_input_tokens`（命中）和 `usage.cache_creation_input_tokens`（写入），命中后费用仅 0.1x。

---

## Streaming vs Session 事件流

**Q: Streaming 和之前的事件流是不是一样的？**

不一样，虽然底层都是 SSE，但层级不同：

- **Streaming（Messages API）**：token 级别，模型输出一个字吐一个字（`content_block_delta`）
- **Session 事件流（Managed Agents）**：任务编排级别，描述 Agent 在做什么（`agent.tool_use`、`session.status_idle`）

Managed Agents 内部每条 `agent.message` 底层也是 token 流式生成的，但平台封装好了，客户端看到的是完整消息事件。

---

## Compaction

**Q: 介绍一下这里的压缩技术？**

解决长对话 context 越来越大的问题。触发阈值（默认 150k token）后，把旧消息压缩成摘要 block 插入，原始消息丢弃：

```
[旧对话原始内容]（丢弃）
↓
[compaction block: "前面讨论了X、决定了Y、完成了Z..."]
[近期对话]（保留）
```

本质是取舍：损失旧对话细节，换来 token 成本可控和突破 context 上限。

**Q: `instructions` 参数是用户传递的？**

是开发者写的，硬编码在系统配置里，不是终端用户传的。告诉压缩模型"哪些信息重要，要保留在摘要里"：

```python
compaction={"instructions": "保留：所有文件修改记录、未解决的 bug、架构决策"}
```

**Q: `pause_after_compaction` 是用户自己传递的吗？**

也是开发者配置的。开发/测试时开启，检查摘要质量；生产环境关闭，全自动不打断。压缩后 session 进入 `idle`（stop_reason: compaction_pause），开发者检查 compaction block 内容后决定是否补充再继续。

---

## Context Editing

**Q: 这是什么场景？具体如何使用？**

这是 Messages API 场景下的技术（不是 Managed Agents）。因为 Messages API 下对话历史由客户端自己维护，可以直接操作 messages 数组：

```python
# 工具结果清除：分析完大文件后替换为占位符
messages[3]["content"] = "[日志内容已清除，分析结果见上]"

# thinking block 清除：只保留最近 N 个
# 客户端 compaction：自己调便宜模型生成摘要，截断历史
```

**Q: 客户端怎么实现？**

Managed Agents 用户不需要实现这个，历史在服务端，用服务端 Compaction 即可。这个技术适用于自己用 Messages API 手写 Agent 循环的场景，messages 数组在手里，想怎么改就怎么改。

---

## Token Counting

**Q: 这个接口是免费的？**

对，`POST /v1/messages/count_tokens` 调用本身不收费，不消耗 token，是估算值（与实际计费可能略有差异）。

典型用法：发送前先估算，判断是否需要先压缩 context 再发送。

注意：有独立的速率限制（100-8000 RPM），高频调用要注意别把这个额度打满。
