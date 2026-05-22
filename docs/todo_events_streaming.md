# TODO: Events & Streaming（事件流）实现

> 来源标记：`anthropic-docs-summary.md` → `====》 思考 ing`
> 优先级：**Phase 1（核心基础设施）**

---

## 背景

Events & Streaming 是客户端与 Managed Agent 之间的**实时双向通信机制**，基于 SSE（Server-Sent Events）。理解所有事件类型及其处理方式是构建健壮客户端的关键。

---

## 与 Messages API Streaming 的区别

虽然底层都是 SSE，但层级完全不同：

| 维度 | Messages API Streaming | Managed Agents 事件流 |
|------|----------------------|-------------------|
| 层级 | token 级别 | 任务编排级别 |
| 典型事件 | `content_block_delta`（一个字） | `agent.tool_use`（调了一次工具） |
| 目的 | 流式显示生成中的文字 | 追踪 Agent 在做什么 |
| 内部关系 | — | 每条 `agent.message` 内部仍是 token 流，但已封装 |

---

## 完整事件类型目录

### 用户发送（Client → Server）

| 事件类型 | 说明 | 何时发送 |
|---------|------|---------|
| `user.message` | 普通用户消息 | Session 处于 idle 时 |
| `user.interrupt` | 中断 Agent 执行 | Session 处于 running 时 |
| `user.custom_tool_result` | 自定义工具执行结果 | 收到 `agent.custom_tool_use` 后 |
| `user.tool_confirmation` | 工具执行确认（Human-in-the-loop） | 收到 `session.requires_action` 后 |
| `user.define_outcome` | 定义验收标准（目标驱动迭代） | Session 处于 idle 时 |

### Agent 发送（Server → Client）

| 事件类型 | 说明 |
|---------|------|
| `agent.thinking` | Agent 正在推理思考中 |
| `agent.message` | Agent 的文本回复内容 |
| `agent.tool_use` | Agent 调用了某个工具（内置工具） |
| `agent.tool_result` | 内置工具执行完成，返回结果 |
| `agent.custom_tool_use` | Agent 需要调用自定义工具（需客户端执行） |

### Session 状态事件

| 事件类型 | 说明 |
|---------|------|
| `session.status_idle` | Session 空闲，等待用户（含 `stop_reason`） |
| `session.status_running` | Session 开始处理 |
| `session.requires_action` | 需要人工确认（权限策略 `always_ask`） |
| `session.error` | 发生错误（含 `retry_status`） |

### 计费/监控事件

| 事件类型 | 说明 |
|---------|------|
| `span.model_request_end` | 每次模型请求结束，含 token 使用量 |

---

## 典型事件流示意

```
客户端 → user.message("分析这段代码")
       ← session.status_running
       ← agent.thinking（推理中）
       ← agent.tool_use（调 bash 工具）
       ← agent.tool_result（bash 返回结果）
       ← agent.thinking（继续推理）
       ← agent.message（"这段代码存在以下问题..."）
       ← span.model_request_end（token 计费数据）
       ← session.status_idle（stop_reason: end_turn）
```

---

## 订阅事件流实现

### 基础 SSE 监听

```python
import httpx
import json

def stream_session_events(session_id: str, api_key: str):
    """订阅 Session 事件流"""
    with httpx.stream(
        "GET",
        f"https://api.anthropic.com/v1/sessions/{session_id}/events",
        headers={
            "Authorization": f"Bearer {api_key}",
            "anthropic-version": "2023-06-01",
        },
        timeout=None  # 长连接，不设超时
    ) as response:
        for line in response.iter_lines():
            if not line:
                continue
            if line.startswith("data:"):
                event = json.loads(line[5:].strip())
                yield event
```

### 完整事件处理器

