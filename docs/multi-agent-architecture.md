# Multi-Agent 架构设计

> 本文档描述 ai-forge 在生产多租户场景下引入 Multi-Agent 的完整设计方案，覆盖两个演进阶段：多进程同节点、K8s 多 Deployment。
>
> **实现顺序**：先实现 AgentTool + OrchestratorHarness（进程内基础设施），再按阶段扩展到多进程和多节点部署。

---

## 背景与动机

单 Agent 的局限：
- 大型任务（代码库探索、复杂重构）只能串行执行，耗时长
- 所有工作使用同一个模型，成本不可控（探索类任务用 Sonnet 与用 Haiku 效果相近但贵数倍）
- Coordinator（规划）和 Executor（执行）混在同一个 ReAct 循环，职责不清

引入 Multi-Agent 的目标：
- **并行化**：Explore/Verify 等子任务并发执行
- **专用化**：不同类型 Agent 使用不同模型和工具集
- **可扩展**：Worker 独立扩缩，不影响用户交互层

---

## 核心约束

在生产多租户场景下，Multi-Agent 设计必须满足：

1. **用户隔离**：用户 A 的 sub-agent 只能访问用户 A 的工作区，不能读写用户 B 的数据
2. **工具执行隔离**：所有工具调用必须在该用户的沙箱内执行，不能跨用户共享沙箱
3. **资源公平性**：单用户不能无限 spawn sub-agent 打爆系统
4. **费用归属**：每个 sub-agent 的 LLM token 消耗必须归属到对应用户
5. **优雅降级**：部分 Worker 类型不可用时，系统仍能工作，而非卡死
6. **仅一层委托**：Sub-agent 物理上无法再 spawn sub-agent，防止递归失控

---

## 架构总览

```
用户请求（SSE 长连接）
    │
    ▼
OrchestratorHarness（持有 SSE 连接，外层协调循环）
    │
    ├── 正常 Brain Turn（读历史 → 推理 → 写 Session）
    │
    ├── Brain 调用 AgentTool
    │     └── INSERT sub_agent_tasks（立即返回 task_id，不阻塞）
    │                    │
    │           PostgreSQL 任务队列
    │                    │
    │         ┌──────────┴──────────┐
    │         │                     │
    │   内嵌 WorkerPool         外部 Worker 进程
    │   （coordinator 进程内）   （可选，独立部署）
    │         └──────────┬──────────┘
    │              FOR UPDATE SKIP LOCKED
    │              认领 → 检查 session → 执行 sub-agent
    │              写回 sub_agent_tasks.result
    │
    └── 等待子任务完成 → 注入 <task-notification> → 继续 Brain Turn
```

Coordinator 和 Worker 之间**唯一的耦合是 PostgreSQL**，这是整个架构能够平滑演进的基础。

---

## Agent 类型

| 类型 | 工具集 | 模型 | MaxTurns | 用途 |
|------|--------|------|----------|------|
| `explore` | Read、Glob、Grep（只读） | Haiku | 20 | 代码库搜索、模式分析；高频并发，使用 Haiku 降成本 |
| `verify`  | Bash、Read、Glob、Grep | Sonnet | 20 | 运行测试、验证正确性；需要 bash 执行权限 |
| `general` | 全部工具 | Sonnet | 50 | 通用实现、规划、多步骤任务；兜底所有类型 |

> **plan 折入 general**：planning 是 coordinator 的本职工作；需要 sub-agent 做规划时，spawn `general` 并在 prompt 中说明"只分析不修改"即可，无需独立类型。
>
> **compact 不进队列**：GlobalCompact 是 `History.Prepare()` 的同步子流程，主 agent 必须等它完成才能继续推理，外包给 worker 只会增加延迟，保持在进程内运行。

### Worker 生命周期模型

Worker 全部是 **Stateless**——每次认领任务都用全新的空白 context，完成后丢弃：

