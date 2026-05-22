# TODO: Compaction（上下文压缩）实现

> 来源标记：`anthropic-docs-summary.md` → `=====》 TODO`
> 优先级：**Phase 2（中优先级）**
> Beta 功能：需要 `compact_20260112` header

---

## 背景与价值

长对话中 context 会越来越大，最终导致：

- Token 成本线性增长
- 推理质量下降（Context Rot）
- 超出 context 上限（Opus 4.6 / Sonnet 4.6 最大 1M token）

Compaction 解决方案：触发阈值后，把旧消息压缩成摘要 block 插入，丢弃原始旧消息，换来 token 成本可控。

```
[旧对话原始内容]（丢弃）
↓
[compaction block: "前面讨论了X、决定了Y、完成了Z..."]
[近期对话]（保留）
```

---

## 核心机制

### 触发阈值

- 默认触发阈值：**150k token**
- 可配置最小值：**50k token**
- 压缩结果作为 `compaction` block 插入
- 后续请求自动丢弃被压缩的原始内容

### 两种模式

| 模式 | 配置 | 适用场景 |
|------|------|---------|
| 全自动 | 默认 | 生产环境，不打断用户 |
| 暂停检查 | `pause_after_compaction: true` | 开发/测试阶段，检查摘要质量 |

### 关键特性

- 支持模型：Mythos Preview、Opus 4.6、Sonnet 4.6
- `usage.iterations` 数组提供每步计费细分
- 与 Streaming（SSE）兼容：compaction block 作为单个 delta 流式输出（无中间流）

---

## 实现方式

### 基础配置（Messages API）

```python
import anthropic

client = anthropic.Anthropic()

response = client.beta.messages.create(
    model="claude-sonnet-4-6",
    max_tokens=8096,
    compaction={
        "enabled": True,
        "threshold_tokens": 150000,   # 触发阈值，默认 150k
        "instructions": "保留：所有文件修改记录、未解决的 bug、架构决策",  # 告诉压缩模型哪些信息重要
        "pause_after_compaction": False  # 生产环境关闭，全自动
    },
    messages=[...]
)
```

### 开发调试模式（暂停检查）

```python
response = client.beta.messages.create(
    model="claude-sonnet-4-6",
    compaction={
        "enabled": True,
        "pause_after_compaction": True   # 压缩后暂停，返回 idle，stop_reason: compaction_pause
    },
    ...
)

# 压缩后 session 进入 idle，stop_reason: compaction_pause
if response.stop_reason == "compaction_pause":
    # 检查 compaction block 内容
    compaction_block = next(b for b in response.content if b.type == "compaction")
    print("摘要内容：", compaction_block.summary)
    
    # 确认质量后继续
    if quality_ok(compaction_block):
        continue_conversation(response)
```

### 在 Managed Agents Session 中的处理

Managed Agents 内部自动处理 compaction，客户端不需要额外干预。但可以通过系统提示配置压缩指令：

```python
# 在 Agent system prompt 中加入提示
system = """你是一个编码助手。
在处理长对话时，请记住以下内容对后续工作非常重要：
- 已修改的文件列表
- 当前未解决的问题
- 用户确认的架构决策
"""
```

---

## `instructions` 参数使用指南

`instructions` 是开发者写的配置（硬编码，不是用户传的），告诉压缩模型"哪些信息重要，要保留在摘要里"：

```python
# 代码审查场景
instructions = "保留：所有发现的安全漏洞、待修复的代码行、已确认的修改"

# 客服场景
instructions = "保留：用户问题核心诉求、已解决的问题、待跟进事项"

# 分析场景
instructions = "保留：关键数据指标、分析结论、下一步建议"
```

---

## 与 Context Editing 的区别

| 维度 | Compaction（服务端） | Context Editing（客户端） |
|------|---------------------|------------------------|
| 实现位置 | Anthropic 服务端自动执行 | 客户端手动操作 messages 数组 |
| 控制粒度 | 粗（按 token 阈值触发） | 细（精确控制每条消息） |
| 适用场景 | Managed Agents、长对话 | Messages API 手写循环 |
| 开发成本 | 低（配置即可） | 高（需要自己实现摘要逻辑） |

> Managed Agents 用户优先选 Compaction，自行维护 messages 数组时才需要 Context Editing。

---

## 实施步骤

- [ ] 添加 `compact_20260112` beta header
- [ ] 在开发环境开启 `pause_after_compaction: true`，检查摘要质量
- [ ] 根据业务场景编写 `instructions`，保留关键信息
- [ ] 配置合适的 `threshold_tokens`（代码库分析场景建议 80k，节省成本）
- [ ] 生产环境关闭 `pause_after_compaction`，设置为全自动
- [ ] 监控 `usage.iterations` 了解每轮压缩成本
