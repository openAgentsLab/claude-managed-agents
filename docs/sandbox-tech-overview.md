# 沙箱实现技术概览

> 本文介绍 ai-forge 沙箱系统的设计思路与实现方式，从 Anthropic 官方的设计哲学出发，逐步讲解 ai-forge 如何在此基础上构建出面向终端用户的完整沙箱体系。

---

## 一、Anthropic 官方沙箱设计

### 1.1 核心理念：执行与持久化分离

Anthropic 官方 Managed Agents 的沙箱设计，建立在一个核心原则上：**将"执行"和"持久化"彻底解耦**。

- **执行层（容器）**：短生命周期，空闲即销毁，按需重建。
- **持久层（存储）**：长生命周期，绑定 session，session 终止才删除。
- **资源层（Resources）**：可在 session 运行中动态挂载和卸载。

用户感知到的"连续性"来自持久层，而不是容器一直活着。这使得系统可以在不影响用户体验的前提下，随时回收空闲容器资源。

```
┌─────────────────────────────────────────┐
│  Session（逻辑层）                        │
│  生命周期：天/周级别                        │
│  包含：对话历史、资源声明、状态机              │
│                                         │
│   ┌─────────────────────────────────┐   │
│   │  Container（执行层）              │   │
│   │  生命周期：active window          │   │
│   │  idle 4.5 分钟后销毁              │   │
│   │                                 │   │
│   │   ┌─────────────────────────┐   │   │
│   │   │  PVC / Volume（存储层）   │   │   │
│   │   │  生命周期 = Session       │   │   │
│   │   │  容器重建后重新挂载         │   │   │
│   │   └─────────────────────────┘   │   │
│   └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### 1.2 核心概念

**Environment（环境模板）**

类似"容器镜像 + 启动配置"的模板。声明需要预装的包（pip / npm / apt / cargo），并定义网络访问策略（`unrestricted` / `limited`）。Environment 是模板，不是运行实例，多个 session 可共享同一 Environment 定义。

**Session（会话）**

一次完整对话任务的上下文容器，绑定 Agent 配置、Environment 模板和资源列表，是状态机（idle / running / terminated），可存活 30 天。

**Resources（资源）**

挂载到容器中的外部数据：
- **Files**：上传文件，以只读副本形式挂载到容器内指定路径
- **GitHub**：指定仓库 URL + branch，容器启动时自动 clone，token 安全注入不回显
- **Memory Store**：跨 session 持久化记忆，Agent 自动获得读写能力

**动态资源管理**

Session 运行期间支持 `resources.add` 和 `resources.delete`，实现运行时资源挂载与卸载。

### 1.3 官方能力总览

| 能力 | 说明 |
|------|------|
| 容器 per session | 每个 session 独立容器，完全隔离 |
| workspace 持久化 | 存储卷绑定 session，容器重建不丢数据 |
| 容器 idle 自动回收 | 空闲超时销毁容器，节省资源 |
| 文件资源挂载 | 只读挂载，动态 add/remove |
| Git 仓库挂载 | 启动时 clone，token 安全注入 |
| 跨 session 记忆 | Memory Store，Agent 自动检索 |
| 包预装缓存 | Environment packages，跨 session 复用 |
| 网络隔离 | unrestricted / limited 两档 |
| 输出收集 | `/mnt/session/outputs/` + Files API 取回 |

---

## 二、ai-forge 沙箱架构

### 2.1 整体架构

ai-forge 的沙箱系统由五个核心层次构成：

```
Orchestration（触发层）
     │  组装 Resources，传入 session 上下文
     ▼
   Pool（生命周期管理层）
     │  Acquire(sessionID) → Sandbox
     │  ReleaseSession(sessionID)
     ├── LocalPool     （本地进程内，无隔离，开发用）
     ├── DockerPool    （per-session 容器，Docker 部署）
     └── K8sWatchPool  （per-session Pod，K8s 部署）
     │
     ▼
  Sandbox（执行层）
     │  Provision(tools) / Execute(name, input)
     ├── LocalSandbox  （进程内直接调用）
     └── RemoteSandbox （HTTP 转发到 tool-server）
     │
     ▼
  ToolRegistry（工具代理层）
     │  统一分发：沙箱工具 → Sandbox.Execute
     │            直连工具 → 进程内直接执行
     ▼
  Resources（工作区上下文）
     WorkspaceRoot / Mounts / FileResource / GitResource / Environment