```
worker 进程（常驻，跨 session 抢任务）
  ├── 认领 task-A（tenantA/alice, session-X）→ 全新 context → 完成 → 丢弃
  ├── 认领 task-B（tenantB/bob,   session-Y）→ 全新 context → 完成 → 丢弃
  └── 认领 task-C（tenantA/alice, session-Z）→ 全新 context → 完成 → 丢弃
```

**任务间的上下文传递由 coordinator 负责**，在 spawn 子任务时将必要信息写入 prompt：

```
task.prompt = """
背景：explore 阶段已发现：
  - auth 逻辑在 internal/auth/handler.go
  - JWT 验证在 middleware/jwt.go
  - refresh token 未实现（见 TODO 注释）

现在请实现 refresh token 逻辑。
"""
```

### Coordinator 与 Worker 的工具集差异

两者的本质区别是 ToolRegistry 中是否注册了 **AgentTool**：

```
coordinator 进程
  ToolRegistry: [read, write, bash, glob, grep, memory, ... + AgentTool]
  → 可 spawn 子任务（INSERT sub_agent_tasks）
  → 对简单请求直接执行；对复杂请求自行判断是否拆解

worker 进程（内嵌或外部）
  ToolRegistry: [read, write, bash, glob, grep, memory, ...]（按类型裁剪）
  → 无 AgentTool，物理上无法 spawn 子任务
  → 防止 worker 链式 spawn 导致失控
```

---

## AgentTool 实现

这是 Multi-Agent 的核心接入点，在现有七组件架构上最小化扩展。

### 包结构

```
internal/subagent/
  tool.go           ← AgentTool：工具声明 + 执行（仅 INSERT task，不创建 agent）
  queue.go          ← TaskQueue 接口 + PostgreSQL 实现
  worker.go         ← WorkerPool：认领 + session 检查 + 执行 sub-agent 任务
  registry.go       ← WorkerRegistry：心跳注册 + 存活感知
  session.go        ← sub-agent SessionID 命名约定
  brain_cache.go    ← Worker 专用 brainCache（TTL 驱逐，独立于 coordinator）
```

### AgentTool 工具定义

AgentTool 做且只做一件事：**向任务队列投递一个待执行的 sub-agent 任务**，立即返回 `task_id`。真正创建并运行 sub-agent 的是 Worker（内嵌或外部）。

```go
// AgentTool 的 JSON Schema（工具声明）
{
  "name": "dispatch_agent_task",
  "description": "向任务队列投递一个子任务，由专用 Worker 异步执行。返回 task_id，结果在后续轮次通过 <task-notification> 自动注入。适合可并行的独立子任务：代码探索、测试验证、文件分析等。",
  "input_schema": {
    "type": "object",
    "properties": {
      "agent_type": {
        "type": "string",
        "enum": ["explore", "verify", "general"],
        "description": "explore=只读搜索(Haiku)，verify=运行测试(Sonnet)，general=通用执行(Sonnet)"
      },
      "prompt": {
        "type": "string",
        "description": "子任务的完整上下文和指令，必须自包含（Worker 没有主对话历史）"
      },
      "effort": {
        "type": "string",
        "enum": ["low", "medium", "high"],
        "description": "可选，覆盖默认 effort。explore 默认 low，verify/general 默认 high"
      }
    },
    "required": ["agent_type", "prompt"]
  }
}
```

AgentTool 的 `Execute()` 实现：

```go
func (t *AgentTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
    var params AgentToolParams
    json.Unmarshal(input, &params)

    taskID := ulid.Make().String()
    err := t.queue.Insert(ctx, Task{
        TaskID:    taskID,
        SessionID: t.sessionID,
        UserID:    t.userID,
        AgentType: params.AgentType,
        Prompt:    params.Prompt,
        AgentConfig: AgentConfig{
            Effort: params.Effort,
        },
    })

    // 立即返回，不等待 Worker 执行
    return `{"task_id":"` + taskID + `","status":"pending"}`, err
}
```

### Sub-Agent SessionID 命名规范

