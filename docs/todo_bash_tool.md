# TODO: Bash Tool 实现

> 来源标记：`anthropic-docs-summary.md` → `====》 TODO`
> 优先级：**Phase 1（最高优先级，安全隔离必须实现）**

---

## 背景与价值

Bash Tool 让 Claude 能够执行 shell 命令，是 ai-coding Agent 最核心的工具能力之一。但它同时也是安全风险最高的工具，**必须在隔离环境中运行**。

核心特性：
- **持久 bash session**：同一 session 里环境变量、工作目录状态保持
- **API 是无状态的**：客户端负责维护 bash 子进程的生命周期
- **每次 API 调用固定增加 245 input tokens**：Bash tool 定义本身的固定成本
- **不支持交互命令**：vim、less、sudo 要密码等需要用户输入的命令不支持

---

## 工具定义

```json
{
  "type": "bash_20250124",
  "name": "bash"
}
```

Managed Agents 内置工具集（`agent_toolset_20260401`）已包含 bash，无需单独声明。

---

## 核心实现：持久 Bash Session

持久 bash session 的关键：**在客户端保持一个长期运行的 bash 子进程**，不要每次都新开一个。

### Python 实现

```python
import subprocess

class BashSession:
    def __init__(self):
        self.process = subprocess.Popen(
            ["bash"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT
        )
    
    def run(self, command: str) -> str:
        marker = "___END_OF_COMMAND___"
        # 写入命令 + 标记符
        self.process.stdin.write(f"{command}\necho {marker}\n".encode())
        self.process.stdin.flush()
        
        output = []
        while True:
            line = self.process.stdout.readline().decode()
            if marker in line:
                break
            output.append(line)
        return "".join(output)
    
    def restart(self):
        """对应 bash tool 的 restart: true"""
        self.process.kill()
        self.process = subprocess.Popen(
            ["bash"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT
        )

# 全程保活，跨多次 Claude API 调用复用
bash_session = BashSession()
```

### Go 实现（适合本项目）

```go
package bash

import (
    "bufio"
    "fmt"
    "io"
    "os/exec"
    "strings"
    "sync"
)

type BashSession struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout *bufio.Reader
    mu     sync.Mutex
}

func NewBashSession() (*BashSession, error) {
    cmd := exec.Command("bash")
    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, err
    }
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, err
    }
    cmd.Stderr = cmd.Stdout
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    return &BashSession{
        cmd:    cmd,
        stdin:  stdin,
        stdout: bufio.NewReader(stdout),
    }, nil
}

const endMarker = "___END_OF_CMD___"

func (s *BashSession) Run(command string) (string, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    _, err := fmt.Fprintf(s.stdin, "%s\necho %s\n", command, endMarker)
    if err != nil {
        return "", err
    }
    
    var sb strings.Builder
    for {
        line, err := s.stdout.ReadString('\n')
        if err != nil {
            return "", err
        }
        if strings.Contains(line, endMarker) {
            break
        }
        sb.WriteString(line)
    }
    return sb.String(), nil
}

func (s *BashSession) Restart() error {
    s.stdin.Close()
    s.cmd.Process.Kill()
    s.cmd.Wait()
    
    newSession, err := NewBashSession()
    if err != nil {
        return err
    }
    *s = *newSession
    return nil
}
```

---

## 与 Claude API 集成

当 Claude 调用 bash 工具时，客户端接收 `tool_use` block，执行后返回 `tool_result`：

```python
import anthropic
import json

client = anthropic.Anthropic()
bash = BashSession()

def handle_tool_call(tool_use_block):
    if tool_use_block.name == "bash":
        command = tool_use_block.input.get("command", "")
        
        # restart 操作
        if tool_use_block.input.get("restart"):
            bash.restart()
            return {"type": "tool_result", "tool_use_id": tool_use_block.id, "content": "Bash session restarted"}
        
        # 执行命令
        result = bash.run(command)
        return {
            "type": "tool_result",
            "tool_use_id": tool_use_block.id,
            "content": result
        }

# Agent 循环
messages = [{"role": "user", "content": "列出当前目录结构并统计 Go 文件数量"}]
while True:
    response = client.messages.create(
        model="claude-sonnet-4-6",
        max_tokens=4096,
        tools=[{"type": "bash_20250124", "name": "bash"}],
        messages=messages
    )
    
    if response.stop_reason == "end_turn":
        break
    
    if response.stop_reason == "tool_use":
        tool_results = []
        for block in response.content:
            if block.type == "tool_use":
                tool_results.append(handle_tool_call(block))
        
        messages.append({"role": "assistant", "content": response.content})
        messages.append({"role": "user", "content": tool_results})
```

---

## 安全要求（必须实现）

### 隔离环境

```bash
# 必须在 Docker 容器中运行 bash session
docker run --rm -it \
  --memory=512m \
  --cpus=1 \
  --read-only \
  --tmpfs /tmp \
  --network=none \   # 按需开放网络
  ubuntu:22.04 bash
```

### 命令白名单（不用黑名单）

```python
ALLOWED_COMMANDS = {
    "ls", "cat", "grep", "find", "git", "go", "python3",
    "npm", "node", "curl", "wget", "echo", "pwd", "cd",
    "mkdir", "cp", "mv", "rm", "touch", "chmod"
}

def validate_command(command: str) -> bool:
    first_word = command.strip().split()[0] if command.strip() else ""
    return first_word in ALLOWED_COMMANDS
```

### 资源约束

```bash
# 在容器启动脚本中设置
ulimit -t 30    # CPU 时间限制 30 秒
ulimit -v 524288  # 虚拟内存限制 512MB
ulimit -f 102400  # 文件大小限制 100MB
```

### Git Checkpoint（长期任务推荐）

```bash
# 每个关键步骤提交一次，便于回滚
git add -A && git commit -m "checkpoint: before refactor step 3"
```

---

## 注意事项

### 不支持的命令类型

- 交互式编辑器：`vim`, `nano`, `emacs`
- 分页器：`less`, `more`
- 需要密码的 sudo 命令
- GUI 应用程序

### Token 成本提醒

每次 API 调用固定 **+245 input tokens**（bash 工具定义本身的开销）。频繁调用时注意积累成本。

---

## 在 Managed Agents 中的说明

Managed Agents 的 bash 工具由 Anthropic 基础设施在托管容器（Ubuntu 22.04 LTS）中执行，客户端无需自行维护 bash session。安全隔离由 Anthropic 提供。

以下情况才需要自行实现：
- 使用 Messages API 手写 Agent 循环
- 需要访问本地或私有环境的 bash 能力
- 自定义工具通过 `agent.custom_tool_use` 事件触发的场景

---

## 实施步骤

- [ ] 确认 bash 工具的执行环境（Managed Agents 托管 vs 自建 Docker）
- [ ] 如果自建：创建 `BashSession` 类，维护持久子进程
- [ ] 实现命令白名单验证
- [ ] 配置 Docker 容器资源约束（memory/cpu/network）
- [ ] 在 Agent 循环中实现 `tool_use` 事件处理
- [ ] 对长期任务设置 git checkpoint 策略
- [ ] 测试 `restart: true` 行为是否正确重置状态
