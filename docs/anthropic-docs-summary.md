# Anthropic 官方文档完整解析

> 基于 https://platform.claude.com/docs 全部约 120 个页面的分析整理
> 整理目的：为 ai-coding 项目架构设计和实现提供参考

---

## 所有页面地址

```
https://platform.claude.com/docs/en/intro
https://platform.claude.com/docs/en/get-started

# About Claude
https://platform.claude.com/docs/en/about-claude/models/overview
https://platform.claude.com/docs/en/about-claude/models/whats-new-claude-4-6
https://platform.claude.com/docs/en/about-claude/models/choosing-a-model
https://platform.claude.com/docs/en/about-claude/models/migration-guide
https://platform.claude.com/docs/en/about-claude/pricing
https://platform.claude.com/docs/en/about-claude/use-case-guides/overview
https://platform.claude.com/docs/en/about-claude/use-case-guides/content-moderation
https://platform.claude.com/docs/en/about-claude/use-case-guides/customer-support-chat
https://platform.claude.com/docs/en/about-claude/use-case-guides/legal-summarization
https://platform.claude.com/docs/en/about-claude/use-case-guides/ticket-routing
https://platform.claude.com/docs/en/about-claude/glossary
https://platform.claude.com/docs/en/about-claude/model-deprecations

# Build with Claude
https://platform.claude.com/docs/en/build-with-claude/overview
https://platform.claude.com/docs/en/build-with-claude/working-with-messages
https://platform.claude.com/docs/en/build-with-claude/streaming
https://platform.claude.com/docs/en/build-with-claude/context-windows
https://platform.claude.com/docs/en/build-with-claude/token-counting
https://platform.claude.com/docs/en/build-with-claude/adaptive-thinking
https://platform.claude.com/docs/en/build-with-claude/effort
https://platform.claude.com/docs/en/build-with-claude/extended-thinking
https://platform.claude.com/docs/en/build-with-claude/fast-mode
https://platform.claude.com/docs/en/build-with-claude/prompt-caching
https://platform.claude.com/docs/en/build-with-claude/compaction
https://platform.claude.com/docs/en/build-with-claude/context-editing
https://platform.claude.com/docs/en/build-with-claude/handling-stop-reasons
https://platform.claude.com/docs/en/build-with-claude/structured-outputs
https://platform.claude.com/docs/en/build-with-claude/vision
https://platform.claude.com/docs/en/build-with-claude/pdf-support
https://platform.claude.com/docs/en/build-with-claude/files
https://platform.claude.com/docs/en/build-with-claude/citations
https://platform.claude.com/docs/en/build-with-claude/search-results
https://platform.claude.com/docs/en/build-with-claude/embeddings
https://platform.claude.com/docs/en/build-with-claude/batch-processing
https://platform.claude.com/docs/en/build-with-claude/multilingual-support
https://platform.claude.com/docs/en/build-with-claude/skills-guide
https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/overview
https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices
https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-tools
https://platform.claude.com/docs/en/build-with-claude/administration-api
https://platform.claude.com/docs/en/build-with-claude/workspaces
https://platform.claude.com/docs/en/build-with-claude/usage-cost-api
https://platform.claude.com/docs/en/build-with-claude/claude-code-analytics-api
https://platform.claude.com/docs/en/build-with-claude/api-and-data-retention
https://platform.claude.com/docs/en/build-with-claude/data-residency
https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock
https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock-research-preview
https://platform.claude.com/docs/en/build-with-claude/claude-on-vertex-ai
https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry

# Agents and Tools — Tool Use
https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview
https://platform.claude.com/docs/en/agents-and-tools/tool-use/how-tool-use-works
https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-reference
https://platform.claude.com/docs/en/agents-and-tools/tool-use/bash-tool
https://platform.claude.com/docs/en/agents-and-tools/tool-use/text-editor-tool
https://platform.claude.com/docs/en/agents-and-tools/tool-use/code-execution-tool
https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool
https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool
https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-fetch-tool
https://platform.claude.com/docs/en/agents-and-tools/tool-use/memory-tool
https://platform.claude.com/docs/en/agents-and-tools/tool-use/fine-grained-tool-streaming
https://platform.claude.com/docs/en/agents-and-tools/tool-use/programmatic-tool-calling
https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool
https://platform.claude.com/docs/en/agents-and-tools/tool-use/advisor-tool

# Agents and Tools — Agent Skills & MCP
https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview
https://platform.claude.com/docs/en/agents-and-tools/agent-skills/quickstart
https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices
https://platform.claude.com/docs/en/agents-and-tools/agent-skills/enterprise
https://platform.claude.com/docs/en/agents-and-tools/mcp-connector
https://platform.claude.com/docs/en/agents-and-tools/remote-mcp-servers

# Managed Agents
https://platform.claude.com/docs/en/managed-agents/overview
https://platform.claude.com/docs/en/managed-agents/onboarding
https://platform.claude.com/docs/en/managed-agents/quickstart
https://platform.claude.com/docs/en/managed-agents/agent-setup
https://platform.claude.com/docs/en/managed-agents/sessions
https://platform.claude.com/docs/en/managed-agents/events-and-streaming
https://platform.claude.com/docs/en/managed-agents/tools
https://platform.claude.com/docs/en/managed-agents/environments
https://platform.claude.com/docs/en/managed-agents/cloud-containers
https://platform.claude.com/docs/en/managed-agents/files
https://platform.claude.com/docs/en/managed-agents/vaults
https://platform.claude.com/docs/en/managed-agents/memory
https://platform.claude.com/docs/en/managed-agents/skills
https://platform.claude.com/docs/en/managed-agents/mcp-connector
https://platform.claude.com/docs/en/managed-agents/permission-policies
https://platform.claude.com/docs/en/managed-agents/multi-agent
https://platform.claude.com/docs/en/managed-agents/define-outcomes
https://platform.claude.com/docs/en/managed-agents/observability
https://platform.claude.com/docs/en/managed-agents/github
https://platform.claude.com/docs/en/managed-agents/migration

# Test & Evaluate
https://platform.claude.com/docs/en/test-and-evaluate/develop-tests
https://platform.claude.com/docs/en/test-and-evaluate/eval-tool
https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/handle-streaming-refusals
https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/increase-consistency
https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/mitigate-jailbreaks
https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/reduce-hallucinations
https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/reduce-latency
https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/reduce-prompt-leak

# API Reference
https://platform.claude.com/docs/en/api/overview
https://platform.claude.com/docs/en/api/versioning
https://platform.claude.com/docs/en/api/beta-headers
https://platform.claude.com/docs/en/api/errors
https://platform.claude.com/docs/en/api/rate-limits
https://platform.claude.com/docs/en/api/service-tiers
https://platform.claude.com/docs/en/api/ip-addresses
https://platform.claude.com/docs/en/api/supported-regions
https://platform.claude.com/docs/en/api/client-sdks
https://platform.claude.com/docs/en/api/openai-sdk
https://platform.claude.com/docs/en/api/sdks/python
https://platform.claude.com/docs/en/api/sdks/typescript
https://platform.claude.com/docs/en/api/sdks/go
https://platform.claude.com/docs/en/api/sdks/java
https://platform.claude.com/docs/en/api/sdks/csharp
https://platform.claude.com/docs/en/api/sdks/ruby
https://platform.claude.com/docs/en/api/sdks/php
https://platform.claude.com/docs/en/api/sdks/cli
https://platform.claude.com/docs/en/api/beta/sessions
https://platform.claude.com/docs/en/api/beta/sessions/events/stream

# Release Notes & Resources
https://platform.claude.com/docs/en/release-notes/overview
https://platform.claude.com/docs/en/release-notes/system-prompts
https://platform.claude.com/docs/en/resources/overview
```

