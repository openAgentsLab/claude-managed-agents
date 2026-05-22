# Claude 官方文档页面摘要

---

## 入门

页面：https://platform.claude.com/docs/en/intro
总结：Claude 是 Anthropic 打造的高性能、可信赖的 AI 平台，擅长语言理解、推理分析、代码编写等任务。开发者可通过两种方式集成 Claude：直接调用 Messages API（适合自定义 Agent 循环）或使用 Claude Managed Agents（适合长时运行的异步任务）。页面还为新开发者提供了从首次 API 调用到探索高级功能的推荐学习路径。

页面：https://platform.claude.com/docs/en/get-started
总结：本页是 Claude API 的快速入门指南，介绍如何设置 API 密钥并通过 cURL、Python、TypeScript、Java 等多种方式发起第一次 API 调用。示例以构建一个简单的"网页搜索助手"为演示场景，展示完整的请求与响应结构。完成第一次调用后，页面引导开发者进一步了解 Messages API 的核心模式、模型选择及 SDK 文档。

---

## About Claude — 模型

页面：https://platform.claude.com/docs/en/about-claude/models/overview
总结：本页列出了 Claude 当前所有可用模型及其详细规格对比，最新一代主力模型包括：Claude Opus 4.7（最强推理与 Agentic 编程能力，128k 输出，1M 上下文）、Claude Sonnet 4.6（速度与智能的最佳平衡）和 Claude Haiku 4.5（速度最快、价格最低）。同时列出了仍可使用的旧版模型，并提示 Claude Sonnet 4 和 Claude Opus 4（首代 4.x）将于 2026 年 6 月 15 日退役，建议尽快迁移。

页面：https://platform.claude.com/docs/en/about-claude/models/whats-new-claude-4-6
总结：本页实际内容为 Claude Opus 4.7 的新功能说明，介绍了该模型相比 4.6 的重要升级：支持高分辨率图像（最高 2576px / 3.75MP）、新增 `xhigh` effort 等级、以及 Task Budgets（任务预算，beta）功能，使模型能在指定 token 配额内自主调控 Agentic 任务的深度。同时说明了若干破坏性变更，包括移除了 Extended Thinking Budget 参数、采样参数（temperature/top_p/top_k）不再支持自定义值，以及 token 计数因新 Tokenizer 的引入可能增加约 1～1.35 倍。

页面：https://platform.claude.com/docs/en/about-claude/models/choosing-a-model
总结：本页提供模型选择的决策框架，核心考量因素为能力、速度和成本三个维度。推荐两种策略：对成本敏感或原型阶段可从 Claude Haiku 4.5 起步，按需升级；对精度要求高的复杂任务则直接从 Claude Opus 4.7 开始，再逐步优化效率。页面还附有场景对应模型的选型矩阵，帮助开发者快速定位合适模型。

页面：https://platform.claude.com/docs/en/about-claude/models/migration-guide
总结：本页是从旧版 Claude 迁移至 Claude Opus 4.7 及 Claude 4.6 系列的完整操作指南，涵盖 API 参数变更（如废弃 Extended Thinking Budget、移除采样参数）、行为变化（如 Thinking 内容默认不返回、指令遵循更严格字面化）以及 Tokenizer 更新后的 token 用量变化等注意事项。对于使用 Claude Managed Agents 的用户，迁移仅需更新模型名称，无需修改 API 代码。

---

## About Claude — 定价与用例

页面：https://platform.claude.com/docs/en/about-claude/pricing
总结：该页面详细列出了 Anthropic 各 Claude 模型的定价，涵盖 Opus、Sonnet、Haiku 三个系列，按输入/输出 token 计费（每百万 token 单价从 $0.25 到 $15 不等）。此外还介绍了提示缓存（最低可降至标准价格的 10%）、批量处理（享 50% 折扣）、快速模式、工具使用、数据驻留等特殊功能的附加定价规则。企业用户可联系销售团队获取自定义价格和更高速率限制。

页面：https://platform.claude.com/docs/en/about-claude/use-case-guides/overview
总结：该页面是 Claude 常见应用场景的导航总览，列出了四个主要使用案例指南：工单路由、客户支持聊天机器人、内容审核和法律文件摘要。每个指南均提供生产级的最佳实践和实现示例，帮助开发者快速上手。

页面：https://platform.claude.com/docs/en/about-claude/use-case-guides/content-moderation
总结：该页面介绍如何使用 Claude 进行内容审核，说明了相较于传统 ML 方案的优势（如语义理解、多语言支持、策略灵活可调整等）。文档提供了详细的 Python 示例代码，包括基础违规检测、多级风险评估、批量处理等实现方式，并给出了在 10 亿条帖子/月场景下 Haiku 与 Opus 的成本估算对比。

页面：https://platform.claude.com/docs/en/about-claude/use-case-guides/customer-support-chat
总结：该页面提供了将 Claude 用作客户支持聊天机器人的完整实现指南，涵盖从确定使用场景、设计理想交互流程、定义评估标准，到编写系统提示、集成工具调用（如报价生成 API）和部署 Streamlit 界面的全流程。同时给出了优化性能的进阶建议，包括 RAG 降低延迟、流式响应提升体验及多语言支持等。

页面：https://platform.claude.com/docs/en/about-claude/use-case-guides/legal-summarization
总结：该页面介绍如何利用 Claude 对法律文件（如租约合同）进行智能摘要，重点说明了文本提取与清洗、结构化提示设计（使用 XML 标签分段输出）以及质量评估方法（ROUGE/BLEU 分数、嵌入相似度、人工审核）。对于超长文件，还提供了"元摘要"（分块摘要后再合并）及摘要索引文档等高级检索技术。

页面：https://platform.claude.com/docs/en/about-claude/use-case-guides/ticket-routing
总结：该页面介绍如何使用 Claude 自动分类和路由客服工单，详细说明了意图类别定义、分类提示编写（含少样本示例与 XML 标签解析）、评估指标（准确率、路由准确度、首次解决率等）及部署架构（推拉两种集成方式）。对于类别超过 20 个的复杂场景，还推荐使用分层分类器或基于向量数据库的相似度检索来提升性能。

页面：https://platform.claude.com/docs/en/about-claude/glossary
总结：该页面是 Claude 及大语言模型领域的术语词典，涵盖上下文窗口、微调、HHH（有用/诚实/无害）、延迟、LLM、MCP 协议、预训练、RAG、RLHF、温度参数、TTFT 和 Token 等核心概念。每个术语均附有简明解释及相关文档链接，适合初学者和开发者快速查阅。

页面：https://platform.claude.com/docs/en/about-claude/model-deprecations
总结：该页面说明 Anthropic 的模型生命周期管理策略，模型状态分为"活跃 → 遗留 → 弃用 → 停用"四个阶段，停用前至少提前 60 天通知用户。目前 `claude-opus-4-20250514` 和 `claude-sonnet-4-20250514` 已于 2026 年 4 月 14 日被标记为弃用，计划于 2026 年 6 月 15 日停用，建议迁移至 `claude-opus-4-7` 和 `claude-sonnet-4-6`。

---

## Build with Claude — 核心功能

页面：https://platform.claude.com/docs/en/build-with-claude/overview
总结：该页面是 Claude API 功能的总览，将 API 功能划分为五大类：模型能力、工具、工具基础设施、上下文管理以及文件与资产。页面列出了每项功能的可用状态（Beta、正式发布、已弃用或已下线），并提供各功能的详细文档链接。新用户建议从"模型能力"和"工具"两个板块入门。

页面：https://platform.claude.com/docs/en/build-with-claude/working-with-messages
总结：该页面介绍了 Messages API 的常见使用模式，包括基础请求与响应、多轮对话构建、预填充技巧（Prefill）以及视觉图像输入等。Messages API 是无状态的，每次请求需携带完整的对话历史；预填充功能可在最后一条消息中预设 Claude 的回答开头，但该功能在部分新模型上已不再支持。页面还介绍了如何通过 base64、URL 或 Files API 向 Claude 提交图像。