```

这个分层设计有一个重要特性：**Brain（推理层）对沙箱实现完全无感知**。Brain 只看到 ToolRegistry 暴露的工具列表，不知道工具最终在哪里执行。

### 2.2 Pool 接口：统一的生命周期管理

Pool 是整个沙箱系统的核心抽象，三种实现（Local / Docker / K8s）共享同一接口：

```
Pool
 ├── Acquire(sessionID)   → 返回该 session 的 Sandbox（不存在则创建）
 ├── ReleaseSession(sessionID) → 销毁容器 + 存储（best-effort）
 ├── Isolated()           → 报告是否提供执行隔离
 ├── StartBackground(ctx) → 启动心跳/清理/Watch 等后台任务
 └── CloseAll()           → 关闭所有活跃沙箱
```

`Acquire` 是幂等的：同一个 sessionID 多次调用，只要容器健康，返回缓存实例，不重复创建。这是多轮对话中保持容器复用的关键。

### 2.3 Driver 注册模式

三种 Pool 实现通过 Driver 注册模式接入，主程序只需 blank import 对应包，无需感知具体实现：

```
import _ "ai-forge/internal/hands/docker"   // 触发 init()，注册 DockerPool
import _ "ai-forge/internal/hands/k8s"      // 触发 init()，注册 K8sWatchPool
```

配置文件中的 `sandbox.driver` 字段决定运行时使用哪个 Pool，其余代码完全不变。

---

## 三、Environment：多层环境配置

### 3.1 四层继承模型

Environment 不是单一的配置对象，而是从四个层次合并而来，越靠上优先级越高：

```
┌─────────────────────────────────────────────────┐
│  Session 创建时的覆盖（inline environment）         │  ← 最高优先级
├─────────────────────────────────────────────────┤
│  Project 绑定的 Environment（project_id 引用）    │
├─────────────────────────────────────────────────┤
│  用户默认 Environment（user settings）            │
├─────────────────────────────────────────────────┤
│  租户默认 Environment（tenant settings）          │  ← 兜底
└─────────────────────────────────────────────────┘
```

合并规则有三条：

1. **Packages 取并集**：各层的包列表合并，自动去重，保留插入顺序
2. **Networking 取最严格**：只要有一层设置了 `limited`，结果就是 `limited`；AllowedHosts 取并集
3. **Env 变量后写覆盖**：高优先级层的同名变量覆盖低优先级层

### 3.2 Environment 作为独立实体

Environment 不只是内嵌配置，而是可命名、可复用的独立资源，支持租户级和用户级两种 scope：

```
租户 Environment（IT 管理员维护）
  ├── "Go 后端标准环境"   → packages: [go, golangci-lint, ...]
  ├── "Python 数据科学"  → packages: [numpy, pandas, jupyter, ...]
  └── "Node.js Web"     → packages: [node, yarn, ...]

用户 Environment（个人偏好）
  └── "个人工具集"        → packages: [ripgrep, jq, fzf, ...]
```

Project 创建时选择一个 Environment（存储 `environment_id` 引用，不是内嵌配置）。Environment 更新后，所有引用它的 Project 在下次创建 Session 时自动生效。

### 3.3 packages 安装与缓存

包的安装结果**写入持久卷，而非容器镜像层**。这样容器 idle 销毁重建后不需要重新安装：

```
容器启动流程
     │
     ├─ 1. 读取 packages 配置，计算哈希 H_new
     │
     ├─ 2. 读取 .forge-env/version 中记录的 H_old
     │
     ├─ 3. H_new == H_old？
     │         ├── 是 → 跳过安装（幂等）
     │         └── 否 → 执行 pip/npm/apt install
     │                   写入 .forge-env/ 子目录
     │                   更新 .forge-env/version = H_new
     │
     └─ 4. 设置 PYTHONPATH / NODE_PATH 指向安装目录