---

## 一、关于 Claude 模型（About Claude）

### Models Overview
当前 Claude 模型分三档：

| 模型 | Context | Max Output | 输入价格 | 输出价格 | 定位 |
|------|---------|-----------|---------|---------|------|
| claude-opus-4-6 | 1M token | 128k | $5/MTok | $25/MTok | 最强智能，复杂推理/多步 agent |
| claude-sonnet-4-6 | 1M token | 64k | $3/MTok | $15/MTok | 速度与智能最佳平衡（推荐默认） |
| claude-haiku-4-5 | 200k token | 64k | $1/MTok | $5/MTok | 最快最便宜，简单任务/高吞吐 |

Batch API 配合 `output-300k-2026-03-24` beta header，Opus 4.6 和 Sonnet 4.6 最大输出扩展到 300k token。模型 ID 中的日期（如 `20240620`）是快照版本号，保证跨平台行为稳定。

### What's New in Claude 4.6
Claude 4.6 系列的关键变化：

- **Adaptive Thinking**：`thinking: {type: "adaptive"}` 取代旧的 `type: "enabled"` + `budget_tokens`，Claude 自动决定思考深度
- **Effort 参数 GA**：`output_config.effort` 的 `max` 级别对 Opus 4.6 正式可用
- **代码执行免费**：与 `web_search_20260209` 或 `web_fetch_20260209` 联用时，代码执行不额外计费
- **动态过滤**：web search/fetch 的新版本支持 Claude 先写代码过滤结果再注入 context，降低 token 消耗
- **Breaking Change**：Opus 4.6 上移除了 Prefill（最后一条 assistant 消息）——返回 400 错误
- **Compaction API (beta)**：服务端自动压缩历史 context，支持实际无限长对话
- **Fast Mode (beta)**：Opus 4.6 最高 2.5x 输出速度，$30/$150/MTok（6x 标准价格）

### Choosing a Model
决策框架：从两端出发
- 从 **Haiku 4.5** 出发：原型阶段、高吞吐、延迟敏感（分类、摘要、简单问答）
- 从 **Opus 4.6** 出发：复杂推理、多步骤 agent、高精度要求的任务

核心原则：先建评测集，再基于真实数据选模型；模型选择是性价比最高的单一优化手段。

### Pricing
关键定价结构：
- **Prompt Cache 读取**：0.1x 基础输入价格（5 分钟 TTL 的写入为 1.25x，1 小时为 2x）
- **Batch API**：50% 折扣（输入+输出）
- **Fast Mode (Opus 4.6)**：$30/$150/MTok（6x 标准）
- **Web Search**：$10/千次（token 另计）
- **Web Fetch**：无额外费用（仅 token 费用）
- **Code Execution**：与 web search/fetch 联用时免费；单独使用 $0.05/小时（每月 1550 小时免费额度）
- **Managed Agents 运行时**：$0.08/session-hour（替代容器计费）
- **数据驻留（美国）**：所有 token 类别 1.1x 乘数，仅 Opus 4.6+

### Glossary
核心术语定义：
- **Context Window**：模型工作记忆（非训练数据）；1M token = 约 750k 单词
- **HHH**：Helpful, Honest, Harmless —— Anthropic 对 Claude 的核心设计原则
- **MCP（Model Context Protocol）**：连接 LLM 与外部工具的开放标准，被称为"AI 的 USB-C"
- **RAG（检索增强生成）**：结合外部知识检索与生成的技术
- **RLHF**：强化学习从人类反馈 —— Claude 对齐训练的核心技术
- **Tokens**：英文约 3.5 字符/token，中文约 1-2 字符/token；输入输出均按 token 计费
- **Temperature**：控制输出随机性；即使设为 0 仍有少量非确定性

---