每个 sub-agent 任务创建独立的 sessionID，格式固定：

```
sub-{parentSessionID}-{taskID}
```

例：`sub-01HZXYZ-01JA001`

这样设计的理由：
- Session Store 层面天然关联父子关系
- 通过前缀查询可拿到主 session 下所有 sub-agent 的历史，便于 Observability
- Sub-agent session 的生命周期与任务绑定，任务完成后可独立清理

### OrchestratorHarness：外层协调循环

```
OrchestratorHarness.Run(sessionID, userText):

  defer: store.TerminateSession(sessionID)          ← 任何退出路径都标记 session 终止
         taskQueue.CancelPendingBySession(sessionID) ← 取消该 session 下所有 pending 任务

  外层循环:
    1. 注入待通知的已完成子任务（drainCompleted → <task-notification>）
    2. 执行标准 Harness.Run()（Brain Turn）
    3. 检查本 session 是否有 pending 子任务
       ├── 有 pending → 等待（LISTEN/NOTIFY 或轮询），回到步骤 1
       └── 无 pending → 外层循环结束，返回给用户
```

**Pending 子任务等待机制**：
- **LISTEN/NOTIFY**（推荐）：Worker 完成任务时 `pg_notify('task_done_{sessionID}', taskID)`，OrchestratorHarness 阻塞等待
- **轮询降级**：每 500ms 查一次 `sub_agent_tasks WHERE session_id=? AND status!=pending`，设 30s 超时

**超时与失败处理**：子任务超时（默认 5 分钟）或失败时，以 `<task-notification status="failed">` 注入，coordinator Brain 自行决定是否重试或降级。

### <task-notification> 结果注入格式

```xml
<task-notifications>
  <task id="01JA001" type="explore" status="completed" tokens="1240">
  internal/auth/handler.go 包含 JWT 验证逻辑（第 45-89 行）。
  middleware/jwt.go 定义 Claims struct，refresh token 字段存在但未实现（TODO 注释在第 23 行）。
  </task>
  <task id="01JA002" type="verify" status="failed" error="测试超时">
  bash 执行 go test ./... 超时（300s），最后输出：TestAuth PASS，TestRefresh panic: nil pointer
  </task>
  <pending count="1" ids="01JA003"/>
</task-notifications>
```

设计要点：
- `result` 存储 sub-agent **最后一条 assistant 文本消息**，不含工具调用历史
- `<pending count="N">` 告知 Brain 还有几个任务未完成，Brain 可据此决定是否等待
- 单条 result 超过 4000 tokens 时自动截断，追加 `[truncated, full result in sub-session: sub-{...}]`

### 沙箱复用机制

同一用户的 coordinator 和所有 sub-agent 共享同一个沙箱：

```
coordinator 启动时：
  DockerSandbox.Provision() → 写入 user_sandboxes{user_id, endpoint, sandbox_id}

worker 认领任务后：
  1. 查询 user_sandboxes WHERE user_id = task.user_id
  2. 若存在 → 构造 RemoteSandbox(endpoint)，复用沙箱
  3. 若不存在 → 自行 Provision 并注册
```

`internal/hands/remote/` 实现 `RemoteSandbox` driver：通过 HTTP 代理到已有沙箱的 tool-server，不重新 provision 容器。

---

## Worker 设计

### 内嵌兜底 Worker

Coordinator 进程内嵌一个 WorkerPool goroutine，**始终存在，无需额外部署**：

```
coordinator 进程
  ├── HTTP server（SSE 长连接）
  ├── OrchestratorHarness（协调循环）
  └── 内嵌 WorkerPool goroutine
        concurrency = 2（不抢占主进程资源）
        types = [explore, verify, general]  ← 全部类型，不限 general
```

外部 Worker 进程存在时，通过 `FOR UPDATE SKIP LOCKED` 公平竞争，专用 Worker 响应更快自然优先拿到任务。