页面：https://platform.claude.com/docs/en/build-with-claude/streaming
总结：该页面介绍了如何通过设置 `"stream": true` 来启用流式响应，利用服务器推送事件（SSE）实时获取 Claude 的输出内容。Python 和 TypeScript SDK 提供了多种流式处理方式，可以在生成过程中实时接收 `text_delta` 等事件类型，适合需要低延迟展示效果的场景。

页面：https://platform.claude.com/docs/en/build-with-claude/context-windows
总结：该页面解释了"上下文窗口"的工作原理——它是模型生成响应时可引用的所有文本，最多可达 100 万 token（较新款模型），并随对话轮次线性增长。页面特别说明了启用"扩展思考"（Extended Thinking）时思考块的 token 计算规则：API 会自动将先前轮次的思考块从上下文中剥离，以节省 token 空间。对于长时间对话，推荐使用服务端"压缩（Compaction）"功能来管理上下文。

页面：https://platform.claude.com/docs/en/build-with-claude/token-counting
总结：Token 计数功能允许开发者在正式发送消息前，通过专用端点预估请求的 token 数量，有助于主动管理速率限制和成本。该功能支持系统提示、工具、图像和 PDF 等多种内容类型，返回的是估算值，实际使用量可能略有偏差。Token 计数本身免费，但有独立的每分钟请求速率限制，与正式消息创建的限额互相独立。

页面：https://platform.claude.com/docs/en/build-with-claude/adaptive-thinking
总结：自适应思考（Adaptive Thinking）是扩展思考功能的推荐使用方式，允许 Claude 根据请求复杂度自动决定是否使用以及使用多少思考 token，无需手动设置 `budget_tokens`。该模式在 Claude Opus 4.7 上是唯一支持的思考模式，并自动启用交叉思考（Interleaved Thinking），特别适合多步骤的 Agent 工作流。可配合 `effort` 参数控制思考深度，取代已弃用的 `budget_tokens` 参数。

页面：https://platform.claude.com/docs/en/build-with-claude/effort
总结：`effort` 参数允许开发者控制 Claude 在响应时的 token 消耗意愿，从 `low`（速度最快、成本最低）到 `max`（能力最强、消耗最多）共五个级别，影响范围涵盖文本输出、工具调用以及扩展思考。该参数是对 `budget_tokens` 的替代方案，适用于 Claude Opus 4.7、4.6 及 Sonnet 4.6 等模型，默认值为 `high`。在 Agent 场景中，较低的 `effort` 级别会让 Claude 减少工具调用次数，从而在性能与成本之间取得平衡。

页面：https://platform.claude.com/docs/en/build-with-claude/extended-thinking
总结：扩展思考功能让 Claude 在生成最终回答前，通过 `thinking` 内容块展示逐步推理过程，适用于复杂任务。Claude 4 模型默认返回思考内容的摘要版本（仍按完整思考 token 计费），也可设置 `display: "omitted"` 来省略思考内容以降低延迟。使用工具时，必须将思考块原封不动地随工具结果一起回传，以保证推理的连续性。

页面：https://platform.claude.com/docs/en/build-with-claude/fast-mode
总结：Fast 模式（目前处于研究预览 Beta 阶段）为 Claude Opus 4.6 提供最高 2.5 倍的输出 token 速率提升，通过在请求中设置 `speed: "fast"` 并附加指定 Beta 头部来启用，适合对延迟敏感的 Agent 工作流。该模式不改变模型权重或能力，但定价为标准 Opus 费率的 6 倍，并拥有独立的速率限制配额。需注意，Fast 模式与标准模式之间切换会导致提示词缓存失效，且不支持 Batch API。

页面：https://platform.claude.com/docs/en/build-with-claude/prompt-caching
总结：Prompt Caching 通过缓存和复用请求前缀来降低处理延迟和成本，缓存命中的读取费用仅为基础输入 token 价格的 10%，但首次写入缓存需额外支付 25% 的费用，默认缓存有效期为 5 分钟（可升级为 1 小时）。该功能支持两种方式：自动缓存（适合多轮对话）和手动设置 `cache_control` 断点（适合需要精细控制的场景）。使用时需注意，可缓存内容有最低 token 数量限制（1,024 至 4,096 token 不等），且缓存断点应置于不常变化的内容之后，否则将无法命中缓存。

页面：https://platform.claude.com/docs/en/build-with-claude/compaction
总结：上下文压缩（Compaction）是一项服务器端功能，用于管理长对话中接近上下文窗口上限的情况，通过自动将旧的对话历史摘要化来延长有效上下文长度。它不仅能防止超出 token 上限，还能保持模型对当前任务的专注度，避免因冗长历史导致的性能下降。该功能目前处于 beta 阶段，需在请求中加入 `compact-2026-01-12` 头部，适用于长期多轮对话和涉及大量工具调用的 Agent 工作流。

页面：https://platform.claude.com/docs/en/build-with-claude/context-editing
总结：上下文编辑（Context Editing）允许开发者在运行时对对话历史进行精细化管理，选择性地清除不再需要的内容（如旧的工具结果或思考块）。与服务器端压缩相比，该功能适用于需要对上下文内容进行更细粒度控制的场景，例如 Agent 工作流中清理重型工具调用结果，或在扩展思考模式下管理思考块。核心策略包括工具结果清除、思考块清除和基于 SDK 的客户端压缩。

页面：https://platform.claude.com/docs/en/build-with-claude/handling-stop-reasons
总结：该页面详细介绍了 Messages API 响应中 `stop_reason` 字段的各种取值及其处理方式，包括 `end_turn`（正常结束）、`max_tokens`（达到 token 限制）、`tool_use`（需要执行工具）、`pause_turn`（服务器工具循环达到上限）、`refusal`（安全拒绝）等。开发者应始终检查该字段以构建健壮的应用逻辑，例如在 `max_tokens` 时继续生成、在 `tool_use` 时执行工具并返回结果、在 `pause_turn` 时继续对话。`stop_reason` 属于成功响应的一部分，与表示请求失败的 HTTP 错误码（4xx/5xx）有本质区别。

页面：https://platform.claude.com/docs/en/build-with-claude/structured-outputs
总结：结构化输出功能通过强制 Claude 的回复符合指定的 JSON Schema，确保输出可被程序直接解析，消除格式错误的风险。该功能包含两个互补机制：JSON 输出（保证响应符合 Schema）和严格工具调用（验证工具参数合规性），并支持 Python（Pydantic）、TypeScript（Zod）等多语言原生类型定义。需注意其与引用（Citations）功能不兼容，且对递归 Schema、外部 `$ref` 等有一定限制。

页面：https://platform.claude.com/docs/en/build-with-claude/vision
总结：Claude 的视觉能力支持对图像进行理解和分析，可通过 claude.ai 界面、控制台或 API 请求传入图片（支持 base64 编码或 URL 引用）。单次请求最多支持 100 张图片（API），单张图片最大尺寸为 8000×8000 像素；对于大量图片的场景，建议通过 Files API 上传后以 `file_id` 引用，以减少请求体积。该功能适用于图像内容分析、视觉问答、多图对比等多模态交互场景。

页面：https://platform.claude.com/docs/en/build-with-claude/pdf-support
总结：Claude 支持处理标准 PDF 文件，可对文档中的文字、图表、表格等内容进行分析，适用于财务报告分析、法律文档提取、文档格式转换等场景。PDF 可通过 URL 引用、base64 编码或 Files API 的 `file_id` 三种方式传入，每次请求最多支持 600 页（200k token 上下文窗口的模型为 100 页）。系统会将每页转换为图像并同时提取文字，结合视觉能力实现对非文本内容（如图表）的深度理解。