## 二、构建应用（Build with Claude）

### Adaptive Thinking
**推荐方式（Claude 4.6 系列）**：`thinking: {type: "adaptive"}` 配合 `output_config: {effort: "medium"}`。Claude 自动决定是否思考以及思考多深，不需要手动设置 budget_tokens。

关键特性：
- 自动启用 Interleaved Thinking（工具调用之间也会推理）
- `display: "summarized"` 或 `"omitted"` 控制是否返回思考文本（即使 omitted 也按 token 计费）
- Claude Mythos Preview 上是默认模式
- 可通过系统提示影响思考频率，但需测试其影响

### Extended Thinking（旧方式）
通过 `thinking: {type: "enabled", budget_tokens: N}` 手动控制思考深度。在 Opus 4.6 / Sonnet 4.6 上已废弃，建议迁移到 Adaptive Thinking。

关键注意：
- Thinking block 携带加密的 `signature` 字段，多轮工具调用中必须原样保留
- Thinking token 仅计费一次（输出时），不作为后续轮的输入重新计费
- `display: "summarized"` vs `"omitted"` —— 即使 omit 也按完整 token 计费

### Effort Parameter
`output_config.effort` 四档控制 Claude 在整个响应（文本 + 工具调用 + 思考）上消耗的 token 量：

| Level | 适用场景 | 说明 |
|-------|---------|------|
| `max` | 最复杂任务 | 无 token 约束，最深推理 |
| `high` | 默认值 | 等同于不设置该参数 |
| `medium` | Sonnet 4.6 推荐 | 平衡速度与质量 |
| `low` | subagent、简单查询 | 最快最便宜，工具调用更少更合并 |

对编程 agent：主 brain 用 `high`，subagent（文件扫描、简单分类）用 `low`。

### Fast Mode
Beta 功能（waitlist 制），仅限 Opus 4.6。最高 2.5x 输出 token/秒，适合需要快速生成大量代码的场景。

注意点：
- 提升的是输出速度（OTPS），不是首 token 延迟（TTFT）
- `response.usage.speed` 字段确认实际使用的速度模式
- 切换 fast/standard 会使 prompt cache 失效
- 不兼容 Batch API 和 Priority Tier
- 遭遇 429 时应回退到标准模式

### Prompt Caching     =====》 TODO
最重要的成本优化手段之一：

- 读取仅 **0.1x** 基础输入价格；写入 **1.25x**（5 分钟 TTL）或 **2x**（1 小时 TTL）
- 每次请求最多设置 **4 个缓存断点**
- **Prompt Cache 命中不计入 ITPM 速率限制**（大多数模型），是提高有效吞吐量的关键
- 最小可缓存长度：1024-4096 token（取决于模型）
- 应将稳定内容（系统提示、工具定义）放前，变化内容（用户消息）放后
- thinking block、speed 设置、图片变化会使缓存失效

对 ai-coding：系统提示和工具定义每次请求都重复，设置 cache_control 可节省 ~90% 的系统提示 token 费用。

### Streaming
通过 SSE（Server-Sent Events）实现 token 级流式输出：

关键事件类型：
- `message_start` → `content_block_start` → `content_block_delta`（text_delta / thinking_delta）→ `content_block_stop` → `message_delta` → `message_stop`
- 流式中扩展思考：`signature_delta` 在 `content_block_stop` 前发出
- Compaction block 作为单个 delta 流式（无中间流）

Go SDK 通过 `.Stream()` 方法提供高层抽象。

### Compaction     =====》 TODO
Beta 功能（`compact_20260112`）：服务端自动压缩旧对话历史。

核心机制：
- 触发阈值：默认 150k token（最小 50k，可配置）
- 压缩结果作为 `compaction` block 插入，后续请求自动丢弃被压缩内容
- `pause_after_compaction: true`：压缩后暂停，允许检查/修改摘要再继续
- `instructions` 字段控制摘要保留什么
- `usage.iterations` 数组提供每步计费细分

支持模型：Mythos Preview、Opus 4.6、Sonnet 4.6。

### Context Windows
- Opus 4.6 / Sonnet 4.6：**1M token** context window
- Haiku 4.5：200k token
- **Context Rot**：随着 context 填满，准确率下降 —— context 大小 ≠ 可随意填满
- 扩展思考 token 只计费一次，不作为后续输入重新计费
- Context Awareness：Sonnet 4.6、4.5、Haiku 4.5 自动注入 `<budget:token_budget>` 和 `<system_warning>` 告知剩余预算

### Context Editing
客户端细粒度上下文控制，与服务端 Compaction 互补：

1. **工具结果清除**：将不再需要的旧工具输出替换为占位文本
2. **thinking block 清除**：清除旧轮次的 thinking block，保留文本回复
3. **保留最近 N 个 thinking block**：维持推理连续性
4. **客户端 Compaction**：程序化生成摘要并截断历史

适用场景：需要精确运行时控制时；服务端 Compaction 不够细粒度时。

### Batch Processing
Message Batches API —— 50% 折扣的异步处理：

- 每批请求独立处理，无依赖关系
- 大多数批次在 1 小时内完成
- 不支持 Fast Mode；不适用 ZDR
- 适合：批量评估、内容审核、大规模数据分析

### Structured Outputs
约束解码（非提示词技巧）保证输出合规：

- `output_config.format`：JSON schema 约束整体响应格式
- `strict: true`（工具定义）：保证工具调用参数合法
- 两者可独立使用也可组合
- 不兼容 Citations 功能
- ZDR 合规（"limited technical retention"）

### Token Counting
`POST /v1/messages/count_tokens`：免费预估 token 数，用于速率限制管理和成本估算。

- 返回 `{input_tokens: N}`，是估算值（可能与实际计费略有差异）
- 支持工具、图片、PDF、扩展思考块的计数
- 速率限制独立于 Messages API（100-8000 RPM 取决于 Tier）