| 层 | 部署要求 | 职责 |
|---|---|---|
| 内嵌 Worker（coordinator 进程内）| 始终存在 | 兜底执行，保证系统不卡死 |
| 外部 Worker 进程 | 可选，按需部署 | 独立扩缩 + 故障隔离 |

> 外部 Worker 的价值在于**独立扩缩和隔离**，不在于独占模型配置——内嵌 Worker 同样能跑 Haiku（模型由 task.agent_config 决定，不由 Worker 类型决定）。

### Worker 认领后：Session 存活检查

Worker 认领任务（`FOR UPDATE SKIP LOCKED`，状态已改为 `running`）后，立即检查 coordinator session 是否还活着：

```
Worker 认领 task
  └── 查 session_status WHERE session_id = task.session_id
        ├── status = 'active'    → 正常执行
        └── status = 'terminated' → 标记 task 为 cancelled，跳过执行
```

OrchestratorHarness 在任何退出路径（正常结束、用户断开、超时）都通过 `defer` 执行：

```go
defer func() {
    store.TerminateSession(ctx, sessionID)
    taskQueue.CancelPendingBySession(ctx, sessionID)
}()
```

这样可以避免 coordinator 已断开后 Worker 还在浪费 LLM token 执行孤儿任务。

### Worker 的 Brain Cache

Worker 无论是内嵌还是独立进程，**始终独立维护自己的 brainCache**，不与 coordinator 共享。原因是 Worker 的部署形态不固定——共享 brainCache 会让两种部署模式的代码路径不一致，带来不必要的复杂度。

Worker brainCache 使用 **TTL 驱逐**（而非 coordinator 的 ref-count 释放）：

```
Worker 认领 task
  └── Resolver.Resolve(tenantID, userID) → fingerprint
  └── brainCache.getOrCreate(fingerprint)  ← 命中则复用（MCP 连接不重建），未命中则新建
  └── 执行任务
  └── brainCache.markIdle(fingerprint)     ← 不释放，更新 lastUsed 时间戳

空闲超过 TTL（默认 5min）
  └── brainCache.evict(fingerprint)        ← 关闭 MCP 连接，释放资源
```

同一 Worker 进程内，同租户的顺序任务命中相同 fingerprint，直接复用 Brain（MCP 连接不重建）。

### Resolver 集成：四层优先级

Worker 执行 sub-agent 时，模型和工具配置按四层优先级解析（从低到高）：

```
global config（全局默认）
  → tenant config（租户覆盖：MCP、Skill、模型）
    → user config（用户个人偏好）
      → task.agent_config（spawn 时指定，最高优先级）
```

`task.agent_config` 是关键：coordinator 在调用 AgentTool 时指定，Worker 执行时优先使用。这确保 explore 任务始终用 Haiku + effort:low，即使租户默认配置是 Sonnet。

**MCP 和 Skill 是租户级别的**，通过 `Resolver.Resolve()` 动态解析，Worker 本身没有固定业务配置：

```
task.user_id = "acme/alice"
  → tenantID = "acme"
  → Resolver.Resolve(ctx, "acme", "alice", tenantSettings)
  → EffectiveConfig{
      MCPs:     acme 租户配置的 MCP servers,   ← 租户级
      Skills:   acme 租户配置的 skills,        ← 租户级
      BrainCfg: 被 task.agent_config 覆盖后的模型配置
    }
```

---

## 方案一：多进程同节点

### 适用场景

- Phase 4 起步阶段，功能完整但运维简单
- 中等规模（< 100 并发用户）
- 单台物理机或 VM，不依赖 K8s

### 架构

