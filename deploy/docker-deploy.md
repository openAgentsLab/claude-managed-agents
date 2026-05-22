# AI-Forge Docker 沙箱部署指南

每个用户会话独立一个 Docker 容器，进程和文件系统完全隔离。适合生产多用户环境，比 local 沙箱有更强的安全边界。

## 目录

- [环境要求](#环境要求)
- [架构说明](#架构说明)
- [共享存储要求](#共享存储要求)
- [构建沙箱镜像](#构建沙箱镜像)
- [依赖服务安装](#依赖服务安装)
- [配置](#配置)
- [运行服务](#运行服务)
- [多节点部署](#多节点部署)
- [Nginx 反向代理](#nginx-反向代理)
- [验证部署](#验证部署)

---

## 环境要求

| 组件 | 最低版本 | 说明 |
|------|---------|------|
| OS | Linux (Ubuntu 22.04+ / CentOS 8+) | 推荐 Ubuntu 22.04 LTS |
| Go | 1.26+ | 编译后端二进制 |
| Docker Engine | 24+ | 运行沙箱容器 |
| Node.js | 20+ | 构建前端静态文件 |
| PostgreSQL | 14+ | 生产环境数据库 |
| Nginx | 1.20+ | 反向代理 |
| 共享文件系统 | — | 多节点必须；单节点可用本地目录 |

---

## 架构说明

```
用户请求
  ↓
Nginx (反向代理 + 可选 Session Affinity)
  ↓
forge serve (forge 进程)
  ├─ 收到工具调用请求
  ├─ 按 sessionID 找到或新建 Docker 容器
  │    └─ docker run -v /volumes_root/<hash>:/workspace ...
  └─ 通过 HTTP 将工具调用转发给容器内 tool-server

/volumes_root/          ← 宿主机目录（单节点本地 / 多节点共享存储）
  <tenant>/<hash>/      ← 每个 session 独立 workspace 目录
    .forge-env/         ← sandbox-init 安装的 pip/npm/cargo 包
    outputs/            ← 工具写出的文件
    ...                 ← 用户工作区文件
```

每个 Docker 容器：
- 在宿主机随机端口（`127.0.0.1:0`）暴露 tool-server，仅本机可达
- 5 分钟无活动自动停止，workspace 保留；下次访问重建容器（秒级）
- `--rm` 参数：容器退出后自动清理，不留僵尸容器

---

## 共享存储要求

> **这是 Docker 沙箱最核心的运维约束，部署前必须阅读本节。**

`sandbox.volumes_root` 是所有 workspace 的根目录，forge 进程通过 bind mount 将其子目录挂载到容器里。forge 启动时会校验该目录存在且可访问；若未配置，Docker 沙箱拒绝启动。

### 单节点部署

只有一个 forge 实例，`volumes_root` 可以是本机任意目录：

```
volumes_root: /var/lib/ai-forge/volumes
```

无需共享文件系统，配置最简单。

### 多节点部署

多个 forge 实例（水平扩展）时，每个 Docker 容器只在启动它的那台宿主机上运行，endpoint 为 `http://localhost:PORT`，其他节点无法直连。

**如果一个会话的请求被路由到不同节点**，当前节点会发现 DB 里记录的容器端点不可达，从而在本节点重建容器。重建后能否恢复工作区取决于存储方式：

| 存储方式 | 节点切换后行为 | 推荐度 |
|---------|--------------|-------|
| **共享存储**（NFS / Ceph / 云 NAS） | 容器在新节点重建，workspace 文件完整保留，用户无感知 | ✅ 推荐 |
| **本地目录 + Session Affinity** | 请求始终打到同一节点，无重建开销；节点故障时 workspace 丢失 | ⚠️ 可用，有风险 |
| **本地目录，无 Session Affinity** | 节点切换时 workspace 全部丢失，工具调用上下文中断 | ❌ 不可用 |

**方案一：共享存储（推荐）**

所有 forge 节点挂载同一个网络文件系统，`volumes_root` 指向挂载点：

```bash
# 示例：挂载 NFS
sudo mount -t nfs nfs-server:/exports/forge-volumes /mnt/forge-volumes

# 或写入 /etc/fstab 持久化
nfs-server:/exports/forge-volumes  /mnt/forge-volumes  nfs  defaults,_netdev  0 0
```

forge 配置：
```yaml
sandbox:
  volumes_root: /mnt/forge-volumes
```

即使不配置 Session Affinity，session 也能在节点间无缝切换（有一次容器重建的冷启动开销，通常 1-3 秒）。建议同时配置 Session Affinity 以消除这个开销。

**方案二：Session Affinity（无共享存储时的替代方案）**

`volumes_root` 使用本地目录，通过负载均衡器将同一用户固定到同一节点：

```nginx
# Nginx — 按客户端 IP 哈希（简单场景）
upstream forge_backend {
    ip_hash;
    server 10.0.0.1:8080;
    server 10.0.0.2:8080;
}
```

> ⚠️ **注意**：ip_hash 在用户走 NAT 时效果差，且节点故障后 session 无法恢复。生产环境请优先使用共享存储。

---

## 构建沙箱镜像

沙箱镜像需要包含 `forge` 二进制（用于在容器内启动 tool-server）和 bash 等基础工具。

```dockerfile
# Dockerfile.toolserver
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    bash git curl wget python3 python3-pip nodejs npm \
    && rm -rf /var/lib/apt/lists/*

# 从构建产物复制 forge 二进制
COPY forge /forge
RUN chmod +x /forge

# 默认启动命令（实际由 DockerPool 通过 sh -c 覆盖）
CMD ["/forge", "tool-server", "--addr", ":7777"]
```

```bash
# 先编译 forge 二进制（Linux amd64）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o forge ./cmd/forge

# 构建并推送镜像
docker build -f Dockerfile.toolserver -t your-registry/ai-forge-toolserver:v1.0.0 .
docker push your-registry/ai-forge-toolserver:v1.0.0
```

---

## 依赖服务安装

### 安装 Docker Engine

```bash
# Ubuntu 22.04
curl -fsSL https://get.docker.com | sudo sh

# 将 forge 运行用户加入 docker 组（免 sudo）
sudo usermod -aG docker forge

# 验证
docker --version
```

### 安装 PostgreSQL

```bash
sudo apt install -y postgresql postgresql-contrib
sudo systemctl enable --now postgresql

sudo -u postgres psql <<'EOF'
CREATE USER forge WITH PASSWORD 'your_strong_password';
CREATE DATABASE forge OWNER forge;
GRANT ALL PRIVILEGES ON DATABASE forge TO forge;
EOF
```

### 安装 Go

```bash
wget https://go.dev/dl/go1.26.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

---

## 配置

### 创建目录

```bash
sudo mkdir -p /etc/ai-forge
sudo mkdir -p /var/lib/ai-forge/volumes    # volumes_root，单节点用本地目录
sudo mkdir -p /var/log/ai-forge
```

### 主配置文件 `/etc/ai-forge/forge.yaml`

```yaml
server:
  http_addr: ":8080"

log:
  level: "info"
  format: "json"

model:
  provider: "anthropic"
  api_key: "${ANTHROPIC_API_KEY}"
  base_url: "https://api.anthropic.com"
  model: "claude-sonnet-4-6"

storage:
  postgres:
    default:
      dsn: "postgres://forge:your_strong_password@localhost:5432/forge?sslmode=disable"

session:
  driver: "postgres.default"

memory:
  static:
    driver: "postgres.default"

sandbox:
  driver: "docker"
  volumes_root: "/var/lib/ai-forge/volumes"    # 必须配置；多节点须为共享存储挂载点
  options:
    image: "your-registry/ai-forge-toolserver:v1.0.0"   # 替换为实际镜像
    network: "bridge"          # 默认网络；limited 模式自动改为 "none"
    # forge_bin: ""            # 留空则自动使用当前 forge 可执行文件路径

tasks:
  driver: "postgres.default"
```

### 环境变量文件 `/etc/ai-forge/forge.env`

```bash
ANTHROPIC_API_KEY=sk-ant-your-key-here
```

```bash
sudo chmod 600 /etc/ai-forge/forge.env
sudo chmod 600 /etc/ai-forge/forge.yaml
```

---

## 运行服务

### 创建系统用户

```bash
sudo useradd --system --no-create-home --shell /bin/false forge

# forge 用户需要能执行 docker 命令
sudo usermod -aG docker forge

sudo chown -R forge:forge /var/lib/ai-forge /var/log/ai-forge /etc/ai-forge
```

### 编译并安装 forge 二进制

```bash
cd /path/to/ai-forge
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o /usr/local/bin/forge ./cmd/forge
```

### 手动启动（测试）

```bash
sudo -u forge forge serve \
  --config /etc/ai-forge/forge.yaml \
  --mcp-config /etc/ai-forge/forge.mcp.yaml
```

### systemd 单元文件 `/etc/systemd/system/ai-forge.service`

```ini
[Unit]
Description=AI-Forge Backend Service (Docker Sandbox)
After=network.target postgresql.service docker.service
Wants=postgresql.service docker.service

[Service]
Type=simple
User=forge
Group=forge
WorkingDirectory=/var/lib/ai-forge

EnvironmentFile=/etc/ai-forge/forge.env

ExecStart=/usr/local/bin/forge serve \
    --config /etc/ai-forge/forge.yaml \
    --mcp-config /etc/ai-forge/forge.mcp.yaml

Restart=on-failure
RestartSec=5s
KillMode=mixed
TimeoutStopSec=30

StandardOutput=append:/var/log/ai-forge/forge.log
StandardError=append:/var/log/ai-forge/forge-error.log

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/ai-forge /var/log/ai-forge /etc/ai-forge

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable ai-forge
sudo systemctl start ai-forge

sudo systemctl status ai-forge
sudo journalctl -u ai-forge -f
```

---

## 多节点部署

以两台节点为例（`10.0.0.1`、`10.0.0.2`），均按上述步骤部署 forge + Docker。

### 使用共享存储（推荐）

1. 准备共享文件系统（NFS 示例）：

```bash
# NFS 服务端
sudo apt install -y nfs-kernel-server
sudo mkdir -p /exports/forge-volumes
echo "/exports/forge-volumes  10.0.0.0/24(rw,sync,no_subtree_check,no_root_squash)" \
  | sudo tee -a /etc/exports
sudo exportfs -ra

# 两台 forge 节点各自挂载
sudo mkdir -p /mnt/forge-volumes
sudo mount -t nfs 10.0.0.100:/exports/forge-volumes /mnt/forge-volumes
# 写入 /etc/fstab 持久化：
# 10.0.0.100:/exports/forge-volumes  /mnt/forge-volumes  nfs  defaults,_netdev  0 0
```

2. 两台节点的 `forge.yaml` 中配置相同的 `volumes_root`：

```yaml
sandbox:
  volumes_root: "/mnt/forge-volumes"
```

3. Nginx 负载均衡（可不配置 Session Affinity，共享存储保障 workspace 一致）：

```nginx
upstream forge_backend {
    least_conn;
    server 10.0.0.1:8080;
    server 10.0.0.2:8080;
}
```

### 使用 Session Affinity（本地存储方案）

两台节点各自使用本地 `volumes_root`（如 `/var/lib/ai-forge/volumes`），必须在 Nginx 层面做 Session Affinity，将同一用户的请求固定到同一节点：

```nginx
upstream forge_backend {
    # 按客户端 IP 哈希
    ip_hash;
    server 10.0.0.1:8080;
    server 10.0.0.2:8080;
}
```

> ⚠️ 如果某节点宕机，该节点上所有用户的 workspace 将丢失，需重新建立对话上下文。

---

## Nginx 反向代理

```nginx
server {
    listen 80;
    server_name forge.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name forge.example.com;

    ssl_certificate     /etc/nginx/ssl/forge.crt;
    ssl_certificate_key /etc/nginx/ssl/forge.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    root /var/www/ai-forge;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://forge_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE 流式响应
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
        chunked_transfer_encoding on;
    }

    location /healthz {
        proxy_pass http://forge_backend;
        access_log off;
    }

    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff2?)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/ai-forge /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

---

## 验证部署

```bash
# 1. 检查后端健康状态
curl http://localhost:8080/healthz

# 2. 检查 volumes_root 权限
ls -la /var/lib/ai-forge/volumes/

# 3. 触发一次对话后检查是否创建了 session 目录和容器
ls /var/lib/ai-forge/volumes/
docker ps | grep forge

# 4. 检查后端日志
sudo tail -f /var/log/ai-forge/forge.log

# 5. 多节点：确认共享存储两端可见
# 在节点 1 上创建文件：
touch /mnt/forge-volumes/test-from-node1
# 在节点 2 上验证：
ls /mnt/forge-volumes/test-from-node1
```

---

## 常见问题

### forge 启动时报 `docker pool requires sandbox.volumes_root`

`sandbox.volumes_root` 未配置或为空。Docker 沙箱强依赖共享目录，必须设置该字段。

```yaml
sandbox:
  volumes_root: "/var/lib/ai-forge/volumes"   # 确保目录存在且 forge 用户可写
```

### forge 启动时报 `cannot access volumes_root`

目录不存在或 forge 用户没有写权限。

```bash
sudo mkdir -p /var/lib/ai-forge/volumes
sudo chown forge:forge /var/lib/ai-forge/volumes
```

### 容器无法启动：`docker: permission denied`

forge 用户未加入 `docker` 组：

```bash
sudo usermod -aG docker forge
# 重启服务使组成员生效
sudo systemctl restart ai-forge
```

### SSE 流式响应中断

确认 Nginx 配置了 `proxy_buffering off`，并在 `location /api/` 中设置了足够长的 `proxy_read_timeout`（建议 600s 以上）。