### Files API
文件一次上传，多次复用（Beta）：

- `file_id` 用于 document、image、container_upload 内容块
- 最大 500MB/文件，500GB/组织
- 文件 API 操作免费；在 Messages 中使用时按输入 token 计费
- 不可在 Bedrock/Vertex 上使用；不适用 ZDR

### Citations
`citations: {enabled: true}` 自动生成文档引用：

- 三种文档类型：纯文本（字符索引）、PDF（页码）、自定义内容（块索引）
- `cited_text` 不计入 output token，成本高效
- 与 Prompt Caching 兼容（在文档块上设置 cache_control）
- 不兼容 Structured Outputs

### Vision
图片分析能力：

- 三种输入方式：base64 内联 / URL / file_id（推荐复用场景）
- API 最多 600 张/请求；超过 20 张时单图限制 2000x2000px
- 所有当前 Claude 模型均支持视觉能力
- 大图建议降采样以降低 token 成本

### PDF Support
- 每页约 1500-3000 text token + image token
- 三种输入：URL / base64 / Files API（推荐复用）
- 在 Bedrock Converse API 上，需要 citations 才能使用完整视觉分析
- 最大 600 页（200k context 模型为 100 页）

### Embeddings    =====》规划
Anthropic 无自有向量模型，推荐 Voyage AI：

| 模型 | 定位 |
|------|------|
| voyage-3-large | 最高质量 |
| voyage-3.5 | 平衡 |
| voyage-3.5-lite | 最快/最便宜 |
| voyage-code-3 | 代码专用 |
| voyage-law-2 | 法律专用 |

关键：`input_type` 参数（`query` vs `document`）对检索质量至关重要。

### Working with Messages
- Messages API 无状态，每次需传完整历史
- **Prefill 已在 Opus 4.6 / Sonnet 4.6 上移除**（返回 400）—— 改用 Structured Outputs
- Vision：支持 base64 / URL / file_id 三种图片输入

### Prompt Engineering Best Practices
Claude 4.x 模型的提示词工程核心指南：

- **XML 标签结构化**：`<instructions>`, `<context>`, `<example>` 隔离不同内容区域
- **并行工具调用优化**：明确要求 Claude 并行执行独立工具调用
- **多 context window 状态管理**：使用 JSON 状态文件 + git 跟踪跨轮次状态
- **Adaptive Thinking 迁移**：从 budget_tokens 迁移的注意事项
- **Subagent 编排**：为 subagent 明确划定职责范围和接口规范

---

## 三、工具使用（Agents and Tools - Tool Use）

### Tool Use Overview
工具执行的三种模式：

| 类型 | 执行方 | 示例 |
|------|--------|------|
| 用户自定义客户端工具 | 你的代码 | 自定义 API 调用、数据库查询 |
| Anthropic Schema 客户端工具 | 你的代码 | bash、text_editor、memory、computer |
| 服务端执行工具 | Anthropic 基础设施 | web_search、code_execution、web_fetch |

工具使用约有 **~346 tokens** 的系统提示开销（`auto`/`none` 模式），`any`/`tool` 模式约 313 tokens。`strict: true` 保证工具调用参数符合 schema。

### How Tool Use Works
Agentic Loop 核心流程：

```
发送请求
   → stop_reason == "tool_use"
   → 执行所有 tool_use block（可并行）
   → 发送带 tool_result 的新请求
   → 继续直到 stop_reason == "end_turn"
```

`pause_turn` 表示服务端循环达到迭代上限，需要重新发送请求继续。工具调用是工具定义、tool_use block 和 tool_result block 均计入输入 token 的原因。

### Advisor Tool（重要 Beta 功能）
**双模型协作模式**：executor 模型（Sonnet，成本低速度快）+ advisor 模型（Opus，策略智能）。

工作流程：
1. Sonnet 执行工具调用，期间触发 Advisor Tool
2. Anthropic 服务端将完整对话发给 Opus
3. Opus 返回策略建议（约 400-700 token 计划）
4. Sonnet 接收建议后继续执行

关键特性：
- `executor`/`advisor` 模型配对（有效组合列表由 API 定义）
- `advisor_tool_result` 和 `advisor_redacted_result`（Opus 拒绝时）两种响应变体
- 两层独立的 Prompt Cache（advisor 侧和 executor 侧）
- `max_uses` 参数控制单次请求的 advisor 调用次数上限
- Beta header：`advisor-tool-2026-03-01`

**对 ai-coding 的价值**：Brain 分层架构 —— Sonnet 做日常编码任务，Opus 在复杂架构决策时提供顾问意见，在保持低成本的同时获得高质量。

### Memory Tool（重要客户端工具）
基于文件系统的跨会话持久化记忆（客户端执行）：

存储路径：`/memories` 目录（开发者完全控制后端存储）

支持命令：
- `view`：列出 /memories 目录内容
- `create`：创建新记忆文件
- `str_replace`：替换文件中的特定字符串
- `insert`：在指定行后插入内容
- `delete`：删除文件
- `rename`：重命名文件

关键注意：
- 必须将所有操作沙箱化到 `/memories` 目录，防止路径遍历攻击
- 系统提示中自动注入指令，要求 Claude 在开始任务前检查记忆
- 工具类型：`memory_20250818`
- 与 Context Editing 和 Compaction 配合使用效果最佳

**对 ai-coding 的价值**：Session 层的跨会话扩展 —— 将项目状态、已完成任务、用户偏好写入 `/memories`，实现重启后恢复。

### Code Execution Tool
服务端沙箱执行 Python/Bash：