```
同一台机器
  ├── forge-coordinator 进程（1 个，持有 SSE 长连接）
  │     ToolRegistry: 全部工具 + AgentTool
  │     内嵌 WorkerPool: concurrency=2, types=[explore, verify, general]
  │
  ├── forge-worker-explore 进程（N 个，可选）
  │     ToolRegistry: Read、Glob、Grep
  │
  ├── forge-worker-verify 进程（M 个，可选）
  │     ToolRegistry: Bash、Read、Glob、Grep
  │
  └── forge-worker-general 进程（K 个，可选，提升并发上限）
        ToolRegistry: 全部工具（无 AgentTool）

共享依赖：
  PostgreSQL（Session Store + 任务队列 + Worker 注册表 + 沙箱注册表）
  DockerSandbox（每用户一个容器，通过 RemoteSandbox 共享）
```

### 启动方式

```bash
# Coordinator（含内嵌 Worker，单进程即可工作）
forge serve --role=coordinator --addr=:8080

# 可选：独立 Worker 进程，提升并发和专用化
forge serve --role=worker --types=explore --concurrency=5
forge serve --role=worker --types=verify  --concurrency=3
forge serve --role=worker --types=general --concurrency=3
```

### 进程配置结构

```yaml
# coordinator 启动配置
role: coordinator
subagent:
  enabled: true
  task_timeout: 5m
  per_user_limit: 3        # 单用户最多 3 个并发 sub-agent
  builtin_worker:
    concurrency: 2         # 内嵌 Worker 并发数
    types: [explore, verify, general]

---
# 外部 worker（可选）
role: worker
worker:
  agent_types: [explore]
  concurrency: 5
```

### Worker Pool 核心（LISTEN/NOTIFY 驱动）

```go
func (p *WorkerPool) Start(ctx context.Context) {
    conn.Exec(ctx, "LISTEN new_task")
    go p.heartbeat(ctx)  // 每 15s 更新 worker_registry.last_seen_at

    globalSem := semaphore.NewWeighted(p.concurrency)
    userSems  := sync.Map{}

    for {
        conn.WaitForNotification(ctx)

        globalSem.Acquire(ctx, 1)
        go func() {
            defer globalSem.Release(1)

            task := p.queue.Claim(ctx, p.agentTypes)
            if task == nil { return }

            // per-user 限流
            sem := p.userSem(task.UserID)
            if !sem.TryAcquire(1) {
                p.queue.Unclaim(ctx, task.TaskID)
                return
            }
            defer sem.Release(1)

            // session 存活检查
            if !p.sessionStore.IsActive(ctx, task.SessionID) {
                p.queue.Cancel(ctx, task.TaskID)
                return
            }

            p.executeTask(ctx, task)
        }()
    }
}
```

---

## 方案二：K8s 多 Deployment

### 适用场景

- 大规模多租户（100+ 并发用户）
- 需要独立扩缩不同类型 Worker
- 需要故障隔离（Worker 崩溃不影响 Coordinator）

### 架构

```
                      用户
                       │
                  Load Balancer
                  (sticky session)
                       │
      ┌────────────────┼────────────────┐
      │                │                │
coordinator-0    coordinator-1    coordinator-2..4
（各含内嵌 Worker，standalone 也能工作）
      │
      └──────── PostgreSQL ─────────────┘
                    │
      ┌─────────────┼──────────────────────┐
      │             │                      │
explore-worker  verify-worker       general-worker
×2~50 (可选)    ×2~20 (可选)       ×2~10 (可选)
KEDA 自动扩缩   KEDA 自动扩缩      KEDA 自动扩缩
      │             │                      │
      └─────────────┴──────────────────────┘
                    │ RemoteSandbox（HTTP /execute）
            forge-sandboxes namespace
            每用户一个 tool-server Pod
```

### Coordinator Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: forge-coordinator
  namespace: forge-system
spec:
  replicas: 5
  template:
    spec:
      containers:
      - name: forge
        image: forge:latest
        args: ["serve", "--role=coordinator", "--addr=:8080"]
        resources:
          requests: { memory: "512Mi", cpu: "500m" }
          limits:   { memory: "1Gi",   cpu: "2" }
---
apiVersion: v1
kind: Service
metadata:
  name: forge-coordinator
  namespace: forge-system
spec:
  sessionAffinity: ClientIP
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 3600
  ports:
  - port: 80
    targetPort: 8080
