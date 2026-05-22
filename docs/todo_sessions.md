# TODO: Sessions 状态机设计与实现

> 来源标记：`anthropic-docs-summary.md` → `====》 思考 ing`
> 优先级：**Phase 1（核心基础设施）**

---

## 背景

Session 是 Managed Agents 的核心运行单元，代表一个正在执行中的 Agent 实例。它不是简单的 HTTP 请求，而是一个**有状态的、异步的、长时运行的后台 Job**。

理解 Session 状态机是构建可靠 Agent 平台的基础。

---

## Session 状态机

```
          用户发送事件
               │
               ▼
          ┌─────────┐
          │  idle   │◄──────────────────────────────┐
          └─────────┘                               │
               │ 开始处理                           │ 完成
               ▼                                    │
          ┌─────────┐      基础设施问题        ┌────────────┐
          │ running │─────────────────────────►│rescheduling│
          └─────────┘                          └────────────┘
               │ 不可恢复错误 / 任务完成 / interrupt
               ▼
          ┌────────────┐
          │ terminated │
          └────────────┘
```

| 状态 | 含义 | 客户端行为 |
|------|------|---------|
| `idle` | 等待用户事件，可以发新消息 | 可以发送 `user.message` 或 `user.define_outcome` |
| `running` | 正在处理，不能发新消息 | 等待 `session.status_idle` 事件 |
| `rescheduling` | 基础设施临时问题，自动重试中 | 等待，不需要干预 |
| `terminated` | 永久结束，不可恢复 | 停止监听，清理资源 |

---

## 设计价值

### 为什么 Session 需要状态？

因为 Managed Agent 的执行是**异步的、长时运行的**：

1. **连接断了任务不丢**：重连后 poll 状态，任务还在 running，等 idle 再取结果
2. **Human-in-the-loop 自然支持**：Agent 需要人确认时回到 idle，人发消息后继续
3. **基础设施故障透明恢复**：rescheduling 状态自动迁移重试，对客户端透明
4. **多客户端观察同一任务**：同一 session_id，移动端、Web 端都可以 poll

---

## Session 创建

```python
import anthropic

client = anthropic.Anthropic()

session = client.beta.sessions.create(
    agent_id="agt_xxx",
    # 可选：绑定到 Agent 具体版本（灰度发布）
    agent={"id": "agt_xxx", "version": 2},
    # 可选：挂载资源
    resources=[
        # GitHub 仓库
        {
            "type": "github_repository",
            "owner": "myorg",
            "repo": "myrepo",
            "authorization_token": "ghp_xxx"
        },
        # Memory Store（跨 Session 持久记忆）
        {
            "type": "memory_store",
            "memory_store_id": "memstore_xxx",
            "access": "read_write"
        },
        # 文件挂载
        {
            "type": "file",
            "file_id": "file_xxx",
            "mount_path": "/workspace/config.json"
        }
    ]
)
```

---

## 版本 Pin（灰度发布）

Session 可以 pin 到 Agent 具体版本，旧 Session 不受 Agent 更新影响：

```python
# 创建 Session 时 pin 到版本 3
session = client.beta.sessions.create(
    agent={"id": "agt_xxx", "version": 3}
)

# 新 Session 自动用最新版本
new_session = client.beta.sessions.create(
    agent_id="agt_xxx"  # 使用当前最新版本
)
```

**灰度发布策略**：
```python
import random

def create_session(agent_id: str, new_version: int, rollout_percentage: float):
    if random.random() < rollout_percentage:
        # 新版本流量
        return client.beta.sessions.create(
            agent={"id": agent_id, "version": new_version}
        )
    else:
        # 旧版本流量
        return client.beta.sessions.create(
            agent={"id": agent_id, "version": new_version - 1}
        )
```

---

## 删除运行中的 Session

不能直接删除 running 状态的 Session，必须先发送 interrupt：

```python
# 1. 发送 interrupt
client.beta.sessions.events.create(
    session_id=session.id,
    type="user.interrupt"
)

# 2. 等待 Session 进入 idle（Agent 干净地停下来）
# 然后才能删除

# 3. 删除
client.beta.sessions.delete(session_id=session.id)
```

---

## Session 状态轮询（断线重连场景）

```python
import httpx
import time

def poll_until_idle(session_id: str, api_key: str, timeout_seconds: int = 300):
    """轮询 Session 状态直到 idle（用于 SSE 断线后的恢复）"""
    start = time.time()
    
    while time.time() - start < timeout_seconds:
        response = httpx.get(
            f"https://api.anthropic.com/v1/sessions/{session_id}",
            headers={"Authorization": f"Bearer {api_key}"}
        )
        session = response.json()
        
        status = session["status"]
        if status == "idle":
            return session
        elif status == "terminated":
            raise Exception("Session 已终止")
        elif status in ["running", "rescheduling"]:
            time.sleep(2)  # 轮询间隔
    
    raise TimeoutError("等待 Session idle 超时")
```

---

## SSE 断线重连（不丢事件）

SSE 连接断开后，使用 `Last-Event-ID` 头重连，从上次断点续传：

```python
def stream_with_reconnect(session_id: str, api_key: str):
    last_event_id = None
    
    while True:
        headers = {"Authorization": f"Bearer {api_key}"}
        if last_event_id:
            headers["Last-Event-ID"] = last_event_id
        
        try:
            with httpx.stream(
                "GET",
                f"https://api.anthropic.com/v1/sessions/{session_id}/events",
                headers=headers
            ) as stream:
                for line in stream.iter_lines():
                    if line.startswith("id:"):
                        last_event_id = line[3:].strip()
                    elif line.startswith("data:"):
                        event = json.loads(line[5:])
                        handle_event(event)
                        
                        if event["type"] == "session.status_idle":
                            return  # 正常结束
        
        except httpx.ConnectError:
            print(f"连接断开，从 event_id={last_event_id} 重连...")
            time.sleep(1)
            continue
```

---

## 多租户场景下的 Session 设计

参考 tmp.md 中的资源归属设计：

```
Platform  →  Agent 模板（可选，复用基础配置）
Tenant    →  Agent 实例（定制 system prompt / tools / MCP / skills）+ Environment
User      →  Session + Vault（用户 OAuth 凭证）+ Files + Memory Store
```

**关键原则**：
- **租户级**：Agent 定义（behavior）、Environment（容器模板）、MCP server URL
- **用户级**：Session（每次对话）、Vault（每个用户的 OAuth token）、Memory Store（用户记忆）

```python
# 为用户创建 Session，注入用户级凭证
def create_user_session(agent_id: str, user_id: str, user_vault_id: str):
    return client.beta.sessions.create(
        agent_id=agent_id,  # 租户级 Agent
        vault_ids=[user_vault_id],  # 用户级凭证
        resources=[
            {
                "type": "memory_store",
                "memory_store_id": get_user_memory_store(user_id),
                "access": "read_write"
            }
        ]
    )
```

---

## 实施步骤

- [ ] 实现 Session 创建逻辑，支持资源挂载（GitHub/Memory/Files）
- [ ] 实现带 `Last-Event-ID` 的 SSE 断线重连机制
- [ ] 实现 Session 状态轮询（断线后快速恢复）
- [ ] 实现 interrupt + 等待 idle + 删除 的优雅关闭流程
- [ ] 设计多租户场景的 Session-per-user 模式
- [ ] 实现版本 pin 机制（灰度发布支持）
- [ ] 设置 Session 监控：记录创建时间、状态变化、terminated 原因