- 与 web_search/web_fetch 联用时**免费**
- 单独使用：$0.05/小时（首月 1550 小时免费）
- 两个版本：`code_execution_20250825`（Python+Bash）和 `code_execution_20260120`（额外支持 Programmatic Tool Calling）
- 容器生命周期：4.5 分钟空闲超时，30 天最大存活时间
- 不适用 ZDR

### Computer Use Tool（Beta）
屏幕截图 + 鼠标/键盘控制桌面（客户端执行）：

支持动作：`screenshot`, `left_click`, `type`, `key`, `scroll`, `zoom`（v20251124+）

核心注意：
- **Prompt Injection 风险高**，必须在隔离环境中运行
- 坐标缩放：高分辨率显示器需要缩放（API 约束图片到 ~1568px）
- 适用 ZDR（数据在你的环境中）

### Fine-Grained Tool Streaming（重要性能优化）
`eager_input_streaming: true` 开启工具参数字符级流式输出：

- 不等待 JSON 完整生成即开始流式传输
- 将大工具输入（如写大文件）的首字节延迟从 **15 秒级降至 3 秒级**
- `input_json_delta` 事件需要在 `content_block_stop` 后才能解析完整 JSON
- 注意：`max_tokens` 中途截断时可能产生不完整 JSON，需要错误处理

### Programmatic Tool Calling（重大性能突破）====》 TODO
Claude 在代码沙箱内用 Python 代码批量调用工具：

工作原理：
```python
# Claude 生成如下代码，在沙箱中执行
results = await asyncio.gather(
    read_file("/path/to/file1.go"),
    read_file("/path/to/file2.go"),
    bash("git log --oneline -10")
)
```

关键优势：
- N 轮工具推理 → 1 轮推理：中间工具结果不进 context window
- 工具结果不计入 input/output token（大幅降低成本）
- 工具以异步 Python 函数形式暴露给 Claude

配置要求：
- 需要 `code_execution_20260120`
- 工具定义中添加 `allowed_callers: ["code_execution_20260120"]`
- 不兼容 `strict: true`、`disable_parallel_tool_use`、MCP Connector 工具

**对 ai-coding 的价值**：大型重构任务中（同时读写多个文件），将 N 轮循环压缩为 1 轮，是 Harness 效果循环最重要的优化方向。

### Text Editor Tool
Schema-less 客户端工具，结构化文件编辑：

命令：`view`, `str_replace`, `create`, `insert`

注意：
- `str_replace` 需要唯一精确匹配（0 或多于 1 个匹配会返回错误）
- 最新版本 `text_editor_20250728`（Claude 4 模型）
- Claude 4 版本移除了 `undo_edit` 命令

### Bash Tool   ====》 TODO
持久 bash session（Schema-less 客户端工具）：

- session 内状态（环境变量、工作目录）在命令间持久
- API 是无状态的：客户端负责在 API 调用间维护 session
- `restart: true` 重置 session
- 每次 API 调用固定增加 **245 input tokens**
- 不支持交互命令（vim、less、密码提示）和 GUI 应用

安全要求：
- **必须在隔离环境（Docker/VM）中运行**
- 使用命令白名单，不用黑名单
- 设置 `ulimit` 资源约束
- 长期 agent 工作流推荐 git checkpoint 模式

### Web Search Tool
服务端实时网络搜索，自动生成引用：

两个版本：
- `web_search_20260209`：动态过滤（Claude 写代码预处理结果，需要 code_execution tool）
- `web_search_20250305`：基础版本

配置参数：`max_uses`, `allowed_domains`, `blocked_domains`, `user_location`
计费：$10/千次搜索（token 另计）

### Web Fetch Tool
服务端抓取 URL 完整内容：

安全约束：**Claude 只能抓取已在上下文中出现的 URL**（防止 SSRF）
- `max_content_tokens` 参数防止意外大 token 消耗
- PDF 以 base64 返回；网页以纯文本返回
- 无额外计费

### Tool Search Tool
大规模工具集的按需加载（支持最多 10000 个工具）：

两种变体：
- `tool_search_tool_regex_20251119`：Python 正则匹配
- `tool_search_tool_bm25_20251119`：BM25 自然语言搜索

关键机制：
- 工具标记 `defer_loading: true` → 从初始 context 中排除
- Claude 先搜索工具目录，获取 3-5 个最相关的工具引用
- 工具引用自动展开为完整定义
- 减少 **85%+** 的工具定义 token 开销
- 延迟加载与 Prompt Cache 兼容

### Tool Reference
所有工具的完整目录和可选属性：

**服务端执行（GA）**：`web_search`, `web_fetch`, `code_execution`, `tool_search`
**客户端执行（GA）**：`memory`, `bash`, `text_editor`
**Beta**：`advisor_20260301`, `mcp_toolset`, `computer`

可选工具属性：
- `cache_control`：工具定义缓存
- `strict`：参数验证
- `defer_loading`：延迟加载（与 Tool Search Tool 配合）
- `allowed_callers`：Programmatic Tool Calling 授权
- `input_examples`：辅助 Claude 理解工具用法
- `eager_input_streaming`：工具参数字符级流式

---

## 四、Agent 技能（Agent Skills）

### Overview
基于文件系统的模块化能力包，三级渐进加载：

| 加载级别 | 内容 | 占用 token |
|---------|------|-----------|
| 级别 1（始终加载） | SKILL.md frontmatter（name + description） | ~100 token/skill |
| 级别 2（触发时加载） | SKILL.md 完整 body | 按内容计 |
| 级别 3（按需加载） | 绑定的脚本/资源文件 | 按使用计 |

Anthropic 预置 Skill：pptx、xlsx、docx、pdf（文档生成/处理）

- 最多 20 个 Skill/Session
- 跨平台：Claude.ai（用户级）、API（工作区级）、Claude Code（文件系统）
- 不适用 ZDR；安全警告：未知来源的 Skill 类似安装不可信软件