页面：https://platform.claude.com/docs/en/build-with-claude/files
总结：Files API 允许开发者将文件预先上传至 Anthropic 的安全存储，获得唯一 `file_id` 后在后续 API 请求中直接引用，避免每次请求重复传输相同内容。支持的文件类型包括 PDF、纯文本、图片及代码执行工具所用的数据集等，单文件最大 500 MB，组织总存储上限 500 GB；文件操作（上传、下载、列出、删除）本身免费，但在 Messages 请求中使用文件内容按输入 token 计费。该功能目前处于 beta 阶段，需在请求头中加入 `anthropic-beta: files-api-2025-04-14`，暂不支持 Amazon Bedrock 和 Google Vertex AI。

页面：https://platform.claude.com/docs/en/build-with-claude/citations
总结：引用（Citations）功能使 Claude 在回答文档相关问题时能够提供精确的来源引用，支持纯文本（字符索引）、PDF（页码）和自定义内容（块索引）三种文档类型。启用后，响应中的文本会被拆分为多个文本块，每个涉及引用的块都会附带对应文档的精确位置信息；`cited_text` 字段不计入输出 token，因此相比纯提示词方案具有成本优势和更高的引用可靠性。需注意，引用功能与结构化输出（Structured Outputs）不兼容，且目前仅支持文本引用，尚不支持图片引用。

页面：https://platform.claude.com/docs/en/build-with-claude/search-results
总结：搜索结果内容块（Search Result Content Blocks）专为 RAG（检索增强生成）应用设计，让 Claude 能够以类似网络搜索的质量对自定义数据源进行带来源标注的引用。每个搜索结果块包含来源（source）和标题（title）信息，可在工具返回或顶级内容中灵活使用，无需再通过文档块变通处理。该功能消除了 RAG 场景中的来源归因难题，提供与 Claude 内置网络搜索一致的引用格式和质量。

页面：https://platform.claude.com/docs/en/build-with-claude/embeddings
总结：本页介绍了文本嵌入（Embeddings）的概念及其在语义搜索、推荐系统和异常检测中的应用。Anthropic 本身不提供嵌入模型，而是推荐使用合作伙伴 Voyage AI 的嵌入服务，其最新一代模型（如 voyage-4-large、voyage-4、voyage-4-lite）支持最长 32,000 token 的上下文和多种向量维度。页面还提供了使用 Python SDK 和 HTTP API 调用 Voyage 嵌入的完整示例代码。

页面：https://platform.claude.com/docs/en/build-with-claude/batch-processing
总结：本页介绍了 Anthropic 的 Message Batches API，这是一种用于异步批量处理大量请求的高效方案，相比实时调用可降低 50% 的成本。批量任务通常在 1 小时内完成，适用于大规模评估、内容审核和数据分析等无需即时响应的场景。用户可以提交批次请求后轮询状态，并在所有请求处理完成后统一获取结果。

页面：https://platform.claude.com/docs/en/build-with-claude/multilingual-support
总结：本页展示了 Claude 系列模型在多语言任务上的零样本推理能力，以英文为基线（100%），列出了西班牙语、中文、日语、阿拉伯语等 14 种语言的相对性能分数——大多数主流语言的得分都在 93% 以上。最新的 Claude Opus 4.1 和 Sonnet 4.5 模型在所有测试语言中均表现优异，但低资源语言（如约鲁巴语）的性能相对较低。页面还给出了多语言提示的最佳实践，包括明确指定语言、使用原生文字而非音译，以及考虑文化背景。

页面：https://platform.claude.com/docs/en/build-with-claude/skills-guide
总结：本页介绍了 Agent Skills API 的使用方式，Skills 是一种通过有组织的指令、脚本和资源文件夹来扩展 Claude 能力的机制，通过代码执行工具与 Messages API 集成。Skills 分为两类：Anthropic 预置技能（如处理 PDF、Excel、PPT、Word 文件）和用户自定义上传的技能，均可在 API 调用的 `container` 参数中指定。页面还详细说明了文件下载、多轮对话、长时间任务暂停（pause_turn）以及自定义 Skills 的增删改查操作。

---

## Build with Claude — 提示词工程

页面：https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/overview
总结：本页是提示词工程（Prompt Engineering）的概述入口，指引用户在已有明确成功标准和初步提示草稿的前提下进行优化。所有具体技巧（包括清晰度、示例、XML 结构、角色设定、思维链和提示链等）均集中在《提示词最佳实践》文章中，本页作为导航页指向该文章及 Console 中的提示工具。页面还提供了 GitHub 和 Google Sheets 两种交互式教程供动手学习。

页面：https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices
总结：本页是针对 Claude 最新模型（包括 Opus 4.7、Sonnet 4.6 等）的提示词工程全面参考指南，涵盖基础技巧、输出控制、工具使用、思维链（thinking）以及 Agent 系统等多个方面。页面特别针对 Claude Opus 4.7 的新特性（如自适应响应长度、增强的长文档处理和 interleaved thinking）给出了具体的提示调优建议。这是 Anthropic 官方推荐的单一权威参考文档，供开发者持续查阅。

页面：https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-tools
总结：本页介绍了 Claude Console 中三款提示词辅助工具：提示生成器（Prompt Generator，解决"空白页问题"，快速生成结构化初稿）、提示模板与变量（Prompt Templates & Variables，将固定内容与动态内容分离，便于测试和版本管理），以及提示改进器（Prompt Improver，通过四步流程自动分析并增强提示词，添加思维链指令和 XML 结构化组织）。这些工具相互协作，帮助开发者从零开始构建并持续优化高质量的提示词模板。

---

## Build with Claude — 管理与云平台

页面：https://platform.claude.com/docs/en/build-with-claude/administration-api
总结：Admin API 允许开发者以编程方式管理组织资源，包括成员、工作区和 API 密钥，需使用以 `sk-ant-admin` 开头的专用管理员密钥。该 API 支持自动化用户入职/离职、工作区访问管理及 API 密钥监控等操作。组织共有五种角色（user、claude_code_user、developer、billing、admin），仅 admin 角色可使用此 API。

页面：https://platform.claude.com/docs/en/build-with-claude/workspaces
总结：工作区（Workspace）是组织内对 API 使用进行分组管理的单元，可用于隔离不同项目、环境或团队，同时共享统一的账单和管理。每个组织最多可创建 100 个工作区，可通过控制台或 Admin API 管理成员、设置费用/速率限制，并在工作区维度跟踪用量和成本。API 密钥、文件、批量任务等资源均与特定工作区绑定，归档工作区将立即吊销其下所有 API 密钥。

页面：https://platform.claude.com/docs/en/build-with-claude/usage-cost-api
总结：用量与费用 Admin API 提供对组织历史 API 用量及成本数据的程序化访问，支持按模型、工作区、服务层级等维度进行过滤与分组。用量 API 以分钟/小时/天为粒度返回 Token 消耗数据，费用 API 则以美元返回每日成本明细，两者均支持分页。数据通常在请求完成后约 5 分钟内可查，可与 CloudZero、Datadog、Grafana 等观测平台集成。

页面：https://platform.claude.com/docs/en/build-with-claude/claude-code-analytics-api
总结：Claude Code Analytics API 提供按天聚合的 Claude Code 使用指标，帮助组织分析开发者生产力。每条记录包含单个用户当日的会话数、新增/删除代码行数、提交数、PR 数，以及各工具（Edit/Write/NotebookEdit）的接受率和模型维度的 Token/费用明细。该 API 仅支持 Anthropic 原生 API 上的 Claude Code 使用数据，不包含 Bedrock 或 Vertex AI 的使用情况。

页面：https://platform.claude.com/docs/en/build-with-claude/api-and-data-retention
总结：该页面介绍 Anthropic 的两种数据处理安排：零数据留存（ZDR）和 HIPAA 合规访问，前者在 API 响应返回后不在服务器持久化客户数据，后者面向需处理受保护健康信息（PHI）的组织并需签署 BAA 协议。ZDR 适用于 Claude Messages API、Token Counting API 及特定 Claude Code 使用场景，但不适用于控制台/Workbench、批处理 API、Files API 等有状态功能。页面还提供了各 API 功能的 ZDR 和 HIPAA 资格对照表，帮助开发者评估合规性。