```

### Worker Deployment（以 explore-worker 为例）

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: forge-explore-worker
  namespace: forge-system
spec:
  replicas: 10
  template:
    spec:
      containers:
      - name: forge
        image: forge:latest
        args: ["serve", "--role=worker", "--types=explore"]
        resources:
          requests: { memory: "256Mi", cpu: "250m" }
          limits:   { memory: "512Mi", cpu: "1" }
```

### KEDA 自动扩缩

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: explore-worker-scaler
  namespace: forge-system
spec:
  scaleTargetRef:
    name: forge-explore-worker
  minReplicaCount: 0        # 无任务时缩到 0，coordinator 内嵌 Worker 兜底
  maxReplicaCount: 50
  cooldownPeriod: 60
  triggers:
  - type: postgresql
    metadata:
      query: >
        SELECT COUNT(*) FROM sub_agent_tasks
        WHERE status = 'pending' AND agent_type = 'explore'
      targetQueryValue: "5"
```

### K8s 原生沙箱：KubernetesSandbox Driver

```go
// internal/hands/kubernetes/kubernetes.go
func (s *KubernetesSandbox) Provision(ctx context.Context, res resources.Resources) error {
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "sandbox-" + sanitize(s.userID),
            Namespace: "forge-sandboxes",
            Labels:    map[string]string{"forge/user": s.userID},
        },
        Spec: corev1.PodSpec{
            Containers: []corev1.Container{{
                Name:  "tool-server",
                Image: "forge:latest",
                Args:  []string{"tool-server", "--workspace", res.WorkspaceRoot},
                Ports: []corev1.ContainerPort{{ContainerPort: 7777}},
                Resources: corev1.ResourceRequirements{
                    Limits: corev1.ResourceList{
                        corev1.ResourceCPU:    resource.MustParse("2"),
                        corev1.ResourceMemory: resource.MustParse("2Gi"),
                    },
                },
            }},
            Volumes: []corev1.Volume{{
                Name: "workspace",
                VolumeSource: corev1.VolumeSource{
                    PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
                        ClaimName: "workspace-" + sanitize(s.userID),
                    },
                },
            }},
        },
    }
    created, _ := s.client.CoreV1().Pods("forge-sandboxes").Create(ctx, pod, metav1.CreateOptions{})
    s.podIP, _ = s.waitReady(ctx)
    return s.registry.Register(ctx, s.userID, "http://"+s.podIP+":7777", created.Name)
}
```

### 工作区共享：PVC 方案

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: workspace-{userID}
  namespace: forge-sandboxes
spec:
  accessModes: [ReadWriteMany]
  storageClassName: efs
  resources:
    requests:
      storage: 20Gi
```

### 命名空间隔离与 RBAC

```
forge-system      ← Coordinator Pod、Worker Pod
forge-sandboxes   ← Sandbox Pod（每用户一个 tool-server Pod）
```

```yaml
# Agent ServiceAccount 最小权限
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: forge-sandboxes
rules:
- apiGroups: [""]
  resources: ["pods", "pods/status"]
  verbs: ["create", "delete", "get", "watch", "list"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["create", "delete", "get"]
```

```yaml
# 沙箱网络隔离
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  namespace: forge-sandboxes
spec:
  podSelector: {}
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: forge-system
  egress: []
```

### Helm Chart 统一管理