### Best Practices
编写高效 Skill 的核心原则：

- **SKILL.md 保持 <500 行**；超出拆分为引用文件
- **描述用第三人称**，说明"做什么"和"何时用"
- **Plan-validate-execute 模式**：先生成 JSON 计划，验证通过再执行，防止破坏性操作
- 避免深层嵌套文件引用（所有引用距 SKILL.md 一层）
- MCP 工具引用必须使用 `ServerName:tool_name` 全限定名
- 使用一个 Claude 实例编写 Skill，另一个实例测试

---

## 五、Managed Agents API（托管服务）

### Overview
Claude Managed Agents 将 Agent Loop、工具执行和上下文管理移入 Anthropic 服务端基础设施：

四个核心概念：
- **Agent**：可复用的版本化配置（model + system + tools + MCP + skills）
- **Environment**：云容器模板（packages, networking）
- **Session**：运行中的 Agent 实例，执行特定任务
- **Events**：应用与 Agent 之间的通信（用户事件 ↔ Agent 事件）

Rate Limits：创建类端点 60 RPM，读取类 600 RPM。

### Quickstart
完整 5 步流程：创建 Agent → 创建 Environment → 创建 Session → 发送用户事件 → SSE 流式接收

关键模式：在 SSE 流打开后再发送用户消息（API 会缓冲事件直到流连接）。`session.status_idle` 事件表示 Agent 完成当前工作。

### Onboarding（Console 可视化原型化）
在 Anthropic Console 中无需写代码即可构建和测试 Agent：
- 可视化配置所有 Agent 字段（模型、系统提示、MCP、工具、Skill）
- 实时显示等效 API 代码
- 内置 Session 运行器用于事件流测试
- Console 和 API 使用相同的 `/v1/agents`、`/v1/sessions` 资源

### Agent Setup
Agent 是版本化资源（version 从 1 起递增，每次更新递增）：

配置字段：`name`、`model`、`system`、`tools`、`mcp_servers`、`skills`、`callable_agents`、`description`、`metadata`

更新语义：
- 标量字段（model/system/name）直接替换
- 数组字段（tools/mcp_servers/skills）全量替换
- metadata 键级合并（null 值删除键）
- No-op 检测：无变化则不生成新版本

生命周期：创建 → 更新（版本递增）→ 归档（只读，现有 Session 继续）

### Environments
云容器模板配置：

**packages 字段**：预装包（pip/npm/apt/cargo/gem/go），跨 Session 缓存

**networking 字段**：
- `unrestricted`：完全网络访问（默认）
- `limited`：`allowed_hosts` + `allow_mcp_servers` + `allow_package_managers` 白名单控制

生命周期：多 Session 共享模板，每 Session 独立容器实例，不版本化。

### Sessions   ====》 思考 ing
Session 是状态机，由事件驱动：

| 状态 | 含义 |
|------|------|
| `idle` | 等待用户事件 |
| `running` | 正在处理 |
| `rescheduling` | 暂时不可用，自动重试 |
| `terminated` | 永久结束 |

- 可 pin 到 Agent 具体版本（`agent: {id: "...", version: N}`）用于灰度发布
- 删除运行中的 Session 需要先发送 interrupt 事件

### Tools（Managed Agents 内置工具）
8 种内置工具：bash、read、write、edit、glob、grep、web_fetch、web_search

配置模式：
- `default_config.enabled: false` + 按需 enable → 白名单模式
- 指定 disabled 工具 → 黑名单模式

自定义工具：`type: "custom"` + JSON schema，由客户端执行，通过 `agent.custom_tool_use` 事件触发。

### Events & Streaming  ====》 思考 ing
20+ 种事件类型，双向通信：

**用户发送**：`user.message`, `user.interrupt`, `user.custom_tool_result`, `user.tool_confirmation`, `user.define_outcome`

**Agent 发送**：`agent.message`, `agent.thinking`, `agent.tool_use`, `agent.tool_result`, `agent.custom_tool_use`

**Session 状态**：`session.status_idle`, `session.status_running`, `session.error`

`stop_reason: requires_action`：工具确认门控，`event_ids` 数组指定需要响应的工具调用 ID。

### Skills（Managed Agents）
Session 级别的按需加载技能：
- 最多 20 个 Skill/Session（含所有 Agent 的 Skill 总数）
- 按需加载，不常驻 context（Progressive Disclosure）
- Anthropic 预置：pptx、xlsx、docx、pdf

### MCP Connector（Managed Agents）
在 Agent 上配置 MCP server URL，在 Session 创建时注入认证（通过 Vault）：
- Agent 定义：MCP server URL 和名称
- Session 创建：vault_ids 提供认证凭证
- 默认权限策略：`always_ask`（不同于内置工具的 `always_allow`）
- 仅支持 HTTPS 可访问的远程 MCP server

### Permission Policies
两种权限策略：
- `always_allow`：内置工具集默认
- `always_ask`：MCP 工具集默认；会触发 `session.requires_action` 事件

配置粒度：工具集级别（`default_config.permission_policy`）→ 单工具级别（`configs[].permission_policy`）

### Vaults（凭证安全）
凭证与执行环境隔离的关键机制：

- 凭证**不进入容器环境变量**
- 支持类型：`mcp_oauth`（含自动 token 刷新）和 `static_bearer`
- 每个 vault 最多 20 个凭证；每个 MCP server URL 只能有 1 个活跃凭证（否则 409）
- Session 运行期间定期重新解析凭证，支持热更新（无需重启）
- 归档 vault 级联删除所有凭证并清除 secret

### Files（Managed Agents）
文件挂载到容器（只读副本）：