```

在 K8s 中，包安装通过 init container 执行，在主容器就绪之前完成。

---

## 四、动态资源加载

### 4.1 资源的三种类型

ai-forge 的 Resources 层定义了三种可挂载的外部数据：

| 资源类型 | 主要字段 | 容器内效果 |
|---------|---------|-----------|
| FileResource | ID、TargetPath、Content | 文件写入 workspace 指定路径 |
| GitResource | ID、URL、Branch、TargetPath、Token | 仓库 clone 到 workspace 子目录 |
| Environment | Packages、Networking、Env | 包安装 + 网络策略 + 环境变量 |

### 4.2 动态挂载的本质

容器和 Pod 均不支持运行时追加新的 volume mount。因此 ai-forge 的动态挂载本质是**向已挂载的卷写文件**：

```
动态挂载 FileResource
         │
         ├─ 共享存储可用时：
         │     Orchestration 直接写入 volumes_root/{sessionHash}/{targetPath}
         │     容器通过 bind mount 立即可见（无需容器存活）
         │
         └─ 共享存储不可用时：
               API 返回 503（降级模式）
```

Git 仓库挂载的安全提升：在共享存储可用时，git clone 在 Orchestration 侧执行，完成后只把目录挂载给容器。Token 不进入容器 env，满足凭证隔离要求。

### 4.3 资源声明持久化

动态 add/remove 的资源列表持久化到 `session_resources` 表。容器每次重建时，扫描该表，对每项资源检测目标路径是否已存在，不存在才执行初始化，保证幂等：

```
容器重建
   │
   └─ 读取 session_resources 表
         │
         ├─ FileResource: targetPath 不存在 → 重新写入
         ├─ GitResource:  targetPath 不存在 → 重新 clone
         └─ 已存在 → 跳过（幂等）
```

---

## 五、两种部署实现

### 5.1 Docker 部署

**存储策略**

Docker 部署使用 bind mount 实现 workspace 持久化，依赖基础设施层预先挂载的共享网络存储（NFS 等）：

```
共享存储（NFS / 网络存储）
  └── volumes_root/
        └── {tenantID}/
              └── {sessionHash}/
                    ├── workspace/         → 挂载为容器 /workspace
                    ├── .forge-env/        → 包安装缓存
                    └── outputs/           → 输出收集目录
```

forge 本身不管理 NFS 连接，只需要知道共享路径在哪里（`sandbox.volumes_root` 配置项）。

**两层生命周期**

```
存储目录（volumes_root 子目录）
  生命周期 = session
  容器销毁时目录保留，session 终止时才删除

容器
  生命周期 = active window
  idle 超时后停止删除容器
  下次消息到来时，重建容器并 bind mount 同一存储目录
  workspace 文件完好
```

**后台任务**

| 任务 | 周期 | 职责 |
|------|------|------|
| heartbeat | 30s | 健康检查，更新 last_seen |
| idle cleanup | 1min | 删除超时容器（默认 5min 无活动） |
| stale cleanup | 1min | 清理 DB 中孤立记录（默认 10min） |

**降级模式**

服务启动时对 `volumes_root` 执行写可达性探测。探测失败则进入降级模式，部分功能禁用：

| 功能 | 共享存储可用 | 降级模式 |
|------|------------|---------|
| workspace 跨重建持久化 | ✅ | ⚠️ 不保证（同节点可能保留） |
| packages 跨重建复用 | ✅ | ⚠️ 不保证 |
| Git clone 不进容器 env | ✅ | ❌ 退化为容器内 clone |
| 动态资源挂载 | ✅ | ❌ 返回 503 |
| Outputs 收集 API | ✅ | ❌ 返回 503 |

能力状态通过 `GET /api/v1/capabilities` 对外暴露，前端据此显示或隐藏相关功能入口。

### 5.2 K8s 部署

**三层资源，两种生命周期**

```
PVC（workspace 卷）
  名称：forge-ws-{sessionHash}
  生命周期 = session（session 终止时删除）