```yaml
# values.yaml
roles:
  coordinator:
    replicas: 5
    args: ["serve", "--role=coordinator"]
    service: true
    resources: { memory: "1Gi", cpu: "2" }

  explore-worker:
    args: ["serve", "--role=worker", "--types=explore"]
    service: false
    resources: { memory: "512Mi", cpu: "1" }
    keda:
      query: "SELECT COUNT(*) FROM sub_agent_tasks WHERE status='pending' AND agent_type='explore'"
      targetValue: "5"
      min: 0          # coordinator 内嵌 Worker 兜底，可以缩到 0
      max: 50

  verify-worker:
    args: ["serve", "--role=worker", "--types=verify"]
    service: false
    keda:
      query: "SELECT COUNT(*) FROM sub_agent_tasks WHERE status='pending' AND agent_type='verify'"
      targetValue: "3"
      min: 0
      max: 20

  general-worker:
    args: ["serve", "--role=worker", "--types=general"]
    service: false
    keda:
      query: >
        SELECT COUNT(*) FROM sub_agent_tasks
        WHERE status = 'pending'
          AND (agent_type = 'general'
            OR created_at < NOW() - INTERVAL '10 seconds')
      targetValue: "3"
      min: 0          # coordinator 内嵌 Worker 兜底，可以缩到 0
      max: 10
```

---

## 优雅降级

有内嵌 Worker 兜底后，降级路径更简单：

```
任何类型任务创建
    │
    ▼
外部专用 Worker 在线？──是──▶ 立即认领，专用模型/工具集
    │否
    ▼
内嵌 Worker（coordinator 进程内）──▶ 认领并执行，Sonnet 兜底
    （任务不会卡死）
```

**三类 Worker 的认领 SQL**（外部 Worker 间的优先级兜底仍然保留）：

```sql
-- explore-worker：只抢 explore
SELECT * FROM sub_agent_tasks
WHERE status = 'pending' AND agent_type = 'explore'
FOR UPDATE SKIP LOCKED LIMIT 1;

-- verify-worker：优先抢 verify，10s 后兜底 general
SELECT * FROM sub_agent_tasks
WHERE status = 'pending'
  AND (agent_type = 'verify'
    OR (agent_type = 'general' AND created_at < NOW() - INTERVAL '10 seconds'))
ORDER BY CASE agent_type WHEN 'verify' THEN 0 ELSE 1 END, created_at
FOR UPDATE SKIP LOCKED LIMIT 1;

-- general-worker 和内嵌 Worker：抢 general，10s 后兜底 verify，30s 后兜底 explore
SELECT * FROM sub_agent_tasks
WHERE status = 'pending'
  AND (agent_type = 'general'
    OR (agent_type = 'verify'  AND created_at < NOW() - INTERVAL '10 seconds')
    OR (agent_type = 'explore' AND created_at < NOW() - INTERVAL '30 seconds'))
ORDER BY
  CASE agent_type WHEN 'general' THEN 0 WHEN 'verify' THEN 1 ELSE 2 END, created_at
FOR UPDATE SKIP LOCKED LIMIT 1;
```

---

## 存储方案