页面：https://platform.claude.com/docs/en/build-with-claude/data-residency
总结：数据驻留功能通过两个独立设置控制数据处理位置：`inference_geo` 参数（逐请求控制推理运行地理位置，支持 `global` 和 `us`）以及工作区层级的 Workspace Geo（控制数据静态存储位置）。使用 `inference_geo: "us"` 将推理限定在美国基础设施时，对 Claude Opus 4.6 及更新模型收取 1.1 倍价格溢价，全球路由则按标准定价。该功能目前仅支持 Anthropic 原生 API，不适用于 AWS Bedrock 或 Google Vertex AI。

页面：https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock
总结：该页面介绍通过 Amazon Bedrock 的传统集成方式（`InvokeModel` 和 `Converse` API，使用 ARN 版本化模型 ID）访问 Claude 模型的方法，支持 Python、TypeScript、Go、Java 等多语言 SDK。从 Claude Sonnet 4.5 起，Bedrock 提供全球端点（动态路由，无价格溢价）和区域端点（CRIS，需加 10% 溢价）两种选择，以满足不同的数据驻留需求。此为传统集成文档；对于使用 Messages API 格式（`/anthropic/v1/messages`）的新版集成，应参考 "Claude in Amazon Bedrock" 页面。

页面：https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock-research-preview
总结：Claude in Amazon Bedrock（新版）通过 AWS 托管基础设施提供 Messages API（`/anthropic/v1/messages`），Anthropic 人员无法访问推理基础设施，适合对安全性要求高的应用场景。该集成支持三种身份验证方式：Bedrock 服务角色（推荐）、IAM 假设角色和 Bearer Token，可在全球超过 20 个 AWS 区域使用，默认配额为每分钟 200 万输入 Token。目前开放访问的模型为 Claude Opus 4.7 和 Claude Haiku 4.5，支持提示词缓存、扩展思维、工具调用等功能，但不支持 Anthropic 托管工具（如网络搜索、Files API 等）。

页面：https://platform.claude.com/docs/en/build-with-claude/claude-on-vertex-ai
总结：Claude 模型已在 Google Cloud Vertex AI 上正式开放，调用方式与 Anthropic 原生 Messages API 基本一致，主要区别在于模型 ID 通过 GCP 端点 URL 传递，且需在请求体中传入 `anthropic_version: "vertex-2023-10-16"`。Vertex AI 提供全球、多区域（`us`/`eu`）和单区域三种端点类型，区域/多区域端点在 Sonnet 4.5 及更新模型上附加 10% 价格溢价。Claude Opus 4.7、Opus 4.6、Sonnet 4.6 在 Vertex AI 上支持 100 万 Token 上下文窗口。

页面：https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry
总结：Claude in Microsoft Foundry 通过 Azure 原生端点（`https://{resource}.services.ai.azure.com/anthropic/v1/*`）提供访问，使用 Azure 订阅计费，支持 API 密钥和 Entra ID（Azure AD）两种身份验证方式。集成采用"资源→部署"两层结构，支持 Python、TypeScript、C#、Java、PHP 等 SDK，但不支持 Go 和 Ruby。与 Bedrock/Vertex 类似，该集成不支持 Admin API、Models API 和 Message Batch API，Claude 运行于 Anthropic 基础设施之上，Anthropic 的数据承诺（包括 ZDR 可用性）同样适用。

---

## Agents and Tools — 工具使用

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview
总结：工具使用允许 Claude 调用你定义的函数或 Anthropic 提供的函数，Claude 根据用户请求和工具描述决定何时调用工具。工具分为两类：客户端工具（由你的应用负责执行，Claude 返回 `tool_use` 块后由你执行并返回结果）和服务端工具（由 Anthropic 基础设施执行，如 web_search、code_execution 等）。工具使用的定价基于输入输出 token 数量，服务端工具还可能有额外的按使用量计费。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/how-tool-use-works
总结：本页解释了工具使用的核心机制：Claude 不会直接执行任何操作，而是发出结构化请求，由你的代码或 Anthropic 服务器来运行，结果再回流入对话。工具主要分三类：用户自定义的客户端工具、Anthropic 定义 schema 的客户端工具（如 bash、text_editor）、以及服务端执行工具（如 web_search、code_execution）。客户端工具需要应用层实现"代理循环"，不断处理 `tool_use` 响应并返回结果，直到 Claude 完成任务。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-reference
总结：本页是 Anthropic 提供的所有工具的目录参考，涵盖服务端工具（web_search、web_fetch、code_execution、advisor、tool_search、mcp_connector）和客户端工具（memory、bash、text_editor、computer）。每个工具都有带日期版本号的 `type` 字段以区分版本，新版本在行为或模型支持变化时发布，旧版本继续可用以保持兼容性。工具定义还支持多种可选属性，如 `cache_control`、`strict`、`defer_loading`、`allowed_callers` 等，用于控制缓存、验证、加载时机和调用权限。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/bash-tool
总结：Bash 工具让 Claude 能在持久 bash 会话中执行 shell 命令，支持系统操作、脚本执行和命令行自动化，会话状态（环境变量、工作目录）在多次命令之间保持。应用层负责实际执行命令并将结果返回给 Claude，建议实现命令白名单过滤、超时机制、沙箱隔离等安全措施。该工具不支持交互式命令（如 vim），每次调用额外消耗约 245 个输入 token。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/text-editor-tool
总结：文本编辑器工具（`str_replace_based_edit_tool`）允许 Claude 查看和修改文本文件，适用于代码调试、重构、文档生成等场景。工具支持四种命令：`view`（查看文件或目录）、`str_replace`（精确替换文本）、`create`（创建新文件）、`insert`（在指定行插入文本）。应用层负责实际执行文件操作，需要注意安全校验（防路径穿越）、备份和错误处理，每次调用额外消耗约 700 个输入 token。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/code-execution-tool
总结：代码执行工具让 Claude 能在安全沙箱容器中运行 Python 和 Bash 代码，实现数据分析、文件生成、系统命令等功能，是构建高性能 Agent 的核心原语。该工具在与 web_search 或 web_fetch 组合使用时免费，单独使用时按额外费率计费；支持动态过滤功能（`code_execution_20260120` 版本），可在数据进入上下文前先用代码过滤，提升精度并降低 token 消耗。工具不支持零数据保留（ZDR），数据按标准保留策略存储。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool
总结：计算机使用工具（Beta）让 Claude 能通过截图、鼠标控制和键盘输入与桌面环境交互，实现跨应用的自动化操作。应用层负责实际执行截图捕获、鼠标移动和键盘输入等操作，需要在虚拟机或容器中运行以保证安全隔离，避免提示注入攻击。该工具存在延迟较高、坐标精度有限等局限性，且需要特定 Beta 请求头，每次调用额外消耗约 466–499 个系统 prompt token 和 735 个工具定义 token。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-search-tool
总结：Web 搜索工具（服务端执行）让 Claude 能访问实时网页内容，超越知识截止日期，响应中包含来源引用。最新版本（`web_search_20260209`）支持动态过滤，Claude 可编写并执行代码在结果进入上下文前过滤，提升准确性并降低 token 消耗（需同时启用代码执行工具）。定价为每 1000 次搜索 $10，加上标准 token 费用，支持域名过滤、地理位置本地化和最大搜索次数限制等参数。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/web-fetch-tool
总结：Web Fetch 工具（服务端执行）让 Claude 能获取指定 URL 的完整内容，支持网页和 PDF 文档，无额外费用（只收标准 token 费用）。最新版本（`web_fetch_20260209`）同样支持动态过滤，可在加载前用代码提取相关内容，减少 token 消耗。出于安全考虑，该工具只能获取在对话上下文中已出现的 URL，Claude 不能随意构造新 URL 进行请求，建议在处理不受信任输入时限制使用。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/memory-tool
总结：Memory 工具让 Claude 能通过文件系统在多次对话之间存储和检索信息，实现跨会话的知识积累。这是一个客户端工具，应用层控制数据存储位置，需要实现 `view`、`create`、`str_replace`、`insert`、`delete`、`rename` 等命令的处理逻辑，并限制所有操作在 `/memories` 目录内防止路径穿越攻击。工具启用后，Claude 会在每次任务开始前自动查看记忆目录，适合长时运行的 Agent 工作流跨会话保持进度。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/fine-grained-tool-streaming
总结：细粒度工具流式传输功能通过在工具定义上设置 `eager_input_streaming: true`，让工具输入参数无需缓冲或 JSON 验证即可逐字符流式传输，大幅降低接收大参数的延迟。由于跳过了 JSON 验证，可能会收到不完整或无效的 JSON（尤其是遇到 `max_tokens` 中断时），需要应用层特别处理。使用时需累积所有 `input_json_delta` 事件的 `partial_json` 字段，在 `content_block_stop` 后再进行完整解析。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/programmatic-tool-calling
总结：程序化工具调用让 Claude 在代码执行容器内用代码批量调用工具，无需每次调用都往返模型，显著降低延迟和 token 消耗。通过在工具定义中设置 `"allowed_callers": ["code_execution_20260120"]`，Claude 可以在 Python 代码中直接 `await` 调用工具，中间结果不进入 Claude 上下文窗口，只有最终代码输出才反馈给模型。这种模式特别适合大数据集处理、多步循环工作流和条件分支逻辑等场景。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool
总结：工具搜索工具通过按需动态加载，让 Claude 能高效管理数百乃至数千个工具，避免上下文膨胀和选择准确率下降。通过在工具定义上设置 `defer_loading: true`，工具不会预加载到上下文中，Claude 使用 Regex 或 BM25 搜索后按需加载，通常可将工具 token 消耗减少 85% 以上。每次搜索返回 3–5 个最相关的工具引用，系统自动将其展开为完整定义，全程对应用层透明，同时与提示缓存（prompt caching）完全兼容。