Service（ClusterIP）
  生命周期 = session
  提供稳定的集群内 DNS 地址
  Pod 重建后 Service 不变，外部访问地址不变

Pod
  生命周期 = active window
  RestartPolicy: Never（不自动重启）
  idle 超时后直接删 Pod，下次重建时挂载同一 PVC
```

用 Pod 替代 Deployment 是 K8s 侧的关键设计决策——Deployment controller 会自动重启被删除的 Pod，idle timeout 机制无法生效。

**Init Container 初始化**

```
Pod 启动
  ├─ Init Container 运行 sandbox-init
  │     ├─ 检查 .forge-env/version 哈希
  │     ├─ 安装/跳过 packages（写入 PVC）
  │     └─ Clone git 资源（写入 PVC）
  │
  └─ Main Container 启动 tool-server
        └─ source .forge-env/env.sh（加载已安装的包路径）
```

**两层 idle 清理**

```
Pod 层（idle > 30min，默认）
  → 删除 Pod（保留 Service + PVC）
  → 下次请求到来时重建 Pod，Service 自动关联，PVC 重新挂载

Session 层（idle > 24h，默认）
  → 删除 Pod + Service + PVC
  → 完整释放 session 所有资源
```

**NetworkPolicy**

当 Environment.Networking.Mode == `limited` 时，为该 session 的 Pod 创建 K8s NetworkPolicy，声明出站只允许 DNS + 白名单域名：

```
NetworkPolicy: forge-sandbox-{sessionHash}
  podSelector: forge.io/sandbox-key = <sessionID>
  egress:
    - to: DNS（port 53）
    - to: AllowedHosts 解析后的 CIDR
```

---

## 六、容器空闲清除与重建

### 6.1 Docker：三条后台循环

DockerPool 启动后会同时运行三条独立的后台 goroutine：

| 循环 | 周期 | 职责 |
|------|------|------|
| heartbeat | 30s | 对所有缓存容器并发健康检查；不健康则从内存缓存驱逐并删除容器 |
| idleTimeout | 1min | 扫描 DB 中 `last_seen` 超过 5min 的记录，删除容器，但**保留 DB 记录和 workspace 目录** |
| cleanup | 1min | 删除 `last_seen` 超过 10min 的孤立 DB 记录（进程崩溃未调 ReleaseSession 遗留） |

`last_seen` 在每次 `Acquire` 时通过 `repo.Touch(sessionID)` 更新，heartbeat 不更新它，只做健康判断。

### 6.2 Docker：idle 清除的关键设计

idle timeout 和 heartbeat 驱逐的行为有一个**重要差异**：

```
heartbeat 驱逐（容器不健康）
  → docker rm -f {containerID}
  → 从内存缓存删除
  → 从 DB 删除记录          ← 记录被清掉

idle timeout 清除（容器空闲太久）
  → docker rm -f {containerID}
  → 从内存缓存删除
  → DB 记录保留！            ← 记录保留，用于重建
  → workspace 目录保留！     ← 文件保留，用于重建
```

这个差异是重建能正常工作的前提——idle 清除后，DB 中留有含 `EnvironmentSpec` 的记录，workspace 目录完整，下次 Acquire 时可以透明重建。

### 6.3 Docker：Acquire 重建流程

当用户发送下一条消息，`Acquire(sessionID)` 触发时，Pool 按三层顺序查找：

```
Acquire(sessionID)
   │
   ├─ 1. 内存缓存命中？健康？
   │        └── 是 → 直接返回（Touch last_seen）
   │
   ├─ 2. 内存缓存未命中 → 查 DB
   │        ├── 有记录，EnvironmentSpec 非空
   │        │     └── 反序列化为 storedEnv（包含 packages、networking）
   │        ├── 有 SandboxID，容器仍存活？
   │        │     └── 是 → 重建 RemoteSandbox，写入内存缓存，返回
   │        └── 容器已不存活（被 idle 删除）→ 删除 DB 记录，进入步骤 3
   │
   └─ 3. 启动新容器（携带 storedEnv）
          ├── start(sessionID, token, quota, storedEnv)
          │     └── 容器获得完整的 Environment（packages 列表、网络策略、Env 变量）
          ├── Upsert DB 记录（新 containerID + 原有 EnvironmentSpec）
          └── ensureSessionResources()（补齐缺失的动态资源文件）