```sql
-- 任务队列
CREATE TABLE sub_agent_tasks (
    task_id        TEXT PRIMARY KEY,
    session_id     TEXT NOT NULL,
    sub_session_id TEXT,                       -- sub-{parentSessionID}-{taskID}
    user_id        TEXT NOT NULL,              -- tenantID/username
    agent_type     TEXT NOT NULL,
    prompt         TEXT NOT NULL,
    agent_config   JSONB NOT NULL DEFAULT '{}',-- {"effort": "low"}（模型由 Resolver 按 agent_type 决定）
    status         TEXT NOT NULL DEFAULT 'pending', -- pending|running|completed|failed|cancelled
    worker_id      TEXT,
    result         TEXT,                       -- sub-agent 最后一条 assistant 文本消息
    result_tokens  INT,
    error          TEXT,
    notified       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at     TIMESTAMPTZ,
    completed_at   TIMESTAMPTZ
);
CREATE INDEX ON sub_agent_tasks (created_at) WHERE status = 'pending';
CREATE INDEX ON sub_agent_tasks (session_id, notified) WHERE notified = FALSE;

-- Session 状态表（Worker 检查 coordinator session 是否还活着）
CREATE TABLE session_status (
    session_id    TEXT PRIMARY KEY,
    status        TEXT NOT NULL DEFAULT 'active', -- active | terminated
    terminated_at TIMESTAMPTZ
);

-- Worker 注册表
CREATE TABLE worker_registry (
    worker_id    TEXT PRIMARY KEY,
    agent_types  TEXT[] NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 沙箱注册表
CREATE TABLE user_sandboxes (
    user_id      TEXT PRIMARY KEY,
    sandbox_type TEXT NOT NULL,   -- "docker" | "kubernetes"
    endpoint     TEXT NOT NULL,   -- http://{ip}:7777
    sandbox_id   TEXT NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 实时通知：LISTEN/NOTIFY

```sql
-- 写入任务 → 唤醒 Worker
CREATE FUNCTION notify_new_task() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('new_task', NEW.task_id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_new_task
AFTER INSERT ON sub_agent_tasks
FOR EACH ROW EXECUTE FUNCTION notify_new_task();

-- 任务完成 → 唤醒对应 coordinator 的 OrchestratorHarness
CREATE FUNCTION notify_task_done() RETURNS trigger AS $$
BEGIN
    IF NEW.status IN ('completed', 'failed', 'cancelled') AND OLD.status = 'running' THEN
        PERFORM pg_notify('task_done_' || NEW.session_id, NEW.task_id);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_task_done
AFTER UPDATE ON sub_agent_tasks
FOR EACH ROW EXECUTE FUNCTION notify_task_done();
```

---

## 演进路径

```
当前（Phase 2 完成）
  forge serve（单进程，单 Agent，无 sub-agent）

  ↓ Phase 4：实现 AgentTool + OrchestratorHarness + WorkerPool

方案一：多进程同节点（Phase 4 起步）
  forge serve --role=coordinator（含内嵌 Worker，单进程即可完整工作）
  forge serve --role=worker      （可选，提升并发）
  PostgreSQL 任务队列 + session_status 表
  DockerSandbox + RemoteSandbox

  ↓ 流量增长，需要多节点高可用

方案二：K8s 多 Deployment（Phase 5）
  coordinator Deployment × 5（sticky session，各含内嵌 Worker）
  worker Deployment × N（KEDA 自动扩缩，min=0）
  KubernetesSandbox driver
  PVC 共享工作区（EFS/NFS）
```

每步只改：
- **Phase 4**：新增 `internal/subagent/` 包；`main.go` 增加 `--role` flag；`config.go` 增加 `Subagent`、`Worker` 配置；Session Store 增加 `session_status` 表
- **Phase 5**：Sandbox driver 换 `kubernetes`；添加 K8s YAML 和 KEDA ScaledObject

AgentTool 接口和 PostgreSQL 队列协议全程不变。

---

## 与现有代码的接入点

| 现有文件 | 改动内容 |
|---------|---------|
| `cmd/forge/main.go` | 增加 `--role` flag；coordinator 启动时同时启动内嵌 WorkerPool goroutine |
| `internal/harness/harness.go` | 增加 `RunOnce(ctx, sessionID, prompt) (string, error)` 供 Worker 调用 |
| `internal/orchestration/http_session.go` | coordinator 模式下使用 OrchestratorHarness 替代标准 Harness |
| `internal/resolver/resolver.go` | `Resolve()` 增加第四层：task `agent_config` 的 effort 覆盖 |
| `internal/hands/docker/docker.go` | `Provision()` 写入 `user_sandboxes` 表 |
| `internal/hands/remote/` | 新增 RemoteSandbox driver |
| `internal/hands/kubernetes/` | 新增 KubernetesSandbox driver（方案二用）|
| `internal/subagent/` | 新增包：AgentTool、TaskQueue、WorkerPool、WorkerRegistry、Worker brainCache（TTL） |
| `internal/config/config.go` | 增加 `Role`、`Worker`、`Subagent{Enabled, TaskTimeout, PerUserLimit, BuiltinWorker}` |
| `internal/gateway/store/sqlite/sqlite.go` | 新增 `sub_agent_tasks`、`session_status`、`worker_registry`、`user_sandboxes` 表迁移 |