页面：https://platform.claude.com/docs/en/agents-and-tools/tool-use/advisor-tool
总结：Advisor 工具（Beta）允许将更快、低成本的执行模型与更高智能的顾问模型配对，执行模型在生成过程中可随时咨询顾问模型获取策略指导，全程在单次 API 请求内完成，无需额外往返。顾问模型读取完整对话历史后输出计划或建议（约 400–700 文本 token），执行模型据此继续生成，从而在接近顾问模型质量的同时，大部分 token 生成以执行模型的较低费率计费。该工具特别适合长时 Agent 工作流，如编程 Agent、计算机使用和多步研究流程。

---

## Agents and Tools — Agent Skills & MCP

页面：https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview
总结：Agent Skills 是一种模块化能力扩展机制，允许开发者将特定领域的专业知识（工作流程、指令、代码脚本等）打包成可复用的 Skill 供 Claude 调用。Skill 采用"渐进式加载"架构，分三级加载内容（元数据、指令、资源/代码），仅在需要时才消耗上下文窗口，从而高效管理 token 使用。目前提供 PowerPoint、Excel、Word、PDF 四种官方预置 Skill，用户也可自定义 Skill 并在 API、Claude.ai 或 Claude Code 中使用。

页面：https://platform.claude.com/docs/en/agents-and-tools/agent-skills/quickstart
总结：本页为 Agent Skills API 快速入门教程，以 10 分钟内创建一个 PowerPoint 演示文稿为示例，介绍如何在 Messages API 请求中通过 `container.skills` 参数启用预置 Skill（如 `pptx`、`xlsx`、`docx`、`pdf`），并需同时提供 `code-execution` 和 `skills` 两个 beta 头。Claude 生成的文件保存在代码执行容器中，需通过 Files API 下载获取。

页面：https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices
总结：本页提供编写高质量 Skill 的最佳实践，核心原则是"简洁第一"——避免向 Claude 解释它本已知晓的内容，并根据任务的风险程度设定适当的指令自由度（从精确脚本到开放式指南）。推荐采用"评估驱动开发"方式，先创建测试用例再编写指令，同时利用 Claude 本身来协助迭代优化 Skill 内容。还包括命名规范、描述字段写法、反模式（如嵌套引用过深、提供太多选项等）的详细指导。

页面：https://platform.claude.com/docs/en/agents-and-tools/agent-skills/enterprise
总结：本页面面向企业管理员，介绍如何在组织规模下安全部署和治理 Agent Skills。内容涵盖 Skill 的安全审查流程（风险评级表和审查清单，重点关注代码执行、网络访问、凭证硬编码等风险点）、质量评估维度（触发准确性、隔离行为、共存性等），以及 Skill 全生命周期管理（计划、创建审查、测试、部署、监控、迭代或弃用）。此外还提供了基于角色的 Skill 分组策略和多平台版本控制建议。

页面：https://platform.claude.com/docs/en/agents-and-tools/mcp-connector
总结：MCP Connector 允许开发者在 Messages API 中直接连接远程 MCP 服务器，无需自行实现 MCP 客户端，当前使用 beta 头 `mcp-client-2025-11-20`。配置分为两部分：`mcp_servers` 数组定义服务器连接信息（URL、认证 token），`tools` 数组中的 `mcp_toolset` 对象控制哪些工具可用（支持白名单、黑名单及按工具细粒度配置）。仅支持通过 HTTP 公开暴露的服务器（不支持本地 STDIO），且目前仅支持工具调用，不支持 MCP 的 Prompt 和 Resource 功能。

页面：https://platform.claude.com/docs/en/agents-and-tools/remote-mcp-servers
总结：本页列举了多家已部署并可通过 Anthropic MCP Connector API 连接的第三方远程 MCP 服务器，提供了连接前的注意事项：这些服务器均为第三方服务，Anthropic 不对其安全性背书，用户需自行审查服务条款和安全实践。连接前需准备好认证凭据，并参照各服务商的具体文档进行配置；更多 MCP 服务器可在 GitHub 上的 MCP Servers 仓库找到。

---

## Managed Agents

页面：https://platform.claude.com/docs/en/managed-agents/overview
总结：Claude Managed Agents 是 Anthropic 提供的托管式 Agent 运行框架，相比直接调用 Messages API，它为长时间运行、异步工作负载提供了预构建的 Agent 循环、工具执行和沙盒容器基础设施。核心概念包括四个层次：Agent（模型+系统提示+工具配置）、Environment（容器模板）、Session（运行实例）和 Events（消息流）；Claude 可在容器内执行 Bash 命令、读写文件、搜索网页等操作，并通过 SSE 流实时返回结果。目前处于 Beta 阶段，所有请求需携带 `managed-agents-2026-04-01` 头。

页面：https://platform.claude.com/docs/en/managed-agents/onboarding
总结：本页介绍如何通过 Console 可视化界面原型化和测试 Managed Agent，用户可在界面中配置模型、系统提示、MCP 服务器、工具和 Skills，而无需编写代码。Console 内置了会话测试运行器，可实时查看事件流，满意后直接复制生成的 API 请求代码或 Agent ID 用于生产代码。

页面：https://platform.claude.com/docs/en/managed-agents/quickstart
总结：本页是 Claude Managed Agents 的代码快速入门指南，分四步演示如何创建 Agent、创建 Environment（云端容器）、启动 Session，以及通过 SSE 事件流发送消息并接收响应。示例任务让 Agent 编写并执行 Python 脚本生成 Fibonacci 数列，展示了 Agent 自主选择工具、执行命令、输出 `session.status_idle` 事件结束流的完整流程，支持 Python、TypeScript、Go、Java、C#、Ruby、PHP 多语言示例。

