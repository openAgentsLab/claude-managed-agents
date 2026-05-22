# TODO: Define Outcomes（目标驱动自动评估迭代）实现

> 来源标记：`anthropic-docs-summary.md` → `====》 TODO`
> 优先级：**Phase 3（长期演进）**
> 状态：Research Preview

---

## 背景与价值

Define Outcomes 将"人工 review → 打回修改"的循环自动化：

```
传统流程：
  用户发需求 → Agent 做一版 → 用户 review → 发现问题 → 打回修改 → 循环...

Define Outcomes 流程：
  用户发需求 + 验收标准 → Agent 做一版 → Grader 自动评估 
  → 不达标则自动反馈 → Agent 修改 → 循环直到达标
```

**价值**：对有明确、可量化标准的任务（代码必须通过测试、文档必须覆盖所有 API），彻底解放人工 review 循环。

---

## 核心机制

### 完整工作流

```
1. 定义 rubric（验收标准，Markdown 格式，按标准列出）
2. 发送 user.define_outcome 事件（含 description、rubric、max_iterations）
3. Agent 开始工作，产出第一版结果
4. 独立 Grader（独立 context，避免被主 Agent 影响）对照 rubric 评估
5. Grader 返回 per-criterion 细分反馈
6. 结果判断：
   - satisfied：达标，Session 进入 idle
   - needs_revision：不达标，反馈自动注入 Agent context → Agent 修改重做
   - max_iterations_reached：达到最大迭代次数，停止
   - failed：执行失败
   - interrupted：被用户中断
7. 循环直到 satisfied 或 max_iterations_reached
```

### Grader 独立 context 的原因

Grader 使用独立 context 是为了避免受主 Agent 实现过程影响，判断更客观——就像代码审查时，审查者不应该看到实现过程，只看最终结果对比标准。

### 参数说明

| 参数 | 说明 | 默认值 |
|------|------|------|
| `description` | 任务描述 | 必填 |
| `rubric` | 验收标准（Markdown） | 必填 |
| `max_iterations` | 最大迭代次数 | 3（最大 20） |

---

## 实现方式

### 发送 define_outcome 事件

```python
import anthropic
import json

client = anthropic.Anthropic()

# 先创建 Session
session = client.beta.sessions.create(
    agent_id="agt_xxx"
)

# 发送 define_outcome 事件代替普通 user.message
response = client.beta.sessions.events.create(
    session_id=session.id,
    type="user.define_outcome",
    content={
        "description": "审查 src/auth/ 目录下的所有 Go 代码，找出安全问题并修复",
        "rubric": """
## 验收标准

### 1. SQL 注入检查
- [ ] 所有数据库查询使用参数化查询，不拼接字符串

### 2. 认证验证
- [ ] 所有需要认证的接口都有 token 验证
- [ ] token 过期校验逻辑正确

### 3. 输入验证
- [ ] 用户输入在使用前进行长度和格式校验

### 4. 代码测试
- [ ] 修复后所有原有测试仍然通过（go test ./...）
""",
        "max_iterations": 5
    }
)
```

### 监听评估结果（SSE 事件流）

```python
import httpx
import json

def stream_session_events(session_id: str, api_key: str):
    with httpx.stream(
        "GET",
        f"https://api.anthropic.com/v1/sessions/{session_id}/events",
        headers={
            "Authorization": f"Bearer {api_key}",
            "anthropic-version": "2023-06-01",
        }
    ) as response:
        for line in response.iter_lines():
            if not line.startswith("data:"):
                continue
            event = json.loads(line[5:])
            
            event_type = event.get("type")
            
            # Agent 正在工作
            if event_type == "agent.message":
                print(f"[Agent] {event['content']}")
            
            # Grader 评估结果事件
            if event_type == "agent.outcome_evaluation":
                result = event.get("result")
                if result == "satisfied":
                    print("✅ 达标！任务完成")
                elif result == "needs_revision":
                    print(f"🔄 需要修改，第 {event.get('iteration')} 轮")
                    for criterion in event.get("criteria_feedback", []):
                        print(f"  - {criterion['criterion']}: {criterion['feedback']}")
                elif result == "max_iterations_reached":
                    print(f"⚠️ 达到最大迭代次数（{event.get('max_iterations')}），停止")
            
            # Session 进入空闲（所有流程结束）
            if event_type == "session.status_idle":
                stop_reason = event.get("stop_reason")
                print(f"Session 结束，原因: {stop_reason}")
                break
```

### Rubric 编写最佳实践

```markdown
## 验收标准模板

### [标准名称1]（具体可量化）
- [ ] 具体的可验证条件（可以是代码检查、测试结果等）
- [ ] 另一个条件

### [标准名称2]
- [ ] 条件...
```

**好的 Rubric 特征**：
- 每条标准具体、可量化
- 有明确的通过/失败判断依据
- 包含可自动验证的条件（如"测试通过"）

**不适合用 Define Outcomes 的场景**：
- 开放性任务（"写一篇好文章"—— "好"无法量化）
- 标准主观难以量化
- 一次性简单任务（直接发消息更快）

---

## 串联多个 Outcome

前一个 Outcome 结束后可以发送新的 `define_outcome`，实现多阶段目标驱动工作流：

```python
# 阶段1：代码分析
send_define_outcome(session_id, rubric=ANALYSIS_RUBRIC, max_iterations=3)
wait_for_idle(session_id)

# 阶段2：代码修复
send_define_outcome(session_id, rubric=FIX_RUBRIC, max_iterations=5)
wait_for_idle(session_id)

# 阶段3：测试验证
send_define_outcome(session_id, rubric=TEST_RUBRIC, max_iterations=2)
wait_for_idle(session_id)
```

---

## 与 Harness 循环的集成

在 ai-coding 项目中，Define Outcomes 应与 Harness 效果循环集成：

```
Harness 循环（现有）：
  任务 → Agent 执行 → 检查结果 → 不达标则重试

集成 Define Outcomes 后：
  任务 + Rubric → user.define_outcome → Agent 执行 → Grader 自动检查
  → 不达标则 Agent 自动修改 → 满足或超出迭代次数 → Harness 收到结果
```

**集成要点**：
- 对于有明确验收标准的任务，将 Rubric 配置化（不要硬编码）
- Harness 监听 `session.status_idle`，检查 `stop_reason`
- 如果 `stop_reason == "max_iterations_reached"`，Harness 可以上报给人工
- 输出文件在 `/mnt/session/outputs/`，通过 Files API + `scope_id` 取回

---

## 实施步骤

- [ ] 确认当前项目中哪些任务类型有明确、可量化的验收标准
- [ ] 为这些任务设计 Rubric 模板（Markdown 格式，per-criterion）
- [ ] 在 Session 事件流中处理 `agent.outcome_evaluation` 事件类型
- [ ] 实现 `user.define_outcome` 事件发送逻辑
- [ ] 测试迭代循环：验证 `needs_revision` 时 Agent 确实能根据反馈改进
- [ ] 配置 `max_iterations`（建议从 3 开始，根据任务复杂度调整）
- [ ] 处理 `max_iterations_reached` 情况（告警 + 人工介入流程）