```

### 6.4 Environment 如何在重建中复用

这是 packages 不需要重装的核心机制，分为**存储**和**复用**两个阶段：

**阶段一：存储 Environment（Session 创建时）**

```
Session 创建
   │
   ├── 四层合并 → MergedEnvironment（packages + networking + env vars）
   │
   └── Pool.SetSessionEnvironment(sessionID, mergedEnv)
          └── 将 mergedEnv 序列化为 JSON
                存入 DB sandbox 记录的 EnvironmentSpec 字段
```

`EnvironmentSpec` 与容器 ID 存在同一条 DB 记录里。容器被删除时，DB 记录（含 EnvironmentSpec）**刻意保留**。

**阶段二：重建时读取 Environment**

```
Acquire → DB 查询
   │
   ├── rec.EnvironmentSpec != "" → 反序列化为 storedEnv
   │
   └── start(ctx, sessionID, token, quota, storedEnv)
          │
          ├── Docker 容器启动时
          │     设置环境变量 FORGE_PACKAGES_SPEC = storedEnv.Packages（JSON）
          │     设置网络模式 = storedEnv.Networking
          │     传入 Env 变量
          │
          └── 容器内 sandbox-init 执行
                ├── 读取 FORGE_PACKAGES_SPEC
                ├── 计算 packages 哈希 H_new
                ├── 读取 .forge-env/version 中的 H_old
                └── H_new == H_old → 跳过安装（已安装）
                                ← 此处体现出 Environment 复用
```

### 6.5 包安装的分层缓存策略

sandbox-init 对不同包管理器采用不同的缓存策略，原因是安装路径不同：

| 包管理器 | 安装目录 | 是否写入持久卷 | 容器重建后 |
|---------|---------|-------------|-----------|
| pip | `.forge-env/python/` | ✅ | 跳过重装（hash 匹配） |
| npm | `.forge-env/node/` | ✅ | 跳过重装（hash 匹配） |
| cargo | `.forge-env/cargo/` | ✅ | 跳过重装（hash 匹配） |
| apt | 系统路径（`/usr/lib` 等） | ❌（容器 overlay 层） | **每次重建都重装** |

apt 安装的是系统依赖（curl、git、build-essential 等），路径在容器 overlay 层，容器销毁即消失，无法缓存，因此每次重建无条件重跑。哈希检测只覆盖 pip/npm/cargo。

安装完成后，sandbox-init 将各安装目录的路径写入 `.forge-env/env.sh`：

```bash
# .forge-env/env.sh（安装后自动生成）
export PYTHONPATH="/workspace/.forge-env/python${PYTHONPATH:+:$PYTHONPATH}"
export NODE_PATH="/workspace/.forge-env/node/node_modules${NODE_PATH:+:$NODE_PATH}"
export PATH="/workspace/.forge-env/node/.bin:$PATH"
export PATH="/workspace/.forge-env/cargo/bin:$PATH"
```

主容器启动时 `source .forge-env/env.sh`，安装的包对所有工具调用立即可见。

### 6.6 K8s：两层 idle 清除

K8s 的清除逻辑每 5 分钟执行一次，按两个阈值分层处理：

```
cleanupIdlePods()
   │
   ├─ last_seen > sessionIdleTimeout（默认 24h）
   │     → 删 Pod + Service + PVC
   │     → 从内存缓存删除
   │     → 完整释放 session 所有资源
   │
   └─ last_seen > podIdleTimeout（默认 30min）
         → 只删 Pod（保留 Service + PVC）
         → 内存缓存保留（entry.ready = false）
         → 下次 Acquire 时重建 Pod，PVC 直接挂载