页面：https://platform.claude.com/docs/en/managed-agents/agent-setup
总结：本页详细介绍如何创建和管理可复用、带版本的 Agent 配置资源，包括各字段说明（`name`、`model`、`system`、`tools`、`mcp_servers`、`skills` 等）及创建、更新 Agent 的代码示例。更新时只需传递变更字段，数组字段为全量替换，元数据字段按键合并；每次有效更新都会生成新版本号，版本历史可通过 List Versions 接口查询。还提供了 Archive 操作使 Agent 变为只读状态，现有 Session 不受影响。

页面：https://platform.claude.com/docs/en/managed-agents/sessions
总结：Session（会话）是在指定环境中运行的 Agent 实例，创建时需要提供 Agent ID 和 Environment ID，并可通过版本号固定到特定 Agent 版本。会话本身是状态机，创建后不会自动执行任务，需通过发送用户事件（如 `user.message`）来驱动执行。会话状态包括 `idle`、`running`、`rescheduling` 和 `terminated`，支持归档、删除等生命周期操作。

页面：https://platform.claude.com/docs/en/managed-agents/events-and-streaming
总结：Managed Agents 采用事件驱动模型，分为用户发送的 User Events（如 `user.message`、`user.interrupt`、`user.tool_confirmation`）和系统返回的 Agent/Session/Span Events。响应支持实时流式传输（SSE），客户端可以订阅 session ID 的事件流，逐步接收 Agent 的消息、工具调用及状态变更。还支持在 Agent 执行中途通过 interrupt 事件中断，或通过 tool_confirmation 审批/拒绝工具调用。

页面：https://platform.claude.com/docs/en/managed-agents/tools
总结：Managed Agents 提供一套内置工具集（`agent_toolset_20260401`），包含 Bash、文件读写、编辑、Glob、Grep、Web Fetch 和 Web Search 等 8 种工具，默认全部启用，可通过配置按需禁用或选择性开启。此外还支持自定义工具（`type: custom`），开发者定义工具契约，Claude 决定何时调用，实际执行由应用代码完成并将结果回传。建议为自定义工具提供详尽描述，并将相关操作合并为少量多功能工具以提升准确性。

页面：https://platform.claude.com/docs/en/managed-agents/environments
总结：Environment（环境）定义了 Agent 运行所在容器的配置，创建一次后可被多个 Session 共享，但每个 Session 拥有独立隔离的容器实例，不共享文件系统状态。环境支持预安装 pip、npm、apt 等多种包管理器的依赖，以及配置网络策略（`unrestricted` 全开放或 `limited` 白名单限制）。环境持久存在直到被归档或删除，建议生产环境使用 `limited` 网络模式遵循最小权限原则。

页面：https://platform.claude.com/docs/en/managed-agents/cloud-containers
总结：云容器预装了多种主流编程语言（Python 3.12+、Node.js 20+、Go、Rust、Java、Ruby、PHP、C/C++）及其包管理工具，无需额外安装即可直接使用。内置 SQLite 及 PostgreSQL/Redis 客户端工具，另有 git、curl、jq、ripgrep 等常用系统和开发工具。容器基于 Ubuntu 22.04 LTS（x86_64），提供最高 8GB 内存、10GB 磁盘，网络默认关闭，需在环境配置中显式启用。

页面：https://platform.claude.com/docs/en/managed-agents/files
总结：可通过 Files API 上传文件，并在创建 Session 时将其挂载到容器指定路径，Agent 即可直接读取这些文件。每个 Session 最多支持挂载 100 个文件，挂载的文件为只读副本，Agent 的修改需写入容器内的新路径。还支持通过 Session 资源 API 在运行中的 Session 动态添加或删除文件，并可通过 Files API 按 Session 范围列出和下载 Agent 生成的输出文件。

页面：https://platform.claude.com/docs/en/managed-agents/vaults
总结：Vault（密钥库）是每个用户的凭证集合，用于存储访问第三方 MCP 服务器所需的 OAuth 令牌或静态 Bearer Token，由 Anthropic 自动管理令牌刷新，无需开发者自建密钥存储。创建 Session 时传入 `vault_ids` 即可引用对应用户的凭证，实现 Agent 配置（workspace 级）与用户认证（session 级）的解耦。每个 Vault 最多支持 20 个凭证，每个 MCP 服务器 URL 只能绑定一个有效凭证，支持归档、轮换和 OAuth 刷新失败诊断等完整生命周期管理。

页面：https://platform.claude.com/docs/en/managed-agents/memory
总结：Memory Store（记忆存储）是跨 Session 持久化信息的机制，每个存储是一个文本文档集合，在 Session 中以目录形式挂载到容器 `/mnt/memory/` 路径，Agent 用普通文件工具读写。每个 Session 最多挂载 8 个记忆存储，支持 `read_write` 和 `read_only` 两种访问模式；每次写入都会创建不可变的版本记录，提供完整的审计追踪和历史回滚能力。建议将记忆组织为多个小文件（单个上限 100KB），对于可能受不可信输入影响的存储使用只读模式以防止提示注入攻击。

页面：https://platform.claude.com/docs/en/managed-agents/skills
总结：Skills（技能）是可复用的文件系统资源，赋予 Agent 特定领域的专业能力，仅在需要时加载，不会持续占用上下文窗口。支持两类技能：Anthropic 预置技能（如 Excel、PowerPoint、Word、PDF 处理）和开发者自定义上传的技能，均在 Agent 创建时通过 `skills` 数组配置。每个 Session 最多支持 20 个技能（跨所有 Agent 合计），自定义技能可指定版本号或使用 `latest`。

页面：https://platform.claude.com/docs/en/managed-agents/mcp-connector
总结：MCP Connector 允许在 Agent 中声明 MCP 服务器（在 Agent 创建时通过 `mcp_servers` 数组配置 URL），并在 Session 创建时通过 `vault_ids` 注入对应的认证凭证，实现配置与密钥的解耦。Agent 通过 `mcp_toolset` 类型的工具条目引用具体的 MCP 服务器，默认权限策略为 `always_ask`（每次工具调用均需用户确认）。当前仅支持通过 HTTP streamable transport 暴露端点的远程 MCP 服务器，认证失败不会阻止 Session 创建，但会通过 `session.error` 事件通知。

页面：https://platform.claude.com/docs/en/managed-agents/permission-policies
总结：此页面介绍了如何通过权限策略控制 Agent 和 MCP 工具的执行方式。支持两种策略：`always_allow`（自动执行）和 `always_ask`（暂停等待用户确认后再执行）。自定义工具不受此策略管辖，由开发者自行控制。

页面：https://platform.claude.com/docs/en/managed-agents/multi-agent
总结：此页面介绍了多智能体（Multi-agent）协作机制，允许一个协调者 Agent 将复杂任务分发给多个专职子 Agent 并行处理。每个 Agent 运行在独立的会话线程中，拥有各自隔离的上下文和工具配置，最多支持 25 个并发线程。适用于并行化、专业化分工及任务升级等场景。

页面：https://platform.claude.com/docs/en/managed-agents/define-outcomes
总结：此页面介绍了"结果导向"（Outcome）会话模式：开发者通过 `user.define_outcome` 事件定义任务目标和评分标准（Rubric），系统会自动启动独立的评估器（Grader）对 Agent 的输出进行逐项打分。Agent 根据反馈迭代修改，直到满足所有标准或达到最大迭代次数为止。

页面：https://platform.claude.com/docs/en/managed-agents/observability
总结：此页面详细说明了与 Agent 会话的事件驱动通信机制，包括用户可发送的事件（如消息、中断、工具确认）和系统返回的事件（如 Agent 消息、工具调用、会话状态变更）。开发者通过订阅事件流来监控 Agent 的执行进度，并可随时中断或重定向会话。

页面：https://platform.claude.com/docs/en/managed-agents/github
总结：此页面介绍了如何将 GitHub 仓库集成到 Agent 会话中。在创建会话时通过 `resources` 字段挂载 GitHub 仓库，并结合 GitHub MCP 服务器，Agent 可以读取代码、创建分支、提交更改并发起 Pull Request。授权令牌仅用于克隆操作，不会在 API 响应中暴露。