```python
class AgentEventHandler:
    def __init__(self, session_id: str, api_key: str):
        self.session_id = session_id
        self.api_key = api_key
        self.token_usage = {"input": 0, "output": 0, "cache_read": 0}
    
    def handle(self, event: dict):
        event_type = event.get("type")
        
        match event_type:
            case "agent.thinking":
                # Agent 思考过程（可展示给用户或仅记录日志）
                self._on_thinking(event.get("thinking", ""))
            
            case "agent.message":
                # Agent 正式回复
                self._on_message(event.get("content", ""))
            
            case "agent.tool_use":
                # 内置工具被调用（信息性，无需客户端响应）
                print(f"[工具调用] {event['name']}: {event.get('input', {})}")
            
            case "agent.tool_result":
                # 内置工具返回结果（信息性）
                print(f"[工具结果] {event.get('content', '')[:100]}...")
            
            case "agent.custom_tool_use":
                # 自定义工具：需要客户端执行并返回结果
                result = self._execute_custom_tool(
                    event["name"], 
                    event["input"]
                )
                self._send_custom_tool_result(event["id"], result)
            
            case "session.requires_action":
                # Human-in-the-loop：需要用户确认
                for tool_id in event.get("event_ids", []):
                    confirmed = self._ask_user_confirmation(event)
                    self._send_tool_confirmation(tool_id, confirmed)
            
            case "session.error":
                error = event.get("error", {})
                retry_status = error.get("retry_status")
                if retry_status == "will_retry":
                    print(f"[可恢复错误] 自动重试中: {error.get('message')}")
                else:
                    raise Exception(f"不可恢复错误: {error}")
            
            case "span.model_request_end":
                # 记录 token 使用量
                usage = event.get("model_usage", {})
                self.token_usage["input"] += usage.get("input_tokens", 0)
                self.token_usage["output"] += usage.get("output_tokens", 0)
                self.token_usage["cache_read"] += usage.get("cache_read_input_tokens", 0)
            
            case "session.status_idle":
                stop_reason = event.get("stop_reason")
                print(f"[完成] stop_reason={stop_reason}")
                return False  # 信号：停止监听
        
        return True  # 继续监听
    
    def _execute_custom_tool(self, name: str, input_data: dict) -> str:
        """执行自定义工具逻辑"""
        if name == "query_db":
            return query_database(input_data["sql"])
        elif name == "call_internal_api":
            return call_api(input_data["endpoint"], input_data.get("params"))
        return f"Unknown tool: {name}"
    
    def _send_custom_tool_result(self, tool_use_id: str, result: str):
        httpx.post(
            f"https://api.anthropic.com/v1/sessions/{self.session_id}/events",
            json={
                "type": "user.custom_tool_result",
                "tool_use_id": tool_use_id,
                "content": result
            },
            headers={"Authorization": f"Bearer {self.api_key}"}
        )
    
    def _send_tool_confirmation(self, tool_use_id: str, confirmed: bool):
        httpx.post(
            f"https://api.anthropic.com/v1/sessions/{self.session_id}/events",
            json={
                "type": "user.tool_confirmation",
                "tool_use_id": tool_use_id,
                "confirmed": confirmed
            },
            headers={"Authorization": f"Bearer {self.api_key}"}
        )
    
    def _on_thinking(self, thinking: str):
        pass  # 可以打印或记录日志
    
    def _on_message(self, content: str):
        print(content)
    
    def _ask_user_confirmation(self, event: dict) -> bool:
        print(f"Agent 请求执行操作，是否确认？\n{event}")
        return input("确认(y/n): ").lower() == "y"
```

---

## 自定义工具完整交互流程

```
1. Agent 生成工具调用意图
   Server → Client: agent.custom_tool_use { id: "xxx", name: "query_db", input: {...} }

2. 客户端执行工具
   Client: result = db.query(input["sql"])

3. 客户端返回结果
   Client → Server: user.custom_tool_result { tool_use_id: "xxx", content: result }

4. Agent 继续推理（基于工具结果）
   Server → Client: agent.message（使用了查询结果的回答）
```

---

## 事件历史回放（离线分析）

实时 SSE 只能消费一次，但事件会持久化存储，支持多次查询历史：

```python
def get_session_history(session_id: str, api_key: str):
    """拉取完整事件历史（分页）"""
    all_events = []
    page = 1
    
    while True:
        response = httpx.get(
            f"https://api.anthropic.com/v1/sessions/{session_id}/events",
            params={"page": page},
            headers={"Authorization": f"Bearer {api_key}"}
        )
        data = response.json()
        all_events.extend(data.get("events", []))
        
        if not data.get("has_more"):
            break
        page += 1
    
    return all_events
```

**适用场景**：
- 连接中断后补拉遗漏事件
- 离线分析完整执行过程
- 排查生产问题（复现 Agent 行为）

---

## Multi-Agent 场景下的事件路由

多 Sub-Agent 并发时，事件流包含 `session_thread_id` 字段：

```python
# Session 主流：汇总视图（所有 Agent 的 agent.message）
# Thread 流：单 Agent 详细视图（含 tool_use、thinking 等）

def route_event(event: dict):
    thread_id = event.get("session_thread_id")
    
    if thread_id is None:
        # 主 Session 事件
        handle_main_event(event)
    else:
        # 特定 Sub-Agent 的事件
        handle_thread_event(thread_id, event)

# 工具确认回复必须带 session_thread_id，路由到正确的 Thread
def send_tool_confirmation_to_thread(session_id, tool_use_id, thread_id, confirmed):
    httpx.post(
        f"/sessions/{session_id}/events",
        json={
            "type": "user.tool_confirmation",
            "tool_use_id": tool_use_id,
            "session_thread_id": thread_id,  # 关键：路由到正确 Thread
            "confirmed": confirmed
        }
    )
```

---

## 实施步骤

- [ ] 实现基础 SSE 订阅客户端（支持 `Last-Event-ID` 断线重连）
- [ ] 实现完整的事件分发处理器（覆盖所有 20+ 种事件类型）
- [ ] 实现 `agent.custom_tool_use` → 执行 → `user.custom_tool_result` 完整链路
- [ ] 实现 `session.requires_action` → 人工确认 → `user.tool_confirmation` 流程
- [ ] 监听 `span.model_request_end`，对接监控系统记录 token 使用量
- [ ] 监听 `session.error`，根据 `retry_status` 决定是告警还是等自动重试
- [ ] 实现事件历史回放接口（用于调试和离线分析）
- [ ] Multi-Agent 场景：正确处理 `session_thread_id` 路由