```

这种两层设计的意图是：30 分钟的"暂停"很常见（用户离开去开会），不应该清掉所有数据；只有超过 24 小时的深度空闲，才彻底释放存储资源。

### 6.7 K8s：重建中的 Environment 复用

K8s 的重建逻辑比 Docker 更直接——Environment 随 Pod 定义重新传入，不依赖 DB 中的 EnvironmentSpec：

```
K8s Acquire（Pod 不存在时）
   │
   ├─ 从 DB 或内存 cache 读取 storedEnv
   │
   └─ createSandbox(key, token, storedEnv, quota)
          ├─ ensurePVC → PVC 已存在，复用（幂等）
          ├─ ensureService → Service 已存在，复用（幂等）
          └─ ensurePod → 创建新 Pod，携带完整 env 配置
                 │
                 └─ buildPod() 中：
                       ├─ Init Container（forge-init）
                       │     FORGE_PACKAGES_SPEC = storedEnv.Packages（JSON）
                       │     挂载同一 PVC → sandbox-init 读 .forge-env/version
                       │                    hash 匹配 → 跳过安装
                       │
                       └─ Main Container（tool-server）
                             启动命令：source .forge-env/env.sh; exec /forge tool-server
```

由于 PVC 是幂等的（已存在则直接复用），`.forge-env/` 目录和 workspace 文件在 Pod 删除后依然保留在 PVC 中，新 Pod 挂载后立即可用。

### 6.8 完整重建时间线（以 Docker 为例）

```
T+0      用户发消息 → Acquire(session-A)
         容器健康 → 直接返回

T+0      user last_seen 更新

T+5min   idleTimeoutLoop 触发
         last_seen > 5min → docker rm -f {container-A}
         内存缓存清除，DB 记录保留（EnvironmentSpec 保留）

         → workspace/  目录保留在 volumes_root 中

T+30min  用户回来发消息 → Acquire(session-A)
         内存缓存 miss
         DB 查询 → 有记录，EnvironmentSpec 有值
         解析 storedEnv → {pip: ["requests", "pandas"], ...}
         容器已不在 → 删 DB 记录
         start(sessionID, newToken, quota, storedEnv)
           → docker run -e FORGE_PACKAGES_SPEC='...'
                        -v volumes_root/{hash}:/workspace
           → 容器内 sandbox-init: hash 匹配，跳过安装 ✓
           → 主进程: source .forge-env/env.sh → pip 包可用 ✓
         Upsert 新 DB 记录（新 containerID，原 EnvironmentSpec）
         ensureSessionResources() → 检查动态资源文件是否存在
         返回新 RemoteSandbox

         用户无感知，workspace 文件完好，包已就位
```

---

## 七、Project：面向用户的资源管理

### 6.1 为什么需要 Project

Anthropic 官方的资源与 session 关联由调用方的应用代码负责——因为其客户是开发者。ai-forge 面向终端用户，用户不写代码，需要一个平台层概念来承载"这个项目默认用哪些资源"。这个概念就是 **Project**。

### 6.2 实体层级

```
Tenant（租户）
  └── User（用户）
        └── Project（项目）
              ├── Git 仓库
              ├── Memory Store（跨 session 持久记忆）
              ├── Environment（绑定引用，非内嵌）
              ├── 默认参考文件
              └── Sessions（会话列表）
                    └── Session（会话）
```

### 6.3 资源配置的完整优先级链路

Session 最终拿到的资源配置由以下层级合并决定：

```
Session 创建时的覆盖参数          ← 最高优先级，临时覆盖当次
    ↑
Project 配置（git、memory、files）
    ↑
用户默认 Environment
    ↑
租户默认 Environment              ← 兜底
```

Project 提供默认值，不是锁定值。用户每次创建 Session 时可以临时覆盖（切换 branch、额外挂载文件），但不改变 Project 本身的配置。

### 6.4 典型使用流程

```
首次使用：创建项目（只需配置一次）
  1. 用户点"New Project"
  2. 关联 git 仓库（URL + branch + token）
  3. 平台自动创建同名 Memory Store
  4. 选择 Environment（从租户/个人环境列表选）

日常使用：在项目下开启 Session
  1. 用户进入某个 Project
  2. 点"New Session"
  3. Git repo、Memory Store、Environment 已自动填好
  4. 只需填：任务描述（以及可选的临时覆盖参数）
  5. 启动