页面：https://platform.claude.com/docs/en/managed-agents/migration
总结：此页面提供了从 Messages API 自定义 Agent 循环或 Claude Agent SDK 迁移到 Claude Managed Agents 的详细指南。迁移后，对话历史、工具调度和沙箱环境均由 Anthropic 基础设施托管，开发者只需创建 Agent 定义、发送事件并响应自定义工具调用即可。模型版本升级只需更改 Agent 定义中的 `model` 字段。

---

## Test & Evaluate

页面：https://platform.claude.com/docs/en/test-and-evaluate/develop-tests
总结：本页介绍如何为基于 LLM 的应用程序定义成功标准并构建评估体系。成功标准需具备具体性、可衡量性、可达性和相关性，并推荐使用精确匹配、余弦相似度、ROUGE-L、Likert 量表等多种评估方法。评估的评分方式分为代码评分、人工评分和 LLM 评分三类，优先选择自动化程度高的方式。

页面：https://platform.claude.com/docs/en/test-and-evaluate/eval-tool
总结：本页介绍 Claude Console 中的"Evaluation"工具，用于在不同场景下测试提示词效果。用户可手动添加测试用例、让 Claude 自动生成测试用例，或从 CSV 文件导入；工具还支持多版本提示词的并排对比和质量评分（5 分制）。该工具要求提示词中包含至少一个双大括号变量（如 `{{variable}}`）才能正常使用。

页面：https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/handle-streaming-refusals
总结：从 Claude 4 系列模型开始，当流式输出内容违反策略时，API 将返回 `stop_reason: "refusal"` 作为停止原因。开发者在收到该信号后必须重置对话上下文（移除被拒绝的轮次），否则后续请求将持续被拒绝。文档提供了多种编程语言的代码示例，并建议监控拒绝模式以优化提示词设计。

页面：https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/increase-consistency
总结：本页介绍多种提高 Claude 输出一致性的策略，包括：明确指定输出格式（如 JSON、XML 模板）、预填充 Assistant 回复以绕过前导语、通过示例约束输出风格，以及使用检索机制将回答锚定在固定知识库中。对于有严格 JSON Schema 要求的场景，推荐使用"结构化输出"功能以获得完全合规的格式保证。

页面：https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/mitigate-jailbreaks
总结：本页介绍防范越狱（jailbreak）和提示注入攻击的多层防护策略。主要措施包括：使用轻量模型（如 Claude Haiku）预筛选输入内容、通过输入验证过滤已知越狱模式、在系统提示中明确设定伦理边界，以及对反复违规用户进行限流或封禁。文档还提供了一个金融顾问机器人的多层防护链式示例，展示如何将各策略组合使用。

页面：https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/reduce-hallucinations
总结：本页介绍减少 Claude 幻觉（hallucination）的实用技术。基础策略包括：允许模型承认"不知道"、要求先逐字引用原文再分析、让模型为每项声明找到支持引用（找不到则删除该声明）。高级技术包括思维链验证、多次运行对比（Best-of-N）、迭代精炼以及限制模型只能使用提供的文档而非内部知识。

页面：https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/reduce-latency
总结：本页介绍降低 Claude API 响应延迟的主要方法。核心建议包括：根据速度需求选择合适的模型（如 Claude Haiku 4.5 最快）、精简提示词和输出长度（使用句段数量而非字数限制）、设置合理的 `max_tokens` 上限，以及启用流式传输（streaming）以改善用户感知响应速度。文档强调应先优化功能正确性，再考虑延迟优化。

页面：https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/reduce-prompt-leak
总结：本页介绍降低提示词泄露风险的策略，包括：将敏感指令与用户输入隔离、在 User 和 Assistant 轮次中重申关键保密要求、使用正则或 LLM 对输出进行后处理过滤，以及仅在提示词中保留任务必需的信息以减少泄露面。文档也提示过度的防泄露措施可能降低模型性能，需注意权衡取舍。

---

## API Reference

页面：https://platform.claude.com/docs/en/api/overview
总结：Claude API 是一个基于 `https://api.anthropic.com` 的 RESTful API，提供对 Claude 模型及 Claude Managed Agents 的程序化访问。可用 API 分为正式版（Messages、批量消息、Token 计数、模型列表）和测试版（Files、Skills、Agents、Sessions、Environments）两类。所有请求须携带 `x-api-key`（或 `Authorization`）、`anthropic-version` 和 `content-type` 三个必要请求头。

页面：https://platform.claude.com/docs/en/api/versioning
总结：调用 Claude API 时，请求头中必须包含 `anthropic-version` 字段（如 `2023-06-01`）以指定 API 版本。Anthropic 承诺在同一版本内保持已有输入/输出参数不变，但可能会新增可选参数、新增输出字段或新增枚举值。建议始终使用最新版本，旧版本被视为已弃用，新用户可能无法使用。

页面：https://platform.claude.com/docs/en/api/beta-headers
总结：通过在请求中添加 `anthropic-beta` 头部，可访问尚未正式发布的实验性功能，多个 beta 功能名称以逗号分隔。Beta 功能名称通常遵循 `feature-name-YYYY-MM-DD` 格式，可能存在破坏性变更或被废弃，不保证与正式版相同的 SLA。如传入无效的 beta 头部，API 将返回 `invalid_request_error` 错误。

页面：https://platform.claude.com/docs/en/api/errors
总结：Claude API 使用标准 HTTP 错误码体系，常见错误包括 400（请求格式错误）、401（认证失败）、429（触发速率限制）、500（服务内部错误）和 529（服务过载）等，错误响应统一以 JSON 格式返回，包含 `type`、`message` 和 `request_id` 字段。每个 API 响应都携带唯一的 `request-id` 响应头，便于联系支持时快速定位问题。对于耗时较长的请求，建议使用流式 API（Streaming）或批量消息 API，以避免因连接超时导致请求失败。

页面：https://platform.claude.com/docs/en/api/rate-limits
总结：API 限制分为两类：月度消费上限（Spend Limits）和速率限制（Rate Limits，含 RPM/ITPM/OTPM），按使用层级（Tier 1 至 Tier 4）自动累进升级。重要特性是已缓存的输入 Token（`cache_read_input_tokens`）在大多数模型中不计入 ITPM 限额，因此合理使用 Prompt Caching 可大幅提升有效吞吐量。API 使用令牌桶算法进行限流，超出限制时返回 429 错误，响应头中也包含剩余配额和重置时间等信息。

页面：https://platform.claude.com/docs/en/api/service-tiers
总结：Anthropic 提供三个服务层级：标准层（Standard，默认）、优先层（Priority Tier，需承诺用量）和批量层（Batch，异步场景）。优先层在高峰期享有更高的请求处理优先级，目标可用性达 99.5%，通过 `service_tier: "auto"` 参数启用，在优先层容量不足时自动回退到标准层。优先层通过签订承诺合同（支持 1/3/6/12 个月）来获取固定的每分钟 input/output token 容量，不支持 Claude Mythos Preview 模型。

页面：https://platform.claude.com/docs/en/api/ip-addresses
总结：Anthropic 服务使用固定 IP 地址，可用于配置防火墙规则。入站 IP 为 `160.79.104.0/23`（IPv4）和 `2607:6bc0::/48`（IPv6）；出站 IP（如 MCP 工具回调等对外请求）为 `160.79.104.0/21`（IPv4）。一组旧 IP 地址（`34.162.x.x/32` 系列）已停用，可从防火墙白名单中移除。

页面：https://platform.claude.com/docs/en/api/supported-regions
总结：Claude API 目前支持全球 150 余个国家和地区，覆盖欧洲、亚洲、非洲、美洲及大洋洲的绝大部分地区，包括美国、英国、日本、印度、巴西、澳大利亚、中国台湾等主要市场。乌克兰被支持，但克里米亚、顿涅茨克和卢甘斯克地区除外。中国大陆、俄罗斯、朝鲜等地区不在支持范围内。

