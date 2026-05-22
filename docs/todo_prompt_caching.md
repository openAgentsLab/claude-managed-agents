# TODO: Prompt Caching 实现

> 来源标记：`anthropic-docs-summary.md` → `=====》 TODO`
> 优先级：**Phase 1（最高优先级，必须考虑）**

---

## 背景与价值

Prompt Caching 是最重要的成本优化手段之一。系统提示和工具定义在每次请求中都会重复传输，启用缓存后：

- 读取仅 **0.1x** 基础输入价格（节省 90%）
- 系统提示 + 工具定义首次写入 **1.25x**（5 分钟 TTL）或 **2x**（1 小时 TTL）
- **Prompt Cache 命中不计入 ITPM 速率限制**（大多数模型），是提高有效吞吐量的关键
- 对 ai-coding 场景：系统提示和工具定义每次请求都重复，设置 cache_control 可节省 ~90% 的系统提示 token 费用

---

## 核心机制

### 缓存断点规则

- 每次请求最多设置 **4 个缓存断点**
- 最小可缓存长度：1024-4096 token（取决于模型）
- 内容排列原则：**稳定内容在前，变化内容在后**
  - 稳定：系统提示、工具定义、历史对话
  - 变化：当前用户消息

### 缓存失效条件

- thinking block 内容变化
- speed（Fast Mode）设置改变
- 图片内容变化

---

## 实现方式

### 基础用法：给系统提示打缓存标记

```python
client.messages.create(
    model="claude-sonnet-4-6",
    system=[{
        "type": "text",
        "text": "很长的系统提示...",
        "cache_control": {"type": "ephemeral"}  # 打标记，缓存到这里
    }],
    messages=[{"role": "user", "content": "用户消息（变化内容，不 cache）"}]
)
```

### 工具定义缓存

工具定义通常很稳定，同样可以打标记：

```python
tools = [
    {
        "name": "bash",
        "description": "...",
        "input_schema": {...},
        "cache_control": {"type": "ephemeral"}  # 在最后一个工具定义上打标记
    }
]
```

### 验证缓存命中

检查响应的 usage 字段：

```python
response = client.messages.create(...)
usage = response.usage

# 首次请求：写入缓存
print(usage.cache_creation_input_tokens)  # > 0，产生写入费用（1.25x）

# 后续请求：命中缓存
print(usage.cache_read_input_tokens)      # > 0，只需 0.1x 费用
print(usage.input_tokens)                 # 仅计非缓存 token
```

---

## 内容排列顺序（关键）

```
请求结构（从上到下，稳定 → 变化）：

1. system prompt          ← cache_control 打在这里
2. 工具定义               ← 可以也打 cache_control
3. 历史对话（固定部分）   ← 可以打 cache_control
4. 当前用户消息           ← 不打，每次变化
```

> 变化内容放前面会导致后面的缓存全部失效。

---

## TTL 配置选择

| TTL | 写入价格倍数 | 适用场景 |
|-----|-----------|---------|
| 5 分钟（默认） | 1.25x | 短对话、低频请求 |
| 1 小时 | 2x | 长会话、高频请求（节省更多） |

---

## 在 Managed Agents 中的注意事项

Managed Agents（`/v1/agents` + `/v1/sessions`）的 system prompt 和工具定义在 Agent 创建时配置，每个 Session 初始化时都会重新装载。建议：

1. 在 Agent 的 `system` 字段设置稳定系统提示并配置 cache_control
2. `tools` 字段中对长工具描述打缓存标记
3. Session 复用同一 Agent（`agent_id`），避免频繁重建 Agent

---

## 实施步骤

- [ ] 确认系统提示结构，将稳定内容移到最前
- [ ] 在 `system` 字段最后一个 block 添加 `cache_control: {type: "ephemeral"}`
- [ ] 在工具定义的最后一个工具添加 `cache_control`
- [ ] 部署后监控 `cache_read_input_tokens` 确认命中率
- [ ] 根据请求频率决定是否升级到 1 小时 TTL