```

---

## 八、工具执行路径

### 7.1 完整请求流程

以下展示一次工具调用从 Brain 到实际执行的完整链路：

```
用户消息
   │
   ▼
Brain 推理（LLM 决策）
   │  返回 tool_use: {name: "bash", input: "ls /workspace"}
   ▼
ToolRegistry（工具代理层）
   │  查找 "bash" → sandboxTool{inner: nil}（沙箱工具）
   │  从 ctx 取出当前 session 的 Sandbox
   ▼
Sandbox.Execute("bash", input)
   │
   ├─ LocalSandbox → 进程内直接调用工具函数
   │
   └─ RemoteSandbox → HTTP POST tool-server/execute
                          │
                          └─ 容器内 tool-server 执行
```

### 7.2 多租户并发隔离

在 serve 模式（多租户）下，每个请求到来时，Orchestration 层从 Pool 获取该 session 对应的 Sandbox，并注入到请求的 context 中。ToolRegistry 从 context 读取 Sandbox，实现请求级隔离：

```
请求 A（session-1）                 请求 B（session-2）
     │                                    │
     ▼                                    ▼
Pool.Acquire("session-1")        Pool.Acquire("session-2")
     │ → RemoteSandbox(容器A)          │ → RemoteSandbox(容器B)
     │                                    │
WithSandbox(ctx, sandboxA)      WithSandbox(ctx, sandboxB)
     │                                    │
     ▼                                    ▼
同一个 ToolRegistry                同一个 ToolRegistry
SandboxFromContext(ctx) = A     SandboxFromContext(ctx) = B
```

两个请求共享同一个 ToolRegistry（工具声明），但执行路径指向各自独立的容器。

---

## 九、安全设计

### 8.1 凭证隔离

Git token 和 API key 类凭证有严格的隔离机制：

| 凭证 | 处理方式 |
|------|---------|
| Git Token | 嵌入 HTTPS URL 格式（`https://x-token:TOKEN@host/path`），clone 完成后移除 origin remote，token 不进入容器 env |
| Anthropic API Key | 只在 Brain 调用 LLM 时通过函数参数传递，不经环境变量传给 Sandbox |
| Tool-server Token | 64 字节随机 hex，随容器生命周期生成，存 DB 用于跨 worker 复用认证 |

### 8.2 路径沙箱

所有 Agent 文件操作的根路径被限定在 `/workspace`，通过 `SafeJoin` 函数在写操作前进行路径规范化和前缀验证，防止路径遍历攻击（`../` 跳出目录）。

### 8.3 网络隔离

两档网络策略：

```
unrestricted（默认）
  容器可访问任何网络，无出站限制

limited（安全敏感场景）
  Docker: 自定义网络 + iptables，只允许 DNS + 白名单 IP 出站
  K8s:    NetworkPolicy，只允许 DNS + 白名单域名出站
```

---

## 十、文件持久化保证

容器/Pod 因 idle 超时被销毁后，用户最关心的问题是：**之前写的代码、生成的文件还在吗？**

答案是**在**，因为文件操作根路径 `/workspace` 对应持久卷，卷的生命周期绑定 Session 而非容器：

```
容器 overlay 层（临时）
  → OS、运行时，容器重建后清空，对用户不可见

/workspace（持久卷）
  → 代码文件、agent 产出、已安装的包，始终保留

/mnt/session/outputs（PVC 子目录）
  → 输出收集目录，同样持久
```

`/tmp` 等临时路径的文件会在容器重建后丢失。通过系统提示约定 agent 将重要产出写到 `/workspace`，规避这一问题。

---

## 十一、Outputs 收集

容器内 `/mnt/session/outputs/` 是约定的输出区域。Agent 将生成的代码、报告、构建产物写入该目录，由平台提供两个端点供外部收集：

```
GET  /sessions/{id}/outputs           列出输出文件列表
GET  /sessions/{id}/outputs/{path}    下载指定文件
```

在 Docker 部署中，outputs 目录对应 `volumes_root/{sessionHash}/outputs/`，Orchestration 进程可通过共享路径直接读取，无需依赖容器存活。在降级模式（共享存储不可用）下，该 API 返回 503。