页面：https://platform.claude.com/docs/en/api/client-sdks
总结：Anthropic 提供了多种官方客户端 SDK，包括 Python、TypeScript、Java、Go、Ruby、C#、PHP 以及命令行工具，方便开发者以符合各语言习惯的方式调用 Claude API。所有 SDK 均支持流式输出、自动重试和错误处理等功能。此外，所有 SDK 均支持通过 Amazon Bedrock、Google Vertex AI 和 Microsoft Foundry 等云平台使用 Claude。

页面：https://platform.claude.com/docs/en/api/openai-sdk
总结：Anthropic 提供了一个兼容层，允许开发者直接使用 OpenAI SDK 对接 Claude API，只需修改 base URL、API 密钥和模型名称即可快速评估 Claude 的能力。该兼容层主要用于测试和对比模型性能，不建议用于生产环境，且部分 OpenAI 功能（如音频输入、prompt caching、严格模式工具调用）不受支持。若需完整功能（如 PDF 处理、引用、扩展思考），建议使用原生 Claude API。

页面：https://platform.claude.com/docs/en/api/sdks/python
总结：Anthropic Python SDK 支持同步和异步两种客户端，可通过 `pip install anthropic` 安装，要求 Python 3.9 及以上版本。SDK 提供流式响应、工具调用、消息批处理、文件上传、自动分页等丰富功能，并内置自动重试和超时机制。此外，SDK 还支持通过额外依赖项接入 Amazon Bedrock、Google Vertex AI 和 Microsoft Foundry 等云平台。

页面：https://platform.claude.com/docs/en/api/sdks/typescript
总结：Anthropic TypeScript SDK 通过 `npm install @anthropic-ai/sdk` 安装，要求 TypeScript 4.9 及以上，支持 Node.js、Deno、Bun、Cloudflare Workers 等多种运行环境。SDK 提供完整的类型定义、流式响应助手、工具调用辅助（支持 Zod schema）、MCP 集成以及消息批处理等功能。值得注意的是，浏览器环境默认禁用，需显式设置 `dangerouslyAllowBrowser: true` 才能在前端使用。

页面：https://platform.claude.com/docs/en/api/sdks/go
总结：Anthropic Go SDK 通过 `go get github.com/anthropics/anthropic-sdk-go` 安装，要求 Go 1.23 及以上，使用 context 进行请求取消与超时控制，并采用函数式选项（functional options）模式进行配置。SDK 的请求字段遵循 Go 1.24+ 的 `omitzero` 语义，支持流式响应、工具调用、文件上传和自动分页等功能。平台集成方面，可通过子包支持 Amazon Bedrock 和 Google Vertex AI。

页面：https://platform.claude.com/docs/en/api/sdks/java
总结：Anthropic Java SDK 支持 Java 8 及以上版本，采用 Builder 模式构建请求参数，并通过 `CompletableFuture` 实现异步操作。SDK 提供同步和异步流式响应（含 `MessageAccumulator`）、工具调用（可自动从 Java 类派生 JSON Schema）、消息批处理和自动分页等功能。同时支持通过独立依赖包接入 Amazon Bedrock、Google Vertex AI 和 Microsoft Foundry 等云平台。

页面：https://platform.claude.com/docs/en/api/sdks/csharp
总结：Anthropic C# SDK（目前处于 Beta 阶段）通过 `dotnet add package Anthropic` 安装，要求 .NET Standard 2.0 及以上。SDK 支持异步流式响应（使用 `IAsyncEnumerable`）、自动分页、响应验证，并实现了 `IChatClient` 接口，可与 Microsoft.Extensions.AI 和 MCP C# SDK 等工具集成。平台方面支持通过独立 NuGet 包接入 Amazon Bedrock 和 Microsoft Foundry。

页面：https://platform.claude.com/docs/en/api/sdks/ruby
总结：Anthropic Ruby SDK 要求 Ruby 3.2.0 及以上，使用标准库 `net/http` 作为 HTTP 传输层，并通过 `connection_pool` gem 实现连接池管理。SDK 提供流式响应助手、工具调用（含 `BaseTool` 和 `tool_runner` 用于自动执行工具循环）、Sorbet 类型支持（RBI/RBS 定义）以及自动分页等功能。平台集成方面，支持通过专属客户端类接入 Amazon Bedrock 和 Google Vertex AI。

页面：https://platform.claude.com/docs/en/api/sdks/php
总结：Anthropic PHP SDK（目前处于 Beta 阶段）通过 `composer require anthropic-ai/sdk` 安装，要求 PHP 8.1.0 及以上，使用命名参数风格构建请求，并提供值对象（value objects）和 Builder 模式初始化方式。SDK 支持流式响应（SSE）、自动分页、自定义请求选项（含重试次数控制）以及未文档化参数的传递。错误处理方面，提供了覆盖多种 HTTP 状态码的细分异常类型。

页面：https://platform.claude.com/docs/en/api/sdks/cli
总结：`ant` CLI 工具让开发者可以直接从终端访问 Claude API，采用 `resource action` 命令结构，所有 API 资源均以子命令形式暴露。相比 `curl`，`ant` 支持 YAML/JSON 文件输入、类型化标志、`@path` 引用内联文件内容，以及内置的 `--transform`（GJSON 路径）对响应进行过滤和重塑，无需额外 JSON 工具。CLI 还原生支持 Claude Code，可在 Claude Code 中直接通过自然语言操作 API 资源。

页面：https://platform.claude.com/docs/en/api/beta/sessions
总结：Sessions API 是 Anthropic 托管 Agent 体系的一部分，提供创建、列举、查询、更新、删除和归档 Agent 会话的接口。创建会话时需要指定 Agent ID 和环境 ID，还可附加 GitHub 仓库、文件、内存存储等资源，并支持元数据和自定义标题。会话返回的数据包括解析后的 Agent 配置、资源挂载信息、Token 使用统计以及结果评估等完整信息。

页面：https://platform.claude.com/docs/en/api/beta/sessions/events/stream
总结：该接口通过 GET `/v1/sessions/{session_id}/events/stream` 以 Server-Sent Events（SSE）方式实时流式传输会话中的事件。支持多达 30 余种事件类型，涵盖用户消息（`user.message`）、工具确认（`user.tool_confirmation`）、Agent 思考（`agent.thinking`）、工具调用（`agent.tool_use`）、代码执行、截图以及会话结束等各类事件。请求头支持通过 `anthropic-beta` 指定所需的 Beta 版本（如 `managed-agents-2026-04-01`）。

---

## Release Notes & Resources

页面：https://platform.claude.com/docs/en/release-notes/overview
总结：该页面记录了 Claude 平台（API、SDK、控制台）的历次更新日志，时间跨度从 2024 年至今。近期重要更新包括：2026 年 5 月推出多智能体会话与 Webhooks 公测、4 月发布 Claude Opus 4.7 模型、以及 3 月将 1M token 上下文窗口正式商用化。整体涵盖模型发布、功能上线、模型退役等多类事件。

页面：https://platform.claude.com/docs/en/release-notes/system-prompts
总结：该页面展示了 Claude 在 claude.ai 及移动应用中各版本模型（如 Opus 4.7、Sonnet 4.6 等）所使用的系统提示词内容。这些提示词涵盖安全规范、儿童保护政策、拒绝有害请求的原则，以及语气和格式偏好（强调简洁、温和的沟通风格）。需注意，这些系统提示仅适用于 claude.ai，通过 API 使用时所依赖的是不同的系统提示。

页面：https://platform.claude.com/docs/en/resources/overview
总结：该页面汇集了开发者使用 Claude API 所需的各类学习资源，包括各模型的系统卡（System Card）文档、快速入门示例、在线课程、代码食谱（Cookbook）以及使用案例指南。此外还提供了专为 AI 摄取优化的 API 文档（如 llms.txt），方便开发者和模型自身快速获取接口信息。
