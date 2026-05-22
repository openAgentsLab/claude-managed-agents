# TODO: Programmatic Tool Calling（编程式工具调用）实现

> 来源标记：`anthropic-docs-summary.md` → `====》 TODO`
> 优先级：**Phase 3（长期演进）**
> 需要工具版本：`code_execution_20260120`

---

## 背景与价值

传统工具调用是"一轮一轮"的推理循环，N 个工具调用 = N 轮推理，每轮结果都进 context window，成本随轮次线性增长。

Programmatic Tool Calling 让 Claude 用 Python 代码**一次性批量调用多个工具**，将 N 轮压缩为 1 轮。

| 对比维度 | 传统方式 | Programmatic 方式 |
|---------|---------|-----------------|
| 推理轮次 | N 个工具 = N 轮推理 | N 个工具 = 1 轮推理 |
| Context 增长 | 每轮结果都进 context | 中间结果不进 context |
| Token 计费 | 每次工具结果都计 input/output token | 工具结果不计 token |
| 适用场景 | 顺序依赖的工具调用 | 独立并发的工具调用 |

**对 ai-coding 的价值**：大型重构任务中（同时读写多个文件），将 N 轮循环压缩为 1 轮，是 Harness 效果循环最重要的优化方向。

---

## 核心原理

### 工作流程

```
1. 模型生成 Python 代码（包含预处理逻辑）
2. 沙箱执行这段代码（调用工具、处理结果）
3. 只有最终 return 值回到 context
```

模型在"写代码"这一步就已经决定了：要调用哪些工具、结果怎么过滤裁剪、最后返回什么给自己看。本质上是模型在用代码描述自己的信息需求。

### Claude 生成的代码示例

```python
# Claude 生成如下代码，在沙箱中执行
results = await asyncio.gather(
    read_file("/path/to/file1.go"),
    read_file("/path/to/file2.go"),
    bash("git log --oneline -10")
)

# 预处理：过滤只返回包含 "auth" 的行（而不是三个完整文件）
return [line for result in results for line in result.split('\n') if 'auth' in line]
```

### 预处理能力

Python 代码可以做真正的预处理，而不只是转手传递结果：

```python
# 过滤：只返回包含关键字的行
files = await asyncio.gather(read_file("a.go"), read_file("b.go"))
return [line for f in files for line in f.split('\n') if 'auth' in line]

# 条件分支：根据结果决定是否继续读文件
log = await bash("git log --oneline -50")
if "security" in log:
    return await bash("git show HEAD")
return "no security commits"

# 聚合：合并多个来源的结果
results = await asyncio.gather(
    bash("grep -r 'TODO' src/"),
    bash("grep -r 'FIXME' src/")
)
return "\n".join(results)
```

> **注意**：如果 Claude 写的代码没有任何处理直接 return，节省的只是多轮往返的 overhead，而不是 context 大小本身。

---

## 配置要求

### 必要条件

1. 启用新版代码执行工具（支持 Programmatic Tool Calling 的版本）
2. 在需要可被 Python 代码调用的工具定义中添加 `allowed_callers`

```json
{
  "type": "bash",
  "allowed_callers": ["code_execution_20260120"]
}
```

```json
{
  "type": "custom",
  "name": "read_file",
  "description": "Read file content",
  "input_schema": {...},
  "allowed_callers": ["code_execution_20260120"]
}
```

### 不兼容项

- `strict: true`（工具参数严格验证）
- `disable_parallel_tool_use`
- MCP Connector 工具

---

## 实现示例

### Python SDK

```python
import anthropic

client = anthropic.Anthropic()

response = client.messages.create(
    model="claude-sonnet-4-6",
    max_tokens=4096,
    tools=[
        {
            "type": "code_execution_20260120",  # 必须是新版本
        },
        {
            "name": "read_file",
            "description": "Read file content from path",
            "input_schema": {
                "type": "object",
                "properties": {
                    "path": {"type": "string"}
                },
                "required": ["path"]
            },
            "allowed_callers": ["code_execution_20260120"]  # 允许被 Python 代码调用
        },
        {
            "name": "bash",
            "description": "Execute bash command",
            "input_schema": {
                "type": "object", 
                "properties": {
                    "command": {"type": "string"}
                },
                "required": ["command"]
            },
            "allowed_callers": ["code_execution_20260120"]
        }
    ],
    messages=[{
        "role": "user",
        "content": "分析 src/ 目录下所有 Go 文件中的错误处理模式"
    }]
)
```

---

## 适用场景

### 高收益场景（优先实现）

1. **批量文件读取**：同时读取多个文件进行分析
   ```python
   files = await asyncio.gather(
       read_file("main.go"), read_file("handler.go"), read_file("service.go")
   )
   ```

2. **大型重构**：同时读多个文件、写多个文件
   ```python
   originals = await asyncio.gather(*[read_file(f) for f in file_list])
   # 处理后...
   await asyncio.gather(*[write_file(f, content) for f, content in results])
   ```

3. **代码库分析**：并发 grep + 文件读取
   ```python
   grep_result = await bash("grep -r 'deprecated' src/")
   files_to_check = extract_file_paths(grep_result)
   contents = await asyncio.gather(*[read_file(f) for f in files_to_check])
   return summarize(contents)
   ```

### 低收益场景（不值得引入）

- 顺序依赖的工具调用（A 的结果决定 B 的参数）
- 单次工具调用
- 需要人工确认的交互式工具

---

## 实施步骤

- [ ] 评估当前工具调用模式，统计每个 Session 平均工具调用轮次
- [ ] 识别可以并行化的工具调用组合（独立的文件读取、bash 命令）
- [ ] 升级 code_execution 工具版本到 `code_execution_20260120`
- [ ] 在独立工具定义中添加 `allowed_callers` 字段
- [ ] 在系统提示中引导 Claude 使用 `asyncio.gather` 并行调用工具
- [ ] 测试验证：对比启用前后的 token 消耗和响应时间
- [ ] 注意处理兼容性：移除 `strict: true` 和 `disable_parallel_tool_use`