- `resources[]` 数组中通过 `file_id` 引用
- 可选 `mount_path` 指定容器内挂载位置
- Session 运行中支持动态增减（`resources.add`/`resources.delete`）
- 最多 100 个文件/Session；Session 范围的文件副本不计入存储限制
- 输出文件写入 `/mnt/session/outputs/`，通过 Files API + `scope_id` 取回

### GitHub
Git 仓库作为 Session Resource：

- `type: "github_repository"` 挂载仓库到容器
- 仓库在首次使用后缓存，后续 Session 启动更快
- `authorization_token` 只写（不回显）
- 配合 GitHub MCP server 获得完整读写/PR 能力
- 推荐细粒度 PAT（fine-grained personal access tokens）

### Memory（Managed Agents）
跨 Session 持久化记忆（Research Preview）：

- Memory Store：工作区范围的文本文档集合，自动被 Agent 检查和写入
- 通过 `session.resources[{type: "memory_store", ...}]` 挂载
- 每个 Session 最多 8 个 memory store
- 每条记忆最大 100KB（约 25k token）
- Agent 自动获得 6 个记忆工具：memory_list/search/read/write/edit/delete
- 版本化（immutable memory version）+ redact（合规清除）
- 乐观并发：`content_sha256` 前置条件防止并发写冲突

### Multi-Agent（Research Preview）
一个 Orchestrator Agent 协调多个 Sub-Agent：

- `callable_agents` 在 Agent 定义中声明可调用的其他 Agent
- 所有 Agent 共享同一容器和文件系统，但每个 Agent 运行在独立的 **Thread**（独立对话历史）
- Session 主流显示汇总视图；Thread 流显示单 Agent 详细视图
- 仅支持一层委托（Orchestrator → Sub-Agent，Sub-Agent 不能再调用 Agent）
- `session_thread_id` 字段用于路由工具确认回复到正确的 Thread

### Define Outcomes（Research Preview）  ====》 TODO
目标驱动的自动评估迭代：

工作流：
1. 定义 rubric（Markdown 格式，按标准列出）
2. 发送 `user.define_outcome` 事件（含 description、rubric、max_iterations）
3. Agent 开始工作，每轮工作后由独立 **Grader**（独立 context）评估
4. Grader 返回 per-criterion 细分反馈
5. 结果：`satisfied` / `needs_revision`（继续）/ `max_iterations_reached` / `failed` / `interrupted`

关键点：
- 默认 3 次迭代，最多 20 次（`max_iterations` 参数）
- Grader 使用独立 context，避免受主 Agent 实现选择影响
- 支持内联文本或 Files API 文件引用作为 rubric
- 输出文件在 `/mnt/session/outputs/`，通过 Files API + `scope_id` 取回
- 可以串联多个 Outcome（前一个结束后发送新的 define_outcome）

### Cloud Containers
容器规格：Ubuntu 22.04 LTS，x86_64，最多 8GB RAM，最多 10GB 磁盘

预装 8 种语言：Python 3.12+、Node.js 20+、Go 1.22+、Rust 1.77+、Java 21+、Ruby 3.3+、PHP 8.3+、C/C++ GCC 13+

注意：数据库 server（PostgreSQL、Redis）**默认不启动**，只有客户端工具。`docker` 可用但有限制。

### Observability
监控和调试工具：

- Console 追踪视图（Developer/Admin 权限）：Session 列表 → 时间线 → 工具执行细节
- `span.model_request_end`：含 model_usage（input/output token 计费数据）
- `session.error`：含 `error.retry_status` 指示是否自动重试
- 历史事件列表：完整分页历史，可用于离线调试
- 调试技巧：在系统提示中加入日志指令，让 Agent 输出推理过程

### Migration     ======》 TODO
从 Messages API 或 Claude Agent SDK 迁移到 Managed Agents：

核心变化：
- 手写的 while 循环（`stop_reason: "tool_use"`）→ 服务端托管循环（监听 `session.status_idle`）
- 客户端维护的 messages 数组 → 服务端维护的 Session 事件日志
- 自定义工具仍在客户端执行（通过 `agent.custom_tool_use` 事件）

从 Agent SDK 迁移额外注意：
- `max_turns` → 客户端侧计数实现
- `PreToolUse`/`PostToolUse` hooks → 客户端事件处理实现
- Plan mode → 客户端 `user.tool_confirmation` 实现

---

## 六、测试与评估（Test and Evaluate）

### Develop Tests（建立评测）
构建 LLM 评估管道的方法论：

1. 定义 **SMART 成功标准**（Specific, Measurable, Achievable, Relevant）
2. 设计任务特定的评测集（含边缘 case）
3. 选择打分方式：
   - 代码打分（精确匹配、余弦相似度、ROUGE-L）→ 优先
   - LLM 打分（详细 rubric + 鼓励推理后判断）→ 次选
   - 人工打分 → 最后选

用 Claude 自动生成更多测试用例（给定少量示例 → 批量生成变体）。

### Eval Tool（Console 评测工具）
无需写代码的可视化评测：
- 双括号变量 `{{variable}}` 语法
- AI 辅助生成测试用例
- 5 分制质量评分
- Prompt 版本并排对比

### Reduce Hallucinations
关键技术（按效果排序）：
1. 允许 Claude 说"我不知道"（允许不确定性）
2. 先引用原文再分析（>20k token 文档场景）
3. 每个断言附引用，无引用自动撤回
4. Best-of-N 一致性验证（N 次运行检查矛盾）
5. Chain-of-Thought 自我验证
6. 限制在提供文档范围内回答

### Reduce Latency
优先级顺序：
1. **模型选择**（Haiku 4.5 最快）
2. **Prompt 压缩**（减少不必要的系统提示）
3. **Streaming**（降低感知延迟）
4. **Prompt Caching**（减少 TTFT）
5. `max_tokens` 上限（避免过长输出，但可能截断）

定义：**TTFT**（首 token 延迟）vs **基线延迟**（完整响应时间）。

### Increase Consistency
标准化输出的方法：
- JSON/XML 输出规范（Structured Outputs）
- Few-shot 示例（用结构化标签包裹）
- RAG 固定上下文
- 系统提示 role 锚定
- **Prefill 已在 4.6 上废弃**，改用 Structured Outputs

### Mitigate Jailbreaks
分层防御策略：
1. **Haiku 4.5 预筛**：结构化输出布尔分类（速度快、成本低）
2. 已知越狱模式输入验证
3. 系统提示伦理边界强化
4. 输出监控 + 迭代 prompt 优化
5. 重复攻击用户节流/封禁

---

## 七、API 参考

### Rate Limits
速率限制体系：

**两种限制类型**：
- Spend Limits：每月消费上限（$5/$40/$200/$400 充值升级 Tier 1-4）
- Rate Limits：RPM/ITPM/OTPM（按模型类别分别限制）

**关键机制**：
- Token Bucket 算法（连续补充，非固定间隔重置）
- **Prompt Cache 命中 token 不计入 ITPM**（大多数模型）—— 提高有效吞吐量的关键
- 响应头 `anthropic-ratelimit-*` 实时暴露限制/剩余/重置信息
- 工作区级别子限制（Default Workspace 除外）

### Errors
完整 HTTP 错误码：

| 错误码 | 类型 | 处理方式 |
|--------|------|---------|
| 400 | invalid_request_error | 检查请求格式 |
| 401 | authentication_error | 检查 API key |
| 403 | permission_error | 检查权限 |
| 404 | not_found_error | 检查资源 ID |
| 429 | rate_limit_error | 指数退避 + retry-after 头 |
| 500 | api_error | 重试 |
| 529 | overloaded_error | 指数退避（区别于 429） |

流式响应中的错误可在 200 之后出现，需要单独的 SSE 错误处理。
最大请求大小：Messages 32MB，Batch 256MB，Files 500MB。

### Versioning
- `anthropic-version` header 必填（SDKs 自动设置）
- 当前版本：`2023-06-01`
- 向后兼容变更策略：可新增可选字段，不删除/重命名现有字段

### Beta Headers
- 语法：`anthropic-beta: feature-name-YYYY-MM-DD`
- 多功能：逗号分隔 `feature1,feature2`
- 无效 beta header 返回 400 `invalid_request_error`
- Managed Agents 使用统一 header 覆盖所有相关端点

### Client SDKs
官方支持 7 种语言：Python、TypeScript、Go、Java、C#、Ruby、PHP

Go SDK 关键特性：
- `anthropic.NewClient()` 从环境变量自动读取 API key
- `client.Beta.Agents.New(ctx, params)` 等类型安全接口
- 流式：`client.Beta.Sessions.Events.StreamEvents(ctx, id, params)`

---

## 八、MCP（Model Context Protocol）

### Agents and Tools - MCP Connector
通过 Messages API 直接连接远程 MCP Server，无需实现 MCP 客户端：

两组件配置：
1. `mcp_servers` 数组（顶层）：定义 server 连接信息（type, name, url, authorization_token）
2. `mcp_toolset`（tools 数组中）：启用特定 server 的工具

响应中新增：`mcp_tool_use` 和 `mcp_tool_result` block 类型

注意：
- 不支持 Amazon Bedrock 和 Google Vertex
- server 必须通过 HTTPS 公开可访问
- Beta header：`mcp-client-2025-11-20`（旧版 `2025-04-04` 已废弃）
- `defer_loading: true` 配合 Tool Search Tool 支持大型 MCP 工具集

---

## 九、与 ai-coding 项目的关联总结

### 最高优先级（Phase 1 必须考虑）

| 技术 | 影响的组件 | 关键决策 |
|------|-----------|---------|
| Adaptive Thinking + Effort | Brain | 使用 `thinking: {type: "adaptive"}` + `effort: "high"`，取代旧的 budget_tokens 配置 |
| Prompt Caching | Session + Brain | 系统提示 + 工具定义设置 cache_control，节省 ~90% 重复 token 费用 |
| Fine-Grained Tool Streaming | Tools + Harness | `eager_input_streaming: true` 大幅降低写文件等大输入工具的首字节延迟 |
| 429/529 错误重试 | Brain | 指数退避 + retry-after 头，429 和 529 分开处理 |
| Bash Tool 隔离 | Sandbox | bash 工具必须在 Docker/VM 隔离环境中执行 |

### 中优先级（Phase 2）

| 技术 | 影响的组件 | 关键决策 |
|------|-----------|---------|
| Compaction | Session | `compact_20260112` beta，触发阈值 150k token，防止大型 codebase 分析时超限 |
| Memory Tool | Session + Resources | `/memories` 目录实现跨 Session 项目状态持久化 |
| Advisor Tool | Brain | Sonnet 执行 + Opus 顾问，平衡成本与复杂任务质量 |

### 长期演进（Phase 3+）

| 技术 | 影响的组件 | 关键决策 |
|------|-----------|---------|
| Programmatic Tool Calling | Harness + Tools | 批量文件操作（重构、批量读写）N 轮压缩为 1 轮，大幅降低 token 消耗 |
| Tool Search Tool | ToolRegistry | 工具数量 >30 时引入 defer_loading + BM25/regex 按需发现 |
| Outcomes（Define Outcomes） | Harness | 在 Harness 循环中集成 Evaluator，支持目标驱动的自动迭代直到达标 |
| Multi-Agent | Orchestration | 多个 Harness 并行实例，各自独立 Session，共享同一 Sandbox |
| Credential Vault | gateway/auth | 凭证移出容器环境变量，通过 vault + proxy 机制注入 |
